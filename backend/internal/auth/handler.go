package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/edsuwarna/anjungan/internal/common"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.Login(r.Context(), req.Email, req.Password, req.TOTPCode)
	if err != nil {
		if errors.Is(err, ErrTOTPRequired) {
			common.JSON(w, http.StatusOK, map[string]string{"status": "totp_required"})
			return
		}
		common.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
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
		common.Error(w, http.StatusConflict, "email already registered")
		return
	}

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
	common.JSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *Handler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
