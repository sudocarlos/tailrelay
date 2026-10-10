<script>
	import { cn } from "@/lib/utils/cn.js";
	let {
		ref = $bindable(null),
		class: className,
		children,
		size = "default",
		...restProps
	} = $props();
</script>

<div
	bind:this={ref}
	data-slot="card"
	data-size={size}
	class={cn("bg-card text-card-foreground ring-foreground/5 gap-(--card-spacing) overflow-hidden py-(--card-spacing) text-sm shadow-sm ring-1 [--card-spacing:--spacing(8)] has-[>img:first-child]:pt-0 data-[size=sm]:[--card-spacing:--spacing(5)] *:[img:first-child]:rounded-none *:[img:last-child]:rounded-none group/card flex flex-col", className)}
	{...restProps}
>
	{@render children?.()}
</div>