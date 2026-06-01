package container

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/edsuwarna/anjungan/internal/common"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/{id}", h.Get)
	r.Post("/{id}/start", h.Start)
	r.Post("/{id}/stop", h.Stop)
	r.Post("/{id}/restart", h.Restart)
	r.Get("/{id}/logs", h.Logs)
	r.Get("/stats", h.Stats)
	r.Route("/compose", func(r chi.Router) {
		r.Post("/", h.ComposeUp)
		r.Post("/{stack}/down", h.ComposeDown)
		r.Get("/{stack}", h.ComposeStatus)
	})
	return r
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) ComposeUp(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) ComposeDown(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
func (h *Handler) ComposeStatus(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
