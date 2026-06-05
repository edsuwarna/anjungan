# Anjungan — PRD: Compliance & Security Scanning

> **Version:** 2.0
> **Status:** Draft — ✅ Fully Implemented (Trivy: 🔴 Planned | TruffleHog: 🔴 Planned)
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Server production harus comply sama standard keamanan — CIS (Center for Internet Security) Benchmark. Tapi ngecek 128+ konfigurasi manual tiap server itu:

- **Manual** — SSH ke server, jalanin command satu-satu, catat hasilnya
- **Inconsistent** — beda orang beda interpretasi
- **No history** — kemarin udah fix, minggu depan lupa cek lagi
- **No trend** — hardening index naik/turun? Service mana yang paling rentan?

Padahal ada tools kayak **Lynis** (auto system audit) dan **CIS benchmark scripts** yang bisa automate sebagian besar. Tapi output-nya masih mentah — JSON/terminal doang.

**Compliance solving this:**
- **Single dashboard** — lihat score semua server dari satu layar
- **Automated scanning** — trigger dari UI, execute via SSH, parse otomatis
- **80+ checks across 9 categories** — CIS L1, L2, Docker, Container Security
- **History & trend** — tau hardening score naik/turun dari waktu ke waktu
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
| **Trivy Vulnerability Scanner** | ❌ **Not implemented** | Planned — Phase 4 (vulns + misconfig only) |
| **TruffleHog Secret Scanner** | ❌ **Not implemented** | Planned — Phase 4 (replaces Trivy secrets) |
| **Scheduled Scans** | ❌ **Not implemented** | Planned — Phase 4 |
| **Compliance Report Export** | ❌ **Not implemented** | Planned — Phase 4 |

### Target Audience

- **Endang** (platform engineer) — enforce security baseline, track hardening progress
- **Auditor / Compliance Officer (future)** — export report buat audit

### Goals

| Goal | Metric |
|------|--------|
| CIS L1 compliance score per server | ✅ Tracked (0-100%) |
| CIS Docker compliance score | ✅ Tracked |
| Lynis hardening index | ✅ Tracked |
| Container security score per container | ✅ Tracked |
| Automated scan dari UI | ✅ < 3 detik trigger |
| Real-time vulnerability scanning (Trivy) | 🔴 Planned |
| Real-time secret leak detection (TruffleHog) | 🔴 Planned |

---

## 2. Product Overview

### Arsitektur

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
│ └─ RunTruffleHog()       ──SSH/filesystem──▶ Repo / Container FS
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
1. User klik "Run Scan" → pilih profile (cis_l1 / cis_l2 / cis_docker / lynis)
2. Backend SSH ke server target
3. Eksekusi command sesuai profile:
   - CIS L1: 58 individual checks via SSH
   - CIS Docker: `docker ps`, `docker inspect`, `stat`, dll
   - Lynis: `lynis audit system --quick --json` + parse output
   - Container: `docker inspect {container}` + `docker exec {container} ...`
4. Parse output → format terstruktur
5. Simpan ke scan_results + scan_findings
6. Return hasil ke frontend
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
| **Backend** | `internal/compliance/scanner.go` — scanner engine. `Run()` — execute profile (all/cis_l1/cis_l2/cis_docker/container). `RunSingle()` — single check by ID. `RunLynis()` — SSH + `lynis audit system` + parse JSON output. Parallel execution untuk checks yang independen. SSH connection pooling. Timeout per check: 30s. |
| **Data** | Nilai langsung dari SSH output → `scan_findings` table. Lynis: hardening index, warnings, suggestions. |

### F2 — CIS Level 1 & Level 2 (Linux Benchmark)

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
| **Implementation** | 58 checks across 7 categories: SSH (18), Kernel (8), Filesystem (8), Users (7), Services (6), Network (5), Logging (3). Tiap check punya: ID (CIS standard ID), title, description, risk, remediation, category, severity (high/medium/low), profile (L1/L2). Execute via SSH `RunCommand`. Scoring: PASS/FAIL/NA per check, percentage per category, overall. |
| **Frontend** | **Compliance Dashboard** (`/compliance`) — KPI cards (scanned servers, total checks, overall score, compliance trend). Benchmark cards (CIS L1, CIS L2, CIS Docker) — tiap card: score circular gauge, server list, last scan time. **Detail page** (`/compliance/cis-level-1`) — score gauge, category breakdown (expandable), per-check table (expandable: risk, remediation, raw output). |
| **UX** | Score circular gauge: 🟢 >80, 🟡 60-80, 🔴 <60. Category cards: name, pass/fail count, click → scroll ke table. Per-check: expandable panel — CIS ID, title, description, severity badge, pass/fail label, remediation text, raw command output copyable. |

### F3 — CIS Docker Benchmark

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
| **Implementation** | `internal/compliance/checks_docker.go` — 653 lines. 22 checks across 6 sections: Host Config (2), Daemon Config (4), Daemon Files (3), Images & Build (6), Container Runtime (5), Swarm Ops (2). Commands: `docker ps`, `docker inspect`, `stat`, `docker version`, dll. Auto-detect if Docker not installed → skip all. |
| **Frontend** | Route `/compliance/cis-docker`. Sama format kayak CIS L1/L2: score gauge, category breakdown, per-check detail. Quick action dari server detail page. |
| **UX** | Kalo Docker ga terinstall: "Docker not available on this server" card — ga error, cuma skip. Kalo Docker install tapi ga ada container running: beberapa check tetep jalan (daemon config, host config). |

### F4 — Lynis System Audit

| | |
|---|---|
| **Priority** | P1 |
| **Status** | ✅ **Done** |
| **Backend** | `internal/compliance/lynis.go` — 140 lines. SSH ke server: `lynis audit system --quick --json`. Parse JSON output → hardening_index, warnings, suggestions, tests passed/failed. Lynis auto-detect: kalo belum install → return error + install URL. |
| **Frontend** | Route `/compliance/lynis`. Hardening index gauge (0-100). Warnings list: severity, description, suggestion. Suggestions list: category, suggestion text. Raw JSON toggle. |
| **UX** | Hardening index: 🟢 >70, 🟡 50-70, 🔴 <50. Warnings grouped by severity (high/medium/low). Suggestions with copy button. "Install Lynis" button kalo belum terinstall → link ke `https://github.com/cisofy/lynis`. |

### F5 — Container Security Scanner (Runtime)

| | |
|---|---|
| **Priority** | P1 |
| **Status** | ✅ **Done** |
| **Backend** | `internal/compliance/container_scanner.go` — 598 lines. 10 runtime checks per container: Privileged mode, Root user, Seccomp profile, Capabilities (NET_ADMIN, SYS_ADMIN, etc.), Read-only rootfs, Host network, Port mapping (0.0.0.0), Resource limits, Health check, AppArmor/SELinux. Check dilakukan via `docker inspect` + `docker exec`. Scoring per container + grouped per server. |
| **Frontend** | Route `/containers/{serverId}/{containerId}/security` — 1247 lines. Score gauge. Findings table: check name, status (PASS/FAIL), description, remediation. Remediation commands copyable. Per-container security trend (if historical data exists). |
| **UX** | FAIL items highlighted merah. Remediation command auto-copy. Re-scan button. "Fix all" — show all remediation commands in sequence. |

### F6 — Compliance Dashboard & History

| | |
|---|---|
| **Priority** | P0 |
| **Status** | ✅ **Done** |
| **Backend** | `GET /api/v1/compliance/summary` — aggregate: total servers scanned, total checks, pass/fail count, overall score, trend. `GET /api/v1/compliance/{serverID}/history` — per-server scan history: score, profile, timestamp, trend arrow. `GET /api/v1/compliance/{serverID}/latest/categories` — category breakdown of latest scan. **18+ DB repository methods** — CreateScanResult, GetLatestScanResult, GetScanResultWithFindings, ListScanResults, GetComplianceSummary, ListGlobalScanHistory, ListActiveScans, GetCategoryBreakdowns, dll. |
| **Frontend** | Route `/compliance` — 503 lines. **KPI cards** (top row): scanned servers, total checks, overall compliance score, trend. **Benchmark cards**: CIS L1, CIS L2, CIS Docker, Lynis — tiap card punya score gauge + "View Details" + last scanned. **Server list**: table per server dengan score, last scan time, scan button. Active scans indicator. |
| **UX** | KPI cards number animasi. Score gauge animated ring. Trend arrow 📈/📉/➡️. Hover gauge → tooltip exact percentage. Scanning in progress → spinner + "Scanning..." badge — polling status. |

---

## 4. Future Roadmap (Trivy + Enhancement)

### F7 — Trivy Vulnerability Scanner (🔴 Planned — Phase 4)

| | |
|---|---|
| **Priority** | P1 |
| **Status** | ❌ **Not implemented** |
| **Scope** | **Vulnerabilities + Misconfigurations only.** Secrets scanning **delegated to TruffleHog** (see F12). |
| **Backend** | Dua source scanning: (1) **CI/CD Webhook** — `POST /api/v1/trivy/webhook` menerima Trivy JSON dari GitHub Action, extract summary + misconfigs + vulns, simpan ke `trivy_scans` table. (2) **Live Scan** — SSH ke target → `docker run aquasec/trivy:latest image --format json IMAGE:TAG`, parse output, simpan dengan source=live. Backend parser bedain tiap `Result` dari Trivy JSON berdasarkan `Type` field: OS packages (alpine/debian/ubuntu), language deps (npm/gomod/pip), Dockerfile misconfig. |
| **Frontend** | Route `/containers/vulnerabilities`. **Dashboard per-image card**: nama image, source badge (CI/CD vs Live), severity count, trend bar. **Cross-image trends**: CVE yang ngaruh ke multiple images. **KPI bar**: total critical, high, fix rate, scan count (7d). **Scan Detail** dengan 2 sub-tab — Vulnerabilities (expandable CVE cards), Misconfigurations (Dockerfile lint), Raw JSON. **Live vs CI/CD Comparison**: dual pane highlighting discrepancy. |
| **UX** | Badge system: 🔴 OS vs 📦 Dep packages. Status filter (fixable/unfixable/NEW). Delta badge vs previous scan (🔺 Critical +1). Trend chart 30 scans: bar chart critical+high. |

### F8 — Scheduled Scans (🔴 Planned)

| | |
|---|---|
| **Backend** | `compliance_schedules` table: server_id, profile, cron expression, notification config. Background scheduler via asynq. Generate scan result + notifikasi (Telegram) kalo score turun. |
| **Frontend** | Schedule editor: select server + profile + cron + notification. Schedule list: server, profile, next run, last run. Enable/disable toggle. |

### F9 — Compliance Report (PDF Export) (🔴 Planned)

| | |
|---|---|
| **Backend** | `GET /api/v1/compliance/report?server_id=&period=month&format=pdf` — Generate PDF report: score summary, category breakdown, top failures, remediation recommendations, trend chart. Gunakan Go `chromedp` atau wkhtmltoimage + HTML template. |
| **Frontend** | "Export Report" button di compliance dashboard. Format: PDF, CSV, JSON. Option: include remediation steps, include recommendations. |

### F10 — Compliance Trend Graph (🔴 Planned)

| | |
|---|---|
| **Backend** | `GET /api/v1/compliance/trends?server_id=&period=90d` — time series score per scan. Aggregasi per day/week/month. |
| **Frontend** | Line chart: X = scan date, Y = score %. Multiple lines: CIS L1, L2, Docker. Tooltip: exact score + scan count. |

### F11 — Kubernetes Compliance (🔴 Planned — Future)

| | |
|---|---|
| **Backend** | K8s checks: kube-bench integration atau custom CIS K8s checks (8 categories, 100+ checks). SSH ke K8s node atau via kubectl. |
| **Frontend** | Tab baru di compliance: "Kubernetes" — sama format kayak CIS L1/L2. |

### F12 — TruffleHog Secret Scanner (🔴 Planned — Phase 4)

| | |
|---|---|
| **Priority** | P1 |
| **Status** | ❌ **Not implemented** |
| **Scope** | **Replaces Trivy secrets.** Full-spectrum secret detection — 700+ detectors, verification engine, git history forensics. |
| **Backend** | Tiga mode scanning: (1) **Git Scan** — `trufflehog git https://github.com/org/repo --json --no-verification`, scan repos via SSH atau GitHub API. (2) **Filesystem Scan** — `trufflehog filesystem --directory=/path --json`, scan container filesystem atau workspace mount. (3) **Webhook Receiver** — `POST /api/v1/secrets/webhook` menerima TruffleHog JSON output (dari GitHub Action / CI pipeline). Backend parser bedain **verified** (credential tested live via API) vs **unverified** (detected but not confirmed). |
| **Verification** | **Unik vs secret scanner lain** — TruffleHog verification engine langsung nyoba credential ke API target (AWS, GitHub, Slack, Discord, 100+ services). Hasil: `verified: true/false`. Verified = **critical severity**, immediate notification. |
| **Data Model** | Scan results: server_id/repo_url, findings array (file path, line number, detector name, verified status, raw value truncated). Historical leak tracking via git blame: secret yang udah dihapus dari file tapi masih ada di commit history. |
| **Frontend** | Route `/secrets`. **Dashboard**: total scans, verified leaks count, unverified findings, trend. **Repo/Server cards**: recent findings, severity pie. **Detail view**: finding card — detector name (e.g. AWS Access Key), file path + line, verified badge, raw snippet (masked), first commit date, commit SHA. Filters: verified/unverified, detector type, repo. **Historical Leaks tab**: timeline — secret masuk commit → detected → fixed. |
| **UX** | **Verified badge**: 🟢 verified (critical) vs 🟡 unverified (medium). Detector icons per category (AWS, GitHub, Slack, Generic). Raw snippet truncated + mask toggle (show/hide). "Scan Repo" button from repo detail page. |

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
// === Future: Trivy ===
POST   /api/v1/trivy/webhook                    // Receive Trivy JSON from CI/CD
GET    /api/v1/trivy/scans                      // List all scans (?image=&source=&limit=)
GET    /api/v1/trivy/scans/{id}                 // Detail: vulns, misconfigs, raw
GET    /api/v1/trivy/scans/latest/{image}       // Latest scan per image
GET    /api/v1/trivy/scans/{id}/compare/prev    // Delta vs previous scan
POST   /api/v1/trivy/live-scan                 // Trigger live scan: {server_id, image, tag}

// === Future: TruffleHog Secrets ===
POST   /api/v1/secrets/webhook                 // Receive TruffleHog JSON from CI/CD
POST   /api/v1/secrets/scan                    // Trigger scan: {type: git|filesystem, target, server_id?}
GET    /api/v1/secrets/scans                   // List all scans (?repo=&status=&limit=)
GET    /api/v1/secrets/scans/{id}              // Detail: findings, verified status
GET    /api/v1/secrets/findings                // List findings (?verified=&detector=&repo=)
GET    /api/v1/secrets/summary                 // Stats: total verified, unverified, by detector
GET    /api/v1/secrets/history                 // Historical leak timeline

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
  error_message TEXT,                      -- kalo failed
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

### Future Tables

```sql
-- Future: Trivy scans
CREATE TABLE trivy_scans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  image_name VARCHAR(255) NOT NULL,
  image_tag VARCHAR(255),
  source VARCHAR(50) DEFAULT 'ci',          -- ci, live
  server_id UUID REFERENCES servers(id),   -- null for CI scans
  scan_number INTEGER NOT NULL,
  commit_sha VARCHAR(40),
  branch VARCHAR(255),
  workflow_url TEXT,
  summary JSONB,                            -- {critical: N, high: N, medium: N, low: N}
  misconfigs JSONB,                         -- Dockerfile lint findings
  raw_results JSONB,                        -- Full Trivy output (vulns + misconfigs)
  created_at TIMESTAMP DEFAULT NOW()
);

-- Future: TruffleHog secret scans
CREATE TABLE trufflehog_scans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scan_type VARCHAR(20) NOT NULL,           -- git, filesystem, webhook
  target VARCHAR(500) NOT NULL,             -- repo URL or filesystem path
  server_id UUID REFERENCES servers(id),   -- null for CI/webhook scans
  status VARCHAR(20) DEFAULT 'pending',     -- pending, running, completed, failed
  total_findings INTEGER DEFAULT 0,
  verified_count INTEGER DEFAULT 0,
  unverified_count INTEGER DEFAULT 0,
  raw_output JSONB,                         -- Full TruffleHog JSON output
  error_message TEXT,
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMP DEFAULT NOW()
);

-- Future: TruffleHog findings
CREATE TABLE trufflehog_findings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  scan_id UUID REFERENCES trufflehog_scans(id),
  detector_name VARCHAR(200) NOT NULL,      -- e.g. AWS Access Key, GitHub PAT
  detector_type VARCHAR(100),               -- category: AWS, GitHub, Slack, Generic
  verified BOOLEAN DEFAULT FALSE,           -- TruffleHog verification engine result
  file_path TEXT,
  line_number INTEGER,
  commit_sha VARCHAR(40),
  commit_timestamp TIMESTAMP,
  author VARCHAR(255),
  raw_value TEXT,                           -- masked for display, full for audit
  severity VARCHAR(20) DEFAULT 'medium',    -- critical (verified) / medium (unverified)
  created_at TIMESTAMP DEFAULT NOW()
);

-- Future: Scan schedules
CREATE TABLE compliance_schedules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  server_id UUID REFERENCES servers(id),
  profile VARCHAR(50) NOT NULL,             -- cis_l1, cis_l2, cis_docker, lynis
  cron_expression VARCHAR(100) NOT NULL,
  enabled BOOLEAN DEFAULT TRUE,
  notify_on_drop BOOLEAN DEFAULT FALSE,     -- notif kalo score turun
  last_run TIMESTAMP,
  next_run TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW()
);
```

---

## 7. UX Flow

### Existing Flow: Run Scan + View Results

```
1. Buka /compliance
2. Liat KPI: "4 servers scanned", "58 checks", "Overall 72% 🟠", "Trend +3%"
3. Benchmark cards: CIS L1 (82% 🟢), CIS L2 (65% 🟡), CIS Docker (90% 🟢)
4. Klik "CIS L1" → /compliance/cis-level-1
5. Score gauge: 82% (🟢). Category breakdown:
   🔒 SSH       95%  (17/18 passes)  [expand ▶]
   ⚙️ Kernel     75%  (6/8 passes)   [expand ▶]
   👥 Users      71%  (5/7 passes)   [expand ▶]
6. Klik "Users" → scroll ke table:
   ┌───┬─────────────┬──────┬────────┐
   │ # │ Check       │ Status│ Remed. │
   ├───┼─────────────┼──────┼────────┤
   │ 1 │ 5.3.1 SSH    │ PASS  │        │
   │ 2 │ 5.3.2 UID 0  │ FAIL  │ [Copy] │
   └───┴─────────────┴──────┴────────┘
7. Klik FAIL row → expand: "Check 5.3.2: Ensure root is the only UID 0 account"
   Risk: Multiple accounts with UID 0 grants root privileges
   Remediation: `usermod -u <new_uid> <username>`
   Raw output: `awk -F: '($3 == 0) {print}' /etc/passwd`
8. Klik "Copy" → copy remediation command
9. Atas: "Run New Scan" → select CIS L2 → trigger → progress spinner
```

### Flow: Container Security Check

```
1. Buka /containers → click "anjungan-backend" container
2. Tab "Security" → /containers/{server}/{id}/security
3. Score gauge: 70% 🟡
4. Findings:
   ✅ Root user       → FAIL  → container runs as root
   ✅ Privileged mode → PASS
   ✅ Seccomp         → FAIL  → no seccomp profile
   ✅ Capabilities    → PASS
   ✅ Read-only FS    → FAIL  → writable rootfs
   ✅ Resource limits → PASS  → CPU 0.5, RAM 512MB
5. Klik "Root user" → detail: container runs as UID 0
   Fix: Add `USER appuser` to Dockerfile + recreate
   Remediation command copyable
```

---

## 8. Implementation Roadmap

### 🟢 Phase 1 — Foundation (✅ Done)

**Goal:** CIS scanning dasar + compliance dashboard

| Order | Feature | Status | Effort |
|-------|---------|--------|--------|
| 1 | DB schema (scan_results, scan_findings) | ✅ Done | 0.5 hari |
| 2 | Scanner engine (SSH runner) | ✅ Done | 2 hari |
| 3 | SSH checks (18 checks) | ✅ Done | 2 hari |
| 4 | Kernel + FS + Users checks (23 checks) | ✅ Done | 2 hari |
| 5 | Services + Network + Logging checks (14 checks) | ✅ Done | 1 hari |
| 6 | Compliance dashboard + detail pages | ✅ Done | 3 hari |

### 🟢 Phase 2 — Docker + Lynis (✅ Done)

**Goal:** Container-specific security

| Order | Feature | Status | Effort |
|-------|---------|--------|--------|
| 7 | CIS Docker checks (22 checks) | ✅ Done | 2 hari |
| 8 | Lynis integration (SSH runner + parser) | ✅ Done | 1.5 hari |
| 9 | Lynis frontend page | ✅ Done | 1 hari |
| 10 | CIS Docker frontend page | ✅ Done | 1 hari |

### 🟢 Phase 3 — Container Security (✅ Done)

**Goal:** Runtime container scanning

| Order | Feature | Status | Effort |
|-------|---------|--------|--------|
| 11 | Container security scanner (10 checks) | ✅ Done | 2 hari |
| 12 | Container security frontend (1247 lines) | ✅ Done | 2 hari |
| 13 | Scan history + trend | ✅ Done | 1 hari |

### 🔴 Phase 4 — Trivy + TruffleHog + Automation (Planned)

**Goal:** Vulnerability scanning + secret detection + automation

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 14 | `trivy_scans` table + migration | 0.5 hari | — |
| 15 | Trivy webhook receiver | 1 hari | #14 |
| 16 | Trivy live scan (SSH runner) | 1 hari | #14 |
| 17 | Trivy frontend dashboard | 2-3 hari | #15, #16 |
| 18 | CI/CD vs Live comparison | 1 hari | #17 |
| 19 | `trufflehog_scans` + `trufflehog_findings` tables + migration | 0.5 hari | — |
| 20 | TruffleHog Git scan backend (SSH runner) | 1 hari | #19 |
| 21 | TruffleHog filesystem scan backend | 0.5 hari | #19 |
| 22 | TruffleHog webhook receiver (`POST /secrets/webhook`) | 1 hari | #19 |
| 23 | TruffleHog frontend dashboard + detail pages | 2 hari | #20, #21, #22 |
| 24 | Verified vs unverified severity scoring | 0.5 hari | #20 |
| 25 | Scheduled scans engine | 2 hari | Phase 1 |
| 26 | Schedule editor UI | 1 hari | #25 |
| 27 | Compliance report PDF export | 2 hari | — |
| 28 | Trend graph (line chart 90d) | 1 hari | — |
| 29 | Notifikasi score drop | 1 hari | #25 |

---

## 9. Non-Functional Requirements

| Requirement | Target | Status |
|-------------|--------|--------|
| Scan execution (CIS, 58 checks) | < 30 detik per server | ✅ |
| Lynis scan | < 60 detik per server | ✅ |
| Container security scan (10 checks) | < 10 detik per container | ✅ |
| Dashboard load | < 2 detik (4 servers) | ✅ |
| History query (90 days) | < 500ms | ✅ |
| Concurrent scans | 3 parallel | ✅ |
| SSH connection timeout | 10s per check | ✅ |
| DB cleanup | Auto-delete > 90 day results | 🟡 Manual |

---

## 10. Glossary

| Term | Definition |
|------|------------|
| **CIS Benchmark** | Center for Internet Security standard — best practice config untuk OS, Docker, K8s |
| **CIS Level 1** | Basic security — recommended, minimal performance impact |
| **CIS Level 2** | Defense-in-depth — lebih ketat, mungkin ngaruh ke performance |
| **CIS Docker** | Docker-specific benchmark — 22 checks across 6 sections |
| **Lynis** | Open-source security auditing tool — hardening index, warnings, suggestions |
| **Hardening Index** | Lynis score (0-100) — seberapa hardened suatu sistem |
| **Scan Profile** | Kategori scan: all, cis_l1, cis_l2, cis_docker, lynis, container |
| **Container Security** | Runtime security checks per container — 10 checks |
| **Trivy** | Vulnerability scanner by Aqua Security — OS packages, language deps, Dockerfile misconfigs (secrets delegated to TruffleHog) |
| **Fix Rate** | Percentage of vulnerabilities that have a fix version available |
| **TruffleHog** | Open-source secret scanner by Truffle Security — 700+ detectors, verification engine (tests credentials live), git-aware, supports git/filesystem/Docker/S3/GitHub API |

## 11. References

- [PRD.md](./PRD.md) — Main Anjungan PRD (Phase 3 Security & Governance)
- [PRD-registry.md](./PRD-registry.md) — Registry & image management
- [sketches/container-compliance/](../sketches/container-compliance/) — Trivy design sketches
- [DECISIONS.md](../DECISIONS.md)
- [TruffleHog](https://github.com/trufflesecurity/trufflehog) — GitHub repo, 27K+ stars
- [Trivy](https://github.com/aquasecurity/trivy) — GitHub repo
