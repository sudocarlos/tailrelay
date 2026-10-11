<script>
  import { onMount } from 'svelte';
  import { fetchJSON, postFormData } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import { Button } from '@/lib/components/ui/button';
  import { Input } from '@/lib/components/ui/input';
  import { Skeleton } from '@/lib/components/ui/skeleton';
  import {
    Download,
    Upload,
    Trash2,
    RotateCcw,
    Plus,
    Archive,
    HardDrive,
    Pencil,
    Check,
    RefreshCw,
  } from '@lucide/svelte';

  let backups = $state([]);
  let loading = $state(true);
  let creating = $state(false);
  let uploading = $state(false);
  let restoring = $state('');
  let deleting = $state('');
  let renaming = $state('');
  let renamingFile = $state('');
  let renameInput = $state('');

  onMount(() => {
    loadBackups();
  });

  async function loadBackups() {
    loading = true;
    try {
      backups = await fetchJSON('/api/backup/list');
    } catch (err) {
      showToast('danger', err.message);
      backups = [];
    } finally {
      loading = false;
    }
  }

  async function createBackup(backupType = 'full') {
    creating = true;
    try {
      await fetchJSON('/api/backup/create', {
        method: 'POST',
        body: JSON.stringify({ backup_type: backupType }),
      });
      showToast('success', 'Backup created successfully');
      await loadBackups();
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      creating = false;
    }
  }

  async function restoreBackup(filename) {
    if (!confirm(`Restore from "${filename}"? Services may need to be restarted.`)) return;

    restoring = filename;
    try {
      const result = await fetchJSON('/api/backup/restore', {
        method: 'POST',
        body: JSON.stringify({ filename }),
      });
      showToast('success', result.message || 'Backup restored successfully');
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      restoring = '';
    }
  }

  async function deleteBackup(filename) {
    if (!confirm(`Delete backup "${filename}"?`)) return;

    deleting = filename;
    try {
      await fetchJSON(`/api/backup/delete?filename=${encodeURIComponent(filename)}`, {
        method: 'POST',
      });
      showToast('success', 'Backup deleted');
      await loadBackups();
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      deleting = '';
    }
  }

  function startRename(filename) {
    renamingFile = filename;
    renameInput = filename.endsWith('.tar.gz') ? filename.slice(0, -7) : filename;
  }

  function cancelRename() {
    renamingFile = '';
    renameInput = '';
  }

  async function saveRename() {
    const originalName = renamingFile.endsWith('.tar.gz')
      ? renamingFile.slice(0, -7)
      : renamingFile;
    if (!renameInput.trim() || renameInput.trim() === originalName) {
      cancelRename();
      return;
    }
    const oldFilename = renamingFile;
    renaming = oldFilename;
    try {
      await fetchJSON('/api/backup/rename', {
        method: 'POST',
        body: JSON.stringify({ old_filename: oldFilename, new_filename: renameInput.trim() }),
      });
      showToast('success', 'Backup renamed successfully');
      cancelRename();
      await loadBackups();
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      renaming = '';
    }
  }

  function downloadBackup(filename) {
    window.open(`/api/backup/download?filename=${encodeURIComponent(filename)}`, '_blank');
  }

  async function handleUpload(e) {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.name.endsWith('.tar.gz')) {
      showToast('danger', 'Invalid file type. Please upload a .tar.gz backup file.');
      e.target.value = '';
      return;
    }

    uploading = true;
    try {
      const formData = new FormData();
      formData.append('backup', file);
      await postFormData('/api/backup/upload', formData);
      showToast('success', 'Backup uploaded successfully');
      await loadBackups();
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      uploading = false;
      e.target.value = '';
    }
  }

  function formatSize(bytes) {
    if (bytes < 1024) return `${bytes} B`;
    const units = ['KB', 'MB', 'GB'];
    let i = -1;
    let size = bytes;
    do {
      size /= 1024;
      i++;
    } while (size >= 1024 && i < units.length - 1);
    return `${size.toFixed(1)} ${units[i]}`;
  }

  function formatDate(timestamp) {
    if (!timestamp) return '';
    const d = new Date(timestamp);
    return d.toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }
</script>

<!-- Header -->
<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-6">
  <div>
    <h1 class="text-xl font-semibold">Backups</h1>
    <p class="text-sm text-muted-foreground mt-0.5">
      {backups.length} backup{backups.length === 1 ? '' : 's'}
    </p>
  </div>

  <div class="flex items-center gap-2">
    <!-- Upload -->
    <label
      class="inline-flex items-center gap-1.5 rounded-lg border border-input px-3 py-1.5 text-sm font-medium transition-colors hover:bg-accent hover:text-foreground cursor-pointer {uploading ? 'pointer-events-none opacity-50' : ''}"
    >
      <Upload size={15} />
      {uploading ? 'Uploading...' : 'Upload'}
      <input type="file" accept=".tar.gz" onchange={handleUpload} class="hidden" />
    </label>

    <!-- Create -->
    <Button size="sm" class="gap-1.5" disabled={creating} onclick={() => createBackup('full')}>
      <Plus size={15} />
      {creating ? 'Creating...' : 'Create Backup'}
    </Button>
  </div>
</div>

<!-- Backup list -->
{#if loading}
  <div class="flex justify-center py-16">
    <Skeleton class="h-24 w-2/3" />
  </div>
{:else if backups.length === 0}
  <div class="text-center px-6 py-16">
    <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-muted">
      <Archive size={28} class="text-muted-foreground/60" />
    </div>
    <p class="text-muted-foreground mb-4">No backups yet. Create one to save your configuration.</p>
    <Button class="gap-1.5" disabled={creating} onclick={() => createBackup('full')}>
      <Plus size={15} />
      {creating ? 'Creating...' : 'Create First Backup'}
    </Button>
  </div>
{:else}
  <div class="flex flex-col gap-3">
    {#each backups as backup (backup.filename)}
      <div class="rounded-lg border border-border bg-card px-4 py-3">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
          <!-- Info -->
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <HardDrive size={16} class="text-primary shrink-0" />
              {#if renamingFile === backup.filename}
                <div class="flex min-w-0 flex-1 items-center gap-1.5">
                  <Input
                    type="text"
                    class="h-8 min-w-0 flex-1 font-mono"
                    bind:value={renameInput}
                    onkeydown={(e) => {
                      if (e.key === 'Enter') saveRename();
                      if (e.key === 'Escape') cancelRename();
                    }}
                  />
                  {#if renameInput.trim() && renameInput.trim() !== (renamingFile.endsWith('.tar.gz') ? renamingFile.slice(0, -7) : renamingFile)}
                    <Button
                      size="xs"
                      class="tracking-wider"
                      onclick={saveRename}
                      disabled={renaming === backup.filename}
                      title="Apply"
                    >
                      {#if renaming === backup.filename}
                        <RefreshCw size={12} class="animate-spin" />
                      {:else}
                        <Check size={12} />
                      {/if}
                      Apply
                    </Button>
                  {/if}
                </div>
              {:else}
                <span class="truncate text-sm font-medium">{backup.filename}</span>
              {/if}
            </div>
            <div class="ml-6 mt-1 flex items-center gap-3 text-xs text-muted-foreground">
              <span>{formatSize(backup.size)}</span>
              <span>&middot;</span>
              <span>{formatDate(backup.timestamp)}</span>
              {#if backup.metadata?.backup_type}
                <span>&middot;</span>
                <span class="capitalize">{backup.metadata.backup_type}</span>
              {/if}
            </div>
          </div>

          <!-- Actions -->
          <div class="ml-6 flex items-center gap-2 sm:ml-0">
            <Button
              variant="ghost"
              size="icon-xs"
              class={renamingFile === backup.filename ? 'text-primary' : 'text-muted-foreground'}
              onclick={() => renamingFile === backup.filename ? cancelRename() : startRename(backup.filename)}
              disabled={renaming === backup.filename}
              title={renamingFile === backup.filename ? 'Cancel rename' : 'Rename'}
            >
              <Pencil size={15} />
            </Button>

            <Button
              variant="ghost"
              size="icon-xs"
              class="text-muted-foreground"
              onclick={() => downloadBackup(backup.filename)}
              disabled={renamingFile === backup.filename}
              title="Download"
            >
              <Download size={15} />
            </Button>

            <Button
              variant="ghost"
              size="icon-xs"
              class="text-primary"
              disabled={restoring === backup.filename || renamingFile === backup.filename}
              onclick={() => restoreBackup(backup.filename)}
              title="Restore"
            >
              <RotateCcw size={15} class={restoring === backup.filename ? 'animate-spin' : ''} />
            </Button>

            <Button
              variant="ghost"
              size="icon-xs"
              class="text-destructive"
              disabled={deleting === backup.filename || renamingFile === backup.filename}
              onclick={() => deleteBackup(backup.filename)}
              title="Delete"
            >
              <Trash2 size={15} />
            </Button>
          </div>
        </div>
      </div>
    {/each}
  </div>
{/if}
