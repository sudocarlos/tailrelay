import { writable } from 'svelte/store';

function createThemeStore() {
  const stored = localStorage.getItem('theme') ||
    (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');

  const { subscribe, set, update } = writable(stored);

  return {
    subscribe,
    toggle() {
      update((current) => {
        const next = current === 'dark' ? 'light' : 'dark';
        localStorage.setItem('theme', next);
        document.documentElement.classList.toggle('dark', next === 'dark');
        return next;
      });
    },
    init() {
      // Apply on load
      document.documentElement.classList.toggle('dark', stored === 'dark');
    },
  };
}

export const theme = createThemeStore();
