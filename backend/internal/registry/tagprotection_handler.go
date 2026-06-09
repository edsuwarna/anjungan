package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

// tagProtectionRoutes returns routes for tag protection management.
func (h *Handler) tagProtectionRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListTagProtections)
	r.Post("/", h.requireAdmin(h.CreateTagProtection))
	r.Get("/check", h.CheckTagProtection)
	r.Delete("/by-repo", h.requireAdmin(h.DeleteTagProtectionByRepoTag))
	r.Delete("/{id}", h.requireAdmin(h.DeleteTagProtection))
	return r
}

// ListTagProtections returns all tag protections across all repos.
func (h *Handler) ListTagProtections(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	var protections []*model.RegistryTagProtection
	var err error

	if repo != "" {
		protections, err = h.repo.ListTagProtections(r.Context(), repo)
	} else {
		protections, err = h.repo.ListAllTagProtections(r.Context())
	}

	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list protections")
		return
	}
	if protections == nil {
		protections = []*model.RegistryTagProtection{}
	}
	common.JSON(w, http.StatusOK, protections)
}

// CreateTagProtection adds a tag protection.
func (h *Handler) CreateTagProtection(w http.ResponseWriter, r *http.Request) {
	var req model.TagProtectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := req.Validate(); msg != "" {
		common.Error(w, http.StatusBadRequest, msg)
		return
	}

	// Normalize
	req.Repo = strings.TrimSpace(req.Repo)
	req.Tag = strings.TrimSpace(req.Tag)

	claims := auth.GetClaims(r.Context())
	createdBy := ""
	if claims != nil {
		createdBy = claims.Email
	}

	p := &model.RegistryTagProtection{
		ID:        uuid.New().String(),
		Repo:      req.Repo,
		Tag:       req.Tag,
		CreatedBy: createdBy,
	}

	if err := h.repo.CreateTagProtection(r.Context(), p); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create protection")
		return
	}

	h.logAudit(r, "registry.protection.create", "registry_tag_protection", p.ID,
		fmt.Sprintf("Protected tag %s:%s", p.Repo, p.Tag))

	common.JSON(w, http.StatusCreated, p)
}

// DeleteTagProtection removes a tag protection by ID.
func (h *Handler) DeleteTagProtection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.repo.DeleteTagProtection(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete protection")
		return
	}

	h.logAudit(r, "registry.protection.delete", "registry_tag_protection", id,
		"Removed tag protection")

	common.JSON(w, http.StatusOK, map[string]string{"message": "protection removed"})
}

// DeleteTagProtectionByRepoTag removes a tag protection by repo + tag.
// Uses query params to avoid URL path issues with repo names containing slashes.
func (h *Handler) DeleteTagProtectionByRepoTag(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	tag := r.URL.Query().Get("tag")

	if repo == "" || tag == "" {
		common.Error(w, http.StatusBadRequest, "repo and tag query parameters are required")
		return
	}

	if err := h.repo.DeleteTagProtectionByRepoTag(r.Context(), repo, tag); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete protection")
		return
	}

	h.logAudit(r, "registry.protection.delete", "registry_tag_protection", repo+":"+tag,
		fmt.Sprintf("Removed tag protection for %s:%s", repo, tag))

	common.JSON(w, http.StatusOK, map[string]string{"message": "protection removed"})
}

// CheckTagProtection checks if a specific tag is protected.
// Uses query params to avoid URL path issues with repo names containing slashes.
func (h *Handler) CheckTagProtection(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	tag := r.URL.Query().Get("tag")

	protected, err := h.repo.IsTagProtected(r.Context(), repo, tag)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to check protection")
		return
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"repo":      repo,
		"tag":       tag,
		"protected": protected,
	})
}

// checkTagProtectionBeforeDelete checks if a tag is protected before allowing delete.
// Returns true if delete should proceed, false if blocked.
func (h *Handler) checkTagProtectionBeforeDelete(w http.ResponseWriter, r *http.Request, repo, tag string) bool {
	protected, err := h.repo.IsTagProtected(r.Context(), repo, tag)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to check protection")
		return false
	}
	if protected {
		common.Errorf(w, http.StatusForbidden, "tag %s:%s is protected and cannot be deleted", repo, tag)
		return false
	}
	return true
}
