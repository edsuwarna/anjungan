<script>
	import { onMount } from 'svelte';
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';

	let repos = $state([]);
	let connections = $state([]);
	let loading = $state(true);
	let error = $state('');
	let filter = $state('all');
	let search = $state('');
	let expandedRepo = $state(null);
	let loadingDeployments = $state({});

	// Visibility selections state
	let selections = $state([]);
	let showManageModal = $state(false);
	let manageSearch = $state('');
	let manageSubmitting = $state(false);
	let manageItems = $state([]);

	// Connect modal state
	let showConnectModal = $state(false);
	let connectProvider = $state('github');
	let connectToken = $state('');
	let connectLabel = $state('');
	let connectBaseURL = $state('');
	let connectSubmitting = $state(false);
	let connectError = $state('');
	let connectSuccess = $state('');
	let connectAffiliations = $state(['owner', 'collaborator', 'organization_member']);

	const affiliationOptions = [
		{ value: 'owner', label: 'Personal repos', desc: 'Repos owned by you' },
		{ value: 'collaborator', label: 'Collaborator repos', desc: 'Repos you contribute to' },
		{ value: 'organization_member', label: 'Organization repos', desc: 'Repos in orgs you belong to' },
	];

	// Delete state
	let deletingId = $state(null);

	onMount(async () => {
		await loadData();
		await loadSelections();
	});

	async function loadData() {
		loading = true;
		error = '';
		try {
			const [repoData, connData] = await Promise.all([
				api.repositories.list(),
				api.repositories.connections.list(),
			]);
			repos = repoData.repositories || [];
			connections = connData.connections || [];
		} catch (e) {
			error = e.message || 'Failed to load repositories';
		} finally {
			loading = false;
		}
	}

	async function loadSelections() {
		try {
			const data = await api.repositories.selections.list();
			selections = data.selections || [];
		} catch (e) {
			// silently fail — selections just won't be preloaded
			selections = [];
		}
	}

	// Build a lookup map of provider/owner/name -> selected
	function selectionMap() {
		const map = {};
		for (const s of selections) {
			map[s.provider + '/' + s.owner + '/' + s.repo_name] = s.selected;
		}
		return map;
	}

	// Check if a repo is currently hidden (only after user has saved selections)
	function isRepoHidden(repo) {
		const selMap = selectionMap();
		const key = repo.provider + '/' + repo.owner + '/' + repo.name;
		if (key in selMap) {
			return !selMap[key];
		}
		return false; // not in selections = visible by default
	}

	let filtered = $derived.by(() => {
		let list = repos;
		if (filter !== 'all') {
			list = list.filter(r => r.provider === filter);
		}
		if (search) {
			const q = search.toLowerCase();
			list = list.filter(r =>
				r.full_name?.toLowerCase().includes(q) ||
				r.description?.toLowerCase().includes(q)
			);
		}
		return list;
	});

	function providerIcon(p) {
		return p === 'github' ? 'mdi:github' : 'simple-icons:forgejo';
	}

	function ciBadge(state) {
		if (state === 'success') return { icon: 'solar:check-circle-bold', cls: 'pass', label: '● Pass' };
		if (state === 'failure') return { icon: 'solar:close-circle-bold', cls: 'fail', label: '✕ Fail' };
		if (state === 'pending') return { icon: 'solar:clock-circle-bold', cls: 'pending', label: '◐ Pending' };
		return null;
	}

	function timeAgo(dateStr) {
		if (!dateStr) return '';
		const now = Date.now();
		const then = new Date(dateStr).getTime();
		const diff = now - then;
		const mins = Math.floor(diff / 60000);
		if (mins < 1) return 'just now';
		if (mins < 60) return `${mins}m ago`;
		if (mins < 1440) return `${Math.floor(mins / 60)}h ago`;
		const days = Math.floor(mins / 1440);
		if (days < 30) return `${days}d ago`;
		const months = Math.floor(days / 30);
		return `${months}mo ago`;
	}

	async function loadDeployments(repo) {
		if (expandedRepo === repo.full_name) {
			expandedRepo = null;
			return;
		}
		expandedRepo = repo.full_name;
		const key = repo.full_name;
		loadingDeployments[key] = true;
		try {
			const data = await api.repositories.deployments(repo.provider, repo.owner, repo.name);
			repo._deployments = data.deployments || [];
		} catch (e) {
			repo._deployments = [];
		} finally {
			loadingDeployments[key] = false;
		}
	}

	function statusBadge(status) {
		const map = {
			running: '● Running',
			success: '✓ Success',
			failed: '✕ Failed',
			pending: '◐ Pending',
			deploying: '◐ Deploying',
			rolled_back: '↩ Rolled Back',
		};
		return map[status] || status;
	}

	// ─── Connect / Disconnect ──────────────────────────────────

	function openConnectModal(provider) {
		connectProvider = provider || 'github';
		connectToken = '';
		connectLabel = '';
		connectBaseURL = '';
		connectError = '';
		connectSuccess = '';
		connectAffiliations = ['owner', 'collaborator', 'organization_member'];
		showConnectModal = true;
	}

	function closeConnectModal() {
		showConnectModal = false;
		connectError = '';
		connectSuccess = '';
	}

	async function submitConnect() {
		connectError = '';
		connectSuccess = '';
		if (!connectToken.trim()) {
			connectError = 'Token is required';
			return;
		}
		if (connectProvider === 'forgejo' && !connectBaseURL.trim()) {
			connectError = 'Instance URL is required for Forgejo';
			return;
		}
		connectSubmitting = true;
		try {
			const body = {
				provider: connectProvider,
				token: connectToken.trim(),
				label: connectLabel.trim() || undefined,
				base_url: connectBaseURL.trim() || undefined,
				affiliations: connectAffiliations,
			};
			await api.repositories.connections.create(body);
			connectSuccess = 'Account connected successfully!';
			connectToken = '';
			connectLabel = '';
			connectBaseURL = '';
			await loadData();
			setTimeout(() => {
				showConnectModal = false;
				connectSuccess = '';
			}, 1200);
		} catch (e) {
			connectError = e.message || 'Failed to connect';
		} finally {
			connectSubmitting = false;
		}
	}

	async function deleteConnection(id) {
		if (!confirm('Remove this connection?')) return;
		deletingId = id;
		try {
			await api.repositories.connections.delete(id);
			await loadData();
		} catch (e) {
			// silent
		} finally {
			deletingId = null;
		}
	}

	// ─── Visibility Selections ──────────────────────────────────

	async function toggleRepoVisibility(repo) {
		const key = repo.provider + '/' + repo.owner + '/' + repo.name;
		const selMap = selectionMap();
		const current = key in selMap ? selMap[key] : true; // default visible
		const newSelected = !current;

		// Optimistic update
		const newSelections = selections.filter(s =>
			!(s.provider === repo.provider && s.owner === repo.owner && s.repo_name === repo.name)
		);
		newSelections.push({
			provider: repo.provider,
			owner: repo.owner,
			repo_name: repo.name,
			selected: newSelected,
		});
		selections = newSelections;

		try {
			await api.repositories.selections.save({
				selections: [{ provider: repo.provider, owner: repo.owner, repo_name: repo.name, selected: newSelected }],
			});
		} catch (e) {
			// revert on failure
			await loadSelections();
		}
	}

	function openManageModal() {
		// Build a full list of all repos with their current selection state
		manageItems = repos.map(r => {
			const key = r.provider + '/' + r.owner + '/' + r.name;
			const selMap = selectionMap();
			const selected = key in selMap ? selMap[key] : true;
			return {
				provider: r.provider,
				owner: r.owner,
				repo_name: r.name,
				full_name: r.full_name,
				description: r.description,
				selected: selected,
				_origSelected: selected,
			};
		});
		manageSearch = '';
		manageSubmitting = false;
		showManageModal = true;
	}

	function closeManageModal() {
		showManageModal = false;
	}

	async function submitManageSelections() {
		manageSubmitting = true;
		try {
			const changed = manageItems.filter(m => m.selected !== m._origSelected);
			if (changed.length === 0) {
				showManageModal = false;
				return;
			}
			const payload = changed.map(m => ({
				provider: m.provider,
				owner: m.owner,
				repo_name: m.repo_name,
				selected: m.selected,
			}));
			await api.repositories.selections.save({ selections: payload });

			// Reload selections from server
			await loadSelections();
			showManageModal = false;
		} catch (e) {
			// silent
		} finally {
			manageSubmitting = false;
		}
	}

	function toggleManageItem(item) {
		item.selected = !item.selected;
	}

	let manageFiltered = $derived.by(() => {
		if (!manageSearch) return manageItems;
		const q = manageSearch.toLowerCase();
		return manageItems.filter(m =>
			m.full_name?.toLowerCase().includes(q) ||
			m.description?.toLowerCase().includes(q)
		);
	});

	const hiddenRepoCount = $derived.by(() => {
		let count = 0;
		const selMap = selectionMap();
		for (const r of repos) {
			const key = r.provider + '/' + r.owner + '/' + r.name;
			if (key in selMap && !selMap[key]) count++;
		}
		return count;
	});
</script>

<div class="page-container">
	<!-- Header -->
	<div class="flex items-center justify-between mb-5">
		<div class="flex items-center gap-3">
			<h2 class="text-xl font-bold" style="color: var(--color-text);">Repositories</h2>
			<span class="text-xs px-2.5 py-0.5 rounded-full border"
				style="color: var(--color-text-secondary); border-color: var(--color-border);">
				{repos.length} repos
			</span>
			{#if hiddenRepoCount > 0}
				<span class="text-xs px-2.5 py-0.5 rounded-full border"
					style="color: var(--color-text-muted); border-color: var(--color-border);">
					{hiddenRepoCount} hidden
				</span>
			{/if}
		</div>
		<div class="flex items-center gap-2">
			{#if repos.length > 0}
				<button class="btn-secondary flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg"
					onclick={openManageModal}>
					<Icon icon="solar:eye-linear" class="w-4 h-4" />
					Manage Visibility
				</button>
			{/if}
			<button class="btn-primary flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg"
				onclick={() => openConnectModal('github')}>
				<Icon icon="solar:link-circle-bold" class="w-4 h-4" />
				Connect Account
			</button>
		</div>
	</div>

	<!-- Connected Accounts Status -->
	<div class="flex gap-3 mb-5 flex-wrap">
		{#each connections as conn}
			<div class="card flex items-center gap-3 px-4 py-3.5"
				style="flex: 1 1 200px; border-color: {conn.is_active ? 'rgba(16,185,129,0.2)' : 'rgba(239,68,68,0.2)'};">
				<Icon icon={conn.provider === 'github' ? 'mdi:github' : 'simple-icons:forgejo'}
					class="w-5 h-5 flex-shrink-0"
					style="color: var(--color-primary);" />
				<div class="flex-1 min-w-0">
					<div class="text-sm font-semibold truncate" style="color: var(--color-text);">
						{conn.label || conn.provider}
					</div>
					<div class="text-xs truncate" style="color: var(--color-text-muted);">
						{conn.base_url || conn.provider === 'github' ? 'github.com' : 'self-hosted'}
					</div>
					{#if conn.affiliations && conn.affiliations.length > 0 && conn.affiliations.length < 3}
						<div class="flex gap-1 mt-1 flex-wrap">
							{#each conn.affiliations as aff}
								<span class="text-[10px] px-1.5 py-0.5 rounded font-medium"
									style="background: rgba(16,185,129,0.08); color: var(--color-primary);">
									{aff === 'owner' ? 'Personal' : aff === 'collaborator' ? 'Collab' : 'Org'}
								</span>
							{/each}
						</div>
					{/if}
				</div>
				<span class="text-xs whitespace-nowrap" style="color: {conn.is_active ? 'var(--color-primary)' : '#ef4444'};">
					{conn.is_active ? '● Connected' : '○ Disconnected'}
				</span>
				<button class="action-btn w-6 h-6 flex items-center justify-center rounded flex-shrink-0"
					title="Remove connection"
					onclick={() => deleteConnection(conn.id)}>
					{#if deletingId === conn.id}
						<Icon icon="svg-spinners:dots-scale" class="w-3 h-3" style="color: var(--color-text-muted);" />
					{:else}
						<Icon icon="solar:trash-bin-minimistic-linear" class="w-3 h-3" style="color: #ef4444;" />
					{/if}
				</button>
			</div>
		{/each}
		{#if connections.length === 0}
			<div class="card flex items-center gap-3 px-4 py-3.5"
				style="flex: 1 1 200px; opacity: 0.6; cursor: pointer;"
				onclick={() => openConnectModal('github')}
				role="button" tabindex="0"
				onkeydown={(e) => e.key === 'Enter' && openConnectModal('github')}>
				<Icon icon="solar:link-circle-linear" class="w-5 h-5" style="color: var(--color-text-muted);" />
				<span class="text-sm" style="color: var(--color-text-muted);">No providers connected — click to add</span>
			</div>
		{/if}
	</div>

	<!-- Filters -->
	<div class="flex items-center gap-3 mb-5 flex-wrap">
		<button class="filter-chip" class:active={filter === 'all'} onclick={() => filter = 'all'}>All</button>
		<button class="filter-chip flex items-center gap-1.5" class:active={filter === 'github'} onclick={() => filter = 'github'}>
			<Icon icon="mdi:github" class="w-4 h-4" /> GitHub
		</button>
		<button class="filter-chip flex items-center gap-1.5" class:active={filter === 'forgejo'} onclick={() => filter = 'forgejo'}>
			<Icon icon="simple-icons:forgejo" class="w-4 h-4" /> Forgejo
		</button>
		<div class="relative ml-auto">
			<Icon icon="solar:magnifer-linear" class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4"
				style="color: var(--color-text-muted);" />
			<input type="text" placeholder="Search repositories..."
				class="search-input" bind:value={search}>
		</div>
	</div>

	<!-- Loading -->
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<Icon icon="svg-spinners:dots-scale" class="w-8 h-8" style="color: var(--color-primary);" />
		</div>

	<!-- Error -->
	{:else if error}
		<div class="card py-16 text-center">
			<Icon icon="solar:danger-triangle-bold" class="w-12 h-12 mx-auto mb-3" style="color: var(--color-text-muted);" />
			<p class="text-sm" style="color: var(--color-text-secondary);">{error}</p>
		</div>

	<!-- Empty state -->
	{:else if repos.length === 0}
		<div class="card py-16 text-center">
			<Icon icon="solar:code-square-bold" class="w-12 h-12 mx-auto mb-3" style="color: var(--color-text-muted);" />
			<h3 class="text-base font-semibold mb-1" style="color: var(--color-text);">No repositories yet</h3>
			<p class="text-sm mb-4" style="color: var(--color-text-secondary);">
				Connect your GitHub or Forgejo account to get started.
			</p>
			<button class="btn-primary inline-flex items-center gap-1.5 text-sm px-4 py-2 rounded-lg"
				onclick={() => openConnectModal('github')}>
				<Icon icon="solar:link-circle-bold" class="w-4 h-4" />
				Connect Account
			</button>
		</div>

	<!-- Repo Cards Grid -->
	{:else}
		<div class="grid gap-3" style="grid-template-columns: repeat(auto-fill, minmax(420px, 1fr));">
			{#each filtered as repo (repo.full_name)}
				<div class="card repo-card" onclick={() => loadDeployments(repo)}
					role="button" tabindex="0"
					onkeydown={(e) => e.key === 'Enter' && loadDeployments(repo)}>
					<div class="flex items-start gap-3">
						<!-- Avatar / Icon -->
						<div class="w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0"
							style="background: rgba(16,185,129,0.1);">
							<Icon icon={providerIcon(repo.provider)} class="w-5 h-5"
								style="color: var(--color-primary);" />
						</div>

						<div class="flex-1 min-w-0">
							<!-- Title + Provider badge -->
							<div class="flex items-center gap-2 mb-0.5">
								<span class="font-semibold text-sm truncate" style="color: var(--color-text);">
									{repo.full_name}
								</span>
								<span class="provider-badge px-2 py-0.5 text-[10px] font-semibold rounded uppercase tracking-wider"
									class:provider-github={repo.provider === 'github'}
									class:provider-forgejo={repo.provider === 'forgejo'}
									style="background: rgba(255,255,255,0.08); color: var(--color-text-secondary);">
									{repo.provider}
								</span>
							</div>

							<!-- Description -->
							<p class="text-xs mb-2 truncate" style="color: var(--color-text-muted);">
								{repo.description || 'No description'}
							</p>

							<!-- Info row: CI status, branch, time ago, PRs -->
							<div class="flex items-center gap-2.5 text-xs flex-wrap">
								{#if repo.ci_status}
									{@const badge = ciBadge(repo.ci_status.state)}
									{#if badge}
										<span class="ci-badge {badge.cls} inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-semibold">
											{badge.label}
										</span>
									{/if}
								{/if}
								{#if repo.default_branch}
									<span class="stat-chip inline-flex items-center gap-1" style="color: var(--color-text-secondary);">
										<Icon icon="solar:code-branch-linear" class="w-3.5 h-3.5" />
										{repo.default_branch}
									</span>
								{/if}
								{#if repo.updated_at}
									<span class="stat-chip inline-flex items-center gap-1" style="color: var(--color-text-secondary);">
										<Icon icon="solar:clock-circle-linear" class="w-3.5 h-3.5" />
										{timeAgo(repo.updated_at)}
									</span>
								{/if}
								{#if repo.open_prs > 0}
									<span class="stat-chip inline-flex items-center gap-1" style="color: var(--color-text-secondary);">
										<Icon icon="solar:share-linear" class="w-3.5 h-3.5" />
										{repo.open_prs} PR{repo.open_prs > 1 ? 's' : ''}
									</span>
								{/if}
							</div>

							<!-- Deployments linkage pills -->
							{#if repo._deployments && repo._deployments.length > 0}
								<div class="flex items-center gap-2 mt-2 flex-wrap">
									{#each repo._deployments as dep}
										<a href="/deployments" class="pill-link text-xs flex items-center gap-1 px-2 py-0.5 rounded-full"
											style="background: rgba(99,102,241,0.1); color: #818cf8;">
											<Icon icon="solar:rocket-bold" class="w-3 h-3" />
											{dep.name}
										</a>
									{/each}
								</div>
							{/if}

							<!-- Loading deployments indicator -->
							{#if loadingDeployments[repo.full_name]}
								<div class="mt-2">
									<Icon icon="svg-spinners:dots-scale" class="w-4 h-4"
										style="color: var(--color-primary);" />
								</div>
							{/if}
						</div>

						<!-- Actions -->
						<div class="flex gap-1 flex-shrink-0">
							<button class="action-btn w-7 h-7 flex items-center justify-center rounded"
								title={isRepoHidden(repo) ? 'Show this repo' : 'Hide this repo'}
								onclick={(e) => { e.stopPropagation(); toggleRepoVisibility(repo); }}>
								<Icon icon={isRepoHidden(repo) ? 'solar:eye-closed-linear' : 'solar:eye-linear'}
									class="w-3.5 h-3.5"
									style="color: {isRepoHidden(repo) ? '#ef4444' : 'var(--color-text-muted)'};" />
							</button>
							<button class="action-btn w-7 h-7 flex items-center justify-center rounded"
								title="Deploy this repo"
								onclick={(e) => { e.stopPropagation(); window.location.href = '/deployments?repo=' + encodeURIComponent(repo.full_name); }}>
								<Icon icon="solar:arrow-right-linear" class="w-3.5 h-3.5" />
							</button>
							<a href={repo.html_url} target="_blank" rel="noopener"
								class="action-btn w-7 h-7 flex items-center justify-center rounded"
								title="Open in {repo.provider}"
								onclick={(e) => e.stopPropagation()}>
								<Icon icon="solar:external-link-linear" class="w-3.5 h-3.5" />
							</a>
						</div>
					</div>

					<!-- Expandable Detail Panel -->
					{#if expandedRepo === repo.full_name && repo._deployments}
						<div class="mt-3 pt-3 border-t" style="border-color: var(--color-border);"
							onclick={(e) => e.stopPropagation()}>
							<div class="text-xs font-semibold mb-2 uppercase tracking-wider"
								style="color: var(--color-text-muted);">Linked Deployments</div>
							{#if repo._deployments.length === 0}
								<div class="text-xs" style="color: var(--color-text-muted);">No deployments linked to this repository</div>
							{:else}
								<div class="flex flex-col gap-2">
									{#each repo._deployments as dep}
										<div class="flex items-center justify-between px-3 py-2 rounded-lg"
											style="background: rgba(255,255,255,0.03); border: 1px solid var(--color-border);">
											<div class="flex items-center gap-2 text-xs">
												<Icon icon="solar:rocket-bold" class="w-3.5 h-3.5"
													style="color: var(--color-primary);" />
												<span style="color: var(--color-text);">{dep.name}</span>
												<span class="px-1.5 py-0.5 rounded text-[10px] font-semibold"
													class:status-running={dep.status === 'running' || dep.status === 'success'}
													class:status-failed={dep.status === 'failed'}
													class:status-pending={dep.status === 'pending' || dep.status === 'deploying'}
													style="background: rgba(16,185,129,0.1); color: var(--color-primary);">
													{statusBadge(dep.status)}
												</span>
											</div>
											<a href="/deployments" class="text-xs hover:underline"
												style="color: var(--color-primary);">View →</a>
										</div>
									{/each}
								</div>
							{/if}
							<div class="flex gap-2 mt-3">
								<a href="/deployments?new=true&provider={repo.provider}&owner={repo.owner}&repo={repo.name}"
									class="action-link text-xs px-3 py-1.5 rounded flex items-center gap-1.5"
									style="border: 1px solid var(--color-border); color: var(--color-text-secondary);">
									<Icon icon="solar:rocket-bold" class="w-3 h-3" /> Deploy Branch
								</a>
								<a href={repo.html_url} target="_blank" rel="noopener"
									class="action-link text-xs px-3 py-1.5 rounded flex items-center gap-1.5"
									style="border: 1px solid var(--color-border); color: var(--color-text-secondary);">
									<Icon icon="solar:external-link-linear" class="w-3 h-3" /> Open on {repo.provider}
								</a>
							</div>
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- ─── Manage Visibility Modal ──────────────────────────── -->
{#if showManageModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background-color: rgba(0,0,0,0.5);"
		onclick={closeManageModal}
		role="button" tabindex="0"
		onkeydown={(e) => e.key === 'Escape' && closeManageModal()}>
		<div class="w-full max-w-2xl rounded-xl border shadow-xl animate-modal-in max-h-[85vh] flex flex-col"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
			role="dialog" aria-modal="true" aria-label="Manage Repo Visibility">
			<!-- Header -->
			<div class="flex items-center justify-between border-b px-5 py-4 flex-shrink-0" style="border-color: var(--color-border);">
				<h3 class="text-base font-semibold flex items-center gap-2" style="color: var(--color-text);">
					<Icon icon="solar:eye-linear" class="w-5 h-5" style="color: var(--color-primary);" />
					Manage Repo Visibility
				</h3>
				<button class="action-btn w-7 h-7 flex items-center justify-center rounded" onclick={closeManageModal}>
					<Icon icon="solar:close-circle-linear" class="w-4 h-4" style="color: var(--color-text-muted);" />
				</button>
			</div>

			<!-- Search -->
			<div class="px-5 py-3 border-b flex-shrink-0" style="border-color: var(--color-border);">
				<div class="relative">
					<Icon icon="solar:magnifer-linear" class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4"
						style="color: var(--color-text-muted);" />
					<input type="text" placeholder="Search repos to toggle..."
						class="search-input w-full" bind:value={manageSearch}>
				</div>
			</div>

			<!-- Repo list -->
			<div class="flex-1 overflow-y-auto px-5 py-3 space-y-1">
				{#each manageFiltered as item (item.provider + '/' + item.owner + '/' + item.repo_name)}
					<div class="flex items-center gap-3 px-3 py-2 rounded-lg cursor-pointer transition"
						style="background: {item.selected ? 'rgba(16,185,129,0.04)' : 'transparent'}; border: 1px solid {item.selected ? 'rgba(16,185,129,0.15)' : 'transparent'};"
						onclick={() => toggleManageItem(item)}
						role="button" tabindex="0"
						onkeydown={(e) => e.key === 'Enter' && toggleManageItem(item)}>
						<div class="w-5 h-5 rounded flex items-center justify-center flex-shrink-0"
							style="background: {item.selected ? 'var(--color-primary)' : 'var(--color-border)'};">
							{#if item.selected}
								<Icon icon="solar:check-bold" class="w-3 h-3" style="color: white;" />
							{/if}
						</div>
						<div class="flex-1 min-w-0">
							<div class="text-sm font-medium truncate" style="color: var(--color-text);">
								<span class="text-xs px-1.5 py-0.5 rounded mr-1.5 font-mono"
									style="background: rgba(255,255,255,0.06); color: var(--color-text-muted);">
									{item.provider === 'github' ? 'GH' : 'FJ'}
								</span>
								{item.full_name}
							</div>
							{#if item.description}
								<div class="text-xs truncate mt-0.5" style="color: var(--color-text-muted);">{item.description}</div>
							{/if}
						</div>
						<span class="text-xs whitespace-nowrap"
							style="color: {item.selected ? 'var(--color-primary)' : 'var(--color-text-muted)'};">
							{item.selected ? '● Visible' : '○ Hidden'}
						</span>
					</div>
				{/each}
				{#if manageFiltered.length === 0}
					<div class="text-sm py-8 text-center" style="color: var(--color-text-muted);">
						No repos match your search.
					</div>
				{/if}
			</div>

			<!-- Summary + footer -->
			<div class="flex items-center justify-between border-t px-5 py-3 flex-shrink-0" style="border-color: var(--color-border);">
				<div class="text-xs" style="color: var(--color-text-muted);">
					{manageItems.filter(m => m.selected).length} visible · {manageItems.filter(m => !m.selected).length} hidden
				</div>
				<div class="flex items-center gap-2">
					<button class="btn-secondary text-xs px-4 py-1.5 rounded-lg" onclick={closeManageModal}>
						Cancel
					</button>
					<button class="btn-primary text-xs px-4 py-1.5 rounded-lg flex items-center gap-1.5"
						disabled={manageSubmitting}
						onclick={submitManageSelections}>
						{#if manageSubmitting}
							<Icon icon="svg-spinners:dots-scale" class="w-4 h-4" />
							Saving...
						{:else}
							<Icon icon="solar:check-circle-bold" class="w-4 h-4" />
							Save Visibility
						{/if}
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}

<!-- ─── Connect Account Modal ─────────────────────────────── -->
{#if showConnectModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background-color: rgba(0,0,0,0.5);"
		onclick={closeConnectModal}
		role="button" tabindex="0"
		onkeydown={(e) => e.key === 'Escape' && closeConnectModal()}>
		<div class="w-full max-w-md rounded-xl border shadow-xl animate-modal-in"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
			role="dialog" aria-modal="true" aria-label="Connect Account">
			<!-- Header -->
			<div class="flex items-center justify-between border-b px-5 py-4" style="border-color: var(--color-border);">
				<h3 class="text-base font-semibold flex items-center gap-2" style="color: var(--color-text);">
					<Icon icon="solar:link-circle-bold" class="w-5 h-5" style="color: var(--color-primary);" />
					Connect Account
				</h3>
				<button class="action-btn w-7 h-7 flex items-center justify-center rounded" onclick={closeConnectModal}>
					<Icon icon="solar:close-circle-linear" class="w-4 h-4" style="color: var(--color-text-muted);" />
				</button>
			</div>

			<!-- Body -->
			<div class="px-5 py-4 space-y-4">
				<!-- Provider selector -->
				<div>
					<label class="mb-1.5 block text-xs font-medium uppercase tracking-wider"
						style="color: var(--color-text-secondary);">Provider</label>
					<div class="flex gap-2">
						<button class="provider-option flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium flex-1 border"
							class:provider-active={connectProvider === 'github'}
							style="border-color: {connectProvider === 'github' ? 'var(--color-primary)' : 'var(--color-border)'}; background: {connectProvider === 'github' ? 'rgba(16,185,129,0.08)' : 'transparent'}; color: {connectProvider === 'github' ? 'var(--color-primary)' : 'var(--color-text-secondary)'};"
							onclick={() => connectProvider = 'github'}>
							<Icon icon="mdi:github" class="w-5 h-5" />
							GitHub
						</button>
						<button class="provider-option flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium flex-1 border"
							class:provider-active={connectProvider === 'forgejo'}
							style="border-color: {connectProvider === 'forgejo' ? 'var(--color-primary)' : 'var(--color-border)'}; background: {connectProvider === 'forgejo' ? 'rgba(16,185,129,0.08)' : 'transparent'}; color: {connectProvider === 'forgejo' ? 'var(--color-primary)' : 'var(--color-text-secondary)'};"
							onclick={() => connectProvider = 'forgejo'}>
							<Icon icon="simple-icons:forgejo" class="w-5 h-5" />
							Forgejo
						</button>
					</div>
				</div>

				<!-- Token -->
				<div>
					<label for="connect-token" class="mb-1.5 block text-xs font-medium uppercase tracking-wider"
						style="color: var(--color-text-secondary);">
						Personal Access Token
						<span style="color: var(--color-danger);">*</span>
					</label>
					<input id="connect-token" type="password" bind:value={connectToken}
						class="input w-full" placeholder="ghp_... or glpat-..." />
					<p class="text-xs mt-1" style="color: var(--color-text-muted);">
						{connectProvider === 'github' ? 'Requires repo, read:org, and workflow scopes.' : 'Requires read:repository and read:user scopes.'}
					</p>
				</div>

				<!-- Label (optional) -->
				<div>
					<label for="connect-label" class="mb-1.5 block text-xs font-medium uppercase tracking-wider"
						style="color: var(--color-text-secondary);">
						Label <span style="color: var(--color-text-muted);">(optional)</span>
					</label>
					<input id="connect-label" type="text" bind:value={connectLabel}
						class="input w-full" placeholder="e.g. Personal Account" />
				</div>

				<!-- Base URL (Forgejo only) -->
				{#if connectProvider === 'forgejo'}
					<div>
						<label for="connect-url" class="mb-1.5 block text-xs font-medium uppercase tracking-wider"
							style="color: var(--color-text-secondary);">
							Instance URL
							<span style="color: var(--color-danger);">*</span>
						</label>
						<input id="connect-url" type="url" bind:value={connectBaseURL}
							class="input w-full" placeholder="https://git.yourdomain.com" />
					</div>
				{/if}

				<!-- Repository Affiliation Filter -->
				<div>
					<label class="mb-1.5 block text-xs font-medium uppercase tracking-wider"
						style="color: var(--color-text-secondary);">
						Show repositories
					</label>
					<div class="space-y-2">
						{#each affiliationOptions as opt}
							<label class="flex items-start gap-2.5 px-3 py-2 rounded-lg cursor-pointer transition"
								style="background: {connectAffiliations.includes(opt.value) ? 'rgba(16,185,129,0.06)' : 'transparent'}; border: 1px solid {connectAffiliations.includes(opt.value) ? 'rgba(16,185,129,0.2)' : 'var(--color-border)'};"
								onclick={() => {
									if (connectAffiliations.includes(opt.value)) {
										// Don't allow deselecting all — at least one must remain
										if (connectAffiliations.length > 1) {
											connectAffiliations = connectAffiliations.filter(a => a !== opt.value);
										}
									} else {
										connectAffiliations = [...connectAffiliations, opt.value];
									}
								}}>
								<div class="w-4 h-4 rounded mt-0.5 flex items-center justify-center flex-shrink-0"
									style="background: {connectAffiliations.includes(opt.value) ? 'var(--color-primary)' : 'var(--color-border)'};">
									{#if connectAffiliations.includes(opt.value)}
										<Icon icon="solar:check-bold" class="w-2.5 h-2.5" style="color: white;" />
									{/if}
								</div>
								<div>
									<div class="text-sm font-medium" style="color: var(--color-text);">{opt.label}</div>
									<div class="text-xs mt-0.5" style="color: var(--color-text-muted);">{opt.desc}</div>
								</div>
							</label>
						{/each}
					</div>
				</div>

				<!-- Error / Success -->
				{#if connectError}
					<div class="text-sm px-3 py-2 rounded-lg" style="background: rgba(239,68,68,0.1); color: #ef4444;">
						<Icon icon="solar:danger-triangle-linear" class="w-4 h-4 inline mr-1" />
						{connectError}
					</div>
				{/if}
				{#if connectSuccess}
					<div class="text-sm px-3 py-2 rounded-lg" style="background: rgba(16,185,129,0.1); color: var(--color-primary);">
						<Icon icon="solar:check-circle-linear" class="w-4 h-4 inline mr-1" />
						{connectSuccess}
					</div>
				{/if}
			</div>

			<!-- Footer -->
			<div class="flex items-center justify-end gap-2 border-t px-5 py-3" style="border-color: var(--color-border);">
				<button class="btn-secondary text-sm px-4 py-1.5 rounded-lg" onclick={closeConnectModal}>
					Cancel
				</button>
				<button class="btn-primary text-sm px-4 py-1.5 rounded-lg flex items-center gap-1.5"
					disabled={connectSubmitting}
					onclick={submitConnect}>
					{#if connectSubmitting}
						<Icon icon="svg-spinners:dots-scale" class="w-4 h-4" />
						Connecting...
					{:else}
						<Icon icon="solar:link-circle-bold" class="w-4 h-4" />
						Connect
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Modal animation -->
<svelte:head>
	<style>
		@keyframes modal-in {
			from { opacity: 0; transform: scale(0.95) translateY(8px); }
			to { opacity: 1; transform: scale(1) translateY(0); }
		}
		.animate-modal-in { animation: modal-in 0.15s ease-out; }
	</style>
</svelte:head>

<style>
	:global(.repo-card) {
		cursor: pointer;
		transition: border-color 0.15s, box-shadow 0.15s;
		padding: 18px;
	}
	:global(.repo-card:hover) {
		border-color: var(--color-primary) !important;
		box-shadow: 0 2px 12px rgba(16,185,129,0.08);
	}

	:global(.filter-chip) {
		padding: 5px 14px;
		border-radius: 8px;
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		border: 1px solid var(--color-border);
		background: transparent;
		color: var(--color-text-secondary);
		transition: all 0.15s;
	}
	:global(.filter-chip:hover) {
		border-color: var(--color-primary);
		color: var(--color-text);
	}
	:global(.filter-chip.active) {
		background: rgba(16,185,129,0.1);
		border-color: var(--color-primary);
		color: var(--color-primary);
	}

	:global(.search-input) {
		padding: 7px 12px 7px 36px;
		border-radius: 8px;
		font-size: 13px;
		border: 1px solid var(--color-border);
		background: var(--color-card-bg);
		color: var(--color-text);
		outline: none;
		width: 260px;
		transition: border-color 0.15s;
	}
	:global(.search-input:focus) {
		border-color: var(--color-primary);
		box-shadow: 0 0 0 3px rgba(16,185,129,0.1);
	}

	:global(.action-btn) {
		border: 1px solid var(--color-border);
		background: transparent;
		color: var(--color-text-muted);
		cursor: pointer;
		transition: all 0.15s;
	}
	:global(.action-btn:hover) {
		border-color: var(--color-primary);
		color: var(--color-primary);
		background: rgba(16,185,129,0.06);
	}

	:global(.action-link) {
		text-decoration: none;
		transition: all 0.15s;
	}
	:global(.action-link:hover) {
		border-color: var(--color-primary) !important;
		color: var(--color-primary) !important;
	}

	:global(.pill-link) {
		text-decoration: none;
		transition: opacity 0.15s;
	}
	:global(.pill-link:hover) {
		opacity: 0.8;
	}

	:global(.provider-badge) {
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}

	:global(.ci-badge.pass) {
		background: rgba(16,185,129,0.12);
		color: var(--color-primary);
	}
	:global(.ci-badge.fail) {
		background: rgba(239,68,68,0.12);
		color: #ef4444;
	}
	:global(.ci-badge.pending) {
		background: rgba(234,179,8,0.12);
		color: #eab308;
	}

	:global(.stat-chip) {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}
</style>
