<script>
  import { KeyRound } from '@lucide/svelte';
  import { fetchJSON } from '../api.js';
  import * as Dialog from '@/lib/components/ui/dialog';
  import { Button } from '@/lib/components/ui/button';
  import { Input } from '@/lib/components/ui/input';
  import { Label } from '@/lib/components/ui/label';

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
        <div class="p-3 text-sm rounded-lg rounded-md bg-emerald-50 px-3 py-2 text-sm text-emerald-700 mb-4 border border-emerald-200">
          Password changed successfully.
        </div>
        <Button class="w-full" onclick={onClose}>Close</Button>
      {:else}
        <form onsubmit={handleSubmit} class="space-y-3">
          {#if error}
            <div class="p-3 text-sm rounded-lg rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive border border-destructive/30">
              {error}
            </div>
          {/if}

          <div class="grid gap-2">
            <Label for="cp-current">Current password</Label>
            <Input
              id="cp-current"
              type="password"
              bind:value={currentPassword}
              autocomplete="current-password"
              required
              disabled={loading}
            />
          </div>

          <div class="grid gap-2">
            <Label for="cp-new">New password</Label>
            <Input
              id="cp-new"
              type="password"
              bind:value={newPassword}
              autocomplete="new-password"
              required
              disabled={loading}
            />
          </div>

          <div class="grid gap-2">
            <Label for="cp-confirm">Confirm new password</Label>
            <Input
              id="cp-confirm"
              type="password"
              bind:value={confirmPassword}
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
