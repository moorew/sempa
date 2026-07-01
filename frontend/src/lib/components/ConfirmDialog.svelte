<script lang="ts">
  /**
   * Sempa-themed confirmation modal, driven by the confirmDialog store. Replaces
   * the browser's native confirm() for destructive actions. Mounted once in the
   * root layout. Escape / backdrop = cancel; Enter = confirm.
   */
  import { confirmDialog } from '$lib/stores/confirmDialog.svelte';

  function onKeydown(e: KeyboardEvent) {
    if (!confirmDialog.open) return;
    if (e.key === 'Escape') { e.preventDefault(); confirmDialog.cancel(); }
    else if (e.key === 'Enter') { e.preventDefault(); confirmDialog.confirm(); }
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if confirmDialog.open}
  <div class="fixed inset-0 z-[120] flex items-center justify-center p-4"
       style="background: rgba(0,0,0,0.45);"
       onclick={(e) => { if (e.target === e.currentTarget) confirmDialog.cancel(); }}
       role="presentation">
    <div class="w-full max-w-sm overflow-hidden rounded-2xl shadow-2xl"
         style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);"
         role="alertdialog" aria-modal="true" aria-label={confirmDialog.opts.title}>
      <div class="px-5 pt-5 pb-4">
        <h2 class="text-sm font-semibold" style="color: var(--sempa-text);">{confirmDialog.opts.title}</h2>
        {#if confirmDialog.opts.message}
          <p class="mt-1.5 text-xs leading-relaxed" style="color: var(--sempa-text-soft);">{confirmDialog.opts.message}</p>
        {/if}
      </div>
      <div class="flex justify-end gap-2 px-5 py-3.5" style="border-top: 1px solid var(--sempa-border);">
        <button onclick={() => confirmDialog.cancel()}
                class="rounded-xl px-3.5 py-2 text-sm font-medium transition-opacity hover:opacity-70"
                style="color: var(--sempa-text-soft);">
          {confirmDialog.opts.cancelLabel ?? 'Cancel'}
        </button>
        <button onclick={() => confirmDialog.confirm()}
                class="rounded-xl px-3.5 py-2 text-sm font-semibold text-white transition-opacity hover:opacity-90"
                style="background: {confirmDialog.opts.danger ? '#dc2626' : 'var(--sempa-accent)'};">
          {confirmDialog.opts.confirmLabel ?? 'Confirm'}
        </button>
      </div>
    </div>
  </div>
{/if}
