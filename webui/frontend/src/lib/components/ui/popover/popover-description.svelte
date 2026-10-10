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
	data-slot="popover-description"
	class={cn("text-muted-foreground text-sm leading-relaxed mt-0.5", className)}
	{...restProps}
>
	{@render children?.()}
</div>