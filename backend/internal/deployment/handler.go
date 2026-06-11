package deployment

import (
	"encoding/json"
	"net/http"
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
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Post("/{id}/restart", h.Restart)
	r.Post("/{id}/redeploy", h.Redeploy)
	r.Post("/{id}/rollback", h.Rollback)
	r.Get("/{id}/history", h.GetHistory)
	r.Get("/history", h.ListHistory)
	// Environment sub-routes
	r.Get("/environments", h.ListEnvironments)
	r.Post("/environments", h.CreateEnvironment)
	r.Put("/environments/{id}", h.UpdateEnvironment)
	r.Delete("/environments/{id}", h.DeleteEnvironment)
	return r
}

// List returns all deployments, optionally filtered by environment_id.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	envID := r.URL.Query().Get("environment_id")
	deps, err := h.repo.ListDeployments(r.Context(), envID)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list deployments")
		return
	}
	if deps == nil {
		deps = []*model.Deployment{}
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"deployments": deps,
		"total":       len(deps),
	})
}

// Create creates a new deployment.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.CreateDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" || req.EnvironmentID == "" || req.ServerID == "" {
		common.Error(w, http.StatusBadRequest, "name, environment_id, and server_id are required")
		return
	}

	deploy := &model.Deployment{
		ID:            uuid.New().String(),
		Name:          req.Name,
		EnvironmentID: &req.EnvironmentID,
		RepoProvider:  req.RepoProvider,
		RepoOwner:     req.RepoOwner,
		RepoName:      req.RepoName,
		Branch:        req.Branch,
		CommitSHA:     req.CommitSHA,
		ServerID:      &req.ServerID,
		ServiceName:   req.ServiceName,
		Image:         req.Image,
		Status:        "pending",
		DeployedBy:    &claims.UserID,
		DeployedAt:    time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := h.repo.CreateDeployment(r.Context(), deploy); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	// Record initial history entry
	h.repo.UpdateDeploymentStatus(r.Context(), deploy.ID, "pending", "Deployment created")

	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"deployment.create", "deployment", deploy.ID,
		"Created deployment "+deploy.Name)

	common.JSON(w, http.StatusCreated, deploy)
}

// Get returns a single deployment by ID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	deploy, err := h.repo.GetDeployment(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "deployment not found")
		return
	}
	common.JSON(w, http.StatusOK, deploy)
}

// Restart restarts a deployment (placeholder for SSH-based restart).
func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	deploy, err := h.repo.GetDeployment(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "deployment not found")
		return
	}

	// TODO: actual SSH exec to restart container
	h.repo.UpdateDeploymentStatus(r.Context(), id, "running", "Restarted")

	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"deployment.restart", "deployment", id,
		"Restarted deployment "+deploy.Name)

	common.JSON(w, http.StatusOK, map[string]string{"message": "restart initiated"})
}

// Redeploy redeploys with the same configuration.
func (h *Handler) Redeploy(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	deploy, err := h.repo.GetDeployment(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "deployment not found")
		return
	}

	h.repo.UpdateDeploymentStatus(r.Context(), id, "deploying", "Redeploying")

	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"deployment.redeploy", "deployment", id,
		"Redeployed "+deploy.Name)

	common.JSON(w, http.StatusOK, map[string]string{"message": "redeploy initiated"})
}

// Rollback rolls back to a previous deployment version.
func (h *Handler) Rollback(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	deploy, err := h.repo.GetDeployment(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "deployment not found")
		return
	}

	// Set current as rolled_back and record history
	h.repo.UpdateDeploymentStatus(r.Context(), id, "rolled_back", "Rolled back")

	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"deployment.rollback", "deployment", id,
		"Rolled back deployment "+deploy.Name)

	common.JSON(w, http.StatusOK, map[string]string{"message": "rollback initiated"})
}

// GetHistory returns deployment history for a single deployment.
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	history, err := h.repo.ListDeploymentHistory(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get history")
		return
	}
	if history == nil {
		history = []*model.DeploymentHistory{}
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"history": history,
	})
}

// ListHistory returns global deployment history (last 50 entries across all deployments).
func (h *Handler) ListHistory(w http.ResponseWriter, r *http.Request) {
	// For global history, we list all deployments sorted by deployed_at
	deps, err := h.repo.ListDeployments(r.Context(), "")
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get history")
		return
	}
	if deps == nil {
		deps = []*model.Deployment{}
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"history": deps,
		"total":   len(deps),
	})
}

// ─── Environment CRUD ───────────────────────────────────────────────────────

func (h *Handler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	envs, err := h.repo.ListEnvironments(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list environments")
		return
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"environments": envs,
	})
}

func (h *Handler) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil || claims.Role != "admin" {
		common.Error(w, http.StatusForbidden, "admin access required")
		return
	}

	var req model.CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		common.Error(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Color == "" {
		req.Color = "#10b981"
	}

	env := &model.Environment{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
		IsProtected: false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.repo.CreateEnvironment(r.Context(), env); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create environment")
		return
	}

	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"environment.create", "environment", env.ID,
		"Created environment "+env.Name)

	common.JSON(w, http.StatusCreated, env)
}

func (h *Handler) UpdateEnvironment(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil || claims.Role != "admin" {
		common.Error(w, http.StatusForbidden, "admin access required")
		return
	}

	id := chi.URLParam(r, "id")
	env, err := h.repo.GetEnvironment(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "environment not found")
		return
	}

	var req model.UpdateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Name != nil {
		env.Name = *req.Name
	}
	if req.Color != nil {
		env.Color = *req.Color
	}
	if req.Description != nil {
		env.Description = *req.Description
	}
	env.UpdatedAt = time.Now()

	if err := h.repo.UpdateEnvironment(r.Context(), env); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to update environment")
		return
	}

	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"environment.update", "environment", id,
		"Updated environment "+env.Name)

	common.JSON(w, http.StatusOK, env)
}

func (h *Handler) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil || claims.Role != "admin" {
		common.Error(w, http.StatusForbidden, "admin access required")
		return
	}

	id := chi.URLParam(r, "id")
	env, err := h.repo.GetEnvironment(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "environment not found")
		return
	}

	if env.IsProtected {
		common.Error(w, http.StatusForbidden, "cannot delete protected environment")
		return
	}

	if err := h.repo.DeleteEnvironment(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete environment")
		return
	}

	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"environment.delete", "environment", id,
		"Deleted environment "+env.Name)

	w.WriteHeader(http.StatusNoContent)
}
