<script>
	import Icon from '@iconify/svelte';
	import '../app.css';
	import Sidebar from '$lib/components/layout/Sidebar.svelte';
	import TopBar from '$lib/components/layout/TopBar.svelte';
	import ToastContainer from '$lib/components/layout/ToastContainer.svelte';
	import { theme, sidebarCollapsed, user } from '$lib/stores/auth.js';
	import { addToast } from '$lib/stores/toasts.js';
	import { api } from '$lib/api.svelte.js';
	import { page } from '$app/stores';
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';

	let { children } = $props();

	let hasToken = $state(false);
	let checked = $state(false);
	let isMobile = $state(false);

	// Scan notification polling
	/** @type {Set<string>} */
	let knownScanIds = new Set();
	let scanPollInterval = null;

	onMount(() => {
		// Responsive: auto-collapse sidebar on mobile
		isMobile = window.innerWidth < 1024;
		if (isMobile) sidebarCollapsed.set(true);
		window.addEventListener('resize', () => {
			isMobile = window.innerWidth < 1024;
		});

		// Check for existing token in localStorage
		const token = localStorage.getItem('access_token');
		hasToken = !!token;

		// If no token and not on login page, redirect
		if (!token && $page.url.pathname !== '/login') {
			goto('/login');
		}

		// Restore user data from localStorage (persists across refresh)
		const savedUser = localStorage.getItem('user');
		if (savedUser && $user === null) {
			try {
				user.set(JSON.parse(savedUser));
			} catch (_) {
				localStorage.removeItem('user');
			}
		}

		// Load theme
		const saved = localStorage.getItem('theme') || 'light';
		theme.set(saved);
		if (saved === 'dark') document.documentElement.classList.add('dark');

		checked = true;

		// Start scan activity polling (only for authenticated users)
		async function pollScanActivity() {
			try {
				const data = await api.compliance.activeScans();
				if (!data) return;

				// Track new completed scans (appeared in recent)
				for (const item of data.recent || []) {
					if (!knownScanIds.has(item.id)) {
						knownScanIds.add(item.id);
						// Only notify for scans that completed recently (not already known running)
						if (item.status === 'completed') {
							addToast({
								type: 'success',
								title: `${item.server_name} — ${item.scan_type}`,
								message: `✅ Scan completed — Score: ${item.score ?? 'N/A'} | ${item.passed} passed, ${item.warnings} warnings, ${item.criticals} criticals`,
							});
						} else if (item.status === 'failed') {
							addToast({
								type: 'error',
								title: `${item.server_name} — ${item.scan_type}`,
								message: `❌ Scan failed`,
							});
						}
					}
				}

				// Track running scans
				for (const item of data.running || []) {
					knownScanIds.add(item.id);
				}
			} catch (_) {
				// Silently fail — polling is best-effort
			}
		}

		// Only start polling if user is authenticated and NOT on login page
		if (hasToken && $page.url.pathname !== '/login') {
			scanPollInterval = setInterval(pollScanActivity, 10000);
			// Initial run
			pollScanActivity();
		}
	});

	onDestroy(() => {
		if (scanPollInterval) {
			clearInterval(scanPollInterval);
			scanPollInterval = null;
		}
	});

	// Authenticated if user store has data OR localStorage has token
	let authed = $derived($user !== null || hasToken);
</script>

{#if !checked}
	<!-- Initial loading state -->
	<div class="flex min-h-screen items-center justify-center" style="background-color: var(--color-surface);">
		<div class="flex flex-col items-center gap-3">
			<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
		</div>
	</div>
{:else if !authed || $page.url.pathname === '/login'}
	<!-- Auth pages: full-screen, no sidebar -->
	<div class="flex min-h-screen" class:dark={$theme === 'dark'}>
		{@render children()}
	</div>
{:else}
	<!-- Dashboard layout: sidebar + topbar + content -->
	<div class="flex h-screen overflow-hidden" class:dark={$theme === 'dark'}>
		<Sidebar />
		<!-- On mobile, sidebar is fixed/overlay so no margin; on desktop, margin shifts content -->
		<div class="flex flex-1 flex-col overflow-hidden"
			class:lg:ml-64={!$sidebarCollapsed}
			class:ml-0={$sidebarCollapsed}
		>
			<TopBar />
			<main class="flex-1 overflow-y-auto p-4 sm:p-6 animate-fade-in">
				{@render children()}
			</main>
		</div>
	</div>
	<ToastContainer />
{/if}
