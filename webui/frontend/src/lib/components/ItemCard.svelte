<script>
  import { Network, ShieldCheck, AlertTriangle } from '@lucide/svelte';
  import Toggle from './Toggle.svelte';
  import ItemMenu from './ItemMenu.svelte';

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

  // The type icon (Network/ShieldCheck) doubles as the status indicator:
  // neutral background + accent icon when stopped, colored background +
  // white icon when running or mid-toggle (replacing the old status dot).
  function statusBadgeClass(isToggling, isRunning) {
    if (isToggling) return 'bg-amber-400 animate-pulse';
    return isRunning ? 'bg-green-500 status-dot-running' : 'bg-gray-100 dark:bg-gray-800';
  }

  function statusIconClass(isToggling, isRunning) {
    return isToggling || isRunning ? 'text-white' : 'text-blue-500';
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
          <span
            class="flex items-center justify-center w-6 h-6 rounded-full flex-shrink-0 transition-colors {statusBadgeClass(toggling, running)}"
            title={toggling ? 'Updating…' : running ? 'Running' : 'Stopped'}
          >
            <Network size={14} class={statusIconClass(toggling, running)} />
          </span>
          <span class="font-medium text-sm truncate"><span class="text-sm font-normal text-gray-400 dark:text-gray-500">tcp://{fqdn || 'unknown'}</span><span>:{relay.listen_port}</span></span>
        </div>
        <p class="font-medium text-sm mt-1 ml-8">
          &rarr; {formatRelayTarget(relay)}
        </p>
      </div>

      <!-- Actions -->
      <div class="flex items-center gap-1 ml-8 sm:ml-0">
        <Toggle
          checked={running}
          disabled={toggling}
          onChange={() => onToggle('relay', relay.id, running)}
          label={running ? 'Stop relay' : 'Start relay'}
        />
        <ItemMenu
          {autostart}
          onAutostartChange={(v) => onAutostart('relay', relay.id, v)}
          onEdit={() => onEdit('relay', relay)}
          onDelete={() => onDelete('relay', relay.id, formatRelayTitle(relay), formatRelayTarget(relay))}
        />
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
          <span
            class="flex items-center justify-center w-6 h-6 rounded-full flex-shrink-0 transition-colors {statusBadgeClass(toggling, running)}"
            title={toggling ? 'Updating…' : running ? 'Running' : 'Stopped'}
          >
            <ShieldCheck size={14} class={statusIconClass(toggling, running)} />
          </span>
          <a
            href={proxyUrl}
            target="_blank"
            rel="noopener"
            class="font-medium text-sm truncate hover:underline"
          ><span class="text-sm font-normal text-gray-400 dark:text-gray-500">https://{proxy.hostname || fqdn}</span><span>{proxy.listen_port && proxy.listen_port !== 443 ? `:${proxy.listen_port}` : ''}</span></a>
          {#if tlsError}
            <span title={tlsError} class="flex-shrink-0 text-amber-500 dark:text-amber-400 cursor-help">
              <AlertTriangle size={14} />
            </span>
          {/if}
        </div>
        <p class="font-medium text-sm mt-1 ml-8">
          &rarr; {proxy.target_host}:{proxy.target_port}
        </p>
        {#if tlsError}
          <p class="text-xs text-amber-600 dark:text-amber-400 mt-1 ml-8 flex items-start gap-1">
            <AlertTriangle size={11} class="mt-0.5 flex-shrink-0" />
            <span>TLS cert issue: {tlsError}</span>
          </p>
        {/if}
      </div>

      <!-- Actions -->
      <div class="flex items-center gap-1 ml-8 sm:ml-0">
        <Toggle
          checked={running}
          disabled={toggling}
          onChange={() => onToggle('proxy', proxy.id, proxy.enabled)}
          label={running ? 'Stop proxy' : 'Start proxy'}
        />
        <ItemMenu
          {autostart}
          onAutostartChange={(v) => onAutostart('proxy', proxy.id, v)}
          onEdit={() => onEdit('proxy', proxy)}
          onDelete={() => onDelete('proxy', proxy.id, proxyUrl, `${proxy.target_host}:${proxy.target_port}`)}
        />
      </div>
    </div>
  </div>
{/if}
