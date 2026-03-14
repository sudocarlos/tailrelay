<script>
  import { theme } from '../stores/theme.js';
  import { authenticated, needsSetup } from '../stores/app.js';
  import { fetchJSON } from '../api.js';

  let currentTheme = $state('dark');
  let password = $state('');
  let confirmPassword = $state('');
  let loading = $state(false);
  let error = $state('');

  theme.subscribe((v) => (currentTheme = v));

  async function handleSubmit(e) {
    e.preventDefault();

    if (password.length < 6) {
      error = 'Password must be at least 6 characters.';
      return;
    }

    if (password !== confirmPassword) {
      error = 'Passwords do not match.';
      return;
    }

    error = '';
    loading = true;

    try {
      await fetchJSON('/api/auth/setup', {
        method: 'POST',
        body: JSON.stringify({ password })
      });
      needsSetup.set(false);
      authenticated.set(true);
      window.location.replace('/');
    } catch (err) {
      error = err.message || 'Setup failed. Please try again.';
    } finally {
      loading = false;
    }
  }
</script>

<div class="flex-1 flex flex-col items-center justify-center p-4">
  <div class="w-full max-w-md">
    <div class="text-center mb-8">
      <img src="/icon-192.png" alt="Tailrelay" class="w-16 h-16 rounded-2xl mb-4 mx-auto" />
      <h1 class="text-2xl font-bold mb-2">Initial Setup</h1>
      <p class="text-gray-500 dark:text-gray-400">
        Welcome to Tailrelay. Please set an administrator password to secure the dashboard.
      </p>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6 overflow-hidden">
      <form onsubmit={handleSubmit} class="space-y-4">
        {#if error}
          <div class="p-3 text-sm rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 border border-red-200 dark:border-red-800">
            {error}
          </div>
        {/if}

        <div>
          <label for="password" class="block text-sm font-medium mb-1.5">New Password</label>
          <input
            id="password"
            type="password"
            bind:value={password}
            class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none transition-shadow"
            required
            autocomplete="new-password"
            disabled={loading}
          />
        </div>

        <div>
          <label for="confirm_password" class="block text-sm font-medium mb-1.5">Confirm Password</label>
          <input
            id="confirm_password"
            type="password"
            bind:value={confirmPassword}
            class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none transition-shadow"
            required
            autocomplete="new-password"
            disabled={loading}
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
              Saving...
            {:else}
              Save Password & Continue
            {/if}
          </button>
        </div>
      </form>
    </div>
  </div>
</div>
