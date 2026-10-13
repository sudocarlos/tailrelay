<script>
  import { Network, ShieldCheck, Globe, AlertTriangle } from '@lucide/svelte';
  import RelayIcon from './RelayIcon.svelte';
  import { Switch } from '@/lib/components/ui/switch';
  import ItemMenu from './ItemMenu.svelte';
  import CopyButton from './CopyButton.svelte';
  import { statusIconClass } from '../utils/statusBadge.js';
  import { splitHost } from '../utils/hostname.js';

  // A single flat list row (a <tr>) for a serve relay (tcp, https proxy,
  // or funnel), styled after the Tailscale console machines list: icon +
  // address column, target column, and headerless toggle /
  // overflow-menu columns.
  let { item, fqdn, toggling = false, onToggle, onAutostart, onEdit, onDelete } = $props();

  function formatRelayAddress(fqdnName, relay) {
    return `tcp://${fqdnName}:${relay.listen_port}`;
  }

  function formatRelayTarget(relay) {
    return `${relay.target_host}:${relay.target_port}`;
  }

  function formatProxyUrl(proxy) {
    const port = proxy.listen_port && proxy.listen_port !== 443 ? `:${proxy.listen_port}` : '';
    return `${proxy.listener_scheme || 'https'}://${proxy.hostname || fqdn}${port}`;
  }

  function funnelScheme(funnel) {
    return funnel.funnel_transport === 'tcp' ? 'tcp' : 'https';
  }

  function funnelPortLabel(funnel) {
    return funnelScheme(funnel) === 'https' && funnel.listen_port === 443
      ? ''
      : `:${funnel.listen_port}`;
  }

  function formatFunnelUrl(funnel) {
    return `${funnelScheme(funnel)}://${funnel.hostname || fqdn}${funnelPortLabel(funnel)}`;
  }

  function formatFunnelTarget(funnel) {
    return `${funnel.target_host}:${funnel.target_port}`;
  }
</script>

{#if item.type === 'relay'}
  {@const relay = item.relay}
  {@const running = item.running}
  {@const autostart = relay.autostart ?? false}
  {@const address = formatRelayAddress(fqdn || 'unknown', relay)}
  {@const host = splitHost(fqdn || 'unknown')}
  <tr class="transition-colors hover:bg-muted/40">
    <td class="py-3 pr-4">
      <div class="flex min-w-0 items-center gap-3">
        <RelayIcon size="sm" iconUrl={relay.icon_url} {toggling} {running} alt={address}>
          <Network size={18} strokeWidth={2.5} class={statusIconClass(toggling, running)} />
        </RelayIcon>
        <div class="min-w-0">
          <div class="flex min-w-0 items-center gap-0.5">
            <span class="flex min-w-0 items-baseline text-sm">
              <span class="shrink-0 font-normal text-muted-foreground">tcp://</span>
              <span class="min-w-0 truncate font-normal text-muted-foreground">{host.short}{#if host.suffix}<span class="hidden sm:inline">{host.suffix}</span>{/if}</span>
              <span class="shrink-0 font-medium">:{relay.listen_port}</span>
            </span>
            <CopyButton text={address} size={14} />
          </div>
          <p class="text-xs text-muted-foreground truncate sm:hidden">&rarr; {formatRelayTarget(relay)}</p>
        </div>
      </div>
    </td>
    <td class="hidden py-3 pr-4 sm:table-cell">
      <p class="truncate text-sm">&rarr; {formatRelayTarget(relay)}</p>
    </td>
    <td class="w-12 py-2.5 pr-2 align-middle">
      <Switch
        checked={running}
        disabled={toggling}
        onCheckedChange={() => onToggle('relay', relay.id, running)}
        aria-label={running ? 'Stop relay' : 'Start relay'}
      />
    </td>
    <td class="w-9 py-2 pr-1 align-middle">
      <ItemMenu
        {autostart}
        onAutostartChange={(v) => onAutostart('relay', relay.id, v)}
        onEdit={() => onEdit('relay', relay)}
        onDelete={() => onDelete('relay', relay.id, formatRelayAddress(fqdn || 'unknown', relay), formatRelayTarget(relay), relay.icon_url, running)}
      />
    </td>
  </tr>
{:else if item.type === 'funnel'}
  {@const funnel = item.funnel}
  {@const running = item.running}
  {@const autostart = funnel.autostart ?? false}
  {@const funnelUrl = formatFunnelUrl(funnel)}
  {@const scheme = funnelScheme(funnel)}
  {@const host = splitHost(funnel.hostname || fqdn)}
  <tr class="transition-colors hover:bg-muted/40">
    <td class="py-3 pr-4">
      <div class="flex min-w-0 items-center gap-3">
        <RelayIcon size="sm" iconUrl={funnel.icon_url} {toggling} {running} alt={funnelUrl}>
          <Globe size={18} strokeWidth={2.5} class={statusIconClass(toggling, running)} />
        </RelayIcon>
        <div class="min-w-0">
          <div class="flex min-w-0 items-center gap-0.5">
            <a
              href={funnelUrl}
              target="_blank"
              rel="noopener"
              class="flex min-w-0 items-baseline text-sm hover:underline"
            >
              <span class="shrink-0 font-normal text-muted-foreground">{scheme}://</span>
              <span class="min-w-0 truncate font-normal text-muted-foreground">{host.short}{#if host.suffix}<span class="hidden sm:inline">{host.suffix}</span>{/if}</span>
              <span class="shrink-0 font-medium">{funnelPortLabel(funnel)}</span>
            </a>
            <CopyButton text={funnelUrl} size={14} />
          </div>
          <p class="text-xs text-muted-foreground truncate sm:hidden">&rarr; {formatFunnelTarget(funnel)}</p>
        </div>
      </div>
    </td>
    <td class="hidden py-3 pr-4 sm:table-cell">
      <p class="truncate text-sm">&rarr; {formatFunnelTarget(funnel)}</p>
    </td>
    <td class="w-12 py-2.5 pr-2 align-middle">
      <Switch
        checked={running}
        disabled={toggling}
        onCheckedChange={() => onToggle('funnel', funnel.id, running)}
        aria-label={running ? 'Stop funnel' : 'Start funnel'}
      />
    </td>
    <td class="w-9 py-2 pr-1 align-middle">
      <ItemMenu
        {autostart}
        onAutostartChange={(v) => onAutostart('funnel', funnel.id, v)}
        onEdit={() => onEdit('funnel', funnel)}
        onDelete={() => onDelete('funnel', funnel.id, funnelUrl, formatFunnelTarget(funnel), funnel.icon_url, running)}
      />
    </td>
  </tr>
{:else}
  {@const proxy = item.proxy}
  {@const running = proxy.running ?? proxy.Running}
  {@const autostart = proxy.autostart ?? false}
  {@const proxyUrl = formatProxyUrl(proxy)}
  {@const listenerScheme = proxy.listener_scheme || 'https'}
  {@const tlsError = proxy.tls_error || proxy.TLSError || ''}
  {@const host = splitHost(proxy.hostname || fqdn)}
  <tr class="transition-colors hover:bg-muted/40">
    <td class="py-3 pr-4">
      <div class="flex min-w-0 items-center gap-3">
        <RelayIcon size="sm" iconUrl={proxy.icon_url} {toggling} {running} alt={proxyUrl}>
          <ShieldCheck size={18} strokeWidth={2.5} class={statusIconClass(toggling, running)} />
        </RelayIcon>
        <div class="min-w-0">
          <div class="flex min-w-0 items-center gap-0.5">
            <a
              href={proxyUrl}
              target="_blank"
              rel="noopener"
              class="flex min-w-0 items-baseline text-sm hover:underline"
            >
              <span class="shrink-0 font-normal text-muted-foreground">{listenerScheme}://</span>
              <span class="min-w-0 truncate font-normal text-muted-foreground">{host.short}{#if host.suffix}<span class="hidden sm:inline">{host.suffix}</span>{/if}</span>
              <span class="shrink-0 font-medium">{proxy.listen_port && proxy.listen_port !== 443 ? `:${proxy.listen_port}` : ''}</span>
            </a>
            <CopyButton text={proxyUrl} size={14} />
            {#if tlsError}
              <span title={tlsError} class="shrink-0 cursor-help text-amber-500 dark:text-amber-400">
                <AlertTriangle size={14} />
              </span>
            {/if}
          </div>
          <p class="text-xs text-muted-foreground truncate sm:hidden">&rarr; {proxy.target_host}:{proxy.target_port}</p>
        </div>
      </div>
    </td>
    <td class="hidden py-3 pr-4 sm:table-cell">
      <p class="truncate text-sm">&rarr; {proxy.target_host}:{proxy.target_port}</p>
      {#if tlsError}
        <p class="truncate text-xs text-amber-600 dark:text-amber-400" title={tlsError}>TLS issue: {tlsError}</p>
      {/if}
    </td>
    <td class="w-12 py-2.5 pr-2 align-middle">
      <Switch
        checked={running}
        disabled={toggling}
        onCheckedChange={() => onToggle('proxy', proxy.id, running)}
        aria-label={running ? 'Stop proxy' : 'Start proxy'}
      />
    </td>
    <td class="w-9 py-2 pr-1 align-middle">
      <ItemMenu
        {autostart}
        onAutostartChange={(v) => onAutostart('proxy', proxy.id, v)}
        onEdit={() => onEdit('proxy', proxy)}
        onDelete={() => onDelete('proxy', proxy.id, proxyUrl, `${proxy.target_host}:${proxy.target_port}`, proxy.icon_url, running)}
      />
    </td>
  </tr>
{/if}
