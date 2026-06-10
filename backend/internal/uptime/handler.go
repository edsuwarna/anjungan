package uptime

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
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

// ─── Handler ─────────────────────────────────────────────────────────────────

type Handler struct {
	repo      *db.Repository
	hub       *sseHub
	jwtSecret string
}

func NewHandler(repo *db.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) SetJWTSecret(secret string) {
	h.jwtSecret = secret
}

// ─── SSE Hub ─────────────────────────────────────────────────────────────────

type sseEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type sseHub struct {
	mu      sync.RWMutex
	clients map[string]chan sseEvent
}

func newSSEHub() *sseHub {
	return &sseHub{
		clients: make(map[string]chan sseEvent),
	}
}

// register adds a new client channel and returns its ID.
func (h *sseHub) register() (string, chan sseEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := uuid.New().String()
	ch := make(chan sseEvent, 64)
	h.clients[id] = ch
	return id, ch
}

// unregister removes a client channel by ID.
func (h *sseHub) unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ch, ok := h.clients[id]; ok {
		close(ch)
		delete(h.clients, id)
	}
}

// broadcast sends an event to all connected clients. Non-blocking per client.
func (h *sseHub) broadcast(event sseEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.clients {
		select {
		case ch <- event:
		default:
			// Client too slow, drop event to avoid blocking
		}
	}
}

func (h *Handler) InitSSE() {
	h.hub = newSSEHub()
}

// ─── Routes ──────────────────────────────────────────────────────────────────

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
	r.Post("/{id}/pause", h.Pause)
	r.Post("/{id}/resume", h.Resume)
	r.Get("/{id}/history", h.History)
	r.Get("/{id}/trend", h.Trend)
	r.Post("/{id}/test-notification", h.TestNotification)
	r.Get("/{id}/maintenance", h.ListMaintenanceWindows)
	r.Post("/{id}/maintenance", h.CreateMaintenanceWindow)
	r.Delete("/{id}/maintenance/{mwId}", h.DeleteMaintenanceWindow)
	return r
}

// ─── NotificationTargetRoutes ────────────────────────────────────────────────

func (h *Handler) NotificationTargetRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListNotificationTargets)
	r.Post("/", h.CreateNotificationTarget)
	r.Get("/{id}", h.GetNotificationTarget)
	r.Put("/{id}", h.UpdateNotificationTarget)
	r.Delete("/{id}", h.DeleteNotificationTarget)
	r.Post("/{id}/test", h.TestNotificationTargetDelivery)
	return r
}

// ─── Request / Response types ────────────────────────────────────────────────

type createUptimeMonitorRequest struct {
	Name                  string   `json:"name"`
	URL                   string   `json:"url"`
	CheckType             string   `json:"check_type"`
	IntervalSeconds       *int     `json:"interval_seconds,omitempty"`
	TimeoutSeconds        *int     `json:"timeout_seconds,omitempty"`
	ExpectedStatusMin     *int     `json:"expected_status_min,omitempty"`
	ExpectedStatusMax     *int     `json:"expected_status_max,omitempty"`
	ExpectedBody          string   `json:"expected_body,omitempty"`
	Enabled               *bool    `json:"enabled,omitempty"`
	NotificationTargetIDs []string `json:"notification_target_ids,omitempty"`
}

type updateUptimeMonitorRequest struct {
	Name                  *string  `json:"name,omitempty"`
	URL                   *string  `json:"url,omitempty"`
	CheckType             *string  `json:"check_type,omitempty"`
	IntervalSeconds       *int     `json:"interval_seconds,omitempty"`
	TimeoutSeconds        *int     `json:"timeout_seconds,omitempty"`
	ExpectedStatusMin     *int     `json:"expected_status_min,omitempty"`
	ExpectedStatusMax     *int     `json:"expected_status_max,omitempty"`
	ExpectedBody          *string  `json:"expected_body,omitempty"`
	Enabled               *bool    `json:"enabled,omitempty"`
	NotificationTargetIDs []string `json:"notification_target_ids,omitempty"`
}

// ─── List ────────────────────────────────────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")
	sort := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")

	monitors, total, err := h.repo.ListUptimeMonitors(r.Context(), page, limit, status, search, sort, order)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list monitors")
		return
	}

	totalPages := (total + limit - 1) / limit
	if total == 0 {
		totalPages = 0
	}

	common.JSONWithMeta(w, http.StatusOK, monitors, &common.Meta{
		Page:       page,
		PerPage:    limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

// ─── Create ──────────────────────────────────────────────────────────────────

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input createUptimeMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Name == "" {
		common.Error(w, http.StatusBadRequest, "name is required")
		return
	}
	if input.URL == "" {
		common.Error(w, http.StatusBadRequest, "url is required")
		return
	}

	checkType := input.CheckType
	if checkType == "" {
		checkType = "http"
	}

	timeoutSec := 30
	if input.TimeoutSeconds != nil && *input.TimeoutSeconds > 0 {
		timeoutSec = *input.TimeoutSeconds
	}

	expectedMin := 200
	if input.ExpectedStatusMin != nil {
		expectedMin = *input.ExpectedStatusMin
	}

	expectedMax := 299
	if input.ExpectedStatusMax != nil {
		expectedMax = *input.ExpectedStatusMax
	}

	intervalSec := 300
	if input.IntervalSeconds != nil && *input.IntervalSeconds > 0 {
		intervalSec = *input.IntervalSeconds
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	userID := ""
	claims := auth.GetClaims(r.Context())
	if claims != nil {
		userID = claims.UserID
	}

	now := time.Now()
	monitor := &model.UptimeMonitor{
		ID:                    uuid.New().String(),
		Name:                  input.Name,
		URL:                   input.URL,
		CheckType:             checkType,
		IntervalSeconds:       intervalSec,
		TimeoutSeconds:        timeoutSec,
		ExpectedStatusMin:     expectedMin,
		ExpectedStatusMax:     expectedMax,
		ExpectedBody:          input.ExpectedBody,
		Enabled:               enabled,
		NotificationTargetIDs: input.NotificationTargetIDs,
		Status:                "pending",
		LastStatus:            "",
		CreatedBy:             userID,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := h.repo.CreateUptimeMonitor(r.Context(), monitor); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create monitor")
		return
	}

	// Audit log
	if claims != nil {
		meta, _ := json.Marshal(map[string]interface{}{
			"name":       monitor.Name,
			"url":        monitor.URL,
			"check_type": monitor.CheckType,
		})
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"uptime_monitor.created", "uptime_monitor", monitor.ID,
			fmt.Sprintf("Created uptime monitor %s", monitor.Name),
			json.RawMessage(meta))
	}

	// Run initial check in background
	go h.runSingleCheck(context.Background(), monitor)

	common.JSON(w, http.StatusCreated, monitor)
}

// ─── Get ─────────────────────────────────────────────────────────────────────

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	monitor, err := h.repo.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	// Compute uptime stats
	stats, _ := h.repo.GetUptimeStats(r.Context(), id)

	// Merge monitor data with computed stats
	result := map[string]interface{}{
		"id":                      monitor.ID,
		"name":                    monitor.Name,
		"url":                     monitor.URL,
		"check_type":              monitor.CheckType,
		"interval_seconds":        monitor.IntervalSeconds,
		"timeout_seconds":         monitor.TimeoutSeconds,
		"expected_status_min":     monitor.ExpectedStatusMin,
		"expected_status_max":     monitor.ExpectedStatusMax,
		"expected_body":           monitor.ExpectedBody,
		"enabled":                 monitor.Enabled,
		"notification_target_ids": monitor.NotificationTargetIDs,
		"status":                  monitor.Status,
		"last_status":             monitor.LastStatus,
		"last_status_code":        monitor.LastStatusCode,
		"last_response_time_ms":   monitor.LastResponseTimeMs,
		"last_error":              monitor.LastError,
		"last_check_at":           monitor.LastCheckAt,
		"created_by":              monitor.CreatedBy,
		"created_at":              monitor.CreatedAt,
		"updated_at":              monitor.UpdatedAt,
	}
	if stats != nil {
		result["uptime_24h"] = stats.Uptime24h
		result["uptime_3d"] = stats.Uptime3d
		result["uptime_7d"] = stats.Uptime7d
		result["uptime_30d"] = stats.Uptime30d
		result["total_checks"] = stats.TotalChecks
		result["up_checks"] = stats.UpChecks
		result["down_checks"] = stats.DownChecks
	}

	common.JSON(w, http.StatusOK, result)
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	monitor, err := h.repo.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	var input updateUptimeMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Name != nil {
		monitor.Name = *input.Name
	}
	if input.URL != nil {
		monitor.URL = *input.URL
	}
	if input.CheckType != nil {
		monitor.CheckType = *input.CheckType
	}
	if input.IntervalSeconds != nil {
		monitor.IntervalSeconds = *input.IntervalSeconds
	}
	if input.TimeoutSeconds != nil {
		monitor.TimeoutSeconds = *input.TimeoutSeconds
	}
	if input.ExpectedStatusMin != nil {
		monitor.ExpectedStatusMin = *input.ExpectedStatusMin
	}
	if input.ExpectedStatusMax != nil {
		monitor.ExpectedStatusMax = *input.ExpectedStatusMax
	}
	if input.ExpectedBody != nil {
		monitor.ExpectedBody = *input.ExpectedBody
	}
	if input.Enabled != nil {
		monitor.Enabled = *input.Enabled
	}
	if input.NotificationTargetIDs != nil {
		monitor.NotificationTargetIDs = input.NotificationTargetIDs
	}
	monitor.UpdatedAt = time.Now()

	if err := h.repo.UpdateUptimeMonitor(r.Context(), monitor); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to update monitor")
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"uptime_monitor.updated", "uptime_monitor", monitor.ID,
			fmt.Sprintf("Updated uptime monitor %s", monitor.Name), nil)
	}

	common.JSON(w, http.StatusOK, monitor)
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	monitor, err := h.repo.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	if err := h.repo.DeleteUptimeMonitor(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete monitor")
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"uptime_monitor.deleted", "uptime_monitor", id,
			fmt.Sprintf("Deleted uptime monitor %s", monitor.Name), nil)
	}

	common.JSON(w, http.StatusOK, map[string]string{"message": "monitor deleted"})
}

// ─── Summary ─────────────────────────────────────────────────────────────────

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.repo.GetUptimeSummary(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get summary")
		return
	}
	common.JSON(w, http.StatusOK, summary)
}

// ─── SSE Events ──────────────────────────────────────────────────────────────

func (h *Handler) SSEEvents(w http.ResponseWriter, r *http.Request) {
	// Allow auth via query param for EventSource (can't set custom headers)
	tokenStr := r.URL.Query().Get("token")
	if tokenStr != "" {
		token, err := jwt.ParseWithClaims(tokenStr, &auth.Claims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(h.jwtSecret), nil
		})
		if err == nil {
			if claims, ok := token.Claims.(*auth.Claims); ok && token.Valid {
				ctx := context.WithValue(r.Context(), auth.ClaimsKey, claims)
				r = r.WithContext(ctx)
			}
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientID, ch := h.hub.register()
	defer h.hub.unregister(clientID)

	// Send initial connection event
	initial, _ := json.Marshal(sseEvent{Type: "connected", Data: map[string]string{"client_id": clientID}})
	fmt.Fprintf(w, "data: %s\n\n", initial)
	flusher.Flush()

	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			_, err := fmt.Fprintf(w, ": keepalive\n\n")
			if err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, err = fmt.Fprintf(w, "data: %s\n\n", data)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// PublishCheck broadcasts a check result to all connected SSE clients.
func (h *Handler) PublishCheck(result *CheckResult, monitor *model.UptimeMonitor) {
	if h.hub == nil {
		return
	}
	event := sseEvent{
		Type: "check_result",
		Data: map[string]interface{}{
			"monitor_id":       monitor.ID,
			"monitor_name":     monitor.Name,
			"url":              monitor.URL,
			"check_type":       monitor.CheckType,
			"status":           monitor.Status,
			"last_status":      monitor.LastStatus,
			"last_status_code": monitor.LastStatusCode,
			"last_error":       monitor.LastError,
			"last_check_at":    monitor.LastCheckAt,
			"last_response_time_ms": monitor.LastResponseTimeMs,
			"enabled":          monitor.Enabled,
			"result":           result,
		},
	}
	h.hub.broadcast(event)
}

// ─── CheckNow ────────────────────────────────────────────────────────────────

func (h *Handler) CheckNow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	monitor, err := h.repo.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	result := h.runSingleCheck(r.Context(), monitor)

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "check completed",
		"result":  result,
	})
}

// ─── CheckAll ────────────────────────────────────────────────────────────────

func (h *Handler) CheckAll(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.repo.ListEnabledUptimeMonitors(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list monitors")
		return
	}

	type checkResult struct {
		MonitorID string       `json:"monitor_id"`
		Result    *CheckResult `json:"result"`
		Error     string       `json:"error,omitempty"`
	}

	results := make([]checkResult, 0, len(monitors))
	for _, m := range monitors {
		mm := m // copy to avoid pointer issues in sequential checks
		result := h.runSingleCheck(r.Context(), &mm)
		results = append(results, checkResult{
			MonitorID: mm.ID,
			Result:    result,
		})
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("checked %d monitors", len(results)),
		"results": results,
	})
}

// ─── Pause ───────────────────────────────────────────────────────────────────

func (h *Handler) Pause(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify monitor exists
	_, err := h.repo.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	if err := h.repo.PauseUptimeMonitor(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to pause monitor")
		return
	}

	common.JSON(w, http.StatusOK, map[string]string{"message": "monitor paused"})
}

// ─── Resume ──────────────────────────────────────────────────────────────────

func (h *Handler) Resume(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify monitor exists
	monitor, err := h.repo.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	if err := h.repo.ResumeUptimeMonitor(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to resume monitor")
		return
	}

	// Immediately trigger a check after resume
	go h.runSingleCheck(context.Background(), monitor)

	common.JSON(w, http.StatusOK, map[string]string{"message": "monitor resumed"})
}

// ─── History ─────────────────────────────────────────────────────────────────

func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	history, err := h.repo.ListUptimeCheckHistory(r.Context(), id, limit, offset)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list history")
		return
	}

	common.JSON(w, http.StatusOK, history)
}

// ─── Trend ───────────────────────────────────────────────────────────────────

func (h *Handler) Trend(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	period := r.URL.Query().Get("period")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if period == "" && from == "" {
		period = "24h"
	}

	var entries []model.UptimeCheckHistory
	var err error

	if from != "" {
		entries, err = h.repo.GetUptimeTrendCustom(r.Context(), id, from, to)
	} else {
		entries, err = h.repo.GetUptimeTrend(r.Context(), id, period)
	}
	if err != nil {
		common.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []model.UptimeCheckHistory{}
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"period":  period,
	})
}

// ─── TestNotification ────────────────────────────────────────────────────────

func (h *Handler) TestNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	monitor, err := h.repo.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	if len(monitor.NotificationTargetIDs) == 0 {
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"message": "no notification targets configured for this monitor",
			"results": []interface{}{},
		})
		return
	}

	// Load all notification targets (no scope filter)
	targets, err := h.repo.ListNotificationTargets(r.Context(), "")
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to load notification targets")
		return
	}

	// Build target lookup
	targetMap := make(map[string]model.NotificationTarget, len(targets))
	for _, t := range targets {
		targetMap[t.ID] = t
	}

	// Build test payload
	testPayload := map[string]interface{}{
		"event_type":   "uptime.test",
		"monitor_name": monitor.Name,
		"monitor_url":  monitor.URL,
		"check_type":   monitor.CheckType,
		"status":       monitor.LastStatus,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"message":      fmt.Sprintf("🔍 Uptime Monitor Test — %s (%s)", monitor.Name, monitor.URL),
	}

	type targetResult struct {
		TargetID   string `json:"target_id"`
		TargetName string `json:"target_name"`
		Status     int    `json:"status"`
		Response   string `json:"response"`
		Error      string `json:"error,omitempty"`
	}

	results := make([]targetResult, 0, len(monitor.NotificationTargetIDs))
	for _, targetID := range monitor.NotificationTargetIDs {
		target, ok := targetMap[targetID]
		if !ok {
			results = append(results, targetResult{
				TargetID: targetID,
				Status:   0,
				Error:    "target not found",
			})
			continue
		}

		statusCode, respBody, err := sendToTarget(&target, testPayload)
		if err != nil {
			results = append(results, targetResult{
				TargetID:   target.ID,
				TargetName: target.Name,
				Status:     0,
				Error:      err.Error(),
			})
			continue
		}

		results = append(results, targetResult{
			TargetID:   target.ID,
			TargetName: target.Name,
			Status:     statusCode,
			Response:   truncateString(respBody, 500),
		})
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("sent test to %d target(s)", len(results)),
		"results": results,
	})
}

// ─── Notification Target CRUD ────────────────────────────────────────────────

func (h *Handler) ListNotificationTargets(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")

	targets, err := h.repo.ListNotificationTargets(r.Context(), scope)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list notification targets")
		return
	}
	if targets == nil {
		targets = []model.NotificationTarget{}
	}

	common.JSON(w, http.StatusOK, targets)
}

func (h *Handler) CreateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	var req model.NotificationTargetRequest
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

	userID := ""
	claims := auth.GetClaims(r.Context())
	if claims != nil {
		userID = claims.UserID
	}

	now := time.Now()
	target := &model.NotificationTarget{
		ID:            uuid.New().String(),
		Name:          req.Name,
		URL:           req.URL,
		Platform:      req.Platform,
		WebhookSecret: req.WebhookSecret,
		Enabled:       enabled,
		Scopes:        req.Scopes,
		CreatedBy:     userID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := h.repo.CreateNotificationTarget(r.Context(), target); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create notification target")
		return
	}

	// Audit log
	if claims != nil {
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"notification_target.created", "notification_target", target.ID,
			fmt.Sprintf("Created notification target %s (%s)", target.Name, target.Platform), nil)
	}

	common.JSON(w, http.StatusCreated, target)
}

func (h *Handler) GetNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	target, err := h.repo.GetNotificationTarget(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "notification target not found")
		return
	}
	common.JSON(w, http.StatusOK, target)
}

func (h *Handler) UpdateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	existing, err := h.repo.GetNotificationTarget(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "notification target not found")
		return
	}

	var req model.NotificationTargetRequest
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
	existing.Scopes = req.Scopes
	existing.UpdatedAt = time.Now()

	if err := h.repo.UpdateNotificationTarget(r.Context(), existing); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to update notification target")
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"notification_target.updated", "notification_target", id,
			fmt.Sprintf("Updated notification target %s", existing.Name), nil)
	}

	common.JSON(w, http.StatusOK, existing)
}

func (h *Handler) DeleteNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	_, err := h.repo.GetNotificationTarget(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "notification target not found")
		return
	}

	if err := h.repo.DeleteNotificationTarget(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete notification target")
		return
	}

	// Audit log
	if claims := auth.GetClaims(r.Context()); claims != nil {
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"notification_target.deleted", "notification_target", id,
			"Deleted notification target", nil)
	}

	common.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── TestNotificationTargetDelivery ──────────────────────────────────────────

func (h *Handler) TestNotificationTargetDelivery(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	target, err := h.repo.GetNotificationTarget(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "notification target not found")
		return
	}

	// Build a test payload mimicking an uptime alert
	testPayload := map[string]interface{}{
		"event_type":   "uptime.test",
		"monitor_name": "Test Monitor",
		"monitor_url":  "https://example.com",
		"check_type":   "http",
		"status":       "up",
		"status_code":  200,
		"response_time_ms": 42,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"message":      "🔍 Uptime Monitor Test — This is a test notification from your uptime monitoring system.",
	}

	statusCode, respBody, err := sendToTarget(target, testPayload)
	if err != nil {
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	success := statusCode >= 200 && statusCode < 300
	if !success {
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"success":     false,
			"status_code": statusCode,
			"error":       fmt.Sprintf("Webhook returned HTTP %d: %s", statusCode, truncateString(respBody, 200)),
			"response":    truncateString(respBody, 1000),
		})
		return
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"status_code": statusCode,
		"response":    truncateString(respBody, 1000),
	})
}

// ─── Maintenance Windows ────────────────────────────────────────────────

type createMaintenanceWindowRequest struct {
	Description string `json:"description"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
}

// ListMaintenanceWindows returns all maintenance windows for a monitor.
func (h *Handler) ListMaintenanceWindows(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify monitor exists
	_, err := h.repo.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	windows, err := h.repo.ListUptimeMaintenanceWindows(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list maintenance windows")
		return
	}

	common.JSON(w, http.StatusOK, windows)
}

// CreateMaintenanceWindow creates a new maintenance window for a monitor.
func (h *Handler) CreateMaintenanceWindow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify monitor exists
	_, err := h.repo.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	var input createMaintenanceWindowRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Description == "" {
		common.Error(w, http.StatusBadRequest, "description is required")
		return
	}
	if input.StartsAt == "" || input.EndsAt == "" {
		common.Error(w, http.StatusBadRequest, "starts_at and ends_at are required")
		return
	}

	startsAt, err := time.Parse(time.RFC3339, input.StartsAt)
	if err != nil {
		common.Error(w, http.StatusBadRequest, "invalid starts_at format (use RFC3339)")
		return
	}

	endsAt, err := time.Parse(time.RFC3339, input.EndsAt)
	if err != nil {
		common.Error(w, http.StatusBadRequest, "invalid ends_at format (use RFC3339)")
		return
	}

	if endsAt.Before(startsAt) || endsAt.Equal(startsAt) {
		common.Error(w, http.StatusBadRequest, "ends_at must be after starts_at")
		return
	}

	userID := ""
	if claims := auth.GetClaims(r.Context()); claims != nil {
		userID = claims.UserID
	}

	now := time.Now()
	mw := &model.UptimeMaintenanceWindow{
		ID:          uuid.New().String(),
		MonitorID:   id,
		Description: input.Description,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.repo.CreateUptimeMaintenance(r.Context(), mw); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create maintenance window")
		return
	}

	common.JSON(w, http.StatusCreated, mw)
}

// DeleteMaintenanceWindow deletes a maintenance window.
func (h *Handler) DeleteMaintenanceWindow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mwID := chi.URLParam(r, "mwId")

	// Verify monitor exists
	_, err := h.repo.GetUptimeMonitor(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "monitor not found")
		return
	}

	// Verify maintenance window exists
	mw, err := h.repo.GetUptimeMaintenance(r.Context(), mwID)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to get maintenance window")
		return
	}
	if mw == nil {
		common.Error(w, http.StatusNotFound, "maintenance window not found")
		return
	}

	if err := h.repo.DeleteUptimeMaintenance(r.Context(), mwID); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete maintenance window")
		return
	}

	common.JSON(w, http.StatusOK, map[string]string{"message": "maintenance window deleted"})
}

// ─── Internal: runSingleCheck ────────────────────────────────────────────────

// runSingleCheck executes a check for the given monitor, saves the result to
// history, and updates the monitor's status. It is safe to call from a goroutine.
func (h *Handler) runSingleCheck(ctx context.Context, monitor *model.UptimeMonitor) *CheckResult {
	var result *CheckResult

	switch monitor.CheckType {
	case "tcp":
		host, port := parseTCPAddress(monitor.URL)
		result = CheckTCP(host, port, monitor.TimeoutSeconds)
	default:
		// "http" or any unknown type defaults to HTTP check
		result = CheckHTTP(monitor.URL, monitor.TimeoutSeconds,
			monitor.ExpectedStatusMin, monitor.ExpectedStatusMax, monitor.ExpectedBody)
	}

	now := time.Now().UTC()

	// Save to check history
	history := &model.UptimeCheckHistory{
		ID:             uuid.New().String(),
		MonitorID:      monitor.ID,
		CheckedAt:      now,
		Status:         result.Status,
		StatusCode:     result.StatusCode,
		ResponseTimeMs: result.ResponseTimeMs,
		ErrorMessage:   result.ErrorMessage,
	}
	if err := h.repo.CreateUptimeCheckHistory(ctx, history); err != nil {
		log.Printf("[uptime] failed to save check history for %s: %v", monitor.ID, err)
	}

	// Update monitor status
	if err := h.repo.UpdateUptimeMonitorStatus(ctx, monitor.ID, result.Status,
		result.StatusCode, result.ResponseTimeMs, result.ErrorMessage); err != nil {
		log.Printf("[uptime] failed to update monitor status for %s: %v", monitor.ID, err)
	}

	return result
}

// ─── Internal: helpers ───────────────────────────────────────────────────────

// parseTCPAddress extracts host and port from a TCP URL like "tcp://host:port" or "host:port".
func parseTCPAddress(rawURL string) (string, int) {
	s := strings.TrimPrefix(rawURL, "tcp://")
	if idx := strings.LastIndex(s, ":"); idx != -1 {
		host := s[:idx]
		portStr := s[idx+1:]
		port, err := strconv.Atoi(portStr)
		if err == nil && port > 0 && port < 65536 {
			return host, port
		}
	}
	return s, 80 // default port
}

// sendToTarget sends a platform-formatted notification to the target URL.
func sendToTarget(target *model.NotificationTarget, payload map[string]interface{}) (int, string, error) {
	var bodyBytes []byte
	var err error

	switch target.Platform {
	case "discord":
		bodyBytes, err = formatDiscordUptimeNotification(payload)
	case "telegram":
		bodyBytes, err = formatTelegramUptimeNotification(payload)
	case "slack":
		bodyBytes, err = formatSlackUptimeNotification(payload)
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
	req.Header.Set("User-Agent", "anjungan-uptime-webhook/1.0")

	if target.WebhookSecret != "" {
		req.Header.Set("X-Webhook-Secret", target.WebhookSecret)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

// formatDiscordUptimeNotification formats payload as a Discord embed.
func formatDiscordUptimeNotification(payload map[string]interface{}) ([]byte, error) {
	monitorName, _ := payload["monitor_name"].(string)
	monitorURL, _ := payload["monitor_url"].(string)
	checkType, _ := payload["check_type"].(string)
	status, _ := payload["status"].(string)
	msg, _ := payload["message"].(string)

	var color int
	switch status {
	case "down":
		color = 0xEF4444
	case "up":
		color = 0x10B981
	default:
		color = 0x94A3B8
	}

	if msg == "" {
		msg = fmt.Sprintf("⚠️ Uptime Alert — %s", monitorName)
	}

	embed := map[string]interface{}{
		"title":       msg,
		"color":       color,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"fields": []map[string]interface{}{
			{"name": "Monitor", "value": monitorName, "inline": true},
			{"name": "URL", "value": monitorURL, "inline": true},
			{"name": "Type", "value": strings.ToUpper(checkType), "inline": true},
			{"name": "Status", "value": strings.ToUpper(status), "inline": true},
		},
	}

	return json.Marshal(map[string]interface{}{
		"embeds": []map[string]interface{}{embed},
	})
}

// formatTelegramUptimeNotification formats payload as a Telegram message.
func formatTelegramUptimeNotification(payload map[string]interface{}) ([]byte, error) {
	monitorName, _ := payload["monitor_name"].(string)
	monitorURL, _ := payload["monitor_url"].(string)
	checkType, _ := payload["check_type"].(string)
	status, _ := payload["status"].(string)
	msg, _ := payload["message"].(string)

	var emoji string
	switch status {
	case "down":
		emoji = "🔴"
	case "up":
		emoji = "🟢"
	default:
		emoji = "⚪"
	}

	if msg == "" {
		msg = fmt.Sprintf("⚠️ Uptime Alert — %s", monitorName)
	}

	text := fmt.Sprintf(`%s *Uptime Monitor Alert*

%s

*Monitor:* %s
*URL:* %s
*Type:* %s
*Status:* %s`, emoji, msg, monitorName, monitorURL, strings.ToUpper(checkType), strings.ToUpper(status))

	return json.Marshal(map[string]interface{}{
		"text":             text,
		"parse_mode":       "Markdown",
		"disable_web_page_preview": true,
	})
}

// formatSlackUptimeNotification formats payload as a Slack message.
func formatSlackUptimeNotification(payload map[string]interface{}) ([]byte, error) {
	monitorName, _ := payload["monitor_name"].(string)
	monitorURL, _ := payload["monitor_url"].(string)
	checkType, _ := payload["check_type"].(string)
	status, _ := payload["status"].(string)
	msg, _ := payload["message"].(string)

	var emoji string
	switch status {
	case "down":
		emoji = ":red_circle:"
	case "up":
		emoji = ":white_check_mark:"
	default:
		emoji = ":white_circle:"
	}

	if msg == "" {
		msg = fmt.Sprintf("Uptime Alert — %s", monitorName)
	}

	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("%s *%s*\n*Monitor:* %s\n*URL:* %s\n*Type:* %s\n*Status:* %s",
					emoji, msg, monitorName, monitorURL, strings.ToUpper(checkType), strings.ToUpper(status)),
			},
		},
	}

	return json.Marshal(map[string]interface{}{
		"text":   fmt.Sprintf("%s Uptime Alert: %s", emoji, monitorName),
		"blocks": blocks,
	})
}

// truncateString truncates a string to the given max length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
