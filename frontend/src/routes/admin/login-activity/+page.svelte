<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';

	let summary = $state(null);
	let events = $state([]);
	let alerts = $state([]);
	let trend = $state([]);
	let heatmapData = $state([]);
	let topIPs = $state([]);
	let topUsers = $state([]);
	let blockedIPs = $state([]);
	let loading = $state(true);
	let error = $state('');
	let expandedRow = $state(null);
	let blockingIP = $state('');

	// Filters
	let eventTypeFilter = $state('');
	let statusFilter = $state('');
	let emailFilter = $state('');
	let ipFilter = $state('');
	let countryFilter = $state('');
	let searchQuery = $state('');
	let startDate = $state('');
	let endDate = $state('');

	// Trend period
	let trendDays = $state(7);

	// Sort
	let sortColumn = $state('created_at');
	let sortOrder = $state('desc');

	// Pagination
	let page = $state(1);
	let total = $state(0);
	let totalPages = $state(1);
	const limit = 50;

	let isFiltered = $derived(eventTypeFilter || statusFilter || emailFilter || ipFilter || countryFilter || searchQuery || startDate || endDate);

	const EVENT_TYPES = [
		{ value: '', label: 'All Events' },
		{ value: 'login_success', label: 'Login Success' },
		{ value: 'login_failure', label: 'Login Failure' },
		{ value: 'login_attempt', label: 'Login Attempt' },
		{ value: 'logout', label: 'Logout' },
		{ value: 'lockout', label: 'Lockout' },
		{ value: 'rate_limited', label: 'Rate Limited' },
		{ value: 'register', label: 'Register' },
		{ value: 'password_change', label: 'Password Change' },
		{ value: 'totp_setup', label: 'TOTP Setup' },
		{ value: 'totp_disable', label: 'TOTP Disable' },
		{ value: 'refresh_token', label: 'Token Refresh' },
	];

	onMount(() => {
		loadAll();
	});

	async function loadAll() {
		loading = true;
		error = '';
		try {
			const [summaryData, eventsData, alertsData, trendData, heatData, ipsData, usersData, blkData] = await Promise.all([
				api.authActivity.summary().catch(() => null),
				loadEventsRaw(),
				api.authActivity.bruteForce().catch(() => []),
				api.authActivity.trend(trendDays).catch(() => []),
				api.authActivity.heatmap(trendDays).catch(() => []),
				api.authActivity.topIPs(trendDays).catch(() => []),
				api.authActivity.topUsers(trendDays).catch(() => []),
				api.authActivity.blockedIPs().catch(() => []),
			]);
			summary = summaryData;
			events = eventsData?.data || eventsData || [];
			if (eventsData?._meta) {
				total = eventsData._meta.total || 0;
				totalPages = eventsData._meta.total_pages || 1;
			}
			alerts = alertsData || [];
			trend = trendData || [];
			heatmapData = heatData || [];
			topIPs = ipsData || [];
			topUsers = usersData || [];
			blockedIPs = blkData || [];
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadEventsRaw() {
		try {
			return await api.authActivity.events({
				page,
				limit,
				event_type: eventTypeFilter || undefined,
				status: statusFilter || undefined,
				email: emailFilter || undefined,
				ip_address: ipFilter || undefined,
				search: searchQuery || undefined,
				start_date: startDate || undefined,
				end_date: endDate || undefined,
				sort: sortColumn,
				order: sortOrder,
			});
		} catch { return []; }
	}

	async function loadEvents() {
		expandedRow = null;
		const result = await loadEventsRaw();
		events = result?.data || result || [];
		if (result?._meta) {
			total = result._meta.total || 0;
			totalPages = result._meta.total_pages || 1;
		}
	}

	function loadTrend() {
		api.authActivity.trend(trendDays).then(d => { trend = d || []; }).catch(() => {});
		api.authActivity.heatmap(trendDays).then(d => { heatmapData = d || []; }).catch(() => {});
		api.authActivity.topIPs(trendDays).then(d => { topIPs = d || []; }).catch(() => {});
		api.authActivity.topUsers(trendDays).then(d => { topUsers = d || []; }).catch(() => {});
	}

	function search() {
		page = 1;
		loadEvents();
	}

	function resetFilters() {
		eventTypeFilter = '';
		statusFilter = '';
		emailFilter = '';
		ipFilter = '';
		countryFilter = '';
		searchQuery = '';
		startDate = '';
		endDate = '';
		page = 1;
		loadEvents();
	}

	function goToPage(p) {
		if (p < 1 || p > totalPages) return;
		page = p;
		loadEvents();
	}

	function toggleRow(id) {
		expandedRow = expandedRow === id ? null : id;
	}

	function toggleSort(col) {
		if (sortColumn === col) {
			sortOrder = sortOrder === 'desc' ? 'asc' : 'desc';
		} else {
			sortColumn = col;
			sortOrder = 'desc';
		}
		page = 1;
		loadEvents();
	}

	function sortIcon(col) {
		if (sortColumn !== col) return 'solar:sort-bold';
		return sortOrder === 'desc' ? 'solar:sort-from-top-to-bottom-bold' : 'solar:sort-from-bottom-to-top-bold';
	}

	function eventBadge(type) {
		const map = {
			login_success: { label: 'Login OK', icon: 'solar:login-2-bold', color: 'var(--color-success)' },
			login_failure: { label: 'Login Fail', icon: 'solar:shield-warning-bold', color: 'var(--color-danger)' },
			login_attempt: { label: 'Login Attempt', icon: 'solar:login-2-bold', color: 'var(--color-warning)' },
			logout: { label: 'Logout', icon: 'solar:logout-2-bold', color: 'var(--color-text-secondary)' },
			lockout: { label: 'Lockout', icon: 'solar:lock-bold', color: 'var(--color-danger)' },
			rate_limited: { label: 'Rate Limited', icon: 'solar:clock-bold', color: '#f59e0b' },
			register: { label: 'Register', icon: 'solar:user-plus-bold', color: 'var(--color-success)' },
			password_change: { label: 'Password Change', icon: 'solar:key-minimalistic-bold', color: 'var(--color-warning)' },
			totp_setup: { label: 'TOTP Setup', icon: 'solar:password-minimalistic-bold', color: 'var(--color-primary)' },
			totp_disable: { label: 'TOTP Disable', icon: 'solar:password-minimalistic-bold', color: 'var(--color-text-secondary)' },
			refresh_token: { label: 'Token Refresh', icon: 'solar:refresh-bold', color: 'var(--color-primary)' },
		};
		return map[type] || { label: type, icon: 'solar:question-circle-bold', color: 'var(--color-text-muted)' };
	}

	function statusBadge(status) {
		if (status === 'success') return { label: 'Success', color: 'var(--color-success)' };
		return { label: 'Failed', color: 'var(--color-danger)' };
	}

	function formatTime(ts) {
		if (!ts) return '-';
		return new Date(ts).toLocaleString();
	}

	function shortTime(ts) {
		if (!ts) return '-';
		const d = new Date(ts);
		return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
	}

	function geoFlag(country) {
		if (!country) return '';
		const code = country.toUpperCase();
		const flags = { ID:'🇮🇩', US:'🇺🇸', CN:'🇨🇳', SG:'🇸🇬', JP:'🇯🇵', KR:'🇰🇷',
			GB:'🇬🇧', DE:'🇩🇪', FR:'🇫🇷', NL:'🇳🇱', RU:'🇷🇺', IN:'🇮🇳',
			BR:'🇧🇷', AU:'🇦🇺', CA:'🇨🇦', MY:'🇲🇾', PH:'🇵🇭', TH:'🇹🇭',
			VN:'🇻🇳', HK:'🇭🇰', TW:'🇹🇼', AE:'🇦🇪', SA:'🇸🇦' };
		return flags[code] || '';
	}

	function maxTrendVal() {
		let max = 0;
		for (const t of trend) {
			if (t.success > max) max = t.success;
			if (t.failure > max) max = t.failure;
		}
		return max || 1;
	}

	function maxHeatmapVal() {
		let max = 0;
		for (const h of heatmapData) {
			const total = h.success + h.failure;
			if (total > max) max = total;
		}
		return max || 1;
	}

	async function handleBlockIP(ip) {
		blockingIP = ip;
		try {
			await api.authActivity.blockIP(ip, 'brute force');
			blockedIPs = await api.authActivity.blockedIPs().catch(() => []) || [];
		} catch (e) {
			console.error('Failed to block IP:', e);
		} finally {
			blockingIP = '';
		}
	}

	async function handleUnblockIP(ip) {
		try {
			await api.authActivity.unblockIP(ip);
			blockedIPs = await api.authActivity.blockedIPs().catch(() => []) || [];
		} catch (e) {
			console.error('Failed to unblock IP:', e);
		}
	}

	async function loadBlockedIPs() {
		blockedIPs = await api.authActivity.blockedIPs().catch(() => []) || [];
	}

	function isIPBlocked(ip) {
		return blockedIPs.some(b => b.ip_address === ip);
	}

	const COUNTRY_CODES = [
		'', 'ID', 'US', 'CN', 'SG', 'JP', 'KR', 'GB', 'DE', 'FR',
		'NL', 'RU', 'IN', 'BR', 'AU', 'CA', 'MY', 'PH', 'TH', 'VN',
		'HK', 'TW', 'AE', 'SA'
	];
</script>

<div class="page-container">
	<!-- Header -->
	<div class="flex items-center justify-between">
		<div>
			<h1 class="page-title">Login Activity</h1>
			<p class="page-subtitle">Monitor authentication events and detect suspicious activity</p>
		</div>
		<div class="flex items-center gap-2">
			<select bind:value={trendDays} onchange={loadTrend} class="input text-sm py-1.5">
				<option value={7}>7 days</option>
				<option value={14}>14 days</option>
				<option value={30}>30 days</option>
				<option value={90}>90 days</option>
			</select>
			<button onclick={() => loadAll()} class="btn-ghost flex items-center gap-1.5 text-sm">
				<Icon icon="solar:refresh-bold" class="h-4 w-4" />
				Refresh
			</button>
		</div>
	</div>

	<!-- Summary Cards -->
	{#if summary}
		<div class="mb-6 grid grid-cols-2 gap-4 md:grid-cols-5">
			<div class="stat-card" style="border-left: 3px solid var(--color-primary);">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text-secondary);">Logins Today</p>
					<p class="mt-1 text-2xl font-bold" style="color: var(--color-text);">{summary.logins_today}</p>
				</div>
				<div class="mt-2 flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: var(--color-primary-subtle);">
					<Icon icon="solar:login-2-bold" class="h-5 w-5" style="color: var(--color-primary);" />
				</div>
			</div>
			<div class="stat-card" style="border-left: 3px solid var(--color-danger);">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text-secondary);">Failed Today</p>
					<p class="mt-1 text-2xl font-bold" style="color: var(--color-danger);">{summary.failed_today}</p>
				</div>
				<div class="mt-2 flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: #ef444415;">
					<Icon icon="solar:shield-warning-bold" class="h-5 w-5" style="color: var(--color-danger);" />
				</div>
			</div>
			<div class="stat-card" style="border-left: 3px solid #f59e0b;">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text-secondary);">Locked Today</p>
					<p class="mt-1 text-2xl font-bold" style="color: #f59e0b;">{summary.locked_today}</p>
				</div>
				<div class="mt-2 flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: #f59e0b15;">
					<Icon icon="solar:lock-bold" class="h-5 w-5" style="color: #f59e0b;" />
				</div>
			</div>
			<div class="stat-card" style="border-left: 3px solid #8b5cf6;">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text-secondary);">Unique IPs</p>
					<p class="mt-1 text-2xl font-bold" style="color: #8b5cf6;">{summary.unique_ips}</p>
				</div>
				<div class="mt-2 flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: #8b5cf615;">
					<Icon icon="solar:global-bold" class="h-5 w-5" style="color: #8b5cf6;" />
				</div>
			</div>
			<div class="stat-card" style="border-left: 3px solid #22c55e;">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text-secondary);">Success Rate</p>
					<p class="mt-1 text-2xl font-bold" style="color: #22c55e;">{summary.success_rate}%</p>
				</div>
				<div class="mt-2 flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: #22c55e15;">
					<Icon icon="solar:chart-2-bold" class="h-5 w-5" style="color: #22c55e;" />
				</div>
			</div>
		</div>
	{/if}

	<!-- Brute Force / Blocked IPs Alert -->
	<div class="mb-6 flex flex-col gap-3">
		{#if alerts.length > 0}
			<h3 class="text-sm font-semibold" style="color: var(--color-text);">🚨 Active Alerts</h3>
			{#each alerts as alert}
				<div class="card flex items-start gap-4" style="border-left: 4px solid #ef4444; background-color: #ef444405;">
					<div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full" style="background-color: #ef444415;">
						<Icon icon={alert.user_count > 5 ? 'solar:users-group-rounded-bold' : 'solar:danger-triangle-bold'} class="h-5 w-5" style="color: #ef4444;" />
					</div>
					<div class="flex-1 min-w-0">
						<div class="flex items-center gap-2">
							<h3 class="text-sm font-bold" style="color: #ef4444;">
								{alert.user_count > 5 ? '🚨 Credential Stuffing' : '🚨 Brute Force'}
							</h3>
							<span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold" style="background-color: #ef444415; color: #ef4444;">
								{alert.failures} failures / {alert.window_minutes}min ({alert.user_count} user{alert.user_count > 1 ? 's' : ''})
							</span>
						</div>
						<p class="mt-1 text-sm" style="color: var(--color-text);">
							IP: <code class="rounded bg-red-50 px-1.5 py-0.5 text-xs font-mono dark:bg-red-900/20">{alert.ip_address}</code>
							(affecting {alert.user_count} user{alert.user_count > 1 ? 's' : ''})
							{#if isIPBlocked(alert.ip_address)}
								<span class="ml-2 text-xs" style="color: #22c55e;">✓ Blocked</span>
							{/if}
						</p>
						<p class="text-xs mt-0.5" style="color: var(--color-text-muted);">
							{alert.first_attempt} &ndash; {alert.last_attempt}
						</p>
					</div>
					<button onclick={() => handleBlockIP(alert.ip_address)} disabled={blockingIP === alert.ip_address || isIPBlocked(alert.ip_address)}
						class="{isIPBlocked(alert.ip_address) ? 'btn-ghost' : 'btn-danger'} flex items-center gap-1.5 text-xs whitespace-nowrap shrink-0">
						<Icon icon={isIPBlocked(alert.ip_address) ? 'solar:shield-check-bold' : 'solar:shield-cross-bold'} class="h-3.5 w-3.5" />
						{isIPBlocked(alert.ip_address) ? 'Blocked' : blockingIP === alert.ip_address ? 'Blocking...' : 'Block IP'}
					</button>
				</div>
			{/each}
		{/if}

		<!-- Blocked IPs List -->
		{#if blockedIPs.length > 0}
			<div class="card" style="border-left: 3px solid #8b5cf6;">
				<div class="flex items-center justify-between mb-2">
					<h3 class="text-sm font-semibold" style="color: var(--color-text);">
						<Icon icon="solar:shield-check-bold" class="inline h-4 w-4 mr-1" style="color: #8b5cf6;" />
						Blocked IPs ({blockedIPs.length})
					</h3>
				</div>
				<div class="flex flex-wrap gap-2">
					{#each blockedIPs as b}
						<div class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs" style="background-color: #8b5cf615; color: #8b5cf6;">
							<code class="font-mono">{b.ip_address}</code>
							<button onclick={() => handleUnblockIP(b.ip_address)} class="hover:opacity-70" title="Unblock">
								<Icon icon="solar:close-circle-bold" class="h-3.5 w-3.5" />
							</button>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<!-- Trend Chart + Heatmap Row -->
	<div class="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
		{#if trend.length > 0}
			<div class="card">
				<h3 class="mb-3 text-sm font-semibold" style="color: var(--color-text);">
					Trend &mdash; Login {trendDays}-Day
				</h3>
				<div class="flex h-32 items-end gap-1">
					{#each trend as t}
						<div class="flex flex-1 flex-col items-center justify-end gap-0.5" title="{t.date}: {t.success} success, {t.failure} failed">
							<div class="w-full rounded-t" style="height: {(t.failure / maxTrendVal()) * 100}%; background-color: #ef4444; min-height: {t.failure > 0 ? '2px' : '0'};"></div>
							<div class="w-full rounded-t" style="height: {(t.success / maxTrendVal()) * 100}%; background-color: #22c55e; min-height: {t.success > 0 ? '2px' : '0'};"></div>
							<span class="mt-1 text-[10px]" style="color: var(--color-text-muted);">{new Date(t.date + 'T00:00:00').toLocaleDateString(undefined, {month:'short', day:'numeric'})}</span>
						</div>
					{/each}
				</div>
				<div class="mt-2 flex items-center gap-4 text-xs" style="color: var(--color-text-secondary);">
					<span class="flex items-center gap-1"><span class="inline-block h-2.5 w-2.5 rounded-sm" style="background-color:#22c55e;"></span> Success</span>
					<span class="flex items-center gap-1"><span class="inline-block h-2.5 w-2.5 rounded-sm" style="background-color:#ef4444;"></span> Failed</span>
				</div>
			</div>
		{/if}

		<!-- Hourly Heatmap -->
		{#if heatmapData.length > 0}
			<div class="card">
				<h3 class="mb-3 text-sm font-semibold" style="color: var(--color-text);">
					Hourly Distribution &mdash; {trendDays}-Day
				</h3>
				<div class="flex h-32 items-end gap-1">
					{#each heatmapData as h}
						<div class="flex flex-1 flex-col items-center justify-end" title="Hour {h.hour}:00 &mdash; {h.success} success, {h.failure} failed">
							<div class="w-full" style="height: {((h.failure + h.success) / maxHeatmapVal()) * 100}%; background-color: hsl({(h.failure / Math.max(h.success + h.failure, 1)) * 10}, 70%, {(1 - (h.success + h.failure) / maxHeatmapVal()) * 40 + 30}%); min-height: {h.failure + h.success > 0 ? '2px' : '0'};"></div>
							<span class="mt-1 text-[10px]" style="color: var(--color-text-muted);">{h.hour}h</span>
						</div>
					{/each}
				</div>
				<div class="mt-2 flex items-center gap-4 text-xs" style="color: var(--color-text-secondary);">
					<span>Color intensity = attempt volume, Hue = failure ratio</span>
				</div>
			</div>
		{/if}
	</div>

	<!-- Top IPs + Top Users Row -->
	<div class="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
		{#if topIPs.length > 0}
			<div class="card">
				<h3 class="mb-3 text-sm font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:global-bold" class="inline h-4 w-4 mr-1" style="color: var(--color-danger);" />
					Top IPs &mdash; Most Failures ({trendDays}d)
				</h3>
				<div class="space-y-2">
					{#each topIPs as ip, i}
						<div class="flex items-center gap-2 text-xs">
							<span class="w-5 text-center font-bold" style="color: var(--color-text-muted);">{i + 1}.</span>
							<span class="font-mono">{ip.ip_address}</span>
							<span class="text-base">{geoFlag(ip.country)}</span>
							<span class="ml-auto font-semibold" style="color: var(--color-danger);">{ip.failures} failures</span>
							<span class="text-xs" style="color: var(--color-text-muted);">({ip.users} user{ip.users > 1 ? 's' : ''})</span>
							<button onclick={() => handleBlockIP(ip.ip_address)} disabled={blockingIP === ip.ip_address || isIPBlocked(ip.ip_address)}
								class="{isIPBlocked(ip.ip_address) ? 'text-green-500' : 'text-red-500'} hover:opacity-70">
								<Icon icon={isIPBlocked(ip.ip_address) ? 'solar:shield-check-bold' : 'solar:shield-cross-bold'} class="h-3.5 w-3.5" />
							</button>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		{#if topUsers.length > 0}
			<div class="card">
				<h3 class="mb-3 text-sm font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:users-group-rounded-bold" class="inline h-4 w-4 mr-1" style="color: var(--color-warning);" />
					Top Users &mdash; Most Failures ({trendDays}d)
				</h3>
				<div class="space-y-2">
					{#each topUsers as u, i}
						<div class="flex items-center gap-2 text-xs">
							<span class="w-5 text-center font-bold" style="color: var(--color-text-muted);">{i + 1}.</span>
							<span class="font-mono">{u.email}</span>
							<span class="ml-auto font-semibold" style="color: var(--color-warning);">{u.failures} failures</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<!-- Filters -->
	<div class="card mb-6" style="border-left: 3px solid var(--color-primary);">
		<div class="grid grid-cols-1 gap-3 md:grid-cols-4 lg:grid-cols-7">
			<div>
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Event Type</label>
				<select bind:value={eventTypeFilter} class="input text-sm">
					{#each EVENT_TYPES as et}
						<option value={et.value}>{et.label}</option>
					{/each}
				</select>
			</div>
			<div>
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Status</label>
				<select bind:value={statusFilter} class="input text-sm">
					<option value="">All</option>
					<option value="success">Success</option>
					<option value="failure">Failed</option>
				</select>
			</div>
			<div>
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Email</label>
				<input bind:value={emailFilter} class="input text-sm" placeholder="Filter by email..."
					onkeydown={(e) => { if (e.key === 'Enter') search(); }} />
			</div>
			<div>
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">IP Address</label>
				<input bind:value={ipFilter} class="input text-sm" placeholder="e.g. 192.168.1.1"
					onkeydown={(e) => { if (e.key === 'Enter') search(); }} />
			</div>
			<div>
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Country</label>
				<select bind:value={countryFilter} class="input text-sm">
					<option value="">All Countries</option>
					{#each COUNTRY_CODES as cc}
						{#if cc}
							<option value={cc}>{geoFlag(cc)} {cc}</option>
						{/if}
					{/each}
				</select>
			</div>
			<div>
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Start Date</label>
				<input bind:value={startDate} type="date" class="input text-sm" />
			</div>
			<div>
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">End Date</label>
				<input bind:value={endDate} type="date" class="input text-sm" />
			</div>
		</div>
		<div class="mt-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
			<div class="flex items-center gap-2">
				<button onclick={search} class="btn-primary flex items-center gap-2">
					<Icon icon="solar:filter-bold" class="h-4 w-4" />
					Apply
				</button>
				{#if isFiltered}
					<button onclick={resetFilters} class="btn-ghost text-sm">Clear filters</button>
				{/if}
			</div>
			<div class="flex items-center gap-2">
				{#if !loading}
					<span class="text-xs" style="color: var(--color-text-muted);">{total} events</span>
					<button onclick={() => api.authActivity.exportCSV({event_type: eventTypeFilter || undefined, status: statusFilter || undefined, search: searchQuery || undefined, email: emailFilter || undefined})} class="btn-ghost text-xs flex items-center gap-1 py-1 px-2">
						<Icon icon="solar:export-bold" class="h-3 w-3" />
						Export CSV
					</button>
				{/if}
			</div>
		</div>
	</div>

	<!-- Events Table -->
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading login activity...</p>
			</div>
		</div>
	{:else if error}
		<div class="card flex flex-col items-center gap-3 py-10 text-center" style="border-left: 3px solid var(--color-danger);">
			<Icon icon="solar:danger-triangle-bold" class="mb-1 h-8 w-8" style="color: var(--color-danger);" />
			<p style="color: var(--color-danger);">Failed to load login activity</p>
			<p class="text-sm" style="color: var(--color-text-muted);">{error}</p>
			<button onclick={loadAll} class="btn-secondary mt-2">Retry</button>
		</div>
	{:else if events.length === 0}
		<div class="card flex flex-col items-center py-16 text-center" style="border-left: 3px solid var(--color-text-muted);">
			<Icon icon="solar:login-2-bold" class="mb-3 h-12 w-12" style="color: var(--color-text-muted);" />
			<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No events found</h3>
			<p class="text-sm" style="color: var(--color-text-secondary);">
				{isFiltered ? 'Try different filter criteria.' : 'Auth events will appear as users log in and perform actions.'}
			</p>
		</div>
	{:else}
		<div class="data-table">
			<table class="w-full">
				<thead>
					<tr>
						<th class="w-6"></th>
						<th class="cursor-pointer" onclick={() => toggleSort('event_type')}>
							<div class="flex items-center gap-1">
								Event <Icon icon={sortIcon('event_type')} class="h-3 w-3" />
							</div>
						</th>
						<th class="cursor-pointer" onclick={() => toggleSort('email')}>
							<div class="flex items-center gap-1">
								User <Icon icon={sortIcon('email')} class="h-3 w-3" />
							</div>
						</th>
						<th class="cursor-pointer" onclick={() => toggleSort('status')}>
							<div class="flex items-center gap-1">
								Status <Icon icon={sortIcon('status')} class="h-3 w-3" />
							</div>
						</th>
						<th class="cursor-pointer" onclick={() => toggleSort('ip_address')}>
							<div class="flex items-center gap-1">
								IP Address <Icon icon={sortIcon('ip_address')} class="h-3 w-3" />
							</div>
						</th>
						<th>Geo</th>
						<th>Reason / Agent</th>
						<th class="cursor-pointer" onclick={() => toggleSort('created_at')}>
							<div class="flex items-center gap-1">
								Time <Icon icon={sortIcon('created_at')} class="h-3 w-3" />
							</div>
						</th>
					</tr>
				</thead>
				<tbody>
					{#each events as e}
						<tr class="cursor-pointer" onclick={() => toggleRow(e.id)}>
							<td class="text-center">
								<Icon icon={expandedRow === e.id ? 'solar:alt-arrow-down-bold' : 'solar:alt-arrow-right-bold'} class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
							</td>
							<td>
								<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
									style="background-color: {eventBadge(e.event_type).color}15; color: {eventBadge(e.event_type).color};">
									<Icon icon={eventBadge(e.event_type).icon} class="h-3 w-3" />
									{eventBadge(e.event_type).label}
								</span>
							</td>
							<td class="max-w-[160px] truncate font-mono text-xs" title={e.email}>{e.email}</td>
							<td>
								<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
									style="background-color: {statusBadge(e.status).color}15; color: {statusBadge(e.status).color};">
									<Icon icon={e.status === 'success' ? 'solar:check-circle-bold' : 'solar:close-circle-bold'} class="h-3 w-3" />
									{statusBadge(e.status).label}
								</span>
							</td>
							<td class="font-mono text-xs">{e.ip_address}</td>
							<td class="text-base">{geoFlag(e.country)}</td>
							<td class="max-w-[200px] truncate text-xs" style="color: var(--color-text-secondary);" title={e.failure_reason || e.user_agent}>
								{e.failure_reason || e.user_agent || '-'}
							</td>
							<td class="text-xs whitespace-nowrap" style="color: var(--color-text-secondary);" title={formatTime(e.created_at)}>
								{shortTime(e.created_at)}
							</td>
						</tr>
						{#if expandedRow === e.id}
							<tr class="expanded-row">
								<td colspan="8">
									<div class="grid grid-cols-2 gap-3 p-3 text-xs md:grid-cols-4">
										<div>
											<span class="block font-medium" style="color: var(--color-text-secondary);">Event ID</span>
											<span class="font-mono" style="color: var(--color-text);">{e.id}</span>
										</div>
										<div>
											<span class="block font-medium" style="color: var(--color-text-secondary);">User ID</span>
											<span class="font-mono" style="color: var(--color-text);">{e.user_id || '-'}</span>
										</div>
										<div>
											<span class="block font-medium" style="color: var(--color-text-secondary);">Country</span>
											<span>{geoFlag(e.country)} {e.country || '-'}</span>
										</div>
										<div>
											<span class="block font-medium" style="color: var(--color-text-secondary);">ASN / ISP</span>
											<span>{e.asn || '-'} {e.isp ? '/ ' + e.isp : ''}</span>
										</div>
										<div class="col-span-2 md:col-span-4">
											<span class="block font-medium" style="color: var(--color-text-secondary);">User Agent</span>
											<span class="break-all" style="color: var(--color-text);">{e.user_agent || '-'}</span>
										</div>
									</div>
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Pagination -->
		{#if totalPages > 1}
			<div class="mt-4 flex items-center justify-center gap-2">
				<button onclick={() => goToPage(1)} disabled={page === 1} class="btn-ghost px-2 py-1 text-xs">
					<Icon icon="solar:alt-arrow-left-bold" class="h-3 w-3" />
				</button>
				<button onclick={() => goToPage(page - 1)} disabled={page === 1} class="btn-ghost px-2 py-1 text-xs">
					<Icon icon="solar:arrow-left-bold" class="h-3 w-3" />
				</button>
				<span class="text-xs" style="color: var(--color-text-secondary);">
					Page {page} of {totalPages}
				</span>
				<button onclick={() => goToPage(page + 1)} disabled={page === totalPages} class="btn-ghost px-2 py-1 text-xs">
					<Icon icon="solar:arrow-right-bold" class="h-3 w-3" />
				</button>
				<button onclick={() => goToPage(totalPages)} disabled={page === totalPages} class="btn-ghost px-2 py-1 text-xs">
					<Icon icon="solar:alt-arrow-right-bold" class="h-3 w-3" />
				</button>
			</div>
		{/if}
	{/if}
</div>
