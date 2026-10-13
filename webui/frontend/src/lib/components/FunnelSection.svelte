<script>
  import { Globe, Ban, Plus } from '@lucide/svelte';
  import RelayIcon from './RelayIcon.svelte';
  import { FUNNEL_PORTS } from '../stores/app.js';
  import { Switch } from '@/lib/components/ui/switch';
  import ItemMenu from './ItemMenu.svelte';
  import CopyButton from './CopyButton.svelte';
  import { statusIconClass } from '../utils/statusBadge.js';
  import { splitHost } from '../utils/hostname.js';

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

<!-- Section header -->
<div class="mb-3">
  <h2 class="text-lg font-semibold">Funnel</h2>
  <p class="text-sm text-muted-foreground mt-0.5">
    {funnels.length} of {FUNNEL_PORTS.length} configured &middot; expose services to the public internet
  </p>
</div>

{#if funnels.length === 0 && usedFunnelPorts.size === 0}
  <!-- No funnels at all: single configure prompt -->
  <div class="mb-6">
    <table class="w-full">
      <tbody>
        <tr class="transition-colors hover:bg-muted/40">
          <td class="py-3">
            <button
              type="button"
              class="flex items-center gap-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
              onclick={() => onConfigure(FUNNEL_PORTS[0])}
            >
              <Plus size={16} class="flex-shrink-0" />
              <span>Configure funnel on port {FUNNEL_PORTS[0]}</span>
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
{:else}
  <div class="mb-6">
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
        {#each FUNNEL_PORTS as port (port)}
          {@const funnel = funnelsByPort.get(port)}
          {@const usedBy = usedFunnelPorts.get(port)}

          {#if funnel}
            {@const running = funnel.running}
            {@const autostart = funnel.autostart ?? false}
            {@const toggling = togglingId === `funnel:${funnel.id}`}
            {@const funnelUrl = formatFunnelUrl(funnel)}
            {@const scheme = funnel.funnel_transport === 'tcp' ? 'tcp' : 'https'}
            {@const portLabel = scheme === 'https' && funnel.listen_port === 443 ? '' : `:${funnel.listen_port}`}
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
                        <span class="shrink-0 font-medium">{portLabel}</span>
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
                  onCheckedChange={() => onToggle(funnel.id, running)}
                  aria-label={running ? 'Stop funnel' : 'Start funnel'}
                />
              </td>
              <td class="w-8 py-2 pr-1 align-middle">
                <ItemMenu
                  {autostart}
                  onAutostartChange={(v) => onAutostart(funnel.id, v)}
                  onEdit={() => onEdit(funnel)}
                  onDelete={() => onDelete(funnel.id, funnelUrl, formatFunnelTarget(funnel), funnel.icon_url, running)}
                />
              </td>
            </tr>
          {:else if usedBy}
            <tr>
              <td class="py-3 pr-4" colspan="4">
                <div class="flex items-center gap-2 text-sm text-muted-foreground/70" title="Port {port} is in use by a serve relay">
                  <Ban size={15} class="shrink-0" />
                  <span>Port {port} is in use by a serve relay</span>
                </div>
              </td>
            </tr>
          {:else}
            <tr class="transition-colors hover:bg-muted/40">
              <td class="py-3" colspan="4">
                <button
                  type="button"
                  class="flex items-center gap-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
                  onclick={() => onConfigure(port)}
                >
                  <Plus size={16} class="flex-shrink-0" />
                  <span>Configure funnel on port {port}</span>
                </button>
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
{/if}
