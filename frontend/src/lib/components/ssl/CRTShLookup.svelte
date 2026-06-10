<script>
  import Icon from '@iconify/svelte';

  let { monitorId, domain } = $props();
  let certs = $state([]);
  let loading = $state(false);
  let error = $state('');
  let lookupAt = $state('');

  async function lookup() {
    loading = true;
    error = '';
    certs = [];
    try {
      const res = await fetch(`/api/v1/ssl-monitors/${monitorId}/crt-lookup`, { method: 'POST' });
      const data = await res.json();
      certs = data?.certificates || [];
      lookupAt = data?.lookup_at || '';
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function formatDate(dateStr) {
    if (!dateStr) return '—';
    return new Date(dateStr).toLocaleDateString('en-GB', { year: 'numeric', month: 'short', day: 'numeric' });
  }

  function certStatus(notBefore, notAfter) {
    const now = new Date();
    const after = new Date(notAfter);
    if (after < now) return { label: 'Expired', color: 'var(--color-danger)' };
    const before = new Date(notBefore);
    if (before > now) return { label: 'Future', color: 'var(--color-warning)' };
    return { label: 'Valid', color: 'var(--color-success)' };
  }

  // Group by common name
  let grouped = $derived.by(() => {
    const map = {};
    for (const c of certs) {
      if (!map[c.common_name]) map[c.common_name] = [];
      map[c.common_name].push(c);
    }
    for (const cn in map) {
      map[cn].sort((a, b) => new Date(b.not_after) - new Date(a.not_after));
    }
    return map;
  });
</script>

<div class="mt-4">
  <div class="card">
    <div class="flex items-center justify-between mb-3">
      <h3 class="text-sm font-semibold flex items-center gap-2">
        <Icon icon="solar:documents-bold" class="h-4 w-4" style="color: var(--color-primary);" />
        Certificate Transparency (CRT.sh)
      </h3>
      <button onclick={lookup} disabled={loading}
        class="px-3 py-1.5 rounded-lg text-xs font-medium transition-all"
        style="background-color: var(--color-primary); color: #fff; border: none;"
      >
        {#if loading}
          <Icon icon="solar:spinner-bold" class="inline h-3.5 w-3.5 animate-spin" />
        {/if}
        {loading ? 'Fetching...' : 'Lookup'}
      </button>
    </div>

    {#if error}
      <p class="text-xs" style="color: var(--color-danger);">{error}</p>
    {/if}

    {#if lookupAt}
      <p class="text-xs mb-2" style="color: var(--color-text-muted);">
        Last lookup: {formatDate(lookupAt)}
      </p>
    {/if}

    {#if Object.keys(grouped).length > 0}
      <div class="space-y-2 max-h-80 overflow-y-auto">
        {#each Object.entries(grouped) as [cn, entries]}
          {@const status = certStatus(entries[0].not_before, entries[0].not_after)}
          <div class="rounded-lg p-3" style="background-color: var(--color-card-alt); border: 1px solid var(--color-border);">
            <div class="flex items-center justify-between mb-1">
              <code class="text-xs font-medium">{cn}</code>
              <span class="text-[10px] font-medium px-1.5 py-0.5 rounded"
                style="background-color: {status.color}15; color: {status.color};">{status.label}</span>
            </div>
            <p class="text-[11px]" style="color: var(--color-text-muted);">
              Valid: {formatDate(entries[0].not_before)} → {formatDate(entries[0].not_after)}
            </p>
            <p class="text-[11px]" style="color: var(--color-text-muted);">
              Issuer: {entries[0].issuer_name || '—'}
            </p>
            {#if entries.length > 1}
              <details class="mt-1">
                <summary class="text-[10px] cursor-pointer" style="color: var(--color-primary);">
                  {entries.length - 1} more entr{entries.length - 1 === 1 ? 'y' : 'ies'}
                </summary>
                <div class="mt-1 space-y-1">
                  {#each entries.slice(1) as e}
                    <div class="text-[10px] p-1 rounded" style="background-color: var(--color-bg);">
                      <code>{e.serial_number?.slice(0, 16) || ''}…</code> — {formatDate(e.not_before)} → {formatDate(e.not_after)}
                    </div>
                  {/each}
                </div>
              </details>
            {/if}
          </div>
        {/each}
      </div>
    {:else if !loading && !error}
      <p class="text-xs py-4 text-center" style="color: var(--color-text-muted);">
        Click "Lookup" to fetch certificate transparency logs for <strong>{domain}</strong>.
      </p>
    {/if}
  </div>
</div>
