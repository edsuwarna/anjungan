<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
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
	let sortField = $state('domain');
	let sortOrder = $state('asc');

	// Modal
	let showAddModal = $state(false);

	// Checking state
	let checking = $state({});
	let checkingAll = $state(false);

	// Batch import
	let showBatchModal = $state(false);
	let batchDomains = $state('');
	let batchImporting = $state(false);
	let batchResult = $state(null);

	// Webhooks for notification config in add modal
	let webhooks = $state([]);
	let webhooksLoading = $state(false);

	// Computed filter state
	let hasFilters = $derived(searchQuery || statusFilter);

	// ─── Load ───
	async function loadData() {
		loading = true;
		try {
			const [listData, summaryData] = await Promise.all([
				api.sslMonitors.list({ page, limit, search: searchQuery, status: statusFilter, sort: sortField, order: sortOrder }),
				api.sslMonitors.summary(),
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
			summary = await api.sslMonitors.summary();
		} catch (_) {}
	}

	onMount(() => {
		loadData();
		loadWebhooks();
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
			await api.sslMonitors.checkNow(id);
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
			await api.sslMonitors.checkAll();
			await loadData();
		} catch (e) {
			// Keep page state
		} finally {
			checkingAll = false;
		}
	}

	// ─── Delete ───
	async function deleteMonitor(id) {
		if (!confirm('Delete this SSL monitor?')) return;
		try {
			await api.sslMonitors.delete(id);
			await loadData();
			await loadSummary();
		} catch (e) {
			alert('Failed to delete: ' + e.message);
		}
	}

	// ─── Export CSV ───
	async function downloadCsv() {
		try {
			const token = typeof window !== 'undefined' ? localStorage.getItem('access_token') : null;
			const res = await fetch('/api/v1/ssl-monitors/export/csv', {
				headers: token ? { 'Authorization': `Bearer ${token}` } : {},
			});
			if (!res.ok) throw new Error('Export failed');
			const blob = await res.blob();
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = 'ssl-monitors-export.csv';
			document.body.appendChild(a);
			a.click();
			document.body.removeChild(a);
			URL.revokeObjectURL(url);
		} catch (e) {
			alert('Failed to export: ' + e.message);
		}
	}

	// ─── Batch Import ───
	async function handleBatchImport() {
		const domains = batchDomains
			.split('\n')
			.map(d => d.trim())
			.filter(d => d.length > 0 && !d.startsWith('#'));
		if (domains.length === 0) {
			alert('Please enter at least one domain.');
			return;
		}
		batchImporting = true;
		batchResult = null;
		try {
			batchResult = await api.sslMonitors.batchImport({ domains });
			await loadData();
			await loadSummary();
		} catch (e) {
			alert('Import failed: ' + e.message);
			batchImporting = false;
		} finally {
			batchImporting = false;
		}
	}

	// ─── Modal ───
	async function loadWebhooks() {
		webhooksLoading = true;
		try {
			webhooks = await api.registryWebhooks.list() || [];
		} catch (_) {
			webhooks = [];
		} finally {
			webhooksLoading = false;
		}
	}

	async function handleAdd(data) {
		try {
			await api.sslMonitors.create(data);
			showAddModal = false;
			await loadData();
			await loadSummary();
		} catch (e) {
			alert('Failed to create: ' + e.message);
		}
	}

	// ─── Helpers ───
	const statusConfig = {
		valid: { label: 'Valid', color: '#10b981', icon: 'solar:shield-check-bold' },
		expiring_soon: { label: 'Expiring Soon', color: '#f59e0b', icon: 'solar:clock-circle-bold' },
		expired: { label: 'Expired', color: '#ef4444', icon: 'solar:danger-circle-bold' },
		error: { label: 'Error', color: '#6b7280', icon: 'solar:close-circle-bold' },
		pending: { label: 'Pending', color: '#6366f1', icon: 'solar:hourglass-bold' },
	};

	function getStatusConfig(s) {
		return statusConfig[s] || statusConfig.pending;
	}

	function daysLabel(d) {
		if (d <= 0) return 'Expired';
		if (d === 1) return '1 day';
		return `${d} days`;
	}

	function statusBadgeClass(status) {
		const cfg = getStatusConfig(status);
		return `inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium`;
	}

	function statusBadgeStyle(status) {
		const cfg = getStatusConfig(status);
		const alpha = status === 'valid' ? '0.12' : '0.15';
		return `background-color: ${cfg.color}${alpha}; color: ${cfg.color};`;
	}

	const filterChips = [
		{ value: '', label: 'All' },
		{ value: 'valid', label: 'Valid' },
		{ value: 'expiring_soon', label: 'Expiring' },
		{ value: 'expired', label: 'Expired' },
		{ value: 'error', label: 'Error' },
	];

	function cipherGradeColor(grade) {
		switch(grade) {
			case 'A+': case 'A': return '#10b981';
			case 'B': return '#f59e0b';
			case 'C': case 'D': return '#ef4444';
			default: return '#6b7280';
		}
	}
</script>

<div class="page-container">
	<!-- Page Header -->
	<div class="mb-6 flex flex-wrap items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold" style="color: var(--color-text);">SSL Certificate Monitoring</h1>
			<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
				Monitor TLS/SSL certificate expiry, chain validation, and cipher strength
			</p>
		</div>
		<div class="flex items-center gap-3">
			<button
				class="btn-ghost"
				onclick={downloadCsv}
				title="Export CSV"
			>
				<Icon icon="solar:export-bold" class="h-4 w-4" />
				CSV
			</button>
			<button
				class="btn-secondary"
				onclick={() => showBatchModal = true}
			>
				<Icon icon="solar:import-bold" class="h-4 w-4" />
				Batch Import
			</button>
			<button
				class="btn-secondary"
				onclick={checkAll}
				disabled={checkingAll}
			>
				<Icon icon="solar:refresh-bold" class="h-4 w-4" />
				{checkingAll ? 'Checking...' : 'Check All'}
			</button>
			<button
				class="btn-primary"
				onclick={() => showAddModal = true}
			>
				<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
				Add Domain
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
					<Icon icon="solar:shield-bold" class="h-5 w-5" style="color: var(--color-primary);" />
				</div>
			</div>
		</div>
		{#each filterChips.filter(f => f.value) as { value, label }}
			{@const cfg = getStatusConfig(value)}
			<div class="stat-card">
				<div class="flex items-center justify-between">
					<div>
						<p class="text-sm font-medium" style="color: var(--color-text-secondary);">{label}</p>
						<p class="mt-1 text-2xl font-bold" style="color: {cfg.color};">
							{summary?.[value === 'expiring_soon' ? 'expiring_soon' : value] ?? 0}
						</p>
					</div>
					<div class="flex h-10 w-10 items-center justify-center rounded-lg" style="background-color: {cfg.color}15;">
						<Icon icon={cfg.icon} class="h-5 w-5" style="color: {cfg.color};" />
					</div>
				</div>
			</div>
		{/each}
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
					placeholder="Search domain..."
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
			<Icon icon="solar:shield-warning-bold" class="mx-auto h-12 w-12" style="color: var(--color-text-muted);" />
			<p class="mt-4 text-lg font-medium" style="color: var(--color-text);">No SSL monitors</p>
			<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
				{hasFilters ? 'No monitors match your filters.' : 'Add your first domain to start monitoring certificates.'}
			</p>
			{#if hasFilters}
				<button class="btn-secondary mt-4" onclick={clearFilters}>Clear Filters</button>
			{:else}
				<button class="btn-primary mt-4" onclick={() => showAddModal = true}>Add Domain</button>
			{/if}
		</div>
	{:else}
		<!-- Domain Cards -->
		<div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
			{#each monitors as m (m.id)}
				{@const cfg = getStatusConfig(m.last_status)}
				<div
					class="card cursor-pointer transition-all duration-200 hover:scale-[1.02]"
					onclick={() => goto(`/ssl-monitors/${m.id}`)}
					role="button"
					tabindex="0"
					onkeydown={(e) => e.key === 'Enter' && goto(`/ssl-monitors/${m.id}`)}
				>
					<!-- Card Header -->
					<div class="mb-3 flex items-start justify-between">
						<div class="min-w-0 flex-1">
							<h3 class="truncate text-base font-semibold" style="color: var(--color-text);">
								{m.display_name || m.domain}
							</h3>
							<p class="truncate text-sm" style="color: var(--color-text-secondary);">
								{m.domain}:{m.port}
							</p>
						</div>
						<div class="ml-2 flex items-center gap-1.5">
							<div class={statusBadgeClass(m.last_status)} style={statusBadgeStyle(m.last_status)}>
								<Icon icon={cfg.icon} class="h-3.5 w-3.5" />
								{cfg.label}
							</div>
						</div>
					</div>

					<!-- Cert Info -->
					<div class="mb-3 grid grid-cols-2 gap-2 text-sm">
						<div>
							<p class="text-xs" style="color: var(--color-text-muted);">Issuer</p>
							<p class="truncate font-medium" style="color: var(--color-text);">{m.issuer || '-'}</p>
						</div>
						<div>
							<p class="text-xs" style="color: var(--color-text-muted);">Subject</p>
							<p class="truncate font-medium" style="color: var(--color-text);">{m.subject || '-'}</p>
						</div>
						<div>
							<p class="text-xs" style="color: var(--color-text-muted);">Expires</p>
							<p class="font-medium" style="color: {m.days_remaining <= 14 ? '#ef4444' : m.days_remaining <= 30 ? '#f59e0b' : 'var(--color-text)'};">
								{m.cert_expires_at ? new Date(m.cert_expires_at).toLocaleDateString() : '-'}
								<span class="text-xs" style="color: var(--color-text-muted);">
									({daysLabel(m.days_remaining)})
								</span>
							</p>
						</div>
						<div>
							<p class="text-xs" style="color: var(--color-text-muted);">Cipher</p>
							<p class="font-medium" style="color: {cipherGradeColor(m.cipher_grade)};">
								{m.cipher_grade || '-'}
							</p>
						</div>
					</div>

					<!-- Badges -->
					<div class="mb-3 flex flex-wrap items-center gap-1.5">
						{#if m.chain_valid === true}
							<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs" style="background: #10b98118; color: #10b981;">
								<Icon icon="solar:link-bold" class="h-3 w-3" />Chain OK
							</span>
						{:else if m.chain_valid === false}
							<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs" style="background: #ef444418; color: #ef4444;">
								<Icon icon="solar:link-broken-bold" class="h-3 w-3" />Chain
							</span>
						{/if}
						{#if m.ocsp_status === 'good'}
							<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs" style="background: #10b98118; color: #10b981;">
								<Icon icon="solar:check-circle-bold" class="h-3 w-3" />OCSP OK
							</span>
						{:else if m.ocsp_status === 'revoked'}
							<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs" style="background: #ef444418; color: #ef4444;">
								<Icon icon="solar:danger-circle-bold" class="h-3 w-3" />Revoked
							</span>
						{/if}
						{#if m.san_mismatch}
							<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs" style="background: #f59e0b18; color: #f59e0b;">
								<Icon icon="solar:subtitles-bold" class="h-3 w-3" />SAN!
							</span>
						{/if}
						{#if m.last_status === 'error'}
							<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs" style="background: #ef444418; color: #ef4444;">
								<Icon icon="solar:bug-bold" class="h-3 w-3" />Error
							</span>
						{/if}
					</div>

					<!-- Footer -->
					<div class="flex items-center justify-between border-t pt-3" style="border-color: var(--color-border);">
						<p class="text-xs" style="color: var(--color-text-muted);">
							Last checked: {m.last_check_at ? new Date(m.last_check_at).toLocaleString() : 'Never'}
						</p>
						<div class="flex items-center gap-2">
							<button
								class="btn-icon"
								onclick={(e) => { e.stopPropagation(); checkMonitor(m.id); }}
								disabled={checking[m.id]}
								title="Check now"
							>
								<Icon icon={checking[m.id] ? 'svg-spinners:180-ring' : 'solar:refresh-bold'} class="h-4 w-4" />
							</button>
							<button
								class="btn-icon text-red-500 hover:bg-red-500/10"
								onclick={(e) => { e.stopPropagation(); deleteMonitor(m.id); }}
								title="Delete"
							>
								<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
							</button>
						</div>
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

<!-- Add Modal -->
{#if showAddModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
		onclick={() => showAddModal = false}
		role="presentation"
	>
		<div
			class="card w-full max-w-lg p-6"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
		>
			<h2 class="mb-1 text-lg font-bold" style="color: var(--color-text);">Add SSL Monitor</h2>
			<p class="mb-5 text-sm" style="color: var(--color-text-secondary);">
				Monitor TLS certificate for any domain
			</p>

			<form onsubmit={(e) => { e.preventDefault(); const fd = new FormData(e.target); const whIds = Array.from(fd.getAll('webhook_ids')); handleAdd({ domain: fd.get('domain'), port: parseInt(fd.get('port')) || 443, display_name: fd.get('display_name'), check_interval: fd.get('check_interval') || '1h', notify_before: fd.get('notify_before') || '14d', webhook_ids: whIds, }); }}>
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Domain *</label>
					<input type="text" name="domain" required placeholder="app.example.com" class="input w-full" />
				</div>
				<div class="mb-4 grid grid-cols-2 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Port</label>
						<input type="number" name="port" value="443" min="1" max="65535" class="input w-full" />
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Display Name</label>
						<input type="text" name="display_name" placeholder="My App" class="input w-full" />
					</div>
				</div>
				<div class="mb-4 grid grid-cols-2 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Check Interval</label>
						<select name="check_interval" class="input w-full">
							<option value="30m">Every 30 minutes</option>
							<option value="1h" selected>Every hour</option>
							<option value="6h">Every 6 hours</option>
							<option value="12h">Every 12 hours</option>
							<option value="24h">Every 24 hours</option>
						</select>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Notify Before</label>
						<select name="notify_before" class="input w-full">
							<option value="7d">7 days</option>
							<option value="14d" selected>14 days</option>
							<option value="21d">21 days</option>
							<option value="30d">30 days</option>
							<option value="never">Never</option>
						</select>
					</div>
				</div>

				<!-- Notification Channels -->
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Notify Via</label>
					{#if webhooksLoading}
						<p class="text-xs" style="color: var(--color-text-muted);">Loading webhooks...</p>
					{:else if webhooks.length === 0}
						<p class="text-xs" style="color: var(--color-text-muted);">No webhooks configured. Create one in Registry.</p>
					{:else}
						<div class="space-y-1 max-h-40 overflow-y-auto">
							{#each webhooks as wh}
								<label class="flex cursor-pointer items-center gap-3 rounded-lg p-2 text-sm">
									<input type="checkbox" name="webhook_ids" value={wh.id} class="h-4 w-4 rounded border-gray-300" />
									<div>
										<p class="text-sm" style="color: var(--color-text);">{wh.name || wh.url}</p>
										<p class="text-xs" style="color: var(--color-text-muted);">{wh.platform}</p>
									</div>
								</label>
							{/each}
						</div>
					{/if}
				</div>

				<div class="flex items-center justify-end gap-3 pt-4">
					<button type="button" class="btn-secondary" onclick={() => showAddModal = false}>Cancel</button>
					<button type="submit" class="btn-primary">Add Monitor</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Batch Import Modal -->
{#if showBatchModal}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div class="modal-overlay" onclick={() => { if (!batchImporting) showBatchModal = false; }}>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div class="modal-panel" onclick={(e) => e.stopPropagation()}>
			<div class="flex items-center justify-between border-b pb-4" style="border-color: var(--color-border);">
				<div>
					<h2 class="text-lg font-bold" style="color: var(--color-text);">Batch Import Domains</h2>
					<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
						One domain per line. Lines starting with # are ignored.
					</p>
				</div>
				<button class="btn-icon" onclick={() => showBatchModal = false}>
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>

			<div class="py-5">
				<textarea
					bind:value={batchDomains}
					placeholder="app1.example.com&#10;app2.example.com&#10;staging.example.com&#10;# this is a comment"
					class="input w-full"
					rows="10"
					disabled={batchImporting}
					style="font-family: 'Courier New', monospace; font-size: 0.875rem; resize: vertical;"
				></textarea>

				{#if batchDomains.trim()}
					{@const domains = batchDomains.split('\n').map(d => d.trim()).filter(d => d.length > 0 && !d.startsWith('#'))}
					<p class="mt-2 text-xs" style="color: var(--color-text-secondary);">
						{domains.length} domain{domains.length !== 1 ? 's' : ''} detected
					</p>
				{/if}

				{#if batchResult}
					<div class="mt-4 rounded-lg border p-4" style="border-color: var(--color-border); background: var(--color-bg);">
						<p class="text-sm font-medium" style="color: var(--color-text);">Import Complete</p>
						<div class="mt-2 flex gap-4 text-sm">
							<span style="color: var(--color-primary);">✅ {batchResult.created} created</span>
							{#if batchResult.skipped > 0}
								<span style="color: #f59e0b;">⏭️ {batchResult.skipped} skipped</span>
							{/if}
							{#if batchResult.errors > 0}
								<span style="color: #ef4444;">❌ {batchResult.errors} errors</span>
							{/if}
						</div>
						{#if batchResult.details?.length > 0}
							<div class="mt-3 max-h-32 space-y-1 overflow-y-auto text-xs" style="color: var(--color-text-secondary);">
								{#each batchResult.details as d}
									<div>
										{d.domain} —
										{#if d.status === 'created'}
											<span style="color: var(--color-primary);">created</span>
										{:else if d.status === 'skipped'}
											<span style="color: #f59e0b;">skipped ({d.error})</span>
										{:else}
											<span style="color: #ef4444;">error: {d.error}</span>
										{/if}
									</div>
								{/each}
							</div>
						{/if}
					</div>
				{/if}
			</div>

			<div class="flex items-center justify-between gap-3 pt-4">
				<button type="button" class="btn-ghost text-sm" onclick={() => batchDomains = ''}>
					Clear
				</button>
				<div class="flex items-center gap-3">
					<button type="button" class="btn-secondary" onclick={() => { showBatchModal = false; batchResult = null; batchDomains = ''; }}>
						{batchResult ? 'Close' : 'Cancel'}
					</button>
					{#if !batchResult}
						<button
							type="button"
							class="btn-primary"
							onclick={handleBatchImport}
							disabled={batchImporting || !batchDomains.trim()}
						>
							<Icon icon="solar:import-bold" class="h-4 w-4" />
							{batchImporting ? 'Importing...' : 'Import Domains'}
						</button>
					{/if}
				</div>
			</div>
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
	select.input {
		appearance: auto;
	}
	:global(body.dark) .card { background: #1a1d23; border-color: rgba(148,163,184,0.08); }
	:global(body.dark) .stat-card { background: #1a1d23; border-color: rgba(148,163,184,0.08); }
</style>
