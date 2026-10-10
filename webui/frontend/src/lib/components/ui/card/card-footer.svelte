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
	data-slot="card-footer"
	class={cn("px-(--card-spacing) [.border-t]:pt-(--card-spacing) flex items-center", className)}
	{...restProps}
>
	{@render children?.()}
</div>