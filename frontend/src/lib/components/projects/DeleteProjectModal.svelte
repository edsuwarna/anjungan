<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';

	let { show = false, project = null, onClose = () => {}, onDeleted = () => {} } = $props();

	// Confirmation state
	let confirmText = $state('');
	let deleting = $state(false);
	let error = $state('');

	let canDelete = $derived(confirmText === 'delete');

	$effect(() => {
		if (show) {
			confirmText = '';
			error = '';
		}
	});

	function handleClose() {
		confirmText = '';
		onClose();
	}

	async function handleDelete() {
		if (!canDelete) return;

		deleting = true;
		error = '';
		try {
			const result = await api.projects.delete(project.id);
			onDeleted(result);
			handleClose();
		} catch (e) {
			error = e.message;
		} finally {
			deleting = false;
		}
	}

	function handleKeydown(e) {
		if (e.key === 'Escape') handleClose();
	}
</script>

{#if show}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background-color: rgba(0,0,0,0.5);"
		onclick={handleClose}
		onkeydown={handleKeydown}
		role="presentation"
	>
		<!-- svelte-ignore a11y_interactive_supports_focus -->
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div
			class="w-full max-w-md rounded-xl border shadow-xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-label="Delete {project?.name || 'Project'}"
		>
			<!-- Header -->
			<div class="flex items-center justify-between border-b px-6 py-4" style="border-color: var(--color-border);">
				<h2 class="flex items-center gap-2 text-lg font-semibold" style="color: var(--color-danger);">
					<Icon icon="solar:danger-triangle-bold" class="h-5 w-5" />
					Delete {project?.name || 'Project'}?
				</h2>
				<button
					onclick={handleClose}
					class="rounded-lg p-1.5 transition-colors hover:opacity-80"
					style="color: var(--color-text-muted);"
					aria-label="Close"
				>
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>

			<!-- Body -->
			<div class="space-y-4 px-6 py-4">
				{#if error}
					<div class="rounded-lg border px-4 py-3 text-sm" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2); color: var(--color-danger);">
						{error}
					</div>
				{/if}

				<!-- Warning info -->
				<div class="rounded-lg border p-4" style="background-color: rgba(239,68,68,0.05); border-color: rgba(239,68,68,0.15);">
					{#if project?.resource_count}
						<div class="mb-3 space-y-1">
							<p class="text-sm font-medium" style="color: var(--color-text);">This project contains:</p>
							<ul class="space-y-1">
								{#each Object.entries(project.resource_count) as [type, count]}
									{#if count > 0}
										<li class="flex items-center gap-2 text-sm" style="color: var(--color-text-secondary);">
											<Icon icon="solar:document-bold" class="h-3.5 w-3.5 shrink-0" style="color: var(--color-text-muted);" />
											{count} {type.replace(/_/g, ' ')}
										</li>
									{/if}
								{/each}
							</ul>
						</div>
					{/if}

					<p class="text-sm" style="color: var(--color-text-secondary);">
						All resources will be moved to the Default Project. This cannot be undone.
					</p>
				</div>

				<!-- Confirm text input -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="del-confirm">
						Type <span class="font-bold">delete</span> to confirm
					</label>
					<input
						id="del-confirm"
						bind:value={confirmText}
						placeholder="type &quot;delete&quot;"
						class="input w-full font-mono text-sm"
						class:input-error={confirmText && !canDelete}
					/>
				</div>
			</div>

			<!-- Footer -->
			<div class="flex items-center justify-end gap-3 border-t px-6 py-4" style="border-color: var(--color-border);">
				<button onclick={handleClose} class="btn-secondary">Cancel</button>
				<button
					onclick={handleDelete}
					disabled={!canDelete || deleting}
					class="flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-all"
					style="background-color: {canDelete && !deleting ? 'var(--color-danger)' : 'var(--color-border)'}; color: #fff; border: none;"
				>
					{#if deleting}
						<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" />
					{:else}
						<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
					{/if}
					Delete Project
				</button>
			</div>
		</div>
	</div>
{/if}
