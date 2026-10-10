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
	data-slot="select-label"
	class={cn("px-3 py-2 text-xs uppercase tracking-wider font-semibold text-muted-foreground", className)}
	{...restProps}
>
	{@render children?.()}
</div>