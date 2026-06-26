<script lang="ts">
  /**
   * Calm prompt shown when the device has travelled to a new timezone. Offers to
   * make the new zone the server's home, or to keep it just for the trip. The
   * day boundary already follows the device either way — this only decides where
   * the server anchors its background work. Rendered by +layout.svelte.
   */
  import { timezone } from '$lib/stores/timezone.svelte';
  import { Plane, X } from 'lucide-svelte';

  let saving = $state(false);

  async function update() {
    saving = true;
    try {
      await timezone.updateHome();
    } finally {
      saving = false;
    }
  }
</script>

{#if timezone.mismatch}
  <div class="flex items-center gap-2.5 rounded-lg border px-3 py-2"
       style="border-color: var(--sempa-accent-bg); background: var(--sempa-accent-bg);">
    <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg"
         style="background: var(--sempa-bg-panel); color: var(--sempa-accent);">
      <Plane size={15} strokeWidth={2} />
    </div>

    <div class="flex min-w-0 flex-1 items-baseline gap-2">
      <p class="shrink-0 font-semibold" style="font-size: 13px; color: var(--sempa-text);">
        Now in {timezone.deviceLabel}
      </p>
      <p class="truncate" style="font-size: 11.5px; color: var(--sempa-text-soft);">
        Your day follows this zone. Make it Sempa's home, or keep it just for the trip?
      </p>
    </div>

    <button
      onclick={update}
      disabled={saving}
      class="shrink-0 rounded-md px-2.5 py-1 font-semibold transition-opacity hover:opacity-90 disabled:opacity-60"
      style="font-size: 12px; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
      {saving ? 'Saving…' : 'Update home'}
    </button>

    <button
      onclick={() => timezone.keepForTrip()}
      class="shrink-0 rounded-md px-2.5 py-1 font-medium transition-colors"
      style="font-size: 12px; color: var(--sempa-text-soft);">
      Just this trip
    </button>

    <button
      onclick={() => timezone.keepForTrip()}
      aria-label="Dismiss"
      class="shrink-0 rounded-md p-1 transition-colors"
      style="color: var(--sempa-text-dim);">
      <X size={15} />
    </button>
  </div>
{/if}
