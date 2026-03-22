<script>
  import { untrack } from 'svelte';
  import { X, Info } from '@lucide/svelte';
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
    // Derive tlsMode from stored proxy fields:
    //   tls == true && no cert file  → insecure mode
    //   cert file present            → custom CA mode
    //   otherwise                   → plain HTTP target
    let tlsMode = 'off';
    if (i && t === 'proxy') {
      if (i.tls) tlsMode = 'insecure';
      else if (i.tls_cert_file) tlsMode = 'cert';
    }
    return {
      editing: isEditing,
      // When editing, httpRelay reflects the item type; when adding, always start with HTTP relay on
      httpRelay: isEditing ? isProxy : true,
      title: isEditing ? (isProxy ? 'Edit HTTPS Relay' : 'Edit TCP Relay') : 'Add Relay',
      relayId: i && t === 'relay' ? i.id : '',
      proxyId: i && t === 'proxy' ? i.id : '',
      // Unified listen port: relay uses listen_port, proxy uses port
      listenPort: i ? (t === 'relay' ? String(i.listen_port) : String(i.port || '')) : '',
      // Unified target: relay combines host:port, proxy uses target verbatim
      target: i ? (t === 'relay' ? `${i.target_host}:${i.target_port}` : (i.target || '')) : '',
      trustedProxies: i && t === 'proxy' ? (i.trusted_proxies ?? false) : false,
      autostart: i ? (i.autostart ?? true) : true,
      existingCert: i && t === 'proxy' ? i.tls_cert_file : null,
      tlsMode,
    };
  });

  let saving = $state(false);
  let removeTlsCert = $state(false);
  let showInsecureTooltip = $state(false);
  let showCertTooltip = $state(false);
  let showTrustedProxiesTooltip = $state(false);

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
  // tlsMode: 'off' | 'insecure' | 'cert'
  let tlsMode = $state(initialState.tlsMode);
  let tlsCertFile = $state(null);
  let existingCert = $state(initialState.existingCert);

  const editing = initialState.editing;
  const title = initialState.title;

  // Switch the HTTPS target mode. Clears any uploaded/existing cert when moving away from cert mode.
  function setTlsMode(mode) {
    if (mode !== 'cert' && (existingCert || tlsCertFile)) {
      removeTlsCert = true;
      existingCert = null;
      tlsCertFile = null;
    }
    if (mode === 'cert') removeTlsCert = false;
    tlsMode = mode;
  }

  function handlePreset(e) {
    const idx = e.target.value;
    if (idx === '') return;
    const t = targets[parseInt(idx)];
    target = t.host ? (t.port ? `${t.host}:${t.port}` : t.host) : '';
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
      const url = relayId ? '/api/socat/update' : '/api/socat/create';
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

    if (tlsMode === 'cert' && tlsCertFile) {
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
    formData.append('target', target.trim());
    formData.append('tls', (tlsMode === 'insecure').toString());
    formData.append('trusted_proxies', trustedProxies.toString());
    formData.append('autostart', autostart.toString());
    formData.append('enabled', 'true');
    formData.append('port', listenPort);

    if (proxyId) formData.append('id', proxyId);
    if (tlsMode === 'cert' && tlsCertFile) formData.append('tls_cert_upload', tlsCertFile);
    if (removeTlsCert) formData.append('remove_tls_cert', 'true');

    saving = true;
    try {
      const url = proxyId ? '/api/caddy/update' : '/api/caddy/create';
      await postFormData(url, formData);
      showToast('success', `HTTPS relay ${proxyId ? 'updated' : 'created'} successfully`);
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
    tlsCertFile = null;
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      if (showInsecureTooltip) {
        showInsecureTooltip = false;
      } else if (showCertTooltip) {
        showCertTooltip = false;
      } else if (showTrustedProxiesTooltip) {
        showTrustedProxiesTooltip = false;
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

      <!-- HTTP relay toggle (only when adding) — shown first -->
      {#if !editing}
        <div>
          <label class="flex items-center justify-between cursor-pointer">
            <span class="text-sm font-medium">HTTPS relay</span>
            <button
              type="button"
              role="switch"
              aria-checked={httpRelay}
              aria-label="Enable HTTPS relay"
              onclick={() => (httpRelay = !httpRelay)}
              class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {httpRelay ? 'bg-blue-500' : 'bg-gray-300 dark:bg-gray-600'}"
            >
              <span
                class="inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform {httpRelay ? 'translate-x-6' : 'translate-x-1'}"
              ></span>
            </button>
          </label>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
            Enable to proxy HTTPS traffic through Caddy with TLS termination.
          </p>
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
        <input
          id="relay-target"
          type="text"
          bind:value={target}
          placeholder="e.g. 192.168.1.10:3000 or server.local:8080"
          class="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
        />
      </div>

      <!-- HTTP-only fields (HTTPS target mode + trusted proxies) -->
      {#if httpRelay}
        <div class="space-y-4">

          <!-- HTTPS target — three-segment control -->
          <div class="flex rounded-lg border border-gray-300 dark:border-gray-700 text-sm overflow-visible">

            <!-- HTTP target -->
            <button
              type="button"
              onclick={() => setTlsMode('off')}
              class="flex-1 px-3 py-2 text-center rounded-l-lg transition-colors focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500
                {tlsMode === 'off'
                  ? 'bg-blue-500 text-white font-medium'
                  : 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'}"
            >
              HTTP target
            </button>

            <!-- HTTPS target (insecure) -->
            <button
              type="button"
              onclick={() => setTlsMode('insecure')}
              class="flex-1 px-3 py-2 text-center border-l border-gray-300 dark:border-gray-700 transition-colors focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500
                {tlsMode === 'insecure'
                  ? 'bg-blue-500 text-white font-medium'
                  : 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'}"
            >
              <span class="flex items-center justify-center gap-1">
                HTTPS target (insecure)
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <span
                  class="relative inline-flex"
                  onmouseenter={() => (showInsecureTooltip = true)}
                  onmouseleave={() => (showInsecureTooltip = false)}
                  onclick={(e) => { e.stopPropagation(); showInsecureTooltip = !showInsecureTooltip; }}
                  onkeydown={() => {}}
                >
                  <Info size={13} class={tlsMode === 'insecure' ? 'text-white/80' : 'text-gray-400'} />
                  {#if showInsecureTooltip}
                    <div class="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-72 z-[100] rounded-lg bg-gray-900 dark:bg-gray-700 text-white text-xs px-3 py-2 shadow-lg pointer-events-none">
                      Turns off TLS handshake verification, making the connection insecure and vulnerable to man-in-the-middle attacks. Do not use in production.
                      <div class="absolute left-1/2 -translate-x-1/2 top-full w-0 h-0 border-x-4 border-x-transparent border-t-4 border-t-gray-900 dark:border-t-gray-700"></div>
                    </div>
                  {/if}
                </span>
              </span>
            </button>

            <!-- HTTPS target (custom CA cert) -->
            <button
              type="button"
              onclick={() => setTlsMode('cert')}
              class="flex-1 px-3 py-2 text-center border-l border-gray-300 dark:border-gray-700 rounded-r-lg transition-colors focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500
                {tlsMode === 'cert'
                  ? 'bg-blue-500 text-white font-medium'
                  : 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'}"
            >
              <span class="flex items-center justify-center gap-1">
                HTTPS target (custom CA cert)
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <span
                  class="relative inline-flex"
                  onmouseenter={() => (showCertTooltip = true)}
                  onmouseleave={() => (showCertTooltip = false)}
                  onclick={(e) => { e.stopPropagation(); showCertTooltip = !showCertTooltip; }}
                  onkeydown={() => {}}
                >
                  <Info size={13} class={tlsMode === 'cert' ? 'text-white/80' : 'text-gray-400'} />
                  {#if showCertTooltip}
                    <div class="absolute right-0 bottom-full mb-2 w-72 z-[100] rounded-lg bg-gray-900 dark:bg-gray-700 text-white text-xs px-3 py-2 shadow-lg pointer-events-none">
                      Upload a CA certificate to trust when your upstream uses HTTPS with a self-signed or private CA certificate. Without this, Caddy uses the system trust pool to verify the upstream's TLS certificate.
                      <div class="absolute right-2 top-full w-0 h-0 border-x-4 border-x-transparent border-t-4 border-t-gray-900 dark:border-t-gray-700"></div>
                    </div>
                  {/if}
                </span>
              </span>
            </button>

          </div>

          <!-- CA cert upload (only in cert mode) -->
          {#if tlsMode === 'cert'}
            <div>
              {#if existingCert}
                <div class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400 mb-2">
                  <span class="truncate">{existingCert.split('/').pop()}</span>
                  <button
                    type="button"
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
          {/if}

          <!-- Trusted proxies: private ranges -->
          <div class="flex items-start gap-2">
            <input
              id="trusted-proxies"
              type="checkbox"
              bind:checked={trustedProxies}
              class="mt-0.5 rounded border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500 dark:bg-gray-800"
            />
            <div class="flex items-center gap-1">
              <label for="trusted-proxies" class="text-sm cursor-pointer select-none">Trusted proxies: private ranges</label>
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <span
                class="relative inline-flex text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 cursor-help"
                onmouseenter={() => (showTrustedProxiesTooltip = true)}
                onmouseleave={() => (showTrustedProxiesTooltip = false)}
                onclick={() => (showTrustedProxiesTooltip = !showTrustedProxiesTooltip)}
                onkeydown={() => {}}
              >
                <Info size={14} />
                {#if showTrustedProxiesTooltip}
                  <div class="absolute right-0 bottom-full mb-2 w-80 z-[100] rounded-lg bg-gray-900 dark:bg-gray-700 text-white text-xs px-3 py-2 shadow-lg pointer-events-none">
                    Enable this if Caddy is behind another proxy (e.g. a CDN or load balancer). Caddy will then trust <span class="font-mono">X-Forwarded-For</span> and related headers from all private IP ranges, allowing it to identify the real client IP instead of the intermediate proxy's IP.
                    <a
                      href="https://caddyserver.com/docs/caddyfile/options#trusted-proxies"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="block mt-1.5 text-blue-300 hover:text-blue-200 pointer-events-auto"
                    >Learn more →</a>
                    <div class="absolute right-2 top-full w-0 h-0 border-x-4 border-x-transparent border-t-4 border-t-gray-900 dark:border-t-gray-700"></div>
                  </div>
                {/if}
              </span>
            </div>
          </div>

        </div>
      {/if}

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
