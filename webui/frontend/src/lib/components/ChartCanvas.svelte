<script>
  import { onMount, onDestroy } from 'svelte';
  import { Chart } from 'chart.js/auto';

  /** @type {{ type: string, data: object, options?: object }} */
  let { type, data, options = {} } = $props();

  let canvas;
  let chart;

  onMount(() => {
    chart = new Chart(canvas, { type, data, options });
  });

  onDestroy(() => {
    chart?.destroy();
  });

  $effect(() => {
    if (!chart) return;
    chart.data = data;
    chart.options = options;
    chart.update('none');
  });
</script>

<canvas bind:this={canvas}></canvas>
