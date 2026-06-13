package admin

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	zlog "github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
	"github.com/edsuwarna/anjungan/internal/ratelimit"
)

type Handler struct {
	repo        *db.Repository
	rateLimiter *ratelimit.RateLimiter
}

func NewHandler(repo *db.Repository, rl *ratelimit.RateLimiter) *Handler {
	return &Handler{repo: repo, rateLimiter: rl}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireAdmin)
	r.Get("/users", h.ListUsers)
	r.Post("/users", h.CreateUser)
	r.Get("/users/{id}", h.GetUser)
	r.Put("/users/{id}", h.UpdateUser)
	r.Delete("/users/{id}", h.DeleteUser)
	r.Post("/users/{id}/unlock", h.UnlockUser)
	r.Get("/audit-log", h.ListAuditLogs)
	r.Get("/audit-log/actions", h.ListAuditActions)
	r.Get("/audit-log/entity-types", h.ListAuditEntityTypes)
	r.Get("/audit-log/export", h.ExportAuditLogs)
	return r
}

// ─── User CRUD ─────────────────────────────────────────────────────────────

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListUsers(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	// Strip sensitive fields
	type userResponse struct {
		ID              string     `json:"id"`
		Email           string     `json:"email"`
		Name            string     `json:"name"`
		Role            string     `json:"role"`
		AllowedGroups   []string   `json:"allowed_groups"`
		TOTPEnabled     bool       `json:"totp_enabled"`
		LockedUntil     *time.Time `json:"locked_until"`
		FailedAttempts  int        `json:"failed_attempts"`
		Status          string     `json:"status"`
		CreatedAt       time.Time  `json:"created_at"`
		UpdatedAt       time.Time  `json:"updated_at"`
	}
	resp := make([]userResponse, 0, len(users))
	now := time.Now()
	for _, u := range users {
		status := "unlocked"
		if u.LockedUntil != nil && u.LockedUntil.After(now) {
			status = "locked"
		}
		groups, _ := h.repo.GetUserServerGroups(r.Context(), u.ID)
		resp = append(resp, userResponse{
			ID:              u.ID,
			Email:           u.Email,
			Name:            u.Name,
			Role:            u.Role,
			AllowedGroups:   groups,
			TOTPEnabled:     u.TOTPEnabled,
			LockedUntil:     u.LockedUntil,
			FailedAttempts:  u.FailedLoginAttempts,
			Status:          status,
			CreatedAt:       u.CreatedAt,
			UpdatedAt:       u.UpdatedAt,
		})
	}

	common.JSON(w, http.StatusOK, resp)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.repo.GetUserByID(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "user not found")
		return
	}

	groups, _ := h.repo.GetUserServerGroups(r.Context(), user.ID)

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"id":            user.ID,
		"email":         user.Email,
		"name":          user.Name,
		"role":          user.Role,
		"allowed_groups": groups,
		"totp_enabled":  user.TOTPEnabled,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
	})
}

type createUserRequest struct {
	Email         string   `json:"email"`
	Name          string   `json:"name"`
	Password      string   `json:"password"`
	Role          string   `json:"role"`
	AllowedGroups []string `json:"allowed_groups,omitempty"`
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	if req.Email == "" || req.Name == "" || req.Password == "" {
		common.Error(w, http.StatusBadRequest, "email, name, and password are required")
		return
	}
	if len(req.Password) < 6 {
		common.Error(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}
	if req.Role == "" {
		req.Role = model.RoleDeveloper
	}
	if req.Role != model.RoleAdmin && req.Role != model.RoleDeveloper && req.Role != model.RoleViewer {
		common.Error(w, http.StatusBadRequest, "invalid role: must be admin, developer, or viewer")
		return
	}

	// Check existing
	existing, _ := h.repo.GetUserByEmail(r.Context(), req.Email)
	if existing != nil {
		common.Error(w, http.StatusConflict, "email already registered")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	now := time.Now()
	user := &model.User{
		ID:           uuid.New().String(),
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: string(hash),
		Role:         req.Role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.repo.CreateUser(r.Context(), user); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Save allowed server groups
	if len(req.AllowedGroups) > 0 {
		_ = h.repo.SetUserServerGroups(r.Context(), user.ID, req.AllowedGroups)
	}

	// Audit log
	meta, _ := json.Marshal(map[string]interface{}{
		"user_email":     user.Email,
		"user_name":      user.Name,
		"user_role":      user.Role,
		"allowed_groups": req.AllowedGroups,
	})
	h.logAudit(r, "user.create", "user", user.ID,
		fmt.Sprintf("Created user %s (%s) as %s", user.Name, user.Email, user.Role),
		json.RawMessage(meta))

	common.JSON(w, http.StatusCreated, map[string]interface{}{
		"id":    user.ID,
		"email": user.Email,
		"name":  user.Name,
		"role":  user.Role,
	})
}

type updateUserRequest struct {
	Email         *string  `json:"email,omitempty"`
	Name          *string  `json:"name,omitempty"`
	Role          *string  `json:"role,omitempty"`
	Password      *string  `json:"password,omitempty"`
	AllowedGroups []string `json:"allowed_groups,omitempty"`
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	user, err := h.repo.GetUserByID(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "user not found")
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	changes := []string{}

	if req.Name != nil && *req.Name != user.Name {
		user.Name = strings.TrimSpace(*req.Name)
		changes = append(changes, "name")
	}
	if req.Email != nil {
		newEmail := strings.TrimSpace(strings.ToLower(*req.Email))
		if newEmail != user.Email {
			// Check not taken
			existing, _ := h.repo.GetUserByEmail(r.Context(), newEmail)
			if existing != nil && existing.ID != user.ID {
				common.Error(w, http.StatusConflict, "email already in use")
				return
			}
			user.Email = newEmail
			changes = append(changes, "email")
		}
	}
	if req.Role != nil && *req.Role != user.Role {
		role := *req.Role
		if role != model.RoleAdmin && role != model.RoleDeveloper && role != model.RoleViewer {
			common.Error(w, http.StatusBadRequest, "invalid role")
			return
		}
		// Prevent removing last admin
		if user.Role == model.RoleAdmin && role != model.RoleAdmin {
			count, err := h.repo.CountAdminUsers(r.Context())
			if err == nil && count <= 1 {
				common.Error(w, http.StatusBadRequest, "cannot remove the last admin")
				return
			}
		}
		user.Role = role
		changes = append(changes, "role")
	}
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 6 {
			common.Error(w, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			common.Error(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		if err := h.repo.UpdateUserPassword(r.Context(), user.ID, string(hash)); err != nil {
			common.Error(w, http.StatusInternalServerError, "failed to update password")
			return
		}
		changes = append(changes, "password")
	}

	if len(changes) == 0 && req.AllowedGroups == nil {
		common.JSON(w, http.StatusOK, map[string]string{"message": "no changes"})
		return
	}

	if err := h.repo.UpdateUser(r.Context(), user); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	// Update allowed server groups (must happen before audit log)
	if req.AllowedGroups != nil {
		_ = h.repo.SetUserServerGroups(r.Context(), user.ID, req.AllowedGroups)
		changes = append(changes, "allowed_groups")
	}

	meta, _ := json.Marshal(map[string]interface{}{
		"user_email":     user.Email,
		"user_name":      user.Name,
		"user_role":      user.Role,
		"changes":        changes,
		"allowed_groups": req.AllowedGroups,
	})
	h.logAudit(r, "user.update", "user", user.ID,
		fmt.Sprintf("Updated user %s: %s", user.Email, strings.Join(changes, ", ")),
		json.RawMessage(meta))

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"id":    user.ID,
		"email": user.Email,
		"name":  user.Name,
		"role":  user.Role,
	})
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	user, err := h.repo.GetUserByID(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "user not found")
		return
	}

	// Prevent deleting self
	claims := auth.GetClaims(r.Context())
	if claims != nil && claims.UserID == id {
		common.Error(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	// Prevent deleting last admin
	if user.Role == model.RoleAdmin {
		count, err := h.repo.CountAdminUsers(r.Context())
		if err == nil && count <= 1 {
			common.Error(w, http.StatusBadRequest, "cannot delete the last admin")
			return
		}
	}

	userEmail := user.Email
	userName := user.Name

	if err := h.repo.DeleteUser(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	// Clean up server group associations
	_ = h.repo.DeleteUserServerGroups(r.Context(), id)

	meta, _ := json.Marshal(map[string]string{
		"user_email": userEmail,
		"user_name":  userName,
	})
	h.logAudit(r, "user.delete", "user", id,
		fmt.Sprintf("Deleted user %s (%s)", userName, userEmail),
		json.RawMessage(meta))

	common.JSON(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

// ─── Unlock User ────────────────────────────────────────────────────────────

func (h *Handler) UnlockUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	user, err := h.repo.GetUserByID(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "user not found")
		return
	}

	if err := h.repo.ResetUserLockout(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to unlock user")
		return
	}

	// Clear Redis-based lockout so the user can log in immediately
	if h.rateLimiter != nil {
		h.rateLimiter.ClearLockout(r.Context(), user.Email)
	}

	h.logAudit(r, "user.unlock", "user", id,
		fmt.Sprintf("Unlocked user %s (%s)", user.Name, user.Email))

	// Record unlock event in auth_events
	unlockEvent := &model.AuthEvent{
		ID:        uuid.New().String(),
		UserID:    id,
		Email:     user.Email,
		EventType: model.EventTypeUnlock,
		Status:    model.EventStatusSuccess,
		CreatedAt: time.Now(),
	}
	if err := h.repo.CreateAuthEvent(r.Context(), unlockEvent); err != nil {
		zlog.Warn().Err(err).Str("user_id", id).Msg("failed to record unlock auth event")
	}

	common.JSON(w, http.StatusOK, map[string]string{"message": "user unlocked"})
}

// ─── Audit Log ─────────────────────────────────────────────────────────────

func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := model.AuditLogQuery{
		Page:       common.ParseQueryInt(r, "page", 1),
		Limit:      common.ParseQueryInt(r, "limit", 50),
		Action:     r.URL.Query().Get("action"),
		EntityType: r.URL.Query().Get("entity_type"),
		UserID:     r.URL.Query().Get("user_id"),
		Search:     r.URL.Query().Get("search"),
		Sort:       r.URL.Query().Get("sort"),
		Order:      r.URL.Query().Get("order"),
	}

	if sd := r.URL.Query().Get("start_date"); sd != "" {
		q.StartDate = &sd
	}
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		q.EndDate = &ed
	}

	result, err := h.repo.ListAuditLogs(r.Context(), q)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	common.JSONWithMeta(w, http.StatusOK, result.Entries, &common.Meta{
		Page:       result.Page,
		PerPage:    result.Limit,
		Total:      result.Total,
		TotalPages: result.TotalPages,
	})
}

func (h *Handler) ListAuditActions(w http.ResponseWriter, r *http.Request) {
	actions, err := h.repo.ListAuditActions(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list actions")
		return
	}
	common.JSON(w, http.StatusOK, actions)
}

func (h *Handler) ListAuditEntityTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.repo.ListAuditEntityTypes(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list entity types")
		return
	}
	common.JSON(w, http.StatusOK, types)
}

func (h *Handler) ExportAuditLogs(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format") // "csv" or "json"

	q := model.AuditLogQuery{
		Action:     r.URL.Query().Get("action"),
		EntityType: r.URL.Query().Get("entity_type"),
		UserID:     r.URL.Query().Get("user_id"),
		Search:     r.URL.Query().Get("search"),
		Sort:       r.URL.Query().Get("sort"),
		Order:      r.URL.Query().Get("order"),
	}
	if sd := r.URL.Query().Get("start_date"); sd != "" {
		q.StartDate = &sd
	}
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		q.EndDate = &ed
	}

	entries, err := h.repo.ListAuditLogsAll(r.Context(), q)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to export audit logs")
		return
	}

	filename := fmt.Sprintf("audit-log-%s", time.Now().Format("2006-01-02"))

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, filename))

		writer := csv.NewWriter(w)
		writer.Write([]string{"Time", "Action", "EntityType", "EntityID", "Description", "UserID", "UserEmail", "IPAddress", "Metadata"})
		for _, e := range entries {
			writer.Write([]string{
				e.CreatedAt.Format(time.RFC3339),
				e.Action,
				e.EntityType,
				e.EntityID,
				e.Description,
				e.UserID,
				e.UserEmail,
				e.IPAddress,
				string(e.Metadata),
			})
		}
		writer.Flush()
		return
	}

	// Default: JSON
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, filename))
	json.NewEncoder(w).Encode(entries)
}

// ─── Audit logger helper ───────────────────────────────────────────────────

func (h *Handler) logAudit(r *http.Request, action, entityType, entityID, description string, metadata ...json.RawMessage) {
	claims := auth.GetClaims(r.Context())
	userID := ""
	userEmail := ""
	if claims != nil {
		userID = claims.UserID
		userEmail = claims.Email
	}

	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	// Handle X-Forwarded-For
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.Split(fwd, ",")[0]
		ip = strings.TrimSpace(ip)
	}

	meta := json.RawMessage("{}")
	if len(metadata) > 0 {
		meta = metadata[0]
	}

	entry := &model.AuditLogEntry{
		ID:          uuid.New().String(),
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		Description: description,
		UserID:      userID,
		UserEmail:   userEmail,
		IPAddress:   ip,
		Metadata:    meta,
		CreatedAt:   time.Now(),
	}

	// Best-effort async log
	go func() {
		_ = h.repo.CreateAuditLog(context.Background(), entry)
	}()
}
