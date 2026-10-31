<script lang="ts">
  /**
   * One-time nudge to connect a local AI model. All AI features hide themselves
   * when no model is reachable — which means a new user has no way to discover
   * them. This calm, dismissible banner closes that gap: it appears only after
   * the reachability check has run and come back empty, and never again once
   * dismissed. Rendered by +layout.svelte.
   */
  import { aiStatus } from '$lib/stores/aiStatus.svelte';
  import { goto } from '$app/navigation';
  import { Sparkles, X } from 'lucide-svelte';

  const DISMISS_KEY = 'sempa.aiSetupDismissed';
  let dismissed = $state(
    typeof localStorage !== 'undefined' && localStorage.getItem(DISMISS_KEY) === '1',
  );

  function dismiss() {
    dismissed = true;
    if (typeof localStorage !== 'undefined') localStorage.setItem(DISMISS_KEY, '1');
  }

  const show = $derived(aiStatus.loaded && !aiStatus.reachable && !dismissed);
</script>

{#if show}
  <div class="banner flex-wrap">
    <div class="flex min-w-0 flex-[1_1_15rem] items-center gap-3">
      <div class="banner-icon">
        <Sparkles size={17} strokeWidth={2} />
      </div>

      <div class="min-w-0 flex-1">
        <p class="banner-title truncate">
          Turn on AI assist
        </p>
        <p class="banner-hint truncate">
          Connect a model (on your server, or through Aperture) to unlock quick-add, planning and recipe import.
        </p>
      </div>
    </div>

    <div class="ml-auto flex shrink-0 items-center gap-1.5">
      <button
        onclick={() => { dismiss(); goto('/settings/accounts'); }}
        class="btn-primary banner-btn">
        Set up
      </button>

      <button
        onclick={dismiss}
        aria-label="Dismiss"
        class="banner-x">
        <X size={16} />
      </button>
    </div>
  </div>
{/if}

<style>
  /* 1.27: a soft raised surface with a tinted icon tile and pill actions. */
  .banner {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px;
    border-radius: 18px;
    background: var(--sempa-bg-panel);
    box-shadow: var(--sempa-shadow-1);
  }
  .banner-icon {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 38px;
    height: 38px;
    border-radius: 12px;
    background: var(--sempa-accent-bg);
    color: var(--sempa-accent-ink);
  }
  .banner-title { font-size: 13.5px; font-weight: 700; line-height: 1.25; color: var(--sempa-text); }
  .banner-hint  { font-size: 12px; font-weight: 500; line-height: 1.35; color: var(--sempa-text-soft); }
  .banner-btn   { height: 36px; padding: 0 15px; font-size: 12.5px; flex-shrink: 0; }
  .banner-x {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: 999px;
    color: var(--sempa-text-dim);
    transition: background-color 120ms ease, color 120ms ease;
  }
  .banner-x:hover { background: var(--sempa-fill); color: var(--sempa-text); }
</style>
