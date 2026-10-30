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
  <div class="pointer-events-auto flex items-center gap-3 py-2 pl-2 pr-2"
       style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-2); border-radius: 999px;">
    <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full"
         style="background: var(--sempa-success-bg); color: var(--sempa-success);">
      <Check size={16} strokeWidth={2.5} />
    </div>

    <div class="min-w-0">
      <p class="truncate" style="font-size: 13px; font-weight: 500; color: var(--sempa-text);">
        Logged <strong style="font-weight: 800;">{formatMinutes(entry.minutes)}</strong> — {bucket.label}
      </p>
      <p class="truncate" style="font-size: 11.5px; color: var(--sempa-text-dim);">
        Learned from your history
      </p>
    </div>

    <button
      onclick={() => timeCapture.correctAutoLogged()}
      class="btn-soft shrink-0"
      style="height: 32px; padding: 0 14px; font-size: 12.5px;">
      Change
    </button>

    <button
      onclick={() => timeCapture.clearAutoLogged()}
      aria-label="Dismiss"
      class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full transition-colors hover:bg-[var(--sempa-fill)]"
      style="color: var(--sempa-text-dim);">
      <X size={14} />
    </button>
  </div>
{/if}
