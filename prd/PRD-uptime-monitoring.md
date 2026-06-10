# Anjungan — PRD: Uptime Monitoring

> **Version:** 1.0
> **Status:** 🔴 Draft — Branch `feat/uptime-monitoring`
> **Author:** Endang Suwarna
> **Last Updated:** June 10, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan manages dozens of services across multiple servers, but there is **no centralized uptime monitoring**. Currently:

- Services go down and nobody knows until users report "site not working"
- Relies on external tools (Uptime Kuma, Pingdom) — fragmented visibility, another login to manage
- No historical uptime data — impossible to prove SLA or track reliability trends
- SSL monitoring exists (certs only) but doesn't detect HTTP/TCP service outages
- No unified notification channel for service degradation

### What This Solves

| Problem | Solution |
|---------|----------|
| Services go down without alerting anyone | Automated HTTP/TCP health checks + instant notifications |
| Fragmented uptime tools across different platforms | Native Anjungan feature — one dashboard |
| No reliability history for services | Persistent check history + daily uptime summary |
| No clear "is everything OK?" at a glance | Uptime StatCard on main dashboard + color-coded status |
| SSL monitoring only covers cert expiry | Full service-layer uptime — complementary to SSL |

### Target Audience

- **Endang** (platform engineer) — see all service status at a glance, get notified before users
- **DevOps** — monitor services they're responsible for, prove uptime to stakeholders
- **Admins** — quick triage: "what's down right now?"

### Goals

| Goal | Metric |
|------|--------|
| Add uptime monitor from UI | < 10 seconds |
| HTTP(S) health check | Per configurable interval (default 5 min) |
| TCP port health check | Per configurable interval |
| Detect service down | < interval + 30s |
| Notifications via existing channels | Telegram / Discord / Slack / generic webhook |
| Historical uptime data | 30-day retention, daily summary for longer view |
| Standalone — zero external dependencies | Self-contained in Anjungan backend |

### Non-Goals

- ❌ Not a replacement for log monitoring (no log parsing)
- ❌ Not a full APM solution (no traces, no profiling)
- ❌ No ICMP/ping checks (requires privileged access in Docker)
- ❌ No distributed probe network (checks originate from Anjungan server only)
- ❌ No synthetic transaction / scripted browser checks (v2 consideration)

---

## 2. Product Overview

### This Feature in the Context of Anjungan

```
User Dashboard
│
├── Uptime Monitoring Menu (/uptime)
│   ├── Monitor List → status badges, response time, last checked
│   ├── Add Uptime Monitor → manual entry form
│   ├── Monitor Detail → check history + response time chart
│   └── Check All → manual trigger batch check
│
├── Shared Notification Targets (unified with SSL)
│   ├── Telegram / Discord / Slack / generic webhook
│   └── Target assignment via multi-select in monitor form
│
├── Dashboard
│   └── Uptime StatCard → all-green / some-down / all-paused
│
└── Cron Engine (backend)
    ├── CheckAllMonitors() → periodic HTTP/TCP check
    └── NotifyOnStatusChange() → alert if up→down or down→up
```

### Flow: Add & Monitor Service

```
User input                    Anjungan                            Internet/Target
┌────────────────┐          ┌───────────────────────┐         ┌──────────────────┐
│ URL: X          │          │ 1. Save to DB          │         │ 4. HTTP GET/TCP  │
│ Check: HTTP     │ ───────▶ │ 2. Initial check       │ ──────▶ │ 5. Response code │
│ Interval: 5m    │          │ 3. Update status        │  OK/ERR │ 6. Response time │
│ Timeout: 30s    │          │ 7. Display in UI        │         └──────────────────┘
└────────────────┘          └───────────────────────┘
```

### Key Design Decision: Standalone Feature + Shared Notification

Unlike SSL monitoring which has its own `ssl_notification_targets` table, this feature **shares a generalised `notification_targets` table** with SSL monitoring to avoid duplicate configuration.

**Migration plan:**
1. Create `notification_targets` table (supersedes `ssl_notification_targets`)
2. Migrate existing `ssl_notification_targets` data into `notification_targets` with `scopes = ['ssl']`
3. SSL monitoring reads from `notification_targets` where `scopes @> ARRAY['ssl']`
4. Uptime monitoring reads from `notification_targets` where `scopes @> ARRAY['uptime']`

---

## 3. Feature Specifications

> **Priority Key:** P0 = Must have, P1 = Should have, P2 = Nice to have

### F1 — Uptime Monitor CRUD (P0)

| | |
|---|---|
| **Backend** | `uptime_monitors` table. CRUD: `GET/POST/PUT/DELETE /api/v1/uptime-monitors`. Fields: name, url, check_type (http, tcp), interval_seconds (default 300), timeout_seconds (default 30), expected_status_min (default 200), expected_status_max (default 399), expected_body (optional regex), enabled, notification_target_ids (TEXT[]), created_by. **Duplicate check**: (name + url) unique → 409 Conflict. Paginated list with search, filter by status, sort by name/status/last_check_at. |
| **Frontend** | Route `/uptime`. Monitor list with status badge, response time, last checked, notification targets pill. "+ Add Uptime Monitor" button → modal form. Click card → detail view. **Summary KPI cards**: total, up, down, paused counts. Status filter chips. Search bar. |
| **UX** | Badge color: 🟢 UP, 🔴 DOWN, ⚪ PAUSED, 🟡 PENDING. Quick action on each card: Check Now, Edit, Delete, Pause/Resume. "Check All" button for batch. Empty state when no monitors configured. |

### F2 — HTTP(S) Health Check (P0)

| | |
|---|---|
| **Backend** | Go `net/http` client → GET request to URL → capture: HTTP status code, response time (ms), response body snippet (first 1KB). Validate: `expected_status_min <= status_code <= expected_status_max`. If `expected_body` set, regex match body. Store result: status (up/down/error), status_code, response_time_ms, error_message. |
| **Frontend** | Detail page: last response info. Status code badge. Response time with ms unit. |
| **UX** | Response time shown with color coding: <200ms 🟢, 200-1000ms 🟡, >1000ms 🔴. |

### F3 — TCP Port Health Check (P0)

| | |
|---|---|
| **Backend** | Go `net.DialTimeout` → TCP connect to `host:port` → success = up, failure = down. Capture response time. |
| **Frontend** | Same as HTTP — status + response time displayed. |
| **UX** | Simpler than HTTP — just connect/not-connect. |

### F4 — Automated Scheduled Checks (P1)

| | |
|---|---|
| **Backend** | `scheduler.go` — background goroutine. Pattern identical to SSL monitoring scheduler. Iterates enabled monitors, runs check per monitor interval. **Notification dedup**: only fire on status change (up→down or down→up), not every cycle. `POST /api/v1/uptime-monitors/{id}/check` — manual trigger. `POST /api/v1/uptime-monitors/check-all` — batch check. |
| **Frontend** | "Last checked: X min ago". "Check Now" per monitor. "Check All" in list header — sequential batch with per-monitor progress. |
| **UX** | During check: spinner/pulsing status. Check All shows per-monitor results inline. |

### F5 — Notification Integration (P1)

| | |
|---|---|
| **Backend** | On status change (up→down / down→up / first check), dispatch to `notification_targets` where `scopes @> ARRAY['uptime']`. Formats per-platform: Telegram HTML, Discord embed, Slack JSON. Event payload: name, url, status, status_code, response_time_ms, previous_status. Dedup: only on status change, not every cycle. `POST /api/v1/uptime-monitors/{id}/test-notification` — test delivery. |
| **Frontend** | In Add/Edit form: "Notify via" section — multi-select from saved notification targets. Detail page: assigned targets with test button. |
| **UX** | Test button sends sample up/down notification to verify delivery. |

### F6 — Check History & Response Time Chart (P1)

| | |
|---|---|
| **Backend** | `uptime_check_history` table. Each check: monitor_id, checked_at, status, status_code, response_time_ms, error_message. `GET /api/v1/uptime-monitors/{id}/history` — paginated (?limit=&offset=). `GET /api/v1/uptime-monitors/{id}/trend?period=7d` — daily summary for chart. **Retention**: auto-purge rows older than configured days (default 30). |
| **Frontend** | Detail page → "Check History" panel. **Response time line chart** (SVG): x=time, y=ms. Color overlay per status (red for down). Hover tooltip. **Uptime percentage card**: last 24h / 7d / 30d. **Event timeline**: recent status changes. |
| **UX** | Chart shows last 24h by default. Tabs: 24h, 7d, 30d. Down events highlighted red on chart. |

### F7 — Daily Summary (P2)

| | |
|---|---|
| **Backend** | `uptime_daily_summary` table. Aggregation cron (runs every hour): monitor_id, date, total_checks, up_count, down_count, avg_response_ms, min_response_ms, max_response_ms, uptime_percent (DECIMAL 5,2). Used by trend endpoint for periods > 24h. |
| **Frontend** | Detail page uptime percentage card reads from daily summary. |
| **UX** | "99.5% uptime (last 30 days)" — click for breakdown. |

### F8 — Notification Targets Generalisation (P1)

| | |
|---|---|
| **Backend** | New `notification_targets` table — unified replacement for `ssl_notification_targets`. Same columns + `scopes TEXT[]`. Migration 000028: create table, migrate data from `ssl_notification_targets`, add scopes column. **Backward compat**: SSL monitoring handler routes updated to read from new table filtered by scope. Old table kept during transition. |
| **Frontend** | Single "Notification Targets" modal — create/edit/delete + test. Each target has checkboxes for which features use it (SSL, Uptime, both). |
| **UX** | Unified UX — user sets up webhook once, uses across features. |

---

## 4. API Design

### New Endpoints — Uptime Monitors

```
GET    /api/v1/uptime-monitors                           // List (?page=&limit=&search=&status=&sort=&order=)
POST   /api/v1/uptime-monitors                           // Create
GET    /api/v1/uptime-monitors/summary                   // KPI counts: total, up, down, paused
POST   /api/v1/uptime-monitors/check-all                 // Trigger check all enabled
GET    /api/v1/uptime-monitors/{id}                      // Detail
PUT    /api/v1/uptime-monitors/{id}                      // Update
DELETE /api/v1/uptime-monitors/{id}                      // Delete
POST   /api/v1/uptime-monitors/{id}/check                // Manual check
POST   /api/v1/uptime-monitors/{id}/pause                // Pause (disable without deleting)
POST   /api/v1/uptime-monitors/{id}/resume               // Resume
GET    /api/v1/uptime-monitors/{id}/history              // Paginated history (?limit=&offset=)
GET    /api/v1/uptime-monitors/{id}/trend                // Trend data for chart (?period=7d)
POST   /api/v1/uptime-monitors/{id}/test-notification    // Send test notification
```

### Response Format

```json
// POST /api/v1/uptime-monitors (Create)
{
  "success": true,
  "data": {
    "id": "uuid-upt-1",
    "name": "App 1 Production",
    "url": "https://app1.edsuwarna.id/health",
    "check_type": "http",
    "interval_seconds": 300,
    "timeout_seconds": 30,
    "expected_status_min": 200,
    "expected_status_max": 399,
    "expected_body": "",
    "enabled": true,
    "notification_target_ids": ["uuid-nt-1"],
    "created_at": "2026-06-10T10:00:00Z"
  }
}

// GET /api/v1/uptime-monitors/{id} (Detail)
{
  "success": true,
  "data": {
    "id": "uuid-upt-1",
    "name": "App 1 Production",
    "url": "https://app1.edsuwarna.id/health",
    "check_type": "http",
    "interval_seconds": 300,
    "timeout_seconds": 30,
    "expected_status_min": 200,
    "expected_status_max": 399,
    "expected_body": "",
    "enabled": true,
    "status": "up",
    "last_status": "down",
    "last_status_code": 200,
    "last_response_time_ms": 145,
    "last_error": "",
    "last_check_at": "2026-06-10T10:00:00Z",
    "notification_target_ids": ["uuid-nt-1"],
    "created_at": "2026-06-10T10:00:00Z",
    "updated_at": "2026-06-10T10:00:00Z"
  }
}

// GET /api/v1/uptime-monitors/summary
{
  "success": true,
  "data": {
    "total": 8,
    "up": 6,
    "down": 1,
    "paused": 1
  }
}

// POST /api/v1/uptime-monitors/{id}/check (Manual check)
{
  "success": true,
  "data": {
    "status": "up",
    "status_code": 200,
    "response_time_ms": 145,
    "checked_at": "2026-06-10T10:00:00Z"
  }
}
```

### Modified Endpoints — Notification Targets (Unified)

```
GET    /api/v1/notification-targets                      // List all
POST   /api/v1/notification-targets                      // Create
GET    /api/v1/notification-targets/{id}                 // Get
PUT    /api/v1/notification-targets/{id}                 // Update
DELETE /api/v1/notification-targets/{id}                 // Delete
POST   /api/v1/notification-targets/{id}/test            // Send test notification
```

Query params: `?scope=ssl` or `?scope=uptime` to filter by feature.

**Backward compat**: Old `/api/v1/ssl-monitors/notification-targets/*` routes redirect or continue working until deprecated.

---

## 5. Database Schema

### New Tables

```sql
-- 000028_create_uptime_monitors.up.sql
CREATE TABLE uptime_monitors (
    id TEXT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    check_type VARCHAR(16) NOT NULL DEFAULT 'http',        -- http, tcp
    interval_seconds INTEGER NOT NULL DEFAULT 300,
    timeout_seconds INTEGER NOT NULL DEFAULT 30,
    expected_status_min INTEGER DEFAULT 200,
    expected_status_max INTEGER DEFAULT 399,
    expected_body TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    notification_target_ids TEXT[] NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',          -- pending, up, down, paused
    last_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    last_status_code INTEGER,
    last_response_time_ms INTEGER,
    last_error TEXT NOT NULL DEFAULT '',
    last_check_at TIMESTAMPTZ,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(name, url)
);

-- 000029_create_uptime_check_history.up.sql
CREATE TABLE uptime_check_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    monitor_id TEXT NOT NULL REFERENCES uptime_monitors(id) ON DELETE CASCADE,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL,                             -- up, down, error, timeout
    status_code INTEGER,
    response_time_ms INTEGER,
    error_message TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_uptime_check_history_monitor_time
    ON uptime_check_history(monitor_id, checked_at DESC);

-- 000030_create_uptime_daily_summary.up.sql
CREATE TABLE uptime_daily_summary (
    monitor_id TEXT NOT NULL REFERENCES uptime_monitors(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    total_checks INTEGER NOT NULL DEFAULT 0,
    up_count INTEGER NOT NULL DEFAULT 0,
    down_count INTEGER NOT NULL DEFAULT 0,
    avg_response_ms INTEGER,
    min_response_ms INTEGER,
    max_response_ms INTEGER,
    uptime_percent DECIMAL(5,2),
    PRIMARY KEY (monitor_id, date)
);

-- 000031_generalise_notification_targets.up.sql
-- Step 1: Create unified notification_targets table
CREATE TABLE notification_targets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT 'generic',                -- telegram, discord, slack, generic
    webhook_secret TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    scopes TEXT[] NOT NULL DEFAULT '{}',                     -- {'ssl'}, {'uptime'}, {'ssl', 'uptime'}
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Step 2: Migrate existing ssl_notification_targets
INSERT INTO notification_targets (id, name, url, platform, webhook_secret, enabled, scopes, created_by, created_at, updated_at)
SELECT id, name, url, platform, webhook_secret, enabled, ARRAY['ssl'], created_by, created_at, updated_at
FROM ssl_notification_targets;

-- Step 3: Add index
CREATE INDEX idx_notification_targets_scopes ON notification_targets USING GIN(scopes);
CREATE INDEX idx_notification_targets_enabled ON notification_targets(enabled);
```

### Retention Policy

```sql
-- Cron job (runs daily):
DELETE FROM uptime_check_history
WHERE checked_at < NOW() - INTERVAL '30 days';
```

### Migration Plan

| # | Table | Description |
|---|-------|-------------|
| 000028 | `uptime_monitors` | Core monitor table |
| 000029 | `uptime_check_history` | Check result history with composite index |
| 000030 | `uptime_daily_summary` | Aggregated daily uptime data |
| 000031 | `notification_targets` + migrate ssl data | Unified notification targets (shared with SSL) |
| 000032 | — | Drop `ssl_notification_targets` (after migration verified) |

---

## 6. UX Flow

### Flow: Add Uptime Monitor

```
1. Click "+ Add Uptime Monitor"
2. Fill form:
   [Name *]         App 1 Health Check       → required
   [URL *]          https://app1.edsuwarna.id → required, URL validation
   [Check Type]     HTTP(S) ▼                → dropdown: HTTP(S), TCP
   [Interval]       Every 5 minutes           → dropdown: 30s, 1m, 5m, 15m, 30m, 1h
   [Timeout]        30 seconds                → dropdown: 5s, 10s, 30s, 60s
   [Expected Status] 200 - 399                → min/max range
   [Expected Body]  "ok"                      → optional regex
   [Notify Via]     [📢 Telegram Slack ▼]     → multi-select from shared targets
3. Click "Add Monitor" → saves → immediately triggers first check
4. Show spinner "Checking service..."
5. Result → status badge appears, response time populated
```

### Flow: Dashboard View

```
┌─────────────────────────────────────────────┐
│  📊 Uptime Monitoring                        │
│                                              │
│  ┌────────── ┌────────── ┌──────────┐        │
│  │ 🟢 UP     │ 🔴 DOWN   │ ⚪ PAUSED │        │
│  │   6       │   1       │    1     │        │
│  └────────── └────────── └──────────┘        │
│                                              │
│  [+ Add Monitor]  [🔃 Check All]             │
│                                              │
│  ┌──────────────────────────────────────┐    │
│  │ 🟢 App 1 Health                        │    │
│  │    https://app1.edsuwarna.id/health    │    │
│  │    145ms · 200 · 2m ago               │    │
│  │    ⋮ (Check Now · Edit · Pause · Del) │    │
│  ├──────────────────────────────────────┤    │
│  │ 🔴 API Gateway                        │    │
│  │    https://api.edsuwarna.id/health    │    │
│  │    timeout · 15m ago                  │    │
│  │    ⋮ (Check Now · Edit · Resume · Del)│    │
│  └──────────────────────────────────────┘    │
└─────────────────────────────────────────────┘
```

### Flow: Monitor Detail

```
┌──────────────────────────────────────────┐
│  🔙 Back to Uptime Monitors              │
│                                          │
│  🟢 App 1 Health                         │
│     https://app1.edsuwarna.id/health     │
│     Added 10 Jun 2026 · HTTP check        │
│                                          │
│  ┌── Current Status ────────────────┐    │
│  │ Status:      UP 🟢               │    │
│  │ Code:        200 OK              │    │
│  │ Response:    145ms 🟢            │    │
│  │ Last Check:  2 min ago           │    │
│  └──────────────────────────────────┘    │
│                                          │
│  ┌── Uptime ────────────────────────┐    │
│  │ [24h: 100%] [7d: 99.8%] [30d: ...│    │
│  │                                  │    │
│  │  📈 Response Time (24h)          │    │
│  │  │\    ┌─┐                      │    │
│  │  │ └──┘ └──┐  ┌─┐              │    │
│  │  │         └──┘ └──            │    │
│  │  └─────────────────────────▶    │    │
│  └──────────────────────────────────┘    │
│                                          │
│  ┌── Check History ─────────────────┐    │
│  │ + 10m ago 🟢 200  150ms          │    │
│  │ + 5m ago  🟢 200  142ms          │    │
│  │ - 2h ago  🔴 502  2000ms         │    │
│  │ + 2h ago  🟢 200  138ms          │    │
│  └──────────────────────────────────┘    │
│                                          │
│  │ ⋮ Check Now │ Edit │ Delete │         │
└──────────────────────────────────────────┘
```

### Empty State

```
┌──────────────────────────────────────────┐
│  📊 Uptime Monitoring                     │
│                                          │
│  ┌──────────────────────────────────────┐│
│  │                                      ││
│  │       🔍 No monitors yet             ││
│  │                                      ││
│  │   Add your first uptime monitor      ││
│  │   to start tracking service health.  ││
│  │                                      ││
│  │      [➕ Add Uptime Monitor]          ││
│  │                                      ││
│  └──────────────────────────────────────┘│
└──────────────────────────────────────────┘
```

### Flow: Pause/Resume

```
1. Pause: POST /api/v1/uptime-monitors/{id}/pause
   → status changes to "paused"
   → scheduler skips this monitor
   → card shows ⚪ badge
   → "Pause" button becomes "Resume"

2. Resume: POST /api/v1/uptime-monitors/{id}/resume
   → status changes to "pending"
   → immediately triggers a check
   → card shows 🟡 while checking, then 🟢/🔴
```

---

## 7. Implementation Roadmap

### 🟢 Phase 1 — Core (Sprint 1)

**Goal:** Add monitors, run HTTP/TCP checks, see status in UI

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 1 | Migration 000028: `uptime_monitors` table | 0.5 day | — |
| 2 | Backend: Uptime monitor CRUD (handler + repo + model) | 1 day | #1 |
| 3 | Backend: HTTP check engine (`net/http` client) | 1 day | #2 |
| 4 | Backend: TCP check engine (`net.DialTimeout`) | 0.5 day | #2 |
| 5 | Backend: Uptime summary endpoint | 0.5 day | #3 |
| 6 | Frontend: Route `/uptime` + monitor list + status badges | 1 day | #2 |
| 7 | Frontend: Add/Edit uptime monitor form | 0.5 day | #6 |
| 8 | Frontend: Monitor detail page (status, response info) | 0.5 day | #3, #6 |
| 9 | Frontend: Dashboard StatCard | 0.25 day | #5 |
| 10 | Frontend: Pause/Resume UI | 0.25 day | #6 |
| | **Total** | **6 days** | |

### 🟡 Phase 2 — History, Charts & Notifications (Sprint 2)

**Goal:** Auto-check, response time chart, notifications

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 11 | Migration 000029: `uptime_check_history` | 0.25 day | — |
| 12 | Backend: History endpoint + auto-purge | 0.5 day | #11 |
| 13 | Backend: Scheduler (background goroutine) | 1 day | #3, #4 |
| 14 | Backend: Notification engine (on status change) | 1 day | #13 |
| 15 | Backend: Trend endpoint + test-notification | 0.5 day | #12 |
| 16 | Frontend: Response time SVG line chart | 1 day | #15 |
| 17 | Frontend: History timeline | 0.5 day | #12 |
| 18 | Frontend: Notification config in monitor form | 0.5 day | #14 |
| | **Total** | **5 days** | |

### ✅ Phase 3 — Enhancements

| Order | Feature | Effort | Priority |
|-------|---------|--------|----------|
| 19 | Migration 000030: `uptime_daily_summary` table | 0.25 day | P2 |
| 20 | Backend: Daily summary aggregation cron | 0.5 day | P2 |
| 21 | Frontend: Uptime percentage card (24h/7d/30d tabs) | 0.5 day | P2 |
| 22 | Migration 000031: `notification_targets` migration + generalisation | 1 day | P1 |
| 23 | Backend: Update SSL monitoring to use new unified table | 1 day | P1 |
| 24 | Frontend: Update Notification Targets modal for scopes | 0.5 day | P1 |
| 25 | Migration 000032: Drop `ssl_notification_targets` | 0.25 day | P2 |
| 26 | Export CSV (`GET /api/v1/uptime-monitors/export/csv`) | 0.25 day | P2 |
| 27 | Batch import (`POST /api/v1/uptime-monitors/import`) | 0.5 day | P2 |
| | **Total** | **4.75 days** | |

### Total Estimated Effort: ~15.75 days (All 3 phases)
