<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';

	let targets = $state([]);
	let loading = $state(true);
	let error = $state('');

	// Modal
	let showModal = $state(false);
	let editTarget = $state(null);
	let formData = $state({ name: '', url: '', bot_token: '', chat_id: '', platform: 'generic', enabled: true });
	let saving = $state(false);
	let formError = $state('');
	let deleteConfirm = $state(null);

	// Test
	let testing = $state(null);
	let testResult = $state(null);

	onMount(loadTargets);

	function urlHostname(url) {
		try { return new URL(url).hostname; }
		catch { return url; }
	}

	async function loadTargets() {
		loading = true;
		try {
			targets = await api.notificationTargets.list() || [];
		} catch (e) {
			error = e.message;
			targets = [];
		} finally {
			loading = false;
		}
	}

	function resetForm() {
		formData = { name: '', url: '', bot_token: '', chat_id: '', platform: 'generic', enabled: true };
		editTarget = null;
		formError = '';
		deleteConfirm = null;
	}

	function openAdd() {
		resetForm();
		showModal = true;
	}

	function openEdit(t) {
		formData = {
			name: t.name || '',
			url: t.url || '',
			bot_token: t.bot_token || '',
			chat_id: t.chat_id || '',
			platform: t.platform || 'generic',
			enabled: t.enabled !== false,
		};
		editTarget = t;
		formError = '';
		deleteConfirm = null;
		showModal = true;
	}

	async function handleSave(e) {
		e.preventDefault();
		formError = '';
		if (!formData.name.trim()) { formError = 'Name is required.'; return; }
		if (formData.platform === 'telegram') {
			if (!formData.bot_token.trim()) { formError = 'Bot Token is required.'; return; }
			if (!formData.chat_id.trim()) { formError = 'Chat ID is required.'; return; }
		} else if (!formData.url.trim()) {
			formError = 'Webhook URL is required.'; return;
		}
		saving = true;
		try {
			if (editTarget) {
				await api.notificationTargets.update(editTarget.id, formData);
			} else {
				await api.notificationTargets.create(formData);
			}
			await loadTargets();
			showModal = false;
			resetForm();
		} catch (e) {
			formError = e.message || 'Failed to save.';
		} finally {
			saving = false;
		}
	}

	async function handleDelete(id) {
		try {
			await api.notificationTargets.delete(id);
			await loadTargets();
			deleteConfirm = null;
		} catch (e) {
			alert('Failed to delete: ' + e.message);
		}
	}

	async function handleTest(id) {
		testing = id;
		testResult = null;
		try {
			const result = await api.notificationTargets.test(id);
			testResult = { id, ...result };
		} catch (e) {
			testResult = { id, success: false, error: e.message || 'Test request failed' };
		} finally {
			testing = false;
		}
	}

	function platformIcon(platform) {
		switch (platform) {
			case 'telegram': return 'solar:telegram-bold';
			case 'discord': return 'solar:discord-bold';
			case 'slack': return 'solar:slack-bold';
			default: return 'solar:link-bold';
		}
	}

	// ── Derived Stats ──
	let totalTargets = $derived(targets.length);
</script>

<div class="page-container">
	<!-- Page Header -->
	<div class="mb-6 flex flex-wrap items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold" style="color: var(--color-text);">Notification Targets</h1>
			<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
				Shared notification channels for SSL monitoring and uptime alerts
			</p>
		</div>
		<div class="flex items-center gap-3">
			<button class="btn-primary" onclick={openAdd}>
				<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
				Add Target
			</button>
		</div>
	</div>

	<!-- Total Targets Count -->
	{#if targets.length > 0}
	<div class="flex flex-wrap items-center gap-3 mb-5 p-3 rounded-xl" style="background-color: var(--color-surface); border: 1px solid var(--color-border);">
		<div class="flex items-center gap-1.5">
			<Icon icon="solar:bell-bold" class="h-4 w-4" style="color: var(--color-primary);" />
			<span class="text-sm font-bold" style="color: var(--color-text);">{totalTargets}</span>
			<span class="text-xs" style="color: var(--color-text-muted);">targets</span>
		</div>
	</div>
	{/if}

	<!-- Loading -->
	{#if loading}
		<div class="flex items-center justify-center py-16">
			<Icon icon="svg-spinners:180-ring" class="h-8 w-8" style="color: var(--color-primary);" />
		</div>
	{:else if error}
		<div class="card p-6 text-center" style="border-left: 3px solid var(--color-accent);">
			<p style="color: var(--color-error);">{error}</p>
			<button class="btn-secondary mt-3" onclick={loadTargets}>Retry</button>
		</div>
	{:else if targets.length === 0}
		<div class="card p-12 text-center" style="border-left: 3px solid var(--color-accent);">
			<Icon icon="solar:bell-bold" class="mx-auto h-12 w-12" style="color: var(--color-text-muted);" />
			<p class="mt-4 text-lg font-medium" style="color: var(--color-text);">No notification targets</p>
			<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
				Add webhooks to receive alerts from SSL monitoring and uptime checks.
			</p>
			<button class="btn-primary mt-4" onclick={openAdd}>Add Target</button>
		</div>
	{:else}
		<!-- Targets List -->
		<div class="space-y-3 max-w-2xl">
			{#each targets as t (t.id)}
				<div
					class="card"
					style="display: flex; align-items: center; border-left: 3px solid var(--color-accent);"
				>
					<div class="flex items-center gap-3 min-w-0 flex-1">
						<div class="flex h-10 w-10 items-center justify-center rounded-full" style="background: var(--color-primary-subtle);">
							<Icon icon={platformIcon(t.platform)} class="h-5 w-5" style="color: var(--color-primary);" />
						</div>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2">
								<p class="font-medium text-sm" style="color: var(--color-text);">{t.name}</p>
								{#if t.enabled}
									<span class="inline-flex h-2 w-2 rounded-full bg-emerald-500"></span>
								{:else}
									<span class="inline-flex h-2 w-2 rounded-full" style="background: var(--color-text-muted);"></span>
								{/if}
							</div>
							{#if t.platform === 'telegram'}
								<p class="truncate text-xs font-mono" style="color: var(--color-text-muted);" title={`Bot: ${t.bot_token} | Chat: ${t.chat_id}`}>
									Bot: {t.bot_token?.slice(0, 20)}... | Chat: {t.chat_id}
								</p>
							{:else}
								<p class="truncate text-xs font-mono" style="color: var(--color-text-muted);" title={t.url}>{t.url}</p>
							{/if}
							<div class="mt-1 flex flex-wrap gap-1.5">
								<!-- Enabled badge -->
								<span
									class="scope-chip"
									class:active={t.enabled !== false}
								>
									{t.enabled !== false ? 'Active' : 'Inactive'}
								</span>
							</div>
						</div>
					</div>
					<div class="flex items-center gap-2 ml-4 shrink-0">
						<button
							class="btn-sm"
							onclick={() => handleTest(t.id)}
							disabled={testing === t.id}
							title="Test notification"
						>
							<Icon icon={testing === t.id ? 'svg-spinners:180-ring' : 'solar:play-circle-bold'} class="h-3.5 w-3.5" />
							Test
						</button>
						<button
							class="btn-icon-sm"
							onclick={() => openEdit(t)}
							title="Edit"
						>
							<Icon icon="solar:pen-bold" class="h-3.5 w-3.5" />
						</button>
						<button
							class="btn-icon-sm"
							style="color: #ef4444;"
							onclick={() => { if (confirm('Delete this target?')) handleDelete(t.id); }}
							title="Delete"
						>
							<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
						</button>
					</div>
				</div>

				<!-- Test result inline -->
				{#if testResult?.id === t.id}
					<div class="test-result" class:success={testResult.success}>
						<div class="flex items-center gap-2">
							<Icon icon={testResult.success ? 'solar:check-circle-bold' : 'solar:danger-circle-bold'} class="h-4 w-4" />
							<span>{testResult.success ? 'Notification sent! Check your channel.' : 'Test failed'}</span>
							<button class="ml-auto btn-icon-sm" onclick={() => testResult = null}>
								<Icon icon="solar:close-circle-bold" class="h-4 w-4" />
							</button>
						</div>
						{#if !testResult.success && testResult.error}
							<p class="mt-1 text-xs">{testResult.error}</p>
						{/if}
					</div>
				{/if}
			{/each}
		</div>
	{/if}
</div>

<!-- Add/Edit Modal -->
{#if showModal}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div class="modal-overlay" onclick={() => { if (!saving) { showModal = false; resetForm(); } }}>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div class="modal-panel" onclick={(e) => e.stopPropagation()}>
			<div class="flex items-center justify-between border-b pb-4" style="border-color: var(--color-border);">
				<div>
					<h2 class="text-lg font-bold" style="color: var(--color-text);">
						{editTarget ? 'Edit' : 'Add'} Notification Target
					</h2>
					<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
						{editTarget ? 'Update webhook configuration' : 'Add a webhook for alerts'}
					</p>
				</div>
				<button class="btn-icon" onclick={() => { showModal = false; resetForm(); }}>
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>

			<form onsubmit={handleSave} class="mt-5">
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Name *</label>
					<input type="text" bind:value={formData.name} placeholder="e.g. Slack Ops Alerts" class="input w-full" required />
				</div>
				<div class="mb-4 grid grid-cols-2 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Platform</label>
						<select bind:value={formData.platform} class="input w-full">
							<option value="generic">Generic Webhook</option>
							<option value="telegram">Telegram</option>
							<option value="discord">Discord</option>
							<option value="slack">Slack</option>
						</select>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Enabled</label>
						<div class="mt-2 flex items-center gap-2">
							<button type="button" role="switch" aria-checked={formData.enabled}
								onclick={() => formData.enabled = !formData.enabled}
								class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors"
								style={formData.enabled ? 'background-color: var(--color-primary);' : 'background-color: var(--color-border);'}>
								<span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform"
									class:translate-x-[18px]={formData.enabled}
									class:translate-x-[1px]={!formData.enabled} />
							</button>
							<span class="text-sm" style="color: var(--color-text);">{formData.enabled ? 'Active' : 'Inactive'}</span>
						</div>
					</div>
				</div>
				{#if formData.platform === 'telegram'}
					<div class="mb-4">
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Bot Token *</label>
						<input type="text" bind:value={formData.bot_token} placeholder="1234567890:ABCdefGHIjklmNOPqrSTUvWXZ" class="input w-full" required />
						<p class="mt-1 text-xs" style="color: var(--color-text-muted);">From @BotFather — token format: 123456:ABC-def</p>
					</div>
					<div class="mb-4">
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Chat / Group / Channel ID *</label>
						<input type="text" bind:value={formData.chat_id} placeholder="-1001234567890" class="input w-full" required />
						<p class="mt-1 text-xs" style="color: var(--color-text-muted);">Positive for user, negative starting with -100 for group/channel</p>
					</div>
				{:else}
					<div class="mb-4">
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Webhook URL *</label>
						<input type="url" bind:value={formData.url} placeholder="https://hooks.example.com/..." class="input w-full" required />
					</div>
				{/if}

				{#if formError}
					<p class="mb-4 text-sm" style="color: #ef4444;">{formError}</p>
				{/if}

				<div class="flex items-center justify-between gap-3 pt-4">
					<div>
						{#if editTarget && !deleteConfirm}
							<button type="button" class="btn-ghost text-sm" style="color:#ef4444;" onclick={() => deleteConfirm = editTarget.id}>
								<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
								Delete
							</button>
						{/if}
						{#if deleteConfirm}
							<div class="flex items-center gap-2 text-sm">
								<span style="color: var(--color-text-secondary);">Delete?</span>
								<button type="button" class="btn-secondary px-3 py-1 text-xs" onclick={() => handleDelete(deleteConfirm)}>Yes</button>
								<button type="button" class="btn-ghost text-xs" onclick={() => deleteConfirm = null}>No</button>
							</div>
						{/if}
					</div>
					<div class="flex items-center gap-3">
						<button type="button" class="btn-secondary" onclick={() => { showModal = false; resetForm(); }}>
							{editTarget ? 'Cancel' : 'Close'}
						</button>
						<button type="submit" class="btn-primary" disabled={saving}>
							<Icon icon={saving ? 'svg-spinners:180-ring' : 'solar:check-circle-bold'} class="h-4 w-4" />
							{saving ? 'Saving...' : editTarget ? 'Update' : 'Add Target'}
						</button>
					</div>
				</div>
			</form>
		</div>
	</div>
{/if}

<style>
	.page-container {
		max-width: 960px;
		margin: 0 auto;
		padding: 1.5rem;
	}
	.card {
		border-radius: 12px;
		padding: 1.25rem;
		background: var(--color-card);
		border: 1px solid var(--color-border);
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
	.btn-secondary:disabled { opacity: 0.5; cursor: not-allowed; }
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
	.btn-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 2rem;
		height: 2rem;
		border-radius: 6px;
		border: none;
		background: transparent;
		cursor: pointer;
		color: var(--color-text-secondary);
		transition: background 0.15s;
	}
	.btn-icon:hover { background: var(--color-hover); }
	.btn-icon-sm {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.75rem;
		height: 1.75rem;
		border-radius: 6px;
		border: none;
		background: transparent;
		cursor: pointer;
		color: var(--color-text-secondary);
		transition: background 0.15s;
	}
	.btn-icon-sm:hover { background: var(--color-hover); }
	.btn-sm {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		border-radius: 6px;
		padding: 0.25rem 0.625rem;
		font-size: 0.75rem;
		font-weight: 500;
		border: 1px solid var(--color-border);
		cursor: pointer;
		background: var(--color-card);
		color: var(--color-text);
		transition: all 0.15s;
	}
	.btn-sm:hover { background: var(--color-hover); }
	.input {
		border-radius: 8px;
		padding: 0.5rem 0.75rem;
		font-size: 0.875rem;
		background: var(--color-bg);
		border: 1px solid var(--color-border);
		color: var(--color-text);
		outline: none;
		transition: border-color 0.15s;
	}
	.input:focus { border-color: var(--color-primary); }
	select.input {
		appearance: auto;
	}
	select.input option {
		color: #1e293b;
	}
	.target-card {
		display: flex;
		align-items: center;
		padding: 1rem 1.25rem;
		border-radius: 12px;
		background: var(--color-card);
		border: 1px solid var(--color-border);
		transition: all 0.15s;
	}
	.target-card:hover {
		border-color: var(--color-primary);
	}
	.scope-chip {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		padding: 1px 8px;
		border-radius: 10px;
		font-size: 0.6875rem;
		font-weight: 500;
		background: rgba(148,163,184,0.1);
		color: var(--color-text-muted);
	}
	.scope-chip.active {
		background: var(--color-primary-subtle);
		color: var(--color-primary);
	}
	.test-result {
		margin-top: -0.5rem;
		margin-bottom: 0.5rem;
		padding: 0.75rem;
		border-radius: 8px;
		font-size: 0.8125rem;
		border: 1px solid;
	}
	.test-result.success {
		border-color: rgba(16,185,129,0.2);
		background: rgba(16,185,129,0.08);
		color: var(--color-primary);
	}
	.test-result:not(.success) {
		border-color: rgba(239,68,68,0.2);
		background: rgba(239,68,68,0.08);
		color: #ef4444;
	}
	.modal-overlay {
		position: fixed;
		inset: 0;
		z-index: 50;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(0,0,0,0.5);
		padding: 1rem;
	}
	.modal-panel {
		background: var(--color-card);
		border-radius: 16px;
		width: 100%;
		max-width: 520px;
		max-height: 90vh;
		overflow-y: auto;
		padding: 1.5rem;
		border: 1px solid var(--color-border);
		box-shadow: 0 20px 60px rgba(0,0,0,0.2);
	}
	:global(body.dark) .card { background: #1a1d23; border-color: rgba(148,163,184,0.08); }
	:global(body.dark) .modal-panel { background: #1a1d23; }
	:global(body.dark) .scope-chip.active { background: rgba(16,185,129,0.15); }
</style>
