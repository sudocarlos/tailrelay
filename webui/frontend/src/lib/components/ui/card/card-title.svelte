<script>
	import { cn } from "@/lib/utils/cn.js";
	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	} = $props();
</script>

<div
	bind:this={ref}
	data-slot="card-title"
	class={cn("font-heading text-lg font-semibold tracking-wider uppercase", className)}
	{...restProps}
>
	{@render children?.()}
</div>