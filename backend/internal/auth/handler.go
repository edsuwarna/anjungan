package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/authactivity"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

type Handler struct {
	svc        *Service
	repo       audit.Repository
	authEvents authactivity.Repository
}

func NewHandler(svc *Service, repo audit.Repository, authEvents authactivity.Repository) *Handler {
	return &Handler{svc: svc, repo: repo, authEvents: authEvents}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Normalize email to match DB (admin CreateUser normalizes it too)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	ip := audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
	userAgent := r.Header.Get("User-Agent")
	resp, err := h.svc.Login(r.Context(), req.Email, req.Password, req.TOTPCode, ip)
	if err != nil {
		if errors.Is(err, ErrTOTPRequired) {
			h.recordAuthEvent("", req.Email, model.EventTypeLoginAttempt, model.EventStatusSuccess, "", ip, userAgent)
			common.JSON(w, http.StatusOK, map[string]string{"status": "totp_required", "email": req.Email})
			return
		}
		if errors.Is(err, ErrAccountLocked) {
			h.recordAuthEvent("", req.Email, model.EventTypeLoginFailure, model.EventStatusFailure, "account_locked", ip, userAgent)
			if u, lookupErr := h.svc.GetUserByEmail(r.Context(), req.Email); lookupErr == nil {
				h.recordAuthEvent(u.ID, u.Email, model.EventTypeLockout, model.EventStatusSuccess, "", ip, userAgent)
				audit.Log(h.repo, u.ID, u.Email, ip,
					"user.locked", "user", u.ID,
					"Account locked due to too many failed login attempts")
			}
			common.Error(w, http.StatusTooManyRequests, "account locked. too many failed attempts")
			return
		}

		if errors.Is(err, ErrRateLimited) {
			h.recordAuthEvent("", req.Email, model.EventTypeLoginFailure, model.EventStatusFailure, "rate_limited", ip, userAgent)
			common.Error(w, http.StatusTooManyRequests, "too many attempts. please wait before trying again")
			return
		}

		if errors.Is(err, ErrInvalidCredentials) {
			h.recordAuthEvent("", req.Email, model.EventTypeLoginFailure, model.EventStatusFailure, "invalid_password", ip, userAgent)
			if locked, _ := h.svc.IsLocked(r.Context(), req.Email); locked {
				// Lockout event already handled above in the account_locked path
				// But we also need to re-raise if the account got locked just now
			}
			if u, lookupErr := h.svc.GetUserByEmail(r.Context(), req.Email); lookupErr == nil {
				if locked, _ := h.svc.IsLocked(r.Context(), req.Email); locked {
					h.recordAuthEvent(u.ID, u.Email, model.EventTypeLockout, model.EventStatusSuccess, "", ip, userAgent)
					audit.Log(h.repo, u.ID, u.Email, ip,
						"user.locked", "user", u.ID,
						"Account locked due to too many failed login attempts")
				}
			}
		}

		common.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Success — audit + record auth event
	if resp.User != nil {
		h.recordAuthEvent(resp.User.ID, resp.User.Email, model.EventTypeLoginSuccess, model.EventStatusSuccess, "", ip, userAgent)

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

	if !h.svc.IsRegistrationEnabled(r.Context()) {
		common.Error(w, http.StatusForbidden, "registration is disabled")
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

	ip := audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
	h.recordAuthEvent(user.ID, user.Email, model.EventTypeRegister, model.EventStatusSuccess, "", ip, r.Header.Get("User-Agent"))

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

// LoginHistory returns auth events for the currently authenticated user.
// Mounted under /auth/ so it only needs auth middleware, not admin.
func (h *Handler) LoginHistory(w http.ResponseWriter, r *http.Request) {
	claims := extractClaims(h.svc, r)
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := model.AuthEventQuery{
		Page:      common.ParseQueryInt(r, "page", 1),
		Limit:     common.ParseQueryInt(r, "limit", 30),
		EventType: r.URL.Query().Get("event_type"),
		Status:    r.URL.Query().Get("status"),
		Email:     claims.Email,
		IPAddress: r.URL.Query().Get("ip_address"),
		Search:    r.URL.Query().Get("search"),
		Sort:      r.URL.Query().Get("sort"),
		Order:     r.URL.Query().Get("order"),
	}

	resp, err := h.authEvents.ListAuthEvents(r.Context(), q)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list login history")
		return
	}

	common.JSONWithMeta(w, http.StatusOK, resp.Events, &common.Meta{
		Page:       resp.Page,
		PerPage:    resp.Limit,
		Total:      resp.Total,
		TotalPages: resp.TotalPages,
	})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims != nil && h.repo != nil {
		ip := audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
		h.recordAuthEvent(claims.UserID, claims.Email, model.EventTypeLogout, model.EventStatusSuccess, "", ip, r.Header.Get("User-Agent"))

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
	var req VerifyTOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ip := audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
	userAgent := r.Header.Get("User-Agent")
	resp, err := h.svc.VerifyTOTPCode(r.Context(), req.Email, req.Token)
	if err != nil {
		h.recordAuthEvent("", req.Email, model.EventTypeLoginFailure, model.EventStatusFailure, "totp_invalid", ip, userAgent)
		common.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	if resp.User != nil {
		h.recordAuthEvent(resp.User.ID, resp.User.Email, model.EventTypeLoginSuccess, model.EventStatusSuccess, "", ip, userAgent)
		meta, _ := json.Marshal(map[string]string{
			"user_name": resp.User.Name,
			"user_role": resp.User.Role,
			"method":    "totp",
		})
		audit.Log(h.repo, resp.User.ID, resp.User.Email, ip,
			"auth.login", "user", resp.User.ID,
			"User logged in with 2FA", json.RawMessage(meta))
	}

	common.JSON(w, http.StatusOK, resp)
}

// ─── TOTP 2FA endpoints ─────────────────────────────────────────────────────

type SetupTOTPRequest struct{}

type VerifyTOTPSetupRequest struct {
	Token string `json:"token"`
}

type DisableTOTPRequest struct {
	Password string `json:"password"`
}

type VerifyTOTPRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

// SetupTOTP generates TOTP secret + QR code for the authenticated user.
func (h *Handler) SetupTOTP(w http.ResponseWriter, r *http.Request) {
	claims := extractClaims(h.svc, r)
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resp, err := h.svc.SetupTOTP(r.Context(), claims.Email)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to setup 2FA")
		return
	}

	h.recordAuthEvent(claims.UserID, claims.Email, model.EventTypeTOTPSetup, model.EventStatusSuccess, "",
		audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For")), r.Header.Get("User-Agent"))

	meta, _ := json.Marshal(map[string]string{"user_email": claims.Email})
	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"auth.2fa_setup", "user", claims.UserID,
		"User initiated 2FA setup", json.RawMessage(meta))

	common.JSON(w, http.StatusOK, resp)
}

// VerifyTOTPSetup confirms TOTP setup with a code and enables 2FA.
func (h *Handler) VerifyTOTPSetup(w http.ResponseWriter, r *http.Request) {
	var req VerifyTOTPSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims := extractClaims(h.svc, r)
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.svc.VerifyTOTPSetup(r.Context(), claims.Email, req.Token); err != nil {
		common.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// No separate auth event for verify-setup; the setup event already captured the initiation
	meta, _ := json.Marshal(map[string]string{"user_email": claims.Email})
	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"auth.2fa_enable", "user", claims.UserID,
		"User enabled 2FA", json.RawMessage(meta))

	common.JSON(w, http.StatusOK, map[string]string{"message": "2FA enabled successfully"})
}

// DisableTOTP disables 2FA for the authenticated user.
func (h *Handler) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	var req DisableTOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims := extractClaims(h.svc, r)
	if claims == nil {
		common.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.svc.DisableTOTP(r.Context(), claims.Email, req.Password); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			common.Error(w, http.StatusUnauthorized, "password is incorrect")
			return
		}
		common.Error(w, http.StatusInternalServerError, "failed to disable 2FA")
		return
	}

	h.recordAuthEvent(claims.UserID, claims.Email, model.EventTypeTOTPDisable, model.EventStatusSuccess, "",
		audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For")), r.Header.Get("User-Agent"))

	meta, _ := json.Marshal(map[string]string{"user_email": claims.Email})
	audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
		"auth.2fa_disable", "user", claims.UserID,
		"User disabled 2FA", json.RawMessage(meta))

	common.JSON(w, http.StatusOK, map[string]string{"message": "2FA disabled successfully"})
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

	h.recordAuthEvent(claims.UserID, claims.Email, model.EventTypePasswordChange, model.EventStatusSuccess, "",
		audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For")), r.Header.Get("User-Agent"))

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

	ip := audit.RemoteIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
	meta, _ := json.Marshal(map[string]string{
		"user_name": user.Name,
		"user_role": user.Role,
	})
	audit.Log(h.repo, user.ID, user.Email, ip,
		"auth.profile_update", "user", user.ID,
		"User updated profile", json.RawMessage(meta))

	resp, err := h.svc.generateTokenPair(user)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	common.JSON(w, http.StatusOK, resp)
}

// recordAuthEvent records an auth event asynchronously (best-effort).
func (h *Handler) recordAuthEvent(userID, email, eventType, status, failureReason, ip, userAgent string) {
	if h.authEvents == nil {
		return
	}
	authactivity.RecordEvent(h.authEvents, userID, email, eventType, status, failureReason, ip, userAgent)
}
