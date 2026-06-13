<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';
	import BookmarkFormModal from '$lib/components/bookmarks/BookmarkFormModal.svelte';
	import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte';

	let bookmarks = $state([]);
	let loading = $state(true);
	let error = $state('');

	// Search & filter
	let searchQuery = $state('');
	let activeCategory = $state('All');

	// Modal
	let showModal = $state(false);
	let editingBookmark = $state(null);

	// Delete confirm
	let deletingBookmark = $state(null);

	// Derive categories dynamically from data
	let allCategories = $derived(['All', ...new Set(bookmarks.map(b => b.category).filter(Boolean))].sort());

	// Categorize items for grouped display — sort by frequency (most used first), then alpha
	let usedCategories = $derived([...new Set(bookmarks.map(b => b.category).filter(Boolean))]);

	let filteredBookmarks = $derived.by(() => {
		let filtered = bookmarks;
		if (activeCategory !== 'All') {
			filtered = filtered.filter(b => b.category === activeCategory);
		}
		if (searchQuery.trim()) {
			const q = searchQuery.trim().toLowerCase();
			filtered = filtered.filter(b =>
				b.title.toLowerCase().includes(q) || b.url.toLowerCase().includes(q)
			);
		}
		return filtered;
	});

	let groupedBookmarks = $derived.by(() => {
		const groups = {};
		// Order categories by frequency (most bookmarks first)
		const freq = {};
		for (const b of bookmarks) {
			freq[b.category] = (freq[b.category] || 0) + 1;
		}
		const catOrder = [...usedCategories].sort((a, b) => (freq[b] || 0) - (freq[a] || 0));
		for (const cat of catOrder) {
			const items = filteredBookmarks.filter(b => b.category === cat);
			if (items.length > 0) groups[cat] = items;
		}
		return groups;
	});

	let hasBookmarks = $derived(bookmarks.length > 0);
	let hasResults = $derived(filteredBookmarks.length > 0);

	// Generate a consistent color from any string
	function categoryColor(cat) {
		const predefined = {
			'Monitoring': '#10b981',
			'CI/CD': '#8b5cf6',
			'Logging': '#f59e0b',
			'Code & Registry': '#3b82f6',
			'Internal Tools': '#ec4899',
			'Other': '#64748b',
		};
		if (predefined[cat]) return predefined[cat];
		// Generate color from string hash
		let hash = 0;
		for (let i = 0; i < cat.length; i++) {
			hash = cat.charCodeAt(i) + ((hash << 5) - hash);
		}
		const h = Math.abs(hash) % 360;
		return `hsl(${h}, 55%, 50%)`;
	}

	onMount(async () => {
		await loadBookmarks();
	});

	async function loadBookmarks() {
		loading = true;
		error = '';
		try {
			bookmarks = await api.bookmarks.list() || [];
		} catch (e) {
			error = e.message || 'Failed to load bookmarks';
		} finally {
			loading = false;
		}
	}

	function openAdd() {
		editingBookmark = null;
		showModal = true;
	}

	function openEdit(b) {
		editingBookmark = b;
		showModal = true;
	}

	async function handleSave(data) {
		try {
			if (editingBookmark) {
				await api.bookmarks.update(editingBookmark.id, data);
			} else {
				await api.bookmarks.create(data);
			}
			showModal = false;
			editingBookmark = null;
			await loadBookmarks();
			window.dispatchEvent(new CustomEvent('bookmarks-changed'));
		} catch (e) {
			throw e; // Let modal handle error
		}
	}

	function handleDelete(id) {
		deletingBookmark = bookmarks.find(b => b.id === id) || null;
	}

	async function confirmDelete() {
		if (!deletingBookmark) return;
		try {
			await api.bookmarks.delete(deletingBookmark.id);
			deletingBookmark = null;
			await loadBookmarks();
			window.dispatchEvent(new CustomEvent('bookmarks-changed'));
		} catch (e) {
			error = e.message || 'Failed to delete bookmark';
			deletingBookmark = null;
		}
	}

	async function togglePin(b) {
		try {
			await api.bookmarks.update(b.id, { pinned: !b.pinned });
			await loadBookmarks();
			window.dispatchEvent(new CustomEvent('bookmarks-changed'));
		} catch (e) {
			error = e.message || 'Failed to update bookmark';
		}
	}

	function faviconUrl(url) {
		try {
			const u = new URL(url);
			return `https://www.google.com/s2/favicons?domain=${u.hostname}&sz=40`;
		} catch {
			return '';
		}
	}

	function firstLetter(name) {
		return name.charAt(0).toUpperCase();
	}

	function openUrl(url) {
		window.open(url, '_blank', 'noopener,noreferrer');
	}
</script>

<div class="page-container">
	<!-- Page Header -->
	<div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-xl font-bold" style="color: var(--color-text);">Bookmarks</h1>
			<p class="text-sm mt-1" style="color: var(--color-text-muted);">Your favorite tool shortcuts</p>
		</div>
		<div class="flex items-center gap-3">
			{#if hasBookmarks}
				<div class="relative flex-1 sm:flex-none">
					<Icon icon="solar:magnifer-bold" class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2" style="color: var(--color-text-muted);" />
					<input
						type="text"
						placeholder="Search bookmarks..."
						bind:value={searchQuery}
						class="w-full rounded-lg border py-2 pl-9 pr-3 text-sm outline-none transition-colors sm:w-64"
						style="border-color: var(--color-border); background: var(--color-card); color: var(--color-text);"
					/>
				</div>
			{/if}
			<button
				onclick={openAdd}
				class="flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors"
				style="background-color: var(--color-primary);"
			>
				<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
				Add Bookmark
			</button>
		</div>
	</div>

	<!-- Error -->
	{#if error}
		<div class="mb-4 rounded-lg px-4 py-3 text-sm" style="background: #ef444418; color: #ef4444; border: 1px solid #ef444430;">
			{error}
		</div>
	{/if}

	<!-- Loading -->
	{#if loading}
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
			{#each { length: 4 } as _}
				<div class="animate-pulse rounded-xl border p-4" style="border-color: var(--color-border); background: var(--color-card);">
					<div class="flex items-start gap-3">
						<div class="h-10 w-10 rounded-xl bg-gray-200 dark:bg-gray-700"></div>
						<div class="flex-1 space-y-2">
							<div class="h-4 w-24 rounded bg-gray-200 dark:bg-gray-700"></div>
							<div class="h-3 w-32 rounded bg-gray-200 dark:bg-gray-700"></div>
						</div>
					</div>
				</div>
			{/each}
		</div>

	<!-- Empty state -->
	{:else if !hasBookmarks}
		<div class="flex flex-col items-center justify-center rounded-xl border py-16" style="border-color: var(--color-border); background: var(--color-card);">
			<Icon icon="solar:bookmark-square-bold" class="mb-4 h-16 w-16" style="color: var(--color-text-muted);" />
			<h3 class="text-lg font-semibold mb-2" style="color: var(--color-text);">No bookmarks yet</h3>
			<p class="mb-6 text-sm" style="color: var(--color-text-muted);">Add your first tool to get started — Grafana, Jenkins, Harbor, etc.</p>
			<button
				onclick={openAdd}
				class="flex items-center gap-2 rounded-lg px-5 py-2.5 text-sm font-medium text-white transition-colors"
				style="background-color: var(--color-primary);"
			>
				<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
				Add Bookmark
			</button>
		</div>

	<!-- Category filter pills + results -->
	{:else}
		<div class="mb-4 flex flex-wrap gap-2">
			{#each allCategories as cat}
				<button
					onclick={() => activeCategory = cat}
					class="rounded-full px-3 py-1.5 text-xs font-medium transition-all"
					style={activeCategory === cat
						? 'background: var(--color-primary); color: white;'
						: `background: ${categoryColor(cat === 'All' ? 'Other' : cat)}12; color: ${categoryColor(cat === 'All' ? 'Other' : cat)};`}
				>
					{cat}
				</button>
			{/each}
		</div>

		<!-- No results after search/filter -->
		{#if !hasResults}
			<div class="flex flex-col items-center justify-center rounded-xl border py-12" style="border-color: var(--color-border); background: var(--color-card);">
				<Icon icon="solar:magnifer-bold" class="mb-3 h-10 w-10" style="color: var(--color-text-muted);" />
				<p class="text-sm" style="color: var(--color-text-muted);">No bookmarks match your search</p>
				<button onclick={() => { searchQuery = ''; activeCategory = 'All'; }} class="mt-2 text-xs font-medium hover:underline" style="color: var(--color-primary);">
					Clear filters
				</button>
			</div>
		{:else}
			<!-- Category sections -->
			{#each Object.entries(groupedBookmarks) as [category, items]}
				<div class="mb-6">
					<h2 class="mb-3 text-sm font-semibold uppercase tracking-wider" style="color: {categoryColor(category)};">
						{category}
					</h2>
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
					{#each items as b}
							<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
							<div
								onclick={() => openUrl(b.url)}
								role="button"
								tabindex="0"
								onkeypress={(e) => { if (e.key === 'Enter') openUrl(b.url); }}
								class="group rounded-xl border p-4 text-left transition-all hover:-translate-y-0.5"
								style="border-color: var(--color-border); background: var(--color-card);"
							>
								<div class="flex items-start gap-3">
									<!-- Icon -->
									<div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl" style="background: {categoryColor(b.category)}15;">
										{#if b.icon_type === 'auto' && b.url}
											<img
												src={faviconUrl(b.url)}
												alt=""
												class="h-6 w-6 rounded"
												onerror={(e) => { e.target.style.display = 'none'; e.target.nextElementSibling.style.display = 'flex'; }}
											/>
											<div class="hidden h-6 w-6 items-center justify-center text-sm font-bold" style="color: {categoryColor(b.category)};">
												{firstLetter(b.title)}
											</div>
										{:else if b.icon_type === 'iconify' && b.icon_value}
											<Icon icon={b.icon_value} class="h-5 w-5" style="color: {categoryColor(b.category)};" />
										{:else if b.icon_type === 'emoji' && b.icon_value}
											<span class="text-lg">{b.icon_value}</span>
										{:else}
											<span class="text-sm font-bold" style="color: {categoryColor(b.category)};">{firstLetter(b.title)}</span>
										{/if}
									</div>

									<!-- Content -->
									<div class="min-w-0 flex-1">
										<p class="truncate text-sm font-semibold group-hover:underline" style="color: var(--color-text);">
											{b.title}
										</p>
										<p class="mt-0.5 truncate text-xs" style="color: var(--color-text-muted);">
											{b.url.replace(/^https?:\/\//, '')}
										</p>
										{#if b.description}
											<p class="mt-1 line-clamp-2 text-xs leading-relaxed" style="color: var(--color-text-muted); opacity: 0.8;">
												{b.description}
											</p>
										{/if}
										<div class="mt-2">
											<span class="inline-block rounded-full px-2 py-0.5 text-[10px] font-medium" style="background: {categoryColor(b.category)}15; color: {categoryColor(b.category)};">
												{b.category}
											</span>
										</div>
									</div>

									<!-- Actions -->
									<div class="flex shrink-0 flex-col gap-1 opacity-0 transition-opacity group-hover:opacity-100" onclick={(e) => e.stopPropagation()}>
										<button
											onclick={() => togglePin(b)}
											class="flex h-7 w-7 items-center justify-center rounded-lg transition-colors hover:bg-black/5 dark:hover:bg-white/5"
											title={b.pinned ? 'Unpin from Quick Access' : 'Pin to Quick Access'}
										>
											<Icon icon={b.pinned ? 'solar:pin-bold' : 'solar:pin-outline'} class="h-3.5 w-3.5" style="color: {b.pinned ? 'var(--color-primary)' : 'var(--color-text-muted)'};" />
										</button>
										<button
											onclick={() => openEdit(b)}
											class="flex h-7 w-7 items-center justify-center rounded-lg transition-colors hover:bg-black/5 dark:hover:bg-white/5"
											title="Edit"
										>
											<Icon icon="solar:pen-2-bold" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
										</button>
										<button
											onclick={() => handleDelete(b.id)}
											class="flex h-7 w-7 items-center justify-center rounded-lg transition-colors hover:bg-red-500/10"
											title="Delete"
										>
											<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" style="color: #ef4444;" />
										</button>
									</div>
								</div>
							</div>
						{/each}
					</div>
				</div>
			{/each}
		{/if}
	{/if}
</div>

<!-- Add/Edit Modal -->
{#if showModal}
	<BookmarkFormModal
		bookmark={editingBookmark}
		existingCategories={usedCategories}
		onsave={handleSave}
		onclose={() => { showModal = false; editingBookmark = null; }}
	/>
{/if}

<!-- Delete Confirmation Modal -->
<ConfirmModal
	open={deletingBookmark !== null}
	title="Delete Bookmark"
	message={deletingBookmark ? `Delete "${deletingBookmark.title}"? This cannot be undone.` : ''}
	confirmText="Delete"
	variant="danger"
	onconfirm={confirmDelete}
	oncancel={() => deletingBookmark = null}
	onclose={() => deletingBookmark = null}
/>

<style>
	.page-container {
		padding: 24px;
		max-width: 1280px;
		margin: 0 auto;
	}
</style>
