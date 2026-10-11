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
	class={cn("font-heading text-foreground text-base font-medium", className)}
	{...restProps}
/>