package audit

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/edsuwarna/anjungan/internal/common/model"
)

// Repository interface — only the method we need
type Repository interface {
	CreateAuditLog(ctx context.Context, e *model.AuditLogEntry) error
}

// Log creates an audit log entry asynchronously (best-effort).
// Extracts IP from the X-Forwarded-For header or RemoteAddr.
// Optional metadata can be passed as the last argument (json.RawMessage).
func Log(repo Repository, userID, userEmail, ip, action, entityType, entityID, description string, metadata ...json.RawMessage) {
	// Clean IP
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	meta := json.RawMessage("{}")
	if len(metadata) > 0 {
		meta = metadata[0]
	}

	entry := &model.AuditLogEntry{
		ID:          uuid.New().String(),
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		Description: description,
		UserID:      userID,
		UserEmail:   userEmail,
		IPAddress:   ip,
		Metadata:    meta,
		CreatedAt:   time.Now(),
	}

	// Best-effort async — never block the request
	go func() {
		_ = repo.CreateAuditLog(context.Background(), entry)
	}()
}

// Helper to extract remote IP from request context
func RemoteIP(remoteAddr string, forwardedFor string) string {
	if forwardedFor != "" {
		ips := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}
