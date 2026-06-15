---
title: API Reference
description: Complete API reference for Anjungan — all endpoints, request/response formats.
---

# API Reference

All API endpoints are prefixed with `/api/v1`. All endpoints except `/auth/*` and `/health` require JWT authentication via `Authorization: Bearer ***` header.

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
{"email": "user@example.com", "password": "..."}
```

### Register
```
POST /api/v1/auth/register
```
```json
{"email": "user@example.com", "password": "...", "name": "User Name"}
```

### Refresh Token
```
POST /api/v1/auth/refresh
```
```json
{"refresh_token": "..."}
```

### Verify 2FA
```
POST /api/v1/auth/verify-2fa
```
```json
{"code": "123456"}
```

### Get Current User
```
GET /api/v1/auth/me
```

### Logout
```
POST /api/v1/auth/logout
```

## Dashboard

### Summary
```
GET /api/v1/dashboard
```
Returns aggregated counts: servers, containers, deployments, users, compliance summary, recent activity.

## Servers

```
GET    /api/v1/servers              — List all servers
POST   /api/v1/servers              — Create server
GET    /api/v1/servers/{id}         — Get server details
PUT    /api/v1/servers/{id}         — Update server
DELETE /api/v1/servers/{id}         — Delete server
GET    /api/v1/servers/{id}/stats   — Server resource stats
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
POST   /api/v1/ssh-keys         — Create key
DELETE /api/v1/ssh-keys/{id}    — Delete key
```

## Containers

```
GET /api/v1/containers                        — List all containers
GET /api/v1/containers?server_id={id}         — Filter by server
GET /api/v1/containers/stats                  — Container statistics
GET /api/v1/containers/{id}                   — Container detail
```

Each container includes optional security scan data.

## Container Registry

### User Self-Service
```
GET  /api/v1/registry/config                      — Get registry URL
GET  /api/v1/registry/my-credentials               — Get personal credentials
POST /api/v1/registry/my-credentials/reset-password — Reset password
```

### Repository Browser
```
GET    /api/v1/registry/repos                          — List repos
GET    /api/v1/registry/repos/{name}/tags              — List tags
GET    /api/v1/registry/repos/{name}/{tag}             — Image detail
DELETE /api/v1/registry/repos/{name}/manifests/{digest} — Delete manifest (admin)
DELETE /api/v1/registry/repos/{name}/tags/{tag}         — Delete tag (admin)
POST   /api/v1/registry/gc                             — Trigger GC (admin)
```

### User Management (Admin)
```
GET    /api/v1/registry/users              — List registry users
POST   /api/v1/registry/users              — Create registry user
PUT    /api/v1/registry/users/{id}         — Update registry user
DELETE /api/v1/registry/users/{id}         — Delete registry user
POST   /api/v1/registry/sync-htpasswd      — Sync htpasswd + restart Zot
```

## Compliance

### Global
```
GET /api/v1/compliance/summary       — Summary across all servers
GET /api/v1/compliance/checks        — List available checks by category
```

### Per-Server
```
GET  /api/v1/compliance/{serverID}/latest            — Latest scan result
POST /api/v1/compliance/{serverID}/scan               — Trigger CIS scan (?profile=...)
POST /api/v1/compliance/{serverID}/scan/lynis         — Trigger Lynis audit
POST /api/v1/compliance/{serverID}/scan/containers    — Scan all containers
```

### History
```
GET /api/v1/compliance/history                              — Global scan history
GET /api/v1/compliance/{serverID}/history                   — Server scan history
GET /api/v1/compliance/{serverID}/history/{scanID}          — Scan detail with findings
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
GET /api/v1/admin/audit-log               — List audit logs (paginated)
GET /api/v1/admin/audit-log/export        — Export as CSV
```

## Auth Activity

Login monitoring, brute force detection, and IP blocking.

```
GET    /api/v1/auth-activity/events             — Auth events (paginated)
GET    /api/v1/auth-activity/events/mine        — Current user's login history
GET    /api/v1/auth-activity/summary            — Dashboard summary cards
GET    /api/v1/auth-activity/trend              — Daily stats for charts (?days=7)
GET    /api/v1/auth-activity/brute-force        — Brute force detection
GET    /api/v1/auth-activity/blocked-ips        — List blocked IPs
POST   /api/v1/auth-activity/block-ip           — Block an IP
POST   /api/v1/auth-activity/unblock-ip         — Unblock an IP
```

### Events Query Parameters

| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Page number (default 1) |
| `limit` | int | Page size (default 50) |
| `event_type` | string | Filter by event type |
| `status` | string | success/failure |
| `email` | string | Filter by email |
| `ip_address` | string | Filter by IP |
| `search` | string | Full-text search |
| `start_date` | string | ISO date (YYYY-MM-DD) |
| `end_date` | string | ISO date (YYYY-MM-DD) |
| `sort` | string | Sort column |
| `order` | string | asc/desc |

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
    "created_at": "2026-06-11T08:30:00Z"
  }],
  "_meta": { "total": 543, "page": 1, "per_page": 50, "total_pages": 11 }
}
```

## Settings

```
GET  /api/v1/settings/compliance-thresholds    — Get thresholds
PUT  /api/v1/settings/compliance-thresholds    — Update thresholds
```

```json
{"compliant": 90, "warning": 70}
```

## SSL Monitoring

### Monitors CRUD

```
GET    /api/v1/ssl-monitors                          — List monitors
POST   /api/v1/ssl-monitors                          — Create monitor
GET    /api/v1/ssl-monitors/{id}                     — Get detail
PUT    /api/v1/ssl-monitors/{id}                     — Update
DELETE /api/v1/ssl-monitors/{id}                     — Delete
POST   /api/v1/ssl-monitors/{id}/check               — Manual TLS check
GET    /api/v1/ssl-monitors/{id}/history             — Check history
GET    /api/v1/ssl-monitors/{id}/trend               — Trend chart data
POST   /api/v1/ssl-monitors/check-all                — Check all enabled
POST   /api/v1/ssl-monitors/discover                 — Server-side discovery
```

Create monitor payload:
```json
{
  "domain": "app1.edsuwarna.id",
  "port": 443,
  "display_name": "App 1",
  "check_interval": "1h",
  "notify_before": "14d",
  "enabled": true
}
```

### Notification Targets

```
GET    /api/v1/notification-targets             — List all
POST   /api/v1/notification-targets             — Create
DELETE /api/v1/notification-targets/{id}        — Delete
POST   /api/v1/notification-targets/{id}/test   — Send test notification
```

Platforms: `telegram`, `discord`, `slack`, `generic`.

## Uptime Monitoring

```
GET    /api/v1/uptime-monitors                — List monitors
POST   /api/v1/uptime-monitors                — Create monitor
GET    /api/v1/uptime-monitors/{id}           — Get detail
PUT    /api/v1/uptime-monitors/{id}           — Update
DELETE /api/v1/uptime-monitors/{id}           — Delete
```

## Bookmarks

```
GET    /api/v1/bookmarks        — List bookmarks
POST   /api/v1/bookmarks        — Create bookmark
DELETE /api/v1/bookmarks/{id}   — Delete bookmark
```
