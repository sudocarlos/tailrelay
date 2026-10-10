<script>
	import { cn } from "@/lib/utils/cn.js";
	let {
		ref = $bindable(null),
		class: className,
		...restProps
	} = $props();
</script>

<div
	bind:this={ref}
	data-slot="skeleton"
	class={cn("bg-muted animate-pulse", className)}
	{...restProps}
></div>