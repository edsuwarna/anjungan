<script>
	import StatCard from '$lib/components/charts/StatCard.svelte';
	import { onMount } from 'svelte';

	let stats = { servers: 0, containers: 0, deployments: 0, alerts: 0 };

	onMount(async () => {
		// TODO: fetch from api.dashboard.summary()
	});
</script>

<div class="space-y-6">
	<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<StatCard title="Servers" value={stats.servers} icon="solar:server-square-bold" color="#10b981" />
		<StatCard title="Containers" value={stats.containers} icon="solar:box-bold" color="#6366f1" />
		<StatCard title="Deployments" value={stats.deployments} icon="solar:rocket-bold" color="#f59e0b" />
		<StatCard title="Alerts" value={stats.alerts} icon="solar:bell-bold" color="#ef4444" />
	</div>

	<div class="rounded-xl border p-6" style="background-color: var(--color-sidebar); border-color: var(--color-border);">
		<h3 class="mb-4 text-base font-semibold">Recent Activity</h3>
		<p class="text-sm" style="color: var(--color-text-secondary);">
			No recent activity yet. Start by adding a server.
		</p>
	</div>
</div>
