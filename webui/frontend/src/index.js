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
    backups: [],
    targets: [],
    currentView: "dashboard",
    tourActive: false,
  };

  const elements = {
    items: document.getElementById("items"),
    lastUpdated: document.getElementById("last-updated"),
    itemCount: document.getElementById("item-count"),
    alertContainer: document.getElementById("alert-container"),
    logOutput: document.getElementById("log-output"),
    logLevelBtn: document.getElementById("log-level-btn"),
    logLevelItems: document.querySelectorAll(".log-level-item"),
    refresh: document.getElementById("refresh"),
    clearLogs: document.getElementById("clear-logs"),
    copyLogs: document.getElementById("copy-logs"),
    consoleToggle: document.getElementById("console-toggle"),
    filterRelay: document.getElementById("filter-relay"),
    filterProxy: document.getElementById("filter-proxy"),
    themeToggle: document.getElementById("theme-toggle"),
    fabButton: document.getElementById("fab-button"),
    saveItemBtn: document.getElementById("save-item-btn"),
    confirmDeleteBtn: document.getElementById("confirm-delete-btn"),
    removeTlsCertBtn: document.getElementById("proxy-tls-cert-remove"),
    helpTourBtn: document.getElementById("help-tour-btn"),
    toastContainer: document.getElementById("toast-container"),

    // Modal tabs
    relayTab: document.getElementById("relay-tab"),
    proxyTab: document.getElementById("proxy-tab"),

    // Navigation
    navDashboard: document.getElementById("nav-dashboard"),
    navBackups: document.getElementById("nav-backups"),

    // Views
    dashboardView: document.getElementById("dashboard-view"),
    backupsView: document.getElementById("backups-view"),

    // Backup elements
    backupList: document.getElementById("backup-list"),
    backupEmptyState: document.getElementById("backup-empty-state"),
    createBackupBtn: document.getElementById("create-backup-btn"),
    uploadBackupBtn: document.getElementById("upload-backup-btn"),
    confirmUploadBtn: document.getElementById("confirm-upload-btn"),
    uploadBackupForm: document.getElementById("uploadBackupForm"),
    backupFile: document.getElementById("backupFile"),

    // Targets
    relayPresetTarget: document.getElementById("relay-preset-target"),
    proxyPresetTarget: document.getElementById("proxy-preset-target"),
  };

  const tooltips = [];

  // =============================================
  // Dark mode management (provided by theme.bundle.js)
  // =============================================

  // =============================================
  // Network
  // =============================================
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

  // =============================================
  // Toast notifications (replaces showAlert)
  // =============================================
  const showToast = (type, message) => {
    const iconMap = {
      success: "bi-check-circle-fill",
      danger: "bi-exclamation-triangle-fill",
      warning: "bi-exclamation-triangle-fill",
      info: "bi-info-circle-fill",
    };

    const colorMap = {
      success: "text-success",
      danger: "text-danger",
      warning: "text-warning",
      info: "text-info",
    };

    const icon = iconMap[type] || "bi-info-circle-fill";
    const color = colorMap[type] || "text-info";

    const toastEl = document.createElement("div");
    toastEl.className = "toast show border-0 shadow-sm";
    toastEl.setAttribute("role", "alert");
    toastEl.setAttribute("aria-live", "assertive");
    toastEl.setAttribute("aria-atomic", "true");
    toastEl.innerHTML = `
      <div class="toast-body d-flex align-items-start gap-2">
        <svg class="bi ${color} flex-shrink-0" style="width:1.25em;height:1.25em" aria-hidden="true">
          <use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#${icon}"></use>
        </svg>
        <div class="flex-grow-1">${message}</div>
        <button type="button" class="btn-close btn-close-sm ms-2" aria-label="Close"></button>
      </div>
    `;

    toastEl.querySelector(".btn-close").addEventListener("click", () => {
      toastEl.classList.remove("show");
      setTimeout(() => toastEl.remove(), 200);
    });

    elements.toastContainer.appendChild(toastEl);

    setTimeout(() => {
      toastEl.classList.remove("show");
      setTimeout(() => toastEl.remove(), 200);
    }, 5000);
  };

  // Keep showAlert as alias for backward compat in backup code
  const showAlert = showToast;

  // =============================================
  // Render helpers
  // =============================================
  const formatRelayTitle = (relay) => {
    const fqdn = state.tailnetFQDN || "unknown";
    return `tcp://${fqdn}:${relay.listen_port}`;
  };

  const formatRelayTarget = (relay) => {
    return `→ ${relay.target_host}:${relay.target_port}`;
  };

  const formatProxyLink = (proxy) => {
    const portLabel = proxy.port ? `:${proxy.port}` : "";
    const url = `https://${proxy.hostname}${portLabel}`;
    return `<a class="proxy-link" href="${url}" target="_blank" rel="noopener">${url}</a>`;
  };

  const emptyStateSvg = `
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 80 80" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
      <rect x="10" y="20" width="60" height="40" rx="4" />
      <line x1="10" y1="32" x2="70" y2="32" />
      <circle cx="18" cy="26" r="2" fill="currentColor" />
      <circle cx="26" cy="26" r="2" fill="currentColor" />
      <circle cx="34" cy="26" r="2" fill="currentColor" />
      <line x1="30" y1="44" x2="50" y2="44" />
      <line x1="25" y1="50" x2="55" y2="50" />
    </svg>
  `;

  const renderEmpty = (message) => {
    let ctaLabel, ctaType;
    if (state.showRelays && !state.showProxies) {
      ctaLabel = "Add a Relay";
      ctaType = "relay";
    } else if (!state.showRelays && state.showProxies) {
      ctaLabel = "Add a Proxy";
      ctaType = "proxy";
    } else {
      ctaLabel = "Add a Proxy or Relay";
      ctaType = "both";
    }

    elements.items.innerHTML = `
      <div class="col-12">
        <div class="card">
          <div class="card-body empty-state">
            ${emptyStateSvg}
            <p>${message}</p>
            <button class="btn btn-primary btn-sm empty-state-cta" data-type="${ctaType}">
              <svg class="bi me-1" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#plus-lg"></use></svg>
              ${ctaLabel}
            </button>
          </div>
        </div>
      </div>
    `;

    // Bind CTA
    elements.items.querySelector(".empty-state-cta")?.addEventListener("click", (e) => {
      const type = e.currentTarget.dataset.type;
      if (type === "relay") {
        openAddModal("relay");
      } else if (type === "proxy") {
        openAddModal("proxy");
      } else {
        openAddModal("relay");
      }
    });
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
        renderEmpty("No TCP relays configured. Get started by adding one.");
      } else if (!state.showRelays && state.showProxies) {
        renderEmpty("No HTTPS proxies configured. Get started by adding one.");
      } else {
        renderEmpty("No relays or proxies configured. Get started by adding one.");
      }
      return;
    }

    elements.items.innerHTML = filtered
      .map((item) => {
        if (item.type === "relay") {
          const relay = item.relay;
          const running = item.running;
          const autostart = relay.autostart ?? false;
          const statusClass = running ? "running" : "stopped";
          const statusLabel = running ? "Running" : "Stopped";
          const actionIcon = running ? "bi-pause-fill" : "bi-play-fill";
          const actionTooltip = running ? "Pause" : "Start";
          const actionBtnClass = running ? "btn-outline-secondary" : "btn-outline-success";
          return `
            <div class="col-12">
              <div class="card h-100">
                <div class="card-body d-flex flex-column flex-lg-row align-items-lg-center gap-3">
                  <div class="flex-grow-1">
                    <div class="d-flex align-items-center gap-2 flex-wrap">
                      <svg class="bi text-primary" data-bs-toggle="tooltip" title="TCP Relay (served by socat)" aria-hidden="true" style="width: 1.25em; height: 1.25em;"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#diagram-3"></use></svg>
                      <span class="fw-semibold">${formatRelayTitle(relay)}</span>
                      <span class="status-dot ${statusClass} ms-1" data-bs-toggle="tooltip" title="${statusLabel}"></span>
                    </div>
                    <div class="small text-muted mt-1">
                      ${formatRelayTarget(relay)}
                    </div>
                  </div>
                  <div class="d-flex align-items-center gap-2">
                    <div class="form-check form-switch m-0" data-bs-toggle="tooltip" title="Start automatically on container boot">
                      <label class="form-check-label small text-muted">Autostart</label>
                      <input class="form-check-input autostart-toggle" type="checkbox" role="switch" 
                             ${autostart ? "checked" : ""} 
                             data-type="relay" data-id="${relay.id}">
                    </div>
                    <div class="vr"></div>
                    <button class="btn ${actionBtnClass} btn-sm action-btn" data-type="relay" data-id="${relay.id}" data-running="${running}" data-bs-toggle="tooltip" title="${actionTooltip}">
                      <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#${actionIcon}"></use></svg>
                    </button>
                    <button class="btn btn-outline-primary btn-sm edit-btn" data-type="relay" data-id="${relay.id}" data-bs-toggle="tooltip" title="Edit">
                      <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#pencil"></use></svg>
                    </button>
                    <button class="btn btn-outline-danger btn-sm delete-btn" data-type="relay" data-id="${relay.id}" data-name="tcp://${state.tailnetFQDN}:${relay.listen_port}" data-bs-toggle="tooltip" title="Delete">
                      <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#trash"></use></svg>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          `;
        }

        const proxy = item.proxy;
        const running = proxy.running ?? proxy.Running;
        const autostart = proxy.autostart ?? false;
        const proxyName = proxy.port ? `${proxy.hostname}:${proxy.port}` : proxy.hostname;
        const statusClass = running ? "running" : "stopped";
        const statusLabel = running ? "Running" : "Stopped";
        const actionIcon = proxy.enabled ? "bi-pause-fill" : "bi-play-fill";
        const actionTooltip = proxy.enabled ? "Pause" : "Start";
        const actionBtnClass = proxy.enabled ? "btn-outline-secondary" : "btn-outline-success";
        return `
          <div class="col-12">
            <div class="card h-100">
              <div class="card-body d-flex flex-column flex-lg-row align-items-lg-center gap-3">
                <div class="flex-grow-1">
                  <div class="d-flex align-items-center gap-2 flex-wrap">
                    <svg class="bi text-primary" data-bs-toggle="tooltip" title="HTTPS Proxy (served by Caddy)" aria-hidden="true" style="width: 1.25em; height: 1.25em;"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#shield-lock"></use></svg>
                    <span class="fw-semibold">${formatProxyLink(proxy)}</span>
                    <span class="status-dot ${statusClass} ms-1" data-bs-toggle="tooltip" title="${statusLabel}"></span>
                  </div>
                  <div class="small text-muted mt-1">
                    → ${proxy.target}
                  </div>
                </div>
                <div class="d-flex align-items-center gap-2">
                  <div class="form-check form-switch m-0" data-bs-toggle="tooltip" title="Start automatically on container boot">
                    <label class="form-check-label small text-muted">Autostart</label>
                    <input class="form-check-input autostart-toggle" type="checkbox" role="switch" 
                           ${autostart ? "checked" : ""} 
                           data-type="proxy" data-id="${proxy.id}">
                  </div>
                  <div class="vr"></div>
                  <button class="btn ${actionBtnClass} btn-sm action-btn" data-type="proxy" data-id="${proxy.id}" data-enabled="${proxy.enabled}" data-bs-toggle="tooltip" title="${actionTooltip}">
                    <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#${actionIcon}"></use></svg>
                  </button>
                  <button class="btn btn-outline-primary btn-sm edit-btn" data-type="proxy" data-id="${proxy.id}" data-bs-toggle="tooltip" title="Edit">
                    <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#pencil"></use></svg>
                  </button>
                  <button class="btn btn-outline-danger btn-sm delete-btn" data-type="proxy" data-id="${proxy.id}" data-name="https://${proxyName}" data-bs-toggle="tooltip" title="Delete">
                    <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#trash"></use></svg>
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

  // =============================================
  // Backup logic (kept intact, nav hidden)
  // =============================================
  const renderBackups = () => {
    const list = elements.backupList;
    if (!list) return;

    list.innerHTML = "";

    if (!state.backups.length) {
      elements.backupEmptyState.classList.remove("d-none");
      return;
    }

    elements.backupEmptyState.classList.add("d-none");

    state.backups.forEach(backup => {
      const row = document.createElement("tr");

      const date = new Date(backup.Timestamp);
      const sizeFormatted = formatSize(backup.Size);
      const type = backup.Metadata?.BackupType || "full";

      row.innerHTML = `
        <td><strong>${backup.Filename}</strong></td>
        <td>${sizeFormatted}</td>
        <td>${date.toLocaleString()}</td>
        <td><span class="badge text-bg-secondary">${type}</span></td>
        <td class="text-end">
          <button class="btn btn-sm btn-outline-primary download-backup-btn me-1" data-filename="${backup.Filename}">
            <svg class="bi me-1" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#download"></use></svg>
            Download
          </button>
          <button class="btn btn-sm btn-outline-warning restore-backup-btn me-1" data-filename="${backup.Filename}">
            <svg class="bi me-1" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#arrow-counterclockwise"></use></svg>
            Restore
          </button>
          <button class="btn btn-sm btn-outline-danger delete-backup-btn" data-filename="${backup.Filename}">
            <svg class="bi" aria-hidden="true"><use href="/static/vendor/bootstrap-icons/bootstrap-icons.svg#trash"></use></svg>
          </button>
        </td>
      `;

      list.appendChild(row);
    });
  };

  const formatSize = (bytes) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  };

  const switchView = (view) => {
    state.currentView = view;

    if (view === "dashboard") {
      elements.navDashboard.classList.add("active");
      if (elements.navBackups) elements.navBackups.classList.remove("active");
      elements.dashboardView.classList.remove("d-none");
      if (elements.backupsView) elements.backupsView.classList.add("d-none");
      document.querySelector(".fab-container").classList.remove("d-none");
    } else {
      elements.navDashboard.classList.remove("active");
      if (elements.navBackups) elements.navBackups.classList.add("active");
      elements.dashboardView.classList.add("d-none");
      if (elements.backupsView) elements.backupsView.classList.remove("d-none");
      document.querySelector(".fab-container").classList.add("d-none");

      refreshBackups();
    }
  };

  const refreshBackups = async () => {
    try {
      const data = await fetchJSON("/api/backup/list");
      state.backups = data.backups || [];
      renderBackups();
    } catch (error) {
      showToast("danger", "Failed to load backups: " + error.message);
    }
  };

  const handleCreateBackup = async () => {
    if (!confirm("Create a new full system backup?")) return;

    try {
      if (elements.createBackupBtn) elements.createBackupBtn.disabled = true;
      await fetchJSON("/api/backup/create", {
        method: "POST",
        body: JSON.stringify({ backup_type: "full" })
      });

      showToast("success", "Backup created successfully");
      await refreshBackups();
    } catch (error) {
      showToast("danger", error.message);
    } finally {
      if (elements.createBackupBtn) elements.createBackupBtn.disabled = false;
    }
  };

  const handleUploadBackup = () => {
    const modal = new bootstrap.Modal(document.getElementById("uploadBackupModal"));
    if (elements.uploadBackupForm) elements.uploadBackupForm.reset();
    modal.show();
  };

  const handleConfirmUpload = async () => {
    const fileInput = elements.backupFile;
    if (!fileInput || !fileInput.files.length) {
      showToast("warning", "Please select a file");
      return;
    }

    const file = fileInput.files[0];
    const formData = new FormData();
    formData.append("backup", file);

    try {
      if (elements.confirmUploadBtn) elements.confirmUploadBtn.disabled = true;

      const uploadResp = await fetch("/api/backup/upload", {
        method: "POST",
        body: formData,
      });

      if (!uploadResp.ok) {
        throw new Error(await uploadResp.text());
      }

      const uploadResult = await uploadResp.json();
      const filename = uploadResult.filename;

      showToast("info", "Upload successful. Restoring...");
      await fetchJSON("/api/backup/restore", {
        method: "POST",
        body: JSON.stringify({ filename })
      });

      bootstrap.Modal.getInstance(document.getElementById("uploadBackupModal")).hide();
      showToast("success", "System restored successfully. Reloading...");
      setTimeout(() => location.reload(), 2000);

    } catch (error) {
      showToast("danger", "Operation failed: " + error.message);
      if (elements.confirmUploadBtn) elements.confirmUploadBtn.disabled = false;
    }
  };

  const handleRestoreBackup = async (filename) => {
    if (!confirm(`Restore from backup "${filename}"? Current configuration will be overwritten.`)) return;

    try {
      await fetchJSON("/api/backup/restore", {
        method: "POST",
        body: JSON.stringify({ filename })
      });

      showToast("success", "System restored successfully. Reloading...");
      setTimeout(() => location.reload(), 2000);
    } catch (error) {
      showToast("danger", error.message);
    }
  };

  const handleDeleteBackup = async (filename) => {
    if (!confirm(`Delete backup "${filename}"?`)) return;

    try {
      await fetchJSON(`/api/backup/delete?filename=${encodeURIComponent(filename)}`, {
        method: "DELETE"
      });

      showToast("success", "Backup deleted");
      await refreshBackups();
    } catch (error) {
      showToast("danger", error.message);
    }
  };

  const handleDownloadBackup = (filename) => {
    window.location.href = `/api/backup/download?filename=${encodeURIComponent(filename)}`;
  };

  const handleBackupListClick = (e) => {
    const btn = e.target.closest("button");
    if (!btn) return;

    const filename = btn.dataset.filename;

    if (btn.classList.contains("download-backup-btn")) {
      handleDownloadBackup(filename);
    } else if (btn.classList.contains("restore-backup-btn")) {
      handleRestoreBackup(filename);
    } else if (btn.classList.contains("delete-backup-btn")) {
      handleDeleteBackup(filename);
    }
  };

  // =============================================
  // Tooltips
  // =============================================
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

  // =============================================
  // Data
  // =============================================
  const refreshData = async () => {
    try {
      const [relays, proxies, status, targets] = await Promise.all([
        fetchJSON("/api/socat/relays"),
        fetchJSON("/api/caddy/proxies"),
        fetchJSON("/api/tailscale/status"),
        fetchJSON("/api/targets"),
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
      state.targets = targets || [];

      renderItems();
      setLastUpdated();
      populateTargetSelects();
    } catch (error) {
      showToast("danger", error.message);
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

    const currentItem = type === "relay"
      ? state.relays.find(r => r.relay.id === id)?.relay
      : state.proxies.find(p => p.id === id);

    if (!currentItem) {
      throw new Error(`${type} not found`);
    }

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
      showToast("danger", error.message);
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
      showToast("danger", error.message);
      toggle.checked = !autostart;
    } finally {
      toggle.disabled = false;
    }
  };

  // =============================================
  // Logs
  // =============================================
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
      if (elements.logLevelBtn) {
        elements.logLevelBtn.textContent = state.logLevel;
      }
      elements.logOutput.textContent = "";
      state.logs.forEach(appendLogEntry);
    } catch (error) {
      showToast("warning", error.message);
    }
  };

  const setLogLevel = async (level) => {
    try {
      const response = await fetchJSON("/api/logs/level", {
        method: "POST",
        body: JSON.stringify({ level }),
      });
      state.logLevel = response.level || level;
      if (elements.logLevelBtn) {
        elements.logLevelBtn.textContent = state.logLevel;
      }
    } catch (error) {
      showToast("warning", error.message);
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
      showToast("warning", "Log stream disconnected. Retrying...");
    };

    state.logStream = stream;
  };

  const copyLogs = async () => {
    const text = elements.logOutput.textContent;
    if (!text.trim()) {
      showToast("info", "No logs to copy");
      return;
    }
    try {
      await navigator.clipboard.writeText(text);
      showToast("success", "Logs copied to clipboard");
    } catch {
      showToast("danger", "Failed to copy logs");
    }
  };

  // =============================================
  // Modals & Targets
  // =============================================
  const populateTargetSelects = () => {
    [elements.relayPresetTarget, elements.proxyPresetTarget].forEach(select => {
      if (!select) return;

      // Keep only the first "Custom..." option
      while (select.options.length > 1) {
        select.remove(1);
      }

      state.targets.forEach((target, index) => {
        // Option text shows target_name if present, else fallback
        const label = target.target_name || `${target.app_id} (${target.port})`;
        const option = new Option(label, index.toString());
        select.add(option);
      });
    });
  };

  // Returns the currently active tab type ("relay" or "proxy")
  const getActiveTabType = () => {
    return elements.proxyTab?.classList.contains("active") ? "proxy" : "relay";
  };

  // Switch the modal to the given tab and set enabled/disabled state
  const setModalTab = (type, editing) => {
    const relayTab = elements.relayTab;
    const proxyTab = elements.proxyTab;
    const relayPane = document.getElementById("relay-tab-pane");
    const proxyPane = document.getElementById("proxy-tab-pane");

    if (editing) {
      // Disable the other tab when editing so only the relevant form is shown
      if (type === "relay") {
        relayTab.classList.add("active");
        relayTab.setAttribute("aria-selected", "true");
        relayTab.disabled = false;
        proxyTab.classList.remove("active");
        proxyTab.setAttribute("aria-selected", "false");
        proxyTab.disabled = true;
        relayPane.classList.add("show", "active");
        proxyPane.classList.remove("show", "active");
      } else {
        proxyTab.classList.add("active");
        proxyTab.setAttribute("aria-selected", "true");
        proxyTab.disabled = false;
        relayTab.classList.remove("active");
        relayTab.setAttribute("aria-selected", "false");
        relayTab.disabled = true;
        proxyPane.classList.add("show", "active");
        relayPane.classList.remove("show", "active");
      }
    } else {
      // Both tabs enabled when adding; activate the requested one
      relayTab.disabled = false;
      proxyTab.disabled = false;
      if (type === "relay") {
        relayTab.classList.add("active");
        relayTab.setAttribute("aria-selected", "true");
        proxyTab.classList.remove("active");
        proxyTab.setAttribute("aria-selected", "false");
        relayPane.classList.add("show", "active");
        proxyPane.classList.remove("show", "active");
      } else {
        proxyTab.classList.add("active");
        proxyTab.setAttribute("aria-selected", "true");
        relayTab.classList.remove("active");
        relayTab.setAttribute("aria-selected", "false");
        proxyPane.classList.add("show", "active");
        relayPane.classList.remove("show", "active");
      }
    }
  };

  const openAddModal = (type = "relay", item = null) => {
    const modal = new bootstrap.Modal(document.getElementById("addModal"));
    const modalTitle = document.getElementById("addModalLabel");
    const editing = item !== null;

    state.currentEditItem = item;
    state.currentEditType = type;
    state.removeTlsCert = false;

    if (editing) {
      modalTitle.textContent = type === "relay" ? "Edit Relay" : "Edit Proxy";
    } else {
      modalTitle.textContent = "Add Configuration";
    }

    setModalTab(type, editing);

    if (type === "relay") {
      if (item) {
        document.getElementById("relay-id").value = item.id;
        document.getElementById("relay-listen-port").value = item.listen_port;
        document.getElementById("relay-target-host").value = item.target_host;
        document.getElementById("relay-target-port").value = item.target_port;
        document.getElementById("relay-autostart").checked = item.autostart ?? false;
        elements.relayPresetTarget.value = "";
      } else {
        document.getElementById("relayForm").reset();
        document.getElementById("relay-id").value = "";
        document.getElementById("relay-autostart").checked = true;
        elements.relayPresetTarget.value = "";
      }
    } else {
      const certCurrent = document.getElementById("proxy-tls-cert-current");
      const certFilename = document.getElementById("proxy-tls-cert-filename");
      const certFileInput = document.getElementById("proxy-tls-cert");

      if (item) {
        document.getElementById("proxy-id").value = item.id;
        document.getElementById("proxy-port").value = item.port || "";
        document.getElementById("proxy-target").value = item.target;
        document.getElementById("proxy-trusted-proxies").checked = item.trusted_proxies ?? false;
        document.getElementById("proxy-autostart").checked = item.autostart ?? false;

        certFileInput.value = "";
        if (item.tls_cert_file) {
          const basename = item.tls_cert_file.split('/').pop();
          certFilename.textContent = basename;
          certCurrent.style.display = "flex";
        } else {
          certCurrent.style.display = "none";
        }
        elements.proxyPresetTarget.value = "";
      } else {
        document.getElementById("proxyForm").reset();
        document.getElementById("proxy-id").value = "";
        document.getElementById("proxy-autostart").checked = true;
        document.getElementById("proxy-trusted-proxies").checked = true;
        certFileInput.value = "";
        certCurrent.style.display = "none";
        elements.proxyPresetTarget.value = "";
      }
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
      showToast("danger", "Please fill in all required fields");
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
      elements.saveItemBtn.disabled = true;
      const url = id ? "/api/socat/update" : "/api/socat/create";
      await fetchJSON(url, {
        method: "POST",
        body: JSON.stringify(relay),
      });

      bootstrap.Modal.getInstance(document.getElementById("addModal")).hide();
      showToast("success", `Relay ${id ? "updated" : "created"} successfully`);
      await refreshData();
    } catch (error) {
      showToast("danger", error.message);
    } finally {
      elements.saveItemBtn.disabled = false;
    }
  };

  const saveProxy = async () => {
    const id = document.getElementById("proxy-id").value;
    const port = document.getElementById("proxy-port").value.trim();
    const target = document.getElementById("proxy-target").value.trim();
    const trustedProxies = document.getElementById("proxy-trusted-proxies").checked;
    const autostart = document.getElementById("proxy-autostart").checked;
    const tlsCertFile = document.getElementById("proxy-tls-cert").files[0];

    const hostname = state.tailnetFQDN.replace(/\.$/, '');

    if (!hostname) {
      showToast("danger", "MagicDNS hostname not available. Please ensure Tailscale is connected.");
      return;
    }

    if (!target) {
      showToast("danger", "Please fill in the target URL");
      return;
    }

    // Frontend validation for cert file
    if (tlsCertFile) {
      const validExtensions = ['.pem', '.crt', '.cer'];
      const fileName = tlsCertFile.name.toLowerCase();
      const isValidExt = validExtensions.some(ext => fileName.endsWith(ext));
      if (!isValidExt) {
        showToast("danger", "Invalid certificate file. Please upload a .pem, .crt, or .cer file.");
        return;
      }

      if (tlsCertFile.size > 1024 * 1024) {
        showToast("danger", "Certificate file too large. Maximum size is 1MB.");
        return;
      }
    }

    const formData = new FormData();
    formData.append("hostname", hostname);
    formData.append("target", target);
    formData.append("trusted_proxies", trustedProxies.toString());
    formData.append("autostart", autostart.toString());
    formData.append("enabled", "true");

    if (!port) {
      showToast("danger", "Port is required");
      return;
    }

    const portNum = parseInt(port);
    if ([80, 443, 8021].includes(portNum)) {
      showToast("danger", "Ports 80, 443, and 8021 are reserved and cannot be used");
      return;
    }

    formData.append("port", port);

    if (id) {
      formData.append("id", id);
    }

    if (tlsCertFile) {
      formData.append("tls_cert_upload", tlsCertFile);
    }

    if (state.removeTlsCert) {
      formData.append("remove_tls_cert", "true");
    }

    try {
      elements.saveItemBtn.disabled = true;
      const url = id ? "/api/caddy/update" : "/api/caddy/create";

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

      bootstrap.Modal.getInstance(document.getElementById("addModal")).hide();
      showToast("success", `Proxy ${id ? "updated" : "created"} successfully`);
      await refreshData();
    } catch (error) {
      showToast("danger", error.message);
    } finally {
      elements.saveItemBtn.disabled = false;
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
      showToast("success", `${type === "relay" ? "Relay" : "Proxy"} deleted successfully`);
      await refreshData();
    } catch (error) {
      showToast("danger", error.message);
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
        openAddModal("relay", relay);
      }
    } else if (type === "proxy") {
      const proxy = state.proxies.find(p => p.id === id);
      if (proxy) {
        openAddModal("proxy", proxy);
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

  // =============================================
  // Guided Tour
  // =============================================
  const tourSteps = [
    {
      target: "#fab-button",
      title: "Add Proxy or Relay",
      description: "Click the + button to create a new HTTPS proxy or TCP relay. Proxies are served by Caddy, relays by socat.",
    },
    {
      target: "#console-card",
      title: "Debug Console",
      description: "View real-time logs from all services. Use the log level selector to filter, and the copy button to grab all output.",
    },
    {
      target: ".autostart-toggle",
      title: "Autostart",
      description: "Toggle this switch to automatically start a proxy or relay when the container boots. No manual intervention needed.",
      fallbackText: "The Autostart toggle appears on each proxy/relay card. It lets services start automatically on boot.",
    },
    {
      target: "#filter-relay-buttons",
      title: "Filter Items",
      description: "Use these toggles to show or hide TCP relays and HTTPS proxies in the list below.",
    },
  ];

  let currentTourStep = -1;
  let tourOverlay = null;

  const startTour = () => {
    if (state.tourActive) return;
    state.tourActive = true;
    currentTourStep = -1;

    // Create overlay container
    tourOverlay = document.createElement("div");
    tourOverlay.className = "tour-overlay";
    tourOverlay.innerHTML = `
      <div class="tour-highlight"></div>
      <div class="tour-popover"></div>
    `;
    document.body.appendChild(tourOverlay);

    // Click backdrop to dismiss
    tourOverlay.addEventListener("click", (e) => {
      if (e.target === tourOverlay) endTour();
    });

    nextTourStep();
  };

  const nextTourStep = () => {
    currentTourStep++;
    if (currentTourStep >= tourSteps.length) {
      endTour();
      return;
    }

    const step = tourSteps[currentTourStep];
    const targetEl = document.querySelector(step.target);
    const highlight = tourOverlay.querySelector(".tour-highlight");
    const popover = tourOverlay.querySelector(".tour-popover");

    const isLast = currentTourStep === tourSteps.length - 1;

    if (!targetEl) {
      // If target not found (e.g., no items yet), show fallback
      highlight.style.display = "none";
      popover.style.position = "fixed";
      popover.style.top = "50%";
      popover.style.left = "50%";
      popover.style.transform = "translate(-50%, -50%)";
      popover.innerHTML = `
        <h6>${step.title}</h6>
        <p>${step.fallbackText || step.description}</p>
        <div class="tour-popover-footer">
          <span class="tour-step-indicator">${currentTourStep + 1} / ${tourSteps.length}</span>
          <div class="d-flex gap-2">
            <button class="btn btn-sm btn-outline-secondary tour-skip-btn">Skip</button>
            <button class="btn btn-sm btn-primary tour-next-btn">${isLast ? "Done" : "Next"}</button>
          </div>
        </div>
      `;
    } else {
      highlight.style.display = "block";
      const rect = targetEl.getBoundingClientRect();
      const pad = 6;

      highlight.style.top = `${rect.top - pad}px`;
      highlight.style.left = `${rect.left - pad}px`;
      highlight.style.width = `${rect.width + pad * 2}px`;
      highlight.style.height = `${rect.height + pad * 2}px`;

      // Position popover below or above target
      popover.style.position = "fixed";
      popover.style.transform = "none";
      const popoverWidth = 320;

      // Try below first
      const belowTop = rect.bottom + 12;
      const aboveTop = rect.top - 12;

      if (belowTop + 200 < window.innerHeight) {
        popover.style.top = `${belowTop}px`;
      } else {
        popover.style.top = `${aboveTop}px`;
        popover.style.transform = "translateY(-100%)";
      }

      // Center horizontally relative to target, clamped to viewport
      let left = rect.left + rect.width / 2 - popoverWidth / 2;
      left = Math.max(16, Math.min(left, window.innerWidth - popoverWidth - 16));
      popover.style.left = `${left}px`;

      popover.innerHTML = `
        <h6>${step.title}</h6>
        <p>${step.description}</p>
        <div class="tour-popover-footer">
          <span class="tour-step-indicator">${currentTourStep + 1} / ${tourSteps.length}</span>
          <div class="d-flex gap-2">
            <button class="btn btn-sm btn-outline-secondary tour-skip-btn">Skip</button>
            <button class="btn btn-sm btn-primary tour-next-btn">${isLast ? "Done" : "Next"}</button>
          </div>
        </div>
      `;
    }

    popover.querySelector(".tour-next-btn").addEventListener("click", nextTourStep);
    popover.querySelector(".tour-skip-btn").addEventListener("click", endTour);
  };

  const endTour = () => {
    state.tourActive = false;
    currentTourStep = -1;
    if (tourOverlay) {
      tourOverlay.remove();
      tourOverlay = null;
    }
  };

  // =============================================
  // Keyboard shortcuts
  // =============================================
  const handleKeyboardShortcuts = (e) => {
    // Skip if typing in an input/textarea or modal is open
    const tag = e.target.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
    if (document.querySelector(".modal.show")) return;
    if (state.tourActive) return;

    switch (e.key.toLowerCase()) {
      case "r":
        e.preventDefault();
        refreshData();
        break;
      case "n":
        e.preventDefault();
        openAddModal("relay");
        break;
      case "?":
        e.preventDefault();
        startTour();
        break;
    }
  };

  // =============================================
  // Console toggle state sync
  // =============================================
  const syncConsoleToggle = () => {
    const consoleEl = document.getElementById("log-console");
    const toggleBtn = elements.consoleToggle;
    if (!consoleEl || !toggleBtn) return;

    consoleEl.addEventListener("shown.bs.collapse", () => {
      toggleBtn.classList.remove("collapsed");
    });
    consoleEl.addEventListener("hidden.bs.collapse", () => {
      toggleBtn.classList.add("collapsed");
    });
  };

  // =============================================
  // Event binding
  // =============================================
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

    if (elements.themeToggle && window.tailrelayTheme) {
      window.tailrelayTheme.initToggle(elements.themeToggle);
      window.tailrelayTheme.initPaletteDropdown(document.querySelectorAll(".palette-btn"));
    }

    elements.refresh.addEventListener("click", refreshData);
    elements.clearLogs.addEventListener("click", () => {
      elements.logOutput.textContent = "";
    });

    if (elements.copyLogs) {
      elements.copyLogs.addEventListener("click", copyLogs);
    }

    if (elements.logLevelItems) {
      elements.logLevelItems.forEach(item => {
        item.addEventListener("click", (event) => {
          event.preventDefault();
          const newLevel = event.target.dataset.level;
          setLogLevel(newLevel);
        });
      });
    }

    // FAB and modal events
    if (elements.fabButton) {
      elements.fabButton.addEventListener("click", () => {
        openAddModal("relay");
      });
    }

    if (elements.saveItemBtn) {
      elements.saveItemBtn.addEventListener("click", () => {
        const activeType = getActiveTabType();
        if (activeType === "proxy") {
          saveProxy();
        } else {
          saveRelay();
        }
      });
    }

    if (elements.confirmDeleteBtn) {
      elements.confirmDeleteBtn.addEventListener("click", confirmDelete);
    }

    // Handle remove TLS cert button
    if (elements.removeTlsCertBtn) {
      elements.removeTlsCertBtn.addEventListener("click", () => {
        state.removeTlsCert = true;
        document.getElementById("proxy-tls-cert-current").style.display = "none";
        showToast("info", "Certificate will be removed when you save the proxy.");
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

    // Handle preset target selection
    if (elements.relayPresetTarget) {
      elements.relayPresetTarget.addEventListener("change", (e) => {
        const idx = e.target.value;
        if (idx !== "") {
          const target = state.targets[parseInt(idx)];
          document.getElementById("relay-target-host").value = target.host || "";
          if (target.port) {
            document.getElementById("relay-target-port").value = target.port;
          }
        }
      });
    }

    if (elements.proxyPresetTarget) {
      elements.proxyPresetTarget.addEventListener("change", (e) => {
        const idx = e.target.value;
        if (idx !== "") {
          const target = state.targets[parseInt(idx)];
          let targetUrl = target.host || "";
          if (target.port) {
            targetUrl += `:${target.port}`;
          }
          document.getElementById("proxy-target").value = targetUrl;
        }
      });
    }

    // Backup events (kept intact — nav link hidden, but event handlers still work)
    if (elements.navDashboard) {
      elements.navDashboard.addEventListener("click", (e) => {
        e.preventDefault();
        switchView("dashboard");
      });
    }

    if (elements.navBackups) {
      elements.navBackups.addEventListener("click", (e) => {
        e.preventDefault();
        switchView("backups");
      });
    }

    if (elements.createBackupBtn) {
      elements.createBackupBtn.addEventListener("click", handleCreateBackup);
    }

    if (elements.uploadBackupBtn) {
      elements.uploadBackupBtn.addEventListener("click", handleUploadBackup);
    }

    if (elements.confirmUploadBtn) {
      elements.confirmUploadBtn.addEventListener("click", handleConfirmUpload);
    }

    if (elements.backupList) {
      elements.backupList.addEventListener("click", handleBackupListClick);
    }

    // Brand link to dashboard
    const brandLink = document.getElementById("nav-brand");
    if (brandLink) {
      brandLink.addEventListener("click", (e) => {
        e.preventDefault();
        switchView("dashboard");
      });
    }

    // Help / Tour
    if (elements.helpTourBtn) {
      elements.helpTourBtn.addEventListener("click", (e) => {
        e.preventDefault();
        startTour();
      });
    }

    // Keyboard shortcuts
    document.addEventListener("keydown", handleKeyboardShortcuts);
  };

  // =============================================
  // Init
  // =============================================
  const init = async () => {
    // Theme is already applied by theme.bundle.js (loaded synchronously).
    // Just bind events and start the app.

    bindEvents();
    syncConsoleToggle();
    initTooltips();
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
