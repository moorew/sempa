<script lang="ts">
  /**
   * Shown when a completed task's time was filled in automatically because Sempa
   * has learned how long that kind of work takes.
   *
   * The alternative designs are both worse: keep prompting forever (the nagging
   * this feature exists to end), or write the number silently (time you never
   * confirmed, appearing on your tasks, with nothing to notice or correct). This
   * needs no action, but it's visible and one tap fixes it — and a correction
   * teaches the profile, so a wrong guess makes the next one better.
   */
  import { timeCapture } from '$lib/stores/timeCapture.svelte';
  import { bucketByKey } from '$lib/activityBuckets';
  import { formatMinutes } from '$lib/utils';
  import { Check, X } from 'lucide-svelte';

  const AUTO_DISMISS_MS = 8000;

  const entry = $derived(timeCapture.autoLogged);
  const bucket = $derived(entry ? bucketByKey(entry.bucketKey) : null);

  // Auto-dismiss. Keyed on the task id so a second completion restarts the clock
  // rather than inheriting the previous one's remaining time.
  $effect(() => {
    const id = entry?.taskId;
    if (!id) return;
    const t = setTimeout(() => timeCapture.clearAutoLogged(), AUTO_DISMISS_MS);
    return () => clearTimeout(t);
  });
</script>

{#if entry && bucket}
  <div class="pointer-events-auto flex items-center gap-2.5 rounded-xl px-3 py-2 shadow-lg"
       style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">
    <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg"
         style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
      <Check size={15} strokeWidth={2.2} />
    </div>

    <div class="min-w-0">
      <p class="truncate" style="font-size: 12.5px; color: var(--sempa-text);">
        Logged <strong>{formatMinutes(entry.minutes)}</strong> — {bucket.emoji} {bucket.label}
      </p>
      <p class="truncate" style="font-size: 11px; color: var(--sempa-text-dim);">
        Learned from your history
      </p>
    </div>

    <button
      onclick={() => timeCapture.correctAutoLogged()}
      class="shrink-0 rounded-md px-2.5 py-1 font-semibold transition-opacity hover:opacity-90"
      style="font-size: 12px; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
      Change
    </button>

    <button
      onclick={() => timeCapture.clearAutoLogged()}
      aria-label="Dismiss"
      class="shrink-0 rounded-md p-1 transition-colors"
      style="color: var(--sempa-text-dim);">
      <X size={14} />
    </button>
  </div>
{/if}
