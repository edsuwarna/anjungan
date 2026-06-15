---
title: API Reference
description: Complete API reference for Anjungan — all endpoints, request/response formats.
---

# API Reference

All API endpoints are prefixed with `/api/v1`. All endpoints except `/auth/*` and `/health` require JWT authentication via `Authorization: Bearer <token>` header.

## Health

```
GET /health
```

Response: `{"status": "ok"}`

## Authentication

### Login
```
POST /api/v1/auth/login
```
```json
{"email": "user@example.com", "password": "...", "totp_code": "123456"}
```

Returns access + refresh tokens. If user has 2FA enabled, first call returns `{"status": "totp_required", "email": "..."}` — then call `POST /verify-2fa` with the TOTP code.

### Register
```
POST /api/v1/auth/register
```
```json
{"email": "user@example.com", "password": "...", "name": "User Name"}
```

Registration can be disabled by admin via settings.

### Refresh Token
```
POST /api/v1/auth/refresh
```
```json
{"refresh_token": "..."}
```

### Verify 2FA (Login Flow)
```
POST /api/v1/auth/verify-2fa
```
```json
{"email": "user@example.com", "token": "123456"}
```
Completes login after `totp_required` response.

### Setup TOTP (Enable 2FA)
```
POST /api/v1/auth/setup-totp
```
Generates TOTP secret + QR code URI for the authenticated user (no request body). Returns secret, QR URI, and recovery codes.

### Verify TOTP Setup (Confirm 2FA)
```
POST /api/v1/auth/verify-totp-setup
```
```json
{"token": "123456"}
```
Confirms the TOTP code and enables 2FA for the user.

### Disable TOTP
```
POST /api/v1/auth/disable-totp
```
```json
{"password": "current-password"}
```
Disables 2FA. Requires current password for security.

### Get Current User
```
GET /api/v1/auth/me
```

### Update Profile
```
PUT /api/v1/auth/profile
```
```json
{"name": "New Name", "email": "new@example.com"}
```
Both fields optional — only send what you want to change.

### Change Password
```
PUT /api/v1/auth/password
```
```json
{"current_password": "...", "new_password": "..."}
```

### Login History
```
GET /api/v1/auth/login-history
```
Returns the current user's recent auth events (paginated). Supports filters: `?page=1&limit=30&event_type=&status=&ip_address=&search=`.

### Logout
```
POST /api/v1/auth/logout
```

## Dashboard

### Summary
```
GET /api/v1/dashboard
```
Returns aggregated dashboard data:

```json
{
  "servers": 3,
  "containers": 12,
  "deployments": 0,
  "users": 5,
  "compliance_summary": {"compliant": 2, "warning": 0, "critical": 1},
  "recent_activity": [
    {"action": "server.created", "entity": "server", "actor": "admin@example.com", "created_at": "..."}
  ]
}
```

## Servers

```
GET    /api/v1/servers              — List all servers
POST   /api/v1/servers              — Create server
GET    /api/v1/servers/{id}         — Get server details
PUT    /api/v1/servers/{id}         — Update server
DELETE /api/v1/servers/{id}         — Delete server
GET    /api/v1/servers/{id}/stats   — Server resource stats (CPU, memory, disk)
```

Server payload:
```json
{
  "name": "production-01",
  "host": "192.168.1.100",
  "port": 22,
  "ssh_user": "root",
  "ssh_auth_type": "key",
  "ssh_key_id": "key-uuid",
  "server_group": "production"
}
```

### SSH Terminal (WebSocket)
```
GET /api/v1/servers/{id}/terminal
```
Upgrades to WebSocket for interactive SSH terminal session.

## SSH Keys

```
GET    /api/v1/ssh-keys         — List keys
POST   /api/v1/ssh-keys         — Create key (private_key, name)
DELETE /api/v1/ssh-keys/{id}    — Delete key
```

## Containers

```
GET /api/v1/containers                        — List all containers across servers
GET /api/v1/containers?server_id={id}         — Filter by server
GET /api/v1/containers/stats                  — Container statistics
GET /api/v1/containers/{id}                   — Container detail
```

Each container includes optional security scan data (latest container image scan results).

## Container Registry

### User Self-Service
```
GET  /api/v1/registry/config                        — Get registry URL + health
GET  /api/v1/registry/my-credentials                 — Get personal credentials (auto-creates if none)
POST /api/v1/registry/my-credentials/reset-password  — Reset personal password
```

### Repository Browser
```
GET    /api/v1/registry/repos                           — List repositories
GET    /api/v1/registry/repos/{name}/tags               — List tags
GET    /api/v1/registry/repos/{name}/{tag}              — Image detail (layers, config, history, vulnerabilities)
DELETE /api/v1/registry/repos/{name}/manifests/{digest}  — Delete manifest (admin)
DELETE /api/v1/registry/repos/{name}/tags/{tag}          — Delete tag (admin)
POST   /api/v1/registry/gc                              — Trigger garbage collection (admin)
```

### User Management (Admin)
```
GET    /api/v1/registry/users              — List registry users
POST   /api/v1/registry/users              — Create registry user
PUT    /api/v1/registry/users/{id}         — Update registry user
DELETE /api/v1/registry/users/{id}         — Delete registry user
POST   /api/v1/registry/users/{id}/reset-password  — Reset user password
POST   /api/v1/registry/sync-htpasswd      — Sync htpasswd + restart Zot
```

### Tag Protection
```
POST   /api/v1/registry/repos/{name}/tags/{tag}/protect    — Lock tag (prevent deletion)
POST   /api/v1/registry/repos/{name}/tags/{tag}/unprotect  — Unlock tag
```

## Compliance

### Global
```
GET /api/v1/compliance/summary       — Compliance summary across all servers
GET /api/v1/compliance/checks        — List all available checks (grouped by category)
```

### Per-Server
```
GET  /api/v1/compliance/{serverID}/latest                  — Latest scan result (?scan_type=, ?category=)
GET  /api/v1/compliance/{serverID}/latest/categories       — Latest scan with category breakdown
POST /api/v1/compliance/{serverID}/scan                    — Trigger CIS scan (?profile=cis_level_1|cis_level_2|cis_docker|all)
POST /api/v1/compliance/{serverID}/scan/lynis              — Trigger Lynis security audit
POST /api/v1/compliance/{serverID}/scan/docker             — Trigger CIS Docker scan
POST /api/v1/compliance/{serverID}/scan/containers         — Scan all containers on server
POST /api/v1/compliance/{serverID}/scan/containers/{id}    — Scan single container
POST /api/v1/compliance/{serverID}/scan/check/{checkID}    — Run a single compliance check
```

### History
```
GET /api/v1/compliance/history                                    — Global scan history (paginated)
GET /api/v1/compliance/active                                     — Currently running scans
GET /api/v1/compliance/{serverID}/history                         — Server scan history
GET /api/v1/compliance/{serverID}/history/{scanID}                — Scan detail with findings
GET /api/v1/compliance/{serverID}/history/categories/{category}  — History by category
GET /api/v1/compliance/{serverID}/containers/{containerName}/history — Container scan history
```

## Admin

### Users
```
GET    /api/v1/admin/users               — List users
POST   /api/v1/admin/users               — Create user
GET    /api/v1/admin/users/{id}          — Get user
PUT    /api/v1/admin/users/{id}          — Update user
DELETE /api/v1/admin/users/{id}          — Delete user
POST   /api/v1/admin/users/{id}/unlock   — Unlock locked user
```

### Audit Log
```
GET /api/v1/admin/audit-log               — List audit logs (paginated, filterable)
GET /api/v1/admin/audit-log/actions       — List unique audit action types
GET /api/v1/admin/audit-log/entity-types  — List entity types
GET /api/v1/admin/audit-log/export        — Export audit log as CSV
```

Audit log query params: `?page=&limit=&action=&entity_type=&actor=&search=&start_date=&end_date=&sort=&order=`

## Auth Activity

All endpoints under `/api/v1/auth-activity` — login monitoring, brute force detection, and IP blocking. Admin-only.

### Summary
```
GET /api/v1/auth-activity/summary
```
Today's KPIs:
```json
{
  "today_logins": 128,
  "today_failures": 47,
  "today_success_rate": 63.3,
  "today_lockouts": 0,
  "unique_ips": 24,
  "blocked_ips_count": 2,
  "active_brute_force_alerts": 2
}
```

### Events
```
GET /api/v1/auth-activity/events            — Paginated events (?page=&limit=&event_type=&status=&email=&ip_address=&search=&start_date=&end_date=&sort=&order=)
GET /api/v1/auth-activity/events/mine       — Current user's own login history (last 20)
GET /api/v1/auth-activity/events/export     — CSV export for audit
GET /api/v1/auth-activity/trend             — Daily aggregated stats (?days=7, max 90)
GET /api/v1/auth-activity/brute-force       — Brute force detection results
GET /api/v1/auth-activity/top-ips           — IPs with most failures (?days=7)
GET /api/v1/auth-activity/top-users         — Users with most failures (?days=7)
GET /api/v1/auth-activity/heatmap           — Hourly distribution (?days=7)
```

### Lockouts
```
GET /api/v1/auth-activity/lockouts          — Currently locked accounts
```

### IP Blocking
```
GET  /api/v1/auth-activity/blocked-ips      — List blocked IPs
POST /api/v1/auth-activity/block-ip         — Block an IP
POST /api/v1/auth-activity/unblock-ip       — Unblock an IP
```

### Brute Force Configuration
```
GET /api/v1/auth-activity/config            — Get brute force config
PUT /api/v1/auth-activity/config            — Update brute force config
```

```json
{
  "threshold": 20,
  "window_minutes": 5,
  "notification_target_ids": ["nt_xxx", "nt_yyy"]
}
```

### Events Response
```json
{
  "events": [{
    "id": "aev_xxx",
    "user_id": "usr_xxx",
    "email": "admin@example.com",
    "event_type": "login_failure",
    "status": "failure",
    "failure_reason": "invalid_password",
    "ip_address": "185.220.101.23",
    "country": "RU",
    "asn": "AS12345",
    "isp": "Example ISP",
    "user_agent": "Mozilla/5.0 ...",
    "auth_method": "password",
    "created_at": "2026-06-11T08:30:00Z"
  }],
  "_meta": { "total": 543, "page": 1, "per_page": 50, "total_pages": 11 }
}
```

## Settings

### Compliance Thresholds
```
GET  /api/v1/settings/compliance-thresholds    — Get thresholds + defaults
PUT  /api/v1/settings/compliance-thresholds    — Update thresholds
```

```json
{"compliant": 90, "warning": 70}
```
Validation: `compliant > warning > 0`. Default: compliant=90, warning=70.

### Registration
```
GET  /api/v1/settings/registration            — Check if registration is enabled (no auth required)
```

## SSL Monitoring

### Monitors CRUD

```
GET    /api/v1/ssl-monitors                          — List (?page=&limit=&search=&status=&sort=&order=&all=)
POST   /api/v1/ssl-monitors                          — Create monitor
GET    /api/v1/ssl-monitors/summary                  — KPI counts (total, valid, expiring_soon, expired, error)
GET    /api/v1/ssl-monitors/export/csv               — Export monitors as CSV
POST   /api/v1/ssl-monitors/import                   — Batch import [{domain, port, display_name}]
POST   /api/v1/ssl-monitors/check-all                — Check all enabled monitors
POST   /api/v1/ssl-monitors/discover                 — Server-side discovery {server_id, provider}
POST   /api/v1/ssl-monitors/discover/import           — Import discovered domains
GET    /api/v1/ssl-monitors/{id}                     — Get detail
PUT    /api/v1/ssl-monitors/{id}                     — Update
DELETE /api/v1/ssl-monitors/{id}                     — Delete
POST   /api/v1/ssl-monitors/{id}/check               — Manual TLS check
GET    /api/v1/ssl-monitors/{id}/history             — Paginated check history (?limit=&offset=)
GET    /api/v1/ssl-monitors/{id}/trend               — Trend chart data (?limit=90, default 90)
```

Create monitor payload:
```json
{
  "domain": "app1.edsuwarna.id",
  "port": 443,
  "display_name": "App 1 Production",
  "check_interval": "1h",
  "notify_before": "14d",
  "webhook_ids": ["target-uuid"],
  "enabled": true
}
```

Summary response:
```json
{
  "total": 8,
  "valid": 5,
  "expiring_soon": 2,
  "expired": 0,
  "error": 1
}
```

### Discovery Providers
| Provider | Source |
|----------|--------|
| `auto` | Auto-detect from server |
| `traefik` | Traefik reverse proxy config |
| `nginx` | Nginx config files |
| `caddy` | Caddy config |
| `letsencrypt` | Let's Encrypt certificate directory |
| `filesystem` | Scan filesystem for cert files |

## Uptime Monitoring

### Monitors CRUD

```
GET    /api/v1/uptime-monitors                   — List (?page=&limit=&search=&status=&sort=&order=)
POST   /api/v1/uptime-monitors                   — Create monitor
GET    /api/v1/uptime-monitors/summary            — KPI counts (up, down, paused, total)
POST   /api/v1/uptime-monitors/check-all          — Check all enabled monitors
GET    /api/v1/uptime-monitors/{id}               — Get detail
PUT    /api/v1/uptime-monitors/{id}               — Update
DELETE /api/v1/uptime-monitors/{id}               — Delete
POST   /api/v1/uptime-monitors/{id}/check         — Manual check now
POST   /api/v1/uptime-monitors/{id}/pause         — Pause monitoring
POST   /api/v1/uptime-monitors/{id}/resume         — Resume monitoring
GET    /api/v1/uptime-monitors/{id}/history        — Check history (?limit=&offset=)
GET    /api/v1/uptime-monitors/{id}/trend          — Daily response time trend (?days=)
POST   /api/v1/uptime-monitors/{id}/test-notification — Send test notification
```

Create monitor payload:
```json
{
  "name": "My App",
  "url": "https://app.example.com/health",
  "check_type": "http",
  "interval_seconds": 60,
  "timeout_seconds": 10,
  "expected_status_min": 200,
  "expected_status_max": 399,
  "expected_body": "ok",
  "enabled": true,
  "notification_target_ids": ["nt_xxx"]
}
```

### Check Types
| Type | Description |
|------|-------------|
| `http` | HTTP GET request, checks status code range + optional body match |
| `tcp` | TCP port check (connectivity only) |
| `ping` | ICMP ping |

### Incidents
```
GET /api/v1/uptime-monitors/{id}/incidents       — Incident timeline (auto-grouped consecutive failures)
```

### Maintenance Windows
```
GET    /api/v1/uptime-monitors/{id}/maintenance         — List maintenance windows
POST   /api/v1/uptime-monitors/{id}/maintenance         — Create maintenance window
DELETE /api/v1/uptime-monitors/{id}/maintenance/{mwId}  — Delete maintenance window
```

```json
{
  "start_time": "2026-06-20T22:00:00Z",
  "end_time": "2026-06-21T06:00:00Z",
  "reason": "Scheduled database migration"
}
```

### SSE Events (Real-time)
```
GET /api/uptime/events?token=<jwt-token>
```
Server-Sent Events stream for real-time uptime status updates. Auth via query param (SSE/EventSource can't set headers).

```
event: check_result
data: {"monitor_id": "...", "status": "up", "response_time_ms": 234, "checked_at": "..."}
```

## Notification Targets

Shared notification targets used by SSL Monitoring, Uptime Monitoring, and Brute Force alerts.

```
GET    /api/v1/notification-targets             — List all targets
POST   /api/v1/notification-targets             — Create target
GET    /api/v1/notification-targets/{id}        — Get target detail
PUT    /api/v1/notification-targets/{id}        — Update target
DELETE /api/v1/notification-targets/{id}        — Delete target
POST   /api/v1/notification-targets/{id}/test   — Send test notification
```

Create payload:
```json
{
  "name": "Team Alerts",
  "platform": "telegram",
  "config": {
    "bot_token": "123:ABC",
    "chat_id": "-1001234567890"
  },
  "enabled": true
}
```

Supported platforms: `telegram`, `discord`, `slack`, `generic` (webhook). Formatting is auto-applied per platform.

## Bookmarks

Tool shortcut bookmarks — user-specific, categorized.

```
GET    /api/v1/bookmarks              — List bookmarks
POST   /api/v1/bookmarks              — Create bookmark
PUT    /api/v1/bookmarks/{id}         — Update bookmark (name, url, category, pinned)
DELETE /api/v1/bookmarks/{id}         — Delete bookmark
PUT    /api/v1/bookmarks/reorder      — Reorder bookmarks [{id, sort_order}]
```

Create payload:
```json
{
  "name": "Grafana",
  "url": "https://grafana.example.com",
  "category": "monitoring",
  "pinned": true
}
```
