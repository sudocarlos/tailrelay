<script>
  import { onMount, onDestroy } from 'svelte';
  import {
    filteredItems,
    relays,
    proxies,
    tailnetFQDN,
    targets,
    showRelays,
    showProxies,
    lastUpdated,
    refreshData,
    logs,
    logLevel,
  } from '../stores/app.js';
  import { get } from 'svelte/store';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import ItemCard from './ItemCard.svelte';
  import AddModal from './AddModal.svelte';
  import DeleteModal from './DeleteModal.svelte';
  import LogConsole from './LogConsole.svelte';
  import {
    Plus,
    Filter,
    HelpCircle,
  } from '@lucide/svelte';

  let items = $state([]);
  let updated = $state('');
  let filterRelays = $state(true);
  let filterProxies = $state(true);
  let fqdn = $state('');
  let targetList = $state([]);

  // Modal state
  let showAddModal = $state(false);
  let editItem = $state(null);
  let editType = $state('relay');

  let showDeleteModal = $state(false);
  let deleteTarget = $state(null);

  filteredItems.subscribe((v) => (items = v));
  lastUpdated.subscribe((v) => (updated = v));
  tailnetFQDN.subscribe((v) => (fqdn = v));
  targets.subscribe((v) => (targetList = v));

  function openAdd(type = 'relay') {
    editItem = null;
    editType = type;
    showAddModal = true;
  }

  function openEdit(type, item) {
    editItem = item;
    editType = type;
    showAddModal = true;
  }

  function openDelete(type, id, name) {
    deleteTarget = { type, id, name };
    showDeleteModal = true;
  }

  async function handleToggleAction(type, id, currentState) {
    try {
      if (type === 'relay') {
        const url = currentState
          ? `/api/socat/stop?id=${encodeURIComponent(id)}`
          : `/api/socat/start?id=${encodeURIComponent(id)}`;
        await fetchJSON(url, { method: 'POST' });
      } else {
        await fetchJSON('/api/caddy/toggle', {
          method: 'POST',
          body: JSON.stringify({ id, enabled: !currentState }),
        });
      }
      await refreshData();
    } catch (err) {
      showToast('danger', err.message);
    }
  }

  async function handleAutostartToggle(type, id, autostart) {
    try {
      const url = type === 'relay' ? '/api/socat/update' : '/api/caddy/update';
      let currentItem;

      if (type === 'relay') {
        let relayList;
        relays.subscribe((v) => (relayList = v))();
        currentItem = relayList.find((r) => r.relay.id === id)?.relay;
      } else {
        let proxyList;
        proxies.subscribe((v) => (proxyList = v))();
        currentItem = proxyList.find((p) => p.id === id);
      }

      if (!currentItem) throw new Error(`${type} not found`);

      await fetchJSON(url, {
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
    await refreshData();
  }

  async function handleDeleted() {
    showDeleteModal = false;
    deleteTarget = null;
    await refreshData();
  }

  // Sync filter toggles with stores
  $effect(() => {
    showRelays.set(filterRelays);
  });

  $effect(() => {
    showProxies.set(filterProxies);
  });

  function handleKeydown(e) {
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return;
    if (showAddModal || showDeleteModal) return;
    if (e.key === 'n') {
      e.preventDefault();
      openAdd('relay');
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

<!-- Header -->
<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-6">
  <div>
    <h1 class="text-xl font-semibold">Relays & Proxies</h1>
    <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
      {items.length} item{items.length === 1 ? '' : 's'}
      {#if updated}
        <span class="mx-1">&middot;</span> updated {updated}
      {/if}
    </p>
  </div>

  <div class="flex items-center gap-2">
    <!-- Filters -->
    <div class="flex items-center gap-3 mr-2">
      <label class="flex items-center gap-1.5 text-sm cursor-pointer">
        <input
          type="checkbox"
          bind:checked={filterRelays}
          class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 dark:bg-gray-800"
        />
        <span class="text-gray-600 dark:text-gray-400">Relays</span>
      </label>
      <label class="flex items-center gap-1.5 text-sm cursor-pointer">
        <input
          type="checkbox"
          bind:checked={filterProxies}
          class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 dark:bg-gray-800"
        />
        <span class="text-gray-600 dark:text-gray-400">Proxies</span>
      </label>
    </div>

    <button
      class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-white bg-blue-500 hover:bg-blue-600 rounded-lg transition-colors"
      onclick={() => openAdd('relay')}
    >
      <Plus size={15} />
      Add
    </button>
  </div>
</div>

<!-- Items -->
<div class="flex flex-col gap-3 mb-6">
  {#if items.length === 0}
    <div class="text-center py-16 px-6">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 80 80" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="w-20 h-20 mx-auto mb-4 text-gray-300 dark:text-gray-600">
        <rect x="10" y="20" width="60" height="40" rx="4" />
        <line x1="10" y1="32" x2="70" y2="32" />
        <circle cx="18" cy="26" r="2" fill="currentColor" />
        <circle cx="26" cy="26" r="2" fill="currentColor" />
        <circle cx="34" cy="26" r="2" fill="currentColor" />
        <line x1="30" y1="44" x2="50" y2="44" />
        <line x1="25" y1="50" x2="55" y2="50" />
      </svg>
      <p class="text-gray-500 dark:text-gray-400 mb-4">
        {#if !filterRelays && !filterProxies}
          Enable TCP relays or HTTPS proxies to view items.
        {:else if filterRelays && !filterProxies}
          No TCP relays configured. Get started by adding one.
        {:else if !filterRelays && filterProxies}
          No HTTPS proxies configured. Get started by adding one.
        {:else}
          No relays or proxies configured. Get started by adding one.
        {/if}
      </p>
      <button
        class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-white bg-blue-500 hover:bg-blue-600 rounded-lg transition-colors"
        onclick={() => openAdd(filterProxies && !filterRelays ? 'proxy' : 'relay')}
      >
        <Plus size={15} />
        {#if filterRelays && !filterProxies}
          Add a Relay
        {:else if !filterRelays && filterProxies}
          Add a Proxy
        {:else}
          Add a Proxy or Relay
        {/if}
      </button>
    </div>
  {:else}
    {#each items as item (item.type === 'relay' ? `relay-${item.relay.id}` : `proxy-${item.proxy.id}`)}
      <ItemCard
        {item}
        {fqdn}
        onToggle={handleToggleAction}
        onAutostart={handleAutostartToggle}
        onEdit={(type, data) => openEdit(type, data)}
        onDelete={(type, id, name) => openDelete(type, id, name)}
      />
    {/each}
  {/if}
</div>

<!-- Log Console -->
<LogConsole />

<!-- FAB (mobile) -->
<div class="sm:hidden fixed bottom-6 right-6 z-30">
  <button
    class="w-14 h-14 flex items-center justify-center bg-blue-500 hover:bg-blue-600 text-white rounded-full shadow-lg hover:shadow-xl transition-all active:scale-95"
    onclick={() => openAdd('relay')}
  >
    <Plus size={24} />
  </button>
</div>

<!-- Modals -->
{#if showAddModal}
  <AddModal
    type={editType}
    item={editItem}
    {fqdn}
    targets={targetList}
    onSave={handleSaved}
    onClose={() => { showAddModal = false; editItem = null; }}
  />
{/if}

{#if showDeleteModal && deleteTarget}
  <DeleteModal
    type={deleteTarget.type}
    id={deleteTarget.id}
    name={deleteTarget.name}
    onDelete={handleDeleted}
    onClose={() => { showDeleteModal = false; deleteTarget = null; }}
  />
{/if}
