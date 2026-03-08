<script>
  import { theme } from '../stores/theme.js';
  import { currentView, refreshData, lastUpdated, logout } from '../stores/app.js';
  import { showToast } from '../stores/toast.js';
  import { Sun, Moon, RefreshCw, LogOut, Menu, X, KeyRound } from '@lucide/svelte';
  import ChangePasswordModal from './ChangePasswordModal.svelte';

  let currentTheme = $state('light');
  let menuOpen = $state(false);
  let updated = $state('');
  let refreshing = $state(false);
  let showChangePassword = $state(false);

  theme.subscribe((v) => (currentTheme = v));
  lastUpdated.subscribe((v) => (updated = v));

  function switchView(view) {
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
</script>

<nav class="border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 sticky top-0 z-40">
  <div class="max-w-6xl mx-auto px-4 sm:px-6">
    <div class="flex items-center justify-between h-14">
      <!-- Brand -->
      <button class="text-lg font-semibold tracking-tight hover:opacity-80 transition-opacity" onclick={() => switchView('dashboard')}>
        Tailrelay
      </button>

      <!-- Desktop nav -->
      <div class="hidden sm:flex items-center gap-1">
        <button
          class="px-3 py-1.5 text-sm rounded-md transition-colors {$currentView === 'dashboard' ? 'bg-gray-100 dark:bg-gray-800 font-medium' : 'hover:bg-gray-50 dark:hover:bg-gray-800/50 text-gray-600 dark:text-gray-400'}"
          onclick={() => switchView('dashboard')}
        >
          Dashboard
        </button>
        <button
          class="px-3 py-1.5 text-sm rounded-md transition-colors {$currentView === 'tailscale' ? 'bg-gray-100 dark:bg-gray-800 font-medium' : 'hover:bg-gray-50 dark:hover:bg-gray-800/50 text-gray-600 dark:text-gray-400'}"
          onclick={() => switchView('tailscale')}
        >
          Tailscale
        </button>
        <button
          class="px-3 py-1.5 text-sm rounded-md transition-colors {$currentView === 'metrics' ? 'bg-gray-100 dark:bg-gray-800 font-medium' : 'hover:bg-gray-50 dark:hover:bg-gray-800/50 text-gray-600 dark:text-gray-400'}"
          onclick={() => switchView('metrics')}
        >
          Metrics
        </button>
        <button
          class="px-3 py-1.5 text-sm rounded-md transition-colors {$currentView === 'backups' ? 'bg-gray-100 dark:bg-gray-800 font-medium' : 'hover:bg-gray-50 dark:hover:bg-gray-800/50 text-gray-600 dark:text-gray-400'}"
          onclick={() => switchView('backups')}
        >
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
        <button
          class="px-3 py-2 text-sm rounded-md text-left transition-colors {$currentView === 'dashboard' ? 'bg-gray-100 dark:bg-gray-800 font-medium' : 'text-gray-600 dark:text-gray-400'}"
          onclick={() => switchView('dashboard')}
        >
          Dashboard
        </button>
        <button
          class="px-3 py-2 text-sm rounded-md text-left transition-colors {$currentView === 'tailscale' ? 'bg-gray-100 dark:bg-gray-800 font-medium' : 'text-gray-600 dark:text-gray-400'}"
          onclick={() => switchView('tailscale')}
        >
          Tailscale
        </button>
        <button
          class="px-3 py-2 text-sm rounded-md text-left transition-colors {$currentView === 'metrics' ? 'bg-gray-100 dark:bg-gray-800 font-medium' : 'text-gray-600 dark:text-gray-400'}"
          onclick={() => switchView('metrics')}
        >
          Metrics
        </button>
        <button
          class="px-3 py-2 text-sm rounded-md text-left transition-colors {$currentView === 'backups' ? 'bg-gray-100 dark:bg-gray-800 font-medium' : 'text-gray-600 dark:text-gray-400'}"
          onclick={() => switchView('backups')}
        >
          Backups
        </button>
      </div>
    {/if}
  </div>
</nav>

{#if showChangePassword}
  <ChangePasswordModal onClose={() => (showChangePassword = false)} />
{/if}
