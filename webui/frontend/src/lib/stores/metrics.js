import { writable } from 'svelte/store';

/**
 * Metrics are no longer available — caddy/socat have been removed.
 * These stubs preserve the store exports so existing component imports
 * compile without modification until the Metrics component is updated.
 *
 * TODO: Replace with tailscale serve metrics if/when they become available.
 */

/** @type {import('svelte/store').Writable<null>} */
export const metricsData = writable(null);

/** @type {import('svelte/store').Writable<string|null>} */
export const metricsError = writable(null);

/** @type {import('svelte/store').Writable<boolean>} */
export const metricsLoading = writable(false);

/** @type {import('svelte/store').Writable<boolean>} */
export const metricsResetting = writable(false);

/**
 * Active time window — retained as a no-op store for compatibility.
 * @type {import('svelte/store').Writable<''|'1h'|'1d'|'1w'|'1m'>}
 */
export const metricsWindow = writable('');

/** No-op: metrics are not available without Caddy. */
export async function fetchMetrics(_window = '') {
  metricsData.set(null);
}

/** No-op: metrics are not available without Caddy. */
export async function resetMetrics(_window = '') {
  metricsData.set(null);
}
