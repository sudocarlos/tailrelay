<script>
  import { X, AlertTriangle } from '@lucide/svelte';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';

  let { type = 'relay', id = '', name = '', onDelete, onClose } = $props();

  let deleting = $state(false);

  async function handleDelete() {
    deleting = true;
    try {
      if (type === 'relay') {
        await fetchJSON(`/api/socat/delete?id=${encodeURIComponent(id)}`, { method: 'POST' });
      } else {
        await fetchJSON(`/api/caddy/delete?id=${encodeURIComponent(id)}`, { method: 'POST' });
      }
      showToast('success', `${type === 'relay' ? 'Relay' : 'Proxy'} deleted successfully`);
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
        Are you sure you want to delete this {type === 'relay' ? 'TCP relay' : 'HTTPS proxy'}?
      </p>
      {#if name}
        <p class="text-sm font-medium mt-2 break-all">{name}</p>
      {/if}
      <p class="text-xs text-gray-500 dark:text-gray-500 mt-2">This action cannot be undone.</p>
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
