<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { formatMinutes, today } from '$lib/utils';
  import SubTaskList from '$lib/components/SubTaskList.svelte';
  import RichText from '$lib/components/RichText.svelte';

  let taskId  = $derived(page.params.taskId);
  let task    = $state<Task | null>(null);
  let loading = $state(true);
  let error   = $state<string | null>(null);

  onMount(async () => {
    try { task = await api.tasks.get(taskId!); }
    catch (e) { error = e instanceof Error ? e.message : 'Failed to load task'; }
    finally { loading = false; }
  });

  async function toggleDone() {
    if (!task) return;
    // If a focus session is running on this task, finish through the timer so
    // the time spent gets logged via the confirm sheet (it also marks it done).
    if (task.status !== 'done' && pomodoro.isFor(task.id)) {
      pomodoro.finishTask();
      return;
    }
    const newStatus = task.status === 'done' ? 'planned' : 'done';
    try {
      task = await api.tasks.update(task.id, {
        status: newStatus,
        completed_at: newStatus === 'done' ? new Date().toISOString() : null,
      });
    } catch { /* ignore */ }
  }

  function startPomodoro() {
    if (!task) return;
    pomodoro.start(task.id, task.title, task.time_actual_minutes ?? 0, task.time_estimate_minutes ?? null);
  }

  const isDone       = $derived(task?.status === 'done' || false);
  const isMyPomodoro = $derived(!!task && pomodoro.taskId === task.id);

  // Display-only: the focus ring. Work phases ring in the accent; breaks use
  // the success hue (matching the floating timer). Idle shows logged vs
  // estimate so the ring is never an empty prop.
  const RING_R = 108;
  const RING_C = 2 * Math.PI * RING_R;
  const ringColor = $derived(isMyPomodoro && pomodoro.phase !== 'work' ? 'var(--sempa-success)' : 'var(--sempa-accent)');
  const ringPct = $derived.by(() => {
    if (isMyPomodoro) return pomodoro.progressPct;
    const est = task?.time_estimate_minutes ?? 0;
    const act = task?.time_actual_minutes ?? 0;
    return est > 0 ? Math.min(100, Math.round((act / est) * 100)) : 0;
  });
  const plannedLabel = $derived(task?.planned_date
    ? new Date(task.planned_date + 'T12:00:00').toLocaleDateString('en-US', { weekday: 'long', month: 'short', day: 'numeric' })
    : null);

  // Reflect a logged/finished session on this page's own task copy so it doesn't
  // show a stale open state after the widget's Done flow.
  $effect(() => {
    const upd = pomodoro.lastTimeUpdate;
    if (!upd) return;
    untrack(() => {
      if (!task || task.id !== upd.taskId) return;
      task = {
        ...task,
        time_actual_minutes: upd.newActual,
        ...(upd.done ? { status: 'done' as const, completed_at: task.completed_at ?? new Date().toISOString() } : {}),
      };
    });
  });
</script>

<svelte:head><title>{task?.title ?? 'Focus'} — Sempa</title></svelte:head>

<!-- Back button — in the page flow (top-left of the content area) so it never
     sits on top of the desktop sidebar. -->
<div class="px-4 sm:px-6" style="padding-top: calc(env(safe-area-inset-top, 0px) + 16px);">
  <button onclick={() => history.back()}
          class="btn-surface"
          style="height: 40px; padding: 0 16px 0 12px; font-size: 14px; color: var(--sempa-text-soft);">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" d="M19 12H5m7-7-7 7 7 7"/>
    </svg>
    Back
  </button>
</div>

<div class="flex min-h-full flex-col items-center px-4 pb-28 pt-6 sm:pb-16 sm:pt-4 animate-fade-in">
  {#if loading}
    <div class="flex h-48 items-center justify-center text-sm" style="color: var(--sempa-text-dim);">Loading…</div>

  {:else if error || !task}
    <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{error ?? 'Task not found'}</div>

  {:else}
    <div class="w-full max-w-xl">

      <!-- Task header -->
      <div class="mb-6 flex flex-col items-center text-center">
        <!-- Status row: the round done-check sits beside the date pill, so the
             title below stays truly centred. -->
        <div class="mb-3 flex items-center gap-2">
          <button onclick={toggleDone}
                  class="done-toggle" data-done={isDone ? 'true' : undefined}
                  aria-label={isDone ? 'Mark not done' : 'Mark done'}>
            <span class="check-ring" data-state={isDone ? 'done' : undefined} style="width: 20px; height: 20px;">
              {#if isDone}
                <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
              {/if}
            </span>
            {isDone ? 'Done' : 'Mark done'}
          </button>
          {#if plannedLabel}
            <span class="chip" style="height: 36px; padding: 0 14px; font-size: 12.5px; background: var(--sempa-bg-panel); color: var(--sempa-text-soft); box-shadow: var(--sempa-shadow-1);">
              {plannedLabel}
            </span>
          {/if}
        </div>
        <h1 class="type-page {isDone ? 'line-through opacity-40' : ''}"
            style="color: var(--sempa-text); font-size: clamp(24px, 4vw, 30px); text-wrap: balance;">
          {task.title}
        </h1>
      </div>

      <!-- Timer: a large calm ring -->
      <section class="surface mb-5 px-5 pb-6 pt-7 text-center" style="border-radius: 28px;">
        <div class="focus-ring mx-auto" role="img"
             aria-label={isMyPomodoro ? `${pomodoro.phaseLabel}: ${pomodoro.display} remaining` : `${ringPct}% of estimate logged`}>
          <svg viewBox="0 0 240 240" class="h-full w-full" style="transform: rotate(-90deg);">
            <circle cx="120" cy="120" r={RING_R} fill="none" stroke="var(--sempa-track)" stroke-width="10" />
            <circle cx="120" cy="120" r={RING_R} fill="none" stroke={ringColor}
                    stroke-width="10" stroke-linecap="round"
                    stroke-dasharray={RING_C}
                    stroke-dashoffset={RING_C * (1 - ringPct / 100)}
                    style="transition: stroke-dashoffset 1s linear, stroke 300ms ease; {ringPct === 0 ? 'opacity: 0;' : ''}" />
          </svg>
          <div class="absolute inset-0 flex flex-col items-center justify-center">
            {#if isMyPomodoro}
              <span class="chip mb-2" style="background: {pomodoro.phase === 'work' ? 'var(--sempa-accent-bg)' : 'var(--sempa-success-bg)'};
                                             color: {pomodoro.phase === 'work' ? 'var(--sempa-accent-ink)' : 'var(--sempa-success)'};">
                {pomodoro.phaseLabel}
              </span>
              <span class="font-mono tabular-nums" style="font-size: 48px; font-weight: 700; letter-spacing: -.03em; line-height: 1; color: var(--sempa-text);">
                {pomodoro.display}
              </span>
              <span class="mt-2 type-meta" style="color: var(--sempa-text-dim);">
                {pomodoro.isRunning ? 'In focus' : 'Paused'}
              </span>
            {:else}
              <span class="font-mono tabular-nums" style="font-size: 40px; font-weight: 700; letter-spacing: -.03em; line-height: 1; color: var(--sempa-text);">
                {task.time_actual_minutes ? formatMinutes(task.time_actual_minutes) : '0m'}
              </span>
              <span class="mt-2 type-meta" style="color: var(--sempa-text-soft);">
                {task.time_actual_minutes ? 'logged' : 'No time logged yet'}
              </span>
              {#if task.time_estimate_minutes}
                <span class="chip chip-time mt-2.5">of {formatMinutes(task.time_estimate_minutes)} estimated</span>
              {/if}
            {/if}
          </div>
        </div>

        {#if isMyPomodoro}
          <div class="mt-6 flex items-center justify-center gap-2.5">
            <button onclick={() => pomodoro.togglePause()}
                    class="btn-primary"
                    style="height: 48px; padding: 0 28px; font-size: 15px;">
              {#if pomodoro.isRunning}
                <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path stroke-linecap="round" d="M10 9v6m4-6v6"/>
                </svg>
                Pause
              {:else}
                <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 3l14 9-14 9V3z"/>
                </svg>
                Resume
              {/if}
            </button>
            <button onclick={() => pomodoro.requestStop()}
                    class="btn-surface"
                    style="height: 48px; padding: 0 24px; font-size: 15px; color: var(--sempa-text-soft); background: var(--sempa-fill); box-shadow: none;">
              <svg class="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 24 24"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
              Stop
            </button>
          </div>
        {:else}
          <button onclick={startPomodoro} disabled={isDone}
                  class="btn-primary mx-auto mt-6 disabled:opacity-40"
                  style="height: 48px; padding: 0 28px; font-size: 15px;">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 3l14 9-14 9V3z"/>
            </svg>
            Start Pomodoro
          </button>
        {/if}
      </section>

      <!-- Description -->
      {#if task.description}
        <div class="surface mb-5 px-5 py-4" style="border-radius: 22px;">
          <p class="mb-1.5 type-label" style="color: var(--sempa-text-soft);">Notes</p>
          <div class="text-sm leading-relaxed" style="color: var(--sempa-text-soft);">
            <RichText text={task.description} />
          </div>
        </div>
      {/if}

      <!-- Sub-tasks -->
      <div class="surface p-4 sm:px-5" style="border-radius: 22px;">
        <SubTaskList parentId={task.id} parentDate={task.planned_date ?? undefined} />
      </div>
    </div>
  {/if}
</div>

<style>
  .done-toggle {
    display: inline-flex; align-items: center; gap: 8px;
    height: 36px; padding: 0 14px 0 8px; border-radius: 999px;
    background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1);
    font-size: 12.5px; font-weight: 600; color: var(--sempa-text-soft);
    transition: color 120ms ease, background-color 120ms ease;
  }
  .done-toggle:hover { color: var(--sempa-text); }
  .done-toggle:hover .check-ring:not([data-state="done"]) { border-color: var(--sempa-success); }
  .done-toggle[data-done="true"] { background: var(--sempa-success-bg); color: var(--sempa-success); box-shadow: none; }
  .focus-ring {
    position: relative;
    width: min(240px, 68vw);
    aspect-ratio: 1;
  }
</style>
