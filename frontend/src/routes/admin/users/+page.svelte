<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';
	import { goto } from '$app/navigation';

	let users = $state([]);
	let loading = $state(true);
	let error = $state('');

	// Sort
	let sortColumn = $state('name');
	let sortOrder = $state('asc');

	function toggleSort(col) {
		if (sortColumn === col) {
			sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		} else {
			sortColumn = col;
			sortOrder = 'asc';
		}
	}

	function sortIcon(col) {
		if (sortColumn !== col) return 'solar:arrow-up-wide-narrow-linear';
		return sortOrder === 'asc' ? 'solar:sort-from-top-bold' : 'solar:sort-from-bottom-bold';
	}

	let sortedUsers = $derived([...users].sort((a, b) => {
		let aVal = a[sortColumn];
		let bVal = b[sortColumn];
		if (sortColumn === 'name') {
			aVal = (a.name || '').toLowerCase();
			bVal = (b.name || '').toLowerCase();
		} else if (sortColumn === 'email') {
			aVal = (a.email || '').toLowerCase();
			bVal = (b.email || '').toLowerCase();
		} else if (sortColumn === 'role') {
			aVal = (a.role || '');
			bVal = (b.role || '');
		} else if (sortColumn === 'status') {
			aVal = (a.status || '');
			bVal = (b.status || '');
		} else if (sortColumn === 'created_at') {
			aVal = a.created_at || '';
			bVal = b.created_at || '';
		}
		if (aVal < bVal) return sortOrder === 'asc' ? -1 : 1;
		if (aVal > bVal) return sortOrder === 'asc' ? 1 : -1;
		return 0;
	}));

	// Modals
	let showAddModal = $state(false);
	let showEditModal = $state(false);
	let showDeleteModal = $state(false);
	let editingUser = $state(null);
	let deletingUser = $state(null);
	let saving = $state(false);

	// Form
	let formEmail = $state('');
	let formName = $state('');
	let formPassword = $state('');
	let formRole = $state('developer');
	let formGroups = $state([]);
	let allGroups = $state([]);

	async function loadGroups() {
		try {
			const result = await api.servers.groups();
			allGroups = Array.isArray(result) ? result : [];
		} catch {
			allGroups = [];
		}
	}

	onMount(() => {
		loadUsers();
		loadGroups();
	});

	function toggleGroup(group) {
		if (formGroups.includes(group)) {
			formGroups = formGroups.filter(g => g !== group);
		} else {
			formGroups = [...formGroups, group];
		}
	}

	$effect(() => {
		if (formRole === 'admin' && formGroups.length > 0) {
			formGroups = [];
		}
	});

	async function loadUsers() {
		loading = true;
		error = '';
		try {
			users = await api.admin.users.list();
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function openAdd() {
		formEmail = '';
		formName = '';
		formPassword = '';
		formRole = 'developer';
		formGroups = [];
		showAddModal = true;
	}

	function openEdit(user) {
		editingUser = user;
		formEmail = user.email;
		formName = user.name;
		formRole = user.role;
		formPassword = '';
		formGroups = user.allowed_groups || [];
		showEditModal = true;
	}

	function openDelete(user) {
		deletingUser = user;
		showDeleteModal = true;
	}

	async function handleCreate() {
		if (!formEmail || !formName || !formPassword) return;
		saving = true;
		try {
			await api.admin.users.create({
				email: formEmail,
				name: formName,
				password: formPassword,
				role: formRole,
				allowed_groups: formGroups,
			});
			showAddModal = false;
			await loadUsers();
		} catch (e) {
			alert(e.message);
		} finally {
			saving = false;
		}
	}

	async function handleUpdate() {
		if (!editingUser) return;
		saving = true;
		try {
			const payload = { email: formEmail, name: formName, role: formRole, allowed_groups: formGroups };
			if (formPassword) payload.password = formPassword;
			await api.admin.users.update(editingUser.id, payload);
			showEditModal = false;
			editingUser = null;
			await loadUsers();
		} catch (e) {
			alert(e.message);
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!deletingUser) return;
		saving = true;
		try {
			await api.admin.users.delete(deletingUser.id);
			showDeleteModal = false;
			deletingUser = null;
			await loadUsers();
		} catch (e) {
			alert(e.message);
		} finally {
			saving = false;
		}
	}

	function roleBadgeClass(role) {
		if (role === 'admin') return 'status-badge online';
		if (role === 'developer') return 'status-badge';
		if (role === 'viewer') return 'status-badge pending';
		return 'status-badge';
	}

	// ─── Unlock ─────────────────────────────────────────────────────────────
	async function handleUnlock(user) {
		if (!confirm(`Unlock user ${user.name} (${user.email})?`)) return;
		try {
			await api.admin.users.unlock(user.id);
			await loadUsers();
		} catch (e) {
			alert(e.message);
		}
	}
</script>

<div class="page-container">
	<div class="flex items-start sm:items-center justify-between gap-3 flex-wrap">
		<div class="min-w-0 flex-1">
			<h1 class="page-title">Users</h1>
			<p class="page-subtitle">Manage registered users and roles</p>
		</div>
		<button onclick={openAdd} class="btn-primary flex items-center gap-2 shrink-0 z-10">
			<Icon icon="solar:user-plus-bold" class="h-4 w-4" />
			<span class="hidden sm:inline">Add User</span>
		</button>
	</div>

	<!-- Login Security Policy -->
	<div class="card" style="border-left: 3px solid var(--color-success);">
		<div class="flex items-center gap-2 mb-3">
			<Icon icon="solar:shield-keyhole-bold" class="h-4 w-4" style="color: var(--color-success);" />
			<h3 class="text-sm font-semibold" style="color: var(--color-text);">Login Security Policy</h3>
		</div>
		<div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
			<div>
				<p class="text-xs font-medium uppercase tracking-wider mb-0.5" style="color: var(--color-text-muted);">Max Login Attempts</p>
				<p class="text-lg font-bold" style="color: var(--color-text);">5</p>
			</div>
			<div>
				<p class="text-xs font-medium uppercase tracking-wider mb-0.5" style="color: var(--color-text-muted);">Lockout Duration</p>
				<p class="text-lg font-bold" style="color: var(--color-text);">30 min</p>
			</div>
			<div>
				<p class="text-xs font-medium uppercase tracking-wider mb-0.5" style="color: var(--color-text-muted);">Rate Limit Window</p>
				<p class="text-lg font-bold" style="color: var(--color-text);">15 min</p>
			</div>
			<div>
				<p class="text-xs font-medium uppercase tracking-wider mb-0.5" style="color: var(--color-text-muted);">Min Password Length</p>
				<p class="text-lg font-bold" style="color: var(--color-text);">8</p>
			</div>
		</div>
	</div>

	<!-- Recent Lockout Events -->
	<div class="card">
		<div class="flex items-center gap-2 mb-3">
			<Icon icon="solar:lock-bold" class="h-4 w-4" style="color: var(--color-warning);" />
			<h3 class="text-sm font-semibold" style="color: var(--color-text);">Recent Lockout Events</h3>
		</div>
		<div class="flex flex-col items-center py-8 text-center">
			<Icon icon="solar:check-circle-bold" class="mb-2 h-8 w-8" style="color: var(--color-text-muted);" />
			<p class="text-sm" style="color: var(--color-text-muted);">No recent lockout events</p>
		</div>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading users...</p>
			</div>
		</div>
	{:else if error}
		<div class="card flex flex-col items-center gap-3 py-10 text-center">
			<Icon icon="solar:danger-triangle-bold" class="mb-1 h-8 w-8" style="color: var(--color-danger);" />
			<p style="color: var(--color-danger);">Failed to load users</p>
			<p class="text-sm" style="color: var(--color-text-muted);">{error}</p>
			<button onclick={loadUsers} class="btn-secondary mt-2">Retry</button>
		</div>
	{:else if users.length === 0}
		<div class="card flex flex-col items-center py-16 text-center">
			<Icon icon="solar:shield-user-bold" class="mb-3 h-12 w-12" style="color: var(--color-text-muted);" />
			<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No users yet</h3>
			<p class="text-sm" style="color: var(--color-text-secondary);">Create the first user to get started.</p>
			<button onclick={openAdd} class="btn-primary mt-4 flex items-center gap-2">
				<Icon icon="solar:user-plus-bold" class="h-4 w-4" />
				Add User
			</button>
		</div>
	{:else}
		<div class="overflow-x-auto -mx-4 md:mx-0">
			<div class="data-table min-w-0 md:min-w-[520px]">
				<table class="w-full">
					<thead>
\t\t\t\t\t\t<tr>
\t\t\t\t\t\t\t<th style="cursor: pointer;" onclick={() => toggleSort('name')}>
\t\t\t\t\t\t\t\t<div class="flex items-center gap-1">
\t\t\t\t\t\t\t\t\tName
\t\t\t\t\t\t\t\t\t<Icon icon={sortIcon('name')} class="h-3 w-3" />
\t\t\t\t\t\t\t\t</div>
\t\t\t\t\t\t\t</th>
\t\t\t\t\t\t\t<th class="hidden sm:table-cell" style="cursor: pointer;" onclick={() => toggleSort('email')}>
\t\t\t\t\t\t\t\t<div class="flex items-center gap-1">
\t\t\t\t\t\t\t\t\tEmail
\t\t\t\t\t\t\t\t\t<Icon icon={sortIcon('email')} class="h-3 w-3" />
\t\t\t\t\t\t\t\t</div>
\t\t\t\t\t\t\t</th>
\t\t\t\t\t\t\t<th class="hidden md:table-cell" style="cursor: pointer;" onclick={() => toggleSort('role')}>
\t\t\t\t\t\t\t\t<div class="flex items-center gap-1">
\t\t\t\t\t\t\t\t\tRole
\t\t\t\t\t\t\t\t\t<Icon icon={sortIcon('role')} class="h-3 w-3" />
\t\t\t\t\t\t\t\t</div>
\t\t\t\t\t\t\t</th>
\t\t\t\t\t\t\t<th class="hidden xl:table-cell">Groups</th>
\t\t\t\t\t\t\t<th class="hidden xl:table-cell">2FA</th>
\t\t\t\t\t\t\t<th class="hidden xl:table-cell" style="cursor: pointer;" onclick={() => toggleSort('status')}>
\t\t\t\t\t\t\t\t<div class="flex items-center gap-1">
\t\t\t\t\t\t\t\t\tStatus
\t\t\t\t\t\t\t\t\t<Icon icon={sortIcon('status')} class="h-3 w-3" />
\t\t\t\t\t\t\t\t</div>
\t\t\t\t\t\t\t</th>
\t\t\t\t\t\t\t<th class="hidden xl:table-cell" style="cursor: pointer;" onclick={() => toggleSort('created_at')}>
\t\t\t\t\t\t\t\t<div class="flex items-center gap-1">
\t\t\t\t\t\t\t\t\tCreated
\t\t\t\t\t\t\t\t\t<Icon icon={sortIcon('created_at')} class="h-3 w-3" />
\t\t\t\t\t\t\t\t</div>
\t\t\t\t\t\t\t</th>
\t\t\t\t\t\t\t<th class="w-20 md:w-24">Actions</th>
\t\t\t\t\t\t</tr>
					</thead>
\t\t\t\t\t<tbody>
\t\t\t\t\t\t{#each sortedUsers as user (user.id)}
							<tr>
								<td class="font-medium max-w-[140px] sm:max-w-none truncate" style="color: var(--color-text);">{user.name}</td>
								<td class="hidden sm:table-cell max-w-[160px] truncate" style="color: var(--color-text-secondary);">{user.email}</td>
								<td class="hidden md:table-cell">
									<span class={roleBadgeClass(user.role)}>
										{user.role}
									</span>
								</td>
							<td class="hidden xl:table-cell">
								{#if user.allowed_groups && user.allowed_groups.length > 0}
									<div class="flex flex-wrap gap-1">
										{#each user.allowed_groups as group}
											<span class="tag-button tag-active" style="font-size: 0.65rem; padding: 1px 6px;">{group}</span>
										{/each}
									</div>
								{:else if user.role === 'admin'}
									<span class="text-xs" style="color: var(--color-text-muted);">All groups</span>
								{:else}
									<span class="text-xs" style="color: var(--color-warning);">No access</span>
								{/if}
							</td>
							<td class="hidden xl:table-cell">
								{#if user.totp_enabled}
									<span class="status-badge online">Enabled</span>
								{:else}
									<span class="status-badge" style="color: var(--color-text-muted);">Disabled</span>
								{/if}
							</td>
							<td class="hidden xl:table-cell">
								{#if user.status === 'locked'}
									<span class="status-badge pending">Locked</span>
								{:else}
									<span class="status-badge online">Active</span>
								{/if}
							</td>
							<td class="hidden text-sm xl:table-cell" style="color: var(--color-text-muted);">
								{new Date(user.created_at).toLocaleDateString()}
							</td>
							<td>
								<div class="flex items-center gap-1">
									{#if user.status === 'locked'}
										<button onclick={() => handleUnlock(user)} class="btn-icon h-8 w-8" title="Unlock" style="color: var(--color-warning);">
											<Icon icon="solar:unlock-bold" class="h-4 w-4" />
										</button>
									{/if}
									<button onclick={() => openEdit(user)} class="btn-icon h-8 w-8" title="Edit">
										<Icon icon="solar:pen-bold" class="h-4 w-4" />
									</button>
									<button onclick={() => openDelete(user)} class="btn-icon h-8 w-8" title="Delete" style="color: var(--color-danger);">
										<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
									</button>
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

<!-- Add User Modal -->
{#if showAddModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onclick={() => showAddModal = false}>
		<div class="w-full max-w-md rounded-xl border p-0 shadow-xl animate-slide-in" style="background-color: var(--color-card); border-color: var(--color-border-light);" onclick={(e) => e.stopPropagation()}>
			<div class="flex items-center justify-between border-b px-6 py-4" style="border-color: var(--color-border-light);">
				<h3 class="text-lg font-semibold" style="color: var(--color-text);">Add User</h3>
				<button onclick={() => showAddModal = false} class="btn-icon">
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>
			<div class="space-y-4 px-6 py-4">
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Name</label>
					<input bind:value={formName} class="input" placeholder="Full name" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Email</label>
					<input bind:value={formEmail} type="email" class="input" placeholder="user@example.com" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Password</label>
					<input bind:value={formPassword} type="password" class="input" placeholder="Min 6 characters" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Role</label>
					<select bind:value={formRole} class="input">
						<option value="admin">Admin</option>
						<option value="developer">Developer</option>
						<option value="viewer">Viewer</option>
					</select>
				</div>
				{#if formRole !== 'admin'}
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Server Groups Access</label>
					<div class="flex flex-wrap gap-2">
						{#each allGroups as group}
							<button
								onclick={() => toggleGroup(group)}
								class="tag-button"
								class:tag-active={formGroups.includes(group)}
							>{group}</button>
						{/each}
						{#if allGroups.length === 0}
							<span class="text-xs" style="color: var(--color-text-muted);">No server groups defined</span>
						{/if}
					</div>
				</div>
				{/if}
			</div>
			<div class="flex justify-end gap-3 border-t px-6 py-4" style="border-color: var(--color-border-light);">
				<button onclick={() => showAddModal = false} class="btn-secondary">Cancel</button>
				<button onclick={handleCreate} disabled={!formEmail || !formName || !formPassword || saving} class="btn-primary">
					{saving ? 'Creating...' : 'Create User'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Edit User Modal -->
{#if showEditModal && editingUser}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onclick={() => showEditModal = false}>
		<div class="w-full max-w-md rounded-xl border p-0 shadow-xl animate-slide-in" style="background-color: var(--color-card); border-color: var(--color-border-light);" onclick={(e) => e.stopPropagation()}>
			<div class="flex items-center justify-between border-b px-6 py-4" style="border-color: var(--color-border-light);">
				<h3 class="text-lg font-semibold" style="color: var(--color-text);">Edit User</h3>
				<button onclick={() => showEditModal = false} class="btn-icon">
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>
			<div class="space-y-4 px-6 py-4">
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Name</label>
					<input bind:value={formName} class="input" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Email</label>
					<input bind:value={formEmail} type="email" class="input" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">New Password</label>
					<input bind:value={formPassword} type="password" class="input" placeholder="Leave empty to keep current" />
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Role</label>
					<select bind:value={formRole} class="input">
						<option value="admin">Admin</option>
						<option value="developer">Developer</option>
						<option value="viewer">Viewer</option>
					</select>
				</div>
				{#if formRole !== 'admin'}
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text-secondary);">Server Groups Access</label>
					<div class="flex flex-wrap gap-2">
						{#each allGroups as group}
							<button
								onclick={() => toggleGroup(group)}
								class="tag-button"
								class:tag-active={formGroups.includes(group)}
							>{group}</button>
						{/each}
						{#if allGroups.length === 0}
							<span class="text-xs" style="color: var(--color-text-muted);">No server groups defined</span>
						{/if}
					</div>
				</div>
				{/if}
			</div>
			<div class="flex justify-end gap-3 border-t px-6 py-4" style="border-color: var(--color-border-light);">
				<button onclick={() => showEditModal = false} class="btn-secondary">Cancel</button>
				<button onclick={handleUpdate} disabled={saving} class="btn-primary">
					{saving ? 'Saving...' : 'Save Changes'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Delete Confirmation Modal -->
{#if showDeleteModal && deletingUser}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onclick={() => showDeleteModal = false}>
		<div class="w-full max-w-sm rounded-xl border p-0 shadow-xl animate-slide-in" style="background-color: var(--color-card); border-color: var(--color-border-light);" onclick={(e) => e.stopPropagation()}>
			<div class="px-6 py-6 text-center">
				<Icon icon="solar:danger-triangle-bold" class="mb-3 inline-block h-12 w-12" style="color: var(--color-danger);" />
				<h3 class="mb-2 text-lg font-semibold" style="color: var(--color-text);">Delete User</h3>
				<p class="text-sm" style="color: var(--color-text-secondary);">
					Are you sure you want to delete <strong style="color: var(--color-text);">{deletingUser.name}</strong> ({deletingUser.email})?
				</p>
				<p class="mt-2 text-xs" style="color: var(--color-warning);">This action cannot be undone.</p>
			</div>
			<div class="flex justify-end gap-3 border-t px-6 py-4" style="border-color: var(--color-border-light);">
				<button onclick={() => showDeleteModal = false} class="btn-secondary">Cancel</button>
				<button onclick={handleDelete} disabled={saving} class="btn-primary flex items-center gap-2" style="background-color: var(--color-danger);">
					{saving ? 'Deleting...' : 'Delete User'}
				</button>
			</div>
		</div>
	</div>
{/if}
