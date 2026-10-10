<script>
	import { DropdownMenu as DropdownMenuPrimitive } from "bits-ui";
	import { cn } from "@/lib/utils/cn.js";
	let {
		ref = $bindable(null),
		class: className,
		inset,
		...restProps
	} = $props();
</script>

<DropdownMenuPrimitive.GroupHeading
	bind:ref
	data-slot="dropdown-menu-group-heading"
	data-inset={inset}
	class={cn("px-3 py-2 text-xs uppercase tracking-wider data-inset:pl-9.5 font-semibold text-muted-foreground", className)}
	{...restProps}
/>