# Overview Page Improvement Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Transform the Overview page from a basic stat dashboard into a real "at a glance" command center — remove fake alert data, add compliance summary, fix deployments count, and redesign the layout for better information density.

**Architecture:** Backend consolidates three existing data sources (servers, deployments, compliance) into a single enriched dashboard endpoint. Frontend replaces the scattered card layout with a cohesive grid: top stat cards, two-column middle section (server status donut + compliance summary), and a full-width bottom section (server health list). No new tables or migrations needed — all data already exists in the DB.

**Tech Stack:** Go (chi router, pgx), Svelte 5 (runes mode), Tailwind CSS, Iconify Solar icons.

---

## HIGH — Backend: Data layer & dashboard endpoint

### Task 1: Add `CountDeployments()` to repository

**Objective:** Add a method to count total deployments from the `deployments` table.

**Files:**
- Modify: `backend/internal/common/db/repository.go` — after `SumContainerCount()` (~line 434)

**Step 1: Add the method**

```go
func (r *Repository) CountDeployments(ctx context.Context) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM deployments").Scan(&count)
	return count, err
}
```

**Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: no errors

**Step 3: Commit**

```bash
git add backend/internal/common/db/repository.go
git commit -m "feat: add CountDeployments to repository"
```

---

### Task 2: Remove alerts from dashboard handler & add compliance + deployments

**Objective:** Update `dashboard/handler.go` — remove `CountUnacknowledgedAlerts`, `CountAlertsBySeverity`; add `CountDeployments` and `GetComplianceSummary`. Return enriched response.

**Files:**
- Modify: `backend/internal/dashboard/handler.go`

**Step 1: Rewrite the handler**

Replace the entire `Summary` function with:

```go
package dashboard

import (
	"net/http"
	"time"

	"github.com/edsuwarna/anjungan/internal/common"
	"github.com/edsuwarna/anjungan/internal/common/db"
)

type Handler struct {
	repo *db.Repository
}

func NewHandler(repo *db.Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	serverCount, _ := h.repo.CountServers(r.Context())
	containerSum, _ := h.repo.SumContainerCount(r.Context())
	deploymentCount, _ := h.repo.CountDeployments(r.Context())
	userCount, _ := h.repo.CountUsers(r.Context())
	statusCounts, _ := h.repo.CountServersByStatus(r.Context())
	compliance, _ := h.repo.GetComplianceSummary(r.Context())
	activity, _ := h.repo.ListRecentActivity(r.Context(), 10)

	if statusCounts == nil {
		statusCounts = map[string]int{}
	}
	if activity == nil {
		activity = []struct {
			Type      string    `json:"type"`
			Message   string    `json:"message"`
			Timestamp time.Time `json:"timestamp"`
		}{}
	}

	type ActivityEntry struct {
		Type      string `json:"type"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"`
	}
	entries := make([]ActivityEntry, len(activity))
	for i, a := range activity {
		entries[i] = ActivityEntry{
			Type:      a.Type,
			Message:   a.Message,
			Timestamp: a.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	// Compact compliance summary (omit full server list to keep response lean)
	type ComplianceBrief struct {
		TotalServers   int            `json:"total_servers"`
		ScannedServers int            `json:"scanned_servers"`
		AverageScore   *int           `json:"average_score"`
		ByStatus       map[string]int `json:"by_status"`
	}
	comp := ComplianceBrief{
		TotalServers: serverCount,
		ByStatus:     map[string]int{},
	}
	if compliance != nil {
		comp.ScannedServers = compliance.ScannedServers
		comp.AverageScore = compliance.AverageScore
		comp.ByStatus = compliance.ByStatus
	}

	common.JSON(w, http.StatusOK, map[string]interface{}{
		"servers":       serverCount,
		"containers":    containerSum,
		"deployments":   deploymentCount,
		"users":         userCount,
		"server_status": statusCounts,
		"compliance":    comp,
		"recent_activity": entries,
	})
}
```

**Step 2: Remove unused imports**

Check that `CountUnacknowledgedAlerts` and `CountAlertsBySeverity` are no longer referenced. The import list stays clean.

**Step 3: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: no errors

**Step 4: Commit**

```bash
git add backend/internal/dashboard/handler.go
git commit -m "feat: enrich dashboard with compliance summary + real deployment count; remove alerts"
```

---

## MEDIUM — Frontend: Redesign Overview page

### Task 3: Update stat cards — replace Alerts with Compliance score

**Objective:** Remove the Alerts stat card, add a Compliance Score card showing average score (or "—" if no scans yet).

**Files:**
- Modify: `frontend/src/routes/+page.svelte` — lines 119-130

**Step 1: Replace the 5 stat cards section**

```svelte
<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-5">
	<StatCard title="Servers" value={stats.servers} icon="solar:server-square-bold" />
	<StatCard title="Containers" value={stats.containers} icon="solar:box-bold" />
	<StatCard title="Deployments" value={stats.deployments} icon="solar:rocket-bold" />
	<StatCard title="Users" value={stats.users} icon="solar:users-group-rounded-bold" />
	<StatCard
		title="Compliance"
		value={stats.compliance?.average_score != null ? stats.compliance.average_score + '%' : '—'}
		icon="solar:shield-check-bold"
		style="color: {(stats.compliance?.average_score ?? 0) >= 80 ? 'var(--color-success)' : (stats.compliance?.average_score ?? 0) >= 60 ? 'var(--color-warning)' : 'var(--color-danger)'};"
	/>
</div>
```

**Step 2: Remove alert badge from header** (lines 110-115)

Remove the `{#if stats.alerts > 0}` block — no more alert references.

**Step 3: Verify**

Run: `cd frontend && npm run build`
Expected: build passes (existing CSS warnings are pre-existing, ignore them)

---

### Task 4: Add Compliance Summary card (replaces Alerts section)

**Objective:** Add a card showing compliance status breakdown with a mini progress bar.

**Files:**
- Modify: `frontend/src/routes/+page.svelte` — replace the Alerts section (lines 222-238)

**Step 1: Add helper state**

Add to the `<script>` section top:

```js
let compliance = $derived(stats.compliance || { total_servers: 0, scanned_servers: 0, average_score: null, by_status: {} });
```

**Step 2: Replace the Alerts section with Compliance card**

Replace lines 222-238 with:

```svelte
<!-- Compliance Summary -->
<div class="card" style="border-left: 3px solid var(--color-primary);">
	<div class="flex items-center justify-between mb-3">
		<h3 class="text-base font-semibold" style="color: var(--color-text);">
			<Icon icon="solar:shield-check-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-primary);" /> Compliance
		</h3>
		<button onclick={() => goto('/compliance')} class="text-xs font-medium hover:underline" style="color: var(--color-primary);">View All</button>
	</div>
	{#if compliance.scanned_servers === 0}
		<div class="flex flex-col items-center py-4 text-center">
			<Icon icon="solar:shield-check-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-text-muted);" />
			<p class="text-sm" style="color: var(--color-text-muted);">No scans yet</p>
			<button onclick={() => goto('/compliance')} class="btn-secondary mt-2 text-xs">Run a Scan</button>
		</div>
	{:else}
		<div class="flex items-center gap-4 mb-3">
			<div class="text-center">
				<span class="text-3xl font-bold" style="color: {compliance.average_score >= 80 ? 'var(--color-success)' : compliance.average_score >= 60 ? 'var(--color-warning)' : 'var(--color-danger)'};">
					{compliance.average_score}%
				</span>
				<p class="text-xs" style="color: var(--color-text-muted);">avg score</p>
			</div>
			<div class="flex-1">
				<div class="flex h-2 rounded-full overflow-hidden" style="background-color: var(--color-border);">
					{@const total = compliance.total_servers || 1}
					{#if compliance.by_status.good}
						<div style="width: {(compliance.by_status.good / total * 100).toFixed(0)}%; background-color: var(--color-success);" class="transition-all duration-500"></div>
					{/if}
					{#if compliance.by_status.warning}
						<div style="width: {(compliance.by_status.warning / total * 100).toFixed(0)}%; background-color: var(--color-warning);" class="transition-all duration-500"></div>
					{/if}
					{#if compliance.by_status.critical}
						<div style="width: {(compliance.by_status.critical / total * 100).toFixed(0)}%; background-color: var(--color-danger);" class="transition-all duration-500"></div>
					{/if}
					{#if compliance.by_status.unscanned}
						<div style="width: {(compliance.by_status.unscanned / total * 100).toFixed(0)}%; background-color: var(--color-border);" class="transition-all duration-500"></div>
					{/if}
				</div>
				<div class="flex flex-wrap gap-x-3 gap-y-0.5 mt-2 text-xs">
					{#if compliance.by_status.good}
						<span style="color: var(--color-success);">● {compliance.by_status.good} good</span>
					{/if}
					{#if compliance.by_status.warning}
						<span style="color: var(--color-warning);">● {compliance.by_status.warning} warn</span>
					{/if}
					{#if compliance.by_status.critical}
						<span style="color: var(--color-danger);">● {compliance.by_status.critical} crit</span>
					{/if}
					{#if compliance.by_status.unscanned}
						<span style="color: var(--color-text-muted);">○ {compliance.by_status.unscanned} unscanned</span>
					{/if}
				</div>
			</div>
		</div>
	{/if}
</div>
```

**Step 3: Remove obsolete alert-related code from <script>**

Remove:
- `alertSeverityColor` object (lines 82-86)
- Any alert references in the loadDashboard function

**Step 4: Verify**

Run: `cd frontend && npm run build`
Expected: build passes

---

### Task 5: Redesign Server Health section

**Objective:** Improve the server list to be more compact with better status visualization and click-through.

**Files:**
- Modify: `frontend/src/routes/+page.svelte` — Server Health section (lines 259-304)

**Step 1: Replace the Server Health section**

Replace lines 258-304 with:

```svelte
<!-- Server List -->
<div class="card min-w-0 overflow-hidden" style="border-left: 3px solid var(--color-success);">
	<div class="flex items-center justify-between mb-3">
		<h3 class="text-base font-semibold" style="color: var(--color-text);">
			<Icon icon="solar:server-square-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-success);" /> Servers
		</h3>
		<button onclick={() => goto('/servers')} class="text-xs font-medium hover:underline" style="color: var(--color-primary);">View All</button>
	</div>
	{#if serversLoading}
		<div class="flex items-center justify-center py-6">
			<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-text-muted);" />
		</div>
	{:else if serverList.length === 0}
		<div class="flex flex-col items-center py-8 text-center">
			<Icon icon="solar:server-square-bold" class="mb-2 inline-block h-10 w-10" style="color: var(--color-text-muted);" />
			<p class="text-sm" style="color: var(--color-text-muted);">No servers yet</p>
			<button onclick={() => goto('/servers')} class="btn-secondary mt-3 text-xs">Add a Server</button>
		</div>
	{:else}
		<div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
			{#each serverList.slice(0, 8) as server}
				<button
					onclick={() => goto(`/servers/${server.id}`)}
					class="flex items-start gap-3 rounded-lg border p-3 text-left transition-colors hover:bg-opacity-50"
					style="border-color: var(--color-border-light); background-color: var(--color-surface);"
				>
					<span class="status-dot {statusClass(server.status)} mt-1.5 shrink-0"></span>
					<div class="min-w-0 flex-1">
						<p class="text-sm font-medium truncate" style="color: var(--color-text);">{server.name}</p>
						<p class="text-xs truncate mt-0.5" style="color: var(--color-text-muted);">{server.host}</p>
						<div class="flex flex-wrap gap-x-2 gap-y-0.5 mt-1 text-xs" style="color: var(--color-text-muted);">
							{#if server.container_count != null && server.container_count > 0}
								<span class="inline-flex items-center gap-1"><Icon icon="solar:box-bold" class="h-3 w-3" />{server.container_count}</span>
							{/if}
							{#if server.os_info}
								<span class="inline-flex items-center gap-1"><Icon icon="solar:monitor-bold" class="h-3 w-3" />{server.os_info.split('(')[0].trim()}</span>
							{/if}
						</div>
					</div>
					<span class="status-badge {statusClass(server.status)} text-xs shrink-0">{server.status || 'unknown'}</span>
				</button>
			{/each}
		</div>
		{#if serverList.length > 8}
			<p class="mt-2 text-center text-xs" style="color: var(--color-text-muted);">Showing 8 of {serverList.length} servers</p>
		{/if}
	{/if}
</div>
```

**Step 2: Verify**

Run: `cd frontend && npm run build`
Expected: build passes

---

### Task 6: Final layout polish — reorder sections

**Objective:** Reorder the page for logical flow: stat cards → (status donut + recent activity) two-column → compliance card → server cards grid.

**Files:**
- Modify: `frontend/src/routes/+page.svelte`

**Step 1: Reorder the sections**

The final layout order after the stat cards:

1. Two-column: Server Status Distribution + Recent Activity (keep as-is)
2. Compliance Summary card (from Task 4)
3. Quick Actions (keep as-is)
4. Server grid (from Task 5)

No code changes needed for this — just verify the order in the template matches.

**Step 2: Verify**

Run: `cd frontend && npm run build`
Expected: build passes

**Step 3: Commit all frontend changes**

```bash
git add frontend/src/routes/+page.svelte
git commit -m "feat: redesign overview page — compliance card, real deployments, remove alerts, server grid"
```

---

## LOW — Polish

### Task 7: Make recent activity items clickable (future enhancement)

**Note:** This requires adding `reference_type` and `reference_id` columns to the `activity_log` table and populating them when activities are created. Defer this to a separate feature PR since it needs a migration + data backfill.

---

## Verification Checklist

- [ ] `go build ./...` passes in backend
- [ ] `npm run build` passes in frontend
- [ ] Dashboard API returns: servers, containers, deployments, users, server_status, compliance, recent_activity
- [ ] Dashboard API does NOT return: alerts, alerts_by_severity
- [ ] Frontend stat cards: 5 cards (Servers, Containers, Deployments, Users, Compliance)
- [ ] No "Alerts" anywhere on the overview page
- [ ] Compliance card shows score + progress bar + status breakdown
- [ ] Server section uses card grid (not list)
- [ ] Docker rebuild: `sudo docker compose build backend frontend && sudo docker compose up -d --force-recreate backend frontend`

---

## Summary

| Area | Before | After |
|---|---|---|
| Deployments stat | Hardcoded `0` | Real count from `deployments` table |
| Alerts | Fake data from unused alerts table | Removed entirely |
| Compliance | Not on overview | Score + status bar + breakdown |
| Server list | Vertical list, 10 items | Card grid, 8 items, more compact |
| Layout | Scattered sections | Logical flow: stats → two-col → compliance → actions → servers |
