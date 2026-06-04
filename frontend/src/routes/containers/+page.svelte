<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { Terminal } from 'xterm';
	import { FitAddon } from 'xterm-addon-fit';
	import 'xterm/css/xterm.css';

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
	let scanningContainer = $state(null); // container ID being scanned

	// ── Expand State ────────────────────────────────────
	let expandedServers = $state({});   // { [serverId]: true/false }
	let expanded = $state({});          // { [containerId]: { ... } }

	// ── Container Exec Terminal Instances ───────────────
	let execTerms = {};

	// ── Confirmation Modal ──────────────────────────────
	let confirmModal = $state({ show: false, title: '', message: '', action: null, danger: false });

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
			// Auto-expand server sections that have containers
			if (res?.servers) {
				const next = {};
				for (const s of res.servers) {
					if (s.containers?.length > 0) {
						next[s.server.id] = true;
					}
				}
				expandedServers = next;
			}
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
		// Apply filters
		if (scanStatusFilter === 'scanned') list = list.filter(c => c.security?.score != null);
		else if (scanStatusFilter === 'unscanned') list = list.filter(c => c.security?.score == null);
		if (serverFilter !== 'all') list = list.filter(c => c.server_id === serverFilter);
		if (searchQuery) {
			const q = searchQuery.toLowerCase();
			list = list.filter(c => c.name.toLowerCase().includes(q) || (c.image || '').toLowerCase().includes(q));
		}
		// Sort: unscanned last, then by score ascending (worst first)
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
		for (const [serverId, containers] of Object.entries(byServer)) {
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

			// Server filter
			if (serverFilter !== 'all') {
				containers = containers.filter(c => c.server_id === serverFilter);
			}

			// State filter
			if (stateFilter === 'running') containers = containers.filter(c => c.state === 'running');
			else if (stateFilter === 'stopped') containers = containers.filter(c => c.state === 'exited' || c.state === 'stopped');
			else if (stateFilter === 'paused') containers = containers.filter(c => c.state === 'paused');

			// Search
			if (searchQuery) {
				const q = searchQuery.toLowerCase();
				containers = containers.filter(c =>
					c.name.toLowerCase().includes(q) ||
					(c.image || '').toLowerCase().includes(q)
				);
			}

			// Sort
			containers.sort((a, b) => {
				switch (sortBy) {
					case 'score': {
						const sa = a.security?.score ?? -1;
						const sb = b.security?.score ?? -1;
						return sa - sb; // worst first
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
					default: { // name
						return (a.name || '').localeCompare(b.name || '');
					}
				}
			});

			return { ...sv, containers };
		}).filter(sv => sv.containers.length > 0);

		// Sort servers by name
		servers.sort((a, b) => (a.server?.name || '').localeCompare(b.server?.name || ''));

		return servers;
	});

	let totalFiltered = $derived(filteredData.reduce((acc, sv) => acc + sv.containers.length, 0));
	let totalScanned = $derived(filteredData.reduce((acc, sv) => {
		return acc + sv.containers.filter(c => c.security?.score != null).length;
	}, 0));

	function parseCreated(created) {
		if (!created) return 0;
		let normalized = created
			.replace(' +0700 WIB', '+07:00')
			.replace(' +0700', '+07:00')
			.replace(' +08', '+08:00')
			.replace(' +09', '+09:00')
			.replace(' UTC', 'Z')
			.replace(' ', 'T');
		if (normalized.endsWith('+07:00') || normalized.endsWith('+08:00') || normalized.endsWith('+09:00') || normalized.endsWith('Z')) {
		} else if (normalized.includes('+')) {
		} else if (normalized.endsWith('+07') || normalized.endsWith('+08') || normalized.endsWith('+09')) {
			normalized = normalized.slice(0, -3) + ':' + normalized.slice(-2);
		} else {
			normalized += 'Z';
		}
		const d = new Date(normalized);
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
				if (expanded[containerId]) {
					const allCtrs = data?.servers?.flatMap(s => s.containers) || [];
					const c = allCtrs.find(ct => ct.id === containerId);
					if (c) loadExpandedData(c);
				}
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

	// ── Multi-Expand Helpers ────────────────────────────
	function calcMemPct(stats) {
		if (!stats || !(stats.memory_limit ?? 0) > 0) return 0;
		return ((stats.memory_usage ?? 0) / (stats.memory_limit ?? 1) * 100);
	}

	function getEinfo(c) {
		return expanded[c.id];
	}

	function toggleExpand(c) {
		if (expanded[c.id]) {
			destroyContainerExecTerm(c.id);
			const next = { ...expanded };
			delete next[c.id];
			expanded = next;
			return;
		}
		expanded = {
			...expanded,
			[c.id]: { stats: null, inspect: null, logs: '', statsLoading: true, inspectLoading: true, logsLoading: false, showLogs: false, showExec: false, showInspect: false, inspectRaw: '', showSecurity: false }
		};
		loadExpandedData(c);
	}

	async function loadExpandedData(c) {
		expanded = {
			...expanded,
			[c.id]: { ...expanded[c.id], statsLoading: true, inspectLoading: true }
		};
		try {
			const [stats, inspect] = await Promise.all([
				api.containers.stats(c.id, c.server_id).catch(() => null),
				api.containers.get(c.id, c.server_id).catch(() => null),
			]);
			let parsedInspect = null;
			if (inspect && Array.isArray(inspect) && inspect.length > 0) {
				const data = inspect[0];
				parsedInspect = {
					restart_count: data.State?.RestartCount ?? 0,
					network_mode: data.HostConfig?.NetworkMode ?? 'bridge',
					ip_address: data.NetworkSettings?.Networks ? Object.values(data.NetworkSettings.Networks).find(n => n.IPAddress)?.IPAddress : '—',
				};
			} else if (inspect && !Array.isArray(inspect)) {
				parsedInspect = {
					restart_count: inspect.restart_count ?? inspect.RestartCount ?? 0,
					network_mode: inspect.network_mode ?? inspect.NetworkMode ?? 'bridge',
					ip_address: inspect.ip_address ?? inspect.IPAddress ?? '—',
				};
			}
			expanded = {
				...expanded,
				[c.id]: { ...expanded[c.id], stats, inspect: parsedInspect, statsLoading: false, inspectLoading: false }
			};
		} catch {
			expanded = {
				...expanded,
				[c.id]: { ...expanded[c.id], statsLoading: false, inspectLoading: false }
			};
		}
	}

	async function ensureExpanded(c) {
		if (!expanded[c.id]) {
			expanded = {
				...expanded,
				[c.id]: { stats: null, inspect: null, logs: '', statsLoading: true, inspectLoading: true, logsLoading: false, showLogs: false, showExec: false, showInspect: false, inspectRaw: '', showSecurity: false }
			};
			loadExpandedData(c);
		}
	}

	// ── Expanded views ─────────────────────────────────
	async function viewLogs(c) {
		await ensureExpanded(c);
		destroyContainerExecTerm(c.id);
		expanded = {
			...expanded,
			[c.id]: { ...expanded[c.id], showLogs: true, showExec: false, showInspect: false, showSecurity: false }
		};
		doFetchLogs(c.id);
	}

	function doFetchLogs(containerId) {
		if (!expanded[containerId]) return;
		expanded = { ...expanded, [containerId]: { ...expanded[containerId], logsLoading: true } };
		const allCtrs = data?.servers?.flatMap(s => s.containers) || [];
		const c = allCtrs.find(ct => ct.id === containerId);
		if (!c) return;
		api.servers.containerLogs(c.server_id, containerId).then(logs => {
			if (!expanded[containerId]) return;
			expanded = { ...expanded, [containerId]: { ...expanded[containerId], logs: logs?.logs || logs || 'No logs available', logsLoading: false } };
		}).catch(() => {
			if (!expanded[containerId]) return;
			expanded = { ...expanded, [containerId]: { ...expanded[containerId], logsLoading: false, logs: 'Failed to load logs' } };
		});
	}

	async function viewExec(c) {
		await ensureExpanded(c);
		destroyContainerExecTerm(c.id);
		expanded = {
			...expanded,
			[c.id]: { ...expanded[c.id], showLogs: false, showExec: true, showInspect: false, showSecurity: false }
		};
	}

	async function viewInspect(c) {
		await ensureExpanded(c);
		destroyContainerExecTerm(c.id);
		expanded = {
			...expanded,
			[c.id]: { ...expanded[c.id], showLogs: false, showExec: false, showInspect: true, showSecurity: false }
		};
		try {
			const data = await api.containers.get(c.id, c.server_id);
			expanded = { ...expanded, [c.id]: { ...expanded[c.id], inspectRaw: JSON.stringify(data, null, 2) } };
		} catch {
			expanded = { ...expanded, [c.id]: { ...expanded[c.id], inspectRaw: 'Failed to load inspect data' } };
		}
	}

	function viewSecurity(c) {
		ensureExpanded(c);
		destroyContainerExecTerm(c.id);
		expanded = {
			...expanded,
			[c.id]: { ...expanded[c.id], showLogs: false, showExec: false, showInspect: false, showSecurity: true }
		};
	}

	async function refreshStats(containerId) {
		if (!expanded[containerId]) return;
		const allCtrs = data?.servers?.flatMap(s => s.containers) || [];
		const c = allCtrs.find(ct => ct.id === containerId);
		if (!c) return;
		expanded = { ...expanded, [containerId]: { ...expanded[containerId], statsLoading: true } };
		try {
			const stats = await api.containers.stats(c.id, c.server_id);
			expanded = { ...expanded, [containerId]: { ...expanded[containerId], stats, statsLoading: false } };
		} catch {
			expanded = { ...expanded, [containerId]: { ...expanded[containerId], statsLoading: false } };
		}
	}

	// ── Scan All Servers ───────────────────────────────
	async function scanAllContainers() {
		scanningAll = true;
		const serverIds = data?.servers?.map(sv => sv.server.id) || [];
		const triggereds = [];
		for (const sid of serverIds) {
			try {
				const res = await api.compliance.scanContainers(sid);
				triggereds.push(res);
			} catch (_) {}
		}
		// Poll for completion — simple delay then reload
		await new Promise(r => setTimeout(r, 5000));
		await loadData();
		scanningAll = false;
	}

	// ── Scan Single Container ──────────────────────────
	async function scanSingleContainer(c) {
		scanningContainer = c.id;
		try {
			await api.compliance.scanContainer(c.server_id, c.id);
			// Wait a bit then reload data
			await new Promise(r => setTimeout(r, 3000));
			await loadData();
			// Re-expand security panel
			if (expanded[c.id]) {
				expanded = { ...expanded, [c.id]: { ...expanded[c.id], showSecurity: true, showLogs: false, showExec: false, showInspect: false } };
			}
		} catch (_) {}
		scanningContainer = null;
	}

	// ── Container Exec Terminal ────────────────────────
	function initContainerExecTerm(containerId, div) {
		destroyContainerExecTerm(containerId);

		const allCtrs = data?.servers?.flatMap(s => s.containers) || [];
		const c = allCtrs.find(ct => ct.id === containerId);
		if (!c) return;

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
				brightCyan: '#22d3ee', brightWhite: '#f4f4f5'
			},
			allowTransparency: true,
			cols: 80,
			rows: 12
		});

		const fitAddon = new FitAddon();
		term.loadAddon(fitAddon);
		term.open(div);
		fitAddon.fit();

		const token = localStorage.getItem('access_token');
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const wsUrl = `${protocol}//${window.location.host}/api/v1/servers/${c.server_id}/containers/${c.id}/exec-ws?token=${token}`;

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
			ws.onmessage = (evt) => {
				term.write(evt.data);
			};
			ws.onclose = () => {
				if (!isClosing) {
					term.writeln('\r\n\x1b[33m[Disconnected]\x1b[0m');
				}
			};
			ws.onerror = () => {
				term.writeln('\r\n\x1b[31m[WebSocket error]\x1b[0m');
			};
		}

		connect();

		term.onData((data) => {
			if (ws && ws.readyState === WebSocket.OPEN) {
				ws.send(data);
			}
		});

		term.onResize(({ cols, rows }) => {
			if (ws && ws.readyState === WebSocket.OPEN) {
				ws.send(JSON.stringify({ resize: true, cols, rows }));
			}
			try { fitAddon.fit(); } catch(e) {}
		});

		const ro = new ResizeObserver(() => {
			try { fitAddon.fit(); } catch(e) {}
		});
		ro.observe(div);

		execTerms[containerId] = { term, fitAddon, ws, ro, isClosing: () => isClosing };
	}

	function destroyContainerExecTerm(containerId) {
		const t = execTerms[containerId];
		if (!t) return;
		t.isClosing = true;
		if (t.ws) t.ws.close();
		if (t.ro) t.ro.disconnect();
		try { t.term.dispose(); } catch(e) {}
		delete execTerms[containerId];
	}

	function containerExecAction(node, containerId) {
		initContainerExecTerm(containerId, node);
		return {
			destroy() {}
		};
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

	function escapeHtml(str) {
		return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
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
		</div>
	</div>

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
						{#if avgScore != null}
							<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-bold"
								style="background-color: {securityBg(avgScore)}; color: {securityColor(avgScore)};">
								<Icon icon="solar:shield-bold" class="h-3 w-3" />
								{avgScore}
							</span>
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
					<div class="grid gap-3 p-3" style="grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));">
						{#each svContainers as c (c.id)}
							{@const col = serverColor(sv.server.name)}
							{@const isRunning = c.state === 'running'}
							{@const isStopped = c.state === 'exited' || c.state === 'stopped'}
							{@const isPaused = c.state === 'paused'}
							{@const isExpanded = !!expanded[c.id]}
							{@const ec = expanded[c.id]}
							{@const sec = c.security}
							{@const hasSecurity = sec?.score != null}
							{@const secScore = sec?.score ?? 0}

							<div
								class="rounded-xl border shadow-sm overflow-hidden transition-all hover:shadow-md hover:-translate-y-0.5 cursor-pointer"
								class:ring-1={isExpanded}
								style="background-color: var(--color-card); border-color: var(--color-border); {isExpanded ? 'box-shadow: 0 4px 12px rgba(0,0,0,0.2);' : ''}"
								onclick={() => toggleExpand(c)}
								role="button"
								tabindex="0"
								onkeydown={(e) => e.key === 'Enter' && toggleExpand(c)}
							>
								<div class="flex" style="min-height: 0;">
									<!-- State-based accent bar -->
									<div style="width: 4px; flex-shrink: 0; background: {isRunning ? 'var(--color-success)' : isStopped ? 'var(--color-danger)' : isPaused ? '#eab308' : 'var(--color-text-muted)'}; border-radius: 4px 0 0 4px;"></div>
									<div class="flex-1 min-w-0">
										<!-- Card Header -->
										<div class="flex items-start justify-between gap-2 px-4 pt-4 pb-2">
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
												<span class="text-[10px] font-mono" style="color: var(--color-text-muted);">#{shortId(c.id)}</span>
												<Icon
													icon={isExpanded ? 'solar:alt-arrow-up-bold' : 'solar:alt-arrow-down-bold'}
													class="h-3.5 w-3.5 transition-transform duration-200"
													style="color: var(--color-text-muted);"
												/>
											</div>
										</div>

										<!-- Card Body -->
										<div class="px-4 pb-2 space-y-1">
											<div class="flex items-center gap-2 text-xs" style="color: var(--color-text-secondary);">
												<Icon icon="solar:box-bold" class="h-3 w-3 shrink-0" style="color: var(--color-text-muted);" />
												<span class="truncate font-mono" title={c.image}>{c.image}</span>
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

										<!-- ═══════ EXPANDED SECTION ═══════ -->
										{#if isExpanded}
											{@const memPct = calcMemPct(ec.stats)}
											<div class="border-t" style="border-color: var(--color-border-light);" onclick={(e) => e.stopPropagation()}>
												<!-- Info Grid -->
												<div class="grid grid-cols-2 gap-x-4 gap-y-3 px-4 py-3" style="background-color: var(--color-surface);">
													<div>
														<p class="text-[10px] font-semibold uppercase tracking-wider mb-0.5" style="color: var(--color-text-muted);">IMAGE</p>
														<p class="text-xs font-mono truncate" style="color: var(--color-text);" title={c.image}>{c.image || '—'}</p>
													</div>
													<div>
														<p class="text-[10px] font-semibold uppercase tracking-wider mb-0.5" style="color: var(--color-text-muted);">PORTS</p>
														<p class="text-xs font-mono truncate" style="color: var(--color-text);">{c.ports || '—'}</p>
													</div>
													<div>
														<p class="text-[10px] font-semibold uppercase tracking-wider mb-0.5" style="color: var(--color-text-muted);">CREATED</p>
														<p class="text-xs font-mono truncate" style="color: var(--color-text);">{c.created || '—'}</p>
													</div>
													<div>
														<p class="text-[10px] font-semibold uppercase tracking-wider mb-0.5" style="color: var(--color-text-muted);">RESTARTS</p>
														<p class="text-xs font-mono" style="color: var(--color-text);">{ec.inspect?.restart_count ?? '—'} restarts</p>
													</div>
													<div class="col-span-2">
														<p class="text-[10px] font-semibold uppercase tracking-wider mb-0.5" style="color: var(--color-text-muted);">NETWORK</p>
														<p class="text-xs font-mono" style="color: var(--color-text);">{ec.inspect?.network_mode || 'bridge'} · {ec.inspect?.ip_address || '—'}</p>
													</div>
												</div>

												<!-- Resource Usage -->
												<div class="px-4 py-3 border-t" style="border-color: var(--color-border-light); background-color: var(--color-card);">
													<div class="flex items-center justify-between mb-2">
														<h3 class="text-[11px] font-bold flex items-center gap-1.5" style="color: var(--color-text);">
															<Icon icon="solar:chart-2-bold" class="h-3.5 w-3.5" style="color: var(--color-primary);" /> RESOURCE USAGE
														</h3>
														<button onclick={() => refreshStats(c.id)} disabled={ec.statsLoading}
															class="inline-flex items-center gap-1 text-[10px] font-medium transition-all hover:opacity-70"
															style="color: var(--color-text-muted);">
															<Icon icon={ec.statsLoading ? 'solar:spinner-bold' : 'solar:refresh-bold'}
																class="h-3 w-3 {ec.statsLoading ? 'animate-spin' : ''}" /> Refresh
														</button>
													</div>
													<div class="mb-2">
														<div class="flex items-center justify-between mb-0.5">
															<span class="text-[10px] font-medium" style="color: var(--color-text-secondary);">CPU</span>
															<span class="text-[10px] font-mono font-semibold" style="color: var(--color-text);">
																{ec.stats?.cpu_percent != null ? ec.stats.cpu_percent.toFixed(1) + '%' : ec.statsLoading ? '...' : '—'}
															</span>
														</div>
														<div class="h-1.5 rounded-full overflow-hidden" style="background-color: var(--color-border-light);">
															<div class="h-full rounded-full transition-all duration-500"
																style="width: {Math.min(ec.stats?.cpu_percent ?? 0, 100)}%; background-color: {(ec.stats?.cpu_percent ?? 0) > 80 ? 'var(--color-danger)' : (ec.stats?.cpu_percent ?? 0) > 60 ? 'var(--color-warning)' : 'var(--color-success)'};">
															</div>
														</div>
													</div>
													<div class="mb-2">
														<div class="flex items-center justify-between mb-0.5">
															<span class="text-[10px] font-medium" style="color: var(--color-text-secondary);">Memory</span>
															<span class="text-[10px] font-mono font-semibold" style="color: var(--color-text);">
																{ec.stats?.memory_usage != null ? formatBytes(ec.stats.memory_usage) : '—'} / {ec.stats?.memory_limit ? formatBytes(ec.stats.memory_limit) : '—'}
															</span>
														</div>
														<div class="h-1.5 rounded-full overflow-hidden" style="background-color: var(--color-border-light);">
															<div class="h-full rounded-full transition-all duration-500"
																style="width: {Math.min(memPct ?? 0, 100)}%; background-color: {(memPct ?? 0) > 80 ? 'var(--color-danger)' : (memPct ?? 0) > 60 ? '#eab308' : '#f59e0b'};">
															</div>
														</div>
													</div>
													<div class="grid grid-cols-2 gap-2">
														<div class="rounded border p-2" style="border-color: var(--color-border-light); background-color: var(--color-surface);">
															<div class="flex items-center gap-1 text-[10px] font-medium mb-0.5" style="color: var(--color-text-muted);">
																<Icon icon="solar:download-square-bold" class="h-3 w-3" /> RX
															</div>
															<p class="text-xs font-semibold font-mono" style="color: var(--color-text);">{ec.stats?.net_rx != null ? formatBytes(ec.stats.net_rx) : '—'}</p>
														</div>
														<div class="rounded border p-2" style="border-color: var(--color-border-light); background-color: var(--color-surface);">
															<div class="flex items-center gap-1 text-[10px] font-medium mb-0.5" style="color: var(--color-text-muted);">
																<Icon icon="solar:upload-square-bold" class="h-3 w-3" /> TX
															</div>
															<p class="text-xs font-semibold font-mono" style="color: var(--color-text);">{ec.stats?.net_tx != null ? formatBytes(ec.stats.net_tx) : '—'}</p>
														</div>
														<div class="rounded border p-2" style="border-color: var(--color-border-light); background-color: var(--color-surface);">
															<div class="flex items-center gap-1 text-[10px] font-medium mb-0.5" style="color: var(--color-text-muted);">
																<Icon icon="solar:document-text-bold" class="h-3 w-3" /> Read
															</div>
															<p class="text-xs font-semibold font-mono" style="color: var(--color-text);">{ec.stats?.block_read != null ? formatBytes(ec.stats.block_read) : '—'}</p>
														</div>
														<div class="rounded border p-2" style="border-color: var(--color-border-light); background-color: var(--color-surface);">
															<div class="flex items-center gap-1 text-[10px] font-medium mb-0.5" style="color: var(--color-text-muted);">
																<Icon icon="solar:pen-bold" class="h-3 w-3" /> Write
															</div>
															<p class="text-xs font-semibold font-mono" style="color: var(--color-text);">{ec.stats?.block_write != null ? formatBytes(ec.stats.block_write) : '—'}</p>
														</div>
													</div>
												</div>

												<!-- Action Buttons -->
												<div class="flex flex-nowrap items-center gap-1.5 border-t px-3 py-2" style="border-color: var(--color-border-light); background-color: var(--color-surface);">
													<button onclick={() => confirmAction(c.id, c.server_id, 'start', c.name)}
														class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-[11px] font-semibold transition-all shrink-0"
														style="background-color: var(--color-success); color: #fff;">
														<Icon icon="solar:play-bold" class="h-3 w-3" /> Start
													</button>
													<button onclick={() => confirmAction(c.id, c.server_id, 'stop', c.name)}
														class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-[11px] font-semibold transition-all shrink-0"
														style="background-color: var(--color-danger); color: #fff;">
														<Icon icon="solar:pause-bold" class="h-3 w-3" /> Stop
													</button>
													<button onclick={() => confirmAction(c.id, c.server_id, 'restart', c.name)}
														class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-[11px] font-semibold transition-all shrink-0"
														style="background-color: #eab308; color: #fff;">
														<Icon icon="solar:refresh-bold" class="h-3 w-3" /> Restart
													</button>
													<span class="flex-1 min-w-0"></span>
													<button onclick={() => viewSecurity(c)}
														class="inline-flex items-center justify-center h-7 w-7 min-w-[28px] rounded-md shrink-0 transition-all"
														title="Security"
														style="border: 1px solid {ec.showSecurity ? 'var(--color-primary)' : 'rgba(148,163,184,0.45)'}; color: {ec.showSecurity ? 'var(--color-primary)' : 'rgba(148,163,184,0.85)'}; background-color: {ec.showSecurity ? 'var(--color-primary-subtle)' : 'rgba(148,163,184,0.1)'};">
														<Icon icon="solar:shield-bold" class="h-3.5 w-3.5" />
													</button>
													<button onclick={() => viewLogs(c)}
														class="inline-flex items-center justify-center h-7 w-7 min-w-[28px] rounded-md shrink-0 transition-all"
														title="Logs"
														style="border: 1px solid {ec.showLogs ? 'var(--color-primary)' : 'rgba(148,163,184,0.45)'}; color: {ec.showLogs ? 'var(--color-primary)' : 'rgba(148,163,184,0.85)'}; background-color: {ec.showLogs ? 'var(--color-primary-subtle)' : 'rgba(148,163,184,0.1)'};">
														<Icon icon="solar:document-text-bold" class="h-3.5 w-3.5" />
													</button>
													<button onclick={() => viewExec(c)}
														class="inline-flex items-center justify-center h-7 w-7 min-w-[28px] rounded-md shrink-0 transition-all"
														title="Exec"
														style="border: 1px solid {ec.showExec ? 'var(--color-primary)' : 'rgba(148,163,184,0.45)'}; color: {ec.showExec ? 'var(--color-primary)' : 'rgba(148,163,184,0.85)'}; background-color: {ec.showExec ? 'var(--color-primary-subtle)' : 'rgba(148,163,184,0.1)'};">
														<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5" /><line x1="12" y1="19" x2="20" y2="19" /></svg>
													</button>
													<button onclick={() => viewInspect(c)}
														class="inline-flex items-center justify-center h-7 w-7 min-w-[28px] rounded-md shrink-0 transition-all"
														title="Inspect"
														style="border: 1px solid {ec.showInspect ? 'var(--color-primary)' : 'rgba(148,163,184,0.45)'}; color: {ec.showInspect ? 'var(--color-primary)' : 'rgba(148,163,184,0.85)'}; background-color: {ec.showInspect ? 'var(--color-primary-subtle)' : 'rgba(148,163,184,0.1)'};">
														<Icon icon="solar:code-bold" class="h-3.5 w-3.5" />
													</button>
												</div>

												<!-- Security Panel -->
												{#if ec.showSecurity}
													<div class="border-t px-4 py-3" style="border-color: var(--color-border-light); background-color: var(--color-card);">
														<div class="flex items-center justify-between mb-2">
															<h3 class="text-[11px] font-bold flex items-center gap-1.5" style="color: var(--color-text);">
																<Icon icon="solar:shield-bold" class="h-3.5 w-3.5" /> SECURITY FINDINGS
															</h3>
															<div class="flex items-center gap-2">
																{#if hasSecurity}
																	<button onclick={() => goto(`/containers/${c.server_id}/${c.id}/security`)}
																		class="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-[10px] font-semibold transition-all shrink-0"
																		style="border: 1px solid var(--color-primary); color: var(--color-primary); background: transparent;">
																		<Icon icon="solar:export-bold" class="h-3 w-3" /> Report
																	</button>
																	<span class="text-[10px]" style="color: var(--color-text-muted);">Scanned: {sec.scanned_at ? sec.scanned_at.substring(0, 10) : '—'}</span>
																{/if}
																<button onclick={() => scanSingleContainer(c)}
																	disabled={scanningContainer === c.id}
																	class="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-[10px] font-semibold transition-all shrink-0"
																	style="background-color: var(--color-primary); color: #fff;">
																	<Icon icon={scanningContainer === c.id ? 'solar:spinner-bold' : 'solar:shield-bold'}
																		class="h-3 w-3 {scanningContainer === c.id ? 'animate-spin' : ''}" />
																	{scanningContainer === c.id ? 'Scanning...' : 'Scan'}
																</button>
															</div>
														</div>
														{#if sec?.findings?.length}
															<div class="space-y-1.5 max-h-48 overflow-y-auto">
																{#each sec.findings as finding}
																	<div class="rounded-lg px-3 py-2 text-[11px]"
																		style="background-color: {severityBg(finding.severity)};">
																		<div class="flex items-center gap-2 mb-0.5">
																			<span class="inline-flex items-center px-1.5 py-0.5 rounded text-[9px] font-bold uppercase tracking-wider"
																				style="background-color: {severityColor(finding.severity)}22; color: {severityColor(finding.severity)};">
																				{finding.severity}
																			</span>
																			<span class="font-semibold" style="color: var(--color-text);">{finding.title}</span>
																			<span class="text-[10px] font-mono" style="color: var(--color-text-muted);">{finding.check_id}</span>
																		</div>
																		<p class="mb-1" style="color: var(--color-text-secondary);">{finding.description}</p>
																		{#if finding.remediation}
																			<p class="text-[10px]" style="color: var(--color-primary);">
																				<span class="font-semibold">Fix:</span> {finding.remediation}
																			</p>
																		{/if}
																	</div>
																{/each}
															</div>
														{:else}
															<p class="text-xs" style="color: var(--color-text-muted);">No security findings available. Run a Container Security scan first.</p>
														{/if}
													</div>
												{/if}

												<!-- Logs Panel -->
												{#if ec.showLogs}
													<div class="border-t px-4 py-3" style="border-color: var(--color-border-light); background-color: var(--color-card);">
														<div class="flex items-center justify-between mb-2">
															<h3 class="text-[11px] font-bold flex items-center gap-1.5" style="color: var(--color-text);">
																<Icon icon="solar:document-text-bold" class="h-3.5 w-3.5" /> RECENT LOGS
															</h3>
															<button onclick={() => doFetchLogs(c.id)} disabled={ec.logsLoading}
																class="inline-flex items-center gap-1 text-[10px] font-medium transition-all hover:opacity-70"
																style="color: var(--color-text-muted);">
																<Icon icon={ec.logsLoading ? 'solar:spinner-bold' : 'solar:refresh-bold'}
																	class="h-3 w-3 {ec.logsLoading ? 'animate-spin' : ''}" /> Refresh
															</button>
														</div>
														<div class="rounded-lg p-3 font-mono text-[10px] leading-relaxed overflow-x-auto"
															style="background-color: #0f172a; min-height: 60px; max-height: 200px; overflow-y: auto;">
															{#if ec.logsLoading}
																<div class="flex items-center gap-2" style="color: #64748b;">
																	<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" /> Loading logs...
																</div>
															{:else}
																{@html formatLogsForDisplay(ec.logs)}
															{/if}
														</div>
													</div>
												{/if}

												<!-- Exec Panel -->
												{#if ec.showExec}
													<div class="border-t" style="border-color: var(--color-border-light); background-color: var(--color-card);">
														<div class="flex items-center gap-2 px-4 py-2">
															<h3 class="text-[11px] font-bold flex items-center gap-1.5" style="color: var(--color-text);">
																<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5" /><line x1="12" y1="19" x2="20" y2="19" /></svg> EXEC
															</h3>
															<span class="ml-auto text-[10px]" style="color: #64748b;">Interactive terminal — type any command</span>
														</div>
														<div
															use:containerExecAction={c.id}
															class="exec-terminal h-48 w-full overflow-hidden"
															style="min-height: 200px;"
														></div>
													</div>
												{/if}

												<!-- Inspect Panel -->
												{#if ec.showInspect && ec.inspectRaw}
													<div class="border-t px-4 py-3" style="border-color: var(--color-border-light); background-color: var(--color-card);">
														<h3 class="text-[11px] font-bold mb-1.5" style="color: var(--color-text);">INSPECT DATA</h3>
														<div class="rounded-lg p-3 font-mono text-[10px] leading-relaxed overflow-x-auto" style="background-color: #0f172a; color: #a5b4fc; max-height: 250px; overflow-y: auto;">
															<pre class="whitespace-pre-wrap">{ec.inspectRaw}</pre>
														</div>
													</div>
												{/if}
											</div>
										{/if}
									</div>
								</div>
							</div>
						{/each}
					</div>
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
