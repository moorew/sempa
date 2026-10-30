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
  <div class="flex items-center gap-2.5 px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <FolderKanban size={20} style="color: var(--sempa-accent);" />
    <h1 class="text-base font-semibold" style="color: var(--sempa-text);">Projects</h1>
    <a href="/settings/projects" class="ml-auto text-xs font-medium transition-opacity hover:opacity-70"
       style="color: var(--sempa-text-dim);">Statuses</a>
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-6 pb-24">
    <form class="mb-6 flex gap-2" onsubmit={(e) => { e.preventDefault(); create(); }}>
      <input bind:value={newTitle} placeholder="New project (e.g. Renovate the kitchen)…"
        class="flex-1 rounded-xl px-3 py-2.5 text-sm outline-none"
        style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
      <button type="submit" disabled={!newTitle.trim()}
        class="flex items-center gap-1.5 rounded-full px-4 py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-40"
        style="background: var(--sempa-btn-bg); box-shadow: var(--sempa-shadow-accent);">
        <Plus size={16} /> Add
      </button>
    </form>

    {#if !projectStore.loaded}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else if projectStore.projects.length === 0}
      <div class="rounded-2xl border border-dashed p-8 text-center" style="border-color: var(--sempa-border);">
        <p class="text-sm" style="color: var(--sempa-text-soft);">No projects yet.</p>
        <p class="mt-1 text-xs" style="color: var(--sempa-text-dim);">
          A project holds longer work — months, not days. Give it a description and a target date, then file
          tasks, weekly objectives and lists into it.
        </p>
      </div>
    {:else}
      {#each groups as g (g.status.id)}
        <div class="mb-6">
          <p class="mb-2 flex items-center gap-2 px-1 text-[12px] font-semibold"
             style="color: var(--sempa-text-dim);">
            <span class="h-2 w-2 rounded-full" style="background: {g.status.color};"></span>{g.status.name}
          </p>
          <div class="grid gap-2 sm:grid-cols-2">
            {#each g.projects as p (p.id)}
              {@const prog = projectProgress(tasksByProject[p.id] ?? [], p.target_date, todayStr)}
              <a href={`/projects/${p.id}`}
                 class="block rounded-xl px-4 py-3 transition-shadow hover:shadow-sm"
                 style="box-shadow: var(--sempa-shadow-1); background: var(--sempa-bg-panel); border-left: 4px solid {p.color};">
                <p class="truncate text-sm font-semibold" style="color: var(--sempa-text);">{p.title || 'Untitled project'}</p>
                <div class="mt-1 flex items-center gap-2 text-[11.5px]" style="color: var(--sempa-text-dim);">
                  <span>{prog.done}/{prog.total} tasks</span>
                  {#if p.target_date}
                    <span>·</span>
                    <span style={prog.overdue ? 'color: var(--sempa-danger, #b3392e);' : ''}
                          title={formatShortDate(p.target_date)}>{relativeDue(p.target_date, todayStr)}</span>
                  {/if}
                </div>
                <div class="mt-2 h-1.5 overflow-hidden rounded-full" style="background: var(--sempa-bg-main);">
                  <div class="h-full rounded-full transition-all" style="width: {prog.pct}%; background: {p.color};"></div>
                </div>
              </a>
            {/each}
          </div>
        </div>
      {/each}

      {#if closedCount > 0}
        <button onclick={() => (showClosed = !showClosed)}
          class="text-xs font-medium transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);">
          {showClosed ? 'Hide' : 'Show'} done &amp; archived ({closedCount})
        </button>
      {/if}
    {/if}
  </div>
</div>
