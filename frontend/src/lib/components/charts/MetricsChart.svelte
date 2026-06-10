<script>
	import { onMount, onDestroy, tick } from 'svelte';
	import uPlot from 'uplot';
	import 'uplot/dist/uPlot.min.css';

	/**
	 * Props:
	 * - title: chart title
	 * - timestamps: number[] (unix seconds)
	 * - series: array of { label, data, color, scale?, width?, fill? }
	 * - height: number (default 200)
	 * - yLabel: string
	 * - formatY: (v) => string
	 * - formatTooltip: (val, seriesIdx) => string
	 */
	let {
		title = '',
		timestamps = [],
		series = [],
		height = 200,
		yLabel = '',
		formatY,
		yMin,
		yMax,
	} = $props();

	let chartContainer;
	let chartInstance = null;
	let darkMode = $state(false);

	$effect(() => {
		// Check theme
		const isDark = document.documentElement.classList.contains('dark');
		if (isDark !== darkMode) darkMode = isDark;
	});

	function getColors() {
		const isDark = document.documentElement.classList.contains('dark');
		const s = getComputedStyle(document.documentElement);
		return {
			grid: isDark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.08)',
			tick: isDark ? 'rgba(255,255,255,0.4)' : 'rgba(0,0,0,0.4)',
			font: isDark ? 'rgba(255,255,255,0.7)' : 'rgba(0,0,0,0.7)',
			mouse: isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.06)',
			bg: 'transparent',
		};
	}

	function buildChart() {
		if (!chartContainer || timestamps.length === 0) return;

		destroyChart();

		const colors = getColors();
		const isDark = document.documentElement.classList.contains('dark');

		// Prepare uPlot data: [timestamps, series1, series2, ...]
		const uData = [timestamps];
		for (const s of series) {
			uData.push(s.data || []);
		}

		// Build uPlot series options
		const uSeries = series.map((s, i) => ({
			label: s.label || `Series ${i + 1}`,
			stroke: s.color || '#10b981',
			width: s.width || 2,
			fill: s.fill || false,
			spanGaps: s.spanGaps ?? false,
			points: s.points ?? { show: false },
			scale: s.scale || '%',
			value: (self, rawValue) => {
				if (formatY) return formatY(rawValue);
				return rawValue?.toFixed(1) ?? '—';
			},
		}));

		// Build scales
		const scales = {
			x: { time: true, dir: 1 },
		};
		const scaleKeys = [...new Set(series.map(s => s.scale || '%'))];
		for (const sk of scaleKeys) {
			scales[sk] = {};
			if (sk !== 'x') {
				if (yMin != null) scales[sk].min = yMin;
				if (yMax != null) scales[sk].max = yMax;
			}
		}

		const opts = {
			width: chartContainer.clientWidth,
			height,
			cursor: {
				show: true,
				drag: { x: false, y: false },
				focus: { prox: 30 },
			},
			select: { show: false, left: 0, top: 0, width: 0, height: 0 },
			legend: { show: true, live: true },
			padding: [8, 8, 4, 4],
			scales,
			series: [
				{
					label: 'Time',
					value: (self, rawValue) => {
						if (!rawValue) return '—';
						const d = new Date(rawValue * 1000);
						return d.toLocaleTimeString();
					},
				},
				...uSeries,
			],
			axes: [
				{
					stroke: colors.tick,
					grid: { stroke: colors.grid, width: 1 },
					ticks: { stroke: colors.tick, width: 1 },
					font: `10px system-ui, sans-serif`,
					color: colors.font,
					values: (self, ticks) => ticks.map(v => {
						const d = new Date(v * 1000);
						return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
					}),
				},
				{
					side: 1,
					stroke: colors.tick,
					grid: { stroke: colors.grid, width: 1 },
					ticks: { stroke: colors.tick, width: 1 },
					font: `10px system-ui, sans-serif`,
					color: colors.font,
					label: yLabel || '',
					size: 48,
					values: (self, ticks) => ticks.map(v => {
						if (formatY) return formatY(v);
						return v?.toFixed(1) ?? '';
					}),
				},
			],
		};

		// Add right axis if there's a 2nd scale
		if (scaleKeys.length > 1) {
			opts.axes.push({
				side: 3,
				stroke: colors.tick,
				grid: { show: false },
				ticks: { stroke: colors.tick, width: 1 },
				font: `10px system-ui, sans-serif`,
				color: colors.font,
				label: yLabel || '',
				size: 48,
			});
		}

		chartInstance = new uPlot(opts, uData, chartContainer);
	}

	function destroyChart() {
		if (chartInstance) {
			chartInstance.destroy();
			chartInstance = null;
		}
	}

	// Watch for data changes
	$effect(() => {
		// Force reactivity
		const _ts = timestamps;
		const _series = series;
		const _height = height;
		if (chartContainer && _ts.length > 0) {
			tick().then(() => buildChart());
		}
	});

	// Resize handler
	function handleResize() {
		if (chartInstance && chartContainer) {
			const w = chartContainer.clientWidth;
			if (w > 0) {
				chartInstance.setSize({ width: w, height });
			}
		}
	}

	onMount(() => {
		window.addEventListener('resize', handleResize);

		// Observe theme changes
		const observer = new MutationObserver(() => {
			buildChart();
		});
		observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });

		return () => {
			window.removeEventListener('resize', handleResize);
			observer.disconnect();
			destroyChart();
		};
	});

	onDestroy(() => {
		destroyChart();
	});
</script>

<div class="chart-wrapper">
	{#if title}
		<p class="chart-title">{title}</p>
	{/if}
	<div bind:this={chartContainer} class="chart-container"></div>
	{#if timestamps.length === 0}
		<div class="chart-empty">
			<p>No data yet — collector needs time to gather metrics</p>
		</div>
	{/if}
</div>

<style>
	.chart-wrapper {
		position: relative;
	}
	.chart-title {
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: 0.25rem;
		color: var(--color-text-muted);
	}
	.chart-container {
		min-height: 200px;
	}
	.chart-container :global(.uplot) {
		width: 100% !important;
	}
	.chart-container :global(.u-title) {
		display: none;
	}
	.chart-empty {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		pointer-events: none;
	}
	.chart-empty p {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		opacity: 0.6;
	}
</style>
