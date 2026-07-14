<script>
  import { untrack } from 'svelte';
  import { Globe, Ban, Plus, ChevronDown, ChevronUp } from '@lucide/svelte';
  import { FUNNEL_PORTS } from '../stores/app.js';
  import Toggle from './Toggle.svelte';
  import ItemMenu from './ItemMenu.svelte';
  import CopyButton from './CopyButton.svelte';

  let {
    funnels = [],
    usedFunnelPorts = new Map(),
    fqdn = '',
    togglingId = null,
    onConfigure,
    onToggle,
    onAutostart,
    onEdit,
    onDelete,
  } = $props();

  // Start expanded if any funnel is already configured (data is loaded
  // before this component first mounts — see App.svelte's initial
  // refreshData() gate), otherwise start collapsed. Subsequent prop
  // updates don't re-trigger this; the user's manual toggle takes over.
  // untrack() reads the initial value only, avoiding a state_referenced_locally warning.
  let expanded = $state(untrack(() => funnels.length > 0));

  // Memoized port → funnel lookup, recomputed only when the funnels list
  // changes (rather than a linear .find() per port on every render).
  const funnelsByPort = $derived(new Map(funnels.map((f) => [f.listen_port, f])));

  function formatFunnelUrl(funnel) {
    const scheme = funnel.funnel_transport === 'tcp' ? 'tcp' : 'https';
    const port = scheme === 'https' && funnel.listen_port === 443 ? '' : `:${funnel.listen_port}`;
    return `${scheme}://${funnel.hostname || fqdn}${port}`;
  }

  function formatFunnelTarget(funnel) {
    return `${funnel.target_host}:${funnel.target_port}`;
  }

  // Mirrors ItemCard.svelte's status-badge helpers: the type icon doubles
  // as the status indicator instead of a separate dot.
  function statusBadgeClass(isToggling, isRunning) {
    if (isToggling) return 'bg-amber-400 animate-pulse';
    return isRunning ? 'bg-green-700 status-dot-running' : 'bg-gray-100 dark:bg-gray-800';
  }

  function statusIconClass(isToggling, isRunning) {
    return isToggling || isRunning ? 'text-white' : 'text-blue-500';
  }
</script>

<!-- Header -->
<button
  type="button"
  class="w-full flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-3 text-left"
  onclick={() => (expanded = !expanded)}
>
  <div>
    <h2 class="text-xl font-semibold">Funnel</h2>
    <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
      {funnels.length} of {FUNNEL_PORTS.length} configured &middot; expose services to the public internet
    </p>
  </div>
  {#if expanded}
    <ChevronUp size={18} class="text-gray-400 flex-shrink-0" />
  {:else}
    <ChevronDown size={18} class="text-gray-400 flex-shrink-0" />
  {/if}
</button>

{#if expanded}
  <div class="flex flex-col gap-3 mb-6">
    {#each FUNNEL_PORTS as port (port)}
      {@const funnel = funnelsByPort.get(port)}
      {@const usedBy = usedFunnelPorts.get(port)}

      {#if funnel}
        {@const running = funnel.running}
        {@const autostart = funnel.autostart ?? false}
        {@const toggling = togglingId === `funnel:${funnel.id}`}
        {@const funnelUrl = formatFunnelUrl(funnel)}

        <div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 px-4 py-3">
          <div class="flex flex-col sm:flex-row sm:items-center gap-3">
            <!-- Info -->
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <span
                  class="flex items-center justify-center w-6 h-6 rounded-full flex-shrink-0 transition-colors {statusBadgeClass(toggling, running)}"
                  title={toggling ? 'Updating…' : running ? 'Running' : 'Stopped'}
                >
                  <Globe size={14} class={statusIconClass(toggling, running)} />
                </span>
                <a
                  href={funnelUrl}
                  target="_blank"
                  rel="noopener"
                  class="font-medium text-sm truncate hover:underline"
                >{funnelUrl}</a>
                <CopyButton text={funnelUrl} />
              </div>
              <p class="font-medium text-sm mt-1 ml-8">
                &rarr; {formatFunnelTarget(funnel)}
              </p>
            </div>

            <!-- Actions -->
            <div class="flex items-center gap-1 ml-8 sm:ml-0">
              <Toggle
                checked={running}
                disabled={toggling}
                onChange={() => onToggle(funnel.id, running)}
                label={running ? 'Stop funnel' : 'Start funnel'}
              />
              <ItemMenu
                {autostart}
                onAutostartChange={(v) => onAutostart(funnel.id, v)}
                onEdit={() => onEdit(funnel)}
                onDelete={() => onDelete(funnel.id, funnelUrl, formatFunnelTarget(funnel))}
              />
            </div>
          </div>
        </div>
      {:else if usedBy}
        <div class="rounded-lg border border-dashed border-gray-200 dark:border-gray-800 px-4 py-3 opacity-40 cursor-not-allowed" title="Port {port} is in use by a serve relay">
          <div class="flex items-center gap-2 text-sm">
            <Ban size={16} class="text-gray-400 flex-shrink-0" />
            <span>Port {port} is in use by a serve relay</span>
          </div>
        </div>
      {:else}
        <button
          type="button"
          class="rounded-lg border border-dashed border-gray-300 dark:border-gray-700 px-4 py-3 text-left hover:border-blue-400 dark:hover:border-blue-600 hover:bg-blue-50/50 dark:hover:bg-blue-950/20 transition-colors"
          onclick={() => onConfigure(port)}
        >
          <div class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            <Plus size={16} class="flex-shrink-0" />
            <span>Configure funnel on port {port}</span>
          </div>
        </button>
      {/if}
    {/each}
  </div>
{/if}
