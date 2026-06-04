<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';

	let entries = $state([]);
	let loading = $state(true);
	let error = $state('');
	let expandedRow = $state(null);

	// Filters
	let actionFilter = $state('');
	let entityFilter = $state('');
	let searchQuery = $state('');
	let startDate = $state('');
	let endDate = $state('');

	// Sort / order
	let sortColumn = $state('created_at');
	let sortOrder = $state('desc');

	function toggleSort(col) {
		if (sortColumn === col) {
			sortOrder = sortOrder === 'desc' ? 'asc' : 'desc';
		} else {
			sortColumn = col;
			sortOrder = 'desc';
		}
		page = 1;
		loadLogs();
	}

	function sortIcon(col) {
		if (sortColumn !== col) return 'solar:arrow-up-wide-narrow-linear';
		return sortOrder === 'desc' ? 'solar:sort-from-top-bold' : 'solar:sort-from-bottom-bold';
	}

	// Filter options
	let actionOptions = $state([]);
	let entityOptions = $state([]);

	// Pagination
	let page = $state(1);
	let total = $state(0);
	let totalPages = $state(1);
	const limit = 50;

	let isFiltered = $derived(actionFilter || entityFilter || searchQuery || startDate || endDate);

	onMount(async () => {
		await Promise.all([loadLogs(), loadFilterOptions()]);
	});

	async function loadFilterOptions() {
		try {
			const [actions, types] = await Promise.all([
				api.admin.auditLog.actions().catch(() => []),
				api.admin.auditLog.entityTypes().catch(() => []),
			]);
			actionOptions = actions || [];
			entityOptions = types || [];
		} catch (_) {}
	}

	async function loadLogs() {
		loading = true;
		error = '';
		expandedRow = null;
		try {
			const result = await api.admin.auditLog.list({
				page,
				limit,
				action: actionFilter || undefined,
				entity_type: entityFilter || undefined,
				search: searchQuery || undefined,
				start_date: startDate || undefined,
				end_date: endDate || undefined,
				sort: sortColumn,
				order: sortOrder,
			});
			entries = result || [];
			if (result && result._meta) {
				total = result._meta.total || 0;
				totalPages = result._meta.total_pages || 1;
			}
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function search() {
		page = 1;
		loadLogs();
	}

	function resetFilters() {
		actionFilter = '';
		entityFilter = '';
		searchQuery = '';
		startDate = '';
		endDate = '';
		page = 1;
		loadLogs();
	}

	function goToPage(p) {
		if (p < 1 || p > totalPages) return;
		page = p;
		loadLogs();
	}

	function toggleRow(id) {
		expandedRow = expandedRow === id ? null : id;
	}

	function actionIcon(action) {
		if (action.startsWith('user.')) return 'solar:user-bold';
		if (action.startsWith('server.')) return 'solar:server-square-bold';
		if (action.startsWith('ssh-key.')) return 'solar:key-minimalistic-bold';
		if (action.startsWith('auth.')) return 'solar:login-2-bold';
		if (action.startsWith('deployment.')) return 'solar:cloud-upload-bold';
		if (action.startsWith('container.')) return 'solar:box-bold';
		if (action.startsWith('registry.')) return 'solar:archive-down-minimlistic-bold';
		return 'solar:notes-bold';
	}

	function actionColor(action) {
		if (action.includes('create') || action.includes('register') || action.includes('login')) return 'var(--color-success)';
		if (action.includes('delete') || action.includes('logout') || action.includes('remove')) return 'var(--color-danger)';
		if (action.includes('update') || action.includes('edit')) return 'var(--color-warning)';
		return 'var(--color-text-secondary)';
	}

	function formatTime(ts) {
		const d = new Date(ts);
		return d.toLocaleString();
	}

	function exportLogs(format) {
		api.admin.auditLog.export({
			action: actionFilter || undefined,
			entity_type: entityFilter || undefined,
			search: searchQuery || undefined,
			start_date: startDate || undefined,
			end_date: endDate || undefined,
			format,
		});
	}

	function parseMetadata(raw) {
		if (!raw || raw === '{}') return null;
		try {
			const obj = typeof raw === 'string' ? JSON.parse(raw) : raw;
			if (Object.keys(obj).length === 0) return null;
			return obj;
		} catch {
			return null;
		}
	}

	function metadataPairs(meta) {
		if (!meta) return [];
		return Object.entries(meta).filter(([_, v]) => v !== null && v !== '');
	}

	function descriptionDetail(entry) {
		const meta = parseMetadata(entry.metadata);
		if (!meta) return null;

		// Pick the most useful fields to show as detail
		switch (entry.action) {
			case 'auth.login':
			case 'auth.register':
				return meta.user_name ? `${meta.user_name} (${meta.user_role})` : null;
			case 'auth.logout':
				return meta.user_name ? `user:${meta.user_name}` : null;
			case 'server.create':
			case 'server.update':
				const parts = [];
				if (meta.server_port) parts.push(`port:${meta.server_port}`);
				if (meta.server_type) parts.push(meta.server_type);
				if (meta.server_group) parts.push(meta.server_group);
				if (meta.region) parts.push(meta.region);
				if (meta.changed_fields) parts.push(`changed:${meta.changed_fields.join(',')}`);
				return parts.length > 0 ? parts.join(' · ') : null;
			case 'server.delete':
				return meta.server_name ? `name:${meta.server_name}` : null;
			case 'server.bulk-delete':
				return meta.count ? `${meta.count} servers` : null;
			case 'server.test':
				return meta.reachable === 'true' ? '✅ reachable' : '❌ unreachable';
			case 'ssh-key.create':
			case 'ssh-key.update':
				const keyParts = [];
				if (meta.key_type) keyParts.push(meta.key_type);
				if (meta.fingerprint) keyParts.push(meta.fingerprint.substring(0, 20) + '…');
				return keyParts.length > 0 ? keyParts.join(' · ') : null;
			case 'ssh-key.delete':
				return meta.key_name ? `name:${meta.key_name}` : null;
			case 'container.start':
				return meta.container_name ? `container:${meta.container_name}` : null;
			case 'container.stop':
				return meta.container_name ? `container:${meta.container_name}` : null;
			case 'container.restart':
				return meta.container_name ? `container:${meta.container_name}` : null;
			case 'container.delete':
				return meta.container_name ? `container:${meta.container_name}` : null;
			case 'deployment.create':
			case 'deployment.deploy':
				return meta.app_name ? `app:${meta.app_name}` : (meta.stack ? `stack:${meta.stack}` : null);
			case 'deployment.remove':
				return meta.app_name ? `app:${meta.app_name}` : null;
			case 'user.create':
				return meta.user_email ? `email:${meta.user_email}` : null;
			case 'user.update':
				const changed = meta.changed_fields ? `changed:${meta.changed_fields.join(',')}` : '';
				return meta.user_email ? `${meta.user_email}${changed ? ' · ' + changed : ''}` : null;
			case 'user.delete':
				return meta.user_email ? `email:${meta.user_email}` : null;
			default:
				// For any action with metadata, show first useful field
				if (meta) {
					const pairs = metadataPairs(meta);
					if (pairs.length > 0) {
						const [k, v] = pairs[0];
						if (v && typeof v !== 'object') return `${k}:${String(v).substring(0, 40)}`;
					}
				}
				return null;
		}
	}
</script>

<div class="page-container">
	<div>
		<h1 class="page-title">Audit Log</h1>
		<p class="page-subtitle">Track all administrative actions across the platform</p>
	</div>

	<!-- Filters -->
	<div class="card">
		<div class="grid grid-cols-1 gap-3 md:grid-cols-5">
			<div>
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Action</label>
				<select bind:value={actionFilter} class="input text-sm">
					<option value="">All Actions</option>
					{#each actionOptions as a}
						<option value={a}>{a}</option>
					{/each}
				</select>
			</div>
			<div>
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Entity</label>
				<select bind:value={entityFilter} class="input text-sm">
					<option value="">All Entities</option>
					{#each entityOptions as t}
						<option value={t}>{t}</option>
					{/each}
				</select>
			</div>
			<div>
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Search</label>
				<input bind:value={searchQuery} class="input text-sm" placeholder="Description, email, ID..."
					onkeydown={(e) => { if (e.key === 'Enter') search(); }} />
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
			{#if !loading}
				<div class="flex items-center gap-2 flex-wrap">
					<span class="text-xs whitespace-nowrap" style="color: var(--color-text-muted);">{total} entries</span>
					<div class="h-4 w-px hidden sm:block" style="background-color: var(--color-border);"></div>
					<button onclick={() => exportLogs('csv')} class="btn-ghost text-xs flex items-center gap-1 py-1 px-2">
						<Icon icon="solar:export-bold" class="h-3 w-3" />
						Export CSV
					</button>
					<button onclick={() => exportLogs('json')} class="btn-ghost text-xs flex items-center gap-1 py-1 px-2">
						<Icon icon="solar:export-bold" class="h-3 w-3" />
						Export JSON
					</button>
				</div>
			{/if}
		</div>
	</div>

	<!-- Table -->
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading audit log...</p>
			</div>
		</div>
	{:else if error}
		<div class="card flex flex-col items-center gap-3 py-10 text-center">
			<Icon icon="solar:danger-triangle-bold" class="mb-1 h-8 w-8" style="color: var(--color-danger);" />
			<p style="color: var(--color-danger);">Failed to load audit log</p>
			<p class="text-sm" style="color: var(--color-text-muted);">{error}</p>
			<button onclick={loadLogs} class="btn-secondary mt-2">Retry</button>
		</div>
	{:else if entries.length === 0}
		<div class="card flex flex-col items-center py-16 text-center">
			<Icon icon="solar:notes-bold" class="mb-3 h-12 w-12" style="color: var(--color-text-muted);" />
			<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No entries found</h3>
			<p class="text-sm" style="color: var(--color-text-secondary);">
				{isFiltered ? 'Try different filter criteria.' : 'Audit log will populate as actions are performed.'}
			</p>
		</div>
	{:else}
		<div class="data-table">
			<table class="w-full">
				<thead>
					<tr>
						<th class="w-8"></th>
						<th class="hidden md:table-cell" style="cursor: pointer;" onclick={() => toggleSort('created_at')}>
							<div class="flex items-center gap-1">
								Time
								<Icon icon={sortIcon('created_at')} class="h-3 w-3" />
							</div>
						</th>
						<th style="cursor: pointer;" onclick={() => toggleSort('action')}>
							<div class="flex items-center gap-1">
								Action
								<Icon icon={sortIcon('action')} class="h-3 w-3" />
							</div>
						</th>
						<th class="hidden lg:table-cell" style="cursor: pointer;" onclick={() => toggleSort('entity_type')}>
							<div class="flex items-center gap-1">
								Entity
								<Icon icon={sortIcon('entity_type')} class="h-3 w-3" />
							</div>
						</th>
						<th style="cursor: pointer;" onclick={() => toggleSort('description')}>
							<div class="flex items-center gap-1">
								Description
								<Icon icon={sortIcon('description')} class="h-3 w-3" />
							</div>
						</th>
						<th class="hidden md:table-cell" style="cursor: pointer;" onclick={() => toggleSort('user_email')}>
							<div class="flex items-center gap-1">
								User
								<Icon icon={sortIcon('user_email')} class="h-3 w-3" />
							</div>
						</th>
						<th class="hidden lg:table-cell" style="cursor: pointer;" onclick={() => toggleSort('ip_address')}>
							<div class="flex items-center gap-1">
								IP
								<Icon icon={sortIcon('ip_address')} class="h-3 w-3" />
							</div>
						</th>
						<th class="w-10"></th>
					</tr>
				</thead>
				<tbody>
					{#each entries as entry (entry.id)}
						<tr class="cursor-pointer row-group" onclick={() => toggleRow(entry.id)}
							style={expandedRow === entry.id ? `background-color: rgba(16, 185, 129, 0.05); border-bottom-color: transparent;` : ''}>
								<td>
									<Icon icon={actionIcon(entry.action)} class="h-4 w-4" style="color: {actionColor(entry.action)};" />
								</td>
								<td class="hidden text-sm md:table-cell whitespace-nowrap" style="color: var(--color-text-muted);">
									{formatTime(entry.created_at)}
								</td>
								<td>
									<span class="code-inline">{entry.action}</span>
								</td>
								<td class="hidden lg:table-cell">
									{#if entry.entity_type}
										<div class="flex items-center gap-1.5">
											<span class="badge badge-ghost text-xs">{entry.entity_type}</span>
											{#if entry.entity_id}
												<span class="font-mono text-xs" style="color: var(--color-text-muted);">{entry.entity_id}</span>
											{/if}
										</div>
									{:else}
										<span style="color: var(--color-text-muted);">—</span>
									{/if}
								</td>
								<td class="max-w-xs" style="color: var(--color-text);">
									<div class="truncate" title={entry.description}>{entry.description}</div>
									{#if descriptionDetail(entry)}
										<div class="text-xs mt-0.5 truncate" style="color: var(--color-text-muted);">{descriptionDetail(entry)}</div>
									{/if}
								</td>
								<td class="hidden md:table-cell" style="color: var(--color-text-secondary);">
									{entry.user_email || '-'}
								</td>
								<td class="hidden font-mono text-xs lg:table-cell" style="color: var(--color-text-muted);">
									{entry.ip_address || '-'}
								</td>
								<td>
									<button class="btn-icon h-7 w-7" onclick={(e) => { e.stopPropagation(); toggleRow(entry.id); }}>
										<Icon icon={expandedRow === entry.id ? 'solar:alt-arrow-up-bold' : 'solar:alt-arrow-down-bold'} class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
									</button>
								</td>
							</tr>
							{#if expandedRow === entry.id}
								<tr class="detail-row">
									<td colspan="8">
										<div class="detail-panel" style="border-left: 3px solid {actionColor(entry.action)};">
											<div class="detail-grid">
												<!-- Entity Info -->
												<div class="detail-section">
													<h4 class="detail-section-title">
														<Icon icon="solar:info-circle-bold" class="h-3.5 w-3.5" />
														Entity
													</h4>
													<div class="detail-fields">
														<div class="detail-field">
															<span class="detail-label">Type</span>
															<span class="detail-value">
																<span class="badge badge-ghost text-xs">{entry.entity_type || '—'}</span>
															</span>
														</div>
														<div class="detail-field">
															<span class="detail-label">ID</span>
															<span class="detail-value code-inline text-xs">{entry.entity_id || '—'}</span>
														</div>
														<div class="detail-field">
															<span class="detail-label">User ID</span>
															<span class="detail-value code-inline text-xs">{entry.user_id || '—'}</span>
														</div>
													</div>
												</div>

												<!-- Request Info -->
												<div class="detail-section">
													<h4 class="detail-section-title">
														<Icon icon="solar:global-bold" class="h-3.5 w-3.5" />
														Request
													</h4>
													<div class="detail-fields">
														<div class="detail-field">
															<span class="detail-label">IP Address</span>
															<span class="detail-value code-inline text-xs">{entry.ip_address || '—'}</span>
														</div>
														<div class="detail-field">
															<span class="detail-label">User</span>
															<span class="detail-value text-xs">{entry.user_email || '—'}</span>
														</div>
														<div class="detail-field">
															<span class="detail-label">Timestamp</span>
															<span class="detail-value text-xs">{formatTime(entry.created_at)}</span>
														</div>
													</div>
												</div>

												<!-- Metadata (only if non-empty) -->
												{#if parseMetadata(entry.metadata)}
													<div class="detail-section md:col-span-2">
														<h4 class="detail-section-title">
															<Icon icon="solar:code-bold" class="h-3.5 w-3.5" />
															Event Details
														</h4>
														<div class="detail-fields grid grid-cols-1 sm:grid-cols-2">
															{#each metadataPairs(parseMetadata(entry.metadata)) as [key, value]}
																<div class="detail-field">
																	<span class="detail-label">{key}</span>
																	<span class="detail-value text-xs font-mono" style="word-break: break-all;">{typeof value === 'object' ? JSON.stringify(value) : String(value)}</span>
																</div>
															{/each}
														</div>
													</div>
												{/if}
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
			<div class="flex items-center justify-center gap-2">
				<button onclick={() => goToPage(page - 1)} disabled={page <= 1} class="btn-icon h-8 w-8">
					<Icon icon="solar:alt-arrow-left-bold" class="h-4 w-4" />
				</button>
				{#each Array.from({length: Math.min(totalPages, 7)}, (_, i) => {
					const start = Math.max(1, page - 3);
					const p = start + i;
					if (p > totalPages) return null;
					return p;
				}).filter(Boolean) as p}
					<button onclick={() => goToPage(p)}
						class="btn-icon h-8 w-8 text-sm font-medium"
						class:active-page={p === page}
						style={p === page ? 'background-color: var(--color-primary); color: white;' : ''}>
						{p}
					</button>
				{/each}
				<button onclick={() => goToPage(page + 1)} disabled={page >= totalPages} class="btn-icon h-8 w-8">
					<Icon icon="solar:alt-arrow-right-bold" class="h-4 w-4" />
				</button>
			</div>
		{/if}
	{/if}
</div>

<style>
	.row-group {
		border-bottom: 1px solid var(--color-border);
	}
	.row-group:last-child {
		border-bottom: none;
	}
	.row-group:hover {
		background-color: var(--color-surface-hover);
	}

	.detail-row td {
		padding: 0 !important;
		border-bottom: 1px solid var(--color-border);
	}

	.detail-panel {
		background-color: var(--color-surface-alt);
		border-top: 1px solid var(--color-border);
		padding: 1rem 1.25rem;
	}

	.detail-grid {
		display: grid;
		grid-template-columns: 1fr;
		gap: 1rem;
	}

	@media (min-width: 768px) {
		.detail-grid {
			grid-template-columns: 1fr 1fr;
		}
	}

	.detail-section {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.detail-section-title {
		display: flex;
		align-items: center;
		gap: 0.375rem;
		font-size: 0.6875rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
		padding-bottom: 0.375rem;
		border-bottom: 1px solid var(--color-border);
	}

	.detail-fields {
		display: flex;
		flex-direction: column;
		gap: 0.375rem;
	}

	.detail-field {
		display: flex;
		align-items: baseline;
		gap: 0.5rem;
	}

	.detail-label {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		min-width: 70px;
		flex-shrink: 0;
	}

	.detail-value {
		color: var(--color-text);
	}

	.badge-ghost {
		display: inline-flex;
		align-items: center;
		padding: 0.125rem 0.5rem;
		border-radius: 9999px;
		background-color: var(--color-surface);
		border: 1px solid var(--color-border);
		color: var(--color-text-secondary);
		font-weight: 500;
	}
</style>
