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
                      <span class="badge text-bg-info" data-bs-toggle="tooltip" title="served by socat">TCP Relay</span>
                      <span class="fw-semibold">${formatRelayTitle(relay)}</span>
                    </div>
                    <div class="small text-muted mt-1">ID: ${relay.id}</div>
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
                  </div>
                </div>
              </div>
            </div>
          `;
        }

        const proxy = item.proxy;
        const running = proxy.running ?? proxy.Running;
        const runningBadge = running ? "text-bg-success" : "text-bg-secondary";
        const runningLabel = running ? "Caddy Running" : "Caddy Down";
        const autostart = proxy.autostart ?? false;
        return `
          <div class="col-12">
            <div class="card h-100">
              <div class="card-body d-flex flex-column flex-lg-row align-items-lg-center gap-3">
                <div class="flex-grow-1">
                  <div class="d-flex align-items-center gap-2 flex-wrap">
                    <span class="badge text-bg-primary" data-bs-toggle="tooltip" title="served by caddy">HTTPS Proxy</span>
                    <span class="fw-semibold">${formatProxyLink(proxy)} → ${proxy.target}</span>
                  </div>
                  <div class="small text-muted mt-1">ID: ${proxy.id}</div>
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

  const bindEvents = () => {
    elements.items.addEventListener("click", handleActionClick);
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
