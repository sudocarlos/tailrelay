<script>
  import { MoreHorizontal, SquarePen, SquarePower, Trash2 } from '@lucide/svelte';
  import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/lib/components/ui/dropdown-menu';

  let { autostart = false, onAutostartChange, onEdit, onDelete } = $props();
</script>

<DropdownMenu>
  <DropdownMenuTrigger
    aria-label="More actions"
    class="inline-flex cursor-pointer items-center justify-center rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground outline-none"
    onclick={(e) => e.stopPropagation()}
  >
    <MoreHorizontal size={11} />
  </DropdownMenuTrigger>
  <DropdownMenuContent align="end" class="min-w-52">
    <DropdownMenuItem onclick={() => onEdit?.()}>
      <SquarePen size={14} />
      Edit
    </DropdownMenuItem>

    <DropdownMenuCheckboxItem
      checked={autostart}
      onCheckedChange={(v) => onAutostartChange?.(v)}
      onclick={(e) => e.stopPropagation()}
    >
      <SquarePower size={14} />
      Start on boot
    </DropdownMenuCheckboxItem>

    <DropdownMenuSeparator />

    <DropdownMenuItem variant="destructive" onclick={() => onDelete?.()}>
      <Trash2 size={14} />
      Delete
    </DropdownMenuItem>
  </DropdownMenuContent>
</DropdownMenu>
