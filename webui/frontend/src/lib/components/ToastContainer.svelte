<script>
  import { toasts, dismissToast } from '../stores/toast.js';
  import { CheckCircle, AlertTriangle, Info, X } from '@lucide/svelte';

  let items = $state([]);
  toasts.subscribe((v) => (items = v));

  const iconMap = {
    success: CheckCircle,
    danger: AlertTriangle,
    warning: AlertTriangle,
    info: Info,
  };

  const colorMap = {
    success: 'text-green-500',
    danger: 'text-red-500',
    warning: 'text-amber-500',
    info: 'text-blue-500',
  };

  const bgMap = {
    success: 'border-green-200 dark:border-green-800',
    danger: 'border-red-200 dark:border-red-800',
    warning: 'border-amber-200 dark:border-amber-600',
    info: 'border-blue-200 dark:border-blue-800',
  };
</script>

<div class="fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-2 max-w-sm">
  {#each items as toast (toast.id)}
    {@const IconComponent = iconMap[toast.type] || Info}
    <div class="toast-enter bg-white dark:bg-gray-800 rounded-lg shadow-lg border {bgMap[toast.type] || bgMap.info} px-4 py-3 flex items-start gap-3">
      <IconComponent size={18} class="{colorMap[toast.type] || colorMap.info} flex-shrink-0 mt-0.5" />
      <p class="text-sm flex-1">{toast.message}</p>
      <button
        class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 flex-shrink-0"
        onclick={() => dismissToast(toast.id)}
      >
        <X size={14} />
      </button>
    </div>
  {/each}
</div>
