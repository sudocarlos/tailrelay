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
	data-slot="sidebar-group"
	data-sidebar="group"
	class={cn("p-2 relative flex w-full min-w-0 flex-col", className)}
	{...restProps}
>
	{@render children?.()}
</div>