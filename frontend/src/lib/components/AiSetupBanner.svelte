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
  <div class="flex items-center gap-2.5 rounded-lg border px-3 py-2"
       style="border-color: var(--sempa-accent-bg); background: var(--sempa-accent-bg);">
    <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg"
         style="background: var(--sempa-bg-panel); color: var(--sempa-accent);">
      <Sparkles size={15} strokeWidth={2} />
    </div>

    <div class="flex min-w-0 flex-1 items-baseline gap-2">
      <p class="shrink-0 font-semibold" style="font-size: 13px; color: var(--sempa-text);">
        Turn on AI assist
      </p>
      <p class="truncate" style="font-size: 11.5px; color: var(--sempa-text-soft);">
        Connect a local model to unlock quick-add, planning and recipe import — nothing leaves your server.
      </p>
    </div>

    <button
      onclick={() => { dismiss(); goto('/settings/accounts'); }}
      class="shrink-0 rounded-md px-2.5 py-1 font-semibold transition-opacity hover:opacity-90"
      style="font-size: 12px; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
      Set up
    </button>

    <button
      onclick={dismiss}
      aria-label="Dismiss"
      class="shrink-0 rounded-md p-1 transition-colors"
      style="color: var(--sempa-text-dim);">
      <X size={15} />
    </button>
  </div>
{/if}
