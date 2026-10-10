<script>
	import { Dialog as SheetPrimitive } from "bits-ui";
	import { cn } from "@/lib/utils/cn.js";

	let {
		ref = $bindable(null),
		class: className,
		...restProps
	} = $props();
</script>

<SheetPrimitive.Description
	bind:ref
	data-slot="sheet-description"
	class={cn("text-muted-foreground text-sm leading-relaxed mt-0.5", className)}
	{...restProps}
/>