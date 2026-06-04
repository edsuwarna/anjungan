<script>
	import Icon from '@iconify/svelte';
	import { sidebarCollapsed, theme } from '$lib/stores/auth.js';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';

	let containerCount = $state(null);

	onMount(async () => {
		try {
			const stats = await api.containers.stats();
			containerCount = stats?.total ?? null;
		} catch (_) {}
	});

	const categories = [
		{
			name: 'Dashboard',
			items: [
				{ href: '/', icon: 'solar:chart-2-bold', label: 'Overview' },
			],
		},
		{
			name: 'Infrastructure',
			items: [
				{ href: '/servers', icon: 'solar:server-square-bold', label: 'Servers' },
				{ href: '/ssh-keys', icon: 'solar:key-minimalistic-bold', label: 'SSH Keys' },
				{ href: '/containers', icon: 'solar:box-bold', label: 'Containers', badge: 'containerCount' },
				{ href: '/registry', icon: 'solar:archive-down-minimlistic-bold', label: 'Registry' },
				{ href: '/repositories', icon: 'solar:code-square-bold', label: 'Repositories' },
			],
		},
		{
			name: 'Ops',
			items: [
				{ href: '/deployments', icon: 'solar:rocket-bold', label: 'Deployments' },
			],
		},
		{
			name: 'Security',
			items: [
				{ href: '/compliance', icon: 'solar:shield-check-bold', label: 'Compliance' },
			],
		},
		{
			name: 'Administration',
			items: [
				{ href: '/admin/users', icon: 'solar:shield-user-bold', label: 'Users' },
				{ href: '/admin/audit-log', icon: 'solar:notes-bold', label: 'Audit Log' },
			],
		},
	];

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
