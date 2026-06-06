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

---

## Dashboard

### Summary
```
GET /api/v1/dashboard
```
Returns aggregated counts: servers, containers, deployments, users, compliance summary, recent activity.

---

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

---

## SSH Keys

```
GET    /api/v1/ssh-keys         — List keys
POST   /api/v1/ssh-keys         — Create key
DELETE /api/v1/ssh-keys/{id}    — Delete key
```

---

## Containers

```
GET /api/v1/containers                        — List all containers across servers
GET /api/v1/containers?server_id={id}         — Filter by server
GET /api/v1/containers/stats                  — Container statistics
GET /api/v1/containers/{id}                   — Container detail
```

Each container includes optional security scan data (latest container image scan).

---

## Container Registry

### User Self-Service
```
GET  /api/v1/registry/config                        — Get registry URL
GET  /api/v1/registry/my-credentials                 — Get personal credentials (auto-creates if none)
POST /api/v1/registry/my-credentials/reset-password  — Reset personal password
```

### Repository Browser
```
GET    /api/v1/registry/repos                       — List repositories
GET    /api/v1/registry/repos/{name}/tags           — List tags
GET    /api/v1/registry/repos/{name}/{tag}          — Image detail (layers, config, history)
DELETE /api/v1/registry/repos/{name}/manifests/{digest}  — Delete manifest (admin)
DELETE /api/v1/registry/repos/{name}/tags/{tag}          — Delete tag (admin)
POST   /api/v1/registry/gc                          — Trigger garbage collection (admin)
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

---

## Compliance

### Global
```
GET /api/v1/compliance/summary       — Compliance summary across all servers
GET /api/v1/compliance/checks        — List all available checks (grouped by category)
```

### Per-Server
```
GET  /api/v1/compliance/{serverID}/latest            — Latest scan result (?scan_type=, ?category=)
GET  /api/v1/compliance/{serverID}/latest/categories  — Latest scan categorized
POST /api/v1/compliance/{serverID}/scan               — Trigger CIS scan (?profile=cis_level_1|cis_level_2|cis_docker|all)
POST /api/v1/compliance/{serverID}/scan/lynis         — Trigger Lynis audit
POST /api/v1/compliance/{serverID}/scan/docker        — Trigger CIS Docker scan (alias)
POST /api/v1/compliance/{serverID}/scan/containers    — Scan all containers on server
POST /api/v1/compliance/{serverID}/scan/containers/{containerID}  — Scan single container
POST /api/v1/compliance/{serverID}/scan/check/{checkID}          — Run single check
```

### History
```
GET /api/v1/compliance/history                              — Global scan history
GET /api/v1/compliance/active                               — Currently running scans
GET /api/v1/compliance/{serverID}/history                   — Server scan history
GET /api/v1/compliance/{serverID}/history/{scanID}          — Scan detail with findings
GET /api/v1/compliance/{serverID}/history/categories/{category}  — History by category
GET /api/v1/compliance/{serverID}/containers/{containerName}/history  — Container scan history
```

---

## Deployments

```
GET    /api/v1/deployments                     — List deployments (?environment_id=)
POST   /api/v1/deployments                     — Create deployment
GET    /api/v1/deployments/{id}                — Get deployment details
POST   /api/v1/deployments/{id}/restart        — Restart deployment
POST   /api/v1/deployments/{id}/redeploy       — Redeploy
POST   /api/v1/deployments/{id}/rollback       — Rollback
GET    /api/v1/deployments/{id}/history        — Deployment history
GET    /api/v1/deployments/history             — Global history
```

### Environments
```
GET    /api/v1/deployments/environments        — List environments
POST   /api/v1/deployments/environments        — Create environment
PUT    /api/v1/deployments/environments/{id}   — Update environment
DELETE /api/v1/deployments/environments/{id}   — Delete environment
```

---

## Repositories

```
GET    /api/v1/repositories                    — List repositories
GET    /api/v1/repositories/connections        — List provider connections
POST   /api/v1/repositories/connections        — Create connection (validate token first)
DELETE /api/v1/repositories/connections/{id}   — Delete connection
GET    /api/v1/repositories/selections         — List selections
```

---

## Admin

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
GET /api/v1/admin/audit-log/actions       — List unique audit actions
GET /api/v1/admin/audit-log/entity-types  — List entity types
GET /api/v1/admin/audit-log/export        — Export as CSV
```

---

## Settings

```
GET  /api/v1/settings/compliance-thresholds    — Get thresholds + defaults
PUT  /api/v1/settings/compliance-thresholds    — Update thresholds
```

Threshold payload:
```json
{"compliant": 90, "warning": 70}
```
Validation: `compliant > warning > 0`. Default: compliant=90, warning=70.
