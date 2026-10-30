<script lang="ts">
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { today, weekStart, offsetDate } from '$lib/utils';
  import {
    parseJiraMeta,
    applyJiraFilters,
    JIRA_TOGGLE_DEFS,
    JIRA_SELECT_DEFS,
    optionsFor,
    activeSelectCount,
    defaultJiraFilterState,
  } from '$lib/jira/filters';
  import BottomSheet from '$lib/components/BottomSheet.svelte';
  import {
    ChevronLeft, Search, X, RefreshCw, SlidersHorizontal,
    CalendarPlus, CirclePlus, ExternalLink, SquareKanban,
  } from 'lucide-svelte';

  type SempaScope = 'unplanned' | 'planned' | 'all';

  let allTasks    = $state<Task[]>([]);
  let loading     = $state(true);
  let syncing     = $state(false);
  let connected   = $state(true);
  let error       = $state('');
  let scope       = $state<SempaScope>('unplanned');
  let showFilters = $state(false);

  let filterState = $state(defaultJiraFilterState());

  // Detail / plan sheet
  let sheetTask  = $state<Task | null>(null);
  let pickDate   = $state(today());
  let planning   = $state(false);

  const todayDate = today();

  $effect(() => { loadAll(); });

  async function loadAll() {
    loading = true; error = '';
    try {
      const cfg = await api.integrations.jira.get() as any;
      connected = cfg.connected ?? false;
      if (!connected) { loading = false; return; }
      allTasks = await api.tasks.listBySource('jira');
    } catch (e: any) {
      error = e.message ?? 'Failed to load Jira issues';
    } finally { loading = false; }
  }

  async function sync() {
    syncing = true; error = '';
    try {
      await api.integrations.jira.sync();
      await loadAll();
    } catch (e: any) { error = e.message ?? 'Sync failed'; }
    finally { syncing = false; }
  }

  const filteredTasks = $derived.by(() => {
    const scoped = allTasks.filter(t => {
      if (scope === 'unplanned' && t.status !== 'backlog') return false;
      if (scope === 'planned' && (t.status === 'backlog' || t.status === 'done')) return false;
      return true;
    });
    return applyJiraFilters(scoped, filterState);
  });

  const activeFilters = $derived(activeSelectCount(filterState));

  function openSheet(task: Task) {
    sheetTask = task;
    pickDate = task.planned_date && task.planned_date >= todayDate ? task.planned_date : todayDate;
  }

  // Plan an issue onto a date — identical mutation to the desktop drag-to-plan,
  // so the Jira-sourced task lands on the board exactly the same way.
  async function planFor(date: string) {
    if (!sheetTask) return;
    planning = true;
    const id = sheetTask.id;
    const prevStatus = sheetTask.status;
    try {
      const updated = await api.tasks.update(id, {
        planned_date: date,
        week_start: weekStart(date),
        status: prevStatus === 'backlog' ? 'planned' : prevStatus,
      });
      allTasks = allTasks.map(t => t.id === id ? updated : t);
      sheetTask = null;
    } catch (e: any) {
      error = e.message ?? 'Could not plan issue';
    } finally { planning = false; }
  }

  function priorityDot(priority?: string): string {
    if (priority === 'Highest' || priority === 'High') return 'bg-red-500';
    if (priority === 'Medium') return 'bg-yellow-400';
    if (priority === 'Low' || priority === 'Lowest') return 'bg-blue-400';
    return 'bg-gray-300 dark:bg-gray-600';
  }

  function plannedLabel(t: Task): string {
    if (!t.planned_date) return '';
    if (t.planned_date === todayDate) return 'Today';
    if (t.planned_date === offsetDate(todayDate, 1)) return 'Tomorrow';
    return new Date(t.planned_date + 'T00:00:00').toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' });
  }
</script>

<div class="flex h-full flex-col" style="background: var(--sempa-bg-main);">

  <!-- Header -->
  <header class="sticky top-0 z-[40]"
          style="background: var(--sempa-bg-main); padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px));">
    <div class="mx-auto max-w-3xl px-4 pb-3 pt-3 sm:px-6">
      <div class="flex items-center gap-2">
        <button onclick={() => history.back()} aria-label="Back"
                class="btn-icon shrink-0" style="width: 38px; height: 38px;">
          <ChevronLeft size={20} />
        </button>
        <h1 class="type-page ml-1 flex items-center gap-2" style="color: var(--sempa-text);">
          Jira
        </h1>
        {#if connected && !loading}
          <span class="chip ml-1" style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text-soft);">
            {filteredTasks.length} issue{filteredTasks.length === 1 ? '' : 's'}
          </span>
        {/if}
        <button onclick={sync} disabled={syncing || loading} aria-label="Sync Jira"
                class="btn-icon ml-auto shrink-0 disabled:opacity-40" style="width: 38px; height: 38px;">
          <RefreshCw size={16} class={syncing ? 'animate-spin' : ''} />
        </button>
      </div>

      {#if connected}
        <!-- Search -->
        <div class="search-pill mt-4">
          <Search size={16} class="shrink-0" style="color: var(--sempa-text-dim);" />
          <input bind:value={filterState.query} type="text" placeholder="Search key or title…"
                 class="min-w-0 flex-1 bg-transparent text-sm outline-none"
                 style="color: var(--sempa-text);" />
          {#if filterState.query}
            <button onclick={() => filterState.query = ''} aria-label="Clear search"
                    class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full"
                    style="background: var(--sempa-fill); color: var(--sempa-text-soft);">
              <X size={14} />
            </button>
          {/if}
        </div>

        <!-- Scope -->
        <div class="segmented mt-3" style="background: color-mix(in srgb, var(--sempa-text) 6%, transparent);" role="tablist" aria-label="Scope">
          {#each [['unplanned', 'Unplanned'], ['planned', 'Planned'], ['all', 'All']] as [val, label]}
            <button onclick={() => scope = val as SempaScope}
                    role="tab" aria-selected={scope === val}
                    class="py-2 text-[12.5px] font-semibold transition-colors"
                    style="color: {scope === val ? 'var(--sempa-text)' : 'var(--sempa-text-soft)'};">
              {label}
            </button>
          {/each}
        </div>

        <!-- Toggles + filters disclosure -->
        <div class="mt-3 flex flex-wrap items-center gap-1.5">
          {#each JIRA_TOGGLE_DEFS as def (def.id)}
            {@const on = filterState.toggles[def.id]}
            <button onclick={() => filterState.toggles[def.id] = !on} aria-pressed={on}
                    class="filter-chip" data-active={on}>
              {def.label}
            </button>
          {/each}
          <button onclick={() => showFilters = !showFilters}
                  class="filter-chip ml-auto flex items-center gap-1.5" data-active={activeFilters > 0}>
            <SlidersHorizontal size={13} />
            {activeFilters > 0 ? `Filters (${activeFilters})` : 'Filters'}
          </button>
        </div>

        {#if showFilters}
          <div class="mt-2.5 grid gap-2 sm:grid-cols-2">
            {#each JIRA_SELECT_DEFS as def (def.id)}
              {@const opts = optionsFor(def, allTasks)}
              {#if opts.length}
                <select bind:value={filterState.selects[def.id]}
                        class="w-full rounded-[12px] border px-3 py-2 text-sm"
                        style="border-color: var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);">
                  <option value="">{def.label}: any</option>
                  {#each opts as o}<option value={o}>{o}</option>{/each}
                </select>
              {/if}
            {/each}
          </div>
        {/if}
      {/if}
    </div>
  </header>

  <!-- List -->
  <div class="flex-1 overflow-y-auto" data-sheet-scroll>
    {#if !connected && !loading}
      <div class="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
        <span class="flex h-12 w-12 items-center justify-center rounded-2xl"
              style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text-dim);">
          <SquareKanban size={22} />
        </span>
        <p class="font-bold" style="font-size: 15px; color: var(--sempa-text);">Jira isn't connected</p>
        <a href="/settings/integrations/jira" class="btn-soft h-9 px-4 text-[13px]">Set up Jira →</a>
      </div>

    {:else if loading}
      <div class="mx-auto max-w-3xl px-4 py-2 sm:px-6">
        <div class="surface overflow-hidden" style="border-radius: 22px;">
          {#each Array(6) as _, i}
            <div class="flex items-start gap-3 px-4 py-3.5"
                 style={i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}>
              <div class="mt-0.5 h-4 w-14 shrink-0 rounded-full animate-pulse" style="background: var(--sempa-fill);"></div>
              <div class="flex-1 space-y-2">
                <div class="h-3 w-full rounded-full animate-pulse" style="background: var(--sempa-fill);"></div>
                <div class="h-2.5 w-1/2 rounded-full animate-pulse" style="background: var(--sempa-fill);"></div>
              </div>
            </div>
          {/each}
        </div>
      </div>

    {:else if error}
      <p class="mx-auto max-w-3xl px-6 py-6 text-sm" style="color: var(--sempa-danger, #c0392b);">{error}</p>

    {:else if filteredTasks.length === 0}
      <div class="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
        <p class="type-meta" style="color: var(--sempa-text-soft);">No issues match</p>
        {#if allTasks.length === 0}
          <button onclick={sync} class="btn-soft h-9 px-4 text-[13px]">Sync now</button>
        {/if}
      </div>

    {:else}
      <div class="mx-auto max-w-3xl px-4 pb-6 pt-2 sm:px-6">
        <ul class="surface overflow-hidden" style="border-radius: 22px;">
          {#each filteredTasks as task, i (task.id)}
            {@const meta = parseJiraMeta(task.source_metadata)}
            <li style={i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}>
              <button onclick={() => openSheet(task)}
                      class="jira-row flex w-full items-start gap-3 px-4 py-3.5 text-left">
                <div class="min-w-0 flex-1">
                  <div class="flex items-baseline gap-2">
                    {#if meta?.key}
                      <span class="shrink-0 font-mono text-[11.5px] font-semibold" style="color: var(--sempa-accent-ink);">{meta.key}</span>
                    {/if}
                    <span class="truncate text-sm font-medium" style="color: var(--sempa-text);">{task.title}</span>
                  </div>
                  <div class="type-meta mt-1 flex items-center gap-1.5" style="color: var(--sempa-text-dim);">
                    {#if meta?.priority}
                      <span class="h-1.5 w-1.5 shrink-0 rounded-full {priorityDot(meta.priority)}"></span>
                    {/if}
                    {#if meta?.status}<span class="truncate">{meta.status}</span>{/if}
                    {#if meta?.issueType}<span aria-hidden="true">·</span><span class="truncate">{meta.issueType}</span>{/if}
                  </div>
                </div>
                {#if task.planned_date && task.status !== 'backlog'}
                  <span class="chip shrink-0"
                        style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
                    {plannedLabel(task)}
                  </span>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      </div>
    {/if}
  </div>
</div>

<!-- Detail + plan sheet -->
<BottomSheet open={sheetTask !== null} onClose={() => { if (!planning) sheetTask = null; }}>
  {#if sheetTask}
    {@const meta = parseJiraMeta(sheetTask.source_metadata)}
    <div class="px-5 pb-6 pt-1" data-sheet-scroll>

      <!-- Plan actions FIRST (anchored at top for thumb reach) -->
      <div class="space-y-2.5">
        {#if sheetTask.planned_date && sheetTask.status !== 'backlog'}
          <p class="type-meta" style="color: var(--sempa-text-dim);">
            Planned for <span style="color: var(--sempa-accent-ink); font-weight: 600;">{plannedLabel(sheetTask)}</span> · choose a new day below
          </p>
        {/if}
        <div class="flex gap-2">
          <button onclick={() => planFor(todayDate)} disabled={planning}
                  class="btn-primary h-11 flex-1 gap-1.5 text-sm disabled:opacity-50">
            <CirclePlus size={16} /> {planning ? 'Adding…' : 'Add to today'}
          </button>
        </div>
        <div class="flex items-center gap-2">
          <span class="type-meta flex shrink-0 items-center gap-1.5" style="color: var(--sempa-text-dim);">
            <CalendarPlus size={15} /> Pick a day
          </span>
          <input type="date" bind:value={pickDate} min={todayDate}
                 class="min-w-0 flex-1 rounded-[12px] border px-3 py-2 text-sm dark:[color-scheme:dark]"
                 style="border-color: var(--sempa-border); background: var(--sempa-fill); color: var(--sempa-text);" />
          <button onclick={() => planFor(pickDate)} disabled={planning || !pickDate}
                  class="btn-soft h-10 shrink-0 px-4 text-sm disabled:opacity-50">
            Plan
          </button>
        </div>
      </div>

      <!-- Issue summary -->
      <div class="mt-5 rounded-[18px] p-4" style="background: var(--sempa-fill);">
        <div class="flex items-center gap-2">
          {#if meta?.key}<span class="font-mono text-sm font-semibold" style="color: var(--sempa-accent-ink);">{meta.key}</span>{/if}
          {#if sheetTask.source_url}
            <a href={sheetTask.source_url} target="_blank" rel="noopener noreferrer"
               class="ml-auto inline-flex items-center gap-1 text-xs font-semibold" style="color: var(--sempa-accent-ink);">Open in Jira <ExternalLink size={12} /></a>
          {/if}
        </div>
        <h2 class="mt-1.5 leading-snug" style="font-size: 16px; font-weight: 700; color: var(--sempa-text);">{sheetTask.title}</h2>
        <div class="mt-2.5 flex flex-wrap items-center gap-2 text-xs">
          {#if meta?.status}
            <span class="chip" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">{meta.status}</span>
          {/if}
          {#if meta?.priority}
            <span class="flex items-center gap-1" style="color: var(--sempa-text-dim);">
              <span class="h-2 w-2 rounded-full {priorityDot(meta.priority)}"></span>{meta.priority}
            </span>
          {/if}
          {#if meta?.issueType}<span style="color: var(--sempa-text-dim);">{meta.issueType}</span>{/if}
          {#if meta?.assignee}<span style="color: var(--sempa-text-dim);">· {meta.assignee}</span>{/if}
        </div>
        {#if meta?.epicName || meta?.epicKey}
          <p class="type-meta mt-2" style="color: var(--sempa-text-dim);">Epic: {meta.epicName || meta.epicKey}</p>
        {/if}
      </div>
    </div>
  {/if}
</BottomSheet>

<style>
  .search-pill {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 44px;
    padding: 0 8px 0 16px;
    border-radius: 999px;
    background: var(--sempa-bg-panel);
    box-shadow: var(--sempa-shadow-1);
  }
  /* The pill carries the focus ring; drop the global rectangular input ring. */
  .search-pill input:focus-visible { box-shadow: none; }
  .search-pill:focus-within { box-shadow: var(--sempa-shadow-1), 0 0 0 2px color-mix(in srgb, var(--sempa-accent) 30%, transparent); }
  .filter-chip {
    height: 32px;
    padding: 0 13px;
    border-radius: 999px;
    font-size: 12.5px;
    font-weight: 600;
    white-space: nowrap;
    background: var(--sempa-bg-panel);
    color: var(--sempa-text-soft);
    box-shadow: var(--sempa-shadow-1);
    transition: background-color 120ms ease, color 120ms ease;
  }
  .filter-chip:hover { color: var(--sempa-text); }
  .filter-chip[data-active="true"] {
    background: var(--sempa-accent-bg);
    color: var(--sempa-accent-ink);
    box-shadow: none;
  }
  .jira-row { transition: background-color 120ms ease; }
  .jira-row:hover, .jira-row:active { background: color-mix(in srgb, var(--sempa-text) 3%, transparent); }
</style>
