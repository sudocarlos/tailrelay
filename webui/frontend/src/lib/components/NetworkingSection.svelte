<script>
  import { onMount } from 'svelte';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import { Network, Route, Terminal, Plus, Trash2, RefreshCw, AlertTriangle } from '@lucide/svelte';
  import Toggle from './Toggle.svelte';

  // `peers` is passed down from Tailscale.svelte, which already fetches
  // /api/tailscale/peers — avoids a duplicate request just for this dropdown.
  let { peers = [] } = $props();

  // Sentinel value for the "Run as exit node" option in the exit-node
  // dropdown, distinguishing it from a real peer IP or the "None" ('') value.
  const ADVERTISE_SELF_VALUE = '__advertise_self__';

  let networking = $state(null);
  let loading = $state(true);

  // Per-control saving flags — each toggle/select applies immediately and
  // only disables itself while in flight, so unrelated controls stay usable.
  let savingLanAccess = $state(false);
  let savingAcceptRoutes = $state(false);
  let savingExitNode = $state(false);
  let savingSSH = $state(false);
  let savingRoutes = $state(false);

  // Advertise-routes editor: a list of free-text CIDR inputs, applied together
  // via an explicit "Apply" button (same pattern as the hostname field on the
  // main Tailscale page — avoid firing a request on every keystroke).
  let routesInput = $state(['']);
  let routesErrors = $state(['']);
  let routesBaseline = $state([]);

  const exitNodePeers = $derived(peers.filter((p) => p.ExitNode));

  // Under Tailscale's userspace-networking mode (tailrelay's only mode) this
  // node has no kernel TUN device and cannot route its own traffic through a
  // peer exit node, so the dropdown only offers "None" and "Run as exit node".
  const userspaceNetworking = $derived(!!networking?.UserspaceNetworking);

  // The dropdown's current value: the advertise-self sentinel, a peer IP, or
  // '' for None. AdvertiseExitNode and ExitNode are mutually exclusive in
  // this UI — selecting one clears the other (see handleExitNodeChange).
  const exitNodeSelection = $derived(networking?.AdvertiseExitNode ? ADVERTISE_SELF_VALUE : networking?.ExitNode || '');

  // "Allow LAN access" only takes effect while actively using another peer
  // as an exit node — it has no effect while merely advertising this device
  // as one, so it's shown only for that selection.
  const usingPeerExitNode = $derived(!!networking?.ExitNode && !networking?.AdvertiseExitNode);

  const routesDirty = $derived.by(() => {
    const cleaned = routesInput.map((r) => r.trim()).filter(Boolean);
    return JSON.stringify([...cleaned].sort()) !== JSON.stringify([...routesBaseline].sort());
  });

  async function fetchNetworking() {
    loading = true;
    try {
      networking = await fetchJSON('/api/tailscale/networking');
      routesBaseline = networking.AdvertiseRoutes || [];
      routesInput = routesBaseline.length > 0 ? [...routesBaseline] : [''];
      routesErrors = routesInput.map(() => '');
    } catch (err) {
      showToast('danger', err.message || 'Failed to load networking settings');
    } finally {
      loading = false;
    }
  }

  function updateNetworking(partial) {
    return fetchJSON('/api/tailscale/networking/update', {
      method: 'POST',
      body: JSON.stringify(partial),
    });
  }

  async function toggleLanAccess(checked) {
    savingLanAccess = true;
    try {
      await updateNetworking({ exit_node_allow_lan_access: checked });
      networking.ExitNodeAllowLANAccess = checked;
    } catch (err) {
      showToast('danger', err.message || 'Failed to update LAN access setting');
    } finally {
      savingLanAccess = false;
    }
  }

  async function toggleAcceptRoutes(checked) {
    savingAcceptRoutes = true;
    try {
      await updateNetworking({ accept_routes: checked });
      networking.AcceptRoutes = checked;
      showToast('success', checked ? 'Now accepting routes from other nodes' : 'Stopped accepting routes from other nodes');
    } catch (err) {
      showToast('danger', err.message || 'Failed to update accept routes setting');
    } finally {
      savingAcceptRoutes = false;
    }
  }

  async function toggleSSH(checked) {
    savingSSH = true;
    try {
      await updateNetworking({ ssh: checked });
      networking.SSH = checked;
      showToast('success', checked ? 'Tailscale SSH enabled' : 'Tailscale SSH disabled');
    } catch (err) {
      showToast('danger', err.message || 'Failed to update SSH setting');
    } finally {
      savingSSH = false;
    }
  }

  // Handles the unified exit-node dropdown: selecting "Advertise this
  // device" enables advertise_exit_node and clears any in-use peer exit
  // node; selecting a peer does the reverse; "None" clears both.
  async function handleExitNodeChange(value) {
    const advertiseSelf = value === ADVERTISE_SELF_VALUE;
    const peerExitNode = advertiseSelf ? '' : value;

    savingExitNode = true;
    try {
      await updateNetworking({ advertise_exit_node: advertiseSelf, exit_node: peerExitNode });
      networking.AdvertiseExitNode = advertiseSelf;
      networking.ExitNode = peerExitNode;
      if (advertiseSelf) {
        showToast('success', 'Advertising as an exit node');
      } else if (peerExitNode) {
        showToast('success', 'Exit node set');
      } else {
        showToast('success', 'Exit node cleared');
      }
    } catch (err) {
      showToast('danger', err.message || 'Failed to update exit node');
    } finally {
      savingExitNode = false;
    }
  }

  // ── Advertise routes editor ───────────────────────────────────────
  function addRouteRow() {
    routesInput = [...routesInput, ''];
    routesErrors = [...routesErrors, ''];
  }

  function removeRouteRow(idx) {
    const nextInput = routesInput.filter((_, i) => i !== idx);
    const nextErrors = routesErrors.filter((_, i) => i !== idx);
    routesInput = nextInput.length > 0 ? nextInput : [''];
    routesErrors = nextErrors.length > 0 ? nextErrors : [''];
  }

  function ipv4ToUint(octets) {
    return ((octets[0] << 24) | (octets[1] << 16) | (octets[2] << 8) | octets[3]) >>> 0;
  }

  function isIPv4HostBitsZero(addr, bits) {
    const octets = addr.split('.').map(Number);
    const value = ipv4ToUint(octets);
    const mask = bits === 0 ? 0 : (0xffffffff << (32 - bits)) >>> 0;
    return (value & ~mask) >>> 0 === 0;
  }

  // Best-effort client-side validation for fast feedback; the backend
  // (net/netip.ParsePrefix) remains the source of truth.
  function validateRoute(raw) {
    const value = raw.trim();
    if (!value) return '';

    const parts = value.split('/');
    if (parts.length !== 2) return 'Must be a CIDR, e.g. 192.168.1.0/24';

    const [addr, bitsStr] = parts;
    const bits = Number(bitsStr);
    const ipv4Match = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(addr);

    if (ipv4Match) {
      const octets = ipv4Match.slice(1).map(Number);
      if (octets.some((o) => o > 255)) return 'Invalid IPv4 address';
      if (!Number.isInteger(bits) || bits < 0 || bits > 32) return 'Prefix length must be 0-32';
      if (value === '0.0.0.0/0') return 'Select "Run as exit node" in the exit node dropdown instead';
      if (!isIPv4HostBitsZero(addr, bits)) return 'Host bits must be zero, e.g. 192.168.1.0/24';
      return '';
    }

    if (addr.includes(':')) {
      if (!Number.isInteger(bits) || bits < 0 || bits > 128) return 'Prefix length must be 0-128';
      if (value === '::/0') return 'Select "Run as exit node" in the exit node dropdown instead';
      return '';
    }

    return 'Must be a valid IPv4 or IPv6 CIDR';
  }

  function handleRouteInput(idx, value) {
    routesInput[idx] = value;
    routesErrors[idx] = validateRoute(value);
  }

  async function applyRoutes() {
    const cleaned = routesInput.map((r) => r.trim()).filter(Boolean);
    const errors = cleaned.map(validateRoute);
    if (errors.some(Boolean)) {
      showToast('danger', 'Fix the highlighted routes before applying');
      return;
    }

    savingRoutes = true;
    try {
      await updateNetworking({ advertise_routes: cleaned });
      networking.AdvertiseRoutes = cleaned;
      routesBaseline = cleaned;
      routesInput = cleaned.length > 0 ? [...cleaned] : [''];
      routesErrors = routesInput.map(() => '');
      showToast('success', 'Advertised routes updated');
    } catch (err) {
      showToast('danger', err.message || 'Failed to update advertised routes');
    } finally {
      savingRoutes = false;
    }
  }

  function peerLabel(peer) {
    return peer.DNSName ? peer.DNSName.split('.')[0] : peer.Hostname || peer.IPv4 || peer.IPv6 || 'Unknown';
  }

  onMount(() => {
    fetchNetworking();
  });
</script>

<div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 p-5 space-y-5">
  <div class="flex items-center gap-2">
    <Network size={18} class="text-gray-500" />
    <h2 class="font-medium text-gray-900 dark:text-gray-100">Networking</h2>
  </div>

  {#if loading}
    <p class="text-sm text-gray-400">Loading networking settings…</p>
  {:else if networking}
    <!-- Exit node selection (includes advertising this device as one) + Allow LAN access sub-option -->
    <div class="space-y-2">
      <span class="text-sm text-gray-900 dark:text-gray-100">Exit node</span>
      <select
        value={exitNodeSelection}
        disabled={savingExitNode}
        onchange={(e) => handleExitNodeChange(e.target.value)}
        class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
      >
        <option value="">None</option>
        <option value={ADVERTISE_SELF_VALUE}>Run as exit node</option>
        {#if !userspaceNetworking}
          <hr />
          {#each exitNodePeers as peer}
            <option value={peer.IPv4 || peer.IPv6}>{peerLabel(peer)}</option>
          {/each}
        {/if}
      </select>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {#if userspaceNetworking}
          This container runs Tailscale in userspace-networking mode, so routing its own internet traffic through a peer exit node isn't supported. "Run as exit node" (serving inbound tailnet traffic) still works.
        {:else}
          Running as an exit node must be approved in the admin console before other devices can use it.
          Selecting a peer routes your internet traffic through it instead.
        {/if}
      </p>

      {#if usingPeerExitNode}
        <div class="flex items-center justify-between gap-3 pl-5 pt-1">
          <span class="text-sm text-gray-700 dark:text-gray-300">Allow LAN access</span>
          <Toggle
            checked={networking.ExitNodeAllowLANAccess}
            disabled={savingLanAccess}
            onChange={toggleLanAccess}
            label="Allow LAN access while using an exit node"
          />
        </div>
      {/if}
    </div>

    <hr class="border-gray-100 dark:border-gray-800" />

    <!-- Advertise routes -->
    <div class="space-y-2">
      <div class="flex items-center gap-2 text-sm text-gray-900 dark:text-gray-100">
        <Route size={15} class="text-gray-400" />
        Advertise routes
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        Expose subnet routes on your local network to the rest of the tailnet.
      </p>

      <div class="space-y-2">
        {#each routesInput as route, idx}
          <div class="flex items-start gap-2">
            <div class="flex-1">
              <input
                type="text"
                value={route}
                oninput={(e) => handleRouteInput(idx, e.target.value)}
                placeholder="e.g. 192.168.1.0/24"
                class="w-full rounded-md border {routesErrors[idx] ? 'border-red-400 dark:border-red-500' : 'border-gray-300 dark:border-gray-600'} bg-transparent px-2 py-1.5 text-xs font-mono focus:outline-none focus:ring-2 focus:ring-blue-500 placeholder-gray-400"
              />
              {#if routesErrors[idx]}
                <p class="mt-1 text-xs text-red-600 dark:text-red-400 flex items-center gap-1">
                  <AlertTriangle size={11} />
                  {routesErrors[idx]}
                </p>
              {/if}
            </div>
            <button
              type="button"
              class="p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
              onclick={() => removeRouteRow(idx)}
              title="Remove route"
            >
              <Trash2 size={14} />
            </button>
          </div>
        {/each}
      </div>

      <div class="flex items-center justify-between pt-1">
        <button
          type="button"
          class="inline-flex items-center gap-1 text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400"
          onclick={addRouteRow}
        >
          <Plus size={13} />
          Add route
        </button>

        {#if routesDirty}
          <button
            type="button"
            class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 transition-colors"
            onclick={applyRoutes}
            disabled={savingRoutes}
          >
            {#if savingRoutes}
              <RefreshCw size={12} class="animate-spin" />
            {/if}
            Apply
          </button>
        {/if}
      </div>
    </div>

    <hr class="border-gray-100 dark:border-gray-800" />

    <!-- Accept routes -->
    <div class="flex items-center justify-between gap-3">
      <span class="text-sm text-gray-900 dark:text-gray-100">Accept routes from other nodes</span>
      <Toggle
        checked={networking.AcceptRoutes}
        disabled={savingAcceptRoutes}
        onChange={toggleAcceptRoutes}
        label="Accept routes from other nodes"
      />
    </div>

    <hr class="border-gray-100 dark:border-gray-800" />

    <!-- SSH -->
    <div class="flex items-center justify-between gap-3">
      <span class="flex items-center gap-2 text-sm text-gray-900 dark:text-gray-100">
        <Terminal size={15} class="text-gray-400" />
        Run Tailscale SSH
      </span>
      <Toggle checked={networking.SSH} disabled={savingSSH} onChange={toggleSSH} label="Run Tailscale SSH" />
    </div>
  {/if}
</div>

