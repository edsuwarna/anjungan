package sslmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/edsuwarna/anjungan/internal/audit"
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

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/summary", h.Summary)
	r.Post("/check-all", h.CheckAll)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/{id}/check", h.CheckNow)
	return r
}

// ─── List ────────────────────────────────────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	if q.Get("all") == "true" {
		monitors, err := h.repo.ListSSLMonitors(r.Context(), q.Get("search"), q.Get("status"), false)
		if err != nil {
			common.Error(w, http.StatusInternalServerError, "failed to list monitors")
			return
		}
		resp := make([]model.SSLMonitorResponse, len(monitors))
		for i, m := range monitors {
			resp[i] = m.ToResponse()
		}
		common.JSON(w, http.StatusOK, resp)
		return
	}

	result, err := h.repo.ListSSLMonitorsPaginated(r.Context(), page, limit,
		q.Get("search"), q.Get("status"), q.Get("sort"), q.Get("order"), false)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list monitors")
		return
	}
	common.JSONWithMeta(w, http.StatusOK, result.Monitors, &common.Meta{
		Page:       result.Page,
		PerPage:    result.Limit,
		Total:      result.Total,
		TotalPages: result.TotalPages,
	})
}

// ─── Create ──────────────────────────────────────────────────────────────────

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input model.CreateSSLMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Domain == "" {
		common.Error(w, http.StatusBadRequest, "domain is required")
		return
	}
	if input.Port == 0 {
		input.Port = 443
	}
	if input.CheckInterval == "" {
		input.CheckInterval = "1h"
	}
	if input.NotifyBefore == "" {
		input.NotifyBefore = "14d"
	}

	// Check duplicate
	existing, _ := h.repo.GetSSLMonitorByDomainPort(r.Context(), input.Domain, input.Port)
	if existing != nil {
		common.Error(w, http.StatusConflict, fmt.Sprintf("monitor for %s:%d already exists", input.Domain, input.Port))
		return
	}

	userID := ""
	if claims := auth.GetClaims(r.Context()); claims != nil {
		userID = claims.UserID
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	now := time.Now()
	monitor := &model.SSLMonitor{
		ID:            uuid.New().String(),
		Domain:        input.Domain,
		Port:          input.Port,
		DisplayName:   input.DisplayName,
		CheckInterval: input.CheckInterval,
		NotifyBefore:  input.NotifyBefore,
		WebhookIDs:    input.WebhookIDs,
		Enabled:       enabled,
		CreatedBy:     userID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := h.repo.CreateSSLMonitor(r.Context(), monitor); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create monitor")
		return
	}

	// Audit
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]interface{}{
			"domain": monitor.Domain,
			"port":   monitor.Port,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"sslmonitor.create", "ssl_monitor", monitor.ID,
			fmt.Sprintf("Created SSL monitor for %s:%d", monitor.Domain, monitor.Port),
			json.RawMessage(meta))
	}

	// Run initial check in background
	go h.runCheck(context.Background(), monitor)

	common.JSON(w, http.StatusCreated, monitor.ToResponse())
}

// ─── Get ─────────────────────────────────────────────────────────────────────

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	monitor, err := h.repo.GetSSLMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}
	common.JSON(w, http.StatusOK, monitor.ToResponse())
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	monitor, err := h.repo.GetSSLMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	var input model.UpdateSSLMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.DisplayName != nil {
		monitor.DisplayName = *input.DisplayName
	}
	if input.Port != nil {
		monitor.Port = *input.Port
	}
	if input.CheckInterval != nil {
		monitor.CheckInterval = *input.CheckInterval
	}
	if input.NotifyBefore != nil {
		monitor.NotifyBefore = *input.NotifyBefore
	}
	if input.WebhookIDs != nil {
		monitor.WebhookIDs = *input.WebhookIDs
	}
	if input.Enabled != nil {
		monitor.Enabled = *input.Enabled
	}

	if err := h.repo.UpdateSSLMonitor(r.Context(), monitor); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to update monitor")
		return
	}

	// Audit
	if claims := auth.GetClaims(r.Context()); claims != nil {
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"sslmonitor.update", "ssl_monitor", monitor.ID,
			fmt.Sprintf("Updated SSL monitor for %s:%d", monitor.Domain, monitor.Port), nil)
	}

	common.JSON(w, http.StatusOK, monitor.ToResponse())
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	monitor, err := h.repo.GetSSLMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	if err := h.repo.DeleteSSLMonitor(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete monitor")
		return
	}

	// Audit
	if claims := auth.GetClaims(r.Context()); claims != nil {
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"sslmonitor.delete", "ssl_monitor", id,
			fmt.Sprintf("Deleted SSL monitor for %s:%d", monitor.Domain, monitor.Port), nil)
	}

	common.JSON(w, http.StatusOK, map[string]string{"message": "monitor deleted"})
}

// ─── CheckNow ────────────────────────────────────────────────────────────────

func (h *Handler) CheckNow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	monitor, err := h.repo.GetSSLMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	result := Check(r.Context(), monitor.Domain, monitor.Port)
	h.saveResult(r.Context(), monitor, result)

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "check completed",
		"result":  result,
	})
}

// ─── CheckAll ────────────────────────────────────────────────────────────────

func (h *Handler) CheckAll(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.repo.ListEnabledSSLMonitors(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list monitors")
		return
	}

	type checkResult struct {
		Domain string       `json:"domain"`
		Result *CheckResult `json:"result"`
		Error  string       `json:"error,omitempty"`
	}

	results := make([]checkResult, 0, len(monitors))
	for _, m := range monitors {
		result := Check(r.Context(), m.Domain, m.Port)
		h.saveResult(r.Context(), m, result)
		results = append(results, checkResult{
			Domain: m.Domain,
			Result: result,
		})
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("checked %d monitors", len(results)),
		"results": results,
	})
}

// ─── Summary ─────────────────────────────────────────────────────────────────

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	byStatus, err := h.repo.CountSSLMonitorsByStatus(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get summary")
		return
	}

	total := 0
	for _, c := range byStatus {
		total += c
	}

	summary := model.SSLSummary{
		Total: total,
	}
	for status, count := range byStatus {
		switch status {
		case "valid":
			summary.Valid = count
		case "expiring_soon":
			summary.ExpiringSoon = count
		case "expired":
			summary.Expired = count
		default:
			summary.Error += count
		}
	}

	common.JSON(w, http.StatusOK, summary)
}

// ─── Internal ────────────────────────────────────────────────────────────────

func (h *Handler) saveResult(ctx context.Context, m *model.SSLMonitor, r *CheckResult) {
	now := time.Now().UTC()

	m.LastStatus = r.Status
	m.LastCheckAt = &now
	m.LastError = r.Error
	m.Issuer = r.Issuer
	m.Subject = r.Subject
	m.CertExpiresAt = r.CertExpiresAt
	m.DaysRemaining = r.DaysRemaining
	m.ChainValid = r.ChainValid
	m.ChainError = r.ChainError
	m.CipherGrade = r.CipherGrade
	m.CipherError = r.CipherError
	m.OCSPStatus = r.OCSPStatus
	m.OCSPError = r.OCSPError
	m.SANNames = r.SANNames
	m.SANMismatch = r.SANMismatch

	_ = h.repo.UpdateSSLMonitorCheckResult(ctx, m)
}

// runCheck is a goroutine-safe check runner (used after create).
func (h *Handler) runCheck(ctx context.Context, m *model.SSLMonitor) {
	result := Check(ctx, m.Domain, m.Port)
	h.saveResult(ctx, m, result)
}
