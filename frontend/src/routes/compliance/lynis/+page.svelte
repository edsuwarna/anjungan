<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api.svelte.js';
	import { goto } from '$app/navigation';
import { loadThresholds, scoreColor, scoreLabel } from '$lib/thresholds.svelte.js';
	import Icon from '@iconify/svelte';

	const profileColor = '#8b5cf6';

	// ── Scan ──
	let servers = $state([]);
	let selectedServerId = $state('');
	let scanning = $state(false);
	let scanMessage = $state('');

	// ── Scan History ──
	let history = $state([]);
	let historyLoading = $state(false);
	let historyPage = $state(1);
	let historyTotal = $state(0);
	const historyLimit = 10;

	const categories = [
		{
			icon: '🔐', label: 'Authentication',
			color: '#3b82f6',
			totalTests: 18,
			desc: 'Verifies password policies, PAM configuration, SSH key settings, account lockout mechanisms, and multi-factor authentication readiness.',
			tests: [
				{ id: 'AUTH-9225', title: 'Password Hashing', desc: 'Verify password hashing algorithm in /etc/login.defs uses SHA512 or yescrypt' },
				{ id: 'AUTH-9226', title: 'Password History', desc: 'Check password reuse restrictions in /etc/pam.d/common-password' },
				{ id: 'AUTH-9252', title: 'Account Policy', desc: 'Verify password aging parameters (max days, min days, warn age) in /etc/login.defs' },
				{ id: 'AUTH-9262', title: 'Kerberos', desc: 'Check Kerberos authentication configuration if applicable' },
				{ id: 'AUTH-9265', title: 'LDAP Auth', desc: 'Verify LDAP authentication configuration and TLS settings' },
				{ id: 'AUTH-9268', title: 'NIS Auth', desc: 'Check if NIS authentication is in use and properly secured' },
				{ id: 'AUTH-9282', title: 'Password Policy', desc: 'Check password aging, complexity, and reuse restrictions in /etc/login.defs and /etc/pam.d/' },
				{ id: 'AUTH-9284', title: 'SSH Key Settings', desc: 'Ensure SSH key directories and files have proper permissions (600/700)' },
				{ id: 'AUTH-9285', title: 'SSH Protocol', desc: 'Verify SSH protocol version and allowed authentication methods' },
				{ id: 'AUTH-9286', title: 'PAM Configuration', desc: 'Verify Pluggable Authentication Modules are properly configured for secure authentication' },
				{ id: 'AUTH-9288', title: 'Passwordless Accounts', desc: 'Check for accounts with empty passwords in /etc/shadow' },
				{ id: 'AUTH-9289', title: 'Root Login', desc: 'Check direct root login restrictions via SSH PermitRootLogin' },
				{ id: 'AUTH-9290', title: 'U2F/OTP Devices', desc: 'Check if multi-factor authentication devices are configured' },
				{ id: 'AUTH-9292', title: 'Session Timeout', desc: 'Verify TMOUT is set in system-wide shell configuration' },
				{ id: 'AUTH-9328', title: 'User Lockout', desc: 'Check account lockout after failed login attempts via pam_tally2 or pam_faillock' },
				{ id: 'AUTH-9330', title: 'Sudoers Security', desc: 'Verify sudoers file permissions and configuration safety' },
				{ id: 'AUTH-9331', title: 'Sudo Password', desc: 'Check if sudo requires password or has NOPASSWD entries' },
				{ id: 'AUTH-9400', title: 'Groups & Membership', desc: 'Verify critical group (root, shadow, wheel) memberships' },
			]
		},
		{
			icon: '📁', label: 'File Systems',
			color: '#f59e0b',
			totalTests: 22,
			desc: 'Audits file system permissions, mount options, disk quotas, world-writable files, and sensitive directory protections.',
			tests: [
				{ id: 'FILE-6250', title: 'Root FS', desc: 'Verify root file system is not world-writable and has no unusual mount options' },
				{ id: 'FILE-6255', title: 'Tmp Partition', desc: 'Check if /tmp is a separate partition with nodev,nosuid,noexec' },
				{ id: 'FILE-6260', title: 'Var Tmp Partition', desc: 'Check if /var/tmp is a separate partition or bind-mounted with restrictions' },
				{ id: 'FILE-6270', title: 'Dev Shm Hardening', desc: 'Verify /dev/shm is mounted with nodev,nosuid,noexec' },
				{ id: 'FILE-6280', title: 'File Permissions', desc: 'Check critical system file permissions (/etc/passwd, /etc/shadow, /etc/group)' },
				{ id: 'FILE-6290', title: 'Home Perms', desc: 'Check home directory permissions are 750 or stricter' },
				{ id: 'FILE-6300', title: 'Cron File Perms', desc: 'Verify cron, anacron, and at file permissions and ownership' },
				{ id: 'FILE-6310', title: 'SUID Check', desc: 'Scan for new SUID/SGID files since last audit' },
				{ id: 'FILE-6328', title: 'Mount Options', desc: 'Verify partition mount options (nodev, nosuid, noexec on /tmp, /var/tmp, /dev/shm)' },
				{ id: 'FILE-6330', title: 'Unused Partitions', desc: 'Check for unused but mounted file systems' },
				{ id: 'FILE-6340', title: 'Banners', desc: 'Verify /etc/issue and /etc/issue.net contain appropriate legal banners' },
				{ id: 'FILE-6350', title: 'Sensitive Files', desc: 'Check permissions on sensitive files like .rhosts, .shosts, hosts.equiv' },
				{ id: 'FILE-6364', title: 'World-Writable Files', desc: 'Identify world-writable files that could allow privilege escalation' },
				{ id: 'FILE-6372', title: 'SUID/SGID Files', desc: 'Scan for SUID/SGID executables that could be exploited' },
				{ id: 'FILE-6375', title: 'SGID Directories', desc: 'Check for SGID directories that may expose group permissions' },
				{ id: 'FILE-6376', title: 'Disk Quotas', desc: 'Check if disk quotas are enabled on user partitions' },
				{ id: 'FILE-6380', title: 'Sticky Bit', desc: 'Verify sticky bit is set on world-writable directories like /tmp' },
				{ id: 'FILE-6382', title: 'Expanded ACLs', desc: 'Check for ACL entries on system binary and library paths' },
				{ id: 'FILE-6384', title: 'Unlinked Files', desc: 'Detect unlinked files still held open by processes' },
				{ id: 'FILE-6390', title: 'Loop Devices', desc: 'Check for mounted loop devices that may expose sensitive data' },
				{ id: 'FILE-6395', title: 'NFS Exports', desc: 'Verify NFS export options are restrictive (root_squash, no_all_squash)' },
				{ id: 'FILE-6400', title: 'Samba Auth', desc: 'Check Samba/CIFS configuration for security options' },
			]
		},
		{
			icon: '⚙️', label: 'Kernel',
			color: '#8b5cf6',
			totalTests: 16,
			desc: 'Checks kernel runtime parameters, module loading, sysctl hardening, ASLR, boot security, and SELinux/AppArmor status.',
			tests: [
				{ id: 'KRNL-5780', title: 'System Architecture', desc: 'Verify kernel architecture bitness and PAE/NX support' },
				{ id: 'KRNL-5785', title: 'Kernel Version', desc: 'Check running kernel version against latest security patches' },
				{ id: 'KRNL-5790', title: 'Kernel Modules', desc: 'Audit loaded kernel modules for unnecessary or risky drivers' },
				{ id: 'KRNL-5795', title: 'Module Blacklist', desc: 'Check if risky modules (bluetooth, firewire, usb-storage) are blacklisted' },
				{ id: 'KRNL-5800', title: 'Kexec', desc: 'Verify kexec is disabled or restricted to prevent unauthorized kernel replacement' },
				{ id: 'KRNL-5805', title: 'KASLR', desc: 'Check kernel address space layout randomization is enabled' },
				{ id: 'KRNL-5810', title: 'SysRq Key', desc: 'Verify SysRq key is disabled or restricted in production' },
				{ id: 'KRNL-5815', title: 'Core Dumps', desc: 'Check if core dumps are restricted to prevent data leakage' },
				{ id: 'KRNL-5820', title: 'Kernel Hardening', desc: 'Check kernel hardening parameters (sysctl settings for network security)' },
				{ id: 'KRNL-5824', title: 'ASLR', desc: 'Verify Address Space Layout Randomization is enabled' },
				{ id: 'KRNL-5828', title: 'Module Loading', desc: 'Check if kernel module loading is restricted' },
				{ id: 'KRNL-5830', title: 'Sysctl Security', desc: 'Check sysctl settings related to IP forwarding, source route, martians' },
				{ id: 'KRNL-5832', title: 'SELinux/AppArmor', desc: 'Verify mandatory access control (MAC) system status' },
				{ id: 'KRNL-5836', title: 'Core Dumps', desc: 'Check if core dumps are restricted to prevent data leakage' },
				{ id: 'KRNL-5840', title: 'SysRq Key', desc: 'Verify SysRq key is disabled or restricted in production' },
				{ id: 'KRNL-5844', title: 'Boot Security', desc: 'Check GRUB password and boot loader protection' },
			]
		},
		{
			icon: '🌐', label: 'Networking',
			color: '#10b981',
			totalTests: 20,
			desc: 'Audits network configuration, firewall rules, listening services, DNS settings, TCP wrappers, and network parameter hardening.',
			tests: [
				{ id: 'NETW-8400', title: 'Network Interfaces', desc: 'List all active network interfaces and their configurations' },
				{ id: 'NETW-8405', title: 'IP Forwarding', desc: 'Check if IP forwarding is enabled — should be disabled unless router' },
				{ id: 'NETW-8410', title: 'Default Gateway', desc: 'Verify default gateway is configured and reachable' },
				{ id: 'NETW-8415', title: 'DNS Servers', desc: 'Check DNS resolver configuration (/etc/resolv.conf)' },
				{ id: 'NETW-8420', title: 'DHCP Client', desc: 'Check if DHCP client is running on static-IP servers' },
				{ id: 'NETW-8425', title: 'Listening Ports', desc: 'Enumerate all listening TCP/UDP ports' },
				{ id: 'NETW-8430', title: 'Wireless Interfaces', desc: 'Detect wireless interfaces that should not be present on servers' },
				{ id: 'NETW-8435', title: 'Bluetooth', desc: 'Check if Bluetooth modules and services are active' },
				{ id: 'NETW-8440', title: 'Avahi/DNS-SD', desc: 'Check if Avahi/mDNS is running — should be disabled in production' },
				{ id: 'NETW-8445', title: 'NTP Config', desc: 'Verify NTP time synchronization is configured and running' },
				{ id: 'NETW-8450', title: 'TCP Wrappers', desc: 'Check hosts.allow/hosts.deny configuration if present' },
				{ id: 'NETW-8455', title: 'Hosts File', desc: 'Check /etc/hosts for proper entries and localhost resolution' },
				{ id: 'NETW-8460', title: 'Name Resolution', desc: 'Verify /etc/nsswitch.conf and name resolution order' },
				{ id: 'NETW-8465', title: 'Network Parameters', desc: 'Check sysctl network hardening (IP forwarding, source routing, etc.)' },
				{ id: 'NETW-8470', title: 'Promiscuous Mode', desc: 'Detect network interfaces in promiscuous mode' },
				{ id: 'NETW-8475', title: 'Routing Table', desc: 'Examine routing table for unexpected routes' },
				{ id: 'NETW-8480', title: 'ARP Table', desc: 'Check ARP table for suspicious entries' },
				{ id: 'NETW-8500', title: 'Firewall Rules', desc: 'Check iptables/nftables rules for proper filtering and default deny policy' },
				{ id: 'NETW-8504', title: 'Listening Services', desc: 'Identify all listening ports and services to minimize attack surface' },
				{ id: 'NETW-8508', title: 'DNS Configuration', desc: 'Verify DNS resolver configuration (/etc/resolv.conf) for security' },
			]
		},
		{
			icon: '📋', label: 'Logging & Auditing',
			color: '#c084fc',
			totalTests: 14,
			desc: 'Verifies audit daemon configuration, syslog setup, log rotation, remote logging, and audit rule coverage.',
			tests: [
				{ id: 'LOGG-2000', title: 'Syslog Daemon', desc: 'Verify syslog daemon (rsyslog/syslog-ng) is installed and active' },
				{ id: 'LOGG-2010', title: 'Syslog Remote', desc: 'Check if syslog messages are forwarded to a remote collector' },
				{ id: 'LOGG-2020', title: 'Log Hostname', desc: 'Verify syslog includes hostname and timestamps in log entries' },
				{ id: 'LOGG-2030', title: 'Audit Daemon', desc: 'Check auditd service status and basic audit rules' },
				{ id: 'LOGG-2040', title: 'Audit Log Size', desc: 'Verify audit log max size and retention policies' },
				{ id: 'LOGG-2050', title: 'Audit Backlog', desc: 'Check audit backlog limit to prevent audit loss during bursts' },
				{ id: 'LOGG-2060', title: 'Audit Scope', desc: 'Check that audit rules cover critical system calls (execve, open, mount)' },
				{ id: 'LOGG-2100', title: 'Audit Daemon', desc: 'Check auditd service status and basic audit rules' },
				{ id: 'LOGG-2130', title: 'Syslog Configuration', desc: 'Verify rsyslog or syslog-ng is running and properly configured' },
				{ id: 'LOGG-2150', title: 'Log Rotation', desc: 'Check logrotate configuration for proper rotation schedule' },
				{ id: 'LOGG-2170', title: 'Remote Logging', desc: 'Verify logs are forwarded to a remote log server/syslog collector' },
				{ id: 'LOGG-2180', title: 'Audit Log Permissions', desc: 'Ensure audit logs and directories have restricted permissions' },
				{ id: 'LOGG-2190', title: 'Audit Rule Coverage', desc: 'Check that audit rules cover critical system calls and file access' },
				{ id: 'LOGG-2200', title: 'Logwatch/Report', desc: 'Check if logwatch or log reporting is configured for daily summaries' },
			]
		},
		{
			icon: '📦', label: 'Docker',
			color: '#06b6d4',
			totalTests: 12,
			desc: 'Checks Docker daemon security configuration, container settings, image management, and runtime security defaults.',
			tests: [
				{ id: 'DOCK-9000', title: 'Docker Version', desc: 'Check Docker CE/EE version against latest security releases' },
				{ id: 'DOCK-9010', title: 'Docker Service', desc: 'Verify Docker daemon service file permissions and ownership' },
				{ id: 'DOCK-9020', title: 'Docker Config', desc: 'Check daemon.json for security options (userns-remap, live-restore, no-new-privileges)' },
				{ id: 'DOCK-9030', title: 'Storage Driver', desc: 'Verify Docker storage driver is overlay2 or similar' },
				{ id: 'DOCK-9310', title: 'Docker Daemon', desc: 'Verify Docker daemon runs with security options (no privileged access)' },
				{ id: 'DOCK-9320', title: 'Running Containers', desc: 'List running containers and check for privileged/exposed ports' },
				{ id: 'DOCK-9330', title: 'Container Users', desc: 'Check containers run as non-root users where possible' },
				{ id: 'DOCK-9340', title: 'Container Capabilities', desc: 'Audit Linux capabilities granted to running containers' },
				{ id: 'DOCK-9350', title: 'Container Volumes', desc: 'Check bind-mounted host paths that could escape container' },
				{ id: 'DOCK-9360', title: 'Image Security', desc: 'Check if containers use signed/trusted images' },
				{ id: 'DOCK-9370', title: 'Container Privileges', desc: 'Check for privilege escalation risks in container configurations' },
				{ id: 'DOCK-9380', title: 'Docker Socket', desc: 'Verify Docker socket ownership and permissions' },
			]
		},
	];

	let totalPages = $derived(Math.max(1, Math.ceil(historyTotal / historyLimit)));

	onMount(() => {
		loadThresholds();
		loadHistory();
		loadServers();
	});

	async function loadServers() {
		try {
			const data = await api.servers.list();
			const list = data.servers || data || [];
			servers = list.filter(s => s.status === 'online');
			if (servers.length > 0 && !selectedServerId) selectedServerId = servers[0].id;
		} catch { /* ignore */ }
	}

	async function runLynisScan() {
		if (!selectedServerId) return;
		scanning = true;
		scanMessage = 'Scan triggered...';
		try {
			await api.compliance.scanLynis(selectedServerId);
			scanMessage = 'Scan started! Check history below.';
			setTimeout(() => { scanMessage = ''; loadHistory(); }, 3000);
		} catch (e) {
			scanMessage = 'Failed: ' + (e.message || 'unknown');
		} finally {
			scanning = false;
		}
	}

	async function loadHistory(pg) {
		if (pg !== undefined) historyPage = pg;
		historyLoading = true;
		try {
			const resp = await api.compliance.globalHistory({ scan_type: 'Lynis', page: historyPage, limit: historyLimit });
			history = resp.results || resp.history || [];
			if (Array.isArray(resp)) history = resp;
			historyTotal = resp.total || resp.count || 0;
		} catch {
			history = [];
			historyTotal = 0;
		} finally {
			historyLoading = false;
		}
	}

	function formatTime(ts) {
		if (!ts) return '—';
		const d = new Date(ts);
		const now = new Date();
		const diff = now - d;
		if (diff < 60000) return 'Just now';
		if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
		if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
		return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
	}

	const statusLabel = { completed: '✅ Done', running: '🔄 Running', failed: '❌ Failed', pending: '⏳ Pending' };
</script>

<div class="page-container">

	<!-- Breadcrumb -->
	<div class="flex items-center gap-2 text-sm mb-4" style="color: var(--color-text-muted);">
		<a href="/compliance" class="hover:underline" style="color: var(--color-text-muted);">Compliance</a>
		<span>/</span>
		<span style="color: var(--color-text);">Lynis Audit</span>
	</div>

	<!-- Hero -->
	<div class="card !p-5 mb-5" style="border-left: 4px solid {profileColor};">
		<div class="flex items-start gap-4 flex-wrap">
			<div class="w-12 h-12 rounded-xl flex items-center justify-center shrink-0" style="background: rgba(139,92,246,0.12);">
				<Icon icon="solar:bug-bold" class="h-6 w-6" style="color: {profileColor};" />
			</div>
			<div class="flex-1 min-w-0">
				<h1 class="text-lg font-bold" style="color: var(--color-text);">Lynis Audit</h1>
				<p class="text-sm mt-0.5" style="color: var(--color-text-secondary);">
					Lynis is a security auditing tool that performs <strong>256+ tests</strong> across <strong>12 categories</strong>. 
					It checks compliance, configuration, and system hardening — identifying misconfigurations and vulnerabilities.
				</p>
				<div class="flex flex-wrap items-center gap-3 mt-2">
					<span class="text-xs px-2.5 py-1 rounded-full font-medium" style="background: rgba(139,92,246,0.12); color: {profileColor};">
						{categories.reduce((s, c) => s + c.totalTests, 0)} tests
					</span>
					<span class="text-xs px-2.5 py-1 rounded-full font-medium" style="background: rgba(59,130,246,0.12); color: #3b82f6;">
						{categories.length} core categories
					</span>
					<span class="text-xs" style="color: var(--color-text-muted);">Reference: Lynis by CISOfy</span>
				</div>
				<!-- Scan controls -->
				<div class="flex items-center gap-2 mt-3">
					{#if servers.length > 0}
						<select bind:value={selectedServerId} class="text-xs rounded-lg px-2.5 py-1.5" style="background: var(--color-surface); border: 1px solid var(--color-border-light); color: var(--color-text);">
							{#each servers as srv}
								<option value={srv.id}>{srv.name}</option>
							{/each}
						</select>
					{/if}
					<button onclick={runLynisScan} disabled={scanning || !selectedServerId} class="btn-primary flex items-center gap-1.5 text-xs py-1.5">
						<Icon icon={scanning ? 'solar:spinner-bold' : 'solar:play-bold'} class="h-3.5 w-3.5 {scanning ? 'animate-spin' : ''}" />
						{scanning ? 'Scanning...' : 'Run Lynis Scan'}
					</button>
					{#if scanMessage}
						<span class="text-xs" style="color: {scanMessage.startsWith('Failed') ? 'var(--color-danger)' : 'var(--color-success)'};">{scanMessage}</span>
					{/if}
				</div>
			</div>
			<button onclick={() => goto('/compliance')} class="btn-secondary flex items-center gap-1.5 shrink-0 text-xs">
				<Icon icon="solar:arrow-left-bold" class="h-3.5 w-3.5" /> Back
			</button>
		</div>
	</div>

	<!-- What is Lynis -->
	<div class="card mb-5">
		<h3 class="text-sm font-semibold mb-2" style="color: var(--color-text);">What is Lynis?</h3>
		<p class="text-xs leading-relaxed" style="color: var(--color-text-secondary); line-height: 1.6;">
			Lynis is an open-source security auditing tool for Linux/Unix systems.
			It scans the system to identify security issues, missing patches, and configuration weaknesses.
			Unlike CIS checks which follow a specific benchmark standard, Lynis is a <strong>broad security scanner</strong>
			that checks everything from file permissions and firewall rules to software versions and kernel settings.
			Results include a <strong>hardening index</strong> score, categorized warnings, and actionable suggestions.
		</p>
	</div>

	<!-- Categories -->
	<div class="space-y-4">
		{#each categories as cat}
			<details class="cat-group" style="background: {cat.color}15; border: 1px solid {cat.color}30; border-radius: 10px; overflow: hidden;">
				<summary class="cat-summary" style="padding: 12px 16px; cursor: pointer;">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-2.5">
							<span class="text-lg">{cat.icon}</span>
							<div>
								<span class="text-sm font-semibold" style="color: var(--color-text);">{cat.label}</span>
								<span class="text-xs ml-1.5" style="color: var(--color-text-muted);">{cat.totalTests} tests</span>
							</div>
						</div>
						<div class="flex items-center gap-2">
							<span class="text-xs" style="color: var(--color-text-muted);">
								Test IDs: <span class="font-mono font-medium" style="color: {cat.color};">{cat.testPrefix}*</span>
							</span>
							<Icon icon="solar:alt-arrow-down-bold" class="h-4 w-4 chevron" style="color: var(--color-text-muted); transition: transform 0.2s;" />
						</div>
					</div>
				</summary>
				<div style="padding: 0 16px 14px; border-top: 1px solid {cat.color}30;">
					<p class="mt-3 text-xs mb-3" style="color: var(--color-text-secondary); line-height: 1.5;">{cat.desc}</p>
					<div class="space-y-2">
						{#each cat.tests as test}
							<div class="test-row" style="background: var(--color-card); border: 1px solid var(--color-border-light); border-left: 3px solid {cat.color}; border-radius: 8px; padding: 10px 12px;">
								<div class="flex items-start justify-between gap-3">
									<div class="flex-1 min-w-0">
										<div class="flex items-center gap-2 flex-wrap">
											<span class="text-sm font-medium" style="color: var(--color-text);">{test.title}</span>
											<span class="text-[10px] font-mono px-1.5 py-0.5 rounded" style="background: {cat.color}15; color: {cat.color};">
												{test.id}
											</span>
										</div>
										<p class="text-xs mt-1.5" style="color: var(--color-text-muted); line-height: 1.4;">{test.desc}</p>
									</div>
								</div>
							</div>
						{/each}
					</div>
				</div>
			</details>
		{/each}
	</div>

	<!-- Scan History -->
	<div class="mt-8">
		<div class="flex items-center gap-2 mb-3">
			<h3 class="text-sm font-semibold" style="color: var(--color-text);">📜 Scan History</h3>
			{#if historyTotal > 0}
				<span class="text-xs" style="color: var(--color-text-muted);">{historyTotal} scan{historyTotal !== 1 ? 's' : ''}</span>
			{/if}
		</div>

		<div class="card !p-0 overflow-hidden">
			{#if historyLoading}
				<div class="flex items-center justify-center py-6">
					<Icon icon="solar:spinner-bold" class="h-5 w-5 animate-spin" style="color: var(--color-text-muted);" />
				</div>
			{:else if history.length > 0}
				<!-- Header row -->
				<div class="grid gap-2 px-4 py-2 text-[11px] font-semibold uppercase tracking-wider"
					style="color: var(--color-text-muted); background: var(--color-surface); border-bottom: 1px solid var(--color-border); grid-template-columns: 1fr 48px 60px 70px 80px 60px;">
					<span>Server</span>
					<span class="text-center">Score</span>
					<span class="text-center">Findings</span>
					<span class="text-center">Status</span>
					<span class="text-right">When</span>
					<span class="text-center">Action</span>
				</div>

				{#each history as item, i}
					{@const sColor = item.score !== null && item.score !== undefined ? scoreColor(item.score) : 'var(--color-text-muted)'}
					{@const bgColor = i % 2 === 0 ? 'transparent' : 'rgba(255,255,255,0.015)'}
					<div class="grid gap-2 px-4 py-2.5 text-xs"
						style="background: {bgColor}; border-bottom: {i < history.length - 1 ? '1px solid var(--color-border)' : 'none'}; grid-template-columns: 1fr 48px 60px 70px 80px 60px; align-items: center;">
						<div class="min-w-0">
							<p class="text-sm font-medium truncate" style="color: var(--color-text);" title="{item.name || item.server_name || 'Unknown'}">{item.name || item.server_name || 'Unknown'}</p>
						</div>
						<div class="text-center">
							<span class="text-sm font-bold" style="color: {sColor};">{item.score ?? '—'}</span>
						</div>
						<div class="flex items-center gap-1 justify-center">
							{#if item.criticals > 0}
								<span style="color: var(--color-danger); font-weight: 600;">{item.criticals}C</span>
							{/if}
							{#if item.warnings > 0}
								<span style="color: var(--color-warning); font-weight: 600;">{item.warnings}W</span>
							{/if}
							{#if !item.criticals && !item.warnings}
								<span style="color: var(--color-text-muted);">—</span>
							{/if}
						</div>
						<div class="text-center text-[11px]" style="color: var(--color-text-muted);">
							{statusLabel[item.status] || item.status || '—'}
						</div>
						<div class="text-right text-[11px]" style="color: var(--color-text-muted);">
							{formatTime(item.completed_at || item.created_at)}
						</div>
						<div class="text-center">
							<button onclick={(e) => { e.stopPropagation(); item.server_id && goto(`/servers/${item.server_id}?scan=${item.id}&tab=compliance`); }}
								class="text-xs font-medium px-2 py-1 rounded"
								style="color: var(--color-primary); background: rgba(16,185,129,0.1); border: none; cursor: pointer;">View</button>
						</div>
					</div>
				{/each}

				<!-- Pagination -->
				{#if totalPages > 1}
					<div class="flex items-center justify-center gap-2 px-4 py-3" style="border-top: 1px solid var(--color-border);">
						<button disabled={historyPage <= 1} onclick={() => loadHistory(historyPage - 1)}
							class="btn-secondary text-xs py-1 px-2" style="opacity: {historyPage <= 1 ? 0.4 : 1};">← Prev</button>
						<span class="text-xs" style="color: var(--color-text-muted);">{historyPage} / {totalPages}</span>
						<button disabled={historyPage >= totalPages} onclick={() => loadHistory(historyPage + 1)}
							class="btn-secondary text-xs py-1 px-2" style="opacity: {historyPage >= totalPages ? 0.4 : 1};">Next →</button>
					</div>
				{/if}
			{:else}
				<div class="flex flex-col items-center py-10 text-center">
					<Icon icon="solar:clipboard-remove-bold" class="mb-2 h-8 w-8" style="color: var(--color-text-muted);" />
					<p class="text-sm" style="color: var(--color-text-secondary);">No scan history yet for Lynis</p>
					<p class="text-xs mt-1" style="color: var(--color-text-muted);">Run a Lynis scan from the Compliance dashboard to see results here.</p>
				</div>
			{/if}
		</div>
	</div>

	<!-- Footer note -->
	<div class="mt-6 text-center">
		<p class="text-xs" style="color: var(--color-text-muted);">
			Based on <strong>Lynis</strong> by CISOfy. Version-specific tests may vary.
			Run Lynis on a server for the actual findings and hardening index.
		</p>
	</div>
</div>

<style>
	.cat-summary { user-select: none; display: flex; align-items: center; }
	.cat-summary::-webkit-details-marker { display: none; }
	.cat-summary::marker { display: none; }
	details[open] .chevron { transform: rotate(180deg); }
	.test-row { transition: all 0.15s; }
	.test-row:hover { border-color: var(--color-primary); box-shadow: 0 1px 4px rgba(139,92,246,0.06); }
</style>
