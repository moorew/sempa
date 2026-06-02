<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Objective, Task } from '$lib/types';
  import { appendPosition, formatWeekRange, weekStart as calcWeekStart } from '$lib/utils';

  let ws = $derived($page.params.weekStart ?? calcWeekStart(new Date().toISOString().split('T')[0]));

  let step = $state(1);
  let loading = $state(true);
  let saving  = $state(false);
  let copied  = $state(false);
  let error   = $state('');

  let objectives = $state<Objective[]>([]);
  let tasks      = $state<Task[]>([]);

  // Step 1: draft objective titles (mirrors objectives array; separate to allow optimistic edits)
  let draftTitles = $state<string[]>([]);
  let addingTitle = $state('');

  // Step 2: per-objective quick-add inputs
  let taskDrafts  = $state<Record<string, string>>({});
  let addingTask  = $state<Record<string, boolean>>({});

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
  onMount(load);

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
    try {
      const t = await api.tasks.create({
        title,
        weekly_objective_id: objId,
        week_start: ws,
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

  // ── Markdown ───────────────────────────────────────────────────────────────

  function generateMarkdown(): string {
    const lines = [
      `# Week of ${formatWeekRange(ws)}`,
      '',
      '## Objectives',
      '',
    ];
    for (const obj of objectives) {
      const linked = objectiveTasks(obj.id);
      const p = pct(obj.id);
      const icon = obj.status === 'completed' ? '✅' : '🎯';
      lines.push(`### ${icon} ${obj.title}${linked.length ? ` — ${p}% complete` : ''}`);
      if (linked.length === 0) {
        lines.push('*No tasks yet*');
      } else {
        for (const t of linked) {
          lines.push(`- [${t.status === 'done' ? 'x' : ' '}] ${t.title}`);
        }
      }
      lines.push('');
    }
    if (objectives.length === 0) lines.push('*No objectives set*\n');
    lines.push('---');
    lines.push(`*Sempa weekly plan · ${new Date().toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })}*`);
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

<div class="mx-auto max-w-2xl px-6 py-10">

  <!-- Header -->
  <div class="mb-8 flex items-start justify-between">
    <div>
      <a href="/week/{ws}" class="text-xs text-gray-400 hover:text-gray-600 dark:text-gray-600 dark:hover:text-gray-400">
        ← Back to week
      </a>
      <h1 class="mt-2 text-xl font-bold text-gray-900 dark:text-gray-50">Plan your week</h1>
      <p class="text-sm text-gray-500 dark:text-gray-500">{formatWeekRange(ws)}</p>
    </div>
    <!-- Step indicator pills -->
    <div class="flex items-center gap-1.5 pt-1">
      {#each [{ n: 1, label: 'Objectives' }, { n: 2, label: 'Tasks' }, { n: 3, label: 'Export' }] as s}
        <button onclick={() => step = s.n}
                class="rounded-full px-3 py-1 text-xs font-medium transition-colors
                       {step === s.n
                         ? 'bg-blue-500 text-white'
                         : step > s.n
                           ? 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-400'
                           : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-500'}">
          {s.n < step ? '✓ ' : ''}{s.label}
        </button>
      {/each}
    </div>
  </div>

  {#if loading}
    <div class="flex h-40 items-center justify-center text-sm text-gray-400">Loading…</div>
  {:else if error}
    <p class="rounded-xl bg-red-50 p-4 text-sm text-red-600">{error}</p>

  <!-- ── Step 1: Objectives ─────────────────────────────────────────────── -->
  {:else if step === 1}
    <div class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        What are your <strong class="text-gray-700 dark:text-gray-200">2–4 big goals</strong> for this week?
        Be specific enough to know when you've achieved each one.
      </p>

      <div class="space-y-2">
        {#each objectives as obj, i (obj.id)}
          <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-white px-3 py-2
                      dark:border-gray-700 dark:bg-gray-800/60">
            <span class="text-gray-300 dark:text-gray-600 select-none">🎯</span>
            <input
              bind:value={draftTitles[i]}
              onblur={() => saveTitle(i)}
              onkeydown={(e) => { if (e.key === 'Enter') { saveTitle(i); addingTitle = ''; (document.getElementById('new-obj') as HTMLInputElement)?.focus(); } }}
              type="text"
              class="flex-1 bg-transparent text-sm font-medium text-gray-800 placeholder-gray-400 outline-none
                     dark:text-gray-100 dark:placeholder-gray-600"
            />
            <button onclick={() => deleteObjective(obj.id)} aria-label="Delete objective"
                    class="text-gray-300 hover:text-red-400 transition-colors dark:text-gray-600 dark:hover:text-red-400">
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
              </svg>
            </button>
          </div>
        {/each}

        <!-- Add new objective input -->
        <div class="flex items-center gap-2 rounded-xl border border-dashed border-gray-200 bg-gray-50 px-3 py-2
                    focus-within:border-blue-400 focus-within:bg-white dark:border-gray-700 dark:bg-gray-800/30
                    dark:focus-within:border-blue-600 dark:focus-within:bg-gray-800">
          <span class="text-gray-300 dark:text-gray-600 select-none">🎯</span>
          <input
            id="new-obj"
            bind:value={addingTitle}
            onkeydown={(e) => { if (e.key === 'Enter') addObjective(); }}
            type="text"
            placeholder="Add an objective… (press Enter)"
            class="flex-1 bg-transparent text-sm text-gray-700 placeholder-gray-400 outline-none
                   dark:text-gray-200 dark:placeholder-gray-600"
          />
          {#if addingTitle.trim()}
            <button onclick={addObjective}
                    class="text-xs text-blue-500 hover:text-blue-700 dark:text-blue-400">Add</button>
          {/if}
        </div>
      </div>

      <div class="pt-2 flex justify-end">
        <button onclick={() => step = 2} disabled={objectives.length === 0}
                class="rounded-xl bg-blue-500 px-6 py-2.5 text-sm font-medium text-white
                       hover:bg-blue-600 disabled:opacity-40 transition-colors">
          Next: add tasks →
        </button>
      </div>
    </div>

  <!-- ── Step 2: Tasks per objective ────────────────────────────────────── -->
  {:else if step === 2}
    <div class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        Break each objective into <strong class="text-gray-700 dark:text-gray-200">concrete tasks</strong>.
        Each task counts equally toward the objective's progress.
      </p>

      {#each objectives as obj (obj.id)}
        {@const linked = objectiveTasks(obj.id)}
        {@const done   = doneTasks(obj.id)}
        {@const p = pct(obj.id)}
        <div class="rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800/60">
          <!-- Objective header -->
          <div class="flex items-center gap-3 px-4 py-3 border-b border-gray-100 dark:border-gray-700/50">
            <div class="flex-1">
              <p class="text-sm font-semibold text-gray-800 dark:text-gray-100">🎯 {obj.title}</p>
              {#if linked.length > 0}
                <div class="mt-1.5 flex items-center gap-2">
                  <div class="h-1.5 w-24 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                    <div class="h-full rounded-full bg-blue-400 transition-all" style="width:{p}%"></div>
                  </div>
                  <span class="text-xs text-gray-400">{done.length}/{linked.length} · {p}%</span>
                </div>
              {/if}
            </div>
          </div>

          <!-- Task list -->
          <div class="px-4 py-2 space-y-1">
            {#each linked as t (t.id)}
              <div class="flex items-center gap-2.5 py-1">
                <button onclick={() => toggleTask(t)}
                        class="h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center transition-all
                               {t.status === 'done' ? 'border-green-500 bg-green-500' : 'border-gray-300 dark:border-gray-600 hover:border-green-400'}">
                  {#if t.status === 'done'}
                    <svg class="h-2.5 w-2.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                    </svg>
                  {/if}
                </button>
                <span class="text-sm {t.status === 'done' ? 'line-through text-gray-400 dark:text-gray-600' : 'text-gray-700 dark:text-gray-200'}">
                  {t.title}
                </span>
                {#if linked.length > 0}
                  <span class="ml-auto text-[10px] text-gray-300 dark:text-gray-600 shrink-0">
                    {Math.round(100 / linked.length)}%
                  </span>
                {/if}
              </div>
            {/each}

            <!-- Quick-add -->
            <div class="flex items-center gap-2 rounded-lg border border-dashed border-gray-200 px-2 py-1.5
                        focus-within:border-blue-400 dark:border-gray-700 dark:focus-within:border-blue-600">
              <svg class="h-3.5 w-3.5 shrink-0 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" d="M12 4v16m8-8H4"/>
              </svg>
              <input
                bind:value={taskDrafts[obj.id]}
                onkeydown={(e) => { if (e.key === 'Enter') addTask(obj.id); }}
                type="text"
                placeholder="Add a task… (Enter to save)"
                class="flex-1 bg-transparent text-xs text-gray-700 placeholder-gray-400 outline-none
                       dark:text-gray-200 dark:placeholder-gray-600"
              />
              {#if taskDrafts[obj.id]?.trim()}
                <button onclick={() => addTask(obj.id)} disabled={addingTask[obj.id]}
                        class="text-xs text-blue-500 hover:text-blue-700 dark:text-blue-400 disabled:opacity-40">
                  Add
                </button>
              {/if}
            </div>
          </div>
        </div>
      {/each}

      <div class="pt-2 flex gap-2 justify-end">
        <button onclick={() => step = 1}
                class="rounded-xl border border-gray-200 px-5 py-2.5 text-sm text-gray-500
                       hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
          ← Back
        </button>
        <button onclick={() => step = 3}
                class="rounded-xl bg-blue-500 px-6 py-2.5 text-sm font-medium text-white hover:bg-blue-600 transition-colors">
          Preview & export →
        </button>
      </div>
    </div>

  <!-- ── Step 3: Export ──────────────────────────────────────────────────── -->
  {:else if step === 3}
    <div class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        Your week is set. Copy this as markdown to paste into any doc or report.
      </p>

      <!-- Markdown preview -->
      <div class="relative">
        <pre class="overflow-x-auto rounded-xl border border-gray-200 bg-gray-50 p-4 text-xs text-gray-700 leading-relaxed
                    dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-300">{generateMarkdown()}</pre>
        <button onclick={copyMarkdown}
                class="absolute right-3 top-3 flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5
                       text-xs font-medium text-gray-600 shadow-sm hover:bg-gray-50 transition-colors
                       dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700">
          {#if copied}
            <svg class="h-3.5 w-3.5 text-green-500" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
            Copied!
          {:else}
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
            </svg>
            Copy markdown
          {/if}
        </button>
      </div>

      <div class="flex gap-2 justify-end">
        <button onclick={() => step = 2}
                class="rounded-xl border border-gray-200 px-5 py-2.5 text-sm text-gray-500
                       hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
          ← Back
        </button>
        <button onclick={() => goto(`/week/${ws}`)}
                class="rounded-xl bg-blue-500 px-6 py-2.5 text-sm font-medium text-white hover:bg-blue-600 transition-colors">
          Start the week →
        </button>
      </div>
    </div>
  {/if}
</div>
