<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Objective, Task } from '$lib/types';
  import { appendPosition, formatWeekRange, offsetDate, weekStart as calcWeekStart } from '$lib/utils';

  let weekStartDate = $derived($page.params.weekStart ?? calcWeekStart(new Date().toISOString().split('T')[0]));

  let objectives   = $state<Objective[]>([]);
  let tasks        = $state<Task[]>([]);
  let loading      = $state(true);
  let error        = $state<string | null>(null);
  let copied       = $state(false);

  let expandedId   = $state<string | null>(null);
  let addingTitle  = $state('');
  let showAddForm  = $state(false);
  let addingInput: HTMLInputElement | undefined = $state();
  let taskDrafts   = $state<Record<string, string>>({});
  let addingTask   = $state<Record<string, boolean>>({});

  async function load() {
    loading = true; error = null;
    try {
      [objectives, tasks] = await Promise.all([
        api.objectives.listByWeek(weekStartDate),
        api.tasks.listByWeek(weekStartDate),
      ]);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    } finally { loading = false; }
  }

  onMount(load);
  $effect(() => { weekStartDate; void load(); });

  // ── Task helpers ───────────────────────────────────────────────────────────
  function objectiveTasks(id: string): Task[] {
    return tasks.filter(t => t.weekly_objective_id === id && t.status !== 'cancelled')
                .sort((a, b) => a.position - b.position);
  }
  function doneTasks(id: string) { return objectiveTasks(id).filter(t => t.status === 'done'); }
  function progressPct(id: string): number {
    const total = objectiveTasks(id).length;
    return total === 0 ? 0 : Math.round((doneTasks(id).length / total) * 100);
  }

  // ── Objective actions ──────────────────────────────────────────────────────
  let totalObjectives     = $derived(objectives.length);
  let completedObjectives = $derived(objectives.filter(o => o.status === 'completed').length);

  async function toggleStatus(obj: Objective) {
    const next = obj.status === 'completed' ? 'active' : 'completed';
    objectives = objectives.map(o => o.id === obj.id ? { ...o, status: next } : o);
    try { await api.objectives.update(obj.id, { status: next }); }
    catch { objectives = objectives.map(o => o.id === obj.id ? obj : o); }
  }

  async function addObjective() {
    const title = addingTitle.trim();
    if (!title) return;
    addingTitle = ''; showAddForm = false;
    const pos = appendPosition(objectives.map(o => o.position));
    try {
      const obj = await api.objectives.create({ week_start: weekStartDate, title, position: pos });
      objectives = [...objectives, obj];
    } catch (e) { error = e instanceof Error ? e.message : 'Failed'; }
  }

  async function deleteObjective(id: string) {
    objectives = objectives.filter(o => o.id !== id);
    await api.objectives.delete(id).catch(() => {});
  }

  // ── Task actions ───────────────────────────────────────────────────────────
  async function addTask(objId: string) {
    const title = taskDrafts[objId]?.trim();
    if (!title) return;
    addingTask = { ...addingTask, [objId]: true };
    taskDrafts = { ...taskDrafts, [objId]: '' };
    const pos = appendPosition(objectiveTasks(objId).map(t => t.position));
    try {
      const t = await api.tasks.create({
        title,
        weekly_objective_id: objId,
        week_start: weekStartDate,
        status: 'planned',
        position: pos,
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

  // ── Markdown export ────────────────────────────────────────────────────────
  function generateMarkdown(): string {
    const lines = [
      `# Week of ${formatWeekRange(weekStartDate)}`,
      '',
      '## Objectives',
      '',
    ];
    for (const obj of objectives) {
      const linked = objectiveTasks(obj.id);
      const p = progressPct(obj.id);
      const icon = obj.status === 'completed' ? '✅' : '🎯';
      lines.push(`### ${icon} ${obj.title}${linked.length ? ` — ${p}% complete` : ''}`);
      if (linked.length === 0) {
        lines.push('*No tasks linked*');
      } else {
        for (const t of linked) {
          lines.push(`- [${t.status === 'done' ? 'x' : ' '}] ${t.title}`);
        }
      }
      lines.push('');
    }
    if (objectives.length === 0) lines.push('*No objectives set*\n');
    lines.push('---');
    lines.push(`*Sempa · ${new Date().toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })}*`);
    return lines.join('\n');
  }

  async function copyMarkdown() {
    try {
      await navigator.clipboard.writeText(generateMarkdown());
      copied = true;
      setTimeout(() => copied = false, 2000);
    } catch {}
  }

  function navigate(delta: number) { goto(`/week/${offsetDate(weekStartDate, delta * 7)}`); }
</script>

<svelte:head><title>Week of {formatWeekRange(weekStartDate)} — Sempa</title></svelte:head>

<!-- Header -->
<header class="sticky top-0 z-10 border-b border-gray-100 bg-white/95 backdrop-blur-sm
               dark:border-gray-800/60 dark:bg-gray-900/95">
  <div class="flex items-center justify-between px-6 py-3">
    <div class="flex items-center gap-2">
      <button onclick={() => navigate(-1)} aria-label="Previous week"
              class="rounded-lg p-1.5 text-gray-300 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-400">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <div>
        <p class="text-sm font-semibold text-gray-900 dark:text-gray-50">{formatWeekRange(weekStartDate)}</p>
        <p class="text-xs text-gray-400 dark:text-gray-600">{completedObjectives}/{totalObjectives} objectives complete</p>
      </div>
      <button onclick={() => navigate(1)} aria-label="Next week"
              class="rounded-lg p-1.5 text-gray-300 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-400">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
        </svg>
      </button>
    </div>

    <div class="flex items-center gap-2">
      <!-- Copy markdown -->
      <button onclick={copyMarkdown}
              class="flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium
                     text-gray-600 hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
        {#if copied}
          <svg class="h-3.5 w-3.5 text-green-500" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
          </svg>
          Copied!
        {:else}
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <rect x="9" y="9" width="13" height="13" rx="2"/>
            <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
          </svg>
          Copy as markdown
        {/if}
      </button>

      <a href="/week/{weekStartDate}/review"
         class="flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium
                text-gray-600 hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2"/>
        </svg>
        Review week
      </a>

      <a href="/week/{weekStartDate}/plan"
         class="flex items-center gap-1.5 rounded-lg bg-blue-500 px-3 py-1.5 text-xs font-semibold
                text-white hover:bg-blue-600 transition-colors shadow-sm shadow-blue-200 dark:shadow-none">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
        </svg>
        Plan week
      </a>
    </div>
  </div>
</header>

<!-- Body -->
<main class="mx-auto max-w-2xl px-6 py-6">
  {#if loading}
    <div class="flex h-48 items-center justify-center text-sm text-gray-400">Loading…</div>

  {:else if error}
    <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-400">
      {error} <button onclick={load} class="ml-2 underline">Retry</button>
    </div>

  {:else}
    <!-- Overall progress -->
    {#if totalObjectives > 0}
      <div class="mb-6">
        <div class="mb-1.5 flex justify-between text-xs text-gray-400 dark:text-gray-600">
          <span>Week progress</span>
          <span>{completedObjectives}/{totalObjectives} objectives · {totalObjectives ? Math.round((completedObjectives/totalObjectives)*100) : 0}%</span>
        </div>
        <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
          <div class="h-full rounded-full bg-blue-500 transition-all duration-500"
               style="width:{totalObjectives ? Math.round((completedObjectives/totalObjectives)*100) : 0}%"></div>
        </div>
      </div>
    {/if}

    <!-- Objectives list -->
    <div class="flex flex-col gap-3">
      {#each objectives as obj (obj.id)}
        {@const linked  = objectiveTasks(obj.id)}
        {@const done    = doneTasks(obj.id)}
        {@const p       = progressPct(obj.id)}
        {@const isExp   = expandedId === obj.id}
        {@const isDone  = obj.status === 'completed'}

        <div class="rounded-xl border border-gray-100 bg-white shadow-sm transition-shadow hover:shadow-md
                    dark:border-gray-700/50 dark:bg-gray-800/60">

          <!-- Objective header row -->
          <div class="flex items-start gap-3 p-4">
            <!-- Completion circle -->
            <button onclick={() => toggleStatus(obj)} title="{isDone ? 'Mark active' : 'Mark complete'}"
                    class="mt-0.5 h-5 w-5 shrink-0 rounded-full border-2 flex items-center justify-center transition-all
                           {isDone ? 'border-green-500 bg-green-500' : 'border-gray-300 hover:border-green-400 dark:border-gray-600 dark:hover:border-green-500'}">
              {#if isDone}
                <svg class="h-3 w-3 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
              {/if}
            </button>

            <div class="flex-1 min-w-0">
              <p class="text-sm font-semibold {isDone ? 'line-through text-gray-400 dark:text-gray-600' : 'text-gray-800 dark:text-gray-100'}">
                {obj.title}
              </p>

              <!-- Task progress bar + label -->
              {#if linked.length > 0}
                <div class="mt-2 flex items-center gap-2">
                  <div class="h-1.5 flex-1 max-w-[160px] overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                    <div class="h-full rounded-full transition-all duration-500
                                {isDone ? 'bg-green-400' : 'bg-blue-400'}"
                         style="width:{p}%"></div>
                  </div>
                  <span class="text-xs font-medium {isDone ? 'text-green-600 dark:text-green-500' : p === 100 ? 'text-green-600 dark:text-green-500' : 'text-gray-400 dark:text-gray-600'}">
                    {p}% · {done.length}/{linked.length} tasks
                  </span>
                </div>
              {:else}
                <p class="mt-1 text-xs text-gray-400 dark:text-gray-600">No tasks linked yet</p>
              {/if}
            </div>

            <!-- Actions -->
            <div class="flex shrink-0 items-center gap-0.5">
              <button onclick={() => expandedId = isExp ? null : obj.id}
                      class="rounded-lg p-1.5 text-gray-300 hover:bg-gray-100 hover:text-gray-500 transition-colors
                             dark:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-400"
                      aria-label="{isExp ? 'Collapse' : 'Expand'}">
                <svg class="h-4 w-4 transition-transform {isExp ? 'rotate-180' : ''}" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7"/>
                </svg>
              </button>
              <button onclick={() => deleteObjective(obj.id)} aria-label="Delete objective"
                      class="rounded-lg p-1.5 text-gray-200 hover:bg-red-50 hover:text-red-400 transition-colors
                             dark:text-gray-700 dark:hover:bg-red-950/40 dark:hover:text-red-400">
                <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
          </div>

          <!-- Expanded: task list + inline add -->
          {#if isExp}
            <div class="border-t border-gray-50 px-4 py-3 space-y-1 dark:border-gray-700/30">
              {#each linked as t (t.id)}
                <div class="group flex items-center gap-2.5 rounded-lg px-1 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-700/30">
                  <button onclick={() => toggleTask(t)}
                          class="h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center transition-all
                                 {t.status === 'done' ? 'border-green-500 bg-green-500' : 'border-gray-300 hover:border-green-400 dark:border-gray-600'}">
                    {#if t.status === 'done'}
                      <svg class="h-2.5 w-2.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                      </svg>
                    {/if}
                  </button>
                  <span class="flex-1 text-sm {t.status === 'done' ? 'line-through text-gray-400 dark:text-gray-600' : 'text-gray-700 dark:text-gray-200'}">
                    {t.title}
                  </span>
                  {#if linked.length > 0}
                    <span class="text-[10px] text-gray-300 dark:text-gray-700 shrink-0">
                      {Math.round(100 / linked.length)}%
                    </span>
                  {/if}
                  {#if t.planned_date}
                    <span class="text-[10px] text-gray-300 dark:text-gray-600 shrink-0">{t.planned_date.slice(5)}</span>
                  {/if}
                </div>
              {/each}

              <!-- Quick add task -->
              <div class="flex items-center gap-2 rounded-lg border border-dashed border-gray-200 px-2 py-1.5 mt-1
                          focus-within:border-blue-400 dark:border-gray-700 dark:focus-within:border-blue-600">
                <svg class="h-3.5 w-3.5 shrink-0 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" d="M12 4v16m8-8H4"/>
                </svg>
                <input bind:value={taskDrafts[obj.id]}
                       onkeydown={(e) => { if (e.key === 'Enter') addTask(obj.id); }}
                       type="text"
                       placeholder="Add a task for this objective… (Enter)"
                       class="flex-1 bg-transparent text-xs text-gray-700 placeholder-gray-400 outline-none
                              dark:text-gray-200 dark:placeholder-gray-600" />
                {#if taskDrafts[obj.id]?.trim()}
                  <button onclick={() => addTask(obj.id)} disabled={addingTask[obj.id]}
                          class="text-xs text-blue-500 hover:text-blue-700 dark:text-blue-400 disabled:opacity-40">
                    Add
                  </button>
                {/if}
              </div>
            </div>
          {/if}
        </div>
      {/each}

      <!-- Empty state -->
      {#if objectives.length === 0 && !showAddForm}
        <div class="rounded-xl border-2 border-dashed border-gray-100 p-10 text-center dark:border-gray-800">
          <p class="text-sm text-gray-400 dark:text-gray-600">No objectives yet.</p>
          <a href="/week/{weekStartDate}/plan"
             class="mt-2 inline-block text-sm text-blue-500 hover:underline dark:text-blue-400">
            Start the weekly planning ritual →
          </a>
        </div>
      {/if}

      <!-- Inline add form -->
      {#if showAddForm}
        <div class="rounded-xl border border-blue-200 bg-blue-50/60 p-4 dark:border-blue-800/40 dark:bg-blue-950/30">
          <input bind:this={addingInput}
                 bind:value={addingTitle}
                 onkeydown={(e) => { if (e.key === 'Enter') addObjective(); if (e.key === 'Escape') showAddForm = false; }}
                 type="text"
                 placeholder="What do you want to accomplish this week?"
                 class="w-full rounded-lg border border-blue-200 bg-white px-3 py-2.5 text-sm
                        text-gray-800 placeholder-gray-400 outline-none focus:ring-2 focus:ring-blue-300
                        dark:border-blue-700 dark:bg-gray-800 dark:text-gray-100" />
          <div class="mt-2 flex gap-2">
            <button onclick={addObjective} disabled={!addingTitle.trim()}
                    class="rounded-lg bg-blue-500 px-4 py-1.5 text-xs font-medium text-white
                           hover:bg-blue-600 disabled:opacity-40 transition-colors">
              Add
            </button>
            <button onclick={() => showAddForm = false}
                    class="rounded-lg px-4 py-1.5 text-xs text-gray-500 hover:bg-blue-100 transition-colors
                           dark:text-gray-400 dark:hover:bg-blue-950">
              Cancel
            </button>
          </div>
        </div>
      {/if}
    </div>
  {/if}
</main>
