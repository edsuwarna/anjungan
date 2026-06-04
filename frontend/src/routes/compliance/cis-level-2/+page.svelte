<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';
	import { goto } from '$app/navigation';

	let checks = $state([]);
	let loading = $state(true);
	let error = $state('');

	// ── Scan History ──
	let history = $state([]);
	let historyLoading = $state(false);
	let historyPage = $state(1);
	let historyTotal = $state(0);
	const historyLimit = 10;

	const profileColor = '#f59e0b';
	const profileInfo = {
		icon: 'solar:lock-keyhole-bold',
		title: 'CIS Level 2',
		subtitle: 'Advanced security hardening — 92 checks across 10 categories',
		desc: 'CIS Level 2 extends Level 1 with additional controls for advanced security hardening environments requiring higher assurance. Includes extra services, Docker, and package audits.',
	};

		const categoryMeta = {
		ssh: { label: 'SSH', icon: '🔒', color: '#3b82f6', checks: 7 },
		filesystem: { label: 'Filesystem', icon: '📁', color: '#f59e0b', checks: 2 },
		users: { label: 'Users & Groups', icon: '👥', color: '#fb923c', checks: 1 },
		services: { label: 'Services', icon: '⚡', color: '#ef4444', checks: 2 },
		network: { label: 'Network', icon: '🌐', color: '#10b981', checks: 2 },
		logging: { label: 'Logging', icon: '📋', color: '#c084fc', checks: 1 },
		docker: { label: 'Docker', icon: '📦', color: '#06b6d4', checks: 1 },
	};


	let categories = $derived.by(() => {
		const groups = {};
		for (const ch of checks) {
			if (!groups[ch.category]) groups[ch.category] = [];
			groups[ch.category].push(ch);
		}
		return Object.entries(categoryMeta).map(([key, meta]) => ({
			key,
			...meta,
			items: groups[key] || [],
		})).filter(c => c.items.length > 0);
	});

	let totalChecks = $derived(checks.length);
	let severeCount = $derived(checks.filter(c => c.severity === 'critical' || c.severity === 'high').length);

	let totalPages = $derived(Math.max(1, Math.ceil(historyTotal / historyLimit)));

	onMount(async () => {
		loading = true;
		try {
			const data = await api.compliance.checks();
			checks = (data.checks || []).filter(c => c.cis_level === 2);
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
		loadHistory();
	});

	async function loadHistory(pg) {
		if (pg !== undefined) historyPage = pg;
		historyLoading = true;
		try {
			const resp = await api.compliance.globalHistory({ scan_type: 'CIS Level 2', page: historyPage, limit: historyLimit });
			history = resp.results || resp.history || [];
			if (Array.isArray(resp)) history = resp;
			historyTotal = resp.total || resp.count || 0;
		} catch {
			history = [];
			historyTotal = 0;
		} finally {
			historyLoading = false;
		}
	}

	function severityIcon(sev) {
		const s = (sev || '').toLowerCase();
		if (s === 'critical') return 'solar:danger-triangle-bold';
		if (s === 'high') return 'solar:alert-triangle-bold';
		if (s === 'medium') return 'solar:info-circle-bold';
		return 'solar:check-circle-bold';
	}

	function severityColor(sev) {
		const s = (sev || '').toLowerCase();
		if (s === 'critical') return 'var(--color-danger)';
		if (s === 'high') return 'var(--color-warning)';
		if (s === 'medium') return 'var(--color-accent)';
		return 'var(--color-success)';
	}

	function severityBg(sev) {
		const s = (sev || '').toLowerCase();
		const colors = {
			critical: 'rgba(239,68,68,0.1)',
			high: 'rgba(245,158,11,0.1)',
			medium: 'rgba(99,102,241,0.1)',
			low: 'rgba(16,185,129,0.1)',
		};
		return colors[s] || 'rgba(100,116,139,0.1)';
	}

	function scoreColor(score) {
		if (score >= 80) return 'var(--color-success)';
		if (score >= 60) return 'var(--color-warning)';
		return 'var(--color-danger)';
	}

	function formatTime(ts) {
		if (!ts) return '—';
		const d = new Date(ts);
		const now = new Date();
		const diff = now - d;
		if (diff < 60000) return 'Just now';
		if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
		if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
	}

	const statusLabel = { completed: '✅ Done', running: '🔄 Running', failed: '❌ Failed', pending: '⏳ Pending' };
</script>

<div class="page-container">

	<!-- Breadcrumb -->
	<div class="flex items-center gap-2 text-sm mb-4" style="color: var(--color-text-muted);">
		<a href="/compliance" class="hover:underline" style="color: var(--color-text-muted);">Compliance</a>
		<span>/</span>
		<span style="color: var(--color-text);">CIS Level 2</span>
	</div>

	<!-- Hero -->
	<div class="card !p-5 mb-5" style="border-left: 4px solid {profileColor};">
		<div class="flex items-start gap-4 flex-wrap">
			<div class="w-12 h-12 rounded-xl flex items-center justify-center shrink-0" style="background: rgba(245,158,11,0.12);">
				<Icon icon={profileInfo.icon} class="h-6 w-6" style="color: {profileColor};" />
			</div>
			<div class="flex-1 min-w-0">
				<h1 class="text-lg font-bold" style="color: var(--color-text);">{profileInfo.title}</h1>
				<p class="text-sm mt-0.5" style="color: var(--color-text-secondary);">{profileInfo.desc}</p>
				<div class="flex flex-wrap items-center gap-3 mt-2">
					<span class="text-xs px-2.5 py-1 rounded-full font-medium" style="background: rgba(245,158,11,0.12); color: {profileColor};">
						{totalChecks} checks
					</span>
					<span class="text-xs px-2.5 py-1 rounded-full font-medium" style="background: rgba(59,130,246,0.12); color: #3b82f6;">
						{categories.length} categories
					</span>
					{#if severeCount > 0}
						<span class="text-xs px-2.5 py-1 rounded-full font-medium" style="background: rgba(239,68,68,0.1); color: var(--color-danger);">
							{severeCount} critical/high severity
						</span>
					{/if}
				</div>
			</div>
			<button onclick={() => goto('/compliance')} class="btn-secondary flex items-center gap-1.5 shrink-0 text-xs">
				<Icon icon="solar:arrow-left-bold" class="h-3.5 w-3.5" /> Back
			</button>
		</div>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading check definitions...</p>
			</div>
		</div>
	{:else if error}
		<div class="card flex flex-col items-center gap-3 py-10 text-center">
			<Icon icon="solar:danger-triangle-bold" class="mb-1 h-8 w-8" style="color: var(--color-danger);" />
			<p style="color: var(--color-danger);">Failed to load checks</p>
			<p class="text-sm" style="color: var(--color-text-secondary);">{error}</p>
		</div>
	{:else}

		<!-- Categories -->
		<div class="space-y-4">
			{#each categories as cat}
				<details class="cat-group" style="background: {cat.color}15; border: 1px solid {cat.color}30; border-radius: 10px; overflow: hidden;">
					<summary class="cat-summary" style="padding: 12px 16px; cursor: pointer;">
						<div class="flex items-center justify-between">
							<div class="flex items-center gap-2.5">
								<span class="text-lg">{cat.icon}</span>
								<div>
									<span class="text-sm font-semibold" style="color: var(--color-text);">{cat.label}</span>
									<span class="text-xs ml-1.5" style="color: var(--color-text-muted);">{cat.items.length} check{cat.items.length !== 1 ? 's' : ''}</span>
								</div>
							</div>
							{#if cat.items.some(c => c.severity)}
								<div class="flex items-center gap-1.5">
									{#each ['critical','high','medium','low'] as sev}
										{@const count = cat.items.filter(c => c.severity === sev || (sev === 'low' && c.severity === 'info')).length}
										{#if count > 0}
											<span class="severity-pill text-[10px] font-semibold px-1.5 py-0.5 rounded" style="background: {severityBg(sev)}; color: {severityColor(sev)};">
												{count}{sev === 'critical' ? 'C' : sev === 'high' ? 'H' : sev === 'medium' ? 'M' : 'L'}
											</span>
										{/if}
									{/each}
								</div>
							{/if}
							<Icon icon="solar:alt-arrow-down-bold" class="h-4 w-4 chevron shrink-0" style="color: var(--color-text-muted); transition: transform 0.2s;" />
						</div>
					</summary>
					<div style="padding: 0 16px 14px; border-top: 1px solid {cat.color}30;">
						<div class="mt-3 space-y-2">
							{#each cat.items as check}
								<div class="check-row" style="background: var(--color-card); border: 1px solid var(--color-border-light); border-left: 3px solid {severityColor(check.severity)}; border-radius: 8px; padding: 10px 12px;">
									<div class="flex items-start justify-between gap-3">
										<div class="flex-1 min-w-0">
											<div class="flex items-center gap-2 flex-wrap">
												<Icon icon={severityIcon(check.severity)} class="h-3.5 w-3.5 shrink-0" style="color: {severityColor(check.severity)};" />
												<span class="text-sm font-medium" style="color: var(--color-text);">{check.title || check.id}</span>
												{#if check.severity}
													<span class="text-[10px] px-1.5 py-0.5 rounded-full font-medium" style="background: {severityBg(check.severity)}; color: {severityColor(check.severity)};">
														{check.severity}
													</span>
												{/if}
												{#if check.cis_id}
													<span class="text-[10px] font-mono" style="color: var(--color-text-muted);">CIS {check.cis_id}</span>
												{/if}
												<span class="text-[10px] font-mono" style="color: var(--color-text-muted);">{check.id}</span>
											</div>
											{#if check.risk}
												<p class="text-xs mt-1.5" style="color: var(--color-text-secondary); line-height: 1.4;">{check.risk}</p>
											{/if}
										</div>
									</div>
								</div>
							{/each}
						</div>
					</div>
				</details>
			{/each}
		</div>

		<!-- Scan History -->
		<div class="mt-8">
			<div class="flex items-center gap-2 mb-3">
				<h3 class="text-sm font-semibold" style="color: var(--color-text);">📜 Scan History</h3>
				{#if historyTotal > 0}
					<span class="text-xs" style="color: var(--color-text-muted);">{historyTotal} scan{historyTotal !== 1 ? 's' : ''}</span>
				{/if}
			</div>

			<div class="card !p-0 overflow-hidden">
				{#if historyLoading}
					<div class="flex items-center justify-center py-6">
						<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-text-muted);" />
					</div>
				{:else if history.length > 0}
					<!-- Header row -->
					<div class="grid gap-2 px-4 py-2 text-[11px] font-semibold uppercase tracking-wider"
						style="color: var(--color-text-muted); background: var(--color-surface); border-bottom: 1px solid var(--color-border); grid-template-columns: 1fr 48px 60px 70px 80px 60px;">
						<span>Server</span>
						<span class="text-center">Score</span>
						<span class="text-center">Findings</span>
						<span class="text-center">Status</span>
						<span class="text-right">When</span>
						<span class="text-center">Action</span>
					</div>

					{#each history as item, i}
						{@const sColor = item.score !== null && item.score !== undefined ? scoreColor(item.score) : 'var(--color-text-muted)'}
						{@const bgColor = i % 2 === 0 ? 'transparent' : 'rgba(255,255,255,0.015)'}
						<div class="grid gap-2 px-4 py-2.5 text-xs"
							style="background: {bgColor}; border-bottom: {i < history.length - 1 ? '1px solid var(--color-border)' : 'none'}; grid-template-columns: 1fr 48px 60px 70px 80px 60px; align-items: center;">
							<div class="min-w-0">
								<p class="text-sm font-medium truncate" style="color: var(--color-text);" title="{item.name || item.server_name || 'Unknown'}">{item.name || item.server_name || 'Unknown'}</p>
							</div>
							<div class="text-center">
								<span class="text-sm font-bold" style="color: {sColor};">{item.score ?? '—'}</span>
							</div>
							<div class="flex items-center gap-1 justify-center">
								{#if item.criticals > 0}
									<span style="color: var(--color-danger); font-weight: 600;">{item.criticals}C</span>
								{/if}
								{#if item.warnings > 0}
									<span style="color: var(--color-warning); font-weight: 600;">{item.warnings}W</span>
								{/if}
								{#if !item.criticals && !item.warnings}
									<span style="color: var(--color-text-muted);">—</span>
								{/if}
							</div>
							<div class="text-center text-[11px]" style="color: var(--color-text-muted);">
								{statusLabel[item.status] || item.status || '—'}
							</div>
							<div class="text-right text-[11px]" style="color: var(--color-text-muted);">
								{formatTime(item.completed_at || item.created_at)}
							</div>
							<div class="text-center">
								<button onclick={(e) => { e.stopPropagation(); item.server_id && goto(`/servers/${item.server_id}?scan=${item.id}&tab=compliance`); }}
									class="text-xs font-medium px-2 py-1 rounded"
									style="color: var(--color-primary); background: rgba(16,185,129,0.1); border: none; cursor: pointer;">View</button>
							</div>
						</div>
					{/each}

					<!-- Pagination -->
					{#if totalPages > 1}
						<div class="flex items-center justify-center gap-2 px-4 py-3" style="border-top: 1px solid var(--color-border);">
							<button disabled={historyPage <= 1} onclick={() => loadHistory(historyPage - 1)}
								class="btn-secondary text-xs py-1 px-2" style="opacity: {historyPage <= 1 ? 0.4 : 1};">← Prev</button>
							<span class="text-xs" style="color: var(--color-text-muted);">{historyPage} / {totalPages}</span>
							<button disabled={historyPage >= totalPages} onclick={() => loadHistory(historyPage + 1)}
								class="btn-secondary text-xs py-1 px-2" style="opacity: {historyPage >= totalPages ? 0.4 : 1};">Next →</button>
						</div>
					{/if}
				{:else}
					<div class="flex flex-col items-center py-10 text-center">
						<Icon icon="solar:clipboard-remove-bold" class="mb-2 h-8 w-8" style="color: var(--color-text-muted);" />
						<p class="text-sm" style="color: var(--color-text-secondary);">No scan history yet for CIS Level 2</p>
						<p class="text-xs mt-1" style="color: var(--color-text-muted);">Run a scan from the Compliance dashboard to see results here.</p>
					</div>
				{/if}
			</div>
		</div>

		<!-- Source note -->
		<div class="mt-6 text-center">
			<p class="text-xs" style="color: var(--color-text-muted);">
				Based on <strong>CIS Benchmark for Distribution Independent Linux</strong>.
				Includes additional checks not present in Level 1.
			</p>
		</div>
	{/if}
</div>

<style>
	.check-row { transition: all 0.15s; }
	.check-row:hover { border-color: var(--color-primary); box-shadow: 0 1px 4px rgba(16,185,129,0.06); }
	.cat-summary { user-select: none; display: flex; align-items: center; }
	.cat-summary::-webkit-details-marker { display: none; }
	.cat-summary::marker { display: none; }
	details[open] .chevron { transform: rotate(180deg); }
</style>
