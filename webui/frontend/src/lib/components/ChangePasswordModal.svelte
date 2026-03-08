<script>
  import { KeyRound, X } from '@lucide/svelte';
  import { fetchJSON } from '../api.js';

  let { onClose } = $props();

  let currentPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let loading = $state(false);
  let error = $state('');
  let success = $state(false);

  async function handleSubmit(e) {
    e.preventDefault();
    error = '';

    if (newPassword !== confirmPassword) {
      error = 'New passwords do not match.';
      return;
    }

    if (newPassword.length < 1) {
      error = 'New password cannot be empty.';
      return;
    }

    loading = true;
    try {
      await fetchJSON('/api/auth/change-password', {
        method: 'POST',
        body: JSON.stringify({ currentPassword, newPassword }),
      });
      success = true;
      currentPassword = '';
      newPassword = '';
      confirmPassword = '';
    } catch (err) {
      const msg = err.message || '';
      if (msg.includes('401') || msg.toLowerCase().includes('incorrect')) {
        error = 'Current password is incorrect.';
      } else {
        error = 'Failed to change password. Please try again.';
      }
    } finally {
      loading = false;
    }
  }

  function handleBackdropClick(e) {
    if (e.target === e.currentTarget) onClose();
  }
</script>

<!-- Backdrop -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm"
  role="dialog"
  tabindex="-1"
  aria-modal="true"
  aria-labelledby="change-pw-title"
  onmousedown={handleBackdropClick}
>
  <div class="w-full max-w-sm bg-white dark:bg-gray-900 rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
    <!-- Header -->
    <div class="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center gap-2">
        <KeyRound size={16} class="text-gray-500 dark:text-gray-400" />
        <h2 id="change-pw-title" class="text-sm font-semibold">Change Password</h2>
      </div>
      <button
        class="p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500 dark:text-gray-400 transition-colors"
        onclick={onClose}
        aria-label="Close"
      >
        <X size={14} />
      </button>
    </div>

    <!-- Body -->
    <div class="px-5 py-4">
      {#if success}
        <div class="p-3 text-sm rounded-lg bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300 border border-green-200 dark:border-green-800 mb-4">
          Password changed successfully.
        </div>
        <button
          class="w-full px-4 py-2 text-sm font-medium rounded-lg bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
          onclick={onClose}
        >
          Close
        </button>
      {:else}
        <form onsubmit={handleSubmit} class="space-y-3">
          {#if error}
            <div class="p-3 text-sm rounded-lg bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 border border-red-200 dark:border-red-800">
              {error}
            </div>
          {/if}

          <div>
            <label for="cp-current" class="block text-sm font-medium mb-1.5">Current password</label>
            <input
              id="cp-current"
              type="password"
              bind:value={currentPassword}
              class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none transition-shadow"
              autocomplete="current-password"
              required
              disabled={loading}
            />
          </div>

          <div>
            <label for="cp-new" class="block text-sm font-medium mb-1.5">New password</label>
            <input
              id="cp-new"
              type="password"
              bind:value={newPassword}
              class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none transition-shadow"
              autocomplete="new-password"
              required
              disabled={loading}
            />
          </div>

          <div>
            <label for="cp-confirm" class="block text-sm font-medium mb-1.5">Confirm new password</label>
            <input
              id="cp-confirm"
              type="password"
              bind:value={confirmPassword}
              class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 outline-none transition-shadow"
              autocomplete="new-password"
              required
              disabled={loading}
            />
          </div>

          <div class="flex gap-2 pt-1">
            <button
              type="button"
              class="flex-1 px-4 py-2 text-sm font-medium rounded-lg bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
              onclick={onClose}
              disabled={loading}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              class="flex-1 inline-flex justify-center items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-blue-500 hover:bg-blue-600 disabled:bg-blue-500/50 disabled:cursor-not-allowed rounded-lg transition-colors"
            >
              {#if loading}
                <div class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
                Saving...
              {:else}
                Save
              {/if}
            </button>
          </div>
        </form>
      {/if}
    </div>
  </div>
</div>
