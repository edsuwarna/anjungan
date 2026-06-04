# Container Compliance & Trivy Integration — Rangkuman

> Diskusi: 3 Juni 2026 — Anjungan platform

---

## 1. CIS Docker Benchmark (Container Compliance)

**Module baru di compliance scanner Anjungan**, sejajar sama CIS L1 / L2 / Lynis.

### Coverage — 6 Sections, 128 Checks

| Section | Jumlah Check | Contoh |
|---|---|---|
| **1 — Host Configuration** | 14 | Partition `/var/lib/docker`, dedicated storage driver |
| **2 — Docker Daemon** | 22 | TLS auth, log driver, no privileged port |
| **3 — Daemon Files** | 18 | Permission `/etc/docker`, CA certs |
| **4 — Images & Build** | 26 | USER directive, trusted base, COPY vs ADD |
| **5 — Container Runtime** | 32 | No `--privileged`, read-only root FS, resource limits |
| **6 — Swarm Ops** | 16 | Auto-lock manager keys, rotate CA |

### Scoring

- Format sama kayak CIS L1/L2: PASS/FAIL per check, presentase akhir
- Detect container yang jalan pake `--privileged`, docker socket mount, etc
- Tiap server bisa punya score Docker sendiri

### UX Flow

```
Compliance Dashboard
  ├─ CIS Level 1 (server)  86%
  ├─ CIS Level 2 (server)  72%
  ├─ CIS Docker (container) 81%   ← NEW
  └─ Lynis Audit (server)  72%
       ↓ klik
Detail CIS Docker → daftar per-section, per-check
                   → filter per section
                   → expand per-check
```

---

## 2. Trivy Vulnerability Scanner

**Dua source scanning**, display menyatu di dashboard.

### 2a. CI/CD Scan (existing)

- Jalan di GitHub Action setiap push & PR
- Trivy scan: **dependency** (go.mod, package.json), **Dockerfile lint**, **image vuln**
- Kirim hasil ke Anjungan via **API webhook**
- Format: **JSON** (full Trivy output, bukan SBOM doang)

### 2b. Live Scan (baru)

- Anjungan SSH ke server → jalanin Trivy via Docker
- `docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:latest image --format json IMAGE:TAG`
- Catch CVE yang muncul **setelah deploy** (zero-day, base image drift)
- Bisa manual (button) atau scheduled (cron)

### Kenapa JSON?

Trivy JSON output udah **lengkap banget** per vulnerability:

| Field | Kegunaan |
|---|---|
| `VulnerabilityID` | CVE ID |
| `Severity` | CRITICAL / HIGH / MEDIUM / LOW |
| `PkgName` / `PkgID` / `PkgIdentifier.PURL` | Identitas package |
| `InstalledVersion` / `FixedVersion` | Versi sekarang → versi fix |
| `CVSS.nvd.V3Score` / `V3Vector` | Skor + vector CVSS |
| `CweIDs` | Kategori CWE |
| `Title` / `Description` | Judul + deskripsi |
| `PrimaryURL` / `References[]` | Link detail CVE (redhat, nvd, debian, ubuntu) |
| `Status` | `affected` / `fixed` / `will_not_fix` |
| `PublishedDate` / `LastModifiedDate` | Tanggal publikasi |
| `Layer.Digest` / `Layer.DiffID` | Layer asal CVE |
| `DataSource` | Sumber data (Debian Security Tracker, dll) |

Tidak perlu scraping NVD atau API external — **semua field sudah ada di JSON**.

---

## 3. Database Model

```sql
-- Tabel utama: menyimpan setiap hasil scan
trivy_scans {
  id            UUID PRIMARY KEY
  image_name    TEXT        -- "anjungan-backend"
  image_tag     TEXT        -- "latest", "v1.2.3"
  source        TEXT        -- "github_action" | "live"
  scan_number   INTEGER     -- auto-increment per image
  commit_sha    TEXT        -- dari GitHub Action
  branch        TEXT        -- "main", "feature/*"
  workflow_url  TEXT        -- link ke GitHub Actions run
  scan_time     TIMESTAMP   -- waktu scan
  summary       JSONB       -- {critical:3, high:9, medium:12, low:8}
  misconfigs    JSONB       -- [{type:"Dockerfile", severity:"HIGH", ...}]
  secrets       JSONB       -- [{category:"GitHub Token", ...}]
  raw_results   JSONB       -- full Trivy JSON output
  created_at    TIMESTAMP DEFAULT NOW()
}

-- Index untuk query cepat
CREATE INDEX idx_scans_image ON trivy_scans(image_name, scan_time DESC);
CREATE INDEX idx_scans_source ON trivy_scans(source);
```

---

## 4. UI / Halaman

### 4a. Container Vulnerabilities Dashboard

- **Per-image card**: nama, source badge (CI/CD / Live), severity count, mini trend bar, top packages
- **Cross-image trends**: CVE yang ngaruh ke multiple images, new/fixed this week
- **KPI bar**: total critical, high, fix rate, scan count (7 hari)
- Filter: source (CI/CD / Live), branch
- Trend chart: critical + high over last 30 scans

### 4b. Scan History

- **Timeline** per image: grouped by day
- Tiap row: source badge, scan number, version + commit sha, branch tag, severity summary, NEW/FIXED badge
- Trend chart 30 scans: bar chart critical+high, min/max/avg, delta vs last week
- Filter: source, branch

### 4c. Scan Detail (dengan Scan Selector)

- **Scan timeline bar** di atas: horizontal scrollable pills — pilih scan yang mau dilihat
  - Tiap pill: waktu, source (Live/CI/CD), mini severity (3C 9H 12M)
  - Klik → ganti seluruh halaman ke scan itu
  - Active pill ada marker "LIVE"
- **Comparison bar**: otomatis nunjukin delta vs previous scan
  - 🔺 Critical +1, 🔺 High +3
  - New CVEs list
- **Summary cards**: CRITICAL 3 / HIGH 9 / MEDIUM 12 / LOW 8 / Misconfig 4
- **4 sub-tab** per scan:
  - 🛡️ **Vulnerabilities** — expandable CVE cards (severity, CVE ID, package, version, fix, CVSS, reference links)
  - 📄 **Misconfigurations** — Dockerfile lint findings
  - 🔑 **Secrets** — secret leak detection
  - 📋 **Raw JSON** — full Trivy output, copy/download/export
- Filter: severity, status (fixable/unfixable/NEW), pagination

### 4d. Live vs CI/CD Comparison

- Dual pane: CI/CD (left) vs Live (right)
- Highlight discrepancy: NEW CVE di Live tapi gak di CI/CD, atau FIXED di CI/CD tapi masih di Live
- Explanation card: kenapa bisa beda (base image drift, new CVE disclosure)
- Action suggestions: rebuild, schedule live scan

---

## 5. Data Flow

```
CI/CD Pipeline (GitHub Action)              Live Scan (dari Anjungan)
  │                                              │
  │ Trivy scan --format json                     │ docker run aquasec/trivy --format json
  │                                              │
  ▼                                              ▼
  POST /api/v1/trivy/webhook              POST /api/v1/trivy/webhook
  { source: "github_action",              { source: "live",
    image_name: "...",                       image_name: "...",
    commit_sha: "...",                       scan_time: "...",
    raw_results: { ... }                     raw_results: { ... }
  }                                        }
  │                                              │
  │                                              │
  ▼                                              ▼
┌──────────────────────────────────────────────────┐
│                  API Endpoint                      │
│  • Parse JSON → extract summary, misconfigs,      │
│    secrets, vulnerabilities                        │
│  • INSERT INTO trivy_scans                        │
│  • Auto-increment scan_number per image           │
│  • Return scan_id                                 │
└──────────────────────┬───────────────────────────┘
                       │
                       ▼
              Dashboard queries
              • Latest per image (GROUP BY)
              • Timeline per image
              • Per-scan detail by id
              • Comparison between scans
```

---

## 6. Prioritas Implementasi

| # | Item | Effort | Impact |
|---|---|---|---|
| 1 | **CIS Docker Benchmark** — module scanner + detail page | 🔴 Medium | 🔥🔥🔥 Container hardening langsung terukur |
| 2 | **Webhook endpoint** — terima Trivy JSON dari GitHub Action | 🟢 Low | 🔥🔥🔥 CI/CD results masuk ke Anjungan |
| 3 | **Dashboard + Scan History** — tampilin hasil scan | 🟢 Medium | 🔥🔥🔥 User bisa lihat hasil |
| 4 | **Live Scan** — SSH + docker run trivy + display | 🟡 Medium | 🔥🔥🔥 Catch zero-day post-deploy |
| 5 | **Scan Selector** — pilih scan dari timeline | 🟢 Low | 🔥🔥 UX enak buat multi-scan |
| 6 | **Live vs CI/CD Comparison** — dual pane + diff | 🟡 Medium | 🔥🔥 Insight beda pipeline vs production |
| 7 | **Trend chart** — 30-day severity graph | 🟡 Medium | 🔥 Lihat pergerakan security |

---
