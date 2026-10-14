<script>
  import { onMount, onDestroy } from 'svelte';
  import { get } from 'svelte/store';
  import { authenticated, needsSetup, currentView, tailscaleConnected, tailscaleStatus, refreshData, findLoggedOutHealthMessage, lastUpdated } from './lib/stores/app.js';
  import { initTheme, resolvedTheme } from './lib/stores/theme.js';
  import { showToast } from './lib/stores/toast.js';
  import { fetchJSON } from './lib/api.js';
  import AppSidebar from './lib/components/AppSidebar.svelte';
  import Dashboard from './lib/components/Dashboard.svelte';
  import Tailscale from './lib/components/Tailscale.svelte';
  import Backups from './lib/components/Backups.svelte';
  import Login from './lib/components/Login.svelte';
  import Setup from './lib/components/Setup.svelte';
  import Footer from './lib/components/Footer.svelte';
  import { Toaster } from '@/lib/components/ui/sonner';
  import * as Sidebar from '@/lib/components/ui/sidebar';
  import { TriangleAlert } from '@lucide/svelte';

  let isAuthenticated = $state(false);
  let isNeedsSetup = $state(false);
  let loading = $state(true);
  let checkingTailscale = $state(false);
  let interval;

  // The sidebar must stay fixed-open on desktop (no visible restore
  // trigger once the collapse button was removed), so neutralize the
  // sidebar component's Ctrl/Cmd+B collapse shortcut there — while on
  // mobile the same shortcut still opens/closes the off-canvas sheet.
  // Capture phase so this runs before the provider's bubble listener.
  function guardSidebarShortcut(e) {
    if (
      e.key === 'b' &&
      (e.metaKey || e.ctrlKey) &&
      window.matchMedia('(min-width: 768px)').matches
    ) {
      e.stopImmediatePropagation();
      e.preventDefault();
    }
  }

  authenticated.subscribe((v) => (isAuthenticated = v));
  needsSetup.subscribe((v) => (isNeedsSetup = v));

  // Track whether the initial data load has completed so the reactive
  // subscription below does not fire prematurely (before any status is known).
  let dataLoaded = false;
  // Track the last known connected state so we only redirect on a
  // true→false transition (Tailscale dropped), not on every false value.
  let wasConnected = false;

  // When Tailscale loses connection after the initial load, lock navigation
  // to the Tailscale tab. Only redirects on a connected→disconnected transition
  // so a transient or initial false value doesn't interfere. The real signal
  // is this BackendState transition, not any particular health message —
  // the health text is only consulted below to enrich the toast wording.
  tailscaleConnected.subscribe((connected) => {
    if (!dataLoaded) return;                     // ignore pre-load transitions
    if (wasConnected && !connected && get(authenticated)) {
      currentView.set('tailscale');
      // Surface the daemon's last-login error (e.g. "authkey already used"
      // after the machine was removed from the tailnet) so the user knows
      // why they were logged out and can re-authenticate.
      const loginError = findLoggedOutHealthMessage(get(tailscaleStatus)?.Health);
      if (loginError) {
        showToast('warning', loginError);
      }
    }
    wasConnected = connected;
  });

  onMount(async () => {
    initTheme();

    try {
      const status = await fetchJSON('/api/auth/status');
      needsSetup.set(status.needsSetup);
      authenticated.set(status.authenticated);

      if (status.authenticated) {
        checkingTailscale = true;
        try {
          await refreshData();
          // After the first data load, navigate based on actual connection state.
          // If Tailscale is connected, stay on dashboard; otherwise show Tailscale.
          const connected = get(tailscaleConnected);
          wasConnected = connected;
          dataLoaded = true;
          if (!connected) {
            currentView.set('tailscale');
          }
        } finally {
          checkingTailscale = false;
        }
      }
    } catch (err) {
      if (err.message?.includes('401') || err.message?.includes('403')) {
        authenticated.set(false);
      }
    } finally {
      loading = false;
    }

    // Auto-refresh every 15 seconds
    interval = setInterval(async () => {
      if (isAuthenticated && !isNeedsSetup) {
        try {
          await refreshData();
        } catch (err) {
          if (err.message?.includes('401')) {
            authenticated.set(false);
          }
        }
      }
    }, 15000);

    // Keyboard shortcuts
    window.addEventListener('keydown', handleKeyboard);
    // Capture phase precedes the sidebar provider's bubble-phase listener
    window.addEventListener('keydown', guardSidebarShortcut, true);
  });

  onDestroy(() => {
    if (interval) clearInterval(interval);
    window.removeEventListener('keydown', handleKeyboard);
    window.removeEventListener('keydown', guardSidebarShortcut, true);
  });

  function handleKeyboard(e) {
    if (!isAuthenticated) return;
    const tag = e.target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
    // Dialog and AlertDialog both render aria-modal contents
    if (document.querySelector('[aria-modal="true"]')) return;

    switch (e.key.toLowerCase()) {
      case 'r':
        e.preventDefault();
        refreshData().catch(() => {});
        break;
    }
  }
</script>

<div class="min-h-svh flex flex-col">
  {#if loading}
    <div class="flex-1 flex items-center justify-center">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
    </div>
  {:else if isNeedsSetup}
    <Setup />
  {:else if !isAuthenticated}
    <Login />
  {:else}
    {#if checkingTailscale}
      <div class="flex-1 flex flex-col items-center justify-center gap-3 text-muted-foreground">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        <p class="text-sm">Checking Tailscale connection…</p>
      </div>
    {:else}
      <Sidebar.Provider>
        <AppSidebar />
        <Sidebar.Inset>
          {#if !$tailscaleConnected}
            <div class="border-b bg-amber-50 px-4 dark:bg-amber-950/30 dark:border-transparent">
              <p class="flex items-center gap-2 py-1.5 text-xs text-amber-700 dark:text-amber-400">
                <TriangleAlert size={13} class="shrink-0" />
                Tailscale is not connected. Relays and Backups are unavailable until Tailscale is authenticated.
              </p>
            </div>
          {/if}
          <!-- No desktop header — each page carries its own title. The
               sidebar is not collapsible, so the trigger only exists for
               the mobile off-canvas sheet. -->
          <header class="flex h-12 items-center justify-between border-b px-4 sm:hidden">
            <Sidebar.Trigger />
            {#if $lastUpdated}
              <span class="text-xs text-muted-foreground">Updated {$lastUpdated}</span>
            {/if}
          </header>
          <main class="mx-auto w-full max-w-6xl flex-1 px-4 py-6 sm:px-6">
            {#if $currentView === 'dashboard'}
              <Dashboard />
            {:else if $currentView === 'tailscale'}
              <Tailscale />
            {:else if $currentView === 'backups'}
              <Backups />
            {/if}
          </main>
          <Footer />
        </Sidebar.Inset>
      </Sidebar.Provider>
    {/if}
  {/if}
  <Toaster theme={$resolvedTheme} position="bottom-right" richColors />
</div>
