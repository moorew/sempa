<script lang="ts">
  import { api } from '$lib/api';
  import type { Project, Task } from '$lib/types';
  import { realtime } from '$lib/stores/realtime.svelte';
  import { projectStore, PROJECT_COLORS } from '$lib/stores/projects.svelte';
  import { projectProgress, relativeDue } from '$lib/projectProgress';
  import { today, formatShortDate } from '$lib/utils';
  import { goto } from '$app/navigation';
  import { FolderKanban, Plus } from 'lucide-svelte';

  let newTitle = $state('');
  let showClosed = $state(false);
  // Top-level tasks per project, for the progress bars.
  let tasksByProject = $state<Record<string, Task[]>>({});
  const todayStr = today();

  async function loadTasks(projects: Project[]) {
    const entries = await Promise.all(projects.map(async (p) => {
      try { return [p.id, await api.projects.tasks(p.id)] as const; }
      catch { return [p.id, [] as Task[]] as const; }
    }));
    tasksByProject = Object.fromEntries(entries);
  }

  $effect(() => { projectStore.load(); });
  // Re-read on any change (a task moved between projects, a status edit, …).
  $effect(() => {
    void realtime.lastEvent;
    void projectStore.projects;
    loadTasks(projectStore.projects);
  });

  const visible = $derived(projectStore.projects.filter((p) => {
    const c = projectStore.categoryOf(p);
    return showClosed || (c !== 'done' && c !== 'archived');
  }));
  const closedCount = $derived(projectStore.projects.length - projectStore.open.length);

  // Group by status, in the user's status order.
  const groups = $derived(projectStore.statuses
    .map((s) => ({ status: s, projects: visible.filter((p) => p.status_id === s.id) }))
    .filter((g) => g.projects.length > 0)
    .concat((() => {
      // Projects whose status isn't known locally yet (not synced) — still show them.
      const known = new Set(projectStore.statuses.map((s) => s.id));
      const orphans = visible.filter((p) => !known.has(p.status_id));
      return orphans.length ? [{ status: { id: '', name: 'Other', color: '#8a8178' } as never, projects: orphans }] : [];
    })()));

  async function create() {
    const title = newTitle.trim();
    if (!title) return;
    newTitle = '';
    try {
      const color = PROJECT_COLORS[projectStore.projects.length % PROJECT_COLORS.length];
      const p = await api.projects.create({ title, color });
      await projectStore.reload();
      goto(`/projects/${p.id}`);
    } catch { /* ignore */ }
  }
</script>

<svelte:head><title>Projects — Sempa</title></svelte:head>

<div class="mx-auto flex h-full max-w-3xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <div class="flex items-end gap-3 px-5 pb-2 pt-7">
    <div class="min-w-0 flex-1">
      <h1 class="type-page" style="color: var(--sempa-text);">Projects</h1>
      {#if projectStore.loaded}
        <p class="type-meta mt-1" style="color: var(--sempa-text-dim);">
          {projectStore.open.length} open{closedCount > 0 ? ` · ${closedCount} done or archived` : ''}
        </p>
      {/if}
    </div>
    <a href="/settings/projects" class="btn-surface h-9 px-4 text-[12.5px]">Statuses</a>
  </div>

  <div class="flex-1 overflow-y-auto px-5 pb-24 pt-4">
    <!-- New project: one raised pill with the input and a round add button. -->
    <form class="new-pill mb-7 flex items-center gap-2 rounded-full py-1.5 pl-4 pr-1.5"
          style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1);"
          onsubmit={(e) => { e.preventDefault(); create(); }}>
      <FolderKanban size={17} class="shrink-0" style="color: var(--sempa-text-dim);" />
      <input bind:value={newTitle} placeholder="New project (e.g. Renovate the kitchen)…"
        class="min-w-0 flex-1 bg-transparent py-1.5 text-sm outline-none"
        style="color: var(--sempa-text);" />
      <button type="submit" disabled={!newTitle.trim()}
        class="btn-primary h-9 shrink-0 px-4 text-[13px] disabled:opacity-40">
        <Plus size={16} strokeWidth={2.5} /> Add
      </button>
    </form>

    {#if !projectStore.loaded}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else if projectStore.projects.length === 0}
      <div class="surface flex flex-col items-center px-8 py-10 text-center" style="border-radius: 24px;">
        <span class="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl"
              style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
          <FolderKanban size={22} />
        </span>
        <p class="font-bold" style="font-size: 15px; color: var(--sempa-text);">No projects yet</p>
        <p class="type-meta mt-1 max-w-sm" style="color: var(--sempa-text-soft);">
          A project holds longer work — months, not days. Give it a description and a target date, then file
          tasks, weekly objectives and lists into it.
        </p>
      </div>
    {:else}
      {#each groups as g (g.status.id)}
        <section class="mb-7">
          <p class="type-label mb-2.5 flex items-center gap-2 px-1" style="color: var(--sempa-text-soft);">
            <span class="h-2 w-2 rounded-full" style="background: {g.status.color};"></span>{g.status.name}
            <span class="chip" style="height: 20px; padding: 0 7px; background: var(--sempa-bg-panel); color: var(--sempa-text-dim); box-shadow: var(--sempa-shadow-1);">{g.projects.length}</span>
          </p>
          <div class="grid gap-3 sm:grid-cols-2">
            {#each g.projects as p (p.id)}
              {@const prog = projectProgress(tasksByProject[p.id] ?? [], p.target_date, todayStr)}
              <!-- Project tile: a soft block in the project's own colour. -->
              <a href={`/projects/${p.id}`} class="project-tile tint" style="--tint: {p.color};">
                <svg class="tile-mark" viewBox="0 0 100 100" fill="none" aria-hidden="true">
                  <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="10" stroke-linecap="round" />
                  <circle cx="50" cy="35" r="7.5" fill="currentColor" />
                </svg>
                <p class="truncate pr-10" style="font-size: 15px; font-weight: 800; letter-spacing: -.01em;">{p.title || 'Untitled project'}</p>
                {#if p.description}
                  <p class="mt-0.5 truncate pr-8 type-meta" style="opacity: .78;">{p.description}</p>
                {/if}
                <div class="mt-auto pt-5">
                  <div class="type-meta mb-2 flex items-center gap-1.5" style="opacity: .85;">
                    <span>{prog.done}/{prog.total} tasks</span>
                    {#if p.target_date}
                      <span aria-hidden="true">·</span>
                      <span style={prog.overdue ? 'color: var(--sempa-danger, #b3392e); font-weight: 700;' : ''}
                            title={formatShortDate(p.target_date)}>{relativeDue(p.target_date, todayStr)}</span>
                    {/if}
                    <span class="ml-auto font-bold tabular-nums">{prog.pct}%</span>
                  </div>
                  <div class="tile-track">
                    <div class="tile-fill" style="width: {prog.pct}%;"></div>
                  </div>
                </div>
              </a>
            {/each}
          </div>
        </section>
      {/each}

      {#if closedCount > 0}
        <button onclick={() => (showClosed = !showClosed)} class="btn-surface h-9 px-4 text-[12.5px]"
                style="color: var(--sempa-text-soft);">
          {showClosed ? 'Hide' : 'Show'} done &amp; archived ({closedCount})
        </button>
      {/if}
    {/if}
  </div>
</div>

<style>
  .new-pill input:focus-visible { box-shadow: none; }
  .new-pill:focus-within { box-shadow: var(--sempa-shadow-1), 0 0 0 2px color-mix(in srgb, var(--sempa-accent) 30%, transparent) !important; }
  .project-tile {
    position: relative;
    display: flex;
    flex-direction: column;
    min-height: 136px;
    padding: 18px 18px 16px;
    border-radius: 24px;
    overflow: hidden;
    transition: box-shadow 160ms ease, transform 160ms ease;
  }
  .project-tile:hover { box-shadow: var(--sempa-shadow-1); transform: translateY(-1px); }
  /* The Sempa cradle mark, faint in the tile's own ink. */
  .tile-mark {
    position: absolute;
    top: 10px;
    right: 10px;
    width: 52px;
    height: 52px;
    opacity: .2;
    pointer-events: none;
  }
  .tile-track {
    height: 6px;
    border-radius: 999px;
    overflow: hidden;
    background: color-mix(in srgb, var(--sempa-bg-panel) 70%, transparent);
  }
  .tile-fill {
    height: 100%;
    border-radius: 999px;
    background: currentColor;
    transition: width 300ms ease;
  }
  @media (prefers-reduced-motion: reduce) {
    .project-tile:hover { transform: none; }
  }
</style>
