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

<div class="mx-auto max-w-[640px] px-5 pt-6 pb-16 sm:px-8 sm:pt-8">
  <a href="/settings/accounts?section=tasks" class="set-back">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" d="m15 18-6-6 6-6"/>
    </svg>
    Settings
  </a>

  <h1 class="set-title type-page">Project statuses</h1>
  <p class="set-lede mb-6">
    Name, colour and order the stages your projects move through. Each status belongs to a
    <em>category</em>, which decides how Sempa treats it — <strong>Active</strong> projects show up in
    weekly planning, and <strong>Done</strong> / <strong>Archived</strong> ones are tucked away.
  </p>

  <div class="set-group mb-4">
    {#each statuses as st, i (st.id)}
      <div class="set-row set-row-compact relative gap-2" style="padding-left: 12px; padding-right: 10px;">
        <button onclick={() => (colorFor = colorFor === st.id ? null : st.id)} aria-label="Status colour"
          class="tint flex h-9 w-9 shrink-0 items-center justify-center rounded-xl" style="--tint: {st.color};">
          <span class="h-3.5 w-3.5 rounded-full" style="background: {st.color};"></span>
        </button>
        {#if colorFor === st.id}
          <div class="surface-raised absolute left-2 top-[52px] z-20 flex gap-1.5 p-2.5" style="border-radius: 18px;">
            {#each PROJECT_COLORS as c}
              <button aria-label={`Colour ${c}`} onclick={() => { colorFor = null; update(st, { color: c }); }}
                class="h-7 w-7 rounded-full" style="background: {c};"></button>
            {/each}
          </div>
        {/if}
        <input value={st.name}
          onblur={(e) => { const v = (e.currentTarget as HTMLInputElement).value.trim(); if (v && v !== st.name) update(st, { name: v }); }}
          onkeydown={(e) => { if (e.key === 'Enter') (e.currentTarget as HTMLInputElement).blur(); }}
          aria-label="Status name"
          class="min-w-0 flex-1 rounded-lg bg-transparent px-1.5 py-1 text-sm font-semibold outline-none focus:bg-[var(--sempa-fill)]" style="color: var(--sempa-text);" />
        <div class="w-36 shrink-0">
          <SempaSelect value={st.category} options={categoryOptions}
            onchange={(v) => { if (typeof v === 'string' && v !== st.category) update(st, { category: v as ProjectCategory }); }} />
        </div>
        <div class="flex shrink-0 items-center">
          <button onclick={() => move(i, -1)} disabled={i === 0} aria-label="Move up"
            class="set-icon-btn"><ChevronUp size={16} /></button>
          <button onclick={() => move(i, 1)} disabled={i === statuses.length - 1} aria-label="Move down"
            class="set-icon-btn"><ChevronDown size={16} /></button>
          <button onclick={() => remove(st)} disabled={statuses.length <= 1} aria-label="Delete status"
            class="set-icon-btn danger"><Trash2 size={15} /></button>
        </div>
      </div>
    {/each}
  </div>

  <form class="set-group" onsubmit={(e) => { e.preventDefault(); create(); }}>
    <div class="set-row set-row-wrap gap-2" style="padding-left: 12px; padding-right: 12px;">
      <input bind:value={newName} placeholder="New status (e.g. Blocked)…" aria-label="New status name"
        class="set-input min-w-[160px] flex-1" />
      <div class="w-36 shrink-0">
        <SempaSelect bind:value={newCategory} options={categoryOptions} />
      </div>
      <button type="submit" disabled={!newName.trim()}
        class="btn-primary set-btn">
        <Plus size={16} /> Add
      </button>
    </div>
  </form>
  {#if error}<p class="set-error mt-3 px-1.5">{error}</p>{/if}

  <h2 class="set-heading">Categories</h2>
  <div class="set-group">
    {#each PROJECT_CATEGORIES as c}
      <div class="set-row set-row-compact">
        <p class="set-row-main text-[13px]" style="color: var(--sempa-text-soft);">
          <span class="font-bold" style="color: var(--sempa-text);">{c.label}</span> — {c.hint}
        </p>
      </div>
    {/each}
  </div>
</div>
