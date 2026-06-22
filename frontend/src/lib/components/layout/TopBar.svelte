<script>
	import { user, sidebarCollapsed } from '$lib/stores/auth.js';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';

	let dropdownOpen = $state(false);

	const pageTitles = {
		'/': 'Overview',
		'/servers': 'Servers',
		'/containers': 'Containers',
		'/registry': 'Registry',
		'/admin/users': 'User Management'
	};

	let title = $derived(pageTitles[$page.url.pathname] || 'Anjungan');

	function toggleDropdown() {
		dropdownOpen = !dropdownOpen;
	}

	function handleLogout() {
		// Fire-and-forget backend logout (logs audit event)
		api.auth.logout().catch(() => {});

		localStorage.removeItem('access_token');
		localStorage.removeItem('refresh_token');
		localStorage.removeItem('user');
		user.set(null);
		goto('/login');
	}

	function handleClickOutside(e) {
		if (!e.target.closest('.user-menu')) {
			dropdownOpen = false;
		}
	}

	function toggleMobileSidebar() {
		$sidebarCollapsed = !$sidebarCollapsed;
	}
</script>

<svelte:window on:click={handleClickOutside} />

<header
	class="flex h-16 items-center justify-between border-b px-4 sm:px-6"
	style="background-color: var(--color-topbar-bg); border-color: var(--color-topbar-border);"
>
	<div class="flex items-center gap-3">
		<!-- Mobile hamburger -->
		<button
			onclick={toggleMobileSidebar}
			class="flex h-9 w-9 items-center justify-center rounded-lg transition-colors hover:opacity-80 lg:hidden"
			style="color: var(--color-text-muted);"
			aria-label="Toggle sidebar"
		>
			<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
				<line x1="3" y1="6" x2="21" y2="6"></line>
				<line x1="3" y1="12" x2="21" y2="12"></line>
				<line x1="3" y1="18" x2="21" y2="18"></line>
			</svg>
		</button>
		<h2 class="text-lg font-semibold" style="color: var(--color-text);">{title}</h2>
	</div>

	<div class="relative user-menu">
		<button
			onclick={toggleDropdown}
			class="flex items-center gap-2 rounded-lg px-3 py-1.5 transition-all hover:opacity-80"
			style="background-color: var(--color-primary-subtle);"
			aria-label="User menu"
		>
			<div
				class="flex h-7 w-7 items-center justify-center rounded-full"
				style="background-color: var(--color-primary);"
			>
				<span class="text-xs font-bold text-white">
					{$user?.name?.charAt(0)?.toUpperCase() || 'A'}
				</span>
			</div>
			<span class="text-sm font-medium hidden sm:inline" style="color: var(--color-primary);">
				{$user?.name || 'Admin'}
			</span>
			<!-- Chevron down -->
			<svg
				class="transition-transform duration-150 hidden sm:block"
				class:rotate-180={dropdownOpen}
				xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24"
				fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
				style="color: var(--color-text-muted);"
			>
				<polyline points="6 9 12 15 18 9"></polyline>
			</svg>
		</button>

		<!-- Dropdown -->
		{#if dropdownOpen}
			<div
				class="absolute right-0 top-full z-50 mt-1 w-48 rounded-xl border py-1 shadow-lg animate-fade-in"
				style="background-color: var(--color-card); border-color: var(--color-border-light);"
			>
				<!-- User info header -->
				<div class="border-b px-4 py-2.5" style="border-color: var(--color-border-light);">
					<p class="text-sm font-medium" style="color: var(--color-text);">{$user?.name || 'User'}</p>
					<p class="text-xs" style="color: var(--color-text-muted);">{$user?.email || ''}</p>
				</div>

				<!-- Role badge -->
				<div class="px-4 py-2">
					<span
						class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium"
						style="background-color: var(--color-primary-subtle); color: var(--color-primary);"
					>
						<span class="h-1.5 w-1.5 rounded-full" style="background-color: var(--color-primary);"></span>
						{$user?.role || 'member'}
					</span>
				</div>

				<!-- Divider -->
				<div class="border-t" style="border-color: var(--color-border-light);"></div>

				<!-- Settings -->
				<a
					href="/settings"
					onclick={() => dropdownOpen = false}
					class="flex w-full items-center gap-2 px-4 py-2.5 text-sm transition-colors hover:opacity-80"
					style="color: var(--color-text);"
				>
					<Icon icon="solar:settings-bold" class="h-4 w-4" />
					Settings
				</a>

				<!-- Divider -->
				<div class="border-t" style="border-color: var(--color-border-light);"></div>

				<!-- Logout -->
				<button
					onclick={handleLogout}
					class="flex w-full items-center gap-2 px-4 py-2.5 text-sm transition-colors hover:opacity-80"
					style="color: var(--color-danger);"
				>
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path>
						<polyline points="16 17 21 12 16 7"></polyline>
						<line x1="21" y1="12" x2="9" y2="12"></line>
					</svg>
					Sign out
				</button>
			</div>
		{/if}
	</div>
</header>
