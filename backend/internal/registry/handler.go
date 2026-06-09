package registry

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
	"github.com/edsuwarna/anjungan/internal/config"
)

// --- Response types ---

type RepoInfo struct {
	Name      string `json:"name"`
	TagsCount int    `json:"tags_count"`
}

type TagInfo struct {
	Name      string         `json:"name"`
	Digest    string         `json:"digest"`
	Size      int64          `json:"size"`
	Created   string         `json:"created"`
	OS        string         `json:"os"`
	Arch      string         `json:"arch"`
	LayerSize int64          `json:"layer_size"`
	Layers    int            `json:"layers"`
	Platforms []PlatformInfo `json:"platforms,omitempty"`
}

type ImageDetail struct {
	Name      string           `json:"name"`
	Tag       string           `json:"tag"`
	Digest    string           `json:"digest"`
	Created   string           `json:"created"`
	OS        string           `json:"os"`
	Arch      string           `json:"arch"`
	Size      int64            `json:"size"`
	Layers    int              `json:"layers"`
	Config    *ImageCfg        `json:"config"`
	LayersArr []LayerInfo      `json:"layers_arr"`
	History   []HistStep       `json:"history"`
	Platforms []PlatformDetail `json:"platforms,omitempty"`
}

type ImageCfg struct {
	Cmd           []string        `json:"cmd"`
	Entrypoint    []string        `json:"entrypoint,omitempty"`
	Workdir       string          `json:"workdir"`
	User          string          `json:"user"`
	ExposedPorts  []string        `json:"exposed_ports"`
	Env           []EnvVar        `json:"env"`
	Labels        []EnvVar        `json:"labels"`
	Volumes       []string        `json:"volumes"`
}

type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type LayerInfo struct {
	Digest      string `json:"digest"`
	Size        int64  `json:"size"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

type HistStep struct {
	Created string `json:"created"`
	Command string `json:"command"`
	Empty   bool   `json:"empty"`
}

type PlatformInfo struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type PlatformDetail struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Variant string `json:"variant,omitempty"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

// --- Handler ---

type Handler struct {
	cfg  config.RegistryConfig
	repo *db.Repository
}

func NewHandler(cfg config.RegistryConfig, repo *db.Repository) *Handler {
	return &Handler{cfg: cfg, repo: repo}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/config", h.Config)
	r.Get("/my-credentials", h.MyCredentials)
	r.Post("/my-credentials/reset-password", h.ResetMyPassword)
	r.Get("/repos", h.ListRepos)
	r.Get("/repos/{name}/tags", h.ListTags)
	r.Get("/repos/{name}/{tag}", h.ImageDetail)
	r.Delete("/repos/{name}/manifests/{digest}", h.requireAdmin(h.DeleteManifest))
	r.Delete("/repos/{name}/tags/{tag}", h.requireAdmin(h.DeleteTag))
	r.Post("/gc", h.requireAdmin(h.TriggerGC))
	r.Get("/users", h.requireAdmin(h.ListUsers))
	r.Post("/users", h.requireAdmin(h.CreateUser))
	r.Put("/users/{id}", h.requireAdmin(h.UpdateUser))
	r.Delete("/users/{id}", h.requireAdmin(h.DeleteUser))
	r.Post("/users/{id}/reset-password", h.requireAdmin(h.ResetUserPassword))
	r.Post("/sync-htpasswd", h.requireAdmin(h.SyncHtpasswd))
	r.Mount("/webhooks", h.webhookRoutes())
	r.Mount("/protections", h.tagProtectionRoutes())
	return r
}

// Config returns the registry URL visible to all authenticated users.
func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	common.JSON(w, http.StatusOK, map[string]string{
		"url": h.cfg.ExternalURL,
	})
}

// MyCredentials returns the current user's personal registry credentials.
// Auto-creates a linked registry user if none exists.
func (h *Handler) MyCredentials(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	newUsername := strings.ToLower(claims.Email)

	// Try to find existing linked registry user
	existing, err := h.repo.GetRegistryUserByAnjunganUserID(r.Context(), claims.UserID)
	if err == nil && existing != nil {
		// Migrate username if it changed (old u- format or email update)
		if existing.Username != newUsername {
			oldUsername := existing.Username
			existing.Username = newUsername
			if err := h.repo.UpdateRegistryUser(r.Context(), existing); err != nil {
				common.Error(w, http.StatusInternalServerError, "failed to update username")
				return
			}
			h.syncZotHtpasswd(r.Context())
			h.logAudit(r, "registry.my-credentials.migrate", "registry_user", existing.ID,
				fmt.Sprintf("Migrated registry username from %s to %s", oldUsername, newUsername))
		}

		// Return credentials (we don't store plaintext, so return a reset flow hint)
		common.JSON(w, http.StatusOK, map[string]string{
			"url":      h.cfg.ExternalURL,
			"username": existing.Username,
			"password": "", // not stored in plaintext
		})
		return
	}

	// Auto-create a linked registry user
	now := time.Now()
	password := generatePassword(16)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to generate credentials")
		return
	}

	regUser := &model.RegistryUser{
		ID:             uuid.New().String(),
		Username:       newUsername,
		PasswordHash:   string(hash),
		Role:           "deploy",
		AnjunganUserID: claims.UserID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.repo.CreateRegistryUser(r.Context(), regUser); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create registry user")
		return
	}

	// Sync htpasswd so Zot picks up the new user
	h.syncZotHtpasswd(r.Context())

	h.logAudit(r, "registry.my-credentials.create", "registry_user", regUser.ID,
		fmt.Sprintf("Created registry user %s for user %s", regUser.Username, claims.Email))

	common.JSON(w, http.StatusOK, map[string]string{
		"url":      h.cfg.ExternalURL,
		"username": regUser.Username,
		"password": password,
	})
}

// ResetMyPassword allows the current user to reset their own registry password.
func (h *Handler) ResetMyPassword(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Password = strings.TrimSpace(req.Password)
	if req.Password == "" {
		common.Error(w, http.StatusBadRequest, "password is required")
		return
	}
	if len(req.Password) < 8 {
		common.Error(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	existing, err := h.repo.GetRegistryUserByAnjunganUserID(r.Context(), claims.UserID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "no registry credentials found. Generate them first.")
		return
	}

	// Migrate username if needed (old u- format or email change)
	newUsername := strings.ToLower(claims.Email)
	if existing.Username != newUsername {
		oldUsername := existing.Username
		existing.Username = newUsername
		if err := h.repo.UpdateRegistryUser(r.Context(), existing); err != nil {
			common.Error(w, http.StatusInternalServerError, "failed to update username")
			return
		}
		h.logAudit(r, "registry.my-credentials.migrate", "registry_user", existing.ID,
			fmt.Sprintf("Migrated registry username from %s to %s", oldUsername, newUsername))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := h.repo.UpdateRegistryUserPassword(r.Context(), existing.ID, string(hash)); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to reset password")
		return
	}

	h.syncZotHtpasswd(r.Context())

	h.logAudit(r, "registry.my-credentials.reset", "registry_user", existing.ID,
		"User reset their own registry password")

	common.JSON(w, http.StatusOK, map[string]string{
		"message":  "password reset",
		"username": existing.Username,
	})
}

// generatePassword creates a random alphanumeric password of given length.
func generatePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// requireAdmin wraps a handler to enforce admin role.
func (h *Handler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetClaims(r.Context())
		if claims == nil || claims.Role != "admin" {
			common.Error(w, http.StatusForbidden, "admin access required")
			return
		}
		next(w, r)
	}
}

// --- helpers ---

func (h *Handler) zotRequest(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", h.cfg.URL+path, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(h.cfg.AdminUser, h.cfg.AdminPass)
	req.Header.Set("User-Agent", "anjungan/1.0")
	return http.DefaultClient.Do(req)
}

func (h *Handler) zotDelete(path string) error {
	req, err := http.NewRequest("DELETE", h.cfg.URL+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(h.cfg.AdminUser, h.cfg.AdminPass)
	req.Header.Set("User-Agent", "anjungan/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zot delete %s: %s — %s", path, resp.Status, string(body))
	}
	return nil
}

// --- Endpoints ---

// ListRepos returns repositories from Zot catalog with pagination.
// Supports query params: n (limit, default 50), last (last repo name cursor).
func (h *Handler) ListRepos(w http.ResponseWriter, r *http.Request) {
	n := r.URL.Query().Get("n")
	last := r.URL.Query().Get("last")
	path := "/v2/_catalog"
	if n != "" || last != "" {
		path += "?"
		if n != "" {
			path += "n=" + n
		}
		if last != "" {
			if n != "" {
				path += "&"
			}
			path += "last=" + last
		}
	}
	resp, err := h.zotRequest(path)
	if err != nil {
		common.Errorf(w, http.StatusBadGateway, "registry connection: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		common.Errorf(w, http.StatusBadGateway, "registry error: %s — %s", resp.Status, string(body))
		return
	}

	var catalog struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		common.Errorf(w, http.StatusInternalServerError, "parse catalog: %v", err)
		return
	}

	// Parse Link header for next cursor
	nextLast := parseNextLink(resp.Header.Get("Link"))

	// Enrich with tag count per repo (parallel fetching)
	type repoResult struct {
		name string
		tags int
	}
	ch := make(chan repoResult, len(catalog.Repositories))
	for _, name := range catalog.Repositories {
		go func(n string) {
			tagsResp, err := h.zotRequest("/v2/" + n + "/tags/list")
			if err != nil {
				ch <- repoResult{n, 0}
				return
			}
			defer tagsResp.Body.Close()
			var tl struct {
				Tags []string `json:"tags"`
			}
			if err := json.NewDecoder(tagsResp.Body).Decode(&tl); err != nil {
				ch <- repoResult{n, 0}
				return
			}
			ch <- repoResult{n, len(tl.Tags)}
		}(name)
	}

	results := make([]repoResult, len(catalog.Repositories))
	for i := 0; i < len(catalog.Repositories); i++ {
		results[i] = <-ch
	}
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].name) < strings.ToLower(results[j].name)
	})

	repos := make([]RepoInfo, 0, len(results))
	for _, res := range results {
		repos = append(repos, RepoInfo{Name: res.name, TagsCount: res.tags})
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"repos":     repos,
		"next_last": nextLast,
	})
}

// ListTags returns tags for a repository with digest & size info.
func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Build path with optional pagination params
	n := r.URL.Query().Get("n")
	last := r.URL.Query().Get("last")
	path := "/v2/" + name + "/tags/list"
	if n != "" || last != "" {
		path += "?"
		if n != "" {
			path += "n=" + n
		}
		if last != "" {
			if n != "" {
				path += "&"
			}
			path += "last=" + last
		}
	}

	resp, err := h.zotRequest(path)
	if err != nil {
		common.Errorf(w, http.StatusBadGateway, "registry connection: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		common.Error(w, http.StatusNotFound, "repository not found")
		return
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		common.Errorf(w, http.StatusBadGateway, "registry error: %s — %s", resp.Status, string(body))
		return
	}

	var tagsResp struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		common.Errorf(w, http.StatusInternalServerError, "parse tags: %v", err)
		return
	}

	// Fetch manifest info for each tag (parallel)
	type tagResult struct {
		TagInfo
		err error
	}
	ch := make(chan tagResult, len(tagsResp.Tags))

	fetchTag := func(tag string) {
		info := TagInfo{Name: tag}
		mResp, err := h.zotRequest("/v2/" + name + "/manifests/" + tag)
		if err != nil {
			ch <- tagResult{err: err}
			return
		}
		defer mResp.Body.Close()

		if mResp.StatusCode >= 400 {
			ch <- tagResult{TagInfo: TagInfo{Name: tag}}
			return
		}

		info.Digest = mResp.Header.Get("Docker-Content-Digest")
		mBody, _ := io.ReadAll(mResp.Body)

		// Detect media type for multi-arch support
		var mtCheck struct {
			MediaType string `json:"mediaType"`
		}
		json.Unmarshal(mBody, &mtCheck)

		if mtCheck.MediaType == "application/vnd.oci.image.index.v1+json" ||
			mtCheck.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" {
			// Multi-arch image index / manifest list
			var idx struct {
				Manifests []struct {
					MediaType string `json:"mediaType"`
					Size      int64  `json:"size"`
					Digest    string `json:"digest"`
					Platform  struct {
						Architecture string `json:"architecture"`
						OS           string `json:"os"`
						Variant      string `json:"variant,omitempty"`
					} `json:"platform"`
				} `json:"manifests"`
			}
			if err := json.Unmarshal(mBody, &idx); err != nil {
				ch <- tagResult{TagInfo: info}
				return
			}

			info.OS = "multi"
			info.Platforms = make([]PlatformInfo, 0, len(idx.Manifests))
			var archs []string
			var totalSize int64
			for _, m := range idx.Manifests {
				info.Platforms = append(info.Platforms, PlatformInfo{
					OS: m.Platform.OS, Arch: m.Platform.Architecture,
				})
				if m.Platform.Architecture != "" {
					archs = append(archs, m.Platform.Architecture)
				}
				totalSize += m.Size
			}
			info.Arch = strings.Join(archs, ", ")
			info.Size = totalSize

			// Get created date from first sub-manifest
			if len(idx.Manifests) > 0 {
				first := idx.Manifests[0].Digest
				subResp, subErr := h.zotRequest("/v2/" + name + "/manifests/" + first)
				if subErr == nil && subResp.StatusCode < 400 {
					defer subResp.Body.Close()
					var subManifest struct {
						Config struct {
							Digest string `json:"digest"`
						} `json:"config"`
						Layers []struct {
							Size int64 `json:"size"`
						} `json:"layers"`
					}
					if err := json.NewDecoder(subResp.Body).Decode(&subManifest); err == nil {
						info.Layers = len(subManifest.Layers)
						if subManifest.Config.Digest != "" {
							cResp, cErr := h.zotRequest("/v2/" + name + "/blobs/" + subManifest.Config.Digest)
							if cErr == nil && cResp.StatusCode < 400 {
								defer cResp.Body.Close()
								var cfgBlob struct {
									Created string `json:"created"`
								}
								if err := json.NewDecoder(cResp.Body).Decode(&cfgBlob); err == nil {
									info.Created = cfgBlob.Created
								}
							}
						}
					}
				}
			}

			ch <- tagResult{TagInfo: info}
			return
		}

		// Single-arch manifest
		var manifest struct {
			MediaType string `json:"mediaType"`
			Config    struct {
				MediaType string `json:"mediaType"`
				Size      int64  `json:"size"`
				Digest    string `json:"digest"`
			} `json:"config"`
			Layers []struct {
				MediaType string `json:"mediaType"`
				Size      int64  `json:"size"`
				Digest    string `json:"digest"`
			} `json:"layers"`
		}
		if err := json.Unmarshal(mBody, &manifest); err != nil {
			ch <- tagResult{TagInfo: info}
			return
		}

		info.Layers = len(manifest.Layers)
		var totalLayerSize int64
		for _, l := range manifest.Layers {
			totalLayerSize += l.Size
		}
		info.Size = totalLayerSize
		if manifest.Config.Size > 0 {
			info.Size += manifest.Config.Size
		}

		// Fetch config blob for created/os/arch
		if manifest.Config.Digest != "" {
			cResp, cErr := h.zotRequest("/v2/" + name + "/blobs/" + manifest.Config.Digest)
			if cErr == nil && cResp.StatusCode < 400 {
				defer cResp.Body.Close()
				var cfgBlob struct {
					Created string `json:"created"`
					OS      string `json:"os"`
					Arch    string `json:"architecture"`
				}
				if err := json.NewDecoder(cResp.Body).Decode(&cfgBlob); err == nil {
					info.Created = cfgBlob.Created
					info.OS = cfgBlob.OS
					info.Arch = cfgBlob.Arch
				}
			}
		}

		ch <- tagResult{TagInfo: info}
	}

	for _, tag := range tagsResp.Tags {
		go fetchTag(tag)
	}

	tags := make([]TagInfo, 0, len(tagsResp.Tags))
	for i := 0; i < len(tagsResp.Tags); i++ {
		res := <-ch
		if res.err == nil {
			tags = append(tags, res.TagInfo)
		}
	}

	// Sort: latest first, then by created desc
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Name == "latest" {
			return true
		}
		if tags[j].Name == "latest" {
			return false
		}
		return tags[i].Created > tags[j].Created
	})

	// Parse Link header for next cursor
	nextLast := parseNextLink(resp.Header.Get("Link"))

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"name":      name,
		"tags":      tags,
		"next_last": nextLast,
	})
}

// ImageDetail returns full image details: manifest layers + config blob.
func (h *Handler) ImageDetail(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	tag := chi.URLParam(r, "tag")

	// 1. Fetch manifest
	mResp, err := h.zotRequest("/v2/" + name + "/manifests/" + tag)
	if err != nil {
		common.Errorf(w, http.StatusBadGateway, "registry connection: %v", err)
		return
	}
	defer mResp.Body.Close()

	if mResp.StatusCode == 404 {
		common.Error(w, http.StatusNotFound, "image not found")
		return
	}
	if mResp.StatusCode >= 400 {
		body, _ := io.ReadAll(mResp.Body)
		common.Errorf(w, http.StatusBadGateway, "registry error: %s — %s", mResp.Status, string(body))
		return
	}

	digest := mResp.Header.Get("Docker-Content-Digest")
	mBody, _ := io.ReadAll(mResp.Body)

	var manifest struct {
		MediaType string `json:"mediaType"`
		SchemaVer int    `json:"schemaVersion"`
		Config    struct {
			MediaType string `json:"mediaType"`
			Size      int64  `json:"size"`
			Digest    string `json:"digest"`
		} `json:"config"`
		Layers []struct {
			MediaType string `json:"mediaType"`
			Size      int64  `json:"size"`
			Digest    string `json:"digest"`
			URLs      []string `json:"urls,omitempty"`
		} `json:"layers"`
		Annotations map[string]string `json:"annotations,omitempty"`
	}
	if err := json.Unmarshal(mBody, &manifest); err != nil {
		common.Errorf(w, http.StatusInternalServerError, "parse manifest: %v", err)
		return
	}

	detail := ImageDetail{
		Name:   name,
		Tag:    tag,
		Digest: digest,
	}

	// Check if multi-arch index (OCI index or Docker manifest list)
	if manifest.MediaType == "application/vnd.oci.image.index.v1+json" ||
		manifest.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" {
		var idx struct {
			Manifests []struct {
				MediaType string `json:"mediaType"`
				Size      int64  `json:"size"`
				Digest    string `json:"digest"`
				Platform  struct {
					Architecture string `json:"architecture"`
					OS           string `json:"os"`
					Variant      string `json:"variant,omitempty"`
				} `json:"platform"`
			} `json:"manifests"`
		}
		if err := json.Unmarshal(mBody, &idx); err != nil {
			common.JSON(w, http.StatusOK, detail)
			return
		}

		detail.OS = "multi"
		var archs []string
		var totalSize int64
		for _, m := range idx.Manifests {
			detail.Platforms = append(detail.Platforms, PlatformDetail{
				OS:      m.Platform.OS,
				Arch:    m.Platform.Architecture,
				Variant: m.Platform.Variant,
				Digest:  m.Digest,
				Size:    m.Size,
			})
			if m.Platform.Architecture != "" {
				archs = append(archs, m.Platform.Architecture)
			}
			totalSize += m.Size
		}
		detail.Arch = strings.Join(archs, ", ")
		detail.Size = totalSize

		// Fetch first sub-manifest for detailed info (config, layers, history)
		if len(idx.Manifests) > 0 {
			first := idx.Manifests[0].Digest
			subResp, subErr := h.zotRequest("/v2/" + name + "/manifests/" + first)
			if subErr == nil && subResp.StatusCode < 400 {
				defer subResp.Body.Close()
				var subManifest struct {
					MediaType string `json:"mediaType"`
					Config    struct {
						Digest string `json:"digest"`
					} `json:"config"`
					Layers []struct {
						MediaType string `json:"mediaType"`
						Size      int64  `json:"size"`
						Digest    string `json:"digest"`
					} `json:"layers"`
				}
				if err := json.NewDecoder(subResp.Body).Decode(&subManifest); err == nil {
					detail.Layers = len(subManifest.Layers)
					var subTotal int64
					for _, l := range subManifest.Layers {
						subTotal += l.Size
						detail.LayersArr = append(detail.LayersArr, LayerInfo{
							Digest:      shortenDigest(l.Digest),
							Size:        l.Size,
							Command:     layerCommand(l.MediaType),
							Description: layerDescription(l.MediaType, l.Digest),
						})
					}
					if subManifest.Config.Digest != "" {
						h.populateConfig(&detail, name, subManifest.Config.Digest)
					}
				}
			}
		}

		common.JSON(w, http.StatusOK, detail)
		return
	}

	var totalSize int64
	detail.LayersArr = make([]LayerInfo, 0, len(manifest.Layers))
	for _, l := range manifest.Layers {
		totalSize += l.Size
		desc := layerDescription(l.MediaType, l.Digest)
		cmd := layerCommand(l.MediaType)
		detail.LayersArr = append(detail.LayersArr, LayerInfo{
			Digest:      shortenDigest(l.Digest),
			Size:        l.Size,
			Command:     cmd,
			Description: desc,
		})
	}
	detail.Size = totalSize
	if manifest.Config.Size > 0 {
		detail.Size += manifest.Config.Size
	}
	detail.Layers = len(manifest.Layers)

	// 2. Fetch config blob
	if manifest.Config.Digest != "" {
		h.populateConfig(&detail, name, manifest.Config.Digest)
	}

	common.JSON(w, http.StatusOK, detail)
}

// populateConfig fetches the image config blob and fills in ImageDetail fields.
func (h *Handler) populateConfig(detail *ImageDetail, name, digest string) {
	cResp, cErr := h.zotRequest("/v2/" + name + "/blobs/" + digest)
	if cErr != nil || cResp.StatusCode >= 400 {
		return
	}
	defer cResp.Body.Close()
	var cfgBlob struct {
		Created      string `json:"created"`
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Config       struct {
			Cmd           []string              `json:"Cmd"`
			Entrypoint    []string              `json:"Entrypoint"`
			WorkingDir    string                `json:"WorkingDir"`
			User          string                `json:"User"`
			ExposedPorts  map[string]interface{} `json:"ExposedPorts"`
			Env           []string              `json:"Env"`
			Labels        map[string]string     `json:"Labels"`
			Volumes       map[string]interface{} `json:"Volumes"`
		} `json:"config"`
		History []struct {
			Created    string `json:"created"`
			CreatedBy  string `json:"created_by"`
			EmptyLayer bool   `json:"empty_layer"`
			Comment    string `json:"comment,omitempty"`
		} `json:"history"`
	}
	if err := json.NewDecoder(cResp.Body).Decode(&cfgBlob); err != nil {
		return
	}

	detail.Created = cfgBlob.Created
	if detail.OS == "" {
		detail.OS = cfgBlob.OS
	}
	if detail.Arch == "" {
		detail.Arch = cfgBlob.Architecture
	}

	cfg := &ImageCfg{}
	cfg.Cmd = cfgBlob.Config.Cmd
	cfg.Entrypoint = cfgBlob.Config.Entrypoint
	cfg.Workdir = cfgBlob.Config.WorkingDir
	cfg.User = cfgBlob.Config.User

	for p := range cfgBlob.Config.ExposedPorts {
		cfg.ExposedPorts = append(cfg.ExposedPorts, p)
	}
	sort.Strings(cfg.ExposedPorts)

	for v := range cfgBlob.Config.Volumes {
		cfg.Volumes = append(cfg.Volumes, v)
	}
	sort.Strings(cfg.Volumes)

	for _, e := range cfgBlob.Config.Env {
		if parts := strings.SplitN(e, "=", 2); len(parts) == 2 {
			cfg.Env = append(cfg.Env, EnvVar{Key: parts[0], Value: parts[1]})
		}
	}

	for k, v := range cfgBlob.Config.Labels {
		cfg.Labels = append(cfg.Labels, EnvVar{Key: k, Value: v})
	}
	sort.Slice(cfg.Labels, func(i, j int) bool {
		return cfg.Labels[i].Key < cfg.Labels[j].Key
	})

	detail.Config = cfg

	for _, h := range cfgBlob.History {
		cmd := h.CreatedBy
		if cmd == "" {
			cmd = h.Comment
		}
		detail.History = append(detail.History, HistStep{
			Created: h.Created,
			Command: cmd,
			Empty:   h.EmptyLayer,
		})
	}
}

// DeleteManifest deletes an image manifest by digest.
func (h *Handler) DeleteManifest(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	digest := chi.URLParam(r, "digest")

	if err := h.zotDelete("/v2/" + name + "/manifests/" + digest); err != nil {
		common.Errorf(w, http.StatusBadGateway, "delete failed: %v", err)
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]string{
			"repo":   name,
			"digest": digest,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"registry.delete", "registry_image", name,
			fmt.Sprintf("Deleted image %s@%s", name, digest),
			json.RawMessage(meta))
	}

	common.JSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
	})
}

// DeleteTag deletes a tag by name — fetches the digest first, then deletes by digest.
func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	tag := chi.URLParam(r, "tag")

	// 1. Fetch manifest to get the digest
	mResp, err := h.zotRequest("/v2/" + name + "/manifests/" + tag)
	if err != nil {
		common.Errorf(w, http.StatusBadGateway, "registry connection: %v", err)
		return
	}
	defer mResp.Body.Close()

	if mResp.StatusCode == 404 {
		common.Error(w, http.StatusNotFound, "tag not found")
		return
	}
	if mResp.StatusCode >= 400 {
		body, _ := io.ReadAll(mResp.Body)
		common.Errorf(w, http.StatusBadGateway, "registry error: %s — %s", mResp.Status, string(body))
		return
	}

	digest := mResp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		common.Error(w, http.StatusInternalServerError, "no digest in response")
		return
	}

	// Check tag protection
	if !h.checkTagProtectionBeforeDelete(w, r, name, tag) {
		return
	}

	// 2. Delete by digest
	if err := h.zotDelete("/v2/" + name + "/manifests/" + digest); err != nil {
		common.Errorf(w, http.StatusBadGateway, "delete failed: %v", err)
		return
	}

	// 3. Audit log
	actor := ""
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]string{
			"repo":   name,
			"tag":    tag,
			"digest": digest,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"registry.delete", "registry_image", name,
			fmt.Sprintf("Deleted image %s:%s (%s)", name, tag, shortenDigest(digest)),
			json.RawMessage(meta))
		actor = claims.Email
	}

	// 4. Fire webhook event (async)
	go h.fireDeleteEvent(context.Background(), name, tag, digest, actor)

	common.JSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"tag":    tag,
		"digest": digest,
	})
}

// --- helpers ---

func shortenDigest(d string) string {
	if len(d) > 19 {
		return d[:19] + "..."
	}
	return d
}

func layerDescription(mediaType, digest string) string {
	switch {
	case strings.Contains(mediaType, "foreign"):
		return "Foreign layer (shared dependency)"
	case strings.Contains(mediaType, "non-distributable"):
		return "Non-distributable layer"
	default:
		return "Filesystem layer " + shortenDigest(digest)
	}
}

func layerCommand(mediaType string) string {
	switch {
	case strings.Contains(mediaType, "foreign"):
		return "FOREIGN"
	default:
		return "LAYER"
	}
}

// parseNextLink extracts the "last" cursor from a Link header like:
// </v2/_catalog?n=10&last=myrepo>; rel="next"
// Returns empty string if no next link is found.
func parseNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	// Check if this is a "next" rel link
	if !strings.Contains(linkHeader, `rel="next"`) {
		return ""
	}
	// Extract the URI between < >
	start := strings.Index(linkHeader, "<")
	end := strings.Index(linkHeader, ">")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	uri := linkHeader[start+1 : end]
	// Extract the "last" query param
	if idx := strings.Index(uri, "last="); idx >= 0 {
		last := uri[idx+5:]
		if amp := strings.Index(last, "&"); amp >= 0 {
			last = last[:amp]
		}
		return last
	}
	return ""
}

// ListUsers returns the configured registry users from the database.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListRegistryUsers(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list registry users")
		return
	}
	resp := make([]model.RegistryUserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, u.ToResponse())
	}
	common.JSON(w, http.StatusOK, resp)
}

// TriggerGC attempts to trigger garbage collection on the Zot registry.
// Tries the Zot GC API endpoint first; falls back to reminding about auto-GC schedule.
func (h *Handler) TriggerGC(w http.ResponseWriter, r *http.Request) {
	// Try Zot's built-in GC endpoint (available in newer versions)
	req, err := http.NewRequest("POST", h.cfg.URL+"/v2/_zot/gc", nil)
	if err == nil {
		req.SetBasicAuth(h.cfg.AdminUser, h.cfg.AdminPass)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, gcErr := client.Do(req)
		if gcErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode < 400 {
				common.JSON(w, http.StatusOK, map[string]interface{}{
					"status":  "gc_triggered",
					"message": "Garbage collection triggered on Zot registry",
				})
				return
			}
		}
	}

	// If direct GC is not available, report auto-GC schedule
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"status":       "auto_gc",
		"message":      "Direct GC not available. Zot runs automatic GC every 24h. Restart the zot container to trigger immediate GC.",
		"gc_interval":  "24h",
		"gc_delay":     "168h",
		"restart_cmd":  "docker restart anjungan-zot",
	})
}

// Ensure used imports
var _ = time.Now
