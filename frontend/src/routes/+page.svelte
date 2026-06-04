<script>
	import Icon from '@iconify/svelte';
	import StatCard from '$lib/components/charts/StatCard.svelte';
	import { api } from '$lib/api.svelte.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	let stats = $state({ servers: 0, containers: 0, deployments: 0, users: 0, alerts: 0, alerts_by_severity: {}, server_status: {}, recent_activity: [] });
	let serverList = $state([]);
	let loading = $state(true);
	let error = $state('');
	let serversLoading = $state(true);

	onMount(async () => {
		await Promise.all([loadDashboard(), loadServers()]);
		const interval = setInterval(() => { loadDashboard(); loadServers(); }, 30000);
		return () => clearInterval(interval);
	});

	async function loadDashboard() {
		try {
			const data = await api.dashboard.summary();
			stats = { servers: 0, containers: 0, deployments: 0, users: 0, alerts: 0, alerts_by_severity: {}, server_status: {}, recent_activity: [], ...data };
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
		const s = stats.server_status || {};
		return Object.values(s).reduce((a, b) => a + b, 0) || stats.servers;
	}

	function statusPercent(status) {
		const total = totalStatus();
		if (total === 0) return 0;
		return ((stats.server_status?.[status] || 0) / total * 100).toFixed(0);
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

	const alertSeverityColor = {
		critical: 'var(--color-danger)',
		warning: 'var(--color-warning)',
		info: 'var(--color-primary)'
	};
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
		<div class="flex items-center justify-between flex-wrap gap-2">
			<div>
				<h1 class="page-title">Overview</h1>
				<p class="page-subtitle">Overview of your infrastructure</p>
			</div>
			{#if stats.alerts > 0}
				<div class="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium" style="background-color: rgba(239,68,68,0.1); color: var(--color-danger);">
					<Icon icon="solar:danger-triangle-bold" class="h-3.5 w-3.5" />
					{stats.alerts} alert{stats.alerts !== 1 ? 's' : ''}
				</div>
			{/if}
		</div>

		<!-- Stat Cards -->
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-5">
			<StatCard title="Servers" value={stats.servers} icon="solar:server-square-bold" />
			<StatCard title="Containers" value={stats.containers} icon="solar:box-bold" />
			<StatCard title="Deployments" value={stats.deployments} icon="solar:rocket-bold" />
			<StatCard title="Users" value={stats.users} icon="solar:users-group-rounded-bold" />
			<StatCard
				title="Alerts"
				value={stats.alerts}
				icon="solar:danger-triangle-bold"
				style="color: {stats.alerts > 0 ? 'var(--color-danger)' : 'inherit'};"
			/>
		</div>

		<div class="grid gap-4 lg:grid-cols-2 min-w-0">
			<!-- Server Status Distribution -->
			<div class="card min-w-0 overflow-hidden" style="border-left: 3px solid var(--color-primary);">
				<h3 class="mb-4 text-base font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:server-square-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-primary);" /> Server Status
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
							<svg width="140" height="140" viewBox="0 0 120 120" class="-rotate-90">
								{#each _STATUSES as status, i}
									{@const val = stats.server_status?.[status] || 0}
									{@const pct = _total > 0 ? val / _total : 0}
									{@const dashLen = pct * _circ}
									{@const prevTotal = _STATUSES.slice(0, i).reduce((sum, s) => sum + (stats.server_status?.[s] || 0), 0)}
									{@const offset = _total > 0 ? (prevTotal / _total) * _circ : 0}
									{#if val > 0}
										<circle
											cx="60" cy="60" r="44"
											fill="none"
											stroke={_STATUS_COLORS[status]}
											stroke-width="16"
											stroke-dasharray="{dashLen} {_circ - dashLen}"
											stroke-dashoffset={-_circ}
											transform="rotate({(offset / _circ) * 360} 60 60)"
											class="transition-all duration-500"
										/>
									{/if}
								{/each}
							</svg>
							<div class="absolute inset-0 flex flex-col items-center justify-center">
								<span class="text-2xl font-bold" style="color: var(--color-text);">{_total}</span>
								<span class="text-xs" style="color: var(--color-text-muted);">servers</span>
							</div>
						</div>
						<div class="mt-4 flex flex-wrap justify-center gap-4">
							{#each _STATUSES as status}
								{@const count = stats.server_status?.[status] || 0}
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

			<!-- Recent Activity -->
			<div class="card min-w-0 overflow-hidden" style="border-left: 3px solid var(--color-accent);">
				<h3 class="mb-4 text-base font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:clock-circle-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-accent);" /> Recent Activity
				</h3>
				{#if stats.recent_activity.length === 0}
					<div class="flex flex-col items-center py-8 text-center">
						<Icon icon="solar:clock-circle-bold" class="mb-2 inline-block h-10 w-10" style="color: var(--color-text-muted);" />
						<p class="text-sm" style="color: var(--color-text-muted);">No recent activity yet</p>
						<p class="text-xs" style="color: var(--color-text-muted);">Start by adding a server</p>
					</div>
				{:else}
					<div class="space-y-1 max-h-[300px] overflow-y-auto">
						{#each stats.recent_activity as activity}
							<div class="flex items-start gap-3 rounded-lg px-3 py-2.5 transition-colors hover:bg-opacity-50" style="background-color: var(--color-surface);">
								<Icon icon={activityIcon(activity.type)} class="mt-0.5 h-4 w-4 shrink-0" style="color: var(--color-primary);" />
								<div class="flex-1 min-w-0">
									<p class="text-sm truncate" style="color: var(--color-text);">{activity.message}</p>
								</div>
								<span class="shrink-0 text-xs whitespace-nowrap" style="color: var(--color-text-muted);">{formatDate(activity.timestamp)}</span>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>

		<!-- Alerts Summary -->
		{#if stats.alerts_by_severity && Object.keys(stats.alerts_by_severity).length > 0}
			<div class="card" style="border-left: 3px solid var(--color-danger);">
				<h3 class="mb-3 text-base font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:danger-triangle-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-danger);" /> Active Alerts
				</h3>
				<div class="flex flex-wrap gap-3">
					{#each Object.entries(stats.alerts_by_severity) as [severity, count]}
						{@const color = alertSeverityColor[severity] || 'var(--color-text-muted)'}
						<div class="flex items-center gap-2 rounded-lg border px-4 py-2.5" style="border-color: {color}; background-color: {color}10;">
							<Icon icon="solar:danger-triangle-bold" class="h-4 w-4" style="color: {color};" />
							<span class="text-sm font-medium" style="color: {color};">{severity}</span>
							<span class="text-lg font-bold" style="color: {color};">{count}</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}

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

		<!-- Server Health -->
		<div class="card min-w-0 overflow-hidden" style="border-left: 3px solid var(--color-success);">
			<div class="flex items-center justify-between mb-3">
				<h3 class="text-base font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:heart-pulse-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-success);" /> Server Health
				</h3>
				<button onclick={() => goto('/servers')} class="text-xs font-medium hover:underline" style="color: var(--color-primary);">View All</button>
			</div>
			{#if serversLoading}
				<div class="flex items-center justify-center py-6">
					<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-text-muted);" />
				</div>
			{:else if serverList.length === 0}
				<div class="flex flex-col items-center py-6 text-center">
					<Icon icon="solar:server-square-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-text-muted);" />
					<p class="text-sm" style="color: var(--color-text-muted);">No servers yet</p>
				</div>
			{:else}
				<div class="divide-y overflow-hidden rounded-lg border" style="border-color: var(--color-border-light);">
					{#each serverList.slice(0, 10) as server}
						<button
							onclick={() => goto(`/servers/${server.id}`)}
							class="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-opacity-50"
							style="background-color: var(--color-surface);"
						>
							<span class="status-dot {statusClass(server.status)}"></span>
							<div class="flex-1 min-w-0">
								<p class="text-sm font-medium truncate" style="color: var(--color-text);">{server.name}</p>
								<p class="text-xs truncate" style="color: var(--color-text-muted);">{server.host}:{server.port || 22}</p>
								<p class="text-xs mt-0.5 flex flex-wrap gap-x-2" style="color: var(--color-text-muted);">
									{#if server.server_group}<span class="inline-flex items-center gap-1"><Icon icon="solar:folder-bold" class="h-3 w-3" />{server.server_group}</span>{/if}
									{#if server.server_type}<span class="inline-flex items-center gap-1"><Icon icon="solar:widget-bold" class="h-3 w-3" />{server.server_type}</span>{/if}
									{#if server.region}<span class="inline-flex items-center gap-1"><Icon icon="solar:global-bold" class="h-3 w-3" />{server.region}</span>{/if}
									{#if server.container_count != null}<span class="inline-flex items-center gap-1"><Icon icon="solar:box-bold" class="h-3 w-3" />{server.container_count} container{server.container_count !== 1 ? 's' : ''}</span>{/if}
									{#if server.os_info}<span title={server.os_info} class="inline-flex items-center gap-1"><Icon icon="solar:monitor-bold" class="h-3 w-3" />{server.os_info.split('(')[0].trim()}</span>{/if}
								</p>
							</div>
							<span class="status-badge {statusClass(server.status)} text-xs">{server.status || 'unknown'}</span>
							<Icon icon="solar:alt-arrow-right-bold" class="h-4 w-4 shrink-0" style="color: var(--color-text-muted);" />
						</button>
					{/each}
				</div>
				{#if serverList.length > 10}
					<p class="mt-2 text-center text-xs" style="color: var(--color-text-muted);">Showing 10 of {serverList.length} servers</p>
				{/if}
			{/if}
		</div>
	{/if}
</div>
