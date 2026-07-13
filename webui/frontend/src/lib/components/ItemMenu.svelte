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

  function clamp(val, min, max) {
    return Math.max(min, Math.min(max, val));
  }

  async function show() {
    // Initial position: below the trigger, right-aligned.
    const r = triggerEl.getBoundingClientRect();
    style = `position:fixed; top:${r.bottom + 4}px; right:${window.innerWidth - r.right}px; z-index:9999;`;
    open = true;

    // Wait for the menu to mount, then clamp to viewport.
    await tick();
    if (!menuEl) return;
    const m = menuEl.getBoundingClientRect();
    const gap = 4;
    let top = r.bottom + gap;
    let left = r.right - m.width;

    // Vertical: flip above if it overflows the bottom.
    if (top + m.height > window.innerHeight) {
      top = r.top - m.height - gap;
    }
    // Clamp vertical.
    top = clamp(top, 0, window.innerHeight - m.height);

    // Horizontal: clamp so the menu stays fully visible.
    left = clamp(left, 0, window.innerWidth - m.width);

    style = `position:fixed; top:${top}px; left:${left}px; z-index:9999;`;
  }

  function hide() {
    open = false;
  }

  function toggleMenu() {
    if (open) hide(); else show();
  }

  function reposition() {
    if (!open || !triggerEl || !menuEl) return;
    const r = triggerEl.getBoundingClientRect();
    const m = menuEl.getBoundingClientRect();
    const gap = 4;
    let top = r.bottom + gap;
    let left = r.right - m.width;

    if (top + m.height > window.innerHeight) {
      top = r.top - m.height - gap;
    }
    top = clamp(top, 0, window.innerHeight - m.height);
    left = clamp(left, 0, window.innerWidth - m.width);

    style = `position:fixed; top:${top}px; left:${left}px; z-index:9999;`;
  }

  function selectEdit() {
    hide();
    onEdit?.();
  }

  function selectDelete() {
    hide();
    onDelete?.();
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

  // Dismiss on outside click or Escape.
  $effect(() => {
    if (!open) return;
    function onDocClick(e) {
      if (!triggerEl?.contains(e.target) && !menuEl?.contains(e.target)) hide();
    }
    function onKeydown(e) {
      if (e.key === 'Escape') hide();
    }
    document.addEventListener('click', onDocClick);
    document.addEventListener('keydown', onKeydown);
    return () => {
      document.removeEventListener('click', onDocClick);
      document.removeEventListener('keydown', onKeydown);
    };
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
  aria-label="More actions"
  class="p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500 transition-colors"
  onclick={toggleMenu}
>
  <MoreHorizontal size={15} />
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
      class="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors"
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
      class="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-red-50 dark:hover:bg-red-900/20 text-red-500 transition-colors"
      onclick={selectDelete}
    >
      <Trash2 size={14} />
      Delete
    </button>
  </div>
{/if}
