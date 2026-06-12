package uptime

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
	"github.com/edsuwarna/anjungan/internal/notification"
)

// ─── Scheduler ──────────────────────────────────────────────────────────────

// Scheduler runs periodic uptime checks and auto-purges old history.
type Scheduler struct {
	repo      *db.Repository
	handler   *Handler
	done      chan struct{}
	wg        sync.WaitGroup
	checkTick time.Duration
	purgeTick time.Duration
	purgeAge  time.Duration
}

// NewScheduler creates a new uptime monitor scheduler.
func NewScheduler(repo *db.Repository, handler *Handler) *Scheduler {
	return &Scheduler{
		repo:      repo,
		handler:   handler,
		done:      make(chan struct{}),
		checkTick: 1 * time.Minute,
		purgeTick: 24 * time.Hour,
		purgeAge:  30 * 24 * time.Hour,
	}
}

// Start begins the scheduler loop in background goroutines.
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(2)
	go s.runCheckLoop(ctx)
	go s.runPurgeLoop(ctx)
	log.Println("[uptime] scheduler started — checking every 1m, purge every 24h")
}

// Stop signals the scheduler to shut down.
func (s *Scheduler) Stop() {
	close(s.done)
}

// runCheckLoop ticks every minute and checks all due monitors.
func (s *Scheduler) runCheckLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.checkTick)
	defer ticker.Stop()

	// Run an initial check shortly after startup
	time.AfterFunc(10*time.Second, func() {
		s.checkDueMonitors(ctx)
	})

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.checkDueMonitors(ctx)
		}
	}
}

// checkDueMonitors loads all enabled monitors and checks those that are due.
func (s *Scheduler) checkDueMonitors(ctx context.Context) {
	monitors, err := s.repo.ListEnabledUptimeMonitors(ctx)
	if err != nil {
		log.Printf("[uptime] failed to list enabled monitors: %v", err)
		return
	}

	now := time.Now()
	for _, m := range monitors {
		// Skip if not due yet (only if it has been checked before)
		if m.LastCheckAt != nil && now.Before(m.LastCheckAt.Add(time.Duration(m.IntervalSeconds)*time.Second)) {
			continue
		}

		// Check if there's an active maintenance window — skip check if so
		activeMW, err := s.repo.ListActiveMaintenanceWindows(ctx, m.ID)
		if err != nil {
			log.Printf("[uptime] failed to check maintenance windows for %s: %v", m.ID, err)
			// Continue with the check on error
		} else if len(activeMW) > 0 {
			// Set status to maintenance and skip the check
			_ = s.repo.UpdateUptimeMonitorStatus(ctx, m.ID, "maintenance", nil, nil, fmt.Sprintf("Maintenance: %s", activeMW[0].Description))
			log.Printf("[uptime] skipping check for %s — active maintenance: %s", m.ID, activeMW[0].Description)
			continue
		}

		result := s.runSingleCheck(ctx, &m)
		if result == nil {
			continue
		}

		// Check for status change to fire notification
		statusChanged := m.Status != result.Status
		prevStatus := m.Status

		// Save check history
		history := &model.UptimeCheckHistory{
			ID:             uuid.New().String(),
			MonitorID:      m.ID,
			CheckedAt:      now,
			Status:         result.Status,
			StatusCode:     result.StatusCode,
			ResponseTimeMs: result.ResponseTimeMs,
			ErrorMessage:   result.ErrorMessage,
		}
		if err := s.repo.CreateUptimeCheckHistory(ctx, history); err != nil {
			log.Printf("[uptime] failed to save check history for %s: %v", m.ID, err)
		}

		// Update monitor status
		_ = s.repo.UpdateUptimeMonitorStatus(ctx, m.ID, result.Status, result.StatusCode, result.ResponseTimeMs, result.ErrorMessage)

		// Notify on status change
		if statusChanged && result.Status != "pending" {
			s.dispatchNotification(ctx, &m, prevStatus, result)
		}

		// Publish check result to SSE clients
		if s.handler != nil {
			s.handler.PublishCheck(result, &m)
		}
	}
}

// runSingleCheck performs a single uptime check and returns the result.
// Reuses the handler's runSingleCheck logic.
func (s *Scheduler) runSingleCheck(ctx context.Context, m *model.UptimeMonitor) *CheckResult {
	timeout := m.TimeoutSeconds
	if timeout < 1 {
		timeout = 30
	}

	switch m.CheckType {
	case "http":
		return CheckHTTP(m.URL, timeout, m.ExpectedStatusMin, m.ExpectedStatusMax, m.ExpectedBody)
	case "tcp":
		host, port := parseTCPAddress(m.URL)
		return CheckTCP(host, port, timeout)
	default:
		log.Printf("[uptime] unknown check type for %s: %s", m.ID, m.CheckType)
		return nil
	}
}

// runPurgeLoop ticks daily and removes old check history.
func (s *Scheduler) runPurgeLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.purgeTick)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			if err := s.repo.PurgeOldUptimeHistory(ctx, 30); err != nil {
				log.Printf("[uptime] failed to purge old history: %v", err)
			} else {
				log.Println("[uptime] old check history purged (retention: 30 days)")
			}
		}
	}
}

// ─── Notifier ──────────────────────────────────────────────────────────────

// dispatchNotification sends a status change notification to all configured targets.
func (s *Scheduler) dispatchNotification(ctx context.Context, m *model.UptimeMonitor, prevStatus string, result *CheckResult) {
	if len(m.NotificationTargetIDs) == 0 {
		return
	}

	// Load all enabled notification targets
	targets, err := s.repo.ListNotificationTargets(ctx, "")
	if err != nil {
		log.Printf("[uptime] failed to load notification targets for %s: %v", m.ID, err)
		return
	}

	// Build target lookup map
	targetMap := make(map[string]model.NotificationTarget, len(targets))
	for _, t := range targets {
		targetMap[t.ID] = t
	}

	for _, targetID := range m.NotificationTargetIDs {
		target, ok := targetMap[targetID]
		if !ok || !target.Enabled {
			continue
		}

		// Build payload based on platform
		payload := buildNotificationPayload(m, prevStatus, result, target.Platform)
		statusCode, respBody, err := notification.SendToTarget(&target, payload)
		if err != nil {
			log.Printf("[uptime] failed to send notification to %s (%s): %v", target.Name, target.URL, err)
		} else {
			log.Printf("[uptime] notification sent to %s — status %d", target.Name, statusCode)
			_ = respBody
		}
	}
}

// buildNotificationPayload creates a unified payload with all available info.
func buildNotificationPayload(m *model.UptimeMonitor, prevStatus string, result *CheckResult, platform string) map[string]interface{} {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	wib := time.Now().In(loc)

	// Build human-readable message
	var message string
	var emoji string
	switch result.Status {
	case "up":
		emoji = "✅"
		message = fmt.Sprintf("✅ Your service %s is back up! ✅", m.Name)
	case "down":
		emoji = "🔴"
		message = fmt.Sprintf("❌ Your service %s went down! ❌", m.Name)
	default:
		emoji = "⚪"
		message = fmt.Sprintf("⚠️ Your service %s status changed — %s", m.Name, result.Status)
	}

	// Build unified payload
	payload := map[string]interface{}{
		"event_type":      "uptime.status_change",
		"monitor_id":      m.ID,
		"monitor_name":    m.Name,
		"monitor_url":     m.URL,
		"check_type":      m.CheckType,
		"status":          result.Status,
		"previous_status": prevStatus,
		"emoji":           emoji,
		"message":         message,
		"status_code":     result.StatusCode,
		"response_time_ms": result.ResponseTimeMs,
		"error":           result.ErrorMessage,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
		"timestamp_wib":   wib.Format("2006-01-02 15:04:05"),
	}

	// Platform-specific envelope (payload fields stay the same for all)
	_ = platform
	return payload
}
