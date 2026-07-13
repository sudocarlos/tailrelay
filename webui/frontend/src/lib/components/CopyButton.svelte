<script>
  import { Copy, Check } from '@lucide/svelte';
  import { showToast } from '../stores/toast.js';

  let { text, label = 'Copy address', size = 13 } = $props();

  let copied = $state(false);
  let timer = null;

  async function handleCopy() {
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
      showToast('success', 'Copied to clipboard');
      clearTimeout(timer);
      timer = setTimeout(() => { copied = false; }, 1500);
    } catch {
      showToast('danger', 'Failed to copy');
    }
  }
</script>

<button
  class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 flex-shrink-0 transition-colors"
  onclick={handleCopy}
  title={label}
>
  {#if copied}
    <Check size={size} class="text-green-600" />
  {:else}
    <Copy size={size} />
  {/if}
</button>
