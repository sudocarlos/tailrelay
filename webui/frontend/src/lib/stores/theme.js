import { writable, get } from 'svelte/store';

const DARK_MEDIA = '(prefers-color-scheme: dark)';
const STORAGE_KEY = 'theme';
let media = null;

// Stored user preference: 'system' | 'light' | 'dark'.
function storedMode() {
  try {
    return localStorage.getItem(STORAGE_KEY) || 'system';
  } catch {
    return 'system';
  }
}

function systemMode() {
  return window.matchMedia(DARK_MEDIA).matches ? 'dark' : 'light';
}

function resolve(mode) {
  return mode === 'system' ? systemMode() : mode;
}

// theme holds the user preference; resolvedTheme publishes the effective
// light/dark value (system preference collapsed) and is the single source
// that applies the `dark` class convention on <html>.
export const theme = writable(storedMode());

export const resolvedTheme = writable(resolve(get(theme)));

function applyResolved() {
  const mode = get(theme);
  resolvedTheme.set(resolve(mode));
}

theme.subscribe((mode) => {
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {} // storage unavailable — session-only preference
  applyResolved();
});

resolvedTheme.subscribe((resolved) => {
  document.documentElement.classList.toggle('dark', resolved === 'dark');
});

export function setTheme(mode) {
  theme.set(mode === 'light' || mode === 'dark' ? mode : 'system');
}

// Follow OS theme changes while the preference is 'system'.
export function initTheme() {
  applyResolved();
  media ??= window.matchMedia(DARK_MEDIA);
  media.addEventListener('change', () => {
    if (get(theme) === 'system') resolvedTheme.set(systemMode());
  });
}

// Brand icon path, swapped between the dark- and light-background PNG
// variants so it stays legible against the current theme's surface color.
export const brandIconSrc = {
  subscribe(run) {
    return resolvedTheme.subscribe((resolved) =>
      run(resolved === 'dark' ? '/icon-192.png' : '/icon-light-192.png')
    );
  },
};
