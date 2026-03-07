<script>
  import { onMount, onDestroy } from 'svelte';
  import { authenticated } from '../stores/app.js';
  import { theme } from '../stores/theme.js';
  import { Sun, Moon, ExternalLink, Copy, Check } from '@lucide/svelte';

  let status = $state('checking');
  let authUrl = $state('');
  let copied = $state(false);
  let loginTriggered = $state(false);
  let currentTheme = $state('light');
  let pollTimer;

  theme.subscribe((v) => (currentTheme = v));

  const statusMessages = {
    checking: 'Checking Tailscale status...',
    waiting: 'Waiting for Tailscale to start...',
    generating: 'Generating Tailscale login link...',
    pending: 'Waiting for Tailscale authentication...',
    connected: 'Tailscale connected. Redirecting...',
    error: 'Failed to generate login link.',
  };

  const statusStyles = {
    checking: 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 border-blue-200 dark:border-blue-800',
    waiting: 'bg-amber-50 dark:bg-amber-900/20 text-amber-700 dark:text-amber-300 border-amber-200 dark:border-amber-800',
    generating: 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 border-blue-200 dark:border-blue-800',
    pending: 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 border-blue-200 dark:border-blue-800',
    connected: 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300 border-green-200 dark:border-green-800',
    error: 'bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 border-red-200 dark:border-red-800',
  };

  onMount(() => {
    theme.init();
    poll();
    pollTimer = setInterval(poll, 4000);
  });

  onDestroy(() => {
    if (pollTimer) clearInterval(pollTimer);
  });

  async function poll() {
    try {
      const response = await fetch('/api/tailscale/poll', { credentials: 'same-origin' });
      if (!response.ok) {
        status = 'waiting';
        return;
      }
      const data = await response.json();

      if (data.connected) {
        status = 'connected';
        if (pollTimer) clearInterval(pollTimer);
        setTimeout(() => {
          authenticated.set(true);
          window.location.replace('/');
        }, 1000);
        return;
      }

      if (!loginTriggered) {
        loginTriggered = true;
        status = 'generating';
        await requestLogin();
      }
    } catch {
      status = 'waiting';
    }
  }

  async function requestLogin() {
    try {
      const response = await fetch('/api/tailscale/login', {
        method: 'POST',
        credentials: 'same-origin',
      });
      if (!response.ok) {
        status = 'error';
        return;
      }
      const data = await response.json();
      if (data.auth_url) {
        authUrl = data.auth_url;
        status = 'pending';
      }
    } catch {
      status = 'error';
    }
  }

  async function copyUrl() {
    try {
      await navigator.clipboard.writeText(authUrl);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      // Fallback: select the input
    }
  }
</script>

<div class="min-h-screen flex items-center justify-center p-4">
  <div class="w-full max-w-md">
    <div class="bg-white dark:bg-gray-900 rounded-xl shadow-lg border border-gray-200 dark:border-gray-800 p-6">
      <!-- Header -->
      <div class="flex items-center justify-between mb-4">
        <h1 class="text-xl font-semibold tracking-tight">Tailrelay</h1>
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
      </div>

      <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
        Connect Tailrelay to your Tailnet to access the Web UI.
      </p>

      <!-- Status banner -->
      <div class="rounded-lg border px-4 py-3 text-sm mb-4 {statusStyles[status] || statusStyles.checking}">
        {statusMessages[status] || 'Checking...'}
      </div>

      <!-- Auth URL section -->
      {#if authUrl && status === 'pending'}
        <div class="flex flex-col gap-3 mb-4">
          <div class="flex gap-2">
            <input
              type="text"
              readonly
              value={authUrl}
              class="flex-1 rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 px-3 py-2 text-sm truncate"
            />
            <button
              class="px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
              onclick={copyUrl}
              title="Copy link"
            >
              {#if copied}
                <Check size={16} class="text-green-500" />
              {:else}
                <Copy size={16} class="text-gray-500" />
              {/if}
            </button>
          </div>

          <a
            href={authUrl}
            target="_blank"
            rel="noopener"
            class="inline-flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-white bg-blue-500 hover:bg-blue-600 rounded-lg transition-colors"
          >
            <ExternalLink size={15} />
            Open Tailscale login
          </a>
        </div>
      {/if}

      <!-- MagicDNS info -->
      <div class="border-t border-gray-200 dark:border-gray-800 pt-4 mt-4">
        <h2 class="text-sm font-semibold mb-1">MagicDNS required for HTTPS</h2>
        <p class="text-xs text-gray-500 dark:text-gray-400 mb-2">
          Enable MagicDNS in the Tailscale admin console. Tailnets created after October 20, 2022 have it enabled by default.
        </p>
        <ul class="text-xs space-y-1">
          <li>
            <a href="https://login.tailscale.com/admin/dns" target="_blank" rel="noopener" class="text-blue-500 hover:underline">
              Enable MagicDNS
            </a>
          </li>
          <li>
            <a href="https://tailscale.com/kb/1081/magicdns" target="_blank" rel="noopener" class="text-blue-500 hover:underline">
              Learn how MagicDNS works
            </a>
          </li>
        </ul>
      </div>
    </div>
  </div>
</div>
