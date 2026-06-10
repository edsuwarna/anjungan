<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';

	let { show = false, project = null, onClose = () => {}, onSaved = () => {} } = $props();

	// Form state
	let name = $state('');
	let slug = $state('');
	let description = $state('');

	// UI state
	let saving = $state(false);
	let error = $state('');
	let nameError = $state('');
	let slugError = $state('');

	let isEdit = $derived(project !== null);

	function slugify(str) {
		return str
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '');
	}

	$effect(() => {
		if (show) {
			if (project) {
				name = project.name || '';
				slug = project.slug || '';
				description = project.description || '';
			} else {
				resetForm();
			}
			error = '';
			nameError = '';
			slugError = '';
		}
	});

	function resetForm() {
		name = '';
		slug = '';
		description = '';
		error = '';
		nameError = '';
		slugError = '';
	}

	function handleClose() {
		resetForm();
		onClose();
	}

	function handleNameInput(e) {
		const val = e.target.value;
		name = val;
		if (!isEdit) {
			slug = slugify(val);
		}
	}

	function validate() {
		let valid = true;
		nameError = '';
		slugError = '';

		if (!name.trim()) {
			nameError = 'Name is required';
			valid = false;
		}
		if (!slug.trim()) {
			slugError = 'Slug is required';
			valid = false;
		}
		return valid;
	}

	async function handleSave() {
		if (!validate()) return;

		saving = true;
		error = '';
		try {
			const data = {
				name: name.trim(),
				slug: slug.trim().toLowerCase(),
				description: description.trim(),
			};
			let result;
			if (isEdit) {
				result = await api.projects.update(project.id, data);
			} else {
				result = await api.projects.create(data);
			}
			onSaved(result);
			handleClose();
		} catch (e) {
			error = e.message;
		} finally {
			saving = false;
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
			class="w-full max-w-lg rounded-xl border shadow-xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-label={isEdit ? 'Edit Project' : 'Create Project'}
		>
			<!-- Header -->
			<div class="flex items-center justify-between border-b px-6 py-4" style="border-color: var(--color-border);">
				<h2 class="text-lg font-semibold" style="color: var(--color-text);">
					{isEdit ? 'Edit Project' : 'Create Project'}
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

				<!-- Name -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="proj-name">Name *</label>
					<input
						id="proj-name"
						value={name}
						oninput={handleNameInput}
						placeholder="My Project"
						class="input w-full"
						class:input-error={nameError}
					/>
					{#if nameError}
						<p class="mt-1 text-xs" style="color: var(--color-danger);">{nameError}</p>
					{/if}
				</div>

				<!-- Slug -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="proj-slug">Slug *</label>
					<input
						id="proj-slug"
						bind:value={slug}
						placeholder="my-project"
						class="input w-full font-mono text-sm"
						class:input-error={slugError}
					/>
					{#if slugError}
						<p class="mt-1 text-xs" style="color: var(--color-danger);">{slugError}</p>
					{/if}
					<p class="mt-1 text-xs" style="color: var(--color-text-muted);">URL-friendly identifier, auto-generated from name</p>
				</div>

				<!-- Description -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="proj-desc">Description</label>
					<textarea
						id="proj-desc"
						bind:value={description}
						placeholder="Optional project description..."
						class="input w-full text-sm"
						rows="3"
					></textarea>
				</div>
			</div>

			<!-- Footer -->
			<div class="flex items-center justify-end gap-3 border-t px-6 py-4" style="border-color: var(--color-border);">
				<button onclick={handleClose} class="btn-secondary">Cancel</button>
				<button onclick={handleSave} disabled={saving} class="btn-primary flex items-center gap-2">
					{#if saving}
						<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" />
					{:else}
						<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
					{/if}
					{isEdit ? 'Save Changes' : 'Save'}
				</button>
			</div>
		</div>
	</div>
{/if}
