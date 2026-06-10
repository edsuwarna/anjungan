# Anjungan — PRD: SSL Certificate Monitoring

> **Version:** 2.1
> **Status:** 🟢 Phase 3 Complete — Branch `feat/ssl-monitoring`
> **Author:** Endang Suwarna
> **Last Updated:** June 10, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan manages multiple servers and domains, but there is **no centralized SSL certificate expiry monitoring**. Currently:

- Domains spread across Traefik (peladen-central), Cloudflare, Vercel, and other external hosts
- No single dashboard to see **which domains are expiring, when, and how critical**
- Relies on email reminders from Let's Encrypt (only for domains on peladen-central)
- Domains on external hosts (Vercel, Cloudflare, other servers) have **zero visibility**
- Expired certs cause service outages — usually discovered when users report "site down"

### What This Solves

| Problem | Solution |
|---------|---------|
| Spread domains with no central watch | Manual entry — monitor ANY domain, anywhere |
| No expiry warning system | Dashboard badges + notification threshold (default 14 days) |
| Unknown certificate quality | Chain validation, cipher grade, OCSP revocation check |
| Manual TLS check per domain | Automated cron checks + history tracking |
| No audit of who added/removed monitors | Full audit log integration |

### Target Audience

- **Endang** (platform engineer) — know all cert status at a glance, get notified before expiry
- **DevOps** — add domains they're responsible for, monitor any external services

### Goals

| Goal | Metric |
|------|--------|
| Add domain to monitor from UI | < 10 seconds |
| Automated TLS health check | Per cron interval (default 1h) |
| Detect expiring certs | < 14 days before expiry → status change |
| Certificate quality insights | Chain valid? OCSP clean? Cipher grade? |
| Notifications via existing channels | Telegram / Discord / Slack via webhook system |
| Zero dependencies on other features | Standalone — no cluster_servers, no domains table |

### Non-Goals

- ❌ Not a replacement for Traefik / Let's Encrypt auto-renewal
- ❌ Not a full SSL certificate manager (no CSR generation, no cert installation)
- ❌ Not a domain health / uptime monitor (SSL only, not HTTP response)

---

## 2. Product Overview

### This Feature in the Context of Anjungan

```
User Dashboard
│
├── SSL Monitoring Menu (/ssl-monitors)
│   ├── Domain List → status badges, days remaining, last checked
│   ├── Add SSL Monitor → manual entry form
│   ├── Domain Detail → full cert info + check history
│   └── Check All → manual trigger batch check
│
├── Notification System (existing)
│   ├── Webhook → Telegram / Discord / Slack
│   └── Dashboard Alert → summary card
│
└── Cron Engine (backend)
    ├── CheckExpiringCerts() → periodic TLS check
    └── NotifyWatchers() → alert if < threshold
```

### Flow: Add & Monitor SSL Domain

```
User input                    Anjungan                            Internet
┌────────────────┐          ┌───────────────────────┐         ┌──────────────┐
│ Domain: X      │          │ 1. Save to DB          │         │ 4. TLS handshake│
│ Port: 443      │ ───────▶ │ 2. TLS check → parse   │ ──────▶ │ 5. Return cert  │
│ Notify: 14d    │          │ 3. Update status        │    TLS  │ 6. Chain + OCSP │
│ Interval: 1h   │          │ 7. Display in UI        │         └──────────────┘
└────────────────┘          └───────────────────────┘
```

### Key Design Decision: Manual Entry + Server-Side Discovery

This feature supports **both manual entry and server-side auto-discovery** from connected servers. Unlike the original PRD-domain-management.md §F6.4 (which relied solely on Traefik config generator), this implementation offers:

1. **Manual entry** — monitor domains on any host (Cloudflare, Vercel, external servers)
2. **Server-side discovery** — auto-detect SSL certs from connected servers (Traefik, Nginx, Caddy, Let's Encrypt, filesystem scan)
3. **Zero infra dependency** — works without cluster_servers or domains tables
4. **Immediate value** — can ship independently without waiting for Domain Management feature
5. **Complete control** — user decides which discovered domains to import

---

## 3. Feature Specifications

> **Priority Key:** P0 = Must have, P1 = Should have, P2 = Nice to have

### F1 — SSL Monitor CRUD (P0) ✅

| | |
|---|---|
| **Backend** | `ssl_monitors` table. CRUD: `GET/POST/PUT/DELETE /api/v1/ssl-monitors`. Fields: domain, port (default 443), display_name, check_interval (default "1h"), notify_before (default "14d"), webhook_ids (UUID[] → ssl_notification_targets), enabled, server_id (for discovered certs), source_provider (manual, traefik, nginx, caddy, letsencrypt). Filter: `?status=`, `?search=domain`, `?sort=`, `?order=`. **Duplicate check** domain+port unique → 409 Conflict. Paginated + "all" mode. |
| **Frontend** | Route `/ssl-monitors`. Domain list with badge status, days remaining, issuer, cipher grade, last checked, notification targets pill. "+ Add SSL Monitor" button → form. Click domain card → detail view. **Summary KPI cards**: total, valid, expiring, expired, error counts. Status filter chips. Search bar. Sortable columns. **Dashboard summary card**: "X certs expiring within 30 days". |
| **UX** | Badge color: 🟢 >30d, 🟡 7-30d, 🔴 <7d/expired, ⚪ pending/error. Duplicate check (domain+port unique). Quick action dropdown on each card: Check Now, Edit, Delete. "Check All" button for batch check. |

### F2 — TLS Certificate Check (P0) ✅

| | |
|---|---|
| **Backend** | TLS handshake to `domain:port` → extract: certificate chain (leaf, intermediate, root), expiry date, issuer (CN + org), subject (CN), SANs (Subject Alternative Names), TLS version, cipher suite, response time. **Chain validation**: build verified chain from server's leaf + intermediates, match to root CAs. **SAN coverage check**: flag if monitored domain not covered by SANs. Store all result fields. Update `status`, `cert_expires_at`, `days_remaining`, `last_check_at`, `last_error`. |
| **Frontend** | Detail page: all cert info displayed. **Certificate Chain panel**: leaf → intermediate → root with validity status per cert. **SAN list** with domain match indicator (✅ monitored domain is in SANs / ❌ mismatch). **Response time** displayed. **Cipher info**: TLS version + cipher suite. **OCSP status badge**. |
| **UX** | SAN mismatch = ❌ warning highlight. Chain error = 🟡 warning. OCSP revoked = 🔴 critical badge. Response time shown with ms unit. |

### F3 — Cipher Quality Grading (P1) ✅

| | |
|---|---|
| **Backend** | Grade based on TLS version + cipher suite: A+ (TLS 1.3), A (TLS 1.2 strong — AEAD ciphers), B (TLS 1.2 weak — CBC), C (TLS 1.1), D (TLS 1.0), F (SSLv3). Store as `cipher_grade` VARCHAR(2). |
| **Frontend** | Badge on domain card + detail: A+ 🟢, A 🟢, B 🟡, C 🟠, D 🔴, F 🔴. Tooltip explaining grade criteria. |
| **UX** | Click grade → expand detailed explanation + recommended fix. |

### F4 — Automated Scheduled Checks (P1) ✅

| | |
|---|---|
| **Backend** | `scheduler.go` — background goroutine with configurable interval (default 1h). Iterates all enabled monitors. TLS check → `saveResult()` → update monitor + insert history. **Notification dedup**: only notify on status change or when crossing threshold. `POST /api/v1/ssl-monitors/{id}/check` — manual trigger. `POST /api/v1/ssl-monitors/check-all` — batch check all enabled. |
| **Frontend** | "Last checked: X min ago" on each domain card. "Check Now" button per domain. "Check All" button in list header — sequential batch with per-domain progress. |
| **UX** | During check: spinner/pulsing status. Check All shows per-domain results inline without page reload. |

### F5 — Notification Integration (P1) ✅

| | |
|---|---|
| **Backend** | When expiration detected within threshold, dispatch to `ssl_notification_targets` (dedicated table, not registry_webhooks). Formats per-platform: Telegram HTML, Discord embed, Slack JSON. Event payload: domain, days_remaining, issuer, expires_at, cipher_grade, previous_status. Dedup: only on status change, not every check cycle. `POST /api/v1/ssl-monitors/notification-targets/{id}/test` — test notification delivery. |
| **Frontend** | In Add/Edit form: "Notify via" section — multi-select from saved notification targets. Detail page: list assigned targets with test button. Dashboard alert card for expiring certs. |
| **UX** | "Notify me via" dropdown (multi-select from configured targets). Test button sends sample notification to verify delivery. |

### F6 — Check History & Trend (P2) ✅

| | |
|---|---|
| **Backend** | `ssl_check_history` table. Each check: ssl_monitor_id, checked_at, days_remaining, status, cipher_grade, tls_version, cipher_suite, response_time_ms, error_message. `GET /api/v1/ssl-monitors/{id}/history` — paginated history (?limit=&offset=). `GET /api/v1/ssl-monitors/{id}/trend?limit=90` — last N check entries for chart. Retention: auto-purge after 90 days. |
| **Frontend** | Detail page → "Check History" panel. Timeline: each check with status + days remaining. **TrendChart component**: SVG line chart (x=date, y=days_remaining). Color gradient fill. Renewal visible as vertical jump. Expiry visible as descending line. Status tooltips on hover. |
| **UX** | Chart: x-axis = date, y-axis = days remaining. Hover tooltip: date, days remaining, status. Empty state for new monitors. |

### F7 — Notification Targets CRUD (P1) ✅

| | |
|---|---|
| **Backend** | Dedicated `ssl_notification_targets` table (separate from registry_webhooks). Full CRUD: `GET/POST /api/v1/ssl-monitors/notification-targets`, `GET/PUT/DELETE /api/v1/ssl-monitors/notification-targets/{id}`, `POST /api/v1/ssl-monitors/notification-targets/{id}/test`. Fields: name, url, platform (telegram, discord, slack, generic), webhook_secret, enabled. Platform-specific formatting: Telegram HTML payload, Discord embed, Slack message. |
| **Frontend** | Modal/popup on SSL Monitors list page: "Notification Targets" button → list, create, edit, delete targets. Test button per target. Create form: name, URL, platform selector, optional secret. |
| **UX** | Test notification button sends live sample (SSL expiry format). Platform selector shows icon per provider (Telegram, Discord, Slack, Webhook). Delete confirmation. Targets assigned to monitors via webhook_ids array. |

### F8 — Server-Side Discovery (P2) ✅

| | |
|---|---|
| **Backend** | `discovery.go` — SSH into connected server → scan for SSL certs. **Providers**: Traefik (parses acme.json v2/v3), Nginx (nginx -T or grep ssl_certificate), Caddy (storage.json + filesystem scan), Let's Encrypt (`/etc/letsencrypt/live/`), Filesystem (generic PEM scan). Parses PEM certs → expiry, issuer, SANs. `POST /api/v1/ssl-monitors/discover` — scan server by id + provider. `POST /api/v1/ssl-monitors/discover/import` — batch import discovered domains as monitors. |
| **Frontend** | **DiscoveryModal**: select server + provider (Auto, Traefik, Nginx, Caddy, LetsEncrypt). Scan button → list discovered domains with expiry, issuer, SANs. Checkbox per domain, "Import Selected" → batch add as monitors with server_id + source_provider attached. |
| **UX** | Server dropdown loads from connected servers. Auto provider tries all in order (Traefik→Nginx→Caddy→LetsEncrypt→Filesystem). First successful hit returned. Progress indicator during scan. Result table: domain, expires, issuer, provider badge. |

---

## 4. API Design

### New Endpoints

```go
// === SSL Monitors ===
GET    /api/v1/ssl-monitors                          // List all (?page=&limit=&search=&status=&sort=&order=&all=)
POST   /api/v1/ssl-monitors                          // Add new monitor
GET    /api/v1/ssl-monitors/summary                  // KPI counts: total, valid, expiring, expired, error
GET    /api/v1/ssl-monitors/export/csv                // Export all monitors as CSV
POST   /api/v1/ssl-monitors/import                   // Batch import domains (array of {domain, port, display_name})
POST   /api/v1/ssl-monitors/check-all                // Trigger check for all enabled monitors
POST   /api/v1/ssl-monitors/discover                 // Server-side discovery: {server_id, provider}
POST   /api/v1/ssl-monitors/discover/import           // Import discovered domains as monitors
GET    /api/v1/ssl-monitors/{id}                     // Detail + current cert info
PUT    /api/v1/ssl-monitors/{id}                     // Update
DELETE /api/v1/ssl-monitors/{id}                     // Remove
POST   /api/v1/ssl-monitors/{id}/check               // Manual TLS check
GET    /api/v1/ssl-monitors/{id}/history             // Paginated check history (?limit=&offset=)
GET    /api/v1/ssl-monitors/{id}/trend               // Last N check entries for chart (?limit=90)

// === SSL Notification Targets ===
GET    /api/v1/ssl-monitors/notification-targets             // List all
POST   /api/v1/ssl-monitors/notification-targets             // Create
GET    /api/v1/ssl-monitors/notification-targets/{id}        // Get
PUT    /api/v1/ssl-monitors/notification-targets/{id}        // Update
DELETE /api/v1/ssl-monitors/notification-targets/{id}        // Delete
POST   /api/v1/ssl-monitors/notification-targets/{id}/test   // Send test notification
```

### Response Format

```json
// POST /api/v1/ssl-monitors (Create)
{
  "success": true,
  "data": {
    "id": "uuid-ssl-1",
    "domain": "app1.edsuwarna.id",
    "port": 443,
    "display_name": "App 1 Production",
    "status": "pending",
    "check_interval": 3600,
    "notify_before_days": 14,
    "enabled": true,
    "webhook_ids": ["uuid-wh-1", "uuid-wh-2"],
    "created_at": "2026-06-10T10:00:00Z"
  }
}

// GET /api/v1/ssl-monitors/{id} (Detail)
{
  "success": true,
  "data": {
    "id": "uuid-ssl-1",
    "domain": "app1.edsuwarna.id",
    "port": 443,
    "display_name": "App 1 Production",
    "status": "valid",
    "cert": {
      "subject_cn": "app1.edsuwarna.id",
      "issuer_cn": "R3",
      "issuer_org": "Let's Encrypt",
      "expires_at": "2026-09-20T00:00:00Z",
      "days_remaining": 102,
      "fingerprint_sha256": "AB:CD:...",
      "sans": ["app1.edsuwarna.id", "www.app1.edsuwarna.id"],
      "san_match": true,
      "tls_version": "TLS 1.3",
      "cipher_suite": "TLS_AES_256_GCM_SHA384",
      "cipher_grade": "A+",
      "chain_status": "valid",
      "ocsp_status": "good"
    },
    "last_checked_at": "2026-06-10T10:00:00Z",
    "last_error": null,
    "check_interval": 3600,
    "notify_before_days": 14,
    "enabled": true,
    "webhook_ids": ["uuid-wh-1"],
    "created_at": "2026-06-10T10:00:00Z",
    "updated_at": "2026-06-10T10:00:00Z"
  }
}

// GET /api/v1/ssl-monitors/summary
{
  "success": true,
  "data": {
    "total": 8,
    "valid": 5,
    "expiring_soon": 2,
    "expired": 0,
    "error": 1
  }
}
```

---

## 5. Database Schema

### New Tables

```sql
-- 000024_create_ssl_monitors.up.sql
CREATE TABLE ssl_monitors (
  id TEXT PRIMARY KEY,
  domain VARCHAR(255) NOT NULL,
  port INTEGER DEFAULT 443,
  display_name VARCHAR(255),                              -- optional label
  check_interval VARCHAR(16) DEFAULT '1h',                 -- Go duration string (1h, 30m, 6h)
  notify_before VARCHAR(16) DEFAULT '14d',                 -- Go duration string (14d, 7d, 30d)
  webhook_ids TEXT[] DEFAULT '{}',                         -- ref to ssl_notification_targets
  enabled BOOLEAN DEFAULT TRUE,
  status VARCHAR(20) DEFAULT 'pending',                    -- pending, valid, expiring_soon, expired, error
  -- Certificate info
  issuer TEXT NOT NULL DEFAULT '',
  subject TEXT NOT NULL DEFAULT '',
  cert_expires_at TIMESTAMPTZ,
  days_remaining INTEGER,
  -- Chain validation
  chain_valid BOOLEAN,
  chain_error TEXT NOT NULL DEFAULT '',
  -- Cipher grade
  cipher_grade VARCHAR(2) DEFAULT '',                      -- A+, A, B, C, D, F
  cipher_error TEXT NOT NULL DEFAULT '',
  -- OCSP revocation
  ocsp_status VARCHAR(20) DEFAULT '',                      -- good, revoked, unknown
  ocsp_error TEXT NOT NULL DEFAULT '',
  -- SAN coverage
  san_names TEXT[] DEFAULT '{}',
  san_mismatch BOOLEAN DEFAULT FALSE,
  -- Server association (discovery)
  server_id TEXT,
  source_provider VARCHAR(32) DEFAULT 'manual',            -- manual, traefik, nginx, caddy, letsencrypt
  -- Timestamps
  last_status VARCHAR(20) DEFAULT 'pending',
  last_check_at TIMESTAMPTZ,
  last_error TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(domain, port)
);

-- 000025_create_ssl_check_history.up.sql
CREATE TABLE ssl_check_history (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ssl_monitor_id TEXT NOT NULL REFERENCES ssl_monitors(id) ON DELETE CASCADE,
  checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  status VARCHAR(20) NOT NULL,
  days_remaining INTEGER,
  cipher_grade VARCHAR(2),
  tls_version VARCHAR(20),
  cipher_suite VARCHAR(100),
  response_time_ms INTEGER,
  issuer TEXT NOT NULL DEFAULT '',
  subject TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_ssl_check_history_monitor_id ON ssl_check_history(ssl_monitor_id);
CREATE INDEX idx_ssl_check_history_checked_at ON ssl_check_history(checked_at);

-- 000026_create_ssl_notification_targets.up.sql
CREATE TABLE ssl_notification_targets (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  platform TEXT NOT NULL DEFAULT 'generic',                -- telegram, discord, slack, generic
  webhook_secret TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ssl_notification_targets_enabled ON ssl_notification_targets(enabled);

-- 000027_add_ssl_server_fields.up.sql
ALTER TABLE ssl_monitors
  ADD COLUMN IF NOT EXISTS server_id TEXT,
  ADD COLUMN IF NOT EXISTS source_provider VARCHAR(32) NOT NULL DEFAULT 'manual';
```

### Migration Plan

| # | Table | Description |
|---|-------|-------------|
| 000024 | `ssl_monitors` | Core monitor table |
| 000025 | `ssl_check_history` | Check result history with TLS version, cipher suite, response time |
| 000026 | `ssl_notification_targets` | Notification targets (Telegram, Discord, Slack, generic webhook) |
| 000027 | — | Add server_id + source_provider columns to ssl_monitors |

---

## 6. UX Flow

### Flow: Add SSL Monitor

```
1. Click "+ Add SSL Monitor"
2. Fill form:
   [Domain *]       app1.edsuwarna.id           → required, domain format validation
   [Port]           443                          → default 443, integer
   [Display Name]   App 1 Production             → optional, defaults to domain
   [Check Interval] Every 1 hour                 → dropdown: 30m, 1h, 6h, 12h, 24h
   [Notify Before]  14 days                      → dropdown: 7d, 14d, 21d, 30d, never
   [Notify Via]      [📢 Telegram Slack ▼]        → multi-select from existing webhooks
3. Click "Add Monitor" → saves → immediately triggers first TLS check
4. Show spinner "Checking domain..."
5. Result → status badge appears, cert info populated
```

### Flow: Dashboard View

```
┌─────────────────────────────────────────────┐
│  🔒 SSL Certificate Monitoring               │
│                                              │
│  ┌──────────── ┌──────────── ┌───────────┐   │
│  │ 🟢 ██ Valid  │ 🟡 ██ Expiring│ 🔴 ██ Expired│   │
│  │     5 certs  │     2 certs  │     0 certs│   │
│  └──────────── └──────────── └───────────┘   │
│                                              │
│  [+ Add SSL Monitor]  [🔃 Check All]         │
│                                              │
│  ┌─────────────────────────────────────┐     │
│  │ 🟢 app1.edsuwarna.id       102d     │     │
│  │    Let's Encrypt R3 · A+ · 1h ago   │     │
│  │    ⋮ (Check Now · Edit · Delete)    │     │
│  ├─────────────────────────────────────┤     │
│  │ 🟡 staging.edsuwarna.id    11d 🔔  │     │
│  │    Let's Encrypt R3 · A · 30m ago   │     │
│  │    ⋮ (Check Now · Edit · Delete)    │     │
│  └─────────────────────────────────────┘     │
└─────────────────────────────────────────────┘
```

### Flow: Domain Detail

```
┌──────────────────────────────────────────┐
│  🔙 Back to SSL Monitors                 │
│                                          │
│  🟢 app1.edsuwarna.id                    │
│     Added 10 Jun 2026 · Check 1h ago      │
│                                          │
│  ┌── Certificate Info ──────────────┐    │
│  │ Subject CN:  app1.edsuwarna.id   │    │
│  │ Issuer:      R3 · Let's Encrypt  │    │
│  │ Expires:     20 Sep 2026 (102d)  │    │
│  │ Fingerprint: AB:CD:...           │    │
│  │                                  │    │
│  │ SANs:                            │    │
│  │  ✅ app1.edsuwarna.id            │    │
│  │  ✅ www.app1.edsuwarna.id        │    │
│  └──────────────────────────────────┘    │
│                                          │
│  ┌── Cipher Quality ───────────────┐    │
│  │ Grade: A+ 🟢                     │    │
│  │ TLS:   TLS 1.3                   │    │
│  │ Suite: TLS_AES_256_GCM_SHA384    │    │
│  └──────────────────────────────────┘    │
│                                          │
│  ┌── Certificate Chain ────────────┐    │
│  │ 🟢 Leaf: app1.edsuwarna.id       │    │
│  │ 🟢 Intermediate: R3              │    │
│  │ 🟢 Root: ISRG Root X1            │    │
│  └──────────────────────────────────┘    │
│                                          │
│  ┌── OCSP Status ──────────────────┐    │
│  │ 🟢 Good (not revoked)            │    │
│  └──────────────────────────────────┘    │
│                                          │
│  │ ⋮ Check Now │ Edit │ Delete │         │
└──────────────────────────────────────────┘
```

---

## 7. Implementation Roadmap

### 🟢 Phase 1 — Core (Sprint 1)

**Goal:** Add domains, TLS check, see status in UI

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 1 | Migration 000024: `ssl_monitors` table | 0.5 day | — |
| 2 | Backend: SSL monitor CRUD (handler + service + store) | 1 day | #1 |
| 3 | Backend: TLS check engine (Go `crypto/tls` + `x509`) | 1.5 days | #2 |
| 4 | Backend: SSL summary endpoint | 0.5 day | #3 |
| 5 | Frontend: Route `/ssl-monitors` + domain list + status badges | 1 day | #2 |
| 6 | Frontend: Add/Edit SSL monitor form | 0.5 day | #5 |
| 7 | Frontend: Domain detail page (cert info, chain, cipher) | 1 day | #3, #5 |
| 8 | Frontend: Dashboard summary card | 0.5 day | #4 |
| | **Total** | **6.5 days** | |

### 🟡 Phase 2 — Monitoring & Notifications (Sprint 2)

**Goal:** Auto-check, history, notifications

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 9 | Backend: Cron job `CheckExpiringCerts()` | 1 day | #3 |
| 10 | Backend: Notification trigger via webhook system | 1 day | #9 |
| 11 | Backend: Migration 000025: `ssl_check_history` | 0.5 day | — |
| 12 | Backend: History endpoint + auto-purge | 0.5 day | #11 |
| 13 | Frontend: Check history timeline + chart | 1 day | #12 |
| 14 | Frontend: Notification config in add/edit form | 0.5 day | #10 |
| | **Total** | **4.5 days** | |

### ✅ Phase 3 — Enhancements (Complete)

| Order | Feature | Effort | Status |
|-------|---------|--------|--------|
| 15 | Cipher grade scoring | 1 day | ✅ Done in Phase 1 (checker.go) |
| 16 | OCSP stapling check | 1 day | ✅ Done in Phase 1 (checker.go) |
| 17 | Export report (CSV) | 0.5 day | ✅ Done |
| 18 | Batch import domains | 0.5 day | ✅ Done |
| 19 | **Dedicated notification targets** (ssl_notification_targets table + CRUD) | 1 day | ✅ Done |
| 20 | **Server-side discovery** (Traefik/Nginx/Caddy/LetsEncrypt/filesystem) | 2 days | ✅ Done |
| 21 | **Trend chart endpoint** (SVG-ready data for frontend) | 0.5 day | ✅ Done |
| 22 | **Platform-specific notifications** (Telegram HTML, Discord embed, Slack) | 1 day | ✅ Done |

### Total Actual Effort: ~18 days (All 3 phases)

---

## 8. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| **TLS check timeout** | < 10s per domain |
| **Concurrent checks** | goroutine pool (max 10 concurrent) |
| **Cron interval** | Configurable, default every hour |
| **History retention** | 90 days, auto-purge |
| **Response time (API)** | < 100ms for CRUD, < 1s for TLS check |
| **Duplicate prevention** | domain+port unique constraint |
| **Graceful failure** | Network error → status "error", retry next cycle |
| **Audit trail** | All CRUD operations logged to `audit_logs` |
| **Notification dedup** | Don't re-notify every check cycle — only on status change |
| **Data privacy** | No SSL private keys stored (only public cert info) |

---

## 9. Dependencies & Integration Points

| Dependency | Type | Notes |
|------------|------|-------|
| Go `crypto/tls` | stdlib | TLS handshake |
| Go `crypto/x509` | stdlib | Cert parsing + chain validation |
| Go `net` | stdlib | Dial timeout |
| `audit_logs` table | existing | Audit CRUD actions |
| `ssl_notification_targets` | Dedicated SSL notification targets | ✅ 000026 |
| `users` table | existing | `created_by` FK |
| Frontend route layout | existing | Sidebar navigation, dark/light mode |

---

## 10. Edge Cases & Error Handling

| Scenario | Behavior |
|----------|----------|
| **Domain unreachable** | Status = "error", `last_error` = "connection refused/timeout" |
| **DNS not resolving** | Status = "error", `last_error` = "no such host" |
| **Cert expired** | Status = "expired", `days_remaining` negative |
| **Self-signed cert** | Chain validation = "invalid", but still show cert info |
| **Cert SAN mismatch** | `san_match = false`, UI warning highlight |
| **OCSP responder unavailable** | `ocsp_status = "unknown"`, not treated as error |
| **Port not SSL (plain HTTP)** | Status = "error", `last_error` = "handshake failure" |
| **Wildcard domain** | Show SAN: `*.edsuwarna.id`, warn if monitor domain doesn't match |
| **Rate limiting** | Max 1 check per domain per 30s (prevent hammering) |
| **Duplicate entry** | Return 409 Conflict — "domain:port already monitored" |

---

## 11. Mockup References

UI mockups created for this feature are available in [`sketches/ssl-monitoring/`](../sketches/ssl-monitoring/):

| Screenshot | Description |
|------------|-------------|
| [`ssl-monitor-list.png`](../sketches/ssl-monitoring/ssl-monitor-list.png) | Main dashboard — KPI cards, filter chips, domain list with status badges, expiry countdown, cipher grade, SAN/Chain/OCSP indicators |
| [`ssl-monitor-add.png`](../sketches/ssl-monitoring/ssl-monitor-add.png) | Add SSL Monitor form — domain, port, display name, check interval, notify threshold, notif channel |
| [`ssl-monitor-detail.png`](../sketches/ssl-monitoring/ssl-monitor-detail.png) | Domain detail — cert info, certificate chain (leaf → intermediate → root), cipher quality grade, OCSP status, check history mini chart, settings |

### Mockup Views

![SSL Monitor List](../sketches/ssl-monitoring/ssl-monitor-list.png)

![Add Monitor](../sketches/ssl-monitoring/ssl-monitor-add.png)

![Domain Detail](../sketches/ssl-monitoring/ssl-monitor-detail.png)

---

## 12. Future Considerations

| Feature | Trigger |
|---------|---------|
| **Auto-import from Traefik** | When Domain Management (F6) is implemented, offer "Import from Traefik" button |
| **Bulk import** | CSV upload for adding many domains at once |
| **Certificate transparency log** | Monitor CT logs for issued certs on monitored domains |
| **Public monitoring page** | Read-only status page (like Uptime Kuma public page) |
| **DNS validation** | Check if DNS resolves before/after expiry |
| **Multi-port** | Monitor multiple ports on same domain (443, 8443, etc.) |
| **Expiry calendar view** | Calendar grid showing all cert expiry dates |
| **Slack DM notification** | Direct message vs channel notification |

---

## 13. PRD Cross-References

| Document | Relation |
|----------|----------|
| `PRD-domain-management.md §F6.4` | Original SSL monitoring spec (replaced by standalone PRD) |
| `PRD-registry.md §F7` | Webhook notification system (inspiration, but SSL uses dedicated notification targets) |
| `TRACKING.md` | Cross-references SSL monitoring feature status |

---

## 14. References

- [Go crypto/tls](https://pkg.go.dev/crypto/tls) — TLS handshake
- [Go crypto/x509](https://pkg.go.dev/crypto/x509) — Certificate parsing
- [Uptime Kuma SSL Certificate](https://github.com/louislam/uptime-kuma) — Reference UX
- [SSLyze](https://github.com/nabla-c0d3/sslyze) — Cipher grading inspiration
- [Let's Encrypt Chain of Trust](https://letsencrypt.org/certificates/) — Chain validation reference
