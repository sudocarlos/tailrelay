<script>
  import { onMount, onDestroy } from 'svelte';
  import { authenticated, needsSetup, currentView, refreshData } from './lib/stores/app.js';
  import { theme } from './lib/stores/theme.js';
  import { showToast } from './lib/stores/toast.js';
  import { fetchJSON } from './lib/api.js';
  import Navbar from './lib/components/Navbar.svelte';
  import Dashboard from './lib/components/Dashboard.svelte';
  import Tailscale from './lib/components/Tailscale.svelte';
  import Backups from './lib/components/Backups.svelte';
  import Metrics from './lib/components/Metrics.svelte';
  import Login from './lib/components/Login.svelte';
  import Setup from './lib/components/Setup.svelte';
  import ToastContainer from './lib/components/ToastContainer.svelte';

  let isAuthenticated = $state(false);
  let isNeedsSetup = $state(false);
  let loading = $state(true);
  let interval;

  authenticated.subscribe((v) => (isAuthenticated = v));
  needsSetup.subscribe((v) => (isNeedsSetup = v));

  onMount(async () => {
    theme.init();

    try {
      const status = await fetchJSON('/api/auth/status');
      needsSetup.set(status.needsSetup);
      authenticated.set(status.authenticated);

      if (status.authenticated) {
        await refreshData();
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
  });

  onDestroy(() => {
    if (interval) clearInterval(interval);
    window.removeEventListener('keydown', handleKeyboard);
  });

  function handleKeyboard(e) {
    if (!isAuthenticated) return;
    const tag = e.target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
    if (document.querySelector('[data-modal-open]')) return;

    switch (e.key.toLowerCase()) {
      case 'r':
        e.preventDefault();
        refreshData().catch(() => {});
        break;
    }
  }
</script>

<div class="min-h-screen flex flex-col">
  {#if loading}
    <div class="flex-1 flex items-center justify-center">
      <div class="animate-spin rounded-full h-8 w-8 border-2 border-blue-500 border-t-transparent"></div>
    </div>
  {:else if isNeedsSetup}
    <Setup />
  {:else if !isAuthenticated}
    <Login />
  {:else}
    <Navbar />
    <main class="flex-1 max-w-6xl w-full mx-auto px-4 sm:px-6 py-6">
      {#if $currentView === 'dashboard'}
        <Dashboard />
      {:else if $currentView === 'tailscale'}
        <Tailscale />
      {:else if $currentView === 'metrics'}
        <Metrics />
      {:else if $currentView === 'backups'}
        <Backups />
      {/if}
    </main>
  {/if}
  <ToastContainer />
</div>
