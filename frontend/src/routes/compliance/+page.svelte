<script>
	import { onMount } from 'svelte';
import { api } from '$lib/api.svelte.js';
import Icon from '@iconify/svelte';
import { goto } from '$app/navigation';

const scanProfilePages = {
	cis_l1: '/compliance/cis-level-1',
	cis_l2: '/compliance/cis-level-2',
	cis_docker: '/compliance/cis-docker',
	lynis: '/compliance/lynis',
};

	// ─── Global State ───
	let servers = $state([]);
	let summary = $state(null);
	let loading = $state(true);
	let error = $state('');
	let scanning = $state({});
	let lynisScanning = $state({});
	let availableChecks = $state([]);
	let checkStats = $state({ total: 0, cis_l1: 0, cis_l2: 0 });

	// ─── Filters ───
	let filterStatus = $state('all');
	let expandedServers = $state({});

	// ─── Category breakdowns ───
	let l1Categories = $state([]);
	let l2Categories = $state([]);
	let dockerCategories = $state([]);
	let l1CategoriesLoading = $state(false);
	let l2CategoriesLoading = $state(false);
	let dockerCategoriesLoading = $state(false);

	// ─── Docker scan ───

	// ─── Global scan history ───
	let globalHistory = $state([]);
	let historyLoading = $state(false);
	let historyFilter = $state('all');
	let historyPage = $state(1);
	let historyTotal = $state(0);
	let historyLimit = 10;

	// ─── On mount ───
	onMount(async () => {
		await Promise.all([loadSummary(), loadCheckInfo()]);
		loadGlobalHistory();
	});

	async function loadGlobalHistory(scanType = null, pg = 1) {
		historyLoading = true;
		historyPage = pg;
		try {
			const params = { page: historyPage, limit: historyLimit };
			if (scanType && scanType !== 'all') params.scan_type = scanType;
			const data = await api.compliance.globalHistory(params);
			globalHistory = data.results || [];
			historyTotal = data.total || 0;
		} catch (_) {
			globalHistory = [];
			historyTotal = 0;
		} finally {
			historyLoading = false;
		}
	}

	function filterHistory(type) {
		historyFilter = type;
		loadGlobalHistory(type === 'all' ? null : type, 1);
	}

	async function loadCheckInfo() {
		try {
			const data = await api.compliance.checks();
			availableChecks = data.checks || [];
			const l1 = data.checks?.filter(c => c.cis_level !== 2).length || 0;
			const l2 = data.checks?.filter(c => c.cis_level === 2).length || 0;
			checkStats = { total: data.total || data.checks?.length || 0, cis_l1: l1, cis_l2: l2 };
		} catch (_) {}
	}

	async function loadSummary() {
		loading = true;
		error = '';
		try {
			const data = await api.compliance.summary();
			summary = {
				total_servers: data.total_servers || 0,
				scanned_servers: data.scanned_servers || 0,
				average_score: data.average_score || 0,
				passing: (data.by_status && data.by_status['good']) || 0,
				warning: (data.by_status && data.by_status['warning']) || 0,
				critical: (data.by_status && data.by_status['critical']) || 0,
				unscanned: (data.by_status && data.by_status['unscanned']) || 0,
				top_findings: data.top_findings || [],
			};
			servers = data.servers || [];
			loadCategoryBreakdowns();
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadCategoryBreakdowns() {
		const scanned = servers.filter(s => s.last_scan);
		if (scanned.length === 0) return;

		const srv = scanned[0];
		l1CategoriesLoading = true;
		l2CategoriesLoading = true;

		try {
			const l1Data = await api.compliance.latestCategories(srv.id);
			if (l1Data.categories) l1Categories = l1Data.categories;
		} catch (_) {}
		l1CategoriesLoading = false;

		try {
			const l2Data = await api.compliance.latestCategories(srv.id);
			if (l2Data.categories) l2Categories = l2Data.categories.filter(c => c.total > 0);
		} catch (_) {}
		l2CategoriesLoading = false;

		try {
			const dockerData = await api.compliance.latestCategories(srv.id, { scan_type: 'CIS Docker' });
			if (dockerData.categories) dockerCategories = dockerData.categories.filter(c => c.total > 0);
		} catch (_) {}
		dockerCategoriesLoading = false;
	}

	async function scanAll() {
		const unscanned = servers.filter(s => !s.last_scan);
		for (const s of unscanned) {
			try { await api.compliance.scan(s.id); } catch (_) {}
		}
		await loadSummary();
	}

	function toggleServer(id) {
		expandedServers = { ...expandedServers, [id]: !expandedServers[id] };
	}

	// ─── Helpers ───
	function formatTime(ts) {
		if (!ts) return 'Never';
		const d = new Date(ts);
		const now = new Date();
		const diff = now - d;
		if (diff < 60000) return 'Just now';
		if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
		if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
	}

	function scoreColor(score) {
		if (score === undefined || score === null) return 'var(--color-text-muted)';
		if (score >= 80) return 'var(--color-success)';
		if (score >= 60) return 'var(--color-warning)';
		return 'var(--color-danger)';
	}

	function scoreLabel(score) {
		if (score === undefined || score === null) return 'Unscanned';
		if (score >= 80) return 'Passing';
		if (score >= 60) return 'Warning';
		return 'Critical';
	}

	function severityColor(severity) {
		if (severity === 'critical') return 'var(--color-danger)';
		if (severity === 'high' || severity === 'warning') return 'var(--color-warning)';
		if (severity === 'medium') return 'var(--color-accent)';
		return 'var(--color-success)';
	}

	// ─── Derived ───
	let totalCritical = $derived(servers.reduce((sum, s) => sum + (s.criticals || 0), 0));
	let totalWarnings = $derived(servers.reduce((sum, s) => sum + (s.warnings || 0), 0));
	let totalPassed = $derived(servers.reduce((sum, s) => sum + (s.passed || 0), 0));
	let totalFindings = $derived(totalCritical + totalWarnings + totalPassed);

	let filteredServers = $derived.by(() => {
		let list = servers;
		if (filterStatus !== 'all') {
			list = list.filter(s => {
				const label = scoreLabel(s.score);
				if (filterStatus === 'passing') return label === 'Passing';
				if (filterStatus === 'warning') return label === 'Warning';
				if (filterStatus === 'critical') return label === 'Critical';
				if (filterStatus === 'unscanned') return s.score === undefined || s.score === null;
				return true;
			});
		}
		return list;
	});

	let scannedCount = $derived(servers.filter(s => s.last_scan).length);

	let profileScore = $derived.by(() => {
		const cats = [...l1Categories, ...l2Categories];
		if (cats.length === 0) return 0;
		const total = cats.reduce((s, c) => s + c.total, 0);
		const passed = cats.reduce((s, c) => s + c.passed, 0);
		return total > 0 ? Math.round((passed / total) * 100) : 0;
	});

	let dockerScore = $derived.by(() => {
		if (dockerCategories.length === 0) return null;
		const total = dockerCategories.reduce((s, c) => s + c.total, 0);
		const passed = dockerCategories.reduce((s, c) => s + c.passed, 0);
		return total > 0 ? Math.round((passed / total) * 100) : null;
	});
</script>

<div class="page-container">

	<!-- HEADER -->
	<div class="flex items-start sm:items-center justify-between gap-3 flex-wrap mb-4">
		<div class="min-w-0 flex-1">
			<h1 class="page-title">Compliance</h1>
			<p class="page-subtitle">Security posture across all servers and containers</p>
		</div>
		<div class="flex items-center gap-2">
			{#if checkStats.total > 0}
				<div class="text-xs hidden sm:flex items-center gap-1.5 px-3 py-1.5 rounded-full"
					style="background-color: var(--color-surface); border: 1px solid var(--color-border-light);">
					<Icon icon="solar:list-check-bold" class="h-3 w-3" style="color: var(--color-primary);" />
					<span style="color: var(--color-text-muted);">
						{checkStats.total} checks · <span style="color: var(--color-success);">{checkStats.cis_l1} L1</span> · <span style="color: var(--color-warning);">{checkStats.cis_l2} L2</span>
					</span>
				</div>
			{/if}
			<button onclick={scanAll} class="btn-secondary flex items-center gap-2 shrink-0">
				<Icon icon="solar:refresh-bold" class="h-4 w-4" />
				<span>Scan All</span>
			</button>
		</div>
	</div>

	<!-- LOADING / ERROR -->
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading compliance data...</p>
			</div>
		</div>
	{:else if error}
		<div class="card flex flex-col items-center gap-3 py-10 text-center">
			<Icon icon="solar:danger-triangle-bold" class="mb-1 h-8 w-8" style="color: var(--color-danger);" />
			<p style="color: var(--color-danger);">Failed to load compliance data</p>
			<p class="text-sm" style="color: var(--color-text-secondary);">{error}</p>
			<button onclick={loadSummary} class="btn-secondary mt-2">Retry</button>
		</div>
	{:else}

		<!-- KPI CARDS -->
		<div class="grid gap-4 grid-cols-2 lg:grid-cols-3 mb-6">
			<div class="stat-card" style="border-left-color: var(--color-primary);">
				<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-text-muted);">
					<Icon icon="solar:server-square-bold" class="h-3.5 w-3.5" />
					Total Servers
				</div>
				<p class="text-2xl font-bold" style="color: var(--color-text);">{summary?.total_servers || 0}</p>
				<p class="mt-1 text-xs" style="color: var(--color-text-muted);">
					<span style="color: var(--color-success);">{scannedCount} scanned</span>
					{summary?.unscanned > 0 ? ' · ' + summary.unscanned + ' pending' : ''}
				</p>
			</div>
			<div class="stat-card" style="border-left-color: #8b5cf6;">
				<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-text-muted);">
					<Icon icon="solar:shield-check-bold" class="h-3.5 w-3.5" />
					Compliance Score
				</div>
				<p class="text-2xl font-bold" style="color: {scannedCount > 0 ? scoreColor(summary?.average_score) : 'var(--color-text-muted)'};">
					{scannedCount > 0 ? summary?.average_score + '%' : '—'}
				</p>
				<p class="mt-1 text-xs" style="color: var(--color-text-muted);">
					Across all benchmarks
				</p>
			</div>
			<div class="stat-card" style="border-left-color: var(--color-warning);">
				<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-text-muted);">
					<Icon icon="solar:chart-2-bold" class="h-3.5 w-3.5" />
					Vulnerabilities
				</div>
				<div class="flex items-center gap-3 mb-1.5">
					{#if totalCritical > 0}
						<span class="text-lg font-bold" style="color: var(--color-danger);">{totalCritical}</span>
						<span class="text-xs" style="color: var(--color-text-muted);">critical</span>
					{/if}
					{#if totalWarnings > 0}
						<span class="text-lg font-bold" style="color: var(--color-warning);">{totalWarnings}</span>
						<span class="text-xs" style="color: var(--color-text-muted);">high</span>
					{/if}
					{#if !totalCritical && !totalWarnings}
						<span class="text-sm" style="color: var(--color-text-muted);">—</span>
					{/if}
				</div>
				<div class="mt-1 h-1.5 rounded-full overflow-hidden flex" style="background: var(--color-border);">
					{#if totalFindings > 0}
						<div class="h-full transition-all" style="width: {totalFindings > 0 ? (totalPassed / totalFindings) * 100 + '%' : '0%'}; background: var(--color-success);"></div>
						<div class="h-full transition-all" style="width: {totalFindings > 0 ? (totalWarnings / totalFindings) * 100 + '%' : '0%'}; background: var(--color-warning);"></div>
						<div class="h-full transition-all" style="width: {totalFindings > 0 ? (totalCritical / totalFindings) * 100 + '%' : '0%'}; background: var(--color-danger);"></div>
					{/if}
				</div>
			</div>
		</div>

<!-- BENCHMARK CARDS — sesuai mockup -->
			<div class="flex items-center gap-2 mb-3">
				<h3 class="text-sm font-semibold" style="color: var(--color-text);">Benchmarks</h3>
				<span class="text-xs" style="color: var(--color-text-muted);">Aggregate scores across all scanned targets</span>
			</div>
			<div class="grid gap-3 grid-cols-1 md:grid-cols-4 mb-6">
			<!-- CIS L1 Card → clickable to detail page -->
			<div class="card !p-4 clickable-card" style="border-left: 3px solid var(--color-success);"
				role="button" tabindex="0"
				onclick={() => goto('/compliance/cis-level-1')}
				onkeydown={(e) => e.key === 'Enter' && goto('/compliance/cis-level-1')}>
				<div class="flex items-center justify-between mb-3">
					<div class="flex items-center gap-2.5">
						<div class="w-9 h-9 rounded-lg flex items-center justify-center text-sm" style="background: rgba(16,185,129,0.12);">
							<Icon icon="solar:shield-check-bold" class="h-5 w-5" style="color: var(--color-success);" />
						</div>
						<div>
							<div class="text-sm font-semibold" style="color: var(--color-text);">CIS Level 1</div>
							<div class="text-xs" style="color: var(--color-text-muted);">Server hardening</div>
						</div>
					</div>
					<div class="text-right">
						<div class="text-lg font-bold" style="color: var(--color-success);">{scannedCount > 0 ? profileScore : '—'}{scannedCount > 0 ? '%' : ''}</div>
						<div class="text-[10px]" style="color: var(--color-text-muted);">avg score</div>
					</div>
				</div>
				<div class="flex items-center gap-2 text-xs mb-2" style="color: var(--color-text-muted);">
					<span><strong style="color: var(--color-text);">{checkStats.cis_l1}</strong> checks</span>
					<span>·</span>
					<span><strong style="color: var(--color-success);">{l1Categories.reduce((s,c) => s + c.passed, 0)}</strong> pass</span>
					<span><strong style="color: var(--color-warning);">{l1Categories.reduce((s,c) => s + c.warnings, 0)}</strong> warn</span>
					<span><strong style="color: var(--color-danger);">{l1Categories.reduce((s,c) => s + c.criticals, 0)}</strong> fail</span>
				</div>
				<div class="progress-track">
					<div class="progress-fill" style="width: {profileScore}%; background: var(--color-success);"></div>
				</div>
				<div class="flex items-center justify-between mt-2">
					<span class="text-[11px]" style="color: var(--color-text-muted);">{l1Categories.length || 7} categories · {scannedCount} server{scannedCount !== 1 ? 's' : ''}</span>
					<Icon icon="solar:alt-arrow-right-bold" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
				</div>
			</div>

			<!-- CIS L2 Card -->
			<div class="card !p-4 clickable-card" style="border-left: 3px solid var(--color-warning);"
				role="button" tabindex="0"
				onclick={() => goto('/compliance/cis-level-2')}
				onkeydown={(e) => e.key === 'Enter' && goto('/compliance/cis-level-2')}>
				<div class="flex items-center justify-between mb-3">
					<div class="flex items-center gap-2.5">
						<div class="w-9 h-9 rounded-lg flex items-center justify-center text-sm" style="background: rgba(245,158,11,0.12);">
							<Icon icon="solar:lock-keyhole-bold" class="h-5 w-5" style="color: var(--color-warning);" />
						</div>
						<div>
							<div class="text-sm font-semibold" style="color: var(--color-text);">CIS Level 2</div>
							<div class="text-xs" style="color: var(--color-text-muted);">Advanced hardening</div>
						</div>
					</div>
					<div class="text-right">
						<div class="text-lg font-bold" style="color: var(--color-warning);">{scannedCount > 0 ? profileScore : '—'}{scannedCount > 0 ? '%' : ''}</div>
						<div class="text-[10px]" style="color: var(--color-text-muted);">avg score</div>
					</div>
				</div>
				<div class="flex items-center gap-2 text-xs mb-2" style="color: var(--color-text-muted);">
					<span><strong style="color: var(--color-text);">{checkStats.total}</strong> checks</span>
					<span>·</span>
					<span><strong style="color: var(--color-success);">{l2Categories.reduce((s,c) => s + c.passed, 0)}</strong> pass</span>
					<span><strong style="color: var(--color-warning);">{l2Categories.reduce((s,c) => s + c.warnings, 0)}</strong> warn</span>
					<span><strong style="color: var(--color-danger);">{l2Categories.reduce((s,c) => s + c.criticals, 0)}</strong> fail</span>
				</div>
				<div class="progress-track">
					<div class="progress-fill" style="width: {profileScore}%; background: var(--color-warning);"></div>
				</div>
				<div class="flex items-center justify-between mt-2">
					<span class="text-[11px]" style="color: var(--color-text-muted);">{l2Categories.length || 10} categories · {scannedCount} server{scannedCount !== 1 ? 's' : ''}</span>
					<Icon icon="solar:alt-arrow-right-bold" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
				</div>
			</div>

			<!-- Lynis Card -->
			<div class="card !p-4 clickable-card" style="border-left: 3px solid var(--color-accent);"
				role="button" tabindex="0"
				onclick={() => goto('/compliance/lynis')}
				onkeydown={(e) => e.key === 'Enter' && goto('/compliance/lynis')}>
				<div class="flex items-center justify-between mb-3">
					<div class="flex items-center gap-2.5">
						<div class="w-9 h-9 rounded-lg flex items-center justify-center text-sm" style="background: rgba(139,92,246,0.12);">
							<Icon icon="solar:info-circle-bold" class="h-5 w-5" style="color: var(--color-accent);" />
						</div>
						<div>
							<div class="text-sm font-semibold" style="color: var(--color-text);">Lynis Audit</div>
							<div class="text-xs" style="color: var(--color-text-muted);">System audit</div>
						</div>
					</div>
					<div class="text-right">
						<div class="text-lg font-bold" style="color: var(--color-accent);">—</div>
						<div class="text-[10px]" style="color: var(--color-text-muted);">hardening idx</div>
					</div>
				</div>
				<div class="flex items-center gap-2 text-xs mb-2" style="color: var(--color-text-muted);">
					<span><strong style="color: var(--color-text);">12</strong> categories</span>
					<span>·</span>
					<span style="color: var(--color-warning);"><strong>8</strong> warnings</span>
					<span style="color: var(--color-accent);"><strong>23</strong> suggestions</span>
				</div>
				<div class="progress-track">
					<div class="progress-fill" style="width: 72%; background: var(--color-accent);"></div>
			</div>
			<div class="flex items-center justify-between mt-2">
				<span class="text-[11px]" style="color: var(--color-text-muted);">12 categories · {scannedCount} server{scannedCount !== 1 ? 's' : ''}</span>
				<Icon icon="solar:alt-arrow-right-bold" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
			</div>
		</div>

		<!-- CIS Docker Card -->
		<div class="card !p-4 clickable-card" style="border-left: 3px solid #06b6d4;"
			role="button" tabindex="0"
			onclick={() => goto('/compliance/cis-docker')}
			onkeydown={(e) => e.key === 'Enter' && goto('/compliance/cis-docker')}>
			<div class="flex items-center justify-between mb-3">
				<div class="flex items-center gap-2.5">
					<div class="w-9 h-9 rounded-lg flex items-center justify-center text-sm" style="background: rgba(6,182,212,0.12);">
						<Icon icon="solar:box-bold" class="h-5 w-5" style="color: #06b6d4;" />
					</div>
					<div>
						<div class="text-sm font-semibold" style="color: var(--color-text);">CIS Docker</div>
						<div class="text-xs" style="color: var(--color-text-muted);">Docker host + container security</div>
					</div>
				</div>
				<div class="text-right">
					{#if dockerScore !== null}
						<div class="text-lg font-bold" style="color: {dockerScore >= 80 ? 'var(--color-success)' : dockerScore >= 60 ? 'var(--color-warning)' : 'var(--color-danger)'};">{dockerScore}%</div>
					{:else}
						<div class="text-lg font-bold" style="color: #06b6d4;">—</div>
					{/if}
					<div class="text-[10px]" style="color: var(--color-text-muted);">avg score</div>
				</div>
			</div>
			<div class="flex items-center gap-2 text-xs mb-2" style="color: var(--color-text-muted);">
				<span><strong style="color: var(--color-text);">{checkStats.total}</strong> checks</span>
				<span>·</span>
				<span><strong style="color: var(--color-success);">{dockerCategories.reduce((s, c) => s + c.passed, 0)}</strong> pass</span>
				<span><strong style="color: var(--color-warning);">{dockerCategories.reduce((s, c) => s + c.warnings, 0)}</strong> warn</span>
				<span><strong style="color: var(--color-danger);">{dockerCategories.reduce((s, c) => s + c.criticals, 0)}</strong> fail</span>
			</div>
			<div class="progress-track">
				<div class="progress-fill" style="width: {dockerScore ?? 0}%; background: #06b6d4;"></div>
			</div>
			<div class="flex items-center justify-between mt-2">
				<span class="text-[11px]" style="color: var(--color-text-muted);">
					{dockerCategories.length > 0 ? dockerCategories.length + ' scanned' : '6 categories'} · {scannedCount} server{scannedCount !== 1 ? 's' : ''}
				</span>
				<Icon icon="solar:alt-arrow-right-bold" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
			</div>
		</div>
	</div>

		<!-- STATUS FILTER -->
		<div class="flex flex-wrap items-center gap-2 mb-4">
			<span class="text-xs font-medium" style="color: var(--color-text-muted);">Filter:</span>
			<button onclick={() => filterStatus = 'all'} class="filter-chip" class:filter-active={filterStatus === 'all'}>All</button>
			<button onclick={() => filterStatus = 'passing'} class="filter-chip" class:filter-active={filterStatus === 'passing'}>🟢 Passing</button>
			<button onclick={() => filterStatus = 'warning'} class="filter-chip" class:filter-active={filterStatus === 'warning'}>🟡 Warning</button>
			<button onclick={() => filterStatus = 'critical'} class="filter-chip" class:filter-active={filterStatus === 'critical'}>🔴 Critical</button>
			<button onclick={() => filterStatus = 'unscanned'} class="filter-chip" class:filter-active={filterStatus === 'unscanned'}>⏳ Unscanned</button>
			{#if filteredServers.length !== servers.length}
				<span class="text-xs ml-2" style="color: var(--color-text-muted);">
					Showing {filteredServers.length} of {servers.length}
				</span>
			{/if}
		</div>

		<!-- SERVER LIST — expandable score cards -->
		{#if servers.length === 0}
			<div class="card flex flex-col items-center py-16 text-center">
				<Icon icon="solar:shield-check-bold" class="mb-3 h-12 w-12" style="color: var(--color-text-muted);" />
				<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No servers</h3>
				<p class="text-sm" style="color: var(--color-text-secondary);">Add servers to start monitoring compliance.</p>
			</div>
		{:else}
			<div class="flex flex-col gap-3">
				{#each filteredServers as srv (srv.id)}
					{@const sc = srv.score}
					{@const sColor = sc !== undefined && sc !== null ? scoreColor(sc) : 'var(--color-text-muted)'}
					{@const isExpanded = expandedServers[srv.id] || false}
					<div class="server-card" style="border-left: 4px solid {sc !== undefined && sc !== null ? sColor : 'var(--color-border)'};">
						<!-- Main row — always visible -->
						<div class="flex items-center gap-3 cursor-pointer" onclick={() => toggleServer(srv.id)} role="button" tabindex="0"
							onkeydown={(e) => e.key === 'Enter' && toggleServer(srv.id)}>
							<!-- Score badge -->
							<div class="flex items-center justify-center rounded-xl shrink-0"
								style="width: 48px; height: 48px; background-color: {sc !== undefined && sc !== null ? sColor + '18' : 'var(--color-border)'};">
								{#if sc !== undefined && sc !== null}
									<span class="text-xl font-extrabold" style="color: {sColor};">{sc}</span>
								{:else}
									<Icon icon="solar:server-square-bold" class="h-5 w-5" style="color: var(--color-text-muted);" />
								{/if}
							</div>
							<!-- Server info -->
							<div class="min-w-0 flex-1">
								<p class="text-sm font-semibold truncate" style="color: var(--color-text);">{srv.name || 'Unknown'}</p>
								<div class="flex items-center gap-2 mt-0.5">
									{#if srv.host}<span class="text-xs font-mono" style="color: var(--color-text-muted);">{srv.host}</span>{/if}
									<span class="text-[10px] px-1.5 py-0.5 rounded-full font-medium" style="background: {sc !== undefined && sc !== null ? sColor + '20' : 'var(--color-border)'}; color: {sc !== undefined && sc !== null ? sColor : 'var(--color-text-muted)'};">
										{scoreLabel(sc)}
									</span>
								</div>
							</div>
							<!-- Severity dots -->
							<div class="flex items-center gap-2 text-xs shrink-0">
								{#if srv.criticals > 0}<span class="flex items-center gap-0.5" style="color: var(--color-danger); font-weight: 600;">🔴 {srv.criticals}</span>{/if}
								{#if srv.warnings > 0}<span class="flex items-center gap-0.5" style="color: var(--color-warning); font-weight: 600;">🟡 {srv.warnings}</span>{/if}
							</div>
							<!-- Expand chevron -->
							<Icon icon="solar:alt-arrow-down-bold" class="h-4 w-4 shrink-0 transition-transform duration-200" 
								style="color: var(--color-text-muted); transform: {isExpanded ? 'rotate(180deg)' : 'rotate(0)'};" />
						</div>

						<!-- Expanded section -->
						{#if isExpanded}
							<div class="expanded-section" style="border-top: 1px solid var(--color-border-light); margin-top: 12px; padding-top: 12px;">
								<div class="grid gap-2" style="grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));">
									<!-- CIS L1 -->
									<div class="flex items-center gap-2 text-xs rounded-lg px-3 py-2" style="background: rgba(16,185,129,0.06);">
										<span style="color: var(--color-success); font-weight: 600;">CIS L1</span>
										<span style="color: var(--color-text-muted);">{l1Categories.length} cat</span>
									</div>
									<!-- CIS L2 -->
									<div class="flex items-center gap-2 text-xs rounded-lg px-3 py-2" style="background: rgba(245,158,11,0.06);">
										<span style="color: var(--color-warning); font-weight: 600;">CIS L2</span>
										<span style="color: var(--color-text-muted);">{l2Categories.length} cat</span>
									</div>
									<!-- CIS Docker -->
									<div class="flex items-center gap-2 text-xs rounded-lg px-3 py-2" style="background: rgba(6,182,212,0.06);">
										<span style="color: #06b6d4; font-weight: 600;">Docker</span>
										<span style="color: var(--color-text-muted);">{dockerCategories.length} cat</span>
									</div>
									<!-- Last scan -->
									<div class="flex items-center gap-2 text-xs rounded-lg px-3 py-2" style="background: var(--color-surface);">
										<Icon icon="solar:history-bold" class="h-3 w-3" style="color: var(--color-text-muted);" />
										<span style="color: var(--color-text-muted);">{formatTime(srv.last_scan)}</span>
									</div>
								</div>
								<!-- Quick actions -->
								<div class="flex items-center gap-2 mt-3">
									<button onclick={(e) => { e.stopPropagation(); goto(`/servers/${srv.id}?tab=compliance`); }}
										class="text-xs font-medium px-3 py-1.5 rounded-lg flex items-center gap-1.5"
										style="background: var(--color-primary); color: #fff; border: none; cursor: pointer;">
										<Icon icon="solar:shield-check-bold" class="h-3.5 w-3.5" /> View Details
									</button>
								</div>
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}

		<!-- SCAN HISTORY -->
		<div class="mt-8">
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-base font-semibold flex items-center gap-2" style="color: var(--color-text);">
					<Icon icon="solar:history-bold" class="h-4 w-4" style="color: var(--color-text-muted);" /> Scan History
				</h3>
			</div>
			<!-- Filter chips -->
			<div class="flex flex-wrap items-center gap-2 mb-4">
				<button onclick={() => filterHistory('all')} class="filter-chip" class:filter-active={historyFilter === 'all'}>All</button>
				<button onclick={() => filterHistory('CIS Level 1')} class="filter-chip" class:filter-active={historyFilter === 'CIS Level 1'}>CIS L1</button>
				<button onclick={() => filterHistory('CIS Level 2')} class="filter-chip" class:filter-active={historyFilter === 'CIS Level 2'}>CIS L2</button>
				<button onclick={() => filterHistory('Lynis')} class="filter-chip" class:filter-active={historyFilter === 'Lynis'}>Lynis</button>
				<button onclick={() => filterHistory('CIS Docker')} class="filter-chip" class:filter-active={historyFilter === 'CIS Docker'}>Docker</button>
			</div>
			<!-- History rows -->
			{#if historyLoading}
				<div class="flex items-center justify-center py-8">
					<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-text-muted);" />
				</div>
			{:else if globalHistory.length > 0}
				<div class="flex flex-col gap-2">
					{#each globalHistory as item}
						<div class="card !p-3 flex items-center justify-between">
							<div class="flex items-center gap-3 min-w-0">
								<div class="w-2 h-2 rounded-full shrink-0" style="background: {scoreColor(item.score)};"></div>
								<div class="min-w-0">
									<div class="flex items-center gap-2 flex-wrap">
										<span class="text-xs font-medium" style="color: var(--color-text);">{item.scan_type || item.profile || 'Scan'}</span>
										<span class="text-[10px] px-1.5 py-0.5 rounded-full font-medium" style="background: {scoreColor(item.score)}22; color: {scoreColor(item.score)};">
											{item.score ?? '—'}%
										</span>
									</div>
									<p class="text-[10px] mt-0.5" style="color: var(--color-text-muted);">
										{item.server_name || item.server_id} · {formatTime(item.created_at || item.scanned_at)}
									</p>
								</div>
							</div>
							<div class="flex items-center gap-3 text-xs shrink-0">
								<span>✓{item.passed_count || item.passed || 0}</span>
								<span style="color: var(--color-warning);">⚠{item.warning_count || item.warnings || 0}</span>
								<span style="color: var(--color-danger);">✗{item.critical_count || item.criticals || 0}</span>
								<button onclick={() => goto(`/servers/${item.server_id}?tab=compliance&scan=${item.id}`)}
									class="text-[10px] font-medium px-2 py-1 rounded"
									style="background: rgba(148,163,184,0.12); color: var(--color-text-muted); border: none; cursor: pointer;">
									View →
								</button>
							</div>
						</div>
					{/each}
				</div>
				<!-- Pagination -->
				{#if historyTotal > historyLimit}
					<div class="flex items-center justify-center gap-3 mt-3">
						<button onclick={() => loadGlobalHistory(historyFilter === 'all' ? null : historyFilter, historyPage - 1)}
							disabled={historyPage <= 1}
							class="text-xs font-medium px-3 py-1.5 rounded"
							style="background: var(--color-surface); color: {historyPage <= 1 ? 'var(--color-border)' : 'var(--color-text)'}; border: 1px solid var(--color-border-light); cursor: {historyPage <= 1 ? 'default' : 'pointer'};">
							← Prev
						</button>
						<span class="text-xs" style="color: var(--color-text-muted);">Page {historyPage} of {Math.ceil(historyTotal / historyLimit)}</span>
						<button onclick={() => loadGlobalHistory(historyFilter === 'all' ? null : historyFilter, historyPage + 1)}
							disabled={historyPage * historyLimit >= historyTotal}
							class="text-xs font-medium px-3 py-1.5 rounded"
							style="background: var(--color-surface); color: {historyPage * historyLimit >= historyTotal ? 'var(--color-border)' : 'var(--color-text)'}; border: 1px solid var(--color-border-light); cursor: {historyPage * historyLimit >= historyTotal ? 'default' : 'pointer'};">
							Next →
						</button>
					</div>
				{/if}
			{:else}
				<div class="card flex flex-col items-center py-10 text-center">
					<Icon icon="solar:history-bold" class="h-10 w-10 mb-3" style="color: var(--color-text-muted);" />
					<p class="text-sm" style="color: var(--color-text-muted);">No scan history yet.</p>
					<p class="text-xs mt-1" style="color: var(--color-text-muted);">Run a scan from any server to see results here.</p>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.server-card {
		border-radius: 10px;
		background: var(--color-card);
		border: 1px solid var(--color-border-light);
		transition: all 0.15s;
		padding: 14px 16px;
	}
	.server-card:hover {
		border-color: var(--color-primary);
		box-shadow: 0 2px 8px rgba(16,185,129,0.08);
	}
	.filter-chip {
		padding: 4px 12px;
		border-radius: 999px;
		font-size: 12px;
		font-weight: 500;
		border: 1px solid var(--color-border);
		background: transparent;
		color: var(--color-text-secondary);
		cursor: pointer;
		transition: all 0.15s;
	}
	.filter-chip:hover { border-color: var(--color-primary); color: var(--color-primary); }
	.filter-active { background: var(--color-primary); color: #fff; border-color: var(--color-primary) !important; }
	.filter-active:hover { color: #fff; }
	.clickable-card { cursor: pointer; transition: all 0.15s; }
	.clickable-card:hover { border-color: var(--color-primary) !important; box-shadow: 0 2px 8px rgba(16,185,129,0.08); }
</style>
