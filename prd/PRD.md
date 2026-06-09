# Anjungan — Product Requirements Document

> **Version:** 2.0
> **Status:** 🟡 Partially Implemented — Phase 1 ✅, Phase 2 🟡, Phase 3–5 🔴
> **Last Updated:** June 2026 — status sync with main branch

---

## 1. Product Overview

### 1.1 Vision

A dual-role platform that serves both **Infrastructure Engineers** (managing servers, containers, networking) and **Application Developers** (deploying services, managing releases, observing health) — all through a unified dashboard with role-based access.

### 1.2 Mission Statement

Make infrastructure management accessible for ops teams AND application deployment self-serve for developers, without requiring Kubernetes or complex toolchain.

### 1.3 Why Anjungan?

| Problem | Solution |
---------|----------|
| Infra tools (Portainer, Cockpit) are ops-only — developers can't use them | **Dual-role UI** — dashboard changes per role |
| IDP tools (Backstage) need Kubernetes | **Runs on VPS** — Go binary + SvelteKit, lightweight |
| Developer always depends on ops to deploy | **Self-service actions** — deploy, rollback, logs from dashboard |
| Credentials spread everywhere | **Centralized vault** + JIT access |
| No visibility into service health | **Service catalog** with golden signals |

### 1.4 Success Metrics

| Metric | Target |
--------|--------|
| Developer self-service adoption | 80% of deployments via Anjungan (not SSH) |
| Time-to-deploy (new service) | < 15 minutes from scaffold → live |
| Infra engineer workload reduction | 50% fewer "can you deploy this?" requests |
| Incident response time | < 5 minutes to detect + notify |

---

## 2. Target Personas

### 2.1 Infra Engineer (Ops)

| Aspek | Detail |
-------|--------|
| **Role** | Sysadmin, DevOps, SRE |
| **Goal** | Manage servers, monitor health, enforce security |
| **Pain** | Spread across SSH terminals, no single pane of glass |
| **Sees** | Servers list, containers, system metrics, Docker logs, SSH terminal, audit trail |

### 2.2 Application Developer (Dev)

| Aspek | Detail |
-------|--------|
| **Role** | Backend/frontend engineer |
| **Goal** | Deploy code, view logs, rollback, check health |
| **Pain** | Needs ops for everything, no self-service |
| **Sees** | Service catalog, deployments, environments, health, logs per service |

### 2.3 Admin (superuser)

| Aspek | Detail |
-------|--------|
| **Role** | Platform owner / lead |
| **Goal** | Manage users, roles, permissions, audit |
| **Sees** | User management, audit log, system config, API keys |

---

## 3. Feature Inventory & Audit

### 3.1 Current Status (June 2026)

| Domain | Backend | Frontend | Notes |
--------|---------|----------|-------|
| Auth (login, register, JWT, refresh) | ✅ Done | ✅ Done | Login, register, refresh, logout, me ✅ |
|| User Settings (Self-Service) | ✅ Done | ✅ Done | Profile update (name/email), change password (with current password validation), auto re-issue JWT on email change. Route `/settings` with sidebar/topbar access. Admin: registration toggle ✅ |
|| TOTP 2FA | ✅ Done | ✅ Done | Self-service setup/verify/disable, login TOTP challenge, admin reset. QR code + manual secret |
| Dashboard | ✅ Done | ✅ Done | Summary API: server count, status dist, user count. StatCards + data |
| Servers CRUD | ✅ Done | ✅ Done | List/Create/Get/Update/Delete ✅. Test connection, metrics, detect info, groups/regions/types |
| Containers (per-server + global) | ✅ Done | ✅ Done | List, start/stop/restart, logs, inspect, exec (WebSocket), stats. Security report per container |
| Docker Compose | ❌ Missing | ❌ Missing | Not implemented — see PRD-software-katalog for catalog-based Docker Compose deployment plan |
| Registry (Zot) | ✅ Done | ✅ Done | Repos, tags, delete, GC, image detail. Self-serve creds. Admin user management + htpasswd sync |
| Repositories (Git) | ✅ Done | ✅ Done | GitHub + Forgejo connections. Selections, branches, CI status |
| Deployments | ✅ Done | ✅ Done | CRUD, restart/redeploy/rollback, history, environments. Full frontend |
| Admin Users | ✅ Done | ✅ Done | List/Create/Get/Update/Delete/Unlock ✅ |
| Audit Log | ✅ Done | ✅ Done | List, filter (action/entity/user/date range), export CSV/JSON |
| SSH Keys | ✅ Done | ✅ Done | List/Create/Get/Update/Delete. Admin-only |
| Server Metrics | ✅ Done | ✅ Done | CPU load, RAM, disk. History with `server_metrics` table. Alerts system |
| Security Scans (Lynis + CIS) | ✅ Done | ✅ Done | Lynis, CIS L1, CIS L2, CIS Docker. Trigger, history, detail, per-container security |
| Container Security Report | ✅ Done | ✅ Done | Per-container findings, scan history, **container page |
| SSH Terminal | ✅ Done | ✅ Done | Server-level + container-level terminal via WebSocket |
| Trivy Vulnerability Scanner | ❌ Missing | ❌ Missing | Not implemented |
| Centralized Vault / Secrets | ❌ Missing | ❌ Missing | Not implemented |
| API Key Management | ❌ Missing | ❌ Missing | Not implemented |
| Deployment Freeze | ❌ Missing | ❌ Missing | Not implemented |
| Service Catalog (IDP) | ❌ Missing | ❌ Missing | Not implemented |
| Bookmarks (Tool Shortcuts) | ❌ Missing | ❌ Missing | Tool shortcut management — using categories, auto-favicon, sidebar quick access, dashboard widget — PRD-bookmarks |
| CLI Tool | ❌ Missing | ❌ Missing | Not implemented |

### 3.2 Database Schema (19 migrations — main branch)

| # | Table | Status | Key Columns |
---|-------|--------|-------------|
| 1 | `users` | ✅ Done | id (UUID PK), email, name, password_hash, totp_secret, totp_enabled, role, locked_until, failed_login_attempts, created_at, updated_at |
| 2 | `servers` | ✅ Done | id (UUID PK), name, host, port, ssh_user, ssh_auth_type, ssh_key, ssh_key_id, status, tags, labels, server_group, region, server_type, description, os_info, cpu_info, last_seen_at, monitoring, created_by, container_count |
| 3 | `server_metrics` | ✅ Done | id (BIGSERIAL PK), server_id (FK), cpu_load_1/5/15, mem_used_bytes, mem_total_bytes, disk_used_bytes, disk_total_bytes, disk_used_pct, net_rx_bytes, net_tx_bytes, collected_at |
| 4 | `alerts` | ✅ Done | id (UUID PK), server_id (FK), type, severity, message, value, threshold, acknowledged, created_at |
| 5 | `ssh_keys` | ✅ Done | id (UUID PK), name, key_type, private_key, public_key, fingerprint, created_by (FK), created_at, updated_at |
| 6 | `audit_logs` | ✅ Done | id (UUID PK), action, entity_type, entity_id, description, user_id (FK), user_email, ip_address, metadata (JSONB), created_at |
| 7 | `user_server_groups` | ✅ Done | user_id (FK), server_group (composite PK) |
| 8 | `scan_results` | ✅ Done | id (UUID PK), server_id (FK), scan_type, status, score, total_checks, passed/failed/warnings/criticals/high/medium/low/info, profile, error_message, started_at, completed_at |
| 9 | `scan_findings` | ✅ Done | id (UUID PK), scan_id (FK), check_id, category, severity, title, description, remediation, raw_output, status (pass/fail/warn/info) |
| 10 | `registry_users` | ✅ Done | id (PK), username, password_hash, role (admin/deploy/readonly), anjungan_user_id (FK nullable), timestamps |
| 11 | `environments` | ✅ Done | id (UUID PK), name, color, description, is_protected, timestamps. Seeds: Production, Staging, Development |
| 12 | `repo_connections` | ✅ Done | id (UUID PK), user_id (FK), provider, label, base_url, token_encrypted, affiliations, is_active, timestamps |
| 13 | `repo_selections` | ✅ Done | id (UUID PK), user_id (FK), provider, owner, repo_name, selected, timestamps. UNIQUE(user_id, provider, owner, repo_name) |
| 14 | `deployments` | ✅ Done | id (UUID PK), name, environment_id (FK), repo_provider/owner/name, branch, commit_sha, server_id (FK), service_name, image, status (pending/deploying/running/success/failed/rolled_back), deployed_by (FK), rollback_from (FK), timestamps |
| 15 | `deployment_history` | ✅ Done | id (UUID PK), deployment_id (FK), status, message, created_at |
| 16 | `activity_log` | ✅ Done | Activity log — referenced in code as `SaveActivity`/`ListRecentActivity` |

#### ❌ Not Yet Created

| Table | Purpose |
-------|---------|
| `trivy_scans` | Trivy vuln scan results — image_name, tag, source (ci/live), scan_number, commit_sha, branch, summary, misconfigs, secrets, raw_results |
| `secrets` | Encrypted vault entries per service/environment |
| `api_keys` | Developer API tokens for CI/CD integration |
| `agents` | Anjungan agent registrations (outbound WebSocket connections) |
| `services` | Service catalog entries (IDP core) |
| `notifications` | Notification channel config (Telegram, email, webhook) |
| `deployment_templates` | Scaffold templates for new services |
| `bookmarks` | Per-user tool shortcuts — title, url, icon_type/value, category, sort_order |

---

## 4. Feature Specifications

> **Priority Key:** P0 = Must have (blocker), P1 = Should have (important), P2 = Nice to have

### ✅ Phase 1 — Foundation (Completed — June 2026)

> All Phase 1 features are implemented on main branch, except Docker Compose.

#### ✅ F1.1 — Server Create & Test Connection
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | `POST /api/v1/servers`, `GET/PUT/DELETE /api/v1/servers/{id}`, `POST /api/v1/servers/{id}/test`, `POST /api/v1/servers/test` (global), `POST /api/v1/servers/bulk-delete` |
| **Frontend** | AddServerModal, server list page, detail page with edit/delete |
| **Data** | `servers` table with SSH, labels, groups, regions, types, OS info, tags |

#### ✅ F1.2 — Server Detail Page
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Route** | `/servers/{id}` |
| **Frontend** | Server info card, status badge, metrics (CPU/RAM/disk), quick actions (containers, terminal, edit, delete) |
| **Backend** | `GET /api/v1/servers/{id}`, `GET /api/v1/servers/{id}/metrics`, `GET /api/v1/servers/{id}/metrics/history`, `POST /api/v1/servers/{id}/detect` |

#### ✅ F1.3 — Containers List per Server
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Route** | `/containers` + `/servers/{id}` (contains list) |
| **Backend** | `GET /api/v1/containers/` (global), `GET /api/v1/containers/by-server`, `GET /api/v1/servers/{id}/containers` (per-server via SSH). Start/stop/restart via SSH. |
| **Frontend** | Table: name, image, status, ports, uptime. Actions: start/stop/restart. Stats. |

#### ✅ F1.4 — Container Logs + Exec
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | `GET /api/v1/servers/{id}/containers/{c}/logs` (tail), `POST /api/v1/servers/{id}/containers/{c}/exec` (cmd), `GET .../exec-ws` (WebSocket interactive), `GET /api/v1/servers/{id}/containers/{c}/inspect` |
| **Frontend** | Log viewer, exec command input, terminal via WebSocket |

#### ✅ F1.5 — Admin Users Page
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Route** | `/admin/users` |
| **Backend** | Admin CRUD: list, create, get, update, delete, unlock. Audit log with filter + export. |
| **Frontend** | User table: edit modal (role, 2FA reset, password reset). Unlock button. Audit log viewer. |

#### ✅ F1.6 — Dashboard Enhancement
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | `GET /api/v1/dashboard` — summary: server count, user count, status distribution |
| **Frontend** | StatCards, server status distribution, recent activity |

#### 🔄 F1.7 — Docker Compose Management (Remaining)
| | |
--|--|
| **Priority** | P1 |
| **Status** | ❌ Not implemented |
| **Backend** | Implement compose up/down/status/ps via SSH `docker compose` |
| **Frontend** | Stack list, status per-service, up/down buttons, logs per-stack |

---

### 🟡 Phase 2 — Platform Engineering (IDP Core) — Partial

> Environments CRUD ✅ Done. Deployments CRUD + restart/redeploy/rollback/history ✅ Done.
> Service Catalog (F2.1) and GitHub Integration (F2.5) still remaining.

#### F2.1 — Service Catalog
| | |
--|--|
| **Priority** | P0 (for IDP) |
| **Status** | ❌ Not implemented |
| **Route** | `/services` |
| **Backend** | New `services` table + CRUD API. Service = deployment target with metadata |
| **Frontend** | Card grid: service name, health badge, tech stack icon, owner, env count |

#### ✅ F2.2 — Environment Management
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | `environments` table (id, name, color, is_protected). CRUD via `/api/v1/deployments/environments`. Seeds: Production (protected), Staging, Development |
| **Frontend** | Environment selector, color-coded badges |

#### ✅ F2.3 — Self-Service Deploy
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | `POST /api/v1/deployments` with environment_id, repo info, branch. `POST /api/v1/deployments/{id}/restart`, `/redeploy`, `/rollback` |
| **Frontend** | Deployments page with list, create, restart/redeploy/rollback actions |

#### ✅ F2.4 — Deployment History & Rollback
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | `deployment_history` table: status, message per step. `GET /api/v1/deployments/history` (global), `GET /api/v1/deployments/{id}/history` (per-deployment). Rollback via `POST /api/v1/deployments/{id}/rollback` |
| **Frontend** | History timeline per deployment |

#### F2.5 — GitHub Integration
| | |
--|--|
| **Priority** | P2 |
| **Status** | 🟡 Partial |
| **Backend** | Repo connections via `/api/v1/repositories/connections` (GitHub + Forgejo). Branch listing, CI status checking ✅. Webhook receiver ❌ |
| **Frontend** | Repo connection UI ✅. Auto-detect commits webhook ❌ |

---

### 🔵 Phase 3 — Security & Governance — Partial

> Lynis scanning ✅, CIS Docker ✅, Audit Log ✅, SSH Keys ✅. Trivy, Vault, API Keys still remaining.

#### F3.1 — Centralized Vault
| | |
--|--|
| **Priority** | P0 (before production use) |
| **Status** | ❌ Not implemented |
| **Backend** | Encrypted secrets storage per service/environment. AES-256-GCM |

#### ✅ F3.2 — Audit Log (Activity Trail)
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | `audit_logs` table: action, entity_type, entity_id, description, user_id, ip_address, metadata (JSONB). Auto-log via middleware. Filter: action, entity_type, user, date range. Export CSV/JSON |
| **Frontend** | Route `/admin/audit-log`. Table with filter controls, export button. Detail modal |

#### ✅ F3.3 — SSH Keys Management
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | CRUD via `/api/v1/ssh-keys`. Key types: ed25519, rsa, ecdsa. Auto-generate fingerprint. Linked to servers via `ssh_key_id` |
| **Frontend** | Route `/ssh-keys`. Admin-only. List, create, edit, delete |

#### ✅ F3.4 — SSH Terminal
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | `GET /api/v1/servers/{id}/terminal` (WebSocket). `GET /api/v1/servers/{id}/containers/{c}/exec-ws` (container terminal). SSH exec via `infra/ssh` module |
| **Frontend** | Route `/servers/{id}/terminal`. xterm.js-based terminal. Server dropdown + container exec |

#### ✅ F3.5 — Security Scanning with Lynis
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | `scan_results` + `scan_findings` tables. Trigger via `/api/v1/compliance/{serverID}/scan/lynis`. SSH exec `lynis audit system --quick`. Parse output, score 0-100 |
| **Frontend** | Route `/compliance/lynis`. Data tables: passed/failed/warnings. Score gauge. Trigger scan button |

#### ✅ F3.6 — CIS Compliance (L1 + L2)
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | CIS Level 1 & Level 2 profiles via `/api/v1/compliance/{serverID}/scan?profile=cis_level_1|2`. Per-check PASS/FAIL scoring |
| **Frontend** | Routes `/compliance/cis-level-1`, `/compliance/cis-level-2`. Section breakdown, per-check expandable detail |

#### ✅ F3.7 — CIS Docker Benchmark
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | 6 sections, 128 checks via SSH. Trigger via `/api/v1/compliance/{serverID}/scan/docker`. Commands: `docker ps`, `docker inspect`, `stat` |
| **Frontend** | Route `/compliance/cis-docker`. Section tables, per-check detail. Auto-detect: skip if Docker not installed |

#### ✅ F3.8 — Container Security Report
| | |
--|--|
| **Status** | ✅ Done (main) |
| **Backend** | Per-container scanning via `/api/v1/compliance/{serverID}/scan/containers` + `/scan/containers/{containerID}`. Findings tagged with container name |
| **Frontend** | Route `/containers/{serverId}/{containerId}/security`. Security tab in container page |

#### F3.9 — Trivy Vulnerability Scanner
| | |
--|--|
| **Priority** | P1 |
| **Status** | ❌ Not implemented |
| **Backend** | Dua source: CI/CD webhook (`POST /api/v1/trivy/webhook`) + Live scan via SSH `aquasec/trivy:latest image --format json`. Parse OS packages, language deps, Dockerfile misconfig, secrets |
| **Frontend** | Dashboard per-image card, CVE lists, misconfigs, secrets, scan history timeline |

#### F3.10 — API Key Management
| | |
--|--|
| **Priority** | P1 |
| **Status** | ❌ Not implemented |
| **Backend** | API keys table. Scoped per user/service. JIT or long-lived |

#### F3.11 — Deployment Freeze
| | |
--|--|
| **Priority** | P2 |
| **Status** | ❌ Not implemented |
| **Backend** | Freeze schedule table. Reject deploy during freeze period |

---

### Phase 4 — Observability

#### F4.1 — Service Health Dashboard
| | |
--|--|
| **Priority** | P1 |
| **Backend** | Health check runner: HTTP health endpoint, response time, status code |
| **Frontend** | Service list with live health. Color-coded. Response time sparkline |
| **UX** | Click → detail with uptime %, response time graph, last 10 checks |

#### F4.2 — Service Dependency Graph
| | |
--|--|
| **Priority** | P2 |
| **Backend** | Dependency mapping via config or auto-detect (Docker network, env vars) |
| **Frontend** | Interactive SVG/Canvas graph. D3.js force-directed layout |
| **UX** | Green edges = healthy, red = issue. Click node → service detail |

#### F4.3 — Alert Routing
| | |
--|--|
| **Priority** | P2 |
| **Backend** | Alert rules: "if service down > 30s → notify team via Telegram/email/webhook" |
| **Frontend** | Notification channel settings. Alert rule editor |
| **UX** | Test notification button. Per-service or global rules |

---

### Phase 5 — Ecosystem

#### F5.1 — CLI Tool
| | |
--|--|
| **Priority** | P2 |
| **Details** | `anjungan deploy my-service --env production`. Single binary, curl install |
| **UX** | JSON output for CI/CD. Table output for human. `--watch` flag |

#### F5.2 — Developer REST API
| | |
--|--|
| **Priority** | P2 |
| **Details** | All features accessible via API. API key auth. OpenAPI spec |
| **UX** | Interactive API docs (Swagger UI) at `/docs` |

#### F5.3 — Terraform / OpenTofu Integration
| | |
--|--|
| **Priority** | P3 |
| **Details** | State viewer + "Apply via Anjungan" button. Plan output |

---

## 5. Non-Functional Requirements

| Requirement | Target |
-------------|--------|
| **Performance** | Page load < 1s. API response < 200ms |
| **Concurrent users** | Support 50+ concurrent users on 2GB VPS |
| **Real-time** | WebSocket for logs, terminal < 100ms latency |
| **Security** | All passwords bcrypt. Secrets AES-256-GCM. HTTPS only. JWT short-lived (15m access, 7d refresh) |
| **Availability** | 99.9% uptime target. Graceful degradation if agent offline |
| **Observability** | Every action logged (who, what, when). Structured logging (JSON) |
| **Portability** | Single binary + PostgreSQL + Redis. Docker Compose deploy |
| **Backup** | Database backup + secrets backup. Point-in-time recovery |

---

## 6. UI/UX Design Guidelines

### 6.1 Design Tokens

| Token | Light | Dark |
-------|-------|------|
| `--color-primary` | `#059669` (emerald) | `#34d399` (soft emerald) |
| `--color-sidebar-bg` | `#ffffff` | `#0f172a` |
| `--color-sidebar-text` | `#1e293b` | `#e2e8f0` |
| `--color-surface` | `#f8fafc` | `#0f172a` |
| `--color-card` | `#ffffff` | `#1e293b` |
| `--color-border` | `#e2e8f0` | `#334155` |
| `--color-text` | `#0f172a` | `#f1f5f9` |
| `--color-text-muted` | `#64748b` | `#64748b` |

### 6.2 Layout

```
┌─────────────────────────────────────────────┐
│  Sidebar (256px)  │  TopBar                  │
│  ───────────────  │───────────────────────── │
│  ● Dashboard      │  Main Content Area       │
│  ● Servers       │                         │
│  ● Containers    │  (Responsive, scroll)     │
│  ● Deployments   │                         │
│  ● Registry      │                         │
│  ● Repositories  │                         │
│  ───────────────  │                         │
│  🌓 Dark Mode     │                         │
└─────────────────────────────────────────────┘
```

### 6.3 UX Principles

1. **≤ 2 clicks** — every common action (deploy, restart, view log) within maximum 2 clicks from any page
2. **Self-service first** — developer must be able to deploy without infra engineer's help
3. **Real-time by default** — status, logs, health must update without page refresh
4. **Error states matter** — every component has loading, empty, error, and success states
5. **Mobile responsive** — sidebar collapse, table scroll, touch-friendly buttons
6. **Consistent empty states** — icon + title + description + CTA button
7. **Keyboard shortcuts** — `d` dashboard, `s` servers, `/` search
8. **Confirmation for destructive** — delete, restart, rollback requires confirmation modal

### 6.4 Color Semantics

| Color | Meaning | Usage |
-------|---------|-------|
| Emerald (`#059669`) | Primary, action, active | Buttons, active nav, links |
| Green (`#22c55e`) | Healthy, online, success | Status badges, health checks |
| Red (`#ef4444`) | Error, offline, danger | Alert badges, delete buttons, errors |
| Yellow (`#eab308`) | Warning, pending, degraded | Pending status, warnings |
| Blue (`#3b82f6`) | Info, neutral | Info badges, links |
| Slate (`#64748b`) | Muted, secondary | Subtle text, disabled |

---

## 7. Technical Architecture Reference

### 7.1 Stack

```
Layer           Technology
───             ─────────
Backend         Go (Chi + pgx + asynq)
Frontend        SvelteKit + Tailwind CSS
Database        PostgreSQL 16
Cache/Queue     Redis 7 + asynq
Auth            JWT (HS256) + bcrypt + TOTP (RFC 6238)
SSO             OIDC (Google, GitHub)
Infra Comms     Agent-based reverse proxy (Go)
Deployment      Docker Compose
Proxy           Traefik (external) or built-in
```

### 7.2 Agent Architecture

```
┌─────────────┐       ┌──────────────┐       ┌─────────────┐
│  Anjungan    │       │  Agent (Go)   │       │  Server     │
│  Dashboard   │◄─────►│  (outbound    │◄─────►│  Target     │
│  (Hub)       │  WS   │   WebSocket)  │  SSH  │  (VPS)      │
└─────────────┘       └──────────────┘       └─────────────┘
     │                                                │
     │  PostgreSQL + Redis                            │  Docker socket
     ▼                                                ▼
┌─────────────┐                               ┌─────────────┐
│  Persistent  │                               │  Containers  │
│  Data        │                               │  & Services  │
└─────────────┘                               └─────────────┘
```

Agent is installed on each target server via one-liner install. Agent:
- Outbound WebSocket to Anjungan hub (no inbound port needed)
- Proxy SSH, Docker API, metrics via tunnel
- Auto-register + auto-update
- Heartbeat every 15s
- Minimal resource (Go binary ~10MB, RAM < 30MB)

### 7.3 API Design (main branch — ~108 registered routes)

```
# Health
GET    /health

# Auth — /api/v1/auth
POST   /api/v1/auth/login
POST   /api/v1/auth/register
POST   /api/v1/auth/refresh
POST   /api/v1/auth/verify-2fa
GET    /api/v1/auth/me
POST   /api/v1/auth/logout

# Dashboard — /api/v1/dashboard
GET    /api/v1/dashboard

# Servers — /api/v1/servers  (Infrastructure)
GET    /api/v1/servers
POST   /api/v1/servers
GET    /api/v1/servers/groups
GET    /api/v1/servers/regions
GET    /api/v1/servers/types
POST   /api/v1/servers/bulk-delete
POST   /api/v1/servers/test            # test connection (global)

GET    /api/v1/servers/{id}
PUT    /api/v1/servers/{id}
DELETE /api/v1/servers/{id}
POST   /api/v1/servers/{id}/test       # test connection
GET    /api/v1/servers/{id}/metrics
GET    /api/v1/servers/{id}/metrics/history
POST   /api/v1/servers/{id}/detect     # detect OS/info
GET    /api/v1/servers/{id}/containers # list containers via SSH
GET    /api/v1/servers/{id}/terminal   # WebSocket SSH terminal

# Container actions via server SSH — /api/v1/servers/{id}/containers/{container_id}
POST   /api/v1/servers/{id}/containers/{c}/start
POST   /api/v1/servers/{id}/containers/{c}/stop
POST   /api/v1/servers/{id}/containers/{c}/restart
GET    /api/v1/servers/{id}/containers/{c}/logs
GET    /api/v1/servers/{id}/containers/{c}/inspect
POST   /api/v1/servers/{id}/containers/{c}/exec
GET    /api/v1/servers/{id}/containers/{c}/exec-ws  # WebSocket

# Containers (global) — /api/v1/containers
GET    /api/v1/containers
GET    /api/v1/containers/by-server   # grouped by server
GET    /api/v1/containers/stats
GET    /api/v1/containers/{id}
GET    /api/v1/containers/{id}/security
POST   /api/v1/containers/{id}/start
POST   /api/v1/containers/{id}/stop
POST   /api/v1/containers/{id}/restart
GET    /api/v1/containers/{id}/logs
GET    /api/v1/containers/{id}/stats

# SSH Keys — /api/v1/ssh-keys (admin-only)
GET    /api/v1/ssh-keys
POST   /api/v1/ssh-keys
GET    /api/v1/ssh-keys/{id}
PUT    /api/v1/ssh-keys/{id}
DELETE /api/v1/ssh-keys/{id}

# Registry — /api/v1/registry (Zot integration)
GET    /api/v1/registry/config
GET    /api/v1/registry/my-credentials
POST   /api/v1/registry/my-credentials/reset-password
GET    /api/v1/registry/repos
GET    /api/v1/registry/repos/{name}/tags
GET    /api/v1/registry/repos/{name}/{tag}         # image detail
DELETE /api/v1/registry/repos/{name}/manifests/{digest}  # admin
DELETE /api/v1/registry/repos/{name}/tags/{tag}         # admin
POST   /api/v1/registry/gc                # garbage collection (admin)
GET    /api/v1/registry/users              # admin
POST   /api/v1/registry/users              # admin
PUT    /api/v1/registry/users/{id}         # admin
DELETE /api/v1/registry/users/{id}         # admin
POST   /api/v1/registry/users/{id}/reset-password  # admin
POST   /api/v1/registry/sync-htpasswd              # admin

# Repositories (Git) — /api/v1/repositories
GET    /api/v1/repositories
GET    /api/v1/repositories/connections
POST   /api/v1/repositories/connections
DELETE /api/v1/repositories/connections/{id}
GET    /api/v1/repositories/selections
POST   /api/v1/repositories/selections
GET    /api/v1/repositories/{provider}/{owner}/{repo}/branches
GET    /api/v1/repositories/{provider}/{owner}/{repo}/ci-status
GET    /api/v1/repositories/{provider}/{owner}/{repo}/deployments

# Deployments — /api/v1/deployments
GET    /api/v1/deployments
POST   /api/v1/deployments
GET    /api/v1/deployments/history                 # global history
GET    /api/v1/deployments/{id}
POST   /api/v1/deployments/{id}/restart
POST   /api/v1/deployments/{id}/redeploy
POST   /api/v1/deployments/{id}/rollback
GET    /api/v1/deployments/{id}/history            # per-deployment history
GET    /api/v1/deployments/environments
POST   /api/v1/deployments/environments            # admin
PUT    /api/v1/deployments/environments/{id}       # admin
DELETE /api/v1/deployments/environments/{id}       # admin

# Compliance / Security Scanning — /api/v1/compliance
GET    /api/v1/compliance/summary
GET    /api/v1/compliance/checks
GET    /api/v1/compliance/history                  # global history
GET    /api/v1/compliance/active                   # active scans
GET    /api/v1/compliance/{serverID}/latest
GET    /api/v1/compliance/{serverID}/latest/categories
POST   /api/v1/compliance/{serverID}/scan          # ?profile=cis_level_1|2|cis_docker|all
POST   /api/v1/compliance/{serverID}/scan/lynis
POST   /api/v1/compliance/{serverID}/scan/docker
POST   /api/v1/compliance/{serverID}/scan/containers
POST   /api/v1/compliance/{serverID}/scan/containers/{containerID}
POST   /api/v1/compliance/{serverID}/scan/check/{checkID}
GET    /api/v1/compliance/{serverID}/history
GET    /api/v1/compliance/{serverID}/history/{scanID}
GET    /api/v1/compliance/{serverID}/history/categories/{category}
GET    /api/v1/compliance/{serverID}/containers/{containerName}/history

# Admin — /api/v1/admin (admin-only)
GET    /api/v1/admin/users
POST   /api/v1/admin/users
GET    /api/v1/admin/users/{id}
PUT    /api/v1/admin/users/{id}
DELETE /api/v1/admin/users/{id}
POST   /api/v1/admin/users/{id}/unlock
GET    /api/v1/admin/audit-log
GET    /api/v1/admin/audit-log/actions
GET    /api/v1/admin/audit-log/entity-types
GET    /api/v1/admin/audit-log/export             # ?format=csv|json
```

All responses follow:
```json
{
  "success": true,
  "data": { ... },
  "error": null
}
```

Error response:
```json
{
  "success": false,
  "data": null,
  "error": "message"
}
```

---

## 8. Implementation Roadmap

### ✅ Phase 1 — Foundation (Completed)

> All Phase 1 features are on main branch: Server CRUD, Containers, Logs/Exec, Admin Users, Dashboard, SSH Terminal, SSH Keys, Server Metrics.
> **Remaining:** Docker Compose (F1.7).

| Feature | Effort | Status |
---------|--------|--------|
| ✅ Server Management (CRUD + test + metrics) | 3-4 days | ✅ Done |
| ✅ Containers (list, start/stop/restart, logs, exec, terminal) | 4-5 days | ✅ Done |
| ✅ Admin Users + Audit Log | 2 days | ✅ Done |
| ✅ Dashboard | 1 day | ✅ Done |
| ✅ SSH Terminal | 2 days | ✅ Done |
| ✅ SSH Keys Management | 1 day | ✅ Done |
| 🔄 Docker Compose Management | 2 days | ❌ Remaining |

### 🟡 Phase 2 — IDP Core (Partial)

> Deployments + Environments ✅. Service Catalog & GitHub webhook integration remaining.

| Order | Feature | Effort | Status |
-------|---------|--------|--------|
| 1 | Service Catalog (CRUD) | 2-3 days | ❌ |
| 2 | ✅ Environment Management | 2 days | ✅ Done |
| 3 | ✅ Deployment Pipeline (basic) | 3-4 days | ✅ Done |
| 4 | ✅ Deployment History + Rollback | 1-2 days | ✅ Done |
| 5 | 🟡 GitHub Integration (webhook) | 1 day | 🟡 Partial |
| 6 | Service Scaffolder | 2-3 days | ❌ |

### 🔵 Phase 3 — Security & Governance (Partial)

> Lynis ✅, CIS L1/L2 ✅, CIS Docker ✅, Audit Log ✅, SSH Terminal ✅, Container Security ✅.
> **Remaining:** Trivy, Vault, API Keys, Deployment Freeze.

| Order | Feature | Effort | Status |
-------|---------|--------|--------|
| 1 | ✅ Security Scanning (Lynis) | 3-4 days | ✅ Done |
| 2 | ✅ CIS Compliance (L1 + L2) | 2-3 days | ✅ Done |
| 3 | ✅ CIS Docker Benchmark | 3-4 days | ✅ Done |
| 4 | ✅ Audit Logs | 2 days | ✅ Done |
| 5 | ✅ Container Security Report | 2 days | ✅ Done |
| 6 | 🔄 Trivy Vulnerability Scanner | 3-4 days | ❌ |
| 7 | Centralized Vault | 3-4 days | ❌ |
| 8 | API Key Management | 1 day | ❌ |
| 9 | Deployment Freeze | 1 day | ❌ |

### 🟣 Phase 4 — Observability
| Order | Feature | Effort | Status |
-------|---------|--------|--------|
| 1 | Health Dashboard (per-service) | 2-3 days | ❌ |
| 2 | Alert Routing | 2 days | ❌ |
| 3 | Service Dependency Graph | 3-4 days | ❌ |
| 4 | Incident Timeline | 1-2 days | ❌ |
| 5 | SLO/SLI Tracking | 2-3 days | ❌ |

### ⚪ Phase 5 — Ecosystem
| Order | Feature | Effort | Status |
-------|---------|--------|--------|
| 1 | Agent System (private servers) | 3-5 days | ❌ (PRD-anj-agent) |
| 2 | REST API Documentation (Swagger) | 1 day | ❌ |
| 3 | CLI Tool | 3-4 days | ❌ |
| 4 | Plugin System | 5-7 days | ❌ |
| 5 | Terraform/OpenTofu Integration | 2-3 days | ❌ |

---

## 9. Appendix

### 9.1 Comparison Matrix

| Feature | Anjungan | Portainer | Dokploy | Coolify |
-------|----------|-----------|---------|---------|
| Server management | ✅ Planned | ❌ | ❌ | ❌ |
| Container management | ✅ Planned | ✅ | ✅ | ✅ |
| SSH Terminal | ✅ Planned | ✅ | ❌ | ❌ |
| Service Catalog (IDP) | ✅ Planned | ❌ | ❌ | ❌ |
| Deployment pipeline | ✅ Planned | ❌ | ✅ Basic | ✅ Basic |
| Role-based access | ✅ Planned | ❌ | ✅ | ✅ |
| Vault/Secrets | ✅ Planned | ❌ | ✅ | ❌ |
| Audit Log | ✅ Planned | ❌ | ❌ | ❌ |
| Docker Compose | ✅ Planned | ✅ | ✅ | ✅ |
| GitHub integration | ✅ Planned | ❌ | ✅ | ✅ |
| Agent-based (no open port) | ✅ Planned | ❌ | ❌ | ❌ |
| CLI Tool | ✅ Phase 5 | ❌ | ❌ | ❌ |
| Weight (binary ~15MB) | ✅ Planned | ❌ (~200MB) | ✅ | ❌ |

### 9.2 Glossary

| Term | Definition |
------|------------|
| **IDP** | Internal Developer Platform — self-service layer for developers |
| **Anjungan** | Indonesian for "dock/pier" — the platform name |
| **Service Catalog** | Registry of all applications/services managed by the platform |
| **Agent** | Go binary installed on target servers, outbound WebSocket tunnel |
| **Environment** | Deployment stage: dev / staging / production |
| **RBAC** | Role-Based Access Control |
| **Vault** | Encrypted secrets storage |
| **JIT** | Just-In-Time — temporary access grants |
| **SLO** | Service Level Objective — uptime/performance target |
| **Golden Signals** | Latency, Traffic, Errors, Saturation (USE/RED method) |
| **Trivy** | Open-source vulnerability scanner by Aqua Security — scans OS packages, language dependencies, Dockerfile misconfigurations, and secrets in a single run |
| **TruffleHog** | Open-source secret scanner — scans Git repos, files, and container images for leaked credentials, API keys, and secrets |
| **Lynis** | Open-source security auditing tool for Linux/Unix systems — hardening assessment, shellshock, malware scan |
| **CIS Benchmark** | Center for Internet Security benchmark — secure configuration guidelines for systems and Docker |
| **SBOM** | Software Bill of Materials — inventory of all components/dependencies in a container image |

### 9.3 References

- [ROADMAP.md](../docs/ROADMAP.md) — Phase planning & status
- [DECISIONS.md](../docs/DECISIONS.md) — Architectural decision records
- docker-compose.yml — Deployment config
- Dockerfile.frontend — Frontend build
- Dockerfile.backend — Backend build

### Individual PRDs

- [PRD-compliance.md](./PRD-compliance.md) — Compliance & Security Scanning
- [PRD-registry.md](./PRD-registry.md) — Registry & Image Management
- [PRD-script-library.md](./PRD-script-library.md) — Script Library & Automation
- [PRD-domain-management.md](./PRD-domain-management.md) — Domain & DNS Management
- [PRD-templates-scaffolding.md](./PRD-templates-scaffolding.md) — Templates & Scaffolding
- [PRD-repositories-deployments.md](./PRD-repositories-deployments.md) — Repositories & Deployments
- [PRD-anj-agent.md](./PRD-anj-agent.md) — Anj Agent (Server-side agent)
- [PRD-resource-usage-cost.md](./PRD-resource-usage-cost.md) — Resource Usage & Cost
- [PRD-bookmarks.md](./PRD-bookmarks.md) — Bookmarks & Tool Shortcuts
