<script lang="ts">
  import { api } from '$lib/api';
  import type { ProjectStatus, ProjectCategory } from '$lib/types';
  import { realtime } from '$lib/stores/realtime.svelte';
  import { projectStore, PROJECT_CATEGORIES, PROJECT_COLORS } from '$lib/stores/projects.svelte';
  import { confirmDialog } from '$lib/stores/confirmDialog.svelte';
  import SempaSelect from '$lib/components/ui/SempaSelect.svelte';
  import { ChevronUp, ChevronDown, Trash2, Plus } from 'lucide-svelte';

  let statuses = $state<ProjectStatus[]>([]);
  let newName = $state('');
  let newCategory = $state<ProjectCategory>('active');
  let error = $state('');
  let colorFor = $state<string | null>(null); // status id whose palette is open

  async function load() {
    try { statuses = await api.projectStatuses.list(); } catch { /* ignore */ }
  }
  $effect(() => { void realtime.lastEvent; load(); });

  const categoryOptions = PROJECT_CATEGORIES.map((c) => ({ value: c.id, label: c.label }));

  async function done() {
    await load();
    projectStore.reload();
  }

  async function create() {
    const name = newName.trim();
    if (!name) return;
    error = '';
    try {
      await api.projectStatuses.create({
        name, category: newCategory, color: PROJECT_COLORS[statuses.length % PROJECT_COLORS.length],
      });
      newName = '';
      await done();
    } catch (e) { error = (e as Error).message; }
  }

  async function update(st: ProjectStatus, patch: { name?: string; color?: string; category?: ProjectCategory }) {
    statuses = statuses.map((s) => (s.id === st.id ? { ...s, ...patch } : s));
    try { await api.projectStatuses.update(st.id, patch); await done(); } catch (e) { error = (e as Error).message; }
  }

  async function move(i: number, delta: number) {
    const j = i + delta;
    if (j < 0 || j >= statuses.length) return;
    const next = [...statuses];
    [next[i], next[j]] = [next[j], next[i]];
    statuses = next;
    try { await api.projectStatuses.reorder(next.map((s) => s.id)); await done(); } catch { /* ignore */ }
  }

  async function remove(st: ProjectStatus) {
    const inUse = projectStore.projects.filter((p) => p.status_id === st.id).length;
    const ok = await confirmDialog.ask({
      title: `Delete “${st.name}”?`,
      message: inUse
        ? `${inUse} project${inUse === 1 ? '' : 's'} will move to another status in the same category.`
        : 'No projects use this status.',
      confirmLabel: 'Delete',
      danger: true,
    });
    if (!ok) return;
    error = '';
    try { await api.projectStatuses.delete(st.id); await done(); } catch (e) { error = (e as Error).message; }
  }
</script>

<svelte:head><title>Project statuses — Sempa</title></svelte:head>

<div class="mx-auto max-w-xl px-6 py-8">
  <a href="/settings/accounts#settings-tasks"
     class="mb-6 inline-flex items-center gap-1.5 text-sm" style="color: var(--sempa-text-soft);">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="m15 18-6-6 6-6"/>
    </svg>
    Settings
  </a>

  <h1 class="mb-1 text-xl font-semibold" style="color: var(--sempa-text);">Project statuses</h1>
  <p class="mb-6 text-sm" style="color: var(--sempa-text-soft);">
    Name, colour and order the stages your projects move through. Each status belongs to a
    <em>category</em>, which decides how Sempa treats it — <strong>Active</strong> projects show up in
    weekly planning, and <strong>Done</strong> / <strong>Archived</strong> ones are tucked away.
  </p>

  <div class="mb-6 flex flex-col gap-2">
    {#each statuses as st, i (st.id)}
      <div class="relative flex items-center gap-2 rounded-xl px-3 py-2.5"
           style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
        <button onclick={() => (colorFor = colorFor === st.id ? null : st.id)} aria-label="Status colour"
          class="h-4 w-4 shrink-0 rounded-full" style="background: {st.color};"></button>
        {#if colorFor === st.id}
          <div class="absolute left-2 top-11 z-20 flex gap-1.5 rounded-xl p-2 shadow-lg"
               style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">
            {#each PROJECT_COLORS as c}
              <button aria-label={`Colour ${c}`} onclick={() => { colorFor = null; update(st, { color: c }); }}
                class="h-6 w-6 rounded-full" style="background: {c};"></button>
            {/each}
          </div>
        {/if}
        <input value={st.name}
          onblur={(e) => { const v = (e.currentTarget as HTMLInputElement).value.trim(); if (v && v !== st.name) update(st, { name: v }); }}
          onkeydown={(e) => { if (e.key === 'Enter') (e.currentTarget as HTMLInputElement).blur(); }}
          class="min-w-0 flex-1 bg-transparent text-sm outline-none" style="color: var(--sempa-text);" />
        <div class="w-32 shrink-0">
          <SempaSelect value={st.category} options={categoryOptions}
            onchange={(v) => { if (typeof v === 'string' && v !== st.category) update(st, { category: v as ProjectCategory }); }} />
        </div>
        <button onclick={() => move(i, -1)} disabled={i === 0} aria-label="Move up"
          class="rounded p-1 disabled:opacity-25" style="color: var(--sempa-text-dim);"><ChevronUp size={15} /></button>
        <button onclick={() => move(i, 1)} disabled={i === statuses.length - 1} aria-label="Move down"
          class="rounded p-1 disabled:opacity-25" style="color: var(--sempa-text-dim);"><ChevronDown size={15} /></button>
        <button onclick={() => remove(st)} disabled={statuses.length <= 1} aria-label="Delete status"
          class="rounded p-1 disabled:opacity-25" style="color: var(--sempa-text-dim);"><Trash2 size={15} /></button>
      </div>
    {/each}
  </div>

  <form class="flex items-center gap-2" onsubmit={(e) => { e.preventDefault(); create(); }}>
    <input bind:value={newName} placeholder="New status (e.g. Blocked)…"
      class="min-w-0 flex-1 rounded-xl px-3 py-2.5 text-sm outline-none"
      style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
    <div class="w-32 shrink-0">
      <SempaSelect bind:value={newCategory} options={categoryOptions} />
    </div>
    <button type="submit" disabled={!newName.trim()}
      class="flex items-center gap-1.5 rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-40"
      style="background: var(--sempa-accent);">
      <Plus size={16} /> Add
    </button>
  </form>
  {#if error}<p class="mt-3 text-xs" style="color: var(--sempa-danger, #c0392b);">{error}</p>{/if}

  <div class="mt-8 rounded-xl px-4 py-3 text-xs" style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
    <p class="mb-1 font-semibold" style="color: var(--sempa-text);">Categories</p>
    {#each PROJECT_CATEGORIES as c}
      <p><strong>{c.label}</strong> — {c.hint}</p>
    {/each}
  </div>
</div>
