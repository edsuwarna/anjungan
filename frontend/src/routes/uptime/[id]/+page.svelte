<script>
	import { onMount, onDestroy } from 'svelte';
	import { api, getAuthToken } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import MetricsChart from '$lib/components/charts/MetricsChart.svelte';

	let monitor = $state(null);
	let loading = $state(true);
	let error = $state('');
	let checking = $state(false);
	let showDeleteConfirm = $state(false);
	let showEditModal = $state(false);
	let editingMonitor = $state(null);

	// History
	let historyEntries = $state([]);
	let historyTotal = $state(0);
	let historyLoading = $state(false);
	let historyLimit = $state(100);
	let historyOffset = $state(0);

	// Trend
	let trendData = $state(null);
	let trendLoading = $state(false);
	let trendPeriod = $state('24h');
	let customFrom = $state('');
	let customTo = $state('');
	let showCustomRange = $state(false);

	// Notification targets
	let notificationTargets = $state([]);
	let notificationTargetsLoading = $state(false);

	// Test notification
	let testingTarget = $state(null);
	let testTargetResult = $state(null);

	// SSE real-time updates
	let eventSource = $state(null);
	let sseConnected = $state(false);
	let retryDelay = $state(1000);

	// Maintenance windows
	let maintenanceWindows = $state([]);
	let maintenanceLoading = $state(false);
	let showMaintenanceModal = $state(false);
	let maintenanceSaving = $state(false);

	// Incidents
	let incidents = $state([]);
	let incidentsTotal = $state(0);
	let incidentsLoading = $state(false);

	// Derived response time stats from monitor.response_time_stats
	let responseTimeStats = $derived(monitor?.response_time_stats || null);

	const id = $derived($page.params.id);

	onMount(() => {
		loadMonitor();
		loadNotificationTargets();
		loadIncidents();
		connectSSE();
	});

	onDestroy(() => {
		disconnectSSE();
	});

	function urlHostname(url) {
		try { return new URL(url).hostname; }
		catch { return url; }
	}

	// ─── Test notification ────────────────────────────────────────────────
	async function handleTestTarget(targetId) {
		testingTarget = targetId;
		testTargetResult = null;
		try {
			const result = await api.notificationTargets.test(targetId);
			testTargetResult = { id: targetId, ...result };
		} catch (e) {
			testTargetResult = { id: targetId, success: false, error: e.message || 'Test request failed' };
		} finally {
			testingTarget = false;
		}
	}

	async function handleTestMonitorNotification() {
		testingTarget = 'monitor';
		testTargetResult = null;
		try {
			const result = await api.uptime.testNotification(id);
			testTargetResult = { id: 'monitor', ...result };
		} catch (e) {
			testTargetResult = { id: 'monitor', success: false, error: e.message || 'Test request failed' };
		} finally {
			testingTarget = false;
		}
	}

	async function loadMonitor() {
		loading = true;
		try {
			monitor = await api.uptime.get(id);
			historyOffset = 0;
			loadHistory();
			loadTrend();
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadHistory() {
		historyLoading = true;
		try {
			const result = await api.uptime.history(id, { limit: historyLimit, offset: historyOffset });
			const newEntries = Array.isArray(result) ? result : (result?.entries || []);
			if (historyOffset > 0) {
				historyEntries = [...historyEntries, ...newEntries];
			} else {
				historyEntries = newEntries;
			}
			historyTotal = result?.total || (Array.isArray(result) ? result.length : 0);
		} catch (_) {
			if (historyOffset === 0) historyEntries = [];
		} finally {
			historyLoading = false;
		}
	}

	async function loadMore() {
		historyOffset += historyLimit;
		await loadHistory();
	}

	async function loadTrend() {
		trendLoading = true;
		try {
			trendData = await api.uptime.trend(id, { period: trendPeriod });
			if (Array.isArray(trendData)) {
				trendData = { points: trendData };
			} else if (trendData?.entries) {
				trendData = { points: trendData.entries };
			}
		} catch (_) {
			trendData = null;
		} finally {
			trendLoading = false;
		}
	}

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

	async function loadIncidents() {
		incidentsLoading = true;
		try {
			const result = await api.uptime.incidents(id, { limit: 10 });
			incidents = result?.incidents || [];
			incidentsTotal = result?.total || 0;
		} catch (_) {
			incidents = [];
		} finally {
			incidentsLoading = false;
		}
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
				const data = JSON.parse(e.data);
				handleSSEMessage(data);
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
		if (monitor && data.monitor_id === id) {
			if (data.status != null) monitor.status = data.status;
			if (data.response_time_ms != null) monitor.last_response_time_ms = data.response_time_ms;
			if (data.status_code != null) monitor.last_status_code = data.status_code;
			if (data.checked_at != null) monitor.last_check_at = data.checked_at;
			if (data.error != null) monitor.last_error = data.error;
			// Trigger refresh of history and trend
			historyOffset = 0;
			loadHistory();
			loadTrend();
		}
	}

	async function checkNow() {
		checking = true;
		try {
			await api.uptime.checkNow(id);
			await loadMonitor();
		} catch (e) {
			alert('Check failed: ' + e.message);
		} finally {
			checking = false;
		}
	}

	async function deleteMonitor() {
		try {
			await api.uptime.delete(id);
			goto('/uptime');
		} catch (e) {
			alert('Delete failed: ' + e.message);
		}
	}

	async function togglePause() {
		try {
			if (monitor.status === 'paused') {
				await api.uptime.resume(id);
			} else {
				await api.uptime.pause(id);
			}
			await loadMonitor();
		} catch (e) {
			alert('Failed: ' + e.message);
		}
	}

	async function handleEdit(e) {
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
			await api.uptime.update(id, data);
			showEditModal = false;
			await loadMonitor();
		} catch (e) {
			alert('Update failed: ' + e.message);
		}
	}

	async function changeTrendPeriod(period) {
		if (period === 'custom') {
			showCustomRange = true;
			return;
		}
		showCustomRange = false;
		trendPeriod = period;
		await loadTrend();
	}

	async function applyCustomRange() {
		if (!customFrom) return;
		showCustomRange = false;
		trendPeriod = 'custom';
		trendLoading = true;
		try {
			const params = { from: customFrom };
			if (customTo) params.to = customTo;
			trendData = await api.uptime.trend(id, params);
			if (Array.isArray(trendData)) {
				trendData = { points: trendData };
			} else if (trendData?.entries) {
				trendData = { points: trendData.entries };
			}
		} catch (_) {
			trendData = null;
		} finally {
			trendLoading = false;
		}
	}

	// ─── Helpers ──────────────────────────────────────────────────────────
	const statusConfig = {
		up: { label: 'UP', color: '#22c55e' },
		down: { label: 'DOWN', color: '#ef4444' },
		paused: { label: 'PAUSED', color: '#94a3b8' },
		pending: { label: 'PENDING', color: '#f59e0b' },
		maintenance: { label: 'MAINTENANCE', color: '#8b5cf6' },
	};

	function getConfig(s) {
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

	function formatDate(iso) {
		if (!iso) return '-';
		const d = new Date(iso);
		return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' });
	}

	function formatDateShort(iso) {
		if (!iso) return '-';
		const d = new Date(iso);
		return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short' });
	}

	function formatDuration(sec) {
		if (sec == null || sec === 0) return '<1s';
		if (sec < 60) return `${sec}s`;
		const min = Math.floor(sec / 60);
		if (min < 60) return `${min}m ${sec % 60}s`;
		const hrs = Math.floor(min / 60);
		if (hrs < 24) return `${hrs}h ${min % 60}m`;
		const days = Math.floor(hrs / 24);
		return `${days}d ${hrs % 24}h`;
	}

	function getNotificationTargetName(id) {
		return notificationTargets.find(t => t.id === id)?.name || id?.slice(0, 8) || 'Unknown';
	}

	// ─── Maintenance Windows ──────────────────────────────────────────
	async function loadMaintenanceWindows() {
		maintenanceLoading = true;
		try {
			maintenanceWindows = await api.uptime.maintenance.list(id) || [];
		} catch (_) {
			maintenanceWindows = [];
		} finally {
			maintenanceLoading = false;
		}
	}

	async function handleCreateMaintenance(e) {
		e.preventDefault();
		maintenanceSaving = true;
		try {
			const fd = new FormData(e.target);
			const startsAt = new Date(fd.get('starts_at') + 'T' + (fd.get('starts_time') || '00:00')).toISOString();
			const endsAt = new Date(fd.get('ends_at') + 'T' + (fd.get('ends_time') || '23:59')).toISOString();
			await api.uptime.maintenance.create(id, {
				description: fd.get('description'),
				starts_at: startsAt,
				ends_at: endsAt,
			});
			showMaintenanceModal = false;
			await loadMaintenanceWindows();
		} catch (e) {
			alert('Failed to create maintenance window: ' + e.message);
		} finally {
			maintenanceSaving = false;
		}
	}

	async function handleDeleteMaintenance(mwId) {
		if (!confirm('Delete this maintenance window?')) return;
		try {
			await api.uptime.maintenance.delete(id, mwId);
			await loadMaintenanceWindows();
		} catch (e) {
			alert('Failed to delete maintenance window: ' + e.message);
		}
	}
	// ─── Chart (uPlot) ───────────────────────────────────────────────────────
	let chartTimestamps = $derived.by(() => {
		const points = trendData?.points || trendData || [];
		if (!Array.isArray(points) || points.length < 2) return [];
		return points.map(p => {
			const t = p.checked_at || p.timestamp;
			if (!t) return 0;
			return new Date(t).getTime() / 1000;
		});
	});

	let chartValues = $derived.by(() => {
		const points = trendData?.points || trendData || [];
		if (!Array.isArray(points) || points.length < 2) return [];
		return points.map(p => p.response_time_ms || p.value || 0);
	});

	let chartStatus = $derived.by(() => {
		const points = trendData?.points || trendData || [];
		if (!Array.isArray(points) || points.length < 2) return [];
		return points.map(p => p.status || (p.down ? 'down' : 'up'));
	});

	// Percentile-based Y-axis range (kept from prev SVG chart)
	let yAxisRange = $derived.by(() => {
		const points = trendData?.points || trendData || [];
		if (!Array.isArray(points) || points.length < 2) return null;
		const values = points.map(p => p.response_time_ms || p.value || 0);
		const okValues = values.filter(v => v > 0);
		if (okValues.length === 0) return null;
		const sorted = [...okValues].sort((a, b) => a - b);
		const p85 = sorted[Math.floor(sorted.length * 0.85)];
		const p15 = sorted[Math.floor(sorted.length * 0.15)];
		const rawMax = p85;
		const rawMin = Math.max(0, p15);
		const rawRange = rawMax - rawMin;
		const padding = Math.max(rawRange * 0.15, Math.max(rawMin * 0.1, 20));
		const maxVal = rawMax + padding;
		const minVal = Math.max(0, rawMin - padding);
		return { min: minVal, max: maxVal };
	});

	// Build uPlot series: up (teal) and down (red) with NaN gaps
	let chartSeries = $derived.by(() => {
		const ts = chartTimestamps;
		const vals = chartValues;
		const st = chartStatus;
		if (ts.length === 0) return [];

		const upData = [];
		const downData = [];
		for (let i = 0; i < ts.length; i++) {
			const isDown = st[i] === 'down';
			upData.push(isDown ? null : vals[i]);
			downData.push(isDown ? vals[i] : null);
		}

		return [
			{
				label: 'Response Time',
				data: upData,
				color: '#10b981',
				width: 2,
				fill: 'rgba(16,185,129,0.08)',
				spanGaps: false,
			},
			{
				label: 'Down',
				data: downData,
				color: '#ef4444',
				width: 2,
				fill: false,
				spanGaps: true,
				points: { show: true, size: 4, stroke: '#ef4444', fill: '#ef4444' },
			},
		];
	});
</script>

<div class="page-container">
	{#if loading}
		<div class="flex items-center justify-center py-24">
			<Icon icon="svg-spinners:180-ring" class="h-8 w-8" style="color: var(--color-primary);" />
		</div>
	{:else if error}
		<div class="card p-8 text-center">
			<Icon icon="solar:danger-circle-bold" class="mx-auto h-10 w-10" style="color: var(--color-error);" />
			<p class="mt-3 font-medium" style="color: var(--color-text);">Failed to load monitor</p>
			<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">{error}</p>
			<button class="btn-secondary mt-4" onclick={loadMonitor}>Retry</button>
			<button class="btn-ghost mt-2 ml-2" onclick={() => goto('/uptime')}>Back to list</button>
		</div>
	{:else if monitor}
		{@const cfg = getConfig(monitor.status)}

		<!-- Back + Header -->
		<div class="mb-4">
			<button class="btn-ghost" onclick={() => goto('/uptime')}>
				<Icon icon="solar:arrow-left-bold" class="h-4 w-4" />
				Back
			</button>
		</div>

		<div class="mb-6 flex flex-wrap items-start justify-between gap-4">
			<div class="flex items-center gap-4">
				<div>
					<h1 class="text-2xl font-bold" style="color: var(--color-text);">
						{monitor.name}
						{#if sseConnected}
							<span class="live-badge">
								<span class="live-dot"></span>
								Live
							</span>
						{/if}
					</h1>
					<p class="mt-1 text-sm font-mono" style="color: var(--color-text-secondary);">
						{monitor.url}
						{#if monitor.check_type}
							<span class="mx-2">&middot;</span>
							<span>{monitor.check_type?.toUpperCase()}</span>
						{/if}
					</p>
				</div>
			</div>
			<div class="flex items-center gap-2">
				<span class="status-badge" style="background: {cfg.color}15; color: {cfg.color};">
					{cfg.label}
				</span>
				<button class="btn-secondary" onclick={checkNow} disabled={checking}>
					<Icon icon={checking ? 'svg-spinners:180-ring' : 'solar:refresh-bold'} class="h-4 w-4" />
					{checking ? 'Checking...' : 'Check Now'}
				</button>
				<button class="btn-secondary" onclick={togglePause}>
					<Icon icon={monitor.status === 'paused' ? 'solar:play-bold' : 'solar:pause-bold'} class="h-4 w-4" />
					{monitor.status === 'paused' ? 'Resume' : 'Pause'}
				</button>
				<button class="btn-secondary" onclick={() => { editingMonitor = monitor; showEditModal = true; }}>
					<Icon icon="solar:settings-bold" class="h-4 w-4" />
					Settings
				</button>
				<button class="btn-danger" onclick={() => showDeleteConfirm = true}>
					<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
					Delete
				</button>
			</div>
		</div>

		<!-- Main grid -->
		<div class="detail-grid">
			<!-- Left Column -->
			<div class="space-y-6">
				<!-- Current Status -->
				<div class="detail-section">
					<div class="detail-section-title">
						Current Status
						<span class="status-badge" style="background: {cfg.color}15; color: {cfg.color};">
							{cfg.label}
						</span>
					</div>
					{#each [
						{ label: 'URL', value: monitor.url, mono: true },
						{ label: 'Check Type', value: `${monitor.check_type?.toUpperCase() || 'HTTP'} · ${intervalLabel(monitor.interval_seconds)}` },
						{ label: 'Last Check', value: formatTimeAgo(monitor.last_check_at) },
						{ label: 'Last Status Code', value: monitor.last_status_code ? `${monitor.last_status_code}` : '-' },
						{ label: 'Response Time', value: monitor.last_response_time_ms != null ? `${monitor.last_response_time_ms}ms` : '-', color: responseTimeColor(monitor.last_response_time_ms) },
						{ label: 'Last Error', value: monitor.last_error || '—' },
					] as row}
						<div class="detail-row">
							<div class="detail-label">{row.label}</div>
							<div class="detail-value" style="color: {row.color || 'var(--color-text)'};{row.mono ? ' font-family: monospace;' : ''}">
								{row.value || '-'}
							</div>
						</div>
					{/each}
				</div>

				<!-- Response Time Chart -->
				<div class="detail-section">
					<div class="detail-section-title">
						📈 Response Time ({trendPeriod})
						<div class="flex gap-2">
							<button
								class="tab-btn-sm"
								class:active={trendPeriod === '24h'}
								onclick={() => changeTrendPeriod('24h')}
							>24h</button>
							<button
								class="tab-btn-sm"
								class:active={trendPeriod === '7d'}
								onclick={() => changeTrendPeriod('7d')}
							>7d</button>
							<button
								class="tab-btn-sm"
								class:active={trendPeriod === '30d'}
								onclick={() => changeTrendPeriod('30d')}
							>30d</button>
							<button
								class="tab-btn-sm"
								class:active={showCustomRange || trendPeriod === 'custom'}
								onclick={() => changeTrendPeriod('custom')}
							>Custom</button>
						</div>
						{#if showCustomRange}
							<div class="flex items-center gap-2 ml-2">
								<input
									type="date"
									bind:value={customFrom}
									class="input !py-1 !px-2 text-xs w-32"
									required
								/>
								<span class="text-xs" style="color: var(--color-text-muted);">to</span>
								<input
									type="date"
									bind:value={customTo}
									class="input !py-1 !px-2 text-xs w-32"
								/>
								<button class="btn-primary !py-1 !px-2 text-xs" onclick={applyCustomRange}>
									Apply
								</button>
							</div>
						{/if}
					</div>
					{#if trendLoading}
						<div class="flex items-center justify-center py-8">
							<Icon icon="svg-spinners:180-ring" class="h-6 w-6" style="color: var(--color-primary);" />
						</div>
					{:else if chartTimestamps.length > 0 && chartSeries.length > 0}
						<div class="chart-container">
							<MetricsChart
								timestamps={chartTimestamps}
								series={chartSeries}
								height={200}
								yLabel="ms"
								yMin={yAxisRange?.min}
								yMax={yAxisRange?.max}
								formatY={(v) => v != null ? `${Math.round(v)}ms` : '—'}
							/>
						</div>
					{:else}
						<p class="py-6 text-center text-sm" style="color: var(--color-text-muted);">
							Not enough data to show chart. Run a few checks first.
						</p>
					{/if}
				</div>

				<!-- Response Time Stats -->
				{#if responseTimeStats}
					<div class="detail-section">
						<div class="detail-section-title">📊 Response Time Stats</div>
						<div class="rt-stats-grid">
							{#each ['period_24h', 'period_7d', 'period_30d'] as periodKey}
								{@const p = responseTimeStats[periodKey]}
								{@const label = periodKey === 'period_24h' ? '24h' : periodKey === 'period_7d' ? '7d' : '30d'}
								{#if p}
									<div class="rt-stats-card">
										<div class="rt-stats-period">{label}</div>
										<div class="rt-stats-row">
											<span class="rt-stats-label">Min</span>
											<span class="rt-stats-value">{p.min_response_ms != null ? `${Math.round(p.min_response_ms)}ms` : '—'}</span>
										</div>
										<div class="rt-stats-row">
											<span class="rt-stats-label">Avg</span>
											<span class="rt-stats-value">{p.avg_response_ms != null ? `${Math.round(p.avg_response_ms)}ms` : '—'}</span>
										</div>
										<div class="rt-stats-row">
											<span class="rt-stats-label">Max</span>
											<span class="rt-stats-value">{p.max_response_ms != null ? `${Math.round(p.max_response_ms)}ms` : '—'}</span>
										</div>
										{#if p.p95_response_ms != null}
											<div class="rt-stats-row">
												<span class="rt-stats-label">P95</span>
												<span class="rt-stats-value">{Math.round(p.p95_response_ms)}ms</span>
											</div>
										{/if}
									</div>
								{/if}
							{/each}
						</div>
					</div>
				{/if}

				<!-- Check History -->
				<div class="detail-section">
					<div class="detail-section-title">
						📋 Check History
						{#if historyTotal > 0}
							<span class="text-xs font-normal" style="color: var(--color-text-muted);">({historyTotal} checks)</span>
						{/if}
					</div>
					{#if historyLoading}
						<div class="flex items-center justify-center py-8">
							<Icon icon="svg-spinners:180-ring" class="h-6 w-6" style="color: var(--color-primary);" />
						</div>
					{:else if historyEntries.length === 0}
						<p class="py-6 text-center text-sm" style="color: var(--color-text-muted);">
							No check history yet. Run your first check to see results here.
						</p>
					{:else}
						<div class="history-list">
							{#each historyEntries as h (h.id || h.checked_at)}
								{@const sc = getConfig(h.status)}
								<div
									class="history-item"
									class:history-item-down={h.status === 'down'}
								>
									<div class="history-time" title={formatDate(h.checked_at)}>
										{formatTimeAgo(h.checked_at)}
									</div>
									<div class="history-status" style="background: {sc.color};"></div>
									<div class="history-code" style="color: {h.status === 'down' ? '#ef4444' : 'var(--color-text-secondary)'};">{h.status_code || '-'}</div>
									<div class="history-ms" style="color: {responseTimeColor(h.response_time_ms)};">
										{h.response_time_ms != null ? `${h.response_time_ms}ms` : '—'}
									</div>
									<span class="status-badge" style="background: {sc.color}15; color: {sc.color}; font-size: 10px; padding: 0 6px;">
										{sc.label}
									</span>
									{#if h.error}
										<span class="text-xs" style="color: #ef4444; margin-left: auto;">{h.error}</span>
									{/if}
								</div>
							{/each}
						</div>
						{#if historyEntries.length >= historyLimit}
							<div class="flex justify-center pt-3">
								<button class="btn-secondary px-4 py-1.5 text-sm" onclick={loadMore}>
									<Icon icon="solar:round-arrow-down-bold" class="h-4 w-4" />
									Load More
								</button>
							</div>
						{/if}
					{/if}
				</div>

				<!-- Incidents -->
				<div class="detail-section">
					<div class="detail-section-title">
						⚠️ Incidents
						{#if incidentsTotal > 0}
							<span class="text-xs font-normal" style="color: var(--color-text-muted);">({incidentsTotal} total)</span>
						{/if}
					</div>
					{#if incidentsLoading}
						<div class="flex items-center justify-center py-8">
							<Icon icon="svg-spinners:180-ring" class="h-6 w-6" style="color: var(--color-primary);" />
						</div>
					{:else if incidents.length === 0}
						<p class="py-6 text-center text-sm" style="color: var(--color-text-muted);">
							No incidents recorded. All checks have been passing.
						</p>
					{:else}
						<div class="incidents-list">
							{#each incidents as inc (inc.id)}
								<div class="incident-item">
									<div class="incident-header">
										<div class="incident-dot"></div>
										<div class="incident-duration">{formatDuration(inc.duration_sec)}</div>
										<span class="incident-count">{inc.failure_count} failed {inc.failure_count === 1 ? 'check' : 'checks'}</span>
									</div>
									<div class="incident-times">
										<span>{formatDate(inc.started_at)}</span>
										<span class="incident-arrow">→</span>
										<span>{formatDate(inc.ended_at)}</span>
									</div>
									{#if inc.error_message}
										<div class="incident-error">{inc.error_message}</div>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				</div>
			</div>

			<!-- Right Column -->
			<div class="space-y-6">
				<!-- Uptime Percentage -->
				<div class="detail-section">
					<div class="detail-section-title">📊 Uptime</div>
					<div class="uptime-cards">
						<div class="uptime-card">
							<div class="uptime-card-value" class:good={monitor.uptime_24h >= 99} class:ok={monitor.uptime_24h >= 95 && monitor.uptime_24h < 99} class:bad={monitor.uptime_24h < 95}>
								{monitor.uptime_24h != null ? `${monitor.uptime_24h.toFixed(1)}%` : '—'}
							</div>
							<div class="uptime-card-label">Last 24h</div>
						</div>
						<div class="uptime-card">
							<div class="uptime-card-value" class:good={monitor.uptime_7d >= 99} class:ok={monitor.uptime_7d >= 95 && monitor.uptime_7d < 99} class:bad={monitor.uptime_7d < 95}>
								{monitor.uptime_7d != null ? `${monitor.uptime_7d.toFixed(1)}%` : '—'}
							</div>
							<div class="uptime-card-label">Last 7 days</div>
						</div>
						<div class="uptime-card">
							<div class="uptime-card-value" class:good={monitor.uptime_30d >= 99} class:ok={monitor.uptime_30d >= 95 && monitor.uptime_30d < 99} class:bad={monitor.uptime_30d < 95}>
								{monitor.uptime_30d != null ? `${monitor.uptime_30d.toFixed(1)}%` : '—'}
							</div>
							<div class="uptime-card-label">Last 30 days</div>
						</div>
					</div>
					<div class="text-xs mt-2" style="color: var(--color-text-muted);">
						Total checks: {monitor.total_checks || '—'} · Up: {monitor.up_checks || '—'} · Down: {monitor.down_checks || '—'}
					</div>
				</div>

				<!-- Notifications -->
				<div class="detail-section">
					<div class="detail-section-title">
						🔔 Notifications
						<button class="btn btn-sm" onclick={handleTestMonitorNotification} disabled={testingTarget === 'monitor'}>
							<Icon icon={testingTarget === 'monitor' ? 'svg-spinners:180-ring' : 'solar:play-circle-bold'} class="h-4 w-4" />
							Test All
						</button>
					</div>
					{#if notificationTargetsLoading}
						<p class="text-sm" style="color: var(--color-text-muted);">Loading...</p>
					{:else if !monitor.notification_target_ids?.length}
						<p class="text-sm" style="color: var(--color-text-muted);">No notification targets assigned.</p>
					{:else}
						<div class="flex flex-col gap-2">
							{#each monitor.notification_target_ids as ntId}
								{@const nt = notificationTargets.find(t => t.id === ntId)}
								<div class="flex items-center justify-between rounded-lg border p-3" style="border-color: var(--color-border);">
									<div>
										<p class="text-sm font-medium" style="color: var(--color-text);">
											{nt?.name || 'Unknown'}
										</p>
										<p class="text-xs" style="color: var(--color-text-muted);">
											{nt?.platform || '-'} &middot; Notify on: status change
										</p>
									</div>
									<div class="flex items-center gap-2">
										<span class="h-2 w-2 rounded-full" style="background: {nt?.enabled ? '#22c55e' : '#94a3b8'};"></span>
										<button
											class="btn-icon text-xs"
											title="Test notification"
											onclick={() => handleTestTarget(ntId)}
										>
											<Icon icon={testingTarget === ntId ? 'svg-spinners:180-ring' : 'solar:play-circle-bold'} class="h-4 w-4" />
										</button>
									</div>
								</div>
							{/each}
						</div>
					{/if}
					{#if testTargetResult}
						<div class="mt-3 rounded-lg border p-3 text-sm" style="border-color: {testTargetResult.success ? 'var(--color-primary)' : '#ef4444'}30; background: {testTargetResult.success ? 'var(--color-primary-subtle)' : '#ef4444'}10;">
							<div class="flex items-center gap-2">
								<Icon icon={testTargetResult.success ? 'solar:check-circle-bold' : 'solar:danger-circle-bold'} class="h-4 w-4" style="color: {testTargetResult.success ? 'var(--color-primary)' : '#ef4444'};" />
								<span style="color: {testTargetResult.success ? 'var(--color-primary)' : '#ef4444'};">
									{testTargetResult.success ? 'Notification sent!' : 'Test failed'}
								</span>
								<button class="ml-auto btn-icon" onclick={() => testTargetResult = null}>
									<Icon icon="solar:close-circle-bold" class="h-4 w-4" />
								</button>
							</div>
							{#if !testTargetResult.success && testTargetResult.error}
								<p class="mt-1 text-xs" style="color: #ef4444;">{testTargetResult.error}</p>
							{/if}
						</div>
					{/if}
				</div>
				<!-- Maintenance Windows -->
				<div class="detail-section">
					<div class="detail-section-title">
						🔧 Maintenance Windows
						<button class="btn btn-sm" onclick={() => { showMaintenanceModal = true; loadMaintenanceWindows(); }}>
							<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
							Add
						</button>
					</div>
					{#if maintenanceLoading}
						<p class="text-sm" style="color: var(--color-text-muted);">Loading...</p>
					{:else if maintenanceWindows.length === 0}
						<p class="text-sm" style="color: var(--color-text-muted);">No maintenance windows scheduled.</p>
					{:else}
						<div class="flex flex-col gap-2">
							{#each maintenanceWindows as mw (mw.id)}
								{@const now = new Date()}
								{@const startsAt = new Date(mw.starts_at)}
								{@const endsAt = new Date(mw.ends_at)}
								{@const isActive = now >= startsAt && now <= endsAt}
								{@const isScheduled = now < startsAt}
								<div class="mw-item" class:mw-active={isActive}>
									<div class="mw-info">
										<div class="mw-desc">{mw.description}</div>
										<div class="mw-time">{formatDate(mw.starts_at)} → {formatDate(mw.ends_at)}</div>
										{#if isActive}
											<span class="mw-badge mw-badge-active">Active</span>
										{:else if isScheduled}
											<span class="mw-badge mw-badge-scheduled">Scheduled</span>
										{:else}
											<span class="mw-badge mw-badge-ended">Ended</span>
										{/if}
									</div>
									<button class="btn-icon" onclick={() => handleDeleteMaintenance(mw.id)} title="Delete">
										<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" style="color: #ef4444;" />
									</button>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<!-- Configuration -->
				<div class="detail-section">
					<div class="detail-section-title">⚙️ Configuration</div>
					<div class="space-y-3 text-sm">
						<div class="flex justify-between">
							<span style="color: var(--color-text-muted);">Check Interval</span>
							<span class="font-medium" style="color: var(--color-text);">{intervalLabel(monitor.interval_seconds)}</span>
						</div>
						<div class="flex justify-between">
							<span style="color: var(--color-text-muted);">Timeout</span>
							<span class="font-medium" style="color: var(--color-text);">{monitor.timeout_seconds || 30}s</span>
						</div>
						<div class="flex justify-between">
							<span style="color: var(--color-text-muted);">Expected Status</span>
							<span class="font-medium" style="color: var(--color-text);">{monitor.expected_status_min || 200}–{monitor.expected_status_max || 399}</span>
						</div>
						{#if monitor.expected_body}
							<div class="flex justify-between">
								<span style="color: var(--color-text-muted);">Expected Body</span>
								<span class="font-medium font-mono text-xs" style="color: var(--color-text);">{monitor.expected_body}</span>
							</div>
						{/if}
						<div class="flex justify-between">
							<span style="color: var(--color-text-muted);">Created</span>
							<span class="font-medium" style="color: var(--color-text);">{formatDate(monitor.created_at)}</span>
						</div>
					</div>
				</div>

				<!-- Quick Actions -->
				<div class="detail-section">
					<div class="detail-section-title">⚡ Quick Actions</div>
					<div class="flex flex-col gap-2">
						<button class="btn btn-outline-primary justify-center" onclick={checkNow} disabled={checking}>
							<Icon icon={checking ? 'svg-spinners:180-ring' : 'solar:refresh-bold'} class="h-4 w-4" />
							{checking ? 'Checking...' : 'Check Now'}
						</button>
						<button class="btn justify-center" onclick={togglePause}>
							<Icon icon={monitor.status === 'paused' ? 'solar:play-bold' : 'solar:pause-bold'} class="h-4 w-4" />
							{monitor.status === 'paused' ? 'Resume Monitoring' : 'Pause Monitoring'}
						</button>
						<button class="btn justify-center" onclick={() => { editingMonitor = monitor; showEditModal = true; }}>
							<Icon icon="solar:pen-bold" class="h-4 w-4" />
							Edit Monitor
						</button>
						<button class="btn justify-center" style="color: #ef4444; border-color: rgba(239,68,68,0.2);" onclick={() => showDeleteConfirm = true}>
							<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
							Delete Monitor
						</button>
					</div>
				</div>
			</div>
		</div>
	{/if}
</div>

<!-- Edit Modal -->
{#if showEditModal}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div class="modal-overlay" onclick={() => { showEditModal = false; }}>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div class="modal-panel" onclick={(e) => e.stopPropagation()}>
			<div class="flex items-center justify-between border-b pb-4" style="border-color: var(--color-border);">
				<div>
					<h2 class="text-lg font-bold" style="color: var(--color-text);">Edit Monitor</h2>
					<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">Update monitor configuration</p>
				</div>
				<button class="btn-icon" onclick={() => { showEditModal = false; }}>
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>

			<form onsubmit={handleEdit} class="mt-5">
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Monitor Name *</label>
					<input type="text" name="name" required value={editingMonitor?.name || ''} class="input w-full" />
				</div>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">URL / Host:Port *</label>
					<input type="text" name="url" required value={editingMonitor?.url || ''} class="input w-full" />
				</div>
				<div class="mb-4 grid grid-cols-2 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Check Type</label>
						<select name="check_type" class="input w-full">
							<option value="http" selected={editingMonitor?.check_type === 'http'}>HTTP(S)</option>
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
							<input type="number" name="expected_status_min" value={editingMonitor?.expected_status_min || 200} min="100" max="599" class="input w-20" />
							<span class="text-xs" style="color: var(--color-text-muted);">to</span>
							<input type="number" name="expected_status_max" value={editingMonitor?.expected_status_max || 399} min="100" max="599" class="input w-20" />
						</div>
					</div>
				</div>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Expected Body (optional)</label>
					<input type="text" name="expected_body" value={editingMonitor?.expected_body || ''} class="input w-full" />
				</div>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Notify Via</label>
					{#if notificationTargetsLoading}
						<p class="text-xs" style="color: var(--color-text-muted);">Loading...</p>
					{:else if notificationTargets.length === 0}
						<p class="text-xs" style="color: var(--color-text-muted);">No targets configured.</p>
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
											<p class="truncate text-xs" style="color: var(--color-text-muted);">{nt.platform} &middot; {urlHostname(nt.url)}</p>
										</div>
									</label>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<div class="flex items-center justify-end gap-3 pt-4">
					<button type="button" class="btn-secondary" onclick={() => { showEditModal = false; }}>Cancel</button>
					<button type="submit" class="btn-primary">Update Monitor</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Delete Confirm -->
{#if showDeleteConfirm}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-overlay" onclick={() => showDeleteConfirm = false}>
		<div class="modal-panel max-w-sm p-6 text-center" onclick={(e) => e.stopPropagation()}>
			<Icon icon="solar:danger-circle-bold" class="mx-auto h-10 w-10" style="color: #ef4444;" />
			<p class="mt-3 text-lg font-bold" style="color: var(--color-text);">Delete Monitor?</p>
			<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
				This will permanently delete "{monitor.name}". This action cannot be undone.
			</p>
			<div class="mt-5 flex items-center justify-center gap-3">
				<button class="btn-secondary" onclick={() => showDeleteConfirm = false}>Cancel</button>
				<button class="btn-danger" onclick={deleteMonitor}>Delete</button>
			</div>
		</div>
	</div>
{/if}

<!-- Maintenance Window Modal -->
{#if showMaintenanceModal}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-overlay" onclick={() => showMaintenanceModal = false}>
		<div class="modal-panel max-w-sm" onclick={(e) => e.stopPropagation()}>
			<div class="flex items-center justify-between border-b pb-4" style="border-color: var(--color-border);">
				<div>
					<h2 class="text-lg font-bold" style="color: var(--color-text);">Schedule Maintenance</h2>
					<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">Pause monitoring during a maintenance period</p>
				</div>
				<button class="btn-icon" onclick={() => showMaintenanceModal = false}>
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>

			<form onsubmit={handleCreateMaintenance} class="mt-5">
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Description *</label>
					<input type="text" name="description" required placeholder="e.g. Database migration, Server upgrade" class="input w-full" />
				</div>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Start Date *</label>
					<input type="date" name="starts_at" required class="input w-full" />
				</div>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Start Time (optional)</label>
					<input type="time" name="starts_time" class="input w-full" />
				</div>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">End Date *</label>
					<input type="date" name="ends_at" required class="input w-full" />
				</div>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">End Time (optional)</label>
					<input type="time" name="ends_time" class="input w-full" />
				</div>

				<div class="flex items-center justify-end gap-3 pt-4">
					<button type="button" class="btn-secondary" onclick={() => showMaintenanceModal = false}>Cancel</button>
					<button type="submit" class="btn-primary" disabled={maintenanceSaving}>
						{maintenanceSaving ? 'Saving...' : 'Schedule'}
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
	.detail-grid {
		display: grid;
		grid-template-columns: 3fr 2fr;
		gap: 1.25rem;
	}
	.detail-section {
		background: var(--color-card);
		border: 1px solid var(--color-border);
		border-radius: 12px;
		padding: 1.25rem;
	}
	.detail-section-title {
		font-size: 0.8125rem;
		font-weight: 600;
		color: var(--color-text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.03em;
		margin-bottom: 0.875rem;
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.detail-row {
		display: flex;
		padding: 0.5rem 0;
		border-bottom: 1px solid var(--color-border);
	}
	.detail-row:last-child { border-bottom: none; }
	.detail-label {
		width: 130px;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		flex-shrink: 0;
	}
	.detail-value {
		font-size: 0.8125rem;
		flex: 1;
		word-break: break-all;
	}
	.chart-container { margin-top: 0.75rem; position: relative; min-height: 200px; }
	.chart-container :global(.uplot) { width: 100% !important; }
	.history-list { max-height: 300px; overflow-y: auto; }
	.history-item {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.5rem 0;
		border-bottom: 1px solid var(--color-border);
	}
	.history-item-down {
		background: rgba(239,68,68,0.05);
		border-radius: 4px;
		padding: 0.5rem 0;
	}
	.history-time { font-size: 0.6875rem; color: var(--color-text-muted); width: 70px; flex-shrink: 0; }
	.history-status { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
	.history-code { font-size: 0.6875rem; color: var(--color-text-secondary); width: 36px; text-align: center; flex-shrink: 0; font-family: monospace; }
	.history-ms { font-size: 0.75rem; font-weight: 500; width: 48px; text-align: right; flex-shrink: 0; }
	.uptime-cards { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.5rem; margin-bottom: 0.5rem; }
	.uptime-card { background: var(--color-bg); border-radius: 8px; padding: 0.625rem; text-align: center; border: 1px solid var(--color-border); }
	.uptime-card-value { font-size: 1.125rem; font-weight: 700; }
	.uptime-card-value.good { color: #22c55e; }
	.uptime-card-value.ok { color: #f59e0b; }
	.uptime-card-value.bad { color: #ef4444; }
	.uptime-card-label { font-size: 0.6875rem; color: var(--color-text-muted); }
	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		padding: 2px 10px;
		border-radius: 10px;
		font-size: 0.75rem;
		font-weight: 600;
	}
	.tab-btn-sm {
		padding: 4px 10px;
		font-size: 0.75rem;
		font-weight: 500;
		cursor: pointer;
		border: 1px solid var(--color-border);
		background: var(--color-card);
		color: var(--color-text-secondary);
		border-radius: 6px;
		transition: all 0.15s;
	}
	.tab-btn-sm.active {
		background: var(--color-primary);
		color: #fff;
		border-color: var(--color-primary);
	}
	.tab-btn-sm:hover:not(.active) { background: var(--color-hover); }
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
	.btn-danger {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		border-radius: 8px;
		padding: 0.5rem 1rem;
		font-size: 0.875rem;
		font-weight: 600;
		color: #fff;
		background: #ef4444;
		border: none;
		cursor: pointer;
		transition: opacity 0.15s;
	}
	.btn-danger:hover { opacity: 0.9; }
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
	.btn {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		border-radius: 8px;
		padding: 0.5rem 0.875rem;
		font-size: 0.8125rem;
		font-weight: 500;
		border: 1px solid var(--color-border);
		cursor: pointer;
		background: var(--color-card);
		color: var(--color-text);
		transition: background 0.15s;
	}
	.btn:hover { background: var(--color-hover); }
	.btn-outline-primary { border-color: var(--color-primary); color: var(--color-primary); }
	.btn-outline-primary:hover { background: var(--color-primary-subtle); }
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
	select.input {
		appearance: auto;
	}
	@media (max-width: 768px) {
		.detail-grid { grid-template-columns: 1fr; }
	}
	:global(body.dark) .card { background: #1a1d23; border-color: rgba(148,163,184,0.08); }
	:global(body.dark) .detail-section { background: #1a1d23; border-color: rgba(148,163,184,0.08); }
	:global(body.dark) .uptime-card { background: rgba(148,163,184,0.06); }
	:global(body.dark) .modal-panel { background: #1a1d23; }
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
	.mw-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.625rem;
		border-radius: 8px;
		border: 1px solid var(--color-border);
		background: var(--color-bg);
		gap: 0.5rem;
	}
	.mw-item.mw-active { border-color: #8b5cf6; background: rgba(139,92,246,0.05); }
	.mw-info { flex: 1; min-width: 0; }
	.mw-desc { font-size: 0.8125rem; font-weight: 500; color: var(--color-text); }
	.mw-time { font-size: 0.6875rem; color: var(--color-text-muted); margin-top: 2px; }
	.mw-badge {
		display: inline-flex;
		padding: 1px 6px;
		border-radius: 10px;
		font-size: 0.625rem;
		font-weight: 600;
		margin-top: 2px;
	}
	.mw-badge-active { background: rgba(139,92,246,0.15); color: #8b5cf6; }
	.mw-badge-scheduled { background: rgba(245,158,11,0.15); color: #f59e0b; }
	.mw-badge-ended { background: rgba(148,163,184,0.15); color: #94a3b8; }

	/* ─── Response Time Stats ────────────────────────────── */
	.rt-stats-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.5rem;
	}
	.rt-stats-card {
		padding: 0.75rem;
		border-radius: 8px;
		background: var(--color-bg);
		border: 1px solid var(--color-border);
	}
	.rt-stats-period {
		font-size: 0.6875rem;
		font-weight: 600;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.03em;
		margin-bottom: 0.5rem;
	}
	.rt-stats-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.1875rem 0;
	}
	.rt-stats-label {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
	}
	.rt-stats-value {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--color-text);
		font-variant-numeric: tabular-nums;
	}
	@media (max-width: 480px) {
		.rt-stats-grid { grid-template-columns: 1fr; }
	}

	/* ─── Incidents ──────────────────────────────────────── */
	.incidents-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.incident-item {
		padding: 0.75rem;
		border-radius: 8px;
		border: 1px solid rgba(239,68,68,0.15);
		background: rgba(239,68,68,0.03);
	}
	.incident-header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-bottom: 0.375rem;
	}
	.incident-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: #ef4444;
		flex-shrink: 0;
	}
	.incident-duration {
		font-size: 0.875rem;
		font-weight: 700;
		color: #ef4444;
	}
	.incident-count {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
		margin-left: auto;
	}
	.incident-times {
		display: flex;
		align-items: center;
		gap: 0.375rem;
		font-size: 0.6875rem;
		color: var(--color-text-secondary);
	}
	.incident-arrow {
		color: var(--color-text-muted);
	}
	.incident-error {
		margin-top: 0.375rem;
		padding: 0.375rem 0.5rem;
		border-radius: 4px;
		font-size: 0.6875rem;
		font-family: monospace;
		color: #ef4444;
		background: rgba(239,68,68,0.06);
		word-break: break-word;
	}
</style>