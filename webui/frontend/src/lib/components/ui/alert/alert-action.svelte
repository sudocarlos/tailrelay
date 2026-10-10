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
	data-slot="alert-action"
	class={cn("absolute top-2.5 right-3", className)}
	{...restProps}
>
	{@render children?.()}
</div>