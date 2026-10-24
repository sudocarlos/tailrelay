<script>
  import { onMount, onDestroy } from 'svelte';
  import { logs } from '../stores/app.js';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import {
    ChevronDown,
    ChevronUp,
    Copy,
    CopyCheck,
    ListFilter,
    Trash2,
    Terminal,
  } from '@lucide/svelte';
  import { Button } from '@/lib/components/ui/button';
  import {
    DropdownMenu,
    DropdownMenuCheckboxItem,
    DropdownMenuContent,
    DropdownMenuLabel,
    DropdownMenuTrigger,
  } from '@/lib/components/ui/dropdown-menu';

  let expanded = $state(false);
  let autoScroll = $state(true);

  let logContainer = $state(null);
  let eventSource;

  const MAX_LOG_ENTRIES = 500;

  const LEVELS = ['DEBUG', 'INFO', 'WARN', 'ERROR'];
  const DEFAULT_VISIBLE = ['INFO', 'WARN', 'ERROR'];
  const LEVELS_STORAGE_KEY = 'logLevels';

  // Which levels the console shows. The backend captures and streams every
  // level; this is a purely client-side view filter, persisted like the
  // theme preference so the choice survives reloads.
  function loadVisibleLevels() {
    try {
      const parsed = JSON.parse(localStorage.getItem(LEVELS_STORAGE_KEY));
      const valid = Array.isArray(parsed) ? parsed.filter((l) => LEVELS.includes(l)) : [];
      if (valid.length > 0) return valid;
    } catch {
      // Fall through to the default set
    }
    return [...DEFAULT_VISIBLE];
  }

  let logEntries = $state([]);
  let visibleLevels = $state(loadVisibleLevels());

  logs.subscribe((v) => (logEntries = v));

  let visibleEntries = $derived(
    logEntries.filter((entry) => visibleLevels.includes(entry.level?.toUpperCase())),
  );
  let levelCounts = $derived.by(() => {
    const counts = {};
    for (const entry of logEntries) {
      const level = entry.level?.toUpperCase();
      if (level) counts[level] = (counts[level] ?? 0) + 1;
    }
    return counts;
  });

  function persistVisibleLevels() {
    try {
      localStorage.setItem(LEVELS_STORAGE_KEY, JSON.stringify(visibleLevels));
    } catch {
      // Storage unavailable — the filter is session-only
    }
  }

  function toggleLevel(level, checked) {
    visibleLevels = checked
      ? [...visibleLevels, level]
      : visibleLevels.filter((l) => l !== level);
    persistVisibleLevels();
  }

  onMount(() => {
    fetchInitialLogs();
    connectSSE();
  });

  onDestroy(() => {
    disconnectSSE();
    clearTimeout(copiedTimer);
  });

  async function fetchInitialLogs() {
    try {
      const data = await fetchJSON('/api/logs');
      const entries = (data.logs || []).map(formatLogEntry);
      logs.set(entries.slice(-MAX_LOG_ENTRIES));
    } catch {
      // Silently fail — logs are non-critical
    }
  }

  function connectSSE() {
    disconnectSSE();
    eventSource = new EventSource('/api/logs/stream');

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.connected) return;

        const entry = formatLogEntry(data);
        logs.update((current) => {
          const updated = [...current, entry];
          return updated.length > MAX_LOG_ENTRIES
            ? updated.slice(-MAX_LOG_ENTRIES)
            : updated;
        });

        if (autoScroll && logContainer) {
          requestAnimationFrame(() => {
            logContainer.scrollTop = logContainer.scrollHeight;
          });
        }
      } catch {
        // Ignore parse errors
      }
    };

    eventSource.onerror = () => {
      // Reconnect after 3 seconds
      disconnectSSE();
      setTimeout(connectSSE, 3000);
    };
  }

  function disconnectSSE() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }

  function formatLogEntry(entry) {
    const ts = entry.timestamp || entry.Timestamp || '';
    const level = entry.level || entry.Level || '';
    const component = entry.source || entry.Source || entry.component || entry.Component || 'main';
    const message = entry.message || entry.Message || '';
    const time = ts ? new Date(ts).toLocaleTimeString() : '';
    return { time, level, component, message, raw: `${time} [${level}] [${component}] ${message}` };
  }

  // Flashing CopyCheck on success matches the row copy buttons
  // (CopyButton) so all copy affordances give the same feedback.
  let logsCopied = $state(false);
  let copiedTimer;

  function flashCopied() {
    logsCopied = true;
    clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => { logsCopied = false; }, 1500);
  }

  function copyLogs() {
    const text = visibleEntries.map((e) => e.raw).join('\n');
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(text).then(
        () => {
          flashCopied();
          showToast('success', 'Logs copied to clipboard');
        },
        () => showToast('danger', 'Failed to copy logs'),
      );
    } else {
      // Fallback for non-secure contexts (plain HTTP)
      try {
        const el = document.createElement('textarea');
        el.value = text;
        el.style.position = 'fixed';
        el.style.opacity = '0';
        document.body.appendChild(el);
        el.select();
        document.execCommand('copy');
        document.body.removeChild(el);
        flashCopied();
        showToast('success', 'Logs copied to clipboard');
      } catch {
        showToast('danger', 'Failed to copy logs');
      }
    }
  }

  function clearLogs() {
    logs.set([]);
  }

  function handleScroll() {
    if (!logContainer) return;
    const { scrollTop, scrollHeight, clientHeight } = logContainer;
    autoScroll = scrollHeight - scrollTop - clientHeight < 40;
  }

  function levelColor(level) {
    switch (level?.toUpperCase()) {
      case 'ERROR': return 'text-red-400';
      case 'WARN': return 'text-amber-400';
      case 'INFO': return 'text-cyan-400';
      case 'DEBUG': return 'text-zinc-500';
      default: return 'text-zinc-400';
    }
  }
</script>

<!-- Terminal-style log panel (port of shadcn.io/ai/terminal): fixed dark
     terminal surface in both themes, mono output, copy + clear actions. -->
<div class="overflow-hidden rounded-lg border border-border">
  <!-- Header / Toggle -->
  <button
    type="button"
    class="flex w-full items-center justify-between bg-muted px-4 py-2.5 text-left transition-colors hover:bg-accent"
    onclick={() => (expanded = !expanded)}
  >
    <div class="flex items-center gap-2">
      <Terminal size={15} class="text-muted-foreground" />
      <span class="text-sm font-medium">Logs</span>
      {#if visibleEntries.length > 0}
        <span class="text-xs text-muted-foreground">{visibleEntries.length}</span>
      {/if}
    </div>
    {#if expanded}
      <ChevronDown size={15} class="text-muted-foreground" />
    {:else}
      <ChevronUp size={15} class="text-muted-foreground" />
    {/if}
  </button>

  {#if expanded}
    <!-- Controls -->
    <div class="flex items-center gap-2 border-t border-border bg-muted px-4 py-2">
      <DropdownMenu>
        <DropdownMenuTrigger>
          <Button variant="outline" size="xs">
            <ListFilter size={13} />
            Levels
            <span class="tabular-nums text-muted-foreground">{visibleLevels.length}/{LEVELS.length}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" class="min-w-44">
          <DropdownMenuLabel>Visible levels</DropdownMenuLabel>
          {#each LEVELS as level (level)}
            <DropdownMenuCheckboxItem
              checked={visibleLevels.includes(level)}
              onCheckedChange={(checked) => toggleLevel(level, checked)}
            >
              {level}
              <span class="ml-1.5 tabular-nums text-muted-foreground">({levelCounts[level] ?? 0})</span>
            </DropdownMenuCheckboxItem>
          {/each}
        </DropdownMenuContent>
      </DropdownMenu>

      <div class="flex-1"></div>

      <Button
        variant="ghost"
        size="icon-xs"
        class="text-muted-foreground {logsCopied ? 'text-green-600' : ''}"
        onclick={copyLogs}
        title={logsCopied ? 'Copied' : 'Copy visible logs'}
      >
        {#if logsCopied}
          <CopyCheck size={14} />
        {:else}
          <Copy size={14} />
        {/if}
      </Button>
      <Button variant="ghost" size="icon-xs" class="text-muted-foreground" onclick={clearLogs} title="Clear logs">
        <Trash2 size={14} />
      </Button>
    </div>

    <!-- Log output -->
    <div
      bind:this={logContainer}
      onscroll={handleScroll}
      class="log-console h-64 overflow-y-auto px-4 py-3"
    >
      {#if visibleEntries.length === 0}
        <p class="text-xs text-zinc-500 italic">
          {visibleLevels.length === 0
            ? 'All levels hidden — enable one from the Levels menu.'
            : 'No log entries yet...'}
        </p>
      {:else}
        {#each visibleEntries as entry}
          <div class="whitespace-pre-wrap break-all leading-relaxed">
            <span class="text-zinc-500">{entry.time}</span>
            {' '}
            <span class={levelColor(entry.level)}>[{entry.level}]</span>
            {' '}
            <span class="text-cyan-400">[{entry.component}]</span>
            {' '}
            <span class="text-zinc-200">{entry.message}</span>
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</div>
