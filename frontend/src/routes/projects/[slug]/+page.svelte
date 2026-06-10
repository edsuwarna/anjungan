<script>
	import Icon from '@iconify/svelte';
	import StatCard from '$lib/components/charts/StatCard.svelte';
	import { api } from '$lib/api.svelte.js';
	import { currentProject } from '$lib/stores/auth.js';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let project = $state(null);
	let loading = $state(true);
	let error = $state('');

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
		<!-- Breadcrumb -->
		<nav class="breadcrumb">
			<a href="/">Dashboard</a>
			<span class="crumb-sep">›</span>
			<span class="current">{project.name}</span>
		</nav>

		<!-- Header -->
		<div class="flex items-start justify-between flex-wrap gap-3 mb-6">
			<div>
				<h1 class="page-title">{project.name}</h1>
				{#if project.description}
					<p class="page-subtitle mt-1">{project.description}</p>
				{/if}
			</div>
			<button
				onclick={() => goto(`/projects/${slug}/settings`)}
				class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors hover:opacity-80"
				style="background-color: var(--color-primary-subtle); color: var(--color-primary);"
				title="Project Settings"
			>
				<Icon icon="solar:settings-bold" class="h-4 w-4" />
				<span class="hidden sm:inline">Settings</span>
			</button>
		</div>

		<!-- KPI Cards -->
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
			<StatCard
				title="Servers"
				value={project.resource_count?.servers ?? 0}
				icon="solar:server-square-bold"
			/>
			<StatCard
				title="SSL Monitors"
				value={project.resource_count?.ssl_monitors ?? 0}
				icon="solar:shield-check-bold"
			/>
			<StatCard
				title="Uptime Monitors"
				value={project.resource_count?.uptime_monitors ?? 0}
				icon="solar:chart-2-bold"
			/>
			<StatCard
				title="Deployments"
				value={project.resource_count?.deployments ?? 0}
				icon="solar:rocket-bold"
			/>
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
