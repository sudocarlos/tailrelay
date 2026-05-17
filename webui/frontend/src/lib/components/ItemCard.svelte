<script>
  import { Network, ShieldCheck, Play, Pause, Pencil, Trash2, AlertTriangle, RefreshCw } from '@lucide/svelte';

  let { item, fqdn, toggling = false, onToggle, onAutostart, onEdit, onDelete } = $props();

  function formatRelayTitle(relay) {
    const hostname = fqdn || 'unknown';
    return `tcp://${hostname}:${relay.listen_port}`;
  }

  function formatRelayTarget(relay) {
    return `${relay.target_host}:${relay.target_port}`;
  }

  function formatProxyUrl(proxy) {
    const port = proxy.listen_port && proxy.listen_port !== 443 ? `:${proxy.listen_port}` : '';
    return `https://${proxy.hostname || fqdn}${port}`;
  }
</script>

{#if item.type === 'relay'}
  {@const relay = item.relay}
  {@const running = item.running}
  {@const autostart = relay.autostart ?? false}

  <div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 px-4 py-3">
    <div class="flex flex-col sm:flex-row sm:items-center gap-3">
      <!-- Info -->
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2">
          <Network size={16} class="text-blue-500 flex-shrink-0" />
          <span class="font-medium text-sm truncate"><span class="text-sm font-normal text-gray-400 dark:text-gray-500">tcp://{fqdn || 'unknown'}</span><span>:{relay.listen_port}</span></span>
          <span
            class="w-2 h-2 rounded-full flex-shrink-0 {toggling ? 'bg-amber-400 animate-pulse' : running ? 'bg-green-500 status-dot-running' : 'bg-gray-400 dark:bg-gray-600'}"
            title={toggling ? 'Updating…' : running ? 'Running' : 'Stopped'}
          ></span>
        </div>
        <p class="font-medium text-sm mt-1 ml-6">
          &rarr; {formatRelayTarget(relay)}
        </p>
      </div>

      <!-- Actions -->
      <div class="flex items-center gap-2 ml-6 sm:ml-0">
        <label class="flex items-center gap-1.5 cursor-pointer" title="Start automatically on boot">
          <span class="text-xs text-gray-500 dark:text-gray-400">Auto</span>
          <input
            type="checkbox"
            checked={autostart}
            onchange={(e) => onAutostart('relay', relay.id, e.target.checked)}
            class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 h-3.5 w-3.5 dark:bg-gray-800"
          />
        </label>

        <div class="w-px h-5 bg-gray-200 dark:bg-gray-700"></div>

        <button
          class="p-1.5 rounded-md transition-colors {toggling ? 'text-amber-500 cursor-not-allowed' : running ? 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500' : 'hover:bg-green-50 dark:hover:bg-green-900/20 text-green-600'}"
          onclick={() => onToggle('relay', relay.id, running)}
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
          onclick={() => onEdit('relay', relay)}
          title="Edit"
        >
          <Pencil size={15} />
        </button>

        <button
          class="p-1.5 rounded-md hover:bg-red-50 dark:hover:bg-red-900/20 text-red-500 transition-colors"
          onclick={() => onDelete('relay', relay.id, formatRelayTitle(relay))}
          title="Delete"
        >
          <Trash2 size={15} />
        </button>
      </div>
    </div>
  </div>

{:else}
  {@const proxy = item.proxy}
  {@const running = proxy.running ?? proxy.Running}
  {@const autostart = proxy.autostart ?? false}
  {@const proxyUrl = formatProxyUrl(proxy)}
  {@const tlsError = proxy.tls_error || proxy.TLSError || ''}

  <div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 px-4 py-3">
    <div class="flex flex-col sm:flex-row sm:items-center gap-3">
      <!-- Info -->
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2">
          <ShieldCheck size={16} class="text-blue-500 flex-shrink-0" />
          <a
            href={proxyUrl}
            target="_blank"
            rel="noopener"
            class="font-medium text-sm truncate hover:underline"
          ><span class="text-sm font-normal text-gray-400 dark:text-gray-500">https://{proxy.hostname || fqdn}</span><span>{proxy.listen_port && proxy.listen_port !== 443 ? `:${proxy.listen_port}` : ''}</span></a>
          <span
            class="w-2 h-2 rounded-full flex-shrink-0 {toggling ? 'bg-amber-400 animate-pulse' : running ? 'bg-green-500 status-dot-running' : 'bg-gray-400 dark:bg-gray-600'}"
            title={toggling ? 'Updating…' : running ? 'Running' : 'Stopped'}
          ></span>
          {#if tlsError}
            <span title={tlsError} class="flex-shrink-0 text-amber-500 dark:text-amber-400 cursor-help">
              <AlertTriangle size={14} />
            </span>
          {/if}
        </div>
        <p class="font-medium text-sm mt-1 ml-6">
          &rarr; {proxy.target_host}:{proxy.target_port}
        </p>
        {#if tlsError}
          <p class="text-xs text-amber-600 dark:text-amber-400 mt-1 ml-6 flex items-start gap-1">
            <AlertTriangle size={11} class="mt-0.5 flex-shrink-0" />
            <span>TLS cert issue: {tlsError}</span>
          </p>
        {/if}
      </div>

      <!-- Actions -->
      <div class="flex items-center gap-2 ml-6 sm:ml-0">
        <label class="flex items-center gap-1.5 cursor-pointer" title="Start automatically on boot">
          <span class="text-xs text-gray-500 dark:text-gray-400">Auto</span>
          <input
            type="checkbox"
            checked={autostart}
            onchange={(e) => onAutostart('proxy', proxy.id, e.target.checked)}
            class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 h-3.5 w-3.5 dark:bg-gray-800"
          />
        </label>

        <div class="w-px h-5 bg-gray-200 dark:bg-gray-700"></div>

        <button
          class="p-1.5 rounded-md transition-colors {toggling ? 'text-amber-500 cursor-not-allowed' : running ? 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500' : 'hover:bg-green-50 dark:hover:bg-green-900/20 text-green-600'}"
          onclick={() => onToggle('proxy', proxy.id, proxy.enabled)}
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
          onclick={() => onEdit('proxy', proxy)}
          title="Edit"
        >
          <Pencil size={15} />
        </button>

        <button
          class="p-1.5 rounded-md hover:bg-red-50 dark:hover:bg-red-900/20 text-red-500 transition-colors"
          onclick={() => onDelete('proxy', proxy.id, proxyUrl)}
          title="Delete"
        >
          <Trash2 size={15} />
        </button>
      </div>
    </div>
  </div>
{/if}
