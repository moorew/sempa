<script lang="ts">
  /**
   * Brand-controlled "update available" toast — the in-app prompt from the
   * updater design. Slides in at the bottom-right when a newer release is
   * available and not yet dismissed; offers Download + What's new, or Later.
   * Purely presentational on top of the `updates` store.
   */
  import { updates } from '$lib/stores/updates.svelte';
  import { openExternal } from '$lib/external';
  import { Download, X, Sparkles } from 'lucide-svelte';

  let expanded = $state(false);
</script>

{#if updates.available && updates.info}
  <div class="fixed bottom-5 right-5 z-[70] w-80 max-w-[calc(100vw-2.5rem)] overflow-hidden"
       style="border-radius: 18px; background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-2);"
       role="status" aria-live="polite">
    <div class="flex items-start gap-3 p-4">
      <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-[12px]" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
        <Sparkles size={17} strokeWidth={2} />
      </span>
      <div class="min-w-0 flex-1">
        <p style="font-size: 14px; font-weight: 700; color: var(--sempa-text);">Update available</p>
        <p class="mt-0.5 text-[12.5px]" style="color: var(--sempa-text-soft);">
          Sempa {updates.info.version} is ready to install.
        </p>

        {#if expanded && updates.info.notes}
          <div class="mt-2 max-h-32 overflow-y-auto whitespace-pre-wrap rounded-[12px] p-2.5 text-[11.5px] leading-relaxed"
               style="background: var(--sempa-fill); color: var(--sempa-text-soft);">{updates.info.notes}</div>
        {/if}

        <div class="mt-3 flex items-center gap-1.5">
          <button onclick={() => openExternal(updates.info!.downloadUrl)}
             class="btn-primary h-9 gap-1.5 px-3.5 text-[12.5px]">
            <Download size={13} strokeWidth={2.25} /> Download
          </button>
          {#if updates.info.notes}
            <button onclick={() => (expanded = !expanded)}
                    class="btn-soft h-9 px-3 text-[12.5px]"
                    style="background: var(--sempa-fill); color: var(--sempa-text-soft);">
              {expanded ? 'Hide notes' : 'What’s new'}
            </button>
          {/if}
          <button onclick={() => updates.dismiss()}
                  class="ml-auto h-9 rounded-full px-2.5 text-[12.5px] font-semibold transition-colors"
                  style="color: var(--sempa-text-dim);">
            Later
          </button>
        </div>
      </div>
      <button onclick={() => updates.dismiss()} aria-label="Dismiss update notification"
              class="-mr-1.5 -mt-1.5 flex h-8 w-8 items-center justify-center rounded-full transition-colors" style="color: var(--sempa-text-dim);">
        <X size={15} strokeWidth={2} />
      </button>
    </div>
  </div>
{/if}
