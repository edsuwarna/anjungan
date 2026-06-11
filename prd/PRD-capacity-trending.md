# Anjungan — PRD: Capacity Trending

> **Version:** 1.0
> **Status:** Draft
> **Author:** Endang Suwarna
> **Last Updated:** June 2026

---

## 1. Executive Summary

### Problem Statement

- Gak tau kapan server mulai penuh sampai terjadi masalah — resource exhaustion always surprises
- Container memory leak gak kelihatan sampai crash, no historical view to correlate
- `docker stats` cuma realtime, gak ada history — no way to look back at what happened
- Mau planning upgrade RAM/storage tapi gak ada data tren — flying blind on capacity planning
- Multiple servers, no unified view of resource utilization over time

### Target Audience

- **Endang** (platform engineer / SRE) — primary user, needs to monitor resource trends, detect anomalies, and plan capacity across all servers
- **DevOps / Platform Engineers** — diagnosing container-level issues (memory leaks, CPU spikes), understanding resource consumption patterns
- **Team Leads / Managers** — high-level overview of infrastructure health, capacity planning decisions

### Goals

| Goal | Metric |
|------|--------|
| Grafik CPU/RAM/Disk per server — 24h, 7d, 30d real-time charts | Line charts with time range selector working |
| Per-container breakdown from server drill-down | Drill-down from server → container chart |
| Trend line (7-day moving average) overlay on charts | Overlay toggle on all time-series charts |
| Anomaly spike detection flagged on chart | >2x std deviation flagged with annotations |
| Capacity projection for disk usage | "Disk will fill in ~47 days" estimate per server |

### Non-Goals

- ❌ Bukan Grafana replacement — not a full observability platform, just focused capacity metrics
- ❌ Bukan alerting — notification engine handles that separately, this is visualization-only
- ❌ Bukan Prometheus setup — using Docker API + df directly, no Prometheus infra needed
- ❌ Bukan network metrics (bandwidth graphs) — net I/O stored but not charted initially
- ❌ Bukan log aggregation or APM — CPU/RAM/Disk only, no application-level tracing

---

## 2. Product Overview

### Tech Stack

| Layer | Technology | Reason |
|-------|-----------|--------|
| Backend | Go (existing) | `docker stats` poller via SSH, `df` reader, aggregation logic |
| Frontend | SvelteKit + Chart.js | Interactive time-series charts, lightweight, sufficient for use case |
| DB | PostgreSQL (existing) | Time-series tables (no TimescaleDB needed at single-user scale) |
| API | REST (existing Gin framework) | New `/api/v1/metrics/*` routes |
| Polling | SSH + Docker CLI | No agent needed, uses existing server SSH keys |

### This Feature in the Context of Anjungan

```
                    ┌─────────────────────────────────────────────┐
                    │            Capacity Trending                │
                    │                                             │
                    │  ┌────────────┐  ┌────────────────────┐    │
                    │  │ Collection │  │  Storage & Query   │    │
                    │  │ Cron Job   │  │                    │    │
                    │  │            │  │  server_metrics    │    │
                    │  │ ssh docker │  │  container_metrics │    │
                    │  │ stats+df   │  │                    │    │
                    │  │ every 60s  ├──┤  indexes by time   │    │
                    │  └────────────┘  └────────┬───────────┘    │
                    │                            │               │
                    │                   ┌────────▼───────────┐   │
                    │                   │  API Layer         │   │
                    │                   │  GET /api/v1/      │   │
                    │                   │  metrics/*         │   │
                    │                   └────────┬───────────┘   │
                    │                            │               │
                    │                   ┌────────▼───────────┐   │
                    │                   │  Frontend          │   │
                    │                   │  SvelteKit +       │   │
                    │                   │  Chart.js          │   │
                    │                   └────────────────────┘   │
                    └─────────────────────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
                    ▼                       ▼
            ┌──────────────┐       ┌──────────────┐
            │  Server A     │       │  Server B    │
            │  (SSH key)    │       │  (SSH key)   │
            └──────────────┘       └──────────────┘
```

The capacity trending feature runs as a background cron job inside Anjungan's backend. It connects to managed servers via SSH, runs `docker stats --no-stream` and `df -B1 /`, parses output, and stores metrics in PostgreSQL. The frontend then queries via REST API to render charts.

---

## 3. Feature Requirements

### 3.1 Feature Inventory

| Domain | Backend | Frontend | Status |
|--------|---------|---------|--------|
| Metrics collection (docker stats) | SSH poller, output parser, 60s cron | — | 🟡 Not started |
| Metrics collection (disk usage) | `df -B1 /` parser, same cron | — | 🟡 Not started |
| Server-level charts | `/api/v1/metrics/servers/{id}` endpoint | Chart.js line chart with time range selector | 🟡 Not started |
| Container-level charts | `/api/v1/metrics/containers/{id}` endpoint | Chart.js line chart, modal on server page | 🟡 Not started |
| Container listing | `/api/v1/metrics/servers/{id}/containers` | Table with current CPU/RAM, click to chart | 🟡 Not started |
| Anomaly spike detection | Comparison vs rolling stats, `anomalies` endpoint | Red markers on chart, annotated list | 🟡 Not started |
| Capacity projection | Linear regression on 30-day disk trend, `projections` endpoint | Warning cards sorted by urgency | 🟡 Not started |
| Overview dashboard | `/api/v1/metrics/overview` endpoint | Summary cards: top consumers, anomaly count | 🟡 Not started |

### 3.2 Database Schema

```sql
CREATE TABLE server_metrics (
    id BIGSERIAL PRIMARY KEY,
    server_id UUID REFERENCES servers(id),
    cpu_percent DOUBLE PRECISION,
    memory_bytes BIGINT,
    memory_percent DOUBLE PRECISION,
    disk_total_bytes BIGINT,
    disk_used_bytes BIGINT,
    disk_percent DOUBLE PRECISION,
    recorded_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_server_metrics_time ON server_metrics(server_id, recorded_at DESC);

CREATE TABLE container_metrics (
    id BIGSERIAL PRIMARY KEY,
    container_id VARCHAR(64),
    container_name VARCHAR(200),
    server_id UUID REFERENCES servers(id),
    cpu_percent DOUBLE PRECISION,
    memory_bytes BIGINT,
    memory_percent DOUBLE PRECISION,
    net_rx_bytes BIGINT,
    net_tx_bytes BIGINT,
    block_rx_bytes BIGINT,
    block_tx_bytes BIGINT,
    recorded_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_container_metrics_time ON container_metrics(container_id, recorded_at DESC);
CREATE INDEX idx_container_server_time ON container_metrics(server_id, recorded_at DESC);
```

**Retention & Rollups:**

```sql
-- Hourly rollup table (populated daily by cron for data > 90 days old)
CREATE TABLE server_metrics_hourly (
    id BIGSERIAL PRIMARY KEY,
    server_id UUID REFERENCES servers(id),
    cpu_percent_avg DOUBLE PRECISION,
    memory_bytes_avg BIGINT,
    memory_percent_avg DOUBLE PRECISION,
    disk_used_bytes_avg BIGINT,
    disk_percent_avg DOUBLE PRECISION,
    sample_count INT,
    hour_start TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX idx_server_metrics_hourly ON server_metrics_hourly(server_id, hour_start);

CREATE TABLE container_metrics_hourly (
    id BIGSERIAL PRIMARY KEY,
    container_id VARCHAR(64),
    container_name VARCHAR(200),
    server_id UUID REFERENCES servers(id),
    cpu_percent_avg DOUBLE PRECISION,
    memory_bytes_avg BIGINT,
    memory_percent_avg DOUBLE PRECISION,
    sample_count INT,
    hour_start TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX idx_container_metrics_hourly ON container_metrics_hourly(container_id, hour_start);
```

### 3.3 Feature Specs

#### F1 — Metrics Collection Cron (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 |
| **Mechanism** | Background goroutine in Anjungan backend. Iterates all registered servers, SSH in with existing key, runs `docker stats --no-stream` and `df -B1 /`. Parses JSON + text output. Runs every 60 seconds. |
| **Backend** | New package `internal/metrics/collector.go`. Poller struct with `Start(ctx) error` and `Stop()`. Uses `golang.org/x/crypto/ssh` for SSH. Parses `docker stats` JSON array output. Parses `df` output (skip `tmpfs`, find `/` mount). Stores batch insert via `COPY` or multi-row INSERT. |
| **Failure handling** | If SSH fails (server down), log error and skip that server until next cycle. If `docker stats` fails (Docker not running), log and continue with `df` only. Exponential backoff: skip 3x after 3 consecutive failures. |
| **Edge cases** | No containers running → store server metrics with zero container stats. New containers appear between polls → picked up next cycle. Container removed → data stays in DB, just stops appearing in new polls. |

**Docker stats output parsing:**

```
# docker stats --no-stream --format '{{json .}}'
{"BlockIO":"0B / 0B","CPUPerc":"0.10%","Container":"web-1","ID":"abc123","MemPerc":"1.50%","MemUsage":"15.5MiB / 1GiB","Name":"/web-1","NetIO":"1.2kB / 3.4kB","PIDs":"8"}
```

Parsed fields: `CPUPerc` → cpu_percent, `MemUsage` → parse "used / limit" → memory_bytes + memory_percent, `NetIO` → net_rx_bytes / net_tx_bytes, `BlockIO` → block_rx_bytes / block_tx_bytes, `ID` → container_id, `Name` → container_name (strip leading `/`).

**df output parsing:**

```
# df -B1 /
Filesystem      1B-blocks          Used     Available Use% Mounted on
/dev/sda1     107374182400  42949672960  64424509440  40% /
```

Parsed fields: `1B-blocks` → disk_total_bytes, `Used` → disk_used_bytes, `Use%` → disk_percent.

#### F2 — Server-Level Charts (P0)

| Aspect | Detail |
|--------|--------|
| **Priority** | P0 |
| **Backend** | `GET /api/v1/metrics/servers/{id}?range=24h&metric=cpu,memory,disk` — returns array of `{ recorded_at, cpu_percent, memory_percent, disk_percent }`. Query `server_metrics` for range ≤ 90 days, `server_metrics_hourly` for > 90 days. Downsample: if range > 7d, return hourly avg; if > 30d, return 4-hour avg. |
| **Frontend** | Line chart with 3 metrics (CPU, RAM, Disk). Time range buttons: 1h, 6h, 24h, 7d, 30d. Hover tooltip with exact values and timestamp. Color scheme: CPU → `#10b981` (emerald), RAM → `#818cf8` (indigo), Disk → `#f59e0b` (amber). |
| **UX** | Toggle checkboxes for each metric visibility. Chart fills container width, responsive. Y-axis auto-scales 0-100%. X-axis shows time labels, adaptive formatting (minutes for 1h, hours for 24h, dates for 7d+). 7-day moving average overlay as dashed line (same color as metric but lighter opacity). |

**Chart.js config sketch:**

```js
{
  type: 'line',
  data: {
    datasets: [
      {
        label: 'CPU',
        data: points.map(p => ({ x: p.recorded_at, y: p.cpu_percent })),
        borderColor: '#10b981',
        backgroundColor: 'rgba(16, 185, 129, 0.1)',
        fill: false,
        tension: 0.3,
        pointRadius: 0  // hide points for clean line
      },
      // ... RAM, Disk similar
    ]
  },
  options: {
    responsive: true,
    interaction: { mode: 'index', intersect: false },
    scales: {
      x: { type: 'time', time: { tooltipFormat: 'yyyy-MM-dd HH:mm' } },
      y: { min: 0, max: 100, title: { display: true, text: '%' } }
    }
  }
}
```

#### F3 — Container-Level Charts (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 |
| **Backend** | `GET /api/v1/metrics/servers/{id}/containers` — returns list of containers with latest metrics: `[{ container_id, container_name, cpu_percent, memory_percent, last_seen }]`. `GET /api/v1/metrics/containers/{id}?range=7d&metric=cpu,memory` — returns array of `{ recorded_at, cpu_percent, memory_percent, memory_bytes }`. |
| **Frontend** | Container table below the server chart. Columns: Name, CPU %, RAM %, Last seen. Click row → opens modal with container-level chart (CPU + Memory lines). Same time range selector as server chart. Memory shown in human-readable format (MiB/GiB) as secondary Y-axis or tooltip. |
| **UX** | Table sortable by CPU or RAM (click header). Search/filter by container name. Modal is centered overlay with close button and same responsive chart. "No containers" empty state when server has no Docker containers. |

#### F4 — Anomaly Spike Detection (P1)

| Aspect | Detail |
|--------|--------|
| **Priority** | P1 |
| **Backend** | `GET /api/v1/metrics/servers/{id}/anomalies?range=24h` — computes rolling stats (7-day avg + stddev) from `server_metrics`. Current value compared to {avg ± 2×stddev}. Returns `[{ recorded_at, metric, value, avg, stddev, severity }]`. Severity: "warning" (2-3× stddev), "critical" (>3× stddev). Computed on the fly, not stored (data already in DB). |
| **Frontend** | Red triangle markers on chart at anomaly points. Expandable list below chart: "⚠️ 3 anomalies detected in last 24h". Each item: timestamp, metric name, actual value, expected range. Click → scrolls chart to that point. |
| **UX** | Anomaly count badge on the chart header. When toggling metrics on/off, anomaly markers filter accordingly. "Dismiss" button per anomaly (hides it, stored in localStorage or backend flag). |

**Algorithm:**

```go
func DetectAnomalies(points []MetricPoint, window int) []Anomaly {
    // window = 7 days of data (10080 points at 60s interval)
    if len(points) < window {
        return nil // not enough data
    }
    // compute rolling avg + stddev over sliding window
    anomalies := []Anomaly{}
    for i := window; i < len(points); i++ {
        windowPoints := points[i-window : i]
        avg, stddev := meanAndStddev(windowPoints)
        val := points[i].Value
        if val > avg+2*stddev || val < avg-2*stddev {
            anomalies = append(anomalies, Anomaly{
                Time:   points[i].Time,
                Value:  val,
                Avg:    avg,
                Stddev: stddev,
                Ratio:  math.Abs(val-avg) / stddev,
            })
        }
    }
    return anomalies
}
```

#### F5 — Capacity Projection (P2)

| Aspect | Detail |
|--------|--------|
| **Priority** | P2 |
| **Backend** | `GET /api/v1/metrics/projections` — runs linear regression on 30-day disk usage trend for each server. Slope → daily growth rate. Project to 100% usage. Returns `[{ server_id, server_name, disk_total, disk_used, daily_growth_bytes, days_until_full, confidence }]`. Confidence: "high" (>14 days of data, R² > 0.7), "medium" (7-14 days), "low" (<7 days or R² < 0.3). |
| **Frontend** | Cards sorted by most urgent (fewest days until full). Each card: server name, disk usage progress bar, "🚨 Disk full in ~47 days at current rate" or "✅ Disk looks healthy (>365 days)". Color code: red (<30 days), amber (30-90 days), green (>90 days). |
| **UX** | Projection section on the main capacity page, above server charts. Collapsible per server. Click → scroll to that server's chart with disk metric auto-selected. Refresh button to recompute. |

**Linear regression:**

```go
func ProjectDaysUntilFull(points []DiskPoint, diskTotal int64) (float64, float64) {
    // Simple linear regression: y = mx + c
    // x = day offset from first data point
    // y = disk_used_bytes
    // m = daily growth rate (bytes/day)
    // days_until_full = (diskTotal - latest_used) / m
    
    n := float64(len(points))
    if n < 2 {
        return 0, 0
    }
    
    var sumX, sumY, sumXY, sumX2 float64
    for i, p := range points {
        x := float64(i) // day offset
        y := float64(p.DiskUsedBytes)
        sumX += x
        sumY += y
        sumXY += x * y
        sumX2 += x * x
    }
    
    slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
    intercept := (sumY - slope*sumX) / n
    
    // R² for confidence
    // ...
    
    daysUntilFull := float64(diskTotal-points[len(points)-1].DiskUsedBytes) / slope
    return daysUntilFull, rSquared
}
```

---

## 4. API Design

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | /api/v1/metrics/servers/{id}?range=24h&metric=cpu,memory,disk | Time-series data for a server | ✅ |
| GET | /api/v1/metrics/containers/{id}?range=7d&metric=cpu,memory | Time-series data for a container | ✅ |
| GET | /api/v1/metrics/servers/{id}/containers | List containers with latest metrics | ✅ |
| GET | /api/v1/metrics/servers/{id}/anomalies?range=24h | Anomaly spikes for a server | ✅ |
| GET | /api/v1/metrics/projections | Disk full estimates for all servers | ✅ |
| GET | /api/v1/metrics/overview | Summary cards (top consumers, anomalies count) | ✅ |

### Request / Response Examples

**GET /api/v1/metrics/servers/abc-123?range=24h&metric=cpu,memory**
```json
{
  "server_id": "abc-123",
  "range": "24h",
  "metrics": ["cpu", "memory"],
  "points": [
    {
      "recorded_at": "2026-06-11T08:00:00Z",
      "cpu_percent": 45.2,
      "memory_percent": 62.1,
      "disk_percent": null
    },
    {
      "recorded_at": "2026-06-11T08:01:00Z",
      "cpu_percent": 47.8,
      "memory_percent": 62.3,
      "disk_percent": null
    }
  ],
  "anomalies": [
    {
      "recorded_at": "2026-06-11T07:30:00Z",
      "metric": "cpu",
      "value": 89.5,
      "avg_7d": 42.1,
      "stddev_7d": 8.3,
      "severity": "critical"
    }
  ],
  "moving_avg_7d": [
    { "recorded_at": "2026-06-11T08:00:00Z", "cpu_percent": 44.8, "memory_percent": 61.5 }
  ]
}
```

**GET /api/v1/metrics/servers/abc-123/containers**
```json
{
  "server_id": "abc-123",
  "containers": [
    {
      "container_id": "def456",
      "container_name": "web-1",
      "cpu_percent": 12.3,
      "memory_percent": 25.4,
      "memory_bytes": 268435456,
      "last_seen": "2026-06-11T08:00:00Z"
    },
    {
      "container_id": "ghi789",
      "container_name": "worker-1",
      "cpu_percent": 45.6,
      "memory_percent": 78.9,
      "memory_bytes": 845571686,
      "last_seen": "2026-06-11T08:00:00Z"
    }
  ]
}
```

**GET /api/v1/metrics/projections**
```json
{
  "servers": [
    {
      "server_id": "abc-123",
      "server_name": "prod-web-01",
      "disk_total_gb": 100.0,
      "disk_used_gb": 72.4,
      "daily_growth_gb": 0.58,
      "days_until_full": 47,
      "confidence": "high",
      "estimated_full_date": "2026-08-27"
    },
    {
      "server_id": "def-456",
      "server_name": "prod-db-01",
      "disk_total_gb": 500.0,
      "disk_used_gb": 310.2,
      "daily_growth_gb": 0.12,
      "days_until_full": 1582,
      "confidence": "medium",
      "estimated_full_date": "2030-10-15"
    }
  ]
}
```

**GET /api/v1/metrics/overview**
```json
{
  "server_count": 5,
  "total_anomalies_24h": 7,
  "critical_anomalies_24h": 2,
  "top_cpu_server": { "server_id": "abc-123", "server_name": "prod-web-01", "avg_cpu_24h": 68.4 },
  "top_memory_server": { "server_id": "def-456", "server_name": "prod-db-01", "avg_memory_24h": 82.1 },
  "most_urgent_projection": { "server_id": "abc-123", "server_name": "prod-web-01", "days_until_full": 47 },
  "avg_cpu_all": 42.3,
  "avg_memory_all": 58.7,
  "avg_disk_all": 65.2
}
```

---

## 5. UI/UX Design Guidelines

### Key Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Anjungan  ●  Capacity                                    [Time Range]  │
│  Dashboard                                                              │
│  Servers                                                                │
│  Domains                                                                │
│  Monitoring                                                             │
│  ├─ Uptime                    ┌── Capacity Overview ────────────────┐  │
│  ├─ SSL                       │  ┌──────┐ ┌──────┐ ┌──────┐       │  │
│  └─ Capacity            ○ ←  │  │ CPU  │ │ RAM  │ │ DISK │       │  │
│  Projects                      │  avg  │ │ avg  │ │ avg  │       │  │
│  Registry                      │  42%  │ │  58% │ │  65% │       │  │
│                                │  └──────┘ └──────┘ └──────┘       │  │
│                                │  ⚠️ 7 anomalies in 24h            │  │
│                                └────────────────────────────────────┘  │
│                                ┌── Capacity Projections ──────────────┐│
│                                │  🚨 prod-web-01 — Full in ~47 days  ││
│                                │  ⚠️ staging-api-01 — Full in ~82 d  ││
│                                │  ✅ prod-db-01 — Full in >1 year    ││
│                                └──────────────────────────────────────┘│
│                                ┌── Server Selector ───────────────┐   │
│                                │  [prod-web-01 ▼]                 │   │
│                                │  ┌──────────────────────────┐    │   │
│                                │  │ CPU ████████████░░ 45%   │    │   │
│                                │  │ RAM ██████████████░░ 62% │    │   │
│                                │  │ DISK ████████████░░ 72%  │    │   │
│                                │  └──────────────────────────┘    │   │
│                                │                                   │   │
│                                │  ┌── Time-Series Chart ────────┐ │   │
│                                │  │  ☑ CPU ☑ RAM ☑ DISK       │ │   │
│                                │  │  ┌─────────────────────┐    │ │   │
│                                │  │  │  📈 line chart      │    │ │   │
│                                │  │  │  ▲ anomaly markers  │    │ │   │
│                                │  │  │  - - moving avg    │    │ │   │
│                                │  │  └─────────────────────┘    │ │   │
│                                │  │  [1h] [6h] [24h] [7d] [30d]│ │   │
│                                │  └─────────────────────────────┘ │   │
│                                │                                   │   │
│                                │  ┌── Anomaly Alerts ────────────┐│   │
│                                │  │  🔴 Jun 11 07:30 — CPU 89.5%││   │
│                                │  │     (avg: 42.1%, 5.7x stddev)││   │
│                                │  │  🟡 Jun 11 03:15 — RAM 91.2%││   │
│                                │  │     (avg: 62.3%, 3.2x stddev)││   │
│                                │  └──────────────────────────────┘│   │
│                                │                                   │   │
│                                │  ┌── Containers ────────────────┐│   │
│                                │  │  Name      │ CPU │ RAM │    ││   │
│                                │  │────────────│─────│──────│    ││   │
│                                │  │ web-1      │ 12% │ 25%  │    ││   │
│                                │  │ worker-1   │ 46% │ 79% ⚠ │   ││   │
│                                │  │ cache-1    │ 2%  │ 18%  │    ││   │
│                                │  └──────────────────────────────┘│   │
│                                └──────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────────┘
```

### Key UX Principles

1. **Server-first navigation** — user selects a server first, then sees all capacity data for that server in one scrollable page
2. **Overview at a glance** — three metric cards (CPU/RAM/Disk) with colored progress bars immediately show current state
3. **Time range as persistent filter** — selecting "7d" changes the chart AND anomaly detection window consistently
4. **Progressive disclosure** — from overview → server chart → container list → container modal; user drills down only as needed
5. **Anomalies as annotations, not alerts** — shown on chart as markers + list below; no push notifications, no noise
6. **Capacity projections as call-to-action** — most urgent servers listed at top with clear "days until full" estimate
7. **Moving average as optional overlay** — dashed line toggled by user, not forced
8. **Empty states** — "No data yet — collecting metrics..." when server just added (first 60s). "Insufficient data for projection" when < 7 days of data

### Color System

| Element | Color | Hex |
|---------|-------|-----|
| CPU line + progress | Emerald | `#10b981` |
| RAM line + progress | Indigo | `#818cf8` |
| Disk line + progress | Amber | `#f59e0b` |
| Anomaly marker (critical) | Red | `#ef4444` |
| Anomaly marker (warning) | Yellow | `#eab308` |
| Moving average overlay | Same metric color, opacity 40% | `rgba(...)` |
| Projection urgent | Red bg | `rgba(239,68,68,0.1)` |
| Projection normal | Green bg | `rgba(16,185,129,0.1)` |

### Container Chart Modal

```
┌───────────────────────────────────────────────────┐
│  Container: web-1                       [× Close] │
│  Server: prod-web-01                              │
│                                                   │
│  ☑ CPU  ☑ RAM                                    │
│  ┌───────────────────────────────────────┐       │
│  │  📈 line chart                        │       │
│  │  (CPU + RAM, time on X, % on Y)       │       │
│  │                                       │       │
│  └───────────────────────────────────────┘       │
│  [1h] [6h] [24h] [7d] [30d]                      │
│                                                   │
│  Current: CPU 12.3% · RAM 268 MB (25.4%)        │
└───────────────────────────────────────────────────┘
```

---

## 6. Non-Functional Requirements

| Aspect | Target | Notes |
|--------|--------|-------|
| Poll interval | 60 seconds | Balance of granularity vs storage cost |
| Raw data retention | 90 days | Auto-purge via cron, delete rows older than 90 days |
| Rollup retention | 1 year (hourly avg) | After 90 days, aggregate into hourly rollup tables, delete raw |
| Chart render time | < 1 second for 30 days of data | Downsampling: hourly avg for > 7d range, 4-hour avg for > 30d |
| API response time | < 500ms for 7d range | Properly indexed queries, limit to 10080 points max |
| Storage per server per day | ~8 MB (raw) | 1440 rows × server_metrics (~200 bytes) + container_metrics per container |
| Concurrent SSH sessions | Max 5 simultaneous | Semaphore to prevent resource exhaustion |
| SSH timeout | 10 seconds per server | Context cancellation with timeout |
| Failure tolerance | Poll continues on per-server failure | One server down doesn't block others |
| Data consistency | No gap-filling initially | Future: linear interpolation for missing points |

---

## 7. Implementation Roadmap

### Phase 1: Collection & Storage Foundation

| Order | Feature | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | Metrics collection cron (`docker stats` + `df` poller, SSH, parser) | 2 days | — |
| 2 | `server_metrics` + `container_metrics` tables + migration | 1 day | — |
| 3 | Batch insert storage layer (bulk INSERT or COPY) | 1 day | #1, #2 |
| 4 | `hourly_rollup` tables + cron for retention | 1 day | #2 |
| 5 | `GET /api/v1/metrics/servers/{id}` endpoint + downsampling | 1 day | #3 |
| 6 | `GET /api/v1/metrics/servers/{id}/containers` endpoint | 0.5 day | #3 |

### Phase 2: Frontend Charts & Container Drill-Down

| Order | Feature | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | Chart.js frontend — server chart with time range selector | 2 days | Phase 1 #5 |
| 2 | Overview bar (CPU/RAM/Disk progress) | 0.5 day | Phase 1 #5 |
| 3 | Container table + drill-down modal with chart | 1 day | Phase 1 #6 |
| 4 | Moving average overlay toggle (7-day) | 0.5 day | #1 |
| 5 | `GET /api/v1/metrics/containers/{id}` endpoint | 0.5 day | Phase 1 #3 |

### Phase 3: Anomaly Detection & Capacity Projection

| Order | Feature | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | Anomaly detection algorithm (rolling avg + stddev) | 1 day | Phase 1 #3 |
| 2 | `GET /api/v1/metrics/servers/{id}/anomalies` endpoint | 0.5 day | #1 |
| 3 | Anomaly markers on chart + anomaly list UI | 1 day | #2, Phase 2 #1 |
| 4 | Capacity projection (linear regression) | 1 day | Phase 1 #3 |
| 5 | `GET /api/v1/metrics/projections` + projection cards | 1 day | #4 |
| 6 | `GET /api/v1/metrics/overview` + overview dashboard cards | 0.5 day | #4, Phase 2 |

### Phase 4: Polish & Hardening (Future)

| Order | Feature | Effort | Dependency |
|-------|---------|--------|------------|
| 1 | CSV export for chart data | 0.5 day | Phase 2 |
| 2 | Gap-filling (linear interpolation for missing points) | 1 day | Phase 1 |
| 3 | Custom date range picker (beyond preset buttons) | 0.5 day | Phase 2 |
| 4 | Dark mode chart colors | 0.5 day | Phase 2 |
| 5 | Multi-server comparison (overlay two servers) | 1 day | Phase 2 |

---

## 8. Design Decisions

### 8.1 Simple PostgreSQL Time-Series (No TimescaleDB)

**Why:** At single-user or small-team scale (~5-10 servers, ~1440 data points per server per day), PostgreSQL handles this trivially. A `BIGSERIAL` PK with proper indexes gives sub-millisecond range queries. TimescaleDB adds operational complexity (extension install, hypertable management) without benefit at this scale.

**Pattern:** Two tables (`server_metrics`, `container_metrics`) with `recorded_at DESC` BRIN or B-tree indexes. Hourly rollups in separate tables for data > 90 days.

**Trade-off:** If scale grows to 100+ servers or sub-second polling, we'd need TimescaleDB or a dedicated TSDB (VictoriaMetrics, InfluxDB). But that's a future problem — YAGNI.

### 8.2 SSH + Docker CLI Over Docker API Socket

**Why:** Servers may not expose Docker API socket remotely (security best practice). SSH is already set up for Anjungan's server management. `docker stats --no-stream --format '{{json .}}'` gives clean JSON output easily parsed in Go.

**Pattern:** `golang.org/x/crypto/ssh` for SSH sessions. Reuse existing server SSH key management. Run command, capture stdout, parse line-by-line JSON.

**Trade-off:** SSH overhead adds ~100-200ms per server per poll cycle. At 5 servers + 60s interval, this is negligible. If we had 50+ servers, consider an agent-based approach (or Prometheus Node Exporter).

### 8.3 Chart.js Over D3.js

**Why:** Chart.js is simpler, lighter (60KB minified vs 250KB+ for D3), and sufficient for our use case (line charts with time axes, tooltips, zoom). D3 would be overkill — we don't need custom SVG animations, complex layouts, or force-directed graphs.

**Pattern:** Chart.js `time` scale with `adapters` for date handling. `chartjs-plugin-annotation` for anomaly markers.

**Trade-off:** Chart.js is less flexible for highly custom visualizations. If we needed multi-pane coordinated charts or custom interactions, D3 would be better. But for line charts with tooltips, Chart.js is perfect.

### 8.4 Anomaly Detection On-the-Fly (Not Pre-Computed)

**Why:** Computing 7-day rolling average + stddev on every query is fast enough (a few milliseconds for 10080 points). Pre-computing and storing anomaly flags adds complexity (when to recompute? how to handle deleted data?).

**Pattern:** Calculate rolling stats in the API handler. Query last 7 days of points, compute mean/stddev for sliding window, compare each current point. Return anomaly array alongside chart data.

**Trade-off:** Slightly higher CPU per query. Mitigated by caching: we could cache the anomaly results for 60 seconds (matching poll interval). If response time becomes an issue, we can materialize anomaly flags in a separate table and update them on each poll.

### 8.5 60-Second Poll Interval

**Why:** Balances granularity (1-minute resolution is sufficient for capacity trends) with storage cost (~1440 rows/day/server = ~8MB raw). Memory leaks and CPU spikes are visible at 1-minute resolution. Sub-second polling would add noise without insight benefit.

**Pattern:** `time.Ticker` in Go. Jitter of ±5s randomized to prevent thundering herd. If poll takes longer than 60s (unlikely), skip next cycle to catch up.

**Trade-off:** 60-second resolution can miss transient spikes (< 60s). If this becomes a concern, we could add event-based triggers (e.g., on container OOM kill). For now, capacity planning doesn't need sub-minute granularity.

### 8.6 Downsampling on Read (Not Pre-Aggregated)

**Why:** Store raw data at 60s resolution, downsample on query based on time range. 1h → raw, 6h → raw, 24h → raw, 7d → hourly avg, 30d → 4-hour avg. This preserves flexibility (future analysis needs raw data) while keeping chart rendering fast.

**Pattern:** SQL query with `date_bin` for downsampling:

```sql
-- Hourly avg for 7d range
SELECT
    date_trunc('hour', recorded_at) AS bucket,
    AVG(cpu_percent) AS cpu_percent_avg,
    AVG(memory_percent) AS memory_percent_avg,
    AVG(disk_percent) AS disk_percent_avg
FROM server_metrics
WHERE server_id = $1
  AND recorded_at > NOW() - INTERVAL '7 days'
GROUP BY bucket
ORDER BY bucket;
```

**Trade-off:** If we had millions of rows per query, pre-aggregated materialized views would be needed. At our scale, on-the-fly aggregation is fast enough with proper indexes.

---

## 9. Glossary

| Term | Definition |
|------|-----------|
| Server Metrics | Time-series CPU%, RAM%, Disk% data collected per server every 60 seconds |
| Container Metrics | Time-series CPU%, RAM%, network/block I/O per container, collected same interval |
| Anomaly | Data point deviating >2× standard deviation from 7-day rolling average |
| Capacity Projection | Linear regression estimate of when disk usage will reach 100% |
| Downsampling | Reducing data resolution for display (e.g., raw → hourly avg) |
| Rollup | Aggregated hourly data stored long-term after raw data retention expires |
| Moving Average | 7-day sliding window average shown as overlay on charts |
| SSE | Server-Sent Events — future option for real-time chart updates |
| BRIN Index | Block Range Index — efficient for time-ordered data in PostgreSQL |

---

## 10. Related Documents

- [PRD-servers.md](./PRD-servers.md) — Server management (SSH keys, connection), dependency for metrics collection
- [PRD-notification-engine.md](./PRD-notification-engine.md) — Anomaly detection could integrate with notification engine for alerts (future)
- [PRD-uptime-monitoring.md](./PRD-uptime-monitoring.md) — Complementary monitoring feature (uptime vs capacity)
- [PRD-ssl-monitoring.md](./PRD-ssl-monitoring.md) — SSL monitoring, another piece of infrastructure health
- [PRD-resource-usage-cost.md](./PRD-resource-usage-cost.md) — Resource usage data could feed into cost calculations
