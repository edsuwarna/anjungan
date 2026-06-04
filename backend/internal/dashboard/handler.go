package dashboard

import (
	"net/http"
	"time"

	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
)

type Handler struct {
	repo *db.Repository
}

func NewHandler(repo *db.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	serverCount, _ := h.repo.CountServers(r.Context())
	containerSum, _ := h.repo.SumContainerCount(r.Context())
	userCount, _ := h.repo.CountUsers(r.Context())
	statusCounts, _ := h.repo.CountServersByStatus(r.Context())
	alertsCount, _ := h.repo.CountUnacknowledgedAlerts(r.Context())
	alertsBySeverity, _ := h.repo.CountAlertsBySeverity(r.Context())
	activity, _ := h.repo.ListRecentActivity(r.Context(), 20)

	if statusCounts == nil {
		statusCounts = map[string]int{}
	}
	if alertsBySeverity == nil {
		alertsBySeverity = map[string]int{}
	}
	if activity == nil {
		activity = []struct {
			Type      string    `json:"type"`
			Message   string    `json:"message"`
			Timestamp time.Time `json:"timestamp"`
		}{}
	}

	type ActivityEntry struct {
		Type      string `json:"type"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
	}
	entries := make([]ActivityEntry, len(activity))
	for i, a := range activity {
		entries[i] = ActivityEntry{
			Type:      a.Type,
			Message:   a.Message,
			Timestamp: a.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"servers":           serverCount,
		"containers":        containerSum,
		"deployments":       0,
		"users":             userCount,
		"alerts":            alertsCount,
		"alerts_by_severity": alertsBySeverity,
		"server_status":     statusCounts,
		"recent_activity":   entries,
	})
}
