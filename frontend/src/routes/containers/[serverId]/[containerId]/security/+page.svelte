<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	// ── Route params ──────────────────────────────────────
	let serverId = $derived($page.params.serverId);
	let containerId = $derived($page.params.containerId);

	// ── Core state ─────────────────────────────────────────
	let data = $state(null);        // container security response
	let loading = $state(true);
	let error = $state('');

	// ── Server/Container switcher ──────────────────────────
	let allServers = $state([]);    // from byServer()
	let selectedServerId = $state('');
	let selectedContainerId = $state('');
	let serverDropdownOpen = $state(false);
	let containerDropdownOpen = $state(false);
	let switcherLoading = $state(true);

	// ── Security data ──────────────────────────────────────
	let security = $derived(data?.security || null);
	let container = $derived(data?.container || null);
	let serverInfo = $derived(data?.server || null);
	let findings = $derived(security?.findings || []);
	let badges = $derived(security?.badges || []);
	let scannedAt = $derived(security?.scanned_at || null);
	let score = $derived(security?.score ?? null);

	// ── Scan history ───────────────────────────────────────
	let history = $state([]);
	let historyLoading = $state(false);
	let selectedHistoryId = $state(null);  // when viewing old scan
	let historicalFindings = $state(null);
	let historicalSecurity = $state(null); // current or historical

	// ── Scanning ───────────────────────────────────────────
	let scanning = $state(false);
	let scanPollId = $state(null);

	// ── Filters ────────────────────────────────────────────
	let activeFilters = $state(['all']);
	let filterSeverities = ['all', 'critical', 'high', 'medium', 'low', 'passed'];

	// ── Severity colors ────────────────────────────────────
	const severityColors = {
		critical: '#ef4444',
		high: '#fb923c',
		medium: '#fbbf24',
		low: '#60a5fa',
		warning: '#f59e0b',
		pass: '#34d399',
		info: '#94a3b8'
	};
	const severityBgs = {
		critical: 'rgba(239,68,68,0.08)',
		high: 'rgba(249,115,22,0.08)',
		medium: 'rgba(251,191,36,0.08)',
		low: 'rgba(96,165,250,0.08)',
		pass: 'rgba(52,211,153,0.08)',
	};

	// ── Derived ────────────────────────────────────────────
	let displayFindings = $derived.by(() => {
		const source = historicalFindings || findings;
		if (!source || source.length === 0) return [];
		if (activeFilters.includes('all')) return source;
		return source.filter(f => {
			const sev = (f.severity || '').toLowerCase();
			const status = (f.status || '').toLowerCase();
			if (activeFilters.includes('passed')) return status === 'pass' || sev === 'info';
			return activeFilters.includes(sev);
		});
	});

	let summaryCounts = $derived.by(() => {
		const counts = { critical: 0, high: 0, medium: 0, low: 0, passed: 0, total: 0 };
		const source = findings || [];
		for (const f of source) {
			const sev = (f.severity || '').toLowerCase();
			const status = (f.status || '').toLowerCase();
			if (status === 'pass' || sev === 'info' || sev === 'passed') {
				counts.passed++;
			} else if (sev in counts) {
				counts[sev]++;
			}
			counts.total++;
		}
		return counts;
	});

	let filteredContainerList = $derived.by(() => {
		const sv = allServers.find(s => s.server.id === selectedServerId);
		return sv?.containers || [];
	});

	function scoreColor(s) {
		if (s == null) return 'var(--text-muted)';
		if (s >= 80) return '#34d399';
		if (s >= 60) return '#fbbf24';
		return '#ef4444';
	}

	function severityClass(sev, status) {
		const s = (sev || '').toLowerCase();
		const st = (status || '').toLowerCase();
		if (st === 'pass') return 'sev-pass';
		if (s === 'critical') return 'sev-critical';
		if (s === 'high') return 'sev-high';
		if (s === 'medium') return 'sev-medium';
		if (s === 'low') return 'sev-low';
		return 'sev-pass';
	}

	function getFindingsCountForSev(sev) {
		const source = findings || [];
		return source.filter(f => (f.severity || '').toLowerCase() === sev.toLowerCase()).length;
	}

	let filtersWithCounts = $derived([
		{ id: 'all', label: 'All' },
		{ id: 'critical', label: `Critical (${summaryCounts.critical})`, cls: 'danger' },
		{ id: 'high', label: `High (${summaryCounts.high})`, cls: 'danger' },
		{ id: 'medium', label: `Medium (${summaryCounts.medium})`, cls: 'warning' },
		{ id: 'low', label: `Low (${summaryCounts.low})`, cls: 'info' },
		{ id: 'passed', label: `Passed (${summaryCounts.passed})`, cls: '' },
	]);

	function toggleFilter(id) {
		if (id === 'all') {
			activeFilters = ['all'];
			return;
		}
		let arr = activeFilters.filter(f => f !== 'all');
		if (arr.includes(id)) {
			arr = arr.filter(f => f !== id);
		} else {
			arr.push(id);
		}
		if (arr.length === 0) arr = ['all'];
		activeFilters = arr;
	}

	// ── Data Loading ───────────────────────────────────────
	async function loadData() {
		loading = true;
		error = '';
		try {
			const [secData, serversData] = await Promise.all([
				api.containers.security(containerId, serverId),
				api.containers.byServer().catch(() => ({ servers: [] })),
			]);
			data = secData;
			allServers = serversData.servers || [];
			selectedServerId = secData.server?.id || serverId;
			selectedContainerId = containerId;
			historicalFindings = null;
			historicalSecurity = null;
			selectedHistoryId = null;

			// Load scan history
			if (secData.container?.name) {
				loadHistory(secData.server?.id, secData.container.name);
			}
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
			switcherLoading = false;
		}
	}

	async function loadHistory(sid, cname) {
		historyLoading = true;
		try {
			const hData = await api.compliance.containerScanHistory(sid, cname);
			history = hData.results || [];
		} catch {
			history = [];
		} finally {
			historyLoading = false;
		}
	}

	async function viewHistoricalScan(scanId) {
		if (!scanId || !serverInfo) return;
		try {
			const detail = await api.compliance.scanDetail(serverInfo.id, scanId);
			if (detail && detail.findings) {
				const cname = container?.name || '';
				const containerFindings = detail.findings.filter(f => f.category === cname);
				historicalFindings = containerFindings;
				historicalSecurity = {
					score: detail.score,
					findings: containerFindings,
					badges: security?.badges || [], // reuse current badges
					scanned_at: detail.completed_at || detail.created_at,
				};
				selectedHistoryId = scanId;
			}
		} catch (_) {}
	}

	function viewCurrentScan() {
		historicalFindings = null;
		historicalSecurity = null;
		selectedHistoryId = null;
	}

	// ── Scan ───────────────────────────────────────────────
	async function runScan() {
		scanning = true;
		try {
			await api.compliance.scanContainer(serverId, containerId);
			// Poll for completion — check latest Container Security scan
			for (let i = 0; i < 60; i++) {
				await new Promise(r => setTimeout(r, 2000));
				try {
					const latest = await api.compliance.latest(serverId, { scan_type: 'Container Security' });
					if (latest && latest.status === 'completed') {
						// Reload data to get fresh security info
						await loadData();
						break;
					}
					if (latest && latest.status === 'failed') break;
				} catch (_) {}
			}
		} catch (_) {}
		scanning = false;
	}

	// ── Switcher navigation ────────────────────────────────
	function navigateToContainer(sid, cid) {
		serverDropdownOpen = false;
		containerDropdownOpen = false;
		if (cid === selectedContainerId && sid === selectedServerId) return;
		goto(`/containers/${sid}/${cid}/security`);
	}

	function changeServer(sid) {
		selectedServerId = sid;
		// Auto-select first container of new server
		const sv = allServers.find(s => s.server.id === sid);
		if (sv?.containers?.length) {
			const first = sv.containers[0];
			navigateToContainer(sid, first.id);
		}
	}

	// ── Helpers ────────────────────────────────────────────
	function formatTime(ts) {
		if (!ts) return '—';
		const d = new Date(ts);
		if (isNaN(d.getTime())) return ts;
		return d.toLocaleString('en-GB', {
			day: 'numeric', month: 'short', year: 'numeric',
			hour: '2-digit', minute: '2-digit'
		}) + ' WIB';
	}

	function formatShortTime(ts) {
		if (!ts) return '';
		const d = new Date(ts);
		if (isNaN(d.getTime())) return ts;
		const now = new Date();
		const diff = now - d;
		if (diff < 60000) return 'Just now';
		if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
		if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
		if (diff < 604800000) return Math.floor(diff / 86400000) + 'd ago';
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
	}

	function badgeColor(badge) {
		if (badge.startsWith('🔓') || badge.includes('no ')) return 'fail';
		return 'pass';
	}

	// ── Lifecycle ──────────────────────────────────────────
	onMount(() => {
		loadData();
		return () => {
			if (scanPollId) clearInterval(scanPollId);
		};
	});

	// Watch for URL param changes (navigate from switcher)
	$effect(() => {
		const sid = $page.params.serverId;
		const cid = $page.params.containerId;
		if (sid && cid && (sid !== selectedServerId || cid !== selectedContainerId)) {
			loadData();
		}
	});
</script>

<div class="container-security-page">

	<!-- ═══ BREADCRUMB ═══ -->
	<nav class="breadcrumb">
		<a href="/dashboard">Dashboard</a>
		<span class="crumb-sep">›</span>
		<a href="/containers">Containers</a>
		<span class="crumb-sep">›</span>
		<span class="current">Security Report</span>
	</nav>

	<!-- ═══ SERVER + CONTAINER SWITCHER ═══ -->
	<div class="switcher-bar">
		<div class="switcher-group">
			<label class="switcher-label">Server</label>
			<div class="dropdown" class:open={serverDropdownOpen}>
				<button class="dropdown-trigger" onclick={() => { serverDropdownOpen = !serverDropdownOpen; containerDropdownOpen = false; }}>
					<Icon icon="solar:server-bold" class="dropdown-icon" />
					<span class="dropdown-text">{serverInfo?.name || 'Select server...'}</span>
					<Icon icon="solar:alt-arrow-down-bold" class="dropdown-chevron" />
				</button>
				{#if serverDropdownOpen}
					<div class="dropdown-menu">
						{#each allServers as sv (sv.server.id)}
							<button class="dropdown-item" class:active={sv.server.id === selectedServerId}
								onclick={() => changeServer(sv.server.id)}>
								<span class="item-name">{sv.server.name}</span>
								<span class="item-meta">{sv.server.host} · {sv.stats?.running || 0} running</span>
							</button>
						{/each}
					</div>
				{/if}
			</div>
		</div>

		<div class="switcher-group">
			<label class="switcher-label">Container</label>
			<div class="dropdown" class:open={containerDropdownOpen}>
				<button class="dropdown-trigger" onclick={() => { containerDropdownOpen = !containerDropdownOpen; serverDropdownOpen = false; }}>
					<Icon icon="solar:box-bold" class="dropdown-icon" />
					<span class="dropdown-text">{container?.name || 'Select container...'}</span>
					<Icon icon="solar:alt-arrow-down-bold" class="dropdown-chevron" />
				</button>
				{#if containerDropdownOpen}
					<div class="dropdown-menu containers-menu">
						{#each filteredContainerList as ctr (ctr.id)}
							{@const ctrScore = ctr.security?.score ?? null}
							<button class="dropdown-item" class:active={ctr.id === selectedContainerId}
								onclick={() => navigateToContainer(selectedServerId, ctr.id)}>
								<span class="item-name">{ctr.name}</span>
								<span class="score-tag-sm" style="color: {ctrScore != null ? scoreColor(ctrScore) : 'var(--text-muted)'};">
									{ctrScore != null ? `${ctrScore}/100` : '—'}
								</span>
							</button>
						{/each}
						{#if filteredContainerList.length === 0}
							<div class="dropdown-empty">No containers on this server</div>
						{/if}
					</div>
				{/if}
			</div>
		</div>
	</div>

	<!-- ═══ MAIN CONTENT ═══ -->
	{#if loading}
		<div class="loading-state">
			<Icon icon="solar:spinner-bold" class="spinner" />
			<span>Loading security report...</span>
		</div>
	{:else if error}
		<div class="error-state">
			<Icon icon="solar:danger-triangle-bold" style="color: var(--color-danger);" />
			<p>{error}</p>
			<button class="btn btn-outline btn-sm" onclick={loadData}>Retry</button>
		</div>
	{:else if container}
		{@const hasSecurity = security !== null && security !== undefined}
		{@const activeSec = historicalSecurity || security}
		{@const activeFindings = displayFindings}
		{@const activeScore = activeSec?.score ?? null}
		{@const activeScannedAt = activeSec?.scanned_at ?? null}

		<!-- ═══ HEADER CARD ═══ -->
		<div class="header-card">
			<div class="header-left">
				<h1 class="header-title">
					{container.name}
					<span class="header-image">{container.image || '—'}</span>
				</h1>
				<div class="header-meta">
					<span class="status-badge" class:running={container.state === 'running'}
						style={container.state === 'running' ? '' : 'background:rgba(239,68,68,0.12);color:#f87171;'}>
						● {container.state === 'running' ? 'Running' : container.state || container.status || 'Unknown'}
					</span>
					<span>Server: <strong>{serverInfo?.name || '—'}</strong> ({serverInfo?.host || '—'})</span>
					{#if container.ports}
						<span>· {container.ports}</span>
					{/if}
				</div>
			</div>
			<div class="header-right">
				<button class="btn btn-outline btn-sm" onclick={loadData}>
					<Icon icon="solar:refresh-bold" class="icon-sm" /> Refresh
				</button>
				<button class="btn btn-primary btn-sm" onclick={runScan} disabled={scanning}>
					<Icon icon={scanning ? 'solar:spinner-bold' : 'solar:shield-bold'}
						class="icon-sm {scanning ? 'animate-spin' : ''}" />
					{scanning ? 'Scanning...' : 'Rescan'}
				</button>
			</div>
		</div>

		<!-- ═══ SCAN BANNER ═══ -->
		{#if hasSecurity || selectedHistoryId}
			<div class="scan-banner">
				<Icon icon="solar:clock-circle-bold" class="banner-icon" />
				<span>
					Last scanned:
					<span class="scan-timestamp">{formatTime(activeScannedAt)}</span>
				</span>
				<span class="scan-type-tag">Container Security scan</span>
				{#if selectedHistoryId}
					<span class="history-badge">Historical scan</span>
					<button class="btn btn-ghost btn-xs" onclick={viewCurrentScan}>View latest</button>
				{/if}
				<span class="score-tag" style="background:{scoreColor(activeScore)}22;color:{scoreColor(activeScore)};">
					Score: {activeScore ?? '—'}/100
				</span>
			</div>
		{:else}
			<div class="scan-banner no-scan">
				<Icon icon="solar:shield-warning-bold" class="banner-icon" style="color: var(--color-warning);" />
				<span>Not scanned yet. Run a Container Security scan to see results.</span>
				<button class="btn btn-primary btn-sm" onclick={runScan} disabled={scanning}>
					<Icon icon={scanning ? 'solar:spinner-bold' : 'solar:shield-bold'}
						class="icon-sm {scanning ? 'animate-spin' : ''}" />
					{scanning ? 'Scanning...' : 'Scan Now'}
				</button>
			</div>
		{/if}

		<!-- ═══ IF HAS DATA ═══ -->
		{#if hasSecurity || selectedHistoryId}

			<!-- ═══ SCORE OVERVIEW ═══ -->
			<div class="score-grid">
				<div class="score-card big-score">
					<div class="score-value" style="color: {scoreColor(activeScore)};">
						{activeScore ?? '—'}
					</div>
					<div class="score-label">Security Score</div>
				</div>
				<div class="severity-breakdown">
					{#each ['critical', 'high', 'medium', 'low', 'passed'] as sev}
						{@const count = sev === 'passed' ? summaryCounts.passed : summaryCounts[sev]}
						<div class="severity-item">
							<span class="sev-dot" style="background: {severityColors[sev]}"></span>
							<span class="sev-count" style="color: {severityColors[sev]}">{count}</span>
							<span class="sev-label">{sev === 'passed' ? 'Passed' : (sev.charAt(0).toUpperCase() + sev.slice(1))}</span>
						</div>
					{/each}
				</div>
			</div>

			<!-- ═══ BADGES ═══ -->
			{#if badges.length > 0}
				<div class="badges-row">
					{#each badges as badge}
						<span class="badge-tag" class:badge-fail={badgeColor(badge) === 'fail'} class:badge-pass={badgeColor(badge) === 'pass'}>
							{badge}
						</span>
					{/each}
				</div>
			{/if}

			<hr class="section-divider" />

			<!-- ═══ FILTERS ═══ -->
			<div class="filter-bar">
				{#each filtersWithCounts as flt}
					<button class="filter-chip"
						class:active={activeFilters.includes(flt.id)}
						class:chip-danger={flt.cls === 'danger' && activeFilters.includes(flt.id)}
						class:chip-warning={flt.cls === 'warning' && activeFilters.includes(flt.id)}
						class:chip-info={flt.cls === 'info' && activeFilters.includes(flt.id)}
						onclick={() => toggleFilter(flt.id)}>
						{flt.label}
					</button>
				{/each}
				<span class="filter-summary">
					{activeFindings.length} findings
				</span>
			</div>

			<!-- ═══ FINDINGS LIST ═══ -->
			<div class="findings-list">
				{#each activeFindings as finding, i}
					{@const id = 'finding-' + i}
					<details class="finding-card">
						<summary class="finding-header">
							<span class="sev-badge {severityClass(finding.severity, finding.status)}">
								{(finding.severity || 'info').toUpperCase()}
							</span>
							{#if (finding.status || '').toLowerCase() === 'pass'}
								<Icon icon="solar:check-circle-bold" class="status-pass" />
							{:else}
								<Icon icon="solar:close-circle-bold" class="status-fail" />
							{/if}
							<span class="finding-title">{finding.title || finding.check_id || 'Unknown finding'}</span>
							<span class="finding-check-id">{finding.check_id}</span>
							<Icon icon="solar:alt-arrow-right-bold" class="finding-chevron" />
						</summary>
						<div class="finding-body">
							{#if finding.description}
								<p class="finding-desc">{finding.description}</p>
							{/if}
							{#if finding.remediation}
								<div class="remediation-box">
									<strong>🔧 Remediation</strong>
									{finding.remediation}
								</div>
							{/if}
						</div>
					</details>
				{:else}
					<div class="empty-findings">
						<Icon icon="solar:shield-check-bold" class="empty-icon" />
						<p>No findings match the selected filters.</p>
					</div>
				{/each}
			</div>

			<hr class="section-divider" />

			<!-- ═══ SCAN HISTORY ═══ -->
			<div class="history-section">
				<div class="history-header">
					<h3 class="section-title">
						<Icon icon="solar:history-bold" class="title-icon" />
						Scan History
					</h3>
					<span class="history-count">{history.length} scan{history.length !== 1 ? 's' : ''}</span>
				</div>

				{#if historyLoading}
					<div class="history-loading">
						<Icon icon="solar:spinner-bold" class="spinner" />
						Loading history...
					</div>
				{:else if history.length > 0}
					<div class="history-list">
						{#each history as entry (entry.scan_id)}
							<button class="history-item" class:active={entry.scan_id === selectedHistoryId}
								onclick={() => entry.scan_id === selectedHistoryId ? viewCurrentScan() : viewHistoricalScan(entry.scan_id)}>
								<div class="history-item-left">
									<span class="history-dot" class:current={entry.scan_id === (selectedHistoryId || (history[0]?.scan_id))}></span>
									<div>
										<div class="history-time">{formatTime(entry.completed_at)}</div>
										<div class="history-summary">
											{entry.failed} failed · {entry.criticals}C {entry.high}H {entry.medium}M · {entry.passed}P
										</div>
									</div>
								</div>
								<div class="history-item-right">
									<span class="history-score" style="color: {scoreColor(entry.score)};">
										{entry.score}/100
									</span>
									<Icon icon="solar:alt-arrow-right-bold" class="history-arrow" />
								</div>
							</button>
						{/each}
					</div>
				{:else}
					<div class="history-empty">
						<p>No previous scan history available for this container.</p>
					</div>
				{/if}
			</div>

		{/if}

	{:else}
		<div class="error-state">
			<Icon icon="solar:box-minimalistic-bold" style="color: var(--text-muted);" />
			<p>Container not found.</p>
		</div>
	{/if}

</div>

<!-- ═══════════════════ STYLES ═══════════════════ -->

<style>
	/* ── Layout ── */
	.container-security-page {
		max-width: 960px;
		margin: 0 auto;
		padding: 24px 20px;
	}

	/* ── Breadcrumb ── */
	.breadcrumb {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-bottom: 16px;
		font-size: 12px;
	}
	.breadcrumb a {
		color: var(--text-muted);
		text-decoration: none;
		transition: color 0.15s;
	}
	.breadcrumb a:hover { color: var(--color-primary); }
	.crumb-sep { color: var(--text-muted); font-size: 10px; }
	.breadcrumb .current { color: var(--text-secondary); font-weight: 500; }

	/* ── Switcher ── */
	.switcher-bar {
		display: flex;
		gap: 12px;
		margin-bottom: 16px;
		flex-wrap: wrap;
	}
	.switcher-group {
		flex: 1;
		min-width: 200px;
	}
	.switcher-label {
		display: block;
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		color: var(--text-muted);
		margin-bottom: 4px;
	}
	.dropdown { position: relative; }
	.dropdown-trigger {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		padding: 7px 12px;
		border-radius: 8px;
		border: 1px solid var(--color-border-light);
		background: var(--color-surface);
		color: var(--text);
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		transition: border-color 0.15s;
	}
	.dropdown-trigger:hover { border-color: var(--color-primary); }
	.dropdown-icon { width: 16px; height: 16px; flex-shrink: 0; color: var(--text-muted); }
	.dropdown-text { flex: 1; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.dropdown-chevron { width: 14px; height: 14px; flex-shrink: 0; color: var(--text-muted); }
	.dropdown-menu {
		position: absolute;
		top: calc(100% + 4px);
		left: 0;
		right: 0;
		z-index: 50;
		max-height: 240px;
		overflow-y: auto;
		border-radius: 8px;
		border: 1px solid var(--color-border);
		background: var(--color-card);
		box-shadow: 0 8px 24px rgba(0,0,0,0.3);
	}
	.dropdown-item {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 8px 12px;
		border: none;
		background: transparent;
		color: var(--text);
		font-size: 12px;
		cursor: pointer;
		text-align: left;
		transition: background 0.1s;
	}
	.dropdown-item:hover { background: rgba(148,163,184,0.08); }
	.dropdown-item.active {
		background: rgba(16,185,129,0.1);
		border-left: 2px solid var(--color-primary);
	}
	.item-name { flex: 1; font-weight: 500; }
	.item-meta { font-size: 10px; color: var(--text-muted); }
	.score-tag-sm { font-size: 11px; font-weight: 700; flex-shrink: 0; }
	.dropdown-empty { padding: 12px; text-align: center; font-size: 12px; color: var(--text-muted); }
	.containers-menu .dropdown-item { padding: 6px 12px; }

	/* ── Loading / Error ── */
	.loading-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
		padding: 60px 20px;
		color: var(--text-muted);
		font-size: 13px;
	}
	.spinner { width: 24px; height: 24px; animation: spin 1s linear infinite; }
	@keyframes spin { to { transform: rotate(360deg); } }
	.error-state {
		text-align: center;
		padding: 60px 20px;
		color: var(--color-danger);
		font-size: 13px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
	}
	.error-state :global(svg) { width: 36px; height: 36px; }

	/* ── Header Card ── */
	.header-card {
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: 12px;
		padding: 18px 20px;
		margin-bottom: 12px;
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
	}
	.header-title {
		font-size: 18px;
		font-weight: 700;
		color: var(--text);
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
		margin-bottom: 4px;
	}
	.header-image {
		font-size: 11px;
		font-weight: 500;
		color: var(--text-muted);
		font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
		background: rgba(148,163,184,0.1);
		padding: 2px 8px;
		border-radius: 4px;
	}
	.header-meta {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 12px;
		color: var(--text-muted);
		flex-wrap: wrap;
	}
	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 8px;
		border-radius: 4px;
		font-size: 11px;
		font-weight: 600;
	}
	.status-badge.running { background: rgba(16,185,129,0.15); color: #34d399; }
	.header-right {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-shrink: 0;
	}

	/* ── Buttons ── */
	.btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 7px 14px;
		border-radius: 8px;
		font-size: 12px;
		font-weight: 600;
		border: none;
		cursor: pointer;
		transition: all 0.15s;
		text-decoration: none;
	}
	.btn-primary { background: var(--color-primary); color: #fff; }
	.btn-primary:hover { opacity: 0.9; }
	.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
	.btn-outline { background: transparent; border: 1px solid rgba(148,163,184,0.25); color: var(--text-secondary); }
	.btn-outline:hover { border-color: rgba(148,163,184,0.5); color: var(--text); }
	.btn-sm { padding: 5px 10px; font-size: 11px; }
	.btn-xs { padding: 3px 8px; font-size: 10px; }
	.btn-ghost { background: transparent; border: none; color: var(--color-primary); }
	.btn-ghost:hover { text-decoration: underline; }
	.icon-sm { width: 14px; height: 14px; }

	/* ── Scan Banner ── */
	.scan-banner {
		background: var(--color-card);
		border: 1px solid var(--color-border);
		border-radius: 10px;
		padding: 12px 16px;
		margin-bottom: 16px;
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 12px;
		color: var(--text-secondary);
		flex-wrap: wrap;
	}
	.scan-banner.no-scan { border-style: dashed; }
	.banner-icon { width: 18px; height: 18px; flex-shrink: 0; color: var(--text-muted); }
	.scan-timestamp { font-weight: 600; color: var(--text); }
	.scan-type-tag {
		padding: 2px 8px;
		border-radius: 4px;
		font-size: 10px;
		font-weight: 600;
		background: rgba(16,185,129,0.12);
		color: #34d399;
	}
	.history-badge {
		padding: 2px 8px;
		border-radius: 4px;
		font-size: 10px;
		font-weight: 600;
		background: rgba(245,158,11,0.12);
		color: #fbbf24;
	}
	.score-tag {
		margin-left: auto;
		padding: 3px 10px;
		border-radius: 6px;
		font-size: 12px;
		font-weight: 700;
	}

	/* ── Score Grid ── */
	.score-grid {
		display: grid;
		grid-template-columns: 1fr 2fr;
		gap: 12px;
		margin-bottom: 16px;
	}
	.score-card {
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: 12px;
		padding: 18px;
		text-align: center;
	}
	.score-value { font-size: 34px; font-weight: 800; line-height: 1; margin-bottom: 4px; }
	.score-label { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-muted); }
	.severity-breakdown {
		display: flex;
		flex-wrap: wrap;
		align-content: center;
		gap: 6px;
		padding: 8px;
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: 12px;
		justify-content: center;
	}
	.severity-item {
		display: flex;
		align-items: center;
		gap: 4px;
		padding: 4px 10px;
		border-radius: 6px;
		background: var(--color-card);
	}
	.sev-dot { width: 7px; height: 7px; border-radius: 50%; }
	.sev-count { font-size: 16px; font-weight: 700; }
	.sev-label { font-size: 10px; font-weight: 500; color: var(--text-muted); }

	/* ── Badges ── */
	.badges-row {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		margin-bottom: 12px;
	}
	.badge-tag {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 4px 10px;
		border-radius: 6px;
		font-size: 11px;
		font-weight: 600;
	}
	.badge-fail { background: rgba(239,68,68,0.12); color: #f87171; }
	.badge-pass { background: rgba(16,185,129,0.12); color: #34d399; }

	.section-divider { border: none; border-top: 1px solid var(--color-border-light); margin: 14px 0; }

	/* ── Filters ── */
	.filter-bar {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-bottom: 12px;
		flex-wrap: wrap;
	}
	.filter-chip {
		padding: 4px 10px;
		border-radius: 6px;
		font-size: 11px;
		font-weight: 600;
		border: 1px solid var(--color-border-light);
		background: transparent;
		color: var(--text-secondary);
		cursor: pointer;
		transition: all 0.15s;
	}
	.filter-chip:hover { border-color: rgba(148,163,184,0.3); }
	.filter-chip.active {
		background: rgba(16,185,129,0.1);
		border-color: var(--color-primary);
		color: var(--color-primary);
	}
	.filter-chip.chip-danger.active {
		background: rgba(239,68,68,0.08);
		border-color: #ef4444;
		color: #ef4444;
	}
	.filter-chip.chip-warning.active {
		background: rgba(245,158,11,0.08);
		border-color: #f59e0b;
		color: #f59e0b;
	}
	.filter-chip.chip-info.active {
		background: rgba(59,130,246,0.08);
		border-color: #3b82f6;
		color: #3b82f6;
	}
	.filter-summary {
		font-size: 11px;
		color: var(--text-muted);
		margin-left: auto;
	}

	/* ── Findings List ── */
	.findings-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
		margin-bottom: 16px;
	}
	.finding-card {
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: 10px;
		overflow: hidden;
		transition: border-color 0.15s;
	}
	.finding-card:hover { border-color: rgba(148,163,184,0.25); }
	.finding-card[open] { border-color: var(--color-border); }
	.finding-header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 14px;
		cursor: pointer;
		user-select: none;
		list-style: none;
	}
	.finding-header::-webkit-details-marker { display: none; }
	.sev-badge {
		padding: 2px 7px;
		border-radius: 4px;
		font-size: 9px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		flex-shrink: 0;
	}
	.sev-critical { background: rgba(239,68,68,0.18); color: #ef4444; }
	.sev-high { background: rgba(249,115,22,0.18); color: #fb923c; }
	.sev-medium { background: rgba(245,158,11,0.18); color: #fbbf24; }
	.sev-low { background: rgba(59,130,246,0.18); color: #60a5fa; }
	.sev-pass { background: rgba(16,185,129,0.14); color: #34d399; }
	.status-pass { width: 16px; height: 16px; flex-shrink: 0; color: #34d399; }
	.status-fail { width: 16px; height: 16px; flex-shrink: 0; color: #ef4444; }
	.finding-title { flex: 1; font-size: 12px; font-weight: 600; color: var(--text); }
	.finding-check-id {
		font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
		font-size: 10px;
		color: var(--text-muted);
		background: rgba(148,163,184,0.08);
		padding: 1px 6px;
		border-radius: 4px;
		flex-shrink: 0;
	}
	.finding-chevron {
		width: 14px; height: 14px;
		color: var(--text-muted);
		transition: transform 0.15s;
		flex-shrink: 0;
	}
	.finding-card[open] .finding-chevron { transform: rotate(90deg); }
	.finding-body {
		padding: 0 14px 12px;
	}
	.finding-desc {
		font-size: 12px;
		color: var(--text-secondary);
		margin-bottom: 8px;
		line-height: 1.6;
	}
	.remediation-box {
		background: rgba(16,185,129,0.06);
		border: 1px solid rgba(16,185,129,0.12);
		border-radius: 8px;
		padding: 10px 12px;
		font-size: 12px;
		color: var(--text-secondary);
		line-height: 1.5;
	}
	.remediation-box strong {
		color: var(--color-primary);
		display: block;
		margin-bottom: 2px;
		font-size: 11px;
	}
	.empty-findings {
		text-align: center;
		padding: 40px 20px;
		color: var(--text-muted);
		font-size: 13px;
	}
	.empty-icon { width: 36px; height: 36px; color: var(--text-muted); margin-bottom: 8px; opacity: 0.5; }

	/* ── Scan History ── */
	.history-section {
		margin-bottom: 16px;
	}
	.history-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 10px;
	}
	.section-title {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 13px;
		font-weight: 700;
		color: var(--text);
	}
	.title-icon { width: 16px; height: 16px; color: var(--text-muted); }
	.history-count { font-size: 11px; color: var(--text-muted); }
	.history-loading {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 20px;
		color: var(--text-muted);
		font-size: 12px;
	}
	.history-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.history-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 14px;
		border-radius: 8px;
		border: 1px solid var(--color-border-light);
		background: var(--color-surface);
		cursor: pointer;
		transition: all 0.15s;
		width: 100%;
		text-align: left;
	}
	.history-item:hover { border-color: rgba(148,163,184,0.25); }
	.history-item.active {
		border-color: var(--color-primary);
		background: rgba(16,185,129,0.05);
	}
	.history-item-left {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.history-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--text-muted);
		flex-shrink: 0;
	}
	.history-dot.current { background: var(--color-primary); }
	.history-time { font-size: 12px; font-weight: 600; color: var(--text); }
	.history-summary { font-size: 11px; color: var(--text-muted); margin-top: 1px; }
	.history-item-right {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.history-score { font-size: 14px; font-weight: 700; }
	.history-arrow { width: 14px; height: 14px; color: var(--text-muted); }
	.history-empty { padding: 20px; text-align: center; font-size: 12px; color: var(--text-muted); }

	/* ── Responsive ── */
	@media (max-width: 640px) {
		.score-grid { grid-template-columns: 1fr; }
		.header-card { flex-direction: column; }
		.header-right { width: 100%; justify-content: flex-end; }
		.switcher-bar { flex-direction: column; }
		.switcher-group { min-width: unset; }
	}
</style>
