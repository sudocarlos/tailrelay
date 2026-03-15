import { writable } from 'svelte/store';
import { fetchJSON } from '../api.js';

/**
 * @typedef {Object} HostMetrics
 * @property {string} host
 * @property {string} label    — compact ":port → target" form; may be empty
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

/**
 * Active time window.  '' means all-time (no ?window= param sent).
 * @type {import('svelte/store').Writable<''|'1h'|'1d'|'1w'|'1m'>}
 */
export const metricsWindow = writable('');

/**
 * Fetch metrics from the backend for the given window.
 * @param {''|'1h'|'1d'|'1w'|'1m'} [window=''] - time window filter
 */
export async function fetchMetrics(window = '') {
  metricsLoading.set(true);
  metricsError.set(null);
  try {
    const url = window ? `/api/caddy/metrics?window=${window}` : '/api/caddy/metrics';
    const data = await fetchJSON(url);
    metricsData.set(data);
  } catch (err) {
    metricsError.set(err.message);
  } finally {
    metricsLoading.set(false);
  }
}
