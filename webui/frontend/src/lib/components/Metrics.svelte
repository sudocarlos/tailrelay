<script>
  import { onMount, onDestroy } from 'svelte';
  import ChartCanvas from './ChartCanvas.svelte';
  import { metricsData, metricsError, metricsLoading, fetchMetrics } from '../stores/metrics.js';

  let data = $state(null);
  let error = $state(null);
  let loading = $state(false);
  let interval;

  metricsData.subscribe((v) => (data = v));
  metricsError.subscribe((v) => (error = v));
  metricsLoading.subscribe((v) => (loading = v));

  onMount(() => {
    fetchMetrics();
    interval = setInterval(fetchMetrics, 15000);
  });

  onDestroy(() => {
    if (interval) clearInterval(interval);
  });

  // ── Derived chart data ─────────────────────────────────────────────

  function sortedHosts() {
    if (!data?.hosts) return [];
    return [...data.hosts].sort((a, b) => b.requests - a.requests);
  }

  function requestsChartData() {
    const hosts = sortedHosts();
    return {
      labels: hosts.map((h) => h.host || '(all hosts)'),
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
      labels: hosts.map((h) => h.host || '(all hosts)'),
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
      labels: hosts.map((h) => h.host || '(all hosts)'),
      datasets: classes.map((cls) => ({
        label: cls,
        data: hosts.map((h) => (h.status_codes?.[cls] ?? 0)),
        backgroundColor: colors[cls],
        borderColor: borders[cls],
        borderWidth: 1,
      })),
    };
  }

  const barOptions = {
    responsive: true,
    maintainAspectRatio: true,
    plugins: { legend: { display: false } },
    scales: { x: { beginAtZero: true } },
    indexAxis: 'y',
  };

  const groupedBarOptions = {
    responsive: true,
    maintainAspectRatio: true,
    plugins: { legend: { position: 'top' } },
    scales: { x: { beginAtZero: true } },
    indexAxis: 'y',
  };

  const stackedBarOptions = {
    responsive: true,
    maintainAspectRatio: true,
    plugins: { legend: { position: 'top' } },
    scales: {
      x: { beginAtZero: true, stacked: true },
      y: { stacked: true },
    },
    indexAxis: 'y',
  };

  function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
  }
</script>

<div class="space-y-8">
  <div class="flex items-center justify-between">
    <h1 class="text-xl font-semibold">Metrics</h1>
    {#if loading}
      <span class="text-xs text-gray-400">Refreshing…</span>
    {/if}
  </div>

  {#if error}
    <div class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
      Failed to load metrics: {error}
    </div>
  {/if}

  {#if !data && !error && !loading}
    <div class="text-sm text-gray-500">Loading metrics…</div>
  {/if}

  {#if data}
    {#if !data.hosts || data.hosts.length === 0}
      <div class="rounded-md border border-gray-200 bg-gray-50 px-4 py-6 text-center text-sm text-gray-500">
        No request data yet. Metrics are collected from active reverse proxy traffic.
      </div>
    {:else}
      <!-- Requests per Host -->
      <section>
        <h2 class="mb-3 text-sm font-medium text-gray-700">Requests per Host</h2>
        <div class="rounded-lg border border-gray-200 bg-white p-4">
          <ChartCanvas type="bar" data={requestsChartData()} options={barOptions} />
        </div>
      </section>

      <!-- Bandwidth per Host -->
      <section>
        <h2 class="mb-3 text-sm font-medium text-gray-700">Bandwidth per Host</h2>
        <div class="rounded-lg border border-gray-200 bg-white p-4">
          <ChartCanvas type="bar" data={bandwidthChartData()} options={groupedBarOptions} />
        </div>
        <!-- Summary table -->
        <div class="mt-2 overflow-x-auto">
          <table class="w-full text-xs text-left text-gray-600">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500">
                <th class="pb-1 pr-4 font-medium">Host</th>
                <th class="pb-1 pr-4 font-medium">Bytes In</th>
                <th class="pb-1 font-medium">Bytes Out</th>
              </tr>
            </thead>
            <tbody>
              {#each sortedHosts() as h}
                <tr class="border-b border-gray-100">
                  <td class="py-1 pr-4 font-mono">{h.host || '(all hosts)'}</td>
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
        <h2 class="mb-3 text-sm font-medium text-gray-700">HTTP Status Codes per Host</h2>
        <div class="rounded-lg border border-gray-200 bg-white p-4">
          <ChartCanvas type="bar" data={statusCodeChartData()} options={stackedBarOptions} />
        </div>
        <!-- Summary table -->
        <div class="mt-2 overflow-x-auto">
          <table class="w-full text-xs text-left text-gray-600">
            <thead>
              <tr class="border-b border-gray-200 text-gray-500">
                <th class="pb-1 pr-4 font-medium">Host</th>
                <th class="pb-1 pr-4 font-medium">2xx</th>
                <th class="pb-1 pr-4 font-medium">3xx</th>
                <th class="pb-1 pr-4 font-medium">4xx</th>
                <th class="pb-1 font-medium">5xx</th>
              </tr>
            </thead>
            <tbody>
              {#each sortedHosts() as h}
                <tr class="border-b border-gray-100">
                  <td class="py-1 pr-4 font-mono">{h.host || '(all hosts)'}</td>
                  <td class="py-1 pr-4 text-emerald-700">{h.status_codes?.['2xx'] ?? 0}</td>
                  <td class="py-1 pr-4 text-blue-700">{h.status_codes?.['3xx'] ?? 0}</td>
                  <td class="py-1 pr-4 text-amber-700">{h.status_codes?.['4xx'] ?? 0}</td>
                  <td class="py-1 text-red-700">{h.status_codes?.['5xx'] ?? 0}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </section>
    {/if}

    <!-- Upstream Health -->
    {#if data.upstreams && data.upstreams.length > 0}
      <section>
        <h2 class="mb-3 text-sm font-medium text-gray-700">Upstream Health</h2>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {#each data.upstreams as u}
            <div class="flex items-center gap-3 rounded-lg border border-gray-200 bg-white px-4 py-3">
              <span
                class="h-2.5 w-2.5 flex-shrink-0 rounded-full {u.healthy === 1
                  ? 'bg-emerald-500'
                  : 'bg-red-500'}"
              ></span>
              <span class="text-sm font-mono text-gray-700 truncate">{u.address}</span>
              <span class="ml-auto text-xs {u.healthy === 1 ? 'text-emerald-600' : 'text-red-600'}">
                {u.healthy === 1 ? 'healthy' : 'unhealthy'}
              </span>
            </div>
          {/each}
        </div>
      </section>
    {/if}
  {/if}
</div>
