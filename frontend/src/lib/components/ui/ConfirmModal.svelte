<script>
	import Icon from '@iconify/svelte';

	let {
		open = false,
		title = 'Confirm',
		message = 'Are you sure?',
		confirmText = 'Delete',
		cancelText = 'Cancel',
		variant = 'danger', // 'danger' or 'default'
		loading = false,
		onconfirm,
		oncancel,
		onclose,
		children,
	} = $props();

	function handleBackdrop(e) {
		if (e.target === e.currentTarget) {
			onclose?.();
		}
	}
</script>

{#if open}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background: rgba(0,0,0,0.5);"
		onclick={handleBackdrop}
		role="dialog"
		aria-modal="true"
	>
		<div
			class="w-full max-w-sm rounded-xl border p-6 shadow-xl"
			style="border-color: var(--color-border); background: var(--color-card);"
			onclick={(e) => e.stopPropagation()}
		>
			<!-- Header -->
			<div class="mb-4 flex items-center justify-between">
				<div class="flex items-center gap-2">
					{#if variant === 'danger'}
						<div class="flex h-8 w-8 items-center justify-center rounded-full" style="background: #ef444418;">
							<Icon icon="solar:danger-triangle-bold" class="h-4 w-4" style="color: #ef4444;" />
						</div>
					{/if}
					<h3 class="text-lg font-semibold" style="color: var(--color-text);">{title}</h3>
				</div>
				<button onclick={onclose} class="flex h-8 w-8 items-center justify-center rounded-lg transition-colors hover:bg-black/5 dark:hover:bg-white/5">
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" style="color: var(--color-text-muted);" />
				</button>
			</div>

			<!-- Body -->
			<div class="mb-6">
				{#if children}
					{@render children()}
				{:else}
					<p class="text-sm leading-relaxed" style="color: var(--color-text-muted);">{message}</p>
				{/if}
			</div>

			<!-- Actions -->
			<div class="flex items-center justify-end gap-3">
				<button
					onclick={() => { oncancel?.(); onclose?.(); }}
					class="rounded-lg border px-4 py-2 text-sm font-medium transition-colors"
					style="border-color: var(--color-border); color: var(--color-text-muted);"
				>
					{cancelText}
				</button>
				<button
					onclick={onconfirm}
					disabled={loading}
					class="flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50"
					style={variant === 'danger' ? 'background-color: #ef4444;' : 'background-color: var(--color-primary);'}
				>
					{#if loading}
						<Icon icon="svg-spinners:180-ring" class="h-4 w-4" />
					{/if}
					{confirmText}
				</button>
			</div>
		</div>
	</div>
{/if}
