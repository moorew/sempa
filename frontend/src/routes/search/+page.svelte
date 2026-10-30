<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import { today } from '$lib/utils';
  import type { SearchResults, Task } from '$lib/types';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { projectStore } from '$lib/stores/projects.svelte';
  import { mobile } from '$lib/stores/mobile.svelte';
  import TagFilterBar from '$lib/components/TagFilterBar.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import { downloadFile, tasksToCSV, tasksToMarkdown, slugify } from '$lib/export';
  import { Search, FileSpreadsheet, FileText, Target } from 'lucide-svelte';

  let q = $state('');
  let tags = $state<string[]>([]);
  let match = $state<'any' | 'all'>('any');

  let results = $state<SearchResults>({ tasks: [], objectives: [], journal: [] });
  let loading = $state(false);
  let ran = $state(false); // a query has been executed at least once

  let panelOpen = $state(false);
  let panelTask = $state<Task | null>(null);

  let searchInput: HTMLInputElement | undefined = $state();
  let debounce: ReturnType<typeof setTimeout> | undefined;
  let reqSeq = 0;

  onMount(() => {
    tagStore.load();
    projectStore.load();
    // Seed from ?q= so the command palette (and shared links) can prefill a query.
    const seed = new URLSearchParams(location.search).get('q');
    if (seed) q = seed;
    if (!mobile.value) setTimeout(() => searchInput?.focus(), 40);
  });

  const hasQuery = $derived(q.trim().length > 0 || tags.length > 0);

  // Re-run (debounced) whenever the query, tags or match mode change.
  $effect(() => {
    // touch deps
    void q; void tags; void match;
    clearTimeout(debounce);
    if (!hasQuery) {
      results = { tasks: [], objectives: [], journal: [] };
      ran = false;
      return;
    }
    debounce = setTimeout(runSearch, 220);
  });

  async function runSearch() {
    const seq = ++reqSeq;
    loading = true;
    try {
      const res = await api.search(q, tags, match);
      if (seq === reqSeq) { results = res; ran = true; }
    } catch {
      if (seq === reqSeq) { results = { tasks: [], objectives: [], journal: [] }; ran = true; }
    } finally {
      if (seq === reqSeq) loading = false;
    }
  }

  // Projects are matched client-side from the (local-first) project store, so
  // they're searchable offline too. Like objectives, they have no tags.
  const projectHits = $derived.by(() => {
    const needle = q.trim().toLowerCase();
    if (!needle || tags.length > 0) return [];
    return projectStore.projects.filter((p) =>
      p.title.toLowerCase().includes(needle) || (p.description ?? '').toLowerCase().includes(needle));
  });

  const total = $derived(results.tasks.length + results.objectives.length + results.journal.length + projectHits.length);

  function openTask(t: Task) { panelTask = t; panelOpen = true; }
  function onPanelSave() { panelOpen = false; runSearch(); }

  // ── Export the filtered task list (CSV or clean Markdown) ──────────────────
  // A human-readable label for the current filter, used as the file title.
  const exportLabel = $derived(
    tags.length
      ? `Tasks tagged ${tags.join(', ')}`
      : q.trim()
        ? `Tasks matching “${q.trim()}”`
        : 'Tasks'
  );
  const exportSlug = $derived(slugify(tags.length ? tags.join('-') : q.trim()));

  function exportCSV() {
    downloadFile(`tasks-${exportSlug}.csv`, tasksToCSV(results.tasks), 'text/csv');
  }
  function exportMarkdown() {
    downloadFile(`tasks-${exportSlug}.md`, tasksToMarkdown(results.tasks, exportLabel), 'text/markdown');
  }

  function fmtDate(d: string): string {
    const dt = new Date(d + 'T12:00:00');
    return dt.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
  }
</script>

<svelte:head><title>Search — Sempa</title></svelte:head>

<header class="sticky top-0 z-[40] backdrop-blur-sm"
        style="background: color-mix(in srgb, var(--sempa-bg-main) 95%, transparent);
               padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px));">
  <div class="mx-auto max-w-3xl px-5 pb-3 pt-4">
    <p class="type-page mb-3" style="color: var(--sempa-text);">Search</p>

    <!-- Text input: a raised pill on the canvas -->
    <div class="search-pill">
      <Search size={17} style="color: var(--sempa-text-dim);" />
      <input bind:this={searchInput} bind:value={q}
             placeholder="Search tasks, projects, objectives, journal…"
             autocomplete="off" autocorrect="off" autocapitalize="none" spellcheck="false"
             class="min-w-0 flex-1 bg-transparent outline-none"
             style="font-size: 14px; color: var(--sempa-text);" />
      {#if q}
        <button onclick={() => { q = ''; searchInput?.focus(); }} aria-label="Clear"
                class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full"
                style="background: var(--sempa-fill); color: var(--sempa-text-soft);">
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      {/if}
    </div>

    <!-- Tag filter -->
    <div class="mt-3">
      <TagFilterBar bind:selected={tags} bind:match />
    </div>
  </div>
</header>

<main class="mx-auto max-w-3xl px-5 pt-3 animate-fade-in" style="padding-bottom: {mobile.value ? '96px' : '32px'};">
  {#if !hasQuery}
    <div class="flex flex-col items-center justify-center gap-3 py-20 text-center">
      <span class="flex h-12 w-12 items-center justify-center rounded-2xl"
            style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text-dim);">
        <Search size={22} />
      </span>
      <p class="type-meta" style="color: var(--sempa-text-soft);">
        Search across all your tasks, objectives and journal entries.<br/>Filter by tag to focus on, say, work or personal.
      </p>
    </div>
  {:else if loading && !ran}
    <div class="py-16 text-center" style="font-size: 13px; color: var(--sempa-text-dim);">Searching…</div>
  {:else if total === 0}
    <div class="py-16 text-center" style="font-size: 13px; color: var(--sempa-text-dim);">
      No matches{q ? ` for “${q}”` : ''}{tags.length ? ` with ${match === 'all' ? 'all' : 'any'} of the selected tags` : ''}.
    </div>
  {:else}
    <div class="flex flex-col gap-7">
      <!-- Tasks -->
      {#if results.tasks.length}
        <section>
          <div class="mb-2.5 flex items-center gap-2 px-1">
            {@render groupLabel('Tasks', results.tasks.length)}
            <!-- Export the filtered list. Shown whenever there are task results;
                 most useful when narrowed to a single tag (e.g. "Tailscale"). -->
            <div class="ml-auto flex items-center gap-1.5">
              <button onclick={exportCSV} title="Export as CSV"
                      class="btn-surface h-8 gap-1.5 px-3 text-[11.5px]" style="color: var(--sempa-text-soft);">
                <FileSpreadsheet size={13} /> CSV
              </button>
              <button onclick={exportMarkdown} title="Export as Markdown"
                      class="btn-surface h-8 gap-1.5 px-3 text-[11.5px]" style="color: var(--sempa-text-soft);">
                <FileText size={13} /> Markdown
              </button>
            </div>
          </div>
          <div class="surface overflow-hidden" style="border-radius: 22px;">
            {#each results.tasks as task, i (task.id)}
              <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
              <button class="result-row"
                      style="{i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}"
                      onclick={() => openTask(task)}>
                <span class="check-ring" style="width: 18px; height: 18px;" data-state={task.status === 'done' ? 'done' : undefined} aria-hidden="true">
                  {#if task.status === 'done'}
                    <svg class="h-2.5 w-2.5" fill="none" stroke="currentColor" stroke-width="3.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>
                  {/if}
                </span>
                <span class="min-w-0 flex-1 truncate {task.status === 'done' ? 'line-through' : ''}"
                      style="font-size: 14px; color: var(--sempa-text); {task.status === 'done' ? 'opacity: .6;' : ''}">{task.title}</span>
                {#if (task.tags ?? []).length}
                  <span class="flex shrink-0 items-center gap-1" title={(task.tags ?? []).join(', ')}>
                    {#each task.tags ?? [] as tg}
                      <span class="h-2 w-2 rounded-full" style="background: {tagStore.colorFor(tg)};"></span>
                    {/each}
                  </span>
                {/if}
                <span class="type-meta shrink-0" style="color: var(--sempa-text-dim);">
                  {task.planned_date ? fmtDate(task.planned_date) : 'Backlog'}
                </span>
              </button>
            {/each}
          </div>
        </section>
      {/if}

      <!-- Projects -->
      {#if projectHits.length}
        <section>
          <div class="mb-2.5 flex items-center gap-2 px-1">
            {@render groupLabel('Projects', projectHits.length)}
          </div>
          <div class="surface overflow-hidden" style="border-radius: 22px;">
            {#each projectHits as p, i (p.id)}
              <button class="result-row"
                      style="{i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}"
                      onclick={() => goto(`/projects/${p.id}`)}>
                <span class="tint flex h-7 w-7 shrink-0 items-center justify-center rounded-[10px]" style="--tint: {p.color};">
                  <span class="h-2.5 w-2.5 rounded-full" style="background: {p.color};"></span>
                </span>
                <span class="min-w-0 flex-1 truncate font-medium" style="font-size: 14px; color: var(--sempa-text);">{p.title || 'Untitled project'}</span>
                <span class="type-meta shrink-0" style="color: var(--sempa-text-dim);">{projectStore.status(p.status_id)?.name ?? ''}</span>
              </button>
            {/each}
          </div>
        </section>
      {/if}

      <!-- Objectives -->
      {#if results.objectives.length}
        <section>
          <div class="mb-2.5 flex items-center gap-2 px-1">
            {@render groupLabel('Objectives', results.objectives.length)}
          </div>
          <div class="surface overflow-hidden" style="border-radius: 22px;">
            {#each results.objectives as obj, i (obj.id)}
              <button class="result-row"
                      style="{i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}"
                      onclick={() => goto(`/day/${obj.week_start}`)}>
                <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-[10px]"
                      style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
                  <Target size={15} />
                </span>
                <span class="min-w-0 flex-1 truncate font-medium" style="font-size: 14px; color: var(--sempa-text);">{obj.title}</span>
                <span class="type-meta shrink-0" style="color: var(--sempa-text-dim);">Week of {fmtDate(obj.week_start)}</span>
              </button>
            {/each}
          </div>
        </section>
      {/if}

      <!-- Journal -->
      {#if results.journal.length}
        <section>
          <div class="mb-2.5 flex items-center gap-2 px-1">
            {@render groupLabel('Journal', results.journal.length)}
          </div>
          <div class="surface overflow-hidden" style="border-radius: 22px;">
            {#each results.journal as hit, i (hit.kind + hit.date)}
              <button class="result-row flex-col !items-start !gap-1"
                      style="{i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}"
                      onclick={() => goto(hit.kind === 'daily' ? `/day/${hit.date}` : '/journal')}>
                <span class="type-label" style="color: var(--sempa-accent-ink);">
                  {hit.kind === 'daily' ? fmtDate(hit.date) : `Week of ${fmtDate(hit.date)}`}
                </span>
                <span class="line-clamp-2" style="font-size: 13.5px; line-height: 1.45; color: var(--sempa-text-soft);">{hit.snippet}</span>
              </button>
            {/each}
          </div>
        </section>
      {/if}
    </div>
  {/if}
</main>

<TaskPanel open={panelOpen} task={panelTask} defaultStatus="backlog" defaultDate={today()}
           onSave={onPanelSave} onClose={() => panelOpen = false} />

{#snippet groupLabel(text: string, count: number)}
  <span style="font-size: 15px; font-weight: 800; letter-spacing: -.015em; color: var(--sempa-text);">{text}</span>
  <span class="chip" style="height: 20px; padding: 0 7px; background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text-soft);">{count}</span>
{/snippet}

<style>
  .search-pill {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 46px;
    padding: 0 8px 0 16px;
    border-radius: 999px;
    background: var(--sempa-bg-panel);
    box-shadow: var(--sempa-shadow-1);
  }
  /* The pill carries the focus ring; drop the global rectangular input ring. */
  .search-pill input:focus-visible { box-shadow: none; }
  .search-pill:focus-within { box-shadow: var(--sempa-shadow-1), 0 0 0 2px color-mix(in srgb, var(--sempa-accent) 30%, transparent); }
  .result-row {
    display: flex;
    width: 100%;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    text-align: left;
    transition: background-color 120ms ease;
  }
  .result-row:hover { background: color-mix(in srgb, var(--sempa-text) 3%, transparent); }
</style>
