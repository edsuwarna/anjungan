<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import Icon from '@iconify/svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	let monitor = $state(null);
	let loading = $state(true);
	let error = $state('');
	let checking = $state(false);
	let showDeleteConfirm = $state(false);
	let showEditModal = $state(false);

	// History
	let historyEntries = $state([]);
	let historyTotal = $state(0);
	let historyLoading = $state(false);
	let historyLimit = $state(50);

	// Webhooks for notification config
	let webhooks = $state([]);
	let webhooksLoading = $state(false);

	const id = $derived($page.params.id);

	onMount(() => {
		loadMonitor();
		loadWebhooks();
	});

	async function loadMonitor() {
		loading = true;
		try {
			monitor = await api.sslMonitors.get(id);
			loadHistory();
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadHistory() {
		historyLoading = true;
		try {
			const result = await api.sslMonitors.history(id, { limit: historyLimit });
			historyEntries = result.entries || [];
			historyTotal = result.total || 0;
		} catch (_) {
			historyEntries = [];
		} finally {
			historyLoading = false;
		}
	}

	async function loadWebhooks() {
		webhooksLoading = true;
		try {
			webhooks = await api.registryWebhooks.list() || [];
		} catch (_) {
			webhooks = [];
		} finally {
			webhooksLoading = false;
		}
	}

	async function checkNow() {
		checking = true;
		try {
			await api.sslMonitors.checkNow(id);
			await loadMonitor();
		} catch (e) {
			alert('Check failed: ' + e.message);
		} finally {
			checking = false;
		}
	}

	async function deleteMonitor() {
		try {
			await api.sslMonitors.delete(id);
			goto('/ssl-monitors');
		} catch (e) {
			alert('Delete failed: ' + e.message);
		}
	}

	async function handleEdit(data) {
		try {
			await api.sslMonitors.update(id, data);
			showEditModal = false;
			await loadMonitor();
		} catch (e) {
			alert('Update failed: ' + e.message);
		}
	}

	const statusConfig = {
		valid: { label: 'Valid', color: '#10b981', icon: 'solar:shield-check-bold' },
		expiring_soon: { label: 'Expiring Soon', color: '#f59e0b', icon: 'solar:clock-circle-bold' },
		expired: { label: 'Expired', color: '#ef4444', icon: 'solar:danger-circle-bold' },
		error: { label: 'Error', color: '#6b7280', icon: 'solar:close-circle-bold' },
		pending: { label: 'Pending', color: '#6366f1', icon: 'solar:hourglass-bold' },
	};

	function getConfig(s) {
		return statusConfig[s] || statusConfig.pending;
	}

	function cipherColor(grade) {
		switch(grade) {
			case 'A+': case 'A': return '#10b981';
			case 'B': return '#f59e0b';
			case 'C': case 'D': return '#ef4444';
			default: return '#6b7280';
		}
	}

	function infoRow(label, value, color = '') {
		return { label, value, color };
	}

	// ─── Chart ──────────────────────────────────────────────────────────────

	let chartEntries = $derived(historyEntries.filter(e => e.days_remaining != null).slice().reverse());

	let chartConfig = $derived.by(() => {
		const entries = chartEntries;
		if (entries.length < 2) return null;

		const values = entries.map(e => e.days_remaining);
		const maxVal = Math.max(...values, 30);
		const minVal = Math.min(...values, 0);
		const range = maxVal - minVal || 1;
		const w = 600;
		const h = 120;
		const px = 40;
		const py = 10;
		const cw = w - px * 2;
		const ch = h - py * 2;

		const points = values.map((v, i) => {
			const x = px + (i / Math.max(entries.length - 1, 1)) * cw;
			const y = py + ch - ((v - minVal) / range) * ch;
			return `${x},${y}`;
		});

		const polyline = points.join(' ');
		const area = `M${px},${py + ch} L${polyline} L${px + cw},${py + ch} Z`;

		const yLabels = [];
		const steps = 4;
		for (let i = 0; i <= steps; i++) {
			const val = Math.round(minVal + (range * i) / steps);
			const y = py + ch - (i / steps) * ch;
			yLabels.push({ val, y });
		}

		return { entries, w, h, px, py, cw, ch, polyline, area, yLabels, values };
	});

	function formatDate(iso) {
		if (!iso) return '-';
		const d = new Date(iso);
		return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' });
	}

	function formatDateShort(iso) {
		if (!iso) return '-';
		const d = new Date(iso);
		return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short' });
	}

	// ─── Webhook helper ─────────────────────────────────────────────────────
	function getWebhookName(id) {
		return webhooks.find(w => w.id === id)?.name || id?.slice(0, 8) || 'Unknown';
	}
</script>

<div class="page-container">
	<!-- Loading -->
	{#if loading}
		<div class="flex items-center justify-center py-24">
			<Icon icon="svg-spinners:180-ring" class="h-8 w-8" style="color: var(--color-primary);" />
		</div>
	{:else if error}
		<div class="card p-8 text-center">
			<Icon icon="solar:danger-circle-bold" class="mx-auto h-10 w-10" style="color: var(--color-error);" />
			<p class="mt-3 font-medium" style="color: var(--color-text);">Failed to load monitor</p>
			<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">{error}</p>
			<button class="btn-secondary mt-4" onclick={loadMonitor}>Retry</button>
			<button class="btn-ghost mt-2 ml-2" onclick={() => goto('/ssl-monitors')}>Back to list</button>
		</div>
	{:else if monitor}
		{@const cfg = getConfig(monitor.last_status)}

		<!-- Back + Header -->
		<div class="mb-4">
			<button class="btn-ghost" onclick={() => goto('/ssl-monitors')}>
				<Icon icon="solar:arrow-left-bold" class="h-4 w-4" />
				Back
			</button>
		</div>

		<div class="mb-6 flex flex-wrap items-start justify-between gap-4">
			<div class="flex items-center gap-4">
				<div>
					<h1 class="text-2xl font-bold" style="color: var(--color-text);">
						{monitor.display_name || monitor.domain}
					</h1>
					<p class="mt-1 text-sm" style="color: var(--color-text-secondary);">
						{monitor.domain}:{monitor.port}
						{#if monitor.issuer}
							<span class="mx-2">&middot;</span>
							<span>{monitor.issuer}</span>
						{/if}
					</p>
				</div>
			</div>
			<div class="flex items-center gap-2">
				<span class="status-badge" style="background: {cfg.color}15; color: {cfg.color};">
					<Icon icon={cfg.icon} class="h-4 w-4" />
					{cfg.label}
				</span>
				<button class="btn-secondary" onclick={checkNow} disabled={checking}>
					<Icon icon={checking ? 'svg-spinners:180-ring' : 'solar:refresh-bold'} class="h-4 w-4" />
					{checking ? 'Checking...' : 'Check Now'}
				</button>
				<button class="btn-secondary" onclick={() => showEditModal = true}>
					<Icon icon="solar:settings-bold" class="h-4 w-4" />
					Settings
				</button>
				<button class="btn-danger" onclick={() => showDeleteConfirm = true}>
					<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" />
					Delete
				</button>
			</div>
		</div>

		<!-- Main grid -->
		<div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
			<!-- Left: Certificate Info -->
			<div class="col-span-2 space-y-6">
				<!-- Certificate Details -->
				<div class="card">
					<h2 class="mb-4 text-lg font-bold" style="color: var(--color-text);">
						<Icon icon="solar:document-text-bold" class="mr-2 inline h-5 w-5" />
						Certificate
					</h2>
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
						{#each [
							infoRow('Subject', monitor.subject),
							infoRow('Issuer', monitor.issuer),
							infoRow('Expires', monitor.cert_expires_at ? new Date(monitor.cert_expires_at).toLocaleString() : '-', monitor.days_remaining <= 14 ? '#ef4444' : ''),
							infoRow('Days Remaining', monitor.days_remaining > 0 ? `${monitor.days_remaining} days` : 'Expired', monitor.days_remaining <= 14 ? '#ef4444' : monitor.days_remaining <= 30 ? '#f59e0b' : ''),
							{ label: 'Last Checked', value: monitor.last_check_at ? new Date(monitor.last_check_at).toLocaleString() : 'Never' },
							{ label: 'Created', value: new Date(monitor.created_at).toLocaleString() },
						] as row}
							<div>
								<p class="mb-0.5 text-xs font-medium" style="color: var(--color-text-muted);">{row.label}</p>
								<p class="text-sm font-medium" style="color: {row.color || 'var(--color-text)'};">{row.value || '-'}</p>
							</div>
						{/each}
					</div>
				</div>

				<!-- Chain Validation -->
				<div class="card">
					<h2 class="mb-4 text-lg font-bold" style="color: var(--color-text);">
						<Icon icon="solar:link-bold" class="mr-2 inline h-5 w-5" />
						Certificate Chain
					</h2>
					{#if monitor.chain_valid === true}
						<div class="mb-3 flex items-center gap-2 rounded-lg p-3" style="background: #10b98110; border: 1px solid #10b98130;">
							<Icon icon="solar:check-circle-bold" class="h-5 w-5" style="color: #10b981;" />
							<div>
								<p class="text-sm font-medium" style="color: #10b981;">Chain Valid</p>
								{#if monitor.chain_error}
									<p class="text-xs" style="color: var(--color-text-secondary);">{monitor.chain_error}</p>
								{/if}
							</div>
						</div>
					{:else if monitor.chain_valid === false}
						<div class="mb-3 flex items-center gap-2 rounded-lg p-3" style="background: #ef444410; border: 1px solid #ef444430;">
							<Icon icon="solar:danger-circle-bold" class="h-5 w-5" style="color: #ef4444;" />
							<div>
								<p class="text-sm font-medium" style="color: #ef4444;">Chain Invalid</p>
								{#if monitor.chain_error}
									<p class="text-xs" style="color: var(--color-text-secondary);">{monitor.chain_error}</p>
								{/if}
							</div>
						</div>
					{:else}
						<p class="text-sm" style="color: var(--color-text-muted);">Not checked yet</p>
					{/if}
				</div>

				<!-- SAN Names -->
				<div class="card">
					<h2 class="mb-4 text-lg font-bold" style="color: var(--color-text);">
						<Icon icon="solar:subtitles-bold" class="mr-2 inline h-5 w-5" />
						Subject Alternative Names (SAN)
					</h2>
					{#if monitor.san_names?.length > 0}
						{#if monitor.san_mismatch}
							<div class="mb-3 flex items-center gap-2 rounded-lg p-3" style="background: #f59e0b10; border: 1px solid #f59e0b30;">
								<Icon icon="solar:warning-circle-bold" class="h-5 w-5" style="color: #f59e0b;" />
								<p class="text-sm" style="color: #f59e0b;">Domain not covered by SAN names</p>
							</div>
						{/if}
						<div class="flex flex-wrap gap-2">
							{#each monitor.san_names as san}
								<span class="rounded-md px-2.5 py-1 text-sm font-medium" style="background: var(--color-primary-subtle); color: var(--color-primary);">
									{san}
								</span>
							{/each}
						</div>
					{:else}
						<p class="text-sm" style="color: var(--color-text-muted);">Not available</p>
					{/if}
				</div>

				<!-- Error message -->
				{#if monitor.last_error}
					<div class="card" style="border-color: #ef444430;">
						<h2 class="mb-2 text-lg font-bold" style="color: #ef4444;">
							<Icon icon="solar:bug-bold" class="mr-2 inline h-5 w-5" />
							Last Error
						</h2>
						<p class="text-sm" style="color: var(--color-text-secondary); font-family: monospace;">{monitor.last_error}</p>
					</div>
				{/if}
			</div>

			<!-- Right: Cipher + OCSP + Misc -->
			<div class="space-y-6">
				<!-- Cipher Grade -->
				<div class="card">
					<h2 class="mb-4 text-lg font-bold" style="color: var(--color-text);">
						<Icon icon="solar:lock-bold" class="mr-2 inline h-5 w-5" />
						Cipher
					</h2>
					<div class="flex items-center gap-3">
						<div
							class="flex h-16 w-16 items-center justify-center rounded-xl text-2xl font-bold"
							style="background: {cipherColor(monitor.cipher_grade)}15; color: {cipherColor(monitor.cipher_grade)};"
						>
							{monitor.cipher_grade || '?'}
						</div>
						<div>
							<p class="text-sm font-medium" style="color: var(--color-text);">{monitor.cipher_name || '-'}</p>
							<p class="text-xs" style="color: var(--color-text-muted);">{monitor.tls_version || '-'}</p>
						</div>
					</div>
					{#if monitor.cipher_error}
						<p class="mt-2 text-xs" style="color: #ef4444;">{monitor.cipher_error}</p>
					{/if}
				</div>

				<!-- OCSP -->
				<div class="card">
					<h2 class="mb-4 text-lg font-bold" style="color: var(--color-text);">
						<Icon icon="solar:checklist-bold" class="mr-2 inline h-5 w-5" />
						OCSP Revocation
					</h2>
					{#if monitor.ocsp_status === 'good'}
						<div class="flex items-center gap-2 rounded-lg p-3" style="background: #10b98110;">
							<Icon icon="solar:check-circle-bold" class="h-5 w-5" style="color: #10b981;" />
							<div>
								<p class="text-sm font-medium" style="color: #10b981;">Certificate is valid</p>
								<p class="text-xs" style="color: var(--color-text-muted);">OCSP check passed</p>
							</div>
						</div>
					{:else if monitor.ocsp_status === 'revoked'}
						<div class="flex items-center gap-2 rounded-lg p-3" style="background: #ef444410;">
							<Icon icon="solar:danger-circle-bold" class="h-5 w-5" style="color: #ef4444;" />
							<div>
								<p class="text-sm font-medium" style="color: #ef4444;">Certificate Revoked!</p>
							</div>
						</div>
					{:else}
						<div class="flex items-center gap-2 rounded-lg p-3" style="background: rgba(148,163,184,0.08);">
							<Icon icon="solar:question-circle-bold" class="h-5 w-5" style="color: var(--color-text-muted);" />
							<div>
								<p class="text-sm" style="color: var(--color-text-secondary);">{monitor.ocsp_status || 'Not checked'}</p>
								{#if monitor.ocsp_error}
									<p class="text-xs" style="color: var(--color-text-muted);">{monitor.ocsp_error}</p>
								{/if}
							</div>
						</div>
					{/if}
				</div>

				<!-- Settings Summary -->
				<div class="card">
					<h2 class="mb-4 text-lg font-bold" style="color: var(--color-text);">
						<Icon icon="solar:settings-bold" class="mr-2 inline h-5 w-5" />
						Configuration
					</h2>
					<div class="space-y-3 text-sm">
						<div class="flex justify-between">
							<span style="color: var(--color-text-muted);">Check Interval</span>
							<span class="font-medium" style="color: var(--color-text);">{monitor.check_interval || '1h'}</span>
						</div>
						<div class="flex justify-between">
							<span style="color: var(--color-text-muted);">Notify Before</span>
							<span class="font-medium" style="color: var(--color-text);">{monitor.notify_before || '14d'}</span>
						</div>
						<div class="flex justify-between">
							<span style="color: var(--color-text-muted);">Enabled</span>
							<span class="font-medium" style="color: {monitor.enabled ? '#10b981' : '#ef4444'};">{monitor.enabled ? 'Yes' : 'No'}</span>
						</div>
						{#if monitor.webhook_ids?.length > 0}
							<div>
								<p class="mb-1 text-xs" style="color: var(--color-text-muted);">Notifications</p>
								<div class="flex flex-wrap gap-1">
									{#each monitor.webhook_ids as wid}
										<span class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs" style="background: var(--color-primary-subtle); color: var(--color-primary);">
											<Icon icon="solar:notification-bold" class="h-3 w-3" />
											{getWebhookName(wid)}
										</span>
									{/each}
								</div>
							</div>
						{/if}
					</div>
				</div>
			</div>
		</div>

		<!-- ─── Check History ──────────────────────────────────────────────── -->
		<div class="mt-6">
			<div class="card">
				<h2 class="mb-4 text-lg font-bold" style="color: var(--color-text);">
					<Icon icon="solar:clock-circle-bold" class="mr-2 inline h-5 w-5" />
					Check History
					{#if historyTotal > 0}
						<span class="ml-2 text-sm font-normal" style="color: var(--color-text-muted);">({historyTotal} checks)</span>
					{/if}
				</h2>

				<!-- Mini Chart -->
				{#if chartConfig}
					<div class="mb-6 overflow-x-auto">
						<svg width={chartConfig.w} height={chartConfig.h} class="w-full" style="max-width: 100%;">
							{#each chartConfig.yLabels as yl}
								<line x1={chartConfig.px} y1={yl.y} x2={chartConfig.px + chartConfig.cw} y2={yl.y} stroke="rgba(148,163,184,0.12)" stroke-dasharray="4,4" />
								<text x={chartConfig.px - 6} y={yl.y + 4} text-anchor="end" fill="rgba(148,163,184,0.5)" font-size="10">{yl.val}d</text>
							{/each}
							<path d={chartConfig.area} fill="url(#sslGradient)" opacity="0.15" />
							<polyline points={chartConfig.polyline} fill="none" stroke="#10b981" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
							{#each chartConfig.entries as e, i}
								{@const val = chartConfig.values[i]}
								{@const x = chartConfig.px + (i / Math.max(chartConfig.entries.length - 1, 1)) * chartConfig.cw}
								{@const y = chartConfig.py + chartConfig.ch - ((val - Math.min(...chartConfig.values, 0)) / (Math.max(...chartConfig.values, 30) - Math.min(...chartConfig.values, 0) || 1)) * chartConfig.ch}
								<circle cx={x} cy={y} r="3" fill={val <= 7 ? '#ef4444' : val <= 30 ? '#f59e0b' : '#10b981'} stroke="#1a1d23" stroke-width="1.5" />
							{/each}
							<defs>
								<linearGradient id="sslGradient" x1="0" y1="0" x2="0" y2="1">
									<stop offset="0%" stop-color="#10b981" />
									<stop offset="100%" stop-color="#10b981" stop-opacity="0" />
								</linearGradient>
							</defs>
						</svg>
						<div class="mt-1 flex justify-between px-10">
							<span class="text-xs" style="color: var(--color-text-muted);">{formatDateShort(chartConfig.entries[0]?.checked_at)}</span>
							<span class="text-xs" style="color: var(--color-text-muted);">{formatDateShort(chartConfig.entries[chartConfig.entries.length - 1]?.checked_at)}</span>
						</div>
					</div>
				{/if}

				<!-- Timeline -->
				{#if historyLoading}
					<div class="flex items-center justify-center py-8">
						<Icon icon="svg-spinners:180-ring" class="h-6 w-6" style="color: var(--color-primary);" />
					</div>
				{:else if historyEntries.length === 0}
					<p class="py-6 text-center text-sm" style="color: var(--color-text-muted);">
						No check history yet. Run your first check to see results here.
					</p>
				{:else}
					<div class="overflow-x-auto">
						<table class="w-full text-sm">
							<thead>
								<tr style="border-bottom: 1px solid var(--color-border);">
									<th class="py-2 pr-4 text-left font-medium" style="color: var(--color-text-muted);">Time</th>
									<th class="py-2 pr-4 text-left font-medium" style="color: var(--color-text-muted);">Status</th>
									<th class="py-2 pr-4 text-right font-medium" style="color: var(--color-text-muted);">Days Left</th>
									<th class="py-2 pr-4 text-center font-medium" style="color: var(--color-text-muted);">Grade</th>
									<th class="py-2 text-right font-medium" style="color: var(--color-text-muted);">TLS</th>
								</tr>
							</thead>
							<tbody>
								{#each historyEntries as h (h.id)}
									{@const sc = getConfig(h.status)}
									<tr style="border-bottom: 1px solid var(--color-border);">
										<td class="py-2 pr-4 whitespace-nowrap" style="color: var(--color-text);">{formatDate(h.checked_at)}</td>
										<td class="py-2 pr-4">
											<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium" style="background: {sc.color}15; color: {sc.color};">
												<Icon icon={sc.icon} class="h-3 w-3" />
												{sc.label}
											</span>
										</td>
										<td class="py-2 pr-4 text-right font-mono font-medium" style="color: {h.days_remaining <= 7 ? '#ef4444' : h.days_remaining <= 30 ? '#f59e0b' : 'var(--color-text)'};">
											{h.days_remaining != null ? `${h.days_remaining}d` : '-'}
										</td>
										<td class="py-2 pr-4 text-center font-mono font-bold" style="color: {cipherColor(h.cipher_grade)};">
											{h.cipher_grade || '-'}
										</td>
										<td class="py-2 text-right font-mono text-xs" style="color: var(--color-text-secondary);">
											{h.tls_version || '-'}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
					{#if historyTotal > historyEntries.length}
						<div class="mt-3 text-center">
							<button class="btn-ghost text-sm" onclick={() => { historyLimit += 50; loadHistory(); }}>
								<Icon icon="solar:round-arrow-down-bold" class="h-4 w-4" />
								Load more ({historyTotal - historyEntries.length} remaining)
							</button>
						</div>
					{/if}
				{/if}
			</div>
		</div>
	{/if}
</div>

<!-- Delete Confirmation -->
{#if showDeleteConfirm}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onclick={() => showDeleteConfirm = false}>
		<div class="card max-w-sm p-6" onclick={(e) => e.stopPropagation()}>
			<h3 class="text-lg font-bold" style="color: var(--color-text);">Delete Monitor?</h3>
			<p class="mt-2 text-sm" style="color: var(--color-text-secondary);">
				Remove SSL monitoring for <strong>{monitor?.domain}</strong>? This cannot be undone.
			</p>
			<div class="mt-6 flex items-center justify-end gap-3">
				<button class="btn-secondary" onclick={() => showDeleteConfirm = false}>Cancel</button>
				<button class="btn-danger" onclick={deleteMonitor}>Delete</button>
			</div>
		</div>
	</div>
{/if}

<!-- Edit Modal -->
{#if showEditModal && monitor}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onclick={() => showEditModal = false}>
		<div class="card w-full max-w-lg p-6" onclick={(e) => e.stopPropagation()}>
			<h2 class="mb-1 text-lg font-bold" style="color: var(--color-text);">Monitor Settings</h2>
			<p class="mb-5 text-sm" style="color: var(--color-text-secondary);">{monitor.domain}:{monitor.port}</p>
			<form onsubmit={(e) => {
				e.preventDefault();
				const fd = new FormData(e.target);
				const whIds = Array.from(fd.getAll('webhook_ids'));
				handleEdit({
					display_name: fd.get('display_name'),
					port: parseInt(fd.get('port')) || monitor.port,
					check_interval: fd.get('check_interval'),
					notify_before: fd.get('notify_before'),
					enabled: fd.get('enabled') === 'true',
					webhook_ids: whIds,
				});
			}}>
				<div class="mb-4 grid grid-cols-2 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Port</label>
						<input type="number" name="port" value={monitor.port} min="1" max="65535" class="input w-full" />
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Display Name</label>
						<input type="text" name="display_name" value={monitor.display_name} placeholder={monitor.domain} class="input w-full" />
					</div>
				</div>
				<div class="mb-4 grid grid-cols-2 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Check Interval</label>
						<select name="check_interval" class="input w-full">
							<option value="30m" selected={monitor.check_interval === '30m'}>Every 30 minutes</option>
							<option value="1h" selected={monitor.check_interval === '1h'}>Every hour</option>
							<option value="6h" selected={monitor.check_interval === '6h'}>Every 6 hours</option>
							<option value="12h" selected={monitor.check_interval === '12h'}>Every 12 hours</option>
							<option value="24h" selected={monitor.check_interval === '24h'}>Every 24 hours</option>
						</select>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Notify Before</label>
						<select name="notify_before" class="input w-full">
							<option value="7d" selected={monitor.notify_before === '7d'}>7 days</option>
							<option value="14d" selected={monitor.notify_before === '14d'}>14 days</option>
							<option value="21d" selected={monitor.notify_before === '21d'}>21 days</option>
							<option value="30d" selected={monitor.notify_before === '30d'}>30 days</option>
							<option value="never" selected={monitor.notify_before === 'never'}>Never</option>
						</select>
					</div>
				</div>

				<!-- Notification Channels -->
				<div class="mb-4">
					<label class="mb-1 block text-sm font-medium" style="color: var(--color-text);">Notify Via</label>
					{#if webhooksLoading}
						<p class="text-xs" style="color: var(--color-text-muted);">Loading webhooks...</p>
					{:else if webhooks.length === 0}
						<p class="text-xs" style="color: var(--color-text-muted);">
							No webhooks configured.
							<a href="/registry/webhooks" class="underline" style="color: var(--color-primary);">Create one</a>
						</p>
					{:else}
						<div class="space-y-2">
							{#each webhooks as wh}
								{@const checked = monitor.webhook_ids?.includes(wh.id) || false}
								<label class="flex cursor-pointer items-center gap-3 rounded-lg p-2" style="background: checked ? 'var(--color-primary-subtle)' : 'transparent';" role="checkbox" tabindex="0" aria-checked={checked}>
									<input type="checkbox" name="webhook_ids" value={wh.id} checked={checked} class="h-4 w-4 rounded border-gray-300" />
									<div>
										<p class="text-sm font-medium" style="color: var(--color-text);">{wh.name || wh.url}</p>
										<p class="text-xs" style="color: var(--color-text-muted);">{wh.platform}</p>
									</div>
								</label>
							{/each}
						</div>
					{/if}
				</div>

				<div class="mb-4">
					<label class="flex items-center gap-3">
						<input type="checkbox" name="enabled" value="true" checked={monitor.enabled} class="h-4 w-4 rounded border-gray-300" />
						<span class="text-sm font-medium" style="color: var(--color-text);">Monitoring Enabled</span>
					</label>
				</div>
				<div class="flex items-center justify-end gap-3 pt-2">
					<button type="button" class="btn-secondary" onclick={() => showEditModal = false}>Cancel</button>
					<button type="submit" class="btn-primary">Save</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<style>
	.page-container {
		max-width: 1280px;
		margin: 0 auto;
		padding: 1.5rem;
	}
	.card {
		border-radius: 12px;
		padding: 1.25rem;
		background: var(--color-card);
		border: 1px solid var(--color-border);
	}
	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		border-radius: 9999px;
		padding: 0.375rem 0.75rem;
		font-size: 0.8125rem;
		font-weight: 600;
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
		background: transparent;
		cursor: pointer;
		color: var(--color-text-secondary);
		padding: 0.375rem 0.5rem;
		border-radius: 6px;
		font-size: 0.875rem;
	}
	.btn-ghost:hover { background: var(--color-hover); }
	.btn-danger {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		border-radius: 8px;
		padding: 0.5rem 1rem;
		font-size: 0.875rem;
		font-weight: 500;
		color: #ef4444;
		background: transparent;
		border: 1px solid #ef444430;
		cursor: pointer;
		transition: background 0.15s;
	}
	.btn-danger:hover { background: #ef444410; }
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
	select.input { appearance: auto; }
	:global(body.dark) .card { background: #1a1d23; border-color: rgba(148,163,184,0.08); }
</style>
