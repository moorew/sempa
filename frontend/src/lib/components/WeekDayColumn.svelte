<script lang="ts">
  import type { Task } from '$lib/types';
  import { flip } from 'svelte/animate';
  import TaskCard from './TaskCard.svelte';
  import { compareTasksForDay, today } from '$lib/utils';
  import { goto } from '$app/navigation';
  import { contextMenu } from '$lib/stores/contextMenu.svelte';
  import { timeTracking } from '$lib/stores/timeTracking.svelte';
  import { plannedMinutes } from '$lib/dayCapacity';
  import { Plus } from 'lucide-svelte';

  let {
    date,
    dayName,     // "Mon"
    dayNum,      // "3"
    isToday,
    isWeekend,
    tasks,       // all non-cancelled tasks for this day
    isDragOver,
    onTaskDragStart,
    onTaskFocusClick,
    onTaskFocusMode,
    onTaskComplete,
    onTaskTrash,
    onTaskClick,
    onTaskHover,
    onDrop,
    onEmailDrop,
    onObjectiveDrop,
    onDragOver,
    onDragLeave,
    onAddClick,
  }: {
    date: string; dayName: string; dayNum: string;
    isToday: boolean; isWeekend: boolean;
    tasks: Task[]; isDragOver: boolean;
    onTaskDragStart: (id: string) => void;
    onTaskFocusClick?: (id: string, title: string) => void;
    onTaskFocusMode?: (id: string) => void;
    onTaskComplete?: (id: string) => void;
    onTaskTrash?: (id: string, title: string) => void;
    onTaskClick?: (task: Task) => void;
    onTaskHover?: (id: string | null) => void;
    onDrop: (date: string, insertIndex?: number) => void;
    onEmailDrop?: (emailData: { id: string; subject: string }, date: string) => void;
    onObjectiveDrop?: (objData: { id: string; title: string }, date: string) => void;
    onDragOver: (date: string) => void;
    onDragLeave: () => void;
    onAddClick: (date: string) => void;
  } = $props();

  const active = $derived(tasks.filter(t => t.status !== 'done').sort(compareTasksForDay));
  const done   = $derived(tasks.filter(t => t.status === 'done').sort(compareTasksForDay));
  let showDone = $state(false);

  // ── Day progress: one clear metric — time worked vs time planned ──────────
  // Planned = sum of estimates; worked = sum of logged actuals. The bar fills
  // worked/planned. Three states drive the labels below (unstarted / in
  // progress / complete) so there's never a green+amber mix.
  const active2     = $derived(tasks.filter(t => t.status !== 'cancelled'));
  const plannedMins = $derived(active2.reduce((s, t) => s + (t.time_estimate_minutes ?? 0), 0));
  const workedMins  = $derived(active2.reduce((s, t) => s + (t.time_actual_minutes  ?? 0), 0));
  const dayComplete = $derived(active2.length > 0 && active2.every(t => t.status === 'done'));
  const dayStarted  = $derived(!dayComplete && workedMins > 0);
  const barPct      = $derived(
    dayComplete ? 100 : plannedMins === 0 ? 0 : Math.min((workedMins / plannedMins) * 100, 100)
  );
  // Subtle over-capacity signal: planned time (calibrated by history when
  // realistic mode is on) exceeds the user's daily limit.
  const effectivePlanned = $derived(plannedMinutes(tasks, undefined, timeTracking.capacityRealistic));
  const overCapacity = $derived(
    timeTracking.capacityEnabled && !dayComplete && effectivePlanned > timeTracking.capacityMinutes
  );

  function fmtCapacity(mins: number): string {
    const h = Math.floor(mins / 60);
    const m = mins % 60;
    if (h === 0) return `${m}m`;
    if (m === 0) return `${h}h`;
    return `${h}h ${m}m`;
  }

  let taskListEl = $state<HTMLElement | undefined>();
  let insertIdx  = $state<number | null>(null);

  function calcInsertIdx(e: DragEvent): number {
    if (!taskListEl) return active.length;
    const els = Array.from(taskListEl.querySelectorAll('[data-task-idx]')) as HTMLElement[];
    for (let i = 0; i < els.length; i++) {
      const rect = els[i].getBoundingClientRect();
      if (e.clientY < rect.top + rect.height / 2) return i;
    }
    return active.length;
  }

  // Right-click empty column space → quick board actions. Right-clicks on a task
  // card are handled by the card itself (it stops propagation), so this only
  // fires for the day's background.
  function onColumnContext(e: MouseEvent) {
    contextMenu.show(e, [
      { label: 'New task here', onClick: () => onAddClick(date) },
      { label: 'Go to today', onClick: () => goto(`/day/${today()}`) },
      { label: 'Search', onClick: () => goto('/search') },
    ]);
  }
</script>

<div class="group flex h-full min-h-0 w-full flex-col"
     oncontextmenu={onColumnContext}
     ondragover={(e) => { e.preventDefault(); insertIdx = calcInsertIdx(e); onDragOver(date); }}
     ondragleave={(e) => {
       if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
         insertIdx = null; onDragLeave();
       }
     }}
     ondrop={(e) => {
       e.preventDefault();
       const emailData = e.dataTransfer?.getData('application/x-sempa-email');
       const objData   = e.dataTransfer?.getData('application/x-sempa-objective');
       if (objData) {
         try { onObjectiveDrop?.(JSON.parse(objData), date); } catch {}
       } else if (emailData) {
         try { onEmailDrop?.(JSON.parse(emailData), date); } catch {}
       } else {
         onDrop(date, insertIdx ?? undefined);
       }
       insertIdx = null;
     }}>

  <!-- Pill header: day name + number. Today is the one solid pill (btn-bg). -->
  <div class="mb-2 flex shrink-0 items-center gap-2 px-0.5">
    <span class="day-pill" class:is-today={isToday} class:is-weekend={isWeekend && !isToday}>
      <span class="day-name">{dayName}</span>
      <span class="day-num tabular-nums">{dayNum}</span>
    </span>
    <!-- Task count -->
    {#if tasks.length > 0}
      <span class="ml-auto pr-1 tabular-nums"
            style="font-size: 11.5px; font-weight: 600; color: {done.length === tasks.length ? 'var(--sempa-success)' : 'var(--sempa-text-dim)'};">
        {done.length}/{tasks.length}
      </span>
    {/if}
  </div>

  <!-- Day progress bar — single metric: time worked vs time planned -->
  {#if plannedMins > 0}
    <div class="mb-2 shrink-0 px-1">
      <div class="day-bar-track">
        <div class="day-bar-fill"
             style="width: {barPct}%; background: {dayComplete ? 'var(--sempa-success)' : 'var(--sempa-accent)'};"></div>
      </div>
      <div class="mt-1.5 flex items-center justify-between tabular-nums" style="font-size: 11px; font-weight: 500;">
        {#if dayComplete}
          <span style="color: var(--sempa-success);">{fmtCapacity(workedMins || plannedMins)} done</span>
        {:else if dayStarted}
          <span style="color: var(--sempa-accent-ink);">{fmtCapacity(workedMins)} done</span>
          <span style="color: {overCapacity ? 'var(--cal-amber-fg)' : 'var(--sempa-text-dim)'};">{fmtCapacity(Math.max(plannedMins - workedMins, 0))} left</span>
        {:else}
          <span style="color: {overCapacity ? 'var(--cal-amber-fg)' : 'var(--sempa-text-dim)'};"
            title={overCapacity ? `${timeTracking.capacityRealistic ? `Realistically ~${fmtCapacity(effectivePlanned)} — ` : ''}over your ${fmtCapacity(timeTracking.capacityMinutes)} day` : undefined}>
            {fmtCapacity(plannedMins)} planned{#if overCapacity} · over{/if}
          </span>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Column body. Fills the column height so its task list scrolls INTERNALLY
       (the board itself only scrolls left/right). It sits slightly recessed
       into the canvas so the raised task cards read as cards on paper. The
       drag-over cue is a soft accent tint + gentle inset outline that eases in. -->
  <div class="drop-zone flex min-h-0 flex-1 flex-col
              {isDragOver ? 'is-drag-over' : isWeekend ? 'is-weekend' : ''}">

    <!-- Scrollable region: active list + (optional) completed. Add-task stays
         pinned below. Scrollbar is hidden globally; only this area scrolls. -->
    <div class="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div role="list" bind:this={taskListEl} class="flex flex-col gap-2 p-2">
        {#each active as task, i (task.id)}
          <div data-task-idx={i} animate:flip={{ duration: 180 }}>
            {#if isDragOver && insertIdx === i}
              <div class="drop-line mx-1 mb-2" style="background:var(--sempa-accent);"></div>
            {/if}
            <TaskCard {task} accent="bg-gray-400"
                     onDragStart={onTaskDragStart}
                     onFocusClick={onTaskFocusClick}
                     onFocusMode={onTaskFocusMode}
                     onComplete={onTaskComplete}
                     onTrash={onTaskTrash}
                     onHover={onTaskHover}
                     onClick={onTaskClick} />
          </div>
        {/each}

        {#if isDragOver && insertIdx === active.length}
          <div class="drop-line mx-1" style="background:var(--sempa-accent);"></div>
        {/if}
        {#if active.length === 0 && !isDragOver}
          <div class="min-h-[80px]"></div>
        {/if}
      </div>

      <!-- Add task — a small pill that flows directly under the last card (and
           scrolls with the list), so it's never pinned to the screen's bottom
           edge. Revealed on column hover / keyboard focus on hover-capable
           devices; always shown on touch. Opens a new task on THIS day. -->
      <button onclick={() => onAddClick(date)} title="Add a task on this day"
              class="add-pill mb-2 ml-2 mt-0.5 flex w-fit shrink-0 items-center gap-1 rounded-full
                     transition-all"
              style="color: var(--sempa-accent-ink); background: var(--sempa-accent-bg);
                     height: 28px; padding: 0 12px; font-size: 12px; font-weight: 600;">
        <Plus size={12} strokeWidth={2.5} />
        Add task
      </button>

      <!-- Completed tasks (collapsed by default) -->
      {#if done.length > 0}
        <div class="px-2 pb-2 pt-1">
        <button onclick={() => showDone = !showDone}
                class="inline-flex items-center gap-1 rounded-full transition-opacity hover:opacity-80"
                style="background: var(--sempa-success-bg); color: var(--sempa-success);
                       font-size: 12px; font-weight: 600; height: 26px; padding: 0 11px;">
          <svg class="h-3 w-3 transition-transform {showDone ? 'rotate-180' : ''}"
               fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M19 9l-7 7-7-7"/>
          </svg>
          {done.length} done
        </button>
        {#if showDone}
          <div class="flex flex-col gap-1.5 pt-1 pb-1">
            {#each done as task (task.id)}
              <TaskCard {task} accent="bg-green-400"
                       onDragStart={onTaskDragStart}
                       onComplete={onTaskComplete}
                       onTrash={onTaskTrash}
                       onClick={onTaskClick} />
            {/each}
          </div>
        {/if}
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  /* Pill day header: soft fill pill; today is the single solid btn-bg pill. */
  .day-pill {
    display: inline-flex; align-items: baseline; gap: 6px;
    height: 28px; padding: 0 12px; border-radius: 999px;
    line-height: 28px; white-space: nowrap;
    background: color-mix(in srgb, var(--sempa-text) 5%, var(--sempa-bg-main));
    color: var(--sempa-text-soft);
  }
  .day-pill .day-name { font-size: 12px; font-weight: 600; }
  .day-pill .day-num  { font-size: 13px; font-weight: 800; letter-spacing: -.01em; color: var(--sempa-text); }
  .day-pill.is-weekend { background: transparent; box-shadow: inset 0 0 0 1px var(--sempa-border); }
  .day-pill.is-weekend .day-num { color: var(--sempa-text-soft); }
  .day-pill.is-today {
    background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);
    box-shadow: var(--sempa-shadow-accent);
  }
  .day-pill.is-today .day-name { color: var(--sempa-btn-fg); opacity: .85; }
  .day-pill.is-today .day-num  { color: var(--sempa-btn-fg); }

  /* Column well: recessed slightly into the canvas (darker in light themes,
     deeper in dark ones) so the raised task cards sit on it like paper. */
  .drop-zone {
    border-radius: 20px;
    background: color-mix(in srgb, var(--sempa-text) 4.5%, var(--sempa-bg-main));
    transition: background-color 200ms ease, box-shadow 200ms ease;
  }
  .drop-zone.is-weekend {
    background: color-mix(in srgb, var(--sempa-text) 2.5%, var(--sempa-bg-main));
  }
  .drop-zone.is-drag-over {
    background: color-mix(in srgb, var(--sempa-accent) 9%, var(--sempa-bg-main));
    box-shadow: inset 0 0 0 1.5px color-mix(in srgb, var(--sempa-accent) 45%, transparent);
  }
  /* Where the card will land — a soft, clearly visible pill. */
  .drop-line {
    height: 3px;
    border-radius: 9999px;
    box-shadow: 0 0 0 1px color-mix(in srgb, var(--sempa-accent) 25%, transparent);
    animation: drop-line-in 140ms ease;
  }
  @keyframes drop-line-in {
    from { opacity: 0; transform: scaleX(0.6); }
    to   { opacity: 1; transform: scaleX(1); }
  }
  @media (prefers-reduced-motion: reduce) {
    .drop-zone { transition: none; }
    .drop-line { animation: none; }
  }

  /* On hover-capable devices the add-task pill stays out of the way until you
     hover the column (or tab to it); touch devices have no hover, so it stays
     visible there. `.group` is the column root's class. */
  @media (hover: hover) {
    .add-pill {
      opacity: 0;
      transform: translateY(2px);
    }
    .group:hover .add-pill,
    .add-pill:focus-visible {
      opacity: 1;
      transform: translateY(0);
    }
  }
  .add-pill:hover {
    filter: brightness(0.97);
  }
  @media (prefers-reduced-motion: reduce) {
    .add-pill { transition: opacity 120ms ease; transform: none; }
  }
</style>
