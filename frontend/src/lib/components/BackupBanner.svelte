<script lang="ts">
  /**
   * Warns when backups have stopped working. Rendered by +layout.svelte alongside
   * the other banners.
   *
   * A failing backup used to be visible only in the server log, so it could stay
   * broken for days — the failure mode that matters most, discovered last. The
   * common cause is an expired Google refresh token, which no amount of retrying
   * fixes: only the user reconnecting does. Hence a prompt, not just a warning.
   */
  import { backupHealth } from '$lib/stores/backupHealth.svelte';
  import { ShieldAlert, X } from 'lucide-svelte';

  const show = $derived(backupHealth.failing && !backupHealth.dismissed);
</script>

{#if show}
  <div class="banner flex-wrap">
    <div class="flex min-w-0 flex-[1_1_15rem] items-center gap-3">
      <div class="banner-icon"
           style="background: color-mix(in srgb, var(--sempa-amber) var(--sempa-tint-bg), var(--sempa-bg-panel));
                  color: color-mix(in srgb, var(--sempa-amber) var(--sempa-tint-fg), var(--sempa-text));">
        <ShieldAlert size={17} strokeWidth={2} />
      </div>

      <div class="min-w-0 flex-1">
        <p class="banner-title truncate">
          {backupHealth.needsReconnect ? 'Backups need reconnecting' : 'Backups are failing'}
        </p>
        <p class="banner-hint truncate">
          {#if backupHealth.needsReconnect}
            Google access expired, so nothing has been backed up since.
          {:else}
            {backupHealth.data?.last_error ?? 'The last scheduled backup did not complete.'}
          {/if}
        </p>
      </div>
    </div>

    <div class="ml-auto flex shrink-0 items-center gap-1.5">
      <a href="/settings/backup" class="btn-primary banner-btn">
        {backupHealth.needsReconnect ? 'Reconnect' : 'Open backups'}
      </a>

      <button
        onclick={() => backupHealth.dismiss()}
        aria-label="Dismiss for now"
        title="Dismiss for now — returns next launch until backups succeed"
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
