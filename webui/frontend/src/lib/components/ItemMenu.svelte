<script>
  import { MoreHorizontal, SquarePen, SquarePower, Trash2 } from '@lucide/svelte';
  import { DropdownMenu } from '@/lib/components/ui/dropdown-menu';
  import { DropdownMenuTrigger } from '@/lib/components/ui/dropdown-menu';
  import { DropdownMenuContent } from '@/lib/components/ui/dropdown-menu';
  import { DropdownMenuItem } from '@/lib/components/ui/dropdown-menu';
  import { DropdownMenuSeparator } from '@/lib/components/ui/dropdown-menu';
  import { Switch } from '@/lib/components/ui/switch';

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

    <div
      class="flex items-center justify-between gap-3 px-3 py-2 text-xs font-medium uppercase tracking-wider"
      onclick={(e) => e.stopPropagation()}
    >
      <span class="flex items-center gap-2"><SquarePower size={14} />Start on boot</span>
      <Switch
        checked={autostart}
        onCheckedChange={onAutostartChange}
        aria-label="Start on boot"
        size="sm"
        onclick={(e) => e.stopPropagation()}
      />
    </div>

    <DropdownMenuSeparator />

    <DropdownMenuItem variant="destructive" onclick={() => onDelete?.()}>
      <Trash2 size={14} />
      Delete
    </DropdownMenuItem>
  </DropdownMenuContent>
</DropdownMenu>
