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

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  data-tauri-drag-region
  style="height:100vh; display:flex; flex-direction:column; gap:10px; padding:16px 18px;
         background: var(--sempa-bg-panel); border:1px solid var(--sempa-border); border-radius:14px;
         box-sizing:border-box; overflow:hidden; user-select:none;">
  <div style="display:flex; align-items:center; gap:8px; color: var(--sempa-accent);">
    <svg width="20" height="20" viewBox="0 0 100 100" fill="none" aria-hidden="true">
      <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="9"
            stroke-linecap="round" stroke-linejoin="round"/>
      <circle cx="50" cy="35" r="7.5" fill="currentColor"/>
    </svg>
    <span style="font-family:'JetBrains Mono', monospace; font-size:11px; letter-spacing:.14em;
                 text-transform:uppercase; color: var(--sempa-text-dim);">Quick add</span>
    <span style="margin-left:auto; font-family:'JetBrains Mono', monospace; font-size:11px;
                 color: var(--sempa-text-dim);">Enter ↵ · Esc</span>
  </div>

  <input
    bind:this={inputEl}
    bind:value={title}
    type="text"
    placeholder="Add a task to today…"
    disabled={busy}
    style="width:100%; box-sizing:border-box; padding:12px 14px; border-radius:10px;
           background: var(--sempa-bg-main); border:1px solid var(--sempa-border);
           color: var(--sempa-text); font-size:16px; font-family:'Plus Jakarta Sans', sans-serif;
           outline:none; user-select:text;"
    onfocus={(e) => ((e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)')}
    onblur={(e) => ((e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)')}
  />
</div>
