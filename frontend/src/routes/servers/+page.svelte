<script>
	import { api } from '$lib/api.js';
	import { onMount } from 'svelte';

	let servers = [];
	let loading = true;

	onMount(async () => {
		try {
			servers = await api.servers.list();
		} catch (e) {
			console.error(e);
		} finally {
			loading = false;
		}
	});
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-xl font-bold">Servers</h2>
		<button class="rounded-lg px-4 py-2 text-sm font-medium text-white" style="background-color: var(--color-primary);">
			+ Add Server
		</button>
	</div>

	<div class="rounded-xl border" style="background-color: var(--color-sidebar); border-color: var(--color-border);">
		{#if loading}
			<div class="p-6 text-center text-sm" style="color: var(--color-text-secondary);">Loading...</div>
		{:else if servers.length === 0}
			<div class="p-6 text-center text-sm" style="color: var(--color-text-secondary);">
				No servers yet. Add your first server to get started.
			</div>
		{:else}
			<!-- server list -->
		{/if}
	</div>
</div>
