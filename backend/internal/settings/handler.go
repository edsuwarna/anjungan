package settings

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

// Handler handles HTTP requests for application settings.
type Handler struct {
	repo *db.Repository
}

// NewHandler creates a new settings Handler.
func NewHandler(repo *db.Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes returns the chi.Router for settings endpoints.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/compliance-thresholds", h.GetComplianceThresholds)
	r.Put("/compliance-thresholds", h.UpdateComplianceThresholds)
	return r
}

// GetComplianceThresholds returns the current compliance score thresholds.
func (h *Handler) GetComplianceThresholds(w http.ResponseWriter, r *http.Request) {
	thresholds, err := h.repo.GetComplianceThresholds(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "Failed to load thresholds")
		return
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"thresholds": thresholds,
		"defaults":   model.DefaultComplianceThresholds(),
	})
}

// UpdateComplianceThresholds updates the compliance score thresholds.
func (h *Handler) UpdateComplianceThresholds(w http.ResponseWriter, r *http.Request) {
	var input model.ComplianceThresholds
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate
	if input.Compliant <= 0 || input.Warning <= 0 || input.Compliant <= input.Warning {
		common.Error(w, http.StatusBadRequest, "compliant must be > warning > 0 (e.g. compliant=90, warning=70)")
		return
	}

	payload, _ := json.Marshal(input)
	if err := h.repo.UpsertSetting(r.Context(), "compliance_thresholds", string(payload),
		"Compliance score thresholds: compliant (green) and warning (yellow) minimum percentages. Below warning = critical (red)."); err != nil {
		common.Error(w, http.StatusInternalServerError, "Failed to save thresholds")
		return
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"thresholds": input,
		"message":    "Thresholds updated",
	})
}
