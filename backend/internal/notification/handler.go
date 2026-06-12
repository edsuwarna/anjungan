package notification

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/edsuwarna/anjungan/internal/audit"
	"github.com/edsuwarna/anjungan/internal/auth"
	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

// Handler handles notification target CRUD and testing.
type Handler struct {
	repo *db.Repository
}

// NewHandler creates a new notification handler.
func NewHandler(repo *db.Repository) *Handler {
	return &Handler{repo: repo}
}

// ─── Routes ───────────────────────────────────────────────────────────────────

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/{id}/test", h.TestDelivery)
	return r
}

// ─── CRUD ─────────────────────────────────────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
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
		BotToken:      req.BotToken,
		ChatID:        req.ChatID,
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

	if claims != nil {
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"notification_target.created", "notification_target", target.ID,
			fmt.Sprintf("Created notification target %s (%s)", target.Name, target.Platform), nil)
	}

	common.JSON(w, http.StatusCreated, target)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	target, err := h.repo.GetNotificationTarget(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "notification target not found")
		return
	}
	common.JSON(w, http.StatusOK, target)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
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
	existing.BotToken = req.BotToken
	existing.ChatID = req.ChatID
	existing.Enabled = enabled
	existing.Scopes = req.Scopes
	existing.UpdatedAt = time.Now()

	if err := h.repo.UpdateNotificationTarget(r.Context(), existing); err != nil {
		common.Error(w, http.StatusInternalServerError, "failed to update notification target")
		return
	}

	if claims := auth.GetClaims(r.Context()); claims != nil {
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"notification_target.updated", "notification_target", id,
			fmt.Sprintf("Updated notification target %s", existing.Name), nil)
	}

	common.JSON(w, http.StatusOK, existing)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
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

	if claims := auth.GetClaims(r.Context()); claims != nil {
		audit.Log(h.repo, claims.UserID, claims.Email, r.RemoteAddr,
			"notification_target.deleted", "notification_target", id,
			"Deleted notification target", nil)
	}

	common.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── TestDelivery ────────────────────────────────────────────────────────────

func (h *Handler) TestDelivery(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	target, err := h.repo.GetNotificationTarget(r.Context(), id)
	if err != nil {
		common.Error(w, http.StatusNotFound, "notification target not found")
		return
	}

	// Check if target has SSL scope
	hasSSL := false
	for _, scope := range target.Scopes {
		if scope == "ssl" {
			hasSSL = true
			break
		}
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	var testPayload map[string]interface{}

	if hasSSL {
		testPayload = map[string]interface{}{
			"event_type":      "ssl.test",
			"monitor_id":      "test",
			"domain":          "example.com",
			"port":            443,
			"display_name":    "Test SSL Monitor",
			"status":          "valid",
			"previous_status": "valid",
			"days_remaining":  30,
			"issuer":          "C=US, O=Let's Encrypt, CN=R3",
			"subject":         "CN=example.com",
			"expires_at":      time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339),
			"cipher_grade":    "A",
			"chain_valid":     true,
			"ocsp_status":     "good",
			"san_mismatch":    false,
			"error":           "",
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
			"timestamp_wib":   time.Now().In(loc).Format("2006-01-02 15:04:05"),
			"message":         "🔍 SSL Certificate Test — example.com (valid, 30 days remaining)",
		}
	} else {
		testPayload = map[string]interface{}{
			"event_type":        "uptime.test",
			"monitor_id":        "test",
			"monitor_name":      "Test Monitor",
			"monitor_url":       "https://example.com",
			"check_type":        "http",
			"status":            "up",
			"previous_status":   "down",
			"status_code":       200,
			"response_time_ms":  42,
			"error":             "",
			"timestamp":         time.Now().UTC().Format(time.RFC3339),
			"timestamp_wib":     time.Now().In(loc).Format("2006-01-02 15:04:05"),
			"message":           "✅ Your service Test Monitor is back up! ✅",
		}
	}

	var statusCode int
	var respBody string
	var sendErr error

	if hasSSL {
		statusCode, respBody, sendErr = SendRawJSON(target, testPayload)
	} else {
		statusCode, respBody, sendErr = SendToTarget(target, testPayload)
	}

	if sendErr != nil {
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   sendErr.Error(),
		})
		return
	}

	success := statusCode >= 200 && statusCode < 300
	if !success {
		common.JSON(w, http.StatusOK, map[string]interface{}{
			"success":     false,
			"status_code": statusCode,
			"error":       fmt.Sprintf("Webhook returned HTTP %d: %s", statusCode, TruncateString(respBody, 200)),
			"response":    TruncateString(respBody, 1000),
		})
		return
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"status_code": statusCode,
		"response":    TruncateString(respBody, 1000),
	})
}
