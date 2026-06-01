package dashboard

import (
	"net/http"

	"github.com/edsuwarna/anjungan/internal/common"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"servers":     0,
		"containers":  0,
		"deployments": 0,
		"alerts":      0,
	})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	common.Error(w, http.StatusNotImplemented, "not implemented yet")
}
