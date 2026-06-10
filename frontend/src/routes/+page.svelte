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
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
			<StatCard title="Servers" value={stats.servers} icon="solar:server-square-bold"
				subtitle={onlineCount > 0 || offlineCount > 0 ? `${onlineCount} online · ${offlineCount} offline` : ''} />
			<StatCard title="Containers" value={stats.containers} icon="solar:box-bold"
				subtitle={stats.servers > 0 ? `across ${stats.servers} server${stats.servers !== 1 ? 's' : ''}` : ''} />

			<StatCard title="Users" value={stats.users} icon="solar:users-group-rounded-bold" />
			<button class="text-left" onclick={() => goto('/ssl-monitors')}>
				<StatCard title="SSL Certs" value={sslSummary.valid ?? '—'} icon="solar:shield-check-bold"
					subtitle={sslExpiringCount > 0 ? `${sslExpiringCount} expiring` : sslSummary.total > 0 ? `${sslSummary.total} monitored` : 'no monitors'} />
			</button>
			<button class="text-left" onclick={() => goto('/uptime')}>
				<StatCard title="Uptime" value={uptimeSummary.up ?? '—'} icon="solar:chart-2-bold"
					subtitle={uptimeSummary.down > 0 ? `${uptimeSummary.down} down` : uptimeSummary.total > 0 ? `${uptimeSummary.total} monitors` : 'no monitors'} />
			</button>
			<StatCard title="Compliance" value={compliance.average_score != null ? compliance.average_score + '%' : '—'} icon="solar:shield-check-bold"
				subtitle={compliance.scanned_servers > 0 ? `${compliance.scanned_servers} scanned` : 'no scans'} />
		</div>

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
		{/if}
</div>

