# Remove Metrics History + VictoriaMetrics — Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Remove the in-app Metrics History section (uPlot charts) and disable VictoriaMetrics entirely — delegate historical metrics to Grafana + Prometheus. Keep only the live snapshot System Metrics cards on the Metrics tab.

**Architecture:** Rip out the VM-backed history pipeline end-to-end: frontend charts → backend query handler → VM client + collector → Docker compose service. The live snapshot (SSH-based CPU/RAM/Disk/Uptime) stays intact. PG `server_metrics` table and alerts can be cleaned up later as a separate pass; this plan focuses on the VM dependency.

**Tech Stack:** Svelte 5 (frontend), Go (backend chi router), Docker Compose

**Branch:** `feat/general-improvement`

---

## HIGH PRIORITY — Core Removal

### Task 1: Remove Metrics History HTML section from frontend

**Objective:** Delete the "Metrics History" card (uPlot charts) from the server detail page.

**Files:**
- Modify: `frontend/src/routes/servers/[id]/+page.svelte`

**What to remove:**
- Lines 982-1038: the entire `<!-- Metrics History -->` card block (from `<div class="card mt-4">` through its closing `</div>`)
- Line 8: `import MetricsChart from '$lib/components/charts/MetricsChart.svelte';`

**Verification:** Page should compile without errors. The Metrics tab should only show "System Metrics" snapshot cards.

---

### Task 2: Remove metrics history state and functions from frontend

**Objective:** Clean up all JavaScript state, functions, and effects related to metrics history.

**Files:**
- Modify: `frontend/src/routes/servers/[id]/+page.svelte`

**State variables to remove:**
- Line 34: `let metricsHistory = $state([]);`
- Line 35: `let historyRange = $state('1h');`
- Line 36: `let historyLoading = $state(false);`

**Functions to remove:**
- Lines 147-156: `loadMetricsHistory()` function
- Lines 161-165: `toggleLiveRefresh()` — Wait, keep this? Let me re-check...

Actually `toggleLiveRefresh` (lines 161-165) toggles live refresh for the System Metrics snapshot cards — NOT history. This should STAY. Only remove what's specific to history.

Wait, let me re-read lines 160-175 more carefully:
- `toggleLiveRefresh` toggles `liveEnabled` which controls a 10s interval that calls `loadMetrics()` — this is for the live System Metrics snapshot. KEEP.
- `changeRange` (lines 171-174) changes `historyRange` and fetches metrics history. REMOVE.

So remove:
- Lines 31-36: `metricsHistory`, `historyRange`, `historyLoading` state
- Lines 147-156: `loadMetricsHistory()` function  
- Lines 171-174: `changeRange()` function
- Any onMount/dev references to `loadMetricsHistory()`

Also check `loadMetricsHistory` call in the `onMount` (should be around line 148 or inside an effect).

**Verification:** No references to `metricsHistory|historyRange|historyLoading|loadMetricsHistory|changeRange` remain in the file.

---

### Task 3: Remove metricsHistory from frontend API client

**Objective:** Delete the `metricsHistory` API function.

**Files:**
- Modify: `frontend/src/lib/api.svelte.js`

**Remove:**
- Line 100: `metricsHistory: (id, range = '1h', limit = 200) => request(...)`

**Verification:** No references to `api.servers.metricsHistory` anywhere in the frontend.

---

### Task 4: Remove MetricsHistory backend handler and route

**Objective:** Delete the `/api/v1/servers/{id}/metrics/history` endpoint and its handler.

**Files:**
- Modify: `backend/internal/infra/handler.go`

**Remove:**
- Line 163: `r.Get("/{id}/metrics/history", h.MetricsHistory)` route registration
- Lines 878-1080(ish): entire `MetricsHistory()` function (all ~200 lines including the `uPlotData`, `buildLookup`, PG fallback logic)
- Lines 38-59: `NullableFloat` type and its `MarshalJSON`/`UnmarshalJSON` methods (only used by MetricsHistory)

**Verification:** 
- `go build ./...` in backend succeeds
- No references to `MetricsHistory` remain

---

### Task 5: Remove VM push from live Metrics handler

**Objective:** Stop pushing metrics snapshots to VictoriaMetrics when viewing the live snapshot.

**Files:**
- Modify: `backend/internal/infra/handler.go`

**Remove:**
- Lines 853-869: the `go func()` goroutine that calls `h.vmClient.InsertMetrics(...)`

**After removal, the Metrics() handler should:**
1. Still run SSH commands to collect CPU/RAM/Disk/Uptime/Network
2. Still save to PG via `h.repo.SaveMetrics(ctx, point)` — this is fine for now (cheap, and the PG data is a useful fallback)
3. Still return the snapshot JSON

**Verification:** `go build ./...` succeeds. No `vmClient` reference remains in Metrics handler.

---

### Task 6: Remove background metrics collector

**Objective:** Delete the background goroutine that periodically collects metrics from all servers and pushes to VM.

**Files:**
- Delete: `backend/internal/metrics/collector.go`
- Delete: `backend/internal/metrics/vm.go`
- If the `metrics/` directory becomes empty, it can be removed too.

**Verification:** `go build ./...` succeeds.

---

### Task 7: Strip VM wiring from server initialization

**Objective:** Remove all VM client and collector initialization from the server startup.

**Files:**
- Modify: `backend/internal/server/server.go`

**Remove:**
- Line 24: `"github.com/edsuwarna/anjungan/internal/metrics"` import (if collector.go/vm.go are deleted)
- Lines 61-62: VM client initialization
- Lines 64-67: Collector creation + goroutine
- Line 75: `vmClient` parameter from `setupRouter(authH, authSvc, repo, rl, vmClient)`
- Line 81: `vmClient *metrics.VMClient` parameter from `setupRouter()` signature
- Line 99: `vmClient` argument from `infra.NewHandler(repo, vmClient)` call

**Verification:** `go build ./...` succeeds.

---

### Task 8: Remove vmClient from infra Handler struct

**Objective:** Strip the `vmClient` field and `VMClient` import from the infra handler.

**Files:**
- Modify: `backend/internal/infra/handler.go`

**Remove:**
- Line 26: `vmmetrics "github.com/edsuwarna/anjungan/internal/metrics"` import
- Line 31: `vmClient *vmmetrics.VMClient` field from `Handler` struct
- Line 34: `vmClient *vmmetrics.VMClient` parameter from `NewHandler(repo *db.Repository, vmClient *vmmetrics.VMClient)`
- Line 37: `vmClient: vmClient` from struct literal in `NewHandler`

**Verification:** `go build ./...` succeeds.

---

### Task 9: Remove VMConfig from backend config

**Objective:** Clean up the VM configuration struct and environment variable.

**Files:**
- Modify: `backend/internal/config/config.go`

**Remove:**
- Line 16: `VM VMConfig` field from `Config` struct
- Lines 61-63: `VMConfig` struct definition
- Lines 133-135 (in `Load()`): the `VM: VMConfig{ URL: getEnv("VM_URL", "...") }` block

**Verification:** `go build ./...` succeeds. No references to `cfg.VM` or `VMConfig` anywhere.

---

### Task 10: Remove VictoriaMetrics from Docker Compose

**Objective:** Remove the VictoriaMetrics service and its volume.

**Files:**
- Modify: `docker-compose.yml`

**Remove:**
- Lines 100-112: the entire `victoria-metrics:` service block
- Line 117: `vmdata:` from `volumes:` section

**Verification:** `docker compose config` (or just visual check) — no references to victoria-metrics or vmdata.

---

## MEDIUM PRIORITY — Cleanup (optional, can be deferred)

### Task 11 (Optional): Remove server_metrics PG table + repository methods

**Objective:** Drop the `server_metrics` table and related repository code. (The live Metrics handler still writes to it — either remove that too, or leave the table as a cheap fallback.)

**Files affected:**
- `backend/internal/common/db/repository.go`: lines 436-484 (`SaveMetrics`, `GetHistoricalMetrics`)
- `backend/internal/common/model/model.go`: `ServerMetricsPoint` struct
- `backend/migrations/`: new down migration or leave as-is
- `backend/internal/infra/handler.go`: lines 835-852 (SaveMetrics call in Metrics handler)

**Decision:** Defer this. The PG writes are cheap and harmless. The table can be dropped later via a dedicated migration when the Grafana pipeline is confirmed working.

### Task 12 (Optional): Remove checkThresholds alerting

**Objective:** Remove threshold-based alert creation from Metrics handler. (Alerts should come from Prometheus/Alertmanager.)

**Files:**
- `backend/internal/infra/handler.go`: lines 1094-1127 (`checkThresholds` function + its call on line 872)

**Decision:** Defer this too. The alerts system is independent of VM and still provides value while Prometheus isn't fully set up.

---

## Verification Checklist

After all HIGH priority tasks are done:

1. **Build:** `cd ~/projects/anjungan/backend && go build ./...` — no errors
2. **Lint (frontend):** `cd ~/projects/anjungan/frontend && npm run build` — no Svelte compile errors
3. **Startup:** Backend starts without VM-related errors in logs
4. **UI:** Server detail page → Metrics tab shows only System Metrics snapshot cards
5. **API:** `GET /api/v1/servers/{id}/metrics/history` returns 404 (route removed)
6. **No VM errors:** No log spam about VM connection failures

---

## Files Summary

| File | Action |
|------|--------|
| `frontend/src/routes/servers/[id]/+page.svelte` | Remove history HTML + state + functions + MetricsChart import |
| `frontend/src/lib/api.svelte.js` | Remove `metricsHistory` function |
| `backend/internal/infra/handler.go` | Remove MetricsHistory handler, route, NullableFloat, VM push, vmClient field, metrics import |
| `backend/internal/metrics/vm.go` | DELETE |
| `backend/internal/metrics/collector.go` | DELETE |
| `backend/internal/server/server.go` | Remove VM/collector init + wiring |
| `backend/internal/config/config.go` | Remove VMConfig struct + field |
| `docker-compose.yml` | Remove victoria-metrics service + vmdata volume |
