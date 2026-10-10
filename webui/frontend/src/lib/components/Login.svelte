<script>
  import { brandIconSrc } from '../stores/theme.js';
  import { authenticated, needsSetup, refreshData } from '../stores/app.js';
  import { fetchJSON } from '../api.js';
  import * as Card from '@/lib/components/ui/card';
  import { Input } from '@/lib/components/ui/input';
  import { Label } from '@/lib/components/ui/label';
  import { Button } from '@/lib/components/ui/button';
  import { Alert, AlertDescription } from '@/lib/components/ui/alert';
  import { TriangleAlert, LoaderCircle } from '@lucide/svelte';

  let password = $state('');
  let loading = $state(false);
  let error = $state('');

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

<div class="flex flex-1 flex-col items-center justify-center p-4">
  <div class="w-full max-w-sm">
    <div class="mb-8 text-center">
      <img src={$brandIconSrc} alt="Tailrelay" class="mx-auto mb-4 h-16 w-16 rounded-2xl" />
      <h1 class="mb-2 text-2xl font-bold">Tail<span class="brand-relay">relay</span></h1>
      <p class="text-muted-foreground">
        Enter your admin password to continue.
      </p>
    </div>

    <Card.Root class="overflow-hidden rounded-xl">
      <Card.Content class="p-6">
        <form onsubmit={handleLogin} class="space-y-4">
          {#if error}
            <Alert variant="destructive">
              <TriangleAlert />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          {/if}

          <div class="grid gap-2">
            <Label for="password">Password</Label>
            <Input
              id="password"
              type="password"
              bind:value={password}
              required
              autocomplete="current-password"
              disabled={loading}
            />
          </div>

          <div class="pt-2">
            <Button type="submit" class="w-full" disabled={loading}>
              {#if loading}
                <LoaderCircle class="animate-spin" />
                Logging in...
              {:else}
                Log In
              {/if}
            </Button>
          </div>
        </form>
      </Card.Content>
    </Card.Root>
  </div>
</div>
