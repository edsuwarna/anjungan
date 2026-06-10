package project

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

const defaultProjectID = "00000000-0000-0000-0000-000000000001"

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
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Get("/{id}/members", h.ListMembers)
	r.Post("/{id}/members", h.AddMember)
	r.Put("/{id}/members/{userId}", h.UpdateMemberRole)
	r.Delete("/{id}/members/{userId}", h.RemoveMember)
	r.Get("/{id}/resource-count", h.GetResourceCount)
	return r
}

// authorizeProjectAccess checks if the user has the required role in the project.
// Super-admin (global admin role) bypasses membership check.
// Default Project is accessible to all authenticated users.
func (h *Handler) authorizeProjectAccess(ctx context.Context, projectID string, minRole string) bool {
	claims := auth.GetClaims(ctx)
	if claims == nil {
		return false
	}
	// Super-admin bypass
	if claims.Role == model.RoleAdmin {
		return true
	}
	// Default Project is accessible to all authenticated users
	if projectID == defaultProjectID {
		return true
	}
	role, err := h.repo.GetProjectMemberRole(ctx, projectID, claims.UserID)
	if err != nil || role == "" {
		return false
	}
	return hasMinRole(role, minRole)
}

func hasMinRole(role, minRole string) bool {
	ranks := map[string]int{"viewer": 1, "developer": 2, "admin": 3}
	return ranks[role] >= ranks[minRole]
}

// ─── CRUD ───────────────────────────────────────────────────────────────────

// List returns all projects. Admin sees all; non-admin sees only their projects.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var projects []*model.Project
	var err error

	if claims.Role == model.RoleAdmin {
		projects, err = h.repo.ListProjects(r.Context())
	} else {
		projects, err = h.repo.ListProjectsByUser(r.Context(), claims.UserID)
		if err == nil {
			// Ensure Default Project is always included
			hasDefault := false
			for _, p := range projects {
				if p.ID == defaultProjectID {
					hasDefault = true
					break
				}
			}
			if !hasDefault {
				defaultProj, getErr := h.repo.GetProjectByID(r.Context(), defaultProjectID)
				if getErr == nil && defaultProj != nil {
					projects = append([]*model.Project{defaultProj}, projects...)
				}
			}
		}
	}

	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	resp := make([]model.ProjectResponse, len(projects))
	for i, p := range projects {
		resp[i] = h.projectToResponse(r.Context(), p)
	}

	common.JSON(w, http.StatusOK, model.ProjectListResponse{
		Projects: resp,
		Total:    len(resp),
	})
}

// Create creates a new project. Admin only.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if claims.Role != model.RoleAdmin {
		common.Error(w, http.StatusForbidden, "only admins can create projects")
		return
	}

	var req model.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		common.Error(w, http.StatusBadRequest, "name is required")
		return
	}
	req.Slug = strings.TrimSpace(req.Slug)
	if req.Slug == "" {
		common.Error(w, http.StatusBadRequest, "slug is required")
		return
	}

	now := time.Now().UTC()
	project := &model.Project{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		CreatedBy:   claims.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.repo.CreateProject(r.Context(), project); err != nil {
		if strings.Contains(err.Error(), "idx_projects_slug") || strings.Contains(err.Error(), "unique") {
			common.Error(w, http.StatusConflict, fmt.Sprintf("slug '%s' already exists", req.Slug))
			return
		}
		common.Error(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	// Auto-add admin as member with admin role
	member := &model.ProjectMember{
		ProjectID: project.ID,
		UserID:    claims.UserID,
		Role:      model.RoleAdmin,
		CreatedAt: now,
	}
	if err := h.repo.AddProjectMember(r.Context(), member); err != nil {
		// Non-fatal: log but don't fail
	}

	common.JSON(w, http.StatusCreated, h.projectToResponse(r.Context(), project))
}

// Get returns a single project by ID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		common.Error(w, http.StatusBadRequest, "missing project id")
		return
	}

	if !h.authorizeProjectAccess(r.Context(), id, model.RoleViewer) {
		common.Error(w, http.StatusForbidden, "access denied")
		return
	}

	project, err := h.repo.GetProjectByID(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get project")
		return
	}
	if project == nil {
		common.Error(w, http.StatusNotFound, "project not found")
		return
	}

	common.JSON(w, http.StatusOK, h.projectToResponse(r.Context(), project))
}

// Update updates a project. Admin or project admin only.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		common.Error(w, http.StatusBadRequest, "missing project id")
		return
	}

	if !h.authorizeProjectAccess(r.Context(), id, model.RoleAdmin) {
		common.Error(w, http.StatusForbidden, "access denied")
		return
	}

	var req model.UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	project, err := h.repo.GetProjectByID(r.Context(), id)
	if err != nil || project == nil {
		common.Error(w, http.StatusNotFound, "project not found")
		return
	}

	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Slug != nil {
		project.Slug = *req.Slug
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	project.UpdatedAt = time.Now().UTC()

	if err := h.repo.UpdateProject(r.Context(), project); err != nil {
		if strings.Contains(err.Error(), "unique") {
			common.Error(w, http.StatusConflict, fmt.Sprintf("slug '%s' already exists", project.Slug))
			return
		}
		common.Error(w, http.StatusInternalServerError, "failed to update project")
		return
	}

	common.JSON(w, http.StatusOK, h.projectToResponse(r.Context(), project))
}

// Delete deletes a project and moves its resources to Default Project. Admin only.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if claims.Role != model.RoleAdmin {
		common.Error(w, http.StatusForbidden, "only admins can delete projects")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		common.Error(w, http.StatusBadRequest, "missing project id")
		return
	}

	// Prevent deleting Default Project
	if id == defaultProjectID {
		common.Error(w, http.StatusBadRequest, "cannot delete the Default Project")
		return
	}

	project, err := h.repo.GetProjectByID(r.Context(), id)
	if err != nil || project == nil {
		common.Error(w, http.StatusNotFound, "project not found")
		return
	}

	// Move resources to Default Project
	moved, err := h.repo.MoveProjectResources(r.Context(), id, defaultProjectID)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to move project resources")
		return
	}

	// Delete project
	if err := h.repo.DeleteProject(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	common.JSON(w, http.StatusOK, model.ProjectDeleteResponse{
		ProjectID:      id,
		ProjectName:    project.Name,
		ResourcesMoved: moved,
		MovedToProject: "Default Project",
	})
}

// ─── Members ──────────────────────────────────────────────────────────────────

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		common.Error(w, http.StatusBadRequest, "missing project id")
		return
	}

	if !h.authorizeProjectAccess(r.Context(), id, model.RoleViewer) {
		common.Error(w, http.StatusForbidden, "access denied")
		return
	}

	members, err := h.repo.ListProjectMembers(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	if members == nil {
		members = []*model.ProjectMember{}
	}

	common.JSON(w, http.StatusOK, members)
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		common.Error(w, http.StatusBadRequest, "missing project id")
		return
	}

	if !h.authorizeProjectAccess(r.Context(), id, model.RoleAdmin) {
		common.Error(w, http.StatusForbidden, "access denied")
		return
	}

	var req model.AddProjectMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		common.Error(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Role == "" {
		req.Role = model.RoleDeveloper
	}
	if req.Role != model.RoleAdmin && req.Role != model.RoleDeveloper && req.Role != model.RoleViewer {
		common.Error(w, http.StatusBadRequest, "role must be admin, developer, or viewer")
		return
	}

	member := &model.ProjectMember{
		ProjectID: id,
		UserID:    req.UserID,
		Role:      req.Role,
		CreatedAt: time.Now().UTC(),
	}

	if err := h.repo.AddProjectMember(r.Context(), member); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to add member")
		return
	}

	common.JSON(w, http.StatusCreated, member)
}

func (h *Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	if id == "" || userID == "" {
		common.Error(w, http.StatusBadRequest, "missing project id or user id")
		return
	}

	if !h.authorizeProjectAccess(r.Context(), id, model.RoleAdmin) {
		common.Error(w, http.StatusForbidden, "access denied")
		return
	}

	var req model.UpdateProjectMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role == "" {
		common.Error(w, http.StatusBadRequest, "role is required")
		return
	}
	if req.Role != model.RoleAdmin && req.Role != model.RoleDeveloper && req.Role != model.RoleViewer {
		common.Error(w, http.StatusBadRequest, "role must be admin, developer, or viewer")
		return
	}

	if err := h.repo.UpdateProjectMemberRole(r.Context(), id, userID, req.Role); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to update member role")
		return
	}

	common.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	if id == "" || userID == "" {
		common.Error(w, http.StatusBadRequest, "missing project id or user id")
		return
	}

	if !h.authorizeProjectAccess(r.Context(), id, model.RoleAdmin) {
		common.Error(w, http.StatusForbidden, "access denied")
		return
	}

	if err := h.repo.RemoveProjectMember(r.Context(), id, userID); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to remove member")
		return
	}

	common.JSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// ─── Resource Count ────────────────────────────────────────────────────────────

func (h *Handler) GetResourceCount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		common.Error(w, http.StatusBadRequest, "missing project id")
		return
	}

	if !h.authorizeProjectAccess(r.Context(), id, model.RoleViewer) {
		common.Error(w, http.StatusForbidden, "access denied")
		return
	}

	counts, err := h.repo.GetProjectResourceCount(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get resource counts")
		return
	}

	common.JSON(w, http.StatusOK, counts)
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

func (h *Handler) projectToResponse(ctx context.Context, p *model.Project) model.ProjectResponse {
	resp := model.ProjectResponse{
		ID:          p.ID,
		Name:        p.Name,
		Slug:        p.Slug,
		Description: p.Description,
		CreatedBy:   p.CreatedBy,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}

	counts, err := h.repo.GetProjectResourceCount(ctx, p.ID)
	if err == nil {
		resp.ResourceCount = counts
	}

	memberCount, err := h.repo.GetProjectMemberCount(ctx, p.ID)
	if err == nil {
		resp.MemberCount = memberCount
	}

	return resp
}
