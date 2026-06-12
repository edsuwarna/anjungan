package sslmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
	"github.com/edsuwarna/anjungan/internal/notification"
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
	r.Get("/export/csv", h.ExportCSV)
	r.Post("/import", h.BatchImport)
	r.Post("/check-all", h.CheckAll)
	r.Post("/discover", h.Discover)
	r.Post("/discover/import", h.DiscoverImport)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/{id}/check", h.CheckNow)
	r.Get("/{id}/history", h.History)
	r.Get("/{id}/trend", h.Trend)
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

// ─── History ──────────────────────────────────────────────────────────────

func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	result, err := h.repo.ListSSLCheckHistory(r.Context(), id, limit, offset)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list history")
		return
	}
	common.JSON(w, http.StatusOK, result)
}

func (h *Handler) Trend(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit := 90
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	entries, err := h.repo.GetSSLMonitorTrend(r.Context(), id, limit)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []model.SSLCheckHistory{}
	}
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
	})
}

// ─── Internal ────────────────────────────────────────────────────────────────

func (h *Handler) saveResult(ctx context.Context, m *model.SSLMonitor, r *CheckResult) {
	now := time.Now().UTC()

	// Capture previous status for notification dedup
	prevStatus := m.LastStatus

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

	// Save to check history
	history := &model.SSLCheckHistory{
		ID:            uuid.New().String(),
		SSLMonitorID:  m.ID,
		CheckedAt:     now,
		Status:        r.Status,
		DaysRemaining: r.DaysRemaining,
		CipherGrade:   r.CipherGrade,
		TLSVersion:    r.TLSVersion,
		CipherSuite:   r.CipherName,
		Issuer:        r.Issuer,
		Subject:       r.Subject,
		ErrorMessage:  r.Error,
	}
	_ = h.repo.CreateSSLCheckHistory(ctx, history)

	// Send notification if status changed to expiring or expired
	needsNotify := (r.Status == "expiring_soon" || r.Status == "expired") &&
		prevStatus != r.Status

	// Also notify if just now crossed the threshold
	if r.Status == "valid" && r.DaysRemaining <= 30 && prevStatus != "expiring_soon" && prevStatus != "expired" {
		needsNotify = true
	}

	if needsNotify && len(m.WebhookIDs) > 0 {
		go h.dispatchNotification(context.Background(), m, r, prevStatus)
	}
}

// runCheck is a goroutine-safe check runner (used after create).
func (h *Handler) runCheck(ctx context.Context, m *model.SSLMonitor) {
	result := Check(ctx, m.Domain, m.Port)
	h.saveResult(ctx, m, result)
}

// ─── Notification Dispatch ──────────────────────────────────────────────────

func (h *Handler) dispatchNotification(ctx context.Context, m *model.SSLMonitor, r *CheckResult, prevStatus string) {
	if len(m.WebhookIDs) == 0 {
		return
	}

	// Load all enabled notification targets
	allTargets, err := h.repo.ListNotificationTargets(ctx, "")
	if err != nil {
		log.Printf("[sslmonitor] failed to load notification targets for %s: %v", m.ID, err)
		return
	}

	// Build target lookup map
	targetMap := make(map[string]model.NotificationTarget, len(allTargets))
	for _, t := range allTargets {
		targetMap[t.ID] = t
	}

	// Build payload once
	payload := buildSSLNotificationPayload(m, r, prevStatus)

	for _, targetID := range m.WebhookIDs {
		target, ok := targetMap[targetID]
		if !ok || !target.Enabled {
			continue
		}

		statusCode, respBody, err := notification.SendSSLToTarget(&target, payload)
		if err != nil {
			log.Printf("[sslmonitor] failed to send notification to %s (%s): %v", target.Name, target.URL, err)
		} else {
			log.Printf("[sslmonitor] notification sent to %s — status %d", target.Name, statusCode)
			_ = respBody
		}
	}
}

// buildSSLNotificationPayload creates a unified payload with full SSL certificate info.
func buildSSLNotificationPayload(m *model.SSLMonitor, r *CheckResult, prevStatus string) map[string]interface{} {
	displayName := m.Domain
	if m.DisplayName != "" {
		displayName = m.DisplayName
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	wib := time.Now().In(loc)

	expiresAt := ""
	if r.CertExpiresAt != nil {
		expiresAt = r.CertExpiresAt.In(loc).Format("2006-01-02 15:04:05 WIB")
	}

	// Build human-readable message
	var message string
	var emoji string
	switch r.Status {
	case "expired":
		emoji = "🔴"
		message = fmt.Sprintf("🔴 SSL Certificate %s has EXPIRED! 🔴", displayName)
	case "expiring_soon":
		emoji = "🟡"
		message = fmt.Sprintf("🟡 SSL Certificate %s is expiring soon! (%d days remaining)", displayName, r.DaysRemaining)
	case "valid":
		emoji = "🟢"
		message = fmt.Sprintf("🟢 SSL Certificate %s is valid (%d days remaining)", displayName, r.DaysRemaining)
	default:
		emoji = "⚪"
		message = fmt.Sprintf("SSL Certificate %s status: %s", displayName, r.Status)
	}

	return map[string]interface{}{
		"event_type":      "ssl.expiry",
		"monitor_id":      m.ID,
		"domain":          m.Domain,
		"port":            m.Port,
		"display_name":    displayName,
		"status":          r.Status,
		"previous_status": prevStatus,
		"days_remaining":  r.DaysRemaining,
		"emoji":           emoji,
		"message":         message,
		"issuer":          r.Issuer,
		"subject":         r.Subject,
		"expires_at":      expiresAt,
		"cipher_grade":    r.CipherGrade,
		"cipher_error":    r.CipherError,
		"chain_valid":     r.ChainValid,
		"chain_error":     r.ChainError,
		"ocsp_status":     r.OCSPStatus,
		"ocsp_error":      r.OCSPError,
		"san_mismatch":    r.SANMismatch,
		"error":           r.Error,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
		"timestamp_wib":   wib.Format("2006-01-02 15:04:05"),
	}
}

// ─── Export CSV ───────────────────────────────────────────────────────────────

func (h *Handler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.repo.ListSSLMonitors(r.Context(), "", "", false)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list monitors")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=ssl-monitors-export.csv")

	// Write BOM for Excel compatibility
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	// CSV header
	w.Write([]byte("domain,port,display_name,status,days_remaining,issuer,subject,cert_expires_at," +
		"cipher_grade,chain_valid,ocsp_status,san_names,san_mismatch," +
		"last_checked_at,last_error,check_interval,notify_before,enabled,created_at\n"))

	for _, m := range monitors {
		certExpires := ""
		if m.CertExpiresAt != nil {
			certExpires = m.CertExpiresAt.Format(time.RFC3339)
		}
		lastChecked := ""
		if m.LastCheckAt != nil {
			lastChecked = m.LastCheckAt.Format(time.RFC3339)
		}
		chainValid := ""
		if m.ChainValid != nil {
			if *m.ChainValid {
				chainValid = "valid"
			} else {
				chainValid = "invalid"
			}
		}
		enabled := "false"
		if m.Enabled {
			enabled = "true"
		}

		sanStr := strings.Join(m.SANNames, "; ")
		sanMismatch := "false"
		if m.SANMismatch {
			sanMismatch = "true"
		}

		line := fmt.Sprintf("%s,%d,%s,%s,%d,%s,%s,%s,%s,%s,%s,\"%s\",%s,%s,%s,%s,%s,%s,%s\n",
			csvEscape(m.Domain),
			m.Port,
			csvEscape(m.DisplayName),
			m.LastStatus,
			m.DaysRemaining,
			csvEscape(m.Issuer),
			csvEscape(m.Subject),
			certExpires,
			m.CipherGrade,
			chainValid,
			m.OCSPStatus,
			sanStr,
			sanMismatch,
			lastChecked,
			csvEscape(m.LastError),
			m.CheckInterval,
			m.NotifyBefore,
			enabled,
			m.CreatedAt.Format(time.RFC3339),
		)
		w.Write([]byte(line))
	}
}

func csvEscape(s string) string {
	if s == "" {
		return ""
	}
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// ─── Batch Import ─────────────────────────────────────────────────────────────

type BatchImportRequest struct {
	Domains       []string `json:"domains"`
	Port          int      `json:"port"`
	CheckInterval string   `json:"check_interval,omitempty"`
	NotifyBefore  string   `json:"notify_before,omitempty"`
	WebhookIDs    []string `json:"webhook_ids,omitempty"`
	Enabled       *bool    `json:"enabled,omitempty"`
}

type BatchImportResult struct {
	Created int                      `json:"created"`
	Skipped int                      `json:"skipped"`
	Errors  int                      `json:"errors"`
	Details []BatchImportDetail      `json:"details"`
}

type BatchImportDetail struct {
	Domain string `json:"domain"`
	Status string `json:"status"` // created, skipped, error
	Error  string `json:"error,omitempty"`
}

func (h *Handler) BatchImport(w http.ResponseWriter, r *http.Request) {
	var input BatchImportRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(input.Domains) == 0 {
		common.Error(w, http.StatusBadRequest, "domains array is required")
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

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	userID := ""
	if claims := auth.GetClaims(r.Context()); claims != nil {
		userID = claims.UserID
	}

	result := BatchImportResult{
		Details: make([]BatchImportDetail, 0, len(input.Domains)),
	}

	for _, domain := range input.Domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}

		// Check duplicate
		existing, _ := h.repo.GetSSLMonitorByDomainPort(r.Context(), domain, input.Port)
		if existing != nil {
			result.Skipped++
			result.Details = append(result.Details, BatchImportDetail{
				Domain: domain,
				Status: "skipped",
				Error:  "already exists",
			})
			continue
		}

		now := time.Now()
		displayName := ""
		monitor := &model.SSLMonitor{
			ID:            uuid.New().String(),
			Domain:        domain,
			Port:          input.Port,
			DisplayName:   displayName,
			CheckInterval: input.CheckInterval,
			NotifyBefore:  input.NotifyBefore,
			WebhookIDs:    input.WebhookIDs,
			Enabled:       enabled,
			CreatedBy:     userID,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		if err := h.repo.CreateSSLMonitor(r.Context(), monitor); err != nil {
			result.Errors++
			result.Details = append(result.Details, BatchImportDetail{
				Domain: domain,
				Status: "error",
				Error:  err.Error(),
			})
			continue
		}

		result.Created++

		// Run initial check in background
		go h.runCheck(context.Background(), monitor)

		result.Details = append(result.Details, BatchImportDetail{
			Domain: domain,
			Status: "created",
		})
	}

	// Audit
	if claims := auth.GetClaims(r.Context()); claims != nil {
		meta, _ := json.Marshal(map[string]interface{}{
			"total":   len(input.Domains),
			"created": result.Created,
			"skipped": result.Skipped,
			"errors":  result.Errors,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"sslmonitor.batch-import", "ssl_monitor", "",
			fmt.Sprintf("Batch imported %d SSL monitors (%d created, %d skipped, %d errors)",
				len(input.Domains), result.Created, result.Skipped, result.Errors),
			json.RawMessage(meta))
	}

	common.JSON(w, http.StatusCreated, result)
}

// ─── Discovery ─────────────────────────────────────────────────────────────────

func (h *Handler) Discover(w http.ResponseWriter, r *http.Request) {
	var req model.SSLDiscoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	server, err := h.repo.GetServerByIDFull(r.Context(), req.ServerID)
	if err != nil {
		common.Error(w, http.StatusNotFound, "Server not found")
		return
	}

	provider := req.Provider
	if provider == "" {
		provider = "auto"
	}

	disc := NewDiscoverer(h.repo)
	result, err := disc.Discover(r.Context(), server, provider)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	common.JSON(w, http.StatusOK, result)
}

func (h *Handler) DiscoverImport(w http.ResponseWriter, r *http.Request) {
	var req model.SSLDiscoveryImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	claims := auth.GetClaims(r.Context())
	createdBy := ""
	if claims != nil {
		createdBy = claims.UserID
	}

	var imported []model.SSLMonitorResponse
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	for _, d := range req.Domains {
		// Check if already exists by domain+port
		existing, _ := h.repo.GetSSLMonitorByDomainPort(r.Context(), d.Domain, d.Port)
		if existing != nil {
			continue // skip duplicates
		}

		now := time.Now()
		monitor := &model.SSLMonitor{
			ID:             uuid.New().String(),
			Domain:         d.Domain,
			Port:           d.Port,
			DisplayName:    d.DisplayName,
			CreatedBy:      createdBy,
			Enabled:        enabled,
			LastStatus:     "pending",
			SourceProvider: d.SourceProvider,
			ServerID:       d.ServerID,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if err := h.repo.CreateSSLMonitor(r.Context(), monitor); err != nil {
			continue
		}
		imported = append(imported, monitor.ToResponse())
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"imported": imported,
		"count":    len(imported),
	})
}
