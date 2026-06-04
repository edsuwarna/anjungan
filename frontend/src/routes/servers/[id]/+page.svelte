<script>
	import Icon from '@iconify/svelte';
	import { api } from '$lib/api.svelte.js';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AddServerModal from '$lib/components/ui/AddServerModal.svelte';
	import MetricsChart from '$lib/components/charts/MetricsChart.svelte';

	// ── Core state ──────────────────────────────────────────
	let server = $state(null);
	let loading = $state(true);
	let error = $state('');
	let testing = $state(false);
	let detecting = $state(false);
	let showEditModal = $state(false);
	let copyFeedback = $state({ show: false, success: false });

	// Confirmation modal
	let confirmModal = $state({ show: false, title: '', message: '', onConfirm: null, danger: false });

	// ── Tab system ──────────────────────────────────────────
	let activeTab = $state('overview');
	const tabs = [
		{ id: 'overview', label: 'Overview', icon: 'solar:info-circle-bold' },
		{ id: 'metrics', label: 'Metrics', icon: 'solar:chart-2-bold' },
		{ id: 'containers', label: 'Containers', icon: 'solar:box-bold' },
		{ id: 'compliance', label: 'Compliance', icon: 'solar:shield-check-bold' },
	];

	// ── Metrics state ───────────────────────────────────────
	let metrics = $state(null);
	let metricsLoading = $state(false);
	let metricsError = $state('');
	let metricsHistory = $state([]);
	let historyRange = $state('1h');
	let historyLoading = $state(false);
	let liveInterval = $state(null);
	let liveEnabled = $state(false);

	// ── Containers state ────────────────────────────────────
	let containers = $state([]);
	let containersLoading = $state(false);
	let containerActions = $state({});
	let logsModal = $state({ show: false, container: '', logs: '', loading: false });
	let inspectModal = $state({ show: false, container: '', data: null, loading: false });

	// ── Compliance state ────────────────────────────────────
	let scan = $state(null);
	let scanLoading = $state(true);
	let scanError = $state('');
	let scanning = $state(false);
	let lynisData = $derived.by(() => {
		if (!scan || scan.scan_type !== 'Lynis') return null;
		const findings = scan.findings || [];
		const warningsList = findings.filter(f => f.status === 'fail').map(f => ({
			test_id: f.check_id || f.id,
			description: f.description || f.title
		}));
		const suggestionsList = findings.filter(f => f.status === 'warn').map(f => ({
			test_id: f.check_id || f.id,
			description: f.description || f.title
		}));
		return {
			hardening_score: scan.score,
			tests: scan.total_checks,
			warnings: scan.warnings || 0,
			suggestions: suggestionsList.length,
			warnings_list: warningsList,
			suggestions_list: suggestionsList
		};
	});
	let profile = $state('cis_level_1');
	let availableChecks = $state([]);
	let l1Categories = $state([]);
	let l2Categories = $state([]);
	let catLoading = $state(false);
	let history = $state([]);
	let compHistoryLoading = $state(false);
	let historyPage = $state(1);
	let historyTotal = $state(0);
	let historyLimit = 10;
let showAllHistory = $state(false);
	let findings = $state([]);
	let summaryCounts = $derived.by(() => {
		const counts = { critical: 0, high: 0, medium: 0, low: 0, warning: 0, total: 0 };
		if (!findings || findings.length === 0) return counts;
		for (const f of findings) {
			if (f.status === 'pass' || f.severity === 'passed' || f.severity === 'info') continue;
			const sev = (f.severity || '').toLowerCase();
			if (sev in counts) { counts[sev]++; counts.total++; }
		}
		return counts;
	});

	// ── Inline category detail state ─────────────────────
	let selectedCategory = $state(null);
	let categoryItems = $state([]);
	let catHistoryData = $state([]);
	let catDetailTab = $state('checks');
		let catHistoryLoading = $state(false);
	let pendingTab = $state(null);
	let pendingScanId = $state(null);

	// ── Init ────────────────────────────────────────────────
	onMount(() => {
		loadServer();
		// Handle URL params from scan history view links
		const tabParam = $page.url.searchParams.get('tab');
		const scanParam = $page.url.searchParams.get('scan');
		if (tabParam === 'compliance') {
			// Set a flag so loadServer triggers compliance after load
			pendingTab = 'compliance';
			pendingScanId = scanParam;
		}
		return () => {
			if (liveInterval) clearInterval(liveInterval);
		};
	});

	// ── Server loading ──────────────────────────────────────
	async function loadServer() {
		loading = true;
		error = '';
		try {
			server = await api.servers.get($page.params.id);
			loadMetrics();
			loadContainers();
			if (pendingTab) {
				activeTab = pendingTab;
				if (activeTab === 'compliance') {
					loadCompliance();
				}
				pendingTab = null;
			}
		} catch (e) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadMetrics() {
		metricsLoading = true;
		metricsError = '';
		try {
			metrics = await api.servers.metrics($page.params.id);
			loadHistory();
		} catch (e) {
			metricsError = e.message;
			metrics = null;
		} finally {
			metricsLoading = false;
		}
	}

	async function loadHistory() {
		historyLoading = true;
		try {
			metricsHistory = await api.servers.metricsHistory($page.params.id, historyRange, 100);
		} catch (_) {
			metricsHistory = [];
		} finally {
			historyLoading = false;
		}
	}

	async function loadContainers() {
		containersLoading = true;
		try {
			containers = await api.servers.containers($page.params.id);
		} catch (_) {
			containers = [];
		} finally {
			containersLoading = false;
		}
	}

	function toggleLiveRefresh() {
		liveEnabled = !liveEnabled;
		if (liveEnabled) {
			liveInterval = setInterval(() => {
				api.servers.metrics($page.params.id).then(d => { metrics = d; }).catch(() => {});
			}, 10000);
		} else {
			if (liveInterval) clearInterval(liveInterval);
			liveInterval = null;
		}
	}

	function changeRange(range) {
		historyRange = range;
		api.servers.metricsHistory($page.params.id, range, 100).then(d => { metricsHistory = d; }).catch(() => {});
	}

	async function containerAction(containerId, action) {
		const key = containerId + ':' + action;
		containerActions = { ...containerActions, [key]: 'loading' };
		try {
			let result;
			switch (action) {
				case 'start': result = await api.servers.containerStart($page.params.id, containerId); break;
				case 'stop': result = await api.servers.containerStop($page.params.id, containerId); break;
				case 'restart': result = await api.servers.containerRestart($page.params.id, containerId); break;
			}
			containerActions = { ...containerActions, [key]: 'success' };
			setTimeout(() => { containerActions = { ...containerActions, [key]: undefined }; }, 2000);
			setTimeout(() => loadContainers(), 1000);
		} catch (e) {
			containerActions = { ...containerActions, [key]: 'error' };
			setTimeout(() => { containerActions = { ...containerActions, [key]: undefined }; }, 3000);
		}
	}

	async function viewLogs(containerId, containerName) {
		logsModal = { show: true, container: containerName, logs: 'Loading...', loading: true };
		try {
			const result = await api.servers.containerLogs($page.params.id, containerId);
			logsModal = { show: true, container: containerName, logs: result.logs || 'No logs', loading: false };
		} catch (e) {
			logsModal = { show: true, container: containerName, logs: 'Error: ' + e.message, loading: false };
		}
	}

	async function viewInspect(containerId, containerName) {
		inspectModal = { show: true, container: containerName, data: null, loading: true };
		try {
			const result = await api.servers.containerInspect($page.params.id, containerId);
			inspectModal = { show: true, container: containerName, data: result, loading: false };
		} catch (e) {
			inspectModal = { show: true, container: containerName, data: { error: e.message }, loading: false };
		}
	}

	async function testConnection() {
		testing = true;
		try {
			await api.servers.testExisting($page.params.id);
		} catch (e) {
			error = e.message;
		}
		testing = false;
	}

	async function autoDetect() {
		detecting = true;
		try {
			server = await api.servers.detect($page.params.id);
		} catch (e) {
			error = e.message;
		}
		detecting = false;
	}

	function copySshCmd() {
		const text = `ssh ${server.ssh_user}@${server.host} -p ${server.port}`;
		try {
			navigator.clipboard.writeText(text);
			copyFeedback = { show: true, success: true };
		} catch {
			try {
				const ta = document.createElement('textarea');
				ta.value = text;
				ta.style.position = 'fixed';
				ta.style.left = '-9999px';
				document.body.appendChild(ta);
				ta.select();
				document.execCommand('copy');
				ta.remove();
				copyFeedback = { show: true, success: true };
			} catch {
				copyFeedback = { show: true, success: false };
			}
		}
		setTimeout(() => { copyFeedback = { show: false, success: false }; }, 2500);
	}

	function confirmDelete() {
		confirmModal = {
			show: true,
			title: 'Delete Server',
			message: `Are you sure you want to delete "${server.name}" (${server.host})? This action cannot be undone.`,
			danger: true,
			onConfirm: async () => {
				try {
					await api.servers.delete($page.params.id);
					goto('/servers');
				} catch (e) {
					error = 'Failed to delete: ' + e.message;
				}
				confirmModal = { show: false, title: '', message: '', onConfirm: null, danger: false };
			}
		};
	}

	// ── Compliance functions ────────────────────────────────
	let complianceLoaded = $state(false);
	let isHistoricalScan = $state(false);

	async function loadCompliance() {
		if (complianceLoaded && !pendingScanId) return;
		scanLoading = true;
		try {
			let scanData;
			if (pendingScanId) {
				// Load a specific historical scan from URL param
				try {
					scanData = await api.compliance.scanDetail($page.params.id, pendingScanId);
					isHistoricalScan = true;
				} catch (_) {
					scanData = null;
				}
				pendingScanId = null;
			}
			if (!scanData) {
				scanData = await getLatestScan(profileToScanType(profile));
				isHistoricalScan = false;
			}
			const [checksData] = await Promise.all([
				api.compliance.checks().catch(() => ({ checks: [] })),
			]);
			scan = scanData;
			availableChecks = checksData.checks || [];
			if (scan && scan.findings) {
				processFindings(scan.findings);
			}
			loadCategoryData();
			loadComplianceHistory();
			complianceLoaded = true;
		} catch (e) {
			scanError = e.message;
		} finally {
			scanLoading = false;
		}
	}

	async function getLatestScan(scanType) {
		try {
			const params = {};
			if (scanType) params.scan_type = scanType;
			return await api.compliance.latest($page.params.id, params);
		} catch {
			return null;
		}
	}

	async function loadCategoryData() {
		catLoading = true;
		try {
			const l1 = await api.compliance.latestCategories($page.params.id, { scan_type: 'CIS Level 1' });
			if (l1.categories) l1Categories = l1.categories;
		} catch (_) {}
		try {
			const l2 = await api.compliance.latestCategories($page.params.id, { scan_type: 'CIS Level 2' });
			if (l2.categories) l2Categories = l2.categories.filter(c => c.total > 0);
		} catch (_) {}
		catLoading = false;
	}

	async function loadComplianceHistory(pg) {
		if (pg !== undefined) historyPage = pg;
		compHistoryLoading = true;
		try {
			const resp = await api.compliance.history($page.params.id, { page: historyPage, limit: historyLimit });
			history = resp.results || resp.history || [];
			if (Array.isArray(resp)) history = resp;
			historyTotal = resp.total || resp.count || (Array.isArray(resp) ? resp.length : 0);
		} catch {
			history = [];
			historyTotal = 0;
		} finally {
			compHistoryLoading = false;
		}
	}

	function processFindings(f) {
		if (!f) { findings = []; return; }
		findings = f;
	}

	// ── Category detail panel ───────────────────────────
	function selectCategory(cat) {
		selectedCategory = cat;
		catDetailTab = 'checks';
		categoryItems = (scan?.findings || []).filter(f => {
			const fcat = (f.category || '').toLowerCase();
			const ccat = (cat.category || '').toLowerCase();
			return fcat === ccat || fcat === cat.category;
		});
		loadCategoryHistory(cat.category);
	}

	function closeCategoryPanel() {
		selectedCategory = null;
		categoryItems = [];
		catHistoryData = [];
	}

	async function loadCategoryHistory(category) {
		catHistoryLoading = true;
		try {
			const resp = await api.compliance.categoryHistory($page.params.id, category, { limit: 10 });
			catHistoryData = resp.results || [];
		} catch (_) {
			catHistoryData = [];
		} finally {
			catHistoryLoading = false;
		}
	}

	async function viewHistoricScan(item) {
		if (!item || !item.id) return;
		scanLoading = true;
		try {
			const detail = await api.compliance.scanDetail($page.params.id, item.id);
			if (detail) {
				scan = detail;
				isHistoricalScan = true;
				if (detail.findings) processFindings(detail.findings);
				loadCategoryData();
				selectedCategory = null;
			}
		} catch (e) {
			console.error('Failed to load scan detail:', e);
		} finally {
			scanLoading = false;
		}
	}

	function profileToScanType(p) {
		if (p === 'lynis') return 'Lynis';
		if (p === 'cis_level_2') return 'CIS Level 2';
		return 'CIS Level 1';
	}

	async function pollScan(scanId, maxAttempts = 60, interval = 2000) {
		for (let i = 0; i < maxAttempts; i++) {
			const detail = await api.compliance.scanDetail($page.params.id, scanId);
			if (detail.status === 'completed' || detail.status === 'failed') {
				return detail;
			}
			await new Promise(r => setTimeout(r, interval));
		}
		throw new Error('Scan timed out after ' + (maxAttempts * interval / 1000) + 's');
	}

	async function runScan() {
		scanning = true;
		scanError = '';
		try {
			const resp = await api.compliance.scan($page.params.id, profile);
			scan = await pollScan(resp.scan_id);
			if (scan && scan.findings) processFindings(scan.findings);
			loadCategoryData();
			loadComplianceHistory();
			selectedCategory = null;
		} catch (e) {
			scanError = e.message || 'Scan failed';
		} finally {
			scanning = false;
		}
	}

	async function runLynisScan() {
		scanning = true;
		scanError = '';
		try {
			const resp = await api.compliance.scanLynis($page.params.id);
			scan = await pollScan(resp.scan_id);
			if (scan && scan.findings) processFindings(scan.findings);
			loadComplianceHistory();
			selectedCategory = null;
		} catch (e) {
			scanError = e.message || 'Scan failed';
		} finally {
			scanning = false;
		}
	}

	async function switchProfile(p) {
		profile = p;
		if (!complianceLoaded) return;
		scanLoading = true;
		try {
			const st = profileToScanType(p);
			scan = await getLatestScan(st);
			isHistoricalScan = false;
			if (scan && scan.findings) processFindings(scan.findings);
			loadCategoryData();
			loadComplianceHistory();
		} catch (_) {} finally {
			scanLoading = false;
		}
	}

	function switchTab(tab) {
		activeTab = tab;
		if (tab === 'compliance' && !complianceLoaded) {
			loadCompliance();
		}
	}

	// ── Helpers ─────────────────────────────────────────────
	function statusClass(status) {
		switch (status) {
			case 'online': return 'online';
			case 'offline': return 'offline';
			default: return 'pending';
		}
	}

	function formatDate(dateStr) {
		if (!dateStr) return '-';
		return new Date(dateStr).toLocaleDateString('en-GB', {
			day: 'numeric', month: 'short', year: 'numeric',
			hour: '2-digit', minute: '2-digit'
		});
	}

	function formatTime(ts) {
		if (!ts) return 'Never';
		const d = new Date(ts);
		const now = new Date();
		const diff = now - d;
		if (diff < 60000) return 'Just now';
		if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
		if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
	}

	function formatTimeFull(ts) {
		if (!ts) return 'Never';
		return new Date(ts).toLocaleDateString('en-GB', {
			day: 'numeric', month: 'short', year: 'numeric',
			hour: '2-digit', minute: '2-digit'
		});
	}

	function formatBytes(bytes) {
		if (!bytes || bytes === 0) return '0 B';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(1024));
		return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
	}

	function scoreColor(score) {
		if (score >= 80) return 'var(--color-success)';
		if (score >= 60) return 'var(--color-warning)';
		return 'var(--color-danger)';
	}

	function severityColor(severity) {
		const s = (severity || '').toLowerCase();
		if (s === 'critical') return 'var(--color-danger)';
		if (s === 'high' || s === 'warn' || s === 'warning') return 'var(--color-warning)';
		if (s === 'medium') return 'var(--color-accent)';
		if (s === 'low' || s === 'passed' || s === 'pass') return 'var(--color-success)';
		return 'var(--color-text-muted)';
	}

	function scoreLabel(score) {
		if (score === undefined || score === null) return 'Unscanned';
		if (score >= 80) return 'Passing';
		if (score >= 60) return 'Warning';
		return 'Critical';
	}

	function containerBorderColor(state) {
		if (state === 'running') return 'var(--color-success)';
		if (state === 'exited' || state === 'stopped') return 'var(--color-danger)';
		return 'var(--color-warning)';
	}

	function containerStatusClass(state) {
		const s = (state || '').toLowerCase();
		if (s === 'running') return 'running';
		if (s === 'exited' || s === 'stopped') return 'stopped';
		return 'pending';
	}

	function sparklineValues(data) {
		if (!data || data.length === 0) return [];
		const reversed = [...data].reverse();
		return reversed.map(d => d.disk_used_pct || 0);
	}

	// ── Compliance derived ─────────────────────────────────
	let scanStats = $derived.by(() => {
		if (!scan) return { critical: 0, high: 0, medium: 0, low: 0, passed: 0, total: 0 };
		const s = { critical: 0, high: 0, medium: 0, low: 0, passed: 0, total: 0 };
		for (const f of scan.findings || []) {
			s.total++;
			const sev = (f.severity || '').toLowerCase();
			if (sev === 'passed' || f.status === 'pass') s.passed++;
			else if (sev === 'critical') s.critical++;
			else if (sev === 'high') s.high++;
			else if (sev === 'medium') s.medium++;
			else if (sev === 'low') s.low++;
		}
		return s;
	});

	let score = $derived(scan?.score ?? null);

	const profileCategories = $derived.by(() => {
		const cats = profile === 'cis_level_2' ? l2Categories : l1Categories;
		const meta = {
			ssh: { label: 'SSH', icon: '🔒', color: '#3b82f6' },
			kernel: { label: 'Kernel', icon: '⚙️', color: '#8b5cf6' },
			filesystem: { label: 'Filesystem', icon: '📁', color: '#f59e0b' },
			network: { label: 'Network', icon: '🌐', color: '#10b981' },
			logging: { label: 'Logging', icon: '📋', color: '#c084fc' },
			users: { label: 'Users & Groups', icon: '👥', color: '#fb923c' },
			services: { label: 'Services', icon: '⚡', color: '#22d3ee' },
			docker: { label: 'Docker', icon: '📦', color: '#06b6d4' },
		};
		return cats.map(c => ({ ...c, meta: meta[c.category] || { label: c.category, icon: '🛡️', color: '#6b7280' } }));
	});

	// What's scanned data
	const whatsScannedInfo = {
		cis_level_1: {
			title: 'CIS Level 1',
			desc: 'CIS Level 1 scans <strong>58 checks</strong> across <strong>7 categories</strong> to verify basic security hardening. These are foundational controls every production server should implement.',
			color: '#10b981',
			bg: '#ecfdf5',
			border: '#bbf7d0',
			items: [
				{ icon: '🔒', label: 'SSH', checks: '12 checks', desc: 'SSH config, key-based auth, root login restrictions' },
				{ icon: '⚙️', label: 'Kernel', checks: '8 checks', desc: 'sysctl params, network hardening, ASLR' },
				{ icon: '📁', label: 'Filesystem', checks: '8 checks', desc: 'Partition mounts, sticky bits, world-writable files' },
				{ icon: '🌐', label: 'Network', checks: '6 checks', desc: 'Firewall rules, IP forwarding, network params' },
				{ icon: '🔐', label: 'Authentication', checks: '8 checks', desc: 'Password policies, lockout, PAM configuration' },
				{ icon: '📋', label: 'Logging', checks: '8 checks', desc: 'Auditd, syslog, log rotation' },
				{ icon: '👥', label: 'Users', checks: '8 checks', desc: 'UID/GID, empty passwords, root accounts' },
			],
		},
		cis_level_2: {
			title: 'CIS Level 2',
			desc: 'CIS Level 2 extends L1 with <strong>92 checks</strong> across <strong>10 categories</strong> for advanced security hardening. Includes additional services, Docker, and packages audits.',
			color: '#f59e0b',
			bg: '#fffbeb',
			border: '#fde68a',
			items: [
				{ icon: '🔒', label: 'SSH', checks: '14 checks', desc: 'Advanced SSH hardening, ciphers, protocol settings' },
				{ icon: '⚙️', label: 'Kernel', checks: '12 checks', desc: 'Kernel modules, apparmor, SELinux' },
				{ icon: '📁', label: 'Filesystem', checks: '10 checks', desc: 'ACLs, filesystem integrity, quotas' },
				{ icon: '🌐', label: 'Network', checks: '8 checks', desc: 'TCP wrappers, host-based firewall, routing' },
				{ icon: '🔐', label: 'Authentication', checks: '10 checks', desc: 'Strong password hashing, MFA config' },
				{ icon: '📋', label: 'Logging', checks: '10 checks', desc: 'Log forwarding, log aggregation, auditd rules' },
				{ icon: '👥', label: 'Users', checks: '8 checks', desc: 'User groups, sudoers, home perms' },
				{ icon: '⚡', label: 'Services', checks: '8 checks', desc: 'Service management, unused services' },
				{ icon: '📦', label: 'Docker', checks: '6 checks', desc: 'Docker daemon, container security' },
				{ icon: '📦', label: 'Packages', checks: '6 checks', desc: 'Package updates, repo signing, versions' },
			],
		},
		lynis: {
			title: 'Lynis',
			desc: 'Lynis is a security auditing tool that performs <strong>256 tests</strong> across <strong>12 categories</strong>. It checks compliance, configuration, and system hardening.',
			color: '#8b5cf6',
			bg: '#f5f3ff',
			border: '#ddd6fe',
			items: [
				{ icon: '🔐', label: 'Authentication', checks: 'AUTH-9282', desc: 'Password policies, PAM, SSH keys' },
				{ icon: '📁', label: 'File Systems', checks: 'FILE-6280', desc: 'Permissions, mounts, disk quotas' },
				{ icon: '⚙️', label: 'Kernel', checks: 'KRNL-5820', desc: 'Sysctl, modules, boot security' },
				{ icon: '🌐', label: 'Networking', checks: 'NETW-8500', desc: 'Firewall, ports, DNS config' },
				{ icon: '📋', label: 'Logging', checks: 'LOGG-2100', desc: 'Auditd, syslog, log forwarding' },
				{ icon: '📦', label: 'Docker', checks: 'DOCK-9310', desc: 'Container security, daemon config' },
			],
		},
	};

	const currentInfo = $derived(whatsScannedInfo[profile] || whatsScannedInfo.cis_level_1);

	const isLynisProfile = $derived(profile === 'lynis');

	const filteredScanCount = $derived.by(() => {
		if (!history || history.length === 0) return 0;
		let items = history;
		if (profile === 'lynis') {
			items = items.filter(h => h.scan_type === 'Lynis' || h.scan_type?.toLowerCase().includes('lynis'));
		} else if (profile === 'cis_level_1') {
			items = items.filter(h => h.scan_type === 'CIS Level 1' || h.scan_type === 'All Checks');
		} else if (profile === 'cis_level_2') {
			items = items.filter(h => h.scan_type === 'CIS Level 2');
		}
		return items.length;
	});

	const filteredHistory = $derived.by(() => {
		if (!history || history.length === 0) return [];
		let items = history;
		if (profile === 'lynis') {
			items = items.filter(h => h.scan_type === 'Lynis' || h.scan_type?.toLowerCase().includes('lynis'));
		} else if (profile === 'cis_level_1') {
			items = items.filter(h => h.scan_type === 'CIS Level 1' || h.scan_type === 'All Checks');
		} else if (profile === 'cis_level_2') {
			items = items.filter(h => h.scan_type === 'CIS Level 2');
		}
		return showAllHistory ? items : items.slice(0, 5);
	});
</script>

<div class="page-container">
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
			<p class="text-sm font-medium" style="color: var(--color-danger);">Failed to load server</p>
			<p class="mt-1 text-xs" style="color: var(--color-text-muted);">{error}</p>
			<button onclick={() => goto('/servers')} class="btn-secondary mt-4">Back to Servers</button>
		</div>
	{:else if server}
		<!-- ── Confirmation Modal ── -->
		{#if confirmModal.show}
			<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,0.5);"
				onclick={() => confirmModal = { show: false, title: '', message: '', onConfirm: null, danger: false }}>
				<div class="w-full max-w-sm rounded-xl border shadow-xl" style="background-color: var(--color-card); border-color: var(--color-border);"
					onclick={(e) => e.stopPropagation()}>
					<div class="px-6 py-5">
						<div class="flex items-center gap-3">
							<div class="flex h-10 w-10 items-center justify-center rounded-full" style="background-color: rgba(239,68,68,0.1);">
								<Icon icon="solar:danger-triangle-bold" class="h-5 w-5" style="color: var(--color-danger);" />
							</div>
							<h3 class="text-base font-semibold" style="color: var(--color-text);">{confirmModal.title}</h3>
						</div>
						<p class="mt-3 text-sm" style="color: var(--color-text-secondary);">{confirmModal.message}</p>
					</div>
					<div class="flex items-center justify-end gap-2 border-t px-6 py-3" style="border-color: var(--color-border);">
						<button onclick={() => confirmModal = { show: false, title: '', message: '', onConfirm: null, danger: false }} class="btn-secondary text-sm">Cancel</button>
						<button onclick={() => confirmModal.onConfirm?.()} class="btn-danger text-sm">Delete</button>
					</div>
				</div>
			</div>
		{/if}

		<!-- ── Logs Modal ── -->
		{#if logsModal.show}
			<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,0.5);"
				onclick={() => logsModal = { show: false, container: '', logs: '', loading: false }}>
				<div class="w-full max-w-2xl rounded-xl border shadow-xl max-h-[80vh] flex flex-col" style="background-color: var(--color-card); border-color: var(--color-border);"
					onclick={(e) => e.stopPropagation()}>
					<div class="flex items-center justify-between border-b px-5 py-3" style="border-color: var(--color-border);">
						<h3 class="text-sm font-semibold" style="color: var(--color-text);">Logs: {logsModal.container}</h3>
						<button onclick={() => logsModal = { show: false, container: '', logs: '', loading: false }} class="btn-icon">
							<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
						</button>
					</div>
					<div class="flex-1 overflow-auto p-4">
						{#if logsModal.loading}
							<div class="flex items-center justify-center py-8">
								<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-text-muted);" />
							</div>
						{:else}
							<pre class="font-mono text-xs leading-relaxed whitespace-pre-wrap" style="color: var(--color-text-secondary);">{logsModal.logs}</pre>
						{/if}
					</div>
				</div>
			</div>
		{/if}

		<!-- ── Inspect Modal ── -->
		{#if inspectModal.show}
			<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,0.5);"
				onclick={() => inspectModal = { show: false, container: '', data: null, loading: false }}>
				<div class="w-full max-w-2xl rounded-xl border shadow-xl max-h-[80vh] flex flex-col" style="background-color: var(--color-card); border-color: var(--color-border);"
					onclick={(e) => e.stopPropagation()}>
					<div class="flex items-center justify-between border-b px-5 py-3" style="border-color: var(--color-border);">
						<h3 class="text-sm font-semibold" style="color: var(--color-text);">Inspect: {inspectModal.container}</h3>
						<button onclick={() => inspectModal = { show: false, container: '', data: null, loading: false }} class="btn-icon">
							<Icon icon="solar:close-circle-bold" class="h-5 w-5" />
						</button>
					</div>
					<div class="flex-1 overflow-auto p-4">
						{#if inspectModal.loading}
							<div class="flex items-center justify-center py-8">
								<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-text-muted);" />
							</div>
						{:else}
							<pre class="font-mono text-xs leading-relaxed whitespace-pre-wrap" style="color: var(--color-text-secondary);">{JSON.stringify(inspectModal.data, null, 2)}</pre>
						{/if}
					</div>
				</div>
			</div>
		{/if}

		<!-- ── Breadcrumb ── -->
		<div class="flex items-center gap-2 text-sm mb-2" style="color: var(--color-text-muted);">
			<a href="/servers" class="hover:underline" style="color: var(--color-text-muted);">Servers</a>
			<span>/</span>
			<span style="color: var(--color-text);">{server.name}</span>
		</div>

		<!-- ── Status Bar ── -->
		<div class="mb-4 h-1.5 w-full rounded-full transition-colors" style="background: linear-gradient(90deg, {server.status === 'online' ? 'var(--color-success)' : server.status === 'offline' ? 'var(--color-danger)' : 'var(--color-border-light)'} 0%, {server.status === 'online' ? 'var(--color-primary)' : server.status === 'offline' ? '#dc2626' : 'var(--color-border-light)'} 100%);"></div>

		<!-- ── Header ── -->
		<div class="flex flex-wrap items-start justify-between gap-3">
			<div class="flex items-center gap-4">
				<div>
					<h1 class="page-title">{server.name}</h1>
					<p class="page-subtitle">{server.host}:{server.port}</p>
				</div>
				<span class="status-badge {statusClass(server.status)}">
					<span class="h-1.5 w-1.5 rounded-full currentColor"></span>
					{server.status || 'unknown'}
				</span>
			</div>
			<div class="flex flex-wrap items-center gap-2">
				<button onclick={() => showEditModal = true} class="btn-secondary flex items-center gap-2">
					<Icon icon="solar:pen-bold" class="h-4 w-4" /> Edit
				</button>
				<button onclick={testConnection} disabled={testing} class="btn-secondary flex items-center gap-2">
					{#if testing}
						<Icon icon="solar:spinner-bold" class="h-4 w-4 animate-spin" />
					{:else}
						<Icon icon="solar:plug-circle-bold" class="h-4 w-4" />
					{/if}
					Test
				</button>
				<button onclick={confirmDelete} class="btn-secondary flex items-center gap-2" style="color: var(--color-danger);">
					<Icon icon="solar:trash-bin-trash-bold" class="h-4 w-4" /> Delete
				</button>
			</div>
		</div>

		<!-- ── Quick Actions ── -->
		<div class="mt-4 flex flex-wrap items-center gap-2">
			<a href={`/servers/${server.id}/terminal`} class="btn-secondary flex items-center gap-2">
				<Icon icon="solar:code-bold" class="h-4 w-4" /> Terminal
			</a>
			<button onclick={copySshCmd} class="btn-secondary flex items-center gap-2">
				<Icon icon={copyFeedback.show ? (copyFeedback.success ? 'solar:check-circle-bold' : 'solar:close-circle-bold') : 'solar:copy-bold'} class="h-4 w-4" style="color: {copyFeedback.show ? (copyFeedback.success ? 'var(--color-success)' : 'var(--color-danger)') : 'inherit'};" />
				{copyFeedback.show ? (copyFeedback.success ? 'Copied!' : 'Failed') : 'Copy SSH Cmd'}
			</button>
			<button onclick={loadContainers} disabled={containersLoading} class="btn-secondary flex items-center gap-2">
				<Icon icon="solar:refresh-bold" class="h-4 w-4 {containersLoading ? 'animate-spin' : ''}" /> Refresh
			</button>
		</div>

		<!-- ── Tab Navigation ── -->
		<div class="mt-5 flex gap-1 rounded-xl p-1" style="background: var(--color-surface); border: 1px solid var(--color-border-light);">
			{#each tabs as tab}
				<button
					onclick={() => switchTab(tab.id)}
					class="tab-btn flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-lg transition-all whitespace-nowrap"
					style={activeTab === tab.id
						? 'background: var(--color-card); color: var(--color-primary); box-shadow: 0 1px 3px rgba(0,0,0,0.08);'
						: 'color: var(--color-text-secondary); background: transparent;'}>
					<Icon icon={tab.icon} class="h-4 w-4" />
					{tab.label}
				</button>
			{/each}
		</div>

		<!-- ════════════════════════ TAB: OVERVIEW ════════════════════════ -->
		{#if activeTab === 'overview'}
			<!-- Connection Info -->
			<div class="mt-5">
				<h3 class="mb-2 text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">
					<Icon icon="solar:plug-circle-bold" class="inline-block h-3.5 w-3.5 -mt-0.5" style="color: var(--color-primary);" /> Connection
				</h3>
				<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
					<div class="card" style="border-left: 3px solid var(--color-primary);">
						<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-primary);">
							<Icon icon="solar:globus-bold" class="h-3.5 w-3.5" /> Host
						</div>
						<p class="font-mono text-sm font-semibold" style="color: var(--color-text);">{server.host}</p>
					</div>
					<div class="card" style="border-left: 3px solid var(--color-accent);">
						<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-accent);">
							<Icon icon="solar:plug-circle-bold" class="h-3.5 w-3.5" /> Port
						</div>
						<p class="font-mono text-sm font-semibold" style="color: var(--color-text);">{server.port || 22}</p>
					</div>
					<div class="card" style="border-left: 3px solid var(--color-warning);">
						<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-warning);">
							<Icon icon="solar:user-bold" class="h-3.5 w-3.5" /> SSH User
						</div>
						<p class="font-mono text-sm font-semibold" style="color: var(--color-text);">{server.ssh_user || 'root'}</p>
					</div>
					<div class="card" style="border-left: 3px solid var(--color-success);">
						<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-success);">
							<Icon icon="{server.ssh_auth_type === 'password' ? 'solar:lock-password-bold' : 'solar:key-minimalistic-bold'}" class="h-3.5 w-3.5" /> Auth
						</div>
						<p class="text-sm font-semibold" style="color: var(--color-text);">{server.ssh_auth_type === 'password' ? 'Password' : 'SSH Key'}</p>
					</div>
				</div>
			</div>

			<!-- Organization -->
			<div class="mt-4">
				<h3 class="mb-2 text-xs font-semibold uppercase tracking-wider" style="color: var(--color-text-muted);">
					<Icon icon="solar:folder-bold" class="inline-block h-3.5 w-3.5 -mt-0.5" style="color: var(--color-primary);" /> Organization
				</h3>
				<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
					{#if server.server_group}
						<div class="card" style="border-left: 3px solid var(--color-primary);">
							<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-primary);">
								<Icon icon="solar:folder-bold" class="h-3.5 w-3.5" /> Group
							</div>
							<p class="text-sm font-semibold" style="color: var(--color-text);">{server.server_group}</p>
						</div>
					{/if}
					{#if server.region}
						<div class="card" style="border-left: 3px solid var(--color-accent);">
							<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-accent);">
								<Icon icon="solar:map-point-bold" class="h-3.5 w-3.5" /> Region
							</div>
							<p class="text-sm font-semibold" style="color: var(--color-text);">{server.region}</p>
						</div>
					{/if}
					{#if server.server_type}
						<div class="card" style="border-left: 3px solid var(--color-warning);">
							<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-warning);">
								<Icon icon="solar:diploma-bold" class="h-3.5 w-3.5" /> Type
							</div>
							<p class="text-sm font-semibold" style="color: var(--color-text);">{server.server_type}</p>
						</div>
					{/if}
					{#if server.created_at}
						<div class="card" style="border-left: 3px solid var(--color-success);">
							<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-success);">
								<Icon icon="solar:calendar-bold" class="h-3.5 w-3.5" /> Added
							</div>
							<p class="text-sm font-semibold" style="color: var(--color-text);">{formatDate(server.created_at)}</p>
						</div>
					{/if}
				</div>
			</div>

			<!-- Tags -->
			{#if server.tags && server.tags.length > 0}
				<div class="mt-3 flex flex-wrap gap-1.5">
					{#each server.tags as tag}
						<span class="inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium" style="background-color: var(--color-primary-subtle); color: var(--color-primary);">
							{tag}
						</span>
					{/each}
				</div>
			{/if}

			<!-- Description -->
			{#if server.description}
				<div class="card mt-4">
					<p class="text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-text-muted);">Description</p>
					<p class="text-sm" style="color: var(--color-text-secondary);">{server.description}</p>
				</div>
			{/if}

			<!-- System Info -->
			<div class="card mt-4" style="border-left: 3px solid var(--color-accent);">
				<div class="flex items-center justify-between mb-3">
					<h3 class="text-base font-semibold" style="color: var(--color-text);">
						<Icon icon="solar:monitor-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-accent);" /> System Info
					</h3>
					<button onclick={autoDetect} disabled={detecting} class="btn-secondary flex items-center gap-1.5 text-xs">
						{#if detecting}
							<Icon icon="solar:spinner-bold" class="h-3.5 w-3.5 animate-spin" />
						{:else}
							<Icon icon="solar:scan-bold" class="h-3.5 w-3.5" />
						{/if}
						{server.os_info ? 'Re-detect' : 'Auto-detect'}
					</button>
				</div>
				<div class="grid gap-3 sm:grid-cols-2">
					<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); border-left: 2px solid var(--color-primary);">
						<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-primary);">
							<Icon icon="solar:monitor-bold" class="h-3 w-3" /> Operating System
						</div>
						<p class="mt-0.5 text-sm font-semibold" style="color: var(--color-text);">
							{server.os_info || '\u2014'}
						</p>
					</div>
					<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); border-left: 2px solid var(--color-warning);">
						<div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-warning);">
							<Icon icon="solar:cpu-bold" class="h-3 w-3" /> CPU
						</div>
						<p class="mt-0.5 text-sm font-semibold" style="color: var(--color-text);">
							{server.cpu_info || '\u2014'}
						</p>
					</div>
				</div>
			</div>

			<!-- Details -->
			<div class="card mt-4" style="border-left: 3px solid var(--color-primary);">
				<h3 class="mb-3 text-base font-semibold" style="color: var(--color-text);">
					<Icon icon="solar:info-circle-bold" class="inline-block h-4 w-4 -mt-0.5" style="color: var(--color-primary);" /> Details
				</h3>
				<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
					<div class="rounded-lg border p-3" style="border-color: var(--color-border-light);">
						<div class="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-text-muted);">
							<Icon icon="solar:hashtag-bold" class="h-3 w-3" /> Server ID
						</div>
						<p class="font-mono text-xs" style="color: var(--color-text-secondary);">{server.id}</p>
					</div>
					<div class="rounded-lg border p-3" style="border-color: var(--color-border-light);">
						<div class="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-text-muted);">
							<Icon icon="solar:calendar-bold" class="h-3 w-3" /> Created
						</div>
						<p class="text-sm" style="color: var(--color-text);">{formatDate(server.created_at)}</p>
					</div>
					<div class="rounded-lg border p-3" style="border-color: var(--color-border-light);">
						<div class="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-text-muted);">
							<Icon icon="solar:refresh-bold" class="h-3 w-3" /> Updated
						</div>
						<p class="text-sm" style="color: var(--color-text);">{formatDate(server.updated_at)}</p>
					</div>
					<div class="rounded-lg border p-3" style="border-color: var(--color-border-light);">
						<div class="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-text-muted);">
							<Icon icon="solar:box-bold" class="h-3 w-3" /> Containers
						</div>
						<p class="text-sm" style="color: var(--color-text);">{containers.length || server.container_count || '\u2014'}</p>
					</div>
				</div>
			</div>

		<!-- ════════════════════════ TAB: METRICS ════════════════════════ -->
		{:else if activeTab === 'metrics'}
			<!-- System Metrics -->
			<div class="card mt-5">
				<div class="flex items-center justify-between mb-3">
					<h3 class="text-base font-semibold" style="color: var(--color-text);">System Metrics</h3>
					<div class="flex items-center gap-2">
						<button onclick={toggleLiveRefresh} class="btn-icon" title="Toggle live refresh (10s)">
							<Icon icon={liveEnabled ? 'solar:pause-bold' : 'solar:play-bold'} class="h-4 w-4" style="color: {liveEnabled ? 'var(--color-success)' : 'var(--color-text-muted)'};" />
						</button>
						<button onclick={loadMetrics} class="btn-icon" title="Refresh" disabled={metricsLoading}>
							<Icon icon="solar:refresh-bold" class="h-4 w-4 {metricsLoading ? 'animate-spin' : ''}" />
						</button>
					</div>
				</div>

				{#if metricsLoading && !metrics}
					<div class="flex items-center justify-center py-8">
						<Icon icon="solar:spinner-bold" class="h-6 w-6 animate-spin" style="color: var(--color-text-muted);" />
					</div>
				{:else if metricsError && !metrics}
					<div class="rounded-lg border px-4 py-3 text-sm" style="background-color: rgba(239,68,68,0.08); border-color: rgba(239,68,68,0.2); color: var(--color-danger);">
						<div class="flex items-center gap-2">
							<Icon icon="solar:danger-triangle-bold" class="h-4 w-4 shrink-0" />
							<span>Failed to fetch metrics</span>
						</div>
						<p class="mt-1 text-xs opacity-80">{metricsError}</p>
					</div>
				{:else if metrics}
					<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
						<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); border-left: 3px solid var(--color-primary);">
							<div class="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-primary);">
								<Icon icon="solar:cpu-bold" class="h-3.5 w-3.5" /> CPU Load
							</div>
							<p class="mt-1 text-lg font-bold font-mono" style="color: var(--color-text);">{metrics.cpu?.load_1?.toFixed(2)}</p>
							<p class="text-xs" style="color: var(--color-text-muted);">1min: {metrics.cpu?.load_1?.toFixed(2)} / 5min: {metrics.cpu?.load_5?.toFixed(2)} / 15min: {metrics.cpu?.load_15?.toFixed(2)}</p>
						</div>
						<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); border-left: 3px solid var(--color-primary);">
							<div class="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-primary);">
								<Icon icon="solar:chart-bold" class="h-3.5 w-3.5" /> Memory
							</div>
							<p class="mt-1 text-lg font-bold" style="color: var(--color-text);">{formatBytes(metrics.memory?.used_bytes)}</p>
							<p class="text-xs" style="color: var(--color-text-muted);">of {formatBytes(metrics.memory?.total_bytes)}</p>
							<div class="mt-1.5 h-1.5 overflow-hidden rounded-full" style="background-color: var(--color-border-light);">
								<div class="h-full rounded-full" style="width: {(metrics.memory?.total_bytes > 0 ? (metrics.memory.used_bytes / metrics.memory.total_bytes * 100) : 0).toFixed(0)}%; background-color: {(metrics.memory?.total_bytes > 0 ? (metrics.memory.used_bytes / metrics.memory.total_bytes * 100) : 0) > 90 ? 'var(--color-danger)' : (metrics.memory?.total_bytes > 0 ? (metrics.memory.used_bytes / metrics.memory.total_bytes * 100) : 0) > 80 ? 'var(--color-warning)' : 'var(--color-primary)'};"></div>
							</div>
						</div>
						<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); border-left: 3px solid var(--color-warning);">
							<div class="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-warning);">
								<Icon icon="solar:database-bold" class="h-3.5 w-3.5" /> Disk
							</div>
							<p class="mt-1 text-lg font-bold" style="color: var(--color-text);">{formatBytes(metrics.disk?.used_bytes)}</p>
							<p class="text-xs" style="color: var(--color-text-muted);">of {formatBytes(metrics.disk?.total_bytes)} ({metrics.disk?.used_percent?.toFixed(1)}%)</p>
							<div class="mt-1.5 h-1.5 overflow-hidden rounded-full" style="background-color: var(--color-border-light);">
								<div class="h-full rounded-full" style="width: {metrics.disk?.used_percent || 0}%; background-color: {metrics.disk?.used_percent > 80 ? 'var(--color-danger)' : metrics.disk?.used_percent > 60 ? 'var(--color-warning)' : 'var(--color-primary)'};"></div>
							</div>
						</div>
						<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); border-left: 3px solid var(--color-accent);">
							<div class="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-accent);">
								<Icon icon="solar:clock-circle-bold" class="h-3.5 w-3.5" /> Uptime
							</div>
							<p class="mt-1 text-lg font-bold" style="color: var(--color-text);">{metrics.uptime?.replace('up ', '') || '\u2014'}</p>
						</div>
						{#if metrics.network}
							<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); border-left: 3px solid var(--color-success);">
								<div class="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-success);">
									<Icon icon="solar:download-square-bold" class="h-3.5 w-3.5" /> Network RX
								</div>
								<p class="mt-1 text-lg font-bold" style="color: var(--color-text);">{formatBytes(metrics.network.rx_bytes)}</p>
							</div>
							<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); border-left: 3px solid var(--color-accent);">
								<div class="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider mb-1" style="color: var(--color-accent);">
									<Icon icon="solar:upload-square-bold" class="h-3.5 w-3.5" /> Network TX
								</div>
								<p class="mt-1 text-lg font-bold" style="color: var(--color-text);">{formatBytes(metrics.network.tx_bytes)}</p>
							</div>
						{/if}
					</div>
				{:else}
					<div class="flex flex-col items-center py-8 text-center">
						<Icon icon="solar:chart-2-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-text-muted);" />
						<p class="text-sm" style="color: var(--color-text-muted);">Metrics not available</p>
						<p class="text-xs" style="color: var(--color-text-muted);">Server may be offline</p>
					</div>
				{/if}
			</div>

			<!-- Metrics History -->
			<div class="card mt-4">
				<div class="flex items-center justify-between mb-3">
					<h3 class="text-base font-semibold" style="color: var(--color-text);">Metrics History</h3>
					<div class="flex gap-1">
						<button onclick={() => changeRange('1h')} class="btn-icon text-xs" style="color: {historyRange === '1h' ? 'var(--color-primary)' : 'var(--color-text-muted)'};">1h</button>
						<button onclick={() => changeRange('6h')} class="btn-icon text-xs" style="color: {historyRange === '6h' ? 'var(--color-primary)' : 'var(--color-text-muted)'};">6h</button>
						<button onclick={() => changeRange('24h')} class="btn-icon text-xs" style="color: {historyRange === '24h' ? 'var(--color-primary)' : 'var(--color-text-muted)'};">24h</button>
						<button onclick={() => changeRange('7d')} class="btn-icon text-xs" style="color: {historyRange === '7d' ? 'var(--color-primary)' : 'var(--color-text-muted)'};">7d</button>
					</div>
				</div>

				{#if historyLoading}
					<div class="flex items-center justify-center py-6">
						<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-text-muted);" />
					</div>
				{:else if metricsHistory && metricsHistory.timestamps && metricsHistory.timestamps.length > 0}
					{@const ts = metricsHistory.timestamps}
					{@const cpu = metricsHistory.cpu}
					{@const mem = metricsHistory.mem}
					{@const disk = metricsHistory.disk}
					{@const netrx = metricsHistory.net_rx}
					{@const nettx = metricsHistory.net_tx}
					<div class="grid gap-4 sm:grid-cols-2">
						<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); background-color: var(--color-surface);">
							<MetricsChart title="CPU Load (1 min)" timestamps={ts}
								series={[{ label: 'CPU Load', data: cpu, color: '#10b981', scale: 'load' }]}
								height={180} yLabel="Load" formatY={(v) => v?.toFixed(2) ?? '\u2014'} />
						</div>
						<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); background-color: var(--color-surface);">
							<MetricsChart title="Memory Usage" timestamps={ts}
								series={[{ label: 'Memory %', data: mem, color: '#3b82f6', scale: '%' }]}
								height={180} yLabel="%" formatY={(v) => v != null ? v.toFixed(1) + '%' : '\u2014'} />
						</div>
						<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); background-color: var(--color-surface);">
							<MetricsChart title="Disk Usage" timestamps={ts}
								series={[{ label: 'Disk %', data: disk, color: '#f59e0b', scale: '%' }]}
								height={180} yLabel="%" formatY={(v) => v != null ? v.toFixed(1) + '%' : '\u2014'} />
						</div>
						<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); background-color: var(--color-surface);">
							<MetricsChart title="Network RX" timestamps={ts}
								series={[{ label: 'RX', data: netrx, color: '#8b5cf6', scale: 'bytes' }]}
								height={180} yLabel="Bytes" formatY={(v) => { if (v == null) return '\u2014'; if (v >= 1e12) return (v / 1e12).toFixed(1) + ' TB'; if (v >= 1e9) return (v / 1e9).toFixed(1) + ' GB'; if (v >= 1e6) return (v / 1e6).toFixed(1) + ' MB'; if (v >= 1e3) return (v / 1e3).toFixed(1) + ' KB'; return v + ' B'; }} />
						</div>
						<div class="rounded-lg border p-3" style="border-color: var(--color-border-light); background-color: var(--color-surface);">
							<MetricsChart title="Network TX" timestamps={ts}
								series={[{ label: 'TX', data: nettx, color: '#ec4899', scale: 'bytes' }]}
								height={180} yLabel="Bytes" formatY={(v) => { if (v == null) return '\u2014'; if (v >= 1e12) return (v / 1e12).toFixed(1) + ' TB'; if (v >= 1e9) return (v / 1e9).toFixed(1) + ' GB'; if (v >= 1e6) return (v / 1e6).toFixed(1) + ' MB'; if (v >= 1e3) return (v / 1e3).toFixed(1) + ' KB'; return v + ' B'; }} />
						</div>
					</div>
				{:else}
					<div class="flex flex-col items-center py-6 text-center">
						<Icon icon="solar:chart-2-bold" class="mb-1 inline-block h-6 w-6" style="color: var(--color-text-muted);" />
						<p class="text-xs" style="color: var(--color-text-muted);">Collecting metrics data... Check back later</p>
					</div>
				{/if}
			</div>

		<!-- ════════════════════════ TAB: CONTAINERS ════════════════════════ -->
		{:else if activeTab === 'containers'}
			<div class="card mt-5">
				<div class="flex items-center justify-between mb-3">
					<h3 class="text-base font-semibold" style="color: var(--color-text);">Containers</h3>
					<span class="text-xs" style="color: var(--color-text-muted);">{containers.length} container{containers.length !== 1 ? 's' : ''}</span>
				</div>

				{#if containersLoading && containers.length === 0}
					<div class="flex items-center justify-center py-8">
						<Icon icon="solar:spinner-bold" class="h-6 w-6 animate-spin" style="color: var(--color-text-muted);" />
					</div>
				{:else if containers.length === 0}
					<div class="flex flex-col items-center py-8 text-center">
						<Icon icon="solar:box-bold" class="mb-2 inline-block h-8 w-8" style="color: var(--color-text-muted);" />
						<p class="text-sm" style="color: var(--color-text-muted);">No containers found</p>
						<p class="text-xs" style="color: var(--color-text-muted);">Docker may not be installed on this server</p>
					</div>
				{:else}
					<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
						{#each containers as c}
							{@const isRunning = c.state === 'running'}
							<div
								class="rounded-lg border text-left w-full min-w-0 transition-all shadow-sm hover:shadow-md cursor-pointer"
								style="background-color: var(--color-surface); border-color: var(--color-border); border-left: 3px solid {containerBorderColor(c.state)};"
								onclick={() => viewInspect(c.id, c.name)}
							>
								<div class="px-4 py-3 border-b" style="border-color: var(--color-border-light);">
									<div class="flex items-center justify-between gap-2">
										<div class="flex items-center gap-2 min-w-0">
											<span class="h-2 w-2 shrink-0 rounded-full" style="background-color: {isRunning ? 'var(--color-success)' : 'var(--color-text-muted)'};"></span>
											<span class="text-sm font-semibold font-mono truncate" style="color: var(--color-text);" title={c.name}>{c.name}</span>
										</div>
										<span class="status-badge {containerStatusClass(c.state)} text-xs shrink-0">
											{c.state || c.status}
										</span>
									</div>
								</div>
								<div class="px-4 py-2.5 space-y-1.5">
									<div class="flex items-center gap-2 text-xs" style="color: var(--color-text-muted);">
										<Icon icon="solar:box-bold" class="h-3 w-3 shrink-0" />
										<span class="truncate" title={c.image}>{c.image}</span>
									</div>
									{#if c.ports}
										<div class="flex items-center gap-2 text-xs" style="color: var(--color-text-muted);">
											<Icon icon="solar:plug-circle-bold" class="h-3 w-3 shrink-0" />
											<span class="truncate">{c.ports}</span>
										</div>
									{/if}
									{#if c.created}
										<div class="flex items-center gap-2 text-xs" style="color: var(--color-text-muted);">
											<Icon icon="solar:clock-circle-bold" class="h-3 w-3 shrink-0" />
											<span class="truncate">{c.created}</span>
										</div>
									{/if}
								</div>
								<div class="flex items-center gap-1 border-t px-4 py-2" style="border-color: var(--color-border-light);" onclick={(e) => e.stopPropagation()}>
									{#if isRunning}
										<button onclick={() => containerAction(c.id, 'stop')} class="btn-icon h-7 w-7" title="Stop" style="color: var(--color-danger);">
											<Icon icon={containerActions[c.id + ':stop'] === 'loading' ? 'solar:spinner-bold' : 'solar:pause-bold'} class="h-3.5 w-3.5 {containerActions[c.id + ':stop'] === 'loading' ? 'animate-spin' : ''}" />
										</button>
										<button onclick={() => containerAction(c.id, 'restart')} class="btn-icon h-7 w-7" title="Restart" style="color: var(--color-warning);">
											<Icon icon={containerActions[c.id + ':restart'] === 'loading' ? 'solar:spinner-bold' : 'solar:refresh-bold'} class="h-3.5 w-3.5 {containerActions[c.id + ':restart'] === 'loading' ? 'animate-spin' : ''}" />
										</button>
									{:else}
										<button onclick={() => containerAction(c.id, 'start')} class="btn-icon h-7 w-7" title="Start" style="color: var(--color-success);">
											<Icon icon={containerActions[c.id + ':start'] === 'loading' ? 'solar:spinner-bold' : 'solar:play-bold'} class="h-3.5 w-3.5 {containerActions[c.id + ':start'] === 'loading' ? 'animate-spin' : ''}" />
										</button>
									{/if}
									<span class="flex-1"></span>
									<button onclick={() => viewLogs(c.id, c.name)} class="btn-icon h-7 w-7" title="Logs">
										<Icon icon="solar:document-text-bold" class="h-3.5 w-3.5" />
									</button>
									<button onclick={() => viewInspect(c.id, c.name)} class="btn-icon h-7 w-7" title="Inspect">
										<Icon icon="solar:code-bold" class="h-3.5 w-3.5" />
									</button>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>

		<!-- ════════════════════════ TAB: COMPLIANCE ════════════════════════ -->
		{:else if activeTab === 'compliance'}
			<div class="mt-5">
				{#if scanLoading}
					<div class="card flex items-center justify-center py-12">
						<div class="flex flex-col items-center gap-3">
							<Icon icon="solar:spinner-bold" class="h-8 w-8 animate-spin" style="color: var(--color-primary);" />
							<p class="text-sm" style="color: var(--color-text-muted);">Loading compliance data...</p>
						</div>
					</div>
				{:else}
					<!-- Profile Selector & Scan Buttons -->
					<div class="flex flex-wrap items-center justify-between gap-3 mb-4">
						<div class="flex items-center gap-2 p-1 rounded-lg" style="background: var(--color-surface); border: 1px solid var(--color-border-light);">
							<button onclick={() => switchProfile('cis_level_1')} class="px-3 py-1.5 text-xs font-medium rounded-md transition-all"
								style={profile === 'cis_level_1' ? 'background: var(--color-primary); color: #fff;' : 'color: var(--color-text-secondary); background: transparent;'}>
								CIS L1
							</button>
							<button onclick={() => switchProfile('cis_level_2')} class="px-3 py-1.5 text-xs font-medium rounded-md transition-all"
								style={profile === 'cis_level_2' ? 'background: var(--color-warning); color: #fff;' : 'color: var(--color-text-secondary); background: transparent;'}>
								CIS L2
							</button>
							<button onclick={() => switchProfile('lynis')} class="px-3 py-1.5 text-xs font-medium rounded-md transition-all"
								style={profile === 'lynis' ? 'background: var(--color-accent); color: #fff;' : 'color: var(--color-text-secondary); background: transparent;'}>
								Lynis
							</button>
						</div>
						<div class="flex items-center gap-2">
							{#if isLynisProfile}
								<button onclick={runLynisScan} disabled={scanning}
									class="btn-secondary flex items-center gap-2 text-sm"
									style="border-color: var(--color-accent); color: var(--color-accent);">
									<Icon icon={scanning ? 'solar:spinner-bold' : 'solar:bug-bold'}
										class="h-4 w-4 {scanning ? 'animate-spin' : ''}" />
									{scanning ? 'Scanning...' : 'Run Lynis'}
								</button>
							{:else}
								<button onclick={runScan} disabled={scanning}
									class="btn-secondary flex items-center gap-2 text-sm"
									style="border-color: var(--color-primary); color: var(--color-primary);">
									<Icon icon={scanning ? 'solar:spinner-bold' : 'solar:play-bold'}
										class="h-4 w-4 {scanning ? 'animate-spin' : ''}" />
									{scanning ? 'Scanning...' : 'Run ' + (profile === 'cis_level_1' ? 'CIS L1' : 'CIS L2')}
								</button>
							{/if}
						</div>
				</div>

				<!-- Reference link -->
				<div class="flex items-center justify-end mb-4">
					<a href={profile === 'lynis' ? '/compliance/lynis' : profile === 'cis_level_2' ? '/compliance/cis-level-2' : '/compliance/cis-level-1'} class="text-xs flex items-center gap-1.5 font-medium" style="color: var(--color-text-muted);">
						View complete {profile === 'lynis' ? 'Lynis' : profile === 'cis_level_2' ? 'CIS L2' : 'CIS L1'} scan details →
					</a>
				</div>

			<!-- Scan Info Banner -->
			{#if scan}
				{#if isHistoricalScan}
					<div class="card !p-3 mb-4" style="border-left: 4px solid var(--color-accent); background: rgba(139,92,246,0.06);">
						<div class="flex items-center justify-between flex-wrap gap-2">
							<div class="flex items-center gap-2">
								<Icon icon="solar:history-bold" class="h-4 w-4 shrink-0" style="color: var(--color-accent);" />
								<span class="text-xs font-semibold" style="color: var(--color-accent);">Historical Scan</span>
								<span class="text-xs" style="color: var(--color-text-muted);">· {formatTimeFull(scan.created_at || scan.completed_at)}</span>
								{#if scan.score !== null && scan.score !== undefined}
									<span class="text-xs" style="color: var(--color-text-muted);">· Score: {scan.score}</span>
								{/if}
							</div>
							<button onclick={() => switchProfile(profile)} class="text-xs font-medium px-2.5 py-1 rounded transition-all" style="color: var(--color-primary); background: rgba(16,185,129,0.1); border: none; cursor: pointer;">← View Latest Scan</button>
						</div>
					</div>
				{:else}
					<div class="flex items-center gap-2 mb-4 px-1">
						<Icon icon="solar:check-circle-bold" class="h-3.5 w-3.5" style="color: var(--color-success);" />
						<span class="text-xs" style="color: var(--color-text-muted);">Latest scan · {formatTimeFull(scan.created_at || scan.completed_at)}</span>
					</div>
				{/if}
			{/if}

			<!-- Error -->
				{#if scanError && !scan}
					<div class="card flex flex-col items-center gap-3 py-6 text-center mb-4" style="border-left: 4px solid var(--color-danger);">
						<Icon icon="solar:danger-triangle-bold" class="h-6 w-6" style="color: var(--color-danger);" />
						<p class="text-sm" style="color: var(--color-danger);">{scanError}</p>
					</div>
				{:else if scan && scan.status === 'failed' && scan.error_message}
					<div class="card flex flex-col items-center gap-3 py-6 text-center mb-4" style="border-left: 4px solid var(--color-danger);">
						<Icon icon="solar:danger-triangle-bold" class="h-6 w-6" style="color: var(--color-danger);" />
						<p class="text-sm font-medium" style="color: var(--color-danger);">Scan Failed</p>
						<p class="text-sm" style="color: var(--color-text-secondary);">{scan.error_message}</p>
					</div>
				{/if}

					{#if isLynisProfile && lynisData}
						<!-- Lynis Stats -->
						<div class="grid gap-3 grid-cols-5 mb-4">
							<div class="stat-card !py-4 !px-3 text-center" style="border-top: 3px solid var(--color-accent);">
								<p class="text-2xl font-bold" style="color: var(--color-accent);">{lynisData.hardening_score || '—'}</p>
								<p class="text-[10px] uppercase tracking-wider mt-0.5" style="color: var(--color-text-muted);">Hardening Index</p>
							</div>
							<div class="stat-card !py-4 !px-3 text-center" style="border-top: 3px solid #6b7280;">
								<p class="text-2xl font-bold" style="color: var(--color-text);">{lynisData.tests || '—'}</p>
								<p class="text-[10px] uppercase tracking-wider mt-0.5" style="color: var(--color-text-muted);">Tests</p>
							</div>
							<div class="stat-card !py-4 !px-3 text-center" style="border-top: 3px solid var(--color-success);">
								<p class="text-2xl font-bold" style="color: var(--color-success);">{lynisData.tests ? lynisData.tests - (lynisData.warnings || 0) - (lynisData.suggestions || 0) : '—'}</p>
								<p class="text-[10px] uppercase tracking-wider mt-0.5" style="color: var(--color-text-muted);">Passed</p>
							</div>
							<div class="stat-card !py-4 !px-3 text-center" style="border-top: 3px solid var(--color-warning);">
								<p class="text-2xl font-bold" style="color: var(--color-warning);">{lynisData.warnings || 0}</p>
								<p class="text-[10px] uppercase tracking-wider mt-0.5" style="color: var(--color-text-muted);">Warnings</p>
							</div>
							<div class="stat-card !py-4 !px-3 text-center" style="border-top: 3px solid var(--color-accent);">
								<p class="text-2xl font-bold" style="color: var(--color-accent);">{lynisData.suggestions || 0}</p>
								<p class="text-[10px] uppercase tracking-wider mt-0.5" style="color: var(--color-text-muted);">Suggestions</p>
							</div>
						</div>

						<!-- Lynis Warnings & Suggestions -->
						<div class="grid gap-4 grid-cols-2 mb-4">
							{#if lynisData.warnings_list?.length > 0}
								<div style="background: #fffbeb; border: 1px solid #fde68a; border-radius: 10px; padding: 14px;">
									<h4 class="text-xs font-semibold" style="color: #d97706; margin-bottom: 8px;">🟡 Warnings</h4>
									{#each lynisData.warnings_list.slice(0, 4) as w}
										<p class="text-xs mb-1.5"><span class="font-mono" style="color: var(--color-accent);">{w.test_id}</span> · {w.description}</p>
									{/each}
								</div>
							{/if}
							{#if lynisData.suggestions_list?.length > 0}
								<div style="background: #f5f3ff; border: 1px solid #ddd6fe; border-radius: 10px; padding: 14px;">
									<h4 class="text-xs font-semibold" style="color: #7c3aed; margin-bottom: 8px;">💡 Suggestions</h4>
									{#each lynisData.suggestions_list.slice(0, 4) as s}
										<p class="text-xs mb-1.5"><span class="font-mono" style="color: var(--color-accent);">{s.test_id}</span> · {s.description}</p>
									{/each}
								</div>
							{/if}
						</div>

					{:else if !isLynisProfile}
						<!-- CIS Score + Stats -->
						{#if scan}
							<div class="grid gap-3 grid-cols-5 mb-4">
								<div class="stat-card !py-4 !px-3 text-center" style="border-top: 3px solid {scoreColor(score)};">
									<p class="text-2xl font-bold" style="color: {scoreColor(score)};">{score ?? '—'}</p>
									<p class="text-[10px] uppercase tracking-wider mt-0.5" style="color: var(--color-text-muted);">Score</p>
								</div>
								<div class="stat-card !py-4 !px-3 text-center" style="border-top: 3px solid #6b7280;">
									<p class="text-2xl font-bold" style="color: var(--color-text);">{scanStats.total}</p>
									<p class="text-[10px] uppercase tracking-wider mt-0.5" style="color: var(--color-text-muted);">Total Checks</p>
								</div>
								<div class="stat-card !py-4 !px-3 text-center" style="border-top: 3px solid var(--color-success);">
									<p class="text-2xl font-bold" style="color: var(--color-success);">{scanStats.passed}</p>
									<p class="text-[10px] uppercase tracking-wider mt-0.5" style="color: var(--color-text-muted);">Passed</p>
								</div>
								<div class="stat-card !py-4 !px-3 text-center" style="border-top: 3px solid var(--color-warning);">
									<p class="text-2xl font-bold" style="color: var(--color-warning);">{scanStats.high + scanStats.medium + scanStats.low}</p>
									<p class="text-[10px] uppercase tracking-wider mt-0.5" style="color: var(--color-text-muted);">Warnings</p>
								</div>
								<div class="stat-card !py-4 !px-3 text-center" style="border-top: 3px solid var(--color-danger);">
									<p class="text-2xl font-bold" style="color: var(--color-danger);">{scanStats.critical}</p>
									<p class="text-[10px] uppercase tracking-wider mt-0.5" style="color: var(--color-text-muted);">Failed</p>
								</div>
							</div>

							<!-- Category Cards Grid -->
							<h4 class="text-sm font-semibold mb-3" style="color: var(--color-text);">Categories</h4>
							{#if profileCategories.length > 0}
								<div class="grid gap-2 mb-4" style="grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));">
									{#each profileCategories as cat}
										<div class="category-card-compact" style="border-left: 3px solid {cat.meta.color}; cursor: pointer;" onclick={() => selectCategory(cat)}>
											<div class="flex items-center justify-between">
												<span class="text-xs font-semibold">{cat.meta.icon} {cat.meta.label}</span>
												<span class="text-sm font-bold" style="color: {cat.meta.color};">{cat.total > 0 ? Math.round((cat.passed / cat.total) * 100) : 0}%</span>
											</div>
											<p class="text-[10px] mt-1" style="color: var(--color-text-muted);">{cat.total} checks · ✓{cat.passed} ⚠{cat.warnings} ✗{cat.criticals}</p>
											<div class="progress-track mt-1.5">
												<div class="progress-fill" style="width: {cat.total > 0 ? (cat.passed / cat.total) * 100 : 0}%; background: {cat.meta.color};"></div>
											</div>
										</div>
									{/each}
								</div>
							{:else}
								<div class="card mb-4 py-4 text-center">
									<p class="text-sm" style="color: var(--color-text-muted);">No category data available. Run a scan first.</p>
								</div>
							{/if}

							<!-- ── Category Detail Panel ── -->
							{#if selectedCategory}
								<div class="card mb-4" style="border-left: 4px solid {selectedCategory.meta.color}; overflow: hidden;">
									<!-- Panel header -->
									<div class="flex items-center gap-2 mb-4">
										<button onclick={closeCategoryPanel} class="text-xs text-gray-400 hover:text-white flex items-center gap-1">
											← Back to Categories
										</button>
										<span class="text-gray-600">|</span>
										<span>{selectedCategory.meta.icon}</span>
										<h4 class="text-sm font-bold" style="color: var(--color-text);">{selectedCategory.meta.label}</h4>
										<span class="text-[10px] font-mono" style="color: var(--color-text-muted);">({selectedCategory.category})</span>
									</div>

									<!-- Sub stats -->
									<div class="grid gap-3 mb-4" style="grid-template-columns: repeat(4, 1fr);">
										<div class="stat-card !py-3">
											<p class="text-lg font-bold" style="color: var(--color-text);">{selectedCategory.total || 0}</p>
											<p class="text-[10px] uppercase tracking-wider" style="color: var(--color-text-muted);">Checks</p>
										</div>
										<div class="stat-card !py-3" style="border-top: 3px solid var(--color-success);">
											<p class="text-lg font-bold" style="color: var(--color-success);">{selectedCategory.passed || 0}</p>
											<p class="text-[10px] uppercase tracking-wider" style="color: var(--color-text-muted);">Passed</p>
										</div>
										<div class="stat-card !py-3" style="border-top: 3px solid var(--color-warning);">
											<p class="text-lg font-bold" style="color: var(--color-warning);">{selectedCategory.warnings || 0}</p>
											<p class="text-[10px] uppercase tracking-wider" style="color: var(--color-text-muted);">Warnings</p>
										</div>
										<div class="stat-card !py-3" style="border-top: 3px solid var(--color-danger);">
											<p class="text-lg font-bold" style="color: var(--color-danger);">{selectedCategory.criticals || 0}</p>
											<p class="text-[10px] uppercase tracking-wider" style="color: var(--color-text-muted);">Failed</p>
										</div>
									</div>

									<!-- Sub tabs -->
									<div class="flex items-center gap-4 mb-4" style="border-bottom: 1px solid var(--color-border-light);">
										<button onclick={() => catDetailTab = 'checks'} class="px-2 pb-2 text-xs font-semibold transition-all"
											style={catDetailTab === 'checks' ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary);' : 'color: var(--color-text-muted); border-bottom: 2px solid transparent;'}>
											📋 Checks ({categoryItems.length})
										</button>
										<button onclick={() => catDetailTab = 'history'} class="px-2 pb-2 text-xs font-semibold transition-all"
											style={catDetailTab === 'history' ? 'color: var(--color-primary); border-bottom: 2px solid var(--color-primary);' : 'color: var(--color-text-muted); border-bottom: 2px solid transparent;'}>
											📜 Category History
										</button>
									</div>

									<!-- Checks tab -->
									{#if catDetailTab === 'checks'}
										{#if categoryItems.length > 0}
											<div class="space-y-2 max-h-72 overflow-y-auto pr-1">
												{#each categoryItems as finding}
													<div class="flex items-start gap-3 p-2.5 rounded-lg" style="background: var(--color-surface); border: 1px solid var(--color-border-light);">
														<div class="w-5 h-5 rounded-full flex items-center justify-center shrink-0 text-[10px] font-bold"
															style="background: {finding.status === 'pass' ? 'rgba(16,185,129,0.15)' : finding.status === 'warn' ? 'rgba(245,158,11,0.15)' : 'rgba(239,68,68,0.15)'}; color: {finding.status === 'pass' ? 'var(--color-success)' : finding.status === 'warn' ? 'var(--color-warning)' : 'var(--color-danger)'};">{finding.status === 'pass' ? '✓' : finding.status === 'warn' ? '⚠' : '✗'}</div>
														<div class="flex-1 min-w-0">
															<div class="flex items-center gap-2 flex-wrap">
																<span class="text-xs font-medium" style="color: var(--color-text);">{finding.title || finding.check_id || 'Unknown check'}</span>
															</div>
															{#if finding.description}
																<p class="text-[10px] mt-0.5" style="color: var(--color-text-muted);">{finding.description}</p>
															{/if}
														</div>
														<div>
															<span class="text-[10px] px-1.5 py-0.5 rounded-full font-medium"
																style="background: {finding.status === 'pass' ? 'rgba(16,185,129,0.12)' : finding.status === 'warn' ? 'rgba(245,158,11,0.12)' : 'rgba(239,68,68,0.12)'}; color: {finding.status === 'pass' ? 'var(--color-success)' : finding.status === 'warn' ? 'var(--color-warning)' : 'var(--color-danger)'};">{finding.status}</span>
														</div>
													</div>
												{/each}
											</div>
										{:else}
											<p class="text-xs py-4 text-center" style="color: var(--color-text-muted);">No findings for this category.</p>
										{/if}
									{:else}

									<!-- History tab -->
									{#if catHistoryLoading}
										<div class="flex items-center justify-center py-6">
											<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-text-muted);" />
										</div>
									{:else if catHistoryData.length > 0}
										<div class="max-h-72 overflow-y-auto pr-1 space-y-1">
											{#each catHistoryData as h}
												<div class="flex items-center justify-between py-2" style="border-bottom: 1px solid var(--color-border-light);">
													<div class="flex items-center gap-2">
														<div class="w-2 h-2 rounded-full" style="background: {scoreColor(h.score)};"></div>
														<div>
															<p class="text-xs font-medium" style="color: var(--color-text);">{h.scan_type || 'Scan'} · {h.score ?? '—'}%</p>
															<p class="text-[10px]" style="color: var(--color-text-muted);">{formatTimeFull(h.created_at)}</p>
														</div>
													</div>
													<div class="flex items-center gap-2 text-[10px]">
														<span>✓{h.passed || 0}</span>
														<span style="color: var(--color-warning);">⚠{h.warnings || 0}</span>
														<span style="color: var(--color-danger);">✗{h.criticals || 0}</span>
													</div>
												</div>
											{/each}
										</div>
									{:else}
										<p class="text-xs py-4 text-center" style="color: var(--color-text-muted);">No history for this category yet.</p>
									{/if}
									{/if}
								</div>
							{/if}

							<!-- No scan yet -->
						{:else}
							<div class="card mb-4 py-6 text-center" style="border-left: 4px solid var(--color-text-muted);">
								<Icon icon="solar:shield-warning-bold" class="h-6 w-6 mb-2" style="color: var(--color-text-muted);" />
								<p class="text-sm" style="color: var(--color-text-muted);">
									No <strong>{profile === 'cis_level_2' ? 'CIS Level 2' : 'CIS Level 1'}</strong> scan data yet.
									Click <strong>Run Scan</strong> above to start.
								</p>
							</div>
						{/if}
					{/if}

					<!-- Failed Findings Summary -->
					{#if summaryCounts.total > 0}
						<div class="card !p-3 mb-4">
							<div class="flex items-center gap-3 text-sm font-medium">
								{#if summaryCounts.critical > 0}
									<span style="color: var(--color-danger);">✗ {summaryCounts.critical} critical</span>
								{/if}
								{#if summaryCounts.high > 0}
									<span style="color: var(--color-warning);">✗ {summaryCounts.high} high</span>
								{/if}
								{#if summaryCounts.medium > 0}
									<span style="color: var(--color-accent);">✗ {summaryCounts.medium} medium</span>
								{/if}
								{#if summaryCounts.low > 0}
									<span style="color: var(--color-text-muted);">✗ {summaryCounts.low} low</span>
								{/if}
							</div>
						</div>
					{/if}

					<!-- Scan History -->
					<div class="card">
						<h4 class="text-sm font-semibold mb-3" style="color: var(--color-text);">📜 Scan History</h4>
						{#if compHistoryLoading}
							<div class="flex items-center justify-center py-4">
								<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-text-muted);" />
							</div>
						{:else if filteredHistory.length > 0}
							<div>
								{#each filteredHistory as item}
									<div class="flex items-center justify-between py-2.5 border-b" style="border-color: var(--color-border-light);">
										<div class="flex items-center gap-3">
											<div class="w-2 h-2 rounded-full {((item.score ?? 0) >= 80) ? 'bg-green' : ((item.score ?? 0) >= 60) ? 'bg-amber' : 'bg-red'}" 
												style="background: {scoreColor(item.score)};"></div>
											<div>
												<p class="text-sm font-medium" style="color: var(--color-text);">
													{item.scan_type || item.profile || 'Scan'} · {item.score ?? '—'}%
												</p>
												<p class="text-xs" style="color: var(--color-text-muted);">{formatTimeFull(item.scanned_at || item.created_at)}</p>
											</div>
										</div>
										<div class="flex items-center gap-3 text-xs">
											<span>✓{item.passed_count || item.passed || 0}</span>
											<span style="color: var(--color-warning);">⚠{item.warning_count || item.warnings || 0}</span>
											<span style="color: var(--color-danger);">✗{item.critical_count || item.criticals || 0}</span>
											<span class="font-medium" style="color: var(--color-primary); cursor: pointer;" onclick={() => viewHistoricScan(item)}>View →</span>
										</div>
									</div>
								{/each}
							</div>
							{#if filteredScanCount > 5}
								<p class="text-xs text-center mt-3 font-medium" style="color: var(--color-primary); cursor: pointer;"
									onclick={() => showAllHistory = !showAllHistory}>
									{showAllHistory ? 'Show less ↑' : 'Show all ' + filteredScanCount + ' scans ↓'}
								</p>
							{/if}
						{:else}
							<p class="text-sm py-4 text-center" style="color: var(--color-text-muted);">No scan history yet.</p>
						{/if}
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</div>

<AddServerModal
	show={showEditModal}
	editServer={server}
	onClose={() => showEditModal = false}
	onSaved={(updated) => {
		server = updated;
		showEditModal = false;
	}}
/>

<style>
	.tab-btn { cursor: pointer; }
	.tab-btn:hover:not([style*="box-shadow"]) { color: var(--color-primary) !important; }
	.stat-card {
		background: var(--color-card);
		border-radius: 10px;
		border: 1px solid var(--color-border-light);
		padding: 16px;
	}
	.category-card-compact {
		background: var(--color-card);
		border-radius: 8px;
		border: 1px solid var(--color-border-light);
		padding: 12px;
		cursor: pointer;
		transition: all 0.15s;
	}
	.category-card-compact:hover {
		border-color: var(--color-primary);
		box-shadow: 0 2px 8px rgba(16,185,129,0.08);
	}
\t.progress-track { height: 5px; border-radius: 3px; background: var(--color-border); overflow: hidden; }
	.progress-fill { height: 100%; border-radius: 3px; transition: width 0.5s; }
	.whats-scanned summary { user-select: none; }
	.whats-scanned summary::-webkit-details-marker { display: none; }
	.whats-scanned summary::marker { display: none; }
	.bg-green { background: var(--color-success); }
	.bg-amber { background: var(--color-warning); }
	.bg-red { background: var(--color-danger); }
</style>
