<script lang="ts">
  import { isTauri } from '$lib/tauri/bridge';
  import { windowChrome, type ControlKind } from '$lib/stores/windowChrome.svelte';
  import { onMount } from 'svelte';

  let win: any = null;
  let isMaximized = $state(false);
  let menu = $state<{ x: number; y: number } | null>(null);

  onMount(async () => {
    if (!isTauri()) return;
    try {
      const { getCurrentWindow } = await import('@tauri-apps/api/window');
      win = getCurrentWindow();
      isMaximized = await win.isMaximized();
      win.onResized(async () => {
        isMaximized = await win.isMaximized();
      });
    } catch { /* Tauri API unavailable */ }
  });

  async function minimize() { await win?.minimize(); }
  async function toggleMax() { await win?.toggleMaximize(); }
  async function close() { await win?.close(); }

  // Right-click the drag strip → a small window menu (the native SSD menu isn't
  // reachable from the webview, so this surfaces the same actions in-brand).
  function openMenu(e: MouseEvent) {
    e.preventDefault();
    menu = { x: e.clientX, y: e.clientY };
  }
  function closeMenu() { menu = null; }
  function runMenu(action: () => void) { action(); closeMenu(); }
</script>

<svelte:window onclick={() => menu && closeMenu()} />

{#if isTauri() && !windowChrome.useSystemTitlebar}
{#snippet control(kind: ControlKind)}
  {#if kind === 'minimize'}
    <button onclick={minimize} title="Minimize" aria-label="Minimize" class="tb-btn">
      <svg width="10" height="1" viewBox="0 0 10 1" fill="currentColor"><rect width="10" height="1"/></svg>
    </button>
  {:else if kind === 'maximize'}
    <button onclick={toggleMax} title={isMaximized ? 'Restore' : 'Maximize'}
            aria-label={isMaximized ? 'Restore' : 'Maximize'} class="tb-btn">
      {#if isMaximized}
        <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1">
          <rect x="0" y="2" width="8" height="8"/><path d="M2 2V0h8v8H8"/>
        </svg>
      {:else}
        <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1">
          <rect x="0" y="0" width="10" height="10"/>
        </svg>
      {/if}
    </button>
  {:else}
    <button onclick={close} title="Close" aria-label="Close" class="tb-btn tb-close">
      <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.2">
        <line x1="0" y1="0" x2="10" y2="10"/><line x1="10" y1="0" x2="0" y2="10"/>
      </svg>
    </button>
  {/if}
{/snippet}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  data-tauri-drag-region
  class="titlebar"
  oncontextmenu={openMenu}
  style="
    height: 38px;
    display: flex;
    align-items: center;
    justify-content: {windowChrome.controlsSide === 'left' ? 'flex-start' : 'flex-end'};
    background: var(--sempa-bg-panel);
    border-bottom: 1px solid var(--sempa-border);
    user-select: none;
    flex-shrink: 0;
  ">
  <!-- Quiet drag strip: brand lives in the sidebar. Controls mirror the system
       layout (side + order from gtk-decoration-layout). -->
  <div class="tb-controls" style="display: flex; align-items: stretch; height: 38px;">
    {#each windowChrome.controlsOrder as kind (kind)}
      {@render control(kind)}
    {/each}
  </div>
</div>

{#if menu}
  <div role="menu" tabindex="-1"
       style="position: fixed; left: {menu.x}px; top: {menu.y}px; z-index: 9999;
              min-width: 160px; padding: 4px; border-radius: 8px;
              background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);
              box-shadow: 0 8px 24px rgba(0,0,0,0.28);">
    {#if isMaximized}
      <button role="menuitem" class="tb-menu-item" onclick={() => runMenu(toggleMax)}>Restore</button>
    {:else}
      <button role="menuitem" class="tb-menu-item" onclick={() => runMenu(toggleMax)}>Maximize</button>
    {/if}
    <button role="menuitem" class="tb-menu-item" onclick={() => runMenu(minimize)}>Minimize</button>
    <div style="height:1px; margin:4px 6px; background: var(--sempa-border);"></div>
    <button role="menuitem" class="tb-menu-item" onclick={() => runMenu(close)}>Close</button>
  </div>
{/if}
{/if}

<style>
  .tb-btn {
    width: 46px; height: 38px; border: none; background: transparent; cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    color: var(--sempa-text-soft); font-size: 12px; line-height: 1;
    transition: background 100ms, color 100ms;
  }
  .tb-btn:hover { background: color-mix(in srgb, var(--sempa-text) 10%, transparent); }
  .tb-close:hover { background: #c42b1c; color: #fff; }
  .tb-btn:active { transform: none !important; } /* override global scale(0.97) */

  .tb-menu-item {
    display: block; width: 100%; text-align: left; padding: 7px 12px;
    font-size: 13px; border: none; background: transparent; cursor: pointer;
    border-radius: 5px; color: var(--sempa-text);
  }
  .tb-menu-item:hover { background: var(--sempa-accent-bg); color: var(--sempa-accent); }
  .tb-menu-item:active { transform: none !important; }
</style>
