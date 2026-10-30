<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/state';
  import { isTauri } from '$lib/tauri/bridge';
  import { X, Pin } from 'lucide-svelte';

  let noteId = $derived(page.url.searchParams.get('id') ?? 'default');
  let content = $state('');
  let pinned = $state(true);
  let saveTimer: ReturnType<typeof setTimeout> | null = null;

  onMount(() => {
    // Load saved content from localStorage (keyed by note ID)
    const saved = localStorage.getItem(`sempa-sticky-${noteId}`);
    if (saved) content = saved;
  });

  onDestroy(() => {
    if (saveTimer) clearTimeout(saveTimer);
  });

  function handleInput(e: Event) {
    content = (e.target as HTMLTextAreaElement).value;
    // Debounced auto-save
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      localStorage.setItem(`sempa-sticky-${noteId}`, content);
    }, 500);
  }

  async function closeNote() {
    localStorage.setItem(`sempa-sticky-${noteId}`, content);
    if (isTauri()) {
      const { closeStickyNote } = await import('$lib/tauri/bridge');
      await closeStickyNote(noteId);
    }
  }

  async function togglePin() {
    pinned = !pinned;
    if (isTauri() && '__TAURI__' in window) {
      const win = (window as any).__TAURI__.window.getCurrentWindow();
      await win.setAlwaysOnTop(pinned);
    }
  }
</script>

<svelte:head>
  <style>
    /* Opaque window (transparent:false in Rust): paint the note colour behind
       everything so no window backing can show at the edges. */
    html, body {
      background: var(--sempa-accent-bg) !important;
      overflow: hidden;
      margin: 0;
      padding: 0;
    }
  </style>
</svelte:head>

<div class="sticky-note" data-tauri-drag-region>
  <!-- Title bar -->
  <div class="sticky-titlebar" data-tauri-drag-region>
    <button class="sticky-btn" class:pinned onclick={togglePin} title={pinned ? 'Unpin' : 'Pin to top'}>
      <Pin size={12} strokeWidth={2} />
    </button>
    <div style="flex: 1" data-tauri-drag-region></div>
    <button class="sticky-btn close" onclick={closeNote} title="Close note">
      <X size={12} strokeWidth={2} />
    </button>
  </div>

  <!-- Content -->
  <textarea
    class="sticky-content"
    value={content}
    oninput={handleInput}
    placeholder="Write a note..."
    spellcheck="true"
  ></textarea>

  <!-- Resize handle -->
  <div class="sticky-resize"></div>
</div>

<style>
  /* A sticky note is a soft accent block that fills its window edge-to-edge
     (the OS rounds the window); controls are round, the body is the note. */
  .sticky-note {
    display: flex;
    flex-direction: column;
    width: 100%;
    height: 100vh;
    background: var(--sempa-accent-bg);
    border-radius: 0;
    overflow: hidden;
    font-family: 'Plus Jakarta Sans', sans-serif;
  }

  .sticky-titlebar {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 6px 2px;
    cursor: grab;
    flex-shrink: 0;
  }

  .sticky-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border: none;
    border-radius: 999px;
    background: transparent;
    color: var(--sempa-text-soft);
    cursor: pointer;
    transition: background 120ms, color 120ms;
  }

  .sticky-btn:hover {
    background: color-mix(in srgb, var(--sempa-accent) 14%, transparent);
    color: var(--sempa-text);
  }

  .sticky-btn:active {
    transform: scale(0.97);
  }

  .sticky-btn.pinned {
    background: var(--sempa-bg-panel);
    box-shadow: var(--sempa-shadow-1);
    color: var(--sempa-accent-ink);
  }

  .sticky-btn.close:hover {
    background: color-mix(in srgb, #c53030 16%, transparent);
    color: #c53030;
  }

  .sticky-content {
    flex: 1;
    padding: 6px 14px 14px;
    border: none;
    background: transparent;
    color: var(--sempa-text);
    font-size: 13px;
    font-weight: 500;
    font-family: 'Plus Jakarta Sans', sans-serif;
    line-height: 1.6;
    resize: none;
    outline: none;
  }
  /* The note is the field; no global input focus ring around the whole window. */
  .sticky-content:focus-visible { box-shadow: none; }

  .sticky-content::placeholder {
    color: color-mix(in srgb, var(--sempa-accent-ink) 55%, var(--sempa-text-dim));
  }

  .sticky-resize {
    width: 12px;
    height: 12px;
    position: absolute;
    bottom: 0;
    right: 0;
    cursor: se-resize;
  }

  @media (prefers-reduced-motion: reduce) {
    .sticky-btn:active { transform: none; }
  }
</style>
