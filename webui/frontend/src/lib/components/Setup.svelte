<script>
  import { brandIconSrc } from '../stores/theme.js';
  import { authenticated, needsSetup } from '../stores/app.js';
  import { fetchJSON } from '../api.js';
  import * as Card from '@/lib/components/ui/card';
  import { Input } from '@/lib/components/ui/input';
  import { Label } from '@/lib/components/ui/label';
  import { Button } from '@/lib/components/ui/button';
  import { Alert, AlertDescription } from '@/lib/components/ui/alert';
  import { TriangleAlert, LoaderCircle } from '@lucide/svelte';

  let password = $state('');
  let confirmPassword = $state('');
  let loading = $state(false);
  let error = $state('');

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

<div class="flex flex-1 flex-col items-center justify-center p-4">
  <div class="w-full max-w-md">
    <div class="mb-8 text-center">
      <img src={$brandIconSrc} alt="Tailrelay" class="mx-auto mb-4 h-16 w-16 rounded-2xl" />
      <h1 class="mb-2 text-2xl font-bold">Initial Setup</h1>
      <p class="text-muted-foreground">
        Welcome to Tailrelay. Please set an administrator password to secure the dashboard.
      </p>
    </div>

    <Card.Root class="overflow-hidden rounded-xl">
      <Card.Content class="p-6">
        <form onsubmit={handleSubmit} class="space-y-4">
          {#if error}
            <Alert variant="destructive">
              <TriangleAlert />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          {/if}

          <div class="grid gap-2">
            <Label for="password">New Password</Label>
            <Input
              id="password"
              type="password"
              bind:value={password}
              required
              autocomplete="new-password"
              disabled={loading}
            />
          </div>

          <div class="grid gap-2">
            <Label for="confirm_password">Confirm Password</Label>
            <Input
              id="confirm_password"
              type="password"
              bind:value={confirmPassword}
              required
              autocomplete="new-password"
              disabled={loading}
            />
          </div>

          <div class="pt-2">
            <Button type="submit" class="w-full" disabled={loading}>
              {#if loading}
                <LoaderCircle class="animate-spin" />
                Saving...
              {:else}
                Save Password & Continue
              {/if}
            </Button>
          </div>
        </form>
      </Card.Content>
    </Card.Root>
  </div>
</div>
