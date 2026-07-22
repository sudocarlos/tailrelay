import { writable, derived, get } from 'svelte/store';
import { fetchJSON } from '../api.js';

// ── Core data stores ──────────────────────────────────────────────
export const relays = writable([]);
export const proxies = writable([]);
export const funnels = writable([]);
export const tailnetFQDN = writable('');
export const tailscaleStatus = writable(null);
export const targets = writable([]);
export const controlServer = writable('');

// ── Funnel-eligible listen ports (see internal/serve.FunnelPorts) ─
export const FUNNEL_PORTS = [443, 8443, 10000];

// ── Derived: all items for the dashboard ─────────────────────────
export const filteredItems = derived(
  [relays, proxies],
  ([$relays, $proxies]) => {
    const items = [];
    $relays.forEach((r) => items.push({ type: 'relay', relay: r.relay, running: r.running }));
    $proxies.forEach((p) => items.push({ type: 'proxy', proxy: p }));
    return items;
  },
);

// ── Derived: funnel-eligible ports already occupied by a serve relay ──
// A funnel port can't be configured while a tcp/https serve relay is
// already listening on that same port.
export const usedFunnelPorts = derived(
  [relays, proxies],
  ([$relays, $proxies]) => {
    const used = new Map();
    $relays.forEach((r) => {
      if (FUNNEL_PORTS.includes(r.relay.listen_port)) {
        used.set(r.relay.listen_port, { type: 'relay', id: r.relay.id });
      }
    });
    $proxies.forEach((p) => {
      if (FUNNEL_PORTS.includes(p.listen_port)) {
        used.set(p.listen_port, { type: 'proxy', id: p.id });
      }
    });
    return used;
  },
);

// ── Logs ──────────────────────────────────────────────────────────
export const logs = writable([]);
export const logLevel = writable('INFO');

// ── Derived: is Tailscale connected? ─────────────────────────────
// True only when BackendState is "Running"; false for NeedsLogin,
// NoState, Stopped, Starting, or unknown.
export const tailscaleConnected = derived(
  tailscaleStatus,
  ($s) => $s?.BackendState === 'Running',
);

// ── Derived: hide Funnel while connected to a custom control server ──
// Funnel is a Tailscale-cloud-only feature not supported by self-hosted
// Headscale, so it's only hidden once actually connected under one —
// not merely because a control server URL happens to be saved.
export const hideFunnel = derived(
  [tailscaleConnected, controlServer],
  ([$connected, $controlServer]) => $connected && $controlServer !== '',
);

// ── Navigation ────────────────────────────────────────────────────
export const currentView = writable('dashboard');

// ── Auth state (set from server) ──────────────────────────────────
export const authenticated = writable(true);
export const needsSetup = writable(false);

// ── Last updated timestamp ────────────────────────────────────────
export const lastUpdated = writable('');

/**
 * Fetch all core data (relays, proxies, funnels, tailscale status, targets)
 * in parallel and update stores.
 */
export async function refreshData() {
  const [relayData, proxyData, funnelData, status, targetData, controlServerData] = await Promise.all([
    fetchJSON('/api/serve/tcp/list'),
    fetchJSON('/api/serve/https/list'),
    fetchJSON('/api/serve/funnel/list'),
    fetchJSON('/api/tailscale/status'),
    fetchJSON('/api/targets'),
    fetchJSON('/api/tailscale/control-server'),
  ]);

  relays.set(
    relayData.map((s) => ({
      relay: s.Relay || s.relay,
      running: s.Running ?? s.running,
    })),
  );

  proxies.set(
    proxyData.map((p) => ({
      ...p,
      running: p.running ?? p.Running,
    })),
  );

  funnels.set(
    funnelData.map((f) => ({
      ...f,
      running: f.running ?? f.Running,
    })),
  );

  tailnetFQDN.set(status.MagicDNSName || status.magicDNSName || '');
  tailscaleStatus.set(status);
  targets.set(targetData || []);
  controlServer.set(controlServerData?.control_server || '');
  lastUpdated.set(new Date().toLocaleTimeString());
}

export async function logout() {
  try {
    await fetchJSON('/api/auth/logout', { method: 'POST' });
  } finally {
    authenticated.set(false);
    window.location.href = '/login';
  }
}
