<script module>
	import { tv } from "tailwind-variants";

	export const alertVariants = tv({
		base: "grid gap-1 bg-background border px-4 py-3 text-left text-sm has-data-[slot=alert-action]:relative has-data-[slot=alert-action]:pr-18 has-[>svg]:grid-cols-[auto_1fr] has-[>svg]:gap-x-2.5 *:[svg]:row-span-2 *:[svg]:translate-y-0.5 *:[svg]:text-current *:[svg:not([class*='size-'])]:size-4 relative after:-inset-y-px after:-left-px after:w-0.5 after:absolute group/alert relative w-full",
		variants: {
			variant: {
				default: "bg-card text-card-foreground after:bg-foreground",
				destructive: "text-destructive bg-card *:data-[slot=alert-description]:text-destructive/90 *:[svg]:text-current after:bg-destructive",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	});
</script>

<script>
	import { cn } from "@/lib/utils/cn.js";
	let {
		ref = $bindable(null),
		class: className,
		variant = "default",
		children,
		...restProps
	} = $props();
</script>

<div
	bind:this={ref}
	data-slot="alert"
	role="alert"
	class={cn(alertVariants({ variant }), className)}
	{...restProps}
>
	{@render children?.()}
</div>