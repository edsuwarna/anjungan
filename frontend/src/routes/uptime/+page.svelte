<script>
	import { onMount, onDestroy } from 'svelte';
	import { api, getAuthToken } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';
	import { goto } from '$app/navigation';

	let monitors = $state([]);
	let loading = $state(true);
	let error = $state('');
	let summary = $state(null);

	// Pagination
	let page = $state(1);
	let limit = $state(20);
	let total = $state(0);
	let totalPages = $state(1);

	// Filters
	let searchQuery = $state('');
	let statusFilter = $state('');
	let sortField = $state('name');
	let sortOrder = $state('asc');

	// Modal
	let showAddModal = $state(false);
	let showEditModal = $state(false);
	let editingMonitor = $state(null);

	// Checking state
	let checking = $state({});
	let checkingAll = $state(false);

	let pauseLoading = $state({});

	// SSE real-time updates
	let eventSource = $state(null);
	let sseConnected = $state(false);
	let retryDelay = $state(1000);

	// Notification targets for add/edit modal
	let notificationTargets = $state([]);
	let notificationTargetsLoading = $state(false);

	// Test notification
	let testingNotif = $state(null);
	let testNotifResult = $state(null);

	// Computed filter state
	let hasFilters = $derived(searchQuery || statusFilter);

	// ─── Load ───
	async function loadData() {
		loading = true;
		try {
			const [listData, summaryData] = await Promise.all([
				api.uptime.list({ page, limit, search: searchQuery, status: statusFilter, sort: sortField, order: sortOrder }),
				api.uptime.summary(),
			]);
			monitors = listData || [];
			summary = summaryData;
			if (listData?._meta) {
				total = listData._meta.total;
				page = listData._meta.page;
				limit = listData._meta.per_page;
				totalPages = listData._meta.total_pages;
			}
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadSummary() {
		try {
			summary = await api.uptime.summary();
		} catch (_) {}
	}

	// ─── SSE Real-time Updates ───
	function connectSSE() {
		disconnectSSE();
		const token = getAuthToken();
		if (!token) return;
		const url = `/api/uptime/events?token=${encodeURIComponent(token)}`;
		const es = new EventSource(url);
		es.onopen = () => {
			sseConnected = true;
			retryDelay = 1000;
		};
		es.addEventListener('uptime_check', (e) => {
			try {
				handleSSEMessage(JSON.parse(e.data));
			} catch (_) {}
		});
		es.onmessage = (e) => {
			try {
				const data = JSON.parse(e.data);
				if (data.type === 'uptime_check') {
					handleSSEMessage(data);
				}
			} catch (_) {}
		};
		es.onerror = () => {
			sseConnected = false;
			es.close();
			const delay = Math.min(retryDelay, 30000);
			retryDelay = Math.min(retryDelay * 2, 30000);
			setTimeout(() => connectSSE(), delay);
		};
		eventSource = es;
	}

	function disconnectSSE() {
		if (eventSource) {
			eventSource.close();
			eventSource = null;
		}
		sseConnected = false;
	}

	function handleSSEMessage(data) {
		const idx = monitors.findIndex(m => m.id === data.monitor_id);
		if (idx !== -1) {
			const m = monitors[idx];
			if (data.status != null) m.status = data.status;
			if (data.response_time_ms != null) m.last_response_time_ms = data.response_time_ms;
			if (data.status_code != null) m.last_status_code = data.status_code;
			if (data.checked_at != null) m.last_check_at = data.checked_at;
			if (data.error != null) m.last_error = data.error;
			monitors = monitors;
		} else {
			loadData();
			loadSummary();
		}
	}

	function urlHostname(url) {
		try { return new URL(url).hostname; }
		catch { return url; }
	}

	onMount(() => {
		loadData();
		loadNotificationTargets();
		connectSSE();
	});

	onDestroy(() => {
		disconnectSSE();
	});

	// ─── Filters ───
	function setFilter(status) {
		statusFilter = status;
		page = 1;
		loadData();
	}

	function handleSearch() {
		page = 1;
		loadData();
	}

	function clearFilters() {
		searchQuery = '';
		statusFilter = '';
		page = 1;
		loadData();
	}

	// ─── Pagination ───
	function goToPage(p) {
		if (p < 1 || p > totalPages) return;
		page = p;
		loadData();
	}

	// ─── Check actions ───
	async function checkMonitor(id) {
		checking[id] = true;
		try {
			await api.uptime.checkNow(id);
			await loadData();
		} catch (e) {
			// Keep page state
		} finally {
			checking[id] = false;
		}
	}

	async function checkAll() {
		checkingAll = true;
		try {
			await api.uptime.checkAll();
			await loadData();
		} catch (e) {
			// Keep page state
		} finally {
			checkingAll = false;
		}
	}

	// ─── Pause/Resume ───
	async function togglePause(m) {
		pauseLoading[m.id] = true;
		try {
			if (m.status === 'paused') {
				await api.uptime.resume(m.id);
			} else {
				await api.uptime.pause(m.id);
			}
			await loadData();
			await loadSummary();
		} catch (e) {
			alert('Failed: ' + e.message);
		} finally {
			pauseLoading[m.id] = false;
		}
	}

	// ─── Delete ───
	async function deleteMonitor(id) {
		if (!confirm('Delete this uptime monitor?')) return;
		try {
			await api.uptime.delete(id);
			await loadData();
			await loadSummary();
		} catch (e) {
			alert('Failed to delete: ' + e.message);
		}
	}

	// ─── Notification Targets ───
	async function loadNotificationTargets() {
		notificationTargetsLoading = true;
		try {
			notificationTargets = await api.notificationTargets.list() || [];
		} catch (_) {
			notificationTargets = [];
		} finally {
			notificationTargetsLoading = false;
		}
	}

	async function handleTestNotif(id) {
		testingNotif = id;
		testNotifResult = null;
		try {
			const result = await api.notificationTargets.test(id);
			testNotifResult = { id, ...result };
		} catch (e) {
			testNotifResult = { id, success: false, error: e.message || 'Test request failed' };
		} finally {
			testingNotif = false;
		}
	}

	// ─── Add/Edit ───
	function openAddModal() {
		editingMonitor = null;
		showAddModal = true;
	}

	function openEditModal(m) {
		editingMonitor = m;
		showAddModal = true;
	}

	async function handleSave(e) {
		e.preventDefault();
		const fd = new FormData(e.target);
		const notifIds = Array.from(fd.getAll('notification_target_ids'));
		const data = {
			name: fd.get('name'),
			url: fd.get('url'),
			check_type: fd.get('check_type') || 'http',
			interval_seconds: parseInt(fd.get('interval_seconds')) || 300,
			timeout_seconds: parseInt(fd.get('timeout_seconds')) || 30,
			expected_status_min: parseInt(fd.get('expected_status_min')) || 200,
			expected_status_max: parseInt(fd.get('expected_status_max')) || 399,
			expected_body: fd.get('expected_body') || '',
			notification_target_ids: notifIds,
		};
		try {
			if (editingMonitor) {
				await api.uptime.update(editingMonitor.id, data);
			} else {
				await api.uptime.create(data);
			}
			showAddModal = false;
			editingMonitor = null;
			await loadData();
			await loadSummary();
		} catch (e) {
			alert('Failed to save: ' + e.message);
		}
	}

	// ─── Helpers ───
	const statusConfig = {
		up: { label: 'UP', color: '#22c55e' },
		down: { label: 'DOWN', color: '#ef4444' },
		paused: { label: 'PAUSED', color: '#94a3b8' },
		pending: { label: 'PENDING', color: '#f59e0b' },
		maintenance: { label: 'MAINTENANCE', color: '#8b5cf6' },
	};

	function getStatusConfig(s) {
		return statusConfig[s] || statusConfig.pending;
	}

	function responseTimeColor(ms) {
		if (ms == null) return 'var(--color-text-muted)';
		if (ms < 200) return '#22c55e';
		if (ms < 1000) return '#f59e0b';
		return '#ef4444';
	}

	function formatTimeAgo(iso) {
		if (!iso) return 'Never';
		const diff = Date.now() - new Date(iso).getTime();
		const sec = Math.floor(diff / 1000);
		if (sec < 60) return `${sec}s ago`;
		const min = Math.floor(sec / 60);
		if (min < 60) return `${min}m ago`;
		const hrs = Math.floor(min / 60);
		if (hrs < 24) return `${hrs}h ago`;
		const days = Math.floor(hrs / 24);
		return `${days}d ago`;
	}

	function intervalLabel(sec) {
		if (!sec) return 'Every 5m';
		if (sec < 60) return `Every ${sec}s`;
		const m = sec / 60;
		if (m < 60) return `Every ${m}m`;
		const h = m / 60;
		return `Every ${h}h`;
	}

	const filterChips = [
		{ value: '', label: 'All' },
		{ value: 'up', label: 'Up' },
		{ value: 'down', label: 'Down' },
		{ value: 'maintenance', label: 'Maintenance' },
		{ value: 'paused', label: 'Paused' },
		{ value: 'pending', label: 'Pending' },
	];

	// Response time response helper
	function getCheckTypeIcon(type) {
		if (type === 'tcp') return 'solar:plug-circle-bold';
		return 'solar:global-bold';
	}
</script>

<div class="page-container">
	<!-- Page Header -->
	<div class="mb-6 flex flex-wrap items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold" style="color: var(--color-text);">
				Uptime Monitoring
				{#if sseConnected}
					<span class="live-badge">
						<span class="live-dot"></span>
						Live
					</span>
				{/if}
			</h1>
			<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
				Service health & response time monitoring
			</p>
		</div>
		<div class="flex items-center gap-3">
			<button
				class="btn-secondary"
				onclick={checkAll}
				disabled={checkingAll}
			>
				<Icon icon={checkingAll ? 'svg-spinners:180-ring' : 'solar:refresh-bold'} class="h-4 w-4" />
				{checkingAll ? 'Checking...' : 'Check All'}
			</button>
			<button
				class="btn-primary"
				onclick={openAddModal}
			>
				<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
				Add Monitor
			</button>
		</div>
	</div>

	<!-- KPI Cards -->
	<div class="mb-6 grid grid-cols-2 gap-4 md:grid-cols-5">
		<div class="stat-card">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text-secondary);">Total</p>
					<p class="mt-1 text-2xl font-bold" style="color: var(--color-text);">{summary?.total ?? '-'}</p>
				</div>
				<div class="flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: var(--color-primary-subtle);">
					<Icon icon="solar:chart-2-bold" class="h-5 w-5" style="color: var(--color-primary);" />
				</div>
			</div>
		</div>
		<div class="stat-card">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text-secondary);">Up</p>
					<p class="mt-1 text-2xl font-bold" style="color: #22c55e;">{summary?.up ?? 0}</p>
				</div>
				<div class="flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: #22c55e15;">
					<Icon icon="solar:check-circle-bold" class="h-5 w-5" style="color: #22c55e;" />
				</div>
			</div>
		</div>
		<div class="stat-card">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text-secondary);">Down</p>
					<p class="mt-1 text-2xl font-bold" style="color: #ef4444;">{summary?.down ?? 0}</p>
				</div>
				<div class="flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: #ef444415;">
					<Icon icon="solar:danger-circle-bold" class="h-5 w-5" style="color: #ef4444;" />
				</div>
			</div>
		</div>
		<div class="stat-card">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text-secondary);">Paused</p>
					<p class="mt-1 text-2xl font-bold" style="color: #94a3b8;">{summary?.paused ?? 0}</p>
				</div>
				<div class="flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: #94a3b815;">
					<Icon icon="solar:pause-circle-bold" class="h-5 w-5" style="color: #94a3b8;" />
				</div>
			</div>
		</div>
		<div class="stat-card">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text-secondary);">Avg Response</p>
					<p class="mt-1 text-2xl font-bold" style="color: var(--color-primary);">
						{summary?.avg_response_time ? `${Math.round(summary.avg_response_time)}ms` : '-'}
					</p>
				</div>
				<div class="flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: var(--color-primary-subtle);">
					<Icon icon="solar:clock-circle-bold" class="h-5 w-5" style="color: var(--color-primary);" />
				</div>
			</div>
		</div>
	</div>

	<!-- Filters -->
	<div class="mb-6 flex flex-wrap items-center gap-3">
		{#each filterChips as chip}
			<button
				class="filter-chip"
				class:active={statusFilter === chip.value}
				onclick={() => setFilter(chip.value)}
			>
				{chip.label}
			</button>
		{/each}

		<div class="ml-auto flex items-center gap-2">
			<div class="relative">
				<Icon icon="solar:magnifer-bold" class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2" style="color: var(--color-text-muted);" />
				<input
					type="text"
					placeholder="Search monitors..."
					bind:value={searchQuery}
					onkeydown={(e) => e.key === 'Enter' && handleSearch()}
					class="input pl-9"
				/>
			</div>
			{#if hasFilters}
				<button class="btn-ghost text-sm" onclick={clearFilters}>
					<Icon icon="solar:close-circle-bold" class="h-4 w-4" />
					Clear
				</button>
			{/if}
		</div>
	</div>

	<!-- Loading -->
	{#if loading}
		<div class="flex items-center justify-center py-16">
			<Icon icon="svg-spinners:180-ring" class="h-8 w-8" style="color: var(--color-primary);" />
		</div>
	{:else if error}
		<div class="card p-6 text-center">
			<p style="color: var(--color-error);">{error}</p>
			<button class="btn-secondary mt-3" onclick={loadData}>Retry</button>
		</div>
	{:else if monitors.length === 0}
		<div class="card p-12 text-center">
			<Icon icon="solar:chart-2-bold" class="mx-auto h-12 w-12" style="color: var(--color-text-muted);" />
			<p class="mt-4 text-lg font-medium" style="color: var(--color-text);">No uptime monitors</p>
			<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
				{hasFilters ? 'No monitors match your filters.' : 'Add your first monitor to start tracking service health.'}
			</p>
			{#if hasFilters}
				<button class="btn-secondary mt-4" onclick={clearFilters}>Clear Filters</button>
			{:else}
				<button class="btn-primary mt-4" onclick={openAddModal}>Add Monitor</button>
			{/if}
		</div>
	{:else}
		<!-- Monitor Cards -->
		<div class="space-y-2">
			{#each monitors as m (m.id)}
				{@const cfg = getStatusConfig(m.status)}
				<div
					class="monitor-card"
					onclick={() => goto(`/uptime/${m.id}`)}
					role="button"
					tabindex="0"
					onkeydown={(e) => e.key === 'Enter' && goto(`/uptime/${m.id}`)}
				>
					<!-- Status dot -->
					<span
						class="status-dot"
						class:status-up={m.status === 'up'}
						class:status-down={m.status === 'down'}
						class:status-paused={m.status === 'paused'}
						class:status-pending={m.status === 'pending'}
						style="background-color: {cfg.color};"
					></span>

					<!-- Info -->
					<div class="monitor-info">
						<div class="monitor-name">
							{m.name}
							<span
								class="status-badge"
								class:up={m.status === 'up'}
								class:down={m.status === 'down'}
								class:paused={m.status === 'paused'}
								class:pending={m.status === 'pending'}
							>
								{cfg.label}
							</span>
						</div>
						<div class="monitor-url">{m.url}</div>
						<div class="monitor-meta">
							{m.check_type?.toUpperCase() || 'HTTP'} &middot; {intervalLabel(m.interval_seconds)} &middot; Checked {formatTimeAgo(m.last_check_at)}
						</div>
						<div class="monitor-stats">
							{#if m.uptime_percent_7d != null}
								<span class="monitor-stat">
									📈 {m.uptime_percent_7d.toFixed(1)}% uptime (7d)
								</span>
							{/if}
							{#if m.notification_target_ids?.length > 0}
								{#each m.notification_target_ids.slice(0, 3) as ntId}
									{@const nt = notificationTargets.find(t => t.id === ntId)}
									<span class="notif-tag">
										<Icon icon="solar:bell-bold" class="h-3 w-3" />
										{nt?.name || ntId?.slice(0, 8) || 'Notif'}
									</span>
								{/each}
								{#if m.notification_target_ids.length > 3}
									<span class="notif-tag">+{m.notification_target_ids.length - 3}</span>
								{/if}
							{/if}
						</div>
					</div>

					<!-- Response time -->
					<div class="monitor-response">
						<div
							class="response-time"
							class:fast={m.last_response_time_ms != null && m.last_response_time_ms < 200}
							class:slow={m.last_response_time_ms != null && m.last_response_time_ms >= 200 && m.last_response_time_ms < 1000}
							class:timeout={m.last_response_time_ms != null && m.last_response_time_ms >= 1000}
							style="color: {m.status === 'down' ? '#ef4444' : m.status === 'paused' ? 'var(--color-text-muted)' : responseTimeColor(m.last_response_time_ms)};"
						>
							{m.status === 'paused' ? '—' : m.status === 'down' ? '—' : m.last_response_time_ms != null ? `${m.last_response_time_ms}ms` : '—'}
						</div>
						<div class="response-label" style="color: {m.status === 'down' ? '#ef4444' : m.status === 'paused' ? 'var(--color-text-muted)' : 'var(--color-text-muted)'};">
							{m.status === 'down' ? 'DOWN' : m.status === 'paused' ? 'paused' : m.last_status_code ? `${m.last_status_code} ✓` : '—'}
						</div>
					</div>

					<!-- Actions -->
					<div class="monitor-actions">
						<button
							class="btn-icon"
							onclick={(e) => { e.stopPropagation(); checkMonitor(m.id); }}
							disabled={checking[m.id]}
							title="Check Now"
						>
							<Icon icon={checking[m.id] ? 'svg-spinners:180-ring' : 'solar:refresh-bold'} class="h-4 w-4" />
						</button>
						<button
							class="btn-icon"
							onclick={(e) => { e.stopPropagation(); togglePause(m); }}
							disabled={pauseLoading[m.id]}
							title={m.status === 'paused' ? 'Resume' : 'Pause'}
						>
							<Icon icon={pauseLoading[m.id] ? 'svg-spinners:180-ring' : m.status === 'paused' ? 'solar:play-bold' : 'solar:pause-bold'} class="h-4 w-4" />
						</button>
						<button
							class="btn-icon"
							onclick={(e) => { e.stopPropagation(); openEditModal(m); }}
							title="Edit"
						>
							<Icon icon="solar:pen-bold" class="h-4 w-4" />
						</button>
						<button
							class="btn-icon"
							style="color: #ef4444;"
							onclick={(e) => { e.stopPropagation(); deleteMonitor(m.id); }}
							title="Delete"
						>
							<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
						</button>
					</div>
				</div>
			{/each}
		</div>

		<!-- Pagination -->
		{#if totalPages > 1}
			<div class="mt-6 flex items-center justify-center gap-2">
				<button
					class="btn-secondary px-3 py-1.5 text-sm"
					disabled={page <= 1}
					onclick={() => goToPage(page - 1)}
				>
					Previous
				</button>
				{#each Array.from({ length: totalPages }, (_, i) => i + 1) as p}
					<button
						class="btn-page"
						class:active={p === page}
						onclick={() => goToPage(p)}
					>
						{p}
					</button>
				{/each}
				<button
					class="btn-secondary px-3 py-1.5 text-sm"
					disabled={page >= totalPages}
					onclick={() => goToPage(page + 1)}
				>
					Next
				</button>
			</div>
		{/if}
	{/if}
</div>

<!-- Add/Edit Modal -->
{#if showAddModal}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div class="modal-overlay" onclick={() => { showAddModal = false; editingMonitor = null; }}>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div class="modal-panel" onclick={(e) => e.stopPropagation()}>
			<div class="flex items-center justify-between border-b pb-4" style="border-color: var(--color-border);">
				<div>
					<h2 class="text-lg font-bold" style="color: var(--color-text);">
						{editingMonitor ? 'Edit' : 'Add'} Uptime Monitor
					</h2>
					<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
						{editingMonitor ? 'Update monitor configuration' : 'Add a new endpoint to monitor'}
					</p>
				</div>
				<button class="btn-icon" onclick={() => { showAddModal = false; editingMonitor = null; }}>
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>

			<form onsubmit={handleSave} class="mt-5">
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Monitor Name *</label>
					<input type="text" name="name" required placeholder="e.g. API Gateway Health" value={editingMonitor?.name || ''} class="input w-full" />
				</div>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">URL / Host:Port *</label>
					<input type="text" name="url" required placeholder="https://example.com/health or tcp://host:port" value={editingMonitor?.url || ''} class="input w-full" />
					<p class="mt-1 text-xs" style="color: var(--color-text-muted);">HTTP(S) URLs or tcp://host:port for TCP checks</p>
				</div>
				<div class="mb-4 grid grid-cols-2 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Check Type</label>
						<select name="check_type" class="input w-full">
							<option value="http" selected={!editingMonitor || editingMonitor.check_type === 'http'}>HTTP(S)</option>
							<option value="tcp" selected={editingMonitor?.check_type === 'tcp'}>TCP</option>
						</select>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Interval</label>
						<select name="interval_seconds" class="input w-full">
							<option value="30" selected={editingMonitor?.interval_seconds === 30}>30 seconds</option>
							<option value="60" selected={editingMonitor?.interval_seconds === 60}>Every 1 minute</option>
							<option value="300" selected={!editingMonitor || editingMonitor?.interval_seconds === 300}>Every 5 minutes</option>
							<option value="900" selected={editingMonitor?.interval_seconds === 900}>Every 15 minutes</option>
							<option value="1800" selected={editingMonitor?.interval_seconds === 1800}>Every 30 minutes</option>
							<option value="3600" selected={editingMonitor?.interval_seconds === 3600}>Every 1 hour</option>
						</select>
					</div>
				</div>
				<div class="mb-4 grid grid-cols-2 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Timeout</label>
						<select name="timeout_seconds" class="input w-full">
							<option value="5" selected={editingMonitor?.timeout_seconds === 5}>5 seconds</option>
							<option value="10" selected={editingMonitor?.timeout_seconds === 10}>10 seconds</option>
							<option value="30" selected={!editingMonitor || editingMonitor?.timeout_seconds === 30}>30 seconds</option>
							<option value="60" selected={editingMonitor?.timeout_seconds === 60}>60 seconds</option>
						</select>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Expected Status</label>
						<div class="flex items-center gap-2">
							<input type="number" name="expected_status_min" value={editingMonitor?.expected_status_min || 200} min="100" max="599" class="input w-20" placeholder="Min" />
							<span class="text-xs" style="color: var(--color-text-muted);">to</span>
							<input type="number" name="expected_status_max" value={editingMonitor?.expected_status_max || 399} min="100" max="599" class="input w-20" placeholder="Max" />
						</div>
					</div>
				</div>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Expected Body (optional)</label>
					<input type="text" name="expected_body" placeholder="e.g. OK or regex pattern" value={editingMonitor?.expected_body || ''} class="input w-full" />
					<p class="mt-1 text-xs" style="color: var(--color-text-muted);">If set, check fails when body doesn't match this pattern</p>
				</div>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Notify Via</label>
					{#if notificationTargetsLoading}
						<p class="text-xs" style="color: var(--color-text-muted);">Loading notification targets...</p>
					{:else if notificationTargets.length === 0}
						<p class="text-xs" style="color: var(--color-text-muted);">
							No notification targets configured. Create one on the Notifications page.
						</p>
					{:else}
						<div class="space-y-1 max-h-40 overflow-y-auto">
							{#each notificationTargets as nt}
								<div class="flex items-center gap-2">
									<label class="flex flex-1 cursor-pointer items-center gap-3 rounded-lg p-2 text-sm min-w-0">
										<input
											type="checkbox"
											name="notification_target_ids"
											value={nt.id}
											checked={editingMonitor?.notification_target_ids?.includes(nt.id)}
											class="h-4 w-4 shrink-0 rounded border-gray-300"
										/>
										<div class="min-w-0 flex-1">
											<p class="truncate text-sm" style="color: var(--color-text);">{nt.name}</p>
											<p class="truncate text-xs" style="color: var(--color-text-muted);" title={nt.url}>{nt.platform} &middot; {urlHostname(nt.url)}</p>
										</div>
									</label>
									<button
										class="btn-icon-sm shrink-0"
										onclick={() => handleTestNotif(nt.id)}
										disabled={testingNotif === nt.id}
										title="Test notification"
									>
										<Icon icon={testingNotif === nt.id ? 'svg-spinners:180-ring' : 'solar:play-circle-bold'} class="h-3.5 w-3.5" />
									</button>
								</div>
								{#if testNotifResult?.id === nt.id}
									<div class="flex items-center gap-2 px-2 pb-1 text-xs" class:text-emerald-500={testNotifResult.success} style="color: {testNotifResult.success ? 'var(--color-primary)' : '#ef4444'};">
										<Icon icon={testNotifResult.success ? 'solar:check-circle-bold' : 'solar:danger-circle-bold'} class="h-3.5 w-3.5 shrink-0" />
										{testNotifResult.success ? 'Sent! Check your channel' : testNotifResult.error || 'Failed'}
										<button class="ml-auto btn-icon-sm" onclick={() => testNotifResult = null}>
											<Icon icon="solar:close-circle-bold" class="h-3.5 w-3.5" />
										</button>
									</div>
								{/if}
							{/each}
						</div>
					{/if}
				</div>

				<div class="flex items-center justify-end gap-3 pt-4">
					<button type="button" class="btn-secondary" onclick={() => { showAddModal = false; editingMonitor = null; }}>Cancel</button>
					<button type="submit" class="btn-primary">
						{editingMonitor ? 'Update' : 'Add Monitor'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<style>
	.page-container {
		max-width: 1280px;
		margin: 0 auto;
		padding: 1.5rem;
	}
	.card {
		border-radius: 12px;
		padding: 1.25rem;
		background: var(--color-card);
		border: 1px solid var(--color-border);
	}
	.stat-card {
		border-radius: 12px;
		padding: 1rem;
		background: var(--color-card);
		border: 1px solid var(--color-border);
	}
	.btn-primary {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		border-radius: 8px;
		padding: 0.5rem 1rem;
		font-size: 0.875rem;
		font-weight: 600;
		color: #fff;
		background: var(--color-primary);
		border: none;
		cursor: pointer;
		transition: opacity 0.15s;
	}
	.btn-primary:hover { opacity: 0.9; }
	.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
	.btn-secondary {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		border-radius: 8px;
		padding: 0.5rem 1rem;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--color-text);
		background: var(--color-card);
		border: 1px solid var(--color-border);
		cursor: pointer;
		transition: background 0.15s;
	}
	.btn-secondary:hover { background: var(--color-hover); }
	.btn-secondary:disabled { opacity: 0.5; cursor: not-allowed; }
	.btn-ghost {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		border: none;
		background: none;
		cursor: pointer;
		color: var(--color-text-secondary);
		padding: 0.375rem 0.5rem;
		border-radius: 6px;
	}
	.btn-ghost:hover { background: var(--color-hover); }
	.btn-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 2rem;
		height: 2rem;
		border-radius: 6px;
		border: none;
		background: transparent;
		cursor: pointer;
		color: var(--color-text-secondary);
		transition: background 0.15s;
	}
	.btn-icon:hover { background: var(--color-hover); }
	.btn-icon:disabled { opacity: 0.4; cursor: not-allowed; }
	.btn-page {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 2rem;
		height: 2rem;
		border-radius: 6px;
		border: 1px solid var(--color-border);
		background: var(--color-card);
		color: var(--color-text);
		font-size: 0.875rem;
		cursor: pointer;
		transition: all 0.15s;
	}
	.btn-page.active {
		background: var(--color-primary);
		color: #fff;
		border-color: var(--color-primary);
	}
	.btn-page:hover:not(.active) { background: var(--color-hover); }
	.input {
		border-radius: 8px;
		padding: 0.5rem 0.75rem;
		font-size: 0.875rem;
		background: var(--color-bg);
		border: 1px solid var(--color-border);
		color: var(--color-text);
		outline: none;
		transition: border-color 0.15s;
	}
	.input:focus { border-color: var(--color-primary); }
	.filter-chip {
		display: inline-flex;
		align-items: center;
		border-radius: 9999px;
		padding: 0.375rem 0.875rem;
		font-size: 0.8125rem;
		font-weight: 500;
		border: 1px solid var(--color-border);
		background: var(--color-card);
		color: var(--color-text-secondary);
		cursor: pointer;
		transition: all 0.15s;
	}
	.filter-chip.active {
		background: var(--color-primary);
		color: #fff;
		border-color: var(--color-primary);
	}
	.filter-chip:hover:not(.active) { background: var(--color-hover); }
	.live-badge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		margin-left: 0.75rem;
		padding: 2px 10px;
		border-radius: 9999px;
		font-size: 0.6875rem;
		font-weight: 600;
		color: #22c55e;
		background: rgba(34,197,94,0.1);
		border: 1px solid rgba(34,197,94,0.2);
		vertical-align: middle;
	}
	.live-dot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: #22c55e;
		animation: live-pulse 1.5s infinite;
	}
	@keyframes live-pulse {
		0%, 100% { opacity: 1; box-shadow: 0 0 4px rgba(34,197,94,0.6); }
		50% { opacity: 0.5; box-shadow: 0 0 8px rgba(34,197,94,0.3); }
	}

	.monitor-card {
		display: flex;
		align-items: center;
		border-radius: 12px;
		padding: 1rem 1.25rem;
		background: var(--color-card);
		border: 1px solid var(--color-border);
		cursor: pointer;
		transition: all 0.15s;
		gap: 0.75rem;
	}
	.monitor-card:hover {
		border-color: var(--color-primary);
		box-shadow: 0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -1px rgba(0,0,0,0.04);
	}
	.status-dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		flex-shrink: 0;
	}
	.status-up { box-shadow: 0 0 6px rgba(34,197,94,0.4); }
	.status-down { box-shadow: 0 0 6px rgba(239,68,68,0.4); }
	.status-pending { animation: pulse 1.5s infinite; }
	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.4; }
	}
	.monitor-info { flex: 1; min-width: 0; }
	.monitor-name { font-weight: 600; font-size: 0.875rem; display: flex; align-items: center; gap: 0.5rem; }
	.monitor-url { font-size: 0.75rem; color: var(--color-text-muted); margin-top: 1px; font-family: monospace; }
	.monitor-meta { font-size: 0.75rem; color: var(--color-text-secondary); margin-top: 2px; }
	.monitor-stats { display: flex; gap: 0.5rem; margin-top: 4px; flex-wrap: wrap; align-items: center; }
	.monitor-stat { display: inline-flex; align-items: center; gap: 3px; font-size: 0.75rem; color: var(--color-text-secondary); }
	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		padding: 1px 8px;
		border-radius: 10px;
		font-size: 0.6875rem;
		font-weight: 600;
	}
	.status-badge.up { background: #dcfce7; color: #16a34a; }
	.status-badge.down { background: #fee2e2; color: #dc2626; }
	.status-badge.paused { background: #f1f5f9; color: #64748b; }
	.status-badge.pending { background: #fef3c7; color: #d97706; }
	.status-badge.maintenance { background: rgba(139,92,246,0.15); color: #8b5cf6; }
	.monitor-response { text-align: right; flex-shrink: 0; margin-left: 0.5rem; }
	.response-time { font-size: 1.125rem; font-weight: 700; }
	.response-time.fast { color: #22c55e; }
	.response-time.slow { color: #f59e0b; }
	.response-time.timeout { color: #ef4444; }
	.response-label { font-size: 0.6875rem; color: var(--color-text-muted); }
	.monitor-actions { display: flex; gap: 2px; margin-left: 0.5rem; }
	.notif-tag {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		padding: 1px 6px;
		border-radius: 10px;
		font-size: 0.6875rem;
		background: #e0f2fe;
		color: #0369a1;
	}
	.modal-overlay {
		position: fixed;
		inset: 0;
		z-index: 50;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(0,0,0,0.5);
		padding: 1rem;
	}
	.modal-panel {
		background: var(--color-card);
		border-radius: 16px;
		width: 100%;
		max-width: 560px;
		max-height: 90vh;
		overflow-y: auto;
		padding: 1.5rem;
		border: 1px solid var(--color-border);
		box-shadow: 0 20px 60px rgba(0,0,0,0.2);
	}
	:global(body.dark) .card { background: #1a1d23; border-color: rgba(148,163,184,0.08); }
	:global(body.dark) .stat-card { background: #1a1d23; border-color: rgba(148,163,184,0.08); }
	:global(body.dark) .monitor-card { background: #1a1d23; border-color: rgba(148,163,184,0.08); }
	:global(body.dark) .status-badge.up { background: rgba(34,197,94,0.15); color: #22c55e; }
	:global(body.dark) .status-badge.down { background: rgba(239,68,68,0.15); color: #ef4444; }
	:global(body.dark) .status-badge.paused { background: rgba(148,163,184,0.15); color: #94a3b8; }
	:global(body.dark) .status-badge.pending { background: rgba(245,158,11,0.15); color: #f59e0b; }
	:global(body.dark) .status-badge.maintenance { background: rgba(139,92,246,0.15); color: #a78bfa; }
	:global(body.dark) .notif-tag { background: rgba(14,165,233,0.15); color: #38bdf8; }
</style>