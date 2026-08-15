<script>
  /**
   * iOS-style toggle switch.
   *
   * Props:
   *   checked   — current state
   *   disabled  — disables interaction and dims the control
   *   onChange  — called with the new boolean value when toggled
   *   label     — accessible label (visually hidden; pair with visible text via aria-label)
   *   size      — "sm" (compact, default) or "md" (original)
   */
  let { checked = false, disabled = false, onChange, label = '', size = 'sm' } = $props();

  const track = $derived(size === 'md' ? 'h-6 w-11' : 'h-4 w-8');
  const knob = $derived(size === 'md' ? 'h-4 w-4' : 'h-3 w-3');
  const knobOn = $derived(size === 'md' ? 'translate-x-6' : 'translate-x-4');
  const knobOff = $derived(size === 'md' ? 'translate-x-1' : 'translate-x-0.5');
</script>

<button
  type="button"
  role="switch"
  aria-checked={checked}
  aria-label={label}
  {disabled}
  onclick={() => onChange?.(!checked)}
  class="relative inline-flex {track} flex-shrink-0 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed {checked
    ? 'bg-blue-600'
    : 'bg-gray-300 dark:bg-gray-600'}"
>
  <span
    class="inline-block {knob} transform rounded-full bg-white shadow transition-transform {checked ? knobOn : knobOff}"
  ></span>
</button>
