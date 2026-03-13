<script>
  import { Lock } from '@lucide/svelte';
  import { theme } from '../stores/theme.js';
  import { authenticated, needsSetup, refreshData } from '../stores/app.js';
  import { fetchJSON } from '../api.js';

  let currentTheme = $state('dark');
  let password = $state('');
  let loading = $state(false);
  let error = $state('');

  theme.subscribe((v) => (currentTheme = v));

  async function handleLogin(e) {
    e.preventDefault();

    if (!password) {
      error = 'Please enter your password.';
      return;
    }

    error = '';
    loading = true;

    try {
      await fetchJSON('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ password })
      });
      authenticated.set(true);
      await refreshData();
      window.location.replace('/');
    } catch (err) {
      error = 'Invalid password.';
    } finally {
      loading = false;
    }
  }
</script>

<div class="flex-1 flex flex-col items-center justify-center p-4">
  <div class="w-full max-w-sm">
    <div class="text-center mb-8">
      <div class="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-blue-500/10 text-blue-500 mb-4">
        <Lock size={32} />
      </div>
      <h1 class="text-2xl font-bold mb-2">Tailrelay</h1>
      <p class="text-gray-500 dark:text-gray-400">
        Enter your admin password to continue.
      </p>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6 overflow-hidden">
      <form onsubmit={handleLogin} class="space-y-4">
        {#if error}
          <div class="p-3 text-sm rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 border border-red-200 dark:border-red-800">
            {error}
          </div>
        {/if}

        <div>
          <label for="password" class="block text-sm font-medium mb-1.5">Password</label>
          <input
            id="password"
            type="password"
            bind:value={password}
            class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none transition-shadow"
            required
            autocomplete="current-password"
            disabled={loading}
            focus
          />
        </div>

        <div class="pt-2">
          <button
            type="submit"
            disabled={loading}
            class="w-full inline-flex justify-center items-center gap-2 px-4 py-2.5 text-sm font-medium text-white bg-blue-500 hover:bg-blue-600 disabled:bg-blue-500/50 disabled:cursor-not-allowed rounded-lg transition-colors"
          >
            {#if loading}
              <div class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
              Logging in...
            {:else}
              Log In
            {/if}
          </button>
        </div>
      </form>
    </div>
  </div>
</div>
