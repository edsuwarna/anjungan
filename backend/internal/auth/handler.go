package auth

import (
	"encoding/json"
	"errors"
	"net/http"

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
