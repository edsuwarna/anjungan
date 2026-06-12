# Anjungan — Integration Roadmap: Security, DevOps, SRE, Cloud, Infra, Software Development

> **Version:** 1.0
> **Status:** 🟡 Strategic Reference — Living Document
> **Author:** Endang Suwarna
> **Last Updated:** June 11, 2026

---

## 1. Executive Summary

### Purpose

This document serves as the **master roadmap** for all third-party open-source tool integrations into Anjungan. It covers six domains:

| Domain | Focus |
|--------|-------|
| 🛡️ **Security** | Threat detection, vulnerability scanning, compliance, secrets, auth security |
| ⚙️ **DevOps** | CI/CD visibility, dependency management, build tooling |
| 📈 **SRE** | Server monitoring, metrics, alerting, error tracking |
| ☁️ **Cloud Engineering** | IaC state management, multi-cloud asset inventory |
| 🏗️ **Infrastructure Engineering** | IPAM, runbook automation, VPN, DCIM |
| 💻 **Software Development** | Workflow automation, storage, documentation |

Each entry covers: **what it is**, **how it integrates into Anjungan**, **effort**, **priority**, and **link to existing PRD** if one exists.

### Usage

- **Planning**: Reference when deciding "what to build next"
- **Discovery**: Browse tools by domain or priority
- **Architecture**: Understand integration patterns across all features
- **Gap analysis**: Identify domains with no coverage yet

---

## 2. Integration Patterns

All integrations follow one of four patterns:

### Pattern A: API Consumer
```
Tool (REST API) ←── Anjungan Backend ←── Frontend
```
Anjungan queries the tool's API and displays results. Tool runs independently.
- *Examples: CrowdSec, Netdata, Renovate, Woodpecker*
- **Effort**: Low-Medium
- **Dependency**: Tool must be deployed first

### Pattern B: Agent/SSH Runner
```
Anjungan Backend ──SSH──► Server ──CLI──► Tool
                                        └──► stdout → parse → store
```
Anjungan executes the tool via SSH on managed servers and parses output.
- *Examples: Trivy, TruffleHog, Gitleaks, OpenSCAP, ClamAV*
- **Effort**: Medium
- **Dependency**: Tool installed on target servers

### Pattern C: Embedded / SDK
```
Anjungan Backend ──SDK──► Tool (library)
```
Tool runs as a Go library/SDK inside Anjungan backend process.
- *Examples: MaxMind GeoIP, Step CA (client)*
- **Effort**: Low
- **Dependency**: Go library import

### Pattern D: Webhook / Event-Driven
```
Tool ──webhook──► Anjungan API
```
Tool pushes events to Anjungan when something happens.
- *Examples: Woodpecker webhook, Netdata alarms, Renovate webhook*
- **Effort**: Low
- **Dependency**: Tool must support outbound webhooks

---

## 3. Tool Inventory

### 🛡️ Security

| # | Tool | Function | Integration Pattern | Has PRD? | Effort | Priority |
|---|------|----------|-------------------|----------|--------|----------|
| S1 | **CrowdSec** | IDS/IPS, real-time attack detection | A (LAPI REST) | ✅ PRD-security-events.md | ⭐⭐⭐ | P1 |
| S2 | **Trivy** | Container image vulnerability scanning | B (SSH runner) | ✅ PRD-container-image-scanning.md | ⭐⭐⭐⭐ | P1 |
| S3 | **TruffleHog** | Secret scanning (git + images) | B (SSH runner) | ✅ PRD-secret-scanning.md | ⭐⭐⭐ | P1 |
| S4 | **Auth Security** | Login activity monitoring, brute force detection | C (SDK — existing code) | ✅ PRD-login-activity.md | ⭐ | P1 |
| S5 | **Container Security** | Runtime container posture (25+ checks) | B (SSH runner — docker inspect) | ✅ PRD-container-security.md | ⭐⭐ | P1 |
| S6 | **Gitleaks** | Lightweight git secret scanning (CI gate) | B (SSH runner) | ❌ | ⭐ | P2 |
| S7 | **ClamAV** | Malware scanning for uploaded files / registry | B (SSH runner) | ❌ | ⭐ | P2 |
| S8 | **OpenSCAP** | Enterprise compliance (NIST, PCI-DSS, STIG) | B (SSH runner) | ❌ | ⭐⭐⭐ | P3 |
| S9 | **Step CA** | Internal TLS certificate authority | C (Go client library) | ❌ | ⭐⭐⭐ | P3 |
| S10 | **HashiCorp Vault** | Secrets management, encryption backend | A (KV API) | ❌ | ⭐⭐⭐⭐ | P3 |

#### Security Cross-Cutting Concerns

- **Notification**: All S1–S10 integrate with shared `notification_targets` system (scoped per feature)
- **Audit Log**: All user-triggered actions (scan, unblock, reveal secret) logged to audit_log
- **Dashboard**: Each tool gets a card/section in the Security category sidebar
- **Compliance Score**: S2, S3, S5, S8 feed into overall compliance health score

---

### ⚙️ DevOps

| # | Tool | Function | Integration Pattern | Has PRD? | Effort | Priority |
|---|------|----------|-------------------|----------|--------|----------|
| D1 | **Renovate** | Auto-dependency update visibility | A (Renovate API / webhook) | ❌ | ⭐⭐ | P1 |
| D2 | **Woodpecker CI** | Pipeline status viewer | A (Woodpecker API) | ❌ | ⭐⭐ | P2 |
| D3 | **Earthly** | Build tooling (Makefile-like) | B (SSH runner) | ❌ | ⭐⭐ | P3 |
| D4 | **Dagger** | CI/CD engine (container pipelines) | A (Dagger API) | ❌ | ⭐⭐⭐⭐ | P4 |

#### D1 — Renovate Integration Detail

**What it shows in Anjungan:**
- Repository list → "Dependencies" tab
  - Table: package name, current version, latest version, update type (major/minor/patch), vulnerability (Y/N)
  - Badge: "3 outdated · 1 vulnerable" on repo card
- Summary: "7 repos tracked · 23 outdated deps · 2 critical vulns"

**Backend:**
- Query Renovate dashboard API or Renovate's PostgreSQL (it stores results in DB)
- Or receive Renovate webhook on completed scan

**Sidebar placement:**
```
Repository Detail → [Dependencies Tab]
```

**Effort breakdown:**
| Task | Effort |
|------|--------|
| API/webhook integration | 0.5d |
| DB table + store | 0.5d |
| Frontend — dep table + badges | 1d |
| Total | **2d** |

---

### 📈 SRE (Site Reliability Engineering)

| # | Tool | Function | Integration Pattern | Has PRD? | Effort | Priority |
|---|------|----------|-------------------|----------|--------|----------|
| R1 | **Netdata** | Real-time server + container metrics | A (Netdata REST API) | ❌ | ⭐⭐ | **P0** |
| R2 | **VictoriaMetrics** | Time-series metrics storage (if Netdata not enough) | A (PromQL HTTP API) | ❌ | ⭐⭐⭐ | P3 |
| R3 | **Sentry** | Error tracking (self-hosted) | A (Sentry API) | ❌ | ⭐⭐ | P3 |

#### R1 — Netdata Integration (P0 — Recommended #1)

**Why P0:** Netdata fills the single biggest blind spot in Anjungan right now — **zero visibility into server metrics**. No CPU, RAM, disk, network graphs. Capacity Trending PRD cannot exist without metrics data.

**Integration Architecture:**
```
Netdata Agent         Netdata REST API          Anjungan Backend
┌────────────┐       ┌──────────────┐         ┌──────────────────┐
│ Server A   │──▶    │ :19999/api   │──▶      │ Metrics Handler   │
│ Netdata    │       │ /v1/data?    │          │ ├─ GET /metrics  │
│            │       │ chart=       │          │ │   /servers/:id │
└────────────┘       │ system.cpu   │          │ ├─ GET /metrics  │
                     │ mem.ram      │          │ │   /containers  │
┌────────────┐       │ net.dropped  │          │ └─ GET /metrics  │
│ Server B   │──▶    │ disk.io      │          │   /summary       │
│ Netdata    │       └──────────────┘          └──────────────────┘
└────────────┘                                          │
                                                  ┌─────▼──────┐
                                                  │  Frontend   │
                                                  │  Chart.js   │
                                                  │  + SVG      │
                                                  └────────────┘
```

**What it shows in Anjungan:**

| Page | Widget | Data Source |
|------|--------|-------------|
| **Dashboard** | "Server Health" StatCard (CPU > 80%? RAM > 90%?) | Netdata alarm status |
| **Server Detail** | Tab "Metrics" — CPU (per-core), RAM, Disk, Network, Processes (real-time sparklines + 1h chart) | `/api/v1/data?chart=system.cpu` |
| **Containers Page** | Per-container CPU/RAM/Network mini-charts | `/api/v1/data?chart=docker.container_net` |
| **Capacity Trending** | 7d/30d CPU, RAM, Disk trend (feeds into future PRD) | `/api/v1/data?after=-3600&points=60` |
| **Alerts** | Netdata alarm events → Anjungan notification | Netdata webhook |

**Netdata Metrics Available for Integration:**
- `system.cpu` — CPU utilization per-core (user, system, iowait, softirq, irq, guest)
- `system.ram` — RAM used, cached, buffers, free
- `system.diskio` — Disk I/O (read/write KB/s, operations/s)
- `system.net` — Network traffic (in/out KB/s, packets, errors, drops)
- `system.uptime` — Server uptime
- `system.load` — Load average (1m, 5m, 15m)
- `system.ram` — Available RAM percentage
- `disk.space` — Disk usage per mount point
- `net.packets` — Network packets per second
- `processes` — Running processes count
- `docker.container_cpu` — Per-container CPU
- `docker.container_mem` — Per-container memory usage
- `docker.container_net` — Per-container network

**Effort Breakdown:**
| Task | Effort |
|------|--------|
| Deploy Netdata agent on servers (docker-compose) | 0.5d |
| Backend: Netdata HTTP client + chart query | 1d |
| Backend: Metrics summary endpoint | 0.5d |
| Frontend: Server detail "Metrics" tab | 1d |
| Frontend: Containers page metric badges | 0.5d |
| Frontend: Dashboard health StatCard | 0.25d |
| Notification integration (Netdata alarms → webhook) | 0.5d |
| **Total** | **~4.25d** |

**Design Note:** Netdata API returns SVG charts natively. For quick implementation, **embed Netdata SVG directly via iframe or `<img>` tag** (zero frontend chart code). For deeper integration (custom styling, combined views), parse the JSON data and render via Chart.js/D3.

```
// Quick integration — embedded SVG
<img src="http://server:19999/api/v1/badge.svg?chart=system.cpu" />

// Deep integration — JSON data
GET http://server:19999/api/v1/data?chart=system.cpu&after=-300&points=60&format=json
→ { labels: [...], data: [...], ... }
```

---

### ☁️ Cloud Engineering

| # | Tool | Function | Integration Pattern | Has PRD? | Effort | Priority |
|---|------|----------|-------------------|----------|--------|----------|
| C1 | **OpenTofu State Viewer** | IaC state visibility | A (state file HTTP/parse) | ❌ | ⭐⭐ | P2 |
| C2 | **CloudQuery** | Multi-cloud asset inventory | A (CloudQuery API/DB) | ❌ | ⭐⭐⭐ | P4 |

---

### 🏗️ Infrastructure Engineering

| # | Tool | Function | Integration Pattern | Has PRD? | Effort | Priority |
|---|------|----------|-------------------|----------|--------|----------|
| I1 | **NetBox** | DCIM, IPAM, devices, cables | A (NetBox REST API) | ❌ | ⭐⭐⭐⭐ | P3 |
| I2 | **Ansible Semaphore** | Runbook automation / playbook runner | A (Semaphore API) | ❌ | ⭐⭐ | P2 |
| I3 | **Headscale / WG Easy** | VPN peer management | A (API) | ❌ | ⭐⭐ | P3 |

#### I2 — Ansible Semaphore Integration Detail

**What it shows in Anjungan:**
- **Runbooks** page (separate sidebar item)
  - List of playbooks from Semaphore
  - "Run" button → parameter form → execute
  - Real-time output stream (task-by-task)
  - Execution history (who ran what, when, result)

**Sidebar placement:**
```
Ops
├── Deployments (existing)
├── Uptime (existing)
├── Notifications (existing)
├── Runbooks (new — Ansible Semaphore)
└── Security Events (future)
```

**Integration Points with Existing Features:**
- Script Library PRD (PRD-script-library.md) → Runbooks could be the execution engine
- Audit Log → all Runbook executions logged
- Notification Targets → alert on runbook failure
- Server list → select target server for playbook

---

### 💻 Software Development

| # | Tool | Function | Integration Pattern | Has PRD? | Effort | Priority |
|---|------|----------|-------------------|----------|--------|----------|
| W1 | **N8N** | Workflow automation | A (N8N REST API) | ❌ | ⭐⭐⭐ | P2 |
| W2 | **Minio** | S3-compatible object storage | A (S3 API) | ❌ | ⭐ | P3 |
| W3 | **Outline / BookStack** | Documentation | A (API) | ❌ | ⭐⭐ | P4 |

---

## 4. Phased Roadmap

### Phase 1: Foundation & Visibility (Current — Q3 2026)

Focus: **see everything clearly** — metrics, events, activity.

| Order | Feature | Domain | Days | Depends On | PRD |
|-------|---------|--------|------|-----------|-----|
| 1 | **Netdata integration** | SRE | 4.25d | Netdata deployed on servers | ❌ New |
| 2 | **Security Events (CrowdSec)** | Security | 6-9d | CrowdSec deployed | ✅ PRD-security-events.md |
| 3 | **Login Activity** | Security | 4-6d | — | ✅ PRD-login-activity.md |
| 4 | **Container Security Posture** | Security | 6-9d | — | ✅ PRD-container-security.md |

**Phase 1 total:** ~20-28 days

### Phase 2: Security Scanning (Q3-Q4 2026)

Focus: **find vulnerabilities before attackers do**.

| Order | Feature | Domain | Days | Depends On | PRD |
|-------|---------|--------|------|-----------|-----|
| 5 | **Trivy Container Scanning** | Security | 6-9d | Agent infra | ✅ PRD-container-image-scanning.md |
| 6 | **TruffleHog Secret Scanning** | Security | 5-8d | Agent infra | ✅ PRD-secret-scanning.md |
| 7 | **Gitleaks CI Gate** | Security | 2d | Forgejo repos | ❌ New |
| 8 | **Renovate Dependency Dashboard** | DevOps | 2d | Renovate deployed | ❌ New |

**Phase 2 total:** ~15-27 days

### Phase 3: Automation & Runbooks (Q4 2026)

Focus: **do things automatically** — runbooks, workflows, CI.

| Order | Feature | Domain | Days | Depends On | PRD |
|-------|---------|--------|------|-----------|-----|
| 9 | **Ansible Semaphore / Runbooks** | Infra | 4-6d | Semaphore deployed | ❌ New |
| 10 | **N8N Workflow Automation** | Dev | 5-7d | N8N deployed | ❌ New |
| 11 | **Woodpecker CI Status** | DevOps | 2-3d | Woodpecker deployed | ❌ New |

**Phase 3 total:** ~11-16 days

### Phase 4: Enterprise & Polish (2027)

Focus: **enterprise compliance, secrets management, advanced infra**.

| Order | Feature | Domain | Days | Depends On | PRD |
|-------|---------|--------|------|-----------|-----|
| 12 | **OpenSCAP Enterprise Compliance** | Security | 4-5d | — | ❌ New |
| 13 | **Vault Secrets Management** | Security | 6-10d | Vault deployed | ❌ New |
| 14 | **NetBox IPAM/DCIM** | Infra | 6-8d | NetBox deployed | ❌ New |
| 15 | **Step CA Internal Certificates** | Security | 3-4d | — | ❌ New |

**Phase 4 total:** ~19-27 days

---

## 5. Sidebar Evolution

### Current (June 2026)

```
Dashboard          Infra               Artifact           Ops                 Security              Administration
└─ Overview         ├─ Servers           ├─ Registry        ├─ Deployments        ├─ SSL Monitors       ├─ Users
                    ├─ SSH Keys          └─ Repositories    ├─ Uptime             ├─ Compliance         ├─ Audit Log
                    └─ Containers                           └─ Notifications                             └─ Settings
```

### Phase 1 Target

```
Dashboard          Infra               Artifact           Ops                 Security              Administration
└─ Overview         ├─ Servers           ├─ Registry        ├─ Deployments        ├─ Security Events    ├─ Users
                    │  └─ [Metrics]      └─ Repositories    ├─ Uptime             ├─ Container Security ├─ Login Activity
                    ├─ SSH Keys                             ├─ Notifications      ├─ SSL Monitors       ├─ Audit Log
                    └─ Containers                                                └─ Compliance         └─ Settings
                    [Health Card]                                                                        ⋮ Settings
```

### Phase 2-3 Target

```
Dashboard          Infra               Artifact           Ops                 Security              Administration
└─ Overview         ├─ Servers           ├─ Registry        ├─ Deployments        ├─ Security Events    ├─ Users
                    │  └─ Metrics       │  └─ Scans        ├─ Uptime             ├─ Container Security ├─ Login Activity
                    ├─ SSH Keys          └─ Repositories    ├─ Notifications      ├─ Vulnerability      ├─ Audit Log
                    └─ Containers        [Dep Badges]       ├─ Runbooks            Scanning              └─ Settings
                    [Metrics Charts]                        └─ Automation         ├─ Secret Scanning
                                                                                  ├─ SSL Monitors
                                                                                  └─ Compliance
```

---

## 6. Architecture Diagram

```
                               Anjungan Backend
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────────────────────┐  │
│  │  API Handlers                                                                        │  │
│  │  ┌───────────┐ ┌───────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────┐   │  │
│  │  │ Security  │ │ Container │ │ Login    │ │ Metrics  │ │ Runbooks │ │ Dependencies│   │  │
│  │  │ Events    │ │ Security  │ │ Activity │ │ (Netdata)│ │ (Semaph.)│ │ (Renovate)  │   │  │
│  │  └─────┬─────┘ └─────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘ └──────┬──────┘   │  │
│  └────────┼─────────────┼────────────┼────────────┼────────────┼──────────────┼──────────┘  │
│           │             │            │            │            │              │             │
│  ┌────────▼─────────────▼────────────▼────────────▼────────────▼──────────────▼──────────┐  │
│  │  Integration Layer                                                                    │  │
│  │                                                                                       │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │  │
│  │  │CrowdSec  │ │Trivy/Tru │ │ Docker   │ │ Netdata  │ │ Semaphore│ │ Renovate      │  │  │
│  │  │ LAPI     │ │ffleHog   │ │ SSH      │ │ HTTP     │ │ API      │ │ API / Webhook │  │  │
│  │  │ Client   │ │SSH Runner│ │ Exec     │ │ Client   │ │ Client   │ │ Client        │  │  │
│  │  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘ └───────┬───────┘  │  │
│  └───────┼────────────┼────────────┼────────────┼────────────┼───────────────┼──────────┘  │
│          │            │            │            │            │               │             │
│  ┌───────▼────────────▼────────────▼────────────▼────────────▼───────────────▼──────────┐  │
│  │  Database (PostgreSQL)                                                                │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐  │  │
│  │  │security_ │ │container │ │auth_     │ │metrics_  │ │runbook_  │ │dep_health_   │  │  │
│  │  │events    │ │_security │ │events    │ │cache     │ │executions│ │snapshots     │  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────────┘  │  │
│  └──────────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                             │
└─────────────────────────────┬───────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│  Frontend (SvelteKit)                                                                      │
│                                                                                            │
│  Sidebar Categories                                                                        │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────────┐  │
│  │Dashboard│ │Infra     │ │Artifact  │ │Ops       │ │Security  │ │Administration      │  │
│  │ Overview│ │ Servers  │ │ Registry │ │Deploy    │ │Events    │ │ Users              │  │
│  │ Stat    │ │ Metrics  │ │ Scans    │ │Uptime    │ │Container │ │ Login Activity     │  │
│  │ Cards   │ │Container │ │ Repos    │ │Notif     │ │Vulnerab  │ │ Audit Log          │  │
│  │         │ │ SSH Keys │ │ Deps     │ │Runbooks  │ │Secrets   │ │ Settings           │  │
│  │         │ │          │ │          │ │Automation│ │SSL       │ │                    │  │
│  │         │ │          │ │          │ │          │ │Compliance│ │                    │  │
│  └─────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘ └────────────────────┘  │
│                                                                                            │
│  Shared Components: Chart.js / D3 / Netdata SVG embed / Notification Targets picker        │
└────────────────────────────────────────────────────────────────────────────────────────────┘

External Services / Agents:
┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐
│  CrowdSec  │ │  Netdata   │ │  Semaphore │ │  Renovate  │ │  Forgejo   │ │  NetBox    │
│  (LAPI)    │ │  Agent     │ │  Server    │ │  Server    │ │  (Git)     │ │  (DCIM)    │
└────────────┘ └────────────┘ └────────────┘ └────────────┘ └────────────┘ └────────────┘
```

---

## 7. Shared Infrastructure Dependencies

All these tools require infrastructure to be deployed before integration. This section tracks what needs to run where.

### Server Requirements

| Tool | Deployment | Resource Needed | Must be running? |
|------|-----------|----------------|------------------|
| Netdata | Docker per VPS | ~100MB RAM, <1% CPU | ✅ Before R1 integration |
| CrowdSec | Docker per VPS | ~150MB RAM, <2% CPU | ✅ Before S1 integration |
| Semaphore | Docker (central) | ~200MB RAM, 1GB disk | ✅ Before I2 integration |
| Renovate | Docker (central) | ~200MB RAM, cron-based | ✅ Before D1 integration |
| N8N | Docker (central) | ~300MB RAM, 1GB disk | ✅ Before W1 integration |
| NetBox | Docker (central) | ~500MB RAM, PostgreSQL | ✅ Before I1 integration |
| Vault | Docker (central) | ~200MB RAM, storage backend | ✅ Before S10 integration |
| Woodpecker | Docker (central) | ~200MB RAM, agent per server | ✅ Before D2 integration |

### MiniPC Capacity Analysis

Current MiniPC specs: **4c/8GB/512GB** + existing services (Dokploy, Forgejo, Zot, Anjungan stack)

| Service | RAM Estimate | Can fit? |
|---------|-------------|----------|
| Existing stack | ~3-4GB | ✅ Yes |
| Netdata (per VPS) | ~200MB | ✅ Yes |
| CrowdSec | ~150MB | ✅ Yes |
| Semaphore | ~200MB | ✅ Yes |
| Renovate | ~200MB | ✅ Yes (cron, not always on) |
| **Subtotal new** | **~750MB** | **✅ Comfortable** |

N8N, NetBox, Vault → better on separate VPS when ready.

---

## 8. Cross-Reference: Features Without PRDs

Tools marked ❌ in "Has PRD?" column need a PRD before implementation. This section tracks PRD creation status.

| Tool | PRD Needed | Priority for PRD | Assigned To |
|------|-----------|------------------|-------------|
| **Netdata Integration** | P0 — needs PRD before Phase 1 start | 🔴 High | — |
| **Renovate Dashboard** | P1 — needed before Phase 2 | 🟡 Medium | — |
| **Gitleaks CI Gate** | P1 — needed before Phase 2 | 🟡 Medium | — |
| **Ansible Semaphore** | P2 — needed before Phase 3 | 🟢 Low (time) | — |
| **N8N Workflow** | P2 — needed before Phase 3 | 🟢 Low (time) | — |
| **Woodpecker CI** | P2 — needed before Phase 3 | 🟢 Low (time) | — |
| **NetBox IPAM** | P3 — Phase 4 | 🟢 Low (time) | — |
| **Vault Secrets** | P3 — Phase 4 | 🟢 Low (time) | — |
| **Step CA** | P3 — Phase 4 | 🟢 Low (time) | — |
| **OpenSCAP** | P4 — Phase 4 | 🟢 Low (time) | — |
| **ClamAV** | P2 | 🟢 Low (time) | — |

---

## 9. Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-06-11 | **Netdata as P0** | Biggest blind spot in current infrastructure — no metrics at all. Directly enables Capacity Trending PRD. |
| 2026-06-11 | **Pattern A (API Consumer)** preferred for new integrations | Loose coupling — tools run independently, Anjungan queries. No SSH overhead, no agent maintenance. |
| 2026-06-11 | **Phase 1 = Visibility first** | Before automating or scanning, we need to see what's happening (metrics + events + activity). |
| 2026-06-11 | **Defer Vault to Phase 4** | Current secrets (webhook URLs, API keys) stored in DB — not ideal but acceptable risk for current scale. Vault introduces significant ops overhead. |
| 2026-06-11 | **Netdata embedded SVG** for quick wins | Netdata generates SVG charts natively — can embed in Anjungan via `<img>` tag with zero frontend chart code. Upgrade to custom Chart.js later. |

---

## 10. PRD Cross-References

| Existing PRD | Domain | Phase | Integration Reference |
|-------------|--------|-------|----------------------|
| PRD-security-events.md | Security | 1 | CrowdSec → Security Events page |
| PRD-container-security.md | Security | 1 | Container runtime posture checks |
| PRD-login-activity.md | Security | 1 | Auth security monitoring |
| PRD-container-image-scanning.md | Security | 2 | Trivy vulnerability scanning |
| PRD-secret-scanning.md | Security | 2 | TruffleHog secret detection |
| PRD-compliance.md | Security | ✅ Done | CIS/Lynis/Container Security |
| PRD-ssl-monitoring.md | Security | ✅ Done | SSL cert monitoring |
| PRD-uptime-monitoring.md | SRE | 🟡 Active | HTTP/TCP health checks |
| PRD-incidents-timeline.md | SRE | P3 (not yet created) | Correlated event timeline |
| PRD-capacity-trending.md | SRE | P3 (not yet created) | Metrics-driven capacity planning |
| PRD-script-library.md | Infra | P2 (not yet created) | Runbook execution (Semaphore candidate) |

---

## 11. Appendices

### A. Tool Maturity Assessment

```
                 HIGH IMPACT
                     │
      Phase 2   ● Trivy      ● Netdata   Phase 1
                ● TruffleHog  ● CrowdSec
                ● Renovate    ● Container Sec
                     │            │
    LOW EFFORT ──────┼────────────┼────── HIGH EFFORT
                     │            │
                ● Gitleaks   ● Semaphore  Phase 3
                ● ClamAV     ● N8N
                ● Woodpecker ● OpenSCAP
                     │            │
                ● Step CA    ● Vault      Phase 4
                ● NetBox
                     │
                 LOW IMPACT
```

### B. Related Tools Not Yet Evaluated

| Tool | Domain | Notes |
|------|--------|-------|
| **Wazuh** (SIEM/XDR) | Security | Full SIEM — too heavy for MiniPC scale |
| **Grafana Loki** | SRE | Log aggregation — PRD needed, separate from metrics |
| **Kuma** (Service Mesh) | Infra | Over-engineering for Docker Compose setup |
| **Kyverno / OPA** | Security | K8s policy engine — not applicable |
| **Teleport** | Infra | SSH access management — may be useful later |
| **Portainer** | DevOps | Container management — Anjungan already covers server+container |
| **Harbor** | Artifact | Full registry — Anjungan already has Zot integration |

---

## 12. References

- [PRD-security-events.md](./PRD-security-events.md) — CrowdSec Security Events
- [PRD-container-security.md](./PRD-container-security.md) — Container Runtime Security
- [PRD-login-activity.md](./PRD-login-activity.md) — Auth Security Monitoring
- [PRD-container-image-scanning.md](./PRD-container-image-scanning.md) — Trivy Vulnerability Scanning
- [PRD-secret-scanning.md](./PRD-secret-scanning.md) — TruffleHog Secret Scanning
- [PRD-uptime-monitoring.md](./PRD-uptime-monitoring.md) — Uptime Monitoring (shared notification pattern)
- [PRD-ssl-monitoring.md](./PRD-ssl-monitoring.md) — SSL Certificate Monitoring
- [PRD-compliance.md](./PRD-compliance.md) — Compliance & Security Scanning
- [Netdata REST API v1](https://learn.netdata.cloud/docs/agent/api/v1)
- [CrowdSec LAPI Documentation](https://doc.crowdsec.net/docs/references/lapi/)
- [Ansible Semaphore API](https://docs.ansible-semaphore.com/)
- [Renovate API](https://docs.renovatebot.com/)
- [Woodpecker CI API](https://woodpecker-ci.org/docs/api)
- [NetBox REST API](https://netbox.dev/)
- [N8N REST API](https://docs.n8n.io/api/)
