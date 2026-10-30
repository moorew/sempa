<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import type { Objective, Task, WeekReview } from '$lib/types';
  import { appendPosition, insertPosition, formatWeekRange, offsetDate, today, weekStart as calcWeekStart } from '$lib/utils';
  import { mobile } from '$lib/stores/mobile.svelte';
  import { prefs } from '$lib/stores/prefs.svelte';
  import { household } from '$lib/stores/household.svelte';
  import SempaPattern from '$lib/components/ui/SempaPattern.svelte';
  import SempaSelect from '$lib/components/ui/SempaSelect.svelte';
  import AttachmentList from '$lib/components/AttachmentList.svelte';
  import { swipeNavigate } from '$lib/actions/swipeNavigate';
  import { tagStore } from '$lib/stores/tags.svelte';
  import TagFilterBar from '$lib/components/TagFilterBar.svelte';
  import { SlidersHorizontal, CalendarArrowDown, GripVertical, Users2 } from 'lucide-svelte';
  import { moveObjectiveToNextWeek, moveAllUnfinishedToNextWeek, unfinishedObjectives } from '$lib/objectives';
  import { contextMenu, type MenuItem } from '$lib/stores/contextMenu.svelte';
  import { celebrate } from '$lib/celebrate';

  let weekStartDate = $derived(page.params.weekStart ?? calcWeekStart(today()));

  // ── Tag filter (shared with the day view via the same localStorage key) ────
  let filterTags  = $state<string[]>([]);
  let filterMatch = $state<'any' | 'all'>('any');
  let showFilter  = $state(false);
  if (typeof localStorage !== 'undefined') {
    try {
      const v = JSON.parse(localStorage.getItem('sempa_tag_filter') ?? '{}');
      if (Array.isArray(v.tags)) filterTags = v.tags;
      if (v.match === 'all' || v.match === 'any') filterMatch = v.match;
      if (filterTags.length) showFilter = true;
    } catch { /* ignore */ }
  }
  $effect(() => {
    if (typeof localStorage === 'undefined') return;
    localStorage.setItem('sempa_tag_filter', JSON.stringify({ tags: filterTags, match: filterMatch }));
  });
  function passesTagFilter(t: Task): boolean {
    if (filterTags.length === 0) return true;
    const tt = t.tags ?? [];
    return filterMatch === 'all' ? filterTags.every(f => tt.includes(f)) : filterTags.some(f => tt.includes(f));
  }

  let objectives       = $state<Objective[]>([]);
  let tasks            = $state<Task[]>([]);
  let loading          = $state(true);
  let error            = $state<string | null>(null);
  let copied           = $state(false);
  let showUnscheduled  = $state(false);

  const unscheduled = $derived(
    tasks.filter(t => !t.planned_date && t.status !== 'done' && t.status !== 'cancelled')
         .filter(passesTagFilter)
         .sort((a, b) => a.position - b.position)
  );

  let expandedId   = $state<string | null>(null);
  let addingTitle  = $state('');
  let showAddForm  = $state(false);
  let addingInput: HTMLInputElement | undefined = $state();
  let taskDrafts   = $state<Record<string, string>>({});
  let addingTask   = $state<Record<string, boolean>>({});
  let taskDays     = $state<Record<string, string>>({});

  let review = $state<WeekReview | null>(null);

  async function load() {
    loading = true; error = null;
    try {
      [objectives, tasks, review] = await Promise.all([
        api.objectives.listByWeek(weekStartDate),
        api.tasks.listByWeek(weekStartDate),
        api.weeks.getReview(weekStartDate).catch(() => null),
      ]);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    } finally { loading = false; }
  }

  function parseList(json: string | null | undefined): string[] {
    if (!json) return [];
    try { const v = JSON.parse(json); return Array.isArray(v) ? v.filter((s) => typeof s === 'string' && s.trim()) : []; }
    catch { return []; }
  }

  const reviewWins      = $derived(parseList(review?.wins));
  const reviewChallenges = $derived(parseList(review?.challenges));
  const reviewNextFocus = $derived(review?.next_focus?.trim() ?? '');
  const reviewDone      = $derived(reviewWins.length > 0 || reviewChallenges.length > 0 || reviewNextFocus.length > 0);

  import { realtime } from '$lib/stores/realtime.svelte';

  onMount(() => { void household.ensure(); load(); });
  $effect(() => { weekStartDate; void load(); });

  $effect(() => {
    const ev = realtime.lastEvent;
    if (!ev) return;
    if (ev.type === 'task:change' || ev.type === 'objective:change') void load();
  });

  // ── Task helpers ───────────────────────────────────────────────────────────
  function objectiveTasks(id: string): Task[] {
    return tasks.filter(t => t.weekly_objective_id === id && t.status !== 'cancelled')
                .filter(passesTagFilter)
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

  // Display-only: the week summary surface (progress ring + task tally) and a
  // compact range for the narrow mobile header.
  const weekPct        = $derived(totalObjectives ? Math.round((completedObjectives / totalObjectives) * 100) : 0);
  const weekTasks      = $derived(tasks.filter(t => t.planned_date && t.status !== 'cancelled'));
  const weekTasksDone  = $derived(weekTasks.filter(t => t.status === 'done').length);
  const RING_R = 26;
  const RING_C = 2 * Math.PI * RING_R;
  const shortRange = $derived.by(() => {
    const s = new Date(weekStartDate + 'T12:00:00');
    const e = new Date(weekStartDate + 'T12:00:00'); e.setDate(e.getDate() + 6);
    const f = (d: Date) => d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
    return `${f(s)} – ${f(e)}`;
  });

  async function toggleStatus(obj: Objective) {
    const next = obj.status === 'completed' ? 'active' : 'completed';
    objectives = objectives.map(o => o.id === obj.id ? { ...o, status: next } : o);
    // Tier 3 — the full cradle-mark moment when this completes the last open
    // objective of the week.
    if (next === 'completed' && objectives.length > 0 && objectives.every(o => o.status === 'completed')) {
      celebrate.week('the week is complete.');
    }
    try { await api.objectives.update(obj.id, { status: next }); }
    catch { objectives = objectives.map(o => o.id === obj.id ? obj : o); }
  }

  // Private/Shared (multi-user household). Optimistic; reverts on failure.
  async function toggleObjShared(obj: Objective) {
    const next = !obj.shared;
    objectives = objectives.map(o => o.id === obj.id ? { ...o, shared: next } : o);
    try { await api.objectives.update(obj.id, { shared: next }); }
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

  async function scheduleToday(t: Task) {
    const d = today();
    tasks = tasks.map(x => x.id === t.id ? { ...x, planned_date: d } : x);
    try {
      const updated = await api.tasks.update(t.id, { planned_date: d, week_start: weekStartDate, status: 'planned' });
      tasks = tasks.map(x => x.id === updated.id ? updated : x);
    } catch { await load(); }
  }

  async function deleteObjective(id: string) {
    objectives = objectives.filter(o => o.id !== id);
    await api.objectives.delete(id).catch(() => {});
  }

  // Right-click an objective → its actions, mirroring the row's buttons.
  function onObjContextMenu(e: MouseEvent, obj: Objective) {
    const done = obj.status === 'completed';
    const items: MenuItem[] = [
      { label: done ? 'Mark active' : 'Mark complete', onClick: () => toggleStatus(obj) },
      { label: 'Add task', onClick: () => { expandedId = obj.id; } },
    ];
    if (!done) items.push({ label: 'Move to next week', onClick: () => replanObjective(obj) });
    if (household.multiUser) {
      items.push({ label: obj.shared ? 'Make private' : 'Share with household', onClick: () => toggleObjShared(obj) });
    }
    items.push('separator');
    items.push({ label: 'Delete', danger: true, onClick: () => deleteObjective(obj.id) });
    contextMenu.show(e, items);
  }

  // ── Drag to reorder objectives ───────────────────────────────────────────────
  // Mirrors task reordering: HTML5 DnD (works on desktop web + the Tauri shell,
  // which has window drag-drop disabled so the webview gets the events). A
  // fractional `position` is persisted so the order survives reload and sync.
  let dragObjId    = $state<string | null>(null);
  let dragOverObjId = $state<string | null>(null);

  function onObjDragStart(e: DragEvent, id: string) {
    dragObjId = id;
    e.dataTransfer?.setData('application/x-sempa-objective', id);
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move';
  }
  function onObjDragEnd() { dragObjId = null; dragOverObjId = null; }

  async function onObjDrop(targetId: string) {
    const id = dragObjId;
    dragObjId = null;
    dragOverObjId = null;
    if (!id || id === targetId) return;

    const ordered = [...objectives].sort((a, b) => a.position - b.position);
    const fromIdx = ordered.findIndex(o => o.id === id);
    const toIdx   = ordered.findIndex(o => o.id === targetId);
    if (fromIdx < 0 || toIdx < 0) return;

    const others = ordered.filter(o => o.id !== id);
    // Drop below the target → land after it; drop above → land before it.
    let insertIdx = others.findIndex(o => o.id === targetId);
    if (toIdx > fromIdx) insertIdx += 1;
    const newPos = insertPosition(others.map(o => o.position), insertIdx);

    const dragged = ordered[fromIdx];
    const reordered = [...others];
    reordered.splice(insertIdx, 0, { ...dragged, position: newPos });

    const prev = objectives;
    objectives = reordered; // optimistic
    try { await api.objectives.update(id, { position: newPos }); }
    catch { objectives = prev; }
  }

  // ── Re-plan: carry unfinished objectives forward to next week ────────────────
  const unfinishedCount = $derived(unfinishedObjectives(objectives).length);
  let replanning = $state(false);

  async function replanObjective(obj: Objective) {
    objectives = objectives.filter(o => o.id !== obj.id); // optimistic — it leaves this week
    try { await moveObjectiveToNextWeek(obj, tasks); }
    catch { await load(); }
  }

  async function replanAllUnfinished() {
    if (replanning || unfinishedCount === 0) return;
    replanning = true;
    const snapshot = objectives.slice();
    objectives = objectives.filter(o => o.status === 'completed' || o.status === 'cancelled');
    try { await moveAllUnfinishedToNextWeek(snapshot, tasks); }
    catch { /* fall through to reload */ }
    finally { replanning = false; await load(); }
  }

  // ── Task actions ───────────────────────────────────────────────────────────
  async function addTask(objId: string) {
    const title = taskDrafts[objId]?.trim();
    if (!title) return;
    addingTask = { ...addingTask, [objId]: true };
    taskDrafts = { ...taskDrafts, [objId]: '' };
    const pos = appendPosition(objectiveTasks(objId).map(t => t.position));
    const plannedDate = taskDays[objId] ?? today();
    try {
      const t = await api.tasks.create({
        title,
        weekly_objective_id: objId,
        week_start: weekStartDate,
        planned_date: plannedDate,
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
    // Linked completion: when every task under an objective is done, complete
    // the objective too (and reopen it if a task is un-checked).
    if (t.weekly_objective_id) {
      const objId = t.weekly_objective_id;
      const linked = tasks.filter(x => x.weekly_objective_id === objId && x.status !== 'cancelled');
      const objStatus = linked.length > 0 && linked.every(x => x.status === 'done') ? 'completed' : 'active';
      const cur = objectives.find(o => o.id === objId);
      if (cur && cur.status !== objStatus) {
        objectives = objectives.map(o => o.id === objId ? { ...o, status: objStatus } : o);
        api.objectives.update(objId, { status: objStatus }).catch(() => {});
      }
    }
    try {
      const updated = await api.tasks.update(t.id, {
        status: newStatus,
        completed_at: newStatus === 'done' ? new Date().toISOString() : null,
      });
      tasks = tasks.map(x => x.id === updated.id ? updated : x);
    } catch { await load(); }
  }

  // ── Markdown export ────────────────────────────────────────────────────────
  // A clean bulleted list: one H1 title, objectives as top-level bullets and
  // their linked tasks nested beneath. No bold, no subheadings — plain Markdown.
  function generateMarkdown(): string {
    const lines = [`# Week of ${formatWeekRange(weekStartDate)}`, ''];
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

  function navigate(delta: number) { goto(`/week/${offsetDate(weekStartDate, delta * 7)}`); }
</script>

<svelte:head><title>Week of {formatWeekRange(weekStartDate)} — Sempa</title></svelte:head>

<!-- Header -->
<header class="sticky top-0 z-10 backdrop-blur-sm"
        style="background: color-mix(in srgb, var(--sempa-bg-main) 92%, transparent);
               padding-top: env(safe-area-inset-top, 0px);">
  <div class="flex items-center justify-between gap-3 px-4 py-3 sm:px-6 sm:py-4">
    <div class="flex min-w-0 items-center gap-1.5 sm:gap-2">
      <button onclick={() => navigate(-1)} aria-label="Previous week"
              class="wk-nav grid shrink-0 place-items-center rounded-full transition-colors"
              style="color: var(--sempa-text-soft);">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <div class="min-w-0">
        <p class="truncate type-date" style="color: var(--sempa-text); {mobile.value ? 'font-size: 18px;' : ''}">
          {mobile.value ? shortRange : formatWeekRange(weekStartDate)}
        </p>
        <p class="truncate type-meta" style="color: var(--sempa-text-soft);">{completedObjectives}/{totalObjectives} objectives complete</p>
      </div>
      <button onclick={() => navigate(1)} aria-label="Next week"
              class="wk-nav grid shrink-0 place-items-center rounded-full transition-colors"
              style="color: var(--sempa-text-soft);">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
        </svg>
      </button>
    </div>

    <div class="flex shrink-0 items-center gap-2">
      {#if mobile.value}
        <!-- Mobile: the primary action stays in the header; copy + review sit
             in the action row at the top of the page so the week title has room. -->
        <a href="/week/{weekStartDate}/plan"
           class="btn-primary"
           style="height: 44px; padding: 0 18px; font-size: 14px; gap: 6px;">
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
          </svg>
          Plan
        </a>
      {:else}
        <!-- Desktop: text buttons -->
        <button onclick={copyMarkdown}
                class="btn-surface"
                style="height: 38px; padding: 0 16px; font-size: 13px; color: var(--sempa-text-soft);">
          {#if copied}
            <svg class="h-3.5 w-3.5" style="color: var(--sempa-success);" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
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
           class={reviewDone ? 'btn-soft' : 'btn-surface'}
           style="height: 38px; padding: 0 16px; font-size: 13px; {reviewDone ? '' : 'color: var(--sempa-text-soft);'}">
          {#if reviewDone}
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
            </svg>
            Reviewed
          {:else}
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2"/>
            </svg>
            Review week
          {/if}
        </a>

      <a href="/week/{weekStartDate}/plan"
         class="btn-primary"
         style="height: 38px; padding: 0 20px; font-size: 13px; gap: 6px;">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
        </svg>
        Plan week
      </a>
      {/if}
    </div>
  </div>
</header>

<!-- Body -->
<main class="mx-auto max-w-2xl px-4 pb-28 pt-3 sm:px-6 sm:pb-10 sm:pt-4 animate-page-in"
      use:swipeNavigate={{ onPrev: () => navigate(-1), onNext: () => navigate(1) }}>
  <!-- Tag filter (in-place) + mobile copy/review actions -->
  {#if tagStore.definitions.length || mobile.value}
    <div class="mb-4 flex flex-wrap items-center gap-3">
      {#if tagStore.definitions.length}
        <button onclick={() => showFilter = !showFilter}
                class="inline-flex items-center gap-1.5 rounded-full transition-colors {filterTags.length ? 'btn-soft' : 'btn-surface'}"
                style="height: {mobile.value ? 40 : 32}px; padding: 0 {mobile.value ? 14 : 12}px; font-size: 12.5px;
                       {filterTags.length ? '' : 'color: var(--sempa-text-soft);'}">
          <SlidersHorizontal size={13} />
          {filterTags.length ? `Filtered · ${filterTags.length}` : 'Filter by tag'}
        </button>
      {/if}
      {#if mobile.value}
        <div class="ml-auto flex items-center gap-2">
          <button onclick={copyMarkdown}
                  title="Copy as markdown" aria-label="Copy as markdown"
                  class="btn-icon"
                  style="width:44px; height:44px; {copied ? 'color: var(--sempa-success);' : ''}">
            {#if copied}
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
              </svg>
            {:else}
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <rect x="9" y="9" width="13" height="13" rx="2"/>
                <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
              </svg>
            {/if}
          </button>
          <a href="/week/{weekStartDate}/review"
             title={reviewDone ? 'Reviewed — edit' : 'Review week'}
             class={reviewDone ? 'btn-soft' : 'btn-surface'}
             style="height: 44px; padding: 0 16px; font-size: 13px; {reviewDone ? '' : 'color: var(--sempa-text-soft);'}">
            {#if reviewDone}
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
              </svg>
              Reviewed
            {:else}
              <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2"/>
              </svg>
              Review
            {/if}
          </a>
        </div>
      {/if}
      {#if showFilter && tagStore.definitions.length}
        <div class="w-full sm:w-auto"><TagFilterBar bind:selected={filterTags} bind:match={filterMatch} /></div>
      {/if}
    </div>
  {/if}

  {#if loading}
    <div class="flex h-48 items-center justify-center text-sm" style="color: var(--sempa-text-dim);">Loading…</div>

  {:else if error}
    <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-400">
      {error} <button onclick={load} class="ml-2 underline">Retry</button>
    </div>

  {:else}
    <!-- Week summary: progress ring + tallies, with the carry-forward action -->
    {#if totalObjectives > 0}
      <section class="surface mb-5" style="border-radius: 22px; padding: 18px 20px;">
        <div class="flex items-center gap-4">
          <div class="relative shrink-0" style="width: 64px; height: 64px;"
               role="img" aria-label="{weekPct}% of objectives complete">
            <svg width="64" height="64" viewBox="0 0 64 64" style="transform: rotate(-90deg);">
              <circle cx="32" cy="32" r={RING_R} fill="none" stroke="var(--sempa-track)" stroke-width="6" />
              <circle cx="32" cy="32" r={RING_R} fill="none"
                      stroke={weekPct === 100 ? 'var(--sempa-success)' : 'var(--sempa-accent)'}
                      stroke-width="6" stroke-linecap="round"
                      stroke-dasharray={RING_C}
                      stroke-dashoffset={RING_C * (1 - weekPct / 100)}
                      style="transition: stroke-dashoffset 500ms ease-out; {weekPct === 0 ? 'opacity: 0;' : ''}" />
            </svg>
            <span class="absolute inset-0 grid place-items-center tabular-nums"
                  style="font-size: 14px; font-weight: 800; letter-spacing: -.02em; color: var(--sempa-text);">
              {weekPct}%
            </span>
          </div>
          <div class="min-w-0 flex-1">
            <h2 class="type-section" style="color: var(--sempa-text);">Week progress</h2>
            <p class="mt-0.5 type-meta" style="color: var(--sempa-text-soft);">
              {completedObjectives} of {totalObjectives} objectives complete
              {#if weekTasks.length}<span class="mx-1" style="color: var(--sempa-text-dim);" aria-hidden="true">·</span>{weekTasksDone} of {weekTasks.length} tasks done{/if}
            </p>
          </div>
        </div>

        <!-- Carry unfinished objectives forward to next week -->
        {#if unfinishedCount > 0}
          <div class="mt-4 pt-4" style="border-top: 1px solid var(--sempa-border);">
            <button onclick={replanAllUnfinished} disabled={replanning}
                    class="btn-soft w-full disabled:opacity-50"
                    style="min-height: 40px; padding: 0 16px; font-size: 13px;">
              <CalendarArrowDown size={15} />
              {replanning ? 'Moving…' : `Move ${unfinishedCount} unfinished ${unfinishedCount === 1 ? 'objective' : 'objectives'} to next week`}
            </button>
          </div>
        {/if}
      </section>
    {:else if unfinishedCount > 0}
      <button onclick={replanAllUnfinished} disabled={replanning}
              class="btn-soft mb-4 w-full disabled:opacity-50"
              style="min-height: 40px; padding: 0 16px; font-size: 13px;">
        <CalendarArrowDown size={15} />
        {replanning ? 'Moving…' : `Move ${unfinishedCount} unfinished ${unfinishedCount === 1 ? 'objective' : 'objectives'} to next week`}
      </button>
    {/if}

    <!-- Contextual week-review summary -->
    {#if prefs.contextualReflections && reviewDone}
      <a href="/week/{weekStartDate}/review"
         class="mb-5 block px-5 py-4 transition-opacity active:opacity-70"
         style="background: var(--sempa-accent-bg); border-radius: 20px;">
        <div class="mb-1.5 flex items-center gap-2">
          <svg class="h-3.5 w-3.5 shrink-0" style="color: var(--sempa-accent-ink);" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
          </svg>
          <p class="type-label" style="color: var(--sempa-accent-ink);">Week reviewed</p>
        </div>
        {#if reviewWins.length}
          <p class="text-sm leading-relaxed" style="color: var(--sempa-text);">
            <span style="color: var(--sempa-text-soft);">Wins:</span> {reviewWins.join(' · ')}
          </p>
        {/if}
        {#if reviewNextFocus}
          <p class="mt-1 text-sm leading-relaxed" style="color: var(--sempa-text);">
            <span style="color: var(--sempa-text-soft);">Next focus:</span> {reviewNextFocus}
          </p>
        {/if}
      </a>
    {/if}

    <!-- Objectives list -->
    {#if objectives.length > 0}
      <h2 class="mb-2.5 px-1 type-label" style="color: var(--sempa-text-soft);">Objectives</h2>
    {/if}
    <div class="flex flex-col gap-3">
      {#each objectives as obj (obj.id)}
        {@const linked  = objectiveTasks(obj.id)}
        {@const done    = doneTasks(obj.id)}
        {@const p       = progressPct(obj.id)}
        {@const isExp   = expandedId === obj.id}
        {@const isDone  = obj.status === 'completed'}

        <div class="obj-card {dragObjId === obj.id ? 'opacity-50' : ''}"
             role="listitem"
             oncontextmenu={(e) => onObjContextMenu(e, obj)}
             ondragover={(e) => { e.preventDefault(); if (dragObjId && dragObjId !== obj.id) dragOverObjId = obj.id; }}
             ondragleave={() => { if (dragOverObjId === obj.id) dragOverObjId = null; }}
             ondrop={(e) => { e.preventDefault(); void onObjDrop(obj.id); }}
             style="border-radius: 20px; background: var(--sempa-bg-panel);
                    box-shadow: {dragOverObjId === obj.id ? '0 0 0 2px var(--sempa-accent), var(--sempa-shadow-1)' : 'var(--sempa-shadow-1)'};">

          <!-- Objective header row -->
          <div class="flex items-start gap-2.5 py-4 pl-2.5 pr-3 sm:pl-3">
            <!-- Drag handle (reorder) — only the handle is draggable so the card's
                 buttons and the expanded add-task input stay fully interactive. -->
            <div role="button" tabindex="-1" aria-label="Drag to reorder" title="Drag to reorder"
                 draggable="true"
                 ondragstart={(e) => onObjDragStart(e, obj.id)}
                 ondragend={onObjDragEnd}
                 class="obj-grip mt-0.5 shrink-0 cursor-grab touch-none active:cursor-grabbing"
                 style="color: var(--sempa-text-dim);"
                 onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text)'}
                 onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
              <GripVertical size={16} />
            </div>

            <!-- Completion circle -->
            <button onclick={() => toggleStatus(obj)} title="{isDone ? 'Mark active' : 'Mark complete'}"
                    class="check-ring" data-state={isDone ? 'done' : undefined}>
              {#if isDone}
                <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
              {/if}
            </button>

            <div class="flex-1 min-w-0">
              <p class="{isDone ? 'line-through' : ''}"
                 style="font-size: 15px; font-weight: 700; letter-spacing: -.01em; line-height: 1.35;
                        color: {isDone ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'}">
                {obj.title}{#if household.multiUser && obj.shared}<Users2 class="ml-1.5 inline h-3.5 w-3.5 align-[-2px]" style="color: var(--sempa-accent-ink);" aria-label="Shared with household" />{/if}
              </p>

              <!-- Task progress bar + label -->
              {#if linked.length > 0}
                <div class="mt-2.5 flex items-center gap-2.5">
                  <div class="day-bar-track flex-1" style="max-width: 220px;">
                    <div class="day-bar-fill"
                         style="width:{p}%; background: {isDone || p === 100 ? 'var(--sempa-success)' : 'var(--sempa-accent)'};"></div>
                  </div>
                  <span class="type-meta tabular-nums"
                        style="color: {isDone || p === 100 ? 'var(--sempa-success)' : 'var(--sempa-text-soft)'}">
                    {p}% · {done.length}/{linked.length} tasks
                  </span>
                </div>
              {:else}
                <p class="mt-1 type-meta" style="color: var(--sempa-text-dim);">No tasks linked yet</p>
              {/if}
            </div>

            <!-- Actions -->
            <div class="-mr-1 -mt-1.5 flex shrink-0 items-center">
              {#if !isDone}
                <button onclick={() => replanObjective(obj)} aria-label="Move to next week"
                        title="Move to next week"
                        class="icon-ghost"
                        style="color: var(--sempa-text-dim);"
                        onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-accent-ink)'}
                        onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
                  <CalendarArrowDown size={16} />
                </button>
              {/if}
              <button onclick={() => expandedId = isExp ? null : obj.id}
                      class="icon-ghost"
                      style="color: {isExp ? 'var(--sempa-text)' : 'var(--sempa-text-dim)'};"
                      aria-label="{isExp ? 'Collapse' : 'Expand'}">
                <svg class="h-4 w-4 transition-transform {isExp ? 'rotate-180' : ''}" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7"/>
                </svg>
              </button>
              <button onclick={() => deleteObjective(obj.id)} aria-label="Delete objective"
                      class="icon-ghost"
                      style="color: var(--sempa-text-dim);"
                      onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = '#f87171'}
                      onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}>
                <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
          </div>

          <!-- Expanded: task list + inline add -->
          {#if isExp}
            <div class="px-4 pb-4 pt-2 sm:pl-[3.25rem]" style="border-top: 1px solid var(--sempa-border);">
              {#each linked as t (t.id)}
                <div class="group flex items-center gap-2.5 py-2" style="border-bottom: 1px solid var(--sempa-border);">
                  <button onclick={() => toggleTask(t)}
                          class="check-ring" data-state={t.status === 'done' ? 'done' : undefined}
                          style="width: 20px; height: 20px;">
                    {#if t.status === 'done'}
                      <svg class="h-2.5 w-2.5" fill="none" stroke="currentColor" stroke-width="3.5" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                      </svg>
                    {/if}
                  </button>
                  <span class="flex-1 text-sm {t.status === 'done' ? 'line-through' : ''}"
                        style="color: {t.status === 'done' ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'}">
                    {t.title}
                  </span>
                  {#if linked.length > 0}
                    <span class="shrink-0 tabular-nums" style="font-size: 11.5px; color: var(--sempa-text-dim);">
                      {Math.round(100 / linked.length)}%
                    </span>
                  {/if}
                  {#if t.planned_date}
                    <span class="chip chip-time shrink-0">{t.planned_date.slice(5)}</span>
                  {/if}
                </div>
              {/each}

              <!-- Quick add task -->
              <div class="mt-3 space-y-2">
                <div class="add-field flex items-center gap-2 px-3"
                     style="min-height: 40px;">
                  <svg class="h-3.5 w-3.5 shrink-0" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path stroke-linecap="round" d="M12 4v16m8-8H4"/>
                  </svg>
                  <input bind:value={taskDrafts[obj.id]}
                         onkeydown={(e) => { if (e.key === 'Enter') addTask(obj.id); }}
                         type="text"
                         placeholder="Add a task… (Enter to save)"
                         class="flex-1 bg-transparent text-sm outline-none"
                         style="color: var(--sempa-text);" />
                </div>
                {#if taskDrafts[obj.id]?.trim()}
                  <div class="flex items-center gap-2">
                    <div class="min-w-[150px]">
                      <SempaSelect
                        value={taskDays[obj.id] ?? offsetDate(weekStartDate, 0)}
                        onchange={(v) => taskDays = { ...taskDays, [obj.id]: v as string }}
                        options={Array.from({length:7},(_,i)=>{ const d=offsetDate(weekStartDate,i); return { value: d, label: new Date(d+'T12:00:00').toLocaleDateString('en-US',{weekday:'short',month:'short',day:'numeric'}) }; })} />
                    </div>
                    <button onclick={() => addTask(obj.id)} disabled={addingTask[obj.id]}
                            class="btn-primary disabled:opacity-40"
                            style="height: 36px; padding: 0 16px; font-size: 13px;">
                      Add
                    </button>
                  </div>
                {/if}
              </div>

              <!-- Attachments for this objective -->
              <div class="mt-4 pt-3" style="border-top: 1px solid var(--sempa-border);">
                <AttachmentList ownerType="objective" ownerId={obj.id} />
              </div>
            </div>
          {/if}
        </div>
      {/each}

      <!-- Empty state -->
      {#if objectives.length === 0 && !showAddForm}
        <div class="surface relative overflow-hidden py-20 text-center" style="border-radius: 24px;">

          <div class="absolute inset-0 pointer-events-none z-0">
            <SempaPattern motif="rings" class="w-full h-full" opacity={0.9} />
          </div>

          <div class="relative z-10 flex flex-col items-center gap-3 px-6">
            <p class="type-section" style="color: var(--sempa-text);">No objectives yet</p>
            <p class="text-sm" style="color: var(--sempa-text-soft);">
              No objectives set for this week.
            </p>
            <a href="/week/{weekStartDate}/plan"
               class="btn-primary mt-1"
               style="height: 40px; padding: 0 20px; font-size: 14px;">
              Start weekly planning →
            </a>
          </div>
        </div>
      {/if}

      <!-- Inline add form -->
      {#if showAddForm}
        <div class="surface" style="border-radius: 20px; padding: 16px;">
          <input bind:this={addingInput}
                 bind:value={addingTitle}
                 onkeydown={(e) => { if (e.key === 'Enter') addObjective(); if (e.key === 'Escape') showAddForm = false; }}
                 onfocus={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'}
                 onblur={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'transparent'}
                 type="text"
                 placeholder="What do you want to accomplish this week?"
                 class="w-full px-3 py-2.5 text-sm outline-none"
                 style="border: 1px solid transparent; background: var(--sempa-fill); border-radius: 12px;
                        color: var(--sempa-text);" />
          <div class="mt-3 flex gap-2">
            <button onclick={addObjective} disabled={!addingTitle.trim()}
                    class="btn-primary disabled:opacity-40"
                    style="height: 36px; padding: 0 18px; font-size: 13px;">
              Add
            </button>
            <button onclick={() => showAddForm = false}
                    class="icon-ghost"
                    style="width: auto; padding: 0 14px; font-size: 13px; font-weight: 600; color: var(--sempa-text-soft);">
              Cancel
            </button>
          </div>
        </div>
      {/if}
    </div>

    <!-- Unscheduled tasks for this week -->
    {#if unscheduled.length > 0}
      <div class="surface mt-5" style="border-radius: 20px;">
        <button class="flex w-full items-center justify-between px-4 py-3.5"
                style="min-height: 48px;"
                onclick={() => showUnscheduled = !showUnscheduled}>
          <span style="font-size: 15px; font-weight: 700; letter-spacing: -.01em; color: var(--sempa-text);">Unscheduled this week</span>
          <div class="flex items-center gap-2">
            <span class="chip" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
              {unscheduled.length}
            </span>
            <svg class="h-4 w-4 transition-transform {showUnscheduled ? 'rotate-180' : ''}"
                 fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"
                 style="color: var(--sempa-text-dim);">
              <path stroke-linecap="round" d="M19 9l-7 7-7-7"/>
            </svg>
          </div>
        </button>
        {#if showUnscheduled}
          <div class="px-4 pb-3" style="border-top: 1px solid var(--sempa-border);">
            {#each unscheduled as t, i (t.id)}
              <div class="group flex items-center gap-3 py-2.5"
                   style={i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}>
                <button onclick={() => toggleTask(t)}
                        class="check-ring" data-state={t.status === 'done' ? 'done' : undefined}
                        style="width: 20px; height: 20px;">
                  {#if t.status === 'done'}
                    <svg class="h-2.5 w-2.5" fill="none" stroke="currentColor" stroke-width="3.5" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                    </svg>
                  {/if}
                </button>
                <span class="flex-1 text-sm {t.status === 'done' ? 'line-through' : ''}"
                      style="color: {t.status === 'done' ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'}">
                  {t.title}
                </span>
                <button onclick={() => scheduleToday(t)}
                        class="btn-soft opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
                        style="height: 28px; padding: 0 12px; font-size: 12px;"
                        title="Schedule to today">
                  Plan today
                </button>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  {/if}
</main>

<style>
  /* Header week arrows: round, quiet until hovered. */
  .wk-nav { width: 36px; height: 36px; }
  .wk-nav:hover { background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text); }
  @media (pointer: coarse) { .wk-nav { width: 40px; height: 40px; } }

  /* Ghost icon button that lives ON a surface: round, fill on hover. */
  .icon-ghost {
    display: inline-flex; align-items: center; justify-content: center;
    width: 36px; height: 36px; border-radius: 999px;
    transition: background-color 120ms ease, color 120ms ease;
  }
  .icon-ghost:hover { background: var(--sempa-fill); }

  .obj-card { transition: box-shadow 150ms ease, opacity 150ms ease; }
  .obj-card .obj-grip { opacity: .55; transition: opacity 120ms ease; }
  .obj-card:hover .obj-grip { opacity: 1; }
  @media (hover: none) { .obj-card .obj-grip { opacity: 1; } }

  /* Inline add-task field: fill on the surface, accent edge on focus. */
  .add-field {
    background: var(--sempa-fill);
    border: 1px solid transparent;
    border-radius: 12px;
    transition: border-color 120ms ease;
  }
  .add-field:focus-within { border-color: var(--sempa-accent); }
</style>
