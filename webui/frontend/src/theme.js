/**
 * Shared theme management for Tailrelay Web UI.
 *
 * Loaded synchronously (no defer) on every page so the correct
 * data-bs-theme is applied before first paint, eliminating FOUC.
 *
 * Usage: window.tailrelayTheme.initToggle(document.getElementById("theme-toggle"))
 */
(() => {
  const ICON_PATH = "/static/vendor/bootstrap-icons/bootstrap-icons.svg";
  const STORAGE_KEY = "theme";
  const ICONS = { dark: "bi-moon-stars-fill", light: "bi-sun-fill" };

  const getPreferred = () => {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) return stored;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  };

  const apply = (theme) => {
    document.documentElement.setAttribute("data-bs-theme", theme);
    localStorage.setItem(STORAGE_KEY, theme);
  };

  const updateIcon = (toggleEl, theme) => {
    if (!toggleEl) return;
    const use = toggleEl.querySelector("use");
    if (use) use.setAttribute("href", `${ICON_PATH}#${ICONS[theme]}`);
  };

  /** Bind a toggle button: click swaps light/dark and updates its icon. */
  const initToggle = (toggleEl) => {
    if (!toggleEl) return;
    updateIcon(toggleEl, getPreferred());
    toggleEl.addEventListener("click", () => {
      const current = document.documentElement.getAttribute("data-bs-theme") || "light";
      const next = current === "dark" ? "light" : "dark";
      apply(next);
      updateIcon(toggleEl, next);
    });
  };

  // Apply theme immediately on load (before any paint).
  apply(getPreferred());

  // Expose a minimal public API for pages to bind their toggle button.
  window.tailrelayTheme = { initToggle };
})();
