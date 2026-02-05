(() => {
  const state = {
    relays: [],
    proxies: [],
    showRelays: true,
    showProxies: true,
    tailnetFQDN: "",
    logs: [],
    logLevel: "INFO",
    logStream: null,
    currentEditItem: null,
    currentEditType: null,
    deleteTarget: null,
    removeTlsCert: false,
  };

  const elements = {
    items: document.getElementById("items"),
    lastUpdated: document.getElementById("last-updated"),
    itemCount: document.getElementById("item-count"),
    alertContainer: document.getElementById("alert-container"),
    logOutput: document.getElementById("log-output"),
    logLevel: document.getElementById("log-level"),
    logLevelSelect: document.getElementById("log-level-select"),
    refresh: document.getElementById("refresh"),
    clearLogs: document.getElementById("clear-logs"),
    filterRelay: document.getElementById("filter-relay"),
    filterProxy: document.getElementById("filter-proxy"),
    themeToggle: document.getElementById("theme-toggle"),
    addRelayBtn: document.getElementById("add-relay-btn"),
    addProxyBtn: document.getElementById("add-proxy-btn"),
    saveRelayBtn: document.getElementById("save-relay-btn"),
    saveProxyBtn: document.getElementById("save-proxy-btn"),
    confirmDeleteBtn: document.getElementById("confirm-delete-btn"),
    removeTlsCertBtn: document.getElementById("proxy-tls-cert-remove"),
  };

  const tooltips = [];

  // Dark mode management
  const getPreferredTheme = () => {
    const stored = localStorage.getItem("theme");
    if (stored) {
      return stored;
    }
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  };

  const setTheme = (theme) => {
    document.documentElement.setAttribute("data-bs-theme", theme);
    localStorage.setItem("theme", theme);
    updateThemeIcon(theme);
  };

  const updateThemeIcon = (theme) => {
    if (!elements.themeToggle) return;
    const icon = theme === "dark" ? "bi-moon-stars-fill" : "bi-sun-fill";
    elements.themeToggle.querySelector("use").setAttribute("href", `/static/vendor/bootstrap-icons/bootstrap-icons.svg#${icon}`);
  };

  const toggleTheme = () => {
    const current = document.documentElement.getAttribute("data-bs-theme") || "light";
    const next = current === "dark" ? "light" : "dark";
    setTheme(next);
  };

  const fetchJSON = async (url, options = {}) => {
    const response = await fetch(url, {
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        ...(options.headers || {}),
      },
      ...options,
    });

    if (!response.ok) {
      const message = await response.text();
      throw new Error(message || `Request failed: ${response.status}`);
    }

    return response.json();
  };

  const setLastUpdated = () => {
    const now = new Date();
    elements.lastUpdated.textContent = now.toLocaleTimeString();
  };

  const showAlert = (type, message) => {
    const alert = document.createElement("div");
    alert.className = `alert alert-${type} alert-dismissible fade show`;
    alert.setAttribute("role", "alert");
    alert.innerHTML = `
      <div>${message}</div>
      <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
    `;
    elements.alertContainer.appendChild(alert);
    setTimeout(() => {
      alert.classList.remove("show");
      alert.addEventListener("transitionend", () => alert.remove());
    }, 6000);
  };

  const formatRelayTitle = (relay) => {
    const fqdn = state.tailnetFQDN || "unknown";
    return `tcp://${fqdn}:${relay.listen_port} → ${relay.target_host}:${relay.target_port}`;
  };

  const formatProxyLink = (proxy) => {
    const portLabel = proxy.port ? `:${proxy.port}` : "";
    const url = `https://${proxy.hostname}${portLabel}`;
    return `<a class="proxy-link" href="${url}" target="_blank" rel="noopener">${url}</a>`;
  };

  const renderEmpty = (message) => {
    elements.items.innerHTML = `
      <div class="col-12">
        <div class="card">
          <div class="card-body text-center text-muted">
            ${message}
          </div>
        </div>
      </div>
    `;
  };

  const renderItems = () => {
    disposeTooltips();

    const combined = [
      ...state.relays.map((item) => ({
        type: "relay",
        relay: item.relay,
        running: item.running,
      })),
      ...state.proxies.map((item) => ({
        type: "proxy",
        proxy: item,
      })),
    ];

    const filtered = combined.filter((item) =>
      item.type === "relay" ? state.showRelays : state.showProxies,
    );

    elements.itemCount.textContent = `${filtered.length} item${filtered.length === 1 ? "" : "s"}`;

    if (!filtered.length) {
      if (!state.showRelays && !state.showProxies) {
        renderEmpty("Enable TCP relays or HTTPS proxies to view items.");
      } else if (state.showRelays && !state.showProxies) {
        renderEmpty("No TCP relays configured.");
      } else if (!state.showRelays && state.showProxies) {
        renderEmpty("No HTTPS proxies configured.");
      } else {
        renderEmpty("No relays or proxies configured.");
      }
      return;
    }

    elements.items.innerHTML = filtered
      .map((item) => {
        if (item.type === "relay") {
          const relay = item.relay;
          const running = item.running;
          const statusBadge = running ? "text-bg-success" : "text-bg-secondary";
          const autostart = relay.autostart ?? false;
          return `
            <div class="col-12">
              <div class="card h-100">
                <div class="card-body d-flex flex-column flex-lg-row align-items-lg-center gap-3">
                  <div class="flex-grow-1">
                    <div class="d-flex align-items-center gap-2 flex-wrap">
                      <svg class="bi text-primary" data-bs-toggle="tooltip" title="TCP Relay (served by socat)" aria-hidden="true" style="width: 1.25em; height: 1.25em;"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#bi-diagram-3"></use></svg>
                      <span class="fw-semibold">${formatRelayTitle(relay)}</span>
                    </div>
                  </div>
                  <div class="d-flex align-items-center gap-2">
                    <span class="badge ${statusBadge}">${running ? "Running" : "Stopped"}</span>
                    <div class="form-check form-switch m-0" data-bs-toggle="tooltip" title="Start automatically on container boot">
                      <input class="form-check-input autostart-toggle" type="checkbox" role="switch" 
                             ${autostart ? "checked" : ""} 
                             data-type="relay" data-id="${relay.id}">
                      <label class="form-check-label small text-muted">Autostart</label>
                    </div>
                    <button class="btn btn-outline-secondary btn-sm action-btn" data-type="relay" data-id="${relay.id}" data-running="${running}">
                      <svg class="bi me-1" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#${running ? "bi-pause-fill" : "bi-play-fill"}"></use></svg>
                      ${running ? "Pause" : "Start"}
                    </button>
                    <button class="btn btn-outline-primary btn-sm edit-btn" data-type="relay" data-id="${relay.id}">
                      <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#bi-pencil"></use></svg>
                    </button>
                    <button class="btn btn-outline-danger btn-sm delete-btn" data-type="relay" data-id="${relay.id}" data-name="tcp://${state.tailnetFQDN}:${relay.listen_port}">
                      <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#bi-trash"></use></svg>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          `;
        }

        const proxy = item.proxy;
        const running = proxy.running ?? proxy.Running;
        const runningBadge = running ? "text-bg-success" : "text-bg-secondary";
        const runningLabel = running ? "Running" : "Stopped";
        const autostart = proxy.autostart ?? false;
        const proxyName = proxy.port ? `${proxy.hostname}:${proxy.port}` : proxy.hostname;
        return `
          <div class="col-12">
            <div class="card h-100">
              <div class="card-body d-flex flex-column flex-lg-row align-items-lg-center gap-3">
                <div class="flex-grow-1">
                  <div class="d-flex align-items-center gap-2 flex-wrap">
                    <svg class="bi text-primary" data-bs-toggle="tooltip" title="HTTPS Proxy (served by Caddy)" aria-hidden="true" style="width: 1.25em; height: 1.25em;"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#bi-shield-lock"></use></svg>
                    <span class="fw-semibold">${formatProxyLink(proxy)} → ${proxy.target}</span>
                  </div>
                </div>
                <div class="d-flex align-items-center gap-2">
                  <span class="badge ${runningBadge}">${runningLabel}</span>
                  <div class="form-check form-switch m-0" data-bs-toggle="tooltip" title="Start automatically on container boot">
                    <input class="form-check-input autostart-toggle" type="checkbox" role="switch" 
                           ${autostart ? "checked" : ""} 
                           data-type="proxy" data-id="${proxy.id}">
                    <label class="form-check-label small text-muted">Autostart</label>
                  </div>
                  <button class="btn btn-outline-secondary btn-sm action-btn" data-type="proxy" data-id="${proxy.id}" data-enabled="${proxy.enabled}">
                    <svg class="bi me-1" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#${proxy.enabled ? "bi-pause-fill" : "bi-play-fill"}"></use></svg>
                    ${proxy.enabled ? "Pause" : "Start"}
                  </button>
                  <button class="btn btn-outline-primary btn-sm edit-btn" data-type="proxy" data-id="${proxy.id}">
                    <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#bi-pencil"></use></svg>
                  </button>
                  <button class="btn btn-outline-danger btn-sm delete-btn" data-type="proxy" data-id="${proxy.id}" data-name="https://${proxyName}">
                    <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#bi-trash"></use></svg>
                  </button>
                </div>
              </div>
            </div>
          </div>
        `;
      })
      .join("");

    initTooltips();
  };

  const initTooltips = () => {
    document.querySelectorAll('[data-bs-toggle="tooltip"]').forEach((node) => {
      tooltips.push(new bootstrap.Tooltip(node));
    });
  };

  const disposeTooltips = () => {
    while (tooltips.length) {
      const tooltip = tooltips.pop();
      tooltip.dispose();
    }
  };

  const refreshData = async () => {
    try {
      const [relays, proxies, status] = await Promise.all([
        fetchJSON("/api/socat/relays"),
        fetchJSON("/api/caddy/proxies"),
        fetchJSON("/api/tailscale/status"),
      ]);

      state.relays = relays.map((status) => ({
        relay: status.Relay || status.relay,
        running: status.Running ?? status.running,
      }));
      state.proxies = proxies.map((proxy) => ({
        ...proxy,
        running: proxy.running ?? proxy.Running,
      }));
      state.tailnetFQDN = status.MagicDNSName || status.magicDNSName || "";

      renderItems();
      setLastUpdated();
    } catch (error) {
      showAlert("danger", error.message);
    }
  };

  const toggleRelay = async (relayId, isRunning) => {
    const url = isRunning ? `/api/socat/stop?id=${encodeURIComponent(relayId)}` : `/api/socat/start?id=${encodeURIComponent(relayId)}`;
    await fetchJSON(url, { method: "POST" });
  };

  const toggleProxy = async (proxyId, isEnabled) => {
    await fetchJSON("/api/caddy/toggle", {
      method: "POST",
      body: JSON.stringify({ id: proxyId, enabled: !isEnabled }),
    });
  };

  const toggleAutostart = async (type, id, autostart) => {
    const url = type === "relay" ? "/api/socat/update" : "/api/caddy/update";
    
    // Get the current item first
    const currentItem = type === "relay"
      ? state.relays.find(r => r.relay.id === id)?.relay
      : state.proxies.find(p => p.id === id);
    
    if (!currentItem) {
      throw new Error(`${type} not found`);
    }
    
    // Update with new autostart value
    const updated = { ...currentItem, autostart };
    
    await fetchJSON(url, {
      method: "POST",
      body: JSON.stringify(updated),
    });
  };

  const handleActionClick = async (event) => {
    const button = event.target.closest(".action-btn");
    if (!button) {
      return;
    }

    button.disabled = true;
    const type = button.dataset.type;

    try {
      if (type === "relay") {
        const isRunning = button.dataset.running === "true";
        await toggleRelay(button.dataset.id, isRunning);
      } else {
        const isEnabled = button.dataset.enabled === "true";
        await toggleProxy(button.dataset.id, isEnabled);
      }

      await refreshData();
    } catch (error) {
      showAlert("danger", error.message);
    } finally {
      button.disabled = false;
    }
  };

  const handleAutostartToggle = async (event) => {
    const toggle = event.target;
    if (!toggle.classList.contains("autostart-toggle")) {
      return;
    }

    const { type, id } = toggle.dataset;
    const autostart = toggle.checked;
    
    toggle.disabled = true;

    try {
      await toggleAutostart(type, id, autostart);
      await refreshData();
    } catch (error) {
      showAlert("danger", error.message);
      // Revert the toggle on error
      toggle.checked = !autostart;
    } finally {
      toggle.disabled = false;
    }
  };

  const appendLogEntry = (entry) => {
    if (!entry || !entry.message) {
      return;
    }

    const timestamp = entry.timestamp ? new Date(entry.timestamp) : new Date();
    const timeLabel = timestamp.toLocaleTimeString();
    const source = entry.source ? ` [${entry.source}]` : "";
    const line = `${timeLabel} [${entry.level}]${source} ${entry.message}`;

    const output = elements.logOutput;
    const isAtBottom = output.scrollTop + output.clientHeight >= output.scrollHeight - 8;
    output.textContent += `${line}\n`;

    if (isAtBottom) {
      output.scrollTop = output.scrollHeight;
    }
  };

  const loadLogs = async () => {
    try {
      const data = await fetchJSON("/api/logs");
      state.logs = data.logs || [];
      state.logLevel = data.level || "INFO";
      elements.logLevel.textContent = state.logLevel;
      if (elements.logLevelSelect) {
        elements.logLevelSelect.value = state.logLevel;
      }
      elements.logOutput.textContent = "";
      state.logs.forEach(appendLogEntry);
    } catch (error) {
      showAlert("warning", error.message);
    }
  };

  const setLogLevel = async (level) => {
    try {
      const response = await fetchJSON("/api/logs/level", {
        method: "POST",
        body: JSON.stringify({ level }),
      });
      state.logLevel = response.level || level;
      elements.logLevel.textContent = state.logLevel;
      if (elements.logLevelSelect) {
        elements.logLevelSelect.value = state.logLevel;
      }
    } catch (error) {
      showAlert("warning", error.message);
    }
  };

  const startLogStream = () => {
    if (state.logStream) {
      state.logStream.close();
    }

    const stream = new EventSource("/api/logs/stream");
    stream.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.connected) {
          return;
        }
        appendLogEntry(data);
      } catch (error) {
        // ignore malformed entries
      }
    };
    stream.onerror = () => {
      showAlert("warning", "Log stream disconnected. Retrying...");
    };

    state.logStream = stream;
  };

  const openRelayModal = (relay = null) => {
    const modal = new bootstrap.Modal(document.getElementById("relayModal"));
    const modalTitle = document.querySelector("#relayModal .modal-title");
    
    state.currentEditItem = relay;
    state.currentEditType = "relay";
    
    if (relay) {
      // Edit mode
      modalTitle.textContent = "Edit Relay";
      document.getElementById("relay-id").value = relay.id;
      document.getElementById("relay-listen-port").value = relay.listen_port;
      document.getElementById("relay-target-host").value = relay.target_host;
      document.getElementById("relay-target-port").value = relay.target_port;
      document.getElementById("relay-autostart").checked = relay.autostart ?? false;
    } else {
      // Add mode
      modalTitle.textContent = "Add Relay";
      document.getElementById("relayForm").reset();
      document.getElementById("relay-id").value = "";
      document.getElementById("relay-autostart").checked = true;
    }
    
    modal.show();
  };

  const openProxyModal = (proxy = null) => {
    const modal = new bootstrap.Modal(document.getElementById("proxyModal"));
    const modalTitle = document.querySelector("#proxyModal .modal-title");
    const certCurrent = document.getElementById("proxy-tls-cert-current");
    const certFilename = document.getElementById("proxy-tls-cert-filename");
    const certFileInput = document.getElementById("proxy-tls-cert");
    
    state.currentEditItem = proxy;
    state.currentEditType = "proxy";
    state.removeTlsCert = false; // Reset remove flag
    
    if (proxy) {
      // Edit mode
      modalTitle.textContent = "Edit Proxy";
      document.getElementById("proxy-id").value = proxy.id;
      document.getElementById("proxy-port").value = proxy.port || "";
      document.getElementById("proxy-target").value = proxy.target;
      document.getElementById("proxy-trusted-proxies").checked = proxy.trusted_proxies ?? false;
      document.getElementById("proxy-autostart").checked = proxy.autostart ?? false;
      
      // Show current TLS cert if exists
      certFileInput.value = "";
      if (proxy.tls_cert_file) {
        const basename = proxy.tls_cert_file.split('/').pop();
        certFilename.textContent = basename;
        certCurrent.style.display = "flex";
      } else {
        certCurrent.style.display = "none";
      }
    } else {
      // Add mode
      modalTitle.textContent = "Add Proxy";
      document.getElementById("proxyForm").reset();
      document.getElementById("proxy-id").value = "";
      document.getElementById("proxy-autostart").checked = true;
      certFileInput.value = "";
      certCurrent.style.display = "none";
    }
    
    modal.show();
  };

  const saveRelay = async () => {
    const id = document.getElementById("relay-id").value;
    const listenPort = parseInt(document.getElementById("relay-listen-port").value);
    const targetHost = document.getElementById("relay-target-host").value.trim();
    const targetPort = parseInt(document.getElementById("relay-target-port").value);
    const autostart = document.getElementById("relay-autostart").checked;

    if (!listenPort || !targetHost || !targetPort) {
      showAlert("danger", "Please fill in all required fields");
      return;
    }

    const relay = {
      listen_port: listenPort,
      target_host: targetHost,
      target_port: targetPort,
      autostart: autostart,
      enabled: true,
    };

    if (id) {
      relay.id = id;
    }

    try {
      elements.saveRelayBtn.disabled = true;
      const url = id ? "/api/socat/update" : "/api/socat/create";
      await fetchJSON(url, {
        method: "POST",
        body: JSON.stringify(relay),
      });

      bootstrap.Modal.getInstance(document.getElementById("relayModal")).hide();
      showAlert("success", `Relay ${id ? "updated" : "created"} successfully`);
      await refreshData();
    } catch (error) {
      showAlert("danger", error.message);
    } finally {
      elements.saveRelayBtn.disabled = false;
    }
  };

  const saveProxy = async () => {
    const id = document.getElementById("proxy-id").value;
    const port = document.getElementById("proxy-port").value.trim();
    const target = document.getElementById("proxy-target").value.trim();
    const trustedProxies = document.getElementById("proxy-trusted-proxies").checked;
    const autostart = document.getElementById("proxy-autostart").checked;
    const tlsCertFile = document.getElementById("proxy-tls-cert").files[0];

    // Always use MagicDNS hostname (strip trailing dot)
    const hostname = state.tailnetFQDN.replace(/\.$/, '');
    
    if (!hostname) {
      showAlert("danger", "MagicDNS hostname not available. Please ensure Tailscale is connected.");
      return;
    }
    
    if (!target) {
      showAlert("danger", "Please fill in the target URL");
      return;
    }

    // Frontend validation for cert file
    if (tlsCertFile) {
      const validExtensions = ['.pem', '.crt', '.cer'];
      const fileName = tlsCertFile.name.toLowerCase();
      const isValidExt = validExtensions.some(ext => fileName.endsWith(ext));
      if (!isValidExt) {
        showAlert("danger", "Invalid certificate file. Please upload a .pem, .crt, or .cer file.");
        return;
      }
      
      // Check file size (max 1MB for cert files)
      if (tlsCertFile.size > 1024 * 1024) {
        showAlert("danger", "Certificate file too large. Maximum size is 1MB.");
        return;
      }
    }

    // Build FormData for multipart upload
    const formData = new FormData();
    formData.append("hostname", hostname);
    formData.append("target", target);
    formData.append("trusted_proxies", trustedProxies.toString());
    formData.append("autostart", autostart.toString());
    formData.append("enabled", "true");

    if (port) {
      formData.append("port", port);
    }

    if (id) {
      formData.append("id", id);
    }

    // Add TLS cert file if selected
    if (tlsCertFile) {
      formData.append("tls_cert_upload", tlsCertFile);
    }

    // Flag to remove existing cert
    if (state.removeTlsCert) {
      formData.append("remove_tls_cert", "true");
    }

    try {
      elements.saveProxyBtn.disabled = true;
      const url = id ? "/api/caddy/update" : "/api/caddy/create";
      
      // Use fetch without JSON content-type for FormData
      const response = await fetch(url, {
        method: "POST",
        credentials: "same-origin",
        body: formData,
      });

      if (!response.ok) {
        const message = await response.text();
        throw new Error(message || `Request failed: ${response.status}`);
      }

      await response.json();

      bootstrap.Modal.getInstance(document.getElementById("proxyModal")).hide();
      showAlert("success", `Proxy ${id ? "updated" : "created"} successfully`);
      await refreshData();
    } catch (error) {
      showAlert("danger", error.message);
    } finally {
      elements.saveProxyBtn.disabled = false;
    }
  };

  const openDeleteModal = (type, id, name) => {
    const modal = new bootstrap.Modal(document.getElementById("deleteModal"));
    const message = document.getElementById("delete-message");
    
    state.deleteTarget = { type, id };
    message.textContent = `Are you sure you want to delete ${type === "relay" ? "relay" : "proxy"} "${name}"? This action cannot be undone.`;
    
    modal.show();
  };

  const confirmDelete = async () => {
    if (!state.deleteTarget) {
      return;
    }

    const { type, id } = state.deleteTarget;

    try {
      elements.confirmDeleteBtn.disabled = true;
      const url = type === "relay" 
        ? `/api/socat/delete?id=${encodeURIComponent(id)}`
        : `/api/caddy/delete?id=${encodeURIComponent(id)}`;
      
      await fetchJSON(url, { method: "POST" });

      bootstrap.Modal.getInstance(document.getElementById("deleteModal")).hide();
      showAlert("success", `${type === "relay" ? "Relay" : "Proxy"} deleted successfully`);
      await refreshData();
    } catch (error) {
      showAlert("danger", error.message);
    } finally {
      elements.confirmDeleteBtn.disabled = false;
      state.deleteTarget = null;
    }
  };

  const handleEditClick = async (event) => {
    const button = event.target.closest(".edit-btn");
    if (!button) {
      return;
    }

    const type = button.dataset.type;
    const id = button.dataset.id;

    if (type === "relay") {
      const relay = state.relays.find(r => r.relay.id === id)?.relay;
      if (relay) {
        openRelayModal(relay);
      }
    } else if (type === "proxy") {
      const proxy = state.proxies.find(p => p.id === id);
      if (proxy) {
        openProxyModal(proxy);
      }
    }
  };

  const handleDeleteClick = async (event) => {
    const button = event.target.closest(".delete-btn");
    if (!button) {
      return;
    }

    const type = button.dataset.type;
    const id = button.dataset.id;
    const name = button.dataset.name;

    openDeleteModal(type, id, name);
  };

  const bindEvents = () => {
    elements.items.addEventListener("click", handleActionClick);
    elements.items.addEventListener("click", handleEditClick);
    elements.items.addEventListener("click", handleDeleteClick);
    elements.items.addEventListener("change", handleAutostartToggle);

    elements.filterRelay.addEventListener("change", () => {
      state.showRelays = elements.filterRelay.checked;
      renderItems();
    });

    elements.filterProxy.addEventListener("change", () => {
      state.showProxies = elements.filterProxy.checked;
      renderItems();
    });

    if (elements.themeToggle) {
      elements.themeToggle.addEventListener("click", toggleTheme);
    }

    elements.refresh.addEventListener("click", refreshData);
    elements.clearLogs.addEventListener("click", () => {
      elements.logOutput.textContent = "";
    });

    if (elements.logLevelSelect) {
      elements.logLevelSelect.addEventListener("change", (event) => {
        setLogLevel(event.target.value);
      });
    }

    // FAB and modal events
    if (elements.addRelayBtn) {
      elements.addRelayBtn.addEventListener("click", (e) => {
        e.preventDefault();
        openRelayModal();
      });
    }

    if (elements.addProxyBtn) {
      elements.addProxyBtn.addEventListener("click", (e) => {
        e.preventDefault();
        openProxyModal();
      });
    }

    if (elements.saveRelayBtn) {
      elements.saveRelayBtn.addEventListener("click", saveRelay);
    }

    if (elements.saveProxyBtn) {
      elements.saveProxyBtn.addEventListener("click", saveProxy);
    }

    if (elements.confirmDeleteBtn) {
      elements.confirmDeleteBtn.addEventListener("click", confirmDelete);
    }

    // Handle remove TLS cert button
    if (elements.removeTlsCertBtn) {
      elements.removeTlsCertBtn.addEventListener("click", () => {
        state.removeTlsCert = true;
        document.getElementById("proxy-tls-cert-current").style.display = "none";
        showAlert("info", "Certificate will be removed when you save the proxy.");
      });
    }

    // Handle Enter key in forms
    document.getElementById("relayForm")?.addEventListener("submit", (e) => {
      e.preventDefault();
      saveRelay();
    });

    document.getElementById("proxyForm")?.addEventListener("submit", (e) => {
      e.preventDefault();
      saveProxy();
    });
  };

  const init = async () => {
    // Set theme before content loads to prevent flash
    setTheme(getPreferredTheme());
    
    bindEvents();
    await refreshData();
    await loadLogs();
    startLogStream();
    setInterval(refreshData, 15000);
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
