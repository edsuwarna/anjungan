<script>
	import Icon from '@iconify/svelte';
	import StatCard from '$lib/components/charts/StatCard.svelte';
	import { api } from '$lib/api.svelte.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	let stats = $state({ servers: 0, containers: 0, deployments: 0, users: 0, server_status: {}, compliance: null, recent_activity: [] });
	let serverList = $state([]);
	let loading = $state(true);
	let error = $state('');
	let serversLoading = $state(true);

	let compliance = $derived(stats.compliance || { total_servers: 0, scanned_servers: 0, average_score: null, by_status: {} });

	onMount(async () => {
		await Promise.all([loadDashboard(), loadServers()]);
		const interval = setInterval(() => { loadDashboard(); loadServers(); }, 30000);
		return () => clearInterval(interval);
	});

	async function loadDashboard() {
		try {
			const data = await api.dashboard.summary();
			stats = { servers: 0, containers: 0, deployments: 0, users: 0, server_status: {}, compliance: null, recent_activity: [], ...data };
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
				<h1 class="page-title">Overview</h1>
				<p class="page-subtitle">Overview of your infrastructure</p>
			</div>
		</div>

		<!-- Stat Cards -->
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-5">
			<StatCard title="Servers" value={stats.servers} icon="solar:server-square-bold" />
			<StatCard title="Containers" value={stats.containers} icon="solar:box-bold" />
			<StatCard title="Deployments" value={stats.deployments} icon="solar:rocket-bold" />
			<StatCard title="Users" value={stats.users} icon="solar:users-group-rounded-bold" />
			<StatCard
				title="Compliance"
				value={compliance.average_score != null ? compliance.average_score + '%' : '—'}
				icon="solar:shield-check-bold"
			/>
		</div>

		<!-- Two-column: Status + Activity -->
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
				{@const total = compliance.total_servers || 1}
				<div class="flex items-center gap-4 mb-3">
					<div class="text-center">
						<span class="text-3xl font-bold" style="color: {(compliance.average_score ?? 0) >= 80 ? 'var(--color-success)' : (compliance.average_score ?? 0) >= 60 ? 'var(--color-warning)' : 'var(--color-danger)'};">
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

		<!-- Servers Grid -->
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
	{/if}
</div>
