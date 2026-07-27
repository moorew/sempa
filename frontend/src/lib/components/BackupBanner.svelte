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
  <div class="flex items-center gap-2.5 rounded-lg border px-3 py-2"
       style="border-color: var(--sempa-accent-bg); background: var(--sempa-accent-bg);">
    <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg"
         style="background: var(--sempa-bg-panel); color: var(--sempa-accent);">
      <ShieldAlert size={15} strokeWidth={2} />
    </div>

    <div class="flex min-w-0 flex-1 items-baseline gap-2">
      <p class="shrink-0 font-semibold" style="font-size: 13px; color: var(--sempa-text);">
        {backupHealth.needsReconnect ? 'Backups need reconnecting' : 'Backups are failing'}
      </p>
      <p class="truncate" style="font-size: 11.5px; color: var(--sempa-text-soft);">
        {#if backupHealth.needsReconnect}
          Google access expired, so nothing has been backed up since.
        {:else}
          {backupHealth.data?.last_error ?? 'The last scheduled backup did not complete.'}
        {/if}
      </p>
    </div>

    <a href="/settings/backup"
       class="shrink-0 rounded-md px-2.5 py-1 font-semibold transition-opacity hover:opacity-90"
       style="font-size: 12px; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
      {backupHealth.needsReconnect ? 'Reconnect' : 'Open backups'}
    </a>

    <button
      onclick={() => backupHealth.dismiss()}
      aria-label="Dismiss for now"
      title="Dismiss for now — returns next launch until backups succeed"
      class="shrink-0 rounded-md p-1 transition-colors"
      style="color: var(--sempa-text-dim);">
      <X size={15} />
    </button>
  </div>
{/if}
