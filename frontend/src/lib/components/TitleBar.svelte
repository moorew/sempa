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

  // Window dragging. `data-tauri-drag-region` is unreliable on WebKitGTK (Linux),
  // so drive it explicitly: primary-button press on the empty strip → startDragging;
  // double-click → maximize/restore. Skip when the press is on a control button.
  function onTitlebarPointerDown(e: MouseEvent) {
    if (e.button !== 0) return;
    if ((e.target as HTMLElement).closest('button')) return;
    if (e.detail >= 2) return; // let the dblclick handler maximize instead
    void win?.startDragging();
  }
  function onTitlebarDblClick(e: MouseEvent) {
    if ((e.target as HTMLElement).closest('button')) return;
    void toggleMax();
  }
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
  class="titlebar"
  onmousedown={onTitlebarPointerDown}
  ondblclick={onTitlebarDblClick}
  oncontextmenu={openMenu}
  style="
    height: 38px;
    display: flex;
    align-items: center;
    justify-content: {windowChrome.controlsSide === 'left' ? 'flex-start' : 'flex-end'};
    background: var(--sempa-bg-nav);
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
              min-width: 168px; padding: 6px; border-radius: 18px;
              background: var(--sempa-bg-panel);
              box-shadow: var(--sempa-shadow-2);">
    {#if isMaximized}
      <button role="menuitem" class="tb-menu-item" onclick={() => runMenu(toggleMax)}>Restore</button>
    {:else}
      <button role="menuitem" class="tb-menu-item" onclick={() => runMenu(toggleMax)}>Maximize</button>
    {/if}
    <button role="menuitem" class="tb-menu-item" onclick={() => runMenu(minimize)}>Minimize</button>
    <div style="height:1px; margin:4px 10px; background: var(--sempa-border);"></div>
    <button role="menuitem" class="tb-menu-item" onclick={() => runMenu(close)}>Close</button>
  </div>
{/if}
{/if}

<style>
  /* The full 46×38 strip cell stays the hit target (edge-of-screen Fitts when
     maximised); the visible hover is a round pill inside it (1.27 controls). */
  .tb-btn {
    position: relative;
    width: 46px; height: 38px; border: none; background: transparent; cursor: pointer;
    display: flex; align-items: center; justify-content: center;
    color: var(--sempa-text-soft); font-size: 12px; line-height: 1;
    transition: color 100ms;
  }
  .tb-btn::before {
    content: ''; position: absolute; left: 50%; top: 50%;
    width: 28px; height: 28px; margin: -14px 0 0 -14px; border-radius: 999px;
    background: transparent; transition: background 100ms;
  }
  .tb-btn > :global(svg) { position: relative; }
  .tb-btn:hover { color: var(--sempa-text); }
  .tb-btn:hover::before { background: color-mix(in srgb, var(--sempa-text) 10%, transparent); }
  .tb-close:hover { color: #fff; }
  .tb-close:hover::before { background: #c42b1c; }
  .tb-btn:active { transform: none !important; } /* override global scale(0.97) */

  .tb-menu-item {
    display: block; width: 100%; text-align: left; padding: 8px 12px;
    font-size: 13px; font-weight: 500; border: none; background: transparent; cursor: pointer;
    border-radius: 12px; color: var(--sempa-text);
  }
  .tb-menu-item:hover { background: var(--sempa-accent-bg); color: var(--sempa-accent-ink); }
  .tb-menu-item:active { transform: none !important; }
</style>
