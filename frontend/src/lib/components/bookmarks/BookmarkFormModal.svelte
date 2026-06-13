<script>
	import Icon from '@iconify/svelte';

	let { bookmark = null, existingCategories = [], onsave, onclose } = $props();

	// Form state
	let title = $state(bookmark?.title || '');
	let url = $state(bookmark?.url || '');
	let category = $state(bookmark?.category || 'Other');
	let description = $state(bookmark?.description || '');
	let pinned = $state(bookmark?.pinned || false);
	let iconType = $state(bookmark?.icon_type || 'auto');
	let iconValue = $state(bookmark?.icon_value || '');
	let saving = $state(false);
	let formError = $state('');

	// Favicon preview
	let faviconPreview = $state('');
	let faviconLoading = $state(false);

	let isEditing = $derived(!!bookmark);

	// Deduplicate and sort existing categories, ensure 'Other' is included
	let allCategoryOptions = $derived([...new Set(['Other', ...existingCategories])].sort());

	function updateFavicon() {
		if (!url.trim()) return;
		faviconLoading = true;
		try {
			const u = new URL(url.startsWith('http') ? url : 'https://' + url);
			faviconPreview = `https://www.google.com/s2/favicons?domain=${u.hostname}&sz=40`;
			iconType = 'auto';
			iconValue = '';
		} catch {
			faviconPreview = '';
		}
		faviconLoading = false;
	}

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

	async function handleSubmit() {
		formError = '';

		if (!title.trim()) {
			formError = 'Tool name is required';
			return;
		}
		if (title.trim().length > 100) {
			formError = 'Tool name must be 100 characters or less';
			return;
		}
		if (!url.trim()) {
			formError = 'URL is required';
			return;
		}

		// Auto-prepend https://
		let finalUrl = url.trim();
		if (!finalUrl.match(/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//)) {
			finalUrl = 'https://' + finalUrl;
		}

		// Validate URL format
		try {
			new URL(finalUrl);
		} catch {
			formError = 'Please enter a valid URL';
			return;
		}

		// Reject dangerous protocols
		const lower = finalUrl.toLowerCase();
		if (lower.startsWith('javascript:') || lower.startsWith('file:') || lower.startsWith('data:')) {
			formError = 'This URL protocol is not supported';
			return;
		}

		saving = true;
		try {
			await onsave({
				title: title.trim(),
				url: finalUrl,
				category: category.trim() || 'Other',
				description: description.trim(),
				pinned: pinned,
				icon_type: iconType,
				icon_value: iconValue,
			});
		} catch (e) {
			formError = e.message || 'Failed to save bookmark';
		} finally {
			saving = false;
		}
	}
</script>

<!-- Overlay -->
<div
	class="fixed inset-0 z-50 flex items-center justify-center p-4"
	style="background: rgba(0,0,0,0.5);"
	onclick={onclose}
	role="dialog"
	aria-modal="true"
>
	<!-- Modal -->
	<div
		class="w-full max-w-md rounded-xl border p-6 shadow-xl"
		style="border-color: var(--color-border); background: var(--color-card);"
		onclick={(e) => e.stopPropagation()}
	>
		<!-- Header -->
		<div class="mb-5 flex items-center justify-between">
			<h2 class="text-lg font-semibold" style="color: var(--color-text);">
				{isEditing ? 'Edit Bookmark' : 'Add Bookmark'}
			</h2>
			<button onclick={onclose} class="flex h-8 w-8 items-center justify-center rounded-lg transition-colors hover:bg-black/5 dark:hover:bg-white/5">
				<Icon icon="solar:close-circle-bold" class="h-5 w-5" style="color: var(--color-text-muted);" />
			</button>
		</div>

		<!-- Form -->
		<div class="space-y-4">
			<!-- Tool Name -->
			<div>
				<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Tool Name</label>
				<input
					type="text"
					bind:value={title}
					placeholder="e.g. Grafana"
					maxlength="100"
					class="w-full rounded-lg border px-3 py-2 text-sm outline-none transition-colors"
					style="border-color: var(--color-border); background: var(--color-surface); color: var(--color-text);"
				/>
			</div>

			<!-- URL -->
			<div>
				<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">URL</label>
				<div class="relative">
					<input
						type="text"
						bind:value={url}
						placeholder="https://grafana.internal"
						onblur={updateFavicon}
						class="w-full rounded-lg border px-3 py-2 pr-10 text-sm outline-none transition-colors"
						style="border-color: var(--color-border); background: var(--color-surface); color: var(--color-text);"
					/>
					<!-- Favicon preview -->
					<div class="absolute right-2 top-1/2 -translate-y-1/2">
						{#if faviconLoading}
							<Icon icon="svg-spinners:180-ring" class="h-5 w-5" style="color: var(--color-text-muted);" />
						{:else if faviconPreview}
							<img src={faviconPreview} alt="" class="h-5 w-5 rounded" onerror={(e) => { e.target.style.display = 'none'; }} />
						{/if}
					</div>
				</div>
			</div>

			<!-- Category: text input + pill suggestions (no confusing dropdown arrow) -->
			<div>
				<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Category</label>
				<input
					type="text"
					bind:value={category}
					placeholder="Type or pick a category..."
					class="w-full rounded-lg border px-3 py-2 text-sm outline-none transition-colors"
					style="border-color: var(--color-border); background: var(--color-surface); color: var(--color-text);"
				/>
				{#if allCategoryOptions.length > 0}
					<div class="mt-1.5 flex flex-wrap gap-1.5">
						{#each allCategoryOptions as cat}
							<button
								type="button"
								onclick={() => category = cat}
								class="inline-block rounded-full px-2 py-0.5 text-[10px] font-medium transition-opacity hover:opacity-80"
								style="background: {categoryColor(cat)}15; color: {categoryColor(cat)}; {category === cat ? 'outline: 2px solid ' + categoryColor(cat) + ';' : ''}"
							>
								{cat}
							</button>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Description -->
			<div>
				<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Description</label>
				<textarea
					bind:value={description}
					placeholder="What is this tool used for? (optional)"
					maxlength="200"
					rows="2"
					class="w-full resize-none rounded-lg border px-3 py-2 text-sm outline-none transition-colors"
					style="border-color: var(--color-border); background: var(--color-surface); color: var(--color-text);"
				></textarea>
			</div>

			<!-- Pinned toggle -->
			<div class="flex items-center justify-between rounded-lg border px-4 py-3" style="border-color: var(--color-border);">
				<div>
					<p class="text-sm font-medium" style="color: var(--color-text);">Pin to Quick Access</p>
					<p class="text-xs mt-0.5" style="color: var(--color-text-muted);">Shows in sidebar for fast access from any page</p>
				</div>
				<button
					onclick={() => pinned = !pinned}
					class="relative h-6 w-11 rounded-full transition-colors"
					style={pinned ? 'background-color: var(--color-primary);' : 'background-color: var(--color-border);'}
					role="switch"
					aria-checked={pinned}
				>
					<span
						class="absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white transition-transform"
						style={pinned ? 'transform: translateX(20px);' : ''}
					></span>
				</button>
			</div>

			<!-- Preview -->
			<div class="rounded-lg p-3" style="background: {categoryColor(category || 'Other')}08; border: 1px solid {categoryColor(category || 'Other')}20;">
				<p class="mb-1 text-xs font-medium" style="color: var(--color-text-muted);">Preview</p>
				<div class="flex items-center gap-3">
					<div class="flex h-10 w-10 items-center justify-center rounded-xl" style="background: {categoryColor(category || 'Other')}15;">
						{#if faviconPreview}
							<img src={faviconPreview} alt="" class="h-6 w-6 rounded" />
						{:else}
							<span class="text-sm font-bold" style="color: {categoryColor(category || 'Other')};">
								{title ? title.charAt(0).toUpperCase() : 'T'}
							</span>
						{/if}
					</div>
					<div class="min-w-0 flex-1">
						<p class="text-sm font-semibold truncate" style="color: var(--color-text);">{title || 'Tool Name'}</p>
						<p class="truncate text-xs" style="color: var(--color-text-muted);">{url ? url.replace(/^https?:\/\//, '') : 'domain.com'}</p>
					</div>
					<span class="inline-block shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium" style="background: {categoryColor(category || 'Other')}15; color: {categoryColor(category || 'Other')};">
						{category || 'Other'}
					</span>
				</div>
			</div>

			<!-- Error -->
			{#if formError}
				<div class="rounded-lg px-3 py-2 text-sm" style="background: #ef444418; color: #ef4444; border: 1px solid #ef444430;">
					{formError}
				</div>
			{/if}
		</div>

		<!-- Actions -->
		<div class="mt-6 flex items-center justify-end gap-3">
			<button
				onclick={onclose}
				class="rounded-lg border px-4 py-2 text-sm font-medium transition-colors"
				style="border-color: var(--color-border); color: var(--color-text-muted);"
			>
				Cancel
			</button>
			<button
				onclick={handleSubmit}
				disabled={saving}
				class="flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
				style="background-color: var(--color-primary);"
			>
				{#if saving}
					<Icon icon="svg-spinners:180-ring" class="h-4 w-4" />
				{/if}
				{isEditing ? 'Update' : 'Add'}
			</button>
		</div>
	</div>
</div>
