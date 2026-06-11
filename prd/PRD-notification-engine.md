# Anjungan — PRD: Unified Alerting / Notification Engine

> **Version:** 1.0
> **Status:** Draft
> **Author:** Endang Suwarna
> **Last Updated:** June 2026

---

## 1. Executive Summary

### Problem Statement

- Notifikasi pecah — Uptime punya sendiri, SSL punya sendiri
- Tiap fitur harus setup notif sendiri, tidak ada shared notification targets
- Tidak ada audit trail pengiriman notifikasi (siapa dikirimi, berhasil/gagal)
- Admin-only restriction membatasi pengguna lain untuk mengelola notifikasi sendiri

### Target Audience

- **Endang** (platform engineer) — mengelola semua notifikasi dari satu tempat, audit trail
- **DevOps / Platform Engineers** — mengkonfigurasi target notifikasi untuk fitur yang mereka kelola
- **All Users** — membuat, melihat, dan menguji notifikasi tanpa tergantung admin

### Goals

| Goal | Metric |
|------|--------|
| Shared notification targets across all features | 3+ feature integrations |
| Satu halaman /notifications di sidebar | Visible in Ops category |
| Support Telegram, Email, Webhook | 3 providers working |
| All users create/view/test notif targets | Not admin-only |

### Non-Goals

- ❌ Bukan email/SMS gateway — tidak mengirim email/SMS dari Anjungan sendiri
- ❌ Bukan alert routing / on-call scheduling — tidak ada escalation policy, PagerDuty integration
- ❌ Bukan notification templating engine — format notifikasi fixed per provider
- ❌ Bukan push notification ke mobile app — no native mobile push (FCM/APNs)

---

## 2. Product Overview

### Tech Stack

| Layer | Technology | Reason |
|-------|-----------|--------|
| Backend | Go (existing) | Notification engine + provider interface |
| Frontend | SvelteKit | New route /notifications, integration selectors |
| DB | PostgreSQL (existing) | notification_targets, notification_logs tables |
| Telegram | Bot API | Existing Telegram connection |
| Email | SMTP (via existing mailer) | Send via configured SMTP relay |
| Webhook | HTTP POST | Generic outgoing webhook |

### This Feature in the Context of Anjungan

```
                        ┌──────────────────────────────────────┐
                        │         Notification Engine          │
                        │                                      │
                        │  ┌─────────────────────────┐        │
                        │  │   notification_targets   │        │
                        │  │   - Telegram             │        │
                        │  │   - Email                │        │
                        │  │   - Webhook              │        │
                        │  └──────────┬──────────────┘        │
                        │             │                         │
                        │  ┌──────────▼──────────────┐        │
                        │  │   notification_logs      │        │
                        │  │   (audit trail)          │        │
                        │  └─────────────────────────┘        │
                        └──────────────────────────────────────┘
                                    │
             ┌──────────────────────┼──────────────────────┐
             │                      │                      │
             ▼                      ▼                      ▼
     ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐
     │   Uptime     │    │     SSL      │    │ Deployment Health│
     │  Monitoring  │    │  Monitoring  │    │                  │
     │              │    │              │    │                  │
     │ scopes:      │    │ scopes:      │    │ scopes:           │
     │ ['uptime']   │    │ ['ssl']      │    │ ['deployment-    │
     │              │    │              │    │  health']        │
     └──────────────┘    └──────────────┘    └──────────────────┘
```

Setiap fitur (Uptime, SSL, Deployment Health) membaca dari `notification_targets` yang memiliki scope yang sesuai. Satu target notifikasi bisa dipakai oleh banyak fitur sekaligus.

---

## 3. Feature Requirements

### 3.1 Feature Inventory

| Domain | Backend | Frontend | Status |
|--------|---------|---------|--------|
| Notification Targets CRUD | CRUD endpoints, provider registry | Card grid, edit/create modal | 🟡 Not started |
| Test Notification | POST /targets/{id}/test | Test button per card, toast result | 🟡 Not started |
| Notification Log | GET /notification-logs, stats endpoint | Table view with filters | 🟡 Not started |
| Feature Integration (Uptime) | Scope filter in uptime monitor config | Notification target selector in uptime settings | 🟡 Not started |
| Feature Integration (SSL) | Scope filter in SSL monitor config | Notification target selector in SSL settings | 🟡 Not started |
| Feature Integration (Deployment Health) | Scope filter in deployment config | Notification target selector in deployment settings | 🟡 Not started |

### 3.2 Database Schema

```sql
CREATE TABLE notification_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL, -- 'telegram', 'email', 'webhook'
    config JSONB NOT NULL, -- {chat_id, token, email, webhook_url, headers...}
    scopes TEXT[] DEFAULT '{}', -- ['uptime', 'ssl', 'deployment-health', 'backup']
    is_active BOOLEAN DEFAULT true,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE notification_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id UUID REFERENCES notification_targets(id),
    feature VARCHAR(50) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    title TEXT NOT NULL,
    message TEXT,
    status VARCHAR(20) DEFAULT 'pending', -- pending, sent, failed
    error TEXT,
    retry_count INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_notification_targets_scopes ON notification_targets USING GIN (scopes);
CREATE INDEX idx_notification_logs_target_id ON notification_logs(target_id);
CREATE INDEX idx_notification_logs_feature ON notification_logs(feature);
CREATE INDEX idx_notification_logs_status ON notification_logs(status);
CREATE INDEX idx_notification_logs_created_at ON notification_logs(created_at);
```

### 3.3 Feature Specs

#### F1 — Notification Targets CRUD (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 |
| **Backend** | CRUD endpoints: `GET /api/v1/notification-targets`, `POST`, `PUT /{id}`, `DELETE /{id}`. Validation: name required, type must be valid provider, config validated per type. Duplicate name → 409 Conflict. Scopes validated against known feature list. |
| **Frontend** | Route `/notifications`. Card grid layout. Each card shows: name, type badge (Telegram/Email/Webhook), config summary (masked), scope tags, active/inactive toggle. "+ Add Target" button top right triggers modal form. Edit/Create modal: name input, type dropdown (changes config fields dynamically), config fields per type, scope checkboxes, active toggle. |
| **UX** | Type badge colors: Telegram → blue, Email → purple, Webhook → gray. Scope tags as small pills. Config summary: Telegram shows "Chat: -12345****", Email shows "admin@****.com", Webhook shows "https://hooks****". Quick toggles for active/inactive without opening modal. Empty state illustration + "Add your first notification target" CTA. |

#### F2 — Test Notification (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 |
| **Backend** | `POST /api/v1/notification-targets/{id}/test`. Sends a test ping using the target's provider. Returns `{ status: "sent" | "failed", error?: string }`. |
| **Frontend** | Test button on each card. On click sends test → shows toast: ✅ "Test sent to [target name]" on success, ❌ "Failed: [error]" on failure. Button shows spinner during test. |
| **UX** | Toast auto-dismisses after 3 seconds. Test message: "🧪 Test notification from Anjungan — [target name]" with timestamp. |

#### F3 — Notification Log (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 |
| **Backend** | `GET /api/v1/notification-logs` — paginated, filterable by feature, target_id, status, date range. `GET /api/v1/notification-logs/stats` — aggregate counts: total, sent, failed, pending. |
| **Frontend** | Table view with columns: timestamp, feature badge, title, target name, status badge (🟢 sent, 🔴 failed, 🟡 pending), retry count. Filters: feature dropdown, target dropdown, status dropdown, date range picker. Search bar. Pagination. |
| **UX** | Row click → expand/collapse detail (error message, retry history). Status badge color-coded. Timestamp in relative format ("2 min ago") with hover tooltip for exact time. Export button (CSV) — stretch goal. |

#### F4 — Feature Integration (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 |
| **Backend** | Each feature (Uptime, SSL, Deployment Health) filters `notification_targets` by `scopes @> ARRAY['feature_name']`. Integration via the existing notification engine interface. On event (service down, cert expiring, deploy failed): look up targets with matching scope, dispatch notification. |
| **Frontend** | In each feature's settings/config form: multi-select component. Shows available targets filtered by that feature's scope. User can add/remove assignments. Each selected target shows: name, type badge, test button. |
| **UX** | Selector shows targets with scope for that feature pre-filtered. User can also toggle scope on the target itself from /notifications page. If a target has multiple scopes, it appears in multiple feature selectors. |

---

## 4. API Design

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | /api/v1/notification-targets | List all targets | ✅ |
| POST | /api/v1/notification-targets | Create target | ✅ |
| PUT | /api/v1/notification-targets/{id} | Update target | ✅ |
| DELETE | /api/v1/notification-targets/{id} | Delete target | ✅ |
| POST | /api/v1/notification-targets/{id}/test | Send test notification | ✅ |
| GET | /api/v1/notification-logs | List notification logs | ✅ |
| GET | /api/v1/notification-logs/stats | Stats (sent/failed/pending count) | ✅ |

### Request / Response Examples

**POST /api/v1/notification-targets**
```json
{
  "name": "DevOps Telegram",
  "type": "telegram",
  "config": {
    "chat_id": "-123456789",
    "token": "bot:token_here"
  },
  "scopes": ["uptime", "ssl"],
  "is_active": true
}
```

**POST /api/v1/notification-targets/{id}/test**
```json
{
  "status": "sent"
}
```

**GET /api/v1/notification-logs/stats**
```json
{
  "total": 1240,
  "sent": 1180,
  "failed": 52,
  "pending": 8
}
```

---

## 5. UI/UX Design Guidelines

### Key Layout

```
┌──────────────────────────────────────────────────────────────────┐
│  Anjungan  ●  Notifications                        [+ Add Target] │
│  Dashboard                                                     │
│  Servers                                                       │
│  Domains                                                       │
│  Monitoring                                                    │
│  ├─ Uptime                 ┌─────────────────────────────┐   │
│  ├─ SSL                    │ 🔵 Telegram                  │   │
│  └─ Notifications    ○ ←  │ DevOps Channel               │   │
│  Projects                  │ 📡 Scopes: [Uptime] [SSL]   │   │
│  Registry                  │ 🟢 Active                    │   │
│                             │ [Test] [Edit] [Delete]      │   │
│                             └─────────────────────────────┘   │
│                             ┌─────────────────────────────┐   │
│                             │ 🟣 Email                     │   │
│                             │ Admin Alerts                 │   │
│                             │ 📡 Scopes: [Uptime]         │   │
│                             │ 🟢 Active                    │   │
│                             │ [Test] [Edit] [Delete]      │   │
│                             └─────────────────────────────┘   │
│                             ┌─────────────────────────────┐   │
│                             │ ⚪ Webhook                   │   │
│                             │ Slack Channel                │   │
│                             │ 📡 Scopes: [Deploy Health]  │   │
│                             │ 🔴 Inactive                  │   │
│                             │ [Test] [Edit] [Delete]      │   │
│                             └─────────────────────────────┘   │
│                                                               │
│  ─── Notification Log ─────────────────────────────────────   │
│                                                               │
│  [Feature: All ▼] [Target: All ▼] [Status: All ▼]            │
│  [2026-06-01 ▼] to [2026-06-11 ▼]                            │
│                                                               │
│  │ Timestamp       │ Feature │ Title              │ Target    │
│  │─────────────────│─────────│────────────────────│───────────│
│  │ 2 min ago       │ Uptime  │ 🟢 anju-web up     │ DevOps    │
│  │ 15 min ago      │ SSL     │ 🔴 cert expired    │ Admin     │
│  │ 1 hour ago      │ Deploy  │ ✅ deploy success  │ Slack WH  │
│  └─────────────────┴─────────┴────────────────────┴───────────┘
└──────────────────────────────────────────────────────────────────┘
```

### Key UX Principles

1. **Card grid layout** — each notification target is a card, not a table row. Cards convey identity (name, type) at a glance.
2. **Visual type distinction** — Telegram (blue), Email (purple), Webhook (gray) badges so users instantly recognize type.
3. **Scope as tags** — small colored pills that show which features use this target. Clicking a scope tag could filter to that feature's targets.
4. **Inline test** — test button directly on the card, no navigation away. Result shown as toast.
5. **Active/inactive toggle** — switch without opening edit modal. Inactive cards are visually muted.
6. **Empty states** — illustration + "Add your first notification target" button when no targets exist.
7. **Notification log** — table with rich filters. Row expansion for error details. Color-coded status badges.

### Creating/Editing Modal

```
┌──────────────────────────────────────┐
│  Add Notification Target             │
│                                      │
│  Name: [_________________________]  │
│                                      │
│  Type: [Telegram ▼]                  │
│                                      │
│  ┌── Telegram Config ──────────┐    │
│  │ Chat ID: [________________] │    │
│  │ Bot Token: [_______________] │    │
│  │                             │    │
│  │ [Test Connection]           │    │
│  └─────────────────────────────┘    │
│                                      │
│  Scopes:                             │
│  ☑ Uptime                           │
│  ☑ SSL                              │
│  ☐ Deployment Health                │
│  ☐ Backup                           │
│                                      │
│  ☑ Active                            │
│                                      │
│              [Cancel]  [Save Target] │
└──────────────────────────────────────┘
```

Config fields dynamically swap based on selected type:
- **Telegram**: chat_id, bot_token (with test connection button)
- **Email**: email address(es), optional name
- **Webhook**: webhook_url, optional custom headers (key/value pairs), HTTP method (POST default)

---

## 6. Non-Functional Requirements

| Aspect | Target | Notes |
|--------|--------|-------|
| Delivery latency | < 5s | Async sending via goroutine pool |
| Retry | 3x with exponential backoff | For failed sends: 5s → 25s → 125s |
| Rate limit | 60 notif/min per target | Prevent spam / provider rate limits |
| Log retention | 90 days | Auto-purge via cron job |
| Duplicate protection | No duplicate within 60s | Same event_type + target_id + feature |
| Concurrent sends | Max 10 goroutines | Prevent resource exhaustion |
| Provider timeout | 10s per provider call | Context cancellation |
| Uptime | Part of Anjungan core | If notification engine down, features still run but alerts skipped |

---

## 7. Implementation Roadmap

### Phase 1: Foundation

| Order | Feature | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | `notification_targets` table + migration | 1 day | — |
| 2 | Notification engine: provider interface + Telegram, Email, Webhook providers | 2 days | #1 |
| 3 | CRUD endpoints (`GET/POST/PUT/DELETE /api/v1/notification-targets`) | 1 day | #2 |
| 4 | Frontend `/notifications` page — card grid, create/edit modal | 2 days | #3 |
| 5 | Test notification functionality (`POST /targets/{id}/test`) | 1 day | #4 |

### Phase 2: Feature Integration

| Order | Feature | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | Integrate Uptime Monitoring → notification targets | 1 day | Phase 1 |
| 2 | Integrate SSL Monitoring → notification targets | 1 day | Phase 1 |
| 3 | Integrate Deployment Health → notification targets | 1 day | Phase 1 |
| 4 | Notification log table + `GET /notification-logs` endpoint | 1 day | #3 |
| 5 | Notification log frontend — table view with filters | 1.5 days | #4 |
| 6 | Stats endpoint + summary cards on /notifications page | 0.5 day | #5 |

### Phase 3: Future

| Order | Feature | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | Slack / Discord provider | 1 day | Phase 1 |
| 2 | Webhook custom headers + HMAC signing | 0.5 day | Phase 1 |
| 3 | Notification template customization | 2 days | Phase 2 |
| 4 | Export notification logs (CSV) | 0.5 day | Phase 2 |
| 5 | Digest mode (batch notifications per interval) | 2 days | Phase 2 |

---

## 8. Design Decisions

### 8.1 Scoped Targets, Not Global

**Why:** Each feature needs different notification targets. Uptime alerts should go to DevOps channel, SSL expirations to Security channel, Deployment Health to Engineering channel. A single global target list would be too coarse.

**Pattern:** Each target has a `scopes TEXT[]` column. Features query targets where `scopes @> ARRAY['feature_name']`. This is PostgreSQL array containment — efficient with GIN index.

**Trade-off:** Users must explicitly assign scopes. Slightly more setup per target but gives precise control.

### 8.2 All Users Can Create Targets (Not Admin-Only)

**Why:** Anjungan is a single-user or small-team platform. Restricting notification target creation to admins creates unnecessary bottlenecks. Every user should be able to configure their own alerts.

**Pattern:** `created_by` tracks ownership but doesn't restrict reads/writes. In multi-user future, could add team-based RBAC.

**Trade-off:** No admin gatekeeping — users can create misconfigured targets. Mitigated by test-notification flow that validates before save.

### 8.3 PostgreSQL for Storage, Not Redis

**Why:** Notification logs need to be durable and auditable — 90-day retention, queryable by feature/status/date. Redis is ephemeral and不适合 persistent audit trail.

**Pattern:** All notification state (targets, logs) in PostgreSQL. Redis could be added later for rate-limit counters and dedup cache.

**Trade-off:** Higher latency on log inserts (disk I/O). Mitigated by async writes and connection pooling.

### 8.4 Provider Interface Pattern

**Why:** New notification providers (Slack, Discord, PagerDuty) should be pluggable without changing core logic.

**Pattern:** Go interface:

```go
type Provider interface {
    Name() string
    Send(ctx context.Context, config json.RawMessage, title, message string) error
    Validate(config json.RawMessage) error
}
```

Each provider registered in a global registry. The engine iterates providers by type string.

**Trade-off:** Interface abstraction adds minor complexity but enables clean extensibility.

---

## 9. Glossary

| Term | Definition |
|------|-----------|
| Target | Notification destination (Telegram chat, email address, webhook URL) |
| Scope | List of features that can use this target (e.g., `["uptime", "ssl"]`) |
| Provider | A notification channel implementation (Telegram, Email, Webhook) |
| Notification Log | Audit trail of all sent/failed notifications |
| Test Notification | A sample notification sent to verify a target is working |

---

## 10. Related Documents

- [PRD-uptime-monitoring.md](./PRD-uptime-monitoring.md) — Uptime monitoring feature, first consumer of notification engine
- [PRD-ssl-monitoring.md](./PRD-ssl-monitoring.md) — SSL monitoring feature, uses notification targets with scope `ssl`
- [PRD-repositories-deployments.md](./PRD-repositories-deployments.md) — Deployment health feature, uses notification targets with scope `deployment-health`
