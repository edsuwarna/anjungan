# Anjungan — PRD: Compliance & Security Scanning

> **Version:** 2.0
> **Status:** ✅ Fully Implemented — CIS L1/L2, Docker, Lynis, Container Security all done
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Production servers must comply with security standards — CIS (Center for Internet Security) Benchmark. But checking 128+ configurations manually on each server is:

- **Manual** — SSH into the server, run commands one by one, record the results
- **Inconsistent** — different people, different interpretations
- **No history** — fixed it yesterday, forgot to check again next week
- **No trend** — hardening index up/down? Which service is most vulnerable?

Yet there are tools like **Lynis** (auto system audit) and **CIS benchmark scripts** that can automate most of it. But the output is still raw — JSON/terminal only.

**Compliance solves this:**
- **Single dashboard** — view all server scores from a single screen
- **Automated scanning** — trigger from UI, execute via SSH, auto-parse
- **80+ checks across 9 categories** — CIS L1, L2, Docker, Container Security
- **History & trend** — know if the hardening score goes up or down over time
- **Container security** — 10 runtime checks per container (privileged, root, capabilities)

### Current Status (June 2026)

| Domain | Status | Detail |
|--------|--------|--------|
| CIS Level 1 (SSH, Kernel, FS, Users, Services, Network, Logging) | ✅ **Fully implemented** | 58 checks across 7 categories |
| CIS Docker Benchmark | ✅ **Fully implemented** | 22 checks across 6 sections |
| Lynis Integration | ✅ **Fully implemented** | SSH-based runner with parser |
| Container Security | ✅ **Fully implemented** | 10 runtime checks per container |
| Compliance Dashboard | ✅ **Fully implemented** | KPI cards, benchmark cards, server list |
| Scan History | ✅ **Fully implemented** | Per-server, per-category, global |
|| **Scheduled Scans** | ❌ **Not implemented** | Planned — Phase 4 |
| **Compliance Report Export** | ❌ **Not implemented** | Planned — Phase 4 |

### Target Audience

- **Endang** (platform engineer) — enforce security baseline, track hardening progress
- **Auditor / Compliance Officer (future)** — export report for auditing

### Goals

| Goal | Metric |
|------|--------|
| CIS L1 compliance score per server | ✅ Tracked (0-100%) |
| CIS Docker compliance score | ✅ Tracked |
| Lynis hardening index | ✅ Tracked |
| Container security score per container | ✅ Tracked |
|| Automated scan from UI | ✅ < 3s trigger |

---

## 2. Product Overview

### Architecture

```
Anjungan Backend                          Target Server
┌──────────────────────────┐             ┌────────────────────┐
│ Compliance Handler       │             │                    │
│ POST /scan ──────SSH────▶│             │ - Lynis            │
│ GET /history ◀───parse───│             │ - Docker           │
│ GET /summary  ◀──────────│             │ - CIS checks       │
│                          │             │ - Container inspect│
│ Scanner Engine           │             │                    │
│ ├─ RunCISL1()            │             └────────────────────┘
│ ├─ RunCISL2()                                        
│ ├─ RunCISDocker()                                        
│ ├─ RunLynis()                                            
│ ├─ RunContainerSecurity() ──SSH──▶ Container runtime
│ Check Registry (80+ checks)
│ ├─ checks_ssh.go       (18 checks)
│ ├─ checks_kernel.go     (8 checks)
│ ├─ checks_fs.go         (8 checks)
│ ├─ checks_users.go      (7 checks)
│ ├─ checks_services.go   (6 checks)
│ ├─ checks_network.go    (5 checks)
│ ├─ checks_logging.go    (3 checks)
│ └─ checks_docker.go    (22 checks)
│
│ DB: scan_results + scan_findings
└──────────────────────────┘
```

### Scanning Flow

```
1. User clicks "Run Scan" → select a profile (cis_l1 / cis_l2 / cis_docker / lynis)
2. Backend SSH into target server
3. Execute commands according to profile:
   - CIS L1: 58 individual checks via SSH
   - CIS Docker: `docker ps`, `docker inspect`, `stat`, etc.
   - Lynis: `lynis audit system --quick --json` + parse output
   - Container: `docker inspect {container}` + `docker exec {container} ...`
4. Parse output → structured format
5. Save to scan_results + scan_findings
6. Return results to frontend
```

### Compliance Check Categories

```
CIS Level 1 (Linux)          CIS Docker                    Container Security
├── 🔒 SSH (18 checks)       ├── Host Config (2)           ├── Privileged mode
├── ⚙️ Kernel (8 checks)      ├── Daemon Config (4)        ├── Root user
├── 📁 Filesystem (8 checks)  ├── Daemon Files (3)        ├── Seccomp profile
├── 👥 Users (7 checks)      ├── Images & Build (6)       ├── Capabilities
├── ⚡ Services (6 checks)    ├── Container Runtime (5)    ├── Read-only rootfs
├── 🌐 Network (5 checks)    └── Swarm Ops (2)            ├── Host network
├── 📋 Logging (3 checks)                                ├── Port mapping
                                                         ├── Resource limits
Lynis: System audit helper                                ├── Health check
├── Hardening index (0-100)                               └── AppArmor/SELinux
├── Warnings + suggestions
└── Raw JSON output
```

---

## 3. Feature Specifications

> **Legend:** ✅ Implemented | 🟡 Partial | 🔴 Planned

### F1 — Compliance Scanner Engine

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
|| **Backend** | `internal/compliance/scanner.go` — scanner engine. `Run()` — execute profile (all/cis_l1/cis_l2/cis_docker/container). `RunSingle()` — single check by ID. `RunLynis()` — SSH + `lynis audit system` + parse JSON output. Parallel execution for independent checks. SSH connection pooling. Timeout per check: 30s. |
|| **Data** | Values directly from SSH output → `scan_findings` table. Lynis: hardening index, warnings, suggestions. |

### F2 — CIS Level 1 & Level 2 (Linux Benchmark)

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
|| **Implementation** | 58 checks across 7 categories: SSH (18), Kernel (8), Filesystem (8), Users (7), Services (6), Network (5), Logging (3). Each check has: ID (CIS standard ID), title, description, risk, remediation, category, severity (high/medium/low), profile (L1/L2). Execute via SSH `RunCommand`. Scoring: PASS/FAIL/NA per check, percentage per category, overall. |
|| **Frontend** | **Compliance Dashboard** (`/compliance`) — KPI cards (scanned servers, total checks, overall score, compliance trend). Benchmark cards (CIS L1, CIS L2, CIS Docker) — each card: score circular gauge, server list, last scan time. **Detail page** (`/compliance/cis-level-1`) — score gauge, category breakdown (expandable), per-check table (expandable: risk, remediation, raw output). |
|| **UX** | Score circular gauge: 🟢 >80, 🟡 60-80, 🔴 <60. Category cards: name, pass/fail count, click → scroll to table. Per-check: expandable panel — CIS ID, title, description, severity badge, pass/fail label, remediation text, raw command output copyable. |

### F3 — CIS Docker Benchmark

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
|| **Implementation** | `internal/compliance/checks_docker.go` — 653 lines. 22 checks across 6 sections: Host Config (2), Daemon Config (4), Daemon Files (3), Images & Build (6), Container Runtime (5), Swarm Ops (2). Commands: `docker ps`, `docker inspect`, `stat`, `docker version`, etc. Auto-detect if Docker not installed → skip all. |
|| **Frontend** | Route `/compliance/cis-docker`. Same format as CIS L1/L2: score gauge, category breakdown, per-check detail. Quick action from server detail page. |
|| **UX** | If Docker is not installed: "Docker not available on this server" card — no error, just skip. If Docker is installed but no containers are running: some checks still run (daemon config, host config). |

### F4 — Lynis System Audit

| | |
|---|---|
| **Priority** | P1 |
| **Status** | ✅ **Done** |
|| **Backend** | `internal/compliance/lynis.go` — 140 lines. SSH to server: `lynis audit system --quick --json`. Parse JSON output → hardening_index, warnings, suggestions, tests passed/failed. Lynis auto-detect: if not installed → return error + install URL. |
|| **Frontend** | Route `/compliance/lynis`. Hardening index gauge (0-100). Warnings list: severity, description, suggestion. Suggestions list: category, suggestion text. Raw JSON toggle. |
|| **UX** | Hardening index: 🟢 >70, 🟡 50-70, 🔴 <50. Warnings grouped by severity (high/medium/low). Suggestions with copy button. "Install Lynis" button if not yet installed → link to `https://github.com/cisofy/lynis`. |

### F5 — Container Security Scanner (Runtime)

| | |
|---|---|
| **Priority** | P1 |
| **Status** | ✅ **Done** |
|| **Backend** | `internal/compliance/container_scanner.go` — 598 lines. 10 runtime checks per container: Privileged mode, Root user, Seccomp profile, Capabilities (NET_ADMIN, SYS_ADMIN, etc.), Read-only rootfs, Host network, Port mapping (0.0.0.0), Resource limits, Health check, AppArmor/SELinux. Checks performed via `docker inspect` + `docker exec`. Scoring per container + grouped per server. |
|| **Frontend** | Route `/containers/{serverId}/{containerId}/security` — 1247 lines. Score gauge. Findings table: check name, status (PASS/FAIL), description, remediation. Remediation commands copyable. Per-container security trend (if historical data exists). |
|| **UX** | FAIL items highlighted in red. Remediation command auto-copy. Re-scan button. "Fix all" — show all remediation commands in sequence. |

### F6 — Compliance Dashboard & History

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
|| **Backend** | `GET /api/v1/compliance/summary` — aggregate: total servers scanned, total checks, pass/fail count, overall score, trend. `GET /api/v1/compliance/{serverID}/history` — per-server scan history: score, profile, timestamp, trend arrow. `GET /api/v1/compliance/{serverID}/latest/categories` — category breakdown of latest scan. **18+ DB repository methods** — CreateScanResult, GetLatestScanResult, GetScanResultWithFindings, ListScanResults, GetComplianceSummary, ListGlobalScanHistory, ListActiveScans, GetCategoryBreakdowns, etc. |
|| **Frontend** | Route `/compliance` — 503 lines. **KPI cards** (top row): scanned servers, total checks, overall compliance score, trend. **Benchmark cards**: CIS L1, CIS L2, CIS Docker, Lynis — each card has a score gauge + "View Details" + last scanned. **Server list**: table per server with score, last scan time, scan button. Active scans indicator. |
|| **UX** | KPI cards with animated numbers. Score gauge animated ring. Trend arrow 📈/📉/➡️. Hover gauge → tooltip exact percentage. Scanning in progress → spinner + "Scanning..." badge — polling status. |

---

## 4. Future Roadmap

### F7 — Scheduled Scans (🔴 Planned)

| | |
|---|---|
|| **Backend** | `compliance_schedules` table: server_id, profile, cron expression, notification config. Background scheduler via asynq. Generate scan result + notification (Telegram) if score drops. |
|| **Frontend** | Schedule editor: select server + profile + cron + notification. Schedule list: server, profile, next run, last run. Enable/disable toggle. |

### F8 — Compliance Report (PDF Export) (🔴 Planned)

| | |
|---|---|
|| **Backend** | `GET /api/v1/compliance/report?server_id=&period=month&format=pdf` — Generate PDF report: score summary, category breakdown, top failures, remediation recommendations, trend chart. Use Go `chromedp` or wkhtmltoimage + HTML template. |
|| **Frontend** | "Export Report" button in compliance dashboard. Format: PDF, CSV, JSON. Option: include remediation steps, include recommendations. |

### F9 — Compliance Trend Graph (🔴 Planned)

| | |
|---|---|
|| **Backend** | `GET /api/v1/compliance/trends?server_id=&period=90d` — time series score per scan. Aggregated per day/week/month. |
| **Frontend** | Line chart: X = scan date, Y = score %. Multiple lines: CIS L1, L2, Docker. Tooltip: exact score + scan count. |

### F10 — Kubernetes Compliance (🔴 Planned — Future)

| | |
|---|---|
|| **Backend** | K8s checks: kube-bench integration or custom CIS K8s checks (8 categories, 100+ checks). SSH to K8s node or via kubectl. |
|| **Frontend** | New tab in compliance: "Kubernetes" — same format as CIS L1/L2. |

---

## 5. API Design

### Existing Endpoints (Implemented)

```go
// === Compliance (Implemented) ===
GET    /api/v1/compliance/summary
GET    /api/v1/compliance/checks
GET    /api/v1/compliance/history
GET    /api/v1/compliance/active
GET    /api/v1/compliance/{serverID}/latest
GET    /api/v1/compliance/{serverID}/latest/categories
POST   /api/v1/compliance/{serverID}/scan              // profiles: all, cis_l1, cis_l2, cis_docker
POST   /api/v1/compliance/{serverID}/scan/lynis
POST   /api/v1/compliance/{serverID}/scan/docker
POST   /api/v1/compliance/{serverID}/scan/containers
POST   /api/v1/compliance/{serverID}/scan/containers/{containerID}
POST   /api/v1/compliance/{serverID}/scan/check/{checkID}
GET    /api/v1/compliance/{serverID}/history
GET    /api/v1/compliance/{serverID}/history/{scanID}
GET    /api/v1/compliance/{serverID}/history/categories/{category}
GET    /api/v1/compliance/{serverID}/containers/{containerName}/history
```

### Future Endpoints

```go
// === Future: Scheduled Scans ===
GET    /api/v1/compliance/schedules
POST   /api/v1/compliance/schedules
PUT    /api/v1/compliance/schedules/{id}
DELETE /api/v1/compliance/schedules/{id}

// === Future: Reports ===
GET    /api/v1/compliance/report?server_id=&format=pdf&period=month

// === Future: Trends ===
GET    /api/v1/compliance/trends?server_id=&period=90d
```

---

## 6. Database Schema

### Existing Tables

```sql
-- 000009: Scan results
CREATE TABLE scan_results (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  server_id UUID REFERENCES servers(id),
  scan_type VARCHAR(50) NOT NULL,          -- compliance, lynis, docker, container
  profile VARCHAR(50),                     -- all, cis_l1, cis_l2, cis_docker
  status VARCHAR(20) DEFAULT 'pending',    -- pending, running, completed, failed
  overall_score DECIMAL(5,2),
  total_checks INTEGER,
  passed_checks INTEGER,
  failed_checks INTEGER,
  na_checks INTEGER DEFAULT 0,
  raw_output TEXT,                         -- Lynis JSON or docker raw output
  error_message TEXT,                         -- if failed
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMP DEFAULT NOW()
);

-- 000009: Scan findings
CREATE TABLE scan_findings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scan_id UUID REFERENCES scan_results(id),
  check_id VARCHAR(100),
  category VARCHAR(100),
  title VARCHAR(500),
  description TEXT,
  risk TEXT,
  remediation TEXT,
  severity VARCHAR(20),                   -- critical, high, medium, low, info
  status VARCHAR(20),                      -- PASS, FAIL, NA
  raw_command TEXT,
  raw_output TEXT,
  created_at TIMESTAMP DEFAULT NOW()
);
```

---

## 7. UX Flow

### Existing Flow: Run Scan + View Results

```
1. Open /compliance
2. View KPI: "4 servers scanned", "58 checks", "Overall 72% 🟠", "Trend +3%"
3. Benchmark cards: CIS L1 (82% 🟢), CIS L2 (65% 🟡), CIS Docker (90% 🟢)
4. Click "CIS L1" → /compliance/cis-level-1
5. Score gauge: 82% (🟢). Category breakdown:
   🔒 SSH       95%  (17/18 passes)  [expand ▶]
   ⚙️ Kernel     75%  (6/8 passes)   [expand ▶]
   👥 Users      71%  (5/7 passes)   [expand ▶]
6. Click "Users" → scroll to table:
   ┌───┬─────────────┬──────┬────────┐
   │ # │ Check       │ Status│ Remed. │
   ├───┼─────────────┼──────┼────────┤
   │ 1 │ 5.3.1 SSH    │ PASS  │        │
   │ 2 │ 5.3.2 UID 0  │ FAIL  │ [Copy] │
   └───┴─────────────┴──────┴────────┘
7. Click FAIL row → expand: "Check 5.3.2: Ensure root is the only UID 0 account"
   Risk: Multiple accounts with UID 0 grants root privileges
   Remediation: `usermod -u <new_uid> <username>`
   Raw output: `awk -F: '($3 == 0) {print}' /etc/passwd`
8. Click "Copy" → copy remediation command
9. Top: "Run New Scan" → select CIS L2 → trigger → progress spinner
```

### Flow: Container Security Check

```
1. Open /containers → click "anjungan-backend" container
2. Tab "Security" → /containers/{server}/{id}/security
3. Score gauge: 70% 🟡
4. Findings:
   ✅ Root user       → FAIL  → container runs as root
   ✅ Privileged mode → PASS
   ✅ Seccomp         → FAIL  → no seccomp profile
   ✅ Capabilities    → PASS
   ✅ Read-only FS    → FAIL  → writable rootfs
   ✅ Resource limits → PASS  → CPU 0.5, RAM 512MB
5. Click "Root user" → detail: container runs as UID 0
   Fix: Add `USER appuser` to Dockerfile + recreate
   Remediation command copyable
```

---

## 8. Implementation Roadmap

### 🟢 Phase 1 — Foundation (✅ Done)

**Goal:** Basic CIS scanning + compliance dashboard

| Order | Feature | Status | Effort |
|-------|---------|--------|--------|
|| 1 | DB schema (scan_results, scan_findings) | ✅ Done | 0.5 day |
|| 2 | Scanner engine (SSH runner) | ✅ Done | 2 days |
|| 3 | SSH checks (18 checks) | ✅ Done | 2 days |
|| 4 | Kernel + FS + Users checks (23 checks) | ✅ Done | 2 days |
|| 5 | Services + Network + Logging checks (14 checks) | ✅ Done | 1 day |
|| 6 | Compliance dashboard + detail pages | ✅ Done | 3 days |

### 🟢 Phase 2 — Docker + Lynis (✅ Done)

**Goal:** Container-specific security

| Order | Feature | Status | Effort |
|-------|---------|--------|--------|
|| 7 | CIS Docker checks (22 checks) | ✅ Done | 2 days |
|| 8 | Lynis integration (SSH runner + parser) | ✅ Done | 1.5 days |
|| 9 | Lynis frontend page | ✅ Done | 1 day |
|| 10 | CIS Docker frontend page | ✅ Done | 1 day |

### 🟢 Phase 3 — Container Security (✅ Done)

**Goal:** Runtime container scanning

| Order | Feature | Status | Effort |
|-------|---------|--------|--------|
|| 11 | Container security scanner (10 checks) | ✅ Done | 2 days |
|| 12 | Container security frontend (1247 lines) | ✅ Done | 2 days |
|| 13 | Scan history + trend | ✅ Done | 1 day |

### 🔴 Phase 4 — Scheduled Scans & Reporting (Planned)

**Goal:** Automated scheduling + trend analysis + export

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
|| 14 | Scheduled scans engine | 2 days | Phase 1 |
|| 15 | Schedule editor UI | 1 day | #14 |
|| 16 | Compliance report PDF export | 2 days | — |
|| 17 | Trend graph (line chart 90d) | 1 day | — |
|| 18 | Score drop notification | 1 day | #14 |

---

## 9. Non-Functional Requirements

|| Requirement | Target | Status |
||-------------|--------|--------|
|| Scan execution (CIS, 58 checks) | < 30s per server | ✅ |
|| Lynis scan | < 60s per server | ✅ |
|| Container security scan (10 checks) | < 10s per container | ✅ |
|| Dashboard load | < 2s (4 servers) | ✅ |
|| History query (90 days) | < 500ms | ✅ |
|| Concurrent scans | 3 parallel | ✅ |
|| SSH connection timeout | 10s per check | ✅ |
|| DB cleanup | Auto-delete > 90 day results | 🟡 Manual |

---

## 10. Glossary

|| Term | Definition |
||------|------------|
|| **CIS Benchmark** | Center for Internet Security standard — best practice config for OS, Docker, K8s |
|| **CIS Level 1** | Basic security — recommended, minimal performance impact |
|| **CIS Level 2** | Defense-in-depth — stricter, may affect performance |
|| **CIS Docker** | Docker-specific benchmark — 22 checks across 6 sections |
|| **Lynis** | Open-source security auditing tool — hardening index, warnings, suggestions |
|| **Hardening Index** | Lynis score (0-100) — how hardened a system is |
|| **Scan Profile** | Scan category: all, cis_l1, cis_l2, cis_docker, lynis, container |
|| **Container Security** | Runtime security checks per container — 10 checks |
|| **Fix Rate** | Percentage of vulnerabilities that have a fix version available |

## 11. References

- [PRD.md](./PRD.md) — Main Anjungan PRD (Phase 3 Security & Governance)
- [PRD-registry.md](./PRD-registry.md) — Registry & image management
- [PRD-container-image-scanning.md](./PRD-container-image-scanning.md) — Trivy vulnerability scanning (separate PRD)
- [PRD-secret-scanning.md](./PRD-secret-scanning.md) — TruffleHog secret scanning (separate PRD)
- [DECISIONS.md](../docs/DECISIONS.md)
