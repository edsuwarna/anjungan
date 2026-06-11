<script>
	import { onMount } from 'svelte';
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';

	let deployments = $state([]);
	let environments = $state([]);
	let loading = $state(true);
	let error = $state('');
	let activeEnv = $state(null);
	let statusFilter = $state('all');
	let showNewModal = $state(false);
	let showEnvManager = $state(false);
	let showAddEnvModal = $state(false);

	// New deployment form
	let form = $state({
		name: '', environment_id: '', repo_provider: 'github',
		repo_owner: '', repo_name: '', branch: 'main',
		commit_sha: '', server_id: '', service_name: '', image: '',
	});

	// Add environment form
	let envForm = $state({ name: '', color: '#10b981', description: '' });

	onMount(async () => {
		try {
			const [envData, depData] = await Promise.all([
				api.deployments.environments.list(),
				api.deployments.list(),
			]);
			environments = envData.environments || [];
			deployments = depData.deployments || [];
			if (environments.length > 0) {
				activeEnv = environments[0].id;
			}
		} catch (e) {
			error = e.message || 'Failed to load deployments';
		} finally {
			loading = false;
		}
	});

	let activeEnvironment = $derived(
		environments.find(e => e.id === activeEnv)
	);

	let envDeployments = $derived.by(() => {
		let list = deployments.filter(d => d.environment_id === activeEnv);
		if (statusFilter === 'running') return list.filter(d => d.status === 'running' || d.status === 'success');
		if (statusFilter === 'failed') return list.filter(d => d.status === 'failed' || d.status === 'rolled_back');
		if (statusFilter === 'pending') return list.filter(d => d.status === 'pending' || d.status === 'deploying');
		return list;
	});

	let runningCount = $derived(
		deployments.filter(d => d.environment_id === activeEnv && (d.status === 'running' || d.status === 'success')).length
	);

	let hasFailed = $derived(
		deployments.filter(d => d.environment_id === activeEnv && (d.status === 'failed')).length > 0
	);

	function statusBadge(status) {
		const map = {
			running: { cls: 'running', label: '● Running' },
			success: { cls: 'running', label: '✓ Success' },
			failed: { cls: 'failed', label: '✕ Failed' },
			pending: { cls: 'pending', label: '◐ Pending' },
			deploying: { cls: 'pending', label: '◐ Deploying' },
			rolled_back: { cls: 'rollback', label: '↩ Rolled Back' },
		};
		return map[status] || { cls: '', label: status };
	}

	function providerTag(p) {
		if (p === 'github') return { label: 'GitHub', icon: 'mdi:github' };
		if (p === 'forgejo') return { label: 'Forgejo', icon: 'simple-icons:forgejo' };
		return { label: p, icon: '' };
	}

	function switchEnv(id) { activeEnv = id; }

	async function doDeploy() {
		try {
			await api.deployments.create(form);
			showNewModal = false;
			const data = await api.deployments.list();
			deployments = data.deployments || [];
		} catch (e) {
			alert(e.message || 'Failed to deploy');
		}
	}

	async function performAction(action, id) {
		try {
			if (action === 'restart') await api.deployments.restart(id);
			else if (action === 'redeploy') await api.deployments.redeploy(id);
			else if (action === 'rollback') {
				if (!confirm('Rollback this deployment?')) return;
				await api.deployments.rollback(id);
			}
			const data = await api.deployments.list();
			deployments = data.deployments || [];
		} catch (e) {
			alert(e.message || `Failed to ${action}`);
		}
	}

	async function addEnvironment() {
		try {
			await api.deployments.environments.create(envForm);
			showAddEnvModal = false;
			const data = await api.deployments.environments.list();
			environments = data.environments || [];
			envForm = { name: '', color: '#10b981', description: '' };
		} catch (e) {
			alert(e.message || 'Failed to add environment');
		}
	}

	async function deleteEnvironment(id) {
		if (!confirm('Delete this environment? Deployments will be unlinked.')) return;
		try {
			await api.deployments.environments.delete(id);
			const data = await api.deployments.environments.list();
			environments = data.environments || [];
			if (activeEnv === id && environments.length > 0) activeEnv = environments[0].id;
		} catch (e) {
			alert(e.message || 'Failed to delete environment');
		}
	}
</script>

<div class="page-container">
	<!-- Header -->
	<div class="flex items-center justify-between mb-4">
		<div class="flex items-center gap-3">
			<h2 class="text-xl font-bold" style="color: var(--color-text);">Deployments</h2>
			<span class="text-xs px-2.5 py-0.5 rounded-full border"
				style="color: var(--color-text-secondary); border-color: var(--color-border);">
				{deployments.filter(d => d.status === 'running' || d.status === 'success').length} active
			</span>
		</div>
		<div class="flex items-center gap-2">
			<button class="btn-secondary text-sm flex items-center gap-1.5 px-3 py-1.5 rounded-lg"
				onclick={() => showEnvManager = !showEnvManager}>
				<Icon icon="solar:settings-linear" class="w-4 h-4" />
				Manage Environments
			</button>
			<button class="btn-primary text-sm flex items-center gap-1.5 px-3 py-1.5 rounded-lg"
				onclick={() => showNewModal = true}>
				<Icon icon="solar:add-circle-bold" class="w-4 h-4" />
				New Deployment
			</button>
		</div>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<Icon icon="svg-spinners:dots-scale" class="w-8 h-8" style="color: var(--color-primary);" />
		</div>

	{:else if error}
		<div class="card py-16 text-center">
			<Icon icon="solar:danger-triangle-bold" class="w-12 h-12 mx-auto mb-3" style="color: var(--color-text-muted);" />
			<p class="text-sm" style="color: var(--color-text-secondary);">{error}</p>
		</div>

	{:else}
		<!-- Environment Tabs -->
		<div class="flex gap-0 border-b mb-5 overflow-x-auto" style="border-color: var(--color-border);">
			{#each environments as env}
				<button class="env-tab" class:active={activeEnv === env.id}
					onclick={() => switchEnv(env.id)}
					style="--env-color: {env.color};">
					<span class="w-2 h-2 rounded-full flex-shrink-0" style="background: {env.color};"></span>
					<span class="text-sm font-medium whitespace-nowrap">{env.name}</span>
					<span class="text-xs ml-1" style="color: var(--color-text-muted);">
						{deployments.filter(d => d.environment_id === env.id).length}
					</span>
				</button>
			{/each}
			<button class="env-tab add-tab" onclick={() => showAddEnvModal = true}>
				<Icon icon="solar:add-circle-linear" class="w-4 h-4" />
				<span class="text-sm">Add Environment</span>
			</button>
			<div class="flex-1 border-b" style="border-color: var(--color-border);"></div>

			<!-- Status filter chips -->
			<div class="flex gap-1 items-center pb-2.5">
				<button class="status-chip" class:active={statusFilter === 'all'} onclick={() => statusFilter = 'all'}>All</button>
				<button class="status-chip flex items-center gap-1" class:active={statusFilter === 'running'} onclick={() => statusFilter = 'running'}>
					<span class="w-1.5 h-1.5 rounded-full" style="background: var(--color-primary);"></span> Running
				</button>
				<button class="status-chip flex items-center gap-1" class:active={statusFilter === 'failed'} onclick={() => statusFilter = 'failed'}>
					<span class="w-1.5 h-1.5 rounded-full" style="background: #ef4444;"></span> Failed
				</button>
			</div>
		</div>

		<!-- Summary bar -->
		{#if activeEnvironment}
			<div class="flex items-center gap-3 mb-4 text-sm px-4 py-2.5 rounded-lg border"
				style="background: var(--color-card-bg); border-color: var(--color-border);">
				<span class="w-3 h-3 rounded-full" style="background: {activeEnvironment.color};"></span>
				<span class="font-semibold" style="color: var(--color-text);">{activeEnvironment.name}</span>
				<span class="text-xs px-2 py-0.5 rounded-full flex items-center gap-1"
					style="background: rgba(16,185,129,0.1); color: var(--color-primary);">
					{runningCount} Running
				</span>
				{#if hasFailed}
					<span class="text-xs px-2 py-0.5 rounded-full flex items-center gap-1"
						style="background: rgba(239,68,68,0.1); color: #ef4444;">
						{deployments.filter(d => d.environment_id === activeEnv && d.status === 'failed').length} Failed
					</span>
				{/if}
				<span class="text-xs" style="color: var(--color-text-muted);">
					{hasFailed ? 'Some services need attention' : 'All services healthy'}
				</span>
				{#if activeEnvironment.is_protected}
					<span class="text-xs px-2 py-0.5 rounded-full ml-auto"
						style="background: rgba(239,68,68,0.1); color: #ef4444;">
						Protected
					</span>
				{/if}
			</div>
		{/if}

		<!-- Deployment Cards Grid -->
		{#if envDeployments.length === 0}
			<div class="card py-12 text-center">
				<Icon icon="solar:rocket-bold" class="w-10 h-10 mx-auto mb-2" style="color: var(--color-text-muted);" />
				<p class="text-sm" style="color: var(--color-text-secondary);">No deployments in this environment</p>
			</div>
		{:else}
			<div class="grid gap-3" style="grid-template-columns: repeat(auto-fill, minmax(420px, 1fr));">
				{#each envDeployments as dep (dep.id)}
					<div class="card deploy-card" style="padding: 18px;">
						<div class="flex items-start gap-3">
							<!-- Icon -->
							<div class="w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0"
								style="background: rgba(16,185,129,0.1);">
								<Icon icon="solar:rocket-bold" class="w-5 h-5" style="color: var(--color-primary);" />
							</div>

							<div class="flex-1 min-w-0">
								<!-- Title + Status -->
								<div class="flex items-center gap-2 mb-1">
									<span class="font-semibold text-sm truncate" style="color: var(--color-text);">
										{dep.name}
									</span>
									<span class="status-badge text-xs px-2 py-0.5 rounded {statusBadge(dep.status).cls}">
										{statusBadge(dep.status).label}
									</span>
								</div>

								<!-- Source chain with provider tag styling -->
								<div class="flex items-center gap-1.5 text-xs mb-1 flex-wrap"
									style="color: var(--color-text-secondary);">
									{#if dep.repo_provider && dep.repo_owner && dep.repo_name}
										<span class="provider-tag inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold"
											class:tag-github={dep.repo_provider === 'github'}
											class:tag-forgejo={dep.repo_provider === 'forgejo'}>
											<Icon icon={dep.repo_provider === 'github' ? 'mdi:github' : 'simple-icons:forgejo'} class="w-2.5 h-2.5" />
											{dep.repo_owner}/{dep.repo_name}
										</span>
										<span style="color: var(--color-text-muted);">→</span>
									{/if}
									{#if dep.commit_sha}
										<span class="font-mono" style="color: var(--color-text-muted);">{dep.commit_sha.slice(0, 7)}</span>
										<span style="color: var(--color-text-muted);">→</span>
									{/if}
									{#if dep.branch}
										<span class="font-mono" style="color: var(--color-primary);">{dep.branch}</span>
										<span style="color: var(--color-text-muted);">→</span>
									{/if}
									{#if activeEnvironment}
										<span class="font-medium" style="color: {activeEnvironment.color};">
											{activeEnvironment.name}
										</span>
									{/if}
								</div>

								<!-- Meta info -->
								<div class="flex items-center gap-3 text-xs flex-wrap"
									style="color: var(--color-text-muted);">
									{#if dep.server_name}
										<span>📦 {dep.server_name}</span>
									{/if}
									{#if dep.service_name}
										<span>⎔ {dep.service_name}</span>
									{/if}
									{#if dep.deployed_at}
										<span>🕐 {new Date(dep.deployed_at).toLocaleString()}</span>
									{/if}
								</div>

								<!-- Quick Actions -->
								<div class="flex items-center gap-1.5 mt-3 pt-3"
									style="border-top: 1px solid var(--color-border);">
									<button class="action-btn text-xs px-2.5 py-1 rounded flex items-center gap-1"
										onclick={() => performAction('restart', dep.id)}>
										<Icon icon="solar:refresh-bold" class="w-3 h-3" /> Restart
									</button>
									<button class="action-btn text-xs px-2.5 py-1 rounded flex items-center gap-1"
										onclick={() => performAction('redeploy', dep.id)}>
										<Icon icon="solar:refresh-square-bold" class="w-3 h-3" /> Redeploy
									</button>
									<button class="action-btn text-xs px-2.5 py-1 rounded flex items-center gap-1"
										onclick={() => performAction('rollback', dep.id)}>
										<Icon icon="solar:undo-left-round-bold" class="w-3 h-3" /> Rollback
									</button>
									<button class="action-btn text-xs px-2.5 py-1 rounded flex items-center gap-1"
										title="View Logs"
										onclick={() => alert('Logs: streaming not yet implemented')}>
										<Icon icon="solar:document-text-linear" class="w-3 h-3" /> Logs
									</button>
									<button class="action-btn text-xs px-2.5 py-1 rounded flex items-center gap-1"
										title="Inspect"
										onclick={() => alert('Inspect: detail view coming soon')}>
										<Icon icon="solar:eye-linear" class="w-3 h-3" /> Inspect
									</button>
									{#if dep.html_url || (dep.repo_provider && dep.repo_owner && dep.repo_name)}
										<a href={dep.html_url || `https://github.com/${dep.repo_owner}/${dep.repo_name}`}
											target="_blank" rel="noopener"
											class="action-btn text-xs px-2 py-1 rounded flex items-center gap-1 ml-auto"
											title="Open in {dep.repo_provider}"
											style="border-color: transparent; color: var(--color-text-muted);"
											onclick={(e) => e.stopPropagation()}>
											<Icon icon="solar:external-link-linear" class="w-3 h-3" />
										</a>
									{/if}
								</div>
							</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	{/if}
</div>

<!-- New Deployment Modal -->
{#if showNewModal}
	<div class="modal-overlay" onclick={() => showNewModal = false} role="presentation">
		<div class="modal-content" onclick={(e) => e.stopPropagation()} role="dialog">
			<div class="flex items-center justify-between mb-5">
				<h3 class="text-lg font-semibold" style="color: var(--color-text);">New Deployment</h3>
				<button class="action-btn w-7 h-7 flex items-center justify-center rounded text-lg"
					onclick={() => showNewModal = false}>✕</button>
			</div>
			<div class="flex flex-col gap-4">
				<div>
					<label class="text-xs font-medium mb-1 block" style="color: var(--color-text-secondary);">Environment *</label>
					<select class="form-input w-full" bind:value={form.environment_id}>
						<option value="">Select environment...</option>
						{#each environments as env}
							<option value={env.id}>{env.name}</option>
						{/each}
					</select>
				</div>
				<div>
					<label class="text-xs font-medium mb-1 block" style="color: var(--color-text-secondary);">Name *</label>
					<input class="form-input w-full" placeholder="e.g. Anjungan Backend" bind:value={form.name} />
				</div>
				<div>
					<label class="text-xs font-medium mb-1 block" style="color: var(--color-text-secondary);">Repository</label>
					<div class="flex gap-2">
						<select class="form-input flex-[2]" bind:value={form.repo_provider}>
							<option value="github">GitHub</option>
							<option value="forgejo">Forgejo</option>
						</select>
						<input class="form-input flex-[3]" placeholder="owner/repo" bind:value={form.repo_owner}
							oninput={() => { const parts = form.repo_owner.split('/'); if (parts.length > 1) { form.repo_owner = parts[0]; form.repo_name = parts.slice(1).join('/'); }}} />
						<input class="form-input flex-[3]" placeholder="repo name" bind:value={form.repo_name} />
					</div>
				</div>
				<div class="flex gap-2">
					<div class="flex-1">
						<label class="text-xs font-medium mb-1 block" style="color: var(--color-text-secondary);">Branch</label>
						<input class="form-input w-full" placeholder="main" bind:value={form.branch} />
					</div>
					<div class="flex-1">
						<label class="text-xs font-medium mb-1 block" style="color: var(--color-text-secondary);">Commit SHA</label>
						<input class="form-input w-full font-mono" placeholder="latest" bind:value={form.commit_sha} />
					</div>
				</div>
				<div>
					<label class="text-xs font-medium mb-1 block" style="color: var(--color-text-secondary);">Server ID</label>
					<input class="form-input w-full" placeholder="Server ID" bind:value={form.server_id} />
				</div>
				<div>
					<label class="text-xs font-medium mb-1 block" style="color: var(--color-text-secondary);">Service Name</label>
					<input class="form-input w-full" placeholder="e.g. anjungan-backend" bind:value={form.service_name} />
				</div>
				<div class="flex gap-2 justify-end mt-2">
					<button class="btn-secondary text-sm px-4 py-2 rounded-lg"
						onclick={() => showNewModal = false}>Cancel</button>
					<button class="btn-primary text-sm px-4 py-2 rounded-lg flex items-center gap-1.5"
						onclick={doDeploy}>
						<Icon icon="solar:rocket-bold" class="w-4 h-4" /> Deploy
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}

<!-- Environment Manager Panel -->
{#if showEnvManager}
	<div class="card mt-4" style="padding: 20px;">
		<div class="flex items-center justify-between mb-4">
			<h3 class="text-base font-semibold" style="color: var(--color-text);">Manage Environments</h3>
			<button class="btn-primary text-sm flex items-center gap-1.5 px-3 py-1.5 rounded-lg"
				onclick={() => showAddEnvModal = true}>
				<Icon icon="solar:add-circle-bold" class="w-4 h-4" /> Add Environment
			</button>
		</div>
		<div class="flex flex-col gap-2">
			{#each environments as env}
				<div class="flex items-center gap-3 px-4 py-3 rounded-lg border"
					style="border-color: var(--color-border);">
					<span class="w-3 h-3 rounded-full flex-shrink-0" style="background: {env.color};"></span>
					<div class="flex-1 min-w-0">
						<div class="text-sm font-medium" style="color: var(--color-text);">{env.name}</div>
						<div class="text-xs flex items-center gap-2" style="color: var(--color-text-muted);">
							<span>{deployments.filter(d => d.environment_id === env.id).length} deployments</span>
							<span class="font-mono">{env.color}</span>
						</div>
					</div>
					<button class="action-btn text-xs px-2 py-1 rounded">Edit</button>
					<button class="action-btn text-xs px-2 py-1 rounded"
						class:opacity-40={env.is_protected}
						disabled={env.is_protected}
						onclick={() => deleteEnvironment(env.id)}>Delete</button>
				</div>
			{/each}
		</div>
	</div>
{/if}

<!-- Add Environment Modal -->
{#if showAddEnvModal}
	<div class="modal-overlay" onclick={() => showAddEnvModal = false} role="presentation">
		<div class="modal-content" onclick={(e) => e.stopPropagation()} role="dialog">
			<div class="flex items-center justify-between mb-5">
				<h3 class="text-lg font-semibold" style="color: var(--color-text);">Add Environment</h3>
				<button class="action-btn w-7 h-7 flex items-center justify-center rounded text-lg"
					onclick={() => showAddEnvModal = false}>✕</button>
			</div>
			<div class="flex flex-col gap-4">
				<div>
					<label class="text-xs font-medium mb-1 block" style="color: var(--color-text-secondary);">Name *</label>
					<input class="form-input w-full" placeholder="e.g. Canary, Review Apps" bind:value={envForm.name} />
				</div>
				<div>
					<label class="text-xs font-medium mb-1 block" style="color: var(--color-text-secondary);">Color</label>
					<div class="flex gap-2 items-center">
						<input class="form-input flex-1 font-mono" bind:value={envForm.color} />
						<div class="flex gap-1.5">
							{#each ['#ef4444','#eab308','#10b981','#8b5cf6','#06b6d4','#f97316'] as c}
								<button class="w-6 h-6 rounded border-2"
									style="background: {c}; border-color: {envForm.color === c ? 'var(--color-primary)' : 'transparent'};"
									onclick={() => envForm.color = c}></button>
							{/each}
						</div>
					</div>
				</div>
				<div>
					<label class="text-xs font-medium mb-1 block" style="color: var(--color-text-secondary);">Description</label>
					<input class="form-input w-full" placeholder="Optional" bind:value={envForm.description} />
				</div>
				<div class="flex gap-2 justify-end mt-2">
					<button class="btn-secondary text-sm px-4 py-2 rounded-lg"
						onclick={() => showAddEnvModal = false}>Cancel</button>
					<button class="btn-primary text-sm px-4 py-2 rounded-lg"
						onclick={addEnvironment}>Add Environment</button>
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	:global(.env-tab) {
		display: flex; align-items: center; gap: 6px;
		padding: 8px 14px;
		border: none; border-bottom: 2px solid transparent;
		background: transparent;
		color: var(--color-text-secondary);
		cursor: pointer;
		transition: all 0.15s;
		margin-bottom: -1px;
	}
	:global(.env-tab:hover) { color: var(--color-text); background: rgba(255,255,255,0.03); }
	:global(.env-tab.active) { color: var(--color-text); border-bottom-color: var(--env-color, var(--color-primary)); }
	:global(.env-tab.add-tab) { opacity: 0.6; }
	:global(.env-tab.add-tab:hover) { opacity: 1; }

	:global(.status-chip) {
		padding: 3px 10px; border-radius: 6px; font-size: 12px; font-weight: 500;
		cursor: pointer; border: 1px solid var(--color-border);
		background: transparent; color: var(--color-text-secondary);
		transition: all 0.15s; white-space: nowrap;
	}
	:global(.status-chip.active) {
		background: rgba(16,185,129,0.1); border-color: var(--color-primary); color: var(--color-primary);
	}

	:global(.status-badge.running) { background: rgba(16,185,129,0.12); color: var(--color-primary); }
	:global(.status-badge.failed) { background: rgba(239,68,68,0.12); color: #ef4444; }
	:global(.status-badge.pending) { background: rgba(99,102,241,0.12); color: #818cf8; }
	:global(.status-badge.rollback) { background: rgba(234,179,8,0.12); color: #eab308; }

	:global(.provider-tag) {
		display: inline-flex;
		align-items: center;
		gap: 3px;
	}
	:global(.tag-github) {
		background: rgba(255,255,255,0.08);
		color: #ccc;
	}
	:global(.tag-forgejo) {
		background: rgba(16,185,129,0.12);
		color: var(--color-primary);
	}

	:global(.action-btn) {
		border: 1px solid var(--color-border);
		background: transparent;
		color: var(--color-text-muted);
		cursor: pointer;
		transition: all 0.15s;
	}
	:global(.action-btn:hover) {
		border-color: var(--color-primary);
		color: var(--color-primary);
		background: rgba(16,185,129,0.06);
	}

	:global(.modal-content) {
		width: 480px; max-width: 90vw; max-height: 85vh; overflow-y: auto;
		border-radius: 16px; border: 1px solid var(--color-border);
		background: var(--color-bg); padding: 24px;
	}

	:global(.form-input) {
		padding: 8px 12px; border-radius: 8px; font-size: 13px;
		border: 1px solid var(--color-border);
		background: var(--color-card-bg);
		color: var(--color-text); outline: none;
	}
	:global(.form-input:focus) {
		border-color: var(--color-primary);
		box-shadow: 0 0 0 3px rgba(16,185,129,0.1);
	}

	:global(.opacity-40) { opacity: 0.4; }
</style>
