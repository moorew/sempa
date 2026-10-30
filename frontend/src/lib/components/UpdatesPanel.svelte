<script lang="ts">
  /**
   * Settings → About / Updates. Shows the running version, the update channel,
   * whether automatic checks are on, when it last checked, and a manual "Check
   * for updates" button. When a newer release exists it surfaces the release
   * notes ("What's new") and a download/install action. Brand-controlled, theme
   * aware — mirrors the in-app updater design.
   */
  import { onMount } from 'svelte';
  import { updates, type UpdateChannel } from '$lib/stores/updates.svelte';
  import { openExternal } from '$lib/external';
  import { RefreshCw, Download, CheckCircle2, Sparkles } from 'lucide-svelte';

  let now = $state(Date.now());
  onMount(() => {
    // A gentle, throttled check when the user lands on this page.
    void updates.check(false);
    const t = setInterval(() => (now = Date.now()), 30_000);
    return () => clearInterval(t);
  });

  function ago(iso: string | null, n: number): string {
    if (!iso) return 'never';
    const secs = Math.max(0, Math.floor((n - new Date(iso).getTime()) / 1000));
    if (secs < 60) return 'just now';
    const m = Math.floor(secs / 60);
    if (m < 60) return `${m}m ago`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h}h ago`;
    return `${Math.floor(h / 24)}d ago`;
  }

  const channels: { id: UpdateChannel; label: string }[] = [
    { id: 'stable', label: 'Stable' },
    { id: 'prerelease', label: 'Beta' },
  ];
</script>

<section class="upd">
  <!-- Identity + current version -->
  <div class="upd-row" style="min-height: 72px;">
    <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-[14px]"
          style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); box-shadow: var(--sempa-shadow-accent);">
      <svg width="24" height="24" viewBox="0 0 100 100" fill="none" aria-hidden="true">
        <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="10" stroke-linecap="round" stroke-linejoin="round"/>
        <circle cx="50" cy="35" r="8" fill="currentColor"/>
      </svg>
    </span>
    <div class="min-w-0 flex-1">
      <p class="upd-label" style="font-size: 15px;">Sempa</p>
      <p class="upd-desc">Version <span class="font-mono" style="font-size: 12px;">{updates.current}</span></p>
    </div>
    {#if !updates.available && !updates.checking}
      <span class="upd-chip">
        <CheckCircle2 size={14} strokeWidth={2.2} /> Up to date
      </span>
    {/if}
  </div>

  <!-- Update available banner -->
  {#if updates.available && updates.info}
    <div class="upd-block">
      <div class="rounded-2xl p-4" style="background: var(--sempa-accent-bg);">
        <div class="flex items-center gap-2">
          <Sparkles size={16} style="color: var(--sempa-accent);" />
          <p class="text-sm font-bold" style="color: var(--sempa-accent-ink);">
            Update available — {updates.info.version}
          </p>
        </div>
        {#if updates.info.notes}
          <div class="mt-2 max-h-44 overflow-y-auto whitespace-pre-wrap text-[12.5px] leading-relaxed" style="color: var(--sempa-text-soft);">{updates.info.notes}</div>
        {/if}
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <button onclick={() => openExternal(updates.info!.downloadUrl)}
             class="btn-primary upd-btn">
            <Download size={14} strokeWidth={2.2} /> Download update
          </button>
          <button onclick={() => openExternal(updates.info!.url)}
             class="btn-surface upd-btn">
            What’s new
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Channel -->
  <div class="upd-row flex-wrap">
    <div class="min-w-0 flex-1">
      <p class="upd-label">Update channel</p>
      <p class="upd-desc">Beta surfaces pre-releases earlier.</p>
    </div>
    <div class="upd-seg">
      {#each channels as c}
        <button onclick={() => updates.setChannel(c.id)}
                aria-pressed={updates.channel === c.id}
                data-active={updates.channel === c.id}>
          {c.label}
        </button>
      {/each}
    </div>
  </div>

  <!-- Automatic updates toggle -->
  <div class="upd-row">
    <div class="min-w-0 flex-1">
      <p class="upd-label">Automatic checks</p>
      <p class="upd-desc">Check for new versions in the background.</p>
    </div>
    <button onclick={() => updates.setAutoCheck(!updates.autoCheck)}
            role="switch" aria-checked={updates.autoCheck} aria-label="Automatic update checks"
            class="upd-switch" data-on={updates.autoCheck}></button>
  </div>

  <!-- Last checked + manual check -->
  <div class="upd-row">
    <p class="min-w-0 flex-1 text-[12.5px]" style="color: var(--sempa-text-soft);">
      Last checked {ago(updates.lastChecked, now)}
      {#if updates.error}· <span style="color: var(--sempa-text);">{updates.error}</span>{/if}
    </p>
    <button onclick={() => updates.check(true)} disabled={updates.checking}
            class="upd-quiet upd-btn">
      <span class:spin={updates.checking}><RefreshCw size={13} strokeWidth={2.2} /></span>
      {updates.checking ? 'Checking…' : 'Check for updates'}
    </button>
  </div>
</section>

<style>
  /* Grouped raised surface with divided rows — the Settings 1.27 pattern,
     scoped here so the panel stands on its own wherever it's rendered. */
  .upd {
    background: var(--sempa-bg-panel);
    border-radius: 20px;
    box-shadow: var(--sempa-shadow-1);
  }
  .upd > * + * { border-top: 1px solid var(--sempa-border); }
  .upd-row {
    display: flex; align-items: center; gap: 14px;
    min-height: 56px; padding: 12px 18px;
  }
  .upd-block { padding: 14px 18px 16px; }
  .upd-label { font-size: 14px; font-weight: 600; line-height: 1.35; color: var(--sempa-text); }
  .upd-desc  { margin-top: 2px; font-size: 12.5px; line-height: 1.45; color: var(--sempa-text-soft); }
  .upd-chip {
    display: inline-flex; align-items: center; gap: 5px; flex-shrink: 0;
    height: 26px; padding: 0 10px; border-radius: 999px;
    background: var(--sempa-success-bg); color: var(--sempa-success);
    font-size: 12px; font-weight: 700;
  }
  .upd-btn { height: 34px; padding: 0 14px; font-size: 12.5px; gap: 6px; }
  .upd-quiet {
    display: inline-flex; align-items: center; justify-content: center;
    border: 0; border-radius: 999px; cursor: pointer; white-space: nowrap; font-weight: 600;
    background: var(--sempa-fill); color: var(--sempa-text);
    box-shadow: inset 0 0 0 1px var(--sempa-border);
    transition: color 120ms ease;
  }
  .upd-quiet:hover:not(:disabled) { color: var(--sempa-accent-ink); }
  .upd-quiet:disabled { opacity: .5; cursor: default; }
  @media (pointer: coarse) { .upd-btn { height: 40px; } }

  .upd-seg { display: inline-flex; gap: 3px; padding: 4px; border-radius: 999px; background: var(--sempa-fill); }
  .upd-seg > button {
    padding: 6px 15px; border: 0; border-radius: 999px; background: transparent; cursor: pointer;
    color: var(--sempa-text-soft); font-size: 13px; font-weight: 600;
    transition: background-color 120ms ease, color 120ms ease;
  }
  .upd-seg > button[data-active="true"] { background: var(--sempa-bg-panel); color: var(--sempa-text); box-shadow: var(--sempa-shadow-1); }

  .upd-switch {
    position: relative; flex-shrink: 0; width: 46px; height: 28px; padding: 0; border: 0;
    border-radius: 999px; background: var(--sempa-track); cursor: pointer;
    transition: background-color 160ms ease;
  }
  .upd-switch::after {
    content: ''; position: absolute; top: 3px; left: 3px; width: 22px; height: 22px;
    border-radius: 999px; background: white;
    box-shadow: 0 1px 3px rgba(0, 0, 0, .22), 0 1px 1px rgba(0, 0, 0, .06);
    transition: transform 180ms cubic-bezier(.3, .7, .4, 1);
  }
  .upd-switch[data-on="true"] { background: var(--sempa-accent); }
  .upd-switch[data-on="true"]::after { transform: translateX(18px); }

  .spin { display: inline-flex; animation: upd-spin 1s linear infinite; }
  @keyframes upd-spin { to { transform: rotate(360deg); } }
</style>
