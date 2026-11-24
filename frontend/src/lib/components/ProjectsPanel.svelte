<script lang="ts">
  import { tick, untrack } from 'svelte';
  import { api } from '$lib/api';
  import type { Project, Task } from '$lib/types';
  import { formatShortDate } from '$lib/utils';
  import { projectProgress, relativeDue } from '$lib/projectProgress';
  import { ChevronRight, Plus } from 'lucide-svelte';

  // Day-board side panel: each open project with the work that isn't on a day
  // yet. Drag a task onto a day column to plan it (same hand-off as the Jira
  // panel: set the board's draggingId; the board fetches the task on drop).
  let {
    projects,
    projectTasks,
    today,
    focusId = null,
    onTaskDragStart,
    onTaskClick,
    onChanged,
  }: {
    projects: Project[];
    projectTasks: Record<string, Task[]>;
    today: string;
    focusId?: string | null;
    onTaskDragStart: (id: string) => void;
    onTaskClick: (t: Task) => void;
    onChanged: () => void;
  } = $props();

  let collapsed = $state<Record<string, boolean>>({});
  let adding = $state<string | null>(null);
  let newTitle = $state('');
  let list = $state<HTMLElement | undefined>();

  // A bar clicked on the timeline: expand that project and bring it into view.
  $effect(() => {
    const id = focusId;
    if (!id) return;
    untrack(() => { collapsed[id] = false; });
    tick().then(() => list?.querySelector(`[data-project="${id}"]`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }));
  });

  const open = (t: Task) => t.status !== 'done' && t.status !== 'cancelled';

  function onDragStart(e: DragEvent, t: Task) {
    onTaskDragStart(t.id);
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move';
  }

  async function addTask(p: Project) {
    const title = newTitle.trim();
    if (!title) { adding = null; return; }
    newTitle = '';
    try {
      await api.tasks.create({ title, status: 'backlog', project_id: p.id, shared: p.shared });
      onChanged();
    } catch { /* ignore */ }
  }
</script>

<div bind:this={list} class="h-full overflow-y-auto px-2.5 pb-3 text-[13px]">
  {#if projects.length === 0}
    <div class="flex h-32 flex-col items-center justify-center gap-2 text-center">
      <p style="color: var(--sempa-text-dim);">No open projects</p>
      <a href="/projects" class="btn-soft h-8 px-3.5 text-[12.5px]">Go to projects</a>
    </div>
  {:else}
    {#each projects as p (p.id)}
      {@const all = projectTasks[p.id] ?? []}
      {@const toPlan = all.filter((t) => open(t) && !t.planned_date)}
      {@const upcoming = all.filter((t) => open(t) && t.planned_date && t.planned_date >= today)
                            .sort((a, b) => (a.planned_date! < b.planned_date! ? -1 : 1))}
      {@const prog = projectProgress(all, p.target_date, today)}
      {@const isOpen = !collapsed[p.id]}
      <section data-project={p.id} class="mb-2 rounded-[18px] p-1"
               style="background: color-mix(in srgb, {p.color} {focusId === p.id ? 14 : 8}%, transparent);">
        <div class="flex items-center gap-1.5 px-1.5 pt-1">
          <button type="button" class="flex min-w-0 flex-1 items-center gap-1.5 py-1 text-left"
                  aria-expanded={isOpen} onclick={() => (collapsed[p.id] = isOpen)}>
            <span class="shrink-0 transition-transform" style="transform: rotate({isOpen ? 90 : 0}deg); color: var(--sempa-text-dim);">
              <ChevronRight size={14} />
            </span>
            <span class="shrink-0" style="width: 9px; height: 9px; border-radius: 2px; background: {p.color};"></span>
            <span class="truncate font-bold" style="color: var(--sempa-text);">{p.title || 'Untitled project'}</span>
          </button>
          <a href="/projects/{p.id}" class="shrink-0 rounded-full px-2 py-0.5 text-[11.5px] font-bold"
             style="color: var(--sempa-accent-ink);">Open</a>
        </div>
        <div class="flex items-center gap-2 px-2 pb-1.5 text-[11.5px]" style="color: var(--sempa-text-dim);">
          <div class="day-bar-track flex-1" style="height: 4px;">
            <div style="height: 100%; width: {prog.pct}%; background: {p.color};"></div>
          </div>
          <span class="shrink-0">{prog.done}/{prog.total}</span>
          {#if p.target_date}
            <span class="shrink-0" style={prog.overdue ? 'color: #c0392b; font-weight: 700;' : ''}>{relativeDue(p.target_date, today)}</span>
          {/if}
        </div>

        {#if isOpen}
          {#if toPlan.length}
            <p class="px-2 pb-0.5 pt-1 text-[10.5px] font-bold uppercase tracking-wide" style="color: var(--sempa-text-dim);">
              To plan · drag onto a day
            </p>
          {/if}
          <ul>
            {#each toPlan as t (t.id)}
              <li class="proj-row flex cursor-grab items-center gap-2 rounded-[12px] px-2 py-1.5 active:cursor-grabbing"
                  draggable="true" ondragstart={(e) => onDragStart(e, t)}>
                <button type="button" class="min-w-0 flex-1 truncate text-left" style="color: var(--sempa-text);"
                        onclick={() => onTaskClick(t)}>{t.title || 'Untitled task'}</button>
              </li>
            {/each}
            {#each upcoming.slice(0, 3) as t (t.id)}
              <li class="proj-row flex items-center gap-2 rounded-[12px] px-2 py-1.5">
                <button type="button" class="min-w-0 flex-1 truncate text-left" style="color: var(--sempa-text-soft);"
                        onclick={() => onTaskClick(t)}>{t.title || 'Untitled task'}</button>
                <span class="shrink-0 text-[11px]" style="color: var(--sempa-text-dim);">{formatShortDate(t.planned_date!)}</span>
              </li>
            {/each}
          </ul>

          {#if adding === p.id}
            <form class="px-1 pb-1 pt-0.5" onsubmit={(e) => { e.preventDefault(); addTask(p); }}>
              <!-- svelte-ignore a11y_autofocus -->
              <input bind:value={newTitle} autofocus placeholder="Task title"
                     class="h-8 w-full rounded-[10px] px-2.5 text-[13px] outline-none"
                     style="background: var(--sempa-bg-panel); color: var(--sempa-text); border: 1px solid var(--sempa-border);"
                     onblur={() => addTask(p)}
                     onkeydown={(e) => { if (e.key === 'Escape') { newTitle = ''; adding = null; } }} />
            </form>
          {:else}
            <button type="button" class="flex items-center gap-1 px-2 py-1.5 text-[12px] font-semibold"
                    style="color: var(--sempa-text-dim);"
                    onclick={() => { adding = p.id; newTitle = ''; }}>
              <Plus size={13} /> Add task
            </button>
          {/if}
        {/if}
      </section>
    {/each}
  {/if}
</div>

<style>
  .proj-row:hover { background: color-mix(in srgb, var(--sempa-text) 5%, transparent); }
</style>
