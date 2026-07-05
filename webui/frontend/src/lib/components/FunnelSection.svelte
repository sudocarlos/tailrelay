<script>
  import { Globe, Ban, Plus, Play, Pause, Pencil, Trash2, RefreshCw } from '@lucide/svelte';
  import { FUNNEL_PORTS } from '../stores/app.js';

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
</script>

<div class="flex items-center justify-between gap-3 mb-3">
  <div>
    <h2 class="text-base font-semibold">Funnel</h2>
    <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
      Expose a service on the public internet over ports 443, 8443, or 10000.
    </p>
  </div>
</div>

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
              <Globe size={16} class="text-blue-500 flex-shrink-0" />
              <a
                href={funnelUrl}
                target="_blank"
                rel="noopener"
                class="font-medium text-sm truncate hover:underline"
              >{funnelUrl}</a>
              <span
                class="w-2 h-2 rounded-full flex-shrink-0 {toggling ? 'bg-amber-400 animate-pulse' : running ? 'bg-green-500 status-dot-running' : 'bg-gray-400 dark:bg-gray-600'}"
                title={toggling ? 'Updating…' : running ? 'Running' : 'Stopped'}
              ></span>
            </div>
            <p class="font-medium text-sm mt-1 ml-6">
              &rarr; {formatFunnelTarget(funnel)}
            </p>
          </div>

          <!-- Actions -->
          <div class="flex items-center gap-2 ml-6 sm:ml-0">
            <label class="flex items-center gap-1.5 cursor-pointer" title="Start automatically on boot">
              <span class="text-xs text-gray-500 dark:text-gray-400">Auto</span>
              <input
                type="checkbox"
                checked={autostart}
                onchange={(e) => onAutostart(funnel.id, e.target.checked)}
                class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 h-3.5 w-3.5 dark:bg-gray-800"
              />
            </label>

            <div class="w-px h-5 bg-gray-200 dark:bg-gray-700"></div>

            <button
              class="p-1.5 rounded-md transition-colors {toggling ? 'text-amber-500 cursor-not-allowed' : running ? 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500' : 'hover:bg-green-50 dark:hover:bg-green-900/20 text-green-600'}"
              onclick={() => onToggle(funnel.id, running)}
              disabled={toggling}
              title={toggling ? 'Updating…' : running ? 'Stop' : 'Start'}
            >
              {#if toggling}
                <RefreshCw size={15} class="animate-spin" />
              {:else if running}
                <Pause size={15} />
              {:else}
                <Play size={15} />
              {/if}
            </button>

            <button
              class="p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500 transition-colors"
              onclick={() => onEdit(funnel)}
              title="Edit"
            >
              <Pencil size={15} />
            </button>

            <button
              class="p-1.5 rounded-md hover:bg-red-50 dark:hover:bg-red-900/20 text-red-500 transition-colors"
              onclick={() => onDelete(funnel.id, funnelUrl, formatFunnelTarget(funnel))}
              title="Delete"
            >
              <Trash2 size={15} />
            </button>
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
