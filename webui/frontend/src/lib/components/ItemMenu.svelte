<script>
  import { onDestroy } from 'svelte';
  import { MoreHorizontal, Pencil, Trash2 } from '@lucide/svelte';
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

  function computeStyle() {
    if (!triggerEl) return;
    const r = triggerEl.getBoundingClientRect();
    style = `position:fixed; top:${r.bottom + 4}px; right:${window.innerWidth - r.right}px; z-index:9999;`;
  }

  function show() {
    computeStyle();
    open = true;
  }

  function hide() {
    open = false;
  }

  function toggleMenu() {
    if (open) hide(); else show();
  }

  function reposition() {
    if (open) computeStyle();
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
    class="min-w-[200px] rounded-lg bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 shadow-xl py-1 text-sm"
  >
    <button
      type="button"
      role="menuitem"
      class="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-200 transition-colors"
      onclick={selectEdit}
    >
      <Pencil size={14} />
      Edit
    </button>

    <div class="flex items-center justify-between gap-3 px-3 py-2">
      <span class="text-gray-700 dark:text-gray-200">Start on boot</span>
      <Toggle checked={autostart} onChange={onAutostartChange} label="Start on boot" />
    </div>

    <hr class="border-gray-100 dark:border-gray-800 my-1" />

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
