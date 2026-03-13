<script>
  import { onMount, onDestroy } from 'svelte';
  import ChartCanvas from './ChartCanvas.svelte';
  import { metricsData, metricsError, metricsLoading, fetchMetrics } from '../stores/metrics.js';
  import { theme } from '../stores/theme.js';

  let data = $state(null);
  let error = $state(null);
  let loading = $state(false);
  let interval;

  metricsData.subscribe((v) => {
    data = v;
  });
  metricsError.subscribe((v) => (error = v));
  metricsLoading.subscribe((v) => (loading = v));

  // Track current theme value reactively
  let currentTheme = $state('light');
  theme.subscribe((v) => (currentTheme = v));

  onMount(() => {
    fetchMetrics();
    interval = setInterval(fetchMetrics, 15000);
  });

  onDestroy(() => {
    if (interval) clearInterval(interval);
  });

  // ── Theme-derived colors ───────────────────────────────────────────

  const isDark = $derived(currentTheme === 'dark');

  const tickColor = $derived(isDark ? 'rgba(156,163,175,1)' : 'rgba(107,114,128,1)');   // gray-400 / gray-500
  const gridColor = $derived(isDark ? 'rgba(55,65,81,1)'   : 'rgba(229,231,235,1)');    // gray-700 / gray-200
  const legendColor = $derived(isDark ? 'rgba(209,213,219,1)' : 'rgba(55,65,81,1)');    // gray-300 / gray-700

  // ── Derived chart data ─────────────────────────────────────────────

  function sortedHosts() {
    if (!data?.hosts) return [];
    return [...data.hosts].sort((a, b) => b.requests - a.requests);
  }

  function requestsChartData() {
    const hosts = sortedHosts();
    return {
      labels: hosts.map((h) => h.label || h.host || '(all hosts)'),
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
      labels: hosts.map((h) => h.label || h.host || '(all hosts)'),
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
      labels: hosts.map((h) => h.label || h.host || '(all hosts)'),
      datasets: classes.map((cls) => ({
        label: cls,
        data: hosts.map((h) => (h.status_codes?.[cls] ?? 0)),
        backgroundColor: colors[cls],
        borderColor: borders[cls],
        borderWidth: 1,
      })),
    };
  }

  // ── Theme-aware chart options ──────────────────────────────────────

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

  function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
  }

  function formatTime(date) {
    if (!date) return '';
    return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

</script>

<div class="space-y-8">
  <div class="flex items-center justify-between">
    <h1 class="text-xl font-semibold">Metrics</h1>
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
    {#if !data.hosts || data.hosts.length === 0}
      <div class="rounded-md border border-gray-200 bg-gray-50 px-4 py-6 text-center text-sm text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400">
        No request data yet. Metrics are collected from active reverse proxy traffic.
      </div>
    {:else}
      <!-- Requests per Host -->
      <section>
        <h2 class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">Requests per Host</h2>
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-900">
          <ChartCanvas type="bar" data={requestsChartData()} options={barOptions} />
        </div>
      </section>

      <!-- Bandwidth per Host -->
      <section>
        <h2 class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">Bandwidth per Host</h2>
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-900">
          <ChartCanvas type="bar" data={bandwidthChartData()} options={groupedBarOptions} />
        </div>
        <!-- Summary table -->
        <div class="mt-2 overflow-x-auto">
          <table class="w-full text-xs text-left text-gray-600 dark:text-gray-400">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500 dark:border-gray-700 dark:text-gray-400">
                <th class="pb-1 pr-4 font-medium">Host</th>
                <th class="pb-1 pr-4 font-medium">Bytes In</th>
                <th class="pb-1 font-medium">Bytes Out</th>
              </tr>
            </thead>
            <tbody>
              {#each sortedHosts() as h}
                <tr class="border-b border-gray-100 dark:border-gray-800">
                  <td class="py-1 pr-4 font-mono">{h.label || h.host || '(all hosts)'}</td>
                  <td class="py-1 pr-4">{formatBytes(h.requests_in)}</td>
                  <td class="py-1">{formatBytes(h.responses_out)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </section>

      <!-- HTTP Status Codes per Host -->
      <section>
        <h2 class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">HTTP Status Codes per Host</h2>
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-900">
          <ChartCanvas type="bar" data={statusCodeChartData()} options={stackedBarOptions} />
        </div>
        <!-- Summary table -->
        <div class="mt-2 overflow-x-auto">
          <table class="w-full text-xs text-left text-gray-600 dark:text-gray-400">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500 dark:border-gray-700 dark:text-gray-400">
                <th class="pb-1 pr-4 font-medium">Host</th>
                <th class="pb-1 pr-4 font-medium">2xx</th>
                <th class="pb-1 pr-4 font-medium">3xx</th>
                <th class="pb-1 pr-4 font-medium">4xx</th>
                <th class="pb-1 font-medium">5xx</th>
              </tr>
            </thead>
            <tbody>
              {#each sortedHosts() as h}
                <tr class="border-b border-gray-100 dark:border-gray-800">
                  <td class="py-1 pr-4 font-mono">{h.label || h.host || '(all hosts)'}</td>
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
