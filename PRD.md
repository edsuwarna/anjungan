# Anjungan — Product Requirements Document

> **Version:** 1.0
> **Status:** Draft
> **Last Updated:** June 2026

---

## 1. Product Overview

### 1.1 Vision

A dual-role platform that serves both **Infrastructure Engineers** (managing servers, containers, networking) and **Application Developers** (deploying services, managing releases, observing health) — all through a unified dashboard with role-based access.

### 1.2 Mission Statement

Make infrastructure management accessible for ops teams AND application deployment self-serve for developers, without requiring Kubernetes or complex toolchain.

### 1.3 Why Anjungan?

| Problem | Solution |
|---------|----------|
| Infra tools (Portainer, Cockpit) are ops-only — developers can't use them | **Dual-role UI** — dashboard changes per role |
| IDP tools (Backstage) need Kubernetes | **Runs on VPS** — Go binary + SvelteKit, lightweight |
| Developer always depends on ops to deploy | **Self-service actions** — deploy, rollback, logs from dashboard |
| Credentials spread everywhere | **Centralized vault** + JIT access |
| No visibility into service health | **Service catalog** with golden signals |

### 1.4 Success Metrics

| Metric | Target |
|--------|--------|
| Developer self-service adoption | 80% of deployments via Anjungan (not SSH) |
| Time-to-deploy (new service) | < 15 minutes from scaffold → live |
| Infra engineer workload reduction | 50% fewer "can you deploy this?" requests |
| Incident response time | < 5 minutes to detect + notify |

---

## 2. Target Personas

### 2.1 Infra Engineer (Ops)

| Aspek | Detail |
|-------|--------|
| **Role** | Sysadmin, DevOps, SRE |
| **Goal** | Manage servers, monitor health, enforce security |
| **Pain** | Spread across SSH terminals, no single pane of glass |
| **Sees** | Servers list, containers, system metrics, Docker logs, SSH terminal, audit trail |

### 2.2 Application Developer (Dev)

| Aspek | Detail |
|-------|--------|
| **Role** | Backend/frontend engineer |
| **Goal** | Deploy code, view logs, rollback, check health |
| **Pain** | Needs ops for everything, no self-service |
| **Sees** | Service catalog, deployments, environments, health, logs per service |

### 2.3 Admin (superuser)

| Aspek | Detail |
|-------|--------|
| **Role** | Platform owner / lead |
| **Goal** | Manage users, roles, permissions, audit |
| **Sees** | User management, audit log, system config, API keys |

---

## 3. Feature Inventory & Audit

### 3.1 Current Status (June 2026)

| Domain | Backend | Frontend | Notes |
|--------|---------|----------|-------|
| Auth (login, register, JWT) | ✅ Done | ✅ Done | Refresh token & 2FA verify masih stub |
| TOTP 2FA | 🟡 Partial | ❌ Missing | Backend db + login flow OK, verify endpoint stub |
| Dashboard | 🟡 Partial | 🟡 Partial | Summary API (server count + user count). StatCards doang |
| Servers CRUD | 🟡 Partial | 🟡 Partial | List/Get/Delete ✅. Create & TestConnection stub |
| Containers | ❌ Stub | ❌ Stub | All handlers return "not implemented" |
| Docker Compose | ❌ Stub | ❌ Stub | ComposeUp/Down/Status all stub |
| Registry | ❌ Stub | ❌ Stub | |
| Repositories | ❌ Stub | ❌ Stub | |
| Deployments | ❌ Stub | ❌ Stub | |
| Admin Users | 🟡 Partial | ❌ Stub | ListUsers ✅. Edit/Delete/GetUser stub |
| Audit Log | ❌ Stub | ❌ Stub | Backend masih stub |
| Security Scanning (Lynis) | ❌ Missing | ❌ Missing | Fitur baru — PRD add: June 2026 |
| Container Compliance (CIS Docker) | ❌ Missing | ❌ Missing | Fitur baru — PRD add: June 2026 |
| Trivy Vulnerability Scanner | ❌ Missing | ❌ Missing | Fitur baru — PRD add: June 2026 |
| SSH Terminal | ❌ Missing | ❌ Missing | Belum ada sama sekali |
| Agent system | ❌ Missing | ❌ Missing | Core architecture decision, belum implement |

### 3.2 Database Schema

#### ✅ Existing Tables

```sql
users (id, email, name, password_hash, totp_secret, totp_enabled, role, created_at, updated_at)
servers (id, name, host, port, status, created_by, created_at, updated_at)
```

#### ❌ Tables Belum Dibuat (future migrations)

- `containers` — cache container data per server
- `deployments` — deployment history
- `environments` — dev/staging/prod config
- `services` — service catalog entries
- `api_keys` — developer API tokens
- `agents` — anjungan agent registrations
- `audit_logs` — action audit trail
- `security_scans` — Lynis scan results: hardening_index, report_json, warnings/suggestions per server
- `trivy_scans` — Trivy scan results: image_name, tag, source (github_action/live), scan_number, commit_sha, branch, workflow_url, summary (JSONB), misconfigs (JSONB), secrets (JSONB), raw_results (JSONB)
- `secrets` — vault entries
- `notifications` — notification channel config
- `deployment_templates` — scaffold templates

---

## 4. Feature Specifications

> **Priority Key:** P0 = Must have (blocker), P1 = Should have (important), P2 = Nice to have

### Phase 1 — Foundation (Current Sprint)

#### F1.1 — Server Create & Test Connection
| | |
|--|--|
| **Priority** | P0 |
| **Backend** | `POST /api/v1/servers` + `POST /api/v1/servers/{id}/test` |
| **Frontend** | Modal form: name, host, port, SSH key/password. Test connection button |
| **UX** | Form validation inline. Test shows spinner → green checkmark or red error |
| **Data** | `servers` table sudah ada |

#### F1.2 — Server Detail Page
| | |
|--|--|
| **Priority** | P0 |
| **Route** | `/servers/{id}` |
| **Frontend** | Server info card, status badge, quick actions (SSH, containers, test), metrics (CPU/RAM/disk placeholder), edit/delete |
| **Backend** | `GET /api/v1/servers/{id}` ✅ done. Need `PUT /api/v1/servers/{id}` for edit |
| **UX** | Loading skeleton, error state, empty state (before agent installed) |

#### F1.3 — Containers List per Server
| | |
|--|--|
| **Priority** | P0 |
| **Route** | `/servers/{id}/containers` or filter on `/containers` |
| **Backend** | `GET /api/v1/containers` — implement via SSH exec `docker ps -a` or agent API |
| **Frontend** | Table: name, image, status, ports, uptime. Actions: start/stop/restart |
| **UX** | Real-time status update (poll or WebSocket). Status badges (running=🟢, stopped=🔴, restarting=🟡) |

#### F1.4 — Container Logs (real-time)
| | |
|--|--|
| **Priority** | P1 |
| **Backend** | `GET /api/v1/containers/{id}/logs` — tail + WebSocket streaming |
| **Frontend** | Terminal-like log viewer with auto-scroll, search, pause, copy |
| **UX** | ANSI color support. "Follow" toggle. Line wrap. Timestamp toggle |

#### F1.5 — Admin Users Page
| | |
|--|--|
| **Priority** | P1 |
| **Route** | `/admin/users` |
| **Backend** | `GET/PUT/DELETE /api/v1/admin/users/{id}` — implement remaining stubs |
| **Frontend** | Table: name, email, role, 2FA status, created. Edit modal: change role, disable 2FA, reset password |

#### F1.6 — Dashboard Enhancement
| | |
|--|--|
| **Priority** | P1 |
| **Backend** | Dashboard API tambah: server status distribution, recent activity, container count |
| **Frontend** | StatCards + Server Status donut/pie chart + Recent Activity list + Quick Actions |
| **UX** | Real-time refresh (30s polling). Click chart segmen → filter ke servers page |

#### F1.7 — Docker Compose Management
| | |
|--|--|
| **Priority** | P2 |
| **Backend** | Implement compose up/down/status via agent or SSH |
| **Frontend** | Stack list, status per service in stack, up/down buttons, logs |

---

### Phase 2 — Platform Engineering (IDP Core)

#### F2.1 — Service Catalog
| | |
|--|--|
| **Priority** | P0 (for IDP) |
| **Route** | `/services` |
| **Backend** | New `services` table + CRUD API. Service = deployment target with metadata |
| **Frontend** | Card grid: service name, health badge, tech stack icon, owner, env count |
| **UX** | Filter by team, search, sort by health. Click → detail page |

#### F2.2 — Environment Management
| | |
|--|--|
| **Priority** | P0 (for IDP) |
| **Backend** | New `environments` table: dev/staging/prod per service. Config overrides |
| **Frontend** | Tab-based view per service. Environment switcher |
| **UX** | Clear visual indicator which env (green=prod, yellow=staging, blue=dev) |

#### F2.3 — Self-Service Deploy
| | |
|--|--|
| **Priority** | P0 (for IDP) |
| **Backend** | Deploy runner — pull image/git, run docker compose, track status |
| **Frontend** | "Deploy" button → modal: select version/tag/branch → confirm → progress view |
| **UX** | Progress stepper: Pulling → Starting → Health Check → Done/Error. Sound on complete |

#### F2.4 — Deployment History & Rollback
| | |
|--|--|
| **Priority** | P1 |
| **Backend** | `deployments` table: service, version, status, triggered_by, timestamps, rollback_to |
| **Frontend** | Timeline view per service. Each entry: who, what, when, duration, status |
| **UX** | Rollback button on each deployment. Confirmation modal: "Rollback to v1.2? This will restart the service" |

#### F2.5 — GitHub Integration
| | |
|--|--|
| **Priority** | P2 |
| **Backend** | GitHub API client: repos, branches, commits, webhook receiver |
| **Frontend** | Link GitHub repo to service. Auto-detect new commits. Webhook status |
| **UX** | OAuth flow untuk connect GitHub account. Select repo → auto-tag deployment with commit SHA |

---

### Phase 3 — Security & Governance

#### F3.1 — Centralized Vault
| | |
|--|--|
| **Priority** | P0 (before production use) |
| **Backend** | Encrypted secrets storage per service/environment. AES-256-GCM |
| **Frontend** | Secret editor: key-value pairs. Masked by default, reveal toggle |
| **UX** | "Inject to environment" checkbox per secret. Audit: who accessed what when |

#### F3.2 — API Key Management
| | |
|--|--|
| **Priority** | P1 |
| **Backend** | API keys table. Scoped per user/service. JIT or long-lived |
| **Frontend** | API keys page: generate, revoke, name, last used, scopes |
| **UX** | Show key once on creation. Copy button. Revoke confirmation |

#### F3.3 — Deployment Freeze & Change Management
| | |
|--|--|
| **Priority** | P2 |
| **Backend** | Freeze schedule: start/end, reason, affected environments. Reject deploy during freeze |
| **Frontend** | Calendar view for freezes. Deployment blocked indicator |
| **UX** | "Deploy blocked — production freeze active until Dec 31" |

#### F3.4 — Audit Log (Activity Trail)
| | |
|--|--|
| **Priority** | P0 |
| **Backend** | `audit_logs` table: user_id, action, resource_type, resource_id, details (jsonb), ip_address, created_at. Auto-log via middleware for all mutating API calls (create/delete/update servers, deploy, etc.) |
| **Frontend** | Route `/security/audit`. Table with: timestamp, user, action, resource, details. Filters: by user, action type, date range, resource. Click row → detail modal |
| **UX** | Real-time updates. Search by keyword. Export to CSV. Retention policy configurable (default 90 days) |

#### F3.5 — Security Scanning with Lynis
| | |
|--|--|
| **Priority** | P1 |
| **Backend** | `security_scans` table: id, server_id, status (pending/running/completed/failed), hardening_index, warnings_count, suggestions_count, report_json, started_at, completed_at, created_by. SSH executor via existing `infra/ssh` module — `RunCommand(cfg, "lynis audit system --quick --json")`, parse JSON output |
| **Frontend** | Route `/security/scans`. Server selector dropdown → "Run Scan" button. Scan list table: server, status badge, hardening index (0-100 gauge), date. Detail page `/security/scans/{id}`: gauge chart, passed/failed tests, warnings list, suggestions list, raw report toggle. Quick action "Run Security Scan" di server detail page |
| **UX** | Scan berjalan async — status polling dari frontend. Hardening index ditampilkan sebagai circular gauge (🟢 >80, 🟡 60-80, 🔴 <60). Warnings & suggestions di-group per kategori. Lynis auto-detect: kalau belum terinstall di target, return error "Lynis not found on {host}. Install: https://github.com/cisofy/lynis" |

#### F3.6 — Container Compliance (CIS Docker Benchmark)

| | |
|--|--|
| **Priority** | P1 |
| **Backend** | New `docker_compliance` scan profile di existing compliance scanner — 6 sections, 128 checks via SSH: Section 1 Host Config (14 checks), Section 2 Daemon Config (22), Section 3 Daemon Files (18), Section 4 Images & Build (26), Section 5 Container Runtime (32), Section 6 Swarm Ops (16). Commands via `docker` CLI over SSH (`docker ps`, `docker inspect`, `stat`, dll). Scoring format sama kayak CIS L1/L2: PASS/FAIL per check, percentage |
| **Frontend** | Route `/compliance/cis-docker`. Tab baru di Compliance Dashboard sejajar dengan CIS L1 / L2 / Lynis. Card score (circular gauge), section breakdown table, per-check expandable detail. Filter per section. Quick action dari server detail page |
| **UX** | Score circular gauge 🟢 >80, 🟡 60-80, 🔴 <60. Section table dengan pass/fail count + expand. Per-check panel: CIS ID, title, severity, risk description, remediation, raw command output. Auto-detect: kalo Docker gak terinstall di target, skip dengan status "Docker not available" |

#### F3.7 — Trivy Vulnerability Scanner

| | |
|--|--|
| **Priority** | P1 |
| **Backend** | **Dua source scanning:** (1) **CI/CD Webhook** — `POST /api/v1/trivy/webhook` menerima Trivy JSON dari GitHub Action, extract summary + misconfigs + secrets + vulnerabilities, simpan ke `trivy_scans` table, auto-increment scan_number per image. (2) **Live Scan** — SSH ke target server → `docker run aquasec/trivy:latest image --format json IMAGE:TAG`, parse output, simpan dengan source=`live`. Backend parser bedain tiap `Result` dari Trivy JSON berdasarkan `Type` field: OS packages (alpine/debian/ubuntu), language deps (npm/gomod/pip), Dockerfile misconfig (type=dockerfile), secrets. Tiap result bisa punya `Vulnerabilities[]`, `Misconfigurations[]`, atau `Secrets[]` |
| **Frontend** | Route `/containers/vulnerabilities`. **Dashboard per-image card**: nama, source badge (CI/CD/Live), severity count, mini trend bar, top packages. **Cross-image trends**: CVE yang ngaruh ke multiple images. **KPI bar**: total critical, high, fix rate, scan count (7d). **Scan History timeline** per image: grouped by day, source badge, scan number, version + commit sha, branch tag, severity summary, NEW/FIXED badge. **Scan Detail** dengan Scan Selector (horizontal pills): 4 sub-tab — Vulnerabilities (expandable CVE cards: severity badge, CVE ID, package, version, fix version, CVSS score, reference links), Misconfigurations (Dockerfile lint findings), Secrets (secret leak detection), Raw JSON (full Trivy output, copy/download/export). **Live vs CI/CD Comparison**: dual pane highlighting discrepancy |
| **UX** | Badge system: 🔴 OS vs 📦 Dep. Status filter (fixable/unfixable/NEW). Severity filter. Delta badge vs previous scan (🔺 Critical +1). Scan pills di horizontal scroll. Trend chart 30 scans: bar chart critical+high. Perbedaan CI/CD vs Live di-highlight dengan explanation card |

---

### Phase 4 — Observability

#### F4.1 — Service Health Dashboard
| | |
|--|--|
| **Priority** | P1 |
| **Backend** | Health check runner: HTTP health endpoint, response time, status code |
| **Frontend** | Service list with live health. Color-coded. Response time sparkline |
| **UX** | Click → detail with uptime %, response time graph, last 10 checks |

#### F4.2 — Service Dependency Graph
| | |
|--|--|
| **Priority** | P2 |
| **Backend** | Dependency mapping via config or auto-detect (Docker network, env vars) |
| **Frontend** | Interactive SVG/Canvas graph. D3.js force-directed layout |
| **UX** | Green edges = healthy, red = issue. Click node → service detail |

#### F4.3 — Alert Routing
| | |
|--|--|
| **Priority** | P2 |
| **Backend** | Alert rules: "if service down > 30s → notify team via Telegram/email/webhook" |
| **Frontend** | Notification channel settings. Alert rule editor |
| **UX** | Test notification button. Per-service or global rules |

---

### Phase 5 — Ecosystem

#### F5.1 — CLI Tool
| | |
|--|--|
| **Priority** | P2 |
| **Details** | `anjungan deploy my-service --env production`. Single binary, curl install |
| **UX** | JSON output for CI/CD. Table output for human. `--watch` flag |

#### F5.2 — Developer REST API
| | |
|--|--|
| **Priority** | P2 |
| **Details** | All features accessible via API. API key auth. OpenAPI spec |
| **UX** | Interactive API docs (Swagger UI) at `/docs` |

#### F5.3 — Terraform / OpenTofu Integration
| | |
|--|--|
| **Priority** | P3 |
| **Details** | State viewer + "Apply via Anjungan" button. Plan output |

---

## 5. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
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
|-------|-------|------|
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

1. **≤ 2 clicks** — every common action (deploy, restart, view log) maksimal 2 klik dari halaman mana pun
2. **Self-service first** — developer harus bisa deploy tanpa bantuan infra engineer
3. **Real-time by default** — status, logs, health must update without page refresh
4. **Error states matter** — setiap komponen punya loading, empty, error, dan success state
5. **Mobile responsive** — sidebar collapse, table scroll, touch-friendly buttons
6. **Consistent empty states** — icon + title + description + CTA button
7. **Keyboard shortcuts** — `d` dashboard, `s` servers, `/` search
8. **Confirmation for destructive** — delete, restart, rollback perlu konfirmasi modal

### 6.4 Color Semantics

| Color | Meaning | Usage |
|-------|---------|-------|
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

Agent dipasang di setiap server target via one-liner install. Agent:
- Outbound WebSocket ke Anjungan hub (no inbound port needed)
- Proxy SSH, Docker API, metrics via tunnel
- Auto-register + auto-update
- Heartbeat setiap 15s
- Minimal resource (Go binary ~10MB, RAM < 30MB)

### 7.3 API Design

```
GET    /api/v1/dashboard/summary
GET    /api/v1/dashboard/recent-activity

GET    /api/v1/servers
POST   /api/v1/servers
GET    /api/v1/servers/{id}
PUT    /api/v1/servers/{id}
DELETE /api/v1/servers/{id}
POST   /api/v1/servers/{id}/test
POST   /api/v1/servers/{id}/ssh

GET    /api/v1/containers?server_id={id}
GET    /api/v1/containers/{id}
POST   /api/v1/containers/{id}/start
POST   /api/v1/containers/{id}/stop
POST   /api/v1/containers/{id}/restart
GET    /api/v1/containers/{id}/logs?tail=100&follow=true
GET    /api/v1/containers/stats

POST   /api/v1/containers/compose
POST   /api/v1/containers/compose/{stack}/down
GET    /api/v1/containers/compose/{stack}

GET    /api/v1/registry
GET    /api/v1/registry/{repo}/tags
DELETE /api/v1/registry/{repo}/tags/{tag}

GET    /api/v1/repositories
GET    /api/v1/repositories/{owner}/{repo}/actions
GET    /api/v1/repositories/{owner}/{repo}/workflows

GET    /api/v1/deployments
POST   /api/v1/deployments
GET    /api/v1/deployments/{id}
POST   /api/v1/deployments/{id}/rollback
GET    /api/v1/deployments/history?service_id={id}

GET    /api/v1/services
POST   /api/v1/services
GET    /api/v1/services/{id}
PUT    /api/v1/services/{id}
DELETE /api/v1/services/{id}
GET    /api/v1/services/{id}/environments

GET    /api/v1/admin/users
GET    /api/v1/admin/users/{id}
PUT    /api/v1/admin/users/{id}
DELETE /api/v1/admin/users/{id}

# --- Security & Compliance ---
GET    /api/v1/audit-logs
GET    /api/v1/audit-logs/export

GET    /api/v1/security/scans
POST   /api/v1/security/scans
GET    /api/v1/security/scans/{id}
GET    /api/v1/security/scans/servers/{serverId}

# --- Trivy Vulnerability Scanner ---
POST   /api/v1/trivy/webhook              # Receive Trivy JSON from CI/CD (GitHub Action)
GET    /api/v1/trivy/scans                # List all scans, filter: ?image=, ?source=, ?limit=
GET    /api/v1/trivy/scans/{id}           # Detail scan: vulns, misconfigs, secrets, raw
GET    /api/v1/trivy/scans/latest/{image} # Latest scan for a specific image
GET    /api/v1/trivy/scans/{id}/compare/prev  # Delta vs previous scan
POST   /api/v1/trivy/live-scan            # Trigger live scan via SSH: {server_id, image}

GET    /api/v1/secrets?service_id={id}&env={env}
POST   /api/v1/secrets
DELETE /api/v1/secrets/{id}

GET    /api/v1/api-keys
POST   /api/v1/api-keys
DELETE /api/v1/api-keys/{id}
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

### 🟢 Phase 1 — Foundation (Current)
**Goal:** Functional server management with working CRUD + containers

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 1 | Server Create + Test Connection | 2-3 days | — |
| 2 | Server Detail Page /{id} | 1-2 days | #1 |
| 3 | Containers List per Server | 2-3 days | #1 (need server agent or SSH) |
| 4 | Container Logs (real-time) | 2 days | #3 |
| 5 | Docker Compose Management | 2 days | #3 |
| 6 | Admin Users (CRUD) | 1 day | — |
| 7 | Dashboard Enhancement | 1-2 days | #1, #3 |

### 🟡 Phase 2 — IDP Core
**Goal:** Developer self-service for deployments

| Order | Feature | Effort |
|-------|---------|--------|
| 1 | Service Catalog (CRUD) | 2-3 days |
| 2 | Environment Management | 2 days |
| 3 | Deployment Pipeline (basic) | 3-4 days |
| 4 | Deployment History + Rollback | 1-2 days |
| 5 | Service Scaffolder | 2-3 days |
| 6 | GitHub Integration | 2 days |

### 🔵 Phase 3 — Security & Governance
| Order | Feature | Effort |
|-------|---------|--------|
| 1 | Centralized Vault | 3-4 days |
| 2 | API Key Management | 1 day |
| 3 | Audit Log (Activity Trail) | 2 days |
| 4 | RBAC Enhancement | 1-2 days |
| 5 | Security Scanning (Lynis) | 3-4 days |
| 6 | Container Compliance (CIS Docker) | 3-4 days |
| 7 | Trivy Webhook Endpoint + DB | 2-3 days |
| 8 | Trivy Live Scan (SSH runner) | 2-3 days |
| 9 | Container Vulnerabilities Dashboard | 3-4 days |
| 10 | Deployment Freeze | 1 day |

### 🟣 Phase 4 — Observability
| Order | Feature | Effort |
|-------|---------|--------|
| 1 | Health Dashboard (per-service) | 2-3 days |
| 2 | Alert Routing | 2 days |
| 3 | Service Dependency Graph | 3-4 days |
| 4 | Incident Timeline | 1-2 days |
| 5 | SLO/SLI Tracking | 2-3 days |

### ⚪ Phase 5 — Ecosystem
| Order | Feature | Effort |
|-------|---------|--------|
| 1 | REST API Documentation (Swagger) | 1 day |
| 2 | CLI Tool | 3-4 days |
| 3 | Plugin System | 5-7 days |
| 4 | Terraform/OpenTofu Integration | 2-3 days |

---

## 9. Appendix

### 9.1 Comparison Matrix

| Fitur | Anjungan | Portainer | Dokploy | Coolify |
|-------|----------|-----------|---------|---------|
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
|------|------------|
| **IDP** | Internal Developer Platform — self-service layer for developers |
| **Service Catalog** | Registry of all applications/services managed by the platform |
| **Agent** | Go binary installed on target servers, outbound WebSocket tunnel |
| **Environment** | Deployment stage: dev / staging / production |
| **RBAC** | Role-Based Access Control |
| **Vault** | Encrypted secrets storage |
| **JIT** | Just-In-Time — temporary access grants |
| **SLO** | Service Level Objective — uptime/performance target |
| **Golden Signals** | Latency, Traffic, Errors, Saturation (USE/RED method) |
| **Trivy** | Open-source vulnerability scanner by Aqua Security — scans OS packages, language dependencies, Dockerfile misconfigurations, and secrets in a single run |
| **CIS Docker Benchmark** | Center for Internet Security benchmark for Docker — 128 checks across 6 sections (Host, Daemon, Files, Images, Runtime, Swarm) |
| **SBOM** | Software Bill of Materials — inventory of all components/dependencies in a container image |

### 9.3 References

- ROADMAP.md — Phase planning & status
- DECISIONS.md — Architectural decision records
- docker-compose.yml — Deployment config
- Dockerfile.frontend — Frontend build
- Dockerfile.backend — Backend build
