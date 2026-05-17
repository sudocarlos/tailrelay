<script>
  import { untrack } from 'svelte';
  import { X, Info } from '@lucide/svelte';
  import { fetchJSON, postFormData } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import Tooltip from './Tooltip.svelte';

  let { type: initType = 'proxy', item: initItem = null, fqdn = '', targets = [], onSave, onClose } = $props();

  // Build initial form state from props once (modal is mounted fresh each time).
  // Using untrack() to read props without reactive tracking avoids state_referenced_locally warnings.
  const initialState = untrack(() => {
    const t = initType;
    const i = initItem;
    const isEditing = i !== null;
    const isProxy = t === 'proxy';
    // Derive isHttpsTarget from stored proxy fields:
    let isHttpsTarget = false;
    if (i && t === 'proxy') {
      if (i.tls) isHttpsTarget = true;
    }
    const tp = i && t === 'proxy' ? (i.trusted_proxies ?? false) : false;
    const hh = i && t === 'proxy' ? (i.host_header ?? '') : '';
    return {
      editing: isEditing,
      // When editing, httpRelay reflects the item type; when adding, always start with HTTP relay on
      httpRelay: isEditing ? isProxy : true,
      title: isEditing ? (isProxy ? 'Edit HTTPS Relay' : 'Edit TCP Relay') : 'Add Relay',
      relayId: i && t === 'relay' ? i.id : '',
      proxyId: i && t === 'proxy' ? i.id : '',
      // Unified listen port: both now use listen_port
      listenPort: i ? String(i.listen_port) : '',
      // Unified target: both now use target_host:target_port
      target: i ? `${i.target_host}:${i.target_port}` : '',
      trustedProxies: tp,
      hostHeader: hh,
      // Auto-open the Advanced section when editing a proxy that already uses either option
      advancedOpen: tp || hh !== '',
      autostart: i ? (i.autostart ?? true) : true,
      isHttpsTarget,
    };
  });

  let saving = $state(false);

  // HTTP relay toggle (controls whether HTTP-only fields are shown)
  let httpRelay = $state(initialState.httpRelay);

  // IDs (needed for update calls)
  let relayId = $state(initialState.relayId);
  let proxyId = $state(initialState.proxyId);

  // Shared fields
  let preset = $state('');
  let listenPort = $state(initialState.listenPort);
  let target = $state(initialState.target);
  let autostart = $state(initialState.autostart);

  // HTTP-only fields
  let trustedProxies = $state(initialState.trustedProxies);
  let hostHeader = $state(initialState.hostHeader);
  let advancedOpen = $state(initialState.advancedOpen);
  // isHttpsTarget: false for http, true for https
  let isHttpsTarget = $state(initialState.isHttpsTarget);

  const editing = initialState.editing;
  const title = initialState.title;



  function handlePreset(e) {
    const idx = e.target.value;
    if (idx === '') return;
    const t = targets[parseInt(idx)];
    target = t.host ? (t.port ? `${t.host}:${t.port}` : t.host) : '';

    // Apply type + protocol from the target definition to set form mode.
    // type: 'relay'/'tcp' → TCP relay; 'proxy'/'https' → HTTPS relay
    // protocol: 'https' → insecure TLS by default; 'http' → plain; 'tcp' → relay
    const isRelay = ['relay', 'tcp'].includes(t.type);
    const isProxy = ['proxy', 'https'].includes(t.type);
    if (isRelay) {
      httpRelay = false;
    } else if (isProxy) {
      httpRelay = true;
      if (t.protocol === 'https') isHttpsTarget = true;
      else isHttpsTarget = false;
    }
  }

  // Parse "host:port" → { host, port } or null on failure.
  // Handles IPv6 addresses in brackets: [::1]:8080
  function parseTarget(raw) {
    const s = raw.trim();
    if (!s) return null;
    // IPv6 bracketed: [::1]:port
    const ipv6 = s.match(/^(\[.+\]):(\d+)$/);
    if (ipv6) return { host: ipv6[1], port: parseInt(ipv6[2]) };
    const lastColon = s.lastIndexOf(':');
    if (lastColon === -1) return null;
    const host = s.slice(0, lastColon);
    const port = parseInt(s.slice(lastColon + 1));
    if (!host || isNaN(port) || port < 1 || port > 65535) return null;
    return { host, port };
  }

  async function handleSave() {
    const isHttp = editing ? initialState.httpRelay : httpRelay;
    if (isHttp) {
      await saveProxy();
    } else {
      await saveRelay();
    }
  }

  async function saveRelay() {
    const parsed = parseTarget(target);
    if (!listenPort || !parsed) {
      showToast('danger', !listenPort ? 'Listen port is required' : 'Target must be in host:port format');
      return;
    }

    const relay = {
      listen_port: parseInt(listenPort),
      target_host: parsed.host,
      target_port: parsed.port,
      autostart,
      enabled: true,
    };

    if (relayId) relay.id = relayId;

    saving = true;
    try {
      const url = relayId ? '/api/serve/tcp/update' : '/api/serve/tcp/create';
      await fetchJSON(url, { method: 'POST', body: JSON.stringify(relay) });
      showToast('success', `TCP relay ${relayId ? 'updated' : 'created'} successfully`);
      onSave();
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      saving = false;
    }
  }

  async function saveProxy() {
    const hostname = fqdn.replace(/\.$/, '');
    if (!hostname) {
      showToast('danger', 'MagicDNS hostname not available. Please ensure Tailscale is connected.');
      return;
    }
    if (!target.trim()) {
      showToast('danger', 'Target is required');
      return;
    }
    if (!listenPort) {
      showToast('danger', 'Listen port is required');
      return;
    }

    const portNum = parseInt(listenPort);
    if ([80, 443, 8021].includes(portNum)) {
      showToast('danger', 'Ports 80, 443, and 8021 are reserved and cannot be used');
      return;
    }



    const formData = new FormData();
    formData.append('hostname', hostname);
    formData.append('target', target.trim());
    formData.append('tls', isHttpsTarget.toString());
    formData.append('trusted_proxies', trustedProxies.toString());
    formData.append('host_header', hostHeader.trim());
    formData.append('autostart', autostart.toString());
    formData.append('enabled', 'true');
    formData.append('port', listenPort);

    if (proxyId) formData.append('id', proxyId);

    saving = true;
    try {
      const url = proxyId ? '/api/serve/https/update' : '/api/serve/https/create';
      await postFormData(url, formData);
      showToast('success', `HTTPS relay ${proxyId ? 'updated' : 'created'} successfully`);
      onSave();
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      saving = false;
    }
  }



  function handleKeydown(e) {
    if (e.key === 'Escape') {
      onClose();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Backdrop -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
  data-modal-open
  onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
  onkeydown={() => {}}
>
  <div class="bg-white dark:bg-gray-900 rounded-xl shadow-2xl w-full max-w-lg max-h-[90vh] overflow-y-auto">
    <!-- Header -->
    <div class="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-800">
      <h2 class="text-lg font-semibold">{title}</h2>
      <button class="p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-400" onclick={onClose}>
        <X size={18} />
      </button>
    </div>

    <!-- Body -->
    <div class="px-5 py-4 space-y-4">

      <!-- HTTP relay toggle (only when adding) — shown first -->
      {#if !editing}
        <div class="flex p-1 rounded-lg bg-gray-100 dark:bg-gray-800 text-sm">
          <button
            type="button"
            onclick={() => (httpRelay = true)}
            class="flex-1 px-3 py-1.5 text-center rounded-md transition-colors {httpRelay ? 'bg-white dark:bg-gray-700 shadow-sm font-medium text-gray-900 dark:text-white' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}"
          >
            HTTPS
          </button>
          <button
            type="button"
            onclick={() => (httpRelay = false)}
            class="flex-1 px-3 py-1.5 text-center rounded-md transition-colors {!httpRelay ? 'bg-white dark:bg-gray-700 shadow-sm font-medium text-gray-900 dark:text-white' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}"
          >
            TCP
          </button>
        </div>
      {/if}

      <!-- Preset Target (shared) -->
      {#if targets.length > 0}
        <div>
          <label for="preset-target" class="block text-sm font-medium mb-1">Preset Target</label>
          <select
            id="preset-target"
            bind:value={preset}
            onchange={handlePreset}
            class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          >
            <option value="">Custom...</option>
            {#each targets as t, i}
              <option value={i.toString()}>{t.target_name || `${t.app_id} (${t.port})`}</option>
            {/each}
          </select>
        </div>
      {/if}

      <!-- Listen Port (shared) -->
      <div>
        <label for="listen-port" class="block text-sm font-medium mb-1">Listen Port</label>
        <input
          id="listen-port"
          type="number"
          bind:value={listenPort}
          placeholder="e.g. 8080"
          class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
        />
      </div>

      <!-- Target (shared) -->
      <div>
        <label for="relay-target" class="block text-sm font-medium mb-1">Target</label>
        <div class="flex gap-2">
          {#if httpRelay}
            <div class="flex p-0.5 rounded-lg bg-gray-100 dark:bg-gray-800 text-xs items-center shrink-0 border border-gray-200 dark:border-gray-700">
              <button
                type="button"
                onclick={() => (isHttpsTarget = false)}
                class="px-2.5 py-1.5 rounded-md transition-colors {!isHttpsTarget ? 'bg-white dark:bg-gray-700 shadow-sm font-medium text-gray-900 dark:text-white' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}"
              >
                http://
              </button>
              <button
                type="button"
                onclick={() => (isHttpsTarget = true)}
                class="px-2.5 py-1.5 rounded-md transition-colors {isHttpsTarget ? 'bg-white dark:bg-gray-700 shadow-sm font-medium text-gray-900 dark:text-white' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}"
              >
                https://
              </button>
            </div>
          {/if}
          <input
            id="relay-target"
            type="text"
            bind:value={target}
            placeholder="e.g. 192.168.1.10:3000 or server.local:8080"
            class="flex-1 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        </div>
      </div>

      <!-- Autostart (shared) -->
      <label class="flex items-center gap-2 cursor-pointer">
        <input
          type="checkbox"
          bind:checked={autostart}
          class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 dark:bg-gray-800"
        />
        <span class="text-sm">Start automatically on boot</span>
      </label>

    </div>

    <!-- Footer -->
    <div class="flex justify-end gap-2 px-5 py-4 border-t border-gray-200 dark:border-gray-800">
      <button
        class="px-4 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
        onclick={onClose}
      >
        Cancel
      </button>
      <button
        class="px-4 py-2 text-sm font-medium rounded-lg bg-blue-500 hover:bg-blue-600 text-white transition-colors disabled:opacity-50"
        disabled={saving}
        onclick={handleSave}
      >
        {saving ? 'Saving...' : (editing ? 'Update' : 'Create')}
      </button>
    </div>
  </div>
</div>
