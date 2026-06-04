package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

// ─── Request types ────────────────────────────────────────────────────────────

type createRegistryUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type updateRegistryUserRequest struct {
	Username *string `json:"username,omitempty"`
	Password *string `json:"password,omitempty"`
	Role     *string `json:"role,omitempty"`
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

// ─── Create ───────────────────────────────────────────────────────────────────

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createRegistryUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)

	if req.Username == "" || req.Password == "" {
		common.Error(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if len(req.Password) < 8 {
		common.Error(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if req.Role == "" {
		req.Role = "deploy"
	}
	if req.Role != "admin" && req.Role != "deploy" && req.Role != "readonly" {
		common.Error(w, http.StatusBadRequest, "role must be admin, deploy, or readonly")
		return
	}

	// Check duplicate
	existing, _ := h.repo.GetRegistryUserByUsername(r.Context(), req.Username)
	if existing != nil {
		common.Error(w, http.StatusConflict, "username already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	now := time.Now()
	user := &model.RegistryUser{
		ID:           uuid.New().String(),
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         req.Role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.repo.CreateRegistryUser(r.Context(), user); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create registry user")
		return
	}

	// Sync htpasswd and restart Zot
	h.syncZotHtpasswd(r.Context())

	// Audit log
	h.logAudit(r, "registry.user.create", "registry_user", user.ID,
		fmt.Sprintf("Created registry user %s as %s", user.Username, user.Role))

	// Return the created user + the generated password (shown once)
	common.JSON(w, http.StatusCreated, map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
		"password": req.Password,
	})
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	user, err := h.repo.GetRegistryUserByID(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "registry user not found")
		return
	}

	var req updateRegistryUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	changes := []string{}

	if req.Username != nil {
		newUsername := strings.TrimSpace(*req.Username)
		if newUsername != user.Username {
			// Check not taken
			existing, _ := h.repo.GetRegistryUserByUsername(r.Context(), newUsername)
			if existing != nil && existing.ID != user.ID {
				common.Error(w, http.StatusConflict, "username already in use")
				return
			}
			user.Username = newUsername
			changes = append(changes, "username")
		}
	}
	if req.Role != nil {
		role := *req.Role
		if role != "admin" && role != "deploy" && role != "readonly" {
			common.Error(w, http.StatusBadRequest, "role must be admin, deploy, or readonly")
			return
		}
		user.Role = role
		changes = append(changes, "role")
	}

	if len(changes) > 0 {
		if err := h.repo.UpdateRegistryUser(r.Context(), user); err != nil {
			common.Error(w, http.StatusInternalServerError, "failed to update registry user")
			return
		}
	}

	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 8 {
			common.Error(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			common.Error(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		if err := h.repo.UpdateRegistryUserPassword(r.Context(), user.ID, string(hash)); err != nil {
			common.Error(w, http.StatusInternalServerError, "failed to update password")
			return
		}
		changes = append(changes, "password")
	}

	if len(changes) == 0 {
		common.JSON(w, http.StatusOK, map[string]string{"message": "no changes"})
		return
	}

	// Sync htpasswd and restart Zot
	h.syncZotHtpasswd(r.Context())

	h.logAudit(r, "registry.user.update", "registry_user", user.ID,
		fmt.Sprintf("Updated registry user %s: %s", user.Username, strings.Join(changes, ", ")))

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	user, err := h.repo.GetRegistryUserByID(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "registry user not found")
		return
	}

	// Prevent deleting last admin
	if user.Role == "admin" {
		users, _ := h.repo.ListRegistryUsers(r.Context())
		adminCount := 0
		for _, u := range users {
			if u.Role == "admin" && u.ID != user.ID {
				adminCount++
			}
		}
		if adminCount == 0 {
			common.Error(w, http.StatusBadRequest, "cannot delete the last admin user")
			return
		}
	}

	username := user.Username

	if err := h.repo.DeleteRegistryUser(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete registry user")
		return
	}

	// Sync htpasswd and restart Zot
	h.syncZotHtpasswd(r.Context())

	h.logAudit(r, "registry.user.delete", "registry_user", id,
		fmt.Sprintf("Deleted registry user %s", username))

	common.JSON(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

// ─── Reset Password ───────────────────────────────────────────────────────────

func (h *Handler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	user, err := h.repo.GetRegistryUserByID(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "registry user not found")
		return
	}

	var req resetPasswordRequest
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

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := h.repo.UpdateRegistryUserPassword(r.Context(), user.ID, string(hash)); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to reset password")
		return
	}

	// Sync htpasswd and restart Zot
	h.syncZotHtpasswd(r.Context())

	h.logAudit(r, "registry.user.reset-password", "registry_user", user.ID,
		fmt.Sprintf("Reset password for registry user %s", user.Username))

	common.JSON(w, http.StatusOK, map[string]string{"message": "password reset"})
}

// ─── Sync HTPasswd (manual trigger) ───────────────────────────────────────────

func (h *Handler) SyncHtpasswd(w http.ResponseWriter, r *http.Request) {
	if err := h.syncZotHtpasswd(r.Context()); err != nil {
		common.Error(w, http.StatusInternalServerError, fmt.Sprintf("sync failed: %v", err))
		return
	}
	common.JSON(w, http.StatusOK, map[string]string{"message": "htpasswd synced and Zot restarted"})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// syncZotHtpasswd reads all registry users from the DB, generates the htpasswd
// file content, writes it to disk, and restarts the Zot container.
func (h *Handler) syncZotHtpasswd(ctx context.Context) error {
	users, err := h.repo.ListRegistryUsers(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	// Build a set of existing usernames to avoid duplicates
	existing := make(map[string]bool, len(users))
	for _, u := range users {
		existing[u.Username] = true
	}

	// Generate htpasswd content — each line: username:bcrypt_hash
	var lines []string
	for _, u := range users {
		lines = append(lines, u.Username+":"+u.PasswordHash)
	}

	// Ensure the configured admin user (used by zotRequest to talk to Zot)
	// is always present in htpasswd, even if not in the registry_users table.
	if !existing[h.cfg.AdminUser] && h.cfg.AdminUser != "" && h.cfg.AdminPass != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(h.cfg.AdminPass), bcrypt.DefaultCost)
		if err == nil {
			lines = append(lines, h.cfg.AdminUser+":"+string(hash))
		}
	}

	content := strings.Join(lines, "\n") + "\n"

	// Write to htpasswd file
	if err := os.WriteFile(h.cfg.HtpasswdPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write htpasswd: %w", err)
	}

	// Restart Zot container via Docker CLI
	if err := restartZotContainer(h.cfg.ZotContainer); err != nil {
		// Non-fatal: if Docker isn't available, log and continue
		// The htpasswd file is already written — Zot will pick it up on next restart
		fmt.Printf("warning: failed to restart Zot container: %v\n", err)
	}

	return nil
}

// restartZotContainer restarts the Zot container via the Docker socket.
func restartZotContainer(containerName string) error {
	cmd := exec.Command("docker", "restart", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker restart %s: %s — %v", containerName, string(output), err)
	}
	return nil
}

// ─── Audit logger helper ──────────────────────────────────────────────────────

func (h *Handler) logAudit(r *http.Request, action, entityType, entityID, description string) {
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
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.Split(fwd, ",")[0]
		ip = strings.TrimSpace(ip)
	}

	audit.Log(h.repo, userID, userEmail, ip, action, entityType, entityID, description)
}
