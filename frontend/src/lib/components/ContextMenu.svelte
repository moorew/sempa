<script lang="ts">
  /**
   * The single app-wide right-click menu surface, mounted once in the root
   * layout. Reads the `contextMenu` store and renders the active menu, clamped
   * to the viewport. Styling matches the calendar's quick-action menu.
   */
  import { contextMenu } from '$lib/stores/contextMenu.svelte';

  const m = $derived(contextMenu.current);

  function clampX(x: number, w = 200): number {
    const max = (typeof window !== 'undefined' ? window.innerWidth : 9999) - w - 8;
    return Math.max(8, Math.min(x, max));
  }
  function clampY(y: number, rows: number): number {
    const h = rows * 34 + 10;
    const max = (typeof window !== 'undefined' ? window.innerHeight : 9999) - h - 8;
    return Math.max(8, Math.min(y, max));
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') contextMenu.close();
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if m}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[95]"
       onclick={() => contextMenu.close()}
       oncontextmenu={(e) => { e.preventDefault(); contextMenu.close(); }}></div>
  <div class="fixed z-[96] w-52 overflow-hidden rounded-lg py-1 shadow-2xl animate-scale-in"
       style="left:{clampX(m.x)}px; top:{clampY(m.y, m.items.length)}px;
              background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">
    {#each m.items as item}
      {#if item === 'separator'}
        <div class="my-1 h-px" style="background: var(--sempa-border);"></div>
      {:else}
        <button class="ctx-item" class:danger={item.danger} disabled={item.disabled}
                onclick={() => { item.onClick(); contextMenu.close(); }}>
          {item.label}
        </button>
      {/if}
    {/each}
  </div>
{/if}

<style>
  .ctx-item {
    display: block;
    width: 100%;
    text-align: left;
    padding: 6px 12px;
    font-size: 12.5px;
    color: var(--sempa-text-soft);
    transition: background-color 120ms ease;
  }
  .ctx-item:hover { background: var(--sempa-accent-bg); color: var(--sempa-accent); }
  .ctx-item:disabled { opacity: 0.4; cursor: default; }
  .ctx-item:disabled:hover { background: transparent; color: var(--sempa-text-soft); }
  .ctx-item.danger:hover { background: color-mix(in srgb, #ef4444 14%, transparent); color: #ef4444; }
</style>
