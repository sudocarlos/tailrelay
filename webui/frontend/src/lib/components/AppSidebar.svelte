<script>
  import { Network, Globe, Archive, Sun, Moon, Monitor, RefreshCw, LogOut, KeyRound } from '@lucide/svelte';
  import * as Sidebar from '@/lib/components/ui/sidebar';
  import { Separator } from '@/lib/components/ui/separator';
  import * as DropdownMenu from '@/lib/components/ui/dropdown-menu';
  import { currentView, refreshData, logout, tailscaleConnected, tailscaleSection, tailscaleActiveSection } from '../stores/app.js';
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

  function switchToSection(section) {
    // Disconnected tailscale only renders the Connection section.
    if (!tsConnected && section !== 'connection') return;
    currentView.set('tailscale');
    // { id, n } nonce so re-clicking the same section scrolls again.
    tailscaleSection.set({ id: section, n: $tailscaleSection?.n ? $tailscaleSection.n + 1 : 1 });
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

<!-- Off-canvas collapsible: with no desktop trigger the sidebar stays
     fixed open on desktop, while mobile still gets the off-canvas sheet
     (opened from the mobile-only header trigger in App.svelte). -->
<Sidebar.Root>
  <Sidebar.Header>
    <div class="flex items-center gap-2 px-2 py-1.5">
      <img src={$brandIconSrc} alt="Tailrelay" class="h-7 w-7 shrink-0 rounded-lg object-contain" width="28" height="28" />
      <span class="text-lg font-semibold tracking-tight">Tailrelay</span>
    </div>
  </Sidebar.Header>

  <Separator class="bg-sidebar-border" />

  <Sidebar.Content>
    <Sidebar.Group>
      <Sidebar.GroupContent>
        <Sidebar.Menu>
          <Sidebar.MenuItem>
            <Sidebar.MenuButton
              isActive={currentViewValue === 'dashboard'}
              disabled={!tsConnected}
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
              onclick={() => switchView('tailscale')}
            >
              <Globe />
              <span>Tailscale</span>
              {#if !tsConnected}
                <span class="ml-auto shrink-0 rounded-full bg-amber-500" style="width:6px;height:6px" title="Tailscale not connected"></span>
              {/if}
            </Sidebar.MenuButton>
            {#if currentViewValue === 'tailscale'}
              <Sidebar.MenuSub>
                <Sidebar.MenuSubItem>
                  <Sidebar.MenuSubButton
                    isActive={$tailscaleActiveSection === 'connection'}
                    onclick={() => switchToSection('connection')}
                  >
                    Connection
                  </Sidebar.MenuSubButton>
                </Sidebar.MenuSubItem>
                {#if tsConnected}
                  <Sidebar.MenuSubItem>
                    <Sidebar.MenuSubButton
                      isActive={$tailscaleActiveSection === 'networking'}
                      onclick={() => switchToSection('networking')}
                    >
                      Networking
                    </Sidebar.MenuSubButton>
                  </Sidebar.MenuSubItem>
                {/if}
                <Sidebar.MenuSubItem>
                  <Sidebar.MenuSubButton
                    isActive={$tailscaleActiveSection === 'machines'}
                    onclick={() => switchToSection('machines')}
                  >
                    Machines
                  </Sidebar.MenuSubButton>
                </Sidebar.MenuSubItem>
              </Sidebar.MenuSub>
            {/if}
          </Sidebar.MenuItem>
          <Sidebar.MenuItem>
            <Sidebar.MenuButton
              isActive={currentViewValue === 'backups'}
              disabled={!tsConnected}
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
        <Sidebar.MenuButton onclick={handleRefresh}>
          <RefreshCw />
          <span>Refresh</span>
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
      <Sidebar.MenuItem>
        <DropdownMenu.Root>
          <DropdownMenu.Trigger>
            {#snippet child({ props })}
              <Sidebar.MenuButton {...props}>
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
        <Sidebar.MenuButton onclick={() => (showChangePassword = true)}>
          <KeyRound />
          <span>Change password</span>
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
      <Sidebar.MenuItem>
        <Sidebar.MenuButton onclick={logout}>
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
