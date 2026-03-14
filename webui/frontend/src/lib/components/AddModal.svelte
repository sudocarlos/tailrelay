<script>
  import { untrack } from 'svelte';
  import { X, Upload, Info } from '@lucide/svelte';
  import { fetchJSON, postFormData } from '../api.js';
  import { showToast } from '../stores/toast.js';

  let { type: initType = 'proxy', item: initItem = null, fqdn = '', targets = [], onSave, onClose } = $props();

  // Build initial form state from props once (modal is mounted fresh each time).
  // Using untrack() to read props without reactive tracking avoids state_referenced_locally warnings.
  const initialState = untrack(() => {
    const t = initType;
    const i = initItem;
    const isEditing = i !== null;
    const isProxy = t === 'proxy';
    return {
      editing: isEditing,
      // When editing, httpRelay reflects the item type; when adding, always start with HTTP relay on
      httpRelay: isEditing ? isProxy : true,
      title: isEditing ? (isProxy ? 'Edit HTTP Relay' : 'Edit TCP Relay') : 'Add Relay',
      relayId: i && t === 'relay' ? i.id : '',
      listenPort: i && t === 'relay' ? i.listen_port : '',
      targetHost: i && t === 'relay' ? i.target_host : '',
      targetPort: i && t === 'relay' ? i.target_port : '',
      proxyId: i && t === 'proxy' ? i.id : '',
      proxyPort: i && t === 'proxy' ? (i.port || '') : '',
      proxyTarget: i && t === 'proxy' ? i.target : '',
      trustedProxies: i && t === 'proxy' ? (i.trusted_proxies ?? false) : false,
      autostart: i ? (i.autostart ?? true) : true,
      existingCert: i && t === 'proxy' ? i.tls_cert_file : null,
    };
  });

  let saving = $state(false);
  let removeTlsCert = $state(false);
  let showCertTooltip = $state(false);

  // HTTP relay toggle (controls whether HTTP relay fields are shown)
  let httpRelay = $state(initialState.httpRelay);

  // TCP relay fields
  let relayId = $state(initialState.relayId);
  let listenPort = $state(initialState.listenPort);
  let targetHost = $state(initialState.targetHost);
  let targetPort = $state(initialState.targetPort);

  // HTTP relay fields
  let proxyId = $state(initialState.proxyId);
  let proxyPort = $state(initialState.proxyPort);
  let proxyTarget = $state(initialState.proxyTarget);
  let trustedProxies = $state(initialState.trustedProxies);
  let tlsCertFile = $state(null);
  let existingCert = $state(initialState.existingCert);

  // Shared fields
  let preset = $state('');
  let autostart = $state(initialState.autostart);

  const editing = initialState.editing;
  const title = initialState.title;

  function handlePreset(e) {
    const idx = e.target.value;
    if (idx === '') return;
    const target = targets[parseInt(idx)];
    // Fill TCP relay fields
    targetHost = target.host || '';
    if (target.port) targetPort = target.port;
    // Fill HTTP relay target URL
    let url = target.host || '';
    if (target.port) url += `:${target.port}`;
    proxyTarget = url;
  }

  async function handleSave() {
    if (editing) {
      // When editing, type is fixed
      if (initialState.httpRelay) {
        await saveProxy();
      } else {
        await saveRelay();
      }
    } else {
      // When adding, httpRelay toggle determines type
      if (httpRelay) {
        await saveProxy();
      } else {
        await saveRelay();
      }
    }
  }

  async function saveRelay() {
    if (!listenPort || !targetHost || !targetPort) {
      showToast('danger', 'Please fill in all required fields');
      return;
    }

    const relay = {
      listen_port: parseInt(listenPort),
      target_host: targetHost.trim(),
      target_port: parseInt(targetPort),
      autostart,
      enabled: true,
    };

    if (relayId) relay.id = relayId;

    saving = true;
    try {
      const url = relayId ? '/api/socat/update' : '/api/socat/create';
      await fetchJSON(url, {
        method: 'POST',
        body: JSON.stringify(relay),
      });
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
    if (!proxyTarget.trim()) {
      showToast('danger', 'Please fill in the target URL');
      return;
    }
    if (!proxyPort) {
      showToast('danger', 'Port is required');
      return;
    }

    const portNum = parseInt(proxyPort);
    if ([80, 443, 8021].includes(portNum)) {
      showToast('danger', 'Ports 80, 443, and 8021 are reserved and cannot be used');
      return;
    }

    if (tlsCertFile) {
      const validExts = ['.pem', '.crt', '.cer'];
      const name = tlsCertFile.name.toLowerCase();
      if (!validExts.some((ext) => name.endsWith(ext))) {
        showToast('danger', 'Invalid certificate file. Please upload a .pem, .crt, or .cer file.');
        return;
      }
      if (tlsCertFile.size > 1024 * 1024) {
        showToast('danger', 'Certificate file too large. Maximum size is 1MB.');
        return;
      }
    }

    const formData = new FormData();
    formData.append('hostname', hostname);
    formData.append('target', proxyTarget.trim());
    formData.append('trusted_proxies', trustedProxies.toString());
    formData.append('autostart', autostart.toString());
    formData.append('enabled', 'true');
    formData.append('port', proxyPort);

    if (proxyId) formData.append('id', proxyId);
    if (tlsCertFile) formData.append('tls_cert_upload', tlsCertFile);
    if (removeTlsCert) formData.append('remove_tls_cert', 'true');

    saving = true;
    try {
      const url = proxyId ? '/api/caddy/update' : '/api/caddy/create';
      await postFormData(url, formData);
      showToast('success', `HTTP relay ${proxyId ? 'updated' : 'created'} successfully`);
      onSave();
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      saving = false;
    }
  }

  function handleFileChange(e) {
    tlsCertFile = e.target.files?.[0] || null;
  }

  function handleRemoveCert() {
    removeTlsCert = true;
    existingCert = null;
    showToast('info', 'Certificate will be removed when you save.');
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      if (showCertTooltip) {
        showCertTooltip = false;
      } else {
        onClose();
      }
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
            {#each targets as target, i}
              <option value={i.toString()}>{target.target_name || `${target.app_id} (${target.port})`}</option>
            {/each}
          </select>
        </div>
      {/if}

      <!-- TCP relay fields (always shown) -->
      <div>
        <label for="relay-listen-port" class="block text-sm font-medium mb-1">Listen Port</label>
        <input
          id="relay-listen-port"
          type="number"
          bind:value={listenPort}
          placeholder="e.g. 8080"
          class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
        />
      </div>

      <div class="grid grid-cols-2 gap-3">
        <div>
          <label for="relay-target-host" class="block text-sm font-medium mb-1">Target Host</label>
          <input
            id="relay-target-host"
            type="text"
            bind:value={targetHost}
            placeholder="e.g. 192.168.1.10"
            class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        </div>
        <div>
          <label for="relay-target-port" class="block text-sm font-medium mb-1">Target Port</label>
          <input
            id="relay-target-port"
            type="number"
            bind:value={targetPort}
            placeholder="e.g. 3000"
            class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
        </div>
      </div>

      <!-- HTTP relay toggle (only when adding) -->
      {#if !editing}
        <div class="border-t border-gray-200 dark:border-gray-700 pt-4">
          <label class="flex items-center justify-between cursor-pointer">
            <span class="text-sm font-medium">HTTP relay</span>
            <button
              type="button"
              role="switch"
              aria-checked={httpRelay}
              aria-label="Enable HTTP relay"
              onclick={() => (httpRelay = !httpRelay)}
              class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {httpRelay ? 'bg-blue-500' : 'bg-gray-300 dark:bg-gray-600'}"
            >
              <span
                class="inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform {httpRelay ? 'translate-x-6' : 'translate-x-1'}"
              ></span>
            </button>
          </label>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
            Enable to proxy HTTP/HTTPS traffic through Caddy with TLS termination.
          </p>
        </div>
      {/if}

      <!-- HTTP relay fields (shown when httpRelay is on, or always when editing a proxy) -->
      {#if httpRelay}
        <div class="space-y-4 {!editing ? '' : ''}">
          <div>
            <label for="proxy-port" class="block text-sm font-medium mb-1">HTTP Port</label>
            <input
              id="proxy-port"
              type="number"
              bind:value={proxyPort}
              placeholder="e.g. 8443"
              class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            />
          </div>

          <div>
            <label for="proxy-target" class="block text-sm font-medium mb-1">Target URL</label>
            <input
              id="proxy-target"
              type="text"
              bind:value={proxyTarget}
              placeholder="e.g. http://192.168.1.10:3000"
              class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            />
          </div>

          <!-- TLS Certificate -->
          <div>
            <div class="flex items-center gap-1.5 mb-1">
              <label for="tls-cert-file" class="block text-sm font-medium">CA Certificate (optional)</label>
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div class="relative">
                <span
                  class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 cursor-help"
                  onmouseenter={() => (showCertTooltip = true)}
                  onmouseleave={() => (showCertTooltip = false)}
                  onclick={() => (showCertTooltip = !showCertTooltip)}
                  onkeydown={() => {}}
                >
                  <Info size={14} />
                </span>
                {#if showCertTooltip}
                  <div class="absolute left-0 bottom-full mb-2 w-72 z-10 rounded-lg bg-gray-900 dark:bg-gray-700 text-white text-xs px-3 py-2 shadow-lg">
                    Upload a CA certificate to trust when your upstream uses HTTPS with a self-signed or private CA certificate. Without this, Caddy uses the system trust pool to verify the upstream's TLS certificate.
                    <div class="absolute left-2 top-full w-0 h-0 border-x-4 border-x-transparent border-t-4 border-t-gray-900 dark:border-t-gray-700"></div>
                  </div>
                {/if}
              </div>
            </div>
            {#if existingCert}
              <div class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400 mb-2">
                <span class="truncate">{existingCert.split('/').pop()}</span>
                <button
                  class="text-red-500 hover:text-red-600 text-xs font-medium"
                  onclick={handleRemoveCert}
                >
                  Remove
                </button>
              </div>
            {/if}
            <input
              id="tls-cert-file"
              type="file"
              accept=".pem,.crt,.cer"
              onchange={handleFileChange}
              class="w-full text-sm text-gray-500 file:mr-3 file:py-1.5 file:px-3 file:rounded-md file:border-0 file:text-sm file:font-medium file:bg-gray-100 dark:file:bg-gray-800 file:text-gray-700 dark:file:text-gray-300 hover:file:bg-gray-200 dark:hover:file:bg-gray-700"
            />
          </div>

          <label class="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              bind:checked={trustedProxies}
              class="rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 dark:bg-gray-800"
            />
            <span class="text-sm">Trust proxy headers</span>
          </label>
        </div>
      {/if}

      <!-- Autostart (shared) -->
      <label class="flex items-center gap-2 cursor-pointer {httpRelay ? '' : ''}">
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
