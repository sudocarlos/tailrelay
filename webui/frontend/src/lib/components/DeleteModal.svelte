<script>
  import { AlertTriangle, Network, ShieldCheck, Globe } from '@lucide/svelte';
  import RelayIcon from './RelayIcon.svelte';
  import { statusIconClass } from '../utils/statusBadge.js';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import * as AlertDialog from '@/lib/components/ui/alert-dialog';
  import { Button } from '@/lib/components/ui/button';

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

</script>

<AlertDialog.Root
  open
  onOpenChange={(v) => { if (!v) onClose(); }}
>
  <AlertDialog.Content class="rounded-xl">
    <!-- Header -->
    <AlertDialog.Header>
      <AlertDialog.Title>Confirm Delete</AlertDialog.Title>
      <AlertDialog.Description>
        Are you sure you want to delete this {typeLabel}?
      </AlertDialog.Description>
    </AlertDialog.Header>

    <!-- Body -->
    <div class="text-center">
      <div class="mx-auto w-12 h-12 flex items-center justify-center rounded-full bg-red-50 dark:bg-red-900/20 mb-4">
        <AlertTriangle size={24} class="text-red-500" />
      </div>
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
    <AlertDialog.Footer>
      <AlertDialog.Cancel>
        {#snippet child({ props })}
          <Button {...props} variant="outline" onclick={onClose}>Cancel</Button>
        {/snippet}
      </AlertDialog.Cancel>
      <Button variant="destructive" disabled={deleting} onclick={handleDelete}>
        {deleting ? 'Deleting...' : 'Delete'}
      </Button>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
