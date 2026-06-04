# Plan: Container Page — Security Badges + Group by Server + Sort + Exit Info

## Goal
Enhance `/containers` page with:
1. 🥇 Security badges/findings on each container card
2. 🥈 Group containers by server
3. 🥉 Sort controls + exit code info for stopped containers

---

## Phase 1: Backend — Attach Security Data to Containers

### 1.1 Add `GetLatestContainerFindingsByServer` (repository)
**File:** `backend/internal/common/db/repository.go`

New method that queries the latest "Container Security" scan for a server and returns findings grouped by container name (using the `category` field which stores container name):

```go
func (r *Repository) GetLatestContainerFindingsByServer(ctx context.Context, serverID string) (map[string][]model.ScanFinding, error)
```

- Uses `GetLatestScanResultByType(serverID, "Container Security")`
- Gets findings via `GetFindingsByScanID`
- Groups findings by `f.Category` (which = container name)
- Returns map: `containerName -> []ScanFinding`

### 1.2 Add Security Fields to `ContainerInfo`
**File:** `backend/internal/container/handler.go`

Extend `ContainerInfo` struct with:
```go
type ContainerSecurityInfo struct {
    Score    int              `json:"score"`
    Findings []model.ScanFinding `json:"findings,omitempty"`
    Badges   []string         `json:"badges,omitempty"`
    ScannedAt *time.Time      `json:"scanned_at,omitempty"`
}

type ContainerInfo struct {
    // ... existing fields ...
    Security *ContainerSecurityInfo `json:"security,omitempty"`
}
```

### 1.3 Attach Security Data in List & ByServer Handlers
**File:** `backend/internal/container/handler.go`

In both `List()` and `ByServer()`:
- After gathering all containers from all servers, for each **server**, fetch the latest Container Security scan findings
- Match findings to each container by `containerName == finding.Category`
- Compute:
  - **Score**: 100 - (critical*20 + high*10 + medium*5) [clamped to 0-100]
  - **Badges**: e.g. "🔒 unprivileged ✓", "🔓 privileged", "📁 writable rootfs"
  - **Findings list**: pass to frontend for expanded view
- Add `ContainerSecurityInfo` to each `ContainerInfo`

### 1.4 Add ExitCode Info for Stopped Containers
**File:** `backend/internal/container/handler.go`

Modify the `docker ps -a` template to include `{{.Status}}` (already included in `status` field), and add a new field `exit_code`:

Current format:
```
docker ps -a --format '{"id":"{{.ID}}","name":"{{.Names}}",...'
```

The `status` field already contains the full status string like:
- "Up 12 days"
- "Exited (137) 3 hours ago"
- "Exited (0) 2 days ago"

We can parse the exit code from the status string in the frontend. No backend change needed — just use the existing `status` field.

---

## Phase 2: Frontend — Container Page Rewrite

### 2.1 Switch to `by-server` API
**File:** `frontend/src/routes/containers/+page.svelte`

- Change from `api.containers.list()` to `api.containers.byServer()`
- Response shape already exists as `ByServerResponse` with `servers[]` grouped by server

### 2.2 Add "Scan Container Security" Button
**File:** `frontend/src/routes/containers/+page.svelte`

- Button in header area: "Scan Container Security" 
- On click: scan ALL servers sequentially via `api.compliance.scanContainers(serverId)`
- Show loading state per server
- After all complete, auto-reload data
- Show last scan timestamp

### 2.3 Grouped Layout by Server
**File:** `frontend/src/routes/containers/+page.svelte`

Change card grid to server-grouped layout:
```
📌 Dokploy VPS (6 · 🛡️ 89% avg) ▼
  ├─ cards...
📌 Peladen Central (4 · 🛡️ 56% avg) ▼
  ├─ cards...
```

- Collapsible server sections
- Avg security score badge per server group header
- Running/exited counts per server

### 2.4 Security Badges on Card Header
Each container card shows:
- **Score badge** (🟢 92%, 🟡 60%, 🔴 36%) — color-coded
- **Inline badges**: pass = green, fail = red/orange
  - `🔒 unprivileged ✓`, `📦 non-root ✓` (green)
  - `🔓 privileged mode`, `🔓 runs as root` (red)
  - `🛡️ seccomp ✓`, `🛡️ no seccomp ⚠`

### 2.5 Security Findings in Expanded Section
When card is expanded:
- **Summary bar**: "🔴 3 critical · 🟠 2 high · 🟡 1 medium"
- **Findings list**: each with severity icon, title, description, remediation
- Same severity order: critical → high → medium → low → info

### 2.6 Sort Controls
**File:** `frontend/src/routes/containers/+page.svelte`

Add sort dropdown:
- Default: "Running first, then by name"
- Options: Name A-Z, Name Z-A, Score (best first), Score (worst first), Created (newest), Uptime (longest)

### 2.7 Exit Info for Stopped Containers
For containers with state "exited", show:
- Exit code badge (red if non-zero, green if 0)
- Time since exited
Example: `Exited (137) 3h ago ❌` or `Exited (0) 2d ago ✅`

### 2.8 Stats Cards Update
Add a "Security Score" stat card next to Running/Stopped/Paused:
- Average security score across all containers
- Total findings count

---

## Phase 3: New Migration (optional)

If we want `container_id` in `scan_findings`, create:
**File:** `backend/migrations/000012_add_container_id_to_findings.up.sql`
```sql
ALTER TABLE scan_findings ADD COLUMN IF NOT EXISTS container_id VARCHAR(64) DEFAULT '';
ALTER TABLE scan_findings ADD COLUMN IF NOT EXISTS container_name VARCHAR(255) DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_scan_findings_container ON scan_findings(container_id);
```

And update `TriggerContainerScan` to set these fields.

---

## Files Changed

| File | Change |
|---|---|
| `backend/internal/container/handler.go` | Add security fields, attach to ContainerInfo, add exit info |
| `backend/internal/common/db/repository.go` | Add `GetLatestContainerFindingsByServer` |
| `backend/internal/common/model/model.go` | Maybe add container scan finding model |
| `backend/internal/compliance/handler.go` | Update TriggerContainerScan to set container_id/name |
| `backend/migrations/000012_*.up.sql` | Add container_id column (optional) |
| `frontend/src/routes/containers/+page.svelte` | Main rewrite |
| `frontend/src/lib/api.svelte.js` | Maybe add byServer endpoint if missing |

## Verification
1. Build backend: `go build ./...` ✅
2. Rebuild Docker: `sudo docker compose build && sudo docker compose up -d --force-recreate` ✅
3. Manual test: visit `/containers`, verify security badges, grouping, sort, exit info
4. Run Container Security scan from the page, verify data appears
