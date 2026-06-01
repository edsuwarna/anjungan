<script>
	import { sidebarCollapsed, theme } from '$lib/stores/auth.js';

	const navItems = [
		{ href: '/', icon: 'solar:chart-2-bold', label: 'Dashboard' },
		{ href: '/servers', icon: 'solar:server-square-bold', label: 'Servers' },
		{ href: '/containers', icon: 'solar:box-bold', label: 'Containers' },
		{ href: '/registry', icon: 'solar:archive-down-minimlistic-bold', label: 'Registry' },
		{ href: '/repositories', icon: 'solar:code-square-bold', label: 'Repositories' },
		{ href: '/deployments', icon: 'solar:rocket-bold', label: 'Deployments' },
		{ href: '/admin/users', icon: 'solar:shield-user-bold', label: 'Admin' }
	];

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
</script>

<!-- sidebar -->
<aside
	class="fixed left-0 top-0 z-40 flex h-screen flex-col border-r transition-all duration-200"
	style="background-color: var(--color-sidebar); border-color: var(--color-border); width: 256px;"
	class:-translate-x-full={$sidebarCollapsed}
>
	<!-- logo -->
	<div class="flex h-16 items-center gap-3 border-b px-6" style="border-color: var(--color-border);">
		<div class="flex h-8 w-8 items-center justify-center rounded-lg" style="background-color: var(--color-primary);">
			<span class="text-sm font-bold text-white">A</span>
		</div>
		<span class="text-lg font-semibold">Anjungan</span>
	</div>

	<!-- nav -->
	<nav class="flex-1 space-y-1 px-3 py-4">
		{#each navItems as item}
			<a
				{href}
				class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors"
				style="color: var(--color-text-secondary);"
				aria-current="page"
			>
				<span class="icon-[{$item.icon}] h-5 w-5"></span>
				{item.label}
			</a>
		{/each}
	</nav>

	<!-- footer -->
	<div class="border-t px-3 py-3" style="border-color: var(--color-border);">
		<button
			onclick={toggleTheme}
			class="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors hover:opacity-80"
			style="color: var(--color-text-secondary);"
		>
			<span class="icon-[{$theme === 'dark' ? 'solar:sun-bold' : 'solar:moon-bold'}] h-5 w-5"></span>
			{$theme === 'dark' ? 'Light Mode' : 'Dark Mode'}
		</button>
	</div>
</aside>

<!-- mobile overlay -->
{#if !$sidebarCollapsed}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		class="fixed inset-0 z-30 bg-black/50 lg:hidden"
		onclick={toggleSidebar}
	></div>
{/if}

<!-- toggle button (mobile) -->
<button
	onclick={toggleSidebar}
	class="fixed left-4 top-4 z-50 flex h-10 w-10 items-center justify-center rounded-lg border bg-white shadow-sm lg:hidden"
	style="border-color: var(--color-border);"
>
	<span class="icon-[solar:menu-dots-bold] h-5 w-5"></span>
</button>
