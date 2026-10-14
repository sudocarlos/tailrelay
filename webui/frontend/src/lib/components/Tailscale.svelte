<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import { tailscaleStatus, refreshData, tailscaleSection, tailscaleActiveSection } from '../stores/app.js';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import { Wifi, WifiOff, GlobeLock, GlobeOff, LogIn, LogOut, RefreshCw, Server, AlertTriangle, ExternalLink, Check, Users } from '@lucide/svelte';
  import AddressMenu from './AddressMenu.svelte';
  import NetworkingSection from './NetworkingSection.svelte';
  import { Input } from '@/lib/components/ui/input';
  import { Button } from '@/lib/components/ui/button';
  import * as Tabs from '@/lib/components/ui/tabs';
  import { Separator } from '@/lib/components/ui/separator';

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

  // Control server (Headscale) settings
  let controlServerInput = $state('');
  let controlServerBaseline = $state('');
  let controlServerSaving = $state(false);
  let controlServerError = $state('');

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

  // ── Sidebar sub-item scroll targets ───────────────────────────────
  // The sidebar's Connection/Networking/Machines sub-items write
  // { id, n } into the shared store; scroll there once the view has
  // rendered, then clear the store so re-clicking scrolls again. The
  // active highlight comes from the scroll spy below, so clicking simply
  // scrolls and lets the spy light up the right sub-item.
  $effect(() => {
    const target = $tailscaleSection;
    if (!target?.id) return;
    tick().then(() => {
      document.getElementById(`tailscale-${target.id}`)?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
      });
      tailscaleSection.set(null);
    });
  });

  // Scroll spy: track which section is currently in view so the sidebar
  // sub-items highlight accordingly. On short pages, a bottom-of-page
  // check pins the last section once scrolling has maxed out.
  function updateActiveSection() {
    const ids = ['connection', 'networking', 'machines'];
    let current = 'connection';
    let last = null;
    for (const id of ids) {
      const el = document.getElementById(`tailscale-${id}`);
      if (!el) continue;
      last = id;
      if (el.getBoundingClientRect().top <= 140) current = id;
    }
    if (
      last &&
      window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2
    ) {
      current = last;
    }
    tailscaleActiveSection.set(current);
  }

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

  function formatLastSeen(ts) {
    if (!ts || ts.startsWith('0001-01-01')) return 'never';
    const d = new Date(ts);
    const diff = Date.now() - d.getTime();
    if (diff < 60000) return 'just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    if (diff < 7 * 86400000) return `${Math.floor(diff / 3600000)}d ago`;
    
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
  }

  // Admin-console deep link for a peer machine, keyed by its Tailscale
  // IP (e.g. https://console.tailscale.com/admin/machines/100.98.52.14).
  function machineConsoleUrl(peer) {
    const ip = peer.IPv4 || peer.IPv6;
    if (!ip) return null;
    return `https://console.tailscale.com/admin/machines/${ip}`;
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

  async function fetchControlServer() {
    try {
      const data = await fetchJSON('/api/tailscale/control-server');
      controlServerInput = data.control_server || '';
      controlServerBaseline = controlServerInput;
    } catch {
      // Non-fatal; leave the field blank if it can't be loaded.
    }
  }

  // Best-effort client-side validation for fast feedback; the backend
  // (net/url.Parse) remains the source of truth.
  function validateControlServer(raw) {
    const value = raw.trim();
    if (!value) return '';
    if (!/^https?:\/\/.+/.test(value)) {
      return 'Must be a valid http(s) URL, e.g. https://headscale.example.com';
    }
    return '';
  }

  async function handleSaveControlServer() {
    const value = controlServerInput.trim();
    controlServerError = validateControlServer(value);
    if (controlServerError) return;

    controlServerSaving = true;
    try {
      await fetchJSON('/api/tailscale/control-server/update', {
        method: 'POST',
        body: JSON.stringify({ control_server: value }),
      });
      controlServerInput = value;
      controlServerBaseline = value;
      showToast('success', value ? 'Control server updated' : "Reset to Tailscale's default control plane");
    } catch (err) {
      controlServerError = err.message || 'Failed to update control server';
    } finally {
      controlServerSaving = false;
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
    if (!key.startsWith('tskey-') && !key.startsWith('hskey-')) {
      authKeyError = "Invalid key: must start with 'tskey-' or 'hskey-'";
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
    fetchControlServer();
    updateActiveSection();
    window.addEventListener('scroll', updateActiveSection, { passive: true });
  });

  onDestroy(() => {
    stopLoginPoll();
    window.removeEventListener('scroll', updateActiveSection);
  });
</script>

<!-- Page header -->
<div>
  <h1 class="text-2xl font-semibold tracking-tight">Tailscale</h1>
  <p class="text-sm text-muted-foreground mt-0.5">Connect this node to your tailnet, manage networking and machines</p>
</div>

<!-- Connection -->
<section id="tailscale-connection" class="mt-8 scroll-mt-4">
  <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
    <div class="flex items-center gap-2">
      {#if status?.BackendState === 'Running'}
        <Wifi size={18} class="text-emerald-500" />
      {:else}
        <WifiOff size={18} class="text-muted-foreground" />
      {/if}
      <h2 class="text-lg font-semibold">Connection</h2>
      <span class="text-sm text-muted-foreground">{stateLabel(status)}</span>
    </div>

    <!-- Connect / Disconnect / Logout -->
    <!-- Connect only makes sense from "Stopped" (previously authenticated,
         just brought down): plain `tailscale up` reuses the existing node
         identity. For "NeedsLogin"/"NoState" that identity may be stale
         (e.g. authkey already used after tailnet removal), so those states
         only offer the Authentication Required form below. "Starting" is
         transient and intentionally shows no action button here. -->
    <div class="flex gap-2">
      {#if status?.BackendState === 'Stopped'}
        <Button
          class="gap-1.5"
          onclick={handleConnect}
          disabled={connectLoading}
        >
          {#if connectLoading}
            <RefreshCw size={14} class="animate-spin" />
          {:else}
            <GlobeLock size={14} />
          {/if}
          Connect
        </Button>
      {:else if status?.BackendState === 'Running'}
        <Button
          variant="outline"
          class="gap-1.5"
          onclick={handleDisconnect}
          disabled={connectLoading}
        >
          {#if connectLoading}
            <RefreshCw size={14} class="animate-spin" />
          {:else}
            <GlobeOff size={14} />
          {/if}
          Disconnect
        </Button>
      {/if}
      {#if status?.BackendState === 'Running' || status?.BackendState === 'Stopped'}
        <Button
          variant="outline"
          class="gap-1.5 border-destructive/50 hover:bg-destructive/10 text-destructive"
          onclick={handleLogout}
          disabled={connectLoading}
        >
          {#if connectLoading}
            <RefreshCw size={14} class="animate-spin" />
          {:else}
            <LogOut size={14} />
          {/if}
          Logout
        </Button>
      {/if}
    </div>
  </div>

  {#if status}
    <dl class="grid grid-cols-2 gap-x-6 gap-y-3 text-sm sm:grid-cols-3">
      {#if status.Hostname}
        <div>
          <dt class="mb-1 text-xs text-muted-foreground">Machine name</dt>
          <dd class="flex gap-2">
            <Input
              type="text"
              class="h-7 flex-1 font-mono text-xs"
              bind:value={hostnameInput}
              onkeydown={(e) => e.key === 'Enter' && hostnameInput !== status.Hostname && handleChangeHostname()}
            />
            {#if hostnameInput !== status.Hostname}
              <Button
                size="xs"
                class="tracking-wider"
                onclick={handleChangeHostname}
                disabled={hostnameLoading || !hostnameInput.trim()}
              >
                {#if hostnameLoading}
                  <RefreshCw size={12} class="animate-spin" />
                {:else}
                  <Check size={12} />
                {/if}
                Apply
              </Button>
            {/if}
          </dd>
        </div>
      {/if}
      {#if status.IPv4}
        <div>
          <dt class="text-xs text-muted-foreground">Tailscale IPv4</dt>
          <dd class="font-mono text-foreground">{status.IPv4}</dd>
        </div>
      {/if}
      {#if status.MagicDNSName}
        <div>
          <dt class="text-xs text-muted-foreground">MagicDNS Name</dt>
          <dd class="truncate font-mono text-foreground">{status.MagicDNSName}</dd>
        </div>
      {/if}
      {#if status.IsCustomControlServer && status.ControlServer}
        <div>
          <dt class="text-xs text-muted-foreground">Control Server</dt>
          <dd class="truncate font-mono text-foreground" title={status.ControlServer}>{status.ControlServer}</dd>
        </div>
      {/if}
      {#if status.TailnetName}
        <div>
          <dt class="text-xs text-muted-foreground">Tailnet</dt>
          <dd class="truncate text-foreground">{status.TailnetName}</dd>
        </div>
      {/if}
      {#if status.Version}
        <div>
          <dt class="text-xs text-muted-foreground">Version</dt>
          <dd class="font-mono text-foreground">{status.Version}</dd>
        </div>
      {/if}
      <div>
        <dt class="text-xs text-muted-foreground">Machines</dt>
        <dd class="text-foreground">{status.PeerCount ?? 0} ({status.ActivePeers ?? 0} active)</dd>
      </div>
    </dl>

    {#if status.Health && status.Health.length > 0}
      <div class="mt-4 space-y-1 border-l-2 border-amber-400 pl-3">
        {#each status.Health as msg}
          <p class="flex items-start gap-1.5 text-xs text-amber-700 dark:text-amber-400">
            <AlertTriangle size={12} class="mt-0.5 flex-shrink-0" />
            {msg}
          </p>
        {/each}
      </div>
    {/if}
  {:else}
    <p class="text-sm text-muted-foreground">Waiting for status…</p>
  {/if}

  <!-- Login section — shown when Tailscale needs authentication. Flat
       (amber text accents, no card box) to match the console's banner
       styling. -->
  {#if status?.BackendState === 'NeedsLogin' || status?.BackendState === 'NoState' || loginURL}
    <div class="mt-6 border-l-2 border-amber-400 pl-3 sm:pl-4">
      <div class="flex items-center gap-2">
        <LogIn size={16} class="text-amber-800 dark:text-amber-400" />
        <h3 class="font-medium text-amber-900 dark:text-amber-200">Authentication Required</h3>
      </div>

      <!-- Tab switcher -->
      <Tabs.Root value={authTab} class="mt-3 w-fit">
        <Tabs.List>
          <Tabs.Trigger
            value="url"
            onclick={() => { authTab = 'url'; authKeyError = ''; }}
          >
            Login URL
          </Tabs.Trigger>
          <Tabs.Trigger
            value="key"
            onclick={() => { authTab = 'key'; stopLoginPoll(); loginURL = ''; }}
          >
            Auth Key
          </Tabs.Trigger>
        </Tabs.List>
      </Tabs.Root>

      {#if authTab === 'url'}
        <!-- Interactive login URL flow -->
        <p class="mt-2 text-sm text-amber-800 dark:text-amber-300">
          Click below to get a login URL, then open it in your browser to authorize this device.
        </p>

        {#if loginURL}
          <div class="mt-2 flex items-center gap-2 overflow-hidden rounded-md border bg-background px-3 py-2">
            <span class="flex-1 truncate font-mono text-xs text-muted-foreground">{loginURL}</span>
            <a
              href={loginURL}
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex flex-shrink-0 items-center gap-1 text-xs text-primary hover:text-primary/80"
            >
              Open <ExternalLink size={12} />
            </a>
          </div>
          <p class="mt-2 flex items-center gap-1 text-xs text-amber-800 dark:text-amber-400">
            <RefreshCw size={12} class="animate-spin" />
            Waiting for authentication to complete…
          </p>
        {:else}
          <Button
            class="mt-2 bg-amber-600 text-amber-50 hover:bg-amber-700"
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
          </Button>
        {/if}
      {:else}
        <!-- Auth key flow -->
        <p class="mt-2 text-sm text-amber-800 dark:text-amber-300">
          Provide a Tailscale auth key (tskey-auth-...) or Headscale auth key (hskey-...) to authenticate headlessly. Generate a Tailscale auth key at
          <a
            href="https://login.tailscale.com/admin/settings/keys"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-0.5 underline hover:text-amber-900 dark:hover:text-amber-100"
          >https://login.tailscale.com/admin/settings/keys <ExternalLink size={11} /></a>
        </p>

        <div class="mt-3 space-y-2">
          <div class="flex gap-2">
            <Input
              type="password"
              aria-invalid={!!authKeyError}
              class="flex-1 font-mono"
              placeholder="tskey-auth-k… or hskey-…"
              bind:value={authKey}
              onkeydown={(e) => e.key === 'Enter' && handleLoginWithKey()}
              autocomplete="off"
              spellcheck="false"
            />
            <Button
              class="bg-amber-600 text-amber-50 hover:bg-amber-700"
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
            </Button>
          </div>
          {#if authKeyError}
            <p class="flex items-center gap-1 text-xs text-destructive">
              <AlertTriangle size={12} />
              {authKeyError}
            </p>
          {/if}
        </div>
      {/if}

      <!-- Control server (Headscale) — full-width since URLs run long.
           Explicitly gated on BackendState (not just the parent section's
           condition, which also stays true briefly via `|| loginURL` after
           a successful login until the poll clears it): an already-
           registered device keeps using the control server it
           authenticated against until logout, so this must never flash
           while actually connected. -->
      {#if status?.BackendState === 'NeedsLogin' || status?.BackendState === 'NoState'}
        <div class="mt-4 space-y-1.5 border-t border-amber-400/40 pt-3">
          <div class="flex items-center gap-1.5">
            <Server size={13} class="text-amber-800 dark:text-amber-400" />
            <span class="text-xs font-medium text-amber-800 dark:text-amber-300">Control Server</span>
          </div>
          <div class="flex gap-2">
            <Input
              type="text"
              aria-invalid={!!controlServerError}
              class="font-mono text-xs"
              placeholder="https://headscale.example.com"
              bind:value={controlServerInput}
              oninput={() => { controlServerError = ''; }}
              onkeydown={(e) => e.key === 'Enter' && controlServerInput.trim() !== controlServerBaseline && handleSaveControlServer()}
              autocomplete="off"
              spellcheck="false"
            />
            {#if controlServerInput.trim() !== controlServerBaseline}
              <Button
                size="xs"
                class="bg-amber-600 text-amber-50 hover:bg-amber-700"
                onclick={handleSaveControlServer}
                disabled={controlServerSaving}
              >
                {#if controlServerSaving}
                  <RefreshCw size={12} class="animate-spin" />
                {:else}
                  <Check size={12} />
                {/if}
                Apply
              </Button>
            {/if}
          </div>
          {#if controlServerError}
            <p class="flex items-center gap-1 text-xs text-destructive">
              <AlertTriangle size={12} />
              {controlServerError}
            </p>
          {/if}
          <p class="text-xs text-amber-800 dark:text-amber-400">
            For self-hosted <a
              href="https://headscale.net"
              target="_blank"
              rel="noopener noreferrer"
              class="underline hover:text-amber-900 dark:hover:text-amber-200"
            >Headscale</a> servers — used as <code class="font-mono">tailscale login --login-server</code>. Leave empty to use Tailscale's official control plane.
          </p>
        </div>
      {/if}
    </div>
  {/if}
</section>

<Separator class="my-8" />

<!-- Networking — placed directly next to connection status since these
     settings are applied via `tailscale set` which requires an active
     daemon; only rendered once connected. -->
{#if status?.BackendState === 'Running'}
  <section id="tailscale-networking" class="scroll-mt-4">
    <h2 class="mb-4 text-lg font-semibold">Networking</h2>
    <NetworkingSection {peers} />
  </section>

  <Separator class="my-8" />
{/if}

<!-- Machines -->
<section id="tailscale-machines" class="scroll-mt-4">
  <div class="mb-4 flex items-center justify-between">
    <div class="flex items-baseline gap-2">
      <h2 class="text-lg font-semibold">Machines</h2>
      {#if peers.length > 0}
        <span class="text-sm text-muted-foreground">({peers.length})</span>
      {/if}
    </div>
    <Button
      variant="ghost"
      size="icon-xs"
      class="text-muted-foreground"
      onclick={fetchPeers}
      title="Refresh machines"
    >
      <RefreshCw size={14} class={peersLoading ? 'animate-spin' : ''} />
    </Button>
  </div>

  {#if peersLoading && peers.length === 0}
    <p class="text-sm text-muted-foreground">Loading machines…</p>
  {:else if peers.length === 0}
    <p class="text-sm text-muted-foreground">
      {status?.BackendState === 'Running' ? 'No machines found in this tailnet.' : 'Connect to Tailscale to see machines.'}
    </p>
  {:else}
    <table class="w-full">
      <thead>
        <tr class="border-b border-border text-left text-xs font-normal text-muted-foreground">
          <th class="w-[30%] pb-2 pr-4 sm:w-[26%]">Machine</th>
          <th class="hidden pb-2 pr-4 sm:table-cell">Addresses</th>
          <th class="hidden pb-2 pr-4 sm:table-cell">OS</th>
          <th class="pb-2 pr-4 sm:pb-3">Last Seen</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-border">
        {#each peers as peer}
          {@const consoleUrl = machineConsoleUrl(peer)}
          <tr class="transition-colors hover:bg-muted/40">
            <!-- MACHINE -->
            <td class="py-3 pr-4 align-top">
              <div class="flex flex-col gap-0.5">
                {#if consoleUrl}
                  <a
                    href={consoleUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    title="Open in the Tailscale admin console"
                    class="font-bold text-[13px] text-foreground transition-colors hover:text-primary hover:underline"
                  >
                    {peer.DNSName ? peer.DNSName.split('.')[0] : (peer.Hostname || '—')}
                    <ExternalLink size={11} class="ml-0.5 inline-block align-baseline text-muted-foreground/70" />
                  </a>
                {:else}
                  <span class="font-bold text-[13px] text-foreground">
                    {peer.DNSName ? peer.DNSName.split('.')[0] : (peer.Hostname || '—')}
                  </span>
                {/if}
                {#if peer.UserEmail}
                  <span class="text-xs text-muted-foreground">
                    {peer.UserEmail}
                  </span>
                {/if}
                <div class="mt-0.5 flex gap-1.5">
                  {#if peer.ExitNode}
                    <span class="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium bg-primary/10 text-primary border border-primary/20">
                      Exit Node
                    </span>
                  {/if}
                </div>
              </div>
            </td>

            <!-- ADDRESSES -->
            <td class="hidden py-3 pr-4 align-top font-mono text-xs text-muted-foreground sm:table-cell">
              <AddressMenu {peer} />
            </td>

            <!-- OS -->
            <td class="hidden py-3 pr-4 align-top sm:table-cell">
              <div class="flex flex-col gap-0.5">
                <span class="flex items-center gap-1.5 text-xs text-muted-foreground">
                  {peer.OS || '—'}
                </span>
              </div>
            </td>

            <!-- LAST SEEN -->
            <td class="py-3 align-top">
              <span class="inline-flex items-center gap-1.5 text-xs {peer.Online ? 'text-foreground' : 'text-muted-foreground'}">
                <span class="h-1.5 w-1.5 rounded-full {peer.Online ? 'bg-emerald-500' : 'bg-muted-foreground/50'}"></span>
                {peer.Online ? 'Connected' : formatLastSeen(peer.LastSeen)}
              </span>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>
