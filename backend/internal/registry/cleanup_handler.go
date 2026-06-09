package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
)

// CleanupConfig represents the cleanup policy configuration.
type CleanupConfig struct {
	Enabled      bool     `json:"enabled"`
	KeepLastN    int      `json:"keep_last_n"`    // 0 = disabled
	MaxAgeDays   int      `json:"max_age_days"`   // 0 = disabled
	ExcludeTags  []string `json:"exclude_tags"`   // tags to never delete
	RunAt        string   `json:"run_at"`          // cron-like schedule (not implemented yet)
}

func DefaultCleanupConfig() CleanupConfig {
	return CleanupConfig{
		Enabled:     false,
		KeepLastN:   10,
		MaxAgeDays:  90,
		ExcludeTags: []string{"latest"},
	}
}

// ─── GetCleanupConfig ───────────────────────────────────────────────────────

func (h *Handler) GetCleanupConfig(w http.ResponseWriter, r *http.Request) {
	cfg := DefaultCleanupConfig()

	s, err := h.repo.GetSetting(r.Context(), "registry.cleanup")
	if err == nil && s.Value != "" {
		json.Unmarshal([]byte(s.Value), &cfg)
	}

	common.JSON(w, http.StatusOK, cfg)
}

// ─── UpdateCleanupConfig ────────────────────────────────────────────────────

func (h *Handler) UpdateCleanupConfig(w http.ResponseWriter, r *http.Request) {
	var cfg CleanupConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate
	if cfg.KeepLastN < 0 {
		cfg.KeepLastN = 0
	}
	if cfg.MaxAgeDays < 0 {
		cfg.MaxAgeDays = 0
	}
	if cfg.KeepLastN == 0 && cfg.MaxAgeDays == 0 {
		cfg.Enabled = false
	}

	payload, _ := json.Marshal(cfg)
	if err := h.repo.UpsertSetting(r.Context(), "registry.cleanup", string(payload),
		"Registry cleanup policy: keep last N tags, max age in days, and excluded tags."); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	h.logAudit(r, "registry.cleanup.config", "registry_cleanup", "",
		fmt.Sprintf("Updated cleanup policy: keep_last_n=%d, max_age=%dd, enabled=%v", cfg.KeepLastN, cfg.MaxAgeDays, cfg.Enabled))

	common.JSON(w, http.StatusOK, cfg)
}

// ─── RunCleanup ─────────────────────────────────────────────────────────────

type CleanupResult struct {
	ReposScanned int      `json:"repos_scanned"`
	TagsDeleted  int      `json:"tags_deleted"`
	SpaceFreed   int64    `json:"space_freed"`
	DeletedTags  []string `json:"deleted_tags"`
	Errors       []string `json:"errors,omitempty"`
}

func (h *Handler) RunCleanup(w http.ResponseWriter, r *http.Request) {
	// Load config
	cfg := DefaultCleanupConfig()
	s, err := h.repo.GetSetting(r.Context(), "registry.cleanup")
	if err == nil && s.Value != "" {
		json.Unmarshal([]byte(s.Value), &cfg)
	}

	if !cfg.Enabled && (cfg.KeepLastN > 0 || cfg.MaxAgeDays > 0) {
		cfg.Enabled = true
	}

	if !cfg.Enabled {
		common.Error(w, http.StatusBadRequest, "cleanup policy is not enabled")
		return
	}

	result := h.executeCleanup(r.Context(), cfg)

	claims := auth.GetClaims(r.Context())
	actor := ""
	if claims != nil {
		actor = claims.Email
	}

	h.logAudit(r, "registry.cleanup.run", "registry_cleanup", "",
		fmt.Sprintf("Cleanup completed: %d tags deleted, %d repos scanned, %s freed by %s",
			result.TagsDeleted, result.ReposScanned, formatBytes(result.SpaceFreed), actor))

	common.JSON(w, http.StatusOK, result)
}

func (h *Handler) executeCleanup(ctx context.Context, cfg CleanupConfig) CleanupResult {
	result := CleanupResult{}

	// Get all repos
	path := "/v2/_catalog?n=100"
	resp, err := h.zotRequest(path)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("catalog: %v", err))
		return result
	}
	defer resp.Body.Close()

	var catalog struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("parse catalog: %v", err))
		return result
	}

	excludeSet := make(map[string]bool)
	for _, t := range cfg.ExcludeTags {
		excludeSet[t] = true
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	for _, repoName := range catalog.Repositories {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			mu.Lock()
			result.ReposScanned++
			mu.Unlock()

			// Get tags for this repo
			tagResp, err := h.zotRequest("/v2/" + repo + "/tags/list")
			if err != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("%s: tags: %v", repo, err))
				mu.Unlock()
				return
			}
			defer tagResp.Body.Close()

			var tl struct {
				Tags []string `json:"tags"`
			}
			if err := json.NewDecoder(tagResp.Body).Decode(&tl); err != nil {
				return
			}

			if len(tl.Tags) == 0 {
				return
			}

			// Collect tag info (created date)
			type tagInfo struct {
				Name    string
				Created string
				Size    int64
				Digest  string
			}

			var tagInfos []tagInfo
			var tagWg sync.WaitGroup
			tagSem := make(chan struct{}, 5)
			var tagMu sync.Mutex

			for _, tag := range tl.Tags {
				tagWg.Add(1)
				go func(t string) {
					defer tagWg.Done()
					tagSem <- struct{}{}
					defer func() { <-tagSem }()

					mResp, err := h.zotRequest("/v2/" + repo + "/manifests/" + t)
					if err != nil {
						return
					}
					defer mResp.Body.Close()

					if mResp.StatusCode >= 400 {
						return
					}

					digest := mResp.Header.Get("Docker-Content-Digest")
					mBody, _ := io.ReadAll(mResp.Body)

					created := ""
					var size int64

					// Parse manifest for created date and size
					var manifest struct {
						Config struct {
							Digest string `json:"digest"`
							Size   int64  `json:"size"`
						} `json:"config"`
						Layers []struct {
							Size int64 `json:"size"`
						} `json:"layers"`
					}
					if err := json.Unmarshal(mBody, &manifest); err == nil {
						size = manifest.Config.Size
						for _, l := range manifest.Layers {
							size += l.Size
						}

						// Try to fetch config blob for created date
						if manifest.Config.Digest != "" {
							cResp, cErr := h.zotRequest("/v2/" + repo + "/blobs/" + manifest.Config.Digest)
							if cErr == nil && cResp.StatusCode < 400 {
								defer cResp.Body.Close()
								var cfgBlob struct {
									Created string `json:"created"`
								}
								if err := json.NewDecoder(cResp.Body).Decode(&cfgBlob); err == nil {
									created = cfgBlob.Created
								}
							}
						}
					}

					tagMu.Lock()
					tagInfos = append(tagInfos, tagInfo{Name: t, Created: created, Size: size, Digest: digest})
					tagMu.Unlock()
				}(tag)
			}
			tagWg.Wait()

			if len(tagInfos) == 0 {
				return
			}

			// Sort by created date descending (newest first)
			sort.Slice(tagInfos, func(i, j int) bool {
				return tagInfos[i].Created > tagInfos[j].Created
			})

			// Determine which tags to delete
			toDelete := []tagInfo{}

			if cfg.KeepLastN > 0 && len(tagInfos) > cfg.KeepLastN {
				for i := cfg.KeepLastN; i < len(tagInfos); i++ {
					if !excludeSet[tagInfos[i].Name] {
						toDelete = append(toDelete, tagInfos[i])
					}
				}
			}

			if cfg.MaxAgeDays > 0 {
				maxAge := time.Now().AddDate(0, 0, -cfg.MaxAgeDays)
				for _, ti := range tagInfos {
					if ti.Created != "" {
						createdTime, err := time.Parse(time.RFC3339, ti.Created)
						if err == nil && createdTime.Before(maxAge) {
							if !excludeSet[ti.Name] {
								// Avoid duplicates
								found := false
								for _, d := range toDelete {
									if d.Name == ti.Name {
										found = true
										break
									}
								}
								if !found {
									toDelete = append(toDelete, ti)
								}
							}
						}
					}
				}
			}

			// Delete tags
			for _, del := range toDelete {
				// Check tag protection
				protected, err := h.repo.IsTagProtected(ctx, repo, del.Name)
				if err == nil && protected {
					continue
				}

				if del.Digest != "" {
					if err := h.zotDelete("/v2/" + repo + "/manifests/" + del.Digest); err != nil {
						mu.Lock()
						result.Errors = append(result.Errors, fmt.Sprintf("%s:%s: %v", repo, del.Name, err))
						mu.Unlock()
						continue
					}

					mu.Lock()
					result.TagsDeleted++
					result.SpaceFreed += del.Size
					result.DeletedTags = append(result.DeletedTags, fmt.Sprintf("%s:%s", repo, del.Name))
					mu.Unlock()
				}
			}
		}(repoName)
	}
	wg.Wait()

	sort.Strings(result.DeletedTags)
	return result
}

// formatBytes is a helper reused from frontend logic
func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	size := float64(bytes)
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", bytes)
	}
	return fmt.Sprintf("%.1f %s", size, units[i])
}
