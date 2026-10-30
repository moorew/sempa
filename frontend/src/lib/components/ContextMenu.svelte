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
  <div class="fixed z-[96] w-52 overflow-hidden p-1.5 animate-scale-in"
       style="left:{clampX(m.x)}px; top:{clampY(m.y, m.items.length)}px; border-radius: 16px;
              background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-2);">
    {#each m.items as item}
      {#if item === 'separator'}
        <div class="mx-2 my-1 h-px" style="background: var(--sempa-border);"></div>
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
    padding: 7px 10px;
    border-radius: 10px;
    font-size: 13px;
    font-weight: 500;
    color: var(--sempa-text);
    transition: background-color 120ms ease;
  }
  .ctx-item:hover { background: var(--sempa-accent-bg); color: var(--sempa-accent-ink); }
  .ctx-item:disabled { opacity: 0.4; cursor: default; }
  .ctx-item:disabled:hover { background: transparent; color: var(--sempa-text); }
  .ctx-item.danger { color: var(--sempa-danger, #c0392b); }
  .ctx-item.danger:hover { background: color-mix(in srgb, var(--sempa-danger, #c0392b) 12%, transparent); color: var(--sempa-danger, #c0392b); }
</style>
