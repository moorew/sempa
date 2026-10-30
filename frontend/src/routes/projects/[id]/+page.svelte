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
  <div class="flex items-center gap-3 px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <a href="/projects" class="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm transition-colors" style="color: var(--sempa-accent);">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" d="M19 12H5m7-7-7 7 7 7"/></svg>
      Projects
    </a>
    {#if project}
      <button onclick={remove} aria-label="Delete project" title="Delete project"
        class="ml-auto rounded-lg p-1.5 transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);">
        <Trash2 size={16} />
      </button>
    {/if}
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-6 pb-24">
    {#if loading && !project}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else if !project}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Project not found.</p>
    {:else}
      <!-- Title + colour -->
      <div class="relative mb-3 flex items-center gap-3">
        <button onclick={() => (colorOpen = !colorOpen)} aria-label="Project colour"
          class="h-5 w-5 shrink-0 rounded-full ring-offset-2 transition-transform hover:scale-110"
          style="background: {project.color};"></button>
        {#if colorOpen}
          <div class="absolute left-0 top-8 z-20 flex gap-1.5 rounded-xl p-2 shadow-lg"
               style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">
            {#each PROJECT_COLORS as c}
              <button aria-label={`Colour ${c}`} onclick={() => { colorOpen = false; patch({ color: c }); }}
                class="h-6 w-6 rounded-full transition-transform hover:scale-110"
                style="background: {c}; {c === project.color ? 'outline: 2px solid var(--sempa-text); outline-offset: 2px;' : ''}"></button>
            {/each}
          </div>
        {/if}
        <input value={project.title} onblur={(e) => rename((e.currentTarget as HTMLInputElement).value.trim())}
          onkeydown={(e) => { if (e.key === 'Enter') (e.currentTarget as HTMLInputElement).blur(); }}
          placeholder="Project name"
          class="min-w-0 flex-1 bg-transparent text-2xl font-bold outline-none" style="color: var(--sempa-text);" />
      </div>

      <!-- Status + dates -->
      <div class="mb-4 grid gap-3 sm:grid-cols-3">
        <div>
          <label class="mb-1 block text-[11px] font-medium uppercase tracking-wider" style="color: var(--sempa-text-dim);" for="proj-status">Status</label>
          <SempaSelect id="proj-status" value={project.status_id} options={statusOptions}
            onchange={(v) => { if (typeof v === 'string') patch({ status_id: v }); }} />
        </div>
        <div>
          <label class="mb-1 block text-[11px] font-medium uppercase tracking-wider" style="color: var(--sempa-text-dim);" for="proj-start">Start</label>
          <SempaDatePicker id="proj-start" value={project.start_date ?? ''} placeholder="No start date"
            onchange={(v) => patch({ start_date: v })} />
        </div>
        <div>
          <label class="mb-1 block text-[11px] font-medium uppercase tracking-wider" style="color: var(--sempa-text-dim);" for="proj-target">Target</label>
          <SempaDatePicker id="proj-target" value={project.target_date ?? ''} placeholder="No target date"
            onchange={(v) => patch({ target_date: v })} />
        </div>
      </div>

      <!-- Progress -->
      <div class="mb-5 rounded-xl px-4 py-3" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
        <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1 text-[12.5px]" style="color: var(--sempa-text-soft);">
          <span class="font-semibold" style="color: var(--sempa-text);">{progress.pct}%</span>
          <span>{progress.done} of {progress.total} tasks done</span>
          {#if progress.estimateMinutes}<span>· ~{formatMinutes(progress.estimateMinutes)} estimated</span>{/if}
          {#if progress.actualMinutes}<span>· {formatMinutes(progress.actualMinutes)} tracked</span>{/if}
          {#if project.target_date}
            <span class="ml-auto" style={progress.overdue ? 'color: var(--sempa-danger, #b3392e);' : ''}>
              {relativeDue(project.target_date, todayStr)}
            </span>
          {/if}
        </div>
        <div class="mt-2 h-2 overflow-hidden rounded-full" style="background: var(--sempa-bg-main);">
          <div class="h-full rounded-full transition-all" style="width: {progress.pct}%; background: {project.color};"></div>
        </div>
      </div>

      <!-- Description -->
      <section class="mb-6">
        <div class="mb-1.5 flex items-center justify-between">
          <h2 class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Description</h2>
          {#if !editingDesc}
            <button onclick={() => { descDraft = project?.description ?? ''; editingDesc = true; }}
              class="flex items-center gap-1 text-xs transition-opacity hover:opacity-70" style="color: var(--sempa-accent);">
              <Pencil size={12} /> Edit
            </button>
          {/if}
        </div>
        {#if editingDesc}
          <!-- svelte-ignore a11y_autofocus -->
          <textarea bind:value={descDraft} onblur={saveDesc} rows="6" autofocus
            placeholder="What is this project, why does it matter, what does done look like?"
            class="w-full rounded-xl px-3 py-2.5 text-sm outline-none"
            style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);"></textarea>
        {:else if project.description}
          <div class="text-sm leading-relaxed" style="color: var(--sempa-text);"><RichText text={project.description} /></div>
        {:else}
          <p class="text-sm" style="color: var(--sempa-text-dim);">No description yet.</p>
        {/if}
      </section>

      <!-- Sharing (multi-user household only) -->
      {#if household.multiUser}
        <div class="mb-6 flex items-center justify-between gap-3 rounded-lg border px-3 py-2.5"
             style="border-color: var(--sempa-border); background: var(--sempa-bg-panel);">
          <div class="min-w-0">
            <p class="text-[13px] font-medium" style="color: var(--sempa-text);">{project.shared ? 'Shared' : 'Private'}</p>
            <p class="text-[11.5px]" style="color: var(--sempa-text-soft);">
              {project.shared ? 'Everyone can see this project. Your private tasks in it stay private.' : 'Only you can see this project.'}
            </p>
          </div>
          <button type="button" role="switch" aria-checked={project.shared} aria-label="Share project"
                  onclick={() => patch({ shared: !project?.shared })}
                  class="relative h-[26px] w-[44px] shrink-0 rounded-full transition-colors"
                  style="background: {project.shared ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
            <span class="absolute top-[3px] h-[20px] w-[20px] rounded-full bg-white transition-all"
                  style="left: {project.shared ? '21px' : '3px'};"></span>
          </button>
        </div>
      {/if}

      <!-- This week -->
      <section class="mb-6">
        <h2 class="mb-2 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">
          <Target size={13} /> This week’s objective
        </h2>
        {#each thisWeekObjectives as o (o.id)}
          {@render objectiveRow(o)}
        {/each}
        <form class="mt-1 flex gap-2" onsubmit={(e) => { e.preventDefault(); addObjective(); }}>
          <input bind:value={newObjective}
            placeholder={thisWeekObjectives.length ? 'Add another objective for this week…' : 'What will move this project forward this week?'}
            class="flex-1 rounded-lg px-3 py-2 text-sm outline-none"
            style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
          <button type="submit" disabled={!newObjective.trim()} aria-label="Add objective"
            class="rounded-lg px-3 text-white disabled:opacity-40" style="background: var(--sempa-accent);"><Plus size={16} /></button>
        </form>
      </section>

      <!-- Tasks -->
      <section class="mb-6">
        <h2 class="mb-2 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Tasks</h2>
        <form class="mb-3 flex gap-2" onsubmit={(e) => { e.preventDefault(); addTask(); }}>
          <input bind:value={newTask} placeholder="Add a task to this project…"
            class="flex-1 rounded-lg px-3 py-2 text-sm outline-none"
            style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
          <button type="submit" disabled={!newTask.trim()} aria-label="Add task"
            class="rounded-lg px-3 text-white disabled:opacity-40" style="background: var(--sempa-accent);"><Plus size={16} /></button>
        </form>

        {#if tasks.length === 0}
          <p class="text-sm" style="color: var(--sempa-text-dim);">No tasks yet. Add one above, or pick this project on any task.</p>
        {/if}
        {#each taskGroups as g (g.key)}
          <p class="mb-1 mt-3 px-1 text-[11px] font-medium" style="color: var(--sempa-text-soft);">{g.label}</p>
          <div class="flex flex-col gap-1">
            {#each g.tasks as t (t.id)}{@render taskRow(t)}{/each}
          </div>
        {/each}
        {#if doneTasks.length > 0}
          <button onclick={() => (showDone = !showDone)} class="mt-3 text-xs font-medium transition-opacity hover:opacity-70"
            style="color: var(--sempa-text-dim);">{showDone ? 'Hide' : 'Show'} done ({doneTasks.length})</button>
          {#if showDone}
            <div class="mt-1 flex flex-col gap-1">
              {#each doneTasks as t (t.id)}{@render taskRow(t)}{/each}
            </div>
          {/if}
        {/if}
      </section>

      <!-- Lists -->
      <section class="mb-6">
        <h2 class="mb-2 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">
          <ListChecks size={13} /> Lists
        </h2>
        <div class="flex flex-col gap-1">
          {#each lists as l (l.id)}
            <div class="flex items-center gap-2 rounded-lg px-3 py-2" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
              <a href={`/lists/${l.id}`} class="min-w-0 flex-1 truncate text-sm hover:underline" style="color: var(--sempa-text);">{l.name || 'Untitled list'}</a>
              <button onclick={() => unlinkList(l)} aria-label="Remove from project" title="Remove from project"
                class="transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);"><X size={14} /></button>
            </div>
          {/each}
        </div>
        <div class="mt-2 flex flex-wrap gap-2">
          <form class="flex min-w-[14rem] flex-1 gap-2" onsubmit={(e) => { e.preventDefault(); createList(); }}>
            <input bind:value={newList} placeholder="New list…"
              class="flex-1 rounded-lg px-3 py-2 text-sm outline-none"
              style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
            <button type="submit" disabled={!newList.trim()} aria-label="Create list"
              class="rounded-lg px-3 text-white disabled:opacity-40" style="background: var(--sempa-accent);"><Plus size={16} /></button>
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
        <section class="mb-6">
          <h2 class="mb-2 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Weekly objectives</h2>
          {#each objectiveWeeks.filter(([ws]) => ws !== thisWeek) as [ws, objs] (ws)}
            <p class="mb-1 mt-3 px-1 text-[11px] font-medium" style="color: var(--sempa-text-soft);">
              <a href={`/week/${ws}`} class="hover:underline">{weekLabel(ws)}</a>
            </p>
            {#each objs as o (o.id)}{@render objectiveRow(o)}{/each}
          {/each}
        </section>
      {/if}

      <!-- Files -->
      <section class="mb-6">
        <h2 class="mb-2 text-[11px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Files</h2>
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

{#snippet taskRow(t: Task)}
  {@const isDone = t.status === 'done'}
  <div class="flex items-center gap-2.5 rounded-lg px-3 py-2"
       style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); {isDone ? 'opacity: 0.6;' : ''}">
    <button onclick={() => toggleDone(t)} aria-label={isDone ? 'Mark not done' : 'Mark done'}
      class="flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-full border transition-colors"
      style="border-color: {isDone ? 'var(--sempa-success)' : 'var(--sempa-border)'}; background: {isDone ? 'var(--sempa-success)' : 'transparent'};">
      {#if isDone}<Check size={11} color="white" />{/if}
    </button>
    <button onclick={() => openTask(t)} class="min-w-0 flex-1 truncate text-left text-sm {isDone ? 'line-through' : ''}"
      style="color: var(--sempa-text);">{t.title}</button>
    {#if t.recurrence_rule}<span class="text-[11px]" style="color: var(--sempa-text-dim);" title="Recurring">↺</span>{/if}
    {#if t.planned_date}
      <span class="shrink-0 text-[11px]" style="color: var(--sempa-text-dim);">{formatShortDate(t.planned_date)}</span>
    {/if}
    {#if t.time_estimate_minutes}
      <span class="shrink-0 font-mono text-[11px]" style="color: var(--sempa-text-dim);">~{formatMinutes(t.time_estimate_minutes)}</span>
    {/if}
  </div>
{/snippet}

{#snippet objectiveRow(o: Objective)}
  <div class="mb-1 flex items-center gap-2 rounded-lg px-3 py-2"
       style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); {o.status !== 'active' ? 'opacity: 0.65;' : ''}">
    <span>🎯</span>
    <span class="min-w-0 flex-1 truncate text-sm {o.status === 'completed' ? 'line-through' : ''}" style="color: var(--sempa-text);">{o.title}</span>
    <span class="shrink-0 text-[11px]" style="color: var(--sempa-text-dim);">
      {o.status === 'completed' ? 'Done' : o.status === 'cancelled' ? 'Dropped' : 'Active'}
    </span>
  </div>
{/snippet}
