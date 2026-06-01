package repository

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/edsuwarna/anjungan/internal/common"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/{owner}/{repo}/actions", h.ListActions)
	r.Get("/{owner}/{repo}/workflows", h.ListWorkflows)
	return r
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) ListActions(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
