<script>
  import { statusBadgeClass, statusIconClass } from '../utils/statusBadge.js';

  // RelayIcon renders either the default type glyph inside the colored
  // status badge, or — when iconUrl is set — a rounded-square app icon
  // with a small corner status dot. The icon keeps using its own
  // transparency/round shape; the square frame is translucent with a
  // 1px border so transparent and round icons still read on light/dark.
  let {
    iconUrl = '',
    toggling = false,
    running = false,
    alt = 'relay icon',
    href = '',
    children,
  } = $props();

  // Fall back to the default glyph if the image fails to load (offline
  // target, 404 favicon, blocked mixed content). Stays false until a
  // load attempt so we don't flash the glyph unnecessarily.
  let imgError = $state(false);

  // Re-arm the fallback when the URL changes (e.g. editing a relay).
  $effect(() => {
    iconUrl;
    imgError = false;
  });

  const hasIcon = $derived(!!iconUrl && iconUrl.trim() !== '' && !imgError);

  // Corner-dot colors mirror the badge background in statusBadgeClass.
  const dotColor = $derived(
    toggling ? 'bg-amber-400' : running ? 'bg-green-700' : 'bg-gray-300 dark:bg-gray-600'
  );
  const dotTitle = $derived((toggling ? 'Updating' : running ? 'Running' : 'Stopped') + '…');
  const cornerRound = $derived(hasIcon ? 'rounded-xl' : 'rounded-full');
</script>

{#snippet icon()}
  {#if hasIcon}
    <span class="relative flex-shrink-0 w-11 h-11" title={dotTitle}>
      <span
        class="flex items-center justify-center w-11 h-11 rounded-xl overflow-hidden border border-gray-200 dark:border-gray-700 bg-gray-50/60 dark:bg-gray-800/60"
      >
        <img
          src={iconUrl}
          {alt}
          loading="lazy"
          referrerpolicy="no-referrer"
          class="w-full h-full object-contain"
          onerror={() => (imgError = true)}
        />
      </span>
      <!-- Corner status dot (bottom-right) keeps state visible without
           recoloring the icon frame. -->
      <span
        class="absolute -bottom-1 -right-1 w-3.5 h-3.5 rounded-full ring-1 ring-white dark:ring-gray-900 {dotColor}{running ? ' status-dot-running' : ''}"
        aria-hidden="true"
      ></span>
    </span>
  {:else}
    <span
      class="flex items-center justify-center w-11 h-11 rounded-full flex-shrink-0 transition-colors {statusBadgeClass(toggling, running)}"
      title={toggling ? 'Updating…' : running ? 'Running' : 'Stopped'}
    >
      {@render children?.()}
    </span>
  {/if}
{/snippet}

{#if href}
  <a
    {href}
    target="_blank"
    rel="noopener"
    class="flex-shrink-0 {cornerRound} hover:opacity-80 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-opacity"
  >
    {@render icon()}
  </a>
{:else}
  {@render icon()}
{/if}
