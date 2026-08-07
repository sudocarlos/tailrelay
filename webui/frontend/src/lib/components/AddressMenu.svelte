<script>
  import { tick, onDestroy } from 'svelte';
  import { portal } from '../actions/portal.js';
  import CopyButton from './CopyButton.svelte';

  /**
   * Per-peer address dropdown for the Tailscale Machines table.
   *
   * Replaces a native `<details>` popover (which macOS WebKit does not
   * reliably toggle for flex-styled summaries — see issue #98) with the
   * hardened portal pattern shared by Tooltip.svelte and ItemMenu.svelte:
   * the menu is portaled to document.body, fixed-positioned beneath the
   * trigger, and dismissed by a `document`-level outside-click + Escape
   * listener that only runs while open.
   *
   * Props:
   *   peer — the peer object (IPv4, IPv6, DNSName, TailscaleIPs)
   */
  let { peer } = $props();

  let open = $state(false);
  let triggerEl = $state(null);
  let menuEl = $state(null);
  let style = $state('');

  const GAP = 4;

  function clamp(val, min, max) {
    return Math.max(min, Math.min(max, val));
  }

  function computePosition(triggerRect, menuRect) {
    const r = triggerRect;
    const m = menuRect;
    let top = r.bottom + GAP;
    // Align the menu's left edge with the trigger so long hostnames grow
    // rightward instead of overflowing the table to the left.
    let left = r.left;

    if (top + m.height > window.innerHeight) {
      top = r.top - m.height - GAP;
    }
    top = clamp(top, 0, window.innerHeight - m.height);
    left = clamp(left, 0, window.innerWidth - m.width);

    return `position:fixed; top:${top}px; left:${left}px; z-index:9999;`;
  }

  async function show() {
    const r = triggerEl.getBoundingClientRect();
    // Start invisible; positioning is computed after mount to measure the
    // portaled menu, which has escaped any table overflow clipping.
    style = `position:fixed; top:${r.bottom + GAP}px; left:${r.left}px; z-index:9999; opacity:0; pointer-events:none;`;
    open = true;

    await tick();
    if (!menuEl) return;
    style = computePosition(r, menuEl.getBoundingClientRect());
  }

  function hide() {
    open = false;
  }

  function toggle(e) {
    e.stopPropagation();
    if (open) hide(); else show();
  }

  function reposition() {
    if (!open || !triggerEl || !menuEl) return;
    style = computePosition(triggerEl.getBoundingClientRect(), menuEl.getBoundingClientRect());
  }

  $effect(() => {
    if (open) {
      window.addEventListener('scroll', reposition, true);
      window.addEventListener('resize', reposition);
    } else {
      window.removeEventListener('scroll', reposition, true);
      window.removeEventListener('resize', reposition);
    }
  });

  // Dismiss on outside click. This is the primary close mechanism; clicking
  // the trigger toggles via stopPropagated onclick above instead.
  $effect(() => {
    if (!open) return;
    function onDocClick(e) {
      if (!triggerEl?.contains(e.target) && !menuEl?.contains(e.target)) hide();
    }
    document.addEventListener('click', onDocClick);
    return () => document.removeEventListener('click', onDocClick);
  });

  // Escape closes and returns focus to the trigger.
  $effect(() => {
    if (!open) return;
    function onKeydown(e) {
      if (e.key === 'Escape') {
        e.preventDefault();
        hide();
        triggerEl?.focus();
      }
    }
    document.addEventListener('keydown', onKeydown);
    return () => document.removeEventListener('keydown', onKeydown);
  });

  onDestroy(() => {
    window.removeEventListener('scroll', reposition, true);
    window.removeEventListener('resize', reposition);
  });
</script>

<button
  bind:this={triggerEl}
  type="button"
  aria-haspopup="menu"
  aria-expanded={open}
  class="inline-flex items-center gap-1 outline-none hover:text-gray-900 dark:hover:text-gray-100 cursor-pointer"
  onclick={toggle}
>
  {peer.IPv4 || peer.IPv6 || '—'}
  <svg class="w-3 h-3 text-gray-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"></polyline></svg>
</button>

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    bind:this={menuEl}
    {style}
    role="menu"
    use:portal
    onkeydown={() => {}}
    class="w-max min-w-[200px] rounded-md shadow-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-1"
  >
    {#if peer.DNSName}
      <div class="flex items-center justify-between px-3 py-2 text-xs hover:bg-gray-50 dark:hover:bg-gray-700 rounded transition-colors">
        <span class="truncate">{peer.DNSName}</span>
        <CopyButton text={peer.DNSName} size={14} label="Copy DNS name" class="ml-4" />
      </div>
    {/if}
    {#if peer.TailscaleIPs && peer.TailscaleIPs.length > 0}
      {#each peer.TailscaleIPs as ip}
        <div class="flex items-center justify-between px-3 py-2 text-xs hover:bg-gray-50 dark:hover:bg-gray-700 rounded transition-colors border-t border-gray-100 dark:border-gray-700/50">
          <span>{ip}</span>
          <CopyButton text={ip} size={14} label="Copy IP address" class="ml-4" />
        </div>
      {/each}
    {/if}
  </div>
{/if}