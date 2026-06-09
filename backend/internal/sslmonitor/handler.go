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
	r.Post("/check-all", h.CheckAll)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/{id}/check", h.CheckNow)
	r.Get("/{id}/history", h.History)
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
	// Fetch the webhooks assigned to this monitor
	hooks, err := h.repo.ListRegistryWebhooksByIDs(ctx, m.WebhookIDs)
	if err != nil || len(hooks) == 0 {
		return
	}

	displayName := m.Domain
	if m.DisplayName != "" {
		displayName = m.DisplayName
	}

	// Build notification payload
	payload := map[string]interface{}{
		"event_type":     "ssl.expiry",
		"domain":         m.Domain,
		"port":           m.Port,
		"display_name":   displayName,
		"status":         r.Status,
		"days_remaining": r.DaysRemaining,
		"issuer":         r.Issuer,
		"subject":        r.Subject,
		"expires_at":     r.CertExpiresAt.Format(time.RFC3339),
		"cipher_grade":   r.CipherGrade,
		"previous_status": prevStatus,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"message":        fmt.Sprintf("🔒 SSL Certificate %s — %s (%d days remaining)", displayName, r.Status, r.DaysRemaining),
	}

	for _, hook := range hooks {
		statusCode, respBody, err := dispatchWebhook(hook, payload)
		if err != nil {
			log.Printf("[sslmonitor] webhook %s failed: %v", hook.Name, err)
			continue
		}
		log.Printf("[sslmonitor] webhook %s delivered: %d", hook.Name, statusCode)
		_ = statusCode
		_ = respBody
	}
}

// dispatchWebhook sends the payload to the webhook URL.
func dispatchWebhook(hook *model.RegistryWebhook, payload map[string]interface{}) (int, string, error) {
	var bodyBytes []byte
	var err error

	switch hook.Platform {
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

	req, err := http.NewRequest("POST", hook.URL, bytes.NewReader(bodyBytes))
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
	status, _ := payload["status"].(string)
	days, _ := payload["days_remaining"].(int)
	issuer, _ := payload["issuer"].(string)
	msg, _ := payload["message"].(string)

	var emoji string
	switch status {
	case "expiring_soon":
		emoji = "🟡"
	case "expired":
		emoji = "🔴"
	default:
		emoji = "🟢"
	}

	text := fmt.Sprintf(`%s *SSL Certificate Alert*

%s

*Domain:* %s
*Status:* %s
*Days Remaining:* %d
*Issuer:* %s`,
		emoji, msg, domain, status, days, issuer)

	return json.Marshal(map[string]string{
		"text":                text,
		"parse_mode":          "Markdown",
		"disable_notification": "false",
	})
}

func formatDiscordNotification(payload map[string]interface{}) ([]byte, error) {
	domain, _ := payload["domain"].(string)
	status, _ := payload["status"].(string)
	days, _ := payload["days_remaining"].(int)

	var color int
	switch status {
	case "expiring_soon":
		color = 0xF59E0B
	case "expired":
		color = 0xEF4444
	default:
		color = 0x10B981
	}

	embed := map[string]interface{}{
		"title":       "🔒 SSL Certificate Alert",
		"description": fmt.Sprintf("**%s** — %s", domain, status),
		"color":       color,
		"fields": []map[string]interface{}{
			{"name": "Domain", "value": domain, "inline": true},
			{"name": "Status", "value": status, "inline": true},
			{"name": "Days Remaining", "value": fmt.Sprintf("%d", days), "inline": true},
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return json.Marshal(map[string]interface{}{
		"embeds": []map[string]interface{}{embed},
	})
}

func formatSlackNotification(payload map[string]interface{}) ([]byte, error) {
	domain, _ := payload["domain"].(string)
	status, _ := payload["status"].(string)
	days, _ := payload["days_remaining"].(int)

	var emoji string
	switch status {
	case "expiring_soon":
		emoji = ":warning:"
	case "expired":
		emoji = ":red_circle:"
	default:
		emoji = ":white_check_mark:"
	}

	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("%s *SSL Certificate Alert*\n*Domain:* %s\n*Status:* %s\n*Days Remaining:* %d",
					emoji, domain, strings.ToUpper(status), days),
			},
		},
	}

	return json.Marshal(map[string]interface{}{
		"text":   fmt.Sprintf("%s SSL Alert: %s", emoji, domain),
		"blocks": blocks,
	})
}
