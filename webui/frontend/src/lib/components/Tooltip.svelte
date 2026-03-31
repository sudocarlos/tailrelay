<script>
  import { onDestroy } from 'svelte';

  /**
   * Portal-based tooltip that mounts directly on document.body,
   * escaping any overflow:hidden / scroll container clipping.
   *
   * Props:
   *   width     — Tailwind width class for the tooltip box (default 'w-72')
   *   trigger   — snippet rendered as the hoverable icon
   *   children  — snippet rendered as the tooltip body content
   */

  let { width = 'w-72', trigger, children } = $props();

  let visible = $state(false);
  let triggerEl = $state(null);
  let tooltipEl = $state(null);
  let style = $state('');

  function computeStyle() {
    if (!triggerEl) return;
    const r = triggerEl.getBoundingClientRect();
    const centerX = r.left + r.width / 2;
    const bottomFromViewport = window.innerHeight - r.top + 8;
    style = `position:fixed; bottom:${bottomFromViewport}px; left:${centerX}px; transform:translateX(-50%); z-index:9999;`;
  }

  function show() {
    computeStyle();
    visible = true;
  }

  function hide() {
    visible = false;
  }

  function toggle(e) {
    e.stopPropagation();
    if (visible) hide(); else show();
  }

  function reposition() {
    if (visible) computeStyle();
  }

  $effect(() => {
    if (visible) {
      window.addEventListener('scroll', reposition, true);
      window.addEventListener('resize', reposition);
    } else {
      window.removeEventListener('scroll', reposition, true);
      window.removeEventListener('resize', reposition);
    }
  });

  // Dismiss when the user clicks/taps outside both the trigger and the popup.
  // This is the primary close mechanism on touch devices.
  $effect(() => {
    if (!visible) return;
    function onDocClick(e) {
      if (!triggerEl?.contains(e.target) && !tooltipEl?.contains(e.target)) {
        hide();
      }
    }
    document.addEventListener('click', onDocClick);
    return () => document.removeEventListener('click', onDocClick);
  });

  onDestroy(() => {
    window.removeEventListener('scroll', reposition, true);
    window.removeEventListener('resize', reposition);
  });

  // Svelte action: moves element to document.body on mount, removes on destroy
  function portal(node) {
    document.body.appendChild(node);
    return {
      destroy() {
        if (node.parentNode) node.parentNode.removeChild(node);
      }
    };
  }
</script>

<!-- Trigger wrapper — p-2 / -m-2 expands the tap target to ~42×42 px
     without affecting surrounding layout -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<span
  bind:this={triggerEl}
  class="relative inline-flex cursor-help p-2 -m-2"
  onmouseenter={show}
  onmouseleave={hide}
  onclick={toggle}
  onkeydown={() => {}}
>
  {@render trigger()}
</span>

{#if visible}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    bind:this={tooltipEl}
    {style}
    class="{width} rounded-lg bg-gray-900 dark:bg-gray-700 text-white text-xs px-3 py-2 shadow-xl relative"
    use:portal
    onmouseenter={show}
    onmouseleave={hide}
  >
    {@render children()}
    <!-- Downward-pointing caret -->
    <div class="absolute left-1/2 -translate-x-1/2 top-full w-0 h-0 border-x-4 border-x-transparent border-t-4 border-t-gray-900 dark:border-t-gray-700"></div>
    <!-- Transparent spacer that bridges the 8 px gap between the popup bottom
         and the trigger icon, preventing mouseleave from firing mid-transit -->
    <div class="absolute left-0 right-0 -bottom-2 h-2"></div>
  </div>
{/if}
