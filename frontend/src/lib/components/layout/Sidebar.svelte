<script>
	import Icon from '@iconify/svelte';
	import { sidebarCollapsed, theme, user } from '$lib/stores/auth.js';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';

	let containerCount = $state(null);

	// Quick Access bookmarks
	let quickAccessBookmarks = $state([]);
	let quickAccessLoading = $state(true);
	let quickAccessCollapsed = $state(false);

	onMount(async () => {
		try {
			const stats = await api.containers.stats();
			containerCount = stats?.total ?? null;
		} catch (_) {}

		// Load sidebar bookmarks
		await loadQuickAccess();

		// Listen for bookmark changes (create/update/delete/pin)
		const handler = () => loadQuickAccess();
		window.addEventListener('bookmarks-changed', handler);

		// Restore collapse state from localStorage
		try {
			const saved = localStorage.getItem('sidebar_quick_access_collapsed');
			if (saved !== null) quickAccessCollapsed = saved === 'true';
		} catch (_) {}
		// Collapsed on mobile by default
		if (window.innerWidth < 1024) {
			quickAccessCollapsed = true;
		}

		return () => {
			window.removeEventListener('bookmarks-changed', handler);
		};
	});

	async function loadQuickAccess() {
		quickAccessLoading = true;
		try {
			const all = await api.bookmarks.list() || [];
			// Only show pinned bookmarks — empty if nothing pinned
			quickAccessBookmarks = all.filter(b => b.pinned).slice(0, 8);
		} catch (_) {
			quickAccessBookmarks = [];
		} finally {
			quickAccessLoading = false;
		}
	}

	function toggleQuickAccess() {
		quickAccessCollapsed = !quickAccessCollapsed;
		try {
			localStorage.setItem('sidebar_quick_access_collapsed', String(quickAccessCollapsed));
		} catch (_) {}
	}

	function qaFaviconUrl(url) {
		try {
			const u = new URL(url);
			return `https://www.google.com/s2/favicons?domain=${u.hostname}&sz=16`;
		} catch {
			return '';
		}
	}

	const allCategories = [
		{
			name: 'Dashboard',
			items: [
				{ href: '/', icon: 'solar:chart-2-bold', label: 'Overview' },
				{ href: '/bookmarks', icon: 'solar:bookmark-square-bold', label: 'Bookmarks' },
			],
		},
		{
			name: 'Infrastructure',
			items: [
				{ href: '/servers', icon: 'solar:server-square-bold', label: 'Servers' },
				{ href: '/ssh-keys', icon: 'solar:key-minimalistic-bold', label: 'SSH Keys', adminOnly: true },
				{ href: '/containers', icon: 'solar:box-bold', label: 'Containers', badge: 'containerCount' },
			],
		},
		{
			name: 'Artifact',
			items: [
				{ href: '/registry', icon: 'solar:archive-down-minimlistic-bold', label: 'Registry' },
			],
		},
		{
			name: 'Ops',
			items: [
				{ href: '/uptime', icon: 'solar:chart-2-bold', label: 'Uptime' },
				{ href: '/notifications', icon: 'solar:bell-bold', label: 'Notifications' },
			],
		},
		{
			name: 'Security',
			items: [
				{ href: '/ssl-monitors', icon: 'solar:shield-check-bold', label: 'SSL Monitors' },
				{ href: '/compliance', icon: 'solar:shield-check-bold', label: 'Compliance' },
				{ href: '/admin/login-activity', icon: 'solar:login-2-bold', label: 'Login Activity', adminOnly: true },
			],
		},
		{
			name: 'Administration',
			items: [
				{ href: '/admin/users', icon: 'solar:shield-user-bold', label: 'Users', adminOnly: true },
				{ href: '/admin/audit-log', icon: 'solar:notes-bold', label: 'Audit Log', adminOnly: true },
			],
		},
		{
			name: 'Account',
			items: [
				{ href: '/auth/login-history', icon: 'solar:login-2-bold', label: 'Login History' },
			],
		},
	];

	// Filter out admin-only items for non-admin users, then remove empty categories
	let categories = $derived(allCategories
		.map(cat => ({
			...cat,
			items: $user?.role === 'admin'
				? cat.items
				: cat.items.filter(item => !item.adminOnly),
		}))
		.filter(cat => cat.items.length > 0)
	);

	function isActive(href) {
		return $page.url.pathname === href;
	}

	function isCategoryActive(items) {
		return items.some(item => $page.url.pathname.startsWith(item.href));
	}

	function toggleSidebar() {
		$sidebarCollapsed = !$sidebarCollapsed;
	}

	function toggleTheme() {
		$theme = $theme === 'dark' ? 'light' : 'dark';
		localStorage.setItem('theme', $theme);
		if ($theme === 'dark') {
			document.documentElement.classList.add('dark');
		} else {
			document.documentElement.classList.remove('dark');
		}
	}

	function handleOverlayClick() {
		toggleSidebar();
	}
</script>

<aside
	class="fixed left-0 top-0 z-40 flex h-screen flex-col border-r transition-transform duration-200 lg:translate-x-0"
	class:-translate-x-full={$sidebarCollapsed}
	style="background-color: var(--color-sidebar-bg); border-color: var(--color-sidebar-border); width: 256px;"
>
	<!-- logo area -->
	<div class="flex h-16 items-center gap-3 border-b px-6" style="border-color: var(--color-sidebar-border);">
		<div class="flex h-8 w-8 items-center justify-center rounded-lg" style="background-color: var(--color-primary);">
			<span class="text-sm font-bold text-white">A</span>
		</div>
		<span class="text-lg font-semibold" style="color: var(--color-sidebar-text);">Anjungan</span>
	</div>

	<!-- navigation with categories -->
	<nav class="flex-1 space-y-4 overflow-y-auto px-3 py-4">
		<!-- Quick Access -->
		{#if !quickAccessLoading && quickAccessBookmarks.length > 0}
			<div>
				<button
					onclick={toggleQuickAccess}
					class="mb-1 flex w-full items-center justify-between px-3 text-xs font-semibold uppercase tracking-wider hover:opacity-80"
					style="color: var(--color-text-muted); opacity: 0.6;"
				>
					<span>⚡ Quick Access</span>
					<Icon
						icon={quickAccessCollapsed ? 'solar:alt-arrow-down-bold' : 'solar:alt-arrow-up-bold'}
						class="h-3 w-3"
					/>
				</button>
				{#if !quickAccessCollapsed}
					<div class="space-y-0.5">
						{#each quickAccessBookmarks as b}
							<a
								href={b.url}
								target="_blank"
								rel="noopener noreferrer"
								class="nav-link"
							>
								<img
									src={qaFaviconUrl(b.url)}
									alt=""
									class="h-4 w-4 shrink-0 rounded-sm"
									onerror={(e) => { e.target.style.display = 'none'; e.target.nextElementSibling.style.display = 'flex'; }}
								/>
								<span class="hidden h-4 w-4 shrink-0 items-center justify-center text-[10px] font-bold" style="color: var(--color-primary);">
									{b.title.charAt(0).toUpperCase()}
								</span>
								<span class="truncate text-[13px]">{b.title}</span>
							</a>
						{/each}
						<a href="/bookmarks" class="nav-link text-xs opacity-60 hover:opacity-100">
							Manage &rarr;
						</a>
					</div>
				{/if}
			</div>
		{/if}

		{#each categories as cat}
			<div>
				<div class="mb-1 px-3 text-xs font-semibold uppercase tracking-wider"
					style="color: var(--color-text-muted); opacity: 0.6;">
					{cat.name}
				</div>
				<div class="space-y-0.5">
					{#each cat.items as item}
						<a
							href={item.href}
							class="nav-link"
							class:active={isActive(item.href)}
							onclick={() => {
								if (window.innerWidth < 1024) $sidebarCollapsed = true;
							}}
						>
							<Icon icon={item.icon} class="h-5 w-5 shrink-0" />
							<span class="flex-1">{item.label}</span>
							{#if item.badge && item.badge === 'containerCount' && containerCount !== null}
								<span class="flex h-5 min-w-[20px] items-center justify-center rounded-full px-1.5 text-[10px] font-bold leading-none"
									style="background-color: var(--color-primary); color: #fff;">
									{containerCount}
								</span>
							{/if}
						</a>
					{/each}
				</div>
			</div>
		{/each}
	</nav>

	<!-- bottom section -->
	<div class="border-t px-3 py-3 space-y-0.5" style="border-color: var(--color-sidebar-border);">
		<button
			onclick={toggleTheme}
			class="nav-link w-full"
		>
			<Icon icon={$theme === 'dark' ? 'solar:sun-bold' : 'solar:moon-star-bold'} class="h-5 w-5 shrink-0" />
			<span>{$theme === 'dark' ? 'Light Mode' : 'Dark Mode'}</span>
		</button>
		<a
			href="/settings"
			class="nav-link"
			onclick={() => { if (window.innerWidth < 1024) $sidebarCollapsed = true; }}
		>
			<Icon icon="solar:settings-bold" class="h-5 w-5 shrink-0" />
			<span>Settings</span>
		</a>
	</div>
</aside>

<!-- mobile overlay (only when sidebar is open on mobile) -->
{#if !$sidebarCollapsed}
	<div
		class="fixed inset-0 z-30 bg-black/50 lg:hidden"
		role="presentation"
		onclick={handleOverlayClick}
	></div>
{/if}
