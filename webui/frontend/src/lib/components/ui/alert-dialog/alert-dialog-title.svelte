<script>
	import { AlertDialog as AlertDialogPrimitive } from "bits-ui";
	import { cn } from "@/lib/utils/cn.js";

	let {
		ref = $bindable(null),
		class: className,
		...restProps
	} = $props();
</script>

<AlertDialogPrimitive.Title
	bind:ref
	data-slot="alert-dialog-title"
	class={cn("font-heading text-lg font-semibold sm:group-data-[size=default]/alert-dialog-content:group-has-data-[slot=alert-dialog-media]/alert-dialog-content:col-start-2 uppercase tracking-wider", className)}
	{...restProps}
/>