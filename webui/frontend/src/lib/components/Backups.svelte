<script>
  import { onMount } from 'svelte';
  import { fetchJSON, postFormData } from '../api.js';
  import { showToast } from '../stores/toast.js';
  import {
    Download,
    Upload,
    Trash2,
    RotateCcw,
    Plus,
    Archive,
    HardDrive,
  } from '@lucide/svelte';

  let backups = $state([]);
  let loading = $state(true);
  let creating = $state(false);
  let uploading = $state(false);
  let restoring = $state('');
  let deleting = $state('');

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
    <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
      {backups.length} backup{backups.length === 1 ? '' : 's'}
    </p>
  </div>

  <div class="flex items-center gap-2">
    <!-- Upload -->
    <label
      class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium border border-gray-300 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors cursor-pointer {uploading ? 'opacity-50 pointer-events-none' : ''}"
    >
      <Upload size={15} />
      {uploading ? 'Uploading...' : 'Upload'}
      <input type="file" accept=".tar.gz" onchange={handleUpload} class="hidden" />
    </label>

    <!-- Create -->
    <button
      class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-white bg-blue-500 hover:bg-blue-600 rounded-lg transition-colors disabled:opacity-50"
      disabled={creating}
      onclick={() => createBackup('full')}
    >
      <Plus size={15} />
      {creating ? 'Creating...' : 'Create Backup'}
    </button>
  </div>
</div>

<!-- Backup list -->
{#if loading}
  <div class="flex justify-center py-16">
    <div class="animate-spin rounded-full h-8 w-8 border-2 border-blue-500 border-t-transparent"></div>
  </div>
{:else if backups.length === 0}
  <div class="text-center py-16 px-6">
    <div class="mx-auto w-16 h-16 flex items-center justify-center rounded-full bg-gray-100 dark:bg-gray-800 mb-4">
      <Archive size={28} class="text-gray-400 dark:text-gray-500" />
    </div>
    <p class="text-gray-500 dark:text-gray-400 mb-4">No backups yet. Create one to save your configuration.</p>
    <button
      class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-white bg-blue-500 hover:bg-blue-600 rounded-lg transition-colors disabled:opacity-50"
      disabled={creating}
      onclick={() => createBackup('full')}
    >
      <Plus size={15} />
      {creating ? 'Creating...' : 'Create First Backup'}
    </button>
  </div>
{:else}
  <div class="flex flex-col gap-3">
    {#each backups as backup (backup.filename)}
      <div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 px-4 py-3">
        <div class="flex flex-col sm:flex-row sm:items-center gap-3">
          <!-- Info -->
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <HardDrive size={16} class="text-blue-500 flex-shrink-0" />
              <span class="font-medium text-sm truncate">{backup.filename}</span>
            </div>
            <div class="flex items-center gap-3 mt-1 ml-6 text-xs text-gray-500 dark:text-gray-400">
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
          <div class="flex items-center gap-2 ml-6 sm:ml-0">
            <button
              class="p-1.5 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500 transition-colors"
              onclick={() => downloadBackup(backup.filename)}
              title="Download"
            >
              <Download size={15} />
            </button>

            <button
              class="p-1.5 rounded-md hover:bg-blue-50 dark:hover:bg-blue-900/20 text-blue-500 transition-colors disabled:opacity-50"
              disabled={restoring === backup.filename}
              onclick={() => restoreBackup(backup.filename)}
              title="Restore"
            >
              <RotateCcw size={15} class={restoring === backup.filename ? 'animate-spin' : ''} />
            </button>

            <button
              class="p-1.5 rounded-md hover:bg-red-50 dark:hover:bg-red-900/20 text-red-500 transition-colors disabled:opacity-50"
              disabled={deleting === backup.filename}
              onclick={() => deleteBackup(backup.filename)}
              title="Delete"
            >
              <Trash2 size={15} />
            </button>
          </div>
        </div>
      </div>
    {/each}
  </div>
{/if}
