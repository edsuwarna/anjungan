# Anjungan — PRD: Resource Usage & Cost Tracking

> **Version:** 1.0
> **Status:** Draft
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Sekarang Endang punya 5 VPS di berbagai provider (Linode, Vultr, DigitalOcean, dll) — masing-masing dengan harga dan spesifikasi beda. Tapi ga ada visibility:

- Service mana yang paling boros resource? CPU? RAM? Disk?
- Berapa total cost infra per bulan?
- Service mana yang paling mahal di-running?
- Trend usage — apakah makin irit atau makin boros?
- Kalo mau scaling, service mana yang first candidate buat di-optimize?

Yang ada sekarang cuma `docker stats` manual lewat SSH, atau buka dashboard masing-masing provider satu-satu.

**Resource Usage solving this:**
- **Single dashboard** — liat semua resource semua server dari satu layar
- **Cost allocation** — tau berapa biaya tiap service per bulan
- **Trend analysis** — liat pattern usage 7d/30d
- **Optimization target** — tau service mana yang perlu di-scale down atau di-optimize

### Target Audience

- **Endang** (platform engineer) — biar tau duit infra abis buat apa
- **Developer (future)** — liat resource usage service mereka sendiri

### Goals

| Goal | Metric |
|------|--------|
| Liat resource usage semua server real-time | < 5 detik refresh |
| Track cost per service per bulan | Akurat ±5% dari actual bill |
| Trend 7d/30d per service | Line chart CPU + RAM |
| Identify optimization candidates | Top 5 service paling boros |
| Export report | CSV monthly |

### Non-Goals

- ❌ Bukan billing system — ga generate invoice buat team
- ❌ Bukan replacement buat VictoriaMetrics — data tetep dari VM
- ❌ Bukan auto-scaling engine — liat doang, scaling manual
- ❌ Bukan harga real-time dari provider API — cost input manual / static config

---

## 2. Product Overview

### Arsitektur Data Flow

```
Docker Host              Anjungan Backend                Frontend
┌─────────────┐         ┌────────────────────┐         ┌──────────────┐
│ docker stats │ ──SSH──▶│ Collector Service   │         │ Dashboard    │
│ (per server) │         │ (poll every 30s)    │         │ - Overview   │
└─────────────┘         │                     │───────▶│ - Per Server  │
                        │ VictoriaMetrics     │         │ - Per Service │
Docker Host              │ (existing, optional)│         │ - Trends     │
┌─────────────┐         │                     │         │ - Report     │
│ docker stats │ ──SSH──▶│ DB: resource_usage  │         │ Export CSV   │
│ (per server) │         │ DB: resource_hourly │         └──────────────┘
└─────────────┘         │ DB: cost_config     │
                        └────────────────────┘
```

### Dua Mode Collect

**Mode A: Direct SSH (default)**
```
Anjungan → SSH ke tiap server → `docker stats --no-stream --format json`
→ Parse → Simpan ke DB resource_usage
```
- Simple, no agent needed
- Polling 30s — ringan
- Bisa dari server manapun yang terdaftar di `cluster_servers`

**Mode B: VictoriaMetrics (existing)**
```
VM udah collect metrics dari Docker → Anjungan query dari VM API
→ Parse series → Resource data
```
- Lebih akurat (data historis udah ada)
- Tapi VM cuma di server A — ga collect server remote (kecuali agent push)

Combined: Mode A buat real-time + Mode B buat trend historis (server A doang).

---

## 3. Feature Specifications

> **Priority Key:** P0 = Must have (blocker), P1 = Should have (important), P2 = Nice to have

### F1 — Resource Collector

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | `ResourceCollector` service — goroutine tiap 30 detik. Collect dari semua server di `cluster_servers` yang status=online. Via SSH: `docker stats --no-stream --format json` → parse JSON array → insert ke `resource_usage`. Juga collect disk: `df -h /` + memory: `free -m`. Handle error: kalo SSH gagal → mark server degraded (tapi ga drop). Endpoint: `GET /api/v1/resources/current?server_id=` — return latest snapshot. |
| **UX** | Background process — ga ada UI langsung. Tapi status collector visible di dashboard: "Collecting from 4/5 servers 🟢" |

### F2 — Resource Dashboard

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | `GET /api/v1/dashboard/resources` — summary: total CPU, RAM, disk all servers. `GET /api/v1/resources/servers/{id}` — per server detail. `GET /api/v1/resources/containers?server_id=` — per container breakdown. Response include: current value, max, used %, trend (arrow up/down/flat). |
| **Frontend** | Route `/infra/resources`. **Overview** tab: KPI cards at top (Total CPU 34%, RAM 52%, Disk 67%, Cost Rp 847K/month). Server cards grid: tiap server punya mini bar CPU/RAM/Disk. **Per Server** tab: detail server + container list with resource bars. **Per Service** tab: aggregated across servers (kalo service jalan di multiple replica). Color threshold: 🟢 <70%, 🟡 70-85%, 🔴 >85%. |
| **UX** | Bar warna sesuai threshold. Hover bar → tooltip exact value. Klik container → detail log / SSH. Auto-refresh (30s polling). |

### F3 — Cost Configuration

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | `cost_config` table. Kolom: id, cluster_server_id (FK), provider (vultr/linode/digitalocean/hetzner/other), monthly_cost DECIMAL (IDR), currency (IDR/USD), billed_until DATE, specs_override JSONB (kalo beda dari yg di cluster_servers). CRUD: `GET/POST/PUT/DELETE /api/v1/resources/costs`. Juga `GET /api/v1/dashboard/cost-summary` — total per provider, total all, trend. |
| **Frontend** | Route `/infra/resources`. Tab "Costs". Tabel: Server | Provider | Spec | Monthly Cost | Billed Until | Status. Total cost card di atas. Pie chart by provider. "Add Cost" modal: select server (dari cluster_servers), provider dropdown, monthly cost, currency. |
| **UX** | Cost format: Rp 150.000 (IDR) atau $5 (USD). Bisa filter by provider. Total cost auto-sum. Pie chart warna per provider. |

### F4 — Per-Service Cost Breakdown

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | Logic: Bagi cost server proporsional ke tiap container berdasarkan CPU/RAM usage. Formula: `container_cost = server_monthly_cost × (container_weight / total_weight)`. Weight: `cpu_percent + ram_percent` (rata-rata dari sample N terakhir). Endpoint: `GET /api/v1/resources/services/cost-breakdown` — service name, server, cpu%, ram%, weight, cost_estimate, trend arrow. |
| **Frontend** | Di tab "Services": tabel sorted by cost descending — service termahal di atas. Bar chart: 5 termahal vs sisanya. Click → detail: breakdown cost per server (kalo multi-replica). |
| **UX** | Disclaimer kecil: "Estimasi berdasarkan resource share — actual bill tergantung provider." Tooltip explanation dari formula. |

### F5 — Trend Analysis

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | `resource_hourly` table — aggregasi per jam (avg, max, min CPU/RAM). Auto-cleanup > 90 hari. Endpoint: `GET /api/v1/resources/trends?server_id=&container_name=&period=7d|30d|90d` — return time series data {timestamp, cpu_avg, cpu_max, ram_avg, ram_max}. |
| **Frontend** | Line chart per server/container: 2 lines — CPU (blue) + RAM (green). Time range selector: 7d / 30d / 90d. Hover → tooltip exact value. Zoom-in select. Per-container toggle: centang container mana yang mau dibandingin. |
| **UX** | Chart library: Chart.js atau D3. Sumbu X: timestamp, sumbu Y kiri: %, sumbu Y kanan: absolute (RAM in GB). Legend click → hide/show line. |

### F6 — Alert & Optimization Suggestions

| | |
|---|---|
| **Priority** | P2 |
| **Backend** | Rule engine: `if avg_cpu > 80% for 1h → suggest scale up`. `if avg_cpu < 10% for 7d → suggest scale down`. `if disk > 85% → suggest cleanup`. Rule config: threshold, duration, action (scale_up, scale_down, cleanup, investigate). Endpoint: `GET /api/v1/resources/suggestions` — list optimization suggestions with impact estimate (cost saving). |
| **Frontend** | Card di dashboard: "💡 Optimization Suggestions". List: "Scale down peladen-cache (CPU < 10% for 7d) — save Rp 50K/month". Click → dismiss / apply (manual). |
| **UX** | Suggestion dismiss = hide for 30 days. Impact estimate in IDR. Kalo udah dismissed — ga muncul lagi unless threshold exceeded lagi. |

### F7 — Export Report

| | |
|---|---|
| **Priority** | P2 |
| **Backend** | `GET /api/v1/resources/report?period=month&format=csv` — generate report: per server cost, per service cost, monthly trend, optimization suggestions. Format: CSV (default) + JSON. |
| **Frontend** | Button "Export Monthly Report" di dashboard. Download CSV. |
| **UX** | Report includes: summary (total cost, top spending), detail (per server + per service), trend (chart data). Ready in < 5s. |

---

## 4. API Design

### New Endpoints

```go
// === Resource Current ===
GET    /api/v1/resources/current                         // All servers current usage
GET    /api/v1/resources/current?server_id={id}          // Single server
GET    /api/v1/resources/containers?server_id={id}       // Per container breakdown

// === Resource Trends ===
GET    /api/v1/resources/trends?server_id={id}&period=7d
GET    /api/v1/resources/trends/container?name={name}&server_id={id}&period=30d

// === Dashboard Summary ===
GET    /api/v1/dashboard/resources                       // Total + per server summary
GET    /api/v1/dashboard/cost-summary                    // Cost total + per provider

// === Cost Config ===
GET    /api/v1/resources/costs                           // All cost configs
POST   /api/v1/resources/costs                           // Add/update cost
PUT    /api/v1/resources/costs/{id}
DELETE /api/v1/resources/costs/{id}

// === Cost Breakdown ===
GET    /api/v1/resources/services/cost-breakdown         // Per service estimated cost
GET    /api/v1/resources/services/{name}/cost-detail     // Detailed breakdown

// === Suggestions & Report ===
GET    /api/v1/resources/suggestions                     // Optimization suggestions
POST   /api/v1/resources/suggestions/{id}/dismiss        // Dismiss suggestion
GET    /api/v1/resources/report?period=month&format=csv  // Export report
```

---

## 5. Database Schema

### New Tables

```sql
-- 000019_create_resource_usage.up.sql
-- Real-time snapshots (30s interval — rolling window: keep 1h of 30s samples)
CREATE TABLE resource_usage (
  id BIGSERIAL PRIMARY KEY,
  cluster_server_id UUID NOT NULL REFERENCES cluster_servers(id),
  container_name VARCHAR(255),
  cpu_percent DECIMAL(5,2),                -- 0.00 - 100.00
  ram_usage_mb DECIMAL(10,2),
  ram_percent DECIMAL(5,2),
  ram_limit_mb DECIMAL(10,2),
  disk_usage_gb DECIMAL(10,2),
  disk_percent DECIMAL(5,2),
  disk_total_gb DECIMAL(10,2),
  net_rx_mb DECIMAL(10,2),
  net_tx_mb DECIMAL(10,2),
  pids INTEGER,
  collected_at TIMESTAMP DEFAULT NOW(),

  -- Cleanup: keep 1h of 30s samples, then hourly aggregation
  INDEX idx_resource_usage_server_time (cluster_server_id, collected_at DESC)
);

-- 000020_create_resource_hourly.up.sql
-- Hourly aggregation for trends (keep 90 days)
CREATE TABLE resource_hourly (
  id BIGSERIAL PRIMARY KEY,
  cluster_server_id UUID NOT NULL REFERENCES cluster_servers(id),
  container_name VARCHAR(255),             -- NULL = server total
  hour_bucket TIMESTAMP NOT NULL,          -- truncated to hour
  cpu_avg DECIMAL(5,2),
  cpu_max DECIMAL(5,2),
  ram_avg_mb DECIMAL(10,2),
  ram_max_mb DECIMAL(10,2),
  samples_count INTEGER,                   -- how many 30s samples in this hour
  UNIQUE(cluster_server_id, container_name, hour_bucket)
);

-- 000021_create_cost_config.up.sql
CREATE TABLE cost_config (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cluster_server_id UUID NOT NULL UNIQUE REFERENCES cluster_servers(id),
  provider VARCHAR(50) NOT NULL,           -- vultr, linode, digitalocean, hetzner, other
  monthly_cost DECIMAL(12,2) NOT NULL,     -- in configured currency
  currency VARCHAR(3) DEFAULT 'IDR',       -- IDR, USD
  billed_until DATE,
  notes TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

-- 000022_create_optimization_suggestions.up.sql
CREATE TABLE optimization_suggestions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  cluster_server_id UUID REFERENCES cluster_servers(id),
  container_name VARCHAR(255),
  suggestion_type VARCHAR(50),             -- scale_up, scale_down, cleanup, investigate
  severity VARCHAR(20),                    -- low, medium, high
  title VARCHAR(255) NOT NULL,
  description TEXT,
  metric_current DECIMAL(10,2),
  metric_threshold DECIMAL(10,2),
  estimated_savings DECIMAL(12,2),        -- IDR per month
  dismissed BOOLEAN DEFAULT FALSE,
  dismissed_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW()
);
```

---

## 6. Cost Formula

Karena Endang punya server dari berbagai provider dengan harga beda, per-service cost dihitung proporsional:

### Formula

```
Server Cost Allocation per Container:

  weight = (cpu_percent × cpu_factor) + (ram_percent × ram_factor)

  Dimana:
    cpu_factor = 0.6    (CPU lebih dominan di harga VPS)
    ram_factor = 0.4    (RAM faktor kedua)

  container_cost = server_monthly_cost × (weight_container / sum_of_all_weights)

  service_cost = sum(container_cost) dari semua container yang termasuk service itu
```

### Contoh

```
Server: peladen-ml (Rp 500.000/month)
Container: app-1-api (cpu=45%, ram=30%)
Container: app-2-web  (cpu=15%, ram=50%)

weight_app1 = (45 × 0.6) + (30 × 0.4) = 27 + 12 = 39
weight_app2 = (15 × 0.6) + (50 × 0.4) = 9 + 20 = 29
total_weight = 68

app-1-api cost = 500.000 × (39/68) = Rp 286.764
app-2-web cost  = 500.000 × (29/68) = Rp 213.236
```

---

## 7. Dashboard Layout (Mockup Reference)

### Tab: Overview

```
┌──────────────────────────────────────────────────────────┐
│ [Overview] [Servers] [Services] [Costs] [Trends] [Export] │
├──────────────────────────────────────────────────────────┤
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐ │
│ │ Total CPU │ │ Total RAM│ │ Total Disk│ │ Monthly Cost │ │
│ │   34%    │ │   52%    │ │   67%    │ │ Rp 847.000   │ │
│ │ 🟢 4 cores│ │ 🟡 8/16GB│ │ 🟡 67/100│ │ 4 providers  │ │
│ └──────────┘ └──────────┘ └──────────┘ └──────────────┘ │
│                                                          │
│ Servers (4/5 online):                                    │
│ ┌──────────────────────────────────────────────────┐     │
│ │ 🟢 peladen-central  CPU ████████░░ 34%  🟢       │     │
│ │    4C · 8GB · 40GB   RAM ██████████░░ 52%  🟡    │     │
│ │    Services: 7       DISK █████████████░░ 67% 🟡 │     │
│ ├──────────────────────────────────────────────────┤     │
│ │ 🟢 peladen-ml       CPU ██░░░░░░░░ 12%  🟢       │     │
│ │    8C · 32GB · 100GB RAM █████░░░░░ 28%  🟢       │     │
│ └──────────────────────────────────────────────────┘     │
│                                                          │
│ 💡 Optimization Suggestions:                             │
│ │ Scale down peladen-cache — CPU < 5% for 7d          │ │
│ │ Estimated saving: Rp 50K/month                       │ │
└──────────────────────────────────────────────────────────┘
```

### Tab: Services (Cost Breakdown)

```
┌──────────────────────────────────────────────────────────┐
│ Service                  Server        CPU   RAM   Cost   │
│ ──────────────────────────────────────────────────────── │
│ 🟢 whatilearned          central      12%   8%   Rp 45K  │
│ 🟢 anjungan-backend      central      22%  18%   Rp 95K  │
│ 🟢 app-1-api             peladen-ml   45%  30%  Rp 287K  │ ← termahal
│ 🟢 victoria-metrics      central      15%  45%  Rp 120K  │
│ 🟢 zot                   central       8%  12%   Rp 52K  │
│ 🟢 app-2-web             peladen-ml   15%  50%  Rp 213K  │
│ 🔴 stem-lab              central       0%   0%    Rp 0   │ ← stopped
└──────────────────────────────────────────────────────────┘
```

---

## 8. UX Flow Detail

### Flow: Cek Resource Usage Harian

```
1. Buka /infra/resources
2. Liat overview — 4 KPI card: CPU 34%, RAM 52%, Disk 67%, Cost Rp 847K
3. Scroll → liat per-server: peladen-ml paling irit (12% CPU), peladen-central weight 52%
4. Klik "Services" tab → sort by cost descending
5. app-1-api Rp 287K — paling mahal
6. Klik app-1-api → detail: CPU 45% konsisten 7 hari, RAM 30%
7. "Resource ini wajar — service production, butuh resource segitu."
8. Balik → lihat "Trends" → app-1-api line chart naik 12% dalam 30 hari
   "Mulai nambah user nih. Pantau terus."
```

### Flow: Cari Service yang Bisa Di-optimize

```
1. Buka /infra/resources → Suggestions card:
   "Scale down peladen-cache — CPU < 5% for 7d — save Rp 50K/month"
2. Klik → liat detail: Redis doang di server itu, CPU jarang dipake
3. "Mending Redis ganti ke server central aja — matiin peladen-cache."
4. Dismiss suggestion (karena udah tau action).
5. Atau ekspor report dulu: klik "Export" → CSV → lampirin ke meeting.
```

### Flow: Setup Cost Config

```
1. Buka /infra/resources → tab "Costs"
2. Liat tabel — 4 server, 3 belum ada cost config-nya
3. Klik "Add Cost":
   Server: [peladen-central ▼]
   Provider: [vultr ▼]
   Monthly Cost: Rp 250.000
   Currency: IDR ▼
4. Save → muncul di tabel + total cost update
5. Lanjut isi server lain sampai semua terisi
```

---

## 9. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| **Collection interval** | 30 detik per server |
| **SSH load** | 1 SSH connection per server per polling — ringan |
| **Data retention** | 30s samples: 1 jam. Hourly: 90 hari. Monthly: selamanya |
| **Query performance** | Trend query < 200ms (indexed by hour_bucket) |
| **Cost accuracy** | ±5% dari actual bill (estimasi proporsional) |
| **Concurrent collection** | Max 10 server parallel (goroutine pool) |
| **Dashboard load** | < 1 detik (summary dari materialized query) |

---

## 10. Implementation Roadmap

### 🟢 Phase 1 — Collector + Dashboard

**Goal:** Bisa liat resource usage real-time dari semua server

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 1 | `resource_usage` table + migration | 0.5 hari | cluster_servers table |
| 2 | SSH Docker stats collector (30s goroutine) | 1.5 hari | #1 |
| 3 | Resource current endpoint | 0.5 hari | #2 |
| 4 | Dashboard overview UI (KPI cards + server bars) | 1 hari | #3 |
| 5 | Per-container breakdown UI | 0.5 hari | #3 |
| **Total** | | **4 hari** | |

### 🟡 Phase 2 — Cost Tracking

**Goal:** Tau berapa duit abis per bulan + per service

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 6 | `cost_config` table + migration | 0.5 hari | — |
| 7 | Cost config CRUD backend + frontend | 1 hari | #6 |
| 8 | Cost summary endpoint + UI | 0.5 hari | #7 |
| 9 | Per-service cost breakdown (weighted formula) | 1 hari | #8 |
| 10 | Cost pie chart + per-service sorted table | 0.5 hari | #9 |
| **Total** | | **3.5 hari** | |

### 🔵 Phase 3 — Trends + History

**Goal:** Tau trend resource usage

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 11 | `resource_hourly` table + aggregator cron | 1 hari | #1 |
| 12 | Trend endpoint + line chart | 1.5 hari | #11 |
| 13 | Time range selector (7d/30d/90d) | 0.5 hari | #12 |
| 14 | Multi-container comparison toggle | 0.5 hari | #13 |
| **Total** | | **3.5 hari** | |

### ⚪ Phase 4 — Intelligence

**Goal:** Actionable insight

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 15 | Optimization rule engine | 1 hari | #1 |
| 16 | Suggestion UI + dismiss | 0.5 hari | #15 |
| 17 | Report export (CSV) | 1 hari | #9, #12 |
| 18 | Chart color threshold (🟢/🟡/🔴) | 0.5 hari | #12 |
| **Total** | | **3 hari** | |

---

## 11. Glossary

| Term | Definition |
|------|------------|
| **Resource Usage** | CPU%, RAM%, Disk% — snapshot real-time dari `docker stats` |
| **Weighted Cost** | Cost proporsional per container berdasarkan CPU + RAM share |
| **Cost Config** | Data harga VPS per bulan per server — di-input manual |
| **Trend** | Aggregasi per jam — CPU avg/max, RAM avg/max |
| **Optimization Suggestion** | Rekomendasi auto-generated buat scale up/down based on usage pattern |
| **VM / VictoriaMetrics** | Time-series database yang udah jalan — optional source buat trend historis |
| **Proporsional** | Pembagian cost berdasarkan resource share, bukan fixed |

## 12. References

- [PRD.md](./PRD.md) — Main Anjungan PRD (Phase 4 Observability)
- [PRD-domain-management.md](./PRD-domain-management.md) — Domain & multi-server routing (cluster_servers table)
- [DECISIONS.md](../DECISIONS.md) — Architectural decisions
