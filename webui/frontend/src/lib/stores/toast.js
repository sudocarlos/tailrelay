import { toast as sonner } from 'svelte-sonner';

/**
 * Notification store backed by svelte-sonner (Sonner Toaster).
 * Keeps the legacy showToast(type, message) signature so call sites
 * stay untouched; types map to Sonner's own feedback kinds.
 */
export function showToast(type, message) {
  const notify = { success: sonner.success, danger: sonner.error, warning: sonner.warning, info: sonner.info }[type] || sonner.message;
  notify(message);
}
