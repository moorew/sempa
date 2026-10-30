<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { today, weekStart, formatMinutes } from '$lib/utils';
  import { tagStore } from '$lib/stores/tags.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import { Plus, Search } from 'lucide-svelte';
  import { mobile } from '$lib/stores/mobile.svelte';
  import { realtime } from '$lib/stores/realtime.svelte';
  import { jiraStatus } from '$lib/stores/jiraStatus.svelte';

  let tasks   = $state<Task[]>([]);
  let loading = $state(true);
  let error   = $state<string | null>(null);

  let panelOpen = $state(false);
  let panelTask = $state<Task | null>(null);

  // Controls
  let search   = $state('');
  let filter   = $state<'all' | 'jira' | 'personal'>('all');
  let sortNewest = $state(false); // default: oldest first

  // The Jira chip is pointless without the integration; "All" still shows any
  // historical Jira-sourced tasks. Derived (not an effect) so a stale `filter`
  // can't wedge the page — see activeFilter below.
  const filterChips = $derived([
    { k: 'all', l: 'All' },
    ...(jiraStatus.connected ? [{ k: 'jira', l: 'Jira' }] : []),
    { k: 'personal', l: 'Personal' },
  ]);
  const activeFilter = $derived(
    filter === 'jira' && !jiraStatus.connected ? 'all' : filter,
  );

  onMount(load);

  $effect(() => {
    const ev = realtime.lastEvent;
    if (!ev) return;
    if (ev.type === 'task:change') void load();
  });

  async function load() {
    loading = true; error = null;
    try { tasks = await api.tasks.listBacklog(); }
    catch (e) { error = e instanceof Error ? e.message : 'Failed to load'; }
    finally { loading = false; }
  }

  async function scheduleToday(id: string) {
    const d  = today();
    const ws = weekStart(d);
    tasks = tasks.filter(t => t.id !== id);
    try { await api.tasks.update(id, { planned_date: d, week_start: ws, status: 'planned' }); }
    catch { await load(); }
  }

  // Park the item in this week without pinning a day: assign the current
  // week_start (so it shows under "Unscheduled this week") but leave it
  // undated, distinct from "Today" which pins planned_date to today.
  async function scheduleThisWeek(id: string) {
    const ws = weekStart(today());
    tasks = tasks.filter(t => t.id !== id);
    try { await api.tasks.update(id, { week_start: ws, status: 'planned', planned_date: null }); }
    catch { await load(); }
  }

  async function complete(id: string) {
    tasks = tasks.filter(t => t.id !== id);
    try { await api.tasks.update(id, { status: 'done', completed_at: new Date().toISOString() }); }
    catch { await load(); }
  }

  async function remove(id: string) {
    tasks = tasks.filter(t => t.id !== id);
    try { await api.tasks.delete(id); }
    catch { await load(); }
  }

  function openCreate() { panelTask = null; panelOpen = true; }
  function openEdit(t: Task) { panelTask = t; panelOpen = true; }

  async function handlePanelSave(saved: Task) {
    panelOpen = false;
    if (saved.status === 'cancelled') { tasks = tasks.filter(t => t.id !== saved.id); return; }
    if (saved.planned_date) { tasks = tasks.filter(t => t.id !== saved.id); return; }
    if (saved.status === 'done') { tasks = tasks.filter(t => t.id !== saved.id); return; }
    const idx = tasks.findIndex(t => t.id === saved.id);
    if (idx >= 0) tasks = tasks.map(t => t.id === saved.id ? saved : t);
    else tasks = [...tasks, saved];
  }

  const sourceLabel: Record<string, string> = {
    gmail: 'Gmail', fastmail: 'Mail', jira: 'Jira', google_calendar: 'Cal',
  };

  // Age since the task was added, in compact d/w units (e.g. 5d, 3w).
  function ageLabel(createdAt: string | null | undefined): string {
    if (!createdAt) return '';
    const then = new Date(createdAt).getTime();
    if (Number.isNaN(then)) return '';
    const days = Math.max(0, Math.floor((Date.now() - then) / 86400000));
    if (days < 1) return 'today';
    if (days < 7) return `${days}d`;
    return `${Math.floor(days / 7)}w`;
  }

  // The group a task belongs to: its source, or "personal" for manual tasks.
  function groupKey(t: Task): string {
    return t.source && t.source !== 'manual' ? t.source : 'personal';
  }
  function groupTitle(key: string): string {
    return key === 'personal' ? 'Personal' : (sourceLabel[key] ?? key);
  }

  // Oldest item's age, for the subtitle ("oldest 8w").
  const oldestAge = $derived.by(() => {
    const stamps = tasks.map(t => t.created_at ? new Date(t.created_at).getTime() : NaN).filter(n => !Number.isNaN(n));
    if (!stamps.length) return '';
    return ageLabel(new Date(Math.min(...stamps)).toISOString());
  });

  // Filter + search, then group by source, then sort within each group by age.
  const filtered = $derived.by(() => {
    const q = search.trim().toLowerCase();
    return tasks.filter(t => {
      if (q && !t.title.toLowerCase().includes(q)) return false;
      if (activeFilter === 'jira') return t.source === 'jira';
      if (activeFilter === 'personal') return groupKey(t) === 'personal';
      return true;
    });
  });

  const groups = $derived.by(() => {
    const byKey = new Map<string, Task[]>();
    for (const t of filtered) {
      const k = groupKey(t);
      if (!byKey.has(k)) byKey.set(k, []);
      byKey.get(k)!.push(t);
    }
    const dir = sortNewest ? -1 : 1;
    const sortByAge = (a: Task, b: Task) =>
      dir * (a.created_at ?? '').localeCompare(b.created_at ?? '');
    // Stable group order: known sources first (jira), then personal, then others.
    const order = (k: string) => (k === 'jira' ? 0 : k === 'personal' ? 2 : 1);
    return [...byKey.entries()]
      .map(([key, items]) => ({ key, items: [...items].sort(sortByAge) }))
      .sort((a, b) => order(a.key) - order(b.key) || a.key.localeCompare(b.key));
  });
</script>

<svelte:head><title>Backlog — Sempa</title></svelte:head>

<header class="sticky top-0 z-[40] backdrop-blur-sm"
        style="background: color-mix(in srgb, var(--sempa-bg-main) 95%, transparent);
               padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px));">
  <div class="mx-auto flex max-w-3xl flex-wrap items-end justify-between gap-3 px-6 pb-3 pt-4">
    <div class="min-w-0 flex-1">
      <p class="type-page" style="color: var(--sempa-text);">Backlog</p>
      <p class="type-meta mt-1" style="color: var(--sempa-text-dim);">
        {tasks.length} item{tasks.length !== 1 ? 's' : ''} waiting to be scheduled{oldestAge ? ` · oldest ${oldestAge}` : ''}
      </p>
    </div>

    <!-- Add (a round icon button on phones so it stays beside the title) -->
    {#if mobile.value}
      <button onclick={openCreate} class="btn-primary h-11 w-11 shrink-0 p-0" aria-label="Add to backlog" title="Add to backlog">
        <Plus size={18} strokeWidth={2.5} />
      </button>
    {:else}
      <button onclick={openCreate} class="btn-primary h-10 shrink-0 px-4 text-[13px]">
        <Plus size={15} strokeWidth={2.5} />
        Add to backlog
      </button>
    {/if}

    <div class="flex w-full flex-wrap items-center gap-2">
      <!-- Search -->
      <label class="search-pill {mobile.value ? 'w-full' : 'w-56'}">
        <Search size={15} style="color: var(--sempa-text-dim);" />
        <input bind:value={search} placeholder="Search backlog"
               class="min-w-0 flex-1 bg-transparent outline-none"
               style="font-size: 13px; color: var(--sempa-text);" />
      </label>

      <!-- Filter chips -->
      <div class="flex items-center gap-1.5">
        {#each filterChips as chip}
          {@const active = activeFilter === chip.k}
          <button onclick={() => filter = chip.k as typeof filter}
                  class="filter-chip" data-active={active} aria-pressed={active}>
            {chip.l}
          </button>
        {/each}
      </div>

      <!-- Sort -->
      <button onclick={() => sortNewest = !sortNewest} class="filter-chip ml-auto">
        {sortNewest ? 'Newest first' : 'Oldest first'}
      </button>
    </div>
  </div>
</header>

<main class="mx-auto max-w-3xl px-6 pb-8 pt-3 animate-fade-in">
  {#if loading}
    <div class="flex h-48 items-center justify-center text-sm" style="color: var(--sempa-text-dim);">Loading…</div>

  {:else if error}
    <div class="surface flex items-center gap-3 px-4 py-3 text-sm" style="color: var(--sempa-danger, #c0392b);">
      <span class="flex-1">{error}</span>
      <button onclick={load} class="btn-soft h-8 px-3.5 text-[12px]">Retry</button>
    </div>

  {:else if tasks.length === 0}
    <div class="surface flex flex-col items-center justify-center gap-3 py-16" style="border-radius: 24px;">
      <div class="flex h-12 w-12 items-center justify-center rounded-2xl"
           style="background: var(--sempa-accent-bg);">
        <svg class="h-6 w-6" style="color: var(--sempa-accent-ink);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2"/>
        </svg>
      </div>
      <p class="font-bold" style="font-size: 15px; color: var(--sempa-text);">Your backlog is clear.</p>
      <button onclick={openCreate} class="btn-primary h-10 px-5 text-[13px]">
        Add a task
      </button>
    </div>

  {:else if filtered.length === 0}
    <div class="flex flex-col items-center justify-center py-20 text-center" style="color: var(--sempa-text-dim);">
      <p class="text-sm">No matching items.</p>
    </div>

  {:else}
    <div class="flex flex-col gap-7">
      {#each groups as group (group.key)}
        <section>
          <!-- Group label -->
          <div class="mb-2.5 flex items-center gap-2 px-1">
            <span style="font-size: 15px; font-weight: 800; letter-spacing: -.015em; color: var(--sempa-text);">{groupTitle(group.key)}</span>
            <span class="chip" style="height: 20px; padding: 0 7px; background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text-soft);">{group.items.length}</span>
          </div>

          <!-- One raised surface; rows divided by a hairline -->
          <div class="surface overflow-hidden" style="border-radius: 22px;">
            {#each group.items as task, i (task.id)}
              <div class="group/row flex gap-3 px-4 transition-colors
                          {mobile.value ? 'flex-col py-3' : 'items-center'}"
                   style="min-height: {mobile.value ? '44px' : '48px'}; {i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}"
                   onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'color-mix(in srgb, var(--sempa-text) 3%, transparent)'}
                   onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = ''}>

                <div class="flex min-w-0 flex-1 items-center gap-3">
                  <!-- Complete circle -->
                  <button onclick={() => complete(task.id)}
                          class="check-ring cursor-pointer"
                          title="Mark done" aria-label="Mark done"></button>

                  <!-- Title (single line, truncates) -->
                  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
                  <button class="min-w-0 flex-1 truncate text-left type-body cursor-pointer"
                          style="color: var(--sempa-text);" onclick={() => openEdit(task)}>
                    {task.title}
                  </button>

                  {#if mobile.value && task.time_estimate_minutes}
                    <span class="chip chip-time shrink-0">{formatMinutes(task.time_estimate_minutes)}</span>
                  {/if}
                </div>

                <!-- Right side: estimate + age, swapped for quick actions on hover.
                     Mobile: a second line under the title, actions always shown. -->
                <div class="flex shrink-0 items-center gap-2 {mobile.value ? 'pl-[34px]' : ''}">
                  <!-- meta (hidden on row hover, desktop only) -->
                  <div class="flex items-center gap-2 {mobile.value ? '' : 'group-hover/row:hidden'}">
                    {#if task.time_estimate_minutes && !mobile.value}
                      <span class="chip chip-time">
                        {formatMinutes(task.time_estimate_minutes)}
                      </span>
                    {/if}
                    <span class="type-meta" style="color: var(--sempa-text-dim);">{ageLabel(task.created_at)}</span>
                  </div>

                  <!-- quick actions (desktop: hover only; mobile: always) -->
                  <div class="items-center gap-1.5 {mobile.value ? 'flex flex-1 justify-end' : 'hidden group-hover/row:flex'}">
                    <button onclick={() => scheduleToday(task.id)}
                            class="btn-soft px-3 text-[12px]" style="height: {mobile.value ? '32px' : '28px'};">
                      Today
                    </button>
                    <button onclick={() => scheduleThisWeek(task.id)}
                            class="btn-soft px-3 text-[12px]" style="height: {mobile.value ? '32px' : '28px'};">
                      This week
                    </button>
                    <button onclick={() => remove(task.id)}
                            class="flex items-center justify-center rounded-full transition-colors"
                            style="width: {mobile.value ? '32px' : '28px'}; height: {mobile.value ? '32px' : '28px'}; color: var(--sempa-text-dim);"
                            onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-danger, #c0392b)'}
                            onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-dim)'}
                            title="Delete" aria-label="Delete">
                      <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                        <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
                      </svg>
                    </button>
                  </div>
                </div>
              </div>
            {/each}
          </div>
        </section>
      {/each}
    </div>
  {/if}
</main>

<TaskPanel open={panelOpen} task={panelTask} defaultStatus="backlog" defaultDate={today()}
           onSave={handlePanelSave} onClose={() => panelOpen = false} />

<style>
  /* Pill search field. On the canvas it sits as a raised pill (a fill would
     vanish into the page); the input itself stays transparent. */
  .search-pill {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 38px;
    padding: 0 14px;
    border-radius: 999px;
    background: var(--sempa-bg-panel);
    box-shadow: var(--sempa-shadow-1);
    cursor: text;
  }
  /* The pill carries the focus ring; drop the global rectangular input ring. */
  .search-pill input:focus-visible { box-shadow: none; }
  .search-pill:focus-within { box-shadow: var(--sempa-shadow-1), 0 0 0 2px color-mix(in srgb, var(--sempa-accent) 30%, transparent); }
  .filter-chip {
    height: 34px;
    padding: 0 14px;
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
</style>
