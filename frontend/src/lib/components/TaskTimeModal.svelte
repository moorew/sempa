<script lang="ts">
  import { untrack } from 'svelte';
  import { timeCapture } from '$lib/stores/timeCapture.svelte';
  import { bucketByKey } from '$lib/activityBuckets';
  import { Sparkles } from 'lucide-svelte';

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
    <div class="w-full max-w-sm rounded-2xl p-5 shadow-2xl"
      style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: var(--sempa-text);">

      <p class="text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-accent);">
        How long did that take?
      </p>
      <p class="mt-1 truncate text-base font-semibold" title={p.title}>{p.title}</p>
      {#if bucket}
        <p class="mt-0.5 text-xs" style="color: var(--sempa-text-dim);">
          {bucket.emoji} looks like {bucket.label}
        </p>
      {/if}

      <!-- Quick chips -->
      <div class="mt-4 flex flex-wrap gap-1.5">
        {#each CHIPS as c}
          <button onclick={() => (minutes = c)}
            class="rounded-lg px-3 py-1.5 text-sm font-medium transition-colors"
            style={minutes === c
              ? 'background: var(--sempa-accent); color: #fff;'
              : 'background: var(--sempa-bg); color: var(--sempa-text-soft); border: 1px solid var(--sempa-border);'}>
            {c}m
          </button>
        {/each}
      </div>

      <!-- Custom + AI suggestion -->
      <div class="mt-3 flex items-center gap-3">
        <label class="flex items-center gap-1.5 text-sm" style="color: var(--sempa-text-soft);">
          <input type="number" min="0" max="600" bind:value={minutes}
            class="w-16 rounded px-2 py-1 text-right text-sm"
            style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
          min
        </label>
        {#if aiSuggestion && aiSuggestion !== minutes}
          <button onclick={() => (minutes = aiSuggestion)}
            class="flex items-center gap-1 rounded-lg px-2 py-1 text-xs transition-opacity hover:opacity-80"
            style="background: var(--sempa-accent-bg); color: var(--sempa-accent);"
            title="Estimated from your history">
            <Sparkles size={12} strokeWidth={2} /> AI: ~{aiSuggestion}m
          </button>
        {/if}
      </div>

      <button onclick={log}
        class="mt-5 w-full rounded-xl py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90"
        style="background: var(--sempa-accent);">
        Log {minutes}m
      </button>
      <div class="mt-2 flex items-center justify-between text-xs">
        <button onclick={skip} class="rounded px-2 py-1.5 transition-opacity hover:opacity-70"
          style="color: var(--sempa-text-dim);">Skip</button>
        <button onclick={disable} class="rounded px-2 py-1.5 transition-opacity hover:opacity-70"
          style="color: var(--sempa-text-dim);">Don't ask again</button>
      </div>
    </div>
  </div>
{/if}
