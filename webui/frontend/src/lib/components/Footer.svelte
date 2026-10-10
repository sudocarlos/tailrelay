<script>
  import { onMount } from 'svelte';

  let version = $state('');
  let commit = $state('');

  onMount(async () => {
    try {
      const res = await fetch('/api/info');
      if (res.ok) {
        const data = await res.json();
        version = data.version ?? '';
        commit = data.commit ?? '';
      }
    } catch {
      // silently ignore — footer version is non-critical
    }
  });
</script>

<footer class="mt-auto border-t border-border py-4">
  <div class="mx-auto flex max-w-6xl items-center justify-center gap-3 px-4 text-xs text-muted-foreground sm:px-6">
    {#if version}
      <span>tailrelay {version}{#if commit && commit !== 'none'} ({commit}){/if}</span>
      <span aria-hidden="true">&middot;</span>
    {/if}
    <a
      href="https://github.com/sudocarlos/tailrelay"
      target="_blank"
      rel="noopener noreferrer"
      class="transition-colors hover:text-foreground"
    >
      Contribute
    </a>
    <span aria-hidden="true">&middot;</span>
    <a
      href="https://github.com/sudocarlos/tailrelay/issues"
      target="_blank"
      rel="noopener noreferrer"
      class="transition-colors hover:text-foreground"
    >
      Feedback
    </a>
  </div>
</footer>
