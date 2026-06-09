<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import { user } from '$lib/stores/auth.js';
	import Icon from '@iconify/svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	let name = $derived($page.params.name);

	// ─── State ───
	let tags = $state([]);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');
	let nextLast = $state('');
	let repoInfo = $state(null);

	let searchQuery = $state('');
	let tagProtections = $state([]);
	let tagProtectionsSet = $derived(new Set(tagProtections.map(p => `${p.repo}:${p.tag}`)));

	let isAdmin = $derived($user?.role === 'admin');

	// Copy state
	let copiedTarget = $state('');

	// Delete modal
	let deleteTagTarget = $state(null);
	let deleteRepoTarget = $state(false);
	let deleting = $state(false);

	// Sort state
	let sortField = $state('name');
	let sortDir = $state('asc');

	let sortedTags = $derived.by(() => {
		const sorted = [...tags];
		const dir = sortDir === 'asc' ? 1 : -1;
		sorted.sort((a, b) => {
			switch (sortField) {
				case 'name': {
					const va = (a.name || '').toLowerCase();
					const vb = (b.name || '').toLowerCase();
					return va < vb ? -dir : va > vb ? dir : 0;
				}
				case 'size': {
					return ((a.layer_size || a.size || 0) - (b.layer_size || b.size || 0)) * dir;
				}
				case 'created': {
					const da = new Date(a.created || 0).getTime();
					const db = new Date(b.created || 0).getTime();
					return (da - db) * dir;
				}
				default:
					return 0;
			}
		});
		return sorted;
	});

	function toggleSort(field) {
		if (sortField === field) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortField = field;
			sortDir = 'asc';
		}
	}

	function sortIcon(field) {
		if (sortField !== field) return '';
		return sortDir === 'asc'
			? 'solar:alt-arrow-up-outline'
			: 'solar:alt-arrow-down-outline';
	}


	// Bulk selection
	let selectedTags = $state(new Set());
	let bulkLoading = $state(false);

	function toggleSelectAll() {
		if (selectedTags.size === sortedTags.length) {
			selectedTags = new Set();
		} else {
			selectedTags = new Set(sortedTags.map(t => t.name));
		}
	}

	async function bulkProtectSelected() {
		if (selectedTags.size === 0) return;
		bulkLoading = true;
		for (const tag of selectedTags) {
			try { await api.registry.protections.create({ repo: name, tag }); } catch {}
		}
		await loadProtections();
		selectedTags = new Set();
		bulkLoading = false;
	}

	async function bulkUnprotectSelected() {
		if (selectedTags.size === 0) return;
		bulkLoading = true;
		for (const tag of selectedTags) {
			try { await api.registry.protections.deleteByRepoTag(name, tag); } catch {}
		}
		await loadProtections();
		selectedTags = new Set();
		bulkLoading = false;
	}

	async function bulkDeleteSelected() {
		if (selectedTags.size === 0) return;
		if (!confirm(`Delete ${selectedTags.size} tags? This cannot be undone.`)) return;
		bulkLoading = true;
		for (const tag of selectedTags) {
			try { await api.registry.deleteTag(name, tag); } catch {}
		}
		await loadTags();
		selectedTags = new Set();
		bulkLoading = false;
	}
	onMount(() => {
		loadTags();
		loadProtections();
	});

	async function loadTags(q) {
		loading = true;
		error = '';
		try {
			const params = { n: 50 };
			if (q) params.q = q;
			const data = await api.registry.listTags(name, params);
			tags = data?.tags || [];
			nextLast = data?.next_last || '';
			repoInfo = { tags_count: data?.tags?.length || tags.length };
		} catch (e) {
			error = e.message || 'Failed to load tags';
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		if (!nextLast || loadingMore) return;
		loadingMore = true;
		try {
			const params = { n: 50, last: nextLast };
			if (searchQuery) params.q = searchQuery;
			const data = await api.registry.listTags(name, params);
			if (data?.tags) {
				tags = [...tags, ...data.tags];
			}
			nextLast = data?.next_last || '';
		} catch (e) {
			// ignore
		} finally {
			loadingMore = false;
		}
	}

	async function loadProtections() {
		try {
			const data = await api.registry.protections.list(name);
			tagProtections = Array.isArray(data) ? data : [];
		} catch (e) {
			// ignore
		}
	}

	async function protectTag(repo, tag) {
		try {
			await api.registry.protections.create({ repo, tag });
			await loadProtections();
		} catch (e) {
			error = e.message || 'Failed to protect tag';
		}
	}

	async function unprotectTag(repo, tag) {
		try {
			await api.registry.protections.deleteByRepoTag(repo, tag);
			await loadProtections();
		} catch (e) {
			error = e.message || 'Failed to unprotect tag';
		}
	}

	function isTagProtected(repo, tag) {
		return tagProtectionsSet.has(`${repo}:${tag}`);
	}

	// ─── Delete Handlers ───
	function promptDeleteTag(repo, tag) {
		if (isTagProtected(repo, tag)) {
			error = `⚠️ Tag "${repo}:${tag}" is protected. Unprotect it first before deleting.`;
			return;
		}
		deleteTagTarget = { repo, tag };
	}

	async function confirmDeleteTag() {
		if (!deleteTagTarget) return;
		const { repo, tag } = deleteTagTarget;
		// Double-check protection before delete
		if (isTagProtected(repo, tag)) {
			error = `⚠️ Tag "${repo}:${tag}" is protected. Unprotect it first before deleting.`;
			deleteTagTarget = null;
			return;
		}
		deleting = true;
		try {
			await api.registry.deleteTag(repo, tag);
			tags = tags.filter(t => t.name !== tag);
			deleteTagTarget = null;
		} catch (e) {
			error = e.message || 'Failed to delete tag';
		} finally {
			deleting = false;
		}
	}

	async function confirmDeleteRepo() {
		deleting = true;
		try {
			await api.registry.deleteRepo(name);
			goto('/registry');
		} catch (e) {
			error = e.message || 'Failed to delete repo';
		} finally {
			deleting = false;
			deleteRepoTarget = false;
		}
	}

	// ─── Search ───
	let searchTimer;
	function onSearchInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			if (searchQuery) {
				loadTags(searchQuery);
			} else {
				loadTags();
			}
		}, 300);
	}

	// ─── Helpers ───
	function formatSize(bytes) {
		if (!bytes || bytes === 0) return '—';
		if (bytes < 1024) return bytes + ' B';
		if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(0) + ' KB';
		if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(0) + ' MB';
		return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
	}

	function formatDate(iso) {
		if (!iso) return '—';
		const d = new Date(iso);
		const now = new Date();
		const diff = now - d;
		if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
		if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
		if (diff < 604800000) return Math.floor(diff / 86400000) + 'd ago';
		if (diff < 2592000000) return Math.floor(diff / 604800000) + 'w ago';
		return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
	}

	function shortDigest(d) {
		if (!d) return '';
		return d.length > 19 ? d.slice(0, 19) + '...' : d;
	}

	let copiedTargetLocal = $state('');

	async function copyToClipboard(text, target) {
		copiedTargetLocal = target;
		try {
			if (navigator.clipboard?.writeText) {
				await navigator.clipboard.writeText(text);
			} else {
				const ta = document.createElement('textarea');
				ta.value = text;
				ta.style.position = 'fixed';
				ta.style.opacity = '0';
				document.body.appendChild(ta);
				ta.select();
				document.execCommand('copy');
				document.body.removeChild(ta);
			}
		} catch {
			// ignore
		}
		setTimeout(() => {
			if (copiedTargetLocal === target) copiedTargetLocal = '';
		}, 2000);
	}
</script>

<div class="page-container">
	<!-- Breadcrumb -->
	<div class="mb-4 flex items-center gap-2 text-xs" style="color: var(--color-text-secondary);">
		<a href="/registry" class="flex items-center gap-1 transition-colors" style="color: var(--color-text-secondary);">
			<Icon icon="solar:alt-arrow-left-outline" class="h-3.5 w-3.5" />
			Registry
		</a>
		<Icon icon="solar:alt-arrow-right-outline" class="h-3 w-3" style="color: var(--color-text-muted);" />
		<span class="font-medium" style="color: var(--color-text);">{name}</span>
	</div>

	<!-- Repo Header -->
	<div class="card p-5">
		<div class="flex items-start justify-between">
			<div class="flex items-start gap-3">
				<div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg" style="background-color: var(--color-primary-subtle);">
					<Icon icon="solar:archive-down-minimlistic-bold" class="h-5 w-5" style="color: var(--color-primary);" />
				</div>
				<div>
					<div class="flex items-center gap-2">
						<h2 class="text-base font-semibold" style="color: var(--color-text);">{name}</h2>
						<span class="rounded px-1.5 py-0.5 text-[10px] font-medium" style="background-color: var(--color-primary-subtle); color: var(--color-primary);">
							{repoInfo?.tags_count || 0} tags
						</span>
					</div>
					<code class="mt-1 block font-mono text-[11px]" style="color: var(--color-text-muted);">registry.anjungan.io/{name}</code>
				</div>
			</div>
			<div class="flex items-center gap-2">
				<button
					class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
					style="background-color: var(--color-primary-subtle); color: var(--color-primary);"
					onclick={() => copyToClipboard(`docker pull registry.anjungan.io/${name}:latest`, 'pull-repo')}
				>
					<Icon icon="solar:copy-bold" class="h-3.5 w-3.5" />
					{copiedTargetLocal === 'pull-repo' ? 'Copied!' : 'Copy Pull Cmd'}
				</button>
				{#if isAdmin}
					<button
						class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
						style="color: var(--color-danger); border: 1px solid rgba(239,68,68,0.3);"
						onclick={() => deleteRepoTarget = true}
					>
						<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
						Delete Repo
					</button>
				{/if}
			</div>
		</div>
	</div>

	<!-- Error -->
	{#if error}
		<div class="mt-3 rounded-lg border p-3 text-xs" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2); color: var(--color-danger);">
			<div class="flex items-center gap-2">
				<Icon icon="solar:danger-triangle-bold" class="h-4 w-4" />
				<span>{error}</span>
			</div>
		</div>
	{/if}

	<!-- Search -->
	<div class="mt-4 flex items-center gap-3">
		<div class="relative flex-1">
			<Icon icon="solar:magnifer-outline" class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2" style="color: var(--color-text-muted);" />
			<input
				type="text"
				class="w-full rounded-lg border py-2 pl-9 pr-3 text-xs outline-none transition-colors"
				style="background-color: var(--color-input); border-color: var(--color-border); color: var(--color-text);"
				placeholder="Search tags..."
				bind:value={searchQuery}
				oninput={onSearchInput}
			/>
		</div>
		{#if tags.length > 0 || !loading}
			<span class="flex-shrink-0 text-[10px]" style="color: var(--color-text-muted);">
				{tags.length} tag{tags.length !== 1 ? 's' : ''}
			</span>
		{/if}
	</div>

	<!-- Tags Table -->
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<Icon icon="solar:spinner-bold" class="h-6 w-6 animate-spin" style="color: var(--color-primary);" />
		</div>
	{:else if tags.length > 0}
		<div class="card mt-3 overflow-hidden">
			<!-- Column headers -->
			<div class="flex items-center gap-3 px-4 py-2.5 text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted); border-bottom: 1px solid var(--color-border);">
				<span class="w-8 flex-shrink-0">
					<input type="checkbox" class="h-3.5 w-3.5" checked={selectedTags.size === sortedTags.length && sortedTags.length > 0} onchange={toggleSelectAll} />
				</span>
				<button class="flex min-w-0 flex-1 items-center gap-1 text-left" onclick={() => toggleSort('name')}>
					TAG
					{#if sortField === 'name'}
						<Icon icon={sortIcon('name')} class="h-3 w-3" />
					{/if}
				</button>
				<button class="flex w-20 flex-shrink-0 items-center justify-end gap-1" onclick={() => toggleSort('size')}>
					SIZE
					{#if sortField === 'size'}
						<Icon icon={sortIcon('size')} class="h-3 w-3" />
					{/if}
				</button>
				<button class="flex w-24 flex-shrink-0 items-center justify-end gap-1" onclick={() => toggleSort('created')}>
					CREATED
					{#if sortField === 'created'}
						<Icon icon={sortIcon('created')} class="h-3 w-3" />
					{/if}
				</button>
				<span class="w-28 flex-shrink-0 text-right">DIGEST</span>
				<span class="w-32 flex-shrink-0 text-right">ACTIONS</span>
			</div>

			{#each sortedTags as tag}
				<div class="flex items-center gap-3 px-4 py-2.5 transition-colors" style="border-bottom: 1px solid var(--color-border);">
					<span class="w-8 flex-shrink-0">
						<input type="checkbox" class="h-3.5 w-3.5" checked={selectedTags.has(tag.name)} onchange={() => {
							const s = new Set(selectedTags);
							if (s.has(tag.name)) s.delete(tag.name); else s.add(tag.name);
							selectedTags = s;
						}} />
					</span>
					<div class="min-w-0 flex-1">
						<button
							class="font-mono text-xs hover:underline"
							style="color: var(--color-primary);"
							onclick={() => goto(`/registry/${name}/${tag.name}`)}
						>
							{#if isTagProtected(name, tag.name)}
								<Icon icon="solar:lock-bold" class="mr-1 inline h-3 w-3" style="color: var(--color-warning);" />
							{/if}
							{tag.name}
						</button>
						{#if tag.name === 'latest'}
							<span class="ml-1.5 rounded px-1.5 py-0.5 text-[9px] font-medium" style="background-color: var(--color-primary-subtle); color: var(--color-primary);">latest</span>
						{/if}
						{#if isTagProtected(name, tag.name)}
							<span class="ml-1.5 rounded px-1.5 py-0.5 text-[9px] font-medium" style="background-color: rgba(245,158,11,0.15); color: var(--color-warning);">Protected</span>
						{/if}
					</div>
					<span class="w-20 flex-shrink-0 text-right font-mono text-xs" style="color: var(--color-text-muted);">{formatSize(tag.layer_size || tag.size)}</span>
					<span class="w-24 flex-shrink-0 text-right text-xs" style="color: var(--color-text-muted);">{formatDate(tag.created)}</span>
					<span class="w-28 flex-shrink-0 truncate text-right font-mono text-[10px]" style="color: var(--color-text-muted);">{shortDigest(tag.digest)}</span>
					<div class="flex w-32 flex-shrink-0 items-center justify-end gap-1">
						{#if isAdmin && !isTagProtected(name, tag.name)}
							<button
								class="rounded-md p-1.5 transition-colors"
								style="color: var(--color-text-muted);"
								onclick={() => protectTag(name, tag.name)}
								title="Protect tag"
							>
								<Icon icon="solar:shield-up-bold" class="h-3.5 w-3.5" />
							</button>
						{/if}
						{#if isAdmin && isTagProtected(name, tag.name)}
							<button
								class="rounded-md p-1.5 transition-colors"
								style="color: var(--color-warning);"
								onclick={() => unprotectTag(name, tag.name)}
								title="Unprotect tag"
							>
								<Icon icon="solar:shield-minus-bold" class="h-3.5 w-3.5" />
							</button>
						{/if}
						<button
							class="rounded-md p-1.5 transition-colors"
							style="color: var(--color-text-muted);"
							onclick={() => copyToClipboard(`docker pull registry.anjungan.io/${name}:${tag.name}`, `pull-${tag.name}`)}
							title="Copy pull command"
						>
							<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" />
						</button>
						{#if copiedTargetLocal === `pull-${tag.name}`}
							<span class="text-[10px]" style="color: var(--color-success);">✓</span>
						{/if}
						{#if isAdmin}
							<button
								class="rounded-md p-1.5 transition-colors hover:opacity-80"
								style="color: {isTagProtected(name, tag.name) ? 'var(--color-warning)' : 'var(--color-text-muted)'};"
								onclick={() => promptDeleteTag(name, tag.name)}
								title={isTagProtected(name, tag.name) ? 'Protected — unprotect first' : 'Delete tag'}
							>
								<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
							</button>
						{/if}
					</div>
				</div>
			{/each}
		</div>

		<!-- Pagination -->
		{#if nextLast}
			<div class="mt-3 flex items-center justify-between">
				<button
					class="inline-flex items-center gap-1.5 rounded-lg px-4 py-2 text-xs font-medium transition-colors"
					style="background-color: var(--color-primary-subtle); color: var(--color-primary);"
					onclick={loadMore}
					disabled={loadingMore}
				>
					{#if loadingMore}
						<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
						Loading...
					{:else}
						Load More Tags
					{/if}
				</button>
				<span class="text-[10px]" style="color: var(--color-text-muted);">Showing {tags.length}+ tags</span>
			</div>
		{/if}
	{:else}
		<div class="card mt-3 py-10 text-center">
			<Icon icon="solar:box-outline" class="mx-auto h-8 w-8" style="color: var(--color-text-muted);" />
			<p class="mt-2 text-xs font-medium" style="color: var(--color-text-secondary);">No tags found</p>
			<p class="mt-0.5 text-[10px]" style="color: var(--color-text-muted);">
				{searchQuery ? 'No tags match your search query.' : 'This repository has no tags.'}
			</p>
		</div>
	{/if}

	<!-- Bulk Action Bar -->
	{#if selectedTags.size > 0}
		<div class="sticky bottom-4 z-40 mt-3">
			<div class="mx-auto flex max-w-lg items-center justify-between gap-3 rounded-xl border px-4 py-3 shadow-lg" style="background-color: var(--color-card); border-color: var(--color-primary);">
				<div class="flex items-center gap-2">
					<Icon icon="solar:check-square-bold" class="h-4 w-4" style="color: var(--color-primary);" />
					<span class="text-xs font-semibold" style="color: var(--color-text);">{selectedTags.size} tag{selectedTags.size !== 1 ? 's' : ''} selected</span>
				</div>
				<div class="flex items-center gap-2">
					<button
						class="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-[10px] font-medium transition-colors"
						style="color: var(--color-text-secondary); border: 1px solid var(--color-border);"
						onclick={() => selectedTags = new Set()}
						disabled={bulkLoading}
					>Clear</button>
					<button
						class="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-[10px] font-medium transition-colors"
						style="background-color: rgba(245,158,11,0.12); color: #f59e0b;"
						onclick={bulkProtectSelected}
						disabled={bulkLoading}
					>
						<Icon icon="solar:shield-up-bold" class="h-3 w-3" />
						Protect All
					</button>
					<button
						class="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-[10px] font-medium transition-colors"
						style="background-color: rgba(234,179,8,0.12); color: #eab308;"
						onclick={bulkUnprotectSelected}
						disabled={bulkLoading}
					>
						<Icon icon="solar:shield-down-bold" class="h-3 w-3" />
						Unprotect All
					</button>
					<button
						class="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-[10px] font-medium transition-colors"
						style="background-color: rgba(239,68,68,0.12); color: #ef4444;"
						onclick={bulkDeleteSelected}
						disabled={bulkLoading}
					>
						<Icon icon="solar:trash-bin-trash-bold" class="h-3 w-3" />
						Delete All
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Delete Tag Modal -->
	{#if deleteTagTarget}
		<div class="fixed inset-0 z-50 flex items-center justify-center" style="background-color: rgba(0,0,0,0.6);" onclick={() => deleteTagTarget = null}>
			<div class="mx-4 w-full max-w-md rounded-xl border shadow-2xl" style="background-color: var(--color-card); border-color: var(--color-border);" onclick={(e) => e.stopPropagation()}>
				<div class="p-5">
					<div class="flex items-start gap-3">
						<div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full" style="background-color: rgba(239,68,68,0.15);">
							<Icon icon="solar:danger-triangle-bold" class="h-4.5 w-4.5" style="color: var(--color-danger);" />
						</div>
						<div class="min-w-0 flex-1">
							<h3 class="text-sm font-semibold" style="color: var(--color-text);">Delete Tag</h3>
							<p class="mt-1 text-xs" style="color: var(--color-text-secondary);">Are you sure you want to delete <strong>{deleteTagTarget.repo}:{deleteTagTarget.tag}</strong>? This action is irreversible.</p>
						</div>
					</div>
				</div>
				<div class="flex items-center justify-end gap-2 rounded-b-xl border-t px-5 py-3" style="border-color: var(--color-border); background-color: var(--color-topbar-bg);">
					<button class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors" style="color: var(--color-text-secondary);" onclick={() => deleteTagTarget = null}>Cancel</button>
					<button class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium text-white transition-colors" style="background-color: var(--color-danger);" onclick={confirmDeleteTag} disabled={deleting}>
						{#if deleting}
							<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
							Deleting...
						{:else}
							<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
							Delete
						{/if}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Delete Repo Modal -->
	{#if deleteRepoTarget}
		<div class="fixed inset-0 z-50 flex items-center justify-center" style="background-color: rgba(0,0,0,0.6);" onclick={() => deleteRepoTarget = false}>
			<div class="mx-4 w-full max-w-md rounded-xl border shadow-2xl" style="background-color: var(--color-card); border-color: var(--color-border);" onclick={(e) => e.stopPropagation()}>
				<div class="p-5">
					<div class="flex items-start gap-3">
						<div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full" style="background-color: rgba(239,68,68,0.15);">
							<Icon icon="solar:danger-triangle-bold" class="h-4.5 w-4.5" style="color: var(--color-danger);" />
						</div>
						<div class="min-w-0 flex-1">
							<h3 class="text-sm font-semibold" style="color: var(--color-text);">Delete Repository</h3>
							<p class="mt-1 text-xs" style="color: var(--color-text-secondary);">Are you sure you want to delete the entire repository <strong>{name}</strong>?</p>
							<p class="mt-1 text-[10px]" style="color: var(--color-text-muted);">All tags, manifests, and blobs will be permanently removed. This action is irreversible.</p>
						</div>
					</div>
				</div>
				<div class="flex items-center justify-end gap-2 rounded-b-xl border-t px-5 py-3" style="border-color: var(--color-border); background-color: var(--color-topbar-bg);">
					<button class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors" style="color: var(--color-text-secondary);" onclick={() => deleteRepoTarget = false}>Cancel</button>
					<button class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium text-white transition-colors" style="background-color: var(--color-danger);" onclick={confirmDeleteRepo} disabled={deleting}>
						{#if deleting}
							<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
							Deleting...
						{:else}
							<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
							Delete Repo
						{/if}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
