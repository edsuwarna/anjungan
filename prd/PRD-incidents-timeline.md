# Anjungan — PRD: Incidents / Timeline

> **Version:** 1.0
> **Status:** 🟡 Planned — Not Started
> **Author:** Endang Suwarna
> **Last Updated:** June 11, 2026

---

## 1. Executive Summary

### Problem Statement

Services in Anjungan go down, but nobody knows **why** — was it a deployment? A config change? An SSL expiry? An uptime blip? Events are scattered across different features (deployments, health checks, uptime, SSL, backups) with **no unified timeline** to answer simple questions like:

- "Why was service X down at 3 AM?"
- "Did the deployment cause the health check failure, or was it pre-existing?"
- "What changed in the last hour across all my services?"

Engineers manually cross-reference logs, deployment history, uptime monitors, and SSL check results — slow, error-prone, and painful during incidents.

### What This Solves

| Problem | Solution |
|---------|----------|
| Events scattered across features (deployments, uptime, SSL, backups, health) | Unified timeline — every event in one chronological view |
| Can't answer "what caused this outage?" quickly | Auto-correlation groups related events by time window + service |
| No visibility into patterns (deployments always break SSL?) | Multi-filter timeline — filter by service, environment, event type, severity |
| Incident response is untracked — who resolved what, when? | Resolve flow — mark events as investigating/resolved/false alarm with notes |

### Target Audience

- **Endang** (platform engineer / SRE) — quickly answer "what happened?" when a service goes down
- **DevOps** — correlate deployments with health/uptime events, track incident response
- **Admins** — get a bird's-eye view of all service events in one place

### Goals

| Goal | Metric |
|------|--------|
| Timeline view showing all events (deployments, health, uptime, SSL, backup) | Single scrollable timeline |
| Auto-correlation of related events | Events from same service within 5 min window grouped together |
| Multi-filter timeline (service, env, type, severity, time range) | < 500ms query response with any filter combination |
| Severity coloring for quick triage | Red (critical), Yellow (warning), Blue (info) at a glance |
| Event ingestion < 100ms | Internal event bus emit → store in < 100ms |

### Non-Goals

- ❌ Not an incident management system (no PagerDuty replacement)
- ❌ Not an on-call scheduling tool
- ❌ Not a post-mortem generator
- ❌ Not a log aggregator (no log parsing or full-text search)
- ❌ Not a metrics / APM solution (no traces, no profiling)

---

## 2. Product Overview

### This Feature in the Context of Anjungan

```
Main Dashboard
│
├── Ops Menu (/incidents)
│   ├── Filter Bar
│   │   ├── Time Range: 1h │ 6h │ 24h │ 7d │ 30d
│   │   ├── Severity:    Critical │ Warning │ Info
│   │   ├── Feature:     Deployment │ Uptime │ SSL │ Backup │ Health
│   │   ├── Service:     [search input]
│   │   └── Status:      Open │ Investigating │ Resolved │ All
│   │
│   ├── Vertical Timeline
│   │   ├── Event Item: dot (severity) + icon (feature) + time + title + badges
│   │   └── Correlation bracket: "3 events in this window"
│   │
│   ├── Event Detail Panel (slide-out)
│   │   ├── Full event data + JSON payload (collapsible)
│   │   ├── Related events (same correlation_group)
│   │   └── Action buttons: Investigate │ Resolve │ False Alarm
│   │
│   └── Stats Dashboard
│       ├── Count by severity (pie/bar)
│       ├── Top affected services
│       ├── Events over time (bar chart)
│       └── MTTR for resolved incidents
│
├── Event Bus (internal Go channels)
│   ├── deployment_feature → emits deployment_success/failure
│   ├── uptime_feature   → emits uptime_down/up
│   ├── ssl_feature      → emits ssl_expiring/valid
│   ├── health_feature   → emits health_check_failed/passed
│   └── backup_feature   → emits backup_failed/success
│
└── Event Store (PostgreSQL)
    └── incidents table → JSONB payload, correlation_group
```

### Flow: Event → Timeline

```
Feature (Deployment, Uptime, etc.)
  │
  │  1. Event occurs (deploy success, health fail, etc.)
  ▼
Event Bus (Go channel)
  │
  │  2. Emit structured payload: {feature, event_type, severity, title, description,
  │     source_service, source_environment, payload}
  ▼
Event Store (incidents table)
  │
  │  3. INSERT into PostgreSQL with auto-generated UUID + timestamp
  │  4. Auto-correlation: if same service within 5 min, same correlation_group
  ▼
Timeline API (GET /api/v1/incidents)
  │
  │  5. Query with filters (time range, severity, feature, service, status)
  ▼
Frontend (SvelteKit)
  │
  │  6. Render vertical timeline with colored dots, icons, badges
  │  7. Click → slide-out detail panel
```

### Key Design Decisions

1. **Internal event bus (Go channels), not external message queue** — Simpler architecture, no Kafka/RabbitMQ infrastructure overhead. Direct emit-and-store pattern is < 100ms. If scale demands it later, swap channels for NATS/PubSub without changing the interface.

2. **Events stored in PostgreSQL (same database)** — Transactional consistency with the rest of Anjungan. No dual-write problem. JSONB payload allows flexible event shapes per feature without schema changes.

3. **Auto-correlation by time window** — Simple, deterministic, no ML needed. Events from same `source_service` within a 5-minute rolling window get the same `correlation_group` hash. Good enough for 90% of correlation needs.

4. **No separate event schema per feature** — Single `incidents` table with `feature` + `event_type` discriminator. Payload is JSONB — each feature stores whatever it needs. Query indexes on `feature`, `event_type`, `severity`, `status`, `created_at`.

5. **Retention 90 days, then archive to JSON files** — Old events don't need to be queryable. Archive to `data/incidents-archive/YYYY-MM.json` for compliance. Configurable retention via env var `INCIDENTS_RETENTION_DAYS` (default 90).

---

## 3. Feature Specifications

> **Priority Key:** P0 = Must have, P1 = Should have, P2 = Nice to have

### F1 — Event Bus (P0)

| | |
|---|---|
| **Backend** | Internal Go channel-based event bus. `EventBus` struct with `ch chan IncidentEvent`. Each feature (deployment, uptime, SSL, health, backup) registers an emitter via `bus.Emit()`. Standard event payload: `{feature, event_type, severity, title, description, source_service, source_environment, payload map[string]any}`. Consumer goroutine reads from channel, builds `Incident` struct, inserts to DB. **Error handling**: DB insert failure → log + retry once after 1s. Channel buffer: 1024 events (backpressure safe). |
| **Frontend** | N/A — backend infrastructure only. |
| **UX** | Transparent to users — events appear automatically on the timeline as features emit them. |

### F2 — Timeline View (P0)

| | |
|---|---|
| **Backend** | `GET /api/v1/incidents?range=24h&severity=&feature=&status=&service=&search=&limit=&offset=` — paginated timeline query. Returns `{success, data: [{id, feature, event_type, severity, title, description, source_service, source_environment, status, correlation_group, created_at, resolved_at, resolved_by}], pagination: {total, limit, offset}}`. Sorted by `created_at DESC`. |
| **Frontend** | Route `/incidents`. **Vertical timeline layout**: Left column — timeline line with colored dots (🔴 critical, 🟡 warning, 🔵 info). Right column — event cards. Each card shows: severity dot → feature icon (🚀 deploy, 📊 uptime, 🔒 SSL, 💾 backup, ❤️ health) → relative timestamp ("3m ago") → title (bold) → source feature badge → status badge. **Infinite scroll** — loads next page on scroll. **Correlation brackets**: visually group events with same `correlation_group` → bracket with "3 events" label. |
| **UX** | Smooth scroll. Newest at top. Loading skeleton on initial load. Empty state: "No incidents in this time range." Correlation bracket expands/collapses on click. |

### F3 — Filtering (P0)

| | |
|---|---|
| **Backend** | Query params: `range` (1h, 6h, 24h, 7d, 30d, custom ISO range), `severity` (critical, warning, info), `feature` (deployment, uptime, ssl, backup, health), `status` (open, investigating, resolved, false_alarm, all), `service` (source_service LIKE), `search` (title/description ILIKE). All params optional — default: last 24h, all severities, all features, all statuses. Combined with AND logic. Indexes on `created_at DESC`, `(feature, event_type)`, `(severity, status)`. |
| **Frontend** | **Filter bar** (sticky top below header): Time range pills (1h, 6h, 24h, 7d, 30d) — selected highlighted. Severity toggle chips (Critical, Warning, Info) — multi-select, colored. Feature icon chips — multi-select with icons. Status dropdown (Open, Investigating, Resolved, All). Service search input with autocomplete. **Active filter count** badge on filter bar. **Clear all** button. Filters persist in URL query params (shareable link). |
| **UX** | Filter changes trigger new API call (debounced 300ms for search). URL updates without page reload. Empty state if no results match filters. |

### F4 — Event Detail Panel (P1)

| | |
|---|---|
| **Backend** | `GET /api/v1/incidents/{id}` — returns full incident record with payload JSON. Also returns `related_events: [...]` — other incidents with same `correlation_group`. `GET /api/v1/incidents/{id}/related` — dedicated endpoint for related events list. |
| **Frontend** | **Slide-out panel** (right side, 480px wide, 100% on mobile). Tap/click outside or ESC to close. Sections: **Header**: severity badge + event_type + timestamp (absolute format: "12 Jun 2026, 03:15 AM"). **Source**: service name (link to service detail if exists) + environment badge. **Description**: full description text. **Payload**: collapsible JSON viewer (syntax-highlighted, copy button). **Related Events**: list of events in same correlation group — each row: dot + title + time, click to open. **Status Timeline**: if status changes exist, show mini-timeline of status transitions (open → investigating → resolved). |
| **UX** | Panel slides in with animation. Backdrop dims. Related events are clickable — loads that event's detail in same panel. JSON viewer is collapsed by default, expand on click. |

### F5 — Auto-Correlation (P1)

| | |
|---|---|
| **Backend** | On event insert, compute `correlation_group` hash: `SHA256( source_service + floor(created_at / 300s) )` — groups events from same service within the same 5-minute window. If `source_service` is null/empty, no correlation group assigned. The hash is deterministic — same service + same time window always produces same group. `GET /api/v1/incidents?group_id=<hash>` — fetch all events in a correlation group. |
| **Frontend** | In timeline, events with same `correlation_group` displayed under a **correlation bracket** — a vertical bracket spanning the time window with label "3 events" and count. Click bracket → expand to show all events in that group. Bracket is collapsed by default (shows only first + count). |
| **UX** | Collapsed bracket: shows first event + "↓ 2 more" badge. Expanded: shows all events indented under bracket. Bracket has subtle background color (light blue). |

### F6 — Resolve / Investigate Flow (P1)

| | |
|---|---|
| **Backend** | `POST /api/v1/incidents/{id}/resolve` — body: `{status: "resolved" | "investigating" | "false_alarm", note: "optional text", resolved_by: user_id}`. Updates `status`, `resolved_at`, `resolved_by`. Optional: add a resolution note to description. Events with `status = "resolved"` or `"false_alarm"` show green checkmark ✓ in timeline. `POST /api/v1/incidents/{id}/investigate` — shortcut to set `status = "investigating"`. |
| **Frontend** | Detail panel bottom: **Action buttons** — "Mark Investigating" (🟡 yellow button), "Resolve" (✅ green button), "False Alarm" (🚫 gray button). Click → confirmation dialog with optional note field. After action → status badge updates in panel + timeline item updates. Resolved events get green checkmark on timeline dot. |
| **UX** | One-click actions with optional note. Note text appended to description with timestamp. Audit: who resolved, when, note. Undo not needed — can re-open by changing status back to open. |

### F7 — Stats Dashboard (P2)

| | |
|---|---|
| **Backend** | `GET /api/v1/incidents/stats?range=7d` — returns: `{total, by_severity: [{severity, count}], by_feature: [{feature, count}], top_services: [{service, count}], daily_counts: [{date, count, critical, warning, info}], mttr_seconds: number (avg resolution time for resolved incidents)}`. Computed from aggregate SQL queries. |
| **Frontend** | **Stats cards** at top of incidents page (above timeline, collapsed by default? or in a separate tab/section). **Severity distribution**: bar chart (red/yellow/blue). **Top affected services**: horizontal bar chart (top 10 by event count). **Events over time**: bar chart — x=date, y=count, stacked bars per severity. **MTTR card**: "Avg resolution time: 12m 34s". **Cards layout**: 4-column grid on desktop, 2-column on tablet, 1-column on mobile. |
| **UX** | Stats update on filter change (scoped to filtered results). Loading skeletons. Hover tooltips on chart bars show exact values. Collapsible section — can hide to focus on timeline. |

---

## 4. API Design

### New Endpoints — Incidents Timeline

```
GET    /api/v1/incidents                            // Timeline list (?range=&severity=&feature=&status=&service=&search=&limit=&offset=)
GET    /api/v1/incidents/stats                      // Stats dashboard (?range=)
GET    /api/v1/incidents/{id}                       // Event detail
GET    /api/v1/incidents/{id}/related               // Related events (same correlation_group)
POST   /api/v1/incidents                            // (Internal) Create event via event bus
POST   /api/v1/incidents/{id}/investigate           // Mark as investigating
POST   /api/v1/incidents/{id}/resolve               // Mark as resolved / false_alarm
```

### Request/Response Examples

```json
// GET /api/v1/incidents?range=24h&severity=critical,error&feature=deployment&status=open&limit=20&offset=0
{
  "success": true,
  "data": [
    {
      "id": "0194f2a1-...",
      "feature": "deployment",
      "event_type": "deployment_failed",
      "severity": "critical",
      "title": "Deployment failed: api-gateway v2.4.1",
      "description": "Container failed to start — exit code 137 (OOMKilled)",
      "payload": {
        "service": "api-gateway",
        "version": "v2.4.1",
        "commit": "a1b2c3d",
        "environment": "production",
        "exit_code": 137,
        "error": "OOMKilled",
        "deployment_id": "dep-123"
      },
      "source_service": "api-gateway",
      "source_environment": "production",
      "status": "open",
      "correlation_group": "abc123def456",
      "resolved_by": null,
      "resolved_at": null,
      "created_at": "2026-06-11T03:15:00Z"
    }
  ],
  "pagination": {
    "total": 47,
    "limit": 20,
    "offset": 0
  }
}

// GET /api/v1/incidents/stats?range=7d
{
  "success": true,
  "data": {
    "total": 312,
    "by_severity": [
      { "severity": "critical", "count": 14 },
      { "severity": "warning",  "count": 89 },
      { "severity": "info",     "count": 209 }
    ],
    "by_feature": [
      { "feature": "deployment", "count": 87 },
      { "feature": "uptime",     "count": 65 },
      { "feature": "ssl",        "count": 23 },
      { "feature": "health",     "count": 54 },
      { "feature": "backup",     "count": 83 }
    ],
    "top_services": [
      { "service": "api-gateway",     "count": 42 },
      { "service": "user-service",    "count": 31 },
      { "service": "worker-queue",    "count": 19 }
    ],
    "daily_counts": [
      { "date": "2026-06-05", "total": 45, "critical": 2, "warning": 12, "info": 31 },
      { "date": "2026-06-06", "total": 38, "critical": 0, "warning": 10, "info": 28 },
      { "date": "2026-06-07", "total": 52, "critical": 5, "warning": 14, "info": 33 }
    ],
    "mttr_seconds": 754
  }
}

// POST /api/v1/incidents/{id}/resolve
// Request:
{
  "status": "resolved",
  "note": "Restarted container — OOM issue caused by memory leak in v2.4.1. Rolled back to v2.4.0.",
  "resolved_by": "uuid-user-1"
}
// Response:
{
  "success": true,
  "data": {
    "id": "0194f2a1-...",
    "status": "resolved",
    "resolved_at": "2026-06-11T03:45:00Z",
    "resolved_by": "uuid-user-1"
  }
}
```

### Internal Emit API (not exposed to HTTP)

```go
// Go interface used by event bus
type IncidentEvent struct {
    Feature           string         // 'deployment', 'uptime', 'ssl', 'backup', 'health'
    EventType         string         // 'deployment_success', 'health_check_failed', etc.
    Severity          string         // 'critical', 'warning', 'info'
    Title             string
    Description       string
    SourceService     string
    SourceEnvironment string
    Payload           map[string]any
}

// Usage:
bus.Emit(IncidentEvent{
    Feature:           "deployment",
    EventType:         "deployment_failed",
    Severity:          "critical",
    Title:             "Deployment failed: api-gateway v2.4.1",
    Description:       "Container failed to start — exit code 137",
    SourceService:     "api-gateway",
    SourceEnvironment: "production",
    Payload: map[string]any{
        "version": "v2.4.1",
        "exit_code": 137,
        "error": "OOMKilled",
    },
})
```

---

## 5. Database Schema

### New Table

```sql
-- 0000XX_create_incidents.up.sql
CREATE TABLE incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    feature VARCHAR(50) NOT NULL,                  -- 'deployment', 'uptime', 'ssl', 'backup', 'health'
    event_type VARCHAR(50) NOT NULL,               -- 'deployment_success', 'health_check_failed', 'uptime_down', 'ssl_expiring', 'backup_failed'
    severity VARCHAR(10) NOT NULL DEFAULT 'info',  -- 'critical', 'warning', 'info'
    title VARCHAR(200) NOT NULL,
    description TEXT,
    payload JSONB,                                  -- original event payload (flexible per feature)
    source_service VARCHAR(100),                    -- which service this relates to
    source_environment VARCHAR(50),
    status VARCHAR(20) DEFAULT 'open',             -- 'open', 'investigating', 'resolved', 'false_alarm'
    correlation_group VARCHAR(64),                  -- SHA256 hash of service + time window
    resolved_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Indexes for query performance
CREATE INDEX idx_incidents_time ON incidents(created_at DESC);
CREATE INDEX idx_incidents_feature ON incidents(feature, event_type);
CREATE INDEX idx_incidents_severity ON incidents(severity, status);
CREATE INDEX idx_incidents_service ON incidents(source_service);
CREATE INDEX idx_incidents_correlation ON incidents(correlation_group);
CREATE INDEX idx_incidents_status_time ON incidents(status, created_at DESC);

-- GIN index for JSONB queries (optional, for future payload search)
-- CREATE INDEX idx_incidents_payload ON incidents USING GIN(payload jsonb_path_ops);
```

### Retention Policy

```sql
-- Daily cron job: archive + delete events older than 90 days
-- Archive: export to JSON file at data/incidents-archive/YYYY-MM-DD.json
-- Then delete from incidents table

-- Archive query (ran by cron):
COPY (
  SELECT * FROM incidents
  WHERE created_at < NOW() - INTERVAL '90 days'
  ORDER BY created_at
) TO '/data/incidents-archive/2026-03-11.json';

-- Delete archived records:
DELETE FROM incidents
WHERE created_at < NOW() - INTERVAL '90 days';
```

### Payload Shape Per Feature

Each feature emits a standard payload structure. The `payload` JSONB column is flexible, but features should follow conventions:

```json
// Deployment events
{
  "service": "api-gateway",
  "version": "v2.4.1",
  "commit": "a1b2c3d",
  "commit_message": "Fix memory leak in request handler",
  "author": "Endang",
  "environment": "production",
  "deployment_id": "dep-123",
  "duration_seconds": 45,
  "error": "OOMKilled"        // only on failure
}

// Uptime events
{
  "monitor_id": "upt-1",
  "monitor_name": "App 1 Health",
  "url": "https://app1.edsuwarna.id/health",
  "status": "down",
  "status_code": 502,
  "response_time_ms": 3000,
  "error": "HTTP 502 Bad Gateway",
  "check_type": "http"
}

// SSL events
{
  "monitor_id": "ssl-1",
  "domain": "app1.edsuwarna.id",
  "days_remaining": 5,
  "expiry_date": "2026-06-16T00:00:00Z",
  "issuer": "R3",
  "cipher_grade": "A",
  "error": "Certificate expires in 5 days"
}

// Health check events
{
  "check_id": "hc-1",
  "check_name": "API Health",
  "endpoint": "/health",
  "status": "unhealthy",
  "response_time_ms": 2500,
  "error": "Database connection pool exhausted"
}

// Backup events
{
  "backup_id": "bkp-1",
  "target": "postgres-main",
  "type": "pg_dump",
  "size_bytes": 4294967296,
  "duration_seconds": 120,
  "destination": "s3://backups/production/",
  "error": "S3 upload failed: connection timeout"
}
```

---

## 6. UX Design

### Page Layout

```
┌──────────────────────────────────────────────────────────────┐
│  Ops > Incidents                           [🕐 Last 24h ▼]  │
│                                                              │
│  ┌── Stats Summary ──────────────────────────────────────┐   │
│  │  ██ Critical: 14   ██ Warning: 89   ██ Info: 209      │   │
│  │  MTTR: 12m 34s    Top: api-gateway (42)               │   │
│  └────────────────────────────────────────────────────────┘   │
│                                                              │
│  ┌── Filter Bar ─────────────────────────────────────────┐   │
│  │  [1h] [6h] [24h*] [7d] [30d]    [Critical] [Warn] [Info]│ │
│  │  [🚀] [📊] [🔒] [💾] [❤️]    [Status ▼]   [🔍 Service] │
│  │                                          [Clear All]     │ │
│  └────────────────────────────────────────────────────────┘   │
│                                                              │
│  ┌── Timeline ───────────────────────────────────────────┐   │
│  │                                                        │   │
│  │  🔴 1m ago  🚀  Deployment failed: api-gateway v2.4.1  │   │
│  │                    ⚬ critical · production · [OPEN]    │   │
│  │  ────────────────────────────────────────────────       │   │
│  │  🟡 3m ago  ❤️  Health check failed: API Health        │   │
│  │                    ⚬ warning · api-gateway · [OPEN]    │   │
│  │  ────────────────────────────────────────────────       │   │
│  │  ╔══ 3 events in this window ═══════════════════╗      │   │
│  │  ║ 🔴 5m ago  📊  Uptime down: App 1 Health    ║      │   │
│  │  ║ 🟡 5m ago  🔒  SSL expiring: api.edsuwarna  ║      │   │
│  │  ║ 🟡 5m ago  💾  Backup failed: postgres-main  ║      │   │
│  │  ╚══════════════════════════════════════════════╝      │   │
│  │  ────────────────────────────────────────────────       │   │
│  │  🔵 15m ago 🚀  Deployment success: worker-queue v1.2   │   │
│  │    ✅ Resolved by Endang · 12m ago                      │   │
│  │  ────────────────────────────────────────────────       │   │
│  │  🔴 32m ago 📊  Uptime down: API Gateway                │   │
│  │                    ⚬ critical · production · [RESOLVED] │   │
│  │                                                        │   │
│  │                    [ Load more... ]                     │   │
│  └────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

### Event Detail Panel (Slide-out)

```
┌─ Panel (480px) ───────────────────────────────┐
│  [✕ Close]                                     │
│                                                 │
│  🔴  CRITICAL                                   │
│  Deployment Failed                              │
│  11 Jun 2026, 03:15:00 AM UTC                   │
│                                                 │
│  ┌── Source ───────────────────────────────┐    │
│  │  api-gateway        🟣 production       │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  Container failed to start — exit code 137      │
│  (OOMKilled). The deployment was of version     │
│  v2.4.1 with commit a1b2c3d.                   │
│                                                 │
│  ┌── JSON Payload ──────── [▶ expand] ─────┐    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  ┌── Related Events ───────────────────────┐    │
│  │  🟡 3m ago  Health check failed         │    │
│  │  🔴 5m ago  Uptime down: App 1 Health  │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  ┌── Status Timeline ──────────────────────┐    │
│  │  🟢 Created  · 03:15                     │    │
│  │  └─ 🟡 Investigating · 03:20 by Endang   │    │
│  │     └─ ✅ Resolved · 03:45 by Endang     │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  ┌── Actions ──────────────────────────────┐    │
│  │  [🟡 Mark Investigating]                │    │
│  │  [✅ Resolve]   [🚫 False Alarm]        │    │
│  │                                         │    │
│  │  Note: [____________________________]   │    │
│  └─────────────────────────────────────────┘    │
└─────────────────────────────────────────────────┘
```

### Empty State

```
┌────────────────────────────────────────────────┐
│  🔍 No incidents found                          │
│                                                 │
│  No events match your current filters.           │
│  Try expanding the time range or clearing        │
│  some filters.                                   │
│                                                 │
│  [🧹 Clear All Filters]                         │
│                                                 │
│  ── Or: No events emitted yet ──                 │
│  The event bus needs to receive events from      │
│  deployments, uptime monitors, SSL checks,       │
│  health checks, or backups before they show      │
│  up here.                                        │
└────────────────────────────────────────────────┘
```

### Filter Interaction Details

- **Time range pills**: Click to select. Selected pill gets filled background. Default: 24h.
- **Severity chips**: Multi-select toggle. Click to toggle on/off. Colored border when active. Can deselect all = show all.
- **Feature icons**: Multi-select toggle with feature icons and labels. Active state: filled background.
- **Status dropdown**: Single-select. Options: All, Open, Investigating, Resolved. Default: All.
- **Service search**: Text input with 300ms debounce. Autocomplete dropdown showing matching service names from recent events.
- **URL persistence**: All active filters serialized to URL query params — `?range=24h&severity=critical,warning&feature=deployment&status=open&service=api`. Shareable, bookmarkable, back-button compatible.
- **Clear all**: Resets all filters to defaults.

---

## 7. Non-Functional Requirements

| Aspect | Target | Details |
|--------|--------|---------|
| Event ingestion latency | < 100ms | Channel emit → DB insert. Bulk insert buffer optional for burst. |
| Timeline query response | < 500ms | For any filter combination on 90 days of data (~100K events). Indexes cover all filter dimensions. |
| Concurrent event throughput | > 100 events/sec | Channel buffer 1024, consumer goroutine pool size 4. |
| Retention | 90 days | Auto-archive to JSON files. Configurable via `INCIDENTS_RETENTION_DAYS`. |
| Archive format | JSONL | One JSON object per line. Compressed with gzip. |
| Storage estimate | ~1 GB / 90 days | Average event ~10KB with payload. 100K events = ~1GB. Negligible vs disk. |
| Availability | Core feature — no single point of failure | Event bus part of backend process. DB failure = events buffered in channel (up to 1024). |

### Event Ingestion Pipeline

```
Feature (emit)
  │
  ▼
bus.Emit() ──→ channel (buffered 1024)
  │
  ▼
consumer goroutine pool (4 workers)
  │
  ├── compute correlation_group (SHA256)
  ├── build Incident struct
  └── INSERT INTO incidents
        │
        if error → retry once after 1s
                  → if still error → log + discard (eventually consistent)
```

---

## 8. Implementation Roadmap

### Phase 1 — Core Infrastructure (4 days)

**Goal:** incidents table, event bus engine, timeline view, filters

| Order | Feature | Effort | Depends On |
|-------|---------|--------|------------|
| 1 | Migration: `incidents` table + indexes | 1 day | — |
| 2 | Backend: Event bus engine (Go channels, consumer, emitter interface) | 1 day | #1 |
| 3 | Backend: Timeline CRUD / query endpoint with filters | 1 day | #1 |
| 4 | Frontend: Route `/incidents` + vertical timeline component | 1 day | #3 |
| 5 | Frontend: Filter bar (time range, severity, feature, status, service) | 1 day | #4 |
| 6 | Frontend: Infinite scroll pagination | 0.5 day | #4 |
| 7 | Frontend: Severity coloring + feature icons | 0.5 day | #4 |
| | **Total** | **6 days** | |

### Phase 2 — Feature Integration (4 days)

**Goal:** Emit events from existing features (deployment, uptime, health, SSL, backup)

| Order | Feature | Effort | Depends On |
|-------|---------|--------|------------|
| 8 | Backend: Register deployment emitter → emit on deploy success/fail | 1 day | Phase 1 |
| 9 | Backend: Register uptime emitter → emit on status change (up→down, down→up) | 1 day | Phase 1 |
| 10 | Backend: Register health check emitter → emit on health check fail/pass | 1 day | Phase 1 |
| 11 | Backend: Register SSL emitter → emit on expiry/critical/warning | 1 day | Phase 1 |
| 12 | Backend: Register backup emitter → emit on backup success/fail | 1 day | Phase 1 |
| | **Total** | **5 days** | |

### Phase 3 — Correlation, Resolve & Stats (3 days)

**Goal:** Auto-correlation, resolve flow, stats dashboard

| Order | Feature | Effort | Depends On |
|-------|---------|--------|------------|
| 13 | Backend: Auto-correlation (correlation_group hash on insert) | 1 day | Phase 2 |
| 14 | Frontend: Correlation bracket UI | 0.5 day | #13 |
| 15 | Backend: Resolve / investigate endpoint + status transitions | 0.5 day | Phase 2 |
| 16 | Frontend: Slide-out detail panel + action buttons | 1 day | #15 |
| 17 | Backend: Stats endpoint (aggregate queries) | 0.5 day | Phase 2 |
| 18 | Frontend: Stats dashboard cards + charts | 1 day | #17 |
| 19 | Frontend: Related events in detail panel | 0.5 day | #16 |
| | **Total** | **5 days** | |

### Phase 4 — Polish & Archive (2 days)

**Goal:** Retention policy, archive, performance tuning

| Order | Feature | Effort | Depends On |
|-------|---------|--------|------------|
| 20 | Backend: Retention cron (archive + delete old events) | 0.5 day | Phase 3 |
| 21 | Backend: Archive format (JSONL + gzip) + archive config | 0.5 day | #20 |
| 22 | Backend: Performance optimization (query tuning, connection pooling) | 0.5 day | Phase 3 |
| 23 | Frontend: Empty state, error state, loading skeletons | 0.5 day | Phase 3 |
| 24 | Frontend: URL filter persistence (shareable links) | 0.5 day | Phase 3 |
| | **Total** | **2.5 days** | |

### Total Estimated Effort: ~18.5 days (All 4 phases)

---

## 9. Design Decisions

### Why Internal Event Bus vs External Message Queue?

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Go channels (internal) | Zero infra, < 1ms latency, simple code | No persistence if process dies, no replay, limited to single process ✅ **Chosen** |
| Kafka / NATS | Persistent, replayable, multi-process | Infra overhead, higher latency, operational complexity ❌ |
| PostgreSQL LISTEN/NOTIFY | Built-in, transactional | 1 notification per event, no batching, limited queue depth ❌ |

**Decision**: Start with Go channels. If Anjungan scales to multi-instance or needs event replay, replace with NATS JetStream (same publisher/subscriber pattern, minimal refactor).

### Why Correlation by Time Window vs ML?

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Time-window hash (5 min) | Deterministic, O(1), no deps | May miss cross-service correlations ✅ **Chosen** |
| ML clustering | Finds non-obvious correlations | Complex, slow, non-deterministic, infra overhead ❌ |
| Manual linking | Precise | Requires user effort, doesn't scale ❌ |

**Decision**: Time-window correlation covers 90%+ of real cases (deployment → health failure → uptime down within minutes). Simple, fast, predictable.

### Why JSONB Payload vs Separate Tables per Feature?

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| JSONB payload | Flexible, no schema changes, single table ✅ **Chosen** | No referential integrity, harder to query inside payload |
| Separate tables (deployment_events, uptime_events, etc.) | Strong schema, queryable | Lots of tables, UNION queries for timeline, migration per feature ❌ |

**Decision**: JSONB. The payload is archival (written once, read occasionally for detail view). Querying happens on indexed columns (feature, event_type, severity, created_at) not inside JSONB.

---

## 10. Event Types Reference

### Feature-Defined Event Types

Each feature registers its event types. The list is extensible — add new types by agreement, no schema change needed.

| Feature | Event Type | Severity | Description |
|---------|-----------|----------|-------------|
| deployment | `deployment_success` | info | Deploy completed successfully |
| deployment | `deployment_failed` | critical | Deploy failed (exit error, OOM, etc.) |
| deployment | `deployment_rolled_back` | warning | Deploy was rolled back |
| uptime | `uptime_down` | critical | Service is down (HTTP/TCP check failed) |
| uptime | `uptime_up` | info | Service recovered (was down, now up) |
| uptime | `uptime_degraded` | warning | Response time > threshold (e.g. > 2s) |
| ssl | `ssl_expiring_critical` | critical | Certificate expires in < 7 days |
| ssl | `ssl_expiring_warning` | warning | Certificate expires in < 14 days |
| ssl | `ssl_expired` | critical | Certificate already expired |
| ssl | `ssl_renewed` | info | Certificate was renewed |
| ssl | `ssl_check_failed` | warning | TLS handshake / OCSP check failed |
| health | `health_check_failed` | critical | Health endpoint returned non-200 or timeout |
| health | `health_check_passed` | info | Health endpoint recovered |
| health | `health_check_degraded` | warning | Response time > 1s |
| backup | `backup_failed` | critical | Backup process failed |
| backup | `backup_success` | info | Backup completed successfully |
| backup | `backup_degraded` | warning | Backup took longer than expected (> 1h) |

---

## 11. Mockup Screenshots

> Mockup HTML: `sketches/incidents-timeline/mockup.html`
> Playwright script: `sketches/incidents-timeline/screenshot.js`

### Timeline View — All Events

![Incidents Timeline List](../sketches/incidents-timeline/incidents-timeline-list.png)

### Event Detail Panel (Slide-Out)

![Incidents Detail Panel](../sketches/incidents-timeline/incidents-detail-panel.png)

### Stats Dashboard

![Incidents Stats Dashboard](../sketches/incidents-timeline/incidents-stats-dashboard.png)

---

## 12. Open Questions

| Question | Status | Decision Needed By |
|----------|--------|-------------------|
| Should we add a `POST /api/v1/incidents` endpoint for manual event creation (for testing)? | 🔴 Open | Start of Phase 1 |
| Should auto-correlation consider `source_environment` alongside `source_service`? | 🔴 Open | Start of Phase 3 |
| Should resolved events auto-close related open events in same correlation group? | 🔴 Open | Start of Phase 3 |
| Retention: archive to S3/GCS as well as local JSON? | 🔴 Open | Start of Phase 4 |
| Should we add webhook integration so external systems can push events into the timeline? | 🔴 Open | Post Phase 4 |

---

## 13. Changelog

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | June 11, 2026 | Endang Suwarna | Initial PRD — full specification |
