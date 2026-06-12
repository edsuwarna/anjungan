<script>
	import Icon from '@iconify/svelte';
	import StatCard from '$lib/components/charts/StatCard.svelte';
	import { api } from '$lib/api.svelte.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
import { loadThresholds, getThresholds, scoreColor, scoreLabel } from '$lib/thresholds.svelte.js';

	let stats = $state({ servers: 0, containers: 0, users: 0, server_status: {}, compliance: null, server_scores: {}, recent_activity: [] });
	let serverList = $state([]);
	let loading = $state(true);
	let error = $state('');
	let serversLoading = $state(true);
	let showActivity = $state(false);
	let hoveredServer = $state(null);

	let sslSummary = $derived(stats.ssl_summary || { total: 0, valid: 0, expiring_soon: 0, expired: 0, error: 0 });
	let sslExpiringCount = $derived(sslSummary.expiring_soon + sslSummary.expired);
	let uptimeSummary = $derived(stats.uptime_summary || { total: 0, up: 0, down: 0, paused: 0 });

	let compliance = $derived(stats.compliance || { total_servers: 0, scanned_servers: 0, average_score: null, by_status: {} });
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

	// Quick Access items
	let quickAccess = $derived([
		{ label: 'Servers', icon: 'solar:server-square-bold', count: stats.servers, route: '/servers', color: '#8b5cf6' },
		{ label: 'Containers', icon: 'solar:box-bold', count: stats.containers, route: '/containers', color: '#8b5cf6' },
		{ label: 'SSL Monitors', icon: 'solar:shield-check-bold', count: sslSummary.total, route: '/ssl-monitors', color: '#10b981' },
		{ label: 'Uptime', icon: 'solar:chart-2-bold', count: uptimeSummary.total, route: '/uptime', color: '#f59e0b' },
		{ label: 'Registry', icon: 'solar:database-bold', count: null, route: '/registry', color: '#14b8a6' },
		{ label: 'SSH Keys', icon: 'solar:key-bold', count: null, route: '/ssh-keys', color: '#ec4899' },
		{ label: 'Compliance', icon: 'solar:shield-check-bold', count: null, route: '/compliance', color: '#14b8a6' },
		{ label: 'Notifications', icon: 'solar:bell-bold', count: stats.recent_activity?.length || 0, route: '/notifications', color: '#f59e0b' },
	]);

	onMount(async () => {
		await Promise.all([loadDashboard(), loadServers(), loadThresholds()]);
		const interval = setInterval(() => { loadDashboard(); loadServers(); }, 30000);
		return () => clearInterval(interval);
	});

	async function loadDashboard() {
		try {
			const data = await api.dashboard.summary();
			stats = { servers: 0, containers: 0, users: 0, server_status: {}, compliance: null, server_scores: {}, recent_activity: [], ...data };
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
			case 'alert': return 'solar:danger-triangle-bold';
			default: return 'solar:info-circle-bold';
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

	function uptimeAvailability() {
		const total = uptimeSummary.up + uptimeSummary.down;
		if (total === 0) return 100;
		return Math.round((uptimeSummary.up / total) * 100);
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

		<!-- Stat Cards Row -->
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
			<StatCard title="Servers" value={stats.servers} icon="solar:server-square-bold"
				subtitle={onlineCount > 0 || offlineCount > 0 ? `${onlineCount} online · ${offlineCount} offline` : ''} />
			<button class="text-left" onclick={() => goto('/ssl-monitors')}>
				<StatCard title="SSL Monitors" value={sslSummary.valid ?? '—'} icon="solar:shield-check-bold"
					subtitle={sslExpiringCount > 0 ? `${sslExpiringCount} expiring` : sslSummary.total > 0 ? `${sslSummary.total} monitored` : 'no monitors'} />
			</button>
			<button class="text-left" onclick={() => goto('/uptime')}>
				<StatCard title="Uptime Monitors" value={uptimeSummary.up ?? '—'} icon="solar:chart-2-bold"
					subtitle={uptimeSummary.down > 0 ? `${uptimeSummary.down} down` : uptimeSummary.total > 0 ? `${uptimeSummary.total} monitors` : 'no monitors'} />
			</button>
			<StatCard title="Compliance" value={compliance.average_score != null ? compliance.average_score + '%' : '—'} icon="solar:shield-check-bold"
				subtitle={compliance.scanned_servers > 0 ? `${compliance.scanned_servers} scanned` : 'no scans'} />
		</div>

		<!-- Middle Cards: 2-column grid -->
		<div class="grid gap-4 mt-4 lg:grid-cols-2">
			<!-- Server Health -->
			<div class="card" style="border-left: 3px solid var(--color-primary);">
				<h3 class="mb-3 text-base font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:server-square-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-primary);" /> Server Health
				</h3>
				{#if totalStatus() === 0}
					<div class="flex flex-col items-center py-4 text-center">
						<Icon icon="solar:server-square-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-text-muted);" />
						<p class="text-sm" style="color: var(--color-text-muted);">No servers yet</p>
						<button onclick={() => goto('/servers')} class="btn-secondary mt-2 text-xs">Add a Server</button>
					</div>
				{:else}
					<div class="space-y-3">
						<!-- Online -->
						<div class="flex items-center justify-between text-sm">
							<div class="flex items-center gap-2">
								<span class="h-2.5 w-2.5 rounded-full" style="background-color: var(--color-success);"></span>
								<span style="color: var(--color-text-secondary);">Online</span>
							</div>
							<div class="flex items-center gap-2">
								<span class="font-semibold" style="color: var(--color-text);">{onlineCount}</span>
								<span class="text-xs" style="color: var(--color-text-muted);">({statusCounts['online'] ? Math.round((statusCounts['online'] / totalStatus()) * 100) : 0}%)</span>
							</div>
						</div>
						<!-- Progress bar -->
						<div class="flex h-2 rounded-full overflow-hidden" style="background-color: var(--color-border);">
							<div class="h-full rounded-full transition-all duration-500" style="width: {totalStatus() > 0 ? Math.round(((statusCounts['online'] || 0) / totalStatus()) * 100) : 0}%; background-color: var(--color-success);"></div>
						</div>
						<!-- Offline -->
						<div class="flex items-center justify-between text-sm">
							<div class="flex items-center gap-2">
								<span class="h-2.5 w-2.5 rounded-full" style="background-color: var(--color-danger);"></span>
								<span style="color: var(--color-text-secondary);">Offline</span>
							</div>
							<div class="flex items-center gap-2">
								<span class="font-semibold" style="color: var(--color-text);">{offlineCount}</span>
								<span class="text-xs" style="color: var(--color-text-muted);">({statusCounts['offline'] ? Math.round((statusCounts['offline'] / totalStatus()) * 100) : 0}%)</span>
							</div>
						</div>
						<!-- Uptime bar -->
						<div class="mt-2 pt-2 border-t" style="border-color: var(--color-border);">
							<div class="flex items-center justify-between text-sm">
								<span style="color: var(--color-text-secondary);">Uptime</span>
								<span class="font-semibold" style="color: var(--color-success);">
									{totalStatus() > 0 ? Math.round(((statusCounts['online'] || 0) / totalStatus()) * 100) : 0}%
								</span>
							</div>
							<div class="flex h-1.5 rounded-full overflow-hidden mt-1.5" style="background-color: var(--color-border);">
								<div class="h-full rounded-full" style="width: {totalStatus() > 0 ? Math.round(((statusCounts['online'] || 0) / totalStatus()) * 100) : 0}%; background-color: var(--color-success);"></div>
							</div>
						</div>
					</div>
				{/if}
			</div>

			<!-- SSL Certificates -->
			<div class="card" style="border-left: 3px solid var(--color-success);">
				<h3 class="mb-3 text-base font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:shield-check-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-success);" /> SSL Certificates
				</h3>
				{#if sslSummary.total === 0}
					<div class="flex flex-col items-center py-4 text-center">
						<Icon icon="solar:shield-check-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-text-muted);" />
						<p class="text-sm" style="color: var(--color-text-muted);">No SSL monitors yet</p>
						<button onclick={() => goto('/ssl-monitors')} class="btn-secondary mt-2 text-xs">Add Monitor</button>
					</div>
				{:else}
					<div class="space-y-3">
						<div class="flex items-center justify-between text-sm">
							<div class="flex items-center gap-2">
								<Icon icon="solar:check-circle-bold" class="h-4 w-4" style="color: var(--color-success);" />
								<span style="color: var(--color-text-secondary);">Valid</span>
							</div>
							<span class="font-semibold" style="color: var(--color-text);">{sslSummary.valid || 0}</span>
						</div>
						<div class="flex items-center justify-between text-sm">
							<div class="flex items-center gap-2">
								<Icon icon="solar:danger-triangle-bold" class="h-4 w-4" style="color: var(--color-warning);" />
								<span style="color: var(--color-text-secondary);">Expiring Soon</span>
							</div>
							<span class="font-semibold" style="color: var(--color-warning);">{sslSummary.expiring_soon || 0}</span>
						</div>
						<div class="flex items-center justify-between text-sm">
							<div class="flex items-center gap-2">
								<Icon icon="solar:close-circle-bold" class="h-4 w-4" style="color: var(--color-danger);" />
								<span style="color: var(--color-text-secondary);">Expired</span>
							</div>
							<span class="font-semibold" style="color: var(--color-danger);">{sslSummary.expired || 0}</span>
						</div>
						<div class="flex items-center justify-between text-sm">
							<div class="flex items-center gap-2">
								<Icon icon="solar:info-circle-bold" class="h-4 w-4" style="color: var(--color-text-muted);" />
								<span style="color: var(--color-text-secondary);">Error</span>
							</div>
							<span class="font-semibold" style="color: var(--color-text);">{sslSummary.error || 0}</span>
						</div>
						<!-- Stacked status bar -->
						{#if (sslSummary.valid + sslSummary.expiring_soon + sslSummary.expired + sslSummary.error) > 0}
							{@const sslTotal = sslSummary.valid + sslSummary.expiring_soon + sslSummary.expired + sslSummary.error}
							<div class="flex h-2 rounded-full overflow-hidden mt-2" style="background-color: var(--color-border);">
								{#if sslSummary.valid > 0}
									<div style="width: {(sslSummary.valid / sslTotal * 100).toFixed(0)}%; background-color: var(--color-success);" class="transition-all duration-500"></div>
								{/if}
								{#if sslSummary.expiring_soon > 0}
									<div style="width: {(sslSummary.expiring_soon / sslTotal * 100).toFixed(0)}%; background-color: var(--color-warning);" class="transition-all duration-500"></div>
								{/if}
								{#if sslSummary.expired > 0}
									<div style="width: {(sslSummary.expired / sslTotal * 100).toFixed(0)}%; background-color: var(--color-danger);" class="transition-all duration-500"></div>
								{/if}
								{#if sslSummary.error > 0}
									<div style="width: {(sslSummary.error / sslTotal * 100).toFixed(0)}%; background-color: var(--color-text-muted);" class="transition-all duration-500"></div>
								{/if}
							</div>
						{/if}
					</div>
				{/if}
			</div>
		</div>

		<!-- Second row: Uptime + Compliance -->
		<div class="grid gap-4 mt-4 lg:grid-cols-2">
			<!-- Uptime -->
			<div class="card" style="border-left: 3px solid var(--color-warning);">
				<h3 class="mb-3 text-base font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:chart-2-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-warning);" /> Uptime
				</h3>
				{#if uptimeSummary.total === 0}
					<div class="flex flex-col items-center py-4 text-center">
						<Icon icon="solar:chart-2-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-text-muted);" />
						<p class="text-sm" style="color: var(--color-text-muted);">No uptime monitors yet</p>
						<button onclick={() => goto('/uptime')} class="btn-secondary mt-2 text-xs">Add Monitor</button>
					</div>
				{:else}
					<div class="space-y-3">
						<div class="flex items-center justify-between text-sm">
							<div class="flex items-center gap-2">
								<span class="h-2.5 w-2.5 rounded-full" style="background-color: var(--color-success);"></span>
								<span style="color: var(--color-text-secondary);">Up</span>
							</div>
							<span class="font-semibold" style="color: var(--color-text);">{uptimeSummary.up || 0}</span>
						</div>
						<div class="flex items-center justify-between text-sm">
							<div class="flex items-center gap-2">
								<span class="h-2.5 w-2.5 rounded-full" style="background-color: var(--color-danger);"></span>
								<span style="color: var(--color-text-secondary);">Down</span>
							</div>
							<span class="font-semibold" style="color: var(--color-danger);">{uptimeSummary.down || 0}</span>
						</div>
						<div class="flex items-center justify-between text-sm">
							<div class="flex items-center gap-2">
								<span class="h-2.5 w-2.5 rounded-sm" style="background-color: var(--color-text-muted);"></span>
								<span style="color: var(--color-text-secondary);">Paused</span>
							</div>
							<span class="font-semibold" style="color: var(--color-text);">{uptimeSummary.paused || 0}</span>
						</div>
						<div class="mt-2 pt-2 border-t" style="border-color: var(--color-border);">
							<div class="flex items-center justify-between text-sm">
								<span style="color: var(--color-text-secondary);">Availability</span>
								<span class="font-semibold" style="color: var(--color-success);">{uptimeAvailability()}%</span>
							</div>
							<div class="flex h-1.5 rounded-full overflow-hidden mt-1.5" style="background-color: var(--color-border);">
								<div class="h-full rounded-full" style="width: {uptimeAvailability()}%; background-color: var(--color-success);"></div>
							</div>
						</div>
					</div>
				{/if}
			</div>

			<!-- Compliance Overview -->
			<div class="card" style="border-left: 3px solid var(--color-primary);">
				<div class="flex items-center justify-between mb-3">
					<h3 class="text-base font-semibold" style="color: var(--color-text);">
						<Icon icon="solar:shield-check-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-primary);" /> Compliance Overview
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
					<div class="space-y-3">
						<div class="text-center">
							<span class="text-3xl font-bold" style="color: {scoreColor(compliance.average_score ?? null)};">
								{compliance.average_score}%
							</span>
							<p class="text-xs" style="color: var(--color-text-muted);">Average Score</p>
						</div>
						<div class="space-y-2">
							<div class="flex items-center justify-between text-sm">
								<div class="flex items-center gap-2">
									<Icon icon="solar:check-circle-bold" class="h-4 w-4" style="color: var(--color-success);" />
									<span style="color: var(--color-text-secondary);">Good</span>
								</div>
								<span class="font-semibold" style="color: var(--color-success);">{compliance.by_status.good || 0}</span>
							</div>
							<div class="flex items-center justify-between text-sm">
								<div class="flex items-center gap-2">
									<Icon icon="solar:danger-triangle-bold" class="h-4 w-4" style="color: var(--color-warning);" />
									<span style="color: var(--color-text-secondary);">Warning</span>
								</div>
								<span class="font-semibold" style="color: var(--color-warning);">{compliance.by_status.warning || 0}</span>
							</div>
							<div class="flex items-center justify-between text-sm">
								<div class="flex items-center gap-2">
									<Icon icon="solar:close-circle-bold" class="h-4 w-4" style="color: var(--color-danger);" />
									<span style="color: var(--color-text-secondary);">Critical</span>
								</div>
								<span class="font-semibold" style="color: var(--color-danger);">{compliance.by_status.critical || 0}</span>
							</div>
							<div class="flex items-center justify-between text-sm">
								<div class="flex items-center gap-2">
									<Icon icon="solar:minus-circle-bold" class="h-4 w-4" style="color: var(--color-text-muted);" />
									<span style="color: var(--color-text-secondary);">Unscanned</span>
								</div>
								<span class="font-semibold" style="color: var(--color-text-muted);">{compliance.by_status.unscanned || 0}</span>
							</div>
						</div>
						<p class="text-xs text-center" style="color: var(--color-text-muted);">{compliance.scanned_servers} of {compliance.total_servers} servers scanned</p>
					</div>
				{/if}
			</div>
		</div>

		<!-- Recent Activity -->
		<div class="card mt-4" style="border-left: 3px solid var(--color-warning);">
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

		<!-- Quick Access Grid -->
		<div class="card mt-4">
			<h3 class="mb-3 text-base font-semibold" style="color: var(--color-text);">
				<Icon icon="solar:flash-drive-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-accent);" /> Quick Access
			</h3>
			<div class="grid gap-2 sm:grid-cols-2 md:grid-cols-4">
				{#each quickAccess as item}
					<button onclick={() => goto(item.route)}
						class="flex items-center gap-3 rounded-lg border p-3 text-left transition-colors hover:bg-opacity-50"
						style="border-color: var(--color-border-light); background-color: var(--color-surface);"
					>
						<div class="flex h-9 w-9 items-center justify-center rounded-lg shrink-0"
							style="background-color: {item.color}15;"
						>
							<Icon icon={item.icon} class="h-4 w-4" style="color: {item.color};" />
						</div>
						<div class="flex-1 min-w-0">
							<p class="text-sm font-medium truncate" style="color: var(--color-text);">{item.label}</p>
							<p class="text-xs" style="color: var(--color-text-muted);">
								{item.count != null ? `${item.count} items` : '— items'}
							</p>
						</div>
					</button>
				{/each}
			</div>
		</div>
	{/if}
</div>
