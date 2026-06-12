<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	let data = $state(null); // ByServerResponse
	let loading = $state(true);
	let error = $state('');

	// Filters
	let searchQuery = $state('');
	let serverFilter = $state('all');
	let stateFilter = $state('all');
	let sortBy = $state('name'); // name, score, state, uptime

	// ── Tab State ────────────────────────────────────────
	let activeTab = $state('containers'); // 'containers' | 'security'
	let scanStatusFilter = $state('all'); // 'all' | 'scanned' | 'unscanned'

	// Action loading states
	let actionLoading = $state({});
	let scanningAll = $state(false);
	let scanningContainer = $state(null);
	let scanningServerId = $state(null);
	let scanningServerIndex = $state(0);
	let scanningServerName = $state('');

	// ── Server Expand State ──────────────────────────────
	let expandedServers = $state({});

	// ── Confirmation Modal ──────────────────────────────
	let confirmModal = $state({ show: false, title: '', message: '', action: null, danger: false });

	function registryLink(image) {
		if (!image) return null;
		if (!/^[^\/]+\//.test(image)) return null;
		const clean = image.replace(/^[^\/]+\//, '');
		const parts = clean.split(':');
		if (parts.length >= 2 && parts[0]) {
			return `/registry/${encodeURIComponent(parts[0])}/${encodeURIComponent(parts[1])}`;
		}
		return `/registry/${encodeURIComponent(clean)}`;
	}

	function serverColor(name) {
		const colors = [
			{ bg: '#10b981', label: '#059669' },
			{ bg: '#3b82f6', label: '#2563eb' },
			{ bg: '#8b5cf6', label: '#7c3aed' },
			{ bg: '#f59e0b', label: '#d97706' },
			{ bg: '#ef4444', label: '#dc2626' },
			{ bg: '#06b6d4', label: '#0891b2' },
		];
		let hash = 0;
		for (let i = 0; i < (name || '').length; i++) {
			hash = name.charCodeAt(i) + ((hash << 5) - hash);
		}
		return colors[Math.abs(hash) % colors.length];
	}

	onMount(async () => {
		await loadData();
	});

	async function loadData() {
		loading = true;
		error = '';
		try {
			const res = await api.containers.byServer();
			data = res;
			expandedServers = {};
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	// ── Derived Stats ───────────────────────────────────
	let stats = $derived.by(() => {
		if (!data) return { total: 0, running: 0, stopped: 0, paused: 0, servers: 0, avgScore: 0, totalFindings: 0 };
		const s = { total: 0, running: 0, stopped: 0, paused: 0, servers: data.servers?.length || 0, avgScore: 0, totalFindings: 0, scores: [] };
		for (const sv of data.servers) {
			for (const c of sv.containers) {
				s.total++;
				if (c.state === 'running') s.running++;
				else if (c.state === 'exited' || c.state === 'stopped') s.stopped++;
				else if (c.state === 'paused') s.paused++;
				if (c.security?.score != null) {
					s.scores.push(c.security.score);
					s.totalFindings += c.security.findings?.length || 0;
				}
			}
		}
		if (s.scores.length > 0) {
			s.avgScore = Math.round(s.scores.reduce((a, b) => a + b, 0) / s.scores.length);
		}
		return s;
	});

	// ── Security Report Derived ──────────────────────────
	let allContainers = $derived.by(() => {
		if (!data?.servers) return [];
		let list = [];
		for (const sv of data.servers) {
			for (const c of sv.containers) {
				list.push({ ...c, serverName: sv.server.name, serverId: sv.server.id });
			}
		}
		if (scanStatusFilter === 'scanned') list = list.filter(c => c.security?.score != null);
		else if (scanStatusFilter === 'unscanned') list = list.filter(c => c.security?.score == null);
		if (serverFilter !== 'all') list = list.filter(c => c.server_id === serverFilter);
		if (searchQuery) {
			const q = searchQuery.toLowerCase();
			list = list.filter(c => c.name.toLowerCase().includes(q) || (c.image || '').toLowerCase().includes(q));
		}
		list.sort((a, b) => {
			const aScore = a.security?.score ?? -1;
			const bScore = b.security?.score ?? -1;
			if (aScore === -1 && bScore === -1) return (a.name || '').localeCompare(b.name || '');
			if (aScore === -1) return 1;
			if (bScore === -1) return -1;
			return aScore - bScore;
		});
		return list;
	});

	let secReportStats = $derived.by(() => {
		const all = !data?.servers ? [] : data.servers.flatMap(sv => sv.containers.map(c => ({ ...c, serverName: sv.server.name })));
		const scanned = all.filter(c => c.security?.score != null);
		const unscanned = all.filter(c => c.security?.score == null);
		const scores = scanned.map(c => c.security.score);
		const avgScore = scores.length > 0 ? Math.round(scores.reduce((a, b) => a + b, 0) / scores.length) : null;
		const bySeverity = { critical: 0, high: 0, medium: 0, low: 0 };
		for (const c of scanned) {
			for (const f of c.security?.findings || []) {
				if (bySeverity[f.severity?.toLowerCase()] != null) bySeverity[f.severity.toLowerCase()]++;
			}
		}
		return { total: all.length, scanned: scanned.length, unscanned: unscanned.length, avgScore, bySeverity };
	});

	let scanningUnscanned = $state(false);

	async function scanAllUnscanned() {
		scanningUnscanned = true;
		const unscannedList = allContainers.filter(c => c.security?.score == null);
		const byServer = {};
		for (const c of unscannedList) {
			if (!byServer[c.serverId]) byServer[c.serverId] = [];
			byServer[c.serverId].push(c);
		}
		for (const [serverId] of Object.entries(byServer)) {
			try {
				await api.compliance.scanContainers(serverId);
			} catch (_) {}
		}
		await new Promise(r => setTimeout(r, 5000));
		await loadData();
		scanningUnscanned = false;
	}

	// ── Filtered & Sorted Servers ───────────────────────
	let filteredData = $derived.by(() => {
		if (!data?.servers) return [];

		let servers = data.servers.map(sv => {
			let containers = [...sv.containers];

			if (serverFilter !== 'all') {
				containers = containers.filter(c => c.server_id === serverFilter);
			}
			if (stateFilter === 'running') containers = containers.filter(c => c.state === 'running');
			else if (stateFilter === 'stopped') containers = containers.filter(c => c.state === 'exited' || c.state === 'stopped');
			else if (stateFilter === 'paused') containers = containers.filter(c => c.state === 'paused');

			if (searchQuery) {
				const q = searchQuery.toLowerCase();
				containers = containers.filter(c =>
					c.name.toLowerCase().includes(q) ||
					(c.image || '').toLowerCase().includes(q)
				);
			}

			containers.sort((a, b) => {
				switch (sortBy) {
					case 'score': {
						const sa = a.security?.score ?? -1;
						const sb = b.security?.score ?? -1;
						return sa - sb;
					}
					case 'state': {
						const order = { running: 0, paused: 1, exited: 2, stopped: 2 };
						const oa = order[a.state] ?? 3;
						const ob = order[b.state] ?? 3;
						if (oa !== ob) return oa - ob;
						return parseCreated(b.created) - parseCreated(a.created);
					}
					case 'uptime': {
						const aRunning = a.state === 'running' ? 0 : 1;
						const bRunning = b.state === 'running' ? 0 : 1;
						if (aRunning !== bRunning) return aRunning - bRunning;
						return parseCreated(b.created) - parseCreated(a.created);
					}
					default: {
						return (a.name || '').localeCompare(b.name || '');
					}
				}
			});

			return { ...sv, containers };
		}).filter(sv => sv.containers.length > 0 || sv.error);

		servers.sort((a, b) => (a.server?.name || '').localeCompare(b.server?.name || ''));

		return servers;
	});

	let totalFiltered = $derived(filteredData.reduce((acc, sv) => acc + sv.containers.length, 0));
	let totalScanned = $derived(filteredData.reduce((acc, sv) => {
		return acc + sv.containers.filter(c => c.security?.score != null).length;
	}, 0));

	function parseCreated(created) {
		if (!created) return 0;
		const cleaned = created.replace(/ [A-Za-z]+$/, '');
		const d = new Date(cleaned);
		return d.getTime() || 0;
	}

	// ── Container Actions ──────────────────────────────
	async function containerAction(containerId, serverId, action) {
		const key = containerId + '-' + action;
		actionLoading[key] = true;
		try {
			if (action === 'start') await api.containers.start(containerId, serverId);
			else if (action === 'stop') await api.containers.stop(containerId, serverId);
			else if (action === 'restart') await api.containers.restart(containerId, serverId);
			await loadData();
		} catch (_) {}
		actionLoading[key] = false;
	}

	function confirmAction(containerId, serverId, action, name) {
		const labels = { start: 'Start', stop: 'Stop', restart: 'Restart' };
		confirmModal = {
			show: true,
			title: `${labels[action]} Container`,
			message: `Confirm ${action} container "${name}"?`,
			danger: action === 'stop',
			action: async () => {
				await containerAction(containerId, serverId, action);
				confirmModal = { show: false, title: '', message: '', action: null, danger: false };
			},
		};
	}

	function goToServer(serverId) {
		goto(`/servers/${serverId}?tab=containers`);
	}

	// ── Server Section Expand ───────────────────────────
	function toggleServer(sv) {
		expandedServers = { ...expandedServers, [sv.server.id]: !expandedServers[sv.server.id] };
	}

	// ── Navigate to detail page ─────────────────────────
	function goToDetail(c) {
		goto(`/containers/${c.server_id}/${c.id}`);
	}

	// ── Display Helpers ────────────────────────────────
	function formatTime(ts) {
		if (!ts) return '';
		const d = new Date(parseCreated(ts));
		if (isNaN(d.getTime())) return ts;
		const now = new Date();
		const diff = now - d;
		if (diff < 60000) return 'Just now';
		if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
		if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
		if (diff < 604800000) return Math.floor(diff / 86400000) + 'd ago';
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
	}

	function formatUptime(c) {
		if (c.status && c.status.startsWith('Up')) return c.status;
		if (c.state === 'running') return 'Running';
		if (c.state === 'exited' || c.state === 'stopped') {
			const ago = c.status ? c.status.replace('Exited ', '').replace('(', '').replace(')', '').trim() : '';
			return ago ? `Stopped ${ago}` : 'Stopped';
		}
		if (c.state === 'paused') return 'Paused';
		return c.status || c.state || 'Unknown';
	}

	function stateColor(state) {
		if (state === 'running' || state === 'up') return 'var(--color-success)';
		if (state === 'exited' || state === 'stopped') return 'var(--color-danger)';
		if (state === 'paused') return '#eab308';
		return 'var(--color-text-muted)';
	}

	function stateBg(state) {
		if (state === 'running' || state === 'up') return 'rgba(16,185,129,0.1)';
		if (state === 'exited' || state === 'stopped') return 'rgba(239,68,68,0.1)';
		if (state === 'paused') return 'rgba(234,179,8,0.1)';
		return 'rgba(100,116,139,0.1)';
	}

	function securityColor(score) {
		if (score == null) return 'var(--color-text-muted)';
		if (score >= 90) return 'var(--color-success)';
		if (score >= 70) return '#eab308';
		if (score >= 50) return '#f97316';
		return 'var(--color-danger)';
	}

	function securityBg(score) {
		if (score == null) return 'rgba(100,116,139,0.08)';
		if (score >= 90) return 'rgba(16,185,129,0.1)';
		if (score >= 70) return 'rgba(234,179,8,0.1)';
		if (score >= 50) return 'rgba(249,115,22,0.1)';
		return 'rgba(239,68,68,0.1)';
	}

	function shortId(id) {
		if (!id) return '';
		return id.substring(0, 12);
	}

	function formatBytes(bytes) {
		if (!bytes || bytes === 0) return '0 B';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let i = 0;
		let val = bytes;
		while (val >= 1024 && i < units.length - 1) { val /= 1024; i++; }
		return val.toFixed(1) + ' ' + units[i];
	}

	function severityColor(severity) {
		switch (severity) {
			case 'critical': return '#ef4444';
			case 'high': return '#f97316';
			case 'medium': return '#eab308';
			case 'low': return '#64748b';
			default: return '#64748b';
		}
	}

	function severityBg(severity) {
		switch (severity) {
			case 'critical': return 'rgba(239,68,68,0.1)';
			case 'high': return 'rgba(249,115,22,0.1)';
			case 'medium': return 'rgba(234,179,8,0.1)';
			case 'low': return 'rgba(100,116,139,0.08)';
			default: return 'rgba(100,116,139,0.08)';
		}
	}

	const statusOptions = [
		{ value: 'all', label: 'All Status' },
		{ value: 'running', label: 'Running' },
		{ value: 'stopped', label: 'Stopped' },
		{ value: 'paused', label: 'Paused' },
	];

	const sortOptions = [
		{ value: 'name', label: 'Name' },
		{ value: 'score', label: 'Security (worst first)' },
		{ value: 'state', label: 'State' },
		{ value: 'uptime', label: 'Uptime' },
	];

	// ── Scan All Servers ───────────────────────────────
	async function scanAllContainers() {
		scanningAll = true;
		scanningServerIndex = 0;
		const serverList = data?.servers || [];
		for (let idx = 0; idx < serverList.length; idx++) {
			const sv = serverList[idx];
			scanningServerIndex = idx + 1;
			scanningServerName = sv.server.name;
			try {
				await api.compliance.scanContainers(sv.server.id);
			} catch (_) {}
		}
		scanningServerName = '';
		await new Promise(r => setTimeout(r, 5000));
		await loadData();
		scanningAll = false;
		scanningServerIndex = 0;
	}

	async function scanServerContainers(serverId) {
		scanningServerId = serverId;
		try {
			await api.compliance.scanContainers(serverId);
			await new Promise(r => setTimeout(r, 5000));
			await loadData();
		} catch (_) {}
		scanningServerId = null;
	}

	async function scanSingleContainer(c) {
		scanningContainer = c.id;
		try {
			await api.compliance.scanContainer(c.server_id, c.id);
			for (let i = 0; i < 60; i++) {
				await new Promise(r => setTimeout(r, 2000));
				try {
					const latest = await api.compliance.latest(c.server_id, { scan_type: 'Container Security' });
					if (latest && latest.status === 'completed') break;
					if (latest && latest.status === 'failed') break;
				} catch (_) {}
			}
			await loadData();
		} catch (_) {}
		scanningContainer = null;
	}
</script>

<div class="page-container">
	<!-- Header -->
	<div class="flex flex-wrap items-start justify-between gap-3 mb-4">
		<div>
			<h1 class="page-title">Containers</h1>
			<p class="page-subtitle">All containers across servers — grouped by server with security insights</p>
		</div>
		<div class="flex items-center gap-2">
			<button onclick={loadData} disabled={loading} class="btn-secondary flex items-center gap-2">
				<Icon icon={loading ? 'solar:spinner-bold' : 'solar:refresh-bold'} class="h-4 w-4 {loading ? 'animate-spin' : ''}" /> Refresh
			</button>
		</div>
	</div>

	<!-- Stats Cards -->
	<div class="grid gap-3 mb-5 sm:grid-cols-2 lg:grid-cols-4">
		<div class="card" style="border-left: 3px solid var(--color-primary);">
			<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-primary);">
				<Icon icon="solar:box-bold" class="h-3.5 w-3.5" /> Total Containers
			</div>
			<p class="mt-1 text-2xl font-bold" style="color: var(--color-text);">
				{loading ? '-' : stats.total}
			</p>
			<p class="text-[10px] mt-0.5" style="color: var(--color-text-muted);">on {stats.servers} server(s)</p>
		</div>
		<div class="card" style="border-left: 3px solid var(--color-success);">
			<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-success);">
				<span class="h-2 w-2 rounded-full" style="background-color: var(--color-success);"></span> Running
			</div>
			<p class="mt-1 text-2xl font-bold" style="color: var(--color-success);">
				{loading ? '-' : stats.running}
			</p>
		</div>
		<div class="card" style="border-left: 3px solid var(--color-danger);">
			<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-danger);">
				<span class="h-2 w-2 rounded-full" style="background-color: var(--color-danger);"></span> Stopped
			</div>
			<p class="mt-1 text-2xl font-bold" style="color: var(--color-danger);">
				{loading ? '-' : stats.stopped}
			</p>
		</div>
		<div class="card" style="border-left: 3px solid #eab308;">
			<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: #eab308;">
				<span class="h-2 w-2 rounded-full" style="background-color: #eab308;"></span> Paused
			</div>
			<p class="mt-1 text-2xl font-bold" style="color: #eab308;">
				{loading ? '-' : stats.paused}
			</p>
		</div>
	</div>

	<!-- Security Stats -->
	<div class="grid gap-3 mb-5 sm:grid-cols-2">
		<div class="card" style="border-left: 3px solid {securityColor(stats.avgScore)};">
			<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: {securityColor(stats.avgScore)};">
				<Icon icon="solar:shield-bold" class="h-3.5 w-3.5" /> Avg Security Score
			</div>
			<p class="mt-1 text-2xl font-bold" style="color: {securityColor(stats.avgScore)};">
				{loading ? '-' : stats.totalFindings > 0 ? stats.avgScore + '/100' : 'No scan'}
			</p>
			<p class="text-[10px] mt-0.5" style="color: var(--color-text-muted);">based on {stats.totalFindings} finding(s) across {totalScanned} container(s)</p>
		</div>
		<div class="card" style="border-left: 3px solid var(--color-primary);">
			<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-primary);">
				<Icon icon="solar:search-bold" class="h-3.5 w-3.5" /> Scan & Insights
			</div>
			<div class="mt-2 flex items-center gap-2">
				<button onclick={scanAllContainers} disabled={scanningAll || loading}
					class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all"
					style="background-color: var(--color-primary); color: #fff;">
					<Icon icon={scanningAll ? 'solar:spinner-bold' : 'solar:shield-bold'}
						class="h-4 w-4 {scanningAll ? 'animate-spin' : ''}" />
					{scanningAll ? 'Scanning...' : 'Scan All Servers'}
				</button>
			</div>
			<div class="mt-3 grid grid-cols-3 gap-2 border-t pt-3" style="border-color: var(--color-border-light);">
				<div class="text-center">
					<p class="text-lg font-bold" style="color: var(--color-text);">{stats.total}</p>
					<p class="text-[10px]" style="color: var(--color-text-muted);">Total Containers</p>
				</div>
				<div class="text-center">
					<p class="text-lg font-bold" style="color: var(--color-success);">{secReportStats.scanned}</p>
					<p class="text-[10px]" style="color: var(--color-text-muted);">Scanned</p>
				</div>
				<div class="text-center">
					<p class="text-lg font-bold" style="color: {stats.avgScore ? securityColor(stats.avgScore) : 'var(--color-text-muted)'};">{stats.avgScore ? stats.avgScore + '/100' : '—'}</p>
					<p class="text-[10px]" style="color: var(--color-text-muted);">Avg Security Score</p>
				</div>
			</div>
		</div>
	</div>

	{#if scanningAll}
		<div class="rounded-lg border px-4 py-3 mb-4" style="background-color: rgba(245,158,11,0.1); border-color: rgba(245,158,11,0.25);">
			<div class="flex items-center gap-2">
				<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" style="color: #f59e0b;" />
				<span class="text-sm font-semibold" style="color: #f59e0b;">
					Scanning all servers{scanningServerName ? ` — ${scanningServerIndex}/${data?.servers?.length || 0} (${scanningServerName})` : ''}...
				</span>
			</div>
			<div class="mt-2 h-1.5 w-full rounded-full overflow-hidden" style="background-color: rgba(245,158,11,0.15);">
				<div class="h-full rounded-full transition-all duration-500 ease-out" style="background-color: #f59e0b; width: {data?.servers?.length > 0 ? (scanningServerIndex / data.servers.length) * 100 : 0}%;"></div>
			</div>
		</div>
	{/if}

	<!-- Tab Navigation -->
	<div class="flex items-center gap-1 mb-4 border-b" style="border-color: var(--color-border-light);">
		<button onclick={() => activeTab = 'containers'}
			class="px-4 py-2 text-sm font-semibold transition-all rounded-t-lg"
			style="border-bottom: 2px solid {activeTab === 'containers' ? 'var(--color-primary)' : 'transparent'}; color: {activeTab === 'containers' ? 'var(--color-primary)' : 'var(--color-text-muted)'}">
			<Icon icon="solar:box-bold" class="h-3.5 w-3.5 inline-block mr-1.5" />
			All Containers
		</button>
		<button onclick={() => activeTab = 'security'}
			class="px-4 py-2 text-sm font-semibold transition-all rounded-t-lg"
			style="border-bottom: 2px solid {activeTab === 'security' ? 'var(--color-primary)' : 'transparent'}; color: {activeTab === 'security' ? 'var(--color-primary)' : 'var(--color-text-muted)'}">
			<Icon icon="solar:shield-bold" class="h-3.5 w-3.5 inline-block mr-1.5" />
			Security Report
			{#if secReportStats.unscanned > 0}
				<span class="ml-1.5 inline-flex items-center justify-center h-4 min-w-[16px] px-1 rounded text-[9px] font-bold"
					style="background-color: rgba(239,68,68,0.15); color: var(--color-danger);">
					{secReportStats.unscanned}
				</span>
			{/if}
		</button>
	</div>

	{#if activeTab === 'containers'}
		<!-- Toolbar: Search + Filters -->
		<div class="flex flex-wrap items-center gap-2 mb-4">
		<div class="relative flex-1 min-w-[200px] max-w-sm">
			<Icon icon="solar:minimalistic-magnifer-bold" class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4" style="color: var(--color-text-muted);" />
			<input
				type="text"
				bind:value={searchQuery}
				placeholder="Search container / image..."
				class="w-full rounded-lg border px-9 py-2 text-sm outline-none transition-colors"
				style="background-color: var(--color-surface); border-color: var(--color-border); color: var(--color-text);"
			/>
			{#if searchQuery}
				<button onclick={() => searchQuery = ''} class="absolute right-2 top-1/2 -translate-y-1/2 btn-icon h-5 w-5">
					<Icon icon="solar:close-circle-bold" class="h-4 w-4" />
				</button>
			{/if}
		</div>
		<select
			bind:value={serverFilter}
			class="rounded-lg border px-3 py-2 text-sm outline-none transition-colors"
			style="background-color: var(--color-surface); border-color: var(--color-border); color: var(--color-text);"
		>
			<option value="all">All Servers</option>
			{#each data?.servers || [] as sv}
				<option value={sv.server.id}>{sv.server.name}</option>
			{/each}
		</select>
		<select
			bind:value={stateFilter}
			class="rounded-lg border px-3 py-2 text-sm outline-none transition-colors"
			style="background-color: var(--color-surface); border-color: var(--color-border); color: var(--color-text);"
		>
			{#each statusOptions as opt}
				<option value={opt.value}>{opt.label}</option>
			{/each}
		</select>
		<select
			bind:value={sortBy}
			class="rounded-lg border px-3 py-2 text-sm outline-none transition-colors"
			style="background-color: var(--color-surface); border-color: var(--color-border); color: var(--color-text);"
		>
			{#each sortOptions as opt}
				<option value={opt.value}>{opt.label}</option>
			{/each}
		</select>
	</div>

	<!-- Loading / Empty -->
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Scanning containers across servers...</p>
			</div>
		</div>
	{:else if error}
		<div class="rounded-lg border px-4 py-3 mb-4 text-sm" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2); color: var(--color-danger);">
			<div class="flex items-center gap-2">
				<Icon icon="solar:danger-triangle-bold" class="h-4 w-4 shrink-0" />
				<span>Failed to load: {error}</span>
			</div>
		</div>
	{:else if !data || filteredData.length === 0}
		<div class="flex flex-col items-center py-16 text-center">
			<Icon icon="solar:box-bold" class="mb-3 inline-block h-12 w-12" style="color: var(--color-text-muted);" />
			<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No containers found</h3>
			<p class="text-sm" style="color: var(--color-text-secondary);">Docker may not be running on your servers</p>
		</div>
	{:else}
		<!-- Active filter indicator -->
		<div class="flex items-center gap-3 mb-3 text-xs" style="color: var(--color-text-muted);">
			<Icon icon="solar:widget-5-bold" class="h-3.5 w-3.5" />
			<span>Showing <strong style="color: var(--color-text);">{totalFiltered}</strong> containers in <strong style="color: var(--color-text);">{filteredData.length}</strong> server(s)</span>
			{#if serverFilter !== 'all'}
				<span>· server: <strong style="color: var(--color-text);">{data?.servers?.find(s => s.server.id === serverFilter)?.server?.name || serverFilter}</strong></span>
			{/if}
			{#if stateFilter !== 'all'}
				<span>· status: <strong style="color: var(--color-text);">{stateFilter}</strong></span>
			{/if}
			{#if searchQuery}
				<span>· search: "<strong style="color: var(--color-text);">{searchQuery}</strong>"</span>
			{/if}
		</div>

		{#each filteredData as sv (sv.server.id)}
			{@const isServerExpanded = expandedServers[sv.server.id]}
			{@const svContainers = sv.containers}
			{@const scorable = svContainers.filter(c => c.security?.score != null)}
			{@const avgScore = scorable.length > 0 ? Math.round(scorable.reduce((a, c) => a + (c.security?.score ?? 0), 0) / scorable.length) : null}

			<!-- Server Section -->
			<div class="mb-4 rounded-xl border overflow-hidden" style="background-color: var(--color-card); border-color: var(--color-border);">
				<!-- Server Header -->
				<button
					class="w-full flex items-center justify-between gap-3 px-4 py-3 transition-colors hover:opacity-80"
					style="border-bottom: {isServerExpanded ? '1px solid var(--color-border-light)' : 'none'};"
					onclick={() => toggleServer(sv)}
				>
					<div class="flex items-center gap-3 min-w-0">
						<Icon icon="solar:server-square-bold" class="h-5 w-5 shrink-0" style="color: var(--color-primary);" />
						<div class="min-w-0 text-left">
							<h3 class="text-sm font-bold truncate" style="color: var(--color-text);">{sv.server.name}</h3>
							<p class="text-[11px] truncate" style="color: var(--color-text-muted);">{sv.server.host}:{sv.server.port}</p>
						</div>
					</div>
					<div class="flex items-center gap-3 shrink-0">
				<span class="text-xs font-semibold" style="color: var(--color-text-muted);">{svContainers.length} container{svContainers.length !== 1 ? 's' : ''}</span>
						{#if sv.error}
							<span class="inline-flex items-center gap-1 rounded px-2 py-0.5 text-[10px] font-medium"
								style="background-color: rgba(239,68,68,0.1); color: var(--color-danger);">
								<Icon icon="solar:danger-triangle-bold" class="h-3 w-3" />
								Docker unreachable
							</span>
						{:else}
							<span onclick={(e) => { e.stopPropagation(); scanServerContainers(sv.server.id); }}
								onkeydown={(e) => e.key === 'Enter' && (e.stopPropagation(), scanServerContainers(sv.server.id))}
								role="button" tabindex="0"
								class="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-[10px] font-semibold transition-all cursor-pointer {scanningAll || scanningServerId === sv.server.id ? 'opacity-60 cursor-not-allowed' : 'hover:opacity-80'}"
								style="background-color: var(--color-primary); color: #fff;">
								<Icon icon={scanningServerId === sv.server.id ? 'solar:spinner-bold' : 'solar:shield-bold'}
									class="h-3 w-3 {scanningServerId === sv.server.id ? 'animate-spin' : ''}" />
								{scanningServerId === sv.server.id ? 'Scanning...' : 'Scan'}
							</span>
							{#if avgScore != null}
								<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-bold"
									style="background-color: {securityBg(avgScore)}; color: {securityColor(avgScore)};">
									<Icon icon="solar:shield-bold" class="h-3 w-3" />
									{avgScore}
								</span>
							{/if}
						{/if}
						<Icon
							icon={isServerExpanded ? 'solar:alt-arrow-up-bold' : 'solar:alt-arrow-down-bold'}
							class="h-4 w-4 transition-transform duration-200"
							style="color: var(--color-text-muted);"
						/>
					</div>
				</button>

				<!-- Server Container Grid -->
				{#if isServerExpanded}
					{#if sv.error}
						<div class="px-4 py-6 text-center">
							<Icon icon="solar:danger-triangle-bold" class="h-8 w-8 mx-auto mb-2" style="color: var(--color-danger);" />
							<p class="text-sm font-semibold" style="color: var(--color-text);">Docker unreachable</p>
							<p class="text-xs mt-1" style="color: var(--color-text-muted);">{sv.error}</p>
						</div>
					{:else}
					<div class="grid gap-3 p-3" style="grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));">
						{#each svContainers as c (c.id)}
							{@const col = serverColor(sv.server.name)}
							{@const isRunning = c.state === 'running'}
							{@const isStopped = c.state === 'exited' || c.state === 'stopped'}
							{@const isPaused = c.state === 'paused'}
							{@const sec = c.security}
							{@const hasSecurity = sec?.score != null}
							{@const secScore = sec?.score ?? 0}

							<div
								class="card !p-0 overflow-hidden"
								style="transition: all 0.15s ease;"
							>
								<div class="flex" style="min-height: 0;">
									<!-- State-based accent bar -->
									<div style="width: 4px; flex-shrink: 0; background: {isRunning ? 'var(--color-success)' : isStopped ? 'var(--color-danger)' : isPaused ? '#eab308' : 'var(--color-text-muted)'}; border-radius: 4px 0 0 4px;"></div>
									<div class="flex-1 min-w-0">
										<!-- Card Content -->
										<div class="px-4 pt-4 pb-3">
											<div class="flex items-start justify-between gap-2">
												<div class="min-w-0 flex-1">
													<div class="flex items-center gap-2">
														<h3 class="text-sm font-bold truncate" style="color: var(--color-text);" title={c.name}>{c.name}</h3>
														{#if hasSecurity}
															<span class="inline-flex items-center justify-center h-5 min-w-[32px] px-1.5 rounded text-[10px] font-bold shrink-0"
																style="background-color: {securityBg(secScore)}; color: {securityColor(secScore)};"
																title="Security score: {secScore}/100">
																{secScore}
															</span>
														{/if}
													</div>
													<div class="flex items-center gap-1.5 mt-1.5 flex-wrap">
														<span
															class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold whitespace-nowrap cursor-pointer hover:opacity-80"
															style="background-color: {col.bg}18; color: {col.label};"
															onclick={(e) => { e.stopPropagation(); goToServer(c.server_id); }}
															title="View server details"
														>
															<Icon icon="solar:server-square-bold" class="h-2.5 w-2.5" />
															{sv.server.name || '—'}
														</span>
														<span
															class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold whitespace-nowrap"
															style="background-color: {stateBg(c.state)}; color: {stateColor(c.state)};"
														>
															<span class="h-1.5 w-1.5 rounded-full" style="background-color: {stateColor(c.state)}; box-shadow: 0 0 4px {stateColor(c.state)};"></span>
															{isRunning ? 'Running' : isStopped ? 'Stopped' : isPaused ? 'Paused' : c.state || 'Unknown'}
														</span>
													</div>
													{#if sec?.badges?.length}
														<div class="flex flex-wrap gap-1 mt-1.5">
															{#each sec.badges as badge}
																<span class="inline-flex items-center px-1.5 py-0.5 rounded text-[9px] font-medium"
																	style="background-color: rgba(148,163,184,0.1); color: rgba(148,163,184,0.8);">
																	{badge}
																</span>
															{/each}
														</div>
													{/if}
												</div>
												<div class="flex flex-col items-end gap-1 shrink-0">
													{#if scanningContainer === c.id}
														<span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[9px] font-bold" style="background: rgba(245,158,11,0.15); color: #fbbf24;">
															<Icon icon="solar:spinner-bold" class="h-2.5 w-2.5 animate-spin" /> Scanning
														</span>
													{/if}
													<span class="text-[10px] font-mono" style="color: var(--color-text-muted);">#{shortId(c.id)}</span>
												</div>
											</div>

											<!-- Image & Ports Row -->
											<div class="mt-2 space-y-1">
												<div class="flex items-center gap-2 text-xs" style="color: var(--color-text-secondary);">
													<Icon icon="solar:box-bold" class="h-3 w-3 shrink-0" style="color: var(--color-text-muted);" />
													<span class="truncate font-mono" title={c.image}>
														{#if registryLink(c.image)}
															<button
																class="hover:underline transition-colors"
																style="color: var(--color-primary);"
																onclick={() => goto(registryLink(c.image))}
															>
																{c.image}
															</button>
														{:else}
															<span style="color: var(--color-text);">{c.image}</span>
														{/if}
													</span>
												</div>
												{#if c.ports}
													<div class="flex items-center gap-2 text-xs" style="color: var(--color-text-secondary);">
														<Icon icon="solar:plug-circle-bold" class="h-3 w-3 shrink-0" style="color: var(--color-text-muted);" />
														<span class="truncate">{c.ports}</span>
													</div>
												{/if}
												<div class="flex items-center gap-2 text-xs" style="color: var(--color-text-secondary);">
													<Icon icon="solar:clock-circle-bold" class="h-3 w-3 shrink-0" style="color: var(--color-text-muted);" />
													<span>{formatUptime(c)}
														{#if c.created}
															<span style="color: var(--color-text-muted);"> · created {formatTime(c.created)}</span>
														{/if}
													</span>
												</div>
											</div>

											<!-- Action Buttons -->
											<div class="flex flex-wrap items-center gap-1.5 mt-3 pt-3 border-t" style="border-color: var(--color-border-light);">
												<button onclick={() => confirmAction(c.id, c.server_id, 'start', c.name)}
													class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-[11px] font-semibold transition-all"
													style="background-color: var(--color-success); color: #fff;">
													<Icon icon="solar:play-bold" class="h-3 w-3" /> Start
												</button>
												<button onclick={() => confirmAction(c.id, c.server_id, 'stop', c.name)}
													class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-[11px] font-semibold transition-all"
													style="background-color: var(--color-danger); color: #fff;">
													<Icon icon="solar:pause-bold" class="h-3 w-3" /> Stop
												</button>
												<button onclick={() => confirmAction(c.id, c.server_id, 'restart', c.name)}
													class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-[11px] font-semibold transition-all"
													style="background-color: #eab308; color: #fff;">
													<Icon icon="solar:refresh-bold" class="h-3 w-3" /> Restart
												</button>
												<span class="flex-1"></span>
												<button onclick={() => goToDetail(c)}
													class="inline-flex items-center gap-1 rounded-lg px-3 py-1.5 text-[11px] font-semibold transition-all hover:opacity-80"
													style="border: 1px solid var(--color-border); color: var(--color-text-secondary); background: transparent;">
													<Icon icon="solar:export-bold" class="h-3 w-3" /> Details
												</button>
											</div>
										</div>
									</div>
								</div>
							</div>
						{/each}
					</div>
					{/if}
				{/if}
			</div>
		{/each}
	{/if}
{:else}
	<!-- ─── Security Report View ──────────────────── -->
	{#if loading}
		<div class="flex items-center justify-center py-20">
			<div class="flex flex-col items-center gap-3">
				<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
				<p class="text-sm" style="color: var(--color-text-muted);">Loading security data...</p>
			</div>
		</div>
	{:else if !data}
		<div class="flex flex-col items-center py-16 text-center">
			<Icon icon="solar:shield-bold" class="mb-3 inline-block h-12 w-12" style="color: var(--color-text-muted);" />
			<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No data available</h3>
		</div>
	{:else}
		<!-- Security Report Stats -->
		<div class="grid gap-3 mb-4 sm:grid-cols-2 lg:grid-cols-4">
			<div class="card" style="border-left: 3px solid var(--color-primary);">
				<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-primary);">
					<Icon icon="solar:box-bold" class="h-3.5 w-3.5" /> Total Containers
				</div>
				<p class="mt-1 text-2xl font-bold" style="color: var(--color-text);">{secReportStats.total}</p>
			</div>
			<div class="card" style="border-left: 3px solid {secReportStats.avgScore != null ? securityColor(secReportStats.avgScore) : 'var(--color-text-muted)'};">
				<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: {secReportStats.avgScore != null ? securityColor(secReportStats.avgScore) : 'var(--color-text-muted)'};">
					<Icon icon="solar:shield-bold" class="h-3.5 w-3.5" /> Avg Score
				</div>
				<p class="mt-1 text-2xl font-bold" style="color: {secReportStats.avgScore != null ? securityColor(secReportStats.avgScore) : 'var(--color-text-muted)'};">
					{secReportStats.avgScore != null ? secReportStats.avgScore + '/100' : '—'}
				</p>
			</div>
			<div class="card" style="border-left: 3px solid var(--color-success);">
				<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-success);">
					<Icon icon="solar:shield-check-bold" class="h-3.5 w-3.5" /> Scanned
				</div>
				<p class="mt-1 text-2xl font-bold" style="color: var(--color-success);">{secReportStats.scanned}</p>
				<p class="text-[10px] mt-0.5" style="color: var(--color-text-muted);">of {secReportStats.total}</p>
			</div>
			<div class="card" style="border-left: 3px solid {secReportStats.unscanned > 0 ? 'var(--color-danger)' : 'var(--color-success)'};">
				<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: {secReportStats.unscanned > 0 ? 'var(--color-danger)' : 'var(--color-success)'};">
					<Icon icon="solar:shield-minimalistic-bold" class="h-3.5 w-3.5" /> Unscanned
				</div>
				<p class="mt-1 text-2xl font-bold" style="color: {secReportStats.unscanned > 0 ? 'var(--color-danger)' : 'var(--color-success)'};">{secReportStats.unscanned}</p>
			</div>
		</div>

		<!-- Severity breakdown -->
		<div class="flex flex-wrap items-center gap-3 mb-4">
			<span class="text-xs font-semibold" style="color: var(--color-text-muted);">Findings:</span>
			{#each Object.entries(secReportStats.bySeverity) as [sev, count]}
				{#if count > 0}
					<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold"
						style="background-color: {severityBg(sev)}; color: {severityColor(sev)};">
						{sev}: {count}
					</span>
				{/if}
			{/each}
			<div class="flex-1"></div>
			<button onclick={scanAllUnscanned} disabled={scanningUnscanned || secReportStats.unscanned === 0}
				class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all"
				style="background-color: var(--color-primary); color: #fff; {secReportStats.unscanned === 0 ? 'opacity:0.5;' : ''}">
				<Icon icon={scanningUnscanned ? 'solar:spinner-bold' : 'solar:shield-bold'}
					class="h-4 w-4 {scanningUnscanned ? 'animate-spin' : ''}" />
				{scanningUnscanned ? 'Scanning...' : `Scan All Unscanned (${secReportStats.unscanned})`}
			</button>
		</div>

		<!-- Security Report Toolbar -->
		<div class="flex flex-wrap items-center gap-2 mb-3">
			<div class="relative flex-1 min-w-[200px] max-w-sm">
				<Icon icon="solar:minimalistic-magnifer-bold" class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4" style="color: var(--color-text-muted);" />
				<input
					type="text"
					bind:value={searchQuery}
					placeholder="Search container / image..."
					class="w-full rounded-lg border px-9 py-2 text-sm outline-none transition-colors"
					style="background-color: var(--color-surface); border-color: var(--color-border); color: var(--color-text);"
				/>
				{#if searchQuery}
					<button onclick={() => searchQuery = ''} class="absolute right-2 top-1/2 -translate-y-1/2 btn-icon h-5 w-5">
						<Icon icon="solar:close-circle-bold" class="h-4 w-4" />
					</button>
				{/if}
			</div>
			<select
				bind:value={serverFilter}
				class="rounded-lg border px-3 py-2 text-sm outline-none transition-colors"
				style="background-color: var(--color-surface); border-color: var(--color-border); color: var(--color-text);"
			>
				<option value="all">All Servers</option>
				{#each data?.servers || [] as sv}
					<option value={sv.server.id}>{sv.server.name}</option>
				{/each}
			</select>
			<select
				bind:value={scanStatusFilter}
				class="rounded-lg border px-3 py-2 text-sm outline-none transition-colors"
				style="background-color: var(--color-surface); border-color: var(--color-border); color: var(--color-text);"
			>
				<option value="all">All Status</option>
				<option value="scanned">Scanned</option>
				<option value="unscanned">Unscanned</option>
			</select>
			<span class="text-xs" style="color: var(--color-text-muted);">
				<strong style="color: var(--color-text);">{allContainers.length}</strong> container{allContainers.length !== 1 ? 's' : ''}
			</span>
		</div>

		<!-- Security Report Table -->
		{#if allContainers.length === 0}
			<div class="flex flex-col items-center py-16 text-center">
				<Icon icon="solar:shield-bold" class="mb-3 inline-block h-12 w-12" style="color: var(--color-text-muted);" />
				<h3 class="mb-1 text-base font-semibold" style="color: var(--color-text);">No containers found</h3>
				<p class="text-sm" style="color: var(--color-text-secondary);">Try adjusting your filters</p>
			</div>
		{:else}
			<div class="rounded-xl border overflow-hidden" style="background-color: var(--color-card); border-color: var(--color-border);">
				<div class="overflow-x-auto">
					<table class="w-full text-xs">
						<thead>
							<tr style="background-color: var(--color-surface); border-bottom: 1px solid var(--color-border-light);">
								<th class="text-left px-3 py-2.5 font-semibold" style="color: var(--color-text-muted);">Container</th>
								<th class="text-left px-3 py-2.5 font-semibold hidden sm:table-cell" style="color: var(--color-text-muted);">Server</th>
								<th class="text-center px-3 py-2.5 font-semibold" style="color: var(--color-text-muted);">Score</th>
								<th class="text-left px-3 py-2.5 font-semibold hidden md:table-cell" style="color: var(--color-text-muted);">Badges</th>
								<th class="text-center px-3 py-2.5 font-semibold hidden lg:table-cell" style="color: var(--color-text-muted);">Findings</th>
								<th class="text-right px-3 py-2.5 font-semibold hidden sm:table-cell" style="color: var(--color-text-muted);">Scanned</th>
							</tr>
						</thead>
						<tbody>
							{#each allContainers as c (c.id)}
								<tr
									onclick={() => goto(`/containers/${c.serverId}/${c.id}/security`)}
									class="cursor-pointer transition-colors hover:opacity-80"
									style="border-bottom: 1px solid var(--color-border-light);"
									role="button"
									tabindex="0"
									onkeydown={(e) => e.key === 'Enter' && goto(`/containers/${c.serverId}/${c.id}/security`)}
								>
									<td class="px-3 py-2.5">
										<div class="flex items-center gap-2">
											<div class="w-1.5 h-1.5 rounded-full shrink-0"
												style="background-color: {c.state === 'running' ? 'var(--color-success)' : c.state === 'exited' || c.state === 'stopped' ? 'var(--color-danger)' : '#eab308'};">
											</div>
											<div class="min-w-0">
												<p class="text-sm font-semibold truncate max-w-[180px] sm:max-w-[240px]" style="color: var(--color-text);" title={c.name}>{c.name}</p>
												<p class="text-[10px] font-mono truncate max-w-[180px] sm:max-w-[240px]" style="color: var(--color-text-muted);">{c.image}</p>
											</div>
										</div>
									</td>
									<td class="px-3 py-2.5 hidden sm:table-cell">
										<span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold"
											style="background-color: {serverColor(c.serverName || c.serverId).bg}18; color: {serverColor(c.serverName || c.serverId).label};">
											{c.serverName || c.serverId}
										</span>
									</td>
									<td class="px-3 py-2.5 text-center">
										{#if c.security?.score != null}
											<span class="inline-flex items-center justify-center h-6 min-w-[36px] px-1.5 rounded text-[11px] font-bold"
												style="background-color: {securityBg(c.security.score)}; color: {securityColor(c.security.score)};">
												{c.security.score}
											</span>
										{:else}
											<span class="text-[10px] font-medium" style="color: var(--color-text-muted);">—</span>
										{/if}
									</td>
									<td class="px-3 py-2.5 hidden md:table-cell">
										<div class="flex flex-wrap gap-1">
											{#if c.security?.badges?.length}
												{#each c.security.badges.slice(0, 3) as badge}
													<span class="inline-flex items-center px-1.5 py-0.5 rounded text-[9px] font-medium"
														style="background-color: rgba(148,163,184,0.1); color: rgba(148,163,184,0.8);">
														{badge}
													</span>
												{/each}
												{#if c.security.badges.length > 3}
													<span class="text-[9px]" style="color: var(--color-text-muted);">+{c.security.badges.length - 3}</span>
												{/if}
											{:else}
												<span class="text-[9px]" style="color: var(--color-text-muted);">—</span>
											{/if}
										</div>
									</td>
									<td class="px-3 py-2.5 text-center hidden lg:table-cell">
										{#if c.security?.findings?.length}
											<span class="text-xs font-semibold" style="color: {c.security.findings.length > 5 ? 'var(--color-danger)' : 'var(--color-text)'};">
												{c.security.findings.length}
											</span>
										{:else}
											<span class="text-[10px]" style="color: var(--color-text-muted);">—</span>
										{/if}
									</td>
									<td class="px-3 py-2.5 text-right hidden sm:table-cell">
										{#if c.security?.scanned_at}
											<span class="text-[10px] font-mono" style="color: var(--color-text-muted);">{formatTime(c.security.scanned_at)}</span>
										{:else}
											<span class="text-[10px] font-medium" style="color: var(--color-danger);">Never</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	{/if}
{/if}
</div>

<!-- ─── Confirmation Modal ───────────────────────── -->
{#if confirmModal.show}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background-color: rgba(0,0,0,0.5);"
		onclick={() => confirmModal = { show: false, title: '', message: '', action: null, danger: false }}>
		<div class="w-full max-w-sm rounded-xl border shadow-xl"
			style="background-color: var(--color-card); border-color: var(--color-border);"
			onclick={(e) => e.stopPropagation()}>
			<div class="px-6 py-5">
				<div class="flex items-center gap-3">
					<div class="flex h-10 w-10 items-center justify-center rounded-full"
						style="background-color: {confirmModal.danger ? 'rgba(239,68,68,0.1)' : 'rgba(16,185,129,0.1)'};">
						<Icon icon={confirmModal.danger ? 'solar:danger-triangle-bold' : 'solar:info-circle-bold'}
							class="h-5 w-5" style="color: {confirmModal.danger ? 'var(--color-danger)' : 'var(--color-primary)'};" />
					</div>
					<h3 class="text-base font-semibold" style="color: var(--color-text);">{confirmModal.title}</h3>
				</div>
				<p class="mt-3 text-sm" style="color: var(--color-text-secondary);">{confirmModal.message}</p>
			</div>
			<div class="flex items-center justify-end gap-2 border-t px-6 py-3" style="border-color: var(--color-border);">
				<button onclick={() => confirmModal = { show: false, title: '', message: '', action: null, danger: false }}
					class="btn-secondary text-sm">Cancel</button>
				<button onclick={() => confirmModal.action?.()}
					class="text-sm" class:btn-danger={confirmModal.danger} class:btn-primary={!confirmModal.danger}>
					{confirmModal.title}
				</button>
			</div>
		</div>
	</div>
{/if}
