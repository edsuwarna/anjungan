<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';
	import { currentProject } from '$lib/stores/auth.js';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let project = $state(null);
	let loading = $state(true);
	let error = '';

	onMount(async () => {
		await loadProject();
	});

	async function loadProject() {
		loading = true;
		error = '';
		try {
			const data = await api.projects.list();
			const projects = data?.projects || [];
			const found = projects.find(p => p.slug === $page.params.slug);
			if (!found) {
				error = `Project "${$page.params.slug}" not found`;
				project = null;
				currentProject.set(null);
			} else {
				project = found;
				currentProject.set(found);
			}
		} catch (e) {
			error = e.message || 'Failed to load project';
			project = null;
		} finally {
			loading = false;
		}
	}

	const slug = $derived($page.params.slug);
</script>

<div class="page-container">
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading project...</p>
			</div>
		</div>
	{:else if error}
		<div class="card text-center py-12">
			<Icon icon="solar:danger-triangle-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-danger);" />
			<p class="text-sm font-medium" style="color: var(--color-danger);">{error}</p>
			<button onclick={() => goto('/')} class="btn-secondary mt-4 text-sm">
				← Back to Dashboard
			</button>
		</div>
	{:else if project}
		<nav class="breadcrumb">
			<a href="/">Dashboard</a>
			<span class="crumb-sep">›</span>
			<a href="/projects/{slug}">{project.name}</a>
			<span class="crumb-sep">›</span>
			<span class="current">Registry</span>
		</nav>

		<div class="flex items-start justify-between flex-wrap gap-3 mb-6">
			<div>
				<h1 class="page-title">Registry</h1>
				<p class="page-subtitle mt-1">Container registry scoped to {project.name}</p>
			</div>
		</div>

		<div class="rounded-xl border px-6 py-12 text-center" style="background-color: var(--color-card); border-color: var(--color-border-light);">
			<Icon icon="solar:archive-down-minimlistic-bold" class="mb-3 inline-block h-12 w-12" style="color: var(--color-text-muted);" />
			<h3 class="mb-2 text-base font-semibold" style="color: var(--color-text);">No Registry Images Yet</h3>
			<p class="mx-auto max-w-md text-sm" style="color: var(--color-text-secondary);">
				This project doesn't have any registry images yet. Push an image to get started.
			</p>
			<button onclick={() => goto('/registry')} class="btn-primary mt-6 inline-flex items-center gap-2 text-sm">
				<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
				Browse Registry
			</button>
		</div>
	{/if}
</div>

<style>
	.breadcrumb {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-bottom: 16px;
		font-size: 12px;
	}
	.breadcrumb a {
		color: var(--color-text-muted);
		text-decoration: none;
		transition: color 0.15s;
	}
	.breadcrumb a:hover {
		color: var(--color-primary);
	}
	.crumb-sep {
		color: var(--color-text-muted);
		font-size: 10px;
	}
	.breadcrumb .current {
		color: var(--color-text-secondary);
		font-weight: 500;
	}
</style>
