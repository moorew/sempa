<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import type { Project, Task, Objective, List } from '$lib/types';
  import { realtime } from '$lib/stores/realtime.svelte';
  import { household } from '$lib/stores/household.svelte';
  import { confirmDialog } from '$lib/stores/confirmDialog.svelte';
  import { projectStore, PROJECT_COLORS } from '$lib/stores/projects.svelte';
  import { projectProgress, relativeDue } from '$lib/projectProgress';
  import { today, weekStart, formatShortDate, formatMinutes } from '$lib/utils';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import RichText from '$lib/components/RichText.svelte';
  import AttachmentList from '$lib/components/AttachmentList.svelte';
  import SempaSelect from '$lib/components/ui/SempaSelect.svelte';
  import SempaDatePicker from '$lib/components/ui/SempaDatePicker.svelte';
  import { Trash2, Plus, Target, ListChecks, Check, X, Pencil, Link2 } from 'lucide-svelte';

  const projectId = $derived(page.params.id ?? '');
  const todayStr = today();
  const thisWeek = weekStart(todayStr);

  let project = $state<Project | null>(null);
  let tasks = $state<Task[]>([]);
  let objectives = $state<Objective[]>([]);
  let lists = $state<List[]>([]);
  let allLists = $state<List[]>([]);
  let loading = $state(true);

  let editingDesc = $state(false);
  let descDraft = $state('');
  let colorOpen = $state(false);
  let showDone = $state(false);
  let newTask = $state('');
  let newObjective = $state('');
  let newList = $state('');
  let linkListId = $state<string | null>(null);

  let panelOpen = $state(false);
  let panelTask = $state<Task | null>(null);

  async function load() {
    try {
      const [p, t, o, l, all] = await Promise.all([
        api.projects.get(projectId),
        api.projects.tasks(projectId),
        api.projects.objectives(projectId),
        api.lists.list(undefined, false, projectId),
        api.lists.list(),
      ]);
      project = p; tasks = t; objectives = o; lists = l; allLists = all;
    } catch { project = null; }
    finally { loading = false; }
  }
  onMount(() => { void household.ensure(); projectStore.load(); });
  $effect(() => { void realtime.lastEvent; void projectId; load(); });

  const progress = $derived(projectProgress(tasks, project?.target_date, todayStr));
  const statusOptions = $derived(projectStore.statuses.map((s) => ({ value: s.id, label: s.name, color: s.color })));
  const thisWeekObjectives = $derived(objectives.filter((o) => o.week_start === thisWeek));

  // Open work: unscheduled first ("Up next"), then by week of the planned date.
  const openTasks = $derived(tasks.filter((t) => t.status !== 'done'));
  const doneTasks = $derived(tasks.filter((t) => t.status === 'done'));
  const taskGroups = $derived.by(() => {
    const unscheduled = openTasks.filter((t) => !t.planned_date);
    const byWeek = new Map<string, Task[]>();
    for (const t of openTasks) {
      if (!t.planned_date) continue;
      const ws = weekStart(t.planned_date);
      if (!byWeek.has(ws)) byWeek.set(ws, []);
      byWeek.get(ws)!.push(t);
    }
    const weeks = [...byWeek.entries()].sort(([a], [b]) => a.localeCompare(b));
    return [
      ...(unscheduled.length ? [{ key: 'none', label: 'Up next', tasks: unscheduled }] : []),
      ...weeks.map(([ws, ts]) => ({ key: ws, label: weekLabel(ws), tasks: ts })),
    ];
  });

  // Objectives across all weeks, newest week first.
  const objectiveWeeks = $derived.by(() => {
    const byWeek = new Map<string, Objective[]>();
    for (const o of objectives) {
      if (!byWeek.has(o.week_start)) byWeek.set(o.week_start, []);
      byWeek.get(o.week_start)!.push(o);
    }
    return [...byWeek.entries()].sort(([a], [b]) => b.localeCompare(a));
  });

  const linkableLists = $derived(allLists.filter((l) => !l.project_id));

  function weekLabel(ws: string): string {
    if (ws === thisWeek) return 'This week';
    if (ws < thisWeek) return `Week of ${formatShortDate(ws)} (past)`;
    return `Week of ${formatShortDate(ws)}`;
  }

  async function patch(p: Parameters<typeof api.projects.update>[1]) {
    if (!project) return;
    try {
      project = await api.projects.update(project.id, p);
      projectStore.reload();
    } catch { /* ignore */ }
  }

  function rename(title: string) {
    if (project && title !== project.title) patch({ title });
  }
  function saveDesc() {
    editingDesc = false;
    const d = descDraft.trim();
    if (project && d !== (project.description ?? '')) patch({ description: d || null });
  }

  async function addTask() {
    const title = newTask.trim();
    if (!title || !project) return;
    newTask = '';
    try {
      const t = await api.tasks.create({ title, status: 'backlog', project_id: project.id, shared: project.shared });
      tasks = [...tasks, t];
    } catch { /* ignore */ }
  }
  async function toggleDone(t: Task) {
    const status = t.status === 'done' ? (t.planned_date ? 'planned' : 'backlog') : 'done';
    tasks = tasks.map((x) => (x.id === t.id ? { ...x, status } : x));
    try {
      await api.tasks.update(t.id, { status, completed_at: status === 'done' ? new Date().toISOString() : null });
    } catch { load(); }
  }
  function openTask(t: Task) { panelTask = t; panelOpen = true; }
  function onPanelSave(saved: Task) {
    panelOpen = false;
    // Moved out of the project, deleted, or a sub-task → drop it from view.
    if (saved.status === 'cancelled' || saved.project_id !== projectId) {
      tasks = tasks.filter((t) => t.id !== saved.id);
      return;
    }
    tasks = tasks.some((t) => t.id === saved.id) ? tasks.map((t) => (t.id === saved.id ? saved : t)) : [...tasks, saved];
  }

  async function addObjective() {
    const title = newObjective.trim();
    if (!title || !project) return;
    newObjective = '';
    try {
      const o = await api.objectives.create({ week_start: thisWeek, title, project_id: project.id });
      objectives = [...objectives, o];
    } catch { /* ignore */ }
  }

  async function createList() {
    const name = newList.trim();
    if (!name || !project) return;
    newList = '';
    try { const l = await api.lists.create({ name, project_id: project.id }); lists = [...lists, l]; } catch { /* ignore */ }
  }
  async function linkList(id: string | null) {
    if (!id || !project) return;
    linkListId = null;
    try { await api.lists.update(id, { project_id: project.id }); await load(); } catch { /* ignore */ }
  }
  async function unlinkList(l: List) {
    lists = lists.filter((x) => x.id !== l.id);
    try { await api.lists.update(l.id, { project_id: '' }); } catch { load(); }
  }

  async function remove() {
    if (!project) return;
    const ok = await confirmDialog.ask({
      title: `Delete “${project.title || 'this project'}”?`,
      message: 'Its tasks, objectives and lists are kept — they just stop belonging to a project.',
      confirmLabel: 'Delete project',
      danger: true,
    });
    if (!ok) return;
    try { await api.projects.delete(project.id); await projectStore.reload(); goto('/projects'); } catch { /* ignore */ }
  }
</script>

<svelte:head><title>{project?.title || 'Project'} — Sempa</title></svelte:head>

<div class="mx-auto flex h-full max-w-3xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <div class="flex items-center gap-3 px-5 pb-1 pt-5">
    <a href="/projects" class="btn-surface h-9 gap-1.5 pl-3 pr-4 text-[13px]">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" d="M19 12H5m7-7-7 7 7 7"/></svg>
      Projects
    </a>
    {#if project}
      <button onclick={remove} aria-label="Delete project" title="Delete project"
        class="btn-icon ml-auto" style="width: 36px; height: 36px;">
        <Trash2 size={16} />
      </button>
    {/if}
  </div>

  <div class="flex-1 overflow-y-auto px-5 pb-24 pt-4">
    {#if loading && !project}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else if !project}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Project not found.</p>
    {:else}
      <!-- Header block: a soft tint in the project's own colour carrying the
           title, colour, progress and dates. -->
      <div class="proj-head tint mb-5" style="--tint: {project.color};">
        <svg class="head-mark" viewBox="0 0 100 100" fill="none" aria-hidden="true">
          <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="10" stroke-linecap="round" />
          <circle cx="50" cy="35" r="7.5" fill="currentColor" />
        </svg>

        <!-- Title + colour -->
        <div class="relative flex items-center gap-3 pr-14">
          <button onclick={() => (colorOpen = !colorOpen)} aria-label="Project colour"
            class="h-6 w-6 shrink-0 rounded-full transition-transform hover:scale-110"
            style="background: {project.color}; box-shadow: 0 0 0 3px var(--sempa-bg-panel);"></button>
          {#if colorOpen}
            <div class="absolute left-0 top-9 z-20 flex gap-1.5 rounded-full p-2"
                 style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-2);">
              {#each PROJECT_COLORS as c}
                <button aria-label={`Colour ${c}`} onclick={() => { colorOpen = false; patch({ color: c }); }}
                  class="h-7 w-7 rounded-full transition-transform hover:scale-110"
                  style="background: {c}; {c === project.color ? 'outline: 2px solid var(--sempa-text); outline-offset: 2px;' : ''}"></button>
              {/each}
            </div>
          {/if}
          <input value={project.title} onblur={(e) => rename((e.currentTarget as HTMLInputElement).value.trim())}
            onkeydown={(e) => { if (e.key === 'Enter') (e.currentTarget as HTMLInputElement).blur(); }}
            placeholder="Project name"
            class="type-page min-w-0 flex-1 bg-transparent outline-none" style="color: var(--sempa-text);" />
        </div>

        <!-- Progress -->
        <div class="mt-5">
          <div class="type-meta flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <span class="font-extrabold" style="font-size: 15px;">{progress.pct}%</span>
            <span style="color: var(--sempa-text-soft);">{progress.done} of {progress.total} tasks done</span>
            {#if progress.estimateMinutes}<span class="font-mono text-[11.5px]" style="color: var(--sempa-text-soft);">~{formatMinutes(progress.estimateMinutes)} estimated</span>{/if}
            {#if progress.actualMinutes}<span class="font-mono text-[11.5px]" style="color: var(--sempa-text-soft);">{formatMinutes(progress.actualMinutes)} tracked</span>{/if}
            {#if project.target_date}
              <span class="ml-auto font-semibold" style={progress.overdue ? 'color: var(--sempa-danger, #b3392e);' : ''}>
                {relativeDue(project.target_date, todayStr)}
              </span>
            {/if}
          </div>
          <div class="head-track mt-2.5">
            <div class="head-fill" style="width: {progress.pct}%;"></div>
          </div>
        </div>

        <!-- Status + dates (inset controls on the tint) -->
        <div class="head-fields mt-5 grid gap-3 sm:grid-cols-3">
          <div>
            <label class="type-label mb-1 block" style="color: var(--sempa-text-soft);" for="proj-status">Status</label>
            <SempaSelect id="proj-status" value={project.status_id} options={statusOptions}
              onchange={(v) => { if (typeof v === 'string') patch({ status_id: v }); }} />
          </div>
          <div>
            <label class="type-label mb-1 block" style="color: var(--sempa-text-soft);" for="proj-start">Start</label>
            <SempaDatePicker id="proj-start" value={project.start_date ?? ''} placeholder="No start date"
              onchange={(v) => patch({ start_date: v })} />
          </div>
          <div>
            <label class="type-label mb-1 block" style="color: var(--sempa-text-soft);" for="proj-target">Target</label>
            <SempaDatePicker id="proj-target" value={project.target_date ?? ''} placeholder="No target date"
              onchange={(v) => patch({ target_date: v })} />
          </div>
        </div>
      </div>

      <!-- Description -->
      <section class="surface mb-4 px-5 py-4">
        <div class="mb-2 flex items-center justify-between">
          <h2 class="sec-title">Description</h2>
          {#if !editingDesc}
            <button onclick={() => { descDraft = project?.description ?? ''; editingDesc = true; }}
              class="btn-soft h-8 gap-1 px-3 text-[12px]">
              <Pencil size={12} /> Edit
            </button>
          {/if}
        </div>
        {#if editingDesc}
          <!-- svelte-ignore a11y_autofocus -->
          <textarea bind:value={descDraft} onblur={saveDesc} rows="6" autofocus
            placeholder="What is this project, why does it matter, what does done look like?"
            class="w-full rounded-xl px-3 py-2.5 text-sm outline-none"
            style="border: 1px solid var(--sempa-border); background: var(--sempa-fill); color: var(--sempa-text);"></textarea>
        {:else if project.description}
          <div class="text-sm leading-relaxed" style="color: var(--sempa-text);"><RichText text={project.description} /></div>
        {:else}
          <p class="text-sm" style="color: var(--sempa-text-dim);">No description yet.</p>
        {/if}
      </section>

      <!-- Sharing (multi-user household only) -->
      {#if household.multiUser}
        <div class="surface mb-4 flex items-center justify-between gap-3 px-5 py-3.5">
          <div class="min-w-0">
            <p class="text-[13.5px] font-semibold" style="color: var(--sempa-text);">{project.shared ? 'Shared' : 'Private'}</p>
            <p class="type-meta" style="color: var(--sempa-text-soft);">
              {project.shared ? 'Everyone can see this project. Your private tasks in it stay private.' : 'Only you can see this project.'}
            </p>
          </div>
          <button type="button" role="switch" aria-checked={project.shared} aria-label="Share project"
                  onclick={() => patch({ shared: !project?.shared })}
                  class="relative h-[26px] w-[44px] shrink-0 rounded-full transition-colors"
                  style="background: {project.shared ? 'var(--sempa-btn-bg)' : 'var(--sempa-track)'};">
            <span class="absolute top-[3px] h-[20px] w-[20px] rounded-full transition-all"
                  style="left: {project.shared ? '21px' : '3px'}; background: var(--sempa-bg-panel); box-shadow: 0 1px 2px rgba(0,0,0,.2);"></span>
          </button>
        </div>
      {/if}

      <!-- This week -->
      <section class="surface mb-4 px-5 py-4">
        <h2 class="sec-title mb-3 flex items-center gap-2">
          <Target size={15} style="color: var(--sempa-accent);" /> This week’s objective
        </h2>
        {#if thisWeekObjectives.length}
          <div class="mb-3 overflow-hidden rounded-[14px]" style="background: var(--sempa-fill);">
            {#each thisWeekObjectives as o, i (o.id)}
              {@render objectiveRow(o, i)}
            {/each}
          </div>
        {/if}
        <form class="add-pill" onsubmit={(e) => { e.preventDefault(); addObjective(); }}>
          <input bind:value={newObjective}
            placeholder={thisWeekObjectives.length ? 'Add another objective for this week…' : 'What will move this project forward this week?'}
            class="min-w-0 flex-1 bg-transparent text-sm outline-none" style="color: var(--sempa-text);" />
          <button type="submit" disabled={!newObjective.trim()} aria-label="Add objective"
            class="btn-primary h-8 w-8 shrink-0 p-0 disabled:opacity-40"><Plus size={16} strokeWidth={2.5} /></button>
        </form>
      </section>

      <!-- Tasks -->
      <section class="surface mb-4 px-5 py-4">
        <div class="mb-3 flex items-baseline gap-2">
          <h2 class="sec-title">Tasks</h2>
          {#if tasks.length}<span class="type-meta" style="color: var(--sempa-text-dim);">{openTasks.length} open</span>{/if}
        </div>
        <form class="add-pill mb-1" onsubmit={(e) => { e.preventDefault(); addTask(); }}>
          <input bind:value={newTask} placeholder="Add a task to this project…"
            class="min-w-0 flex-1 bg-transparent text-sm outline-none" style="color: var(--sempa-text);" />
          <button type="submit" disabled={!newTask.trim()} aria-label="Add task"
            class="btn-primary h-8 w-8 shrink-0 p-0 disabled:opacity-40"><Plus size={16} strokeWidth={2.5} /></button>
        </form>

        {#if tasks.length === 0}
          <p class="mt-3 text-sm" style="color: var(--sempa-text-dim);">No tasks yet. Add one above, or pick this project on any task.</p>
        {/if}
        {#each taskGroups as g (g.key)}
          <p class="type-label mb-1 mt-4 px-1" style="color: var(--sempa-text-soft);">{g.label}</p>
          <div class="flex flex-col">
            {#each g.tasks as t, i (t.id)}{@render taskRow(t, i)}{/each}
          </div>
        {/each}
        {#if doneTasks.length > 0}
          <button onclick={() => (showDone = !showDone)} class="btn-soft mt-3 h-8 px-3.5 text-[12px]"
            style="background: var(--sempa-fill); color: var(--sempa-text-soft);">{showDone ? 'Hide' : 'Show'} done ({doneTasks.length})</button>
          {#if showDone}
            <div class="mt-1 flex flex-col">
              {#each doneTasks as t, i (t.id)}{@render taskRow(t, i)}{/each}
            </div>
          {/if}
        {/if}
      </section>

      <!-- Lists -->
      <section class="surface mb-4 px-5 py-4">
        <h2 class="sec-title mb-3 flex items-center gap-2">
          <ListChecks size={15} style="color: var(--sempa-accent);" /> Lists
        </h2>
        {#if lists.length}
          <div class="mb-3 overflow-hidden rounded-[14px]" style="background: var(--sempa-fill);">
            {#each lists as l, i (l.id)}
              <div class="flex items-center gap-2.5 px-3.5 py-2.5"
                   style={i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}>
                <ListChecks size={15} class="shrink-0" style="color: var(--sempa-text-dim);" />
                <a href={`/lists/${l.id}`} class="min-w-0 flex-1 truncate text-sm font-medium hover:underline" style="color: var(--sempa-text);">{l.name || 'Untitled list'}</a>
                <button onclick={() => unlinkList(l)} aria-label="Remove from project" title="Remove from project"
                  class="flex h-7 w-7 items-center justify-center rounded-full transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);"><X size={14} /></button>
              </div>
            {/each}
          </div>
        {/if}
        <div class="flex flex-wrap gap-2">
          <form class="add-pill min-w-[14rem] flex-1" onsubmit={(e) => { e.preventDefault(); createList(); }}>
            <input bind:value={newList} placeholder="New list…"
              class="min-w-0 flex-1 bg-transparent text-sm outline-none" style="color: var(--sempa-text);" />
            <button type="submit" disabled={!newList.trim()} aria-label="Create list"
              class="btn-primary h-8 w-8 shrink-0 p-0 disabled:opacity-40"><Plus size={16} strokeWidth={2.5} /></button>
          </form>
          {#if linkableLists.length > 0}
            <div class="flex min-w-[14rem] flex-1 items-center gap-2">
              <Link2 size={14} style="color: var(--sempa-text-dim);" />
              <div class="flex-1">
                <SempaSelect bind:value={linkListId} placeholder="Add an existing list…"
                  options={linkableLists.map((l) => ({ value: l.id, label: l.name || 'Untitled list' }))}
                  onchange={(v) => linkList(typeof v === 'string' ? v : null)} />
              </div>
            </div>
          {/if}
        </div>
      </section>

      <!-- Objective history -->
      {#if objectiveWeeks.some(([ws]) => ws !== thisWeek)}
        <section class="surface mb-4 px-5 py-4">
          <h2 class="sec-title mb-1">Weekly objectives</h2>
          {#each objectiveWeeks.filter(([ws]) => ws !== thisWeek) as [ws, objs] (ws)}
            <p class="type-label mb-1.5 mt-3 px-1" style="color: var(--sempa-text-soft);">
              <a href={`/week/${ws}`} class="hover:underline">{weekLabel(ws)}</a>
            </p>
            <div class="overflow-hidden rounded-[14px]" style="background: var(--sempa-fill);">
              {#each objs as o, i (o.id)}{@render objectiveRow(o, i)}{/each}
            </div>
          {/each}
        </section>
      {/if}

      <!-- Files -->
      <section class="surface mb-4 px-5 py-4">
        <h2 class="sec-title mb-2">Files</h2>
        <AttachmentList ownerType="project" ownerId={project.id} />
      </section>
    {/if}
  </div>
</div>

{#if project}
  <!-- defaultDate="": an unscheduled project task must stay unscheduled when edited. -->
  <TaskPanel open={panelOpen} task={panelTask} defaultStatus="backlog" defaultDate=""
             defaultProjectId={project.id} onSave={onPanelSave} onClose={() => (panelOpen = false)} />
{/if}

{#snippet taskRow(t: Task, i: number)}
  {@const isDone = t.status === 'done'}
  <div class="flex items-center gap-3 px-1 py-2.5"
       style="{i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''} {isDone ? 'opacity: 0.6;' : ''}">
    <button onclick={() => toggleDone(t)} aria-label={isDone ? 'Mark not done' : 'Mark done'}
      class="check-ring" data-state={isDone ? 'done' : undefined}>
      {#if isDone}<Check size={13} strokeWidth={3} />{/if}
    </button>
    <button onclick={() => openTask(t)} class="min-w-0 flex-1 truncate text-left text-sm {isDone ? 'line-through' : ''}"
      style="color: var(--sempa-text);">{t.title}</button>
    {#if t.recurrence_rule}<span class="text-[11px]" style="color: var(--sempa-text-dim);" title="Recurring">↺</span>{/if}
    {#if t.planned_date}
      <span class="type-meta shrink-0" style="color: var(--sempa-text-dim);">{formatShortDate(t.planned_date)}</span>
    {/if}
    {#if t.time_estimate_minutes}
      <span class="chip chip-time shrink-0">~{formatMinutes(t.time_estimate_minutes)}</span>
    {/if}
  </div>
{/snippet}

{#snippet objectiveRow(o: Objective, i: number = 0)}
  <div class="flex items-center gap-2.5 px-3.5 py-2.5"
       style="{i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''} {o.status !== 'active' ? 'opacity: 0.65;' : ''}">
    <Target size={15} class="shrink-0" style="color: var(--sempa-accent);" />
    <span class="min-w-0 flex-1 truncate text-sm font-medium {o.status === 'completed' ? 'line-through' : ''}" style="color: var(--sempa-text);">{o.title}</span>
    <span class="chip shrink-0"
          style={o.status === 'completed'
            ? 'background: var(--sempa-success-bg); color: var(--sempa-success);'
            : o.status === 'cancelled'
              ? 'background: var(--sempa-bg-panel); color: var(--sempa-text-dim);'
              : 'background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);'}>
      {o.status === 'completed' ? 'Done' : o.status === 'cancelled' ? 'Dropped' : 'Active'}
    </span>
  </div>
{/snippet}

<style>
  .proj-head {
    position: relative;
    padding: 20px 20px 18px;
    border-radius: 24px;
  }
  .head-mark {
    position: absolute;
    top: 14px;
    right: 16px;
    width: 64px;
    height: 64px;
    opacity: .2;
    pointer-events: none;
  }
  .head-track {
    height: 6px;
    border-radius: 999px;
    overflow: hidden;
    background: color-mix(in srgb, var(--sempa-bg-panel) 70%, transparent);
  }
  .head-fill {
    height: 100%;
    border-radius: 999px;
    background: currentColor;
    transition: width 300ms ease;
  }
  /* The tint sets a coloured text ink for the block; form controls inside it
     keep the theme's normal text colour. The shared select/date triggers paint
     with the canvas tokens, which read as holes on a tint (dark themes most of
     all), so re-point those tokens at the raised surface for this block only. */
  .head-fields {
    color: var(--sempa-text);
    --sempa-bg-main: var(--sempa-bg-panel);
    --sempa-bg-nav: var(--sempa-bg-panel);
  }
  .sec-title {
    font-size: 15px;
    font-weight: 800;
    letter-spacing: -.015em;
    color: var(--sempa-text);
  }
  /* Pill add-row: fill track with the input and a round add button. */
  .add-pill {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 44px;
    padding: 0 6px 0 16px;
    border-radius: 999px;
    background: var(--sempa-fill);
  }
  .add-pill input:focus-visible { box-shadow: none; }
  .add-pill:focus-within { box-shadow: 0 0 0 2px color-mix(in srgb, var(--sempa-accent) 25%, transparent); }
</style>
