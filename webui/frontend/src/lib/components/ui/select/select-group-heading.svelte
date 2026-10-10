<script>
	import { Select as SelectPrimitive } from "bits-ui";
	import { cn } from "@/lib/utils/cn.js";
	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	} = $props();
</script>

<SelectPrimitive.GroupHeading
	bind:ref
	data-slot="select-group-heading"
	class={cn("px-3 py-2 text-xs uppercase tracking-wider font-semibold text-muted-foreground", className)}
	{...restProps}
>
	{@render children?.()}
</SelectPrimitive.GroupHeading>