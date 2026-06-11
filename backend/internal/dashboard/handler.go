package dashboard

import (
	"net/http"
	"time"

	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

type Handler struct {
	repo *db.Repository
}

func NewHandler(repo *db.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	// Determine user's allowed groups for filtered counts
	var allowedGroups []string
	isAdmin := true
	if claims := auth.GetClaims(r.Context()); claims != nil {
		isAdmin = claims.Role == model.RoleAdmin
		if !isAdmin {
			groups, err := h.repo.GetUserServerGroups(r.Context(), claims.UserID)
			if err != nil {
				allowedGroups = []string{}
			} else {
				allowedGroups = groups
			}
		}
	}
	// Admin → allowedGroups = nil → no group filter
	// Non-admin → filter by groups (empty slice → return zeros)

	serverCount, _ := h.repo.CountServersByGroups(r.Context(), allowedGroups)
	containerSum, _ := h.repo.SumContainerCountByGroups(r.Context(), allowedGroups)
	statusCounts, _ := h.repo.CountServersByStatusByGroups(r.Context(), allowedGroups)
	compliance, _ := h.repo.GetComplianceSummary(r.Context(), allowedGroups)

	// SSL Summary — shared across all users (matches SSL Monitors page access)
	var sslSummary model.SSLSummary
	byStatus, _ := h.repo.CountSSLMonitorsByStatus(r.Context())
	if byStatus != nil {
		total := 0
		for _, c := range byStatus {
			total += c
		}
		sslSummary = model.SSLSummary{Total: total}
		for status, count := range byStatus {
			switch status {
			case "valid":
				sslSummary.Valid = count
			case "expiring_soon":
				sslSummary.ExpiringSoon = count
			case "expired":
				sslSummary.Expired = count
			default:
				sslSummary.Error += count
			}
		}
	}

	// Uptime Summary — shared across all users
	var uptimeSummary = &model.UptimeSummary{}
	if s, err := h.repo.GetUptimeSummary(r.Context()); err == nil && s != nil {
		uptimeSummary = s
	}

	if statusCounts == nil {
		statusCounts = map[string]int{}
	}

	// Admin-only fields (deployments, users, activity)
	var deploymentCount int
	var userCount int
	var deploymentStatus map[string]int
	var entries []struct {
		Type      string `json:"type"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
	}
	var depBriefs []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		ServerName string `json:"server_name"`
		DeployedAt string `json:"deployed_at"`
	}

	if isAdmin {
		deploymentCount, _ = h.repo.CountDeployments(r.Context())
		userCount, _ = h.repo.CountUsers(r.Context())
		deploymentStatus, _ = h.repo.CountDeploymentsByStatus(r.Context())

		activity, _ := h.repo.ListRecentActivity(r.Context(), 10)
		if activity == nil {
			activity = []struct {
				Type      string    `json:"type"`
				Message   string    `json:"message"`
				Timestamp time.Time `json:"timestamp"`
			}{}
		}
		entries = make([]struct {
			Type      string `json:"type"`
			Message   string `json:"message"`
			Timestamp string `json:"timestamp"`
		}, len(activity))
		for i, a := range activity {
			entries[i] = struct {
				Type      string `json:"type"`
				Message   string `json:"message"`
				Timestamp string `json:"timestamp"`
			}{
				Type:      a.Type,
				Message:   a.Message,
				Timestamp: a.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			}
		}

		if deploymentStatus == nil {
			deploymentStatus = map[string]int{}
		}

		recentDeployments, _ := h.repo.ListRecentDeployments(r.Context(), 5)
		depBriefs = make([]struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			ServerName string `json:"server_name"`
			DeployedAt string `json:"deployed_at"`
		}, 0)
		for _, d := range recentDeployments {
			srvName := ""
			if d.ServerName != nil {
				srvName = *d.ServerName
			}
			depBriefs = append(depBriefs, struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				Status     string `json:"status"`
				ServerName string `json:"server_name"`
				DeployedAt string `json:"deployed_at"`
			}{
				ID:         d.ID,
				Name:       d.Name,
				Status:     d.Status,
				ServerName: srvName,
				DeployedAt: d.DeployedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	}

	if entries == nil {
		entries = []struct {
			Type      string `json:"type"`
			Message   string `json:"message"`
			Timestamp string `json:"timestamp"`
		}{}
	}
	if deploymentStatus == nil {
		deploymentStatus = map[string]int{}
	}
	if depBriefs == nil {
		depBriefs = []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			ServerName string `json:"server_name"`
			DeployedAt string `json:"deployed_at"`
		}{}
	}

	// Compact compliance summary
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

	// Per-server compliance scores
	type ServerScore struct {
		Score  *int   `json:"score"`
		Status string `json:"status"`
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
		"servers":            serverCount,
		"containers":         containerSum,
		"server_status":      statusCounts,
		"compliance":         comp,
		"server_scores":      serverScores,
		"deployments":        deploymentCount,
		"users":              userCount,
		"deployment_status":  deploymentStatus,
		"recent_activity":    entries,
		"recent_deployments": depBriefs,
		"ssl_summary":        sslSummary,
		"uptime_summary":     uptimeSummary,
	})
}
