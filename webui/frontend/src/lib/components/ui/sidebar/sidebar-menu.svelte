<script>
	import { cn } from "@/lib/utils/cn.js";
	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	} = $props();
</script>

<ul
	bind:this={ref}
	data-slot="sidebar-menu"
	data-sidebar="menu"
	class={cn("gap-0.5 flex w-full min-w-0 flex-col", className)}
	{...restProps}
>
	{@render children?.()}
</ul>