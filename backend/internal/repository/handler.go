package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

type Handler struct {
	repo *db.Repository
}

func NewHandler(repo *db.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/connections", h.ListConnections)
	r.Post("/connections", h.CreateConnection)
	r.Delete("/connections/{id}", h.DeleteConnection)
	r.Get("/selections", h.ListSelections)
	r.Post("/selections", h.SaveSelections)
	r.Get("/{provider}/{owner}/{repo}/branches", h.ListBranches)
	r.Get("/{provider}/{owner}/{repo}/ci-status", h.GetCIRepoStatus)
	r.Get("/{provider}/{owner}/{repo}/deployments", h.ListDeploymentsByRepo)
	return r
}

// enrichedRepo flattens GitRepo fields with CI status and PR count for the API response.
type enrichedRepo struct {
	model.GitRepo
	CIStatus *model.RepoCIStatus `json:"ci_status,omitempty"`
	OpenPRs  int                 `json:"open_prs"`
}

// List returns all repos from all connected providers for the current user,
// enriched with CI status and open PR count.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	conns, err := h.repo.ListRepoConnections(r.Context(), claims.UserID)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to fetch connections")
		return
	}

	type connRepos struct {
		Provider GitProvider
		Repos    []model.GitRepo
		Err      error
	}

	results := make(chan connRepos, len(conns))
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, conn := range conns {
		if !conn.IsActive {
			continue
		}
		wg.Add(1)
		go func(c *model.RepoConnection) {
			defer wg.Done()
			provider := NewGitProvider(c.Provider, c.TokenEncrypted, c.BaseURL, c.Affiliations)
			if provider == nil {
				results <- connRepos{Err: nil}
				return
			}
			repos, err := provider.ListRepos(ctx)
			results <- connRepos{Provider: provider, Repos: repos, Err: err}
		}(conn)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allRepos []enrichedRepo
	hasError := false
	for res := range results {
		if res.Err != nil || res.Provider == nil {
			hasError = true
			continue
		}
		for _, repo := range res.Repos {
			er := enrichedRepo{GitRepo: repo}
			// Fetch CI status and PR count
			ciCtx, ciCancel := context.WithTimeout(context.Background(), 10*time.Second)
			ci, _ := res.Provider.GetCommitStatus(ciCtx, repo.Owner, repo.Name, repo.DefaultBranch)
			ciCancel()
			er.CIStatus = ci

			prCtx, prCancel := context.WithTimeout(context.Background(), 10*time.Second)
			prs, _ := res.Provider.ListOpenPRs(prCtx, repo.Owner, repo.Name)
			prCancel()
			er.OpenPRs = prs

			allRepos = append(allRepos, er)
		}
	}

	// ─── Filter by user selections ────────────────────────────────────
	selections, err := h.repo.GetRepoSelections(r.Context(), claims.UserID)
	if err == nil && len(selections) > 0 {
		selMap := make(map[string]bool)
		for _, s := range selections {
			key := s.Provider + "/" + s.Owner + "/" + s.RepoName
			selMap[key] = s.Selected
		}
		var filtered []enrichedRepo
		for _, repo := range allRepos {
			key := repo.Provider + "/" + repo.Owner + "/" + repo.Name
			if selected, exists := selMap[key]; exists && !selected {
				continue // user explicitly hid this repo
			}
			filtered = append(filtered, repo)
		}
		allRepos = filtered
	}

	if len(allRepos) == 0 && hasError {
		common.Error(w, http.StatusInternalServerError, "failed to fetch repositories")
		return
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"repositories": allRepos,
		"total":        len(allRepos),
	})
}

// ListConnections returns all repo connections for the current user.
func (h *Handler) ListConnections(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	conns, err := h.repo.ListRepoConnections(r.Context(), claims.UserID)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list connections")
		return
	}
	var responses []model.RepoConnectionResponse
	for _, c := range conns {
		responses = append(responses, c.ToResponse())
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"connections": responses,
	})
}

// CreateConnection creates a new provider connection.
func (h *Handler) CreateConnection(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.CreateRepoConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Provider != "github" && req.Provider != "forgejo" {
		common.Error(w, http.StatusBadRequest, "provider must be 'github' or 'forgejo'")
		return
	}
	if req.Token == "" {
		common.Error(w, http.StatusBadRequest, "token is required")
		return
	}
	if req.Provider == "forgejo" && req.BaseURL == "" {
		common.Error(w, http.StatusBadRequest, "base_url is required for forgejo")
		return
	}

	// ─── Validate token against provider API ───────────────────────────
	provider := NewGitProvider(req.Provider, req.Token, req.BaseURL, "")
	if provider == nil {
		common.Error(w, http.StatusBadRequest, "unsupported provider")
		return
	}

	valCtx, valCancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer valCancel()
	identity, err := provider.Validate(valCtx)
	if err != nil {
		common.Error(w, http.StatusBadRequest, "invalid token: "+err.Error())
		return
	}

	// Auto-label if not provided
	if req.Label == "" && identity != nil {
		if identity.Login != "" {
			req.Label = req.Provider + " (" + identity.Login + ")"
		} else {
			req.Label = req.Provider
		}
	}

	// Build affiliations string from request (or use default for backward compatibility)
	affiliations := "owner,collaborator,organization_member"
	if len(req.Affiliations) > 0 {
		affiliations = strings.Join(req.Affiliations, ",")
	}

	conn := &model.RepoConnection{
		ID:             uuid.New().String(),
		UserID:         claims.UserID,
		Provider:       req.Provider,
		Label:          req.Label,
		BaseURL:        req.BaseURL,
		TokenEncrypted: req.Token, // TODO: encrypt at rest
		IsActive:       true,
		Affiliations:   affiliations,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.repo.CreateRepoConnection(r.Context(), conn); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create connection")
		return
	}

	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"repo_connection.create", "repo_connection", conn.ID,
		"Connected "+req.Provider+" account")

	common.JSON(w, http.StatusCreated, conn.ToResponse())
}

// DeleteConnection removes a repo connection.
func (h *Handler) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteRepoConnection(r.Context(), id, claims.UserID); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete connection")
		return
	}

	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"repo_connection.delete", "repo_connection", id,
		"Disconnected git provider")

	w.WriteHeader(http.StatusNoContent)
}

// ListSelections returns all repo visibility selections for the current user.
func (h *Handler) ListSelections(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	selections, err := h.repo.GetRepoSelections(r.Context(), claims.UserID)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to fetch selections")
		return
	}
	if selections == nil {
		selections = []*model.RepoSelection{}
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"selections": selections,
	})
}

// SaveSelections bulk saves repo visibility selections.
func (h *Handler) SaveSelections(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.BulkRepoSaveSelectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if len(req.Selections) == 0 {
		common.Error(w, http.StatusBadRequest, "selections is required")
		return
	}

	if err := h.repo.BulkSetRepoSelections(r.Context(), claims.UserID, req.Selections); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to save selections")
		return
	}

	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"repo_selection.save", "repo_selection", "",
		"Updated repo visibility selections")

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "selections saved",
	})
}

// ListBranches returns branches for a repo from the specified provider.
func (h *Handler) ListBranches(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	owner := chi.URLParam(r, "owner")
	repoName := chi.URLParam(r, "repo")

	gitProvider := h.getProviderFor(r.Context(), providerName)
	if gitProvider == nil {
		common.Error(w, http.StatusNotFound, "no active connection for this provider")
		return
	}

	branches, err := gitProvider.ListBranches(r.Context(), owner, repoName)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list branches")
		return
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"branches": branches,
	})
}

// GetCIRepoStatus returns CI status for a repo.
func (h *Handler) GetCIRepoStatus(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	owner := chi.URLParam(r, "owner")
	repoName := chi.URLParam(r, "repo")
	branch := r.URL.Query().Get("branch")

	gitProvider := h.getProviderFor(r.Context(), providerName)
	if gitProvider == nil {
		common.Error(w, http.StatusNotFound, "no active connection for this provider")
		return
	}

	status, err := gitProvider.GetCommitStatus(r.Context(), owner, repoName, branch)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get CI status")
		return
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"ci_status": status,
	})
}

// ListDeploymentsByRepo returns deployments linked to this repo.
func (h *Handler) ListDeploymentsByRepo(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	owner := chi.URLParam(r, "owner")
	repoName := chi.URLParam(r, "repo")

	deps, err := h.repo.ListDeploymentsByRepo(r.Context(), providerName, owner, repoName)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list deployments")
		return
	}

	if deps == nil {
		deps = []*model.Deployment{}
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"deployments": deps,
	})
}

// getProviderFor finds an active connection for the given provider type.
func (h *Handler) getProviderFor(ctx context.Context, providerType string) GitProvider {
	claims := auth.GetClaims(ctx)
	if claims == nil {
		return nil
	}
	conns, err := h.repo.ListRepoConnections(ctx, claims.UserID)
	if err != nil {
		return nil
	}
	for _, c := range conns {
		if c.Provider == providerType && c.IsActive {
			return NewGitProvider(c.Provider, c.TokenEncrypted, c.BaseURL, c.Affiliations)
		}
	}
	return nil
}
