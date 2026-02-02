(() => {
  const state = {
    relays: [],
    proxies: [],
    filter: "relay",
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
    refresh: document.getElementById("refresh"),
    clearLogs: document.getElementById("clear-logs"),
    filterRelay: document.getElementById("filter-relay"),
    filterProxy: document.getElementById("filter-proxy"),
  };

  const tooltips = [];

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

  const formatRelayTitle = (relay) =>
    `:${relay.listen_port} → ${relay.target_host}:${relay.target_port}`;

  const formatProxyTitle = (proxy) => {
    const portLabel = proxy.port ? `:${proxy.port}` : "";
    return `${proxy.hostname}${portLabel} → ${proxy.target}`;
  };

  const renderEmpty = (message) => {
    elements.items.innerHTML = `
      <div class="col-12">
        <div class="card shadow-sm">
          <div class="card-body text-center text-muted">
            ${message}
          </div>
        </div>
      </div>
    `;
  };

  const renderItems = () => {
    disposeTooltips();

    const items = state.filter === "relay" ? state.relays : state.proxies;
    elements.itemCount.textContent = `${items.length} item${items.length === 1 ? "" : "s"}`;

    if (!items.length) {
      renderEmpty(state.filter === "relay" ? "No TCP relays configured." : "No HTTPS proxies configured.");
      return;
    }

    elements.items.innerHTML = items
      .map((item) => {
        if (state.filter === "relay") {
          const relay = item.relay;
          const running = item.running;
          const statusBadge = running ? "text-bg-success" : "text-bg-secondary";
          const enabledBadge = relay.enabled ? "text-bg-primary" : "text-bg-warning";
          return `
            <div class="col-12">
              <div class="card shadow-sm h-100">
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
                    <span class="badge ${enabledBadge}">${relay.enabled ? "Enabled" : "Disabled"}</span>
                    <button class="btn btn-outline-secondary btn-sm action-btn" data-type="relay" data-id="${relay.id}" data-running="${running}">
                      ${running ? "⏸ Pause" : "▶ Start"}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          `;
        }

        const proxy = item;
        const enabledBadge = proxy.enabled ? "text-bg-success" : "text-bg-secondary";
        return `
          <div class="col-12">
            <div class="card shadow-sm h-100">
              <div class="card-body d-flex flex-column flex-lg-row align-items-lg-center gap-3">
                <div class="flex-grow-1">
                  <div class="d-flex align-items-center gap-2 flex-wrap">
                    <span class="badge text-bg-primary" data-bs-toggle="tooltip" title="served by caddy">HTTPS Proxy</span>
                    <span class="fw-semibold">${formatProxyTitle(proxy)}</span>
                  </div>
                  <div class="small text-muted mt-1">ID: ${proxy.id}</div>
                </div>
                <div class="d-flex align-items-center gap-2">
                  <span class="badge ${enabledBadge}">${proxy.enabled ? "Enabled" : "Disabled"}</span>
                  <button class="btn btn-outline-secondary btn-sm action-btn" data-type="proxy" data-id="${proxy.id}" data-enabled="${proxy.enabled}">
                    ${proxy.enabled ? "⏸ Pause" : "▶ Start"}
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
      const [relays, proxies] = await Promise.all([
        fetchJSON("/api/socat/relays"),
        fetchJSON("/api/caddy/proxies"),
      ]);

      state.relays = relays.map((status) => ({
        relay: status.Relay || status.relay,
        running: status.Running ?? status.running,
      }));
      state.proxies = proxies;

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
      elements.logOutput.textContent = "";
      state.logs.forEach(appendLogEntry);
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

    elements.filterRelay.addEventListener("change", () => {
      state.filter = "relay";
      renderItems();
    });

    elements.filterProxy.addEventListener("change", () => {
      state.filter = "proxy";
      renderItems();
    });

    elements.refresh.addEventListener("click", refreshData);
    elements.clearLogs.addEventListener("click", () => {
      elements.logOutput.textContent = "";
    });
  };

  const init = async () => {
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
