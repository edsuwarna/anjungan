<script>
  import { onMount } from 'svelte';
  import Icon from '@iconify/svelte';
  import { api } from '$lib/api.svelte.js';

  let { show = false, onClose } = $props();
  let servers = $state([]);
  let selectedServer = $state('');
  let selectedProvider = $state('auto');
  let discovered = $state([]);
  let scanning = $state(false);
  let error = $state('');
  let importing = $state(false);
  let selectedDomains = $state(new Set());
  let serversLoading = $state(true);

  onMount(async () => {
    try {
      servers = await api.servers.list({ all: true });
    } catch (_) {
      servers = [];
    } finally {
      serversLoading = false;
    }
  });

  const providers = [
    { value: 'auto', label: 'Auto', icon: 'solar:magic-stick-3-bold' },
    { value: 'traefik', label: 'Traefik', icon: 'solar:tunel-bold' },
    { value: 'nginx', label: 'Nginx', icon: 'solar:code-square-bold' },
    { value: 'caddy', label: 'Caddy', icon: 'solar:code-square-bold' },
    { value: 'letsencrypt', label: 'LetsEncrypt', icon: 'solar:shield-check-bold' },
  ];

  async function scanServer() {
    if (!selectedServer) return;
    scanning = true;
    error = '';
    discovered = [];
    selectedDomains = new Set();
    try {
      const token = typeof window !== 'undefined' ? localStorage.getItem('access_token') : null;
      const headers = { 'Content-Type': 'application/json' };
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const res = await fetch('/api/v1/ssl-monitors/discover', {
        method: 'POST',
        headers,
        body: JSON.stringify({ server_id: selectedServer, provider: selectedProvider }),
      });
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.error || `Request failed (${res.status})`);
      }
      const data = await res.json();
      discovered = data?.domains || [];
      // Auto-select all
      selectedDomains = new Set(discovered.map((_, i) => i));
    } catch (e) {
      error = e.message;
    } finally {
      scanning = false;
    }
  }

  async function importSelected() {
    importing = true;
    try {
      const domains = discovered.filter((_, i) => selectedDomains.has(i))
        .map(d => ({
          domain: d.domain,
          port: d.port,
          display_name: d.display_name || d.domain,
          source_provider: d.source_provider || 'discovered',
          server_id: selectedServer,
        }));
      const token = typeof window !== 'undefined' ? localStorage.getItem('access_token') : null;
      const headers = { 'Content-Type': 'application/json' };
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const res = await fetch('/api/v1/ssl-monitors/discover/import', {
        method: 'POST',
        headers,
        body: JSON.stringify({ domains, enabled: true }),
      });
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.error || `Import failed (${res.status})`);
      }
      const result = await res.json();
      onClose(true); // true = refresh needed
    } catch (e) {
      error = e.message;
    } finally {
      importing = false;
    }
  }

  function toggleDomain(idx) {
    const next = new Set(selectedDomains);
    if (next.has(idx)) next.delete(idx);
    else next.add(idx);
    selectedDomains = next;
  }

  function formatExpiry(dateStr) {
    if (!dateStr) return '—';
    const d = new Date(dateStr);
    const now = new Date();
    const days = Math.ceil((d - now) / 86400000);
    return `${d.toLocaleDateString('en-GB')} (${days}d)`;
  }

  const statusConfig = {
    pending: { label: 'Pending', color: '#6366f1' },
    valid: { label: 'Valid', color: '#10b981' },
    expiring_soon: { label: 'Expiring', color: '#f59e0b' },
    expired: { label: 'Expired', color: '#ef4444' },
    error: { label: 'Error', color: '#ef4444' },
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onclick={() => onClose(false)} role="presentation">
    <div class="card w-full max-w-2xl max-h-[85vh] overflow-y-auto mx-4" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-base font-semibold flex items-center gap-2">
          <Icon icon="solar:search-bold" class="h-5 w-5" style="color: var(--color-primary);" />
          Discover SSL Certificates
        </h2>
        <button onclick={() => onClose(false)} class="btn-ghost p-1" title="Close">
          <Icon icon="solar:close-circle-bold" class="h-5 w-5" style="color: var(--color-text-muted);" />
        </button>
      </div>

      <!-- Server + Provider -->
      <div class="flex flex-col sm:flex-row gap-3 mb-4">
        <div class="flex-1">
          <label class="text-xs font-medium mb-1 block" style="color: var(--color-text);">Server</label>
          {#if serversLoading}
            <p class="text-xs text-muted">Loading servers...</p>
          {:else}
            <select bind:value={selectedServer} class="input w-full">
              <option value="">Select a server...</option>
              {#each servers as s}
                <option value={s.id}>{s.name} ({s.host})</option>
              {/each}
            </select>
          {/if}
        </div>
      </div>

      <!-- Provider chips -->
      <div class="mb-4">
        <label class="text-xs font-medium mb-1.5 block" style="color: var(--color-text);">Provider</label>
        <div class="flex flex-wrap gap-1.5">
          {#each providers as p}
            <button onclick={() => selectedProvider = p.value}
              class="px-3 py-1.5 rounded-lg text-xs font-medium transition-all"
              style={selectedProvider === p.value
                ? 'background-color: var(--color-primary); color: #fff;'
                : 'background-color: var(--color-card-alt); color: var(--color-text); border: 1px solid var(--color-border);'}>
              <Icon icon={p.icon} class="inline h-3.5 w-3.5 mr-1" />
              {p.label}
            </button>
          {/each}
        </div>
      </div>

      <button onclick={scanServer} disabled={!selectedServer || scanning}
        class="w-full text-sm py-2 rounded-lg font-medium transition-all"
        style="background-color: var(--color-primary); color: #fff; border: none; {!selectedServer || scanning ? 'opacity: 0.5;' : ''}">
        {#if scanning}
          <Icon icon="solar:spinner-bold" class="inline h-4 w-4 animate-spin mr-1" />
          Scanning...
        {:else}
          <Icon icon="solar:search-bold" class="inline h-4 w-4 mr-1" />
          Scan Server
        {/if}
      </button>

      {#if error}
        <div class="mt-3 p-3 rounded-lg text-xs" style="background-color: rgba(239,68,68,0.08); color: var(--color-danger); border: 1px solid rgba(239,68,68,0.2);">
          <Icon icon="solar:danger-triangle-bold" class="inline h-3.5 w-3.5 mr-1" />
          {error}
        </div>
      {/if}

      <!-- Results -->
      {#if discovered.length > 0}
        <div class="mt-4">
          <div class="flex items-center justify-between mb-2">
            <p class="text-sm font-medium" style="color: var(--color-text);">
              {discovered.length} certificate{discovered.length !== 1 ? 's' : ''} found
            </p>
            <button onclick={() => {
              if (selectedDomains.size === discovered.length) {
                selectedDomains = new Set();
              } else {
                selectedDomains = new Set(discovered.map((_, i) => i));
              }
            }} class="text-xs font-medium" style="color: var(--color-primary);">
              {selectedDomains.size === discovered.length ? 'Deselect all' : 'Select all'}
            </button>
          </div>

          <div class="space-y-1.5 max-h-60 overflow-y-auto">
            {#each discovered as cert, i}
              <label class="flex items-start gap-3 p-2.5 rounded-lg cursor-pointer transition-colors"
                style="background-color: var(--color-card-alt); border: 1px solid var(--color-border); {selectedDomains.has(i) ? 'border-color: var(--color-primary);' : ''}">
                <input type="checkbox" checked={selectedDomains.has(i)} onchange={() => toggleDomain(i)}
                  class="mt-0.5 h-4 w-4 shrink-0" />
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium"
                      style="background-color: {statusConfig.valid.color}12; color: {statusConfig.valid.color};">
                      <span class="h-1.5 w-1.5 rounded-full" style="background-color: {statusConfig.valid.color};"></span>
                      {cert.days_remaining ? `${cert.days_remaining}d` : '—'}
                    </span>
                    <span class="font-medium text-sm truncate" style="color: var(--color-text);">{cert.display_name || cert.domain}</span>
                  </div>
                  <p class="text-xs mt-0.5" style="color: var(--color-text-muted);">
                    {cert.domain}:{cert.port || 443} · Issuer: {cert.issuer || '?'} · Expires: {formatExpiry(cert.cert_expires_at)}
                  </p>
                  <p class="text-[10px]" style="color: var(--color-text-muted);">
                    Source: <span class="font-medium">{cert.source_provider}</span>
                    {#if cert.san_names?.length}
                      · SAN: {cert.san_names.slice(0, 2).join(', ')}{cert.san_names.length > 2 ? ` +${cert.san_names.length - 2}` : ''}
                    {/if}
                  </p>
                </div>
              </label>
            {/each}
          </div>

          <button onclick={importSelected} disabled={selectedDomains.size === 0 || importing}
            class="w-full mt-3 text-sm py-2 rounded-lg font-medium transition-all"
            style="background-color: var(--color-primary); color: #fff; border: none; {selectedDomains.size === 0 || importing ? 'opacity: 0.5;' : ''}">
            {#if importing}
              <Icon icon="solar:spinner-bold" class="inline h-4 w-4 animate-spin mr-1" />
              Importing...
            {:else}
              <Icon icon="solar:import-bold" class="inline h-4 w-4 mr-1" />
              Import {selectedDomains.size} selected
            {/if}
          </button>
        </div>
      {:else if !scanning && selectedServer && !error}
        <p class="text-xs mt-4 text-center" style="color: var(--color-text-muted);">
          <Icon icon="solar:info-circle-bold" class="inline h-3.5 w-3.5 mr-1" />
          Click "Scan Server" to discover SSL certificates
        </p>
      {/if}
    </div>
  </div>
{/if}
