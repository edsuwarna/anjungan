<script>
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';

	let lockouts = $state([]);
	let loading = $state(true);
	let error = $state('');

	// Config
	let config = $state({ notification_target_ids: [], threshold: 20, window_minutes: 5 });
	let allTargets = $state([]);
	let configLoading = $state(true);
	let configSaving = $state(false);
	let configError = $state('');
	let configSaved = $state(false);
	let testing = $state(false);
	let testResult = $state('');
	let testError = $state('');

	// Unlock
	let unlocking = $state('');

	// Countdown tick
	let now = $state(Date.now());
	let interval;

	onMount(() => {
		loadLockouts();
		loadConfig();

		interval = setInterval(() => {
			now = Date.now();
		}, 1000);

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

	async function loadConfig() {
		configLoading = true;
		try {
			const [cfg, targets] = await Promise.all([
				api.authActivity.config(),
				api.notificationTargets.list(),
			]);
			config = { notification_target_ids: cfg?.notification_target_ids || [], threshold: cfg?.threshold || 20, window_minutes: cfg?.window_minutes || 5 };
			allTargets = targets || [];
		} catch (e) {
			configError = e.message;
		} finally {
			configLoading = false;
		}
	}

	async function saveConfig() {
		configSaving = true;
		configError = '';
		configSaved = false;
		try {
			await api.authActivity.updateConfig({
				notification_target_ids: config.notification_target_ids,
				threshold: config.threshold,
				window_minutes: config.window_minutes,
			});
			configSaved = true;
			setTimeout(() => configSaved = false, 3000);
		} catch (e) {
			configError = e.message;
		} finally {
			configSaving = false;
		}
	}

	async function handleTest() {
		const ids = config.notification_target_ids;
		if (ids.length === 0) return;
		testing = true;
		testResult = '';
		testError = '';
		let allOk = true;
		for (const id of ids) {
			try {
				const res = await api.notificationTargets.test(id, 'bruteforce');
				if (!res.success) {
					allOk = false;
					testError = (res.error || 'delivery failed');
				}
			} catch (e) {
				allOk = false;
				testError = e.message;
			}
		}
		testResult = allOk ? 'Test notification sent!' : 'Test failed';
		testing = false;
		setTimeout(() => { testResult = ''; testError = ''; }, 5000);
	}

	function toggleTarget(id) {
		const idx = config.notification_target_ids.indexOf(id);
		if (idx >= 0) {
			config.notification_target_ids = config.notification_target_ids.filter(x => x !== id);
		} else {
			config.notification_target_ids = [...config.notification_target_ids, id];
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
			<h1 class="page-title">Brute Force Lockouts</h1>
			<p class="page-subtitle">Active lockouts and brute force notification configuration</p>
		</div>
		<button onclick={loadLockouts} class="btn-ghost flex items-center gap-1.5 text-sm">
			<Icon icon="solar:refresh-bold" class="h-4 w-4" />
			Refresh
		</button>
	</div>

	<!-- ─── Notification Config ──────────────────────────────────── -->
	<div class="card mb-6">
		<div class="mb-3 flex items-center justify-between">
			<h3 class="text-sm font-semibold" style="color: var(--color-text);">
				<Icon icon="solar:bell-bold" class="inline h-4 w-4 mr-1.5" style="color: var(--color-primary);" />
				Brute Force Notifications
			</h3>
			{#if configSaved}
				<span class="text-xs font-medium" style="color: var(--color-success);">✓ Saved</span>
			{/if}
		</div>
		<p class="mb-4 text-xs" style="color: var(--color-text-muted);">
			Select notification targets to receive alerts when brute force attacks are detected.
		</p>

		{#if configLoading}
			<div class="flex items-center gap-2 py-3">
				<Icon icon="svg-spinners:180-ring" class="h-4 w-4" style="color: var(--color-primary);" />
				<span class="text-xs" style="color: var(--color-text-muted);">Loading config...</span>
			</div>
		{:else}
			<div class="flex flex-wrap gap-2 mb-4">
				{#if allTargets.length === 0}
					<p class="text-xs" style="color: var(--color-text-muted);">
						No notification targets configured.
						<a href="/notifications" class="underline" style="color: var(--color-primary);">Add one first</a>.
					</p>
				{:else}
					{#each allTargets as t (t.id)}
						<button
							class="config-chip"
							class:selected={config.notification_target_ids.includes(t.id)}
							onclick={() => toggleTarget(t.id)}
						>
							<Icon
								icon={config.notification_target_ids.includes(t.id)
									? 'solar:check-circle-bold'
									: 'solar:add-circle-bold'}
								class="h-3.5 w-3.5"
							/>
							{t.name}
						</button>
					{/each}
				{/if}
			</div>

			{#if configError}
				<p class="mb-3 text-xs" style="color: #ef4444;">{configError}</p>
			{/if}

			<div class="grid grid-cols-2 gap-4 mb-4">
				<div>
					<label class="block text-xs font-medium mb-1" style="color: var(--color-text-secondary);">Threshold (failures)</label>
					<input
						type="number"
						bind:value={config.threshold}
						min="1"
						max="999"
						class="w-full rounded-lg border px-3 py-2 text-sm"
						style="background: var(--color-card); color: var(--color-text); border-color: var(--color-border);"
					/>
					<p class="mt-1 text-xs" style="color: var(--color-text-muted);">Min failed attempts before alert</p>
				</div>
				<div>
					<label class="block text-xs font-medium mb-1" style="color: var(--color-text-secondary);">Window (minutes)</label>
					<input
						type="number"
						bind:value={config.window_minutes}
						min="1"
						max="1440"
						class="w-full rounded-lg border px-3 py-2 text-sm"
						style="background: var(--color-card); color: var(--color-text); border-color: var(--color-border);"
					/>
					<p class="mt-1 text-xs" style="color: var(--color-text-muted);">Time window to count failures</p>
				</div>
			</div>

			<div class="flex items-center gap-3">
				<button
					class="btn-primary text-xs"
					onclick={saveConfig}
					disabled={configSaving}
				>
					<Icon icon={configSaving ? 'svg-spinners:180-ring' : 'solar:diskette-bold'} class="h-3.5 w-3.5" />
					{configSaving ? 'Saving...' : 'Save Configuration'}
				</button>

				<button
					class="btn-secondary text-xs"
					onclick={handleTest}
					disabled={testing || config.notification_target_ids.length === 0}
				>
					<Icon icon={testing ? 'svg-spinners:180-ring' : 'solar:bell-bold'} class="h-3.5 w-3.5" />
					{testing ? 'Testing...' : 'Test Notification'}
				</button>

				{#if testResult && !testError}
					<span class="text-xs font-medium" style="color: var(--color-success);">{testResult}</span>
				{/if}
				{#if testError}
					<span class="text-xs font-medium" style="color: #ef4444;">{testError}</span>
				{/if}
			</div>
		{/if}
	</div>

	<!-- ─── Active Lockouts ──────────────────────────────────────── -->
	<h2 class="mb-3 text-sm font-semibold" style="color: var(--color-text);">
		<Icon icon="solar:lock-bold" class="inline h-4 w-4 mr-1.5" />
		Active Lockouts
	</h2>

	<!-- Loading -->
	{#if loading}
		<div class="flex items-center justify-center py-16">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="svg-spinners:180-ring" class="h-8 w-8" style="color: var(--color-primary);" />
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
		<div class="card flex flex-col items-center py-12 text-center" style="border-left: 3px solid var(--color-success);">
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
									<Icon icon={unlocking === l.email ? 'svg-spinners:180-ring' : 'solar:lock-unlocked-bold'} class="h-3.5 w-3.5" />
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

<style>
	.page-container {
		max-width: 900px;
		margin: 0 auto;
		padding: 1.5rem;
	}
	.card {
		border-radius: 12px;
		padding: 1.25rem;
		background: var(--color-card);
		border: 1px solid var(--color-border);
	}
	.data-table {
		border-radius: 12px;
		border: 1px solid var(--color-border);
		overflow: hidden;
		background: var(--color-card);
	}
	.data-table th {
		padding: 0.625rem 1rem;
		text-align: left;
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		background: var(--color-surface);
		color: var(--color-text-muted);
		border-bottom: 1px solid var(--color-border);
	}
	.data-table td {
		padding: 0.625rem 1rem;
		font-size: 0.875rem;
		border-bottom: 1px solid var(--color-border);
	}
	.data-table tr:last-child td {
		border-bottom: none;
	}
	.config-chip {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		padding: 0.375rem 0.75rem;
		border-radius: 8px;
		font-size: 0.75rem;
		font-weight: 500;
		border: 1px solid var(--color-border);
		background: var(--color-card);
		color: var(--color-text-secondary);
		cursor: pointer;
		transition: all 0.15s;
	}
	.config-chip:hover {
		border-color: var(--color-primary);
		color: var(--color-text);
	}
	.config-chip.selected {
		border-color: var(--color-primary);
		background: var(--color-primary-subtle);
		color: var(--color-primary);
	}
	.btn-primary {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		border-radius: 8px;
		padding: 0.5rem 1rem;
		font-size: 0.875rem;
		font-weight: 600;
		color: #fff;
		background: var(--color-primary);
		border: none;
		cursor: pointer;
		transition: opacity 0.15s;
	}
	.btn-primary:hover { opacity: 0.9; }
	.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
	.btn-secondary {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		border-radius: 8px;
		padding: 0.5rem 1rem;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--color-text);
		background: var(--color-card);
		border: 1px solid var(--color-border);
		cursor: pointer;
		transition: background 0.15s;
	}
	.btn-secondary:hover { background: var(--color-hover); }
	.btn-ghost {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		border: none;
		background: none;
		cursor: pointer;
		color: var(--color-text-secondary);
		padding: 0.375rem 0.5rem;
		border-radius: 6px;
	}
	.btn-ghost:hover { background: var(--color-hover); }
	:global(body.dark) .card { background: #1a1d23; border-color: rgba(148,163,184,0.08); }
	:global(body.dark) .data-table { background: #1a1d23; }
	:global(body.dark) .config-chip.selected { background: rgba(16,185,129,0.15); }
</style>
