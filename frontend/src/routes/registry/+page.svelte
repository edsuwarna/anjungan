<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';
	import { goto } from '$app/navigation';

	// ─── State ───
	let repos = $state([]);
	let loading = $state(true);
	let error = $state('');

	let searchQuery = $state('');
	let expandedRepo = $state(null);
	let repoTags = $state({}); // { [repoName]: [...tags] }
	let tagsLoading = $state({}); // { [repoName]: true|false }

	let pageLoading = $state(false);

	// Delete modal
	let deleteTarget = $state(null);

	// Credential reveal
	let showPassword = $state(false);

	// ─── Derived ───
	let filteredRepos = $derived.by(() => {
		if (!searchQuery) return repos;
		const q = searchQuery.toLowerCase();
		return repos.filter(r => r.name.toLowerCase().includes(q));
	});

	let totalTags = $derived(filteredRepos.reduce((s, r) => s + (r.tags_count || 0), 0));

	// ─── Mount ───
	onMount(() => {
		loadRepos();
	});

	async function loadRepos() {
		loading = true;
		error = '';
		try {
			const data = await api.registry.list();
			repos = Array.isArray(data) ? data : [];
		} catch (e) {
			error = e.message || 'Failed to load repositories';
		} finally {
			loading = false;
		}
	}

	async function toggleRepo(name) {
		if (expandedRepo === name) {
			expandedRepo = null;
			return;
		}
		expandedRepo = name;
		if (!repoTags[name]) {
			tagsLoading[name] = true;
			try {
				const data = await api.registry.listTags(name);
				repoTags[name] = data?.tags || [];
			} catch (e) {
				repoTags[name] = [];
			} finally {
				tagsLoading[name] = false;
			}
		}
	}

	async function handleDelete(repo, tag, digest) {
		deleteTarget = { repo, tag, digest };
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		const { repo, tag, digest } = deleteTarget;
		pageLoading = true;
		try {
			await api.registry.delete(repo, digest);
			// Remove tag from local state
			if (repoTags[repo]) {
				repoTags[repo] = repoTags[repo].filter(t => t.name !== tag);
			}
			deleteTarget = null;
		} catch (e) {
			error = e.message || 'Failed to delete tag';
		} finally {
			pageLoading = false;
		}
	}

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

	function copyToClipboard(text) {
		navigator.clipboard?.writeText(text);
	}
</script>

<div class="page-container">
	<!-- Connection Info -->
	<div class="card p-5">
		<div class="flex items-start justify-between mb-4">
			<div>
				<div class="flex items-center gap-2 mb-0.5">
					<Icon icon="solar:key-minimalistic-bold" class="h-4 w-4" style="color: var(--color-primary);" />
					<h3 class="text-sm font-semibold" style="color: var(--color-text);">Registry Connection</h3>
				</div>
				<p class="text-xs" style="color: var(--color-text-secondary);">Use these credentials to authenticate Docker CLI or CI/CD pipelines.</p>
			</div>
			<button
				class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
				style="background-color: var(--color-primary-subtle); color: var(--color-primary);"
				onclick={() => copyToClipboard(`docker login registry.anjungan.io -u deploy -p 'd3pl0y_c1cD_p4ss'`)}>
				<Icon icon="solar:copy-bold" class="h-3.5 w-3.5" />
				Copy Login Command
			</button>
		</div>
		<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
			<div class="rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
				<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Registry URL</label>
				<div class="mt-1 flex items-center gap-2">
					<code class="font-mono text-xs" style="color: var(--color-text);">registry.anjungan.io</code>
					<button class="flex-shrink-0" onclick={() => copyToClipboard('registry.anjungan.io')}>
						<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
					</button>
				</div>
			</div>
			<div class="rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
				<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Username</label>
				<div class="mt-1 flex items-center gap-2">
					<code class="font-mono text-xs" style="color: var(--color-text);">deploy</code>
					<button class="flex-shrink-0" onclick={() => copyToClipboard('deploy')}>
						<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
					</button>
				</div>
			</div>
			<div class="rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
				<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Password</label>
				<div class="mt-1 flex items-center gap-2">
					<code class="font-mono text-xs" style="color: var(--color-text-secondary);">
						{showPassword ? 'd3pl0y_c1cD_p4ss' : '••••••••••••'}
					</code>
					<button class="flex-shrink-0" onclick={() => showPassword = !showPassword}>
						<Icon icon={showPassword ? 'solar:eye-closed-outline' : 'solar:eye-outline'} class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
					</button>
					<button class="flex-shrink-0" onclick={() => copyToClipboard('d3pl0y_c1cD_p4ss')}>
						<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
					</button>
				</div>
			</div>
		</div>
		<div class="mt-3 rounded-lg border p-3" style="background-color: var(--color-card); border-color: var(--color-border-light);">
			<div class="flex items-center gap-2">
				<Icon icon="solar:code-outline" class="h-4 w-4 flex-shrink-0" style="color: var(--color-primary);" />
				<code class="flex-1 font-mono text-xs break-all" style="color: var(--color-text-secondary);">docker login registry.anjungan.io -u deploy -p 'd3pl0y_c1cD_p4ss'</code>
				<button class="flex-shrink-0" onclick={() => copyToClipboard("docker login registry.anjungan.io -u deploy -p 'd3pl0y_c1cD_p4ss'")}>
					<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
				</button>
			</div>
		</div>
	</div>

	<!-- Search + Stats -->
	<div class="flex items-center justify-between gap-4">
		<div class="relative flex-1 max-w-sm">
			<Icon icon="solar:magnifer-outline" class="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2" style="color: var(--color-text-muted);" />
			<input
				type="text"
				placeholder="Search repositories..."
				bind:value={searchQuery}
				class="w-full rounded-lg border py-2 pl-8 pr-3 text-xs"
				style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
			/>
		</div>
		<div class="flex items-center gap-4 text-xs" style="color: var(--color-text-secondary);">
			<span>{filteredRepos.length} repos</span>
			<span class="h-3 w-px" style="background-color: var(--color-border);"></span>
			<span>{totalTags} tags</span>
		</div>
	</div>

	<!-- Error -->
	{#if error}
		<div class="rounded-lg border p-3 text-xs" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2); color: var(--color-danger);">
			<div class="flex items-center gap-2">
				<Icon icon="solar:danger-triangle-bold" class="h-4 w-4" />
				<span>{error}</span>
			</div>
		</div>
	{/if}

	<!-- Loading -->
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<Icon icon="solar:spinner-bold" class="h-6 w-6 animate-spin" style="color: var(--color-primary);" />
		</div>
	{:else if filteredRepos.length === 0}
		<!-- Empty state -->
		<div class="flex flex-col items-center py-20 text-center">
			<Icon icon="solar:archive-down-minimlistic-bold" class="mb-3 h-10 w-10" style="color: var(--color-text-muted);" />
			<p class="text-sm" style="color: var(--color-text-secondary);">
				{searchQuery ? 'No repositories match your search' : 'No container images in registry'}
			</p>
			{#if searchQuery}
				<p class="mt-1 text-xs" style="color: var(--color-text-muted);">Try a different search term</p>
			{:else}
				<p class="mt-1 text-xs" style="color: var(--color-text-muted);">Push an image to get started</p>
			{/if}
		</div>
	{:else}
		<!-- Repo List -->
		<div class="space-y-1.5">
			{#each filteredRepos as repo}
				<div class="overflow-hidden rounded-lg border" style="background-color: var(--color-card); border-color: var(--color-border);">
					<!-- Repo Header (clickable) -->
					<button
						class="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:opacity-80"
						onclick={() => toggleRepo(repo.name)}
					>
						<Icon
							icon={expandedRepo === repo.name ? 'solar:box-bold' : 'solar:archive-down-minimlistic-bold'}
							class="h-5 w-5 flex-shrink-0"
							style="color: {expandedRepo === repo.name ? 'var(--color-primary)' : 'var(--color-text-muted)'};"
						/>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2">
								<span class="text-sm font-medium" style="color: var(--color-text);">{repo.name}</span>
							</div>
							<div class="mt-0.5 flex items-center gap-2 text-xs" style="color: var(--color-text-muted);">
								<span>{repo.tags_count || 0} tags</span>
							</div>
						</div>
						<Icon
							icon="solar:alt-arrow-down-outline"
							class="h-4 w-4 flex-shrink-0 transition-transform"
							style="color: var(--color-text-muted); transform: {expandedRepo === repo.name ? 'rotate(180deg)' : ''};"
						/>
					</button>

					<!-- Expanded Tags -->
					{#if expandedRepo === repo.name}
						<div class="border-t" style="border-color: var(--color-border);">
							{#if tagsLoading[repo.name]}
								<div class="flex items-center justify-center py-8">
									<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-primary);" />
								</div>
							{:else if repoTags[repo.name]?.length}
								<!-- Column headers -->
								<div class="flex items-center gap-3 px-4 py-2 text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">
									<span class="flex-1">TAG</span>
									<span class="w-20 text-right">SIZE</span>
									<span class="w-24 text-right">CREATED</span>
									<span class="w-28 text-right">DIGEST</span>
									<span class="w-16 text-right">ACTIONS</span>
								</div>
								{#each repoTags[repo.name] as tag}
									<div class="flex items-center gap-3 border-t px-4 py-2.5 transition-colors" style="border-color: var(--color-border);">
										<div class="min-w-0 flex-1">
											<button
												class="font-mono text-xs hover:underline"
												style="color: var(--color-primary);"
												onclick={() => goto(`/registry/${repo.name}/${tag.name}`)}
											>
												{tag.name}
											</button>
											{#if tag.name === 'latest'}
												<span class="ml-1.5 rounded px-1.5 py-0.5 text-[9px] font-medium" style="background-color: var(--color-primary-subtle); color: var(--color-primary);">latest</span>
											{/if}
										</div>
										<span class="w-20 flex-shrink-0 text-right font-mono text-xs" style="color: var(--color-text-muted);">{formatSize(tag.layer_size || tag.size)}</span>
										<span class="w-24 flex-shrink-0 text-right text-xs" style="color: var(--color-text-muted);">{formatDate(tag.created)}</span>
										<span class="w-28 flex-shrink-0 truncate text-right font-mono text-[10px]" style="color: var(--color-text-muted);">{shortDigest(tag.digest)}</span>
										<div class="flex w-16 flex-shrink-0 items-center justify-end gap-1">
											<button
												class="rounded-md p-1.5 transition-colors"
												style="color: var(--color-text-muted);"
												onclick={() => copyToClipboard(`docker pull registry.anjungan.io/${repo.name}:${tag.name}`)}
												title="Copy pull command"
											>
												<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" />
											</button>
											<button
												class="rounded-md p-1.5 transition-colors hover:opacity-80"
												style="color: var(--color-text-muted);"
												onclick={() => handleDelete(repo.name, tag.name, tag.digest)}
												title="Delete tag"
											>
												<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
											</button>
										</div>
									</div>
								{/each}
							{:else}
								<div class="py-6 text-center text-xs" style="color: var(--color-text-muted);">
									No tags found
								</div>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Delete Modal -->
{#if deleteTarget}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center"
		style="background-color: rgba(0,0,0,0.6);"
		onclick={() => deleteTarget = null}
	>
		<div
			class="mx-4 w-full max-w-md rounded-xl border shadow-2xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="p-5">
				<div class="flex items-start gap-3">
					<div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full" style="background-color: rgba(239,68,68,0.15);">
						<Icon icon="solar:danger-triangle-bold" class="h-4.5 w-4.5" style="color: var(--color-danger);" />
					</div>
					<div class="min-w-0 flex-1">
						<h3 class="text-sm font-semibold" style="color: var(--color-text);">Delete Image Tag</h3>
						<p class="mt-1 text-xs" style="color: var(--color-text-secondary);">Are you sure you want to delete this image tag? This action is irreversible.</p>

						<div class="mt-4 rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
							<div class="flex items-center justify-between py-1">
								<span class="text-xs" style="color: var(--color-text-muted);">Repository</span>
								<span class="font-mono text-xs font-medium" style="color: var(--color-text);">{deleteTarget.repo}</span>
							</div>
							<div class="flex items-center justify-between py-1">
								<span class="text-xs" style="color: var(--color-text-muted);">Tag</span>
								<span class="font-mono text-xs font-medium" style="color: var(--color-danger);">{deleteTarget.tag}</span>
							</div>
							<div class="flex items-center justify-between py-1">
								<span class="text-xs" style="color: var(--color-text-muted);">Digest</span>
								<span class="font-mono text-[10px]" style="color: var(--color-text-secondary);">{shortDigest(deleteTarget.digest)}</span>
							</div>
						</div>

						<div class="mt-3 rounded-lg border p-2.5" style="background-color: rgba(245,158,11,0.08); border-color: rgba(245,158,11,0.2);">
							<div class="flex items-start gap-2">
								<Icon icon="solar:info-circle-bold" class="mt-0.5 h-3.5 w-3.5 flex-shrink-0" style="color: var(--color-warning);" />
								<p class="text-xs" style="color: var(--color-text-secondary);">This will delete the manifest by digest. The tag reference will also be removed. Blob layers may be cleaned up by the garbage collector.</p>
							</div>
						</div>
					</div>
				</div>
			</div>
			<div class="flex items-center justify-end gap-2 rounded-b-xl border-t px-5 py-3" style="border-color: var(--color-border); background-color: var(--color-topbar-bg);">
				<button
					class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
					style="color: var(--color-text-secondary);"
					onclick={() => deleteTarget = null}
				>Cancel</button>
				<button
					class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium text-white transition-colors"
					style="background-color: var(--color-danger);"
					onclick={confirmDelete}
				>
					{#if pageLoading}
						<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
					{:else}
						<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
					{/if}
					Delete Tag
				</button>
			</div>
		</div>
	</div>
{/if}
