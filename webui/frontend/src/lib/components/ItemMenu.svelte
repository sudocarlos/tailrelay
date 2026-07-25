<script>
  import { tick, onDestroy } from 'svelte';
  import { MoreHorizontal, SquarePen, SquarePower, Trash2 } from '@lucide/svelte';
  import { portal } from '../actions/portal.js';
  import Toggle from './Toggle.svelte';

  /**
   * "..." overflow menu for a relay/proxy/funnel card.
   * Portal-based dropdown, mirroring Tooltip.svelte's positioning and
   * outside-click/reposition behavior.
   *
   * Props:
   *   autostart          — current "start on boot" state
   *   onAutostartChange  — called with the new boolean when the toggle flips
   *   onEdit             — called when "Edit" is selected
   *   onDelete           — called when "Delete" is selected
   */
  let { autostart = false, onAutostartChange, onEdit, onDelete } = $props();

  let open = $state(false);
  let triggerEl = $state(null);
  let menuEl = $state(null);
  let style = $state('');
  let activeIndex = $state(-1);

  const GAP = 4;

  function clamp(val, min, max) {
    return Math.max(min, Math.min(max, val));
  }

  /**
   * Compute the fixed position for the dropdown so it stays within the
   * viewport. Uses `left`-based positioning throughout to avoid the
   * right→left flash the old implementation had.
   */
  function computePosition(triggerRect, menuRect) {
    const r = triggerRect;
    const m = menuRect;
    let top = r.bottom + GAP;
    let left = r.right - m.width;

    if (top + m.height > window.innerHeight) {
      top = r.top - m.height - GAP;
    }
    top = clamp(top, 0, window.innerHeight - m.height);
    left = clamp(left, 0, window.innerWidth - m.width);

    return `position:fixed; top:${top}px; left:${left}px; z-index:9999;`;
  }

  // The list of focusable menu-item indices (skips the toggle row at index 1).
  const menuItems = [0, 2];

  function focusItem(index) {
    if (!menuEl) return;
    const items = menuEl.querySelectorAll('[role="menuitem"]');
    const target = menuItems.indexOf(index);
    if (target >= 0 && items[target]) items[target].focus();
  }

  async function show() {
    const r = triggerEl.getBoundingClientRect();
    // Start invisible; positioning will be computed after mount.
    style = `position:fixed; top:${r.bottom + GAP}px; left:${r.right}px; z-index:9999; opacity:0; pointer-events:none;`;
    open = true;
    activeIndex = -1;

    await tick();
    if (!menuEl) return;
    style = computePosition(r, menuEl.getBoundingClientRect());
  }

  function hide() {
    open = false;
    activeIndex = -1;
  }

  function toggleMenu() {
    if (open) hide(); else show();
  }

  function reposition() {
    if (!open || !triggerEl || !menuEl) return;
    style = computePosition(triggerEl.getBoundingClientRect(), menuEl.getBoundingClientRect());
  }

  function selectEdit() {
    hide();
    onEdit?.();
  }

  function selectDelete() {
    hide();
    onDelete?.();
  }

  function onKeydown(e) {
    if (!open) return;

    if (e.key === 'Escape') {
      hide();
      return;
    }

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      const next = menuItems[(menuItems.indexOf(activeIndex) + 1) % menuItems.length];
      activeIndex = next;
      focusItem(next);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      const prev = menuItems[(menuItems.indexOf(activeIndex) - 1 + menuItems.length) % menuItems.length];
      activeIndex = prev;
      focusItem(prev);
    } else if (e.key === 'Home') {
      e.preventDefault();
      activeIndex = menuItems[0];
      focusItem(activeIndex);
    } else if (e.key === 'End') {
      e.preventDefault();
      activeIndex = menuItems[menuItems.length - 1];
      focusItem(activeIndex);
    }
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

  // Dismiss on outside click.
  $effect(() => {
    if (!open) return;
    function onDocClick(e) {
      if (!triggerEl?.contains(e.target) && !menuEl?.contains(e.target)) hide();
    }
    document.addEventListener('click', onDocClick);
    return () => document.removeEventListener('click', onDocClick);
  });

  onDestroy(() => {
    window.removeEventListener('scroll', reposition, true);
    window.removeEventListener('resize', reposition);
  });
</script>

<svelte:window on:keydown={onKeydown} />

<button
  bind:this={triggerEl}
  type="button"
  aria-haspopup="menu"
  aria-expanded={open}
  aria-label="More actions"
  class="p-1 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500 transition-colors"
  onclick={toggleMenu}
>
  <MoreHorizontal size={11} />
</button>

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    bind:this={menuEl}
    {style}
    role="menu"
    use:portal
    class="min-w-[200px] rounded-md bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 shadow-lg p-1 text-sm"
  >
    <button
      type="button"
      role="menuitem"
      class="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors focus:outline-none focus:bg-gray-100 dark:focus:bg-gray-700"
      onclick={selectEdit}
    >
      <SquarePen size={14} />
      Edit
    </button>

    <div class="flex items-center justify-between gap-3 px-3 py-2">
      <span class="flex items-center gap-2 text-gray-700 dark:text-gray-300"><SquarePower size={14} />Start on boot</span>
      <Toggle checked={autostart} onChange={onAutostartChange} label="Start on boot" />
    </div>

    <hr class="border-gray-100 dark:border-gray-700 my-1" />

    <button
      type="button"
      role="menuitem"
      class="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-red-50 dark:hover:bg-red-900/20 text-red-500 transition-colors focus:outline-none focus:bg-red-50 dark:focus:bg-red-900/20"
      onclick={selectDelete}
    >
      <Trash2 size={14} />
      Delete
    </button>
  </div>
{/if}
