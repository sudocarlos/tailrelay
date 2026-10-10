<script>
	import { cn } from "@/lib/utils/cn.js";
	let {
		ref = $bindable(null),
		class: className,
		inset,
		children,
		...restProps
	} = $props();
</script>

<div
	bind:this={ref}
	data-slot="dropdown-menu-label"
	data-inset={inset}
	class={cn("px-3 py-2 text-xs uppercase tracking-wider data-inset:pl-9.5 font-semibold text-muted-foreground", className)}
	{...restProps}
>
	{@render children?.()}
</div>