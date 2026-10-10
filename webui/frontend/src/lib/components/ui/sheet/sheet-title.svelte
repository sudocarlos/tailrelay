<script>
	import { Dialog as SheetPrimitive } from "bits-ui";
	import { cn } from "@/lib/utils/cn.js";

	let {
		ref = $bindable(null),
		class: className,
		...restProps
	} = $props();
</script>

<SheetPrimitive.Title
	bind:ref
	data-slot="sheet-title"
	class={cn("font-heading text-foreground text-lg font-semibold uppercase tracking-wider", className)}
	{...restProps}
/>