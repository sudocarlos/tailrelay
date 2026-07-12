<script>
  import { onMount } from 'svelte';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import { Network, Route, Globe, Terminal, Plus, Trash2, RefreshCw, AlertTriangle } from '@lucide/svelte';

  // `peers` is passed down from Tailscale.svelte, which already fetches
  // /api/tailscale/peers — avoids a duplicate request just for this dropdown.
  let { peers = [] } = $props();

  let networking = $state(null);
  let loading = $state(true);

  // Per-control saving flags — each toggle/select applies immediately and
  // only disables itself while in flight, so unrelated controls stay usable.
  let savingAdvertiseExitNode = $state(false);
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

  async function toggleAdvertiseExitNode(checked) {
    savingAdvertiseExitNode = true;
    try {
      await updateNetworking({ advertise_exit_node: checked });
      networking.AdvertiseExitNode = checked;
      showToast('success', checked ? 'Advertising as an exit node' : 'Stopped advertising as an exit node');
    } catch (err) {
      showToast('danger', err.message || 'Failed to update exit node advertisement');
    } finally {
      savingAdvertiseExitNode = false;
    }
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

  async function changeExitNode(value) {
    savingExitNode = true;
    try {
      await updateNetworking({ exit_node: value });
      networking.ExitNode = value;
      showToast('success', value ? 'Exit node set' : 'Exit node cleared');
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
      if (value === '0.0.0.0/0') return 'Use "Advertise as exit node" instead';
      if (!isIPv4HostBitsZero(addr, bits)) return 'Host bits must be zero, e.g. 192.168.1.0/24';
      return '';
    }

    if (addr.includes(':')) {
      if (!Number.isInteger(bits) || bits < 0 || bits > 128) return 'Prefix length must be 0-128';
      if (value === '::/0') return 'Use "Advertise as exit node" instead';
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
    const name = peer.DNSName ? peer.DNSName.split('.')[0] : peer.Hostname || peer.IPv4;
    return `${name} (${peer.IPv4 || peer.IPv6 || '—'})`;
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
    <!-- Advertise as exit node (+ Allow LAN access sub-option) -->
    <div class="space-y-2">
      <label class="flex items-center justify-between gap-3 cursor-pointer">
        <span class="flex items-center gap-2 text-sm text-gray-900 dark:text-gray-100">
          <Globe size={15} class="text-gray-400" />
          Advertise as exit node
        </span>
        <input
          type="checkbox"
          checked={networking.AdvertiseExitNode}
          disabled={savingAdvertiseExitNode}
          onchange={(e) => toggleAdvertiseExitNode(e.target.checked)}
          class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 dark:bg-gray-800 disabled:opacity-50"
        />
      </label>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        Offer this device as an exit node for internet traffic from the tailnet. Must be approved in the admin console before other devices can use it.
      </p>

      {#if networking.AdvertiseExitNode}
        <label class="flex items-center justify-between gap-3 cursor-pointer pl-5 pt-1">
          <span class="text-sm text-gray-700 dark:text-gray-300">Allow LAN access</span>
          <input
            type="checkbox"
            checked={networking.ExitNodeAllowLANAccess}
            disabled={savingLanAccess}
            onchange={(e) => toggleLanAccess(e.target.checked)}
            class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 dark:bg-gray-800 disabled:opacity-50"
          />
        </label>
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
    <label class="flex items-center justify-between gap-3 cursor-pointer">
      <span class="text-sm text-gray-900 dark:text-gray-100">Accept routes from other nodes</span>
      <input
        type="checkbox"
        checked={networking.AcceptRoutes}
        disabled={savingAcceptRoutes}
        onchange={(e) => toggleAcceptRoutes(e.target.checked)}
        class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 dark:bg-gray-800 disabled:opacity-50"
      />
    </label>

    <hr class="border-gray-100 dark:border-gray-800" />

    <!-- Exit node selection -->
    <div class="space-y-1.5">
      <span class="text-sm text-gray-900 dark:text-gray-100">Use exit node</span>
      <select
        value={networking.ExitNode}
        disabled={savingExitNode}
        onchange={(e) => changeExitNode(e.target.value)}
        class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
      >
        <option value="">None</option>
        {#each exitNodePeers as peer}
          <option value={peer.IPv4 || peer.IPv6}>{peerLabel(peer)}</option>
        {/each}
      </select>
      {#if exitNodePeers.length === 0}
        <p class="text-xs text-gray-500 dark:text-gray-400">
          No peers in this tailnet are advertising as exit nodes yet.
        </p>
      {/if}
    </div>

    <hr class="border-gray-100 dark:border-gray-800" />

    <!-- SSH -->
    <label class="flex items-center justify-between gap-3 cursor-pointer">
      <span class="flex items-center gap-2 text-sm text-gray-900 dark:text-gray-100">
        <Terminal size={15} class="text-gray-400" />
        Run Tailscale SSH
      </span>
      <input
        type="checkbox"
        checked={networking.SSH}
        disabled={savingSSH}
        onchange={(e) => toggleSSH(e.target.checked)}
        class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 dark:bg-gray-800 disabled:opacity-50"
      />
    </label>
  {/if}
</div>
