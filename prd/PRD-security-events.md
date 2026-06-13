# Anjungan — PRD: Security Events & Threat Detection

> **Version:** 1.1
> **Status:** 🟡 Partially Implemented — Brute force detection for Anjungan login implemented in auth-activity (v0.14.0). CrowdSec integration, full Security Events dashboard, and threat intel still 🔴 Not Implemented.
> **Author:** Endang Suwarna
> **Last Updated:** June 13, 2026

---

## ⚡ Implemented: Brute Force Detection (auth-activity)

**As of v0.14.0, the brute force detection for Anjungan login is implemented** via the `auth-activity` module. This covers:

- **Brute force detection** — threshold-based (> 20 failures from same IP in 5 min) → security event + notification
- **IP blocking** — block/unblock IPs from dashboard (Redis + DB persistent)
- **Lockout monitoring** — see locked accounts + remaining lockout time
- **Notification** — brute force alerts sent via notification targets

The CrowdSec integration, full Security Events dashboard, and threat intelligence features remain as future work as described below.

---

## 1. Executive Summary

### Problem Statement

Anjungan already monitors SSL certs and uptime, but there is **no visibility into security events** targeting infrastructure. Currently:

- SSH brute-force attacks happen daily but go unnoticed until a server is compromised
- Container escape attempts, port scans, and web app attacks produce no alert
- No centralized "security incident" view — events are scattered across server logs, Docker logs, and Traefik access logs
- No correlation between events — an attack on Server A today might target Server B tomorrow
- Existing compliance scanning (CIS/Lynis) is preventive/audit, not real-time detection

Without security event visibility:
- **Blind to active attacks** — don't know if you're being targeted right now
- **No alerting** — breaches detected days later, or never
- **No threat intel** — IPs attacking you are forgotten after the event
- **Reactive posture** — fix after breach, not prevent during attack

### What This Solves

| Problem | Solution |
|---------|---------|
| SSH brute force attacking servers | **Fail2Ban + CrowdSec integration** — detect + block + alert |
| Web app attacks (path traversal, SQLi, XSS) | **CrowdSec Traefik bouncer** — WAF-level detection |
| Port scans against infrastructure | **CrowdSec network scan detection** |
| No centralized event timeline | **Security Events dashboard** — all detections in one view |
| No attack correlation | **Aggregated threat intelligence** — IP reputation, attack trends |
| Manual log digging for incidents | **One-click event investigation** — context around each event |

### Current Status

| Aspect | Status |
|--------|--------|
| Brute force detection (Anjungan login) | ✅ Implemented — auth-activity module (v0.14.0) |
| IP blocking (Anjungan login) | ✅ Implemented — Redis + DB persistent |
| CrowdSec deployed on VPS | ❌ Not deployed |
| CrowdSec Traefik bouncer | ❌ Not configured |
| CrowdSec API integration | ❌ Not implemented |
| Security Events dashboard | ❌ Not implemented |
| Fail2ban log parsing | ❌ Not implemented |
| Threat Intel / IP reputation | 🟡 Partial — blocked IPs via auth-activity |
| Incident timeline | ❌ Not implemented |

### Target Audience

- **Endang** (platform engineer) — know "is my infrastructure under attack right now?"
- **DevOps** — investigate security events, block malicious IPs
- **Security-conscious teams** — satisfy monitoring requirements

### Goals

| Goal | Metric |
|------|--------|
| Real-time attack detection (Anjungan login) | ✅ Implemented via auth-activity brute force detection |
| IP blocking with context | ✅ Blocked IP + reason + timestamp (auth-activity) |
| Notification on brute force | ✅ Via notification targets |
| Centralized event dashboard (CrowdSec) | ❌ Not implemented |
| Attack trend visualization (full) | 🟡 Partial — login trend only (auth-activity) |
| Zero false positives | ✅ Only confirmed events |
| CrowdSec integration | ❌ Not implemented |

### Non-Goals

- ❌ Not a full SIEM solution (no complex query language, no long-term log storage)
- ❌ Not replacing CrowdSec itself — Anjungan consumes CrowdSec decisions
- ❌ Not a vulnerability scanner — see PRD-container-image-scanning.md
- ❌ Not real-time network packet inspection

---

## 2. Product Overview

### Architecture

```
                          Anjungan Backend
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│  Security Events Handler                                        │
│  ┌────────────────────────────┐    ┌─────────────────────────┐  │
│  │ GET /api/security/events   │    │ Threat Intel Engine      │  │
│  │ GET /api/security/summary  │    │ ├─ Aggregate IPs         │  │
│  │ GET /api/security/blocked  │    │ ├─ Frequency analysis    │  │
│  │ POST /api/security/sync    │    │ └─ Trending detection    │  │
│  └────────────────────────────┘    └─────────────────────────┘  │
│               │                              │                   │
│               ▼                              ▼                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  Data Sources                                               │  │
│  │  ┌────────────────┐  ┌──────────────┐  ┌────────────────┐  │  │
│  │  │ CrowdSec API   │  │ Fail2Ban DB  │  │ Server Logs    │  │  │
│  │  │ REST / LAPI    │  │ /var/log/    │  │ (future)       │  │  │
│  │  │ decisions      │  │ fail2ban.log │  │                │  │  │
│  │  │ alerts         │  │              │  │                │  │  │
│  │  └───────┬────────┘  └──────┬───────┘  └───────┬────────┘  │  │
│  └──────────┼──────────────────┼───────────────────┼──────────┘  │
│             │                  │                   │              │
│             ▼                  ▼                   ▼              │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  DB: security_events, blocked_ips, threat_intel             │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
└──────────────────────────┬───────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────────┐
│  Frontend (SvelteKit)                                            │
│  ┌────────────────┐  ┌──────────────┐  ┌────────────────────┐  │
│  │ Security Events│  │ Blocked IPs  │  │ Threat Intel       │  │
│  │ Timeline       │  │ Table        │  │ Dashboard          │  │
│  └────────────────┘  └──────────────┘  └────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### Integration with CrowdSec

CrowdSec provides a **Local API (LAPI)** endpoint that exposes:
- **Decisions** — IPs currently blocked, with reason and duration
- **Alerts** — security events with full context (timestamp, source, scenario, meta)
- **Metrics** — global statistics (top IPs, top scenarios, event count)

Anjungan queries CrowdSec LAPI on a cron interval (default: 60s) and stores results in its own database for dashboarding, history, and cross-referencing with other Anjungan data (servers, containers, uptime).

### Data Flow

```
Attacker ──► Server (SSH/HTTP)
                │
                ▼
        ┌───────────────┐
        │ CrowdSec      │──► Block + Alert
        │ Agent +       │       │
        │ Bouncer       │       │
        └───────────────┘       │
                                ▼
                        ┌───────────────┐
                        │ CrowdSec LAPI │
                        │ (local API)   │
                        └───────┬───────┘
                                │ GET /v1/alerts?limit=100
                                │ GET /v1/decisions
                                ▼
                        ┌───────────────┐
                        │ Anjungan      │
                        │ Sync Cron     │──► Store in DB
                        │ (every 60s)   │──► Update dashboard
                        └───────────────┘──► Trigger notifications
```

---

## 3. Feature Specifications

### F1: Security Events Timeline

A scrollable, real-time timeline of all security events detected across infrastructure.

**Fields per event:**
- Timestamp (when detected)
- Source IP (attacker)
- Scenario (e.g., `crowdsec/ssh-bf`, `crowdsec/http-path-traversal`)
- Target (server hostname, service)
- Status (active/blocked/expired)
- Action taken (blocked by bouncer, logged only)
- Country / ASN (from MaxMind GeoIP or CrowdSec enrichment)
- Event count (how many times same IP hit same scenario)

**Views:**
- **Timeline** — newest first, paginated
- **By server** — filter events per managed server
- **By scenario** — group by attack type
- **By severity** — critical/medium/info

### F2: Blocked IPs Dashboard

Table of currently blocked IPs with context.

**Columns:**
- IP address
- Scenario (reason for block)
- Duration (remaining block time)
- First seen
- Last seen
- Total events
- Country
- Actions: unblock (via CrowdSec API)

**Enhancements:**
- Search by IP
- Export blocked IPs list (for firewall rules)
- History of previously blocked IPs

### F3: Threat Intelligence Summary

High-level dashboard cards showing:

- **Attacks Today** — total security events in last 24h
- **Currently Blocked** — active IPs in CrowdSec decision list
- **Top Attacker IP** — IP with most events this week
- **Top Attack Type** — most frequent scenario
- **Attack Trend** — 7-day sparkline (events/day)

### F4: Notification Integration

Trigger notifications via shared notification targets when:

- **Attack spike detected** — > 100 events in 10 minutes (configurable threshold)
- **Known bad actor** — IP from known threat feed hits a server
- **New attack type** — first occurrence of a scenario seen
- **Block exhausted** — IP that was blocked resumes attacking after unblock

### F5: Manual Sync / Refresh

- Button to trigger immediate sync from CrowdSec LAPI
- Status indicator showing last sync time and freshness
- Connection health check (CrowdSec LAPI reachable?)

---

## 4. API Design

### REST Endpoints

```
GET    /api/security/events           — List security events (paginated, filterable)
GET    /api/security/events/:id       — Single event detail
GET    /api/security/blocked          — Currently blocked IPs
POST   /api/security/blocked/:ip/unblock — Unblock IP via CrowdSec
GET    /api/security/summary          — Dashboard summary cards
POST   /api/security/sync             — Manual trigger CrowdSec sync
GET    /api/security/stats            — Aggregated statistics (7d, 30d)
GET    /api/security/health           — CrowdSec connection status
```

### Query Parameters (GET /events)

| Param | Type | Description |
|-------|------|-------------|
| `source` | string | Filter by data source (crowdsec, fail2ban) |
| `scenario` | string | Filter by attack scenario |
| `ip` | string | Filter by source IP |
| `server_id` | string | Filter by target server |
| `status` | string | Filter by event status |
| `since` | ISO8601 | Events after timestamp |
| `until` | ISO8601 | Events before timestamp |
| `limit` | int | Page size (default 50, max 200) |
| `offset` | int | Pagination offset |

### Response Shape (GET /events)

```json
{
  "events": [
    {
      "id": "evt_01j2xyz...",
      "source": "crowdsec",
      "scenario": "crowdsec/ssh-bf",
      "source_ip": "185.220.101.x",
      "target_host": "server-01",
      "target_service": "ssh",
      "action": "block",
      "status": "active",
      "country": "RU",
      "asn": "AS12345",
      "event_count": 47,
      "first_seen": "2026-06-11T08:00:00Z",
      "last_seen": "2026-06-11T09:30:00Z",
      "block_expires_at": "2026-06-12T09:30:00Z",
      "crowdsec_alert_id": 12345
    }
  ],
  "total": 234,
  "limit": 50,
  "offset": 0
}
```

---

## 5. Database Schema

```sql
-- Security events ingested from CrowdSec / other sources
CREATE TABLE IF NOT EXISTS security_events (
    id              TEXT PRIMARY KEY,
    source          TEXT NOT NULL DEFAULT 'crowdsec',    -- crowdsec, fail2ban, manual
    source_id       TEXT NOT NULL DEFAULT '',            -- CrowdSec alert ID
    scenario        TEXT NOT NULL,                       -- e.g. 'crowdsec/ssh-bf'
    source_ip       TEXT NOT NULL,
    source_asn      TEXT NOT NULL DEFAULT '',
    source_country  TEXT NOT NULL DEFAULT '',
    target_host     TEXT NOT NULL DEFAULT '',
    target_service  TEXT NOT NULL DEFAULT '',            -- ssh, http, https, unknown
    action          TEXT NOT NULL DEFAULT 'block',       -- block, log, captcha
    status          TEXT NOT NULL DEFAULT 'active',      -- active, expired, manual_unblock
    event_count     INTEGER NOT NULL DEFAULT 1,
    first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    block_expires_at TIMESTAMPTZ,
    raw_alert       JSONB,                               -- original CrowdSec alert payload
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_security_events_source_ip ON security_events(source_ip);
CREATE INDEX idx_security_events_scenario ON security_events(scenario);
CREATE INDEX idx_security_events_status ON security_events(status);
CREATE INDEX idx_security_events_last_seen ON security_events(last_seen);
CREATE INDEX idx_security_events_source ON security_events(source);

-- IP reputation / threat intel (aggregated from events)
CREATE TABLE IF NOT EXISTS threat_intel (
    id              TEXT PRIMARY KEY,
    ip              TEXT NOT NULL UNIQUE,
    country         TEXT NOT NULL DEFAULT '',
    asn             TEXT NOT NULL DEFAULT '',
    first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    total_events    INTEGER NOT NULL DEFAULT 0,
    scenarios       TEXT[] NOT NULL DEFAULT '{}',        -- unique scenarios this IP triggered
    target_hosts    TEXT[] NOT NULL DEFAULT '{}',        -- servers this IP attacked
    is_blocked      BOOLEAN NOT NULL DEFAULT FALSE,
    block_expires_at TIMESTAMPTZ,
    tags            TEXT[] NOT NULL DEFAULT '{}',        -- 'scanner', 'brute-forcer', 'known-bad'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 6. UX Flow

### Sidebar Placement

```
Security                          (new category — or under existing Security)
├── Security Events               (new — main timeline)
├── Threat Intel                  (new — IP reputation, attack stats)
├── SSL Monitors                  (existing)
└── Compliance                    (existing)
```

### Page: Security Events

```
┌─────────────────────────────────────────────────────────────┐
│  Security Events                            [Sync Now] 🔄   │
│  Last synced: 30s ago  •  CrowdSec: ✅ Connected            │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐  │
│  │ ⚠ 147     │ │ 🔒 23    │ │ 🌐 RU     │ │ 📈 +12%     │  │
│  │ Attacks   │ │ Blocked  │ │ Top      │ │ vs yesterday│  │
│  │ Today     │ │ IPs      │ │ Source   │ │              │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────────┘  │
│                                                             │
│  Filters: [All Scenarios ▼] [All Servers ▼] [Active ▼]     │
│                                                             │
│  ┌────────────────────────────────────────────────────┐     │
│  │ 09:32  🔴 SSH Brute Force  185.220.101.x     RU   │     │
│  │        › server-01:22  •  47 attempts  •  Blocked  │     │
│  ├────────────────────────────────────────────────────┤     │
│  │ 09:15  🟡 HTTP Path Trav  91.240.118.x      DE    │     │
│  │        › server-02:443  •  12 attempts  •  Logged  │     │
│  ├────────────────────────────────────────────────────┤     │
│  │ 08:50  🔴 SSH Brute Force  103.235.46.x     CN    │     │
│  │        › server-01:22  •  89 attempts  •  Blocked  │     │
│  └────────────────────────────────────────────────────┘     │
│                                                             │
│  [← Prev]  Page 1 of 12  [Next →]                          │
└─────────────────────────────────────────────────────────────┘
```

### Page: Threat Intel

```
┌─────────────────────────────────────────────────────────────┐
│  Threat Intelligence                                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  Top Attacker IPs (7 days)                │
│  │ IP          │  Events  Country  Status   Action         │
│  │─────────────│───────────────────────────────────         │
│  │ 185.220..   │  1,247   🇷🇺 RU  Blocked   [Unblock]      │
│  │ 103.235..   │    892   🇨🇳 CN  Blocked   [Unblock]      │
│  │ 91.240..    │    445   🇩🇪 DE  Logged    [Block]        │
│  │ 45.33..     │    201   🇺🇸 US  Blocked   [Unblock]      │
│  └─────────────┘                                           │
│                                                             │
│  ┌─────────────────────────────────────────────────┐       │
│  │ Attack Trend (7 days)                           │       │
│  │  ██  ████  ██  ██████  ███  ██████  ████       │       │
│  │  Mon  Tue   Wed  Thu    Fri   Sat     Sun       │       │
│  └─────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────┘
```

---

## 7. Implementation Roadmap

### Phase 1: Backend Foundation (3-4 days)

| Task | Effort | Depends On |
|------|--------|-----------|
| CrowdSec LAPI client (Go) — query alerts + decisions | 1d | CrowdSec deployed on VPS |
| Sync cron engine — periodic fetch + store | 1d | LAPI client |
| DB migrations — security_events + threat_intel tables | 0.5d | — |
| Core REST endpoints (events, blocked, summary) | 1d | DB + sync engine |
| Manual sync + health check endpoints | 0.5d | Core endpoints |

### Phase 2: Frontend (2-3 days)

| Task | Effort | Depends On |
|------|--------|-----------|
| Security Events timeline page | 1d | API ready |
| Blocked IPs table + unblock action | 0.5d | Timeline page |
| Summary cards + filters | 0.5d | Timeline page |
| Threat Intel page (top IPs, trends) | 1d | Summary cards |

### Phase 3: Notifications & Polish (1-2 days)

| Task | Effort | Depends On |
|------|--------|-----------|
| Notification triggers (spike, new scenario) | 1d | Sync engine + notification targets |
| GeoIP enrichment (MaxMind DB) | 0.5d | — |
| Auto-block from Anjungan UI | 0.5d | Blocked IPs page |

**Total:** ~6-9 days

---

## 8. Non-Functional Requirements

| Category | Requirement |
|----------|-------------|
| **Latency** | Events visible in dashboard within 60s of CrowdSec detection |
| **Reliability** | Sync failure → log error, retry on next interval, never drop events |
| **Scalability** | Handle 10K+ events/day per server without performance degradation |
| **Security** | CrowdSec LAPI communication over localhost only (same host) |
| **Resource** | Sync cron uses < 50MB RAM, < 5% CPU per tick |
| **Retention** | Events > 90 days auto-purged; blocked IPs retained indefinitely |
| **Auth** | All endpoints require authenticated session; admin-only for unblock action |

---

## 9. Dependencies & Integration Points

| Dependency | Type | Purpose |
|------------|------|---------|
| **CrowdSec** | External service | Source of security events (must be deployed on VPS) |
| **CrowdSec LAPI** | REST API | Query alerts + decisions (localhost only) |
| **CrowdSec Traefik Bouncer** | External plugin | Blocks malicious HTTP traffic at edge |
| **CrowdSec SSH Bouncer** | External service | Blocks malicious SSH connections |
| **Notification targets** | Existing feature | Send alerts for critical events |
| **Server list** (cluster_servers) | Existing table | Link events to managed servers |
| **Audit log** | Existing feature | Log unblock actions, manual interventions |

### Integration Points with Other Anjungan Features

| Feature | Integration |
|---------|------------|
| **Uptime Monitoring** | Show security events that coincide with downtime (DDoS-related?) |
| **SSL Monitors** | Correlate failed SSL checks with MITM-style attacks |
| **Notifications** | Reuse shared notification targets for security alerts |
| **Incidents Timeline** (future) | Security events feed into incident correlation engine |
| **Containers page** | Flag containers under active attack |

---

## 10. Edge Cases & Error Handling

| Scenario | Behavior |
|----------|----------|
| CrowdSec not installed / LAPI down | Health check returns "disconnected", dashboard shows stale data warning, no error spam |
| CrowdSec LAPI returns empty | Normal — no attacks = good. Show "No events — all clear" instead of empty table |
| Same IP appears in multiple alerts | Deduplicate on IP + scenario within 5-min window; increment event_count |
| CrowdSec LAPI returns error (5xx) | Retry 3x with exponential backoff, then skip tick and log warning |
| Block expired while still in DB | Cron auto-updates status to `expired` based on block_expires_at |
| Huge burst of events (DDoS) | Batch insert; limit API response to 500 most recent; aggregation layer normalizes |
| CrowdSec bouncer removes block early | Next sync detects IP no longer in decision list → mark event as `expired` |
| User unblocks an IP | Call CrowdSec LAPI `DELETE /v1/decisions`, log to audit_log, update status |

---

## 11. Mockup References

- See `sketches/security-events/` for wireframes
- Reference: CrowdSec console UI (https://app.crowdsec.net) for layout inspiration
- KPI cards style consistent with existing Anjungan dashboard cards

---

## 12. Future Considerations

| Feature | Priority | Notes |
|---------|----------|-------|
| **Fail2Ban log parser** | P2 | Secondary source for servers without CrowdSec |
| **GeoIP block map** | P3 | Visual map showing attacker origin locations |
| **Attack playbook automation** | P3 | Auto-create firewall rules, isolate containers |
| **CrowdSec blocklist export** | P3 | Export blocked IPs as firewall-compatible list |
| **Machine learning anomaly** | P4 | Baseline normal traffic → flag anomalies |
| **Multi-server CrowdSec Central API** | P4 | Central API aggregating multiple CrowdSec instances |
| **Integration with Incidents Timeline** | P3 | Push events into incident correlation (future PRD) |

---

## 13. PRD Cross-References

| PRD | Relationship |
|-----|-------------|
| **PRD-login-activity.md** | Brute force detection for Anjungan login implemented in auth-activity module (v0.14.0) |
| **PRD-uptime-monitoring.md** | Uptime events + security events → shared timeline |
| **PRD-ssl-monitoring.md** | SSL errors + security events → certificate attack detection |
| **PRD-container-image-scanning.md** | CVE in containers + active attack → critical combo |
| **PRD-compliance.md** | Compliance failures + security events → targeted attack surface |

---

## 14. References

- CrowdSec LAPI Documentation: https://doc.crowdsec.net/docs/references/lapi/
- CrowdSec Scenarios List: https://hub.crowdsec.net/
- MaxMind GeoLite2 Free Database: https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
- Traefik CrowdSec Bouncer Plugin: https://plugins.traefik.io/plugins/628c3e21f392e4634ae241d9/crowdsec-bouncer
