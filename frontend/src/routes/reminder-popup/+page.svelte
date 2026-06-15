<script lang="ts">
  /**
   * Reminder popup — the contents of the chromeless, always-on-top Tauri window
   * spawned top-right when a task reminder fires (see src-tauri/src/windows.rs
   * create_reminder_popup). It's a Granola-style stack of cards that floats over
   * the desktop, OUTSIDE the main app window, and stays until dismissed.
   *
   * Data flow (the main window owns the truth — $lib/desktopReminderPopup):
   *   • on mount we emit `reminder:ready` and listen for `reminder:list`
   *   • each user action emits `reminder:action` { action, taskId } back to main
   * The window resizes itself to fit the cards (top-right stays anchored since
   * only the height changes).
   */
  import { onMount } from 'svelte';
  import { Bell, Check, X } from 'lucide-svelte';

  interface Card {
    taskId: string;
    title: string;
    subtitle: string;
  }

  let cards = $state<Card[]>([]);
  let rootEl: HTMLElement | undefined = $state();

  // — Tauri bridges (loaded lazily; this route only ever runs in the desktop shell) —
  let emitFn: ((event: string, payload?: unknown) => Promise<void>) | null = null;

  async function act(action: 'open' | 'done' | 'snooze' | 'dismiss', taskId: string) {
    // Optimistically drop the card so the UI feels instant; the main window is
    // the source of truth and will re-emit the authoritative list anyway.
    if (action !== 'open') cards = cards.filter((c) => c.taskId !== taskId);
    await emitFn?.('reminder:action', { action, taskId });
  }

  // Resize the window to EXACTLY the painted panel — no slack. The window is
  // opaque (transparent:false in Rust), so the panel paints every pixel; sizing
  // it to the content means there's no leftover window area at all. (Previously
  // the window was transparent and slightly larger than its content, so Windows'
  // grey window backing showed through as a box around the cards.)
  async function fitWindow() {
    if (typeof window === 'undefined') return;
    try {
      const { getCurrentWebviewWindow } = await import('@tauri-apps/api/webviewWindow');
      const { LogicalSize } = await import('@tauri-apps/api/dpi');
      const h = Math.max(48, Math.ceil(rootEl?.scrollHeight ?? 120));
      await getCurrentWebviewWindow().setSize(new LogicalSize(384, h));
    } catch {
      /* not in Tauri / API unavailable */
    }
  }

  // Re-fit whenever the cards change (after the DOM updates).
  $effect(() => {
    cards.length; // track
    requestAnimationFrame(fitWindow);
  });

  onMount(() => {
    let unlisten: (() => void) | null = null;
    (async () => {
      try {
        const { listen, emit } = await import('@tauri-apps/api/event');
        emitFn = emit;
        unlisten = await listen<Card[]>('reminder:list', (e) => {
          cards = Array.isArray(e.payload) ? e.payload : [];
        });
        // Tell the main window we're ready so it (re)sends the current list —
        // avoids the race where the first emit fires before this window mounts.
        await emit('reminder:ready');
      } catch {
        /* not in Tauri */
      }
    })();
    return () => unlisten?.();
  });
</script>

<!--
  Opaque window (transparent:false in Rust). The panel paints EVERY pixel of the
  window edge-to-edge, so there is no transparent region where Windows' grey
  window backing could show through. Multiple reminders are rows inside this one
  panel, separated by hairline dividers — NOT separate floating cards with gaps
  (those gaps were the grey box the user saw).
-->
<div class="popup-root" bind:this={rootEl}>
  {#each cards as c (c.taskId)}
    <div class="card">
      <!-- Body = click to open the task -->
      <button class="open" onclick={() => act('open', c.taskId)} title="Open task">
        <span class="bar"></span>
        <span class="icon"><Bell size={15} strokeWidth={2} /></span>
        <span class="text">
          <span class="label">Reminder</span>
          <span class="title">{c.title}</span>
          {#if c.subtitle}<span class="sub">{c.subtitle}</span>{/if}
        </span>
      </button>

      <div class="actions">
        <button class="act-btn" onclick={() => act('done', c.taskId)} title="Mark done" aria-label="Mark done">
          <Check size={15} strokeWidth={2.25} />
        </button>
        <button class="act-btn" onclick={() => act('dismiss', c.taskId)} title="Dismiss" aria-label="Dismiss">
          <X size={15} strokeWidth={2.25} />
        </button>
      </div>
    </div>
  {/each}
</div>

<style>
  /* Opaque dark background painted immediately so there's never a white/grey
     flash before the panel renders, and never a bare window region. */
  :global(html), :global(body) {
    background: #1c1612 !important;
    margin: 0;
    overflow: hidden;
  }

  /* The panel IS the window: full width, hugs its rows in height (the window is
     resized to match), one solid fill. No padding/margin/radius — Win11's DWM
     rounds the window corners for us, and painting every pixel means no grey. */
  .popup-root {
    width: 100vw;
    background: #1c1612;
    overflow: hidden;
    font-family: 'Plus Jakarta Sans', system-ui, sans-serif;
  }

  /* Each reminder is a row on the shared panel, divided by a hairline — no
     per-row background, radius, shadow or gap (all of which previously left
     transparent seams the grey backing bled through). */
  .card {
    display: flex;
    align-items: stretch;
  }
  .card:not(:last-child) {
    border-bottom: 1px solid rgba(255, 255, 255, 0.07);
  }

  .open {
    display: flex;
    align-items: center;
    gap: 10px;
    flex: 1 1 auto;
    min-width: 0;
    padding: 11px 4px 11px 0;
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    transition: background 120ms ease;
  }
  .open:hover { background: rgba(255, 255, 255, 0.04); }

  .bar {
    width: 4px;
    align-self: stretch;
    margin-right: 8px;
    background: #cc6e3a; /* terracotta accent */
    flex: 0 0 auto;
  }

  .icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    border-radius: 8px;
    background: rgba(204, 110, 58, 0.16);
    color: #e08a54;
    flex: 0 0 auto;
  }

  .text { display: flex; flex-direction: column; min-width: 0; line-height: 1.25; }
  .label {
    font-size: 9.5px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: #e08a54;
  }
  .title {
    font-size: 13.5px;
    font-weight: 600;
    color: #f4efe9;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .sub { font-size: 11px; color: rgba(244, 239, 233, 0.55); }

  .actions {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 0 8px 0 4px;
    flex: 0 0 auto;
  }
  .act-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: 8px;
    border: none;
    background: none;
    color: rgba(244, 239, 233, 0.55);
    cursor: pointer;
    transition: background 120ms ease, color 120ms ease;
  }
  .act-btn:hover { background: rgba(255, 255, 255, 0.08); color: #f4efe9; }
</style>
