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
	deploymentCount, _ := h.repo.CountDeployments(r.Context())
	userCount, _ := h.repo.CountUsers(r.Context())
	statusCounts, _ := h.repo.CountServersByStatus(r.Context())
	compliance, _ := h.repo.GetComplianceSummary(r.Context())
	activity, _ := h.repo.ListRecentActivity(r.Context(), 10)

	if statusCounts == nil {
		statusCounts = map[string]int{}
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

	// Compact compliance summary (omit full server list to keep response lean)
	type ComplianceBrief struct {
		TotalServers   int            `json:"total_servers"`
		ScannedServers int            `json:"scanned_servers"`
		AverageScore   *int           `json:"average_score"`
		ByStatus       map[string]int `json:"by_status"`
	}
	comp := ComplianceBrief{
		TotalServers: serverCount,
		ByStatus:     map[string]int{},
	}
	if compliance != nil {
		comp.ScannedServers = compliance.ScannedServers
		comp.AverageScore = compliance.AverageScore
		comp.ByStatus = compliance.ByStatus
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"servers":         serverCount,
		"containers":      containerSum,
		"deployments":     deploymentCount,
		"users":           userCount,
		"server_status":   statusCounts,
		"compliance":      comp,
		"recent_activity": entries,
	})
}
