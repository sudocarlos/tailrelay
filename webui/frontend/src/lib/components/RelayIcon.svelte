<script>
  import { statusBadgeClass, statusIconClass } from '../utils/statusBadge.js';

  // RelayIcon renders either the default type glyph inside the colored
  // status badge, or — when iconUrl is set — a rounded-square app icon
  // with a small corner status dot. The icon keeps using its own
  // transparency/round shape; the square frame is translucent with a
  // 1px border so transparent and round icons still read on light/dark.
  //
  // size controls the rendered box: "md" (44px) for detail contexts like
  // modals, "sm" (28px) for table rows.
  let {
    iconUrl = '',
    toggling = false,
    running = false,
    alt = 'relay icon',
    href = '',
    size = 'md',
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
    toggling ? 'bg-amber-400' : running ? 'bg-green-700' : 'bg-muted-foreground/50'
  );
  const dotTitle = $derived((toggling ? 'Updating' : running ? 'Running' : 'Stopped') + '…');

  const boxClass = $derived(
    size === 'sm' ? 'w-7 h-7 rounded-lg' : 'w-11 h-11 rounded-xl'
  );
  const dotClass = $derived(
    size === 'sm' ? '-bottom-0.5 -right-0.5 w-2 h-2' : '-bottom-1 -right-1 w-3.5 h-3.5'
  );

  const cornerRound = $derived(hasIcon ? 'rounded-lg' : 'rounded-full');
</script>

{#snippet icon()}
  {#if hasIcon}
    <span class="relative flex-shrink-0 {boxClass}" title={dotTitle}>
      <span
        class="flex items-center justify-center {boxClass} overflow-hidden border border-border bg-muted/60"
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
        class="absolute ring-1 ring-background {dotClass} {dotColor}{running ? ' status-dot-running' : ''}"
        aria-hidden="true"
      ></span>
    </span>
  {:else}
    <span
      class="flex items-center justify-center {boxClass} flex-shrink-0 transition-colors {statusBadgeClass(toggling, running)}"
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
    class="flex-shrink-0 {cornerRound} transition-opacity hover:opacity-80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
  >
    {@render icon()}
  </a>
{:else}
  {@render icon()}
{/if}
