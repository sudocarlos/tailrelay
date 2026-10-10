<script>
  import { Network, Globe, Archive, Sun, Moon, Monitor, RefreshCw, LogOut, KeyRound } from '@lucide/svelte';
  import * as Sidebar from '@/lib/components/ui/sidebar';
  import * as DropdownMenu from '@/lib/components/ui/dropdown-menu';
  import { currentView, refreshData, logout, tailscaleConnected } from '../stores/app.js';
  import { theme, resolvedTheme, setTheme, brandIconSrc } from '../stores/theme.js';
  import { showToast } from '../stores/toast.js';
  import ChangePasswordModal from './ChangePasswordModal.svelte';

  let refreshing = $state(false);
  let showChangePassword = $state(false);
  let tsConnected = $state(true);
  let currentViewValue = $state('dashboard');

  tailscaleConnected.subscribe((v) => (tsConnected = v));
  currentView.subscribe((v) => (currentViewValue = v));

  function switchView(view) {
    // Block navigation to non-Tailscale views when Tailscale is disconnected.
    if (!tsConnected && view !== 'tailscale') return;
    currentView.set(view);
  }

  async function handleRefresh() {
    refreshing = true;
    try {
      await refreshData();
      showToast('success', 'Data refreshed');
    } catch (err) {
      showToast('danger', err.message);
    } finally {
      refreshing = false;
    }
  }

  const themeOptions = [
    { value: 'system', label: 'System', Icon: Monitor },
    { value: 'light', label: 'Light', Icon: Sun },
    { value: 'dark', label: 'Dark', Icon: Moon },
  ];
</script>

<Sidebar.Root>
  <Sidebar.Header>
    <div class="flex items-center gap-2 px-2 py-1.5">
      <img src={$brandIconSrc} alt="Tailrelay" class="h-7 w-7 shrink-0 rounded-lg object-contain" width="28" height="28" />
      <span class="text-lg font-semibold tracking-tight">Tail<span class="brand-relay">relay</span></span>
    </div>
  </Sidebar.Header>

  <Sidebar.Content>
    <Sidebar.Group>
      <Sidebar.GroupContent>
        <Sidebar.Menu>
          <Sidebar.MenuItem>
            <Sidebar.MenuButton
              isActive={currentViewValue === 'dashboard'}
              disabled={!tsConnected}
              tooltipContent="Relays"
              onclick={() => switchView('dashboard')}
            >
              <Network />
              <span>Relays</span>
              {#if !tsConnected}
                <span class="ml-auto shrink-0 rounded-full bg-amber-500" style="width:6px;height:6px" title="Tailscale not connected"></span>
              {/if}
            </Sidebar.MenuButton>
          </Sidebar.MenuItem>
          <Sidebar.MenuItem>
            <Sidebar.MenuButton
              isActive={currentViewValue === 'tailscale'}
              tooltipContent="Tailscale"
              onclick={() => switchView('tailscale')}
            >
              <Globe />
              <span>Tailscale</span>
              {#if !tsConnected}
                <span class="ml-auto shrink-0 rounded-full bg-amber-500" style="width:6px;height:6px" title="Tailscale not connected"></span>
              {/if}
            </Sidebar.MenuButton>
          </Sidebar.MenuItem>
          <Sidebar.MenuItem>
            <Sidebar.MenuButton
              isActive={currentViewValue === 'backups'}
              disabled={!tsConnected}
              tooltipContent="Backups"
              onclick={() => switchView('backups')}
            >
              <Archive />
              <span>Backups</span>
              {#if !tsConnected}
                <span class="ml-auto shrink-0 rounded-full bg-amber-500" style="width:6px;height:6px" title="Tailscale not connected"></span>
              {/if}
            </Sidebar.MenuButton>
          </Sidebar.MenuItem>
        </Sidebar.Menu>
      </Sidebar.GroupContent>
    </Sidebar.Group>
  </Sidebar.Content>

  <Sidebar.Footer>
    <Sidebar.Menu>
      <Sidebar.MenuItem>
        <Sidebar.MenuButton tooltipContent="Refresh (r)" onclick={handleRefresh}>
          <RefreshCw />
          <span>Refresh</span>
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
      <Sidebar.MenuItem>
        <DropdownMenu.Root>
          <DropdownMenu.Trigger>
            {#snippet child({ props })}
              <Sidebar.MenuButton {...props} tooltipContent="Theme">
                {#if $resolvedTheme === 'dark'}
                  <Moon />
                {:else}
                  <Sun />
                {/if}
                <span>Theme</span>
              </Sidebar.MenuButton>
            {/snippet}
          </DropdownMenu.Trigger>
          <DropdownMenu.Content side="top" align="start" class="min-w-32">
            <DropdownMenu.RadioGroup value={$theme} onValueChange={setTheme}>
              {#each themeOptions as { value, label, Icon } (value)}
                <DropdownMenu.RadioItem value={value}>
                  <Icon />
                  {label}
                </DropdownMenu.RadioItem>
              {/each}
            </DropdownMenu.RadioGroup>
          </DropdownMenu.Content>
        </DropdownMenu.Root>
      </Sidebar.MenuItem>
      <Sidebar.MenuItem>
        <Sidebar.MenuButton tooltipContent="Change password" onclick={() => (showChangePassword = true)}>
          <KeyRound />
          <span>Change password</span>
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
      <Sidebar.MenuItem>
        <Sidebar.MenuButton tooltipContent="Logout" onclick={logout}>
          <LogOut />
          <span>Logout</span>
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
    </Sidebar.Menu>
  </Sidebar.Footer>

  {#if showChangePassword}
    <ChangePasswordModal onClose={() => (showChangePassword = false)} />
  {/if}
</Sidebar.Root>
