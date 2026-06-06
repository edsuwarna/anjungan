<script>
	import Icon from '@iconify/svelte';
	import StatCard from '$lib/components/charts/StatCard.svelte';
	import { api } from '$lib/api.svelte.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
import { loadThresholds, getThresholds, scoreColor, scoreLabel } from '$lib/thresholds.svelte.js';

	let stats = $state({ servers: 0, containers: 0, deployments: 0, users: 0, server_status: {}, deployment_status: {}, compliance: null, server_scores: {}, recent_activity: [], recent_deployments: [] });
	let serverList = $state([]);
	let loading = $state(true);
	let error = $state('');
	let serversLoading = $state(true);
	let showActivity = $state(false);
	let hoveredServer = $state(null);

	let compliance = $derived(stats.compliance || { total_servers: 0, scanned_servers: 0, average_score: null, by_status: {} });
	let deploymentStatus = $derived(stats.deployment_status || {});
	let activeDeployments = $derived(deploymentStatus['running'] || 0);
	let serverScores = $derived(stats.server_scores || {});

	// Overall health: green if all online + avg compliance meets threshold
	let healthStatus = $derived(
		(statusCounts['offline'] || 0) > 0 ? 'warning' :
		(compliance.average_score != null && compliance.average_score < getThresholds().warning) ? 'critical' :
		(compliance.average_score != null && compliance.average_score < getThresholds().compliant) ? 'warning' :
		'good'
	);

	let statusCounts = $derived(stats.server_status || {});
	let onlineCount = $derived(statusCounts['online'] || 0);
	let offlineCount = $derived(statusCounts['offline'] || 0);

	onMount(async () => {
		await Promise.all([loadDashboard(), loadServers(), loadThresholds()]);
		const interval = setInterval(() => { loadDashboard(); loadServers(); }, 30000);
		return () => clearInterval(interval);
	});

	async function loadDashboard() {
		try {
			const data = await api.dashboard.summary();
			stats = { servers: 0, containers: 0, deployments: 0, users: 0, server_status: {}, deployment_status: {}, compliance: null, server_scores: {}, recent_activity: [], recent_deployments: [], ...data };
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadServers() {
		try {
			serverList = await api.servers.list({ all: true });
		} catch (_) {} finally {
			serversLoading = false;
		}
	}

	function statusClass(status) {
		switch (status) {
			case 'online': return 'online';
			case 'offline': return 'offline';
			default: return 'pending';
		}
	}

	function totalStatus() {
		return Object.values(statusCounts).reduce((a, b) => a + b, 0) || stats.servers;
	}

	function statusPercent(status) {
		const total = totalStatus();
		if (total === 0) return 0;
		return ((statusCounts[status] || 0) / total * 100).toFixed(0);
	}

	function formatDate(dateStr) {
		if (!dateStr) return '';
		const d = new Date(dateStr);
		const now = new Date();
		const diff = now - d;
		const mins = Math.floor(diff / 60000);
		const hours = Math.floor(diff / 3600000);
		const days = Math.floor(diff / 86400000);
		if (mins < 1) return 'Just now';
		if (mins < 60) return `${mins}m ago`;
		if (hours < 24) return `${hours}h ago`;
		if (days < 7) return `${days}d ago`;
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
	}

	function activityIcon(type) {
		switch (type) {
			case 'server_added': return 'solar:server-square-bold';
			case 'deployment': return 'solar:rocket-bold';
			case 'alert': return 'solar:danger-triangle-bold';
			default: return 'solar:info-circle-bold';
		}
	}

	function deployStatusColor(status) {
		switch (status) {
			case 'running': return 'var(--color-success)';
			case 'completed': return 'var(--color-primary)';
			case 'failed': return 'var(--color-danger)';
			default: return 'var(--color-text-muted)';
		}
	}

	function scoreBadge(serverId) {
		return serverScores[serverId] || null;
	}

	function healthColor() {
		if (healthStatus === 'good') return 'var(--color-success)';
		if (healthStatus === 'warning') return 'var(--color-warning)';
		return 'var(--color-danger)';
	}

	function healthLabel() {
		if (healthStatus === 'good') return 'All systems operational';
		if (healthStatus === 'warning') return 'Needs attention';
		return 'Issues detected';
	}

	function healthIcon() {
		if (healthStatus === 'good') return 'solar:check-circle-bold';
		if (healthStatus === 'warning') return 'solar:danger-triangle-bold';
		return 'solar:danger-circle-bold';
	}
</script>

<div class="page-container">
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading dashboard...</p>
			</div>
		</div>
	{:else if error}
		<div class="card text-center py-12">
			<Icon icon="solar:danger-triangle-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-danger);" />
			<p class="text-sm font-medium" style="color: var(--color-danger);">Failed to load dashboard</p>
			<p class="mt-1 text-xs" style="color: var(--color-text-muted);">{error}</p>
		</div>
	{:else}
		<!-- Header -->
		<div class="flex items-center justify-between flex-wrap gap-2 mb-4">
			<div>
				<h1 class="page-title inline-flex items-center gap-2">Overview
					<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium" style="background-color: {healthColor()}20; color: {healthColor()}; border: 1px solid {healthColor()}40;">
						<Icon icon={healthIcon()} class="h-3.5 w-3.5" />
						{healthLabel()}
					</span>
				</h1>
				<p class="page-subtitle">Overview of your infrastructure</p>
			</div>
		</div>

		<!-- Stat Cards -->
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
			<StatCard title="Servers" value={stats.servers} icon="solar:server-square-bold"
				subtitle={onlineCount > 0 || offlineCount > 0 ? `${onlineCount} online · ${offlineCount} offline` : ''} />
			<StatCard title="Containers" value={stats.containers} icon="solar:box-bold"
				subtitle={stats.servers > 0 ? `across ${stats.servers} server${stats.servers !== 1 ? 's' : ''}` : ''} />
			<StatCard title="Deployments" value={stats.deployments} icon="solar:rocket-bold"
				subtitle={activeDeployments > 0 ? `${activeDeployments} running` : deploymentStatus['completed'] ? `${deploymentStatus['completed']} completed` : ''} />
			<StatCard title="Users" value={stats.users} icon="solar:users-group-rounded-bold" />
			<StatCard title="Compliance" value={compliance.average_score != null ? compliance.average_score + '%' : '—'} icon="solar:shield-check-bold"
				subtitle={compliance.scanned_servers > 0 ? `${compliance.scanned_servers} scanned` : 'no scans'} />
		</div>

		<!-- Two-column: Server Health + Recent Deployments -->
		<div class="grid gap-4 lg:grid-cols-2 min-w-0 mt-4">
			<!-- Server Status Distribution -->
			<div class="card min-w-0 overflow-hidden" style="border-left: 3px solid var(--color-primary);">
				<h3 class="mb-3 text-base font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:server-square-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-primary);" /> Server Health
				</h3>
				{#if totalStatus() === 0}
					<div class="flex flex-col items-center py-8 text-center">
						<Icon icon="solar:server-square-bold" class="mb-2 inline-block h-10 w-10" style="color: var(--color-text-muted);" />
						<p class="text-sm" style="color: var(--color-text-muted);">No servers yet</p>
						<button onclick={() => goto('/servers')} class="btn-secondary mt-3 text-xs">Add a Server</button>
					</div>
				{:else}
					{@const _STATUSES = ['online', 'offline', 'unknown']}
					{@const _STATUS_COLORS = { online: 'var(--color-success)', offline: 'var(--color-danger)', unknown: 'var(--color-warning)' }}
					{@const _total = totalStatus()}
					{@const _circ = 2 * Math.PI * 44}
					<div class="flex flex-col items-center">
						<div class="relative">
							<svg width="120" height="120" viewBox="0 0 120 120" class="-rotate-90">
								{#each _STATUSES as status, i}
									{@const val = statusCounts[status] || 0}
									{@const pct = _total > 0 ? val / _total : 0}
									{@const dashLen = pct * _circ}
									{@const prevTotal = _STATUSES.slice(0, i).reduce((sum, s) => sum + (statusCounts[s] || 0), 0)}
									{@const offset = _total > 0 ? (prevTotal / _total) * _circ : 0}
									{#if val > 0}
										<circle
											cx="60" cy="60" r="44"
											fill="none"
											stroke={_STATUS_COLORS[status]}
											stroke-width="14"
											stroke-dasharray="{dashLen} {_circ - dashLen}"
											stroke-dashoffset={-_circ}
											transform="rotate({(offset / _circ) * 360} 60 60)"
											class="transition-all duration-500"
										/>
									{/if}
								{/each}
							</svg>
							<div class="absolute inset-0 flex flex-col items-center justify-center">
								<span class="text-xl font-bold" style="color: var(--color-text);">{_total}</span>
								<span class="text-xs" style="color: var(--color-text-muted);">servers</span>
							</div>
						</div>
						<div class="mt-3 flex flex-wrap justify-center gap-3">
							{#each _STATUSES as status}
								{@const count = statusCounts[status] || 0}
								{#if count > 0}
									<div class="flex items-center gap-1.5 text-xs">
										<span class="h-2.5 w-2.5 rounded-full" style="background-color: {_STATUS_COLORS[status]};"></span>
										<span style="color: var(--color-text-secondary);">{status.charAt(0).toUpperCase() + status.slice(1)}</span>
										<span class="font-semibold" style="color: var(--color-text);">{count}</span>
										<span style="color: var(--color-text-muted);">({statusPercent(status)}%)</span>
									</div>
								{/if}
							{/each}
						</div>
					</div>
				{/if}
			</div>

			<!-- Recent Deployments -->
			<div class="card min-w-0 overflow-hidden" style="border-left: 3px solid var(--color-accent);">
				<div class="flex items-center justify-between mb-3">
					<h3 class="text-base font-semibold" style="color: var(--color-text);">
						<Icon icon="solar:rocket-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-accent);" /> Recent Deployments
					</h3>
					<button onclick={() => goto('/deployments')} class="text-xs font-medium hover:underline" style="color: var(--color-primary);">View All</button>
				</div>
				{#if stats.recent_deployments.length === 0}
					<div class="flex flex-col items-center py-8 text-center">
						<Icon icon="solar:rocket-bold" class="mb-2 inline-block h-10 w-10" style="color: var(--color-text-muted);" />
						<p class="text-sm" style="color: var(--color-text-muted);">No deployments yet</p>
						<button onclick={() => goto('/deployments')} class="btn-secondary mt-3 text-xs">Create Deployment</button>
					</div>
				{:else}
					<div class="space-y-1 max-h-[240px] overflow-y-auto">
						{#each stats.recent_deployments as dep}
							<button
								onclick={() => goto(`/deployments/${dep.id}`)}
								class="w-full flex items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-opacity-50"
								style="background-color: var(--color-surface);"
							>
								<span class="h-2 w-2 rounded-full shrink-0" style="background-color: {deployStatusColor(dep.status)};"></span>
								<div class="flex-1 min-w-0">
									<p class="text-sm font-medium truncate" style="color: var(--color-text);">{dep.name}</p>
									{#if dep.server_name}
										<p class="text-xs truncate" style="color: var(--color-text-muted);">{dep.server_name}</p>
									{/if}
								</div>
								<span class="text-xs px-2 py-0.5 rounded-full font-medium shrink-0" style="color: {deployStatusColor(dep.status)}; background-color: {deployStatusColor(dep.status)}15;">{dep.status}</span>
								<span class="shrink-0 text-xs whitespace-nowrap" style="color: var(--color-text-muted);">{formatDate(dep.deployed_at)}</span>
							</button>
						{/each}
					</div>
				{/if}
			</div>
		</div>

		<!-- Compliance Summary -->
		<div class="card mt-4" style="border-left: 3px solid var(--color-primary);">
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
				{@const total = compliance.total_servers || 1}
				<div class="flex items-center gap-4 mb-3">
					<div class="text-center">
						<span class="text-3xl font-bold" style="color: {scoreColor(compliance.average_score ?? null)};">
							{compliance.average_score}%
						</span>
						<p class="text-xs" style="color: var(--color-text-muted);">avg score</p>
					</div>
					<div class="flex-1">
						<div class="flex h-2 rounded-full overflow-hidden" style="background-color: var(--color-border);">
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

		<!-- Quick Actions + Recent Activity -->
		<div class="grid gap-4 lg:grid-cols-2 min-w-0 mt-4">
			<!-- Quick Actions -->
			<div class="card" style="border-left: 3px solid var(--color-accent);">
				<h3 class="mb-3 text-base font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:flash-drive-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-accent);" /> Quick Actions
				</h3>
				<div class="flex flex-wrap gap-2">
					<button onclick={() => goto('/servers')} class="btn-primary flex items-center gap-2">
						<Icon icon="solar:add-circle-bold" class="h-4 w-4" /> Add Server
					</button>
					<button onclick={() => goto('/containers')} class="btn-secondary flex items-center gap-2">
						<Icon icon="solar:box-bold" class="h-4 w-4" /> View Containers
					</button>
					<button onclick={() => goto('/deployments')} class="btn-secondary flex items-center gap-2">
						<Icon icon="solar:rocket-bold" class="h-4 w-4" /> Deployments
					</button>
				</div>
			</div>

			<!-- Recent Activity -->
			<div class="card min-w-0 overflow-hidden" style="border-left: 3px solid var(--color-warning);">
				<div class="flex items-center justify-between mb-3">
					<h3 class="text-base font-semibold" style="color: var(--color-text);">
						<Icon icon="solar:clock-circle-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-warning);" /> Recent Activity
					</h3>
					<button onclick={() => showActivity = !showActivity} class="text-xs font-medium hover:underline" style="color: var(--color-primary);">
						{showActivity ? 'Collapse' : 'Expand'}
					</button>
				</div>
				{#if !showActivity}
					{#if stats.recent_activity.length > 0}
						<p class="text-xs py-2" style="color: var(--color-text-muted);">{stats.recent_activity.length} recent event{stats.recent_activity.length !== 1 ? 's' : ''}</p>
					{:else}
						<p class="text-xs py-2" style="color: var(--color-text-muted);">No recent activity</p>
					{/if}
				{:else}
					{#if stats.recent_activity.length === 0}
						<div class="flex flex-col items-center py-4 text-center">
							<Icon icon="solar:clock-circle-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-text-muted);" />
							<p class="text-sm" style="color: var(--color-text-muted);">No recent activity yet</p>
						</div>
					{:else}
						<div class="space-y-1 max-h-[200px] overflow-y-auto">
							{#each stats.recent_activity as activity}
								<div class="flex items-start gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-opacity-50" style="background-color: var(--color-surface);">
									<Icon icon={activityIcon(activity.type)} class="mt-0.5 h-4 w-4 shrink-0" style="color: var(--color-primary);" />
									<div class="flex-1 min-w-0">
										<p class="text-sm truncate" style="color: var(--color-text);">{activity.message}</p>
									</div>
									<span class="shrink-0 text-xs whitespace-nowrap" style="color: var(--color-text-muted);">{formatDate(activity.timestamp)}</span>
								</div>
							{/each}
						</div>
					{/if}
				{/if}
			</div>
		</div>

		<!-- Servers Grid -->
		<div class="card min-w-0 overflow-hidden mt-4" style="border-left: 3px solid var(--color-success);">
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
						{@const badge = scoreBadge(server.id)}
						<div
							class="relative rounded-lg border p-3 transition-colors group"
							style="border-color: var(--color-border-light); background-color: var(--color-surface);"
							role="button"
							tabindex="0"
							onmouseenter={() => hoveredServer = server.id}
							onmouseleave={() => hoveredServer = null}
						>
							<button
								onclick={() => goto(`/servers/${server.id}`)}
								class="flex items-start gap-3 text-left w-full"
							>
								<span class="status-dot {statusClass(server.status)} mt-1.5 shrink-0"></span>
								<div class="min-w-0 flex-1">
									<div class="flex items-center gap-2">
										<p class="text-sm font-medium truncate" style="color: var(--color-text);">{server.name}</p>
										{#if badge}
											<span class="text-xs px-1.5 py-0.5 rounded-full font-medium shrink-0" style="color: {scoreColor(badge.score)}; background-color: {scoreColor(badge.score)}15;">
												{badge.score != null ? badge.score + '%' : '—'}
											</span>
										{/if}
									</div>
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
							<!-- Quick action buttons on hover -->
							{#if hoveredServer === server.id}
								<div class="absolute top-2 right-2 flex gap-1">
									<button
										onclick={(e) => { e.stopPropagation(); goto(`/servers/${server.id}/terminal`); }}
										class="flex items-center justify-center h-7 w-7 rounded-md transition-colors hover:bg-opacity-80"
										style="background-color: var(--color-surface-hover); color: var(--color-text-secondary);"
										title="Terminal"
									>
										<Icon icon="solar:terminal-bold" class="h-3.5 w-3.5" />
									</button>
									<button
										onclick={(e) => { e.stopPropagation(); goto(`/compliance`); }}
										class="flex items-center justify-center h-7 w-7 rounded-md transition-colors hover:bg-opacity-80"
										style="background-color: var(--color-surface-hover); color: var(--color-text-secondary);"
										title="Scan"
									>
										<Icon icon="solar:shield-check-bold" class="h-3.5 w-3.5" />
									</button>
								</div>
							{/if}
						</div>
					{/each}
				</div>
				{#if serverList.length > 8}
					<p class="mt-2 text-center text-xs" style="color: var(--color-text-muted);">Showing 8 of {serverList.length} servers</p>
				{/if}
			{/if}
		</div>
	{/if}
</div>
