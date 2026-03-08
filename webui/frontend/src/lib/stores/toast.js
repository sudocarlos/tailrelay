import { writable, get } from 'svelte/store';

/**
 * Simple toast notification store.
 * Each toast: { id, type ('success'|'danger'|'warning'|'info'), message }
 */
let nextId = 0;

export const toasts = writable([]);

export function showToast(type, message) {
  const id = nextId++;
  toasts.update((t) => [...t, { id, type, message }]);

  // Auto-dismiss after 5 seconds
  setTimeout(() => dismissToast(id), 5000);
}

export function dismissToast(id) {
  toasts.update((t) => t.filter((toast) => toast.id !== id));
}
