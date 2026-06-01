package infra

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
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Delete("/{id}", h.Delete)
	r.Post("/{id}/test", h.TestConnection)
	return r
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	servers, err := h.repo.ListServers(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list servers")
		return
	}
	common.JSON(w, http.StatusOK, servers)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	srv, err := h.repo.GetServerByID(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "server not found")
		return
	}
	common.JSON(w, http.StatusOK, srv)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteServer(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete server")
		return
	}
	common.JSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
