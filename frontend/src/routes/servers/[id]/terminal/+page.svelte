<script>
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { Terminal } from 'xterm';
	import { FitAddon } from 'xterm-addon-fit';
	import 'xterm/css/xterm.css';

	let container = $state(null);
	let terminalRef = $state(null);
	let connected = $state(false);
	let error = $state('');

	onMount(() => {
		initTerminal();
		return () => {
			// Cleanup on page leave
		};
	});

	function initTerminal() {
		const term = new Terminal({
			cursorBlink: true,
			cursorStyle: 'block',
			fontSize: 14,
			fontFamily: "'JetBrains Mono', 'Cascadia Code', 'Fira Code', 'Consolas', monospace",
			theme: {
				background: '#1a1d23',
				foreground: '#d4d4d4',
				cursor: '#10b981',
				cursorAccent: '#1a1d23',
				selectionBackground: '#10b98140',
				black: '#1a1d23',
				red: '#ef4444',
				green: '#10b981',
				yellow: '#f59e0b',
				blue: '#3b82f6',
				magenta: '#a855f7',
				cyan: '#06b6d4',
				white: '#d4d4d4',
				brightBlack: '#4a4d55',
				brightRed: '#ef4444',
				brightGreen: '#34d399',
				brightYellow: '#fbbf24',
				brightBlue: '#60a5fa',
				brightMagenta: '#c084fc',
				brightCyan: '#22d3ee',
				brightWhite: '#f4f4f5'
			},
			allowTransparency: true,
			cols: 120,
			rows: 40
		});

		const fitAddon = new FitAddon();
		term.loadAddon(fitAddon);

		// Open terminal in DOM
		term.open(terminalRef);
		fitAddon.fit();

		// Also fit on window resize
		const resizeObserver = new ResizeObserver(() => {
			try { fitAddon.fit(); } catch(e) {}
		});
		resizeObserver.observe(terminalRef);

		// Connect WebSocket
		const serverId = $page.params.id;
		const token = localStorage.getItem('access_token');
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const wsUrl = `${protocol}//${window.location.host}/api/v1/servers/${serverId}/terminal?token=${token}`;

		let ws;
		let reconnectTimer;
		let isClosing = false;

		function connect() {
			ws = new WebSocket(wsUrl);

			ws.onopen = () => {
				connected = true;
				error = '';
				term.focus();

				// Send initial resize
				const dims = fitAddon.proposeDimensions();
				if (dims) {
					ws.send(JSON.stringify({ resize: true, cols: dims.cols, rows: dims.rows }));
				}
			};

			ws.onmessage = (evt) => {
				term.write(evt.data);
			};

			ws.onclose = () => {
				connected = false;
				if (!isClosing) {
					term.writeln('\r\n\x1b[33m[Disconnected. Reconnecting in 3s...]\x1b[0m');
					reconnectTimer = setTimeout(connect, 3000);
				}
			};

			ws.onerror = () => {
				error = 'Connection error';
			};
		}

		connect();

		// Send user input to WebSocket
		term.onData((data) => {
			if (ws && ws.readyState === WebSocket.OPEN) {
				ws.send(data);
			}
		});

		// Send resize events
		term.onResize(({ cols, rows }) => {
			if (ws && ws.readyState === WebSocket.OPEN) {
				ws.send(JSON.stringify({ resize: true, cols, rows }));
			}
			fitAddon.fit();
		});

		// Store cleanup function
		container = {
			destroy: () => {
				isClosing = true;
				clearTimeout(reconnectTimer);
				if (ws) ws.close();
				resizeObserver.disconnect();
				term.dispose();
			}
		};
	}
</script>

<svelte:head>
	<title>SSH Terminal — Anjungan</title>
</svelte:head>

<div class="page-container" style="padding: 0; height: calc(100vh - 4rem); display: flex; flex-direction: column;">
	<!-- Toolbar -->
	<div
		class="flex items-center gap-3 border-b px-4 py-2"
		style="background-color: var(--color-surface); border-color: var(--color-border);"
	>
		<button
			onclick={() => goto(`/servers/${$page.params.id}`)}
			class="btn-icon"
			title="Back to Server"
		>
			<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<path d="M19 12H5m0 0l6-6m-6 6l6 6" />
			</svg>
		</button>
		<span class="text-sm font-medium" style="color: var(--color-text);">SSH Terminal</span>
		<div class="flex-1"></div>
		{#if connected}
			<span class="flex items-center gap-1.5 text-xs" style="color: var(--color-success);">
				<span class="h-2 w-2 rounded-full bg-current"></span>
				Connected
			</span>
		{:else}
			<span class="flex items-center gap-1.5 text-xs" style="color: var(--color-text-muted);">
				<span class="h-2 w-2 rounded-full bg-current"></span>
				Disconnected
			</span>
		{/if}
	</div>

	<!-- Terminal -->
	<div
		bind:this={terminalRef}
		class="flex-1 overflow-hidden"
		style="background-color: #1a1d23;"
	></div>
</div>

<style>
	:global(.xterm) {
		height: 100%;
		padding: 8px;
	}
	:global(.xterm-viewport) {
		scrollbar-width: thin;
		scrollbar-color: #4a4d55 transparent;
	}
</style>
