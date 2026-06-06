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
	deploymentStatus, _ := h.repo.CountDeploymentsByStatus(r.Context())
	recentDeployments, _ := h.repo.ListRecentDeployments(r.Context(), 5)

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
	if deploymentStatus == nil {
		deploymentStatus = map[string]int{}
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

	// Deployment brief for dashboard
	type DeploymentBrief struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		ServerName string `json:"server_name"`
		DeployedAt string `json:"deployed_at"`
	}
	depBriefs := make([]DeploymentBrief, 0)
	for _, d := range recentDeployments {
		srvName := ""
		if d.ServerName != nil {
			srvName = *d.ServerName
		}
		depBriefs = append(depBriefs, DeploymentBrief{
			ID:         d.ID,
			Name:       d.Name,
			Status:     d.Status,
			ServerName: srvName,
			DeployedAt: d.DeployedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
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

	// Per-server compliance scores for quick lookup on server cards
	type ServerScore struct {
		Score  *int   `json:"score"`
		Status string `json:"status"` // "good", "warning", "critical", "unscanned"
	}
	serverScores := map[string]ServerScore{}
	if compliance != nil {
		for _, s := range compliance.Servers {
			ss := ServerScore{Score: s.Score}
			if s.Score == nil {
				ss.Status = "unscanned"
			} else if *s.Score >= 80 {
				ss.Status = "good"
			} else if *s.Score >= 60 {
				ss.Status = "warning"
			} else {
				ss.Status = "critical"
			}
			serverScores[s.ID] = ss
		}
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"servers":             serverCount,
		"containers":          containerSum,
		"deployments":         deploymentCount,
		"users":               userCount,
		"server_status":       statusCounts,
		"deployment_status":   deploymentStatus,
		"compliance":          comp,
		"server_scores":       serverScores,
		"recent_activity":     entries,
		"recent_deployments":  depBriefs,
	})
}
