/**
 * Shared theme management for Tailrelay Web UI.
 *
 * Loaded synchronously (no defer) on every page so the correct
 * data-bs-theme and data-palette are applied before first paint, eliminating FOUC.
 *
 * Usage: 
 *   window.tailrelayTheme.initToggle(document.getElementById("theme-toggle"))
 *   window.tailrelayTheme.initPaletteDropdown(document.querySelectorAll(".palette-btn"))
 */
(() => {
  const ICON_PATH = "/static/vendor/bootstrap-icons/bootstrap-icons.svg";
  const THEME_STORAGE_KEY = "theme";
  const PALETTE_STORAGE_KEY = "palette";
  const ICONS = { dark: "moon-stars-fill", light: "sun-fill" };

  const getPreferredTheme = () => {
    const stored = localStorage.getItem(THEME_STORAGE_KEY);
    if (stored) return stored;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  };

  const getPreferredPalette = () => {
    return localStorage.getItem(PALETTE_STORAGE_KEY) || "sunset";
  };

  const applyTheme = (theme) => {
    document.documentElement.setAttribute("data-bs-theme", theme);
    localStorage.setItem(THEME_STORAGE_KEY, theme);
  };

  const applyPalette = (palette) => {
    document.documentElement.setAttribute("data-palette", palette);
    localStorage.setItem(PALETTE_STORAGE_KEY, palette);
  };

  const updateIcon = (toggleEl, theme) => {
    if (!toggleEl) return;
    const use = toggleEl.querySelector("use");
    if (use) use.setAttribute("href", `${ICON_PATH}#${ICONS[theme]}`);
  };

  /** Bind a toggle button: click swaps light/dark and updates its icon. */
  const initToggle = (toggleEl) => {
    if (!toggleEl) return;
    updateIcon(toggleEl, getPreferredTheme());
    toggleEl.addEventListener("click", () => {
      const current = document.documentElement.getAttribute("data-bs-theme") || "light";
      const next = current === "dark" ? "light" : "dark";
      applyTheme(next);
      updateIcon(toggleEl, next);
    });
  };

  /** Bind palette dropdown buttons */
  const initPaletteDropdown = (buttons) => {
    if (!buttons || !buttons.length) return;
    buttons.forEach(btn => {
      btn.addEventListener("click", (e) => {
        const selected = e.currentTarget.getAttribute("data-palette");
        if (selected) {
          applyPalette(selected);
        }
      });
    });
  };

  // Apply theme and palette immediately on load (before any paint).
  applyTheme(getPreferredTheme());
  applyPalette(getPreferredPalette());

  // Expose a minimal public API.
  window.tailrelayTheme = { initToggle, initPaletteDropdown };
})();
