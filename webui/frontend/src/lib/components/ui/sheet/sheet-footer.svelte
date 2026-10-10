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
	data-slot="sheet-footer"
	class={cn("gap-2 p-8 mt-auto flex flex-col", className)}
	{...restProps}
>
	{@render children?.()}
</div>