package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

// ─── Webhook CRUD Routes ────────────────────────────────────────────────────

func (h *Handler) webhookRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListWebhooks)
	r.Post("/", h.requireAdmin(h.CreateWebhook))
	r.Get("/{id}", h.GetWebhook)
	r.Put("/{id}", h.requireAdmin(h.UpdateWebhook))
	r.Delete("/{id}", h.requireAdmin(h.DeleteWebhook))
	r.Post("/{id}/test", h.requireAdmin(h.TestWebhook))
	r.Get("/events", h.ListWebhookEvents)
	r.Post("/receiver", h.WebhookReceiver) // Zot events receiver — no auth
	return r
}

// ─── Webhook CRUD ───────────────────────────────────────────────────────────

func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	hooks, err := h.repo.ListRegistryWebhooks(r.Context())
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}
	if hooks == nil {
		hooks = []*model.RegistryWebhook{}
	}
	common.JSON(w, http.StatusOK, hooks)
}

func (h *Handler) GetWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	hook, err := h.repo.GetRegistryWebhook(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "webhook not found")
		return
	}
	common.JSON(w, http.StatusOK, hook)
}

func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req model.RegistryWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if msg := req.Validate(); msg != "" {
		common.Error(w, http.StatusBadRequest, msg)
		return
	}

	eventsJSON, _ := json.Marshal(req.Events)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now()
	hook := &model.RegistryWebhook{
		ID:        uuid.New().String(),
		Name:      req.Name,
		URL:       req.URL,
		Platform:  req.Platform,
		Events:    string(eventsJSON),
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.repo.CreateRegistryWebhook(r.Context(), hook); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to create webhook")
		return
	}

	h.logAudit(r, "registry.webhook.create", "registry_webhook", hook.ID,
		fmt.Sprintf("Created webhook %s (%s)", hook.Name, hook.Platform))

	common.JSON(w, http.StatusCreated, hook)
}

func (h *Handler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	hook, err := h.repo.GetRegistryWebhook(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "webhook not found")
		return
	}

	var req model.RegistryWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	changes := []string{}

	if req.URL != "" && req.URL != hook.URL {
		hook.URL = req.URL
		changes = append(changes, "url")
	}
	if req.Platform != "" && req.Platform != hook.Platform {
		hook.Platform = req.Platform
		changes = append(changes, "platform")
	}
	if req.Name != "" && req.Name != hook.Name {
		hook.Name = req.Name
		changes = append(changes, "name")
	}
	if len(req.Events) > 0 {
		eventsJSON, _ := json.Marshal(req.Events)
		if string(eventsJSON) != hook.Events {
			hook.Events = string(eventsJSON)
			changes = append(changes, "events")
		}
	}
	if req.Enabled != nil && *req.Enabled != hook.Enabled {
		hook.Enabled = *req.Enabled
		changes = append(changes, "enabled")
	}

	if len(changes) > 0 {
		if err := h.repo.UpdateRegistryWebhook(r.Context(), hook); err != nil {
			common.Error(w, http.StatusInternalServerError, "failed to update webhook")
			return
		}
		h.logAudit(r, "registry.webhook.update", "registry_webhook", hook.ID,
			fmt.Sprintf("Updated webhook %s: %s", hook.Name, strings.Join(changes, ", ")))
	}

	common.JSON(w, http.StatusOK, hook)
}

func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	hook, err := h.repo.GetRegistryWebhook(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "webhook not found")
		return
	}

	if err := h.repo.DeleteRegistryWebhook(r.Context(), id); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to delete webhook")
		return
	}

	h.logAudit(r, "registry.webhook.delete", "registry_webhook", id,
		fmt.Sprintf("Deleted webhook %s", hook.Name))

	common.JSON(w, http.StatusOK, map[string]string{"message": "webhook deleted"})
}

// ─── Webhook Test ───────────────────────────────────────────────────────────

func (h *Handler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	hook, err := h.repo.GetRegistryWebhook(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "webhook not found")
		return
	}

	claims := auth.GetClaims(r.Context())
	actor := ""
	if claims != nil {
		actor = claims.Email
	}

	payload := map[string]interface{}{
		"event_type": "test",
		"repo":       "test/repo",
		"tag":        "v1.0.0",
		"digest":     "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		"actor":      actor,
		"description": fmt.Sprintf("🧪 Test notification from Anjungan by %s", actor),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"registry":   h.cfg.ExternalURL,
	}

	statusCode, respBody, err := dispatchWebhook(hook, payload)
	status := "delivered"
	if err != nil {
		status = "failed"
	}

	// Log the event
	payloadJSON, _ := json.Marshal(payload)
	payloadStr := string(payloadJSON)
	now := time.Now()
	event := &model.RegistryWebhookEvent{
		ID:          uuid.New().String(),
		WebhookID:   hook.ID,
		EventType:   "test",
		Repo:        "test/repo",
		Tag:         "v1.0.0",
		Digest:      "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Actor:       actor,
		Description: fmt.Sprintf("Test webhook %s", hook.Name),
		Payload:     &payloadStr,
		Status:      status,
		StatusCode:  statusCode,
		Response:    truncateStr(respBody, 500),
		CreatedAt:   now,
		DeliveredAt: &now,
	}
	h.repo.CreateRegistryWebhookEvent(r.Context(), event)

	result := map[string]interface{}{
		"status":      status,
		"status_code": statusCode,
		"response":    truncateStr(respBody, 500),
	}
	if err != nil {
		result["error"] = err.Error()
	}
	common.JSON(w, http.StatusOK, result)
}

// ─── Webhook Events ─────────────────────────────────────────────────────────

func (h *Handler) ListWebhookEvents(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	limit := 50
	offset := 0
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	events, err := h.repo.ListRegistryWebhookEvents(r.Context(), limit, offset)
	if err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to list events")
		return
	}
	if events == nil {
		events = []*model.RegistryWebhookEvent{}
	}

	total, _ := h.repo.CountRegistryWebhookEvents(r.Context())

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// ─── Webhook Receiver (for Zot push events) ─────────────────────────────────

func (h *Handler) WebhookReceiver(w http.ResponseWriter, r *http.Request) {
	// This endpoint is called by Zot when events happen
	// Zot webhook payload format: https://zotregistry.dev/docs/webhooks/
	body, err := io.ReadAll(r.Body)
	if err != nil {
		common.Error(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var zotEvent struct {
		Name      string `json:"name"`
		Digest    string `json:"digest"`
		EventType string `json:"eventType"`
		Tag       string `json:"tag"`
	}
	if err := json.Unmarshal(body, &zotEvent); err != nil {
		// Try parsing as an array of events
		var events []struct {
			Name      string `json:"name"`
			Digest    string `json:"digest"`
			EventType string `json:"eventType"`
			Tag       string `json:"tag"`
		}
		if err2 := json.Unmarshal(body, &events); err2 != nil || len(events) == 0 {
			common.Error(w, http.StatusBadRequest, "invalid webhook payload")
			return
		}
		// Process each event
		for _, ev := range events {
			h.processZotEvent(r.Context(), ev.Name, ev.Tag, ev.Digest, ev.EventType)
		}
		common.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	h.processZotEvent(r.Context(), zotEvent.Name, zotEvent.Tag, zotEvent.Digest, zotEvent.EventType)
	common.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) processZotEvent(ctx context.Context, name, tag, digest, eventType string) {
	if eventType == "" {
		eventType = "push"
	}

	// Log the event
	payload := map[string]interface{}{
		"event_type": eventType,
		"repo":       name,
		"tag":        tag,
		"digest":     digest,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"registry":   h.cfg.ExternalURL,
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadStr := string(payloadJSON)
	now := time.Now()

	event := &model.RegistryWebhookEvent{
		ID:          uuid.New().String(),
		EventType:   eventType,
		Repo:        name,
		Tag:         tag,
		Digest:      digest,
		Actor:       "zot",
		Description: fmt.Sprintf("%s %s:%s", eventType, name, tag),
		Payload:     &payloadStr,
		Status:      "pending",
		CreatedAt:   now,
	}
	h.repo.CreateRegistryWebhookEvent(ctx, event)

	// Dispatch to all enabled webhooks that subscribe to this event type
	hooks, err := h.repo.ListEnabledRegistryWebhooks(ctx)
	if err != nil {
		return
	}

	for _, hook := range hooks {
		if !hookSubscribesTo(hook, eventType) {
			continue
		}

		statusCode, respBody, err := dispatchWebhook(hook, payload)
		status := "delivered"
		if err != nil {
			status = "failed"
		}
		h.repo.UpdateRegistryWebhookEventDelivery(ctx, event.ID, status, statusCode, truncateStr(respBody, 500))
	}
}

// ─── Dispatch log for delete events (called from DeleteTag/DeleteManifest) ──

func (h *Handler) fireDeleteEvent(ctx context.Context, name, tag, digest, actor string) {
	eventType := "delete"
	description := fmt.Sprintf("Deleted %s:%s", name, tag)
	if tag == "" {
		description = fmt.Sprintf("Deleted %s@%s", name, digest)
	}

	payload := map[string]interface{}{
		"event_type":  eventType,
		"repo":        name,
		"tag":         tag,
		"digest":      digest,
		"actor":       actor,
		"description": description,
		"registry":    h.cfg.ExternalURL,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadStr := string(payloadJSON)
	now := time.Now()

	event := &model.RegistryWebhookEvent{
		ID:          uuid.New().String(),
		EventType:   eventType,
		Repo:        name,
		Tag:         tag,
		Digest:      digest,
		Actor:       actor,
		Description: description,
		Payload:     &payloadStr,
		Status:      "pending",
		CreatedAt:   now,
	}
	h.repo.CreateRegistryWebhookEvent(ctx, event)

	// Dispatch to all enabled webhooks
	hooks, err := h.repo.ListEnabledRegistryWebhooks(ctx)
	if err != nil {
		return
	}

	for _, hook := range hooks {
		if !hookSubscribesTo(hook, eventType) {
			continue
		}

		statusCode, respBody, err := dispatchWebhook(hook, payload)
		status := "delivered"
		if err != nil {
			status = "failed"
		}
		h.repo.UpdateRegistryWebhookEventDelivery(ctx, event.ID, status, statusCode, truncateStr(respBody, 500))
	}
}

// ─── Dispatch ───────────────────────────────────────────────────────────────

// dispatchWebhook sends the payload to the webhook URL with proper formatting
// based on the platform. Returns status code, response body, and error.
func dispatchWebhook(hook *model.RegistryWebhook, payload map[string]interface{}) (int, string, error) {
	var bodyBytes []byte
	var err error

	switch hook.Platform {
	case "telegram":
		bodyBytes, err = formatTelegramMessage(payload)
	case "discord":
		bodyBytes, err = formatDiscordMessage(payload)
	case "slack":
		bodyBytes, err = formatSlackMessage(payload)
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
	req.Header.Set("User-Agent", "anjungan-registry-webhook/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

// ─── Message Formatters ─────────────────────────────────────────────────────

func formatTelegramMessage(payload map[string]interface{}) ([]byte, error) {
	repo, _ := payload["repo"].(string)
	tag, _ := payload["tag"].(string)
	eventType, _ := payload["event_type"].(string)
	actor, _ := payload["actor"].(string)
	registry, _ := payload["registry"].(string)

	var emoji, title string
	switch eventType {
	case "push":
		emoji = "📦"
		title = "Image Pushed"
	case "delete":
		emoji = "🗑"
		title = "Image Deleted"
	case "test":
		emoji = "🧪"
		title = "Webhook Test"
	default:
		emoji = "🔔"
		title = eventType
	}

	var tagLine string
	if tag != "" {
		tagLine = fmt.Sprintf("Tag: <b>%s</b>\n", tag)
	}

	text := fmt.Sprintf(`%s <b>%s</b>
━━━━━━━━━━━━━━━━
Image: <code>%s</code>
%sActor: %s
Registry: <code>%s</code>`, emoji, title, repo, tagLine, actor, registry)

	msg := map[string]string{
		"text":                  text,
		"parse_mode":            "HTML",
		"disable_web_page_preview": "true",
	}
	return json.Marshal(msg)
}

func formatDiscordMessage(payload map[string]interface{}) ([]byte, error) {
	repo, _ := payload["repo"].(string)
	tag, _ := payload["tag"].(string)
	eventType, _ := payload["event_type"].(string)
	actor, _ := payload["actor"].(string)
	registry, _ := payload["registry"].(string)

	var color int
	var title string
	switch eventType {
	case "push":
		color = 0x10b981 // emerald/green
		title = "📦 Image Pushed"
	case "delete":
		color = 0xef4444 // red
		title = "🗑 Image Deleted"
	case "test":
		color = 0x8b5cf6 // purple
		title = "🧪 Webhook Test"
	default:
		color = 0x3b82f6 // blue
		title = eventType
	}

	var tagField map[string]interface{}
	if tag != "" {
		tagField = map[string]interface{}{
			"name":   "Tag",
			"value":  fmt.Sprintf("`%s`", tag),
			"inline": true,
		}
	} else {
		tagField = map[string]interface{}{
			"name":   "Digest",
			"value":  fmt.Sprintf("`%s`", payload["digest"]),
			"inline": true,
		}
	}

	embed := map[string]interface{}{
		"title":       title,
		"color":       color,
		"description": fmt.Sprintf("**Image:** `%s`", repo),
		"fields": []map[string]interface{}{
			tagField,
			{"name": "Actor", "value": actor, "inline": true},
			{"name": "Registry", "value": fmt.Sprintf("`%s`", registry), "inline": false},
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	msg := map[string]interface{}{
		"embeds": []map[string]interface{}{embed},
	}
	return json.Marshal(msg)
}

func formatSlackMessage(payload map[string]interface{}) ([]byte, error) {
	repo, _ := payload["repo"].(string)
	tag, _ := payload["tag"].(string)
	eventType, _ := payload["event_type"].(string)
	actor, _ := payload["actor"].(string)
	registry, _ := payload["registry"].(string)

	var emoji, title string
	switch eventType {
	case "push":
		emoji = ":package:"
		title = "Image Pushed"
	case "delete":
		emoji = ":wastebasket:"
		title = "Image Deleted"
	case "test":
		emoji = ":test_tube:"
		title = "Webhook Test"
	default:
		emoji = ":bell:"
		title = eventType
	}

	var tagText string
	if tag != "" {
		tagText = fmt.Sprintf("\n*Tag:* `%s`", tag)
	}

	text := fmt.Sprintf("%s *%s*%s\n*Actor:* %s\n*Registry:* `%s`",
		emoji, title, tagText, actor, registry)

	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Repository:* `%s`%s", repo, tagText),
			},
		},
		{
			"type": "section",
			"fields": []map[string]interface{}{
				{"type": "mrkdwn", "text": fmt.Sprintf("*Actor:*\n%s", actor)},
			},
		},
		{
			"type": "context",
			"elements": []map[string]interface{}{
				{"type": "mrkdwn", "text": fmt.Sprintf(":registry: %s", registry)},
			},
		},
	}

	msg := map[string]interface{}{
		"text":   text,
		"blocks": blocks,
	}
	return json.Marshal(msg)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func hookSubscribesTo(hook *model.RegistryWebhook, eventType string) bool {
	for _, e := range hook.EventList() {
		if e == eventType {
			return true
		}
	}
	return false
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ─── Inject delete events ──────────────────────────────────────────────────

// WithWebhookDispatch wraps a delete handler to fire webhook events.
// This is called from DeleteTag and DeleteManifest.
func (h *Handler) withWebhookDeleteEvent(name, tag, digest string, handler func() error) error {
	err := handler()
	if err == nil && tag != "" {
		// Fire delete event in background
		go func() {
			ctx := context.Background()
			// We'll get actor from a context-free version, or use "admin"
			h.fireDeleteEvent(ctx, name, tag, digest, "admin")
		}()
	}
	return err
}
