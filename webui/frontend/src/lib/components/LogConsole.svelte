<script>
  import { onMount, onDestroy } from 'svelte';
  import { logs, logLevel } from '../stores/app.js';
  import { fetchJSON } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import {
    ChevronDown,
    ChevronUp,
    Copy,
    Trash2,
    Terminal,
  } from '@lucide/svelte';
  import { Button } from '@/lib/components/ui/button';
  import { Separator } from '@/lib/components/ui/separator';
  import { Select } from '@/lib/components/ui/select';

  let expanded = $state(false);
  let currentLevel = $state('INFO');
  let autoScroll = $state(true);
  let changingLevel = $state(false);

  let logContainer = $state(null);
  let eventSource;

  const MAX_LOG_ENTRIES = 500;

  const LEVELS = ['DEBUG', 'INFO', 'WARN', 'ERROR'];

  let logEntries = $state([]);

  logs.subscribe((v) => (logEntries = v));
  logLevel.subscribe((v) => (currentLevel = v));

  onMount(() => {
    fetchInitialLogs();
    connectSSE();
  });

  onDestroy(() => {
    disconnectSSE();
  });

  async function fetchInitialLogs() {
    try {
      const data = await fetchJSON('/api/logs');
      const entries = (data.logs || []).map(formatLogEntry);
      logs.set(entries.slice(-MAX_LOG_ENTRIES));
      logLevel.set(data.level || 'INFO');
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

  async function changeLevel(newLevel) {
    changingLevel = true;
    try {
      await fetchJSON('/api/logs/level', {
        method: 'POST',
        body: JSON.stringify({ level: newLevel }),
      });
      logLevel.set(newLevel);
      showToast('success', `Log level set to ${newLevel}`);
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      changingLevel = false;
    }
  }

  function copyLogs() {
    const text = logEntries.map((e) => e.raw).join('\n');
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(text).then(
        () => showToast('success', 'Logs copied to clipboard'),
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
      {#if logEntries.length > 0}
        <span class="text-xs text-muted-foreground">{logEntries.length}</span>
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
      <Select.Root
        type="single"
        value={currentLevel}
        onValueChange={(level) => level && changeLevel(level)}
        disabled={changingLevel}
      >
        <Select.Trigger class="h-7 w-28 text-xs" size="sm">
          <span>{currentLevel}</span>
        </Select.Trigger>
        <Select.Content class="min-w-28">
          {#each LEVELS as level (level)}
            <Select.Item value={level} class="text-xs">{level}</Select.Item>
          {/each}
        </Select.Content>
      </Select.Root>

      <div class="flex-1"></div>

      <Button variant="ghost" size="icon-xs" class="text-muted-foreground" onclick={copyLogs} title="Copy logs">
        <Copy size={14} />
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
      {#if logEntries.length === 0}
        <p class="text-xs text-zinc-500 italic">No log entries yet...</p>
      {:else}
        {#each logEntries as entry}
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
