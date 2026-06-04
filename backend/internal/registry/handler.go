package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/config"
)

// --- Response types ---

type RepoInfo struct {
	Name      string `json:"name"`
	TagsCount int    `json:"tags_count"`
}

type TagInfo struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	Created   string `json:"created"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	LayerSize int64  `json:"layer_size"`
	Layers    int    `json:"layers"`
}

type ImageDetail struct {
	Name      string      `json:"name"`
	Tag       string      `json:"tag"`
	Digest    string      `json:"digest"`
	Created   string      `json:"created"`
	OS        string      `json:"os"`
	Arch      string      `json:"arch"`
	Size      int64       `json:"size"`
	Layers    int         `json:"layers"`
	Config    *ImageCfg   `json:"config"`
	LayersArr []LayerInfo `json:"layers_arr"`
	History   []HistStep  `json:"history"`
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

// --- Handler ---

type Handler struct {
	cfg config.RegistryConfig
}

func NewHandler(cfg config.RegistryConfig) *Handler {
	return &Handler{cfg: cfg}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/repos", h.ListRepos)
	r.Get("/repos/{name}/tags", h.ListTags)
	r.Get("/repos/{name}/{tag}", h.ImageDetail)
	r.Delete("/repos/{name}/manifests/{digest}", h.DeleteManifest)
	return r
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

// ListRepos returns all repositories from Zot catalog.
func (h *Handler) ListRepos(w http.ResponseWriter, r *http.Request) {
	resp, err := h.zotRequest("/v2/_catalog")
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

	common.JSON(w, http.StatusOK, repos)
}

// ListTags returns tags for a repository with digest & size info.
func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	resp, err := h.zotRequest("/v2/" + name + "/tags/list")
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

		// Try OCI manifest first (v2)
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

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"name": name,
		"tags": tags,
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
		cResp, cErr := h.zotRequest("/v2/" + name + "/blobs/" + manifest.Config.Digest)
		if cErr == nil && cResp.StatusCode < 400 {
			defer cResp.Body.Close()
			var cfgBlob struct {
				Created      string `json:"created"`
				OS           string `json:"os"`
				Architecture string `json:"architecture"`
				Config       struct {
					Cmd           []string          `json:"Cmd"`
					Entrypoint    []string          `json:"Entrypoint"`
					WorkingDir    string            `json:"WorkingDir"`
					User          string            `json:"User"`
					ExposedPorts  map[string]interface{} `json:"ExposedPorts"`
					Env           []string          `json:"Env"`
					Labels        map[string]string `json:"Labels"`
					Volumes       map[string]interface{} `json:"Volumes"`
				} `json:"config"`
				History []struct {
					Created    string `json:"created"`
					CreatedBy  string `json:"created_by"`
					EmptyLayer bool   `json:"empty_layer"`
					Comment    string `json:"comment,omitempty"`
				} `json:"history"`
				RootFS struct {
					Type    string   `json:"type"`
					DiffIDs []string `json:"diff_ids"`
				} `json:"rootfs"`
			}
			if err := json.NewDecoder(cResp.Body).Decode(&cfgBlob); err == nil {
				detail.Created = cfgBlob.Created
				detail.OS = cfgBlob.OS
				detail.Arch = cfgBlob.Architecture

				cfg := &ImageCfg{}
				cfg.Cmd = cfgBlob.Config.Cmd
				cfg.Entrypoint = cfgBlob.Config.Entrypoint
				cfg.Workdir = cfgBlob.Config.WorkingDir
				cfg.User = cfgBlob.Config.User

				// Exposed ports
				for p := range cfgBlob.Config.ExposedPorts {
					cfg.ExposedPorts = append(cfg.ExposedPorts, p)
				}
				sort.Strings(cfg.ExposedPorts)

				// Volumes
				for v := range cfgBlob.Config.Volumes {
					cfg.Volumes = append(cfg.Volumes, v)
				}
				sort.Strings(cfg.Volumes)

				// Env
				for _, e := range cfgBlob.Config.Env {
					if parts := strings.SplitN(e, "=", 2); len(parts) == 2 {
						cfg.Env = append(cfg.Env, EnvVar{Key: parts[0], Value: parts[1]})
					}
				}

				// Labels
				for k, v := range cfgBlob.Config.Labels {
					cfg.Labels = append(cfg.Labels, EnvVar{Key: k, Value: v})
				}
				sort.Slice(cfg.Labels, func(i, j int) bool {
					return cfg.Labels[i].Key < cfg.Labels[j].Key
				})

				detail.Config = cfg

				// History
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
		}
	}

	common.JSON(w, http.StatusOK, detail)
}

// DeleteManifest deletes an image manifest by digest.
func (h *Handler) DeleteManifest(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	digest := chi.URLParam(r, "digest")

	if err := h.zotDelete("/v2/" + name + "/manifests/" + digest); err != nil {
		common.Errorf(w, http.StatusBadGateway, "delete failed: %v", err)
		return
	}

	common.JSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
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

// Ensure used imports
var _ = time.Now
