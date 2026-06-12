<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { Terminal } from 'xterm';
	import { FitAddon } from 'xterm-addon-fit';
	import 'xterm/css/xterm.css';

	// ── Route params ──────────────────────────────────────
	let serverId = $derived($page.params.serverId);
	let containerId = $derived($page.params.containerId);

	// ── Tab state ─────────────────────────────────────────
	let activeTab = $derived.by(() => {
		const tab = $page.url.searchParams.get('tab') || 'stats';
		return ['stats', 'logs', 'security', 'exec', 'inspect'].includes(tab) ? tab : 'stats';
	});

	// ── Core data ─────────────────────────────────────────
	let container = $state(null);
	let server = $state(null);
	let stats = $state(null);
	let inspect = $state(null);
	let logs = $state('');
	let security = $state(null);

	let loading = $state(true);
	let statsLoading = $state(false);
	let logsLoading = $state(false);
	let inspectLoading = $state(false);
	let error = $state('');

	// ── Security sub-state ────────────────────────────────
	let scanning = $state(false);
	let scanElapsed = $state(0);
	let scanTimer = null;
	let securityLoading = $state(false);

	// ── Exec terminal ─────────────────────────────────────
	let execTermInstance = null;

	// ── Action state ──────────────────────────────────────
	let actionLoading = $state({});

	// ── Confirm modal ─────────────────────────────────────
	let confirmModal = $state({ show: false, title: '', message: '', action: null, danger: false });

	// ── Container list for switcher ───────────────────────
	let allContainers = $state([]);
	let serverDropdownOpen = $state(false);
	let containerDropdownOpen = $state(false);

	// ── Helpers ───────────────────────────────────────────
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

	function parseCreated(created) {
		if (!created) return 0;
		const cleaned = created.replace(/ [A-Za-z]+$/, '');
		const d = new Date(cleaned);
		return d.getTime() || 0;
	}

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

	function formatBytes(bytes) {
		if (!bytes || bytes === 0) return '0 B';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let i = 0;
		let val = bytes;
		while (val >= 1024 && i < units.length - 1) { val /= 1024; i++; }
		return val.toFixed(1) + ' ' + units[i];
	}

	function escapeHtml(str) {
		return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
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

	function severityColor(severity) {
		switch (severity?.toLowerCase()) {
			case 'critical': return '#ef4444';
			case 'high': return '#f97316';
			case 'medium': return '#eab308';
			case 'low': return '#64748b';
			default: return '#64748b';
		}
	}

	function severityBg(severity) {
		switch (severity?.toLowerCase()) {
			case 'critical': return 'rgba(239,68,68,0.1)';
			case 'high': return 'rgba(249,115,22,0.1)';
			case 'medium': return 'rgba(234,179,8,0.1)';
			case 'low': return 'rgba(100,116,139,0.08)';
			default: return 'rgba(100,116,139,0.08)';
		}
	}

	function shortId(id) {
		if (!id) return '';
		return id.substring(0, 12);
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

	function calcMemPct(stats) {
		if (!stats || !(stats.memory_limit ?? 0) > 0) return 0;
		return ((stats.memory_usage ?? 0) / (stats.memory_limit ?? 1) * 100);
	}

	function formatLogsForDisplay(logStr) {
		if (!logStr) return '<span style="color: #64748b;">No logs available</span>';
		const lines = logStr.split('\n').filter(Boolean);
		return lines.map((line, idx) => {
			const escaped = escapeHtml(line);
			const tsRegex = /^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)\s+(.*)/;
			const match = escaped.match(tsRegex);
			let colored;
			if (match) {
				const prefix = `<span style="color: #60a5fa;">${match[1]}</span>`;
				const msg = colorInnerTimestamps(match[2]);
				colored = prefix + ' ' + msg;
			} else {
				colored = `<span style="color: #22c55e;">${escaped}</span>`;
			}
			const border = idx < lines.length - 1 ? 'border-bottom: 1px solid rgba(255,255,255,0.06);' : '';
			return `<div style="padding: 2px 0; ${border}">${colored}</div>`;
		}).join('\n');
	}

	function colorInnerTimestamps(text) {
		const innerTsRegex = /(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)/g;
		return text.replace(innerTsRegex, '<span style="color: #fbbf24;">$1</span>');
	}

	// ── Data Loading ─────────────────────────────────────
	async function loadData() {
		loading = true;
		error = '';
		try {
			// Load container details + all servers for switcher
			const [ctr, serversData] = await Promise.all([
				api.containers.get(containerId, serverId),
				api.containers.byServer().catch(() => ({ servers: [] })),
			]);
			container = ctr;
			if (ctr.server_info) server = ctr.server_info;
			allContainers = serversData.servers || [];

			// Auto-load stats
			loadStats();
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadStats() {
		statsLoading = true;
		try {
			const [s, ins] = await Promise.all([
				api.containers.stats(containerId, serverId).catch(() => null),
				api.containers.get(containerId, serverId).catch(() => null),
			]);
			stats = s;
			if (ins) {
				if (Array.isArray(ins) && ins.length > 0) {
					const data = ins[0];
					inspect = {
						restart_count: data.State?.RestartCount ?? 0,
						network_mode: data.HostConfig?.NetworkMode ?? 'bridge',
						ip_address: data.NetworkSettings?.Networks ? Object.values(data.NetworkSettings.Networks).find(n => n.IPAddress)?.IPAddress : '—',
						raw: JSON.stringify(data, null, 2),
					};
				} else if (!Array.isArray(ins)) {
					inspect = {
						restart_count: ins.restart_count ?? ins.RestartCount ?? 0,
						network_mode: ins.network_mode ?? ins.NetworkMode ?? 'bridge',
						ip_address: ins.ip_address ?? ins.IPAddress ?? '—',
						raw: JSON.stringify(ins, null, 2),
					};
				}
			}
		} catch {}
		statsLoading = false;
	}

	function setTab(tab) {
		const url = new URL($page.url);
		url.searchParams.set('tab', tab);
		goto(url.pathname + url.search, { replaceState: true, noScroll: true });

		// Load data per tab
		if (tab === 'logs' && !logs) loadLogs();
		if (tab === 'security' && !security) loadSecurity();
		if ((tab === 'stats') && statsLoading === false) loadStats();
	}

	async function loadLogs() {
		logsLoading = true;
		try {
			const result = await api.containers.logs(containerId, serverId, 100);
			logs = result?.logs || result || 'No logs available';
		} catch {
			logs = 'Failed to load logs';
		}
		logsLoading = false;
	}

	async function loadSecurity() {
		securityLoading = true;
		try {
			const secData = await api.containers.security(containerId, serverId);
			security = secData?.security || secData;
		} catch {
			security = null;
		}
		securityLoading = false;
	}

	async function refreshStats() {
		statsLoading = true;
		try {
			const s = await api.containers.stats(containerId, serverId);
			stats = s;
		} catch {}
		statsLoading = false;
	}

	async function refreshLogs() {
		logsLoading = true;
		try {
			const result = await api.containers.logs(containerId, serverId, 100);
			logs = result?.logs || result || 'No logs available';
		} catch {
			logs = 'Failed to load logs';
		}
		logsLoading = false;
	}

	// ── Container Actions ──────────────────────────────
	async function containerAction(action) {
		const key = containerId + '-' + action;
		actionLoading[key] = true;
		try {
			if (action === 'start') await api.containers.start(containerId, serverId);
			else if (action === 'stop') await api.containers.stop(containerId, serverId);
			else if (action === 'restart') await api.containers.restart(containerId, serverId);
			// Reload container data
			await loadData();
		} catch (_) {}
		actionLoading[key] = false;
	}

	function confirmAction(action) {
		const labels = { start: 'Start', stop: 'Stop', restart: 'Restart' };
		confirmModal = {
			show: true,
			title: `${labels[action]} Container`,
			message: `Confirm ${action} container "${container?.name}"?`,
			danger: action === 'stop',
			action: async () => {
				await containerAction(action);
				confirmModal = { show: false, title: '', message: '', action: null, danger: false };
			},
		};
	}

	// ── Security Scan ──────────────────────────────────
	async function runScan() {
		scanning = true;
		scanElapsed = 0;
		scanTimer = setInterval(() => { scanElapsed++; }, 1000);
		try {
			await api.compliance.scanContainer(serverId, containerId);
			for (let i = 0; i < 60; i++) {
				await new Promise(r => setTimeout(r, 2000));
				try {
					const latest = await api.compliance.latest(serverId, { scan_type: 'Container Security' });
					if (latest && latest.status === 'completed') {
						await loadSecurity();
						break;
					}
					if (latest && latest.status === 'failed') break;
				} catch (_) {}
			}
		} catch (_) {}
		clearInterval(scanTimer);
		scanning = false;
	}

	// ── Exec Terminal ──────────────────────────────────
	function initExecTerm(div) {
		destroyExecTerm();

		const term = new Terminal({
			cursorBlink: true,
			cursorStyle: 'block',
			fontSize: 13,
			fontFamily: "'JetBrains Mono', 'Cascadia Code', 'Fira Code', 'Consolas', monospace",
			theme: {
				background: '#0f172a',
				foreground: '#d4d4d4',
				cursor: '#10b981',
				cursorAccent: '#0f172a',
				selectionBackground: '#10b98140',
				black: '#1a1d23', red: '#ef4444', green: '#10b981', yellow: '#f59e0b',
				blue: '#3b82f6', magenta: '#a855f7', cyan: '#06b6d4', white: '#d4d4d4',
				brightBlack: '#4a4d55', brightRed: '#ef4444', brightGreen: '#34d399',
				brightYellow: '#fbbf24', brightBlue: '#60a5fa', brightMagenta: '#c084fc',
				brightCyan: '#22d3ee', brightWhite: '#f4f4f5',
			},
			allowTransparency: true,
			cols: 80,
			rows: 15,
		});

		const fitAddon = new FitAddon();
		term.loadAddon(fitAddon);
		term.open(div);
		fitAddon.fit();

		const token = localStorage.getItem('access_token');
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const wsUrl = `${protocol}//${window.location.host}/api/v1/servers/${serverId}/containers/${containerId}/exec-ws?token=${token}`;

		let ws;
		let isClosing = false;

		function connect() {
			ws = new WebSocket(wsUrl);
			ws.onopen = () => {
				term.focus();
				const dims = fitAddon.proposeDimensions();
				if (dims) {
					ws.send(JSON.stringify({ resize: true, cols: dims.cols, rows: dims.rows }));
				}
			};
			ws.onmessage = (evt) => { term.write(evt.data); };
			ws.onclose = () => {
				if (!isClosing) term.writeln('\r\n\x1b[33m[Disconnected]\x1b[0m');
			};
			ws.onerror = () => { term.writeln('\r\n\x1b[31m[WebSocket error]\x1b[0m'); };
		}

		connect();
		term.onData((data) => {
			if (ws && ws.readyState === WebSocket.OPEN) ws.send(data);
		});
		term.onResize(({ cols, rows }) => {
			if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ resize: true, cols, rows }));
			try { fitAddon.fit(); } catch(e) {}
		});
		const ro = new ResizeObserver(() => { try { fitAddon.fit(); } catch(e) {} });
		ro.observe(div);

		execTermInstance = { term, fitAddon, ws, ro, isClosing: () => isClosing };
	}

	function destroyExecTerm() {
		const t = execTermInstance;
		if (!t) return;
		t.isClosing = true;
		if (t.ws) t.ws.close();
		if (t.ro) t.ro.disconnect();
		try { t.term.dispose(); } catch(e) {}
		execTermInstance = null;
	}

	function execAction(node) {
		initExecTerm(node);
		return { destroy() {} };
	}

	// ── Switcher Navigation ────────────────────────────
	const filteredContainers = $derived.by(() => {
		const sv = allContainers.find(s => s.server.id === serverId);
		return sv?.containers || [];
	});

	function navigateToContainer(sid, cid) {
		serverDropdownOpen = false;
		containerDropdownOpen = false;
		if (cid === containerId && sid === serverId) return;
		// Clean up exec terminal before navigating
		destroyExecTerm();
		goto(`/containers/${sid}/${cid}?tab=${activeTab}`);
	}

	function changeServer(sid) {
		serverId = sid;
		const sv = allContainers.find(s => s.server.id === sid);
		if (sv?.containers?.length) {
			navigateToContainer(sid, sv.containers[0].id);
		}
	}

	// ── Lifecycle ──────────────────────────────────────
	onMount(() => {
		loadData();
		return () => {
			destroyExecTerm();
			if (scanTimer) clearInterval(scanTimer);
		};
	});

	$effect(() => {
		const sid = $page.params.serverId;
		const cid = $page.params.containerId;
		if (sid && cid && (sid !== serverId || cid !== containerId)) {
			destroyExecTerm();
			loadData();
		}
	});
</script>

<div class="container-detail-page">
	<!-- Breadcrumb -->
	<nav class="breadcrumb">
		<a href="/dashboard">Dashboard</a>
		<span class="crumb-sep">›</span>
		<a href="/containers">Containers</a>
		<span class="crumb-sep">›</span>
		<span class="current">{container?.name || containerId?.substring(0, 12) || 'Container'}</span>
	</nav>

	<!-- ─── Switcher Bar ─── -->
	<div class="switcher-bar">
		<div class="switcher-group">
			<label class="switcher-label">Server</label>
			<div class="dropdown" class:open={serverDropdownOpen}>
				<button class="dropdown-trigger" onclick={() => { serverDropdownOpen = !serverDropdownOpen; containerDropdownOpen = false; }}>
					<Icon icon="solar:server-bold" class="dropdown-icon" />
					<span class="dropdown-text">{server?.name || 'Select server...'}</span>
					<Icon icon="solar:alt-arrow-down-bold" class="dropdown-chevron" />
				</button>
				{#if serverDropdownOpen}
					<div class="dropdown-menu">
						{#each allContainers as sv (sv.server.id)}
							<button class="dropdown-item" class:active={sv.server.id === serverId}
								onclick={() => changeServer(sv.server.id)}>
								<span class="item-name">{sv.server.name}</span>
								<span class="item-meta">{sv.server.host} · {sv.containers?.length || 0} containers</span>
							</button>
						{/each}
					</div>
				{/if}
			</div>
		</div>
		<div class="switcher-group">
			<label class="switcher-label">Container</label>
			<div class="dropdown" class:open={containerDropdownOpen}>
				<button class="dropdown-trigger" onclick={() => { containerDropdownOpen = !containerDropdownOpen; serverDropdownOpen = false; }}>
					<Icon icon="solar:box-bold" class="dropdown-icon" />
					<span class="dropdown-text">{container?.name || 'Select container...'}</span>
					<Icon icon="solar:alt-arrow-down-bold" class="dropdown-chevron" />
				</button>
				{#if containerDropdownOpen}
					<div class="dropdown-menu containers-menu">
						{#each filteredContainers as ctr (ctr.id)}
							<button class="dropdown-item" class:active={ctr.id === containerId}
								onclick={() => navigateToContainer(serverId, ctr.id)}>
								<span class="item-name">{ctr.name}</span>
								<span class="item-meta" style="color: {ctr.state === 'running' ? 'var(--color-success)' : 'var(--color-text-muted)'};">{ctr.state}</span>
							</button>
						{/each}
						{#if filteredContainers.length === 0}
							<div class="dropdown-empty">No containers on this server</div>
						{/if}
					</div>
				{/if}
			</div>
		</div>
	</div>

	<!-- ─── Loading ─── -->
	{#if loading}
		<div class="loading-state">
			<Icon icon="solar:spinner-bold" class="spinner" />
			<span>Loading container details...</span>
		</div>
	{:else if error}
		<div class="error-state">
			<Icon icon="solar:danger-triangle-bold" style="color: var(--color-danger);" />
			<p>{error}</p>
			<button class="btn btn-outline btn-sm" onclick={loadData}>Retry</button>
		</div>
	{:else if container}
		<!-- ─── Container Header Card ─── -->
		<div class="header-card">
			<div class="header-left">
				<div class="header-title-row">
					<h1 class="header-title">{container.name}</h1>
					<span
						class="state-badge"
						style="background-color: {stateBg(container.state)}; color: {stateColor(container.state)};"
					>
						<span class="state-dot" style="background-color: {stateColor(container.state)}; box-shadow: 0 0 4px {stateColor(container.state)};"></span>
						{container.state === 'running' ? 'Running' : container.state === 'exited' || container.state === 'stopped' ? 'Stopped' : container.state === 'paused' ? 'Paused' : container.state || 'Unknown'}
					</span>
				</div>
				<div class="header-meta">
					<span class="meta-item">
						<Icon icon="solar:box-bold" class="meta-icon" />
						{container.image || '—'}
					</span>
					{#if container.ports}
						<span class="meta-item">
							<Icon icon="solar:plug-circle-bold" class="meta-icon" />
							{container.ports}
						</span>
					{/if}
					<span class="meta-item">
						<Icon icon="solar:hashtag-square-bold" class="meta-icon" />
						{shortId(container.id || containerId)}
					</span>
				</div>
			</div>
			<div class="header-right">
				<div class="header-actions">
					<button onclick={() => confirmAction('start')}
						class="action-btn action-start"
						title="Start container">
						<Icon icon={actionLoading[containerId + '-start'] ? 'solar:spinner-bold' : 'solar:play-bold'}
							class="h-4 w-4 {actionLoading[containerId + '-start'] ? 'animate-spin' : ''}" />
					</button>
					<button onclick={() => confirmAction('stop')}
						class="action-btn action-stop"
						title="Stop container">
						<Icon icon={actionLoading[containerId + '-stop'] ? 'solar:spinner-bold' : 'solar:pause-bold'}
							class="h-4 w-4 {actionLoading[containerId + '-stop'] ? 'animate-spin' : ''}" />
					</button>
					<button onclick={() => confirmAction('restart')}
						class="action-btn action-restart"
						title="Restart container">
						<Icon icon={actionLoading[containerId + '-restart'] ? 'solar:spinner-bold' : 'solar:refresh-bold'}
							class="h-4 w-4 {actionLoading[containerId + '-restart'] ? 'animate-spin' : ''}" />
					</button>
					<button onclick={loadData}
						class="action-btn action-refresh"
						title="Refresh">
						<Icon icon="solar:refresh-bold" class="h-4 w-4" />
					</button>
				</div>
			</div>
		</div>

		<!-- ─── Tab Navigation ─── -->
		<div class="tabs">
			<button onclick={() => setTab('stats')}
				class="tab-btn" class:active={activeTab === 'stats'}
				style="border-bottom-color: {activeTab === 'stats' ? 'var(--color-primary)' : 'transparent'}; color: {activeTab === 'stats' ? 'var(--color-primary)' : 'var(--color-text-muted)'};">
				<Icon icon="solar:chart-2-bold" class="tab-icon" /> Stats
			</button>
			<button onclick={() => setTab('logs')}
				class="tab-btn" class:active={activeTab === 'logs'}
				style="border-bottom-color: {activeTab === 'logs' ? 'var(--color-primary)' : 'transparent'}; color: {activeTab === 'logs' ? 'var(--color-primary)' : 'var(--color-text-muted)'};">
				<Icon icon="solar:document-text-bold" class="tab-icon" /> Logs
			</button>
			<button onclick={() => setTab('security')}
				class="tab-btn" class:active={activeTab === 'security'}
				style="border-bottom-color: {activeTab === 'security' ? 'var(--color-primary)' : 'transparent'}; color: {activeTab === 'security' ? 'var(--color-primary)' : 'var(--color-text-muted)'};">
				<Icon icon="solar:shield-bold" class="tab-icon" /> Security
			</button>
			<button onclick={() => setTab('exec')}
				class="tab-btn" class:active={activeTab === 'exec'}
				style="border-bottom-color: {activeTab === 'exec' ? 'var(--color-primary)' : 'transparent'}; color: {activeTab === 'exec' ? 'var(--color-primary)' : 'var(--color-text-muted)'};">
				<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5" /><line x1="12" y1="19" x2="20" y2="19" /></svg>
				Exec
			</button>
			<button onclick={() => setTab('inspect')}
				class="tab-btn" class:active={activeTab === 'inspect'}
				style="border-bottom-color: {activeTab === 'inspect' ? 'var(--color-primary)' : 'transparent'}; color: {activeTab === 'inspect' ? 'var(--color-primary)' : 'var(--color-text-muted)'};">
				<Icon icon="solar:code-bold" class="tab-icon" /> Inspect
			</button>
		</div>

		<!-- ─── Tab: Stats ─── -->
		{#if activeTab === 'stats'}
			<div class="tab-content">
				<!-- Resource Usage -->
				<div class="section-card">
					<div class="section-header">
						<h2 class="section-title">
							<Icon icon="solar:chart-2-bold" class="section-icon" /> Resource Usage
						</h2>
						<button onclick={refreshStats} disabled={statsLoading}
							class="refresh-btn">
							<Icon icon={statsLoading ? 'solar:spinner-bold' : 'solar:refresh-bold'}
								class="h-3.5 w-3.5 {statsLoading ? 'animate-spin' : ''}" />
							Refresh
						</button>
					</div>
					{#if statsLoading}
						<div class="section-loading">
							<Icon icon="solar:spinner-bold" class="spinner-sm" /> Loading stats...
						</div>
					{:else if stats}
						<div class="stats-grid">
							<div class="stat-bar-item">
								<div class="stat-bar-label">
									<Icon icon="solar:cpu-bold" class="stat-bar-icon" /> CPU
									<span class="stat-bar-value">{stats.cpu_percent != null ? stats.cpu_percent.toFixed(1) + '%' : '—'}</span>
								</div>
								<div class="stat-bar-track">
									<div class="stat-bar-fill"
										style="width: {Math.min(stats.cpu_percent ?? 0, 100)}%; background: {(stats.cpu_percent ?? 0) > 80 ? 'var(--color-danger)' : (stats.cpu_percent ?? 0) > 60 ? '#eab308' : 'var(--color-success)'};">
									</div>
								</div>
							</div>
							<div class="stat-bar-item">
								{@const memPct = calcMemPct(stats)}
								<div class="stat-bar-label">
									<Icon icon="solar:sd-card-bold" class="stat-bar-icon" /> Memory
									<span class="stat-bar-value">{stats.memory_usage != null ? formatBytes(stats.memory_usage) : '—'} / {stats.memory_limit ? formatBytes(stats.memory_limit) : '—'}</span>
								</div>
								<div class="stat-bar-track">
									<div class="stat-bar-fill"
										style="width: {Math.min(memPct ?? 0, 100)}%; background: {(memPct ?? 0) > 80 ? 'var(--color-danger)' : (memPct ?? 0) > 60 ? '#eab308' : '#f59e0b'};">
									</div>
								</div>
							</div>
						</div>
						<div class="network-grid">
							<div class="network-item">
								<Icon icon="solar:download-square-bold" class="net-icon" />
								<div>
									<p class="net-label">RX</p>
									<p class="net-value">{stats.net_rx != null ? formatBytes(stats.net_rx) : '—'}</p>
								</div>
							</div>
							<div class="network-item">
								<Icon icon="solar:upload-square-bold" class="net-icon" />
								<div>
									<p class="net-label">TX</p>
									<p class="net-value">{stats.net_tx != null ? formatBytes(stats.net_tx) : '—'}</p>
								</div>
							</div>
							<div class="network-item">
								<Icon icon="solar:document-text-bold" class="net-icon" />
								<div>
									<p class="net-label">Block Read</p>
									<p class="net-value">{stats.block_read != null ? formatBytes(stats.block_read) : '—'}</p>
								</div>
							</div>
							<div class="network-item">
								<Icon icon="solar:pen-bold" class="net-icon" />
								<div>
									<p class="net-label">Block Write</p>
									<p class="net-value">{stats.block_write != null ? formatBytes(stats.block_write) : '—'}</p>
								</div>
							</div>
						</div>
					{:else}
						<div class="section-empty">No stats available. Container may not be running.</div>
					{/if}
				</div>

				<!-- Container Info -->
				<div class="section-card">
					<h2 class="section-title">
						<Icon icon="solar:info-circle-bold" class="section-icon" /> Container Info
					</h2>
					<div class="info-grid">
						<div class="info-item">
							<span class="info-label">Image</span>
							<span class="info-value">
								{#if registryLink(container.image)}
									<button onclick={() => goto(registryLink(container.image))}
										class="link-value">
										{container.image}
									</button>
								{:else}
									{container.image || '—'}
								{/if}
							</span>
						</div>
						<div class="info-item">
							<span class="info-label">Status</span>
							<span class="info-value">{formatUptime(container)}</span>
						</div>
						<div class="info-item">
							<span class="info-label">Created</span>
							<span class="info-value">{container.created || '—'}</span>
						</div>
						<div class="info-item">
							<span class="info-label">Ports</span>
							<span class="info-value">{container.ports || '—'}</span>
						</div>
						<div class="info-item">
							<span class="info-label">Restarts</span>
							<span class="info-value">{inspect?.restart_count ?? '—'} restarts</span>
						</div>
						<div class="info-item">
							<span class="info-label">Network</span>
							<span class="info-value">{inspect?.network_mode || 'bridge'} · {inspect?.ip_address || '—'}</span>
						</div>
					</div>
				</div>

				<!-- Security Score Summary -->
				{#if container.security?.score != null || container.security?.findings?.length}
					<div class="section-card" onclick={() => setTab('security')} role="button" tabindex="0"
						onkeydown={(e) => e.key === 'Enter' && setTab('security')}
						style="cursor: pointer;">
						<div class="section-header">
							<h2 class="section-title">
								<Icon icon="solar:shield-bold" class="section-icon" /> Security
							</h2>
							<Icon icon="solar:alt-arrow-right-bold" class="chevron-icon" />
						</div>
						<div class="security-summary">
							{@const score = container.security?.score}
							{@const findings = container.security?.findings || []}
							{#if score != null}
								<div class="score-badge-large" style="background-color: {securityColor(score)}22; color: {securityColor(score)};">
									{score}/100
								</div>
							{/if}
							<div class="findings-summary">
								{@const critical = findings.filter(f => f.severity?.toLowerCase() === 'critical').length}
								{@const high = findings.filter(f => f.severity?.toLowerCase() === 'high').length}
								{@const medium = findings.filter(f => f.severity?.toLowerCase() === 'medium').length}
								<div class="finding-dots">
									{#if critical > 0}<span class="sev-dot" style="background: #ef4444;" title="{critical} critical">{critical}</span>{/if}
									{#if high > 0}<span class="sev-dot" style="background: #f97316;" title="{high} high">{high}</span>{/if}
									{#if medium > 0}<span class="sev-dot" style="background: #eab308;" title="{medium} medium">{medium}</span>{/if}
									{#if critical === 0 && high === 0 && medium === 0}
										<span class="text-xs" style="color: var(--color-text-muted);">No critical findings</span>
									{/if}
								</div>
							</div>
						</div>
					</div>
				{/if}
			</div>
		{/if}

		<!-- ─── Tab: Logs ─── -->
		{#if activeTab === 'logs'}
			<div class="tab-content">
				<div class="section-card">
					<div class="section-header">
						<h2 class="section-title">
							<Icon icon="solar:document-text-bold" class="section-icon" /> Container Logs
						</h2>
						<div class="section-header-actions">
							{#if logs && logs !== 'No logs available' && logs !== 'Failed to load logs'}
								<button onclick={() => setTab('exec')}
									class="text-xs font-medium underline underline-offset-2 transition-all hover:opacity-70"
									style="color: var(--color-text-muted);">
									Need interactive shell? → Exec
								</button>
							{/if}
						</div>
					</div>
					{#if logsLoading}
						<div class="section-loading">
							<Icon icon="solar:spinner-bold" class="spinner-sm" /> Loading logs...
						</div>
					{:else}
						<div class="logs-container">
							{@html formatLogsForDisplay(logs)}
						</div>
					{/if}
				</div>
			</div>
		{/if}

		<!-- ─── Tab: Security ─── -->
		{#if activeTab === 'security'}
			<div class="tab-content">
				<div class="section-card">
					<div class="section-header">
						<h2 class="section-title">
							<Icon icon="solar:shield-bold" class="section-icon" /> Container Security
						</h2>
						<div class="section-header-actions">
							<button onclick={runScan} disabled={scanning}
								class="btn btn-primary btn-sm">
								<Icon icon={scanning ? 'solar:spinner-bold' : 'solar:shield-bold'}
									class="h-3.5 w-3.5 {scanning ? 'animate-spin' : ''}" />
								{scanning ? 'Scanning...' : 'Rescan'}
							</button>
							{#if security?.security || container.security?.score != null}
								<a href={$page.url.pathname + '/security'} class="btn btn-outline btn-sm">
									<Icon icon="solar:export-bold" class="h-3.5 w-3.5" /> Full Report
								</a>
							{/if}
						</div>
					</div>

					{#if scanning}
						<div class="scanning-banner">
							<Icon icon="solar:spinner-bold" class="animate-spin" style="color: #fbbf24;" />
							<span>Scanning container security configuration... ({scanElapsed}s)</span>
						</div>
					{:else if securityLoading}
						<div class="section-loading">
							<Icon icon="solar:spinner-bold" class="spinner-sm" /> Loading security data...
						</div>
					{:else}
						{@const sec = security?.security || container?.security || security}
						{@const score = sec?.score ?? null}
						{@const findings = sec?.findings || []}
						{@const badges = sec?.badges || []}

						{#if score != null}
							<div class="security-score-section">
								<div class="score-display" style="color: {securityColor(score)};">
									<span class="score-number">{score}</span>
									<span class="score-divider">/</span>
									<span class="score-total">100</span>
								</div>
								<div class="score-details">
									{#if badges.length > 0}
										<div class="badges-row">
											{#each badges as badge}
												<span class="badge-tag">{badge}</span>
											{/each}
										</div>
									{/if}
								</div>
							</div>
						{/if}

						{#if findings.length > 0}
							<div class="findings-list">
								{#each findings as finding}
									<div class="finding-card" style="border-left: 3px solid {severityColor(finding.severity)};">
										<div class="finding-header">
											<span class="finding-severity" style="background-color: {severityBg(finding.severity)}; color: {severityColor(finding.severity)};">
												{finding.severity}
											</span>
											<span class="finding-title">{finding.title}</span>
											<span class="finding-check-id">{finding.check_id}</span>
										</div>
										<p class="finding-desc">{finding.description}</p>
										{#if finding.remediation}
											<p class="finding-fix">
												<strong>Fix:</strong> {finding.remediation}
											</p>
										{/if}
									</div>
								{/each}
							</div>
						{:else if score == null}
							<div class="section-empty">
								<Icon icon="solar:shield-warning-bold" class="h-8 w-8" style="color: var(--color-text-muted);" />
								<p>No security scan data available. Run a scan to see results.</p>
							</div>
						{:else}
							<div class="section-empty">
								<Icon icon="solar:shield-check-bold" class="h-8 w-8" style="color: var(--color-success);" />
								<p>No findings — container passed all security checks.</p>
							</div>
						{/if}
					{/if}
				</div>
			</div>
		{/if}

		<!-- ─── Tab: Exec ─── -->
		{#if activeTab === 'exec'}
			<div class="tab-content">
				<div class="section-card">
					<div class="section-header">
						<h2 class="section-title">
							<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5" /><line x1="12" y1="19" x2="20" y2="19" /></svg>
							Interactive Terminal
						</h2>
						<span class="text-xs" style="color: var(--color-text-muted);">Type any command — connected via WebSocket</span>
					</div>
					<div
						use:execAction
						class="exec-terminal-container"
					></div>
				</div>
			</div>
		{/if}

		<!-- ─── Tab: Inspect ─── -->
		{#if activeTab === 'inspect'}
			<div class="tab-content">
				<div class="section-card">
					<div class="section-header">
						<h2 class="section-title">
							<Icon icon="solar:code-bold" class="section-icon" /> Inspect Data (Raw JSON)
						</h2>
						<button onclick={() => loadStats()}
							class="refresh-btn">
							<Icon icon="solar:refresh-bold" class="h-3.5 w-3.5" /> Reload
						</button>
					</div>
					{#if inspect?.raw}
						<pre class="inspect-json">{inspect.raw}</pre>
					{:else if statsLoading}
						<div class="section-loading">
							<Icon icon="solar:spinner-bold" class="spinner-sm" /> Loading inspect data...
						</div>
					{:else}
						<div class="section-empty">No inspect data available.</div>
					{/if}
				</div>
			</div>
		{/if}

	{/if}
</div>

<!-- ─── Confirmation Modal ─── -->
{#if confirmModal.show}
	<div class="modal-overlay" onclick={() => confirmModal = { show: false, title: '', message: '', action: null, danger: false }}>
		<div class="modal-content" onclick={(e) => e.stopPropagation()}>
			<div class="modal-header">
				<div class="modal-icon-wrap" class:danger={confirmModal.danger}>
					<Icon icon={confirmModal.danger ? 'solar:danger-triangle-bold' : 'solar:info-circle-bold'}
						class="h-5 w-5" style="color: {confirmModal.danger ? 'var(--color-danger)' : 'var(--color-primary)'};" />
				</div>
				<h3 class="modal-title">{confirmModal.title}</h3>
			</div>
			<p class="modal-message">{confirmModal.message}</p>
			<div class="modal-actions">
				<button onclick={() => confirmModal = { show: false, title: '', message: '', action: null, danger: false }}
					class="btn btn-secondary btn-sm">Cancel</button>
				<button onclick={() => confirmModal.action?.()}
					class="btn btn-sm" class:btn-danger={confirmModal.danger} class:btn-primary={!confirmModal.danger}>
					{confirmModal.title}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	/* ── Layout ── */
	.container-detail-page {
		max-width: 960px;
		margin: 0 auto;
		padding: 24px 20px;
	}

	/* ── Breadcrumb ── */
	.breadcrumb {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-bottom: 16px;
		font-size: 12px;
	}
	.breadcrumb a {
		color: var(--text-muted);
		text-decoration: none;
		transition: color 0.15s;
	}
	.breadcrumb a:hover { color: var(--color-primary); }
	.crumb-sep { color: var(--text-muted); font-size: 10px; }
	.breadcrumb .current { color: var(--text-secondary); font-weight: 500; }

	/* ── Switcher ── */
	.switcher-bar {
		display: flex;
		gap: 12px;
		margin-bottom: 16px;
		flex-wrap: wrap;
	}
	.switcher-group {
		flex: 1;
		min-width: 200px;
	}
	.switcher-label {
		display: block;
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		color: var(--text-muted);
		margin-bottom: 4px;
	}
	.dropdown { position: relative; }
	.dropdown-trigger {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		padding: 7px 12px;
		border-radius: 8px;
		border: 1px solid var(--color-border-light);
		background: var(--color-surface);
		color: var(--text);
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		transition: border-color 0.15s;
	}
	.dropdown-trigger:hover { border-color: var(--color-primary); }
	.dropdown-icon { width: 16px; height: 16px; flex-shrink: 0; color: var(--text-muted); }
	.dropdown-text { flex: 1; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.dropdown-chevron { width: 14px; height: 14px; flex-shrink: 0; color: var(--text-muted); }
	.dropdown-menu {
		position: absolute;
		top: calc(100% + 4px);
		left: 0;
		right: 0;
		z-index: 50;
		max-height: 240px;
		overflow-y: auto;
		border-radius: 8px;
		border: 1px solid var(--color-border);
		background: var(--color-card);
		box-shadow: 0 8px 24px rgba(0,0,0,0.3);
	}
	.dropdown-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		padding: 8px 12px;
		border: none;
		background: transparent;
		color: var(--text);
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
		transition: background 0.1s;
	}
	.dropdown-item:hover { background: rgba(16,185,129,0.08); }
	.dropdown-item.active { background: rgba(16,185,129,0.12); color: var(--color-primary); }
	.item-name { text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.item-meta { font-size: 10px; color: var(--text-muted); white-space: nowrap; }
	.dropdown-empty {
		padding: 12px;
		text-align: center;
		font-size: 11px;
		color: var(--text-muted);
	}
	.containers-menu .dropdown-item { padding: 6px 10px; }

	/* ── Loading/Error states ── */
	.loading-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 48px 0;
		color: var(--text-muted);
		font-size: 13px;
	}
	.spinner { width: 20px; height: 20px; animation: spin 1s linear infinite; }
	@keyframes spin { to { transform: rotate(360deg); } }
	.error-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
		padding: 48px 0;
		color: var(--color-danger);
		font-size: 13px;
	}

	/* ── Header Card ── */
	.header-card {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
		padding: 20px;
		border-radius: 12px;
		border: 1px solid var(--color-border);
		background: var(--color-card);
		margin-bottom: 16px;
	}
	.header-left { min-width: 0; flex: 1; }
	.header-title-row {
		display: flex;
		align-items: center;
		gap: 10px;
		margin-bottom: 6px;
	}
	.header-title {
		font-size: 18px;
		font-weight: 700;
		color: var(--text);
		margin: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.state-badge {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 3px 10px;
		border-radius: 20px;
		font-size: 11px;
		font-weight: 700;
		white-space: nowrap;
		flex-shrink: 0;
	}
	.state-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
	}
	.header-meta {
		display: flex;
		flex-wrap: wrap;
		gap: 12px;
		margin-top: 4px;
	}
	.meta-item {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-size: 12px;
		color: var(--text-secondary);
	}
	.meta-icon { width: 14px; height: 14px; color: var(--text-muted); flex-shrink: 0; }
	.header-right { flex-shrink: 0; }
	.header-actions {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.action-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		border-radius: 8px;
		border: 1px solid var(--color-border-light);
		background: transparent;
		color: var(--text-secondary);
		cursor: pointer;
		transition: all 0.15s;
	}
	.action-btn:hover { border-color: var(--color-primary); color: var(--color-primary); }
	.action-start:hover { border-color: var(--color-success); color: var(--color-success); }
	.action-stop:hover { border-color: var(--color-danger); color: var(--color-danger); }
	.action-restart:hover { border-color: #eab308; color: #eab308; }
	.action-refresh:hover { border-color: var(--color-primary); color: var(--color-primary); }

	/* ── Tabs ── */
	.tabs {
		display: flex;
		align-items: center;
		gap: 0;
		margin-bottom: 16px;
		border-bottom: 1px solid var(--color-border-light);
		overflow-x: auto;
	}
	.tab-btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 10px 16px;
		font-size: 12px;
		font-weight: 600;
		border: none;
		border-bottom: 2px solid transparent;
		background: transparent;
		cursor: pointer;
		transition: all 0.15s;
		white-space: nowrap;
	}
	.tab-btn:hover {
		color: var(--color-primary) !important;
		opacity: 0.8;
	}
	.tab-icon { width: 16px; height: 16px; }

	/* ── Tab Content ── */
	.tab-content {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	/* ── Section Card ── */
	.section-card {
		border-radius: 12px;
		border: 1px solid var(--color-border);
		background: var(--color-card);
		padding: 20px;
	}
	.section-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		margin-bottom: 16px;
	}
	.section-header-actions { display: flex; align-items: center; gap: 8px; }
	.section-title {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		font-weight: 700;
		color: var(--text);
		margin: 0;
	}
	.section-icon { width: 16px; height: 16px; color: var(--color-primary); }
	.section-loading {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 24px;
		justify-content: center;
		color: var(--text-muted);
		font-size: 12px;
	}
	.spinner-sm { width: 14px; height: 14px; animation: spin 1s linear infinite; }
	.section-empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
		padding: 32px 16px;
		text-align: center;
		color: var(--text-muted);
		font-size: 12px;
	}
	.refresh-btn {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-size: 11px;
		font-weight: 500;
		color: var(--text-muted);
		background: transparent;
		border: none;
		cursor: pointer;
		transition: opacity 0.15s;
	}
	.refresh-btn:hover { opacity: 0.7; }
	.chevron-icon { width: 16px; height: 16px; color: var(--text-muted); }

	/* ── Stats ── */
	.stats-grid {
		display: flex;
		flex-direction: column;
		gap: 16px;
		margin-bottom: 20px;
	}
	.stat-bar-item {}
	.stat-bar-label {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--text-secondary);
		margin-bottom: 6px;
	}
	.stat-bar-icon { width: 14px; height: 14px; color: var(--text-muted); }
	.stat-bar-value { margin-left: auto; font-weight: 600; font-family: monospace; color: var(--text); }
	.stat-bar-track {
		height: 8px;
		border-radius: 4px;
		background: var(--color-border-light);
		overflow: hidden;
	}
	.stat-bar-fill {
		height: 100%;
		border-radius: 4px;
		transition: width 0.5s ease;
	}
	.network-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 10px;
	}
	.network-item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 12px;
		border-radius: 8px;
		background: var(--color-surface);
	}
	.net-icon { width: 18px; height: 18px; color: var(--text-muted); flex-shrink: 0; }
	.net-label { font-size: 10px; font-weight: 500; color: var(--text-muted); margin: 0; }
	.net-value { font-size: 13px; font-weight: 700; font-family: monospace; color: var(--text); margin: 2px 0 0; }

	/* ── Info Grid ── */
	.info-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 12px;
	}
	.info-item {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.info-label {
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.3px;
		color: var(--text-muted);
	}
	.info-value {
		font-size: 13px;
		font-weight: 500;
		font-family: monospace;
		color: var(--text);
		word-break: break-all;
	}
	.link-value {
		color: var(--color-primary);
		cursor: pointer;
		background: none;
		border: none;
		font-family: monospace;
		font-size: 13px;
		padding: 0;
	}
	.link-value:hover { text-decoration: underline; }

	/* ── Security Summary ── */
	.security-summary {
		display: flex;
		align-items: center;
		gap: 16px;
	}
	.score-badge-large {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 4px 14px;
		border-radius: 8px;
		font-size: 16px;
		font-weight: 800;
		font-family: monospace;
	}
	.findings-summary {}
	.finding-dots {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.sev-dot {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 22px;
		height: 22px;
		border-radius: 6px;
		padding: 0 6px;
		font-size: 10px;
		font-weight: 800;
		color: #fff;
	}

	/* ── Logs ── */
	.logs-container {
		background: #0f172a;
		border-radius: 8px;
		padding: 16px;
		font-family: monospace;
		font-size: 11px;
		line-height: 1.6;
		max-height: 480px;
		overflow-y: auto;
	}

	/* ── Security Tab ── */
	.security-score-section {
		display: flex;
		align-items: center;
		gap: 16px;
		margin-bottom: 16px;
	}
	.score-display {
		display: flex;
		align-items: baseline;
		gap: 2px;
	}
	.score-number { font-size: 32px; font-weight: 800; font-family: monospace; }
	.score-divider { font-size: 18px; font-weight: 300; opacity: 0.5; }
	.score-total { font-size: 18px; font-weight: 600; opacity: 0.7; }
	.badges-row { display: flex; flex-wrap: wrap; gap: 4px; }
	.badge-tag {
		padding: 2px 8px;
		border-radius: 4px;
		font-size: 10px;
		font-weight: 600;
		background: rgba(148,163,184,0.1);
		color: rgba(148,163,184,0.9);
	}
	.scanning-banner {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 12px;
		border-radius: 8px;
		font-size: 12px;
		color: #fbbf24;
		background: rgba(245,158,11,0.06);
		border: 1px solid rgba(245,158,11,0.25);
	}
	.findings-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.finding-card {
		padding: 12px;
		border-radius: 8px;
		background: var(--color-surface);
		border-left: 3px solid transparent;
	}
	.finding-header {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 4px;
	}
	.finding-severity {
		padding: 1px 8px;
		border-radius: 4px;
		font-size: 9px;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.3px;
	}
	.finding-title { font-size: 12px; font-weight: 600; color: var(--text); }
	.finding-check-id { font-size: 10px; font-family: monospace; color: var(--text-muted); margin-left: auto; }
	.finding-desc { font-size: 11px; color: var(--text-secondary); margin: 4px 0; }
	.finding-fix { font-size: 10px; color: var(--color-primary); margin: 4px 0 0; }

	/* ── Exec ── */
	.exec-terminal-container {
		min-height: 300px;
		height: 400px;
		border-radius: 8px;
		overflow: hidden;
	}

	/* ── Inspect ── */
	.inspect-json {
		background: #0f172a;
		color: #a5b4fc;
		border-radius: 8px;
		padding: 16px;
		font-family: monospace;
		font-size: 10px;
		line-height: 1.5;
		max-height: 500px;
		overflow: auto;
		white-space: pre-wrap;
		word-break: break-all;
	}

	/* ── Modal ── */
	.modal-overlay {
		position: fixed;
		inset: 0;
		z-index: 50;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 16px;
		background: rgba(0,0,0,0.5);
	}
	.modal-content {
		width: 100%;
		max-width: 400px;
		border-radius: 12px;
		border: 1px solid var(--color-border);
		background: var(--color-card);
		box-shadow: 0 16px 48px rgba(0,0,0,0.4);
		padding: 24px;
	}
	.modal-header {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 12px;
	}
	.modal-icon-wrap {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 40px;
		height: 40px;
		border-radius: 50%;
		background: rgba(16,185,129,0.1);
		flex-shrink: 0;
	}
	.modal-icon-wrap.danger { background: rgba(239,68,68,0.1); }
	.modal-title { font-size: 16px; font-weight: 700; color: var(--text); margin: 0; }
	.modal-message { font-size: 13px; color: var(--text-secondary); margin-bottom: 20px; }
	.modal-actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
