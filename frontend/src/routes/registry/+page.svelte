<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import { user } from '$lib/stores/auth.js';
	import Icon from '@iconify/svelte';
	import { goto } from '$app/navigation';

	// ─── State ───
	let repos = $state([]);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');
	let nextLast = $state('');
	let registryUsers = $state([]);
	let registryConfig = $state(null);
	let configLoading = $state(true);
	let myCredentials = $state(null);
	let credentialsLoading = $state(true);
	let credShowPw = $state(false);
	let resetPwOpen = $state(false);
	let resetPwValue = $state('');
	let resetPwError = $state('');

	let searchQuery = $state('');
	let expandedRepo = $state(null);
	let repoTags = $state({}); // { [repoName]: [...tags] }
	let repoTagsNext = $state({}); // { [repoName]: next_last cursor }
	let tagsLoading = $state({}); // { [repoName]: true|false }

	let pageLoading = $state(false);
	let gcRunning = $state(false);

	// Delete modal
	let deleteTarget = $state(null);

	// User management modals
	let showUserModal = $state(false);
	let userModalMode = $state('add'); // 'add' | 'edit'
	let editUserId = $state('');
	let userForm = $state({ username: '', password: '', role: 'deploy' });
	let userFormError = $state('');
	let userFormLoading = $state(false);
	let createdPassword = $state(''); // shown once after creation

	// Copy states
	let showPassword = $state(false);
let copiedTarget = $state('');

	// ─── Derived ───
	let isAdmin = $derived($user?.role === 'admin');
	let filteredRepos = $derived.by(() => {
		if (!searchQuery) return repos;
		const q = searchQuery.toLowerCase();
		return repos.filter(r => r.name.toLowerCase().includes(q));
	});

	let totalTags = $derived(filteredRepos.reduce((s, r) => s + (r.tags_count || 0), 0));

	// ─── Mount ───
	onMount(() => {
		loadConfig();
		loadCredentials();
		loadRepos();
		loadUsers();
	});

	async function loadConfig() {
		configLoading = true;
		try {
			const data = await api.registry.config();
			registryConfig = data;
		} catch (e) {
			registryConfig = null;
		} finally {
			configLoading = false;
		}
	}

	async function loadCredentials() {
		credentialsLoading = true;
		try {
			const data = await api.registry.myCredentials();
			myCredentials = data;
		} catch (e) {
			myCredentials = null;
		} finally {
			credentialsLoading = false;
		}
	}

	async function generateCredentials() {
		credentialsLoading = true;
		try {
			const data = await api.registry.myCredentials();
			myCredentials = data;
			if (data?.password) {
				credShowPw = true;
			}
		} catch (e) {
			error = e.message || 'Failed to generate credentials';
		} finally {
			credentialsLoading = false;
		}
	}

	function openResetPw() {
		resetPwValue = '';
		resetPwError = '';
		resetPwOpen = true;
	}

	async function submitResetPw() {
		if (resetPwValue.length < 8) {
			resetPwError = 'Password must be at least 8 characters';
			return;
		}
		resetPwError = '';
		try {
			const data = await api.registry.resetMyPassword({ password: resetPwValue });
			myCredentials = { ...myCredentials, password: resetPwValue };
			resetPwOpen = false;
			credShowPw = true;
			error = '';
		} catch (e) {
			resetPwError = e.message || 'Failed to reset password';
		}
	}

	async function loadUsers() {
		try {
			const data = await api.registry.users();
			registryUsers = Array.isArray(data) ? data : [];
		} catch (e) {
			// Non-critical — users section just won't show
		}
	}

	async function loadRepos() {
		loading = true;
		error = '';
		try {
			const data = await api.registry.list({ n: 50 });
			repos = data?.repos || [];
			nextLast = data?.next_last || '';
		} catch (e) {
			error = e.message || 'Failed to load repositories';
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		if (!nextLast || loadingMore) return;
		loadingMore = true;
		try {
			const data = await api.registry.list({ n: 50, last: nextLast });
			if (data?.repos) {
				repos = [...repos, ...data.repos];
			}
			nextLast = data?.next_last || '';
		} catch (e) {
			error = e.message || 'Failed to load more';
		} finally {
			loadingMore = false;
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
				const data = await api.registry.listTags(name, { n: 50 });
				repoTags[name] = data?.tags || [];
				repoTagsNext[name] = data?.next_last || '';
			} catch (e) {
				repoTags[name] = [];
				repoTagsNext[name] = '';
			} finally {
				tagsLoading[name] = false;
			}
		}
	}

	async function loadMoreTags(name) {
		if (!repoTagsNext[name] || tagsLoading[name]) return;
		tagsLoading[name] = true;
		try {
			const data = await api.registry.listTags(name, { n: 50, last: repoTagsNext[name] });
			if (data?.tags) {
				repoTags[name] = [...repoTags[name], ...data.tags];
			}
			repoTagsNext[name] = data?.next_last || '';
		} catch (e) {
			// ignore
		} finally {
			tagsLoading[name] = false;
		}
	}

	async function triggerGC() {
		if (gcRunning) return;
		gcRunning = true;
		try {
			await api.registry.gc();
			error = '';
		} catch (e) {
			error = e.message || 'GC failed';
		} finally {
			gcRunning = false;
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

	// ─── User Management Functions ────────────────────────────────────

	function openAddUser() {
		userModalMode = 'add';
		editUserId = '';
		userForm = { username: '', password: '', role: 'deploy' };
		userFormError = '';
		createdPassword = '';
		showUserModal = true;
	}

	function openEditUser(user) {
		userModalMode = 'edit';
		editUserId = user.id;
		userForm = { username: user.username, password: '', role: user.role };
		userFormError = '';
		createdPassword = '';
		showUserModal = true;
	}

	function closeUserModal() {
		showUserModal = false;
		userFormError = '';
		createdPassword = '';
	}

	async function submitUserForm() {
		userFormError = '';
		userFormLoading = true;
		try {
			if (userModalMode === 'add') {
				if (!userForm.username || !userForm.password) {
					userFormError = 'Username and password are required';
					return;
				}
				if (userForm.password.length < 8) {
					userFormError = 'Password must be at least 8 characters';
					return;
				}
				const result = await api.registry.createUser({
					username: userForm.username,
					password: userForm.password,
					role: userForm.role,
				});
				createdPassword = result.password || userForm.password;
				userForm = { username: '', password: '', role: 'deploy' };
				await loadUsers();
			} else {
				// Edit mode
				const payload = {};
				if (userForm.username) payload.username = userForm.username;
				if (userForm.role) payload.role = userForm.role;
				if (userForm.password) payload.password = userForm.password;
				await api.registry.updateUser(editUserId, payload);
				await loadUsers();
				closeUserModal();
			}
		} catch (e) {
			userFormError = e.message || 'Operation failed';
		} finally {
			userFormLoading = false;
		}
	}

	async function deleteRegistryUser(userId) {
		if (!confirm('Delete this registry user? This action cannot be undone.')) return;
		try {
			await api.registry.deleteUser(userId);
			await loadUsers();
		} catch (e) {
			error = e.message || 'Failed to delete user';
		}
	}

	async function resetPassword(userId) {
		const newPw = prompt('Enter new password (min 8 characters):');
		if (!newPw || newPw.length < 8) {
			error = 'Password must be at least 8 characters';
			return;
		}
		try {
			await api.registry.resetPassword(userId, { password: newPw });
			error = '';
			alert('Password reset successfully');
		} catch (e) {
			error = e.message || 'Failed to reset password';
		}
	}

	function roleBadgeStyle(role) {
		if (role === 'admin') return 'background-color: rgba(239,68,68,0.15); color: var(--color-danger);';
		if (role === 'deploy') return 'background-color: rgba(16,185,129,0.15); color: var(--color-success);';
		return 'background-color: rgba(100,116,139,0.15); color: var(--color-text-muted);';
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

	async function copyToClipboard(text, target) {
		copiedTarget = target; // Show feedback immediately (optimistic)
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
			// Clipboard write failed but feedback is already shown
		}
		setTimeout(() => {
			if (copiedTarget === target) copiedTarget = '';
		}, 2000);
	}
</script>

<div class="page-container">
	<!-- Connection Info — Self-Service Credentials -->
	<div class="card p-5">
		<div class="flex items-start justify-between mb-4">
			<div>
				<div class="flex items-center gap-2 mb-0.5">
					<Icon icon="solar:key-minimalistic-bold" class="h-4 w-4" style="color: var(--color-primary);" />
					<h3 class="text-sm font-semibold" style="color: var(--color-text);">Registry Connection</h3>
				</div>
				<p class="text-xs" style="color: var(--color-text-secondary);">Your personal credentials for Docker CLI and CI/CD pipelines.</p>
			</div>
		</div>

		{#if credentialsLoading}
			<div class="flex items-center gap-2 py-2">
				<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" style="color: var(--color-primary);" />
				<span class="text-xs" style="color: var(--color-text-muted);">Loading your credentials...</span>
			</div>
		{:else if myCredentials}
			<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
				<div class="rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
					<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Registry URL</label>
					<div class="mt-1 flex items-center gap-2">
						<code class="font-mono text-xs" style="color: var(--color-text);">{myCredentials.url}</code>
						<button class="flex-shrink-0" onclick={() => copyToClipboard(myCredentials.url, 'reg-url')}>
							<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
						</button>
						{#if copiedTarget === 'reg-url'}
							<span class="text-[10px]" style="color: var(--color-success);">✓ Copied</span>
						{/if}
					</div>
				</div>
				<div class="rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
					<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Username</label>
					<div class="mt-1 flex items-center gap-2">
						<code class="font-mono text-xs" style="color: var(--color-text);">{myCredentials.username}</code>
						<button class="flex-shrink-0" onclick={() => copyToClipboard(myCredentials.username, 'reg-user')}>
							<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
						</button>
						{#if copiedTarget === 'reg-user'}
							<span class="text-[10px]" style="color: var(--color-success);">✓ Copied</span>
						{/if}
					</div>
				</div>
				<div class="rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
					<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Password</label>
					<div class="mt-1 flex items-center gap-2">
						{#if myCredentials.password}
							<code class="font-mono text-xs" style="color: var(--color-text);">{credShowPw ? myCredentials.password : '••••••••••••••••'}</code>
							<button class="flex-shrink-0" onclick={() => { credShowPw = !credShowPw; }}>
								<Icon icon={credShowPw ? 'solar:eye-closed-outline' : 'solar:eye-outline'} class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
							</button>
							{#if credShowPw}
								<button class="flex-shrink-0" onclick={() => copyToClipboard(myCredentials.password, 'reg-pw')}>
									<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
								</button>
							{/if}
							{#if copiedTarget === 'reg-pw'}
								<span class="text-[10px]" style="color: var(--color-success);">✓ Copied</span>
							{/if}
						{:else}
							<code class="font-mono text-xs" style="color: var(--color-text-muted);">••••••••</code>
							<span class="text-[10px]" style="color: var(--color-text-muted);">Hidden</span>
						{/if}
					</div>
				</div>
			</div>
			<div class="mt-3 rounded-lg border p-3" style="background-color: var(--color-card); border-color: var(--color-border-light);">
				<div class="flex items-center justify-between gap-2 flex-wrap">
					<div class="flex items-center gap-2 min-w-0 flex-1">
						<Icon icon="solar:code-outline" class="h-4 w-4 flex-shrink-0" style="color: var(--color-primary);" />
						<code class="font-mono text-xs break-all" style="color: var(--color-text-secondary);">docker login {myCredentials.url} -u {myCredentials.username}</code>
					</div>
					<div class="flex items-center gap-2 flex-shrink-0">
						<button
							class="inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 text-[10px] font-medium transition-colors"
							style="color: var(--color-text-muted); border: 1px solid var(--color-border);"
							onclick={() => copyToClipboard('docker login ' + myCredentials.url + ' -u ' + myCredentials.username, 'login')}
						>
							<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" />
							Copy Command
						</button>
						{#if myCredentials.password}
							<button
								class="inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 text-[10px] font-medium transition-colors"
								style="color: var(--color-warning); border: 1px solid var(--color-border);"
								onclick={openResetPw}
							>
								<Icon icon="solar:key-minimalistic-outline" class="h-3.5 w-3.5" />
								Reset Password
							</button>
						{:else}
							<button
								class="inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 text-[10px] font-medium transition-colors"
								style="background-color: var(--color-primary); color: #fff;"
								onclick={openResetPw}
							>
								<Icon icon="solar:key-minimalistic-bold" class="h-3.5 w-3.5" />
								Set Password
							</button>
						{/if}
					</div>
				</div>
				{#if copiedTarget === 'login'}
					<span class="mt-1 inline-block text-[10px]" style="color: var(--color-success);">✓ Login command copied</span>
				{/if}
			</div>
			{#if myCredentials.password}
				<div class="mt-2 rounded-lg border p-2" style="background-color: rgba(245,158,11,0.08); border-color: rgba(245,158,11,0.2);">
					<p class="text-[10px]" style="color: var(--color-warning);">
						<Icon icon="solar:info-circle-outline" class="inline h-3 w-3" />
						Save your password now — it won't be shown again after this session.
					</p>
				</div>
			{/if}
		{:else}
			<div class="rounded-lg border p-4 text-center" style="border-color: var(--color-border);">
				<p class="text-xs" style="color: var(--color-text-muted);">Could not load registry credentials.</p>
			</div>
		{/if}

		<!-- Reset Password Modal -->
		{#if resetPwOpen}
			<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onclick={() => resetPwOpen = false}>
				<div class="max-w-sm w-full mx-4 rounded-xl p-5" style="background-color: var(--color-card);" onclick={(e) => e.stopPropagation()}>
					<h4 class="text-sm font-semibold mb-1" style="color: var(--color-text);">Reset Registry Password</h4>
					<p class="text-xs mb-4" style="color: var(--color-text-secondary);">Enter a new password for <strong>{myCredentials?.username || 'your registry user'}</strong>.</p>
					<input
						type="text"
						placeholder="New password (min 8 characters)"
						bind:value={resetPwValue}
						class="w-full rounded-lg border px-3 py-2 text-xs mb-3"
						style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
					/>
					{#if resetPwError}
						<p class="text-xs mb-3" style="color: var(--color-danger);">{resetPwError}</p>
					{/if}
					<div class="flex items-center justify-end gap-2">
						<button
							class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
							style="color: var(--color-text-muted);"
							onclick={() => resetPwOpen = false}
						>Cancel</button>
						<button
							class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
							style="background-color: var(--color-primary); color: #fff;"
							onclick={submitResetPw}
						>Reset Password</button>
					</div>
				</div>
			</div>
		{/if}
	</div>

	<!-- Registry Users -->
	{#if isAdmin}
	<div class="card p-5">
		<div class="flex items-center justify-between mb-3">
			<div class="flex items-center gap-2">
				<Icon icon="solar:users-group-rounded-bold" class="h-4 w-4" style="color: var(--color-primary);" />
				<h3 class="text-sm font-semibold" style="color: var(--color-text);">Registry Users</h3>
				<span class="rounded-full px-2 py-0.5 text-[10px] font-medium" style="background-color: var(--color-primary-subtle); color: var(--color-primary);">{registryUsers.length}</span>
			</div>
			<button
				class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
				style="background-color: var(--color-primary); color: #fff;"
				onclick={openAddUser}
			>
				<Icon icon="solar:add-circle-bold" class="h-3.5 w-3.5" />
				Add User
			</button>
		</div>
		{#if registryUsers.length > 0}
			<div class="space-y-2">
				{#each registryUsers as user}
					<div class="flex items-center justify-between rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2">
								<code class="font-mono text-xs font-medium" style="color: var(--color-text);">{user.username}</code>
								<span class="rounded px-1.5 py-0.5 text-[9px] font-medium uppercase" style={roleBadgeStyle(user.role)}>{user.role}</span>
							</div>
							<p class="mt-0.5 text-[10px]" style="color: var(--color-text-muted);">{user.access}</p>
						</div>
						<div class="flex items-center gap-1">
							<button
								class="rounded-md p-1.5 transition-colors"
								style="color: var(--color-text-muted);"
								onclick={() => openEditUser(user)}
								title="Edit user"
							>
								<Icon icon="solar:pen-outline" class="h-3.5 w-3.5" />
							</button>
							<button
								class="rounded-md p-1.5 transition-colors"
								style="color: var(--color-text-muted);"
								onclick={() => resetPassword(user.id)}
								title="Reset password"
							>
								<Icon icon="solar:key-minimalistic-outline" class="h-3.5 w-3.5" />
							</button>
							<button
								class="rounded-md p-1.5 transition-colors"
								style="color: var(--color-danger);"
								onclick={() => deleteRegistryUser(user.id)}
								title="Delete user"
							>
								<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
							</button>
						</div>
					</div>
				{/each}
			</div>
		{:else}
			<div class="rounded-lg border p-4 text-center" style="border-color: var(--color-border);">
				<p class="text-xs" style="color: var(--color-text-muted);">No registry users configured. Add a user to enable Docker login.</p>
			</div>
		{/if}
	</div>
	{/if}

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
			{#if isAdmin}
			<span class="h-3 w-px" style="background-color: var(--color-border);"></span>
			<button
				class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-medium transition-colors"
				style="color: var(--color-text-muted); border: 1px solid var(--color-border);"
				onclick={triggerGC}
				disabled={gcRunning}
			>
				<Icon icon={gcRunning ? 'solar:spinner-bold' : 'solar:refresh-bold'} class="h-3 w-3 {gcRunning ? 'animate-spin' : ''}" />
				GC
			</button>
			{/if}
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

	<!-- Copy tooltip: inline per button -->

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
												onclick={() => copyToClipboard(`docker pull registry.anjungan.io/${repo.name}:${tag.name}`, `pull-${repo.name}-${tag.name}`)}
												title="Copy pull command"
											>
												<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" />
											</button>
											{#if copiedTarget === `pull-${repo.name}-${tag.name}`}
												<span class="text-[10px]" style="color: var(--color-success);">✓</span>
											{/if}
									{#if isAdmin}
											<button
												class="rounded-md p-1.5 transition-colors hover:opacity-80"
												style="color: var(--color-text-muted);"
												onclick={() => handleDelete(repo.name, tag.name, tag.digest)}
												title="Delete tag"
											>
												<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
											</button>
										{/if}
										</div>
									</div>
							{/each}
							{#if repoTagsNext[repo.name]}
								<div class="flex justify-center border-t py-3" style="border-color: var(--color-border);">
									<button
										class="text-xs font-medium transition-colors hover:opacity-80"
										style="color: var(--color-primary);"
										onclick={() => loadMoreTags(repo.name)}
									>
										{#if tagsLoading[repo.name]}
											Loading...
										{:else}
											Load More Tags
										{/if}
									</button>
								</div>
							{/if}
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

		<!-- Load More Repos -->
		{#if nextLast}
			<div class="mt-4 flex justify-center">
				<button
					class="inline-flex items-center gap-1.5 rounded-lg border px-4 py-2 text-xs font-medium transition-colors"
					style="border-color: var(--color-border); color: var(--color-text-secondary);"
					onclick={loadMore}
					disabled={loadingMore}
				>
					{#if loadingMore}
						<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
						Loading...
					{:else}
						Load More
					{/if}
				</button>
			</div>
		{/if}
	{/if}
</div>

<!-- User Modal (Add/Edit) -->
{#if showUserModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center"
		style="background-color: rgba(0,0,0,0.6);"
		onclick={closeUserModal}
	>
		<div
			class="mx-4 w-full max-w-md rounded-xl border shadow-2xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="p-5">
				<div class="flex items-center gap-2 mb-4">
					<Icon icon={userModalMode === 'add' ? 'solar:add-circle-bold' : 'solar:pen-bold'} class="h-5 w-5" style="color: var(--color-primary);" />
					<h3 class="text-sm font-semibold" style="color: var(--color-text);">
						{userModalMode === 'add' ? 'Add Registry User' : 'Edit Registry User'}
					</h3>
				</div>

				<!-- Created password display -->
				{#if createdPassword}
					<div class="mb-4 rounded-lg border p-3" style="background-color: rgba(16,185,129,0.08); border-color: rgba(16,185,129,0.2);">
						<div class="flex items-start gap-2">
							<Icon icon="solar:check-circle-bold" class="mt-0.5 h-4 w-4 flex-shrink-0" style="color: var(--color-success);" />
							<div class="min-w-0 flex-1">
								<p class="text-xs font-medium" style="color: var(--color-success);">User created successfully!</p>
								<p class="mt-1 text-[10px]" style="color: var(--color-text-muted);">Save this password — it won't be shown again.</p>
								<div class="mt-2 flex items-center gap-2 rounded-md p-2" style="background-color: var(--color-card);">
									<code class="flex-1 font-mono text-xs" style="color: var(--color-text);">{createdPassword}</code>
									<button class="flex-shrink-0" onclick={() => copyToClipboard(createdPassword, 'created-pw')}>
										<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
									</button>
								</div>
								<button
									class="mt-2 inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-medium transition-colors"
									style="color: var(--color-success);"
									onclick={closeUserModal}
								>
									Close
								</button>
							</div>
						</div>
					</div>
				{/if}

				<!-- Form -->
				{#if !createdPassword}
					<div class="space-y-3">
						<div>
							<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Username</label>
							<input
								type="text"
								bind:value={userForm.username}
								class="w-full rounded-lg border px-3 py-2 text-xs"
								style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
								placeholder="e.g. jenkins-ci"
							/>
						</div>
						<div>
							<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Password {userModalMode === 'edit' ? '(leave empty to keep current)' : ''}</label>
							<input
								type="password"
								bind:value={userForm.password}
								class="w-full rounded-lg border px-3 py-2 text-xs"
								style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
								placeholder={userModalMode === 'edit' ? 'New password (optional)' : 'Min 8 characters'}
							/>
						</div>
						<div>
							<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Role</label>
							<select
								bind:value={userForm.role}
								class="w-full rounded-lg border px-3 py-2 text-xs"
								style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
							>
								<option value="deploy">Deploy — Read & push</option>
								<option value="admin">Admin — Full access</option>
								<option value="readonly">Read-only — Pull only</option>
							</select>
						</div>

						{#if userFormError}
							<div class="rounded-md p-2 text-xs" style="background-color: rgba(239,68,68,0.08); color: var(--color-danger);">
								{userFormError}
							</div>
						{/if}
					</div>
				{/if}
			</div>

			{#if !createdPassword}
				<div class="flex items-center justify-end gap-2 rounded-b-xl border-t px-5 py-3" style="border-color: var(--color-border); background-color: var(--color-topbar-bg);">
					<button
						class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
						style="color: var(--color-text-secondary);"
						onclick={closeUserModal}
					>Cancel</button>
					<button
						class="inline-flex items-center gap-1.5 rounded-lg px-4 py-1.5 text-xs font-medium text-white transition-colors"
						style="background-color: var(--color-primary);"
						onclick={submitUserForm}
						disabled={userFormLoading}
					>
						{#if userFormLoading}
							<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
							Saving...
						{:else}
							{userModalMode === 'add' ? 'Create User' : 'Save Changes'}
						{/if}
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}

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
