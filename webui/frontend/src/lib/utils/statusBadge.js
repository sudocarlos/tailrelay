/**
 * Tailwind classes for the circular badge behind a type icon.
 * The badge communicates relay/funnel state through its background color.
 */
export function statusBadgeClass(isToggling, isRunning) {
  if (isToggling) return 'bg-amber-400 animate-pulse';
  return isRunning ? 'bg-green-700 status-dot-running' : 'bg-gray-100 dark:bg-gray-800';
}

/**
 * Tailwind text-color class for the icon inside the badge.
 * White when the badge has a colored background, accent blue when neutral.
 */
export function statusIconClass(isToggling, isRunning) {
  return isToggling || isRunning ? 'text-white' : 'text-blue-500';
}
