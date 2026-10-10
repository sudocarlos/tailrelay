<script>
  import { KeyRound } from '@lucide/svelte';
  import { fetchJSON } from '../api.js';
  import * as Dialog from '@/lib/components/ui/dialog';
  import { Button } from '@/lib/components/ui/button';

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

</script>

<Dialog.Root
  open
  onOpenChange={(v) => { if (!v) onClose(); }}
>
  <Dialog.Content class="rounded-xl sm:max-w-sm">
    <!-- Header -->
    <Dialog.Header>
      <Dialog.Title class="flex items-center gap-2 text-sm font-semibold">
        <KeyRound size={16} class="text-muted-foreground" />
        Change Password
      </Dialog.Title>
      <Dialog.Description class="sr-only">Change the Web UI password</Dialog.Description>
    </Dialog.Header>

    <!-- Body -->
    <div class="px-5 py-4">
      {#if success}
        <div class="p-3 text-sm rounded-lg bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300 border border-green-200 dark:border-green-800 mb-4">
          Password changed successfully.
        </div>
        <Button class="w-full" onclick={onClose}>Close</Button>
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
            <Button type="button" variant="secondary" class="flex-1" onclick={onClose} disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" class="flex-1" disabled={loading}>
              {#if loading}
                <div class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
                Saving...
              {:else}
                Save
              {/if}
            </Button>
          </div>
        </form>
      {/if}
    </div>
  </Dialog.Content>
</Dialog.Root>
