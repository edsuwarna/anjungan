package sslmonitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	// Notification Targets management
	r.Route("/notification-targets", func(r chi.Router) {
		r.Get("/", h.ListNotificationTargets)
		r.Post("/", h.CreateNotificationTarget)
		r.Get("/{id}", h.GetNotificationTarget)
		r.Put("/{id}", h.UpdateNotificationTarget)
		r.Delete("/{id}", h.DeleteNotificationTarget)
		r.Post("/{id}/test", h.TestNotificationTarget)
	})
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
	// Fetch notification targets from the shared table with scope 'ssl'
	var targets []model.NotificationTarget
	if len(m.WebhookIDs) > 0 {
		allTargets, err := h.repo.ListNotificationTargets(ctx, "ssl")
		if err == nil {
			for _, t := range allTargets {
				for _, wid := range m.WebhookIDs {
					if t.ID == wid {
						targets = append(targets, t)
						break
					}
				}
			}
		}
	}

	// Fallback: legacy ssl_notification_targets table
	if len(targets) == 0 {
		legacyTargets, err := h.repo.ListSSLNotificationTargetsByIDs(ctx, m.WebhookIDs)
		if err == nil {
			for _, lt := range legacyTargets {
				targets = append(targets, model.NotificationTarget{
					ID:            lt.ID,
					Name:          lt.Name,
					URL:           lt.URL,
					Platform:      lt.Platform,
					WebhookSecret: lt.WebhookSecret,
					Enabled:       lt.Enabled,
				})
			}
		}
	}

	if len(targets) == 0 {
		return
	}

	displayName := m.Domain
	if m.DisplayName != "" {
		displayName = m.DisplayName
	}

	// Format expiry date for display
	expiryStr := "N/A"
	if r.CertExpiresAt != nil {
		expiryStr = r.CertExpiresAt.Format("2006-01-02")
	}

	// Build notification payload
	payload := map[string]interface{}{
		"event_type":      "ssl.expiry",
		"domain":          m.Domain,
		"port":            m.Port,
		"display_name":    displayName,
		"status":          r.Status,
		"days_remaining":  r.DaysRemaining,
		"issuer":          r.Issuer,
		"subject":         r.Subject,
		"expires_at":      expiryStr,
		"cipher_grade":    r.CipherGrade,
		"tls_version":     r.TLSVersion,
		"san_mismatch":    r.SANMismatch,
		"ocsp_status":     r.OCSPStatus,
		"previous_status": prevStatus,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}

	for _, target := range targets {
		statusCode, respBody, err := dispatchToTarget(&target, payload)
		if err != nil {
			log.Printf("[sslmonitor] notification target %s failed: %v", target.Name, err)
			continue
		}
		log.Printf("[sslmonitor] notification target %s delivered: %d", target.Name, statusCode)
		_ = statusCode
		_ = respBody
	}
}

// dispatchToTarget sends the payload to the notification target URL.
func dispatchToTarget(target *model.NotificationTarget, payload map[string]interface{}) (int, string, error) {
	var bodyBytes []byte
	var err error

	switch target.Platform {
	case "telegram":
		bodyBytes, err = formatTelegramNotification(payload)
	case "discord":
		bodyBytes, err = formatDiscordNotification(payload)
	case "slack":
		bodyBytes, err = formatSlackNotification(payload)
	default:
		bodyBytes, err = json.Marshal(payload)
	}

	if err != nil {
		return 0, "", fmt.Errorf("format message: %w", err)
	}

	req, err := http.NewRequest("POST", target.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "anjungan-sslmonitor-webhook/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

// ─── Notification Formatters ─────────────────────────────────────────────────

func formatTelegramNotification(payload map[string]interface{}) ([]byte, error) {
	domain, _ := payload["domain"].(string)
	displayName, _ := payload["display_name"].(string)
	status, _ := payload["status"].(string)
	days, _ := payload["days_remaining"].(int)
	issuer, _ := payload["issuer"].(string)
	expiresAt, _ := payload["expires_at"].(string)
	grade, _ := payload["cipher_grade"].(string)
	tlsVer, _ := payload["tls_version"].(string)
	sanMismatch, _ := payload["san_mismatch"].(bool)

	name := domain
	if displayName != "" {
		name = displayName
	}

	// Timezone WIB
	loc, _ := time.LoadLocation("Asia/Jakarta")
	nowWIB := time.Now().In(loc).Format("2006-01-02 15:04:05")

	var text string

	switch status {
	case "valid":
		text = fmt.Sprintf("🟢 *%s certificate is valid*\n", name) +
			fmt.Sprintf("Domain: `%s`\n", domain) +
			fmt.Sprintf("Expires: `%s` (%d days)\n", expiresAt, days) +
			fmt.Sprintf("Issuer: `%s`\n", issuer)
		if grade != "" {
			text += fmt.Sprintf("Grade: `%s`", grade)
		}

	case "expiring_soon":
		text = fmt.Sprintf("🟡 *%s certificate expiring soon*\n", name) +
			fmt.Sprintf("Domain: `%s`\n", domain) +
			fmt.Sprintf("Expires: `%s` (%d days)\n", expiresAt, days) +
			fmt.Sprintf("Issuer: `%s`\n", issuer)
		if grade != "" {
			text += fmt.Sprintf("Grade: `%s`\n", grade)
		}
		if tlsVer != "" {
			text += fmt.Sprintf("TLS: `%s`\n", tlsVer)
		}
		if sanMismatch {
			text += "⚠️ SAN mismatch detected\n"
		}
		text += fmt.Sprintf("\n⚠️ Renew needed in %d days", days)

	case "expired":
		text = fmt.Sprintf("🔴 *%s certificate EXPIRED*\n", name) +
			fmt.Sprintf("Domain: `%s`\n", domain) +
			fmt.Sprintf("Expired: `%s` (%d days ago)\n", expiresAt, days) +
			fmt.Sprintf("Issuer: `%s`\n", issuer)
		if sanMismatch {
			text += "⚠️ SAN mismatch detected"
		}

	default: // error
		errMsg, _ := payload["error"].(string)
		text = fmt.Sprintf("❌ *%s — check error*\n", name) +
			fmt.Sprintf("Domain: `%s`\n", domain)
		if errMsg != "" {
			text += fmt.Sprintf("Error: `%s`", errMsg)
		}
	}

	text += fmt.Sprintf("\n🕐 `%s WIB`", nowWIB)

	return json.Marshal(map[string]string{
		"text":                 text,
		"parse_mode":           "Markdown",
		"disable_notification": "false",
	})
}

func formatDiscordNotification(payload map[string]interface{}) ([]byte, error) {
	domain, _ := payload["domain"].(string)
	displayName, _ := payload["display_name"].(string)
	status, _ := payload["status"].(string)
	days, _ := payload["days_remaining"].(int)
	issuer, _ := payload["issuer"].(string)
	expiresAt, _ := payload["expires_at"].(string)
	grade, _ := payload["cipher_grade"].(string)
	tlsVer, _ := payload["tls_version"].(string)
	sanMismatch, _ := payload["san_mismatch"].(bool)
	errMsg, _ := payload["error"].(string)

	name := domain
	if displayName != "" {
		name = displayName
	}

	var color int
	var title, desc string
	switch status {
	case "valid":
		color = 0x10B981
		title = fmt.Sprintf("🟢 %s certificate is valid", name)
		desc = fmt.Sprintf("**%s** — certificate is valid", domain)
	case "expiring_soon":
		color = 0xF59E0B
		title = fmt.Sprintf("🟡 %s certificate expiring soon", name)
		desc = fmt.Sprintf("**%s** — expires in **%d days**", domain, days)
	case "expired":
		color = 0xEF4444
		title = fmt.Sprintf("🔴 %s certificate EXPIRED", name)
		desc = fmt.Sprintf("**%s** — expired **%d days ago**", domain, days)
	default:
		color = 0x94A3B8
		title = fmt.Sprintf("❌ %s — check error", name)
		desc = fmt.Sprintf("**%s** — check failed", domain)
	}

	fields := []map[string]interface{}{
		{"name": "Domain", "value": domain, "inline": true},
	}
	if expiresAt != "" {
		fields = append(fields, map[string]interface{}{"name": "Expires", "value": expiresAt, "inline": true})
	}
	if days > 0 || (status == "expired" && days <= 0) {
		label := "Days Remaining"
		if status == "expired" {
			label = "Days Overdue"
		}
		fields = append(fields, map[string]interface{}{"name": label, "value": fmt.Sprintf("%d", days), "inline": true})
	}
	if issuer != "" {
		fields = append(fields, map[string]interface{}{"name": "Issuer", "value": issuer, "inline": false})
	}
	if grade != "" {
		fields = append(fields, map[string]interface{}{"name": "Grade", "value": grade, "inline": true})
	}
	if tlsVer != "" {
		fields = append(fields, map[string]interface{}{"name": "TLS", "value": tlsVer, "inline": true})
	}
	if sanMismatch {
		fields = append(fields, map[string]interface{}{"name": "⚠️ SAN Mismatch", "value": "Detected", "inline": false})
	}
	if errMsg != "" {
		fields = append(fields, map[string]interface{}{"name": "Error", "value": errMsg, "inline": false})
	}

	embed := map[string]interface{}{
		"title":       title,
		"description": desc,
		"color":       color,
		"fields":      fields,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"footer":      map[string]interface{}{"text": "Anjungan SSL Monitor"},
	}

	return json.Marshal(map[string]interface{}{
		"embeds": []map[string]interface{}{embed},
	})
}

func formatSlackNotification(payload map[string]interface{}) ([]byte, error) {
	domain, _ := payload["domain"].(string)
	displayName, _ := payload["display_name"].(string)
	status, _ := payload["status"].(string)
	days, _ := payload["days_remaining"].(int)
	issuer, _ := payload["issuer"].(string)
	expiresAt, _ := payload["expires_at"].(string)
	grade, _ := payload["cipher_grade"].(string)
	sanMismatch, _ := payload["san_mismatch"].(bool)
	errMsg, _ := payload["error"].(string)

	name := domain
	if displayName != "" {
		name = displayName
	}

	var emoji string
	var statusLine string
	switch status {
	case "valid":
		emoji = ":white_check_mark:"
		statusLine = "valid"
	case "expiring_soon":
		emoji = ":warning:"
		statusLine = "expiring soon"
	case "expired":
		emoji = ":red_circle:"
		statusLine = "EXPIRED"
	default:
		emoji = ":x:"
		statusLine = "error"
	}

	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("%s *%s* — %s", emoji, name, strings.ToUpper(statusLine)),
			},
		},
	}

	// Fields block
	fieldText := fmt.Sprintf("*Domain:* %s\n", domain)
	if expiresAt != "" {
		fieldText += fmt.Sprintf("*Expires:* %s\n", expiresAt)
	}
	if days > 0 || status == "expired" {
		label := "Days Remaining"
		if status == "expired" {
			label = "Days Overdue"
		}
		fieldText += fmt.Sprintf("*%s:* %d\n", label, days)
	}
	if issuer != "" {
		fieldText += fmt.Sprintf("*Issuer:* %s\n", issuer)
	}
	if grade != "" {
		fieldText += fmt.Sprintf("*Grade:* %s\n", grade)
	}
	if sanMismatch {
		fieldText += ":warning: SAN mismatch detected\n"
	}
	if errMsg != "" {
		fieldText += fmt.Sprintf("*Error:* %s", errMsg)
	}

	blocks = append(blocks, map[string]interface{}{
		"type": "section",
		"text": map[string]interface{}{
			"type": "mrkdwn",
			"text": fieldText,
		},
	})

	return json.Marshal(map[string]interface{}{
		"text":   fmt.Sprintf("%s SSL Alert: %s", emoji, domain),
		"blocks": blocks,
	})
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

// ─── Notification Targets CRUD ────────────────────────────────────────────────

func (h *Handler) ListNotificationTargets(w http.ResponseWriter, r *http.Request) {
	targets, err := h.repo.ListSSLNotificationTargets(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list notification targets")
		return
	}
	if targets == nil {
		targets = []*model.SSLNotificationTarget{}
	}
	common.JSON(w, http.StatusOK, targets)
}

func (h *Handler) GetNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	target, err := h.repo.GetSSLNotificationTarget(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "notification target not found")
		return
	}
	common.JSON(w, http.StatusOK, target)
}

func (h *Handler) CreateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	var req model.SSLNotificationTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := req.Validate(); msg != "" {
		common.Error(w, http.StatusBadRequest, msg)
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	claims := auth.GetClaims(r.Context())
	createdBy := ""
	if claims != nil {
		createdBy = claims.UserID
	}

	now := time.Now()
	target := &model.SSLNotificationTarget{
		ID:            uuid.New().String(),
		Name:          req.Name,
		URL:           req.URL,
		Platform:      req.Platform,
		WebhookSecret: req.WebhookSecret,
		Enabled:       enabled,
		CreatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := h.repo.CreateSSLNotificationTarget(r.Context(), target); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create notification target")
		return
	}

	audit.Log(h.repo, claims.UserID, "", r.RemoteAddr,
		"sslmonitor.notification-target.create", "ssl_notification_target", target.ID,
		fmt.Sprintf("Created notification target %s (%s)", target.Name, target.Platform))

	common.JSON(w, http.StatusCreated, target)
}

func (h *Handler) UpdateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := h.repo.GetSSLNotificationTarget(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "notification target not found")
		return
	}

	var req model.SSLNotificationTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := req.Validate(); msg != "" {
		common.Error(w, http.StatusBadRequest, msg)
		return
	}

	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	existing.Name = req.Name
	existing.URL = req.URL
	existing.Platform = req.Platform
	existing.WebhookSecret = req.WebhookSecret
	existing.Enabled = enabled
	existing.UpdatedAt = time.Now()

	if err := h.repo.UpdateSSLNotificationTarget(r.Context(), existing); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to update notification target")
		return
	}

	claims := auth.GetClaims(r.Context())
	audit.Log(h.repo, claims.UserID, "", r.RemoteAddr,
		"sslmonitor.notification-target.update", "ssl_notification_target", id,
		fmt.Sprintf("Updated notification target %s", existing.Name))

	common.JSON(w, http.StatusOK, existing)
}

func (h *Handler) DeleteNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify it exists
	_, err := h.repo.GetSSLNotificationTarget(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "notification target not found")
		return
	}

	if err := h.repo.DeleteSSLNotificationTarget(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete notification target")
		return
	}

	claims := auth.GetClaims(r.Context())
	audit.Log(h.repo, claims.UserID, "", r.RemoteAddr,
		"sslmonitor.notification-target.delete", "ssl_notification_target", id,
		"Deleted notification target")

	common.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) TestNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	target, err := h.repo.GetSSLNotificationTarget(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "notification target not found")
		return
	}

	// Convert to shared NotificationTarget for dispatch
	sharedTarget := &model.NotificationTarget{
		ID:            target.ID,
		Name:          target.Name,
		URL:           target.URL,
		Platform:      target.Platform,
		WebhookSecret: target.WebhookSecret,
		Enabled:       target.Enabled,
	}

	// Build a test payload mimicking an SSL expiry alert
	testPayload := map[string]interface{}{
		"event_type":      "ssl.expiry.test",
		"domain":          "example.com",
		"port":            443,
		"display_name":    "Test Notification",
		"status":          "expiring_soon",
		"days_remaining":  14,
		"issuer":          "R3 (Let's Encrypt)",
		"subject":         "CN=example.com",
		"expires_at":      time.Now().AddDate(0, 0, 14).Format("2006-01-02"),
		"cipher_grade":    "A",
		"tls_version":     "TLS 1.3",
		"san_mismatch":    false,
		"ocsp_status":     "good",
		"previous_status": "valid",
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}

	statusCode, respBody, err := dispatchToTarget(sharedTarget, testPayload)
	if err != nil {
		log.Printf("[sslmonitor] test notification target %s failed: %v", target.Name, err)
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"status_code": 0,
		})
		return
	}

	log.Printf("[sslmonitor] test notification target %s delivered: %d", target.Name, statusCode)
	common.JSON(w, http.StatusOK, map[string]interface{}{
		"success":     statusCode >= 200 && statusCode < 300,
		"status_code": statusCode,
		"response":    respBody,
	})
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
