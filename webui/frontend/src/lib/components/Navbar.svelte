<script>
  import { theme } from '../stores/theme.js';
  import { currentView, refreshData, lastUpdated, logout, tailscaleConnected } from '../stores/app.js';
  import { showToast } from '../stores/toast.js';
  import { Sun, Moon, RefreshCw, LogOut, Menu, X, KeyRound, AlertTriangle } from '@lucide/svelte';
  import ChangePasswordModal from './ChangePasswordModal.svelte';

  let currentTheme = $state('light');
  let menuOpen = $state(false);
  let updated = $state('');
  let refreshing = $state(false);
  let showChangePassword = $state(false);
  let tsConnected = $state(true);
  let currentViewValue = $state('dashboard');

  theme.subscribe((v) => (currentTheme = v));
  lastUpdated.subscribe((v) => (updated = v));
  tailscaleConnected.subscribe((v) => (tsConnected = v));
  currentView.subscribe((v) => (currentViewValue = v));

  function switchView(view) {
    // Block navigation to non-Tailscale views when Tailscale is disconnected.
    if (!tsConnected && view !== 'tailscale') return;
    currentView.set(view);
    menuOpen = false;
  }

  async function handleRefresh() {
    refreshing = true;
    try {
      await refreshData();
      showToast('success', 'Data refreshed');
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      refreshing = false;
    }
  }

  // CSS helpers for nav buttons depending on connection state.
  function navBtnClass(view) {
    const active = currentViewValue === view;
    const locked = !tsConnected && view !== 'tailscale';
    if (locked) {
      return 'px-3 py-1.5 text-sm rounded-md opacity-40 cursor-not-allowed text-gray-400 dark:text-gray-600';
    }
    return `px-3 py-1.5 text-sm rounded-md transition-colors ${
      active
        ? 'bg-gray-100 dark:bg-gray-800 font-medium'
        : 'hover:bg-gray-50 dark:hover:bg-gray-800/50 text-gray-600 dark:text-gray-400'
    }`;
  }

  function mobileNavBtnClass(view) {
    const active = currentViewValue === view;
    const locked = !tsConnected && view !== 'tailscale';
    if (locked) {
      return 'px-3 py-2 text-sm rounded-md text-left opacity-40 cursor-not-allowed text-gray-400 dark:text-gray-600';
    }
    return `px-3 py-2 text-sm rounded-md text-left transition-colors ${
      active ? 'bg-gray-100 dark:bg-gray-800 font-medium' : 'text-gray-600 dark:text-gray-400'
    }`;
  }
</script>

<nav class="border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 sticky top-0 z-40">
  <div class="max-w-6xl mx-auto px-4 sm:px-6">
    <div class="flex items-center justify-between h-14">
      <!-- Brand -->
      <button class="flex items-center gap-2 text-lg font-semibold tracking-tight hover:opacity-80 transition-opacity" onclick={() => switchView('dashboard')}>
        <img src="/icon-192.png" alt="Tailrelay" class="w-7 h-7 rounded-lg object-contain flex-shrink-0" width="28" height="28" />
        Tailrelay
      </button>

      <!-- Desktop nav -->
      <div class="hidden sm:flex items-center gap-1">
        <button class={navBtnClass('dashboard')} onclick={() => switchView('dashboard')}>
          Relays
        </button>
        <button class={navBtnClass('tailscale')} onclick={() => switchView('tailscale')}>
          <span class="inline-flex items-center gap-1.5">
            Tailscale
            {#if !tsConnected}
              <span class="w-1.5 h-1.5 rounded-full bg-amber-500 flex-shrink-0" title="Tailscale not connected"></span>
            {/if}
          </span>
        </button>
        <button class={navBtnClass('backups')} onclick={() => switchView('backups')}>
          Backups
        </button>
      </div>

      <!-- Right actions -->
      <div class="flex items-center gap-2">
        {#if updated}
          <span class="hidden sm:inline text-xs text-gray-400 dark:text-gray-500">
            Updated {updated}
          </span>
        {/if}

        <button
          class="p-2 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors text-gray-500 dark:text-gray-400"
          onclick={handleRefresh}
          title="Refresh (r)"
        >
          <RefreshCw size={16} class={refreshing ? 'animate-spin' : ''} />
        </button>

        <button
          class="p-2 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors text-gray-500 dark:text-gray-400"
          onclick={() => theme.toggle()}
          title="Toggle theme"
        >
          {#if currentTheme === 'dark'}
            <Sun size={16} />
          {:else}
            <Moon size={16} />
          {/if}
        </button>

        <button
          class="p-2 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors text-gray-500 dark:text-gray-400"
          onclick={() => (showChangePassword = true)}
          title="Change password"
        >
          <KeyRound size={16} />
        </button>

        <button
          class="p-2 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors text-gray-500 dark:text-gray-400"
          title="Logout"
          onclick={logout}
        >
          <LogOut size={16} />
        </button>

        <!-- Mobile menu toggle -->
        <button
          class="sm:hidden p-2 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors text-gray-500 dark:text-gray-400"
          onclick={() => (menuOpen = !menuOpen)}
        >
          {#if menuOpen}
            <X size={16} />
          {:else}
            <Menu size={16} />
          {/if}
        </button>
      </div>
    </div>

    <!-- Mobile nav -->
    {#if menuOpen}
      <div class="sm:hidden pb-3 pt-1 flex flex-col gap-1 border-t border-gray-200 dark:border-gray-800">
        <button class={mobileNavBtnClass('dashboard')} onclick={() => switchView('dashboard')}>
          Relays
        </button>
        <button class={mobileNavBtnClass('tailscale')} onclick={() => switchView('tailscale')}>
          <span class="inline-flex items-center gap-1.5">
            Tailscale
            {#if !tsConnected}
              <span class="w-1.5 h-1.5 rounded-full bg-amber-500 flex-shrink-0"></span>
            {/if}
          </span>
        </button>
        <button class={mobileNavBtnClass('backups')} onclick={() => switchView('backups')}>
          Backups
        </button>
      </div>
    {/if}
  </div>

  <!-- Tailscale disconnected warning banner -->
  {#if !tsConnected}
    <div class="border-t border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 px-4 sm:px-6 py-2">
      <p class="max-w-6xl mx-auto flex items-center gap-2 text-xs text-amber-700 dark:text-amber-400">
        <AlertTriangle size={13} class="flex-shrink-0" />
        Tailscale is not connected. Relays and Backups are unavailable until Tailscale is authenticated.
      </p>
    </div>
  {/if}
</nav>

{#if showChangePassword}
  <ChangePasswordModal onClose={() => (showChangePassword = false)} />
{/if}
