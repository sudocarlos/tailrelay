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
	data-slot="popover-header"
	class={cn("flex flex-col gap-1 text-sm", className)}
	{...restProps}
>
	{@render children?.()}
</div>