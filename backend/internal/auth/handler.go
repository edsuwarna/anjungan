package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/common"
)

type Handler struct {
	svc  *Service
	repo audit.Repository
}

func NewHandler(svc *Service, repo audit.Repository) *Handler {
	return &Handler{svc: svc, repo: repo}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip := audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
	resp, err := h.svc.Login(r.Context(), req.Email, req.Password, req.TOTPCode, ip)
	if err != nil {
		if errors.Is(err, ErrTOTPRequired) {
			common.JSON(w, http.StatusOK, map[string]string{"status": "totp_required"})
			return
		}
		if errors.Is(err, ErrAccountLocked) {
			common.Error(w, http.StatusTooManyRequests, "account locked. too many failed attempts")
			return
		}
		common.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Audit log: successful login
	if resp.User != nil {
		meta, _ := json.Marshal(map[string]string{
			"user_name": resp.User.Name,
			"user_role": resp.User.Role,
		})
		audit.Log(h.repo, resp.User.ID, resp.User.Email, ip,
			"auth.login", "user", resp.User.ID,
			"User logged in", json.RawMessage(meta))
	}

	common.JSON(w, http.StatusOK, resp)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.svc.Register(r.Context(), req.Email, req.Name, req.Password)
	if err != nil {
		if errors.Is(err, ErrPasswordTooShort) {
			common.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		common.Error(w, http.StatusConflict, "email already registered")
		return
	}

	meta, _ := json.Marshal(map[string]string{
		"user_name": user.Name,
		"user_role": user.Role,
	})
	audit.Log(h.repo, user.ID, user.Email, r.RemoteAddr,
		"auth.register", "user", user.ID,
		"User registered", json.RawMessage(meta))

	common.JSON(w, http.StatusCreated, user)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	common.JSON(w, http.StatusOK, claims)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims != nil && h.repo != nil {
		meta, _ := json.Marshal(map[string]string{
			"user_email": claims.Email,
			"user_role":  claims.Role,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"auth.logout", "user", claims.UserID,
			"User logged out", json.RawMessage(meta))
	}
	common.JSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *Handler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}

// ─── Self-service types ────────────────────────────────────────────────────

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type UpdateProfileRequest struct {
	Name  *string `json:"name,omitempty"`
	Email *string `json:"email,omitempty"`
}

// extractClaims validates the Bearer token and returns claims.
func extractClaims(svc *Service, r *http.Request) *Claims {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == authHeader {
		return nil
	}
	claims, err := svc.ValidateAccessToken(tokenStr)
	if err != nil {
		return nil
	}
	return claims
}

// ChangePassword updates the authenticated user's password.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		common.Error(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}
	claims := extractClaims(h.svc, r)
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.svc.ChangePassword(r.Context(), claims.Email, req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			common.Error(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}
		if errors.Is(err, ErrPasswordTooShort) {
			common.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		common.Error(w, http.StatusInternalServerError, "failed to change password")
		return
	}
	common.JSON(w, http.StatusOK, map[string]string{"message": "password changed"})
}

// UpdateProfile updates the authenticated user's name and/or email.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == nil && req.Email == nil {
		common.Error(w, http.StatusBadRequest, "at least one field (name or email) must be provided")
		return
	}
	claims := extractClaims(h.svc, r)
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	newName := ""
	if req.Name != nil {
		newName = *req.Name
	}
	newEmail := ""
	if req.Email != nil {
		newEmail = *req.Email
	}
	user, err := h.svc.UpdateProfile(r.Context(), claims.Email, newName, newEmail)
	if err != nil {
		if err.Error() == "email already in use" {
			common.Error(w, http.StatusConflict, "email already in use")
			return
		}
		common.Error(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	common.JSON(w, http.StatusOK, user)
}
