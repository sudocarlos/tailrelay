<script>
  import { onMount, onDestroy } from 'svelte';
  import { tailscaleStatus, refreshData } from '../stores/app.js';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import { Wifi, WifiOff, LogIn, LogOut, RefreshCw, Server, AlertTriangle, ExternalLink, Check, Users } from '@lucide/svelte';

  // ── Local state ───────────────────────────────────────────────────
  let status = $state(null);
  let peers = $state([]);
  let peersLoading = $state(false);
  let loginLoading = $state(false);
  let loginURL = $state('');
  let hostnameInput = $state('');
  let hostnameLoading = $state(false);
  let connectLoading = $state(false);
  let pollInterval = null;

  // Auth key tab state
  let authTab = $state('url'); // 'url' | 'key'
  let authKey = $state('');
  let authKeyLoading = $state(false);
  let authKeyError = $state('');

  // ── Subscribe to the shared status store ─────────────────────────
  tailscaleStatus.subscribe((v) => {
    status = v;
    // Pre-fill hostname input from current status when it first loads
    if (v?.Hostname && !hostnameInput) {
      hostnameInput = v.Hostname;
    }
  });

  // ── Derived helpers ───────────────────────────────────────────────
  function stateLabel(s) {
    if (!s) return 'Unknown';
    switch (s.BackendState) {
      case 'Running': return 'Connected';
      case 'Starting': return 'Starting…';
      case 'Stopped': return 'Disconnected';
      case 'NeedsLogin': return 'Needs Login';
      case 'NoState': return 'Not Started';
      default: return s.BackendState || 'Unknown';
    }
  }

  function stateBadgeClass(s) {
    if (!s) return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400';
    switch (s.BackendState) {
      case 'Running':
        return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-400';
      case 'NeedsLogin':
      case 'NoState':
        return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-400';
      default:
        return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400';
    }
  }

  function dotClass(s) {
    if (!s) return 'bg-gray-400';
    switch (s.BackendState) {
      case 'Running': return 'bg-emerald-500';
      case 'Starting': return 'bg-amber-500 animate-pulse';
      case 'NeedsLogin':
      case 'NoState': return 'bg-amber-500';
      default: return 'bg-red-500';
    }
  }

  function formatLastSeen(ts) {
    if (!ts) return 'never';
    const d = new Date(ts);
    const diff = Date.now() - d.getTime();
    if (diff < 60000) return 'just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    return `${Math.floor(diff / 86400000)}d ago`;
  }

  // ── Actions ───────────────────────────────────────────────────────
  async function fetchPeers() {
    peersLoading = true;
    try {
      const data = await fetchJSON('/api/tailscale/peers');
      peers = data || [];
    } catch (err) {
      // Peers failing is non-fatal; just show empty list
      peers = [];
    } finally {
      peersLoading = false;
    }
  }

  async function handleGetLoginURL() {
    loginLoading = true;
    loginURL = '';
    try {
      const data = await fetchJSON('/api/tailscale/login', { method: 'POST' });
      loginURL = data.auth_url;
      // Start faster polling to detect when auth completes
      startLoginPoll();
    } catch (err) {
      showToast('danger', err.message || 'Failed to get login URL');
    } finally {
      loginLoading = false;
    }
  }

  async function handleLoginWithKey() {
    authKeyError = '';
    const key = authKey.trim();
    if (!key) {
      authKeyError = 'Auth key cannot be empty';
      return;
    }
    if (!key.startsWith('tskey-')) {
      authKeyError = "Invalid key: must start with 'tskey-'";
      return;
    }
    authKeyLoading = true;
    try {
      await fetchJSON('/api/tailscale/login-with-key', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ auth_key: key }),
      });
      authKey = '';
      showToast('success', 'Authenticated and connected via auth key!');
      await refreshData();
      await fetchPeers();
    } catch (err) {
      authKeyError = err.message || 'Failed to authenticate with auth key';
    } finally {
      authKeyLoading = false;
    }
  }

  function startLoginPoll() {
    stopLoginPoll();
    pollInterval = setInterval(async () => {
      try {
        const data = await fetchJSON('/api/tailscale/poll');
        if (data.connected) {
          stopLoginPoll();
          loginURL = '';
          showToast('success', 'Tailscale connected!');
          await refreshData();
          await fetchPeers();
        }
      } catch {
        // Ignore poll errors
      }
    }, 3000);
  }

  function stopLoginPoll() {
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
  }

  async function handleConnect() {
    connectLoading = true;
    try {
      await fetchJSON('/api/tailscale/connect', { method: 'POST' });
      showToast('success', 'Tailscale connecting…');
      await refreshData();
      await fetchPeers();
    } catch (err) {
      showToast('danger', err.message || 'Failed to connect');
    } finally {
      connectLoading = false;
    }
  }

  async function handleDisconnect() {
    connectLoading = true;
    try {
      await fetchJSON('/api/tailscale/disconnect', { method: 'POST' });
      showToast('success', 'Tailscale disconnected');
      await refreshData();
      peers = [];
    } catch (err) {
      showToast('danger', err.message || 'Failed to disconnect');
    } finally {
      connectLoading = false;
    }
  }

  async function handleChangeHostname() {
    const newHostname = hostnameInput.trim();
    if (!newHostname) {
      showToast('danger', 'Hostname cannot be empty');
      return;
    }
    hostnameLoading = true;
    try {
      await fetchJSON('/api/tailscale/hostname', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ hostname: newHostname }),
      });
      showToast('success', `Hostname changed to ${newHostname}`);
      await refreshData();
    } catch (err) {
      showToast('danger', err.message || 'Failed to change hostname');
    } finally {
      hostnameLoading = false;
    }
  }

  onMount(() => {
    fetchPeers();
  });

  onDestroy(() => {
    stopLoginPoll();
  });
</script>

<div class="space-y-6">
  <!-- Page header -->
  <div class="flex items-center justify-between">
    <h1 class="text-xl font-semibold">Tailscale</h1>
    {#if !status}
      <span class="text-xs text-gray-400">Loading…</span>
    {/if}
  </div>

  <!-- Status card -->
  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 p-5 space-y-4">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        {#if status?.BackendState === 'Running'}
          <Wifi size={20} class="text-emerald-500" />
        {:else}
          <WifiOff size={20} class="text-gray-400" />
        {/if}
        <span class="font-medium text-gray-900 dark:text-gray-100">Connection Status</span>
      </div>
      <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium {stateBadgeClass(status)}">
        <span class="h-1.5 w-1.5 rounded-full {dotClass(status)}"></span>
        {stateLabel(status)}
      </span>
    </div>

    {#if status}
      <dl class="grid grid-cols-2 sm:grid-cols-3 gap-x-6 gap-y-3 text-sm">
        {#if status.Hostname}
          <div>
            <dt class="text-xs text-gray-500 dark:text-gray-400">Hostname</dt>
            <dd class="font-mono text-gray-900 dark:text-gray-100 truncate">{status.Hostname}</dd>
          </div>
        {/if}
        {#if status.IPv4}
          <div>
            <dt class="text-xs text-gray-500 dark:text-gray-400">Tailscale IPv4</dt>
            <dd class="font-mono text-gray-900 dark:text-gray-100">{status.IPv4}</dd>
          </div>
        {/if}
        {#if status.MagicDNSName}
          <div>
            <dt class="text-xs text-gray-500 dark:text-gray-400">MagicDNS Name</dt>
            <dd class="font-mono text-gray-900 dark:text-gray-100 truncate">{status.MagicDNSName}</dd>
          </div>
        {/if}
        {#if status.TailnetName}
          <div>
            <dt class="text-xs text-gray-500 dark:text-gray-400">Tailnet</dt>
            <dd class="text-gray-900 dark:text-gray-100 truncate">{status.TailnetName}</dd>
          </div>
        {/if}
        {#if status.Version}
          <div>
            <dt class="text-xs text-gray-500 dark:text-gray-400">Version</dt>
            <dd class="font-mono text-gray-900 dark:text-gray-100">{status.Version}</dd>
          </div>
        {/if}
        <div>
          <dt class="text-xs text-gray-500 dark:text-gray-400">Peers</dt>
          <dd class="text-gray-900 dark:text-gray-100">{status.PeerCount ?? 0} ({status.ActivePeers ?? 0} active)</dd>
        </div>
      </dl>

      {#if status.Health && status.Health.length > 0}
        <div class="rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 px-3 py-2 space-y-1">
          {#each status.Health as msg}
            <p class="text-xs text-amber-700 dark:text-amber-400 flex items-start gap-1.5">
              <AlertTriangle size={12} class="mt-0.5 flex-shrink-0" />
              {msg}
            </p>
          {/each}
        </div>
      {/if}
    {:else}
      <p class="text-sm text-gray-400">Waiting for status…</p>
    {/if}

    <!-- Connect / Disconnect -->
    <div class="flex gap-2 pt-1">
      {#if status?.BackendState !== 'Running'}
        <button
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 transition-colors"
          onclick={handleConnect}
          disabled={connectLoading}
        >
          {#if connectLoading}
            <RefreshCw size={14} class="animate-spin" />
          {:else}
            <LogIn size={14} />
          {/if}
          Connect
        </button>
      {:else}
        <button
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50 transition-colors text-gray-700 dark:text-gray-300"
          onclick={handleDisconnect}
          disabled={connectLoading}
        >
          {#if connectLoading}
            <RefreshCw size={14} class="animate-spin" />
          {:else}
            <LogOut size={14} />
          {/if}
          Disconnect
        </button>
      {/if}
    </div>
  </div>

  <!-- Login section — shown when Tailscale needs authentication -->
  {#if status?.BackendState === 'NeedsLogin' || status?.BackendState === 'NoState' || loginURL}
    <div class="rounded-lg border border-amber-200 dark:border-amber-700 bg-amber-50 dark:bg-amber-900/20 p-5 space-y-3">
      <div class="flex items-center gap-2">
        <LogIn size={18} class="text-amber-600 dark:text-amber-400" />
        <h2 class="font-medium text-amber-900 dark:text-amber-200">Authentication Required</h2>
      </div>

      <!-- Tab switcher -->
      <div class="flex gap-1 rounded-md bg-amber-100 dark:bg-amber-900/40 p-0.5 w-fit">
        <button
          class="px-3 py-1 text-xs font-medium rounded transition-colors {authTab === 'url'
            ? 'bg-white dark:bg-gray-800 text-amber-800 dark:text-amber-200 shadow-sm'
            : 'text-amber-700 dark:text-amber-400 hover:text-amber-900 dark:hover:text-amber-200'}"
          onclick={() => { authTab = 'url'; authKeyError = ''; }}
        >
          Login URL
        </button>
        <button
          class="px-3 py-1 text-xs font-medium rounded transition-colors {authTab === 'key'
            ? 'bg-white dark:bg-gray-800 text-amber-800 dark:text-amber-200 shadow-sm'
            : 'text-amber-700 dark:text-amber-400 hover:text-amber-900 dark:hover:text-amber-200'}"
          onclick={() => { authTab = 'key'; stopLoginPoll(); loginURL = ''; }}
        >
          Auth Key
        </button>
      </div>

      {#if authTab === 'url'}
        <!-- Interactive login URL flow -->
        <p class="text-sm text-amber-800 dark:text-amber-300">
          Click below to get a login URL, then open it in your browser to authorize this device.
        </p>

        {#if loginURL}
          <div class="rounded-md border border-amber-300 dark:border-amber-600 bg-white dark:bg-gray-900 px-3 py-2 flex items-center gap-2 overflow-hidden">
            <span class="font-mono text-xs text-gray-700 dark:text-gray-300 truncate flex-1">{loginURL}</span>
            <a
              href={loginURL}
              target="_blank"
              rel="noopener noreferrer"
              class="flex-shrink-0 inline-flex items-center gap-1 text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400"
            >
              Open <ExternalLink size={12} />
            </a>
          </div>
          <p class="text-xs text-amber-700 dark:text-amber-400 flex items-center gap-1">
            <RefreshCw size={12} class="animate-spin" />
            Waiting for authentication to complete…
          </p>
        {:else}
          <button
            class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md bg-amber-600 text-white hover:bg-amber-700 disabled:opacity-50 transition-colors"
            onclick={handleGetLoginURL}
            disabled={loginLoading}
          >
            {#if loginLoading}
              <RefreshCw size={14} class="animate-spin" />
              Getting URL…
            {:else}
              <LogIn size={14} />
              Get Login URL
            {/if}
          </button>
        {/if}
      {:else}
        <!-- Auth key flow -->
        <p class="text-sm text-amber-800 dark:text-amber-300">
          Paste a pre-generated auth key from
          <a
            href="https://login.tailscale.com/admin/machines/new-linux"
            target="_blank"
            rel="noopener noreferrer"
            class="underline hover:text-amber-900 dark:hover:text-amber-100 inline-flex items-center gap-0.5"
          >Tailscale Admin <ExternalLink size={11} /></a>
          to authenticate without a browser.
        </p>

        <div class="space-y-2">
          <div class="flex gap-2">
            <input
              type="password"
              class="flex-1 rounded-md border {authKeyError ? 'border-red-400 dark:border-red-500' : 'border-amber-300 dark:border-amber-600'} bg-white dark:bg-gray-900 px-3 py-1.5 text-sm font-mono text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-amber-500 placeholder-gray-400"
              placeholder="tskey-auth-k…"
              bind:value={authKey}
              onkeydown={(e) => e.key === 'Enter' && handleLoginWithKey()}
              autocomplete="off"
              spellcheck="false"
            />
            <button
              class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md bg-amber-600 text-white hover:bg-amber-700 disabled:opacity-50 transition-colors"
              onclick={handleLoginWithKey}
              disabled={authKeyLoading || !authKey.trim()}
            >
              {#if authKeyLoading}
                <RefreshCw size={14} class="animate-spin" />
                Connecting…
              {:else}
                <LogIn size={14} />
                Connect
              {/if}
            </button>
          </div>
          {#if authKeyError}
            <p class="text-xs text-red-600 dark:text-red-400 flex items-center gap-1">
              <AlertTriangle size={12} />
              {authKeyError}
            </p>
          {/if}
        </div>
      {/if}
    </div>
  {/if}

  <!-- Hostname change -->
  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 p-5 space-y-3">
    <div class="flex items-center gap-2">
      <Server size={18} class="text-gray-500" />
      <h2 class="font-medium text-gray-900 dark:text-gray-100">Hostname</h2>
    </div>

    <div class="rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 px-3 py-2 flex items-start gap-2">
      <AlertTriangle size={14} class="mt-0.5 flex-shrink-0 text-amber-600 dark:text-amber-400" />
      <p class="text-xs text-amber-700 dark:text-amber-300">
        Changing the hostname updates the device name in your tailnet. Any Caddy reverse proxy routes that use the current MagicDNS hostname will need to be updated manually.
      </p>
    </div>

    <div class="flex gap-2">
      <input
        type="text"
        class="flex-1 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-1.5 text-sm font-mono text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500 placeholder-gray-400"
        placeholder="my-device-name"
        bind:value={hostnameInput}
        onkeydown={(e) => e.key === 'Enter' && handleChangeHostname()}
      />
      <button
        class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 transition-colors"
        onclick={handleChangeHostname}
        disabled={hostnameLoading || !hostnameInput.trim()}
      >
        {#if hostnameLoading}
          <RefreshCw size={14} class="animate-spin" />
        {:else}
          <Check size={14} />
        {/if}
        Apply
      </button>
    </div>
  </div>

  <!-- Peers -->
  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 p-5 space-y-3">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Users size={18} class="text-gray-500" />
        <h2 class="font-medium text-gray-900 dark:text-gray-100">Peers</h2>
        {#if peers.length > 0}
          <span class="text-xs text-gray-400">({peers.length})</span>
        {/if}
      </div>
      <button
        class="p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-400 transition-colors"
        onclick={fetchPeers}
        title="Refresh peers"
      >
        <RefreshCw size={14} class={peersLoading ? 'animate-spin' : ''} />
      </button>
    </div>

    {#if peersLoading && peers.length === 0}
      <p class="text-sm text-gray-400">Loading peers…</p>
    {:else if peers.length === 0}
      <p class="text-sm text-gray-400">
        {status?.BackendState === 'Running' ? 'No peers found in this tailnet.' : 'Connect to Tailscale to see peers.'}
      </p>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-xs text-left">
          <thead>
            <tr class="border-b border-gray-200 dark:border-gray-700 text-gray-500 dark:text-gray-400">
              <th class="pb-2 pr-4 font-medium">Hostname</th>
              <th class="pb-2 pr-4 font-medium">IPv4</th>
              <th class="pb-2 pr-4 font-medium hidden sm:table-cell">OS</th>
              <th class="pb-2 pr-4 font-medium">Status</th>
              <th class="pb-2 font-medium hidden sm:table-cell">Last Seen</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-gray-800">
            {#each peers as peer}
              <tr>
                <td class="py-2 pr-4 font-mono text-gray-900 dark:text-gray-100 truncate max-w-[140px]">
                  {peer.DNSName ? peer.DNSName.split('.')[0] : (peer.Hostname || '—')}
                </td>
                <td class="py-2 pr-4 font-mono text-gray-600 dark:text-gray-300">
                  {peer.IPv4 || '—'}
                </td>
                <td class="py-2 pr-4 text-gray-500 dark:text-gray-400 hidden sm:table-cell">
                  {peer.OS || '—'}
                </td>
                <td class="py-2 pr-4">
                  <span class="inline-flex items-center gap-1 {peer.Online ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400'}">
                    <span class="h-1.5 w-1.5 rounded-full {peer.Online ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'}"></span>
                    {peer.Online ? (peer.Active ? 'Active' : 'Online') : 'Offline'}
                  </span>
                </td>
                <td class="py-2 text-gray-400 dark:text-gray-500 hidden sm:table-cell">
                  {peer.Online ? 'just now' : formatLastSeen(peer.LastSeen)}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</div>
