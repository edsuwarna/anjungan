<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.svelte.js';
  import Icon from '@iconify/svelte';

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

  // SVG chart dimensions
  const W = 600, H = 200, PAD = { top: 20, right: 20, bottom: 30, left: 40 };
  const chartW = W - PAD.left - PAD.right;
  const chartH = H - PAD.top - PAD.bottom;

  let points = $derived.by(() => {
    if (entries.length < 2) return [];
    const maxDays = Math.max(...entries.map(e => e.days_remaining), 30);
    const minDays = Math.min(...entries.map(e => e.days_remaining), 0);
    const paddedMax = Math.max(maxDays, minDays + 10); // at least 10d range so flat lines aren't invisible
    const range = paddedMax - minDays || 10;
    return entries.map((e, i) => ({
      x: PAD.left + (i / (entries.length - 1)) * chartW,
      y: PAD.top + chartH - ((e.days_remaining - minDays) / range) * chartH,
      displayDays: e.days_remaining,
      days: e.days_remaining,
      date: new Date(e.checked_at).toLocaleDateString('en-GB'),
      status: e.status,
    }));
  });

  let pathD = $derived(
    points.length > 1
      ? points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ')
      : ''
  );

  let gradientId = $derived(`trend-${Math.random().toString(36).slice(2, 8)}`);

  // Y-axis ticks
  let yTicks = $derived.by(() => {
    if (entries.length < 2) return [];
    const maxDays = Math.max(...entries.map(e => e.days_remaining), 30);
    const minDays = Math.min(...entries.map(e => e.days_remaining), 0);
    const max = Math.max(maxDays, minDays + 10);
    const step = max > 60 ? 30 : max > 30 ? 15 : 7;
    const ticks = [];
    for (let v = 0; v <= max; v += step) {
      const y = PAD.top + chartH - (v / max) * chartH;
      ticks.push({ value: v, y });
    }
    return ticks;
  });
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
    <div class="overflow-x-auto">
      <svg width={W} height={H} viewBox={`0 0 ${W} ${H}`} class="w-full max-w-full" style="min-width: 500px;">
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="var(--color-primary)" stop-opacity="0.15" />
            <stop offset="100%" stop-color="var(--color-primary)" stop-opacity="0.01" />
          </linearGradient>
        </defs>

        <!-- Grid lines -->
        {#each yTicks as tick}
          <line x1={PAD.left} y1={tick.y} x2={W - PAD.right} y2={tick.y}
            stroke="var(--color-border)" stroke-width="1" stroke-dasharray="4,4" />
          <text x={PAD.left - 6} y={tick.y + 4} text-anchor="end" font-size="11"
            fill="var(--color-text-muted)">{tick.value}d</text>
        {/each}

        <!-- Area fill -->
        {#if points.length > 1}
          <path d={`${pathD} L ${points[points.length-1].x} ${H - PAD.bottom} L ${points[0].x} ${H - PAD.bottom} Z`}
            fill={`url(#${gradientId})`} />
        {/if}

        <!-- Line -->
        <path d={pathD} fill="none" stroke="var(--color-primary)" stroke-width="2"
          stroke-linejoin="round" stroke-linecap="round" />

        <!-- Points -->
        {#each points as p}
          <circle cx={p.x} cy={p.y} r="3" fill="var(--color-primary)" stroke="#fff" stroke-width="1.5">
            <title>{p.date}: {p.days} days remaining ({p.status})</title>
          </circle>
        {/each}
      </svg>
    </div>
  {/if}
</div>
