import { writable } from 'svelte/store';
import { fetchJSON } from '../api.js';

/**
 * @typedef {Object} HostMetrics
 * @property {string} host
 * @property {number} requests
 * @property {number} requests_in
 * @property {number} responses_out
 * @property {Object.<string,number>} status_codes  // "2xx","3xx","4xx","5xx"
 */

/**
 * @typedef {Object} UpstreamHealth
 * @property {string} address
 * @property {number} healthy
 */

/**
 * @typedef {Object} MetricsData
 * @property {HostMetrics[]} hosts
 * @property {UpstreamHealth[]} upstreams
 */

/** @type {import('svelte/store').Writable<MetricsData|null>} */
export const metricsData = writable(null);

/** @type {import('svelte/store').Writable<string|null>} */
export const metricsError = writable(null);

/** @type {import('svelte/store').Writable<boolean>} */
export const metricsLoading = writable(false);

export async function fetchMetrics() {
  metricsLoading.set(true);
  metricsError.set(null);
  try {
    const data = await fetchJSON('/api/caddy/metrics');
    metricsData.set(data);
  } catch (err) {
    metricsError.set(err.message);
  } finally {
    metricsLoading.set(false);
  }
}
