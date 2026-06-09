<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import { user } from '$lib/stores/auth.js';
	import Icon from '@iconify/svelte';
	import { page } from '$app/stores';
	let name = $derived($page.params.name);
	let tag = $derived($page.params.tag);

	let detail = $state(null);
	let loading = $state(true);
	let error = $state('');
	let activeTab = $state('info');
	let showPassword = $state(false);
	let deleting = $state(false);
	let deleteConfirm = $state(false);
	let isAdmin = $derived($user?.role === 'admin');
	let cveAvailable = $state(false);
	let cveData = $state(null);
	let cveLoading = $state(false);

	onMount(() => {
		loadDetail();
	});

	async function loadDetail() {
		loading = true;
		error = '';
		try {
			const data = await api.registry.detail(name, tag);
			detail = data;
			// Check CVE availability
			loadCve();
		} catch (e) {
			error = e.message || 'Failed to load image details';
		} finally {
			loading = false;
		}
	}

	async function loadCve() {
		cveLoading = true;
		try {
			const check = await api.registry.cve.check();
			if (check?.available) {
				cveAvailable = true;
				const data = await api.registry.cve.tagDetail(name, tag);
				cveData = data || null;
			}
		} catch (e) {
			cveAvailable = false;
		} finally {
			cveLoading = false;
		}
	}

	async function handleDelete() {
		if (!deleteConfirm) return;
		deleting = true;
		try {
			await api.registry.deleteTag(name, tag);
			window.history.back();
		} catch (e) {
			error = e.message || 'Failed to delete';
		} finally {
			deleting = false;
			deleteConfirm = false;
		}
	}

	function formatSize(bytes) {
		if (!bytes || bytes === 0) return '—';
		if (bytes < 1024) return bytes + ' B';
		if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(0) + ' KB';
		if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(0) + ' MB';
		return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
	}

	function formatDate(iso) {
		if (!iso) return '—';
		const d = new Date(iso);
		return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
	}

	function shortDigest(d) {
		if (!d) return '';
		return d.length > 25 ? d.slice(0, 25) + '...' : d;
	}

	let copiedTarget = $state('');

	async function copyToClipboard(text, target) {
		copiedTarget = target; // Show feedback immediately (optimistic)
		try {
			if (navigator.clipboard?.writeText) {
				await navigator.clipboard.writeText(text);
			} else {
				// Fallback for HTTP context (no HTTPS → clipboard API blocked)
				const ta = document.createElement('textarea');
				ta.value = text;
				ta.style.position = 'fixed';
				ta.style.opacity = '0';
				document.body.appendChild(ta);
				ta.select();
				document.execCommand('copy');
				document.body.removeChild(ta);
			}
		} catch {
			// Clipboard write failed but feedback is already shown
		}
		setTimeout(() => {
			if (copiedTarget === target) copiedTarget = '';
		}, 2000);
	}

	const tabs = $derived([
		{ id: 'info', label: 'Info', icon: 'solar:info-circle-outline' },
		{ id: 'config', label: 'Configuration', icon: 'solar:settings-outline' },
		{ id: 'layers', label: 'Layers', icon: 'solar:layers-outline' },
		{ id: 'history', label: 'History', icon: 'solar:clock-circle-outline' },
		...(cveAvailable ? [{ id: 'cve', label: 'Vulnerabilities', icon: 'solar:shield-warning-outline' }] : []),
	]);

	let pullCmd = $derived(`docker pull registry.anjungan.io/${name}:${tag}`);
</script>

<div class="page-container">
	<!-- Breadcrumb -->
	<div class="mb-4 flex items-center gap-2 text-xs" style="color: var(--color-text-secondary);">
		<a href="/registry" class="flex items-center gap-1 transition-colors" style="color: var(--color-text-secondary);">
			<Icon icon="solar:alt-arrow-left-outline" class="h-3.5 w-3.5" />
			Registry
		</a>
		<Icon icon="solar:alt-arrow-right-outline" class="h-3 w-3" style="color: var(--color-text-muted);" />
		<span class="font-medium" style="color: var(--color-text);">{name}:{tag}</span>
		<div class="ml-auto flex items-center gap-2">
			{#if isAdmin}
			<button
				class="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
				style="color: var(--color-danger); border: 1px solid rgba(239,68,68,0.3);"
				onclick={() => deleteConfirm = true}
			>
				<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
				Delete Tag
			</button>
			{/if}
		</div>
	</div>

	<!-- Copy tooltip removed — inline per button -->

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<Icon icon="solar:spinner-bold" class="h-6 w-6 animate-spin" style="color: var(--color-primary);" />
		</div>
	{:else if error}
		<div class="rounded-lg border p-3 text-xs" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2); color: var(--color-danger);">
			<div class="flex items-center gap-2">
				<Icon icon="solar:danger-triangle-bold" class="h-4 w-4" />
				<span>{error}</span>
			</div>
		</div>
	{:else if detail}
		<!-- Image Header -->
		<div class="card p-5">
			<div class="flex items-start justify-between">
				<div class="flex items-start gap-3">
					<div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg" style="background-color: var(--color-primary-subtle);">
						<Icon icon="solar:box-bold" class="h-5 w-5" style="color: var(--color-primary);" />
					</div>
					<div>
						<div class="flex items-center gap-2">
							<h2 class="text-base font-semibold" style="color: var(--color-text);">{detail.name}:{detail.tag}</h2>
							{#if detail.tag === 'latest'}
								<span class="rounded px-1.5 py-0.5 text-[10px] font-medium" style="background-color: var(--color-primary-subtle); color: var(--color-primary);">latest</span>
							{/if}
						</div>
						<div class="mt-1 flex items-center gap-2">
							<code class="font-mono text-[11px]" style="color: var(--color-text-muted);">{shortDigest(detail.digest)}</code>
							<button class="flex-shrink-0" onclick={() => copyToClipboard(detail.digest, 'digest')}>
								<Icon icon="solar:copy-outline" class="h-3 w-3" style="color: var(--color-text-muted);" />
							</button>
							{#if copiedTarget === 'digest'}
								<span class="text-[10px]" style="color: var(--color-success);">✓ Copied</span>
							{/if}
						</div>
					</div>
				</div>
				<button
					class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
					style="background-color: var(--color-primary-subtle); color: var(--color-primary);"
					onclick={() => copyToClipboard(pullCmd, 'pull-cmd')}>
					<Icon icon="solar:copy-bold" class="h-3.5 w-3.5" />
					Copy Pull Command
					{#if copiedTarget === 'pull-cmd'}
						<span style="color: var(--color-success);">✓ Copied</span>
					{/if}
				</button>
			</div>
		</div>

		<!-- Tabs -->
		<div class="flex items-center gap-1 border-b" style="border-color: var(--color-border);">
			{#each tabs as t}
				<button
					class="relative flex items-center gap-1.5 px-4 py-2.5 text-xs font-medium transition-colors"
					style="color: {activeTab === t.id ? 'var(--color-primary)' : 'var(--color-text-secondary)'};"
					onclick={() => activeTab = t.id}
				>
					{#if activeTab === t.id}
						<div class="absolute bottom-0 left-0 right-0 h-0.5 rounded-full" style="background-color: var(--color-primary);"></div>
					{/if}
					<Icon icon={t.icon} class="h-3.5 w-3.5" />
					{t.label}
				</button>
			{/each}
		</div>

		<!-- Tab: Info -->
		{#if activeTab === 'info'}
			<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
				<div class="card p-4">
					<h3 class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Metadata</h3>
					<div class="space-y-2.5">
						<div class="flex items-center justify-between">
							<span class="text-xs" style="color: var(--color-text-muted);">OS / Architecture</span>
							<span class="text-xs font-medium" style="color: var(--color-text);">{detail.os || '—'}/{detail.arch || '—'}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-xs" style="color: var(--color-text-muted);">Tag</span>
							<span class="text-xs font-medium font-mono" style="color: var(--color-text);">{detail.tag}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-xs" style="color: var(--color-text-muted);">Created</span>
							<span class="text-xs font-medium" style="color: var(--color-text);">{formatDate(detail.created)}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-xs" style="color: var(--color-text-muted);">Size</span>
							<span class="text-xs font-medium" style="color: var(--color-text);">{formatSize(detail.size)}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-xs" style="color: var(--color-text-muted);">Layer Count</span>
							<span class="text-xs font-medium" style="color: var(--color-text);">{detail.layers || 0} layers</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-xs" style="color: var(--color-text-muted);">Digest</span>
							<code class="max-w-[180px] truncate font-mono text-[10px]" style="color: var(--color-text-secondary);">{detail.digest}</code>
						</div>
					</div>
				</div>

				<div class="card p-4">
					<h3 class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Pull Command</h3>
					<div class="rounded-lg p-3" style="background-color: var(--color-primary-subtle);">
						<div class="flex items-center gap-2">
							<code class="flex-1 font-mono text-xs break-all" style="color: var(--color-text);">{pullCmd}</code>
							<button class="flex-shrink-0" onclick={() => copyToClipboard(pullCmd, 'pull-card')}>
								<Icon icon="solar:copy-outline" class="h-3.5 w-3.5" style="color: var(--color-text-muted);" />
							</button>
							{#if copiedTarget === 'pull-card'}
								<span class="text-[10px]" style="color: var(--color-success);">✓ Copied</span>
							{/if}
						</div>
					</div>
				</div>
			</div>
		{/if}

		<!-- Tab: Config -->
		{#if activeTab === 'config'}
			{@const cfg = detail.config}
			{#if cfg}
				<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
					<!-- Command -->
					<div class="card p-4">
						<h3 class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Entrypoint & CMD</h3>
						<div class="space-y-3">
							<div>
								<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Entrypoint</label>
								<div class="mt-1 rounded-lg px-3 py-2" style="background-color: var(--color-primary-subtle);">
									<code class="font-mono text-xs" style="color: var(--color-text-muted);">{cfg.entrypoint?.length ? cfg.entrypoint.join(' ') : '(none)'}</code>
								</div>
							</div>
							<div>
								<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">CMD</label>
								<div class="mt-1 rounded-lg px-3 py-2" style="background-color: var(--color-primary-subtle);">
									<code class="font-mono text-xs" style="color: var(--color-text);">{cfg.cmd?.length ? cfg.cmd.join(' ') : '(none)'}</code>
								</div>
							</div>
							<div class="grid grid-cols-2 gap-3">
								<div>
									<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Working Dir</label>
									<div class="mt-1 font-mono text-xs" style="color: var(--color-text);">{cfg.workdir || '(none)'}</div>
								</div>
								<div>
									<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">User</label>
									<div class="mt-1 font-mono text-xs" style="color: var(--color-text);">{cfg.user || '(none)'}</div>
								</div>
							</div>
						</div>
					</div>

					<!-- Expose & Volumes -->
					<div class="card p-4">
						<h3 class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Exposed Ports & Volumes</h3>
						<div class="space-y-3">
							<div>
								<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Exposed Ports</label>
								<div class="mt-1 flex flex-wrap gap-1.5">
									{#if cfg.exposed_ports?.length}
										{#each cfg.exposed_ports as port}
											<span class="rounded-lg px-2 py-0.5 text-[10px] font-mono" style="background-color: rgba(6,182,212,0.1); color: #06b6d4;">
												{port}
											</span>
										{/each}
									{:else}
										<span class="text-xs" style="color: var(--color-text-muted);">(none)</span>
									{/if}
								</div>
							</div>
							<div>
								<label class="text-[10px] font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Volumes</label>
								<div class="mt-1">
									{#if cfg.volumes?.length}
										{#each cfg.volumes as vol}
											<code class="font-mono text-xs" style="color: var(--color-text);">{vol}</code>
										{/each}
									{:else}
										<span class="text-xs" style="color: var(--color-text-muted);">(none)</span>
									{/if}
								</div>
							</div>
						</div>
					</div>

					<!-- ENV -->
					<div class="card col-span-full p-4">
						<div class="mb-3 flex items-center justify-between">
							<h3 class="text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Environment Variables</h3>
							<span class="text-[10px]" style="color: var(--color-text-muted);">{cfg.env?.length || 0} variables</span>
						</div>
						<div class="space-y-0.5">
							{#if cfg.env?.length}
								{#each cfg.env as e}
									<div class="flex items-start gap-2 rounded-lg px-3 py-1.5 transition-colors hover:opacity-80">
										<span class="flex-shrink-0 font-mono text-xs" style="color: #06b6d4;">{e.key}</span>
										<span style="color: var(--color-text-muted);">=</span>
										<span class="break-all font-mono text-xs" style="color: var(--color-text-secondary);">{e.value}</span>
									</div>
								{/each}
							{:else}
								<span class="text-xs" style="color: var(--color-text-muted);">No environment variables</span>
							{/if}
						</div>
					</div>

					<!-- Labels -->
					<div class="card col-span-full p-4">
						<div class="mb-3 flex items-center justify-between">
							<h3 class="text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Labels</h3>
							<span class="text-[10px]" style="color: var(--color-text-muted);">{cfg.labels?.length || 0} labels</span>
						</div>
						<div class="space-y-0.5">
							{#if cfg.labels?.length}
								{#each cfg.labels as l}
									<div class="flex items-start gap-2 rounded-lg px-3 py-1.5 transition-colors hover:opacity-80">
										<span class="flex-shrink-0 font-mono text-xs" style="color: #f59e0b;">{l.key}</span>
										<span style="color: var(--color-text-muted);">=</span>
										<span class="break-all font-mono text-xs" style="color: var(--color-text-secondary);">{l.value}</span>
									</div>
								{/each}
							{:else}
								<span class="text-xs" style="color: var(--color-text-muted);">No labels</span>
							{/if}
						</div>
					</div>
				</div>
			{:else}
				<div class="card p-6 text-center">
					<p class="text-xs" style="color: var(--color-text-muted);">No configuration data available for this image.</p>
				</div>
			{/if}
		{/if}

		<!-- Tab: Layers -->
		{#if activeTab === 'layers'}
			<div class="card p-4">
				<div class="mb-3">
					<h3 class="text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Layer Details</h3>
					<p class="mt-0.5 text-[10px]" style="color: var(--color-text-muted);">{detail.layers || 0} layers · {formatSize(detail.size)} total</p>
				</div>
				{#if detail.layers_arr?.length}
					<div class="space-y-0.5">
						{#each detail.layers_arr as layer, i}
							{@const maxSize = Math.max(...detail.layers_arr.map(l => l.size), 1)}
							{@const barWidth = Math.max(12, (layer.size / maxSize) * 100)}
							{@const label = layer.command === 'BASE' ? 'B' : layer.command === 'RUN' ? 'R' : layer.command === 'COPY' ? 'C' : layer.command === 'CMD' ? 'O' : layer.command === 'EXPOSE' ? 'E' : layer.command === 'FOREIGN' ? 'F' : 'L'}
							{@const badgeColor = layer.command === 'BASE' ? 'rgba(139,92,246,0.15)' : layer.command === 'RUN' ? 'rgba(59,130,246,0.15)' : layer.command === 'COPY' ? 'rgba(245,158,11,0.15)' : layer.command === 'CMD' || layer.command === 'EXPOSE' ? 'rgba(16,185,129,0.15)' : layer.command === 'FOREIGN' ? 'rgba(239,68,68,0.15)' : 'transparent'}
							{@const badgeText = layer.command === 'BASE' ? '#8b5cf6' : layer.command === 'RUN' ? '#3b82f6' : layer.command === 'COPY' ? '#f59e0b' : layer.command === 'CMD' || layer.command === 'EXPOSE' ? '#10b981' : layer.command === 'FOREIGN' ? '#ef4444' : 'var(--color-text-muted)'}
							<div class="flex items-center gap-3 rounded-lg border px-3 py-2 transition-colors" style="border-color: transparent;">
								<div class="flex h-6 w-6 items-center justify-center rounded-md text-[10px] font-semibold" style="background-color: {badgeColor}; color: {badgeText};">
									{label}
								</div>
								<div class="min-w-0 flex-1">
									<div class="text-xs font-medium" style="color: var(--color-text);">{layer.description}</div>
									<code class="font-mono text-[10px]" style="color: var(--color-text-muted);">{layer.digest}</code>
								</div>
								<div class="flex items-center gap-2 flex-shrink-0">
									<div style="width: {Math.min(barWidth, 100)}px; height: 6px; border-radius: 3px; background-color: var(--color-primary); opacity: 0.4;"></div>
									<span class="w-12 text-right text-xs" style="color: var(--color-text-muted);">{formatSize(layer.size)}</span>
								</div>
							</div>
						{/each}
					</div>
				{:else}
					<div class="py-6 text-center text-xs" style="color: var(--color-text-muted);">No layer data available.</div>
				{/if}
			</div>
		{/if}

		<!-- Tab: History -->
		{#if activeTab === 'history'}
			<div class="card p-4">
				<div class="mb-3 flex items-center justify-between">
					<h3 class="text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Build History</h3>
					<span class="text-[10px]" style="color: var(--color-text-muted);">{detail.history?.length || 0} steps</span>
				</div>
				{#if detail.history?.length}
					<div class="relative">
						<!-- Timeline line -->
						<div class="absolute left-[12px] top-2 bottom-2 w-0.5" style="background-color: var(--color-border);"></div>
						<div class="relative space-y-0">
							{#each detail.history as step}
								<div class="flex items-start gap-3 px-0 py-2.5">
									<div class="mt-1 flex-shrink-0">
										<div class="flex h-[26px] w-[26px] items-center justify-center rounded-full" style="border: 2px solid; background-color: var(--color-card); {step.empty ? 'border-color: var(--color-border)' : 'border-color: rgba(16,185,129,0.4)'};">
											<div class="h-2 w-2 rounded-full" style="background-color: {step.empty ? 'var(--color-border)' : 'var(--color-primary)'};"></div>
										</div>
									</div>
									<div class="min-w-0 flex-1 -mt-0.5">
										<code class="font-mono text-xs" style="color: {step.empty ? 'var(--color-text-muted)' : 'var(--color-text)'};">{step.command}</code>
										<p class="mt-0.5 text-[10px]" style="color: var(--color-text-muted);">{formatDate(step.created)}</p>
									</div>
								</div>
							{/each}
						</div>
					</div>
				{:else}
					<div class="py-6 text-center text-xs" style="color: var(--color-text-muted);">No build history available.</div>
				{/if}
			</div>
		{/if}

		<!-- Tab: Vulnerabilities -->
		{#if activeTab === 'cve' && cveAvailable}
			<div class="card p-4">
				<div class="mb-3 flex items-center justify-between">
					<h3 class="text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">Vulnerability Scan</h3>
					<span class="text-[10px]" style="color: var(--color-text-muted);">{cveLoading ? 'Loading...' : 'Zot CVE Extension'}</span>
				</div>
				{#if cveLoading}
					<div class="flex items-center justify-center py-8">
						<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-primary);" />
					</div>
				{:else if cveData}
					{@const summary = cveData?.Summary || cveData?.summary || cveData}
					{@const total = summary?.Total || summary?.total || 0}
					{@const critical = summary?.Critical || summary?.critical || 0}
					{@const high = summary?.High || summary?.high || 0}
					{@const medium = summary?.Medium || summary?.medium || 0}
					{@const low = summary?.Low || summary?.low || 0}
					
					{#if total > 0}
						<div class="grid grid-cols-2 gap-3 mb-4">
							<div class="rounded-lg p-4 text-center" style="background-color: rgba(239,68,68,0.1);">
								<div class="text-2xl font-bold" style="color: #ef4444;">{critical}</div>
								<div class="text-[10px]" style="color: #ef4444;">Critical</div>
							</div>
							<div class="rounded-lg p-4 text-center" style="background-color: rgba(249,115,22,0.1);">
								<div class="text-2xl font-bold" style="color: #f97316;">{high}</div>
								<div class="text-[10px]" style="color: #f97316;">High</div>
							</div>
							<div class="rounded-lg p-4 text-center" style="background-color: rgba(234,179,8,0.1);">
								<div class="text-2xl font-bold" style="color: #eab308;">{medium}</div>
								<div class="text-[10px]" style="color: #eab308;">Medium</div>
							</div>
							<div class="rounded-lg p-4 text-center" style="background-color: rgba(34,197,94,0.1);">
								<div class="text-2xl font-bold" style="color: #22c55e;">{low}</div>
								<div class="text-[10px]" style="color: #22c55e;">Low</div>
							</div>
						</div>

						<!-- CVE Detail List -->
						{@const cveList = Array.isArray(cveData?.cve) ? cveData.cve : Array.isArray(cveData) ? cveData : []}
						{#if cveList.length > 0}
							<div class="mb-3 flex items-center justify-between">
								<h4 class="text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">All Vulnerabilities</h4>
								<span class="text-[10px]" style="color: var(--color-text-muted);">{cveList.length} findings</span>
							</div>
							<div class="space-y-1">
								{#each cveList as cve}
									{@const pkgList = cve.PackageList || cve.packageList || cve.Packages || []}
									{@const mainPkg = Array.isArray(pkgList) && pkgList.length > 0 ? pkgList[0] : null}
									<div class="flex items-start gap-3 rounded-lg border px-3 py-2.5 transition-colors hover:opacity-90" style="border-color: var(--color-border);">
										<span class="severity-pill flex-shrink-0 mt-0.5" style="font-size: 10px; min-width: 52px; text-align: center; {cve.Severity === 'CRITICAL' ? 'background: rgba(239,68,68,0.12); color: #ef4444;' : cve.Severity === 'HIGH' ? 'background: rgba(249,115,22,0.12); color: #f97316;' : cve.Severity === 'MEDIUM' ? 'background: rgba(234,179,8,0.12); color: #eab308;' : 'background: rgba(34,197,94,0.12); color: #22c55e;'}">
											{cve.Severity || 'UNKNOWN'}
										</span>
										<div class="min-w-0 flex-1">
											<div class="flex items-center gap-2">
												<a
													href="https://nvd.nist.gov/vuln/detail/{cve.Id}"
													target="_blank"
													rel="noopener noreferrer"
													class="text-xs font-semibold hover:underline"
													style="color: var(--color-text);"
												>{cve.Id}</a>
												{#if mainPkg}
													<span class="text-[10px] font-mono" style="color: var(--color-text-muted);">{mainPkg.Name || mainPkg.PackageName || mainPkg.Package || ''}</span>
												{/if}
											</div>
											{#if cve.Title}
												<p class="mt-0.5 text-[11px] leading-relaxed" style="color: var(--color-text-secondary);">{cve.Title}</p>
											{/if}
											{#if mainPkg}
												<div class="mt-1 flex items-center gap-2 text-[10px] font-mono" style="color: var(--color-text-muted);">
													<span>installed: <span style="color: var(--color-text);">{mainPkg.InstalledVersion || '—'}</span></span>
													<span class="opacity-40">|</span>
													<span>fixed in: <span style="color: var(--color-success);">{mainPkg.FixedVersion || '—'}</span></span>
												</div>
											{/if}
										</div>
										<button
											class="flex-shrink-0 rounded-md p-1 transition-colors hover:opacity-80"
											style="color: var(--color-text-muted);"
											title="View CVE details"
											onclick={() => window.open(`https://nvd.nist.gov/vuln/detail/${cve.Id}`, '_blank')}
										>
											<Icon icon="solar:export-outline" class="h-3.5 w-3.5" />
										</button>
									</div>
								{/each}
							</div>
						{/if}

						<!-- Pagination info -->
						{@const pageMeta = cveData?.page?.TotalCount ?? cveData?.page?.totalCount ?? null}
						{@const shownCount = cveList.length}
						{#if pageMeta !== null && pageMeta > shownCount}
							<div class="mt-3 flex items-center justify-between">
								<p class="text-xs" style="color: var(--color-text-muted);">
									Showing <strong style="color: var(--color-text);">{shownCount}</strong> of <strong style="color: var(--color-text);">{pageMeta}</strong> vulnerabilities
								</p>
								<span class="text-[10px]" style="color: var(--color-text-muted);">(limit: 50 per page)</span>
							</div>
						{:else if pageMeta !== null}
							<p class="mt-3 text-xs" style="color: var(--color-text-secondary);">
								All <strong>{pageMeta}</strong> vulnerabilities shown
							</p>
						{/if}
					{:else}
						<div class="rounded-lg p-6 text-center" style="background-color: rgba(16,185,129,0.08);">
							<Icon icon="solar:shield-check-bold" class="h-8 w-8 mx-auto mb-2" style="color: var(--color-success);" />
							<p class="text-sm font-medium" style="color: var(--color-success);">No vulnerabilities found</p>
							<p class="mt-1 text-xs" style="color: var(--color-text-muted);">This image has passed the vulnerability scan.</p>
						</div>
					{/if}
				{:else}
					<div class="py-6 text-center">
						<p class="text-xs" style="color: var(--color-text-muted);">No CVE data available for this tag.</p>
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</div>

<!-- Delete Confirmation Modal -->
{#if deleteConfirm}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center"
		style="background-color: rgba(0,0,0,0.6);"
		onclick={() => deleteConfirm = false}
	>
		<div
			class="mx-4 w-full max-w-md rounded-xl border shadow-2xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="p-5">
				<div class="flex items-start gap-3">
					<div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full" style="background-color: rgba(239,68,68,0.15);">
						<Icon icon="solar:danger-triangle-bold" class="h-4.5 w-4.5" style="color: var(--color-danger);" />
					</div>
					<div class="min-w-0 flex-1">
						<h3 class="text-sm font-semibold" style="color: var(--color-text);">Delete Image Tag</h3>
						<p class="mt-1 text-xs" style="color: var(--color-text-secondary);">Are you sure you want to delete <strong>{name}:{tag}</strong>? This action is irreversible.</p>
					</div>
				</div>
			</div>
			<div class="flex items-center justify-end gap-2 rounded-b-xl border-t px-5 py-3" style="border-color: var(--color-border); background-color: var(--color-topbar-bg);">
				<button
					class="rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
					style="color: var(--color-text-secondary);"
					onclick={() => deleteConfirm = false}
				>Cancel</button>
				<button
					class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium text-white transition-colors"
					style="background-color: var(--color-danger);"
					onclick={handleDelete}
					disabled={deleting}
				>
					{#if deleting}
						<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
						Deleting...
					{:else}
						<Icon icon="solar:trash-bin-trash-bold" class="h-3.5 w-3.5" />
						Delete Tag
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}
