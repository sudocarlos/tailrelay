<script>
  import { onMount, onDestroy } from 'svelte';
  import { tailscaleStatus, refreshData } from '../stores/app.js';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import { Wifi, WifiOff, LogIn, LogOut, RefreshCw, Server, AlertTriangle, ExternalLink, Check, Users } from '@lucide/svelte';
  import CopyButton from './CopyButton.svelte';
  import NetworkingSection from './NetworkingSection.svelte';

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
    if (!ts || ts.startsWith('0001-01-01')) return 'never';
    const d = new Date(ts);
    const diff = Date.now() - d.getTime();
    if (diff < 60000) return 'just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    if (diff < 7 * 86400000) return `${Math.floor(diff / 86400000)}d ago`;
    
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
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
      // Delayed refresh to pick up relays reconciled after Tailscale fully connects
      setTimeout(async () => { await refreshData(); }, 3000);
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
          // Delayed refresh to pick up relays reconciled after Tailscale fully connects
          setTimeout(async () => { await refreshData(); }, 3000);
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
      // Delayed refresh to pick up relays reconciled after Tailscale fully connects
      setTimeout(async () => { await refreshData(); }, 3000);
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

  async function handleLogout() {
    connectLoading = true;
    try {
      await fetchJSON('/api/tailscale/logout', { method: 'POST' });
      showToast('success', 'Logged out of Tailscale');
      await refreshData();
      peers = [];
    } catch (err) {
      showToast('danger', err.message || 'Failed to logout');
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

  function handleWindowClick(e) {
    const clickedDetails = e.target.closest('details.address-dropdown');
    document.querySelectorAll('details.address-dropdown').forEach(d => {
      if (d !== clickedDetails) {
        d.removeAttribute('open');
      }
    });
  }
</script>

<svelte:window onclick={handleWindowClick} />

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
            <dt class="text-xs text-gray-500 dark:text-gray-400 mb-1">Machine name</dt>
            <dd class="flex gap-2">
              <input
                type="text"
                class="flex-1 w-full rounded-md border border-gray-300 dark:border-gray-600 bg-transparent px-2 py-1 text-xs font-mono text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500 placeholder-gray-400 transition-colors"
                bind:value={hostnameInput}
                onkeydown={(e) => e.key === 'Enter' && hostnameInput !== status.Hostname && handleChangeHostname()}
              />
              {#if hostnameInput !== status.Hostname}
                <button
                  class="inline-flex items-center gap-1 px-2 py-1 text-[10px] uppercase tracking-wider font-semibold rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 transition-colors"
                  onclick={handleChangeHostname}
                  disabled={hostnameLoading || !hostnameInput.trim()}
                >
                  {#if hostnameLoading}
                    <RefreshCw size={12} class="animate-spin" />
                  {:else}
                    <Check size={12} />
                  {/if}
                  Apply
                </button>
              {/if}
            </dd>
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
          <dt class="text-xs text-gray-500 dark:text-gray-400">Machines</dt>
          <dd class="text-gray-900 dark:text-gray-100">{status.PeerCount ?? 0} ({status.ActivePeers ?? 0} active)</dd>
        </div>
      </dl>

      {#if status.Health && status.Health.length > 0}
        <div class="rounded-md border border-amber-200 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/20 px-3 py-2 space-y-1">
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

    <!-- Connect / Disconnect / Logout -->
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
      {#if status?.BackendState === 'Running' || status?.BackendState === 'Stopped'}
        <button
          class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md border border-red-300 dark:border-red-700 hover:bg-red-50 dark:hover:bg-red-900/20 disabled:opacity-50 transition-colors text-red-700 dark:text-red-400"
          onclick={handleLogout}
          disabled={connectLoading}
        >
          {#if connectLoading}
            <RefreshCw size={14} class="animate-spin" />
          {:else}
            <LogOut size={14} />
          {/if}
          Logout
        </button>
      {/if}
    </div>
  </div>

  <!-- Networking — placed directly next to connection status since these
       settings are applied via `tailscale set` which requires an active
       daemon; only rendered once connected. -->
  {#if status?.BackendState === 'Running'}
    <NetworkingSection {peers} />
  {/if}

  <!-- Login section — shown when Tailscale needs authentication -->
  {#if status?.BackendState === 'NeedsLogin' || status?.BackendState === 'NoState' || loginURL}
    <div class="rounded-lg border border-amber-200 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/20 p-5 space-y-3">
      <div class="flex items-center gap-2">
        <LogIn size={18} class="text-amber-600 dark:text-amber-400" />
        <h2 class="font-medium text-amber-900 dark:text-amber-200">Authentication Required</h2>
      </div>

      <!-- Tab switcher -->
      <div class="flex gap-1 rounded-full bg-amber-100 dark:bg-amber-900/40 p-0.5 w-fit">
        <button
          class="px-3 py-1 text-xs font-medium rounded-full transition-colors {authTab === 'url'
            ? 'bg-amber-600 text-white'
            : 'text-amber-700 dark:text-amber-400 hover:text-amber-900 dark:hover:text-amber-200'}"
          onclick={() => { authTab = 'url'; authKeyError = ''; }}
        >
          Login URL
        </button>
        <button
          class="px-3 py-1 text-xs font-medium rounded-full transition-colors {authTab === 'key'
            ? 'bg-amber-600 text-white'
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
          Provide a Tailscale auth key (tskey-auth-...) to authenticate headlessly. Generate an auth key at
          <a
            href="https://login.tailscale.com/admin/settings/keys"
            target="_blank"
            rel="noopener noreferrer"
            class="underline hover:text-amber-900 dark:hover:text-amber-100 inline-flex items-center gap-0.5"
          >https://login.tailscale.com/admin/settings/keys <ExternalLink size={11} /></a>
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

  <!-- Machines -->
  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 p-5 space-y-3">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Users size={18} class="text-gray-500" />
        <h2 class="font-medium text-gray-900 dark:text-gray-100">Machines</h2>
        {#if peers.length > 0}
          <span class="text-xs text-gray-400">({peers.length})</span>
        {/if}
      </div>
      <button
        class="p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-400 transition-colors"
        onclick={fetchPeers}
        title="Refresh machines"
      >
        <RefreshCw size={14} class={peersLoading ? 'animate-spin' : ''} />
      </button>
    </div>

    {#if peersLoading && peers.length === 0}
      <p class="text-sm text-gray-400">Loading machines…</p>
    {:else if peers.length === 0}
      <p class="text-sm text-gray-400">
        {status?.BackendState === 'Running' ? 'No machines found in this tailnet.' : 'Connect to Tailscale to see machines.'}
      </p>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-xs text-left whitespace-nowrap">
          <thead>
            <tr class="border-b border-gray-200 dark:border-gray-700 text-gray-500 dark:text-gray-400 uppercase tracking-wider text-[11px]">
              <th class="pb-3 pr-4 font-semibold">Machine</th>
              <th class="pb-3 pr-4 font-semibold">Addresses</th>
              <th class="pb-3 pr-4 font-semibold">Version</th>
              <th class="pb-3 font-semibold">Last Seen</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-gray-800">
            {#each peers as peer}
              <tr>
                <!-- MACHINE -->
                <td class="py-3 pr-4 align-top">
                  <div class="flex flex-col gap-1">
                    <a 
                      href="https://login.tailscale.com/admin/machines/{peer.IPv4 || peer.IPv6 || ''}"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="font-bold text-gray-900 dark:text-gray-100 text-[13px] hover:text-blue-600 dark:hover:text-blue-400 hover:underline transition-colors"
                    >
                      {peer.DNSName ? peer.DNSName.split('.')[0] : (peer.Hostname || '—')}
                    </a>
                    {#if peer.UserEmail}
                      <span class="text-gray-500 dark:text-gray-400 text-xs">
                        {peer.UserEmail}
                      </span>
                    {/if}
                    <div class="flex gap-1.5 mt-0.5">
                      {#if peer.ExitNode}
                        <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-400 border border-blue-200 dark:border-blue-800">
                          Exit Node
                        </span>
                      {/if}
                    </div>
                  </div>
                </td>
                
                <!-- ADDRESSES -->
                <td class="py-3 pr-4 align-top font-mono text-gray-700 dark:text-gray-300">
                  <details class="relative group cursor-pointer address-dropdown">
                    <summary class="list-none outline-none flex items-center gap-1 hover:text-gray-900 dark:hover:text-gray-100">
                      {peer.IPv4 || peer.IPv6 || '—'}
                      <svg class="w-3 h-3 text-gray-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"></polyline></svg>
                    </summary>
                    <div class="absolute left-0 mt-1 w-max min-w-[200px] z-10 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-1 hidden group-open:block">
                      {#if peer.DNSName}
                        <div class="flex items-center justify-between px-3 py-2 text-xs hover:bg-gray-50 dark:hover:bg-gray-700 rounded transition-colors">
                          <span class="truncate">{peer.DNSName}</span>
                          <CopyButton text={peer.DNSName} size={14} label="Copy DNS name" class="ml-4" />
                        </div>
                      {/if}
                      {#if peer.TailscaleIPs && peer.TailscaleIPs.length > 0}
                        {#each peer.TailscaleIPs as ip}
                          <div class="flex items-center justify-between px-3 py-2 text-xs hover:bg-gray-50 dark:hover:bg-gray-700 rounded transition-colors border-t border-gray-100 dark:border-gray-700/50">
                            <span>{ip}</span>
                            <CopyButton text={ip} size={14} label="Copy IP address" class="ml-4" />
                          </div>
                        {/each}
                      {/if}
                    </div>
                  </details>
                </td>
                
                <!-- VERSION -->
                <td class="py-3 pr-4 align-top">
                  <div class="flex flex-col gap-0.5">
                    <span class="text-gray-600 dark:text-gray-400 flex items-center gap-1.5">
                      {peer.OS || '—'}
                    </span>
                  </div>
                </td>
                
                <!-- LAST SEEN -->
                <td class="py-3 align-top">
                  <span class="inline-flex items-center gap-1.5 {peer.Online ? 'text-gray-900 dark:text-gray-100' : 'text-gray-500'}">
                    <span class="h-1.5 w-1.5 rounded-full {peer.Online ? 'bg-emerald-500' : 'bg-gray-400'}"></span>
                    {peer.Online ? 'Connected' : formatLastSeen(peer.LastSeen)}
                  </span>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</div>
