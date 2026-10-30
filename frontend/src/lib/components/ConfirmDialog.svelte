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
       style="background: color-mix(in srgb, var(--sempa-bg-rail) 30%, rgba(0,0,0,0.42));"
       onclick={(e) => { if (e.target === e.currentTarget) confirmDialog.cancel(); }}
       role="presentation">
    <div class="surface-raised w-full max-w-sm overflow-hidden p-6"
         role="alertdialog" aria-modal="true" aria-label={confirmDialog.opts.title}>
      <h2 style="font-size: 17px; font-weight: 800; letter-spacing: -.02em; line-height: 1.3; color: var(--sempa-text);">{confirmDialog.opts.title}</h2>
      {#if confirmDialog.opts.message}
        <p class="mt-2 text-[13px] leading-relaxed" style="color: var(--sempa-text-soft);">{confirmDialog.opts.message}</p>
      {/if}
      <div class="mt-6 flex justify-end gap-2">
        <button onclick={() => confirmDialog.cancel()}
                class="btn-soft h-10 px-5 text-sm"
                style="background: var(--sempa-fill); color: var(--sempa-text-soft);">
          {confirmDialog.opts.cancelLabel ?? 'Cancel'}
        </button>
        <button onclick={() => confirmDialog.confirm()}
                class="btn-primary h-10 px-5 text-sm"
                style={confirmDialog.opts.danger
                  ? 'background: var(--sempa-danger, #c0392b); color: #fff; box-shadow: 0 10px 22px -10px color-mix(in srgb, var(--sempa-danger, #c0392b) 70%, transparent);'
                  : ''}>
          {confirmDialog.opts.confirmLabel ?? 'Confirm'}
        </button>
      </div>
    </div>
  </div>
{/if}
