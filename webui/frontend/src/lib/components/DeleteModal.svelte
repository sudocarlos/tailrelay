<script>
  import { X, AlertTriangle, Network, ShieldCheck, Globe } from '@lucide/svelte';
  import RelayIcon from './RelayIcon.svelte';
  import { statusIconClass } from '../utils/statusBadge.js';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';

  let {
    type = 'relay',
    id = '',
    name = '',
    target = '',
    iconUrl = '',
    running = false,
    onDelete,
    onClose,
  } = $props();

  const typeLabels = { relay: 'TCP relay', proxy: 'HTTPS relay', funnel: 'Funnel' };
  const typeLabel = $derived(typeLabels[type] ?? 'relay');

  const deleteEndpoints = {
    relay: '/api/serve/tcp/delete',
    proxy: '/api/serve/https/delete',
    funnel: '/api/serve/funnel/delete',
  };

  // Split the URL into a muted prefix and a bold suffix (port or nothing),
  // mirroring the two-span pattern used in ItemCard.svelte.
  const namePrefix = $derived(() => {
    const lastColon = name.lastIndexOf(':');
    return lastColon !== -1 ? name.slice(0, lastColon) : name;
  });
  const nameSuffix = $derived(() => {
    const lastColon = name.lastIndexOf(':');
    return lastColon !== -1 ? name.slice(lastColon) : '';
  });

  let deleting = $state(false);

  async function handleDelete() {
    deleting = true;
    try {
      await fetchJSON(`${deleteEndpoints[type]}?id=${encodeURIComponent(id)}`, { method: 'POST' });
      showToast('success', `${typeLabel} deleted successfully`);
      onDelete();
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      deleting = false;
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') onClose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Backdrop -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
  data-modal-open
  onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
  onkeydown={() => {}}
>
  <div class="bg-white dark:bg-gray-900 rounded-xl shadow-2xl w-full max-w-sm">
    <!-- Header -->
    <div class="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-800">
      <h2 class="text-lg font-semibold">Confirm Delete</h2>
      <button class="p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-400" onclick={onClose}>
        <X size={18} />
      </button>
    </div>

    <!-- Body -->
    <div class="px-5 py-5 text-center">
      <div class="mx-auto w-12 h-12 flex items-center justify-center rounded-full bg-red-50 dark:bg-red-900/20 mb-4">
        <AlertTriangle size={24} class="text-red-500" />
      </div>
      <p class="text-sm text-gray-600 dark:text-gray-400">
        Are you sure you want to delete this {typeLabel}?
      </p>
      {#if name}
        <div class="mt-3 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 px-4 py-3 text-left">
          <div class="flex items-start gap-2">
            <RelayIcon {iconUrl} {running} alt={name}>
              {#if type === 'relay'}
                <Network size={26} strokeWidth={2.5} class={statusIconClass(false, running)} />
              {:else if type === 'funnel'}
                <Globe size={26} strokeWidth={2.5} class={statusIconClass(false, running)} />
              {:else}
                <ShieldCheck size={26} strokeWidth={2.5} class={statusIconClass(false, running)} />
              {/if}
            </RelayIcon>
            <div class="flex-1 min-w-0">
              <span class="font-medium text-sm break-all">
                <span class="font-normal text-gray-400 dark:text-gray-500">{namePrefix()}</span><span>{nameSuffix()}</span>
              </span>
              {#if target}
                <p class="font-medium text-sm mt-1">&rarr; {target}</p>
              {/if}
            </div>
          </div>
        </div>
      {/if}
      <p class="text-xs text-gray-500 dark:text-gray-500 mt-3">This action cannot be undone.</p>
    </div>

    <!-- Footer -->
    <div class="flex justify-end gap-2 px-5 py-4 border-t border-gray-200 dark:border-gray-800">
      <button
        class="px-4 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
        onclick={onClose}
      >
        Cancel
      </button>
      <button
        class="px-4 py-2 text-sm font-medium rounded-lg bg-red-500 hover:bg-red-600 text-white transition-colors disabled:opacity-50"
        disabled={deleting}
        onclick={handleDelete}
      >
        {deleting ? 'Deleting...' : 'Delete'}
      </button>
    </div>
  </div>
</div>
