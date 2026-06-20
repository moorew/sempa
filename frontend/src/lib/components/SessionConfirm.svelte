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
    <div class="w-full max-w-sm rounded-2xl p-5 shadow-2xl"
      style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: var(--sempa-text);">

      <p class="text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-accent);">
        {pc.markDone ? 'Nice work — log your time' : 'How long did that take?'}
      </p>
      <p class="mt-1 mb-4 truncate text-base font-semibold" title={pc.taskTitle}>{pc.taskTitle}</p>

      <!-- Big editable minutes -->
      <div class="flex items-center justify-center gap-3">
        <button onclick={() => bump(-5)}
          class="flex h-10 w-10 items-center justify-center rounded-full text-lg transition-opacity hover:opacity-80"
          style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);" aria-label="Subtract 5 minutes">−</button>
        <div class="flex items-baseline gap-1">
          <input type="number" min="0" max="600" bind:value={minutes}
            class="w-20 bg-transparent text-center font-mono text-4xl font-bold tabular-nums outline-none"
            style="color: var(--sempa-text);" />
          <span class="text-sm" style="color: var(--sempa-text-dim);">min</span>
        </div>
        <button onclick={() => bump(5)}
          class="flex h-10 w-10 items-center justify-center rounded-full text-lg transition-opacity hover:opacity-80"
          style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);" aria-label="Add 5 minutes">+</button>
      </div>

      <!-- Plan comparison -->
      {#if planned}
        <p class="mt-3 text-center text-xs" style="color: var(--sempa-text-dim);">
          Planned {formatMinutes(planned)}
          {#if delta !== null && delta > 0}
            · <span style="color: #c0392b;">{delta}m over</span>
          {:else if delta !== null && delta < 0}
            · <span style="color: var(--sempa-success);">{-delta}m under</span>
          {/if}
        </p>
      {/if}

      <!-- Actions -->
      <button onclick={save}
        class="mt-5 w-full rounded-xl py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90"
        style="background: var(--sempa-accent);">
        {pc.markDone ? `Log ${minutes}m & mark done` : `Log ${minutes}m`}
      </button>
      <div class="mt-2 flex items-center justify-between text-xs">
        <button onclick={none} class="rounded px-2 py-1.5 transition-opacity hover:opacity-70"
          style="color: var(--sempa-text-dim);">Didn't really work on it</button>
        <button onclick={cancel} class="rounded px-2 py-1.5 transition-opacity hover:opacity-70"
          style="color: var(--sempa-text-dim);">Discard</button>
      </div>
    </div>
  </div>
{/if}
