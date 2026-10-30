<script lang="ts">
  import { untrack } from 'svelte';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { formatMinutes } from '$lib/utils';

  // Mounted only while a session is awaiting confirmation. We seed a local,
  // editable minutes value from the measured wall-clock time and re-seed
  // whenever a new session arrives (the task id changes). The seed write is
  // untracked so the effect depends only on pendingConfirm (avoids the
  // read-modify-write $state loop that wedges the app).
  let minutes = $state(0);
  let seededFor = $state<string | null>(null);

  $effect(() => {
    const pc = pomodoro.pendingConfirm;
    if (!pc) return;
    untrack(() => {
      if (seededFor !== pc.taskId) {
        minutes = pc.elapsedMinutes;
        seededFor = pc.taskId;
      }
    });
  });

  const pc = $derived(pomodoro.pendingConfirm);
  const planned = $derived(pc?.plannedMinutes ?? null);
  const delta = $derived(planned ? minutes - planned : null);

  function bump(n: number) { minutes = Math.max(0, minutes + n); }
  function save()   { void pomodoro.confirmSession(minutes); }
  function none()   { void pomodoro.confirmSession(0); }
  function cancel() { pomodoro.discardSession(); }
</script>

{#if pc}
  <div class="fixed inset-0 z-[60] flex items-end justify-center p-4 sm:items-center"
    style="background: rgba(0,0,0,0.45);" role="dialog" aria-modal="true">
    <div class="surface-raised w-full max-w-sm p-5 sm:p-6" style="color: var(--sempa-text);">

      <p class="type-label" style="color: var(--sempa-accent-ink);">
        {pc.markDone ? 'Nice work — log your time' : 'How long did that take?'}
      </p>
      <p class="mb-5 mt-1.5 truncate" style="font-size: 17px; font-weight: 800; letter-spacing: -.015em;" title={pc.taskTitle}>{pc.taskTitle}</p>

      <!-- Big editable minutes -->
      <div class="flex items-center justify-center gap-4 py-4" style="background: var(--sempa-fill); border-radius: 20px;">
        <button onclick={() => bump(-5)}
          class="btn-icon"
          style="width: 44px; height: 44px; font-size: 20px;" aria-label="Subtract 5 minutes">−</button>
        <div class="flex items-baseline gap-1">
          <input type="number" min="0" max="600" bind:value={minutes}
            aria-label="Minutes"
            class="w-20 bg-transparent text-center font-mono tabular-nums outline-none"
            style="font-size: 40px; font-weight: 700; letter-spacing: -.03em; color: var(--sempa-text);" />
          <span class="text-sm font-medium" style="color: var(--sempa-text-soft);">min</span>
        </div>
        <button onclick={() => bump(5)}
          class="btn-icon"
          style="width: 44px; height: 44px; font-size: 20px;" aria-label="Add 5 minutes">+</button>
      </div>

      <!-- Plan comparison -->
      {#if planned}
        <p class="mt-3 flex items-center justify-center gap-1.5 type-meta" style="color: var(--sempa-text-soft);">
          Planned {formatMinutes(planned)}
          {#if delta !== null && delta > 0}
            <span class="chip" style="height: 20px; padding: 0 8px; background: var(--cal-amber-bg); color: var(--cal-amber-fg);">{delta}m over</span>
          {:else if delta !== null && delta < 0}
            <span class="chip" style="height: 20px; padding: 0 8px; background: var(--sempa-success-bg); color: var(--sempa-success);">{-delta}m under</span>
          {/if}
        </p>
      {/if}

      <!-- Actions -->
      <button onclick={save}
        class="btn-primary mt-5 w-full"
        style="height: 46px; font-size: 14.5px;">
        {pc.markDone ? `Log ${minutes}m & mark done` : `Log ${minutes}m`}
      </button>
      <div class="mt-2 flex items-center justify-between text-xs">
        <button onclick={none} class="ghost-link">Didn't really work on it</button>
        <button onclick={cancel} class="ghost-link">Discard</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .ghost-link {
    height: 32px; padding: 0 10px; border-radius: 999px;
    font-size: 12.5px; font-weight: 600; color: var(--sempa-text-dim);
    transition: background-color 120ms ease, color 120ms ease;
  }
  .ghost-link:hover { background: var(--sempa-fill); color: var(--sempa-text-soft); }
</style>
