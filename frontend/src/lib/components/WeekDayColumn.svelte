<script lang="ts">
  import type { Task } from '$lib/types';
  import { flip } from 'svelte/animate';
  import TaskCard from './TaskCard.svelte';
  import { compareTasksForDay, today } from '$lib/utils';
  import { goto } from '$app/navigation';
  import { contextMenu } from '$lib/stores/contextMenu.svelte';
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

<div class="flex h-full min-h-0 w-full flex-col"
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

  <!-- Compact header: MON + day-number circle -->
  <div class="mb-2 flex shrink-0 items-center gap-1.5 px-1">
    <span class="text-[10.5px] font-semibold uppercase tracking-wider
                 {isWeekend ? 'text-gray-400 dark:text-gray-600' : 'text-gray-400 dark:text-gray-500'}">
      {dayName}
    </span>
    <!-- Day number — circle only on today -->
    <span class="flex h-5 w-5 items-center justify-center rounded-full text-xs font-[600] leading-none
                 {isToday ? '' : isWeekend ? 'text-gray-400 dark:text-gray-600' : 'text-gray-600 dark:text-gray-300'}"
          style={isToday ? 'background: var(--sempa-today-bg); color: var(--sempa-today-fg);' : ''}>
      {dayNum}
    </span>
    <!-- Task count -->
    {#if tasks.length > 0}
      <span class="ml-auto text-[10.5px] tabular-nums
                   {isWeekend ? 'text-gray-300 dark:text-gray-700' : 'text-gray-400 dark:text-gray-600'}">
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
      <div class="mt-1 flex items-center justify-between text-[10px] tabular-nums">
        {#if dayComplete}
          <span style="color: var(--sempa-success);">{fmtCapacity(workedMins || plannedMins)} done</span>
        {:else if dayStarted}
          <span style="color: var(--sempa-accent);">{fmtCapacity(workedMins)} done</span>
          <span style="color: var(--sempa-text-dim);">{fmtCapacity(Math.max(plannedMins - workedMins, 0))} left</span>
        {:else}
          <span style="color: var(--sempa-text-dim);">{fmtCapacity(plannedMins)} planned</span>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Column body. Fills the column height so its task list scrolls INTERNALLY
       (the board itself only scrolls left/right). The drag-over cue is a soft
       accent tint + gentle inset outline — no bright ring — that eases in. -->
  <div class="drop-zone flex min-h-0 flex-1 flex-col rounded-xl
              {isDragOver
                ? 'is-drag-over'
                : isWeekend
                  ? 'bg-gray-50/40 dark:bg-gray-800/10'
                  : 'bg-gray-100/60 dark:bg-gray-800/25'}">

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

      <!-- Completed tasks (collapsed by default) -->
      {#if done.length > 0}
        <div class="border-t border-gray-100/80 px-2 pb-1 pt-0.5 dark:border-gray-700/30">
        <button onclick={() => showDone = !showDone}
                class="inline-flex items-center gap-1 rounded-full transition-opacity hover:opacity-80"
                style="background: var(--sempa-success-soft); color: var(--sempa-success);
                       font-size: 11.5px; font-weight: 500; padding: 2px 9px;">
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

    <!-- Add task — pinned below the scroll region -->
    <button onclick={() => onAddClick(date)}
            class="flex shrink-0 items-center gap-1.5 rounded-b-xl px-3 py-2 text-xs text-gray-400
                   hover:bg-white/60 hover:text-gray-600 transition-colors
                   dark:text-gray-600 dark:hover:bg-gray-700/30 dark:hover:text-gray-400">
      <Plus size={11} />
      Add task
    </button>
  </div>
</div>

<style>
  /* Drag-over cue: a calm accent tint + a gentle inset outline that eases in —
     not a bright ring, so the cards and the drop position stay easy to read. */
  .drop-zone {
    transition: background-color 200ms ease, box-shadow 200ms ease;
  }
  .drop-zone.is-drag-over {
    background: color-mix(in srgb, var(--sempa-accent) 7%, var(--sempa-bg-panel));
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
</style>
