# Anjungan — PRD: Container Image Vulnerability Scanning (Trivy)

> **Version:** 1.0
> **Status:** Draft — 🔴 Planned
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Anjungan udah punya **CIS hardening** (server OS + Docker daemon config) dan **Container Security** (runtime checks — privileged, capabilities, root). Tapi dua itu semua **preventive** — ngecek konfigurasi container *sebelum* atau *saat* jalan.

Yang belum: **vulnerability scanning untuk image itu sendiri** — OS packages dan language dependencies di dalam image yang udah jalan di server. Misal:

- `nginx:1.25` jalan di server A — tau ga kalo `libssl3` di dalamnya punya CVE-2024 critical?
- `node:20-alpine` di server B — package `lodash` versi lama ada RCE?
- Image di-push seminggu lalu — base image-nya udah ada CVE baru yang belum di-patch

Tanpa vulnerability scanning:
- **Blind spot** — tau container jalan, tapi ga tau isinya rentan atau aman
- **No prioritization** — 10 server, 30 image — yang mana paling kritis?
- **Reactive** — baru tau ada CVE pas udah kena exploit / lewat newsletter

### What This Solves

| Masalah | Solusi |
|---------|--------|
| CVE di image yang jalan | **Trivy scan** — OS packages + language deps |
| Image yang belum pernah di-scan | **Image Discovery** — agent auto-detect `docker images` |
| Ribet scan 1 per 1 | **Batch scan per server** — agent parallel scan semua image |
| No trend / history | **Scan history** — tau fix rate, CVE baru, trending severity |
| Image dari registry vs via CI/CD | **Source tracking** — CI/CD webhook + agent live scan |

### Current Status

| Aspek | Status |
|-------|--------|
| CIS Docker Benchmark (22 checks) | ✅ Ada di Compliance |
| Container Security (10 runtime checks) | ✅ Ada di Compliance |
| **Trivy Vulnerability Scanner (Agent-based)** | ❌ **Not implemented** — PRD ini |
| Trivy CI/CD Integration | ❌ **Not implemented** — PRD ini |
| Cross-image CVE correlation | ❌ Not implemented |

### Target Audience

- **Endang** (platform engineer) — maintain container security posture across servers
- **DevOps** — tau mana image yang perlu direbuild / di-patch
- **Security-conscious teams** — compliance requirement buat vulnerability management

### Goals

| Goal | Metric |
|------|--------|
| Discover all container images on managed servers | ✅ 100% coverage |
| Scan image vulnerabilities (OS + language deps) | ✅ Trivy integration |
| Severity-based prioritization | ✅ Critical/High/Medium/Low |
| Fix rate tracking | ✅ % fixable vulns with version available |
| Scan history & trend | ✅ Per-image timeline |
| Cross-server CVE visibility | ✅ CVE yang ngaruh ke multiple images |

---

## 2. Product Overview

### Architecture

```
                    Anjungan Server
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│  Agent Gateway ───► Container Image Scanner ──► Frontend    │
│  (WebSocket)       Handler                  Dashboard       │
│                       │                         │            │
│                       ▼                         ▼            │
│                  ┌──────────────┐         ┌──────────┐      │
│                  │ DB:          │         │ SvelteKit│      │
│                  │ image_scans  │         │ Routes   │      │
│                  │ image_assets │         │          │      │
│                  │ cve_findings │         │ /images  │      │
│                  └──────────────┘         │ /images/ │      │
│                                           │  [id]    │      │
└───────────────────────┬───────────────────┴──────────┘      │
                        │
          ┌─────────────┼─────────────┐
          │             │             │
    ┌─────▼─────┐ ┌─────▼─────┐ ┌─────▼─────┐
    │ Server A   │ │ Server B   │ │ Server C   │
    │ Agent      │ │ Agent      │ │ Agent      │
    │            │ │            │ │            │
    │ ├─Trivy    │ │ ├─Trivy    │ │ ├─Trivy    │
    │ └─Docker   │ │ └─Docker   │ │ └─Docker   │
    └───────────┘ └───────────┘ └───────────┘
```

### Scanning Flow (Agent-based)

```
┌────────────────────────────────────────────────────────────────┐
│  IMAGE DISCOVERY                                              │
│                                                               │
│  Agent register → docker images --format json                  │
│                → POST /api/v1/images/discover                  │
│                → Anjungan simpan: server_id, repo:tag,         │
│                  image_id, size, created                       │
│                                                               │
│  Re-discovery setiap agent connect / periodik                 │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│  VULNERABILITY SCAN (Trivy)                                   │
│                                                               │
│  Trigger: manual / scheduled / post-push hook                  │
│                                                               │
│  Agent → trivy image --severity CRITICAL,HIGH,MEDIUM          │
│                    --format json IMAGE:TAG                     │
│                                                               │
│  Agent → POST /api/v1/scans/images/submit                     │
│  {                                                             │
│    server_id: "srv-01",                                       │
│    image: "nginx:1.25",                                       │
│    image_id: "sha256:a1b2...",                                │
│    scanner: "trivy",                                          │
│    summary: { critical: 3, high: 7, medium: 12, low: 4 },    │
│    vulnerabilities: [                                          │
│      {                                                         │
│        id: "CVE-2024-1234",                                   │
│        pkg: "libssl3", installed: "3.0.9", fixed: "3.0.10",   │
│        severity: "CRITICAL", cvss: 9.1,                       │
│        title: "Buffer overflow in SSL_read",                  │
│        url: "https://nvd.nist.gov/..."                        │
│      }                                                         │
│    ],                                                          │
│    misconfigs: [ /* Dockerfile lint */ ]                       │
│  }                                                             │
│                                                               │
│  Anjungan parse → simpan ke image_scans + cve_findings         │
│                 → update server image list (last_scan)         │
└────────────────────────────────────────────────────────────────┘
```

### Integration Points

| Integrasi | Deskripsi |
|-----------|-----------|
| **Anj-Agent** | Agent jalanin Trivy di server, push hasil via HTTP API |
| **Zot Registry** | Webhook post-push trigger scan image yang baru di-push |
| **Existing Compliance** | Container Security (runtime) + CIS Docker — komplementer |
| **Existing Containers page** | Tambah column "Vulnerabilities" with severity badge |
| **Deployments** | Trigger scan ulang pas redeploy |

---

## 3. Feature Specifications

> **Legend:** ✅ Implemented | 🟡 Partial | 🔴 Planned

### F1 — Image Discovery

| | |
|---|---|
| **Priority** | P1 |
| **Status** | 🔴 **Planned** |
| **Backend** | Agent `docker images --format json` → `POST /api/v1/images/discover`. Simpan ke tabel `image_assets`: server_id, repo, tag, image_id (sha256), size, created_at, last_scan_at, status (pending/scanned/error). Endpoint: `GET /api/v1/images` (?server_id=&status=&search=), `GET /api/v1/images/{id}`, `POST /api/v1/images/discover` (agent push). Image deduplication by image_id across servers. |
| **Agent** | On connect dan periodic (every 6h), agent jalanin `docker images --format '{{json .}}' --no-trunc` → kirim daftar ke Anjungan. Delta detection: image baru vs yang udah pernah di-scan. |
| **Data** | `image_assets` table. Satu image bisa muncul di multiple server (image_id sama, tapi server_id beda). |

### F2 — Trivy Vulnerability Scanner

| | |
|---|---|
| **Priority** | P0 |
| **Status** | 🔴 **Planned** |
| **Scope** | **Vulnerabilities (OS + language deps) + Misconfigurations (Dockerfile lint) only.** Secrets scanning delegated to TruffleHog (PRD-secret-scanning.md). |
| **Backend — Agent Push** | Agent jalanin `trivy image --severity CRITICAL,HIGH,MEDIUM --format json IMAGE:TAG`. Parse Trivy JSON → extract `Results[]` per package type (alpine, debian, npm, gomod, pip, etc.). Extract per-CVE: VulnerabilityID, Severity, PkgName, InstalledVersion, FixedVersion, CVSS, Title, PrimaryURL, Status (affected/fixed/will_not_fix), Layer.Digest. Simpan summary + findings ke `image_scans` + `cve_findings`. |
| **Backend — CI/CD Webhook** | `POST /api/v1/trivy/webhook` — terima Trivy JSON dari GitHub Action. Source = `ci`. Simpan dengan commit_sha, branch, workflow_url. Tampil unified dengan agent scans. |
| **Backend — Post-Push (Zot)** | Zot registry webhook → Anjungan trigger scan image yang baru di-push. Agent yang handle? Atau Anjungan panggil Trivy di registry side? |
| **Agent** | Trivy execution modes: (1) **Full scan** — `trivy image` dengan severity filter. (2) **Quick scan** — `trivy image --scanners vuln` tanpa misconfig. Waktu scan dibatasi 5 menit per image. Cache result di agent supaya ga re-scan kalo image_id sama. |
| **Data Model** | `image_scans`: image_name, image_tag, image_id, server_id, source (agent/ci/webhook), scanner_version, summary (JSONB: {critical, high, medium, low}), total_vulns, fixable_count, misconfigs (JSONB), raw_results (JSONB — optional). `cve_findings`: scan_id FK, cve_id, severity, pkg_name, pkg_path, installed_version, fixed_version, cvss_score, cvss_vector, status, title, description, reference_url, layer_digest. |

### F3 — Scan Dashboard & History

| | |
|---|---|
| **Priority** | P0 |
| **Status** | 🔴 **Planned** |
| **Backend** | `GET /api/v1/scans/images/summary` — aggregate: total images, total scans, critical count, high count, fix rate, scan coverage. `GET /api/v1/scans/images` — list scans (?server_id=&image=&severity=&status=). `GET /api/v1/scans/images/{id}` — scan detail with CVE findings. `GET /api/v1/scans/images/latest/{image_name}` — latest scan per image. `GET /api/v1/scans/images/trends` — time series: count per severity per day. |
| **Frontend** | Route `/images`. **Hardening**: sama kayak route `/compliance` — biar konsisten. Tapi isinya dashboard image vulnerability. **Dashboard**: KPI cards — images scanned, total CVEs, critical count, fix rate. **Per-server image list**: expandable card, server name, image count, worst severity badge. **Image card**: image:tag, last scan time, severity badges, scan button. **Trend chart**: 30-day severity distribution bar chart. |
| **UX** | **Empty state**: "No images discovered yet — install agent on target server." **Severity badge**: 🔴 critical, 🟠 high, 🟡 medium, 🟢 low. **Scan in progress**: spinner + "Scanning..." with elapsed time. **Delta badge**: 🔺 Critical +2 vs previous scan. |

### F4 — Image Detail & CVE Drill-Down

| | |
|---|---|
| **Priority** | P1 |
| **Status** | 🔴 **Planned** |
| **Backend** | `GET /api/v1/images/{id}` — image detail + latest scan. `GET /api/v1/images/{id}/history` — scan timeline. `GET /api/v1/images/{id}/cves` — filterable CVE list (?severity=&fixable=&pkg=). |
| **Frontend** | Route `/images/[id]`. **Image header**: name:tag, image_id (sha256 truncated), size, created, server name, last scan time. **Scan timeline bar**: horizontal scrollable pills — select scan. **Summary cards**: CRITICAL N, HIGH N, MEDIUM N, LOW N, Misconfig N. **Vulnerabilities tab**: expandable CVE cards — CVE ID, severity badge, package name + version, fixed version (if available), CVSS score, title, description, reference link. Filter by severity, fixable/unfixable, package type. **Misconfigurations tab**: Dockerfile lint findings with severity. **Scan source badge**: 🔵 Agent / 🟢 CI/CD / 🟣 Registry. |
| **UX** | CVE card expand → CWE category, CVSS vector, published date, reference links dengan external icon. Remediation suggestion: "Upgrade libssl3 from 3.0.9 to 3.0.10". Copy CVE ID button. **Fixed version column** — kalo kosong berarti belum ada fix (will_not_fix). |

### F5 — Cross-Image CVE Correlation

| | |
|---|---|
| **Priority** | P2 |
| **Status** | 🔴 **Planned** |
| **Backend** | `GET /api/v1/scans/images/cross-severity` — CVE yang muncul di multiple images/servers. Group by CVE ID → count server affected → count images affected. `GET /api/v1/scans/images/cross-severity/{cve_id}` — detail mana aja server + image yang kena CVE tertentu. |
| **Frontend** | **Tab/Routes: "Cross-Image CVEs"**. Ranked CVE list: CVE ID, severity, affected images count, affected servers count, total count of occurrences. Klik → detail: which servers, which images, installed version, fix available. |
| **UX** | Prioritize by most widespread: CVE yang muncul di 5 server > 1 server. Sort by severity + count. Action: "Schedule scan all affected images" button. |

### F6 — Scheduled Scan & Auto-Scan

| | |
|---|---|
| **Priority** | P2 |
| **Status** | 🔴 **Planned** |
| **Backend** | `image_scan_schedules` table: server_id (or ALL), cron expression, severity_filter (scan all vs CRITICAL+HIGH only). Background scheduler trigger agent scan. `POST /api/v1/images/schedule` — create schedule. Notifikasi ke admin kalo ada critical CVE baru. |
| **Frontend** | Schedule editor: select server / all servers, cron pattern, severity threshold. Schedule list: target, cron, last run, next run, enable toggle. |
| **UX** | Daily scan recommended. After scan complete, kalo ada critical CVE baru → badge di sidebar + notif (future: Telegram). |

---

## 4. API Design

### Agent-Facing Endpoints (Internal)

```go
POST   /api/v1/images/discover                    // Agent push image list
POST   /api/v1/scans/images/submit                // Agent push scan result
```

### Client-Facing Endpoints

```go
// === Image Discovery ===
GET    /api/v1/images                             // List images (?server_id=&status=&search=)
GET    /api/v1/images/{id}                        // Image detail + latest scan
GET    /api/v1/images/{id}/history                // Scan timeline per image
GET    /api/v1/images/{id}/cves                   // CVE findings (?severity=&fixable=&pkg=)

// === Scanning ===
GET    /api/v1/scans/images/summary               // KPI aggregate
GET    /api/v1/scans/images                       // List scans (?server_id=&image=&source=)
GET    /api/v1/scans/images/{id}                  // Scan detail + CVE list
GET    /api/v1/scans/images/{id}/compare/prev     // Delta vs previous scan
POST   /api/v1/scans/images/live-scan             // Manual: trigger agent scan {server_id, image}
POST   /api/v1/scans/images/scan-all              // Manual: scan all images on server {server_id}

// === Cross-Image ===
GET    /api/v1/scans/images/cross-severity         // Most widespread CVEs
GET    /api/v1/scans/images/cross-severity/{cve_id} // Which servers/images affected

// === CI/CD Webhook ===
POST   /api/v1/trivy/webhook                      // Receive Trivy JSON from CI/CD

// === Schedules ===
GET    /api/v1/images/schedules
POST   /api/v1/images/schedules
PUT    /api/v1/images/schedules/{id}
DELETE /api/v1/images/schedules/{id}

// === Trends ===
GET    /api/v1/scans/images/trends                // Time series (?period=30d)
```

---

## 5. Database Schema

### New Tables

```sql
-- Image assets discovered by agent
CREATE TABLE image_assets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id     UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    repo          TEXT NOT NULL,                    -- "nginx"
    tag           TEXT NOT NULL DEFAULT 'latest',   -- "1.25"
    image_id      TEXT,                              -- sha256:a1b2c3...
    size          BIGINT,                           -- bytes
    created_at    TIMESTAMP DEFAULT NOW(),
    last_scan_at  TIMESTAMP,
    scan_status   VARCHAR(20) DEFAULT 'pending',    -- pending, scanning, scanned, error
    
    UNIQUE(server_id, repo, tag)
);

-- Trivy scan results
CREATE TABLE image_scans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    image_asset_id  UUID REFERENCES image_assets(id),
    server_id       UUID REFERENCES servers(id),      -- denormalized for queries
    image_name      TEXT NOT NULL,
    image_tag       TEXT NOT NULL DEFAULT 'latest',
    image_id        TEXT,                              -- sha256 of scanned image
    source          VARCHAR(20) NOT NULL DEFAULT 'agent', -- agent, ci, webhook, registry
    scanner_version TEXT,                              -- trivy version used
    scan_number     INTEGER NOT NULL,                  -- auto-increment per image
    status          VARCHAR(20) DEFAULT 'completed',   -- pending, running, completed, failed
    summary         JSONB,                             -- {critical:N, high:N, medium:N, low:N}
    total_vulns     INTEGER DEFAULT 0,
    fixable_count   INTEGER DEFAULT 0,
    misconfigs      JSONB,                             -- [{type, severity, message}]
    raw_results     JSONB,                             -- Full Trivy output (optional)
    duration_ms     INTEGER,                           -- scan duration
    commit_sha      VARCHAR(40),                       -- from CI/CD
    branch          VARCHAR(255),                       -- from CI/CD
    workflow_url    TEXT,                               -- from CI/CD
    error_message   TEXT,
    started_at      TIMESTAMP,
    completed_at    TIMESTAMP DEFAULT NOW(),
    created_at      TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_image_scans_image ON image_scans(image_name, image_tag, scan_number DESC);
CREATE INDEX idx_image_scans_server ON image_scans(server_id, completed_at DESC);
CREATE INDEX idx_image_scans_source ON image_scans(source, created_at DESC);
CREATE INDEX idx_image_scans_severity ON image_scans USING gin (summary);

-- Individual CVE findings
CREATE TABLE cve_findings (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id           UUID NOT NULL REFERENCES image_scans(id) ON DELETE CASCADE,
    cve_id            VARCHAR(50) NOT NULL,          -- CVE-2024-1234
    severity          VARCHAR(20) NOT NULL,           -- CRITICAL, HIGH, MEDIUM, LOW, UNKNOWN
    pkg_name          TEXT NOT NULL,                  -- libssl3
    pkg_path          TEXT,                           -- path in image
    installed_version TEXT NOT NULL,
    fixed_version     TEXT,                           -- NULL if will_not_fix
    status            VARCHAR(20) DEFAULT 'affected', -- affected, fixed, will_not_fix
    cvss_score        DECIMAL(3,1),                   -- CVSS v3 score
    cvss_vector       TEXT,                           -- CVSS vector string
    cwe_ids           TEXT[],                         -- CWE-120, CWE-122
    title             TEXT,
    description       TEXT,
    reference_url     TEXT,
    layer_digest      TEXT,                           -- which layer introduced this
    published_date    TIMESTAMP,
    created_at        TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_cve_findings_scan ON cve_findings(scan_id);
CREATE INDEX idx_cve_findings_cve ON cve_findings(cve_id, severity);
CREATE INDEX idx_cve_findings_severity ON cve_findings(severity);
CREATE INDEX idx_cve_findings_fixable ON cve_findings(fixed_version) WHERE fixed_version IS NOT NULL;

-- Scan schedules
CREATE TABLE image_scan_schedules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id       UUID REFERENCES servers(id),      -- NULL = all servers
    image_filter    TEXT,                              -- * / nginx* / empty = all
    cron_expression VARCHAR(100) NOT NULL,
    severity_filter TEXT DEFAULT 'CRITICAL,HIGH',     -- minimum severity to scan
    enabled         BOOLEAN DEFAULT TRUE,
    last_run        TIMESTAMP,
    next_run        TIMESTAMP,
    notify_on_new   BOOLEAN DEFAULT FALSE,             -- notif kalo ada critical baru
    created_at      TIMESTAMP DEFAULT NOW()
);
```

---

## 6. UX Flow

### Flow: First-Time Discovery

```
1. Admin deploy agent ke server target
2. Agent connect ke Anjungan → register → docker images → POST /images/discover
3. Anjungan simpen 12 image dari server A
4. Admin buka /images — liat:
   "3 servers, 28 images discovered. 0 scanned."
   ┌────────────────────────────────────────────┐
   │ Server: prod-api (12 images)               │
   │ 📦 nginx:1.25        ◌ Never scanned      │
   │ 📦 edsuwarna/api:v2  ◌ Never scanned      │
   │ ...                                        │
   │ [Scan All 12 images]                       │
   └────────────────────────────────────────────┘
```

### Flow: Run Scan → View Results

```
1. Admin klik [Scan All 12 images] untuk server prod-api
2. Agent mulai scan: 12 images × ~30s = ~6 menit
   Backend polling agent → progress indicator per image
3. UI update real-time: ◌ Queued → ◐ Scanning → ● Done
4. Selesai semua:
   ┌────────────────────────────────────────────┐
   │ prod-api: ✅ 12/12 scanned                 │
   │ 🔴 3 critical  🟠 7 high  🟡 12 medium   │
   └────────────────────────────────────────────┘
5. Klik image nginx:1.25 → /images/[id]
   - Summary: 3C 7H 12M 4L
   - Timeline: 2h ago (scan #1)
   - CVE list:
     🔴 CVE-2024-1234  libssl3  3.0.9 → 3.0.10
     🔴 CVE-2024-5678  nginx    1.25  → 1.25.1
     🟠 CVE-2024-9012  libxml2  2.9   → 2.10
6. Klik CVE → expand: title, description, CVSS 9.1, reference links
7. "Copy CVE ID" → buat tracking / laporan
```

### Flow: Compare with CI/CD

```
1. Image "api:v2" punya 2 scan:
   🔵 Agent (prod-api) — 3C 7H — Today 10:00
   🟢 CI/CD (main#abc123) — 1C 4H — Yesterday 14:00
2. Klik "Compare" → dual pane:
   CI/CD (build time)       | Agent (production)
   1C 4H 8M                 | 3C 7H 12M
   ─────────────────────────|─────────────────────────
   libssl3 3.0.9→3.0.10 ✅  | libssl3 3.0.9→3.0.10 ✅
   nginx 1.25→1.25.1 ✅     | nginx 1.25→1.25.1 ✅
                            | 🔴 NEW: libxml2 2.9→2.10
3. Explanation: "2 new CVEs detected since build
   — base image drift or new CVE disclosure"
```

### Flow: Dashboard KPI

```
┌────────────────────────────────────────────────────┐
│  Container Image Vulnerability Scanner               │
│                                                      │
│  📦 28        🐞 156        🔴 12        68%       │
│  Images      CVEs          Critical      Fix Rate   │
│                                                      │
│  ─── Cross-Image CVEs ────────────────────────────  │
│  CVE-2024-1234  🔴  5 servers  8 images            │
│  CVE-2024-5678  🟠  3 servers  4 images            │
│                                                      │
│  ─── Servers ─────────────────────────────────────  │
│  prod-api       12 images  🔴 3C 7H 12M  [Scan]    │
│  prod-db         4 images  🟠 1C 3H  8M   [Scan]    │
│  staging        12 images  🟠 2C 6H 15M  [Scan]    │
└────────────────────────────────────────────────────┘
```

---

## 7. Implementation Roadmap

### 🔴 Phase 1 — Image Discovery + Agent Scan (Planned)

| Order | Feature | Effort | Notes |
|-------|---------|--------|-------|
| 1 | `image_assets` table + migration | 0.5 hari | — |
| 2 | Image discovery endpoint + agent push | 1 hari | Agent `docker images` → API |
| 3 | `image_scans` + `cve_findings` tables + migration | 1 hari | — |
| 4 | Trivy scan backend (agent push receiver + parser) | 2 hari | Parse Trivy JSON → extract CVEs |
| 5 | Agent: image discovery module | 1.5 hari | Re-scan on connect + periodic |
| 6 | Agent: Trivy runner (exec + parse + push) | 2 hari | `docker run aquasec/trivy` |
| 7 | Image list dashboard (/images) | 2 hari | Server groups, image cards |
| 8 | Scan detail + CVE drill-down (/images/[id]) | 2.5 hari | Timeline, CVE list, filters |
| 9 | Manual scan trigger from UI | 1 hari | "Scan Now" + "Scan All" |

### 🔴 Phase 2 — CI/CD + Advanced Features (Planned)

| Order | Feature | Effort | Notes |
|-------|---------|--------|-------|
| 10 | CI/CD webhook receiver (`POST /trivy/webhook`) | 1 hari | Unified with agent scans |
| 11 | Scan comparison (CI/CD vs Agent) | 1.5 hari | Delta detection |
| 12 | Cross-image CVE correlation | 1 hari | Most widespread CVEs |
| 13 | Scan history trend chart (30-day) | 1 hari | Bar chart severity over time |

### 🔴 Phase 3 — Automation (Planned)

| Order | Feature | Effort | Notes |
|-------|---------|--------|-------|
| 14 | Scheduled scan engine | 1.5 hari | Cron-based |
| 15 | Schedule editor UI | 1 hari | — |
| 16 | Zot registry post-push hook | 0.5 hari | Auto-scan on push |
| 17 | Notifikasi critical CVE baru | 1 hari | In-app + Telegram (future) |

---

## 8. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| Scan per image (standard) | < 30 detik per image |
| Batch scan (12 images) | < 6 menit (parallel) |
| Dashboard load (28 images) | < 2 detik |
| CVE detail page | < 500ms |
| CVE database entries | < 100K rows per scan (JSONB aggregation) |
| DB retention | Auto-delete > 90 hari raw_results (keep summary) |
| Trivy version | Latest stable — pin in agent config |
| Agent resource | CPU < 5%, RAM < 100MB during scan |
| Network | ~200KB per scan result (compressed JSON) |

---

## 9. References

- [PRD-anj-agent.md](./PRD-anj-agent.md) — Agent system (execution layer)
- [PRD-compliance.md](./PRD-compliance.md) — Existing CIS hardening + Container Security
- [PRD-secret-scanning.md](./PRD-secret-scanning.md) — TruffleHog secret detection
- [Trivy](https://github.com/aquasecurity/trivy) — Vulnerability scanner by Aqua Security
- [sketches/container-compliance/](../sketches/container-compliance/) — Trivy design mockups
- [PRD-registry.md](./PRD-registry.md) — Zot registry integration
