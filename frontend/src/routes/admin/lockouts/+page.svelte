<script>
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';

	let lockouts = $state([]);
	let loading = $state(true);
	let error = $state('');
	let unlocking = $state('');
	let now = $state(Date.now());

	let interval;

	onMount(() => {
		loadLockouts();
		// Update countdown every second
		interval = setInterval(() => {
			now = Date.now();
		}, 1000);

		// Auto-refresh every 30s to catch new lockouts / expirations
		const refreshInterval = setInterval(loadLockouts, 30000);
		interval = { tick: interval, refresh: refreshInterval };
	});

	onDestroy(() => {
		if (interval) {
			if (interval.tick) clearInterval(interval.tick);
			if (interval.refresh) clearInterval(interval.refresh);
		}
	});

	async function loadLockouts() {
		loading = true;
		error = '';
		try {
			lockouts = await api.authActivity.lockouts();
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function handleUnlock(lockout) {
		if (!lockout.user_id) {
			error = 'Cannot unlock: user ID not found';
			return;
		}
		unlocking = lockout.email;
		try {
			await api.admin.users.unlock(lockout.user_id);
			await loadLockouts();
		} catch (e) {
			error = e.message;
		} finally {
			unlocking = '';
		}
	}

	function formatRemaining(remainingSec) {
		if (!remainingSec || remainingSec <= 0) return 'Expired';
		const m = Math.floor(remainingSec / 60);
		const s = remainingSec % 60;
		if (m > 60) {
			const h = Math.floor(m / 60);
			return `${h}h ${m % 60}m remaining`;
		}
		return `${m}m ${s}s remaining`;
	}

	function lockedUntil(ts) {
		if (!ts) return '-';
		return new Date(ts).toLocaleString();
	}
</script>

<div class="page-container">
	<!-- Header -->
	<div class="flex items-center justify-between mb-6">
		<div>
			<h1 class="page-title">Active Lockouts</h1>
			<p class="page-subtitle">Currently locked-out user accounts with remaining lockout time</p>
		</div>
		<button onclick={loadLockouts} class="btn-ghost flex items-center gap-1.5 text-sm">
			<Icon icon="solar:refresh-bold" class="h-4 w-4" />
			Refresh
		</button>
	</div>

	<!-- Loading -->
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading lockouts...</p>
			</div>
		</div>
	{:else if error}
		<div class="card flex flex-col items-center gap-3 py-10 text-center" style="border-left: 3px solid var(--color-danger);">
			<Icon icon="solar:danger-triangle-bold" class="mb-1 h-8 w-8" style="color: var(--color-danger);" />
			<p style="color: var(--color-danger);">Failed to load lockouts</p>
			<p class="text-sm" style="color: var(--color-text-muted);">{error}</p>
			<button onclick={loadLockouts} class="btn-secondary mt-2">Retry</button>
		</div>
	{:else if lockouts.length === 0}
		<div class="card flex flex-col items-center py-16 text-center" style="border-left: 3px solid var(--color-success);">
			<Icon icon="solar:lock-unlocked-bold" class="mb-3 h-12 w-12" style="color: var(--color-success);" />
			<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No active lockouts</h3>
			<p class="text-sm" style="color: var(--color-text-secondary);">
				All user accounts are currently unlocked and can log in normally.
			</p>
		</div>
	{:else}
		<div class="data-table">
			<table class="w-full">
				<thead>
					<tr>
						<th>Email</th>
						<th>Locked Until</th>
						<th>Remaining Time</th>
						<th class="w-24">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each lockouts as l}
						<tr>
							<td class="font-mono text-xs max-w-[240px] truncate" title={l.email}>{l.email}</td>
							<td class="text-xs whitespace-nowrap" style="color: var(--color-text-secondary);">
								{lockedUntil(l.locked_until)}
							</td>
							<td>
								{#if l.remaining_sec > 0}
									<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
										style="background-color: {l.remaining_sec > 300 ? '#f59e0b15' : '#ef444415'}; color: {l.remaining_sec > 300 ? '#f59e0b' : '#ef4444'};">
										<Icon icon="solar:clock-bold" class="h-3 w-3" />
										{formatRemaining(l.remaining_sec)}
									</span>
								{/if}
							</td>
							<td>
								<button onclick={() => handleUnlock(l)}
									disabled={unlocking === l.email || !l.user_id}
									class="btn-ghost flex items-center gap-1.5 text-xs whitespace-nowrap"
									style="color: var(--color-success);"
									title={!l.user_id ? 'User not found' : 'Unlock this account'}>
									<Icon icon={unlocking === l.email ? 'solar:spinner-bold' : 'solar:lock-unlocked-bold'} class="h-3.5 w-3.5" />
									{unlocking === l.email ? 'Unlocking...' : 'Unlock'}
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		{#if lockouts.length > 0}
			<div class="mt-4 text-center">
				<p class="text-xs" style="color: var(--color-text-muted);">
					{lockouts.length} locked account{lockouts.length > 1 ? 's' : ''} — auto-refresh every 30s
				</p>
			</div>
		{/if}
	{/if}
</div>
