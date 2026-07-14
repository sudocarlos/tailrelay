<script>
  import { onDestroy } from 'svelte';
  import { Copy } from '@lucide/svelte';
  import { showToast } from '../stores/toast.js';

  let { text, label = 'Copy address', size = 13, class: className = '' } = $props();

  let copied = $state(false);
  let timer = null;

  onDestroy(() => clearTimeout(timer));

  async function handleCopy(e) {
    e.stopPropagation();
    if (copied) return;

    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      } else {
        // Fallback for non-secure contexts (plain HTTP)
        const el = document.createElement('textarea');
        el.value = text;
        el.style.position = 'fixed';
        el.style.opacity = '0';
        document.body.appendChild(el);
        el.select();
        document.execCommand('copy');
        document.body.removeChild(el);
      }
      copied = true;
      clearTimeout(timer);
      timer = setTimeout(() => { copied = false; }, 1500);
    } catch {
      showToast('danger', 'Failed to copy');
    }
  }
</script>

<button
  class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 flex-shrink-0 transition-colors {className}"
  onclick={handleCopy}
  title={copied ? 'Copied' : label}
>
  {#if copied}
    <span class="text-green-600 font-medium" style="font-size: {size}px">Copied</span>
  {:else}
    <Copy size={size} />
  {/if}
</button>
