<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';
	import { onMount } from 'svelte';

	let keys = $state([]);
	let loading = $state(true);
	let error = $state('');
	let showModal = $state(false);
	let modalMode = $state('add'); // 'add' | 'edit'
	let editingKey = $state(null);

	// Form fields
	let keyName = $state('');
	let keyType = $state('ed25519');
	let privateKey = $state('');
	let publicKey = $state('');

	let totalServersUsing = $derived(keys.reduce((sum, k) => sum + (k.server_count || 0), 0));

	// Key type badge colors
	let keyTypeColor = $derived.by(() => (type) => {
		const colors = {
			'ed25519': { bg: 'rgba(16,185,129,0.12)', text: '#10b981' },
			'rsa': { bg: 'rgba(99,102,241,0.12)', text: '#6366f1' },
			'ecdsa': { bg: 'rgba(245,158,11,0.12)', text: '#f59e0b' },
		};
		return colors[type] || { bg: 'var(--color-primary-subtle)', text: 'var(--color-primary)' };
	});

	onMount(loadKeys);

	async function loadKeys() {
		loading = true;
		error = '';
		try {
			keys = await api.sshKeys.list();
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function openAdd() {
		modalMode = 'add';
		editingKey = null;
		keyName = '';
		keyType = 'ed25519';
		privateKey = '';
		publicKey = '';
		showModal = true;
	}

	function openEdit(k) {
		modalMode = 'edit';
		editingKey = k;
		keyName = k.name;
		keyType = k.key_type;
		privateKey = '';
		publicKey = k.public_key || '';
		showModal = true;
	}

	async function handleSave() {
		if (!keyName || !privateKey) return;
		error = '';
		try {
			const payload = { name: keyName, key_type: keyType, private_key: privateKey };
			if (publicKey) payload.public_key = publicKey;

			if (modalMode === 'edit' && editingKey) {
				await api.sshKeys.update(editingKey.id, payload);
			} else {
				await api.sshKeys.create(payload);
			}
			showModal = false;
			await loadKeys();
		} catch (e) {
			error = e.message;
		}
	}

	async function handleDelete(k) {
		if (!confirm(`Delete SSH key "${k.name}"? This cannot be undone.`)) return;
		if (k.server_count > 0 && !confirm(`This key is used by ${k.server_count} server(s). Servers using it will lose SSH access if no other key is configured. Delete anyway?`)) return;
		try {
			await api.sshKeys.delete(k.id);
			await loadKeys();
		} catch (e) {
			alert('Failed to delete: ' + e.message);
		}
	}

	function truncatedFingerprint(fp) {
		if (!fp) return '-';
		return fp.length > 24 ? fp.substring(0, 24) + '...' : fp;
	}
</script>

<div class="page-container">
	<!-- Header -->
	<div class="flex items-center justify-between mb-4">
		<div>
			<h1 class="page-title">SSH Keys</h1>
			<p class="page-subtitle">Manage saved SSH keys for server access</p>
		</div>
		<button onclick={openAdd} class="btn-primary flex items-center gap-2">
			<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
			Add SSH Key
		</button>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading...</p>
			</div>
		</div>
	{:else if error}
		<div class="rounded-xl border p-6 text-center" style="background-color: var(--color-card); border-color: var(--color-border-light);">
			<Icon icon="solar:danger-triangle-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-danger);" />
			<p class="text-sm font-medium" style="color: var(--color-danger);">Failed to load SSH keys</p>
			<p class="mt-1 text-xs" style="color: var(--color-text-muted);">{error}</p>
			<button onclick={loadKeys} class="btn-secondary mt-4">Retry</button>
		</div>
	{:else if keys.length === 0}
		<div class="flex flex-col items-center py-16 text-center">
			<Icon icon="solar:key-minimalistic-bold" class="mb-3 h-12 w-12" style="color: var(--color-text-muted);" />
			<h3 class="text-base font-semibold" style="color: var(--color-text);">No SSH Keys Yet</h3>
			<p class="mt-1 max-w-sm text-sm" style="color: var(--color-text-muted);">
				Save your SSH keys here so you can quickly assign them to servers without pasting the full key each time.
			</p>
			<button onclick={openAdd} class="btn-primary mt-4 flex items-center gap-2">
				<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
				Add Your First Key
			</button>
		</div>
	{:else}
		<!-- Stat bar -->
		<div class="flex items-center gap-4 mb-4 px-1">
			<span class="flex items-center gap-1.5 text-sm font-medium" style="color: var(--color-text);">
				<Icon icon="solar:key-minimalistic-bold" class="h-4 w-4" style="color: var(--color-primary);" />
				{keys.length} key{keys.length !== 1 ? 's' : ''}
			</span>
			<span class="flex items-center gap-1.5 text-sm" style="color: var(--color-text-secondary);">
				<Icon icon="solar:server-square-bold" class="h-4 w-4" />
				{totalServersUsing} server{totalServersUsing !== 1 ? 's' : ''} using keys
			</span>
		</div>

		<!-- Key Cards -->
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each keys as k}
				{@const badgeColor = keyTypeColor(k.key_type)}
				<div class="card" style="border-left: 3px solid var(--color-primary);">
					<!-- Top row -->
					<div class="flex items-start justify-between">
						<div class="flex items-center gap-3 min-w-0">
							<div class="flex h-10 w-10 items-center justify-center rounded-lg shrink-0" style="background-color: var(--color-primary-subtle);">
								<Icon icon="solar:key-minimalistic-bold" class="h-5 w-5" style="color: var(--color-primary);" />
							</div>
							<div class="min-w-0">
								<h3 class="text-sm font-semibold truncate" style="color: var(--color-text);">{k.name}</h3>
								<p class="text-xs font-mono truncate" style="color: var(--color-text-muted);">{truncatedFingerprint(k.fingerprint)}</p>
							</div>
						</div>
						<span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium shrink-0 ml-2" style="background-color: {badgeColor.bg}; color: {badgeColor.text};">
							{k.key_type}
						</span>
					</div>

					<!-- Meta row -->
					<div class="mt-3 flex items-center gap-4 text-xs" style="color: var(--color-text-muted);">
						<span class="flex items-center gap-1">
							<Icon icon="solar:server-square-bold" class="h-3.5 w-3.5" />
							{k.server_count || 0} server{k.server_count !== 1 ? 's' : ''}
						</span>
						<span class="flex items-center gap-1">
							<Icon icon="solar:calendar-bold" class="h-3.5 w-3.5" />
							{new Date(k.created_at).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })}
						</span>
					</div>

					<!-- Actions -->
					<div class="mt-3 pt-3 border-t flex items-center gap-2" style="border-color: var(--color-border);">
						<button onclick={() => openEdit(k)} class="btn-secondary text-xs flex items-center gap-1">
							<Icon icon="solar:pen-bold" class="h-3.5 w-3.5" />
							Edit
						</button>
						<button onclick={() => handleDelete(k)} class="btn-danger text-xs flex items-center gap-1">
							<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
							Delete
						</button>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Add/Edit Modal -->
{#if showModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,0.5);" onclick={() => showModal = false} role="presentation">
		<div class="w-full max-w-lg rounded-xl border shadow-xl" style="background-color: var(--color-card); border-color: var(--color-border);" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
			<!-- Header -->
			<div class="flex items-center justify-between border-b px-6 py-4" style="border-color: var(--color-border);">
				<h2 class="text-lg font-semibold" style="color: var(--color-text);">
					{modalMode === 'edit' ? 'Edit SSH Key' : 'Add SSH Key'}
				</h2>
				<button onclick={() => showModal = false} class="rounded-lg p-1.5 transition-colors hover:opacity-80" style="color: var(--color-text-muted);" aria-label="Close">
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>

			<!-- Body -->
			<div class="space-y-4 px-6 py-4">
				{#if error}
					<div class="rounded-lg border px-4 py-3 text-sm" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2); color: var(--color-danger);">
						{error}
					</div>
				{/if}

				<!-- Name -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="key-name">Key Name *</label>
					<input id="key-name" bind:value={keyName} placeholder="my-deploy-key" class="input w-full" />
				</div>

				<!-- Key Type -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="key-type">Key Type</label>
					<select id="key-type" bind:value={keyType} class="input w-full">
						<option value="ed25519">Ed25519</option>
						<option value="rsa">RSA</option>
						<option value="ecdsa">ECDSA</option>
					</select>
				</div>

				<!-- Private Key -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="key-private">Private Key *</label>
					<textarea
						id="key-private"
						bind:value={privateKey}
						placeholder="-----BEGIN OPENSSH PRIVATE KEY-----
..."
						class="input w-full font-mono text-xs"
						rows="8"
					></textarea>
					<p class="mt-1 text-xs" style="color: var(--color-text-muted);">
						{modalMode === 'edit' ? 'Leave empty to keep existing key. Fill to replace.' : 'Paste your private key (PEM format)'}
					</p>
				</div>

				<!-- Public Key (optional) -->
				<details class="rounded-lg border" style="border-color: var(--color-border);">
					<summary class="cursor-pointer px-4 py-2.5 text-sm font-medium select-none" style="color: var(--color-text-secondary);">
						Public Key (optional)
					</summary>
					<div class="border-t px-4 py-3" style="border-color: var(--color-border);">
						<textarea
							id="key-public"
							bind:value={publicKey}
							placeholder="ssh-ed25519 AAAAC3... user@host"
							class="input w-full font-mono text-xs"
							rows="3"
						></textarea>
						<p class="mt-1 text-xs" style="color: var(--color-text-muted);">Paste the public key if available for fingerprint verification</p>
					</div>
				</details>
			</div>

			<!-- Footer -->
			<div class="flex items-center justify-end gap-3 border-t px-6 py-4" style="border-color: var(--color-border);">
				<button onclick={() => showModal = false} class="btn-secondary">Cancel</button>
				<button onclick={handleSave} disabled={!keyName || !privateKey} class="btn-primary flex items-center gap-2">
					<Icon icon="solar:diskette-bold" class="h-4 w-4" />
					{modalMode === 'edit' ? 'Update Key' : 'Save Key'}
				</button>
			</div>
		</div>
	</div>
{/if}
