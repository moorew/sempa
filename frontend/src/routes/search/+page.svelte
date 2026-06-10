<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import { today } from '$lib/utils';
  import type { SearchResults, Task } from '$lib/types';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { mobile } from '$lib/stores/mobile.svelte';
  import TagFilterBar from '$lib/components/TagFilterBar.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import { Search } from 'lucide-svelte';

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

  const total = $derived(results.tasks.length + results.objectives.length + results.journal.length);

  function openTask(t: Task) { panelTask = t; panelOpen = true; }
  function onPanelSave() { panelOpen = false; runSearch(); }

  function fmtDate(d: string): string {
    const dt = new Date(d + 'T12:00:00');
    return dt.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
  }
</script>

<svelte:head><title>Search — Sempa</title></svelte:head>

<header class="sticky top-0 z-[40] backdrop-blur-sm"
        style="background: color-mix(in srgb, var(--sempa-bg-main) 95%, transparent);
               border-bottom: 1px solid var(--sempa-border);
               padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px));">
  <div class="px-5 py-3">
    <p class="type-page mb-2" style="color: var(--sempa-text);">Search</p>

    <!-- Text input -->
    <div class="flex items-center gap-2 rounded-xl px-3"
         style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); height: 40px;">
      <Search size={16} style="color: var(--sempa-text-dim);" />
      <input bind:this={searchInput} bind:value={q}
             placeholder="Search tasks, objectives, journal…"
             autocomplete="off" autocorrect="off" autocapitalize="none" spellcheck="false"
             class="flex-1 bg-transparent outline-none"
             style="font-size: 14px; color: var(--sempa-text);" />
      {#if q}
        <button onclick={() => { q = ''; searchInput?.focus(); }} aria-label="Clear"
                style="color: var(--sempa-text-dim);">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      {/if}
    </div>

    <!-- Tag filter -->
    <div class="mt-2.5">
      <TagFilterBar bind:selected={tags} bind:match />
    </div>
  </div>
</header>

<main class="px-5 py-4 animate-fade-in" style="padding-bottom: {mobile.value ? '96px' : '32px'};">
  {#if !hasQuery}
    <div class="flex flex-col items-center justify-center gap-2 py-20 text-center">
      <Search size={28} style="color: var(--sempa-text-dim); opacity: .6;" />
      <p style="font-size: 13px; color: var(--sempa-text-dim);">
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
    <div class="flex flex-col gap-6">
      <!-- Tasks -->
      {#if results.tasks.length}
        <section>
          <div class="mb-2 flex items-center gap-2 px-1">
            <span class="type-label" style="color: var(--sempa-accent);">Tasks</span>
            <span class="type-label" style="color: var(--sempa-text-dim);">{results.tasks.length}</span>
          </div>
          <div style="border: 1px solid var(--sempa-border); border-radius: 12px; background: var(--sempa-bg-panel); overflow: hidden;">
            {#each results.tasks as task, i (task.id)}
              <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
              <button class="flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors"
                      style="{i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}"
                      onclick={() => openTask(task)}>
                {#if (task.tags ?? []).length}
                  <span class="flex shrink-0 items-center gap-1" title={(task.tags ?? []).join(', ')}>
                    {#each task.tags ?? [] as tg}
                      <span class="h-2 w-2 rounded-full" style="background: {tagStore.colorFor(tg)};"></span>
                    {/each}
                  </span>
                {/if}
                <span class="min-w-0 flex-1 truncate {task.status === 'done' ? 'line-through' : ''}"
                      style="font-size: 13.5px; color: var(--sempa-text);">{task.title}</span>
                <span class="shrink-0" style="font-size: 11px; color: var(--sempa-text-dim);">
                  {task.planned_date ? fmtDate(task.planned_date) : 'Backlog'}
                </span>
              </button>
            {/each}
          </div>
        </section>
      {/if}

      <!-- Objectives -->
      {#if results.objectives.length}
        <section>
          <div class="mb-2 flex items-center gap-2 px-1">
            <span class="type-label" style="color: var(--sempa-accent);">Objectives</span>
            <span class="type-label" style="color: var(--sempa-text-dim);">{results.objectives.length}</span>
          </div>
          <div style="border: 1px solid var(--sempa-border); border-radius: 12px; background: var(--sempa-bg-panel); overflow: hidden;">
            {#each results.objectives as obj, i (obj.id)}
              <button class="flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors"
                      style="{i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}"
                      onclick={() => goto(`/day/${obj.week_start}`)}>
                <span class="shrink-0">🎯</span>
                <span class="min-w-0 flex-1 truncate" style="font-size: 13.5px; color: var(--sempa-text);">{obj.title}</span>
                <span class="shrink-0" style="font-size: 11px; color: var(--sempa-text-dim);">Week of {fmtDate(obj.week_start)}</span>
              </button>
            {/each}
          </div>
        </section>
      {/if}

      <!-- Journal -->
      {#if results.journal.length}
        <section>
          <div class="mb-2 flex items-center gap-2 px-1">
            <span class="type-label" style="color: var(--sempa-accent);">Journal</span>
            <span class="type-label" style="color: var(--sempa-text-dim);">{results.journal.length}</span>
          </div>
          <div style="border: 1px solid var(--sempa-border); border-radius: 12px; background: var(--sempa-bg-panel); overflow: hidden;">
            {#each results.journal as hit, i (hit.kind + hit.date)}
              <button class="flex w-full flex-col gap-0.5 px-4 py-2.5 text-left transition-colors"
                      style="{i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}"
                      onclick={() => goto(hit.kind === 'daily' ? `/day/${hit.date}` : '/journal')}>
                <span style="font-size: 11px; color: var(--sempa-text-dim);">
                  {hit.kind === 'daily' ? fmtDate(hit.date) : `Week of ${fmtDate(hit.date)}`}
                </span>
                <span class="line-clamp-2" style="font-size: 13px; color: var(--sempa-text-soft);">{hit.snippet}</span>
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
