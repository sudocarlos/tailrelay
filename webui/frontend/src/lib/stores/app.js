import { writable, derived } from 'svelte/store';
import { fetchJSON } from '../api.js';

// ── Core data stores ──────────────────────────────────────────────
export const relays = writable([]);
export const proxies = writable([]);
export const tailnetFQDN = writable('');
export const tailscaleStatus = writable(null);
export const targets = writable([]);

// ── Filter toggles ────────────────────────────────────────────────
export const showRelays = writable(true);
export const showProxies = writable(true);

// ── Derived: filtered items for the dashboard ─────────────────────
export const filteredItems = derived(
  [relays, proxies, showRelays, showProxies],
  ([$relays, $proxies, $showRelays, $showProxies]) => {
    const items = [];
    if ($showRelays) {
      $relays.forEach((r) => items.push({ type: 'relay', relay: r.relay, running: r.running }));
    }
    if ($showProxies) {
      $proxies.forEach((p) => items.push({ type: 'proxy', proxy: p }));
    }
    return items;
  },
);

// ── Logs ──────────────────────────────────────────────────────────
export const logs = writable([]);
export const logLevel = writable('INFO');

// ── Navigation ────────────────────────────────────────────────────
export const currentView = writable('dashboard');

// ── Auth state (set from server) ──────────────────────────────────
export const authenticated = writable(true);
export const needsSetup = writable(false);

// ── Last updated timestamp ────────────────────────────────────────
export const lastUpdated = writable('');

/**
 * Fetch all core data (relays, proxies, tailscale status, targets)
 * in parallel and update stores.
 */
export async function refreshData() {
  const [relayData, proxyData, status, targetData] = await Promise.all([
    fetchJSON('/api/socat/relays'),
    fetchJSON('/api/caddy/proxies'),
    fetchJSON('/api/tailscale/status'),
    fetchJSON('/api/targets'),
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

  tailnetFQDN.set(status.MagicDNSName || status.magicDNSName || '');
  tailscaleStatus.set(status);
  targets.set(targetData || []);
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
