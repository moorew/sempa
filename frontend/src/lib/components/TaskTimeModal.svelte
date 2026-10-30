<script lang="ts">
  import { untrack } from 'svelte';
  import { timeCapture } from '$lib/stores/timeCapture.svelte';
  import { bucketByKey } from '$lib/activityBuckets';
  import { Sparkles, Clock } from 'lucide-svelte';

  // One-tap-fast time capture shown when a task is completed without tracked
  // time. Seeded with the activity-bucket / estimate default; the AI suggestion
  // (if any) arrives asynchronously as an extra chip.
  const CHIPS = [5, 10, 15, 30, 45, 60, 90, 120];

  let minutes = $state(0);
  let seededFor = $state<string | null>(null);

  $effect(() => {
    const p = timeCapture.pending;
    if (!p) return;
    untrack(() => {
      if (seededFor !== p.taskId) {
        minutes = p.defaultMinutes;
        seededFor = p.taskId;
      }
    });
  });

  const p = $derived(timeCapture.pending);
  const bucket = $derived(p ? bucketByKey(p.bucketKey) : null);
  const aiSuggestion = $derived(timeCapture.aiSuggestion);

  function log() { void timeCapture.log(minutes); }
  function skip() { timeCapture.skip(); }
  function disable() { timeCapture.disable(); }
</script>

{#if p}
  <div class="fixed inset-0 z-[60] flex items-end justify-center p-4 sm:items-center"
    style="background: rgba(0,0,0,0.45);" role="dialog" aria-modal="true">
    <div class="surface-raised w-full max-w-sm p-5 sm:p-6" style="color: var(--sempa-text);">

      <p class="flex items-center gap-1.5 type-label" style="color: var(--sempa-accent-ink);">
        <Clock size={13} strokeWidth={2.25} />
        How long did that take?
      </p>
      <p class="mt-1.5 truncate" style="font-size: 17px; font-weight: 800; letter-spacing: -.015em;" title={p.title}>{p.title}</p>
      {#if bucket}
        <p class="mt-1 type-meta" style="color: var(--sempa-text-soft);">
          Looks like {bucket.label}
        </p>
      {/if}

      <!-- Quick chips -->
      <div class="mt-4 flex flex-wrap gap-1.5">
        {#each CHIPS as c}
          <button onclick={() => (minutes = c)}
            class="time-chip"
            aria-pressed={minutes === c}>
            {c}m
          </button>
        {/each}
      </div>

      <!-- Custom + AI suggestion -->
      <div class="mt-3.5 flex items-center gap-3">
        <label class="flex items-center gap-2 text-sm" style="color: var(--sempa-text-soft);">
          <input type="number" min="0" max="600" bind:value={minutes}
            class="text-right text-sm"
            style="width: 72px; height: 36px; padding: 0 10px; border-radius: 12px;
                   border: 1px solid var(--sempa-border); background: var(--sempa-fill); color: var(--sempa-text);" />
          min
        </label>
        {#if aiSuggestion && aiSuggestion !== minutes}
          <button onclick={() => (minutes = aiSuggestion)}
            class="btn-soft"
            style="height: 32px; padding: 0 12px; font-size: 12.5px;"
            title="Estimated from your history">
            <Sparkles size={12} strokeWidth={2} /> AI: ~{aiSuggestion}m
          </button>
        {/if}
      </div>

      <button onclick={log}
        class="btn-primary mt-5 w-full"
        style="height: 46px; font-size: 14.5px;">
        Log {minutes}m
      </button>
      <div class="mt-2 flex items-center justify-between text-xs">
        <button onclick={skip} class="ghost-link">Skip</button>
        <button onclick={disable} class="ghost-link">Don't ask again</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .time-chip {
    height: 36px; padding: 0 14px; border-radius: 999px;
    font-size: 13.5px; font-weight: 600;
    background: var(--sempa-fill); color: var(--sempa-text-soft);
    transition: background-color 120ms ease, color 120ms ease;
  }
  .time-chip:hover { color: var(--sempa-text); }
  .time-chip[aria-pressed="true"] {
    background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); box-shadow: var(--sempa-shadow-accent);
  }
  .ghost-link {
    height: 32px; padding: 0 10px; border-radius: 999px;
    font-size: 12.5px; font-weight: 600; color: var(--sempa-text-dim);
    transition: background-color 120ms ease, color 120ms ease;
  }
  .ghost-link:hover { background: var(--sempa-fill); color: var(--sempa-text-soft); }
</style>
