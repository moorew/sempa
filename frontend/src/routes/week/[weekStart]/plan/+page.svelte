<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import type { Objective, Task } from '$lib/types';
  import { appendPosition, formatWeekRange, offsetDate, today, weekStart as calcWeekStart } from '$lib/utils';
  import { projectStore } from '$lib/stores/projects.svelte';
  import SempaSelect from '$lib/components/ui/SempaSelect.svelte';
  import { Target, Check, ArrowLeft, ArrowRight, Plus, X, Copy } from 'lucide-svelte';

  let ws = $derived(page.params.weekStart ?? calcWeekStart(today()));
  const thisWeek = calcWeekStart(today());

  let step = $state(1);
  let loading = $state(true);
  let saving  = $state(false);
  let copied  = $state(false);
  let error   = $state('');

  let objectives = $state<Objective[]>([]);
  let tasks      = $state<Task[]>([]);

  // Active projects, shown for reference while choosing objectives (no prompts).
  $effect(() => { projectStore.load(); });
  const activeProjects = $derived(projectStore.projects.filter((p) => projectStore.categoryOf(p) === 'active'));
  const projectOptions = $derived([
    { value: null, label: 'No project' },
    ...projectStore.open.map((p) => ({ value: p.id, label: p.title || 'Untitled project', color: p.color })),
  ]);
  async function setObjectiveProject(obj: Objective, projectId: string | null) {
    if ((obj.project_id ?? null) === projectId) return;
    try {
      const updated = await api.objectives.update(obj.id, { project_id: projectId ?? '' });
      objectives = objectives.map(o => o.id === updated.id ? updated : o);
    } catch { /* ignore */ }
  }

  // Step 1: draft objective titles (mirrors objectives array; separate to allow optimistic edits)
  let draftTitles = $state<string[]>([]);
  let addingTitle = $state('');

  // Step 2: per-objective quick-add inputs
  let taskDrafts  = $state<Record<string, string>>({});
  let taskDays    = $state<Record<string, string>>({}); // per-obj planned_date
  let addingTask  = $state<Record<string, boolean>>({});

  // Day options for the current week (Mon–Sun)
  const weekDays = $derived.by(() => {
    const days: { label: string; value: string }[] = [];
    for (let i = 0; i < 7; i++) {
      const d = offsetDate(ws, i);
      days.push({
        value: d,
        label: new Date(d + 'T12:00:00').toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' }),
      });
    }
    return days;
  });

  async function load() {
    loading = true;
    try {
      [objectives, tasks] = await Promise.all([
        api.objectives.listByWeek(ws),
        api.tasks.listByWeek(ws),
      ]);
      draftTitles = objectives.map(o => o.title);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    } finally {
      loading = false;
    }
  }
  // Reload whenever the week changes (mount + prev/next navigation, which keeps
  // this same component mounted and only swaps the [weekStart] route param).
  $effect(() => { ws; load(); });

  // Jump to another week's planner. `ws` is derived from the URL, so navigating
  // re-runs the load effect above with the new week's objectives.
  function goWeek(delta: number) {
    step = 1;
    goto(`/week/${offsetDate(ws, delta * 7)}/plan`);
  }

  // ── Objective helpers ─────────────────────────────────────────────────────

  function objectiveTasks(id: string): Task[] {
    return tasks.filter(t => t.weekly_objective_id === id && t.status !== 'cancelled');
  }
  function doneTasks(id: string) {
    return objectiveTasks(id).filter(t => t.status === 'done');
  }
  function pct(id: string): number {
    const total = objectiveTasks(id).length;
    return total === 0 ? 0 : Math.round((doneTasks(id).length / total) * 100);
  }

  // ── Step 1: objectives ────────────────────────────────────────────────────

  async function addObjective() {
    const title = addingTitle.trim();
    if (!title) return;
    addingTitle = '';
    const pos = appendPosition(objectives.map(o => o.position));
    try {
      const obj = await api.objectives.create({ week_start: ws, title, position: pos });
      objectives = [...objectives, obj];
      draftTitles = [...draftTitles, obj.title];
    } catch {}
  }

  async function saveTitle(i: number) {
    const obj = objectives[i];
    const t = draftTitles[i]?.trim();
    if (!t || t === obj.title) return;
    try {
      const updated = await api.objectives.update(obj.id, { title: t });
      objectives = objectives.map(o => o.id === updated.id ? updated : o);
    } catch {}
  }

  async function deleteObjective(id: string) {
    objectives = objectives.filter(o => o.id !== id);
    draftTitles = draftTitles.filter((_, i) => objectives[i]?.id !== id);
    await api.objectives.delete(id).catch(() => {});
  }

  // ── Step 2: tasks per objective ────────────────────────────────────────────

  async function addTask(objId: string) {
    const title = taskDrafts[objId]?.trim();
    if (!title) return;
    addingTask = { ...addingTask, [objId]: true };
    taskDrafts = { ...taskDrafts, [objId]: '' };
    const pos = appendPosition(objectiveTasks(objId).map(t => t.position));
    const plannedDate = taskDays[objId] ?? ws; // default to Monday
    try {
      const t = await api.tasks.create({
        title,
        weekly_objective_id: objId,
        week_start: ws,
        planned_date: plannedDate,
        status: 'planned',
        position: pos,
        // Work toward a project's objective belongs to that project too.
        project_id: objectives.find(o => o.id === objId)?.project_id ?? undefined,
      });
      tasks = [...tasks, t];
    } catch {}
    addingTask = { ...addingTask, [objId]: false };
  }

  async function toggleTask(t: Task) {
    const newStatus = t.status === 'done' ? 'planned' : 'done';
    tasks = tasks.map(x => x.id === t.id ? { ...x, status: newStatus } : x);
    try {
      const updated = await api.tasks.update(t.id, {
        status: newStatus,
        completed_at: newStatus === 'done' ? new Date().toISOString() : null,
      });
      tasks = tasks.map(x => x.id === updated.id ? updated : x);
    } catch { await load(); }
  }

  // ── Markdown ───────────────────────────────────────────────────────────────

  // A clean bulleted list: one H1 title, objectives as top-level bullets and
  // their linked tasks nested beneath. No bold, no subheadings — plain Markdown.
  function generateMarkdown(): string {
    const lines = [`# Week of ${formatWeekRange(ws)}`, ''];
    for (const obj of objectives) {
      lines.push(`- [${obj.status === 'completed' ? 'x' : ' '}] ${obj.title}`);
      for (const t of objectiveTasks(obj.id)) {
        lines.push(`  - [${t.status === 'done' ? 'x' : ' '}] ${t.title}`);
      }
    }
    if (objectives.length === 0) lines.push('- No objectives set');
    return lines.join('\n');
  }

  async function copyMarkdown() {
    try {
      await navigator.clipboard.writeText(generateMarkdown());
      copied = true;
      setTimeout(() => copied = false, 2000);
    } catch {}
  }
</script>

<svelte:head><title>Plan week · {formatWeekRange(ws)} — Sempa</title></svelte:head>

<div class="mx-auto max-w-2xl px-4 pb-28 pt-8 sm:px-6 sm:py-10 animate-page-in"
     style="padding-top: calc(env(safe-area-inset-top, 0px) + 32px);">

  <!-- Header -->
  <div class="mb-6">
    <a href="/week/{ws}" class="back-link">
      <ArrowLeft size={14} strokeWidth={2.25} />
      Back to week
    </a>
    <div class="mt-3 flex flex-wrap items-end justify-between gap-x-4 gap-y-3">
      <div class="min-w-0">
        <h1 class="type-page" style="color: var(--sempa-text);">Plan your week</h1>
        <!-- Week switcher — plan any week, not just the current one -->
        <div class="mt-1.5 flex items-center gap-1">
          <button onclick={() => goWeek(-1)} aria-label="Previous week" class="nav-round">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7"/></svg>
          </button>
          <p class="min-w-[9.5rem] text-center type-meta" style="color: var(--sempa-text-soft);">{formatWeekRange(ws)}</p>
          <button onclick={() => goWeek(1)} aria-label="Next week" class="nav-round">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/></svg>
          </button>
          {#if ws !== thisWeek}
            <button onclick={() => { step = 1; goto(`/week/${thisWeek}/plan`); }}
                    class="btn-soft ml-1"
                    style="height: 28px; padding: 0 12px; font-size: 12px;">
              This week
            </button>
          {/if}
        </div>
      </div>
      <!-- Step indicator: a segmented control -->
      <div class="segmented on-canvas w-full sm:w-auto" role="tablist" aria-label="Planning steps">
        {#each [{ n: 1, label: 'Objectives' }, { n: 2, label: 'Tasks' }, { n: 3, label: 'Export' }] as s}
          <button onclick={() => step = s.n}
                  role="tab"
                  aria-selected={step === s.n}
                  class="seg-step"
                  style="color: {step === s.n ? 'var(--sempa-text)' : step > s.n ? 'var(--sempa-success)' : 'var(--sempa-text-soft)'};">
            {#if s.n < step}<Check size={13} strokeWidth={3} />{/if}
            {s.label}
          </button>
        {/each}
      </div>
    </div>
  </div>

  {#if loading}
    <div class="flex h-40 items-center justify-center text-sm" style="color: var(--sempa-text-dim);">Loading…</div>
  {:else if error}
    <p class="rounded-xl bg-red-50 p-4 text-sm text-red-600">{error}</p>

  <!-- ── Step 1: Objectives ─────────────────────────────────────────────── -->
  {:else if step === 1}
    <div class="space-y-4">
      <div class="intro">
        <h2 class="type-section" style="color: var(--sempa-text);">What do you want to accomplish?</h2>
        <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">
          Be specific enough to know when you've achieved each one.
        </p>
      </div>

      <div class="space-y-2.5">
        {#each objectives as obj, i (obj.id)}
          {@const hasProject = projectStore.open.length > 0 || !!obj.project_id}
          <!-- On narrow screens the project picker drops to its own line under
               the title, so the objective text is never squeezed off-screen. -->
          <div class="obj-row surface py-2.5 pl-4 pr-2" class:has-project={hasProject} style="border-radius: 18px; min-height: 56px;">
            <span class="obj-icon" style="grid-area: icon;" aria-hidden="true"><Target size={15} strokeWidth={2.25} /></span>
            <input
              bind:value={draftTitles[i]}
              onblur={() => saveTitle(i)}
              onkeydown={(e) => { if (e.key === 'Enter') { saveTitle(i); addingTitle = ''; (document.getElementById('new-obj') as HTMLInputElement)?.focus(); } }}
              type="text"
              aria-label="Objective {i + 1}"
              class="min-w-0 bg-transparent outline-none"
              style="grid-area: title; font-size: 14.5px; font-weight: 600; color: var(--sempa-text);"
            />
            {#if hasProject}
              <div class="obj-project" style="grid-area: project;">
                <SempaSelect value={obj.project_id ?? null} placeholder="No project" options={projectOptions}
                  onchange={(v) => setObjectiveProject(obj, typeof v === 'string' ? v : null)} />
              </div>
            {/if}
            <button onclick={() => deleteObjective(obj.id)} aria-label="Delete objective"
                    class="icon-ghost"
                    style="grid-area: del; color: var(--sempa-text-dim);"
                    onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = '#f87171'}
                    onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
              <X size={16} strokeWidth={2} />
            </button>
          </div>
        {/each}

        <!-- Add new objective input -->
        <div class="add-obj flex items-center gap-3 py-2.5 pl-4 pr-2" style="min-height: 56px;">
          <span class="obj-icon is-new" aria-hidden="true"><Plus size={15} strokeWidth={2.5} /></span>
          <input
            id="new-obj"
            bind:value={addingTitle}
            onkeydown={(e) => { if (e.key === 'Enter') addObjective(); }}
            type="text"
            placeholder="Add an objective… (press Enter)"
            class="min-w-0 flex-1 bg-transparent text-sm outline-none"
            style="color: var(--sempa-text);"
          />
          {#if addingTitle.trim()}
            <button onclick={addObjective}
                    class="btn-primary"
                    style="height: 34px; padding: 0 16px; font-size: 13px;">Add</button>
          {/if}
        </div>
      </div>

      <!-- Active projects — reference only, to inform this week's objectives. -->
      {#if activeProjects.length > 0}
        <div class="surface px-4 py-3.5" style="border-radius: 18px;">
          <p class="mb-2 type-label" style="color: var(--sempa-text-soft);">Active projects</p>
          <div class="flex flex-col gap-1">
            {#each activeProjects as p (p.id)}
              {@const linked = objectives.filter(o => o.project_id === p.id).length}
              <a href={`/projects/${p.id}`} class="proj-row flex items-center gap-2.5 text-sm" style="color: var(--sempa-text);">
                <span class="h-2.5 w-2.5 shrink-0 rounded-full" style="background: {p.color};"></span>
                <span class="min-w-0 flex-1 truncate font-medium">{p.title || 'Untitled project'}</span>
                {#if linked > 0}
                  <span class="chip tint shrink-0" style="--tint: {p.color};">
                    <Target size={11} strokeWidth={2.5} /> {linked}
                  </span>
                {/if}
                {#if p.target_date}
                  <span class="shrink-0 type-meta" style="color: var(--sempa-text-dim);">
                    target {new Date(p.target_date + 'T12:00:00').toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                  </span>
                {/if}
              </a>
            {/each}
          </div>
        </div>
      {/if}

      <div class="pt-2 flex justify-end">
        <button onclick={() => step = 2} disabled={objectives.length === 0}
                class="btn-primary disabled:opacity-40"
                style="height: 44px; padding: 0 24px; font-size: 14px;">
          Next: add tasks
          <ArrowRight size={16} strokeWidth={2.25} />
        </button>
      </div>
    </div>

  <!-- ── Step 2: Tasks per objective ────────────────────────────────────── -->
  {:else if step === 2}
    <div class="space-y-4">
      <div class="intro">
        <h2 class="type-section" style="color: var(--sempa-text);">Break it into tasks</h2>
        <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">
          Give each objective a few <strong style="color: var(--sempa-text);">concrete tasks</strong>.
          Each task counts equally toward the objective's progress.
        </p>
      </div>

      {#each objectives as obj (obj.id)}
        {@const linked = objectiveTasks(obj.id)}
        {@const done   = doneTasks(obj.id)}
        {@const p = pct(obj.id)}
        <div class="surface" style="border-radius: 20px;">
          <!-- Objective header -->
          <div class="flex items-start gap-3 px-4 pb-3 pt-4">
            <span class="obj-icon mt-px" aria-hidden="true"><Target size={15} strokeWidth={2.25} /></span>
            <div class="min-w-0 flex-1">
              <p style="font-size: 15px; font-weight: 700; letter-spacing: -.01em; color: var(--sempa-text);">{obj.title}</p>
              {#if linked.length > 0}
                <div class="mt-2 flex items-center gap-2.5">
                  <div class="day-bar-track flex-1" style="max-width: 200px;">
                    <div class="day-bar-fill" style="width:{p}%; background: {p === 100 ? 'var(--sempa-success)' : 'var(--sempa-accent)'};"></div>
                  </div>
                  <span class="type-meta tabular-nums" style="color: var(--sempa-text-soft);">{done.length}/{linked.length} · {p}%</span>
                </div>
              {:else}
                <p class="mt-0.5 type-meta" style="color: var(--sempa-text-dim);">No tasks yet</p>
              {/if}
            </div>
          </div>

          <!-- Task list -->
          <div class="px-4 pb-4 sm:pl-[3.25rem]">
            {#each linked as t (t.id)}
              <div class="flex items-center gap-2.5 py-2" style="border-top: 1px solid var(--sempa-border);">
                <button onclick={() => toggleTask(t)}
                        class="check-ring" data-state={t.status === 'done' ? 'done' : undefined}
                        style="width: 20px; height: 20px;">
                  {#if t.status === 'done'}
                    <svg class="h-2.5 w-2.5" fill="none" stroke="currentColor" stroke-width="3.5" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                    </svg>
                  {/if}
                </button>
                <span class="text-sm {t.status === 'done' ? 'line-through' : ''}"
                      style="color: {t.status === 'done' ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'}">
                  {t.title}
                </span>
                {#if linked.length > 0}
                  <span class="ml-auto shrink-0 tabular-nums" style="font-size: 11.5px; color: var(--sempa-text-dim);">
                    {Math.round(100 / linked.length)}%
                  </span>
                {/if}
              </div>
            {/each}

            <!-- Quick-add -->
            <div class="mt-2 space-y-2">
              <div class="add-field flex items-center gap-2 px-3" style="min-height: 40px;">
                <Plus size={14} strokeWidth={2.25} style="color: var(--sempa-text-dim); flex-shrink: 0;" />
                <input
                  bind:value={taskDrafts[obj.id]}
                  onkeydown={(e) => { if (e.key === 'Enter') addTask(obj.id); }}
                  type="text"
                  placeholder="Add a task… (Enter to save)"
                  class="flex-1 bg-transparent text-sm outline-none"
                  style="color: var(--sempa-text);"
                />
              </div>
              {#if taskDrafts[obj.id]?.trim()}
                <div class="flex items-center gap-2">
                  <select bind:value={taskDays[obj.id]}
                          class="px-3 text-sm outline-none"
                          style="height: 36px; border: 1px solid var(--sempa-border); background: var(--sempa-fill);
                                 border-radius: 12px; color: var(--sempa-text);">
                    {#each weekDays as d}
                      <option value={d.value}>{d.label}</option>
                    {/each}
                  </select>
                  <button onclick={() => addTask(obj.id)} disabled={addingTask[obj.id]}
                          class="btn-primary disabled:opacity-40"
                          style="height: 36px; padding: 0 16px; font-size: 13px;">
                    Add task
                  </button>
                </div>
              {/if}
            </div>
          </div>
        </div>
      {/each}

      <div class="pt-2 flex gap-2 justify-end">
        <button onclick={() => step = 1}
                class="btn-surface"
                style="height: 44px; padding: 0 20px; font-size: 14px; color: var(--sempa-text-soft);">
          <ArrowLeft size={16} strokeWidth={2.25} />
          Back
        </button>
        <button onclick={() => step = 3}
                class="btn-primary"
                style="height: 44px; padding: 0 24px; font-size: 14px;">
          Preview & export
          <ArrowRight size={16} strokeWidth={2.25} />
        </button>
      </div>
    </div>

  <!-- ── Step 3: Export ──────────────────────────────────────────────────── -->
  {:else if step === 3}
    <div class="space-y-4">
      <div class="intro">
        <h2 class="type-section" style="color: var(--sempa-text);">Your week is set</h2>
        <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">
          Copy this as markdown to paste into any doc or report.
        </p>
      </div>

      <!-- Markdown preview -->
      <div class="surface relative" style="border-radius: 20px; padding: 8px;">
        <pre class="font-mono text-xs leading-relaxed"
             style="background: var(--sempa-fill); border-radius: 14px; padding: 16px 18px; padding-right: 150px;
                    overflow-x: auto; color: var(--sempa-text); min-height: 64px;">{generateMarkdown()}</pre>
        <button onclick={copyMarkdown}
                class="btn-surface absolute right-4 top-4"
                style="height: 34px; padding: 0 14px; font-size: 12.5px; {copied ? 'color: var(--sempa-success);' : 'color: var(--sempa-text-soft);'}">
          {#if copied}
            <Check size={14} strokeWidth={2.75} />
            Copied!
          {:else}
            <Copy size={14} strokeWidth={2} />
            Copy markdown
          {/if}
        </button>
      </div>

      <div class="flex gap-2 justify-end">
        <button onclick={() => step = 2}
                class="btn-surface"
                style="height: 44px; padding: 0 20px; font-size: 14px; color: var(--sempa-text-soft);">
          <ArrowLeft size={16} strokeWidth={2.25} />
          Back
        </button>
        <button onclick={() => goto(`/week/${ws}`)}
                class="btn-primary"
                style="height: 44px; padding: 0 24px; font-size: 14px;">
          Start the week
          <ArrowRight size={16} strokeWidth={2.25} />
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .back-link {
    display: inline-flex; align-items: center; gap: 6px;
    font-size: 13px; font-weight: 600; color: var(--sempa-text-soft);
    transition: color 120ms ease;
  }
  .back-link:hover { color: var(--sempa-accent-ink); }

  .nav-round {
    display: grid; place-items: center; width: 32px; height: 32px; border-radius: 999px;
    color: var(--sempa-text-soft); transition: background-color 120ms ease, color 120ms ease;
  }
  .nav-round:hover { background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text); }

  /* The segmented track normally sits on a surface; here it sits on the
     canvas, so recess it a notch so the raised selected pill reads. */
  .on-canvas { background: color-mix(in srgb, var(--sempa-text) 6%, var(--sempa-bg-main)); }

  .seg-step {
    display: inline-flex; align-items: center; justify-content: center; gap: 5px;
    height: 34px; padding: 0 16px; font-size: 13px; font-weight: 600; white-space: nowrap;
  }
  .seg-step[aria-selected="true"] { font-weight: 700; }

  .intro { padding: 0 2px; }

  /* Objective row: icon · title · project · delete on one line; on narrow
     screens the project picker wraps under the title. */
  .obj-row {
    display: grid; align-items: center; column-gap: 12px; row-gap: 8px;
    grid-template-columns: auto minmax(0, 1fr) auto;
    grid-template-areas: "icon title del";
  }
  .obj-row.has-project {
    grid-template-columns: auto minmax(0, 1fr) 9rem auto;
    grid-template-areas: "icon title project del";
  }
  @media (max-width: 560px) {
    .obj-row.has-project {
      grid-template-columns: auto minmax(0, 1fr) auto;
      grid-template-areas: "icon title del" ". project project";
      padding-bottom: 12px;
    }
    .obj-row.has-project .obj-project { max-width: 14rem; }
  }

  /* Objective marker: a small accent-tinted rounded square. */
  .obj-icon {
    display: inline-grid; place-items: center; flex: 0 0 auto;
    width: 28px; height: 28px; border-radius: 10px;
    background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);
  }
  .obj-icon.is-new { background: var(--sempa-fill); color: var(--sempa-text-soft); }

  /* The add-objective row sits on the canvas as a soft dashed slot that
     firms up on focus, so it reads as "the next card goes here". */
  .add-obj {
    border-radius: 18px;
    border: 1.5px dashed color-mix(in srgb, var(--sempa-text) 16%, transparent);
    transition: border-color 120ms ease, background-color 120ms ease;
  }
  .add-obj:focus-within { border-style: solid; border-color: var(--sempa-accent); background: var(--sempa-bg-panel); }

  .add-field {
    background: var(--sempa-fill);
    border: 1px solid transparent;
    border-radius: 12px;
    transition: border-color 120ms ease;
  }
  .add-field:focus-within { border-color: var(--sempa-accent); }

  .icon-ghost {
    display: inline-flex; align-items: center; justify-content: center; flex: 0 0 auto;
    width: 36px; height: 36px; border-radius: 999px;
    transition: background-color 120ms ease, color 120ms ease;
  }
  .icon-ghost:hover { background: var(--sempa-fill); }

  .proj-row { min-height: 36px; border-radius: 10px; padding: 0 6px; margin: 0 -6px; }
  .proj-row:hover { background: var(--sempa-fill); }
</style>
