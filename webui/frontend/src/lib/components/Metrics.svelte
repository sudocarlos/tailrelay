<script>
  import { onMount, onDestroy } from 'svelte';
  import ChartCanvas from './ChartCanvas.svelte';
  import {
    metricsData,
    metricsError,
    metricsLoading,
    metricsResetting,
    metricsWindow,
    fetchMetrics,
    resetMetrics,
  } from '../stores/metrics.js';
  import { theme } from '../stores/theme.js';

  let data = $state(null);
  let error = $state(null);
  let loading = $state(false);
  let resetting = $state(false);
  let interval;

  metricsData.subscribe((v) => { data = v; });
  metricsError.subscribe((v) => (error = v));
  metricsLoading.subscribe((v) => (loading = v));
  metricsResetting.subscribe((v) => (resetting = v));

  // ── Time window ────────────────────────────────────────────────────
  /** @type {''|'1h'|'1d'|'1w'|'1m'} */
  let activeWindow = $state('');
  metricsWindow.subscribe((v) => (activeWindow = v));

  const windows = [
    { value: '',   label: 'All time' },
    { value: '1h', label: '1h' },
    { value: '1d', label: '1d' },
    { value: '1w', label: '1w' },
    { value: '1m', label: '1m' },
  ];

  function setWindow(w) {
    activeWindow = w;
    metricsWindow.set(w);
    fetchMetrics(w);
    restartPoll();
  }

  async function handleReset() {
    if (!confirm('Reset all metrics counters? This will clear all history and cannot be undone.')) return;
    if (interval) clearInterval(interval);
    await resetMetrics(activeWindow);
    restartPoll();
  }

  // ── Relay filter ───────────────────────────────────────────────────
  /** '' means "all relays" */
  let selectedRelay = $state('');

  /** Unique host keys from the data (label if present, else host). */
  const relayOptions = $derived(
    data?.hosts
      ? [...new Set(data.hosts.map((h) => h.label || h.host || '(all hosts)'))]
      : []
  );

  // Reset relay filter if the selected relay disappears from the data.
  $effect(() => {
    if (selectedRelay && !relayOptions.includes(selectedRelay)) {
      selectedRelay = '';
    }
  });

  // ── Theme ──────────────────────────────────────────────────────────
  let currentTheme = $state('light');
  theme.subscribe((v) => (currentTheme = v));

  const isDark = $derived(currentTheme === 'dark');
  const tickColor   = $derived(isDark ? 'rgba(156,163,175,1)' : 'rgba(107,114,128,1)');
  const gridColor   = $derived(isDark ? 'rgba(55,65,81,1)'    : 'rgba(229,231,235,1)');
  const legendColor = $derived(isDark ? 'rgba(209,213,219,1)' : 'rgba(55,65,81,1)');

  // ── Polling ────────────────────────────────────────────────────────
  onMount(() => {
    fetchMetrics(activeWindow);
    interval = setInterval(() => fetchMetrics(activeWindow), 15000);
  });

  onDestroy(() => { if (interval) clearInterval(interval); });

  function restartPoll() {
    if (interval) clearInterval(interval);
    interval = setInterval(() => fetchMetrics(activeWindow), 15000);
  }

  // ── Filtered host list ─────────────────────────────────────────────
  function sortedHosts() {
    if (!data?.hosts) return [];
    let hosts = [...data.hosts].sort((a, b) => b.requests - a.requests);
    if (selectedRelay) {
      hosts = hosts.filter(
        (h) => (h.label || h.host || '(all hosts)') === selectedRelay
      );
    }
    return hosts;
  }

  /** Return the display label for a host entry. */
  function hostLabel(h) {
    // Prefer the compact ":port → target" label set by the backend.
    // Fall back to the raw host FQDN, then a generic placeholder.
    return h.label || h.host || '(all hosts)';
  }

  /** Return the chart-axis label for a host entry (appends paused marker). */
  function chartLabel(h) {
    return h.paused ? `${hostLabel(h)} (paused)` : hostLabel(h);
  }

  // ── Chart datasets ─────────────────────────────────────────────────
  function requestsChartData() {
    const hosts = sortedHosts();
    return {
      labels: hosts.map(chartLabel),
      datasets: [
        {
          label: 'Requests',
          data: hosts.map((h) => h.requests),
          backgroundColor: 'rgba(59,130,246,0.7)',
          borderColor: 'rgba(59,130,246,1)',
          borderWidth: 1,
        },
      ],
    };
  }

  function bandwidthChartData() {
    const hosts = sortedHosts();
    return {
      labels: hosts.map(chartLabel),
      datasets: [
        {
          label: 'Bytes In',
          data: hosts.map((h) => h.requests_in),
          backgroundColor: 'rgba(16,185,129,0.7)',
          borderColor: 'rgba(16,185,129,1)',
          borderWidth: 1,
        },
        {
          label: 'Bytes Out',
          data: hosts.map((h) => h.responses_out),
          backgroundColor: 'rgba(245,158,11,0.7)',
          borderColor: 'rgba(245,158,11,1)',
          borderWidth: 1,
        },
      ],
    };
  }

  function statusCodeChartData() {
    const hosts = sortedHosts();
    const classes = ['2xx', '3xx', '4xx', '5xx'];
    const colors = {
      '2xx': 'rgba(16,185,129,0.7)',
      '3xx': 'rgba(59,130,246,0.7)',
      '4xx': 'rgba(245,158,11,0.7)',
      '5xx': 'rgba(239,68,68,0.7)',
    };
    const borders = {
      '2xx': 'rgba(16,185,129,1)',
      '3xx': 'rgba(59,130,246,1)',
      '4xx': 'rgba(245,158,11,1)',
      '5xx': 'rgba(239,68,68,1)',
    };
    return {
      labels: hosts.map(chartLabel),
      datasets: classes.map((cls) => ({
        label: cls,
        data: hosts.map((h) => (h.status_codes?.[cls] ?? 0)),
        backgroundColor: colors[cls],
        borderColor: borders[cls],
        borderWidth: 1,
      })),
    };
  }

  // ── Chart options ──────────────────────────────────────────────────
  function makeScales(stacked = false) {
    return {
      x: {
        beginAtZero: true,
        ...(stacked ? { stacked: true } : {}),
        ticks: { color: tickColor },
        grid: { color: gridColor },
      },
      y: {
        ...(stacked ? { stacked: true } : {}),
        ticks: { color: tickColor },
        grid: { color: gridColor },
      },
    };
  }

  const barOptions = $derived({
    responsive: true,
    maintainAspectRatio: true,
    plugins: { legend: { display: false } },
    scales: makeScales(),
    indexAxis: 'y',
  });

  const groupedBarOptions = $derived({
    responsive: true,
    maintainAspectRatio: true,
    plugins: { legend: { position: 'top', labels: { color: legendColor } } },
    scales: makeScales(),
    indexAxis: 'y',
  });

  const stackedBarOptions = $derived({
    responsive: true,
    maintainAspectRatio: true,
    plugins: { legend: { position: 'top', labels: { color: legendColor } } },
    scales: makeScales(true),
    indexAxis: 'y',
  });

  // ── Formatting helpers ─────────────────────────────────────────────
  function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
  }
</script>

<div class="space-y-8">
  <!-- Header + filters -->
  <div class="flex flex-wrap items-center justify-between gap-3">
    <h1 class="text-xl font-semibold">Metrics</h1>

    <div class="flex flex-wrap items-center gap-3">
      <!-- Relay filter -->
      {#if relayOptions.length > 0}
        <div class="flex items-center gap-1.5">
          <label for="relay-filter" class="text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">Relay</label>
          <select
            id="relay-filter"
            bind:value={selectedRelay}
            class="rounded border border-gray-300 bg-white px-2 py-1 text-xs text-gray-700 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
          >
            <option value="">All relays</option>
            {#each relayOptions as opt}
              <option value={opt}>{opt}</option>
            {/each}
          </select>
        </div>
      {/if}

      <!-- Time window selector -->
      <div class="flex items-center gap-1.5">
        <span class="text-xs text-gray-500 dark:text-gray-400">Window</span>
        <div class="flex rounded border border-gray-300 overflow-hidden dark:border-gray-600">
          {#each windows as w}
            <button
              onclick={() => setWindow(w.value)}
              class="px-2 py-1 text-xs font-medium transition-colors
                {activeWindow === w.value
                  ? 'bg-blue-600 text-white'
                  : 'bg-white text-gray-600 hover:bg-gray-100 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700'}"
            >
              {w.label}
            </button>
          {/each}
        </div>
      </div>

      <!-- Reset counters -->
      <button
        onclick={handleReset}
        disabled={resetting}
        title="Clear all metric history and baselines"
        class="rounded border border-gray-300 bg-white px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:border-red-300 hover:bg-red-50 hover:text-red-600 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300 dark:hover:border-red-700 dark:hover:bg-red-950 dark:hover:text-red-400"
      >
        {resetting ? 'Resetting…' : 'Reset counters'}
      </button>
    </div>
  </div>

  {#if error}
    <div class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-400">
      Failed to load metrics: {error}
    </div>
  {/if}

  {#if !data && !error && !loading}
    <div class="text-sm text-gray-500 dark:text-gray-400">Loading metrics…</div>
  {/if}

  {#if data}
    {#if sortedHosts().length === 0}
      <div class="rounded-md border border-gray-200 bg-gray-50 px-4 py-6 text-center text-sm text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400">
        {#if selectedRelay}
          No data for the selected relay in this window.
        {:else}
          No request data yet. Metrics are collected from active reverse proxy traffic.
        {/if}
      </div>
    {:else}
      <!-- Requests per Relay -->
      <section>
        <h2 class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">Requests per Relay</h2>
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-900">
          <ChartCanvas type="bar" data={requestsChartData()} options={barOptions} />
        </div>
      </section>

      <!-- Bandwidth per Relay -->
      <section>
        <h2 class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">Bandwidth per Relay</h2>
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-900">
          <ChartCanvas type="bar" data={bandwidthChartData()} options={groupedBarOptions} />
        </div>
        <div class="mt-2 overflow-x-auto">
          <table class="w-full text-xs text-left text-gray-600 dark:text-gray-400">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500 dark:border-gray-700 dark:text-gray-400">
                <th class="pb-1 pr-4 font-medium">Relay</th>
                <th class="pb-1 pr-4 font-medium">Bytes In</th>
                <th class="pb-1 font-medium">Bytes Out</th>
              </tr>
            </thead>
            <tbody>
              {#each sortedHosts() as h}
                <tr class="border-b border-gray-100 dark:border-gray-800 {h.paused ? 'opacity-50' : ''}">
                  <td class="py-1 pr-4 font-mono">
                    {hostLabel(h)}
                    {#if h.paused}<span class="ml-1.5 text-[10px] font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">(paused)</span>{/if}
                  </td>
                  <td class="py-1 pr-4">{formatBytes(h.requests_in)}</td>
                  <td class="py-1">{formatBytes(h.responses_out)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </section>

      <!-- HTTP Status Codes per Relay -->
      <section>
        <h2 class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">HTTP Status Codes per Relay</h2>
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-900">
          <ChartCanvas type="bar" data={statusCodeChartData()} options={stackedBarOptions} />
        </div>
        <div class="mt-2 overflow-x-auto">
          <table class="w-full text-xs text-left text-gray-600 dark:text-gray-400">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500 dark:border-gray-700 dark:text-gray-400">
                <th class="pb-1 pr-4 font-medium">Relay</th>
                <th class="pb-1 pr-4 font-medium">2xx</th>
                <th class="pb-1 pr-4 font-medium">3xx</th>
                <th class="pb-1 pr-4 font-medium">4xx</th>
                <th class="pb-1 font-medium">5xx</th>
              </tr>
            </thead>
            <tbody>
              {#each sortedHosts() as h}
                <tr class="border-b border-gray-100 dark:border-gray-800 {h.paused ? 'opacity-50' : ''}">
                  <td class="py-1 pr-4 font-mono">
                    {hostLabel(h)}
                    {#if h.paused}<span class="ml-1.5 text-[10px] font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500">(paused)</span>{/if}
                  </td>
                  <td class="py-1 pr-4 text-emerald-700 dark:text-emerald-400">{h.status_codes?.['2xx'] ?? 0}</td>
                  <td class="py-1 pr-4 text-blue-700 dark:text-blue-400">{h.status_codes?.['3xx'] ?? 0}</td>
                  <td class="py-1 pr-4 text-amber-700 dark:text-amber-400">{h.status_codes?.['4xx'] ?? 0}</td>
                  <td class="py-1 text-red-700 dark:text-red-400">{h.status_codes?.['5xx'] ?? 0}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </section>
    {/if}
  {/if}
</div>
