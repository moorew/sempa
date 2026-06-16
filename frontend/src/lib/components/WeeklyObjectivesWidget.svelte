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
  <div class="border-b border-gray-100 dark:border-gray-800/60">
    <!-- Header row -->
    <button onclick={() => collapsed = !collapsed}
            class="flex w-full items-center justify-between px-4 py-2.5 text-left
                   hover:bg-gray-50/50 dark:hover:bg-gray-800/30 transition-colors">
      <div class="flex items-center gap-2">
        <span class="text-[10.5px] font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-600">
          This week
        </span>
        <span class="text-[10.5px] text-gray-400 dark:text-gray-600">
          {totalDone}/{total}
        </span>
      </div>
      <div class="flex items-center gap-2">
        <div class="h-1 w-14 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
          <div class="h-full rounded-full transition-all"
               style="width:{overallPct}%; background: var(--sempa-accent);"></div>
        </div>
        <svg class="h-3 w-3 text-gray-300 transition-transform dark:text-gray-600 {collapsed ? '-rotate-90' : ''}"
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
          <a href="/week/{ws}"
             draggable="true"
             ondragstart={(e) => onObjDragStart(e, obj)}
             title="Drag onto a day to add a linked task"
             class="block cursor-grab rounded-xl px-3 py-2.5 transition-colors active:cursor-grabbing"
             style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);"
             onmouseenter={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'}
             onmouseleave={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'}>
            <div class="flex items-center gap-2">
              <div class="h-2 w-2 shrink-0 rounded-full"
                   style="background: {done || p === 100 ? 'var(--sempa-success)' : 'var(--sempa-accent)'};"></div>
              <span class="flex-1 truncate text-[13px] {done ? 'line-through' : ''}"
                    style="color: {done ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'};">
                {obj.title}
              </span>
              <span class="shrink-0 text-[11px] font-medium tabular-nums"
                    style="color: {done || p === 100 ? 'var(--sempa-success)' : 'var(--sempa-text-dim)'};">
                {p}%
              </span>
            </div>
            <div class="mt-2 flex items-center gap-2">
              <div class="h-1 flex-1 overflow-hidden rounded-full" style="background: var(--sempa-border);">
                <div style="width:{p}%; height:100%; border-radius:9999px;
                            background: {done || p === 100 ? 'var(--sempa-success)' : 'var(--sempa-accent)'};
                            transition: width 400ms ease-out;"></div>
              </div>
              <span class="shrink-0 text-[10px]" style="color: var(--sempa-text-dim);">
                {total === 0 ? 'No tasks' : `${objDone(obj.id)}/${total}`}
              </span>
            </div>
          </a>
        {/each}
      </div>
    {/if}
  </div>
{/if}
