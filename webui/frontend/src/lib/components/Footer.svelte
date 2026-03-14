<script>
  import { onMount } from 'svelte';

  let version = $state('');

  onMount(async () => {
    try {
      const res = await fetch('/api/info');
      if (res.ok) {
        const data = await res.json();
        version = data.version ?? '';
      }
    } catch {
      // silently ignore — footer version is non-critical
    }
  });
</script>

<footer class="border-t border-gray-200 dark:border-gray-800 py-4 mt-auto">
  <div class="max-w-6xl mx-auto px-4 sm:px-6 flex items-center justify-center gap-3 text-xs text-gray-400 dark:text-gray-500">
    {#if version}
      <span>tailrelay {version}</span>
      <span aria-hidden="true">&middot;</span>
    {/if}
    <a
      href="https://github.com/sudocarlos/tailrelay"
      target="_blank"
      rel="noopener noreferrer"
      class="hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
    >
      Contribute
    </a>
    <span aria-hidden="true">&middot;</span>
    <a
      href="https://github.com/sudocarlos/tailrelay/issues"
      target="_blank"
      rel="noopener noreferrer"
      class="hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
    >
      Feedback
    </a>
  </div>
</footer>
