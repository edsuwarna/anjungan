<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import { user } from '$lib/stores/auth.js';
	import Icon from '@iconify/svelte';

	let events = $state([]);
	let loading = $state(true);
	let error = $state('');
	let expandedRow = $state(null);

	// Filters
	let eventTypeFilter = $state('');
	let statusFilter = $state('');
	let searchQuery = $state('');

	// Sort
	let sortColumn = $state('created_at');
	let sortOrder = $state('desc');

	// Pagination
	let page = $state(1);
	let total = $state(0);
	let totalPages = $state(1);
	const limit = 30;

	let myEmail = $derived($user?.email || '');

	const EVENT_TYPES = [
		{ value: '', label: 'All Events' },
		{ value: 'login_success', label: 'Login Success' },
		{ value: 'login_failure', label: 'Login Failure' },
		{ value: 'login_attempt', label: 'Login Attempt' },
		{ value: 'logout', label: 'Logout' },
		{ value: 'password_change', label: 'Password Change' },
		{ value: 'totp_setup', label: 'TOTP Setup' },
		{ value: 'totp_disable', label: 'TOTP Disable' },
	];

	onMount(() => {
		loadEvents();
	});

	async function loadEvents() {
		loading = true;
		error = '';
		expandedRow = null;
		try {
			const result = await api.loginHistory.events({
				page,
				limit,
				event_type: eventTypeFilter || undefined,
				status: statusFilter || undefined,
				email: myEmail,
				search: searchQuery || undefined,
				sort: sortColumn,
				order: sortOrder,
			});
			events = result?.data || result || [];
			if (result?._meta) {
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
		loadEvents();
	}

	function resetFilters() {
		eventTypeFilter = '';
		statusFilter = '';
		searchQuery = '';
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
			password_change: { label: 'Password Change', icon: 'solar:key-minimalistic-bold', color: 'var(--color-warning)' },
			totp_setup: { label: 'TOTP Setup', icon: 'solar:password-minimalistic-bold', color: 'var(--color-primary)' },
			totp_disable: { label: 'TOTP Disable', icon: 'solar:password-minimalistic-bold', color: 'var(--color-text-secondary)' },
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
		return new Date(ts).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
	}
</script>

<div class="page-container">
	<div class="mb-6">
		<h1 class="page-title">My Login History</h1>
		<p class="page-subtitle">Review authentication events on your account</p>
	</div>

	<!-- Filters -->
	<div class="card mb-6" style="border-left: 3px solid var(--color-primary);">
		<div class="grid grid-cols-1 gap-3 md:grid-cols-4">
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
				<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Search</label>
				<input bind:value={searchQuery} class="input text-sm" placeholder="IP, reason..."
					onkeydown={(e) => { if (e.key === 'Enter') search(); }} />
			</div>
			<div class="flex items-end gap-2">
				<button onclick={search} class="btn-primary flex items-center gap-2">
					<Icon icon="solar:filter-bold" class="h-4 w-4" />
					Apply
				</button>
				{#if eventTypeFilter || statusFilter || searchQuery}
					<button onclick={resetFilters} class="btn-ghost text-sm">Clear</button>
				{/if}
			</div>
		</div>
	</div>

	<!-- Events Table -->
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading login history...</p>
			</div>
		</div>
	{:else if error}
		<div class="card flex flex-col items-center gap-3 py-10 text-center" style="border-left: 3px solid var(--color-danger);">
			<Icon icon="solar:danger-triangle-bold" class="mb-1 h-8 w-8" style="color: var(--color-danger);" />
			<p style="color: var(--color-danger);">Failed to load login history</p>
			<p class="text-sm" style="color: var(--color-text-muted);">{error}</p>
			<button onclick={loadEvents} class="btn-secondary mt-2">Retry</button>
		</div>
	{:else if events.length === 0}
		<div class="card flex flex-col items-center py-16 text-center" style="border-left: 3px solid var(--color-text-muted);">
			<Icon icon="solar:login-2-bold" class="mb-3 h-12 w-12" style="color: var(--color-text-muted);" />
			<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No events found</h3>
			<p class="text-sm" style="color: var(--color-text-secondary);">
				{eventTypeFilter || statusFilter || searchQuery ? 'Try different filter criteria.' : 'Your login events will appear here as you use the platform.'}
			</p>
		</div>
	{:else}
		<div class="data-table">
			<table class="w-full">
				<thead>
					<tr>
						<th class="w-6"></th>
						<th class="cursor-pointer" onclick={() => toggleSort('event_type')}>
							<div class="flex items-center gap-1">Event <Icon icon={sortIcon('event_type')} class="h-3 w-3" /></div>
						</th>
						<th>IP Address</th>
						<th class="cursor-pointer" onclick={() => toggleSort('status')}>
							<div class="flex items-center gap-1">Status <Icon icon={sortIcon('status')} class="h-3 w-3" /></div>
						</th>
						<th>Reason / Agent</th>
						<th class="cursor-pointer" onclick={() => toggleSort('created_at')}>
							<div class="flex items-center gap-1">Time <Icon icon={sortIcon('created_at')} class="h-3 w-3" /></div>
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
							<td class="font-mono text-xs">{e.ip_address || '-'}</td>
							<td>
								<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
									style="background-color: {statusBadge(e.status).color}15; color: {statusBadge(e.status).color};">
									<Icon icon={e.status === 'success' ? 'solar:check-circle-bold' : 'solar:close-circle-bold'} class="h-3 w-3" />
									{statusBadge(e.status).label}
								</span>
							</td>
							<td class="max-w-[200px] truncate text-xs" style="color: var(--color-text-secondary);" title={e.failure_reason || e.user_agent}>
								{e.failure_reason || e.user_agent || '-'}
							</td>
							<td class="text-xs whitespace-nowrap" style="color: var(--color-text-secondary);" title={formatTime(e.created_at)}>
								{shortTime(e.created_at)}
							</td>
						</tr>
						{#if expandedRow === e.id}
							<tr class="expanded-row">
								<td colspan="6">
									<div class="grid grid-cols-2 gap-3 p-3 text-xs md:grid-cols-3">
										<div>
											<span class="block font-medium" style="color: var(--color-text-secondary);">Event ID</span>
											<span class="font-mono" style="color: var(--color-text);">{e.id}</span>
										</div>
										<div>
											<span class="block font-medium" style="color: var(--color-text-secondary);">Country</span>
											<span>{e.country || '-'}</span>
										</div>
										<div>
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
				<span class="text-xs" style="color: var(--color-text-secondary);">Page {page} of {totalPages}</span>
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
