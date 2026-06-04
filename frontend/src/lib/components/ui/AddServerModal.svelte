<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';

	let { show = false, onClose = () => {}, onSaved = () => {}, editServer = null } = $props();

	// Form state
	let name = $state('');
	let host = $state('');
	let port = $state(22);
	let sshUser = $state('root');
	let authType = $state('key');
	let sshKeyID = $state('');
	let sshKey = $state('');
	let sshPassword = $state('');
	let tags = $state('');
	let serverGroup = $state('');
	let region = $state('');
	let serverType = $state('');
	let description = $state('');

	// SSH key saved keys
	let savedKeys = $state([]);
	let loadingKeys = $state(false);
	let useSavedKey = $state(false);

	// UI state
	let saving = $state(false);
	let testing = $state(false);
	let testResult = $state(null);
	let error = $state('');

	let isEdit = $derived(editServer !== null);

	$effect(() => {
		if (editServer) {
			name = editServer.name || '';
			host = editServer.host || '';
			port = editServer.port || 22;
			sshUser = editServer.ssh_user || 'root';
			authType = editServer.ssh_auth_type || 'key';
			sshKeyID = editServer.ssh_key_id || '';
			sshKey = '';
			sshPassword = '';
			useSavedKey = !!editServer.ssh_key_id;
			tags = (editServer.tags || []).join(', ');
			serverGroup = editServer.server_group || '';
			region = editServer.region || '';
			serverType = editServer.server_type || '';
			description = editServer.description || '';
		}
	});

	$effect(() => {
		if (show && authType === 'key') {
			loadSavedKeys();
		}
	});

	async function loadSavedKeys() {
		loadingKeys = true;
		try {
			savedKeys = await api.sshKeys.list();
		} catch (_) {
			savedKeys = [];
		} finally {
			loadingKeys = false;
		}
	}

	function resetForm() {
		name = ''; host = ''; port = 22; sshUser = 'root';
		authType = 'key'; sshKeyID = ''; sshKey = ''; sshPassword = '';
		useSavedKey = false;
		tags = ''; serverGroup = ''; region = ''; serverType = ''; description = '';
		testResult = null; error = '';
	}

	function handleClose() {
		resetForm();
		onClose();
	}

	function handleKeySelection() {
		useSavedKey = true;
		sshKey = ''; // Clear direct key when using saved
	}

	function handleDirectKey() {
		useSavedKey = false;
		sshKeyID = ''; // Clear key ID when using direct
	}

	async function handleTest() {
		if (!host) return;
		testing = true;
		testResult = null;
		error = '';
		try {
			testResult = await api.servers.testConnection({
				host, port, ssh_user: sshUser,
				ssh_auth_type: authType,
				ssh_key: useSavedKey ? '' : sshKey,
				ssh_key_id: useSavedKey ? sshKeyID : '',
				ssh_password: sshPassword
			});
		} catch (e) {
			testResult = { reachable: false, hostname: '', error: e.message };
		} finally {
			testing = false;
		}
	}

	function parseTags(str) {
		return str.split(',').map(t => t.trim()).filter(t => t.length > 0);
	}

	async function handleSave() {
		if (!name || !host) {
			error = 'Name and host are required';
			return;
		}
		saving = true;
		error = '';
		try {
			const payload = {
				name, host, port, ssh_user: sshUser,
				ssh_auth_type: authType,
				ssh_key_id: useSavedKey ? sshKeyID : '',
				ssh_key: useSavedKey ? '' : sshKey,
				ssh_password: sshPassword,
				tags: parseTags(tags),
				server_group: serverGroup,
				region, server_type: serverType,
				description
			};
			let result;
			if (isEdit) {
				result = await api.servers.update(editServer.id, payload);
			} else {
				result = await api.servers.create(payload);
			}
			onSaved(result);
			handleClose();
		} catch (e) {
			error = e.message;
		} finally {
			saving = false;
		}
	}

	function handleKeydown(e) {
		if (e.key === 'Escape') handleClose();
	}

	function truncatedFingerprint(fp) {
		if (!fp) return '';
		return fp.length > 28 ? fp.substring(0, 28) + '...' : fp;
	}
</script>

{#if show}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background-color: rgba(0,0,0,0.5);"
		onclick={handleClose}
		onkeydown={handleKeydown}
		role="presentation"
	>
		<!-- svelte-ignore a11y_interactive_supports_focus -->
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div
			class="w-full max-w-lg rounded-xl border shadow-xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-label="Add Server"
		>
			<!-- Header -->
			<div class="flex items-center justify-between border-b px-6 py-4" style="border-color: var(--color-border);">
				<h2 class="text-lg font-semibold" style="color: var(--color-text);">{isEdit ? 'Edit Server' : 'Add Server'}</h2>
				<button
					onclick={handleClose}
					class="rounded-lg p-1.5 transition-colors hover:opacity-80"
					style="color: var(--color-text-muted);"
					aria-label="Close"
				>
					<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
				</button>
			</div>

			<!-- Body -->
			<div class="space-y-4 px-6 py-4 max-h-[65vh] overflow-y-auto">
				{#if error}
					<div class="rounded-lg border px-4 py-3 text-sm" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2); color: var(--color-danger);">
						{error}
					</div>
				{/if}

				<!-- Name -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="srv-name">Name *</label>
					<input id="srv-name" bind:value={name} placeholder="my-server" class="input w-full" />
				</div>

				<!-- Host & Port -->
				<div class="grid grid-cols-3 gap-3">
					<div class="col-span-2">
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="srv-host">Host *</label>
						<input id="srv-host" bind:value={host} placeholder="192.168.1.100" class="input w-full font-mono text-sm" />
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="srv-port">Port</label>
						<input id="srv-port" type="number" bind:value={port} placeholder="22" class="input w-full" min="1" max="65535" />
					</div>
				</div>

				<!-- SSH User -->
				<div>
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="srv-ssh-user">SSH User</label>
					<input id="srv-ssh-user" bind:value={sshUser} placeholder="root" class="input w-full" />
				</div>

				<!-- Auth Type Toggle -->
				<div>
					<span class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Authentication</span>
					<div class="flex gap-2" role="radiogroup">
						<button
							onclick={() => { authType = 'key'; sshPassword = ''; }}
							class="flex-1 rounded-lg border px-4 py-2 text-sm font-medium transition-all"
							role="radio"
							aria-checked={authType === 'key'}
							style="background-color: {authType === 'key' ? 'var(--color-primary-subtle)' : 'transparent'}; border-color: {authType === 'key' ? 'var(--color-primary)' : 'var(--color-border)'}; color: var(--color-text);"
						>
							SSH Key
						</button>
						<button
							onclick={() => { authType = 'password'; sshKey = ''; sshKeyID = ''; useSavedKey = false; }}
							class="flex-1 rounded-lg border px-4 py-2 text-sm font-medium transition-all"
							role="radio"
							aria-checked={authType === 'password'}
							style="background-color: {authType === 'password' ? 'var(--color-primary-subtle)' : 'transparent'}; border-color: {authType === 'password' ? 'var(--color-primary)' : 'var(--color-border)'}; color: var(--color-text);"
						>
							Password
						</button>
					</div>
				</div>

				<!-- SSH Key: Saved key selector -->
				{#if authType === 'key'}
					<div class="space-y-3">
						<!-- Toggle between saved key and paste -->
						<div class="flex gap-2">
							<button
								onclick={handleKeySelection}
								class="flex-1 rounded-lg border px-3 py-1.5 text-xs font-medium transition-all"
								style="background-color: {useSavedKey ? 'var(--color-primary-subtle)' : 'transparent'}; border-color: {useSavedKey ? 'var(--color-primary)' : 'var(--color-border)'}; color: var(--color-text);"
							>
								Use Saved Key
							</button>
							<button
								onclick={handleDirectKey}
								class="flex-1 rounded-lg border px-3 py-1.5 text-xs font-medium transition-all"
								style="background-color: {!useSavedKey ? 'var(--color-primary-subtle)' : 'transparent'}; border-color: {!useSavedKey ? 'var(--color-primary)' : 'var(--color-border)'}; color: var(--color-text);"
							>
								Paste Key
							</button>
						</div>

						{#if useSavedKey}
							<!-- Saved key dropdown -->
							<div>
								<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-muted);" for="srv-saved-key">Saved SSH Key</label>
								{#if loadingKeys}
									<div class="flex items-center gap-2 py-2">
										<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" style="color: var(--color-text-muted);" />
										<span class="text-xs" style="color: var(--color-text-muted);">Loading keys...</span>
									</div>
								{:else if savedKeys.length === 0}
									<div class="rounded-lg border px-3 py-2 text-xs" style="border-color: var(--color-border-light); color: var(--color-text-muted);">
										No saved keys yet.
										<a href="/ssh-keys" class="underline" style="color: var(--color-primary);">Add one here</a>
									</div>
								{:else}
									<select id="srv-saved-key" bind:value={sshKeyID} class="input w-full">
										<option value="">Select a saved key...</option>
										{#each savedKeys as k}
											<option value={k.id}>
												{k.name} ({truncatedFingerprint(k.fingerprint)})
											</option>
										{/each}
									</select>
								{/if}
							</div>

							{#if sshKeyID}
								{@const selectedKey = savedKeys.find(k => k.id === sshKeyID)}
								{#if selectedKey}
									<div class="rounded-lg border px-3 py-2 text-xs" style="background-color: var(--color-surface); border-color: var(--color-border-light);">
										<span class="font-medium" style="color: var(--color-primary);">{selectedKey.name}</span>
										<span style="color: var(--color-text-muted);"> — {selectedKey.key_type}</span>
										{#if selectedKey.fingerprint}
											<div class="mt-0.5 font-mono" style="color: var(--color-text-muted);">{selectedKey.fingerprint}</div>
										{/if}
									</div>
								{/if}
							{/if}
						{:else}
							<!-- Direct paste key -->
							<div>
								<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="srv-key">Private Key</label>
								<textarea
									id="srv-key"
									bind:value={sshKey}
									placeholder="-----BEGIN OPENSSH PRIVATE KEY-----
..."
									class="input w-full font-mono text-xs"
									rows="6"
								></textarea>
								<p class="mt-1 text-xs" style="color: var(--color-text-muted);">Paste your private key (PEM format)</p>
							</div>
						{/if}
					</div>
				{:else}
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);" for="srv-password">Password</label>
						<input id="srv-password" type="password" bind:value={sshPassword} placeholder="SSH password" class="input w-full" />
					</div>
				{/if}

				<!-- Metadata accordion -->
				<details class="rounded-lg border" style="border-color: var(--color-border);">
					<summary class="cursor-pointer px-4 py-2.5 text-sm font-medium select-none" style="color: var(--color-text-secondary);">
						Metadata & Organization
					</summary>
					<div class="space-y-3 border-t px-4 py-3" style="border-color: var(--color-border);">
						<!-- Tags -->
						<div>
							<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-muted);" for="srv-tags">Tags</label>
							<input id="srv-tags" bind:value={tags} placeholder="production, web, docker" class="input w-full text-sm" />
							<p class="mt-0.5 text-xs" style="color: var(--color-text-muted);">Comma-separated tags</p>
						</div>

						<!-- Group & Region -->
						<div class="grid grid-cols-2 gap-3">
							<div>
								<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-muted);" for="srv-group">Group</label>
								<input id="srv-group" bind:value={serverGroup} placeholder="production / staging / dev" class="input w-full text-sm" />
							</div>
							<div>
								<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-muted);" for="srv-region">Region</label>
								<input id="srv-region" bind:value={region} placeholder="sgp1 / us-east-1" class="input w-full text-sm" />
							</div>
						</div>

						<!-- Type -->
						<div>
							<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-muted);" for="srv-type">Type</label>
							<select id="srv-type" bind:value={serverType} class="input w-full text-sm">
								<option value="">Select type...</option>
								<option value="bare-metal">Bare Metal</option>
								<option value="vm">Virtual Machine</option>
								<option value="vps">VPS</option>
								<option value="kubernetes-node">Kubernetes Node</option>
								<option value="docker-host">Docker Host</option>
							</select>
						</div>

						<!-- Description -->
						<div>
							<label class="mb-1 block text-xs font-medium" style="color: var(--color-text-muted);" for="srv-desc">Description</label>
							<textarea id="srv-desc" bind:value={description} placeholder="Optional notes about this server..." class="input w-full text-sm" rows="2"></textarea>
						</div>
					</div>
				</details>

				<!-- Test Connection -->
				<div class="rounded-lg border p-4" style="border-color: var(--color-border); background-color: var(--color-surface);">
					<div class="flex items-center gap-3">
						<button
							onclick={handleTest}
							disabled={testing || !host}
							class="btn-secondary flex items-center gap-2"
						>
							{#if testing}
								<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" />
							{:else}
								<Icon icon="solar:plug-circle-bold" class="h-4 w-4" />
							{/if}
							Test Connection
						</button>
						{#if testResult}
							{#if testResult.reachable}
								<span class="flex items-center gap-1.5 text-sm font-medium" style="color: var(--color-success);">
									<Icon icon="solar:check-circle-bold" class="h-4 w-4" />
									Connected — {testResult.hostname?.trim()}
								</span>
							{:else}
								<span class="flex items-center gap-1.5 text-sm font-medium" style="color: var(--color-danger);">
									<Icon icon="solar:danger-triangle-bold" class="h-4 w-4" />
									{testResult.error || 'Connection failed'}
								</span>
							{/if}
						{/if}
					</div>
					<p class="mt-2 text-xs" style="color: var(--color-text-muted);">
						Test SSH connectivity before saving the server
					</p>
				</div>
			</div>

			<!-- Footer -->
			<div class="flex items-center justify-end gap-3 border-t px-6 py-4" style="border-color: var(--color-border);">
				<button onclick={handleClose} class="btn-secondary">Cancel</button>
				<button onclick={handleSave} disabled={saving || !name || !host} class="btn-primary flex items-center gap-2">
					{#if saving}
						<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" />
					{:else}
						<Icon icon="solar:add-circle-bold" class="h-4 w-4" />
					{/if}
					{isEdit ? 'Save Changes' : 'Add Server'}
				</button>
			</div>
		</div>
	</div>
{/if}
