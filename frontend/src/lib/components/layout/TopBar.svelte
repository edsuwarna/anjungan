<script>
	import { user, sidebarCollapsed, currentProject } from '$lib/stores/auth.js';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';

	let dropdownOpen = $state(false);
	let projectDropdownOpen = $state(false);
	let projects = $state([]);

	const pageTitles = {
		'/': 'Overview',
		'/servers': 'Servers',
		'/containers': 'Containers',
		'/registry': 'Registry',
		'/ssl-monitors': 'SSL Monitors',
		'/admin/users': 'User Management'
	};

	let title = $derived(pageTitles[$page.url.pathname] || 'Anjungan');

	onMount(async () => {
		try {
			const data = await api.projects.list();
			projects = data?.projects || [];
		} catch (_) {
			projects = [];
		}
	});

	function toggleDropdown() {
		dropdownOpen = !dropdownOpen;
	}

	function toggleProjectDropdown() {
		projectDropdownOpen = !projectDropdownOpen;
	}

	function switchProject(project) {
		projectDropdownOpen = false;
		if (!project) {
			currentProject.set(null);
			goto('/');
		} else {
			currentProject.set(project);
			goto(`/projects/${project.slug}`);
		}
	}

	function handleLogout() {
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
		if (!e.target.closest('.project-switcher')) {
			projectDropdownOpen = false;
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

	<!-- Project Switcher -->
	<div class="relative project-switcher">
		<button
			onclick={toggleProjectDropdown}
			class="flex items-center gap-2 rounded-lg px-3 py-1.5 transition-all hover:opacity-80"
			style="background-color: var(--color-primary-subtle);"
			aria-label="Switch project"
		>
			<span class="text-sm font-medium" style="color: var(--color-primary);">
				{$currentProject?.name || 'All Projects'}
			</span>
			<svg
				class="transition-transform duration-150"
				class:rotate-180={projectDropdownOpen}
				xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24"
				fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
				style="color: var(--color-text-muted);"
			>
				<polyline points="6 9 12 15 18 9"></polyline>
			</svg>
		</button>

		{#if projectDropdownOpen}
			<div
				class="absolute left-0 top-full z-50 mt-1 w-56 rounded-xl border py-1 shadow-lg animate-fade-in"
				style="background-color: var(--color-card); border-color: var(--color-border-light);"
			>
				{#each projects as project}
					<button
						onclick={() => switchProject(project)}
						class="flex w-full items-center gap-3 px-4 py-2.5 text-sm transition-colors hover:opacity-80"
						style="color: var(--color-text);"
					>
						<span
							class="h-2.5 w-2.5 shrink-0 rounded-full"
							style="background-color: {project.color || '#6366f1'};"
						></span>
						<span class="flex-1 text-left">{project.name}</span>
						{#if $currentProject?.id === project.id}
							<Icon icon="solar:check-circle-bold" class="h-4 w-4" style="color: var(--color-primary);" />
						{/if}
					</button>
				{/each}

				{#if projects.length > 0}
					<div class="border-t" style="border-color: var(--color-border-light);"></div>
				{/if}

				{#if $user?.role === 'admin'}
					<button
						onclick={() => switchProject(null)}
						class="flex w-full items-center gap-3 px-4 py-2.5 text-sm transition-colors hover:opacity-80"
						style="color: var(--color-text);"
					>
						<span class="flex h-2.5 w-2.5 shrink-0 items-center justify-center rounded-full" style="background-color: var(--color-text-muted);">
						</span>
						<span class="flex-1 text-left">All Projects</span>
						{#if !$currentProject}
							<Icon icon="solar:check-circle-bold" class="h-4 w-4" style="color: var(--color-primary);" />
						{/if}
					</button>
				{/if}

				<div class="border-t" style="border-color: var(--color-border-light);"></div>

				{#if $user?.role === 'admin'}
					<a
						href="/admin/projects"
						onclick={() => projectDropdownOpen = false}
						class="flex w-full items-center gap-3 px-4 py-2.5 text-sm transition-colors hover:opacity-80"
						style="color: var(--color-text);"
					>
						<Icon icon="solar:folder-with-files-bold" class="h-4 w-4" style="color: var(--color-text-muted);" />
						<span>Manage Projects</span>
					</a>
				{/if}
			</div>
		{/if}
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
