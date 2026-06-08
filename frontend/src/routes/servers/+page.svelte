<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';
	import AddServerModal from '$lib/components/ui/AddServerModal.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { loadThresholds, scoreColor, scoreLabel } from '$lib/thresholds.svelte.js';

	let servers = $state([]);
	let loading = $state(true);
	let error = $state('');
	let showModal = $state(false);
	let editServer = $state(null);

	// Pagination
	let page = $state(1);
	let limit = $state(20);
	let total = $state(0);
	let totalPages = $state(1);

	// Filters
	let searchQuery = $state('');
	let statusFilter = $state('');
	let groupFilter = $state('');
	let regionFilter = $state('');
	let typeFilter = $state('');
	let sortField = $state('name');
	let sortOrder = $state('asc');

	// Bulk selection
	let selectedIds = $state(new Set());
	let allSelected = $derived(servers.length > 0 && servers.every(s => selectedIds.has(s.id)));

	// Filter options
	let groups = $state([]);
	let regions = $state([]);
	let types = $state([]);

	// Confirmation modal state
	let confirmModal = $state({ show: false, title: '', message: '', onConfirm: null, danger: false });

	let isFiltered = $derived(
		searchQuery || statusFilter || groupFilter || regionFilter || typeFilter
	);

	// Dummy data mode for testing card layout
	let useDummyData = $state(false);

	const dummyServers = [
		{ id: 'dummy-1', name: 'peladen-central', host: '103.25.61.1', port: 22, ssh_user: 'ubuntu', status: 'online', tags: ['production', 'k3s', 'docker', 'monitoring'], server_group: 'infra', region: 'sgp-1', server_type: 'vps', created_at: '2025-06-15T00:00:00Z' },
		{ id: 'dummy-2', name: 'peladen-cadangan', host: '103.25.61.2', port: 22, ssh_user: 'ubuntu', status: 'online', tags: ['staging', 'docker'], server_group: 'infra', region: 'sgp-1', server_type: 'vps', created_at: '2025-08-01T00:00:00Z' },
		{ id: 'dummy-3', name: 'old-server-01', host: '192.168.1.10', port: 2222, ssh_user: 'root', status: 'offline', tags: ['legacy', 'decommissioned'], server_group: 'on-prem', region: 'jkt-1', server_type: 'baremetal', created_at: '2024-01-10T00:00:00Z' },
		{ id: 'dummy-4', name: 'dev-box-01', host: '10.0.0.101', port: 22, ssh_user: 'developer', status: 'online', tags: ['dev', 'testing', 'gpu', 'pytorch', 'docker', 'nginx'], server_group: 'development', region: 'local', server_type: 'workstation', created_at: '2025-11-20T00:00:00Z' },
		{ id: 'dummy-5', name: 'db-primary', host: '10.0.1.50', port: 22, ssh_user: 'postgres', status: 'online', tags: ['database', 'production', 'postgresql', 'replication-master'], server_group: 'database', region: 'sgp-1', server_type: 'vps', created_at: '2025-03-05T00:00:00Z' },
		{ id: 'dummy-6', name: 'db-replica-asia', host: '10.0.1.51', port: 22, ssh_user: 'postgres', status: 'pending', tags: ['database', 'replica'], server_group: 'database', region: 'hkg-1', server_type: 'vps', created_at: '2026-01-15T00:00:00Z' },
		{ id: 'dummy-7', name: 'cache-redis-01', host: '10.0.2.10', port: 22, ssh_user: 'ubuntu', status: 'online', tags: ['cache', 'redis', 'production'], server_group: 'infra', region: 'sgp-1', server_type: 'vps', created_at: '2025-07-22T00:00:00Z' },
		{ id: 'dummy-8', name: 'ci-runner-linux', host: 'ci-01.internal', port: 2222, ssh_user: 'runner', status: 'online', tags: ['ci', 'github-actions', 'docker'], server_group: 'devops', region: 'cloud', server_type: 'container', created_at: '2025-09-10T00:00:00Z' },
		{ id: 'dummy-9', name: 'vps-storage-01', host: '103.25.61.20', port: 22, ssh_user: 'root', status: 'offline', tags: [], server_group: 'infra', region: 'fra-1', server_type: 'vps', created_at: '2025-04-18T00:00:00Z' },
		{ id: 'dummy-10', name: 'monitoring-box', host: '10.0.3.1', port: 22, ssh_user: 'monitor', status: 'unknown', tags: ['monitoring', 'prometheus', 'grafana'], server_group: 'devops', region: 'sgp-1', server_type: 'vps', created_at: '2026-02-01T00:00:00Z' },
		{ id: 'dummy-11', name: 'edge-proxy-jkt', host: '203.xx.xx.50', port: 22, ssh_user: 'proxy', status: 'online', tags: ['edge', 'proxy', 'cloudflare', 'ssl', 'cdn'], server_group: 'networking', region: 'jkt-1', server_type: 'baremetal', created_at: '2025-05-30T00:00:00Z' },
		{ id: 'dummy-12', name: 'nas-backup', host: '192.168.10.1', port: 22, ssh_user: 'backup', status: 'online', tags: ['storage', 'backup', 'nas'], server_group: 'storage', region: 'local', server_type: 'baremetal', created_at: '2024-11-01T00:00:00Z' },
	];

	// Expanded card actions state
	let expandedCard = $state(null);

	// Close expanded card on outside click
	function handleWindowClick(e) {
		if (expandedCard !== null) {
			const cardEl = e.target.closest('.server-card');
			if (!cardEl || cardEl.dataset.serverId !== expandedCard) {
				expandedCard = null;
			}
		}
	}

	onMount(() => {
		window.addEventListener('click', handleWindowClick);
		loadThresholds();
		return () => window.removeEventListener('click', handleWindowClick);
	});

	onMount(async () => {
		await Promise.all([loadServers(), loadFilterOptions()]);
	});

	async function loadFilterOptions() {
		try {
			const [g, r, t] = await Promise.all([
				api.servers.groups(),
				api.servers.regions(),
				api.servers.types()
			]);
			groups = g || [];
			regions = r || [];
			types = t || [];
		} catch (_) {}
	}

	async function loadServers() {
		if (useDummyData) {
			servers = dummyServers;
			total = dummyServers.length;
			totalPages = 1;
			loading = false;
			error = '';
			return;
		}
		loading = true;
		error = '';
		try {
			const result = await api.servers.list({
				page, limit,
				sort: sortField, order: sortOrder,
				search: searchQuery || undefined,
				status: statusFilter || undefined,
				server_group: groupFilter || undefined,
				region: regionFilter || undefined,
				server_type: typeFilter || undefined,
			});
			servers = result.servers || [];
			total = result.total || 0;
			totalPages = result.total_pages || 1;
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function toggleDummy() {
		useDummyData = !useDummyData;
		page = 1;
		selectedIds = new Set();
		loadServers();
	}

	function toggleActions(id) {
		expandedCard = expandedCard === id ? null : id;
	}

	function doSearch() {
		page = 1;
		selectedIds = new Set();
		loadServers();
	}

	function applyFilter() {
		doSearch();
	}

	function goToPage(p) {
		if (p < 1 || p > totalPages) return;
		page = p;
		loadServers();
	}

	function setSort(field) {
		if (sortField === field) {
			sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		} else {
			sortField = field;
			sortOrder = 'asc';
		}
		loadServers();
	}

	function sortIcon(field) {
		if (sortField !== field) return 'solar:sort-vertical-bold';
		return sortOrder === 'asc' ? 'solar:sort-from-top-bold' : 'solar:sort-from-bottom-bold';
	}

	function confirmDelete(server) {
		expandedCard = null;
		confirmModal = {
			show: true,
			title: 'Delete Server',
			message: `Are you sure you want to delete "${server.name}" (${server.host})? This action cannot be undone.`,
			danger: true,
			onConfirm: async () => {
				try {
					await api.servers.delete(server.id);
					loadServers();
				} catch (e) {
					error = 'Failed to delete server: ' + e.message;
				}
				confirmModal = { show: false, title: '', message: '', onConfirm: null, danger: false };
			}
		};
	}

	function confirmBulkDelete() {
		if (selectedIds.size === 0) return;
		confirmModal = {
			show: true,
			title: 'Bulk Delete',
			message: `Are you sure you want to delete ${selectedIds.size} server(s)? This action cannot be undone.`,
			danger: true,
			onConfirm: async () => {
				try {
					await api.servers.bulkDelete(Array.from(selectedIds));
					selectedIds = new Set();
					loadServers();
				} catch (e) {
					error = 'Bulk delete failed: ' + e.message;
				}
				confirmModal = { show: false, title: '', message: '', onConfirm: null, danger: false };
			}
		};
	}

	async function testConnection(id) {
		try {
			const result = await api.servers.testExisting(id);
			const idx = servers.findIndex(s => s.id === id);
			if (idx !== -1) servers[idx].status = result.reachable ? 'online' : 'offline';
		} catch (_) {}
	}

	function toggleSelect(id) {
		const newSet = new Set(selectedIds);
		if (newSet.has(id)) newSet.delete(id);
		else newSet.add(id);
		selectedIds = newSet;
	}

	function toggleSelectAll() {
		if (allSelected) {
			selectedIds = new Set();
		} else {
			selectedIds = new Set(servers.map(s => s.id));
		}
	}

	async function handleSaved(server) {
		if (server?.id) {
			try {
				await api.servers.testExisting(server.id);
			} catch(e) {}
		}
		loadServers();
	}

	function handleEdit(server) {
		expandedCard = null;
		editServer = server;
		showModal = true;
	}

	function goToDetail(id) {
		expandedCard = null;
		goto(`/servers/${id}`);
	}

	function goToTerminal(id) {
		expandedCard = null;
		goto(`/servers/${id}/terminal`);
	}

	function statusClass(status) {
		switch (status) {
			case 'online': return 'online';
			case 'offline': return 'offline';
			default: return 'pending';
		}
	}

	function formatDate(dateStr) {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
	}

	function formatTime(ts) {
		if (!ts) return '';
		const d = new Date(ts);
		const now = new Date();
		const diff = now - d;
		if (diff < 60000) return 'Just now';
		if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
		if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
		if (diff < 604800000) return Math.floor(diff / 86400000) + 'd ago';
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
	}

	function clearFilters() {
		searchQuery = '';
		statusFilter = '';
		groupFilter = '';
		regionFilter = '';
		typeFilter = '';
		page = 1;
		doSearch();
	}
</script>

<AddServerModal
	show={showModal}
	{editServer}
	onClose={() => { showModal = false; editServer = null; }}
	onSaved={handleSaved}
/>

<!-- Confirmation Modal -->
{#if confirmModal.show}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background-color: rgba(0,0,0,0.5);"
		onclick={() => confirmModal = { show: false, title: '', message: '', onConfirm: null, danger: false }}
		role="presentation"
	>
		<!-- svelte-ignore a11y_interactive_supports_focus -->
		<div
			class="w-full max-w-sm rounded-xl border shadow-xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
			role="alertdialog"
			aria-modal="true"
		>
			<div class="px-6 py-5">
				<div class="flex items-center gap-3">
					<div
						class="flex h-10 w-10 items-center justify-center rounded-full"
						style="background-color: {confirmModal.danger ? 'rgba(239,68,68,0.1)' : 'var(--color-primary-subtle)'};"
					>
						<Icon
							icon={confirmModal.danger ? 'solar:danger-triangle-bold' : 'solar:info-circle-bold'}
							class="h-5 w-5"
							style="color: {confirmModal.danger ? 'var(--color-danger)' : 'var(--color-primary)'};"
						/>
					</div>
					<div class="flex-1">
						<h3 class="text-base font-semibold" style="color: var(--color-text);">{confirmModal.title}</h3>
					</div>
				</div>
				<p class="mt-3 text-sm" style="color: var(--color-text-secondary);">{confirmModal.message}</p>
			</div>
			<div class="flex items-center justify-end gap-2 border-t px-6 py-3" style="border-color: var(--color-border);">
				<button
					onclick={() => confirmModal = { show: false, title: '', message: '', onConfirm: null, danger: false }}
					class="btn-secondary text-sm"
				>
					Cancel
				</button>
				<button
					onclick={() => confirmModal.onConfirm?.()}
					class="text-sm"
					class:btn-danger={confirmModal.danger}
					class:btn-primary={!confirmModal.danger}
				>
					{confirmModal.danger ? 'Delete' : 'Continue'}
				</button>
			</div>
		</div>
	</div>
{/if}

<div class="page-container">
	<!-- Header -->
	<div class="page-header flex items-center justify-between">
		<div>
			<h1 class="page-title">Servers</h1>
			<p class="page-subtitle">Manage your infrastructure endpoints</p>
		</div>
		<button onclick={() => showModal = true} class="btn-primary flex items-center gap-2">
			<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
			Add Server
		</button>
	</div>

	<!-- Filter Bar -->
	<div class="mb-4 flex flex-wrap items-center gap-2">
		<div class="relative flex-1 min-w-[200px] max-w-sm">
			<Icon icon="solar:magnifer-bold" class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2" style="color: var(--color-text-muted);" />
			<input
				type="text"
				bind:value={searchQuery}
				placeholder="Search by name or host..."
				class="input w-full pl-9 text-sm"
				onkeydown={(e) => e.key === 'Enter' && doSearch()}
			/>
		</div>

		<select bind:value={statusFilter} onchange={applyFilter} class="input text-sm min-w-[120px]">
			<option value="">All Status</option>
			<option value="online">Online</option>
			<option value="offline">Offline</option>
			<option value="unknown">Unknown</option>
		</select>

		{#if groups.length > 0}
			<select bind:value={groupFilter} onchange={applyFilter} class="input text-sm min-w-[130px]">
				<option value="">All Groups</option>
				{#each groups as g}
					<option value={g}>{g}</option>
				{/each}
			</select>
		{/if}

		{#if regions.length > 0}
			<select bind:value={regionFilter} onchange={applyFilter} class="input text-sm min-w-[130px]">
				<option value="">All Regions</option>
				{#each regions as r}
					<option value={r}>{r}</option>
				{/each}
			</select>
		{/if}

		{#if types.length > 0}
			<select bind:value={typeFilter} onchange={applyFilter} class="input text-sm min-w-[130px]">
				<option value="">All Types</option>
				{#each types as t}
					<option value={t}>{t}</option>
				{/each}
			</select>
		{/if}

		{#if isFiltered}
			<button onclick={clearFilters} class="btn-icon" title="Clear filters">
				<Icon icon="solar:filter-remove-bold" class="h-4 w-4" />
			</button>
		{/if}

		<button onclick={toggleDummy}
			class="flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors"
			style="background-color: {useDummyData ? 'rgba(245,158,11,0.12)' : 'transparent'}; color: {useDummyData ? '#f59e0b' : 'var(--color-text-muted)'}; border: 1px solid {useDummyData ? '#f59e0b' : 'var(--color-border)'};">
			<Icon icon={useDummyData ? 'solar:database-bold' : 'solar:database-outline'} class="h-3.5 w-3.5" />
			{useDummyData ? 'Dummy ON' : 'Dummy'}
		</button>

		<span class="ml-auto text-xs whitespace-nowrap" style="color: var(--color-text-muted);">
			{total} server{total !== 1 ? 's' : ''}
		</span>
	</div>

	<!-- Content -->
	{#if loading && servers.length === 0}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading servers...</p>
			</div>
		</div>
	{:else if error}
		<div class="rounded-xl border p-6 text-center" style="background-color: var(--color-card); border-color: var(--color-border-light);">
			<Icon icon="solar:danger-triangle-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-danger);" />
			<p class="text-sm font-medium" style="color: var(--color-danger);">Failed to load servers</p>
			<p class="mt-1 text-xs" style="color: var(--color-text-muted);">{error}</p>
		</div>
	{:else if servers.length === 0}
		<div class="rounded-xl border p-12 text-center" style="background-color: var(--color-card); border-color: var(--color-border-light);">
			<Icon icon="solar:server-square-bold" class="mb-3 inline-block h-12 w-12" style="color: var(--color-text-muted);" />
			<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No servers yet</h3>
			<p class="mb-6 text-sm" style="color: var(--color-text-muted);">Add your first server to start managing your infrastructure.</p>
			<button onclick={() => showModal = true} class="btn-primary inline-flex items-center gap-2">
				<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
				Add Your First Server
			</button>
		</div>
	{:else}
		{#if selectedIds.size > 0}
			<!-- Floating bulk action bar -->
			<div class="bulk-bar-backdrop"></div>
			<div class="bulk-bar">
				<span class="bulk-count">{selectedIds.size} selected</span>
				<div class="bulk-actions">
					<button onclick={confirmBulkDelete} class="bulk-btn-danger">
						<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
						Delete Selected
					</button>
					<button onclick={() => { selectedIds = new Set(); }} class="bulk-btn-close" title="Deselect all">
						<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
					</button>
				</div>
			</div>
		{/if}

		{#if servers.length === 0 && isFiltered}
			<div class="mb-4 rounded-xl border px-6 py-8 text-center" style="background-color: var(--color-card); border-color: var(--color-border-light);">
				<Icon icon="solar:server-square-bold" class="mb-2 inline-block h-10 w-10" style="color: var(--color-text-muted);" />
				<p class="text-sm" style="color: var(--color-text-muted);">No servers matching filters</p>
			</div>
		{:else}
			<!-- Select All -->
			<div class="mb-3 flex items-center gap-2 px-1">
				<label class="flex items-center gap-1.5 cursor-pointer select-none text-xs font-medium" style="color: var(--color-text-muted);">
					<input
						type="checkbox"
						checked={allSelected}
						onchange={toggleSelectAll}
						class="h-4 w-4 rounded"
						style="accent-color: var(--color-primary);"
					/>
					Select All
				</label>
			</div>
			<!-- Server Card Grid -->
			<div class="server-grid">
				{#each servers as server}
					<div class="server-card" data-server-id={server.id}>
						<!-- Checkbox -->
						<!-- svelte-ignore a11y_click_events_have_key_events -->
						<div class="sc-check" onclick={(e) => e.stopPropagation()}>
							<input
								type="checkbox"
								checked={selectedIds.has(server.id)}
								onchange={() => toggleSelect(server.id)}
								class="h-4 w-4 rounded"
								style="accent-color: var(--color-primary);"
							/>
						</div>

						<!-- Status icon -->
						<div class="sc-icon {statusClass(server.status)}">
							<Icon
								icon={server.status === 'online' ? 'solar:check-circle-bold' : server.status === 'offline' ? 'solar:close-circle-bold' : 'solar:clock-circle-bold'}
								class="h-5 w-5" />
						</div>

						<!-- Name + host -->
						<div class="sc-info">
							<div class="sc-name-row">
								<span class="sc-name">{server.name}</span>
								{#if server.is_self}
									<span class="self-badge" title="This host">Self</span>
								{/if}
								<span class="status-badge {statusClass(server.status)}">
									<span class="h-1.5 w-1.5 rounded-full currentColor"></span>
									{server.status || 'unknown'}
								</span>
							</div>
							<div class="sc-host">
								{#if server.connection_type === 'docker-socket'}
									<span>🐳 Docker Socket</span>
									<span class="sc-user">· {server.self_hostname || server.host}</span>
								{:else}
									{server.host}:{server.port || 22}
									<span class="sc-user">· {server.ssh_user || 'root'}</span>
								{/if}
							</div>
						</div>

						<!-- Meta row: Group · Type · Region -->
						<div class="sc-meta">
							<span class="sc-meta-item">
								<span class="sc-meta-label">Group</span>
								{server.server_group || '-'}
							</span>
							<span class="sc-meta-divider"></span>
							<span class="sc-meta-item">
								<span class="sc-meta-label">Type</span>
								{server.server_type || '-'}
							</span>
							<span class="sc-meta-divider"></span>
							<span class="sc-meta-item">
								<span class="sc-meta-label">Region</span>
								{server.region || '-'}
							</span>
						</div>

						<!-- Tags -->
						<div class="sc-tags">
							{#if server.tags && server.tags.length > 0}
								{#each server.tags.slice(0, 3) as tag}
									<span class="sc-tag">{tag}</span>
								{/each}
								{#if server.tags.length > 3}
									<span class="sc-tag-more">+{server.tags.length - 3}</span>
								{/if}
							{:else}
								<span class="sc-meta-label">No tags</span>
							{/if}
						</div>

						<!-- Compliance row -->
						{#if server.score !== undefined && server.score !== null}
							{@const sColor = scoreColor(server.score)}
							{@const pct = Math.min(100, Math.round(((server.passed || 0) / Math.max(1, (server.passed || 0) + (server.warnings || 0) + (server.criticals || 0))) * 100))}
							<div class="sc-compliance">
								<div class="sc-comp-score" style="color: {sColor}; background-color: {sColor}18;">
									{server.score}
								</div>
								<div class="sc-comp-bars">
									<div class="progress-track">
										<div class="progress-fill" style="width: {pct}%; background: {sColor};"></div>
									</div>
									<div class="sc-comp-stats">
										<span class="sc-comp-pass">✓{server.passed || 0}</span>
										{#if server.warnings > 0}
											<span class="sc-comp-warn">⚠{server.warnings}</span>
										{/if}
										{#if server.criticals > 0}
											<span class="sc-comp-fail">✗{server.criticals}</span>
										{/if}
									</div>
								</div>
							</div>
						{/if}

						<!-- Bottom: collapsible actions -->
						<div class="sc-bottom">
							<!-- svelte-ignore a11y_click_events_have_key_events -->
							<div class="sc-actions-trigger" onclick={(e) => { e.stopPropagation(); toggleActions(server.id); }}>
								<button class="sc-btn sc-btn-actions" class:expanded={expandedCard === server.id}>
									<Icon icon="solar:alt-arrow-down-bold" class="h-3.5 w-3.5" style={expandedCard === server.id ? 'transform: rotate(180deg);' : ''} />
									<span>Actions</span>
								</button>
							</div>
							{#if server.score !== undefined && server.score !== null}
								<div class="sc-bottom-meta">
									{#if server.last_scan}
										<span class="sc-last-scan" title={new Date(server.last_scan).toLocaleString()}>
											<Icon icon="solar:history-bold" class="h-3 w-3" />
											{formatTime(server.last_scan)}
										</span>
									{/if}
								</div>
							{:else}
								<span class="sc-added">Added {formatDate(server.created_at)}</span>
							{/if}
						</div>

						<!-- Expanded action panel -->
						{#if expandedCard === server.id}
							<!-- svelte-ignore a11y_click_events_have_key_events -->
							<div class="sc-actions-panel" onclick={(e) => e.stopPropagation()}>
								<div class="sc-ap-grid">
									<button onclick={() => testConnection(server.id)} class="sc-ap-btn" title="Test Connection">
										<Icon icon="solar:plug-circle-bold" class="h-4 w-4" />
										<span>Test</span>
									</button>
									<button onclick={() => goToDetail(server.id)} class="sc-ap-btn" title="View Details">
										<Icon icon="solar:eye-bold" class="h-4 w-4" />
										<span>Detail</span>
									</button>
									<button onclick={() => goToTerminal(server.id)} class="sc-ap-btn" title="Open Terminal">
										<Icon icon="solar:code-bold" class="h-4 w-4" />
										<span>Terminal</span>
									</button>
									<button onclick={() => handleEdit(server)} class="sc-ap-btn" title="Edit Server">
										<Icon icon="solar:pen-bold" class="h-4 w-4" />
										<span>Edit</span>
									</button>
									<button onclick={() => confirmDelete(server)} class="sc-ap-btn sc-ap-btn-danger" title="Delete Server">
										<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
										<span>Delete</span>
									</button>
								</div>
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}

		<!-- Pagination -->
		{#if totalPages > 1}
			<div class="mt-4 flex items-center justify-between">
				<p class="text-xs" style="color: var(--color-text-muted);">
					Page {page} of {totalPages} ({total} total)
				</p>
				<div class="flex items-center gap-1">
					<button onclick={() => goToPage(1)} disabled={page <= 1} class="btn-icon" title="First page">
						<Icon icon="solar:multiple-forward-left-bold" class="h-4 w-4" />
					</button>
					<button onclick={() => goToPage(page - 1)} disabled={page <= 1} class="btn-icon" title="Previous">
						<Icon icon="solar:alt-arrow-left-bold" class="h-4 w-4" />
					</button>

					{#each Array.from({length: Math.min(totalPages, 7)}, (_, i) => {
						const start = Math.max(1, Math.min(page - 3, totalPages - 6));
						return start + i;
					}) as p}
						<button
							onclick={() => goToPage(p)}
							class="min-w-[32px] rounded-lg px-2 py-1 text-sm font-medium transition-colors"
							style="background-color: {p === page ? 'var(--color-primary)' : 'transparent'}; color: {p === page ? 'white' : 'var(--color-text-secondary)'};"
						>
							{p}
						</button>
					{/each}

					<button onclick={() => goToPage(page + 1)} disabled={page >= totalPages} class="btn-icon" title="Next">
						<Icon icon="solar:alt-arrow-right-bold" class="h-4 w-4" />
					</button>
					<button onclick={() => goToPage(totalPages)} disabled={page >= totalPages} class="btn-icon" title="Last page">
						<Icon icon="solar:multiple-forward-right-bold" class="h-4 w-4" />
					</button>
				</div>
				<select bind:value={limit} onchange={loadServers} class="input text-xs w-20">
					<option value="10">10/page</option>
					<option value="20">20/page</option>
					<option value="50">50/page</option>
					<option value="100">100/page</option>
				</select>
			</div>
		{/if}
	{/if}
</div>

<style>
	/* ─── Server Card Grid ──────────────────────────────────── */
	.server-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
		gap: 16px;
	}

	.server-card {
		background-color: var(--color-card);
		border: 1px solid var(--color-border);
		border-radius: 12px;
		padding: 0;
		cursor: default;
		transition: all 0.15s ease;
		box-shadow: 0 1px 3px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04);
		position: relative;
		overflow: visible;
		display: flex;
		flex-direction: column;
	}
	.server-card:hover {
		box-shadow: 0 4px 12px rgba(0,0,0,0.08), 0 2px 4px rgba(0,0,0,0.04);
		border-color: var(--color-primary);
	}

	/* Checkbox */
	.sc-check {
		position: absolute;
		top: 12px;
		left: 12px;
		z-index: 2;
	}
	.sc-check input {
		accent-color: var(--color-primary);
	}

	/* Status icon */
	.sc-icon {
		position: absolute;
		top: 12px;
		right: 12px;
		width: 36px;
		height: 36px;
		border-radius: 10px;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.sc-icon.online {
		background-color: rgba(16, 185, 129, 0.1);
		color: var(--color-success);
	}
	.sc-icon.offline {
		background-color: rgba(239, 68, 68, 0.1);
		color: var(--color-danger);
	}
	.sc-icon.pending {
		background-color: rgba(245, 158, 11, 0.1);
		color: var(--color-warning);
	}

	/* Info section */
	.sc-info {
		padding: 16px 16px 8px 44px;
		flex: 1;
	}
	.sc-name-row {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}
	.sc-name {
		font-size: 15px;
		font-weight: 600;
		color: var(--color-text);
		line-height: 1.3;
	}
	.sc-host {
		font-size: 12px;
		font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
		color: var(--color-text-muted);
		margin-top: 3px;
	}
	.sc-user {
		color: var(--color-text-muted);
		font-family: inherit;
	}

	/* Meta row */
	.sc-meta {
		padding: 8px 16px;
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}
	.sc-meta-item {
		font-size: 11px;
		font-weight: 500;
		color: var(--color-text-secondary);
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}
	.sc-meta-label {
		color: var(--color-text-muted);
		font-weight: 400;
	}
	.sc-meta-divider {
		width: 3px;
		height: 3px;
		border-radius: 50%;
		background-color: var(--color-border);
		display: inline-block;
	}

	/* Tags */
	.sc-tags {
		padding: 4px 16px 8px;
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}
	.sc-tag {
		font-size: 10px;
		padding: 2px 7px;
		border-radius: 4px;
		background-color: var(--color-primary-subtle);
		color: var(--color-primary);
		font-weight: 500;
	}
	.sc-tag-more {
		font-size: 10px;
		padding: 2px 7px;
		color: var(--color-text-muted);
		font-weight: 500;
	}

	/* Bottom bar */
	.sc-bottom {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 8px 16px;
		background-color: var(--color-border-light);
		border-top: 1px solid var(--color-border);
		margin-top: auto;
	}
	.sc-actions-trigger {
		display: flex;
	}
	.sc-btn {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 5px 10px;
		border: 1px solid var(--color-border);
		border-radius: 6px;
		background-color: var(--color-card);
		cursor: pointer;
		font-family: inherit;
		font-size: 11px;
		font-weight: 500;
		color: var(--color-text-secondary);
		transition: all 0.12s ease;
		line-height: 1;
	}
	.sc-btn:hover {
		background-color: var(--color-primary-subtle);
		color: var(--color-primary);
		border-color: var(--color-primary);
	}
	.sc-btn-actions.expanded {
		background-color: var(--color-primary-subtle);
		color: var(--color-primary);
		border-color: var(--color-primary);
	}
	.sc-added {
		font-size: 11px;
		color: var(--color-text-muted);
		white-space: nowrap;
	}

	/* Expanded actions panel */
	.sc-actions-panel {
		padding: 10px;
		background-color: var(--color-card);
		border-radius: 0 0 12px 12px;
		border-top: 1px solid var(--color-border-light);
	}
	.sc-ap-grid {
		display: grid;
		grid-template-columns: 1fr 1fr 1fr;
		gap: 6px;
	}
	.sc-ap-btn {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		padding: 10px 6px;
		border: 1.5px solid var(--color-border);
		border-radius: 8px;
		background-color: var(--color-surface);
		cursor: pointer;
		font-family: inherit;
		font-size: 10px;
		font-weight: 500;
		color: var(--color-text-secondary);
		transition: all 0.15s ease;
		line-height: 1.2;
		min-height: 54px;
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
	}
	.sc-ap-btn:hover {
		background-color: var(--color-primary-subtle);
		color: var(--color-primary);
		border-color: var(--color-primary);
		box-shadow: 0 1px 4px rgba(13, 148, 136, 0.15);
	}
	.sc-ap-btn-danger {
		border-color: rgba(239, 68, 68, 0.3);
		color: var(--color-danger);
	}
	.sc-ap-btn-danger:hover {
		background-color: rgba(239, 68, 68, 0.1);
		border-color: var(--color-danger);
		color: var(--color-danger);
		box-shadow: 0 1px 4px rgba(239, 68, 68, 0.15);
	}

	/* ─── Floating Bulk Action Bar ────────────────────────────────── */
	.page-container {
		padding-bottom: 80px;
	}
	.bulk-bar-backdrop {
		position: fixed;
		bottom: 0;
		left: 0;
		right: 0;
		height: 80px;
		background: linear-gradient(transparent, var(--color-bg));
		pointer-events: none;
		z-index: 40;
	}
	.bulk-bar {
		position: fixed;
		bottom: 0;
		left: 0;
		right: 0;
		z-index: 50;
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 16px;
		padding-bottom: calc(12px + env(safe-area-inset-bottom, 0px));
		background-color: var(--color-surface);
		border-top: 1px solid var(--color-border);
		box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.12);
		backdrop-filter: blur(12px);
		-webkit-backdrop-filter: blur(12px);
	}
	.bulk-count {
		font-size: 14px;
		font-weight: 600;
		color: var(--color-text);
	}
	.bulk-actions {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.bulk-btn-danger {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 8px 14px;
		border: 1.5px solid rgba(239, 68, 68, 0.3);
		border-radius: 8px;
		background-color: rgba(239, 68, 68, 0.08);
		color: var(--color-danger);
		font-size: 13px;
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
		transition: all 0.15s ease;
		white-space: nowrap;
	}
	.bulk-btn-danger:hover {
		background-color: rgba(239, 68, 68, 0.15);
		border-color: var(--color-danger);
	}
	.bulk-btn-close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 6px;
		border: none;
		border-radius: 50%;
		background: transparent;
		color: var(--color-text-muted);
		cursor: pointer;
		transition: all 0.15s ease;
	}
	.bulk-btn-close:hover {
		background-color: var(--color-border-light);
		color: var(--color-text);
	}

	/* ─── Self Badge ──────────────────────────────────────────── */
	.self-badge {
		display: inline-flex;
		align-items: center;
		padding: 2px 7px;
		border-radius: 6px;
		font-size: 10px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		background-color: rgba(16, 185, 129, 0.12);
		color: #10b981;
		border: 1px solid rgba(16, 185, 129, 0.3);
		line-height: 1.2;
	}

	/* ─── Compliance Row ─────────────────────────────────────── */
	.sc-compliance {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 6px 16px 6px;
	}
	.sc-comp-score {
		width: 32px;
		height: 22px;
		border-radius: 6px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 11px;
		font-weight: 800;
		flex-shrink: 0;
		line-height: 1;
	}
	.sc-comp-bars {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 3px;
	}
	.progress-track {
		height: 4px;
		border-radius: 2px;
		background: var(--color-border);
		overflow: hidden;
	}
	.progress-fill {
		height: 100%;
		border-radius: 2px;
		transition: width 0.3s;
	}
	.sc-comp-stats {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.sc-comp-stats span {
		font-size: 10px;
		font-weight: 500;
		line-height: 1;
	}
	.sc-comp-pass { color: var(--color-success); }
	.sc-comp-warn { color: var(--color-warning); }
	.sc-comp-fail { color: var(--color-danger); }

	/* ─── Bottom meta ─────────────────────────────────────────── */
	.sc-bottom-meta {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.sc-last-scan {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		font-size: 11px;
		color: var(--color-text-muted);
		white-space: nowrap;
	}

	/* Mobile: single column */
	@media (max-width: 640px) {
		.server-grid {
			grid-template-columns: 1fr;
			gap: 12px;
		}
		.sc-info {
			padding-left: 44px;
		}
	}
</style>
