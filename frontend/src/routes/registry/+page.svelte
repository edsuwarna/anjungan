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

	let pageLoading = $state(false);
	let gcRunning = $state(false);

	// Delete modal
	let deleteTarget = $state(null);
	let protectedDeleteTarget = $state(null);

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

	// ─── Webhook State ──────────────────────────────────────────
	let webhooks = $state([]);
	let webhooksLoading = $state(false);
	let showWebhookModal = $state(false);
	let webhookModalMode = $state('add'); // 'add' | 'edit'
	let editWebhookId = $state('');
	let webhookForm = $state({ name: '', url: '', platform: 'generic', events: ['push', 'pull', 'delete'], enabled: true });
	let webhookFormError = $state('');
	let webhookFormLoading = $state(false);
	let webhookTestResult = $state(null); // { id, status, status_code, error }
	let webhookTestingId = $state(null);
	// Event log
	let webhookEvents = $state([]);
	let webhookEventsTotal = $state(0);
	let webhookEventsLoading = $state(false);
	let showWebhookEvents = $state(false);

	// ─── Tag Protection State ──────────────────────────────────
	let tagProtections = $state([]); // { repo, tag, id }
	let tagProtectionsSet = $derived(new Set(tagProtections.map(p => `${p.repo}:${p.tag}`)));

	// ─── Tag Search State ──────────────────────────────────────
	let searchMode = $state('repo'); // 'repo' | 'tag'
	let tagSearchQuery = $state('');
	let tagSearchResults = $state([]);
	let tagSearchLoading = $state(false);
	let tagSearchTotal = $state(0);
	let tagSearchDebounce = $state(null);

	// ─── CVE State ─────────────────────────────────────────────
	let cveAvailable = $state(false);
	let cveChecking = $state(false);

	// ─── Health State ───────────────────────────────────────────
	let registryHealth = $state(null);
	let healthLoading = $state(true);

	// ─── Activity State ─────────────────────────────────────────
	let showAllEvents = $state(false);

	// ─── Tab State ──────────────────────────────────────────
	let activeTab = $state('repos');
	const tabs = [
		{ id: 'repos', label: 'Repos', icon: 'solar:box-bold', adminOnly: false },
		{ id: 'credentials', label: 'Credentials', icon: 'solar:key-minimalistic-bold', adminOnly: false },
		{ id: 'activity', label: 'Activity', icon: 'solar:clock-circle-bold', adminOnly: false },
		{ id: 'admin', label: 'Admin', icon: 'solar:tuning-2-bold', adminOnly: true },
	];
	let filteredTabs = $derived(tabs.filter(t => !t.adminOnly || isAdmin));

	// ─── Stats State ────────────────────────────────────────────
	let statsSummary = $state(null);
	let statsLoading = $state(false);
	let showStats = $state(false);

	// ─── Cleanup State ──────────────────────────────────────────
	let cleanupConfig = $state(null);
	let cleanupModalOpen = $state(false);
	let cleanupRunning = $state(false);
	let cleanupResult = $state(null);

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
		loadHealth();
		loadRepos();
		loadStatsSummary();
		loadUsers();
		loadWebhooks();
		loadWebhookEvents();
		loadProtections();
		loadCveStatus();
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
		if (isTagProtected(repo, tag)) {
			protectedDeleteTarget = { repo, tag };
		} else {
			deleteTarget = { repo, tag, digest };
		}
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

	// ─── Webhook Functions ──────────────────────────────────────

	async function loadWebhooks() {
		webhooksLoading = true;
		try {
			const data = await api.registry.webhooks.list();
			webhooks = Array.isArray(data) ? data : [];
		} catch (e) {
			// ignore
		} finally {
			webhooksLoading = false;
		}
	}

	async function loadWebhookEvents() {
		webhookEventsLoading = true;
		try {
			const data = await api.registry.webhooks.events({ limit: 30 });
			webhookEvents = data?.events || [];
			webhookEventsTotal = data?.total || 0;
		} catch (e) {
			// ignore
		} finally {
			webhookEventsLoading = false;
		}
	}

	function openAddWebhook() {
		webhookModalMode = 'add';
		editWebhookId = '';
		webhookForm = { name: '', url: '', platform: 'generic', events: ['push', 'pull', 'delete'], enabled: true };
		webhookFormError = '';
		webhookTestResult = null;
		showWebhookModal = true;
	}

	function openEditWebhook(hook) {
		webhookModalMode = 'edit';
		editWebhookId = hook.id;
		webhookForm = {
			name: hook.name,
			url: hook.url,
			platform: hook.platform,
			events: Array.isArray(hook.events) ? hook.events : JSON.parse(hook.events || '["push","pull","delete"]'),
			enabled: hook.enabled
		};
		webhookFormError = '';
		webhookTestResult = null;
		showWebhookModal = true;
	}

	function closeWebhookModal() {
		showWebhookModal = false;
		webhookTestResult = null;
	}

	async function submitWebhookForm() {
		webhookFormError = '';
		if (!webhookForm.url) {
			webhookFormError = 'URL is required';
			return;
		}
		webhookFormLoading = true;
		try {
			const payload = {
				name: webhookForm.name,
				url: webhookForm.url,
				platform: webhookForm.platform,
				events: webhookForm.events,
			};
			if (webhookModalMode === 'add') {
				payload.enabled = webhookForm.enabled;
				await api.registry.webhooks.create(payload);
			} else {
				await api.registry.webhooks.update(editWebhookId, payload);
			}
			await loadWebhooks();
			closeWebhookModal();
		} catch (e) {
			webhookFormError = e.message || 'Operation failed';
		} finally {
			webhookFormLoading = false;
		}
	}

	async function deleteWebhook(id) {
		if (!confirm('Delete this webhook? This cannot be undone.')) return;
		try {
			await api.registry.webhooks.delete(id);
			await loadWebhooks();
		} catch (e) {
			error = e.message || 'Failed to delete webhook';
		}
	}

	async function testWebhook(id) {
		webhookTestingId = id;
		webhookTestResult = null;
		try {
			const result = await api.registry.webhooks.test(id);
			webhookTestResult = { id, ...result };
		} catch (e) {
			webhookTestResult = { id, status: 'failed', error: e.message };
		} finally {
			webhookTestingId = null;
		}
	}

	function toggleWebhookEvents() {
		showWebhookEvents = !showWebhookEvents;
		if (showWebhookEvents && webhookEvents.length === 0) {
			loadWebhookEvents();
		}
	}

	function webhookPlatformIcon(platform) {
		const icons = { telegram: 'solar:telegram-bold', discord: 'solar:discord-bold', slack: 'solar:slack-bold', generic: 'solar:link-bold' };
		return icons[platform] || 'solar:link-bold';
	}

	function webhookEventStatusIcon(status) {
		if (status === 'delivered') return 'solar:check-circle-bold';
		if (status === 'failed') return 'solar:danger-triangle-bold';
		return 'solar:clock-circle-bold';
	}

	function webhookEventStatusColor(status) {
		if (status === 'delivered') return 'var(--color-success)';
		if (status === 'failed') return 'var(--color-danger)';
		return 'var(--color-warning)';
	}

	function webhookEventTypeIcon(type) {
		switch (type) {
			case 'push': return '📦';
			case 'delete': return '🗑';
			case 'test': return '🧪';
			default: return '🔔';
		}
	}

	function webhookEventBadgeStyle(type) {
		if (type === 'push') return 'background-color: rgba(16,185,129,0.12); color: #10b981;';
		if (type === 'delete') return 'background-color: rgba(239,68,68,0.12); color: #ef4444;';
		if (type === 'pull') return 'background-color: rgba(59,130,246,0.12); color: #3b82f6;';
		return 'background-color: rgba(100,116,139,0.12); color: var(--color-text-muted);';
	}

	// ─── Tag Protection Functions ───────────────────────────────

	async function loadProtections() {
		try {
			const data = await api.registry.protections.list();
			tagProtections = Array.isArray(data) ? data : [];
		} catch (e) {
			// ignore
		}
	}

	// ─── CVE Functions ─────────────────────────────────────────

	async function loadCveStatus() {
		cveChecking = true;
		try {
			const data = await api.registry.cve.check();
			cveAvailable = data?.available === true;
		} catch (e) {
			cveAvailable = false;
		} finally {
			cveChecking = false;
		}
	}

	async function loadHealth() {
		healthLoading = true;
		try {
			const data = await api.registry.health();
			registryHealth = data;
		} catch (e) {
			registryHealth = { status: 'down', message: e.message };
		} finally {
			healthLoading = false;
		}
	}

	// ─── Stats Functions ────────────────────────────────────────

	async function loadStatsSummary() {
		statsLoading = true;
		try {
			const data = await api.registry.stats.summary();
			statsSummary = data;
		} catch (e) {
			statsSummary = null;
		} finally {
			statsLoading = false;
		}
	}

	function toggleStats() {
		showStats = !showStats;
	}

	function formatBytes(bytes) {
		if (!bytes || bytes === 0) return '0 B';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let i = 0;
		let size = bytes;
		while (size >= 1024 && i < units.length - 1) {
			size /= 1024;
			i++;
		}
		return size.toFixed(i === 0 ? 0 : 1) + ' ' + units[i];
	}

	function storageBarWidth(size, maxSize) {
		if (!maxSize) return 0;
		return Math.max(2, (size / maxSize) * 100);
	}

	// ─── Cleanup Functions ──────────────────────────────────────

	async function loadCleanupConfig() {
		try {
			const data = await api.registry.cleanup.config();
			cleanupConfig = data;
		} catch (e) {
			cleanupConfig = null;
		}
	}

	function openCleanupModal() {
		cleanupResult = null;
		loadCleanupConfig().then(() => { cleanupModalOpen = true; });
	}

	function closeCleanupModal() {
		cleanupModalOpen = false;
	}

	async function saveCleanupConfig() {
		if (!cleanupConfig) return;
		try {
			const data = await api.registry.cleanup.updateConfig(cleanupConfig);
			cleanupConfig = data;
		} catch (e) {
			error = e.message || 'Failed to save cleanup config';
		}
	}

	async function runCleanup() {
		cleanupRunning = true;
		cleanupResult = null;
		try {
			const data = await api.registry.cleanup.run();
			cleanupResult = data;
		} catch (e) {
			cleanupResult = { error: e.message || 'Cleanup failed' };
		} finally {
			cleanupRunning = false;
		}
	}

	async function protectTag(repo, tag) {
		try {
			await api.registry.protections.create({ repo, tag });
			await loadProtections();
		} catch (e) {
			error = e.message || 'Failed to protect tag';
		}
	}

	async function unprotectTag(repo, tag) {
		try {
			await api.registry.protections.deleteByRepoTag(repo, tag);
			await loadProtections();
		} catch (e) {
			error = e.message || 'Failed to unprotect tag';
		}
	}

	// ─── Delete Repo ──────────────────────────────────────────────
	let deleteRepoTarget = $state(null);
	let deleteRepoLoading = $state(false);
	let deleteRepoResult = $state(null);

	function confirmDeleteRepo(name) {
		deleteRepoTarget = name;
	}

	async function executeDeleteRepo() {
		if (!deleteRepoTarget) return;
		deleteRepoLoading = true;
		deleteRepoResult = null;
		try {
			const data = await api.registry.deleteRepo(deleteRepoTarget);
			deleteRepoResult = data;
			await loadRepos();
			deleteRepoTarget = null;
		} catch (e) {
			deleteRepoResult = { error: e.message || 'Delete failed' };
		} finally {
			deleteRepoLoading = false;
		}
	}

	function isTagProtected(repo, tag) {
		return tagProtectionsSet.has(`${repo}:${tag}`);
	}

	// ─── Tag Search Functions ──────────────────────────────────

	async function doTagSearch() {
		if (!tagSearchQuery.trim()) {
			tagSearchResults = [];
			tagSearchTotal = 0;
			return;
		}
		tagSearchLoading = true;
		try {
			const data = await api.registry.searchTags(tagSearchQuery.trim());
			tagSearchResults = data?.results || [];
			tagSearchTotal = data?.total || 0;
		} catch (e) {
			tagSearchResults = [];
			tagSearchTotal = 0;
		} finally {
			tagSearchLoading = false;
		}
	}

	function onTagSearchInput() {
		if (tagSearchDebounce) clearTimeout(tagSearchDebounce);
		tagSearchDebounce = setTimeout(doTagSearch, 300);
	}

	function switchSearchMode(mode) {
		searchMode = mode;
		if (mode === 'tag' && tagSearchQuery && tagSearchResults.length === 0) {
			doTagSearch();
		}
	}
</script>

<div class="page-container">
	<!-- Tab Bar -->
	<div class="flex items-center gap-1 mb-4 rounded-lg p-1 overflow-x-auto" style="background-color: var(--color-topbar-bg);">
		{#each filteredTabs as tab}
			<button
				class="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors whitespace-nowrap"
				style="color: {activeTab === tab.id ? 'var(--color-text)' : 'var(--color-text-muted)'}; background-color: {activeTab === tab.id ? 'var(--color-card)' : 'transparent'};"
				onclick={() => activeTab = tab.id}
			>
				<Icon icon={tab.icon} class="h-3.5 w-3.5" />
				{tab.label}
			</button>
		{/each}
	</div>

	<!-- ─── Tab: Credentials ─── -->
	{#if activeTab === 'credentials'}
	<!-- Connection Info — Self-Service Credentials -->
	<div class="card p-5">
		<div class="flex items-start justify-between mb-4">
			<div>
				<div class="flex items-center gap-2 mb-0.5">
					<Icon icon="solar:key-minimalistic-bold" class="h-4 w-4" style="color: var(--color-primary);" />
					<h3 class="text-sm font-semibold" style="color: var(--color-text);">Registry Connection</h3>
					{#if healthLoading}
						<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px]" style="background-color: rgba(148,163,184,0.1); color: var(--color-text-muted);">
							<Icon icon="solar:spinner-bold" class="h-2.5 w-2.5 animate-spin" />
							Check...
						</span>
					{:else if registryHealth?.status === 'up'}
						<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px]" style="background-color: rgba(16,185,129,0.1); color: #10b981;">
							<Icon icon="solar:shield-check-bold" class="h-2.5 w-2.5" />
							Online
						</span>
					{:else}
						<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px]" style="background-color: rgba(239,68,68,0.1); color: #ef4444;" title={registryHealth?.message}>
							<Icon icon="solar:shield-warning-bold" class="h-2.5 w-2.5" />
							Offline
						</span>
					{/if}
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
{/if}

	<!-- ─── Tab: Admin ─── -->
	{#if activeTab === 'admin'}
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

	<!-- Registry Webhooks -->
	{#if isAdmin}
	<div class="card p-5">
		<div class="flex items-center justify-between mb-3">
			<div class="flex items-center gap-2">
				<Icon icon="solar:bell-bing-bold" class="h-4 w-4" style="color: var(--color-primary);" />
				<h3 class="text-sm font-semibold" style="color: var(--color-text);">Webhook Notifications</h3>
				<span class="rounded-full px-2 py-0.5 text-[10px] font-medium" style="background-color: var(--color-primary-subtle); color: var(--color-primary);">{webhooks.length}</span>
			</div>
			<div class="flex items-center gap-2">
				<button
					class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
					style="color: var(--color-text-secondary); border: 1px solid var(--color-border);"
					onclick={toggleWebhookEvents}
				>
					<Icon icon="solar:history-bold" class="h-3.5 w-3.5" />
					Events
				</button>
				<button
					class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
					style="background-color: var(--color-primary); color: #fff;"
					onclick={openAddWebhook}
				>
					<Icon icon="solar:add-circle-bold" class="h-3.5 w-3.5" />
					Add Webhook
				</button>
			</div>
		</div>

		{#if webhooks.length > 0}
			<div class="space-y-2">
				{#each webhooks as hook}
					{@const evts = Array.isArray(hook.events) ? hook.events : JSON.parse(hook.events || '[]')}
					<div class="flex items-center justify-between rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
						<div class="flex items-center gap-3 min-w-0 flex-1">
							<Icon icon={webhookPlatformIcon(hook.platform)} class="h-4 w-4 flex-shrink-0" style="color: var(--color-primary);" />
							<div class="min-w-0 flex-1">
								<div class="flex items-center gap-2">
									<span class="text-xs font-medium" style="color: var(--color-text);">{hook.name || hook.platform}</span>
									<span class="rounded px-1.5 py-0.5 text-[9px] font-medium uppercase" style="background-color: rgba(100,116,139,0.15); color: var(--color-text-muted);">{hook.platform}</span>
									{#if !hook.enabled}
										<span class="rounded px-1.5 py-0.5 text-[9px] font-medium uppercase" style="background-color: rgba(245,158,11,0.15); color: var(--color-warning);">Paused</span>
									{/if}
								</div>
								<p class="mt-0.5 truncate text-[10px]" style="color: var(--color-text-muted);">{hook.url}</p>
								<div class="mt-1 flex items-center gap-1.5">
									{#each evts as ev}
										<span class="rounded px-1.5 py-0.5 text-[9px]" style="background-color: rgba(59,130,246,0.1); color: var(--color-info, #3b82f6);">{ev}</span>
									{/each}
								</div>
							</div>
						</div>
						<div class="flex items-center gap-1 flex-shrink-0 ml-2">
							<button
								class="rounded-md p-1.5 transition-colors"
								style="color: var(--color-text-muted);"
								onclick={() => testWebhook(hook.id)}
								disabled={webhookTestingId === hook.id}
								title="Test webhook"
							>
								{#if webhookTestingId === hook.id}
									<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
								{:else}
									<Icon icon="solar:play-stream-bold" class="h-3.5 w-3.5" />
								{/if}
							</button>
							<button
								class="rounded-md p-1.5 transition-colors"
								style="color: var(--color-text-muted);"
								onclick={() => openEditWebhook(hook)}
								title="Edit webhook"
							>
								<Icon icon="solar:pen-outline" class="h-3.5 w-3.5" />
							</button>
							<button
								class="rounded-md p-1.5 transition-colors"
								style="color: var(--color-danger);"
								onclick={() => deleteWebhook(hook.id)}
								title="Delete webhook"
							>
								<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
							</button>
						</div>
					</div>

					<!-- Test result inline -->
					{#if webhookTestResult && webhookTestResult.id === hook.id}
						<div class="mt-1 rounded-lg p-2 text-xs" style="background-color: {webhookTestResult.status === 'delivered' ? 'rgba(16,185,129,0.08)' : 'rgba(239,68,68,0.08)'};">
							<span style="color: {webhookTestResult.status === 'delivered' ? 'var(--color-success)' : 'var(--color-danger)'};">
								{#if webhookTestResult.status === 'delivered'}
									✓ Delivered (HTTP {webhookTestResult.status_code})
								{:else}
									✗ Failed — {webhookTestResult.error || 'HTTP ' + webhookTestResult.status_code}
								{/if}
							</span>
						</div>
					{/if}
				{/each}
			</div>
		{:else if !webhooksLoading}
			<div class="rounded-lg border p-4 text-center" style="border-color: var(--color-border);">
				<p class="text-xs" style="color: var(--color-text-muted);">No webhooks configured. Add a webhook to get notified of registry events.</p>
				<p class="mt-1 text-[10px]" style="color: var(--color-text-muted);">Supports Telegram, Discord, Slack, and generic webhook URLs.</p>
			</div>
		{/if}

		<!-- Webhook Events Log -->
		{#if showWebhookEvents}
			<div class="mt-4 rounded-lg border" style="border-color: var(--color-border);">
				<div class="flex items-center justify-between px-4 py-2.5 border-b" style="border-color: var(--color-border);">
					<span class="text-xs font-medium" style="color: var(--color-text);">Event Timeline ({webhookEventsTotal} total)</span>
					<button
						class="rounded-md p-1 transition-colors"
						style="color: var(--color-text-muted);"
						onclick={toggleWebhookEvents}
					>
						<Icon icon="solar:close-circle-outline" class="h-3.5 w-3.5" />
					</button>
				</div>
				<div class="max-h-60 overflow-y-auto">
					{#if webhookEventsLoading}
						<div class="flex items-center justify-center gap-2 py-4">
							<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" style="color: var(--color-primary);" />
							<span class="text-xs" style="color: var(--color-text-muted);">Loading events...</span>
						</div>
					{:else if webhookEvents.length > 0}
						{#each webhookEvents as ev}
							<div class="flex items-start gap-3 px-4 py-2.5 border-b last:border-b-0" style="border-color: var(--color-border-light);">
								<span class="text-xs">{webhookEventTypeIcon(ev.event_type)}</span>
								<div class="min-w-0 flex-1">
									<div class="flex items-center gap-2">
										<span class="text-xs font-medium" style="color: var(--color-text);">{ev.repo}</span>
										{#if ev.tag}
											<span class="font-mono text-[10px]" style="color: var(--color-text-muted);">:{ev.tag}</span>
										{/if}
										<span class="rounded px-1 py-0.5 text-[9px] uppercase" style="background-color: {webhookEventStatusColor(ev.status)}20; color: {webhookEventStatusColor(ev.status)};">{ev.status}</span>
									</div>
									<div class="flex items-center gap-2 mt-0.5">
										<span class="text-[10px]" style="color: var(--color-text-muted);">{ev.event_type}</span>
										{#if ev.actor}
											<span class="text-[10px]" style="color: var(--color-text-muted);">by {ev.actor}</span>
										{/if}
										<span class="text-[10px]" style="color: var(--color-text-muted);">{formatDate(ev.created_at)}</span>
									</div>
								</div>
								<Icon icon={webhookEventStatusIcon(ev.status)} class="h-3.5 w-3.5 flex-shrink-0" style="color: {webhookEventStatusColor(ev.status)};" />
							</div>
						{/each}
					{:else}
						<div class="py-4 text-center">
							<p class="text-xs" style="color: var(--color-text-muted);">No events recorded yet.</p>
							<p class="text-[10px]" style="color: var(--color-text-muted);">Events will appear when images are pushed or deleted.</p>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	</div>
	{/if}
{/if}

	<!-- ─── Tab: Repos ─── -->
	{#if activeTab === 'repos'}
	<!-- KPI Header Cards -->
	{#if statsSummary}
		<div class="grid grid-cols-2 gap-3 mb-3 sm:grid-cols-4">
			<div class="rounded-lg p-3 text-center" style="background-color: var(--color-primary-subtle);">
				<div class="text-lg font-bold" style="color: var(--color-primary);">{statsSummary.total_repos}</div>
				<div class="text-[10px]" style="color: var(--color-text-muted);">Repositories</div>
			</div>
			<div class="rounded-lg p-3 text-center" style="background-color: var(--color-primary-subtle);">
				<div class="text-lg font-bold" style="color: var(--color-primary);">{statsSummary.total_tags}</div>
				<div class="text-[10px]" style="color: var(--color-text-muted);">Tags</div>
			</div>
			<div class="rounded-lg p-3 text-center" style="background-color: var(--color-primary-subtle);">
				<div class="text-lg font-bold" style="color: var(--color-primary);">{formatBytes(statsSummary.total_storage)}</div>
				<div class="text-[10px]" style="color: var(--color-text-muted);">Storage</div>
			</div>
			<div class="rounded-lg p-3 text-center" style="background-color: {registryHealth?.status === 'up' ? 'rgba(16,185,129,0.08)' : 'rgba(239,68,68,0.08)'};">
				{#if healthLoading}
					<div class="text-lg font-bold" style="color: var(--color-text-muted);">
						<Icon icon="solar:spinner-bold" class="h-5 w-5 inline animate-spin" />
					</div>
				{:else if registryHealth?.status === 'up'}
					<div class="text-lg font-bold" style="color: #10b981;">
						<Icon icon="solar:check-circle-bold" class="h-5 w-5 inline" />
					</div>
				{:else}
					<div class="text-lg font-bold" style="color: #ef4444;">
						<Icon icon="solar:forbidden-circle-bold" class="h-5 w-5 inline" />
					</div>
				{/if}
				<div class="text-[10px]" style="color: var(--color-text-muted);">Registry Status</div>
			</div>
		</div>
	{:else if !statsSummary && statsLoading}
		<div class="grid grid-cols-4 gap-3 mb-3">
			<div class="rounded-lg h-20 animate-pulse" style="background-color: var(--color-primary-subtle);"></div>
			<div class="rounded-lg h-20 animate-pulse" style="background-color: var(--color-primary-subtle);"></div>
			<div class="rounded-lg h-20 animate-pulse" style="background-color: var(--color-primary-subtle);"></div>
			<div class="rounded-lg h-20 animate-pulse" style="background-color: var(--color-primary-subtle);"></div>
		</div>
	{/if}

	<!-- Recent Activity -->
	{#if webhookEvents.length > 0}
	<div class="card p-4 mb-3">
		<div class="flex items-center justify-between mb-3">
			<div class="flex items-center gap-2">
				<Icon icon="solar:clock-circle-bold" class="h-4 w-4" style="color: var(--color-primary);" />
				<h3 class="text-sm font-semibold" style="color: var(--color-text);">Recent Activity</h3>
			</div>
			<button
				class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-medium transition-colors"
				style="color: var(--color-text-muted); border: 1px solid var(--color-border);"
				onclick={() => showAllEvents = !showAllEvents}
			>
				<Icon icon={showAllEvents ? 'solar:alt-arrow-up-outline' : 'solar:alt-arrow-down-outline'} class="h-3 w-3" />
				{showAllEvents ? 'Show Less' : `View All (${webhookEventsTotal})`}
			</button>
		</div>
		<div class="space-y-1">
			{#each (showAllEvents ? webhookEvents : webhookEvents.slice(0, 5)) as ev}
				<div class="flex items-start gap-2 rounded-lg px-3 py-1.5" style="background-color: var(--color-primary-subtle);">
					<span class="rounded px-1.5 py-0.5 text-[9px] font-mono uppercase" style="{webhookEventBadgeStyle(ev.event_type)}">
						{webhookEventTypeIcon(ev.event_type)} {ev.event_type}
					</span>
					<div class="min-w-0 flex-1">
						<div class="flex items-center gap-1.5 flex-wrap">
							<span class="text-xs font-medium" style="color: var(--color-text);">{ev.repo}</span>
							{#if ev.tag}
								<span class="font-mono text-[10px]" style="color: var(--color-text-muted);">:{ev.tag}</span>
							{/if}
							<span class="rounded px-1 py-0.5 text-[9px] uppercase" style="background-color: {webhookEventStatusColor(ev.status)}20; color: {webhookEventStatusColor(ev.status)};">{ev.event_type}</span>
						</div>
						<div class="flex items-center gap-2 mt-0.5">
							{#if ev.actor}
								<span class="text-[10px]" style="color: var(--color-text-muted);">by {ev.actor}</span>
							{/if}
							<span class="text-[10px]" style="color: var(--color-text-muted);">{formatDate(ev.created_at)}</span>
							{#if ev.payload}
								<button
									class="text-[10px] hover:underline"
									style="color: var(--color-primary);"
									onclick={() => { ev._showPayload = !ev._showPayload; }}
								>{ev._showPayload ? 'Hide' : 'Show'} Payload</button>
							{/if}
						</div>
						{#if ev._showPayload && ev.payload}
							<pre class="mt-1 rounded p-2 font-mono text-[9px] overflow-auto max-h-28" style="background-color: #0d1117; color: #e6edf3;">{typeof ev.payload === 'string' ? ev.payload : JSON.stringify(ev.payload, null, 2)}</pre>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	</div>
	{/if}

	<!-- Search + Stats -->
	<div class="flex items-center justify-between gap-4">
		<div class="relative flex-1 max-w-sm">
			{#if searchMode === 'repo'}
				<Icon icon="solar:magnifer-outline" class="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2" style="color: var(--color-text-muted);" />
				<input
					type="text"
					placeholder="Search repositories..."
					bind:value={searchQuery}
					class="w-full rounded-lg border py-2 pl-8 pr-3 text-xs"
					style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
				/>
			{:else}
				<Icon icon="solar:hashtag-outline" class="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2" style="color: var(--color-text-muted);" />
				<input
					type="text"
					placeholder="Search tags across all repos..."
					bind:value={tagSearchQuery}
					oninput={onTagSearchInput}
					class="w-full rounded-lg border py-2 pl-8 pr-3 text-xs"
					style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
				/>
			{/if}
			<div class="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-0.5 rounded-md p-0.5" style="background-color: var(--color-topbar-bg);">
				<button
					class="rounded px-1.5 py-0.5 text-[10px] font-medium transition-colors"
					style="color: {searchMode === 'repo' ? 'var(--color-text)' : 'var(--color-text-muted)'}; {searchMode === 'repo' ? 'background-color: var(--color-card);' : ''}"
					onclick={() => switchSearchMode('repo')}
				>Repo</button>
				<button
					class="rounded px-1.5 py-0.5 text-[10px] font-medium transition-colors"
					style="color: {searchMode === 'tag' ? 'var(--color-text)' : 'var(--color-text-muted)'}; {searchMode === 'tag' ? 'background-color: var(--color-card);' : ''}"
					onclick={() => switchSearchMode('tag')}
				>Tag</button>
			</div>
		</div>
		<div class="flex items-center gap-4 text-xs" style="color: var(--color-text-secondary);">
			{#if searchMode === 'repo'}
				<span>{filteredRepos.length} repos</span>
				<span class="h-3 w-px" style="background-color: var(--color-border);"></span>
				<span>{totalTags} tags</span>
			{:else if tagSearchQuery}
				<span>{tagSearchTotal} results</span>
			{/if}
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
			<span class="h-3 w-px" style="background-color: var(--color-border);"></span>
			<div class="inline-flex items-center gap-1 text-[10px]" style="color: {cveAvailable ? 'var(--color-success)' : 'var(--color-text-muted)'};" title="{cveAvailable ? 'CVE scanning enabled' : 'CVE scanning not available - enable Zot CVE extension'}">
				<Icon icon={cveChecking ? 'solar:spinner-bold animate-spin' : cveAvailable ? 'solar:shield-check-bold' : 'solar:shield-outline'} class="h-3 w-3" />
				{cveChecking ? 'CVE...' : cveAvailable ? 'CVE Active' : 'No CVE'}
			</div>
			<span class="h-3 w-px" style="background-color: var(--color-border);"></span>
			<button
				class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-medium transition-colors"
				style="color: var(--color-text-muted); border: 1px solid var(--color-border);"
				onclick={toggleStats}
			>
				<Icon icon={showStats ? 'solar:alt-arrow-up-outline' : 'solar:chart-square-bold'} class="h-3 w-3" />
				{showStats ? 'Hide Detail' : 'Top Repos'}
			</button>
			<button
				class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-medium transition-colors"
				style="color: var(--color-text-muted); border: 1px solid var(--color-border);"
				onclick={openCleanupModal}
			>
				<Icon icon="solar:eraser-bold" class="h-3 w-3" />
				Cleanup
			</button>
			{/if}
		</div>
	</div>

	<!-- Storage Stats Dashboard -->
	{#if showStats}
		<div class="card p-5">
			<div class="flex items-center justify-between mb-4">
				<div class="flex items-center gap-2">
					<Icon icon="solar:chart-square-bold" class="h-4 w-4" style="color: var(--color-primary);" />
					<h3 class="text-sm font-semibold" style="color: var(--color-text);">Storage Summary</h3>
				</div>
				<button
					class="rounded-md p-1 transition-colors"
					style="color: var(--color-text-muted);"
					onclick={() => showStats = false}
				>
					<Icon icon="solar:close-circle-outline" class="h-4 w-4" />
				</button>
			</div>

			{#if statsLoading}
				<div class="flex items-center justify-center py-8">
					<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-primary);" />
				</div>
			{:else if statsSummary}
				<div class="grid grid-cols-3 gap-3 mb-4">
					<div class="rounded-lg p-3 text-center" style="background-color: var(--color-primary-subtle);">
						<div class="text-lg font-bold" style="color: var(--color-primary);">{statsSummary.total_repos}</div>
						<div class="text-[10px]" style="color: var(--color-text-muted);">Repositories</div>
					</div>
					<div class="rounded-lg p-3 text-center" style="background-color: var(--color-primary-subtle);">
						<div class="text-lg font-bold" style="color: var(--color-primary);">{statsSummary.total_tags}</div>
						<div class="text-[10px]" style="color: var(--color-text-muted);">Tags</div>
					</div>
					<div class="rounded-lg p-3 text-center" style="background-color: var(--color-primary-subtle);">
						<div class="text-lg font-bold" style="color: var(--color-primary);">{formatBytes(statsSummary.total_storage)}</div>
						<div class="text-[10px]" style="color: var(--color-text-muted);">Total Storage</div>
					</div>
				</div>

        {#if statsSummary.top_repos?.length}
          {@const maxSize = statsSummary.top_repos[0]?.total_size || 1}
          <h4 class="mb-2 text-xs font-medium" style="color: var(--color-text-secondary);">Top Repositories by Size</h4>
          <div class="space-y-1.5">
            {#each statsSummary.top_repos as repo}
							<div class="flex items-center gap-2">
								<div class="flex-1 min-w-0">
									<div class="flex items-center justify-between mb-0.5">
										<span class="text-xs truncate" style="color: var(--color-text);">{repo.name}</span>
										<span class="text-[10px] flex-shrink-0 ml-2" style="color: var(--color-text-muted);">{formatBytes(repo.total_size)}</span>
									</div>
									<div class="h-1.5 rounded-full" style="background-color: var(--color-border);">
										<div class="h-full rounded-full transition-all" style="width: {storageBarWidth(repo.total_size, maxSize)}%; background-color: var(--color-primary); opacity: 0.6;"></div>
									</div>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			{:else}
				<div class="py-4 text-center">
					<p class="text-xs" style="color: var(--color-text-muted);">Failed to load storage summary.</p>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Error -->
	{#if error}
		<div class="rounded-lg border p-3 text-xs" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2); color: var(--color-danger);">
			<div class="flex items-center gap-2">
				<Icon icon="solar:danger-triangle-bold" class="h-4 w-4" />
				<span>{error}</span>
			</div>
		</div>
	{/if}

	{#if searchMode === 'tag'}
		<!-- Tag Search Results -->
		{#if tagSearchLoading}
			<div class="flex items-center justify-center py-8">
				<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-primary);" />
			</div>
		{:else if tagSearchQuery && tagSearchResults.length > 0}
			<div class="card p-0 overflow-hidden">
				<div class="divide-y" style="border-color: var(--color-border);">
					{#each tagSearchResults as result}
						<div class="flex items-center justify-between px-4 py-2.5 transition-colors hover:opacity-80">
							<div class="min-w-0 flex-1">
								<button
									class="font-mono text-xs hover:underline"
									style="color: var(--color-primary);"
									onclick={() => goto(`/registry/${result.repo}/${result.tag}`)}
								>
									{result.repo}:{result.tag}
								</button>
							</div>
							<div class="flex items-center gap-2">
								{#if result.digest}
									<span class="font-mono text-[10px]" style="color: var(--color-text-muted);">{shortDigest(result.digest)}</span>
								{/if}
								<button
									class="rounded-md p-1.5 transition-colors"
									style="color: var(--color-text-muted);"
									onclick={() => copyToClipboard(`docker pull registry.anjungan.io/${result.repo}:${result.tag}`, `search-${result.repo}-${result.tag}`)}
									title="Copy pull command"
								>
									<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" />
								</button>
								{#if copiedTarget === `search-${result.repo}-${result.tag}`}
									<span class="text-[10px]" style="color: var(--color-success);">✓</span>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			</div>
		{:else if tagSearchQuery && !tagSearchLoading}
			<div class="rounded-lg border p-6 text-center" style="border-color: var(--color-border);">
				<p class="text-xs" style="color: var(--color-text-muted);">No tags match "{tagSearchQuery}"</p>
			</div>
		{/if}
	{:else}
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
				<button
					class="flex w-full items-center gap-3 rounded-lg border px-4 py-3 text-left transition-colors hover:opacity-80"
					style="background-color: var(--color-card); border-color: var(--color-border);"
					onclick={() => goto(`/registry/${repo.name}`)}
				>
					<Icon icon="solar:archive-down-minimlistic-bold" class="h-5 w-5 flex-shrink-0" style="color: var(--color-primary);" />
					<div class="min-w-0 flex-1">
						<div class="flex items-center justify-between gap-2">
							<span class="text-sm font-medium truncate" style="color: var(--color-text);">{repo.name}</span>
							<div class="flex items-center gap-2 flex-shrink-0">
								<span class="rounded px-1.5 py-0.5 text-[10px] font-medium" style="background-color: var(--color-primary-subtle); color: var(--color-primary);">
									{repo.tags_count || 0} tag{(repo.tags_count || 0) !== 1 ? 's' : ''}
								</span>
								{#if isAdmin}
									<button
										class="rounded-md p-1 transition-colors hover:opacity-80"
										style="color: var(--color-text-muted);"
										onclick={(e) => { e.stopPropagation(); confirmDeleteRepo(repo.name); }}
										title="Delete repository and all its tags"
									>
										<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
									</button>
								{/if}
							</div>
						</div>
					</div>
					<Icon icon="solar:alt-arrow-right-outline" class="h-4 w-4 flex-shrink-0" style="color: var(--color-text-muted);" />
				</button>
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
{/if}
{/if}

	<!-- ─── Tab: Activity ─── -->
	{#if activeTab === 'activity'}
	<!-- Recent Activity -->
	{#if webhookEvents.length > 0}
	<div class="card p-4 mb-3">
		<div class="flex items-center justify-between mb-3">
			<div class="flex items-center gap-2">
				<Icon icon="solar:clock-circle-bold" class="h-4 w-4" style="color: var(--color-primary);" />
				<h3 class="text-sm font-semibold" style="color: var(--color-text);">Recent Activity</h3>
			</div>
			<button
				class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-medium transition-colors"
				style="color: var(--color-text-muted); border: 1px solid var(--color-border);"
				onclick={() => showAllEvents = !showAllEvents}
			>
				<Icon icon={showAllEvents ? 'solar:alt-arrow-up-outline' : 'solar:alt-arrow-down-outline'} class="h-3 w-3" />
				{showAllEvents ? 'Show Less' : `View All (${webhookEventsTotal})`}
			</button>
		</div>
		<div class="space-y-1">
			{#each (showAllEvents ? webhookEvents : webhookEvents.slice(0, 5)) as ev}
				<div class="flex items-start gap-2 rounded-lg px-3 py-1.5" style="background-color: var(--color-primary-subtle);">
					<span class="rounded px-1.5 py-0.5 text-[9px] font-mono uppercase" style="{webhookEventBadgeStyle(ev.event_type)}">
						{webhookEventTypeIcon(ev.event_type)} {ev.event_type}
					</span>
					<div class="min-w-0 flex-1">
						<div class="flex items-center gap-1.5 flex-wrap">
							<span class="text-xs font-medium" style="color: var(--color-text);">{ev.repo}</span>
							{#if ev.tag}
								<span class="font-mono text-[10px]" style="color: var(--color-text-muted);">:{ev.tag}</span>
							{/if}
							<span class="rounded px-1 py-0.5 text-[9px] uppercase" style="background-color: {webhookEventStatusColor(ev.status)}20; color: {webhookEventStatusColor(ev.status)};">{ev.event_type}</span>
						</div>
						<div class="flex items-center gap-2 mt-0.5">
							{#if ev.actor}
								<span class="text-[10px]" style="color: var(--color-text-muted);">by {ev.actor}</span>
							{/if}
							<span class="text-[10px]" style="color: var(--color-text-muted);">{formatDate(ev.created_at)}</span>
							{#if ev.payload}
								<button
									class="text-[10px] hover:underline"
									style="color: var(--color-primary);"
									onclick={() => { ev._showPayload = !ev._showPayload; }}
								>{ev._showPayload ? 'Hide' : 'Show'} Payload</button>
							{/if}
						</div>
						{#if ev._showPayload && ev.payload}
							<pre class="mt-1 rounded p-2 font-mono text-[9px] overflow-auto max-h-28" style="background-color: #0d1117; color: #e6edf3;">{typeof ev.payload === 'string' ? ev.payload : JSON.stringify(ev.payload, null, 2)}</pre>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	</div>
	{/if}

	<!-- Webhook Events Timeline -->
	<div class="card p-4">
		<div class="flex items-center justify-between mb-3">
			<div class="flex items-center gap-2">
				<Icon icon="solar:history-bold" class="h-4 w-4" style="color: var(--color-primary);" />
				<h3 class="text-sm font-semibold" style="color: var(--color-text);">Webhook Events</h3>
				<span class="rounded-full px-2 py-0.5 text-[10px] font-medium" style="background-color: var(--color-primary-subtle); color: var(--color-primary);">{webhookEventsTotal}</span>
			</div>
		</div>
		{#if webhookEventsLoading}
			<div class="flex items-center justify-center gap-2 py-4">
				<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" style="color: var(--color-primary);" />
				<span class="text-xs" style="color: var(--color-text-muted);">Loading events...</span>
			</div>
		{:else if webhookEvents.length > 0}
			<div class="divide-y" style="border-color: var(--color-border);">
				{#each webhookEvents as ev}
					<div class="flex items-start gap-3 px-4 py-2.5">
						<span class="rounded px-1.5 py-0.5 text-[9px] font-mono uppercase" style="{webhookEventBadgeStyle(ev.event_type)}">
							{webhookEventTypeIcon(ev.event_type)} {ev.event_type}
						</span>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2">
								<span class="text-xs font-medium" style="color: var(--color-text);">{ev.repo}</span>
								{#if ev.tag}
									<span class="font-mono text-[10px]" style="color: var(--color-text-muted);">:{ev.tag}</span>
								{/if}
								<span class="rounded px-1 py-0.5 text-[9px] uppercase" style="background-color: {webhookEventStatusColor(ev.status)}20; color: {webhookEventStatusColor(ev.status)};">{ev.status}</span>
							</div>
							<div class="flex items-center gap-2 mt-0.5">
								<span class="text-[10px]" style="color: var(--color-text-muted);">{ev.event_type}</span>
								{#if ev.actor}
									<span class="text-[10px]" style="color: var(--color-text-muted);">by {ev.actor}</span>
								{/if}
								<span class="text-[10px]" style="color: var(--color-text-muted);">{formatDate(ev.created_at)}</span>
								{#if ev.payload}
									<button
										class="text-[10px] hover:underline"
										style="color: var(--color-primary);"
										onclick={() => { ev._showPayload = !ev._showPayload; }}
									>{ev._showPayload ? 'Hide' : 'Show'} Payload</button>
								{/if}
							</div>
							{#if ev._showPayload && ev.payload}
								<pre class="mt-1 rounded p-2 font-mono text-[9px] overflow-auto max-h-28" style="background-color: #0d1117; color: #e6edf3;">{typeof ev.payload === 'string' ? ev.payload : JSON.stringify(ev.payload, null, 2)}</pre>
							{/if}
						</div>
						<Icon icon={webhookEventStatusIcon(ev.status)} class="h-3.5 w-3.5 flex-shrink-0" style="color: {webhookEventStatusColor(ev.status)};" />
					</div>
				{/each}
			</div>
		{:else}
			<div class="py-4 text-center">
				<p class="text-xs" style="color: var(--color-text-muted);">No events recorded yet.</p>
				<p class="text-[10px]" style="color: var(--color-text-muted);">Events will appear when images are pushed or deleted.</p>
			</div>
		{/if}
	</div>
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

<!-- Webhook Modal (Add/Edit) -->
{#if showWebhookModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center"
		style="background-color: rgba(0,0,0,0.6);"
		onclick={closeWebhookModal}
	>
		<div
			class="mx-4 w-full max-w-md rounded-xl border shadow-2xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="p-5">
				<div class="flex items-center gap-2 mb-4">
					<Icon icon="solar:bell-bing-bold" class="h-5 w-5" style="color: var(--color-primary);" />
					<h3 class="text-sm font-semibold" style="color: var(--color-text);">
						{webhookModalMode === 'add' ? 'Add Webhook' : 'Edit Webhook'}
					</h3>
				</div>

				<div class="space-y-3">
					<div>
						<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Name <span class="text-[10px]" style="color: var(--color-text-muted);">(optional)</span></label>
						<input
							type="text"
							bind:value={webhookForm.name}
							class="w-full rounded-lg border px-3 py-2 text-xs"
							style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
							placeholder="e.g. Telegram Alerts"
						/>
					</div>
					<div>
						<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Platform</label>
						<select
							bind:value={webhookForm.platform}
							class="w-full rounded-lg border px-3 py-2 text-xs"
							style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
						>
							<option value="telegram">Telegram</option>
							<option value="discord">Discord</option>
							<option value="slack">Slack</option>
							<option value="generic">Generic Webhook</option>
						</select>
					</div>
					<div>
						<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Webhook URL</label>
						<input
							type="url"
							bind:value={webhookForm.url}
							class="w-full rounded-lg border px-3 py-2 text-xs font-mono"
							style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
							placeholder="https://hooks.example.com/..."
						/>
					</div>
					<div>
						<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Trigger Events</label>
						<div class="flex flex-wrap gap-3">
							<label class="inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs cursor-pointer" style="border-color: var(--color-border); color: var(--color-text);">
								<input type="checkbox" value="push" bind:group={webhookForm.events} class="h-3.5 w-3.5" />
								📦 Push
							</label>
							<label class="inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs cursor-pointer" style="border-color: var(--color-border); color: var(--color-text);">
								<input type="checkbox" value="pull" bind:group={webhookForm.events} class="h-3.5 w-3.5" />
								⬇️ Pull
							</label>
							<label class="inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs cursor-pointer" style="border-color: var(--color-border); color: var(--color-text);">
								<input type="checkbox" value="delete" bind:group={webhookForm.events} class="h-3.5 w-3.5" />
								🗑 Delete
							</label>
						</div>
					</div>
					{#if webhookModalMode === 'add'}
						<div>
							<label class="inline-flex items-center gap-2 text-xs cursor-pointer" style="color: var(--color-text);">
								<input type="checkbox" bind:checked={webhookForm.enabled} class="h-3.5 w-3.5" />
								Enable immediately
							</label>
						</div>
					{/if}
					{#if webhookFormError}
						<div class="rounded-md p-2 text-xs" style="background-color: rgba(239,68,68,0.08); color: var(--color-danger);">
							{webhookFormError}
						</div>
					{/if}
				</div>
			</div>
			<div class="flex items-center justify-end gap-2 rounded-b-xl border-t px-5 py-3" style="border-color: var(--color-border); background-color: var(--color-topbar-bg);">
				<button
					class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
					style="color: var(--color-text-secondary);"
					onclick={closeWebhookModal}
				>Cancel</button>
				<button
					class="inline-flex items-center gap-1.5 rounded-lg px-4 py-1.5 text-xs font-medium text-white transition-colors"
					style="background-color: var(--color-primary);"
					onclick={submitWebhookForm}
					disabled={webhookFormLoading}
				>
					{#if webhookFormLoading}
						<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
						Saving...
					{:else}
						{webhookModalMode === 'add' ? 'Create Webhook' : 'Save Changes'}
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Cleanup Modal -->
{#if cleanupModalOpen}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center"
		style="background-color: rgba(0,0,0,0.6);"
		onclick={closeCleanupModal}
	>
		<div
			class="mx-4 w-full max-w-lg rounded-xl border shadow-2xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="p-5">
				<div class="flex items-center gap-2 mb-4">
					<Icon icon="solar:eraser-bold" class="h-5 w-5" style="color: var(--color-primary);" />
					<h3 class="text-sm font-semibold" style="color: var(--color-text);">Registry Cleanup</h3>
				</div>

				{#if cleanupConfig}
					<div class="space-y-4">
						<div class="flex items-center gap-3">
							<label class="inline-flex items-center gap-2 text-xs cursor-pointer" style="color: var(--color-text);">
								<input type="checkbox" bind:checked={cleanupConfig.enabled} class="h-3.5 w-3.5" onchange={saveCleanupConfig} />
								Enable automatic cleanup
							</label>
						</div>

						<div class="grid grid-cols-2 gap-3">
							<div>
								<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Keep Last N Tags</label>
								<input
									type="number" min="0"
									bind:value={cleanupConfig.keep_last_n}
									class="w-full rounded-lg border px-3 py-2 text-xs"
									style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
									onchange={saveCleanupConfig}
									placeholder="0 = disabled"
								/>
							</div>
							<div>
								<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Max Age (days)</label>
								<input
									type="number" min="0"
									bind:value={cleanupConfig.max_age_days}
									class="w-full rounded-lg border px-3 py-2 text-xs"
									style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
									onchange={saveCleanupConfig}
									placeholder="0 = disabled"
								/>
							</div>
						</div>

						<div>
							<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-secondary);">Excluded Tags (comma separated)</label>
							<input
								type="text"
								bind:value={cleanupConfig.exclude_tags}
								class="w-full rounded-lg border px-3 py-2 text-xs"
								style="background-color: var(--color-card); border-color: var(--color-border); color: var(--color-text);"
								onchange={saveCleanupConfig}
								placeholder="latest, production, staging"
							/>
						</div>
					</div>


					{#if cleanupConfig.scheduler_active}
						<div class="flex items-center gap-2 text-xs" style="color: var(--color-success);">
							<Icon icon="solar:check-circle-bold" class="h-3.5 w-3.5" />
							Scheduler active — runs every hour
						</div>
					{/if}
					<hr class="my-4" style="border-color: var(--color-border);" />

					{#if cleanupRunning}
						<div class="flex items-center justify-center gap-2 py-4">
							<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" style="color: var(--color-primary);" />
							<span class="text-xs" style="color: var(--color-text-muted);">Running cleanup...</span>
						</div>
					{:else if cleanupResult}
						{#if cleanupResult.error}
							<div class="rounded-lg p-3 text-xs" style="background-color: rgba(239,68,68,0.08); color: var(--color-danger);">
								{cleanupResult.error}
							</div>
						{:else}
							<div class="rounded-lg p-3" style="background-color: rgba(16,185,129,0.08);">
								<p class="text-xs font-medium" style="color: var(--color-success);">Cleanup completed</p>
								<div class="mt-2 grid grid-cols-3 gap-2 text-center">
									<div>
										<div class="text-sm font-bold" style="color: var(--color-text);">{cleanupResult.repos_scanned}</div>
										<div class="text-[10px]" style="color: var(--color-text-muted);">Repos</div>
									</div>
									<div>
										<div class="text-sm font-bold" style="color: var(--color-text);">{cleanupResult.tags_deleted}</div>
										<div class="text-[10px]" style="color: var(--color-text-muted);">Deleted</div>
									</div>
									<div>
										<div class="text-sm font-bold" style="color: var(--color-text);">{formatBytes(cleanupResult.space_freed)}</div>
										<div class="text-[10px]" style="color: var(--color-text-muted);">Freed</div>
									</div>
								</div>
								{#if cleanupResult.deleted_tags?.length}
									<details class="mt-2">
										<summary class="text-[10px] cursor-pointer" style="color: var(--color-text-muted);">Deleted tags ({cleanupResult.deleted_tags.length})</summary>
										<div class="mt-1 max-h-32 overflow-y-auto space-y-0.5">
											{#each cleanupResult.deleted_tags as tag}
												<div class="text-[10px] font-mono" style="color: var(--color-text-muted);">{tag}</div>
											{/each}
										</div>
									</details>
								{/if}
							</div>
						{/if}
					{/if}
				{:else}
					<div class="py-4 text-center">
						<p class="text-xs" style="color: var(--color-text-muted);">Loading configuration...</p>
					</div>
				{/if}
			</div>
			<div class="flex items-center justify-end gap-2 rounded-b-xl border-t px-5 py-3" style="border-color: var(--color-border); background-color: var(--color-topbar-bg);">
				<button
					class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
					style="color: var(--color-text-secondary);"
					onclick={closeCleanupModal}
				>Close</button>
				{#if cleanupConfig?.enabled}
					<button
						class="inline-flex items-center gap-1.5 rounded-lg px-4 py-1.5 text-xs font-medium text-white transition-colors"
						style="background-color: var(--color-danger);"
						onclick={runCleanup}
						disabled={cleanupRunning}
					>
						{#if cleanupRunning}
							<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
							Running...
						{:else}
							<Icon icon="solar:eraser-bold" class="h-3.5 w-3.5" />
							Run Cleanup Now
						{/if}
					</button>
				{/if}
			</div>
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

<!-- Protected Tag Delete Warning Modal -->
{#if protectedDeleteTarget}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center"
		style="background-color: rgba(0,0,0,0.6);"
		onclick={() => protectedDeleteTarget = null}
	>
		<div
			class="mx-4 w-full max-w-md rounded-xl border shadow-2xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="p-5">
				<div class="flex items-start gap-3">
					<div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full" style="background-color: rgba(245,158,11,0.15);">
						<Icon icon="solar:shield-warning-bold" class="h-4.5 w-4.5" style="color: var(--color-warning);" />
					</div>
					<div class="min-w-0 flex-1">
						<h3 class="text-sm font-semibold" style="color: var(--color-text);">Cannot Delete Protected Tag</h3>
						<p class="mt-1 text-xs" style="color: var(--color-text-secondary);">
							This tag is <strong>protected</strong> and cannot be deleted directly.
						</p>

						<div class="mt-4 rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
							<div class="flex items-center justify-between py-1">
								<span class="text-xs" style="color: var(--color-text-muted);">Repository</span>
								<span class="font-mono text-xs font-medium" style="color: var(--color-text);">{protectedDeleteTarget.repo}</span>
							</div>
							<div class="flex items-center justify-between py-1">
								<span class="text-xs" style="color: var(--color-text-muted);">Tag</span>
								<span class="font-mono text-xs font-medium" style="color: var(--color-warning);">{protectedDeleteTarget.tag}</span>
							</div>
						</div>

						<div class="mt-3 rounded-lg border p-2.5" style="background-color: rgba(245,158,11,0.08); border-color: rgba(245,158,11,0.2);">
							<div class="flex items-start gap-2">
								<Icon icon="solar:info-circle-bold" class="mt-0.5 h-3.5 w-3.5 flex-shrink-0" style="color: var(--color-warning);" />
								<p class="text-xs" style="color: var(--color-text-secondary);">
									Unprotect the tag first using the <strong>shield icon</strong> next to it, then you can delete it.
								</p>
							</div>
						</div>
					</div>
				</div>
			</div>
			<div class="flex items-center justify-end gap-2 rounded-b-xl border-t px-5 py-3" style="border-color: var(--color-border); background-color: var(--color-topbar-bg);">
				<button
					class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
					style="background-color: var(--color-primary); color: white;"
					onclick={() => protectedDeleteTarget = null}
				>Got it</button>
			</div>
		</div>
	</div>
{/if}

<!-- Delete Repo Modal -->
{#if deleteRepoTarget}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center"
		style="background-color: rgba(0,0,0,0.6);"
		onclick={() => { if (!deleteRepoLoading) deleteRepoTarget = null; }}
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
						<h3 class="text-sm font-semibold" style="color: var(--color-text);">Delete Repository</h3>
						<p class="mt-1 text-xs" style="color: var(--color-text-secondary);">
							Are you sure you want to delete <strong class="font-mono">{deleteRepoTarget}</strong> and all its tags? This action is irreversible.
						</p>

						<div class="mt-4 rounded-lg p-3" style="background-color: rgba(245,158,11,0.08); border: 1px solid rgba(245,158,11,0.2);">
							<div class="flex items-start gap-2">
								<Icon icon="solar:info-circle-bold" class="mt-0.5 h-3.5 w-3.5 flex-shrink-0" style="color: var(--color-warning);" />
								<p class="text-xs" style="color: var(--color-text-secondary);">
									All tags, manifests, and blobs associated with this repository will be permanently removed.
								</p>
							</div>
						</div>

						{#if deleteRepoResult?.error}
							<div class="mt-3 rounded-lg border p-2.5" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2);">
								<div class="flex items-center gap-2">
									<Icon icon="solar:danger-triangle-bold" class="h-3.5 w-3.5 flex-shrink-0" style="color: var(--color-danger);" />
									<p class="text-xs" style="color: var(--color-danger);">{deleteRepoResult.error}</p>
								</div>
							</div>
						{/if}
					</div>
				</div>
			</div>
			<div class="flex items-center justify-end gap-2 rounded-b-xl border-t px-5 py-3" style="border-color: var(--color-border); background-color: var(--color-topbar-bg);">
				<button
					class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
					style="color: var(--color-text-secondary);"
					onclick={() => deleteRepoTarget = null}
					disabled={deleteRepoLoading}
				>Cancel</button>
				<button
					class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium text-white transition-colors"
					style="background-color: var(--color-danger);"
					onclick={executeDeleteRepo}
					disabled={deleteRepoLoading}
				>
					{#if deleteRepoLoading}
						<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
						Deleting...
					{:else}
						<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
						Delete Repository
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}
