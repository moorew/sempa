<script lang="ts">
  import { api } from '$lib/api';
  import type { Objective, Task } from '$lib/types';
  import { weekStart } from '$lib/utils';
  import { realtime } from '$lib/stores/realtime.svelte';

  let { date }: { date: string } = $props();

  const ws = $derived(weekStart(date));
  let objectives = $state<Objective[]>([]);
  let tasks      = $state<Task[]>([]);
  let collapsed  = $state(false);

  $effect(() => { ws; void load(); });

  // Reflect changes made elsewhere (e.g. completing a linked task auto-completes
  // its objective on the day board) without a manual refresh.
  $effect(() => {
    const ev = realtime.lastEvent;
    if (!ev) return;
    if (ev.type === 'task:change' || ev.type === 'objective:change') void load();
  });

  // Drag an objective out of the sidebar to create a linked task in a day column.
  function onObjDragStart(e: DragEvent, obj: Objective) {
    e.dataTransfer?.setData('application/x-sempa-objective', JSON.stringify({ id: obj.id, title: obj.title }));
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'copy';
  }

  async function load() {
    try {
      [objectives, tasks] = await Promise.all([
        api.objectives.listByWeek(ws),
        api.tasks.listByWeek(ws),
      ]);
    } catch { /* ignore */ }
  }

  function objTasks(id: string) {
    return tasks.filter(t => t.weekly_objective_id === id && t.status !== 'cancelled');
  }
  function objDone(id: string)  { return objTasks(id).filter(t => t.status === 'done').length; }
  function objTotal(id: string) { return objTasks(id).length; }
  function objPct(id: string)   {
    const t = objTotal(id); return t === 0 ? 0 : Math.round(objDone(id) / t * 100);
  }

  const totalDone = $derived(objectives.filter(o => o.status === 'completed').length);
  const total     = $derived(objectives.length);
  const overallPct = $derived(total === 0 ? 0 : Math.round(totalDone / total * 100));
</script>

{#if total > 0}
  <div style="border-bottom: 1px solid var(--sempa-border);">
    <!-- Header row -->
    <button onclick={() => collapsed = !collapsed}
            class="wow-head flex w-full items-center justify-between px-4 py-3 text-left transition-colors">
      <div class="flex items-center gap-2">
        <span class="type-label" style="color: var(--sempa-text-soft);">
          This week
        </span>
        <span class="tabular-nums" style="font-size: 11.5px; font-weight: 600; color: var(--sempa-text-dim);">
          {totalDone}/{total}
        </span>
      </div>
      <div class="flex items-center gap-2">
        <div class="day-bar-track" style="width: 56px;">
          <div class="day-bar-fill"
               style="width:{overallPct}%; background: {overallPct === 100 ? 'var(--sempa-success)' : 'var(--sempa-accent)'};"></div>
        </div>
        <svg class="h-3.5 w-3.5 transition-transform {collapsed ? '-rotate-90' : ''}" style="color: var(--sempa-text-dim);"
             fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M19 9l-7 7-7-7"/>
        </svg>
      </div>
    </button>

    {#if !collapsed}
      <div class="px-3 pb-3 space-y-2">
        {#each objectives as obj (obj.id)}
          {@const p = objPct(obj.id)}
          {@const done = obj.status === 'completed'}
          {@const total = objTotal(obj.id)}
          {@const full = done || p === 100}
          <a href="/week/{ws}"
             draggable="true"
             ondragstart={(e) => onObjDragStart(e, obj)}
             title="Drag onto a day to add a linked task"
             class="wow-row block cursor-grab px-3 py-2.5 active:cursor-grabbing">
            <div class="flex items-center gap-2.5">
              <!-- Round status check (display only — the row is a link/drag source) -->
              <span class="check-ring" data-state={done ? 'done' : p > 0 ? 'active' : undefined}
                    style="width: 18px; height: 18px; border-width: 2px;" aria-hidden="true">
                {#if done}
                  <svg class="h-2.5 w-2.5" fill="none" stroke="currentColor" stroke-width="3.5" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                  </svg>
                {/if}
              </span>
              <span class="flex-1 truncate {done ? 'line-through' : ''}"
                    style="font-size: 13.5px; font-weight: 600; color: {done ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'};">
                {obj.title}
              </span>
              <span class="shrink-0 tabular-nums"
                    style="font-size: 11.5px; font-weight: 700; color: {full ? 'var(--sempa-success)' : 'var(--sempa-text-soft)'};">
                {p}%
              </span>
            </div>
            <div class="mt-2 flex items-center gap-2 pl-[28px]">
              <div class="day-bar-track flex-1">
                <div class="day-bar-fill"
                     style="width:{p}%; background: {full ? 'var(--sempa-success)' : 'var(--sempa-accent)'};"></div>
              </div>
              <span class="shrink-0 tabular-nums" style="font-size: 11px; color: var(--sempa-text-dim);">
                {total === 0 ? 'No tasks' : `${objDone(obj.id)}/${total}`}
              </span>
            </div>
          </a>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<style>
  .wow-head:hover { background: var(--sempa-fill); }
  /* Nested block inside the panel: a soft fill, an accent edge on hover. */
  .wow-row {
    background: var(--sempa-fill);
    border-radius: 14px;
    box-shadow: inset 0 0 0 1px transparent;
    transition: box-shadow 120ms ease;
  }
  .wow-row:hover { box-shadow: inset 0 0 0 1.5px color-mix(in srgb, var(--sempa-accent) 55%, transparent); }
</style>
