<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.svelte.js';
  import Icon from '@iconify/svelte';
  import MetricsChart from '$lib/components/charts/MetricsChart.svelte';

  let { monitorId } = $props();
  let entries = $state([]);
  let loading = $state(true);
  let error = $state('');

  onMount(async () => {
    try {
      const result = await api.sslMonitors.trend(monitorId, { limit: 90 });
      entries = result?.entries || [];
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  // Transform into uPlot format
  let chartTimestamps = $derived.by(() => {
    if (entries.length < 2) return [];
    return entries.map(e => new Date(e.checked_at).getTime() / 1000);
  });

  let chartDays = $derived.by(() => {
    if (entries.length < 2) return [];
    return entries.map(e => e.days_remaining);
  });

  // Dynamic Y range based on data
  let yRange = $derived.by(() => {
    if (entries.length < 2) return null;
    const vals = entries.map(e => e.days_remaining);
    const maxVal = Math.max(...vals, 30);
    const minVal = Math.min(...vals, 0);
    const padding = Math.max((maxVal - minVal) * 0.1, 5);
    return { min: Math.max(0, minVal - padding), max: maxVal + padding };
  });

  let chartSeries = $derived.by(() => [
    {
      label: 'Days Remaining',
      data: chartDays,
      color: '#10b981',
      width: 2,
      fill: 'rgba(16,185,129,0.08)',
      spanGaps: false,
      scale: 'days',
    },
  ]);

  function formatDays(v) {
    if (v == null) return '—';
    return `${Math.round(v)} days`;
  }
</script>

<div class="card mt-4">
  <h3 class="text-sm font-semibold mb-3 flex items-center gap-2">
    <Icon icon="solar:chart-bold" class="h-4 w-4" style="color: var(--color-primary);" />
    Certificate Expiry Trend
  </h3>

  {#if loading}
    <div class="flex justify-center py-8">
      <Icon icon="solar:spinner-bold" class="h-6 w-6 animate-spin" style="color: var(--color-primary);" />
    </div>
  {:else if error}
    <p class="text-xs" style="color: var(--color-danger);">{error}</p>
  {:else if entries.length < 2}
    <p class="text-xs py-8 text-center" style="color: var(--color-text-muted);">
      <Icon icon="solar:info-circle-bold" class="inline h-4 w-4 mr-1" />
      Not enough history data yet. Run a few checks to see the trend.
    </p>
  {:else}
    <MetricsChart
      timestamps={chartTimestamps}
      series={chartSeries}
      height={200}
      yLabel="Days"
      formatY={formatDays}
      yMin={yRange?.min}
      yMax={yRange?.max}
    />
  {/if}
</div>
