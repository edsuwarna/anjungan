package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
)

type Handler struct {
	repo *db.Repository
}

func NewHandler(repo *db.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/users", h.ListUsers)
	r.Get("/users/{id}", h.GetUser)
	r.Put("/users/{id}", h.UpdateUser)
	r.Delete("/users/{id}", h.DeleteUser)
	r.Get("/audit-log", h.AuditLog)
	return r
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListUsers(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	common.JSON(w, http.StatusOK, users)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) AuditLog(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
