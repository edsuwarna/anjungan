# Anjungan — PRD: Resource Usage & Cost Tracking

> **Version:** 1.0
> **Status:** 🔴 Not Implemented — Proposed for Phase 4
> **Author:** Endang Suwarna
> **Last Updated:** June 5, 2026

---

## 1. Executive Summary

### Problem Statement

Endang now has 5 VPS across various providers (Linode, Vultr, DigitalOcean, etc.) — each with different prices and specs. But there's no visibility:

- Which service consumes the most resources? CPU? RAM? Disk?
- What's the total infra cost per month?
- Which service is the most expensive to run?
- Usage trends — is it becoming more efficient or more wasteful?
- If scaling is needed, which service is the first candidate to optimize?

What currently exists is only manual `docker stats` via SSH, or opening each provider's dashboard one by one.

**Resource Usage solves this:**
- **Single dashboard** — view all resources across all servers from one screen
- **Cost allocation** — know how much each service costs per month
- **Trend analysis** — view usage patterns over 7d/30d
- **Optimization target** — know which service needs to be scaled down or optimized

### Target Audience

- **Endang** (platform engineer) — so they know what the infra money is spent on
- **Developer (future)** — view their own service's resource usage

### Goals

| Goal | Metric |
|------|--------|
| View real-time resource usage of all servers | < 5 sec refresh |
| Track cost per service per month | Accurate ±5% of actual bill |
| 7d/30d trend per service | Line chart CPU + RAM |
| Identify optimization candidates | Top 5 most resource-intensive services |
| Export report | Monthly CSV |

### Non-Goals

- ❌ Not a billing system — does not generate invoices for the team
- ❌ Not a replacement for VictoriaMetrics — data still comes from VM
- ❌ Not an auto-scaling engine — view only, manual scaling
- ❌ Not real-time pricing from provider API — manual cost input / static config

---

## 2. Product Overview

### Data Flow Architecture

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

### Two Collection Modes

**Mode A: Direct SSH (default)**
```
Anjungan → SSH to each server → `docker stats --no-stream --format json`
→ Parse → Save to resource_usage DB
```
- Simple, no agent needed
- 30s polling — lightweight
- Can work from any server registered in `cluster_servers`

**Mode B: VictoriaMetrics (existing)**
```
VM already collects metrics from Docker → Anjungan queries from VM API
→ Parse series → Resource data
```
- More accurate (historical data already exists)
- But VM is only on server A — doesn't collect remote servers (unless agent push)

Combined: Mode A for real-time + Mode B for historical trends (server A only).

---

## 3. Feature Specifications

> **Priority Key:** P0 = Must have (blocker), P1 = Should have (important), P2 = Nice to have

### F1 — Resource Collector

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | `ResourceCollector` service — goroutine every 30 seconds. Collects from all servers in `cluster_servers` with status=online. Via SSH: `docker stats --no-stream --format json` → parse JSON array → insert into `resource_usage`. Also collects disk: `df -h /` + memory: `free -m`. Error handling: if SSH fails → mark server degraded (but don't drop it). Endpoint: `GET /api/v1/resources/current?server_id=` — return latest snapshot. |
| **UX** | Background process — no direct UI. But collector status is visible on dashboard: "Collecting from 4/5 servers 🟢" |

### F2 — Resource Dashboard

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | `GET /api/v1/dashboard/resources` — summary: total CPU, RAM, disk all servers. `GET /api/v1/resources/servers/{id}` — per server detail. `GET /api/v1/resources/containers?server_id=` — per container breakdown. Response include: current value, max, used %, trend (arrow up/down/flat). |
| **Frontend** | Route `/infra/resources`. **Overview** tab: KPI cards at top (Total CPU 34%, RAM 52%, Disk 67%, Cost Rp 847K/month). Server cards grid: each server has a mini CPU/RAM/Disk bar. **Per Server** tab: detail server + container list with resource bars. **Per Service** tab: aggregated across servers (if the service runs on multiple replicas). Color threshold: 🟢 <70%, 🟡 70-85%, 🔴 >85%. |
| **UX** | Bar color matches threshold. Hover bar → tooltip exact value. Click container → detail log / SSH. Auto-refresh (30s polling). |

### F3 — Cost Configuration

| | |
|---|---|
| **Priority** | P0 |
| **Backend** | `cost_config` table. Columns: id, cluster_server_id (FK), provider (vultr/linode/digitalocean/hetzner/other), monthly_cost DECIMAL (IDR), currency (IDR/USD), billed_until DATE, specs_override JSONB (if different from what's in cluster_servers). CRUD: `GET/POST/PUT/DELETE /api/v1/resources/costs`. Also `GET /api/v1/dashboard/cost-summary` — total per provider, total all, trend. |
| **Frontend** | Route `/infra/resources`. Tab "Costs". Table: Server | Provider | Spec | Monthly Cost | Billed Until | Status. Total cost card at the top. Pie chart by provider. "Add Cost" modal: select server (from cluster_servers), provider dropdown, monthly cost, currency. |
| **UX** | Cost format: Rp 150.000 (IDR) or $5 (USD). Can filter by provider. Total cost auto-sum. Pie chart colored by provider. |

### F4 — Per-Service Cost Breakdown

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | Logic: Allocate server cost proportionally to each container based on CPU/RAM usage. Formula: `container_cost = server_monthly_cost × (container_weight / total_weight)`. Weight: `cpu_percent + ram_percent` (average from last N samples). Endpoint: `GET /api/v1/resources/services/cost-breakdown` — service name, server, cpu%, ram%, weight, cost_estimate, trend arrow. |
| **Frontend** | In the "Services" tab: table sorted by cost descending — most expensive service at the top. Bar chart: top 5 vs the rest. Click → detail: cost breakdown per server (if multi-replica). |
| **UX** | Small disclaimer: "Estimated based on resource share — actual bill depends on provider." Tooltip explanation of the formula. |

### F5 — Trend Analysis

| | |
|---|---|
| **Priority** | P1 |
| **Backend** | `resource_hourly` table — hourly aggregation (avg, max, min CPU/RAM). Auto-cleanup > 90 days. Endpoint: `GET /api/v1/resources/trends?server_id=&container_name=&period=7d|30d|90d` — return time series data {timestamp, cpu_avg, cpu_max, ram_avg, ram_max}. |
| **Frontend** | Line chart per server/container: 2 lines — CPU (blue) + RAM (green). Time range selector: 7d / 30d / 90d. Hover → tooltip exact value. Zoom-in select. Per-container toggle: check which containers to compare. |
| **UX** | Chart library: Chart.js or D3. X-axis: timestamp, left Y-axis: %, right Y-axis: absolute (RAM in GB). Legend click → hide/show line. |

### F6 — Alert & Optimization Suggestions

| | |
|---|---|
| **Priority** | P2 |
| **Backend** | Rule engine: `if avg_cpu > 80% for 1h → suggest scale up`. `if avg_cpu < 10% for 7d → suggest scale down`. `if disk > 85% → suggest cleanup`. Rule config: threshold, duration, action (scale_up, scale_down, cleanup, investigate). Endpoint: `GET /api/v1/resources/suggestions` — list optimization suggestions with impact estimate (cost saving). |
| **Frontend** | Card on dashboard: "💡 Optimization Suggestions". List: "Scale down peladen-cache (CPU < 10% for 7d) — save Rp 50K/month". Click → dismiss / apply (manual). |
| **UX** | Suggestion dismiss = hide for 30 days. Impact estimate in IDR. If already dismissed — won't reappear unless threshold is exceeded again. |

### F7 — Export Report

| | |
|---|---|
| **Priority** | P2 |
| **Backend** | `GET /api/v1/resources/report?period=month&format=csv` — generate report: per server cost, per service cost, monthly trend, optimization suggestions. Format: CSV (default) + JSON. |
| **Frontend** | Button "Export Monthly Report" on dashboard. Download CSV. |
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

Since Endang has servers from various providers with different prices, the per-service cost is calculated proportionally:

### Formula

```
Server Cost Allocation per Container:

  weight = (cpu_percent × cpu_factor) + (ram_percent × ram_factor)

  Where:
    cpu_factor = 0.6    (CPU is more dominant in VPS pricing)
    ram_factor = 0.4    (RAM is the second factor)

  container_cost = server_monthly_cost × (weight_container / sum_of_all_weights)

  service_cost = sum(container_cost) dari semua container yang termasuk service itu
```

### Example

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
│ 🟢 app-1-api             peladen-ml   45%  30%  Rp 287K  │ ← most expensive
│ 🟢 victoria-metrics      central      15%  45%  Rp 120K  │
│ 🟢 zot                   central       8%  12%   Rp 52K  │
│ 🟢 app-2-web             peladen-ml   15%  50%  Rp 213K  │
│ 🔴 stem-lab              central       0%   0%    Rp 0   │ ← stopped
└──────────────────────────────────────────────────────────┘
```

---

## 8. UX Flow Detail

### Flow: Daily Resource Usage Check

```
1. Open /infra/resources
2. View overview — 4 KPI cards: CPU 34%, RAM 52%, Disk 67%, Cost Rp 847K
3. Scroll → view per-server: peladen-ml most efficient (12% CPU), peladen-central weight 52%
4. Click "Services" tab → sort by cost descending
5. app-1-api Rp 287K — most expensive
6. Click app-1-api → detail: CPU 45% consistent for 7 days, RAM 30%
7. "This resource usage is normal — production service, needs that much resource."
8. Back → view "Trends" → app-1-api line chart up 12% in 30 days
   "Starting to gain users. Keep monitoring."
```

### Flow: Find Services to Optimize

```
1. Open /infra/resources → Suggestions card:
   "Scale down peladen-cache — CPU < 5% for 7d — save Rp 50K/month"
2. Click → view detail: only Redis on that server, CPU rarely used
3. "Better move Redis to central server — shut down peladen-cache."
4. Dismiss suggestion (since action is already known).
5. Or export report first: click "Export" → CSV → attach to meeting.
```

### Flow: Setup Cost Config

```
1. Open /infra/resources → tab "Costs"
2. View table — 4 servers, 3 don't have cost config yet
3. Click "Add Cost":
   Server: [peladen-central ▼]
   Provider: [vultr ▼]
   Monthly Cost: Rp 250.000
   Currency: IDR ▼
4. Save → appears in table + total cost updates
5. Continue filling in other servers until all are complete
```

---

## 9. Non-Functional Requirements

| Requirement | Target |
|-------------|--------|
| **Collection interval** | 30 seconds per server |
| **SSH load** | 1 SSH connection per server per polling — lightweight |
| **Data retention** | 30s samples: 1 hour. Hourly: 90 days. Monthly: forever |
| **Query performance** | Trend query < 200ms (indexed by hour_bucket) |
| **Cost accuracy** | ±5% of actual bill (proportional estimate) |
| **Concurrent collection** | Max 10 servers parallel (goroutine pool) |
| **Dashboard load** | < 1 second (summary from materialized query) |

---

## 10. Implementation Roadmap

### 🟢 Phase 1 — Collector + Dashboard

**Goal:** View real-time resource usage from all servers

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 1 | `resource_usage` table + migration | 0.5 days | cluster_servers table |
| 2 | SSH Docker stats collector (30s goroutine) | 1.5 days | #1 |
| 3 | Resource current endpoint | 0.5 days | #2 |
| 4 | Dashboard overview UI (KPI cards + server bars) | 1 day | #3 |
| 5 | Per-container breakdown UI | 0.5 days | #3 |
| **Total** | | **4 days** | |

### 🟡 Phase 2 — Cost Tracking

**Goal:** Know how much money is spent per month + per service

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 6 | `cost_config` table + migration | 0.5 days | — |
| 7 | Cost config CRUD backend + frontend | 1 day | #6 |
| 8 | Cost summary endpoint + UI | 0.5 days | #7 |
| 9 | Per-service cost breakdown (weighted formula) | 1 day | #8 |
| 10 | Cost pie chart + per-service sorted table | 0.5 days | #9 |
| **Total** | | **3.5 days** | |

### 🔵 Phase 3 — Trends + History

**Goal:** Know resource usage trends

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 11 | `resource_hourly` table + aggregator cron | 1 day | #1 |
| 12 | Trend endpoint + line chart | 1.5 days | #11 |
| 13 | Time range selector (7d/30d/90d) | 0.5 days | #12 |
| 14 | Multi-container comparison toggle | 0.5 days | #13 |
| **Total** | | **3.5 days** | |

### ⚪ Phase 4 — Intelligence

**Goal:** Actionable insight

| Order | Feature | Effort | Dependencies |
|-------|---------|--------|-------------|
| 15 | Optimization rule engine | 1 day | #1 |
| 16 | Suggestion UI + dismiss | 0.5 days | #15 |
| 17 | Report export (CSV) | 1 day | #9, #12 |
| 18 | Chart color threshold (🟢/🟡/🔴) | 0.5 days | #12 |
| **Total** | | **3 days** | |

---

## 11. Glossary

| Term | Definition |
|------|------------|
| **Resource Usage** | CPU%, RAM%, Disk% — real-time snapshot from `docker stats` |
| **Weighted Cost** | Proportional cost per container based on CPU + RAM share |
| **Cost Config** | VPS price data per month per server — manually input |
| **Trend** | Hourly aggregation — CPU avg/max, RAM avg/max |
| **Optimization Suggestion** | Auto-generated recommendation for scale up/down based on usage pattern |
| **VM / VictoriaMetrics** | Existing time-series database — optional source for historical trends |
| **Proportional** | Cost allocation based on resource share, not fixed |

## 12. References

- [PRD.md](./PRD.md) — Main Anjungan PRD (Phase 4 Observability)
- [PRD-domain-management.md](./PRD-domain-management.md) — Domain & multi-server routing (cluster_servers table)
- [DECISIONS.md](../docs/DECISIONS.md) — Architectural decisions
