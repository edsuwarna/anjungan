# Anjungan — PRD: Login Activity & Auth Security Monitoring

> **Version:** 1.1
> **Status:** ✅ Implemented — v0.14.0 (Login Activity, Brute Force, Lockouts, GeoIP, Trend Charts, CSV Export)
> **Author:** Endang Suwarna
> **Last Updated:** June 13, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan has authentication (login, TOTP, role-based access), but there is **no visibility into who is trying to log in and whether those attempts are legitimate**. Currently:

- Failed login attempts are recorded in the audit log but have **no dedicated dashboard**
- Brute-force attacks against Anjungan user accounts go undetected until a user reports "I can't log in"
- Rate limiting and lockout mechanisms exist but **no one sees them trigger**
- No pattern analysis — "is someone systematically trying all usernames?"
- No geographic or IP-based context on login attempts
- No alerting when multiple accounts are targeted simultaneously

Without auth security monitoring:
- **Blind to credential attacks** — don't know if Anjungan itself is under brute force
- **No proactive alerts** — only reactive when accounts are actually compromised
- **No forensic data** — can't investigate "who accessed what and when" beyond basic audit log
- **No lockout visibility** — users locked out but no one monitors lockout patterns

### What This Solves

| Problem | Solution |
|---------|----------|
| No visibility into failed logins | **Login Activity dashboard** — success/failure timeline with context |
| Brute force against Anjungan users | **Brute force detection** — threshold-based alerts + IP blocking |
| Account lockout blind spot | **Lockout monitoring** — who, how many times, from where |
| No geographic context on auth events | **Geo IP enrichment** — country, ASN per login attempt |
| No trend analysis | **Login trend charts** — daily auth attempts, success rate, top IPs |
| Manual audit log browsing | **Dedicated auth security page** — filtered, searchable, paginated |

### Current Status

| Aspect | Status |
|--------|--------|
| Audit log (generic) | ✅ Available — records auth events with dedicated auth_events table |
| Rate limiting (Redis) | ✅ Implemented — 5 failed attempts → backoff |
| Account lockout | ✅ Implemented — 10 failed attempts → locked 15 min |
| TOTP 2FA | ✅ Implemented — PRD-totp-2fa.md |
| Login Activity dashboard | ✅ Implemented — v0.14.0 |
| Brute force alerting | ✅ Implemented — threshold-based detection + notification |
| GeoIP on login events | ✅ Implemented — MaxMind GeoLite2 |
| IP blocking | ✅ Implemented — Redis + DB persistent block/unblock |
| CSV export | ✅ Implemented — /events/export endpoint |
| Hourly heatmap | ✅ Implemented — /heatmap endpoint |
| Self-service login history | ✅ Implemented — /events/mine, last 20 events |

### Target Audience

- **Endang** (platform engineer) — know if Anjungan itself is under attack
- **Admins** — monitor user login health, investigate failed access
- **Users** — see their own login history (limited scope)

### Goals

| Goal | Metric |
|------|--------|
| Login attempts visible in dedicated dashboard | ✅ All auth events (success + failure) |
| Brute force detection | ✅ > 20 failures from same IP in 5 min → alert/notification |
| Account lockout visibility | ✅ See locked accounts + remaining lockout time + unlock action |
| GeoIP enrichment on auth events | ✅ Country + ASN + ISP per login IP |
| Login trend (7d/30d) | ✅ Daily auth chart with success rate |
| Self-service login history | ✅ Users see last 20 of their own logins |
| CSV export | ✅ Export auth events for security audit |
| IP blocking | ✅ Block/unblock IPs from dashboard (Redis + DB) |
| Brute force config | ✅ Configurable threshold, window, notification targets |
| Hourly heatmap | ✅ Hourly distribution of auth events |

### Non-Goals

- ❌ Not replacing the audit log — this is a focused view on auth events only
- ❌ Not implementing user-session management (view active sessions, force logout) — future
- ❌ Not implementing IP block list at the Anjungan app level — handled by CrowdSec/edge
- ❌ Not a full SIEM auth module

---

## 2. Product Overview

### Architecture

```
                       Anjungan Backend
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  Login Handler                   Auth Security Handler       │
│  ┌─────────────────────┐        ┌─────────────────────────┐ │
│  │ POST /api/login     │        │ GET  /api/auth/activity │ │
│  │ POST /api/logout    │        │ GET  /api/auth/summary  │ │
│  │ POST /api/register  │        │ GET  /api/auth/lockouts │ │
│  └─────────┬───────────┘        │ POST /api/auth/unlock   │ │
│            │                    │ GET  /api/auth/trend    │ │
│            ▼                    └───────────┬─────────────┘ │
│  ┌─────────────────────┐                    │               │
│  │ Auth Event Recorder │◄───────────────────┘               │
│  │ ├─ record login     │                                    │
│  │ ├─ record failure   │       ┌─────────────────────────┐  │
│  │ ├─ record lockout   │       │ GeoIP Enrichment        │  │
│  │ └─ record unlock    │       │ (MaxMind GeoLite2 DB)   │  │
│  └─────────┬───────────┘       └───────────┬─────────────┘  │
│            │                               │                 │
│            ▼                               ▼                 │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  DB: auth_events table (dedicated, not generic audit)   │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│  Frontend (SvelteKit)                                        │
│  ┌─────────────────┐  ┌──────────────┐  ┌────────────────┐  │
│  │ Login Activity   │  │ Lockouts     │  │ My Login       │  │
│  │ (admin)          │  │ (admin)      │  │ History (user) │  │
│  └─────────────────┘  └──────────────┘  └────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

### Design Decision: Dedicated auth_events Table

Instead of querying the generic audit_log table (which mixes all types of events), auth events get their own optimized table. Rationale:
- Audit log has mixed schema — querying auth events requires filtering by action type
- auth_events table has dedicated fields: IP, country, user_agent, auth_method, failure_reason
- Better performance for auth-specific queries (trends, aggregation, heatmaps)
- Audit log keeps a copy of critical auth events for compliance (redundant but acceptable)

---

## 3. Feature Specifications

### F1: Login Activity Dashboard (Admin)

**Table view** — all auth events with columns:
- Timestamp
- User (email)
- IP address
- Country / ASN (from GeoIP)
- Event type (login_success, login_failure, lockout, unlock, logout)
- Failure reason (invalid_password, account_locked, rate_limited, totp_failed)
- User agent (browser fingerprint)
- Auth method (password, totp, session)

**Filters:**
- Date range (default: last 24h)
- Event type (success, failure, lockout)
- User (search by email)
- IP (search by IP)
- Country

### F2: Summary Cards

- **Logins Today** — total login attempts (success + failure)
- **Failed Logins** — count of failed attempts in period
- **Locked Accounts** — currently locked users
- **Unique IPs** — distinct IPs that attempted login
- **Success Rate** — percentage of successful logins

### F3: Brute Force Detection

Backend cron checks every 60s:
- If IP has > 20 failed attempts in 5 min → create security event
- If IP targets > 5 different usernames in 10 min → create security event (credential stuffing)
- Events are stored in the security_events table (see PRD-security-events.md)

### F4: Login Trend Charts

- **Daily auth chart** — bar chart: success (green) vs failure (red) per day
- **Hourly heatmap** — which hours have most failed attempts
- **Top IPs** — IPs with most failed attempts (with block/unblock action)
- **Top users targeted** — users with most failed login attempts

### F5: Self-Service Login History (User)

Users can see their own last 20 login events:
- Timestamp
- IP (masked: `185.220.***.***`)
- Country
- Success/failure
- Device/Browser

### F6: Manual Unlock

Admin can unlock a locked account directly from the dashboard (reuses existing `/admin/users/{id}/unlock` endpoint).

---

## 4. API Design

### REST Endpoints

```
GET    /api/v1/auth-activity/events             — Auth events (paginated, filterable)
GET    /api/v1/auth-activity/events/mine        — Current user's own login history (last 20)
GET    /api/v1/auth-activity/events/export      — CSV export for audit
GET    /api/v1/auth-activity/summary             — Dashboard summary cards
GET    /api/v1/auth-activity/lockouts            — Currently locked accounts
GET    /api/v1/auth-activity/trend               — Aggregated daily stats for charts
GET    /api/v1/auth-activity/brute-force         — Brute force detection results
GET    /api/v1/auth-activity/top-ips             — IPs with most failures
GET    /api/v1/auth-activity/top-users           — Users with most failures
GET    /api/v1/auth-activity/heatmap             — Hourly auth event distribution
POST   /api/v1/auth-activity/block-ip            — Block an IP address
POST   /api/v1/auth-activity/unblock-ip          — Unblock an IP address
GET    /api/v1/auth-activity/blocked-ips         — List blocked IPs
GET    /api/v1/auth-activity/config              — Brute force notification config
PUT    /api/v1/auth-activity/config              — Update brute force config

### Response Shape (GET /auth-activity/events)

```json
{
  "events": [
    {
      "id": "aev_01j2...",
      "user_id": "usr_xxx",
      "email": "admin@example.com",
      "event_type": "login_failure",
      "status": "failure",
      "failure_reason": "invalid_password",
      "ip_address": "185.220.101.23",
      "ip_obfuscated": "185.220.***.***",
      "country": "RU",
      "asn": "AS12345",
      "isp": "Example ISP",
      "user_agent": "Mozilla/5.0 ...",
      "auth_method": "password",
      "metadata": {},
      "created_at": "2026-06-11T08:30:00Z"
    }
  ],
  "_meta": {
    "total": 543,
    "page": 1,
    "per_page": 50,
    "total_pages": 11
  }
}
```

### Response Shape (GET /auth-activity/summary)

```json
{
  "today_logins": 128,
  "today_failures": 47,
  "today_success_rate": 63.3,
  "today_lockouts": 0,
  "unique_ips": 24,
  "blocked_ips_count": 2,
  "active_brute_force_alerts": 2,
  "trend_7d": {
    "dates": ["2026-06-05", "2026-06-06", ...],
    "success": [95, 102, 88, ...],
    "failure": [12, 8, 145, ...]
  }
}
```

---

## 5. Database Schema

```sql
CREATE TABLE IF NOT EXISTS auth_events (
    id              TEXT PRIMARY KEY,
    user_id         TEXT,                              -- NULL if not authenticated (failed login)
    user_email      TEXT NOT NULL DEFAULT '',          -- captured even on failure
    event_type      TEXT NOT NULL,                     -- login_success, login_failure, lockout, unlock, logout
    failure_reason  TEXT NOT NULL DEFAULT '',          -- invalid_password, account_locked, rate_limited, totp_failed
    ip              TEXT NOT NULL,
    country         TEXT NOT NULL DEFAULT '',
    asn             TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    auth_method     TEXT NOT NULL DEFAULT 'password',  -- password, totp, session
    metadata        JSONB DEFAULT '{}',                -- extra context (rate limit remaining, lockout duration)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_events_type ON auth_events(event_type);
CREATE INDEX idx_auth_events_user ON auth_events(user_id);
CREATE INDEX idx_auth_events_ip ON auth_events(ip);
CREATE INDEX idx_auth_events_created ON auth_events(created_at);
CREATE INDEX idx_auth_events_user_email ON auth_events(user_email);
```

---

## 6. UX Flow

### Sidebar Placement

```
Security
├── SSL Monitors              (existing)
├── Compliance                (existing)
├── Login Activity            (new — admin)
├── Lockouts                  (new — admin)
└── ...

Account
├── Login History             (new — self-service)
└── ...
```

### Page: Login Activity (Admin)

```
┌──────────────────────────────────────────────────────────────┐
│  Login Activity                            [Export CSV]      │
├──────────────────────────────────────────────────────────────┤
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌───────┐ │
│  │ 📊 175   │ │ ❌ 47   │ │ 🔒 3    │ │ 🌐 24   │ │ 73%   │ │
│  │ Logins   │ │ Failed  │ │ Locked  │ │ Unique  │ │ Success│ │
│  │ Today    │ │ Today   │ │ Accts   │ │ IPs     │ │ Rate   │ │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └───────┘ │
│                                                              │
│  Filters: [All Events ▼] [All Users ▼] [Last 24h ▼] [Search]│
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ 08:32  ❌  admin@ex...  185.220.101.x  🇷🇺 RU  wrong pwd │
│  │ 08:31  ❌  admin@ex...  185.220.101.x  🇷🇺 RU  wrong pwd │
│  │ 08:31  ❌  admin@ex...  185.220.101.x  🇷🇺 RU  wrong pwd │
│  │ 08:30  ✅  endang@ex... 10.0.0.5       🏠 Local  OK     │
│  │ 08:15  🔒  user@exam... 103.235.46.x   🇨🇳 CN  Locked   │
│  │ 07:55  ✅  endang@ex... 10.0.0.5       🏠 Local  OK     │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
│  [← Prev]  Page 1 of 8  [Next →]                            │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│  Login Trend (7 Days)                                        │
│  ┌─────────────────────────────────────────────────────┐     │
│  │  ██████████████████████████████████████████████████  │     │
│  │  ████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░  │     │
│  │  ██████████████████████████████████████░░░░░░░░░░░  │     │
│  │  ███░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  │     │
│  │  Jun 05  06   07   08   09   10   11                │     │
│  │  ■ Success  ■ Failure                               │     │
│  └─────────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────────┘
```

---

## 7. Implementation Roadmap

### Phase 1: Backend ✅ Complete

| Task | Effort | Depends On |
|------|--------|-----------|
| Create auth_events table + migration | ✅ Done | — |
| Auth Event Recorder — hook into login handler + Redis lockout | ✅ Done | — |
| REST endpoints (events, summary, lockouts, trend, top-ips, top-users, heatmap) | ✅ Done | Table + recorder |
| GeoIP enrichment (MaxMind DB integration) | ✅ Done | Endpoints |
| Brute force detection config & cron | ✅ Done | Security Events PRD |
| CSV export | ✅ Done | Events endpoint |

### Phase 2: Frontend ✅ Complete

| Task | Effort | Depends On |
|------|--------|-----------|
| Login Activity page — table + filters | ✅ Done | API ready |
| Summary cards | ✅ Done | Activity page |
| Trend chart + hourly heatmap | ✅ Done | Summary cards |
| Self-service "My Login History" | ✅ Done | Activity page |
| Lockouts page | ✅ Done | Lockouts API |
| IP blocking UI (block/unblock/list) | ✅ Done | Block IP API |

### Phase 3: Alerting & Polish ✅ Complete

| Task | Effort | Depends On |
|------|--------|-----------|
| Brute force detection cron + security events integration | ✅ Done | Phase 1 |
| Notification triggers on brute force | ✅ Done | Notification targets |
| CSV export | ✅ Done | Activity page |

**Total:** All phases implemented in v0.14.0

---

## 8. Non-Functional Requirements

| Category | Requirement |
|----------|-------------|
| **Performance** | 10K auth events/day, queries return < 200ms |
| **Retention** | Auth events auto-purged after 90 days (configurable) |
| **Privacy** | User IPs obfuscated in self-service view (last octet masked) |
| **Security** | Admin-only for full IP visibility; users see only own events |
| **Resource** | Event recorder adds < 5ms overhead to login flow |

---

## 9. Dependencies & Integration Points

| Dependency | Type | Purpose |
|------------|------|---------|
| **Login handler** (backend) | Existing | Hook point to record auth events |
| **Redis lockout keys** | Existing | Detect lockout events + clear on unlock |
| **MaxMind GeoLite2 DB** | External | IP → Country/ASN/ISP lookup |
| **Security Events** | Implemented | Brute force alerts feed into auth_activity |
| **Notification targets** | Existing | Alert admins on brute force detection |
| **User management** | Existing | User list, unlock endpoint |

---

## 10. Edge Cases & Error Handling

| Scenario | Behavior |
|----------|----------|
| MaxMind DB not available | GeoIP returns empty strings, log warning, no crash |
| Rapid-fire login attempts | Event recorder still writes each attempt (async write) |
| User cleared from DB but auth_events reference them | user_id = NULL, user_email preserved as string |
| GeoIP DB outdated | Acceptable — country/ASN may be stale, no functional impact |
| Auth event write failure | Non-blocking — login flow continues, error logged |
| Timezone mismatch | All timestamps stored in UTC, converted in frontend |

---

## 11. Mockup References

- See `sketches/login-activity/` for wireframes
- Card + layout consistent with Anjungan dashboard style (existing compliance/SSL pages)
- Chart style: bar + line charts matching existing trend visuals

---

## 12. Future Considerations

| Feature | Priority | Notes |
|---------|----------|-------|
| **Active sessions view** | P2 | See all active sessions per user, force logout |
| **Device fingerprinting** | P3 | Track known devices vs new devices |
| **Suspicious geolocation alert** | P2 | Login from EU + login from Asia in 10 min = impossible travel |
| **MFA adoption tracking** | P3 | % of users with TOTP enabled |
| **Passwordless auth events** | P4 | WebAuthn/passkey login event types |
| **Webhook on brute force** | P3 | Push to external SIEM |

---

## 13. PRD Cross-References

| PRD | Relationship |
|-----|-------------|
| **PRD-security-events.md** | Brute force alerts from auth flow feed into security_events table |
| **PRD-totp-2fa.md** | TOTP verification events tracked in auth_events (login_success via TOTP) |
| **PRD-compliance.md** | Auth security is part of overall compliance posture |
| **PRD-bookmarks.md** | Bookmarks sidebar integration |

---

## 14. References

- MaxMind GeoLite2: https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
- OWASP Brute Force Protection: https://cheatsheetseries.owasp.org/cheatsheets/Blocking_Brute_Force_Attacks.html
- NIST Digital Identity Guidelines (SP 800-63B): https://pages.nist.gov/800-63-3/sp800-63b.html
