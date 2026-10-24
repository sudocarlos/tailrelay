<script>
  import CopyButton from './CopyButton.svelte';
  import { Popover } from '@/lib/components/ui/popover';
  import { PopoverContent } from '@/lib/components/ui/popover';
  import { PopoverTrigger } from '@/lib/components/ui/popover';
  import { ChevronDown } from '@lucide/svelte';

  /**
   * Per-peer address dropdown for the Tailscale Machines table.
   * Rendered as a hover-style Popover listing the peer's DNS name and
   * tailnet IPs with copy actions.
   */
  let { peer } = $props();

  let open = $state(false);
  let leaveTimer = null;

  // Grace period before closing on mouseleave so the pointer can cross the
  // gap between the trigger and the content without dismissing it.
  const LEAVE_DELAY = 120;

  function cancelLeave() {
    if (leaveTimer !== null) {
      clearTimeout(leaveTimer);
      leaveTimer = null;
    }
  }

  function scheduleLeave() {
    cancelLeave();
    leaveTimer = setTimeout(() => {
      leaveTimer = null;
      open = false;
    }, LEAVE_DELAY);
  }
</script>

<Popover bind:open>
  <PopoverTrigger
    class="inline-flex cursor-pointer items-center gap-1 rounded-md px-1 py-0.5 text-muted-foreground transition-colors outline-none hover:text-foreground"
    onclick={(e) => e.stopPropagation()}
    onmouseenter={cancelLeave}
    onmouseleave={scheduleLeave}
  >
    {peer.IPv4 || peer.IPv6 || '—'}
    <ChevronDown size={12} class="opacity-60" />
  </PopoverTrigger>
  <PopoverContent
    class="w-max min-w-52 p-1"
    align="start"
    sideOffset={4}
    onmouseenter={cancelLeave}
    onmouseleave={scheduleLeave}
    onpointerdown={(e) => e.stopPropagation()}
  >
    {#if peer.DNSName}
      <div class="flex items-center justify-between gap-3 rounded-md px-2 py-1.5 text-xs transition-colors hover:bg-accent">
        <span class="truncate">{peer.DNSName}</span>
        <CopyButton text={peer.DNSName} size={14} label="Copy DNS name" class="ml-2 shrink-0" />
      </div>
    {/if}
    {#if peer.TailscaleIPs && peer.TailscaleIPs.length > 0}
      {#each peer.TailscaleIPs as ip}
        <div class="flex items-center justify-between gap-3 rounded-md px-2 py-1.5 text-xs transition-colors hover:bg-accent">
          <span>{ip}</span>
          <CopyButton text={ip} size={14} label="Copy IP address" class="ml-2 shrink-0" />
        </div>
      {/each}
    {/if}
  </PopoverContent>
</Popover>
