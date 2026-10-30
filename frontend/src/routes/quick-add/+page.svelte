<script lang="ts">
  import { onMount } from 'svelte';
  import { quickAddTask } from '$lib/tauri/bridge';
  import { today } from '$lib/utils';

  let title = $state('');
  let busy = $state(false);
  let inputEl: HTMLInputElement | null = $state(null);

  async function closeWin() {
    try {
      const { getCurrentWindow } = await import('@tauri-apps/api/window');
      await getCurrentWindow().close();
    } catch { /* not in Tauri */ }
  }

  async function submit() {
    const t = title.trim();
    if (!t || busy) return;
    busy = true;
    try {
      // Quick capture lands on today's plan.
      await quickAddTask(t, today());
    } catch { /* swallow — closing anyway keeps capture frictionless */ }
    await closeWin();
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') { e.preventDefault(); void closeWin(); }
    else if (e.key === 'Enter') { e.preventDefault(); void submit(); }
  }

  onMount(() => {
    inputEl?.focus();
    // Close if the window loses focus (click-away), like a spotlight capture.
    const onBlur = () => { void closeWin(); };
    window.addEventListener('blur', onBlur);
    return () => window.removeEventListener('blur', onBlur);
  });
</script>

<svelte:window onkeydown={onKey} />

<svelte:head>
  <style>
    /* Opaque window (transparent:false in Rust): the panel paints every pixel,
       so the page behind it matches and no backing shows at the edges. */
    html, body { background: var(--sempa-bg-panel) !important; margin: 0; overflow: hidden; }
  </style>
</svelte:head>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  data-tauri-drag-region
  style="height:100vh; display:flex; flex-direction:column; justify-content:center; gap:12px; padding:14px 18px 18px;
         background: var(--sempa-bg-panel); border-radius:0;
         box-sizing:border-box; overflow:hidden; user-select:none;">
  <div style="display:flex; align-items:center; gap:8px;">
    <span style="display:flex; align-items:center; justify-content:center; width:26px; height:26px; border-radius:999px;
                 background: var(--sempa-accent-bg); color: var(--sempa-accent);">
      <svg width="16" height="16" viewBox="0 0 100 100" fill="none" aria-hidden="true">
        <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="9"
              stroke-linecap="round" stroke-linejoin="round"/>
        <circle cx="50" cy="35" r="7.5" fill="currentColor"/>
      </svg>
    </span>
    <span style="font-size:13px; font-weight:700; color: var(--sempa-text);">Quick add</span>
    <span style="font-size:12px; font-weight:500; color: var(--sempa-text-dim);">to today</span>
    <span style="margin-left:auto; display:flex; gap:6px;">
      <span class="chip chip-time">Enter ↵</span>
      <span class="chip chip-time">Esc</span>
    </span>
  </div>

  <input
    bind:this={inputEl}
    bind:value={title}
    type="text"
    placeholder="Add a task to today…"
    disabled={busy}
    style="width:100%; box-sizing:border-box; height:48px; padding:0 20px; border-radius:999px;
           background: var(--sempa-fill); border:1.5px solid var(--sempa-border);
           color: var(--sempa-text); font-size:16px; font-weight:500; font-family:'Plus Jakarta Sans', sans-serif;
           outline:none; user-select:text;"
    onfocus={(e) => ((e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)')}
    onblur={(e) => ((e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)')}
  />
</div>
