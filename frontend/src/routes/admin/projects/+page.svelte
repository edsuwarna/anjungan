<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';

	const DEFAULT_PROJECT_ID = '00000000-0000-0000-0000-000000000001';

	let projects = $state([]);
	let loading = $state(true);
	let error = $state('');

	// Modals
	let showFormModal = $state(false);
	let showDeleteModal = $state(false);
	let editingProject = $state(null);
	let deletingProject = $state(null);
	let saving = $state(false);

	// Form state
	let formName = $state('');
	let formSlug = $state('');
	let formDescription = $state('');

	let deleteProjectName = $state('');

	onMount(() => {
		loadProjects();
	});

	async function loadProjects() {
		loading = true;
		error = '';
		try {
			const result = await api.projects.list();
			projects = Array.isArray(result) ? result : (result?.projects || []);
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function openAdd() {
		editingProject = null;
		formName = '';
		formSlug = '';
		formDescription = '';
		showFormModal = true;
	}

	function openEdit(project) {
		editingProject = project;
		formName = project.name;
		formSlug = project.slug;
		formDescription = project.description || '';
		showFormModal = true;
	}

	function openDelete(project) {
		deletingProject = project;
		deleteProjectName = project.name;
		showDeleteModal = true;
	}

	async function handleSave() {
		if (!formName) return;
		saving = true;
		try {
			const payload = {
				name: formName,
				slug: formSlug,
				description: formDescription,
			};
			if (editingProject) {
				await api.projects.update(editingProject.id, payload);
			} else {
				await api.projects.create(payload);
			}
			showFormModal = false;
			editingProject = null;
			await loadProjects();
		} catch (e) {
			alert(e.message);
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!deletingProject) return;
		saving = true;
		try {
			await api.projects.delete(deletingProject.id);
			showDeleteModal = false;
			deletingProject = null;
			await loadProjects();
		} catch (e) {
			alert(e.message);
		} finally {
			saving = false;
		}
	}

	function isDefaultProject(project) {
		return project.id === DEFAULT_PROJECT_ID;
	}

	let sortedProjects = $derived([...projects].sort((a, b) => {
		const aName = (a.name || '').toLowerCase();
		const bName = (b.name || '').toLowerCase();
		return aName.localeCompare(bName);
	}));

	function formatResourceCount(project) {
		const rc = project.resource_count || {};
		const parts = [];
		if (rc.servers > 0) parts.push(`${rc.servers} servers`);
		if (rc.ssl_monitors > 0) parts.push(`${rc.ssl_monitors} SSL`);
		if (rc.uptime_monitors > 0) parts.push(`${rc.uptime_monitors} uptime`);
		return parts.length > 0 ? parts.join(', ') : '—';
	}
</script>

<div class="page-container">
	<div class="flex items-start sm:items-center justify-between gap-3 flex-wrap">
		<div class="min-w-0 flex-1">
			<h1 class="page-title">Projects</h1>
			<p class="page-subtitle">Manage projects and their resources</p>
		</div>
		<button onclick={openAdd} class="btn-primary flex items-center gap-2 shrink-0 z-10">
			<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
			<span class="hidden sm:inline">New Project</span>
		</button>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading projects...</p>
			</div>
		</div>
	{:else if error}
		<div class="card flex flex-col items-center gap-3 py-10 text-center">
			<Icon icon="solar:danger-triangle-bold" class="mb-1 h-8 w-8" style="color: var(--color-danger);" />
			<p style="color: var(--color-danger);">Failed to load projects</p>
			<p class="text-sm" style="color: var(--color-text-muted);">{error}</p>
			<button onclick={loadProjects} class="btn-secondary mt-2">Retry</button>
		</div>
	{:else if projects.length === 0}
		<div class="card flex flex-col items-center py-16 text-center">
			<Icon icon="solar:folder-bold" class="mb-3 h-12 w-12" style="color: var(--color-text-muted);" />
			<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No projects yet</h3>
			<p class="text-sm" style="color: var(--color-text-secondary);">Create the first project to get started.</p>
			<button onclick={openAdd} class="btn-primary mt-4 flex items-center gap-2">
				<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
				New Project
			</button>
		</div>
	{:else}
		<div class="overflow-x-auto -mx-4 md:mx-0">
			<div class="data-table min-w-0 md:min-w-[700px]">
				<table class="w-full">
					<thead>
						<tr>
							<th>Name</th>
							<th class="hidden sm:table-cell">Slug</th>
							<th class="hidden md:table-cell">Resources</th>
							<th class="hidden lg:table-cell">Members</th>
							<th class="hidden lg:table-cell">Created</th>
							<th class="w-16 md:w-20">Actions</th>
						</tr>
					</thead>
					<tbody>
						{#each sortedProjects as project (project.id)}
							<tr class:opacity-50={isDefaultProject(project)}>
								<td class="font-medium max-w-[160px] truncate" style="color: var(--color-text);">
									{#if isDefaultProject(project)}
										<span class="flex items-center gap-2">
											<Icon icon="solar:folder-bold" class="h-4 w-4 shrink-0" style="color: var(--color-text-muted);" />
											{project.name}
										</span>
									{:else}
										<a href="/projects/{project.slug}/settings" class="flex items-center gap-2 hover:underline">
											<Icon icon="solar:folder-bold" class="h-4 w-4 shrink-0" style="color: var(--color-primary);" />
											{project.name}
										</a>
									{/if}
								</td>
								<td class="hidden sm:table-cell max-w-[120px] truncate" style="color: var(--color-text-secondary);">
									<code class="text-xs">{project.slug}</code>
								</td>
								<td class="hidden md:table-cell max-w-[200px] truncate" style="color: var(--color-text-secondary); font-size: 0.8125rem;">
									{formatResourceCount(project)}
								</td>
								<td class="hidden lg:table-cell text-sm" style="color: var(--color-text-muted);">
									{project.member_count ?? 0}
								</td>
								<td class="hidden lg:table-cell text-sm" style="color: var(--color-text-muted);">
									{new Date(project.created_at).toLocaleDateString()}
								</td>
								<td>
									<div class="flex items-center gap-1">
										<button onclick={() => openEdit(project)} class="btn-icon h-8 w-8" title="Edit project">
											<Icon icon="solar:pen-bold" class="h-4 w-4" />
										</button>
										{#if !isDefaultProject(project)}
											<button onclick={() => openDelete(project)} class="btn-icon h-8 w-8" title="Delete project" style="color: var(--color-danger);">
												<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
											</button>
										{/if}
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</div>

<!-- Project Form Modal (Create / Edit) -->
{#if showFormModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onclick={() => showFormModal = false}>
		<div class="w-full max-w-md rounded-xl border p-0 shadow-xl animate-slide-in" style="background-color: var(--color-card); border-color: var(--color-border-light);" onclick={(e) => e.stopPropagation()}>
			<div class="flex items-center justify-between border-b px-6 py-4" style="border-color: var(--color-border-light);">
				<h3 class="text-lg font-semibold" style="color: var(--color-text);">
					{editingProject ? 'Edit Project' : 'New Project'}
				</h3>
				<button onclick={() => showFormModal = false} class="btn-icon">
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>
			<div class="space-y-4 px-6 py-4">
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Name</label>
					<input bind:value={formName} class="input" placeholder="My Project" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Slug</label>
					<input bind:value={formSlug} class="input" placeholder="my-project" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Description</label>
					<textarea bind:value={formDescription} class="input" rows="3" placeholder="Optional project description"></textarea>
				</div>
			</div>
			<div class="flex justify-end gap-3 border-t px-6 py-4" style="border-color: var(--color-border-light);">
				<button onclick={() => showFormModal = false} class="btn-secondary">Cancel</button>
				<button onclick={handleSave} disabled={!formName || saving} class="btn-primary">
					{saving ? 'Saving...' : editingProject ? 'Save Changes' : 'Create Project'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Delete Confirmation Modal -->
{#if showDeleteModal && deletingProject}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onclick={() => showDeleteModal = false}>
		<div class="w-full max-w-sm rounded-xl border p-0 shadow-xl animate-slide-in" style="background-color: var(--color-card); border-color: var(--color-border-light);" onclick={(e) => e.stopPropagation()}>
			<div class="px-6 py-6 text-center">
				<Icon icon="solar:danger-triangle-bold" class="mb-3 inline-block h-12 w-12" style="color: var(--color-danger);" />
				<h3 class="mb-2 text-lg font-semibold" style="color: var(--color-text);">Delete Project</h3>
				<p class="text-sm" style="color: var(--color-text-secondary);">
					Are you sure you want to delete <strong style="color: var(--color-text);">{deleteProjectName}</strong>?
				</p>
				<p class="mt-2 text-xs" style="color: var(--color-warning);">This action cannot be undone. All associated resources will be unlinked.</p>
			</div>
			<div class="flex justify-end gap-3 border-t px-6 py-4" style="border-color: var(--color-border-light);">
				<button onclick={() => showDeleteModal = false} class="btn-secondary">Cancel</button>
				<button onclick={handleDelete} disabled={saving} class="btn-primary flex items-center gap-2" style="background-color: var(--color-danger);">
					{saving ? 'Deleting...' : 'Delete Project'}
				</button>
			</div>
		</div>
	</div>
{/if}
