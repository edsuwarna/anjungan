<script>
	import '../app.css';
	import Sidebar from '$lib/components/layout/Sidebar.svelte';
	import TopBar from '$lib/components/layout/TopBar.svelte';
	import { theme, sidebarCollapsed, user } from '$lib/stores/auth.js';
	import { onMount } from 'svelte';

	onMount(() => {
		// Load theme
		const saved = localStorage.getItem('theme') || 'light';
		theme.set(saved);
		if (saved === 'dark') document.documentElement.classList.add('dark');
	});
</script>

<div class="flex h-screen overflow-hidden" class:dark={$theme === 'dark'}>
	<Sidebar />
	<div class="flex flex-1 flex-col overflow-hidden" class:ml-0={$sidebarCollapsed} class:ml-64={!$sidebarCollapsed}>
		<TopBar />
		<main class="flex-1 overflow-y-auto p-6 animate-fade-in">
			<slot />
		</main>
	</div>
</div>
