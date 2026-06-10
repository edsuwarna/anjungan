<script>
	import { onMount } from 'svelte';
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';
	import { currentProject } from '$lib/stores/auth.js';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import ProjectFormModal from '$lib/components/projects/ProjectFormModal.svelte';
	import DeleteProjectModal from '$lib/components/projects/DeleteProjectModal.svelte';

	let project = $state(null);
	let loading = $state(true);
	let error = $state('');

	// General form (inline)
	let formName = $state('');
	let formSlug = $state('');
	let formDescription = $state('');
	let generalSaving = $state(false);
	let generalError = $state('');
	let generalSuccess = $state('');

	// Project form modal (alternative edit)
	let showFormModal = $state(false);

	// Members
	let members = $state([]);
	let membersLoading = $state(true);
	let membersError = $state('');

	// Add member modal
	let showAddMember = $state(false);
	let addMemberUserID = $state('');
	let addMemberRole = $state('developer');
	let allUsers = $state([]);
	let addMemberSaving = $state(false);
	let addMemberError = $state('');

	// Delete modal
	let showDeleteModal = $state(false);

	const slug = $derived($page.params.slug);

	onMount(async () => {
		await loadProject();
	});

	async function loadProject() {
		loading = true;
		error = '';
		try {
			const data = await api.projects.list();
			const projects = data?.projects || [];
			const found = projects.find(p => p.slug === slug);
			if (!found) {
				error = `Project "${slug}" not found`;
				project = null;
				currentProject.set(null);
			} else {
				project = found;
				currentProject.set(found);
				formName = found.name;
				formSlug = found.slug;
				formDescription = found.description || '';
				await loadMembers();
			}
		} catch (e) {
			error = e.message || 'Failed to load project';
			project = null;
		} finally {
			loading = false;
		}
	}

	function slugify(str) {
		return str
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '');
	}

	function handleNameInput(e) {
		formName = e.target.value;
		formSlug = slugify(e.target.value);
	}

	// ─── General ──────────────────────────────────────────────────────────────

	async function handleGeneralSave() {
		if (!formName.trim()) {
			generalError = 'Name is required';
			return;
		}
		if (!formSlug.trim()) {
			generalError = 'Slug is required';
			return;
		}
		generalError = '';
		generalSuccess = '';
		generalSaving = true;
		try {
			const result = await api.projects.update(project.id, {
				name: formName.trim(),
				slug: formSlug.trim().toLowerCase(),
				description: formDescription.trim(),
			});
			// Update local project state
			project = { ...project, ...result };
			currentProject.set(project);
			generalSuccess = 'Project settings saved';
		} catch (e) {
			generalError = e.message;
		} finally {
			generalSaving = false;
		}
	}

	// ─── Members ──────────────────────────────────────────────────────────────

	async function loadMembers() {
		if (!project) return;
		membersLoading = true;
		membersError = '';
		try {
			const result = await api.projects.members.list(project.id);
			members = Array.isArray(result) ? result : (result?.members || []);
		} catch (e) {
			membersError = e.message;
		} finally {
			membersLoading = false;
		}
	}

	async function handleRoleChange(member, newRole) {
		try {
			await api.projects.members.update(project.id, member.user_id, { role: newRole });
			member.role = newRole;
		} catch (e) {
			alert('Failed to update role: ' + e.message);
		}
	}

	async function handleRemoveMember(member) {
		if (!confirm(`Remove ${member.name || member.email} from this project?`)) return;
		try {
			await api.projects.members.remove(project.id, member.user_id);
			members = members.filter(m => m.user_id !== member.user_id);
		} catch (e) {
			alert('Failed to remove member: ' + e.message);
		}
	}

	function openAddMember() {
		addMemberUserID = '';
		addMemberRole = 'developer';
		addMemberError = '';
		loadAllUsers();
		showAddMember = true;
	}

	async function loadAllUsers() {
		try {
			const result = await api.admin.users.list();
			allUsers = Array.isArray(result) ? result : (result?.users || []);
		} catch (e) {
			addMemberError = 'Failed to load users: ' + e.message;
		}
	}

	async function handleAddMember() {
		if (!addMemberUserID) {
			addMemberError = 'Please select a user';
			return;
		}
		addMemberError = '';
		addMemberSaving = true;
		try {
			const result = await api.projects.members.add(project.id, {
				user_id: addMemberUserID,
				role: addMemberRole,
			});
			// Reload members to get updated list
			await loadMembers();
			showAddMember = false;
		} catch (e) {
			addMemberError = e.message;
		} finally {
			addMemberSaving = false;
		}
	}

	// ─── Delete ───────────────────────────────────────────────────────────────

	function handleDeleted() {
		goto('/');
	}

	function roleLabel(role) {
		const labels = { admin: 'Admin', developer: 'Developer', viewer: 'Viewer' };
		return labels[role] || role;
	}
</script>

<div class="page-container">
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading project...</p>
			</div>
		</div>
	{:else if error}
		<div class="card text-center py-12">
			<Icon icon="solar:danger-triangle-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-danger);" />
			<p class="text-sm font-medium" style="color: var(--color-danger);">{error}</p>
			<button onclick={() => goto('/')} class="btn-secondary mt-4 text-sm">
				← Back to Dashboard
			</button>
		</div>
	{:else if project}
		<!-- Breadcrumb -->
		<nav class="breadcrumb">
			<a href="/">Dashboard</a>
			<span class="crumb-sep">›</span>
			<a href="/projects/{slug}">{project.name}</a>
			<span class="crumb-sep">›</span>
			<span class="current">Settings</span>
		</nav>

		<!-- Header -->
		<div class="flex items-start justify-between flex-wrap gap-3 mb-6">
			<div>
				<h1 class="page-title">Project Settings</h1>
				<p class="page-subtitle mt-1">Manage {project.name} configuration</p>
			</div>
		</div>

		<div class="space-y-6">
			<!-- ═══════════════════════════════════════════════════════════════════
			     SECTION: General
			     ═══════════════════════════════════════════════════════════════════ -->
			<div class="card">
				<div class="flex items-center gap-2 mb-4">
					<Icon icon="solar:info-circle-bold" class="h-5 w-5" style="color: var(--color-primary);" />
					<h2 class="text-base font-semibold" style="color: var(--color-text);">General</h2>
				</div>

				{#if generalSuccess}
					<div class="mb-4 rounded-lg px-4 py-2 text-sm" style="background-color: var(--color-success-subtle, #d1fae5); color: var(--color-success);">{generalSuccess}</div>
				{/if}
				{#if generalError}
					<div class="mb-4 rounded-lg px-4 py-2 text-sm" style="background-color: var(--color-danger-subtle, #fee2e2); color: var(--color-danger);">{generalError}</div>
				{/if}

				<div class="space-y-4">
					<!-- Name -->
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);" for="settings-name">Name *</label>
						<input
							id="settings-name"
							value={formName}
							oninput={handleNameInput}
							class="input w-full"
							placeholder="Project name"
						/>
					</div>

					<!-- Slug -->
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);" for="settings-slug">Slug *</label>
						<input
							id="settings-slug"
							bind:value={formSlug}
							class="input w-full font-mono text-sm"
							placeholder="my-project"
						/>
						<p class="mt-1 text-xs" style="color: var(--color-text-muted);">URL-friendly identifier, auto-generated from name</p>
					</div>

					<!-- Description -->
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);" for="settings-desc">Description</label>
						<textarea
							id="settings-desc"
							bind:value={formDescription}
							class="input w-full text-sm"
							rows="3"
							placeholder="Optional project description..."
						></textarea>
					</div>

					<!-- Actions -->
					<div class="flex items-center justify-between">
						<button
							onclick={() => showFormModal = true}
							class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors hover:opacity-80"
							style="background-color: var(--color-primary-subtle); color: var(--color-primary);"
						>
							<Icon icon="solar:pen-bold" class="h-4 w-4" />
							Edit in Modal
						</button>
						<button onclick={handleGeneralSave} disabled={generalSaving || !formName.trim() || !formSlug.trim()} class="btn-primary">
							{generalSaving ? 'Saving...' : 'Save Changes'}
						</button>
					</div>
				</div>
			</div>

			<!-- ═══════════════════════════════════════════════════════════════════
			     SECTION: Members
			     ═══════════════════════════════════════════════════════════════════ -->
			<div class="card">
				<div class="flex items-center justify-between gap-3 mb-4">
					<div class="flex items-center gap-2">
						<Icon icon="solar:users-group-rounded-bold" class="h-5 w-5" style="color: var(--color-primary);" />
						<h2 class="text-base font-semibold" style="color: var(--color-text);">Members</h2>
					</div>
					<button onclick={openAddMember} class="btn-primary flex items-center gap-2 text-sm">
						<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
						Add Member
					</button>
				</div>

				{#if membersLoading}
					<div class="flex items-center justify-center py-6">
						<Icon icon="solar:spinner-bold" class="h-6 w-6 animate-spin" style="color: var(--color-primary);" />
					</div>
				{:else if membersError}
					<div class="rounded-lg px-4 py-3 text-sm" style="background-color: var(--color-danger-subtle, #fee2e2); color: var(--color-danger);">{membersError}</div>
				{:else if members.length === 0}
					<div class="py-8 text-center">
						<Icon icon="solar:users-group-rounded-bold" class="mb-2 inline-block h-10 w-10" style="color: var(--color-text-muted);" />
						<p class="text-sm" style="color: var(--color-text-muted);">No members yet.</p>
					</div>
				{:else}
					<div class="overflow-x-auto -mx-6">
						<div class="data-table min-w-0">
							<table class="w-full">
								<thead>
									<tr>
										<th>Name</th>
										<th>Email</th>
										<th>Role</th>
										<th class="w-24">Actions</th>
									</tr>
								</thead>
								<tbody>
									{#each members as member (member.user_id)}
										<tr>
											<td class="max-w-[180px] truncate font-medium" style="color: var(--color-text);">
												{member.name || '-'}
											</td>
											<td class="max-w-[200px] truncate" style="color: var(--color-text-secondary);">
												{member.email || '-'}
											</td>
											<td>
												<select
													value={member.role}
													onchange={(e) => handleRoleChange(member, e.target.value)}
													class="input text-sm py-1 min-w-[120px]"
												>
													<option value="admin">Admin</option>
													<option value="developer">Developer</option>
													<option value="viewer">Viewer</option>
												</select>
											</td>
											<td>
												<button
													onclick={() => handleRemoveMember(member)}
													class="btn-icon h-8 w-8"
													title="Remove member"
													style="color: var(--color-danger);"
												>
													<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
												</button>
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					</div>
				{/if}
			</div>

			<!-- ═══════════════════════════════════════════════════════════════════
			     SECTION: Danger Zone
			     ═══════════════════════════════════════════════════════════════════ -->
			<div
				class="rounded-xl border-2 px-6 py-5"
				style="background-color: rgba(239,68,68,0.04); border-color: rgba(239,68,68,0.25);"
			>
				<div class="flex items-center gap-2 mb-3">
					<Icon icon="solar:danger-triangle-bold" class="h-5 w-5" style="color: var(--color-danger);" />
					<h2 class="text-base font-semibold" style="color: var(--color-danger);">Danger Zone</h2>
				</div>

				<p class="mb-1 text-sm" style="color: var(--color-text-secondary);">
					{#if project.resource_count}
						This project contains
						<strong>{Object.values(project.resource_count).reduce((a, b) => a + b, 0)} resources</strong>
						that will be rehomed to the Default Project.
					{:else}
						Deleting this project will unlink all associated resources.
					{/if}
				</p>
				<p class="mb-4 text-xs" style="color: var(--color-text-muted);">This action cannot be undone.</p>

				<button
					onclick={() => showDeleteModal = true}
					class="inline-flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium"
					style="background-color: var(--color-danger); color: #fff; border: none;"
				>
					<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
					Delete Project
				</button>
			</div>
		</div>
	{/if}
</div>

<!-- Project Form Modal (Edit) -->
<ProjectFormModal
	show={showFormModal}
	project={project}
	onClose={() => showFormModal = false}
	onSaved={(result) => {
		if (result) {
			project = { ...project, ...result };
			currentProject.set(project);
			// Sync inline form
			formName = project.name;
			formSlug = project.slug;
			formDescription = project.description || '';
		}
		showFormModal = false;
	}}
/>

<!-- Delete Project Modal -->
<DeleteProjectModal
	show={showDeleteModal}
	project={project}
	onClose={() => showDeleteModal = false}
	onDeleted={handleDeleted}
/>

<!-- Add Member Modal -->
{#if showAddMember}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background-color: rgba(0,0,0,0.5);"
		onclick={() => showAddMember = false}
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
			aria-label="Add Member"
		>
			<!-- Header -->
			<div class="flex items-center justify-between border-b px-6 py-4" style="border-color: var(--color-border);">
				<h2 class="text-lg font-semibold" style="color: var(--color-text);">Add Member</h2>
				<button onclick={() => showAddMember = false} class="rounded-lg p-1.5 transition-colors hover:opacity-80" style="color: var(--color-text-muted);" aria-label="Close">
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>

			<!-- Body -->
			<div class="space-y-4 px-6 py-4">
				{#if addMemberError}
					<div class="rounded-lg border px-4 py-3 text-sm" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2); color: var(--color-danger);">
						{addMemberError}
					</div>
				{/if}

				<!-- User selection -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="add-member-user">User *</label>
					<select id="add-member-user" bind:value={addMemberUserID} class="input w-full text-sm">
						<option value="">Select a user...</option>
						{#each allUsers as user}
							<option value={user.id}>
								{user.name || user.email} ({user.email})
							</option>
						{/each}
					</select>
				</div>

				<!-- Role selection -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="add-member-role">Role</label>
					<select id="add-member-role" bind:value={addMemberRole} class="input w-full text-sm">
						<option value="admin">Admin</option>
						<option value="developer">Developer</option>
						<option value="viewer">Viewer</option>
					</select>
				</div>
			</div>

			<!-- Footer -->
			<div class="flex items-center justify-end gap-3 border-t px-6 py-4" style="border-color: var(--color-border);">
				<button onclick={() => showAddMember = false} class="btn-secondary">Cancel</button>
				<button onclick={handleAddMember} disabled={!addMemberUserID || addMemberSaving} class="btn-primary flex items-center gap-2">
					{#if addMemberSaving}
						<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" />
					{:else}
						<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
					{/if}
					Add Member
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.breadcrumb {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-bottom: 16px;
		font-size: 12px;
	}
	.breadcrumb a {
		color: var(--color-text-muted);
		text-decoration: none;
		transition: color 0.15s;
	}
	.breadcrumb a:hover {
		color: var(--color-primary);
	}
	.crumb-sep {
		color: var(--color-text-muted);
		font-size: 10px;
	}
	.breadcrumb .current {
		color: var(--color-text-secondary);
		font-weight: 500;
	}
</style>
