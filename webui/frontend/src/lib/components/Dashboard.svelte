<script>
  import { onMount, onDestroy } from 'svelte';
  import {
    filteredItems,
    relays,
    proxies,
    funnels,
    usedFunnelPorts,
    tailnetFQDN,
    targets,
    lastUpdated,
    refreshData,
    logs,
    hideFunnel,
  } from '../stores/app.js';
  import { get } from 'svelte/store';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import ItemRow from './ItemRow.svelte';
  import FunnelSection from './FunnelSection.svelte';
  import AddModal from './AddModal.svelte';
  import DeleteModal from './DeleteModal.svelte';
  import LogConsole from './LogConsole.svelte';
  import {
    Plus,
    Search,
  } from '@lucide/svelte';
  import { Input } from '@/lib/components/ui/input';
  import { Button } from '@/lib/components/ui/button';

  // Endpoints and item lookups keyed by relay type, shared by the toggle
  // and autostart handlers below so relay/proxy/funnel share one code path.
  const TOGGLE_URLS = {
    relay: '/api/serve/tcp/toggle',
    proxy: '/api/serve/https/toggle',
    funnel: '/api/serve/funnel/toggle',
  };
  const UPDATE_URLS = {
    relay: '/api/serve/tcp/update',
    proxy: '/api/serve/https/update',
    funnel: '/api/serve/funnel/update',
  };

  function findItem(type, id) {
    if (type === 'relay') return get(relays).find((r) => r.relay.id === id)?.relay;
    if (type === 'proxy') return get(proxies).find((p) => p.id === id);
    return get(funnels).find((f) => f.id === id);
  }

  function isItemRunning(type, id) {
    if (type === 'relay') return get(relays).find((r) => r.relay.id === id)?.running;
    if (type === 'proxy') return get(proxies).find((p) => p.id === id)?.enabled ?? false;
    return get(funnels).find((f) => f.id === id)?.enabled ?? false;
  }

  let items = $state([]);
  let funnelList = $state([]);
  let usedPorts = $state(new Map());
  let updated = $state('');
  let searchQuery = $state('');
  let fqdn = $state('');
  let targetList = $state([]);
  let funnelHidden = $state(false);

  // Modal state
  let showAddModal = $state(false);
  let editItem = $state(null);
  let editType = $state('relay');
  let funnelPortToConfigure = $state(null);

  let showDeleteModal = $state(false);
  let deleteTarget = $state(null);

  // Track which item is currently being toggled (by "type:id")
  let togglingId = $state(null);

  filteredItems.subscribe((v) => (items = v));
  funnels.subscribe((v) => (funnelList = v));
  usedFunnelPorts.subscribe((v) => (usedPorts = v));
  lastUpdated.subscribe((v) => (updated = v));
  tailnetFQDN.subscribe((v) => (fqdn = v));
  targets.subscribe((v) => (targetList = v));
  hideFunnel.subscribe((v) => (funnelHidden = v));

  const displayedItems = $derived(
    (() => {
      const q = searchQuery.trim().toLowerCase();
      if (!q) return items;
      return items.filter((item) => {
        const listenPort = item.type === 'relay' ? item.relay.listen_port : item.proxy.listen_port;
        const targetHost = item.type === 'relay' ? item.relay.target_host : item.proxy.target_host;
        const targetPort = item.type === 'relay' ? item.relay.target_port : item.proxy.target_port;
        const magicDns = `${fqdn}:${listenPort}`.toLowerCase();
        const target = `${targetHost}:${targetPort}`.toLowerCase();
        return magicDns.includes(q) || target.includes(q);
      });
    })()
  );

  function openAdd(type = 'proxy') {
    editItem = null;
    editType = type;
    funnelPortToConfigure = null;
    showAddModal = true;
  }

  function openConfigureFunnel(port) {
    editItem = null;
    editType = 'funnel';
    funnelPortToConfigure = port;
    showAddModal = true;
  }

  function openEdit(type, item) {
    editItem = item;
    editType = type;
    funnelPortToConfigure = null;
    showAddModal = true;
  }

  function openDelete(type, id, name, target, iconUrl = '', running = false) {
    deleteTarget = { type, id, name, target, iconUrl, running };
    showDeleteModal = true;
  }

  async function handleToggleAction(type, id, currentState) {
    const key = `${type}:${id}`;
    togglingId = key;
    try {
      await fetchJSON(TOGGLE_URLS[type], {
        method: 'POST',
        body: JSON.stringify({ id, enabled: !currentState }),
      });
      // Poll until status reflects the expected state (up to ~3 s)
      const expected = !currentState;
      for (let i = 0; i < 6; i++) {
        await refreshData();
        const entry = findItem(type, id);
        const actualRunning = type === 'relay'
          ? get(relays).find((r) => r.relay.id === id)?.running
          : (entry?.running ?? false);
        if (actualRunning === expected) break;
        await new Promise((r) => setTimeout(r, 500));
      }
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      togglingId = null;
    }
  }

  async function handleAutostartToggle(type, id, autostart) {
    try {
      const currentItem = findItem(type, id);
      if (!currentItem) throw new Error(`${type} not found`);

      await fetchJSON(UPDATE_URLS[type], {
        method: 'POST',
        body: JSON.stringify({ ...currentItem, autostart }),
      });
      await refreshData();
    } catch (err) {
      showToast('danger', err.message);
    }
  }

  async function handleSaved() {
    showAddModal = false;
    editItem = null;
    funnelPortToConfigure = null;
    await refreshData();
  }

  async function handleDeleted() {
    showDeleteModal = false;
    deleteTarget = null;
    await refreshData();
  }

  // Sync filter toggles with stores — removed (type filters replaced by search)

  function handleKeydown(e) {
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return;
    if (showAddModal || showDeleteModal) return;
    if (e.key === 'n') {
      e.preventDefault();
      openAdd();
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeydown);

    // Warn once on mount about any TLS certificate issues found in the current
    // proxy list. This surfaces problems that would otherwise only be visible
    // via the inline card warning or the container logs.
    get(proxies).forEach((proxy) => {
      const tlsErr = proxy.tls_error || proxy.TLSError || '';
      if (tlsErr) {
        const hostname = proxy.hostname || proxy.Hostname || proxy.id;
        showToast('warning', `TLS cert issue on ${hostname}: ${tlsErr}`);
      }
    });
  });

  onDestroy(() => {
    window.removeEventListener('keydown', handleKeydown);
  });
</script>

<!-- Page title (toolbar below, search left / add right like the console) -->
<div class="mb-5">
  <h1 class="text-2xl font-semibold tracking-tight">Relays</h1>
  <p class="text-sm text-muted-foreground mt-0.5">
    {displayedItems.length} item{displayedItems.length === 1 ? '' : 's'}
    {#if updated}
      <span class="mx-1">·</span> updated {updated}
    {/if}
  </p>
</div>

<!-- Toolbar: search left, Add right -->
<div class="mb-4 flex items-center justify-between gap-3">
  <!-- Search -->
  <div class="relative w-full max-w-sm">
    <Search size={15} class="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
    <Input
      type="search"
      bind:value={searchQuery}
      placeholder="Search relays…"
      class="h-9 pl-8"
    />
  </div>

  <Button class="gap-1.5" onclick={() => openAdd()}>
    <Plus size={15} />
    Add
  </Button>
</div>

<!-- Items as flat rows (no cards), like the console machines list -->
<div class="mb-6">
  {#if displayedItems.length === 0}
    <div class="text-center py-16 px-6">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 80 80" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="w-20 h-20 mx-auto mb-4 text-muted-foreground/60">
        <rect x="10" y="20" width="60" height="40" rx="4" />
        <line x1="10" y1="32" x2="70" y2="32" />
        <circle cx="18" cy="26" r="2" fill="currentColor" />
        <circle cx="26" cy="26" r="2" fill="currentColor" />
        <circle cx="34" cy="26" r="2" fill="currentColor" />
        <line x1="30" y1="44" x2="50" y2="44" />
        <line x1="25" y1="50" x2="55" y2="50" />
      </svg>
      <p class="text-muted-foreground mb-4">
        {#if searchQuery.trim()}
          No relays match "{searchQuery.trim()}".
        {:else}
          No relays configured. Get started by adding one.
        {/if}
      </p>
      {#if !searchQuery.trim()}
        <Button size="sm" class="gap-1.5" onclick={() => openAdd()}>
          <Plus size={15} />
          Add a Relay
        </Button>
      {/if}
    </div>
  {:else}
    <table class="w-full table-fixed">
      <thead>
        <tr class="border-b border-border text-left">
          <th class="w-auto pb-2 text-xs font-normal text-muted-foreground sm:w-[55%]">Address</th>
          <th class="hidden pb-2 pr-4 text-xs font-normal text-muted-foreground sm:table-cell">Target</th>
          <th class="w-12 pb-2" aria-hidden="true"></th>
          <th class="w-8 pb-2" aria-hidden="true"></th>
        </tr>
      </thead>
      <tbody class="divide-y divide-border">
        {#each displayedItems as item (item.type === 'relay' ? `relay-${item.relay.id}` : `proxy-${item.proxy.id}`)}
          <ItemRow
            {item}
            {fqdn}
            toggling={item.type === 'relay'
              ? togglingId === `relay:${item.relay.id}`
              : togglingId === `proxy:${item.proxy.id}`}
            onToggle={handleToggleAction}
            onAutostart={handleAutostartToggle}
            onEdit={(type, data) => openEdit(type, data)}
            onDelete={(type, id, name, target, iconUrl, running) => openDelete(type, id, name, target, iconUrl, running)}
          />
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<!-- Funnel — hidden while connected to a custom control server, since
     Funnel is a Tailscale-cloud-only feature not supported by self-hosted
     Headscale. -->
{#if !funnelHidden}
  <FunnelSection
    funnels={funnelList}
    usedFunnelPorts={usedPorts}
    {fqdn}
    {togglingId}
    onConfigure={openConfigureFunnel}
    onToggle={(id, running) => handleToggleAction('funnel', id, running)}
    onAutostart={(id, autostart) => handleAutostartToggle('funnel', id, autostart)}
    onEdit={(item) => openEdit('funnel', item)}
    onDelete={(id, name, target, iconUrl, running) => openDelete('funnel', id, name, target, iconUrl, running)}
  />
{/if}

<!-- Log Console -->
<LogConsole />

<!-- FAB (mobile) -->
<div class="sm:hidden fixed bottom-6 right-6 z-30">
  <Button class="size-14 rounded-full shadow-lg" onclick={() => openAdd()}>
    <Plus size={24} />
  </Button>
</div>

<!-- Modals -->
{#if showAddModal}
  <AddModal
    type={editType}
    item={editItem}
    {fqdn}
    targets={targetList}
    funnelPort={funnelPortToConfigure}
    onSave={handleSaved}
    onClose={() => { showAddModal = false; editItem = null; funnelPortToConfigure = null; }}
  />
{/if}

{#if showDeleteModal && deleteTarget}
  <DeleteModal
    type={deleteTarget.type}
    id={deleteTarget.id}
    name={deleteTarget.name}
    target={deleteTarget.target}
    iconUrl={deleteTarget.iconUrl}
    running={deleteTarget.running}
    onDelete={handleDeleted}
    onClose={() => { showDeleteModal = false; deleteTarget = null; }}
  />
{/if}
