<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { List } from '$lib/types';
  import { realtime } from '$lib/stores/realtime.svelte';
  import { prefs } from '$lib/stores/prefs.svelte';
  import { aiStatus } from '$lib/stores/aiStatus.svelte';
  import { importModal } from '$lib/stores/importModal.svelte';
  import { confirmDialog } from '$lib/stores/confirmDialog.svelte';
  import { ListChecks, Plus, Archive, ArchiveRestore, Trash2, Sparkles } from 'lucide-svelte';
  import { projectStore } from '$lib/stores/projects.svelte';

  // 1.27.1: each list gets its own soft colour — its project's colour when it
  // belongs to one, otherwise a stable pick from a calm palette (hashed from the
  // id, so a list keeps its colour across sessions and devices).
  const LIST_HUES = ['#d27a4c', '#5d8ccc', '#5f9a6c', '#9a7cc8', '#d3a24a', '#c76a86'];
  function listHue(l: { id: string; project_id?: string | null }): string {
    const pc = projectStore.get(l.project_id)?.color;
    if (pc) return pc;
    let h = 0;
    for (let i = 0; i < l.id.length; i++) h = (h * 31 + l.id.charCodeAt(i)) >>> 0;
    return LIST_HUES[h % LIST_HUES.length];
  }

  const showImport = $derived(
    prefs.aiOn('aiImport') && prefs.importEntryOn('button') && aiStatus.reachable,
  );

  let lists = $state<List[]>([]);
  let loading = $state(true);
  let showArchived = $state(false);
  let newName = $state('');
  let newNameInput = $state<HTMLInputElement | undefined>();

  async function load() {
    try { lists = await api.lists.list(undefined, showArchived); }
    catch { /* server unreachable — leave as-is */ }
    finally { loading = false; }
  }
  onMount(load);
  // Re-fetch on realtime changes and when the archived toggle flips.
  $effect(() => { void realtime.lastEvent; void showArchived; load(); });

  // Item counts for the cards. Read-only and best-effort: a list whose items
  // can't be fetched just shows no count. Written to its own state (never read
  // in this effect), so it can't loop.
  let counts = $state<Record<string, { total: number; done: number }>>({});
  async function loadCounts(ls: List[]) {
    const entries = await Promise.all(ls.map(async (l) => {
      try {
        const its = await api.lists.items(l.id);
        return [l.id, { total: its.length, done: its.filter((i) => i.done).length }] as const;
      } catch { return null; }
    }));
    counts = Object.fromEntries(entries.filter((e) => e !== null));
  }
  $effect(() => { void loadCounts(lists); });

  const active = $derived(lists.filter((l) => !l.archived_at));
  const archived = $derived(lists.filter((l) => l.archived_at));

  async function create() {
    const name = newName.trim();
    if (!name) return;
    newName = '';
    newNameInput?.focus(); // keep the keyboard up for rapid entry
    try { const l = await api.lists.create({ name }); lists = [...lists, l]; }
    catch { /* ignore */ }
  }
  async function setArchived(l: List, archived: boolean) {
    try { await api.lists.update(l.id, { archived }); await load(); } catch { /* ignore */ }
  }
  async function remove(l: List) {
    const ok = await confirmDialog.ask({
      title: `Delete “${l.name || 'this list'}”?`,
      message: 'This also deletes all of its items. This can’t be undone.',
      confirmLabel: 'Delete',
      danger: true,
    });
    if (!ok) return;
    try { await api.lists.delete(l.id); lists = lists.filter((x) => x.id !== l.id); } catch { /* ignore */ }
  }
</script>

<svelte:head><title>Lists — Sempa</title></svelte:head>

<div class="mx-auto flex h-full max-w-2xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <div class="flex items-end gap-3 px-5 pb-2 pt-7">
    <div class="min-w-0 flex-1">
      <h1 class="type-page" style="color: var(--sempa-text);">Lists</h1>
      {#if !loading}
        <p class="type-meta mt-1" style="color: var(--sempa-text-dim);">
          {active.length} list{active.length === 1 ? '' : 's'}
        </p>
      {/if}
    </div>
    {#if showImport}
      <button onclick={() => importModal.show()}
        class="btn-soft h-9 gap-1.5 px-4 text-[12.5px]"
        title="Turn a recipe, itinerary or brief into a task + list">
        <Sparkles size={14} /> Import with AI
      </button>
    {/if}
  </div>

  <div class="flex-1 overflow-y-auto px-5 pb-24 pt-4">
    <!-- New list: one raised pill with the input and the add button. -->
    <form class="new-pill mb-6 flex items-center gap-2 rounded-full py-1.5 pl-4 pr-1.5"
          style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1);"
          onsubmit={(e) => { e.preventDefault(); create(); }}>
      <ListChecks size={17} class="shrink-0" style="color: var(--sempa-text-dim);" />
      <input bind:value={newName} bind:this={newNameInput} placeholder="New list (e.g. Groceries)…"
        class="min-w-0 flex-1 bg-transparent py-1.5 text-sm outline-none"
        style="color: var(--sempa-text);" />
      <button type="submit" disabled={!newName.trim()}
        class="btn-primary h-9 shrink-0 px-4 text-[13px] disabled:opacity-40">
        <Plus size={16} strokeWidth={2.5} /> Add
      </button>
    </form>

    {#if loading && lists.length === 0}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else if active.length === 0 && archived.length === 0}
      <div class="surface flex flex-col items-center px-8 py-10 text-center" style="border-radius: 24px;">
        <span class="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl"
              style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
          <ListChecks size={22} />
        </span>
        <p class="font-bold" style="font-size: 15px; color: var(--sempa-text);">No lists yet</p>
        <p class="type-meta mt-1 max-w-sm" style="color: var(--sempa-text-soft);">Create an ongoing list like Groceries — it sticks around and can be attached to tasks.</p>
      </div>
    {:else}
      <div class="grid gap-3 sm:grid-cols-2">
        {#each active as l (l.id)}
          {@render listRow(l, false)}
        {/each}
      </div>
    {/if}

    <!-- Always render the toggle: it's the only way to load archived lists
         (the initial fetch excludes them), so gating it on archived.length made
         archived lists permanently unreachable. -->
    {#if !loading}
      <button onclick={() => (showArchived = !showArchived)}
        class="btn-surface mt-6 h-9 px-4 text-[12.5px]" style="color: var(--sempa-text-soft);">
        {showArchived ? 'Hide' : 'Show'} archived
      </button>
      {#if showArchived}
        <div class="mt-3 grid gap-3 sm:grid-cols-2">
          {#each archived as l (l.id)}
            {@render listRow(l, true)}
          {/each}
          {#if archived.length === 0}
            <p class="type-meta px-1" style="color: var(--sempa-text-dim);">No archived lists.</p>
          {/if}
        </div>
      {/if}
    {/if}
  </div>
</div>

{#snippet listRow(l: List, isArchived: boolean)}
  {@const c = counts[l.id]}
  <div class="surface flex flex-col gap-3 p-4"
    style="border-radius: 22px; {isArchived ? 'opacity:0.65;' : ''}">
    <div class="flex items-start gap-3">
      <a href={`/lists/${l.id}`} class="flex min-w-0 flex-1 items-center gap-3">
        <span class="tint flex h-10 w-10 shrink-0 items-center justify-center rounded-[14px]"
              style="--tint: {listHue(l)};">
          <ListChecks size={18} />
        </span>
        <span class="min-w-0">
          <span class="block truncate text-[14.5px] font-bold" style="color: var(--sempa-text); letter-spacing: -.01em;">{l.name || 'Untitled list'}</span>
          <span class="type-meta block truncate" style="color: var(--sempa-text-dim);">
            {#if c}
              {c.total} item{c.total === 1 ? '' : 's'}{c.total > 0 ? ` · ${c.done} done` : ''}
            {:else}
              &nbsp;
            {/if}{#if l.task_id}{c ? ' · ' : ''}Linked to a task{/if}
          </span>
        </span>
      </a>
      <div class="flex shrink-0 items-center gap-0.5">
        <button onclick={() => setArchived(l, !isArchived)} aria-label={isArchived ? 'Unarchive' : 'Archive'}
          class="flex h-8 w-8 items-center justify-center rounded-full transition-colors hover:bg-[var(--sempa-fill)]" style="color: var(--sempa-text-dim);">
          {#if isArchived}<ArchiveRestore size={16} />{:else}<Archive size={16} />{/if}
        </button>
        <button onclick={() => remove(l)} aria-label="Delete list"
          class="flex h-8 w-8 items-center justify-center rounded-full transition-colors hover:bg-[var(--sempa-fill)]" style="color: var(--sempa-text-dim);">
          <Trash2 size={16} />
        </button>
      </div>
    </div>
    {#if c && c.total > 0}
      <div class="day-bar-track">
        <div class="day-bar-fill" style="width: {Math.round((c.done / c.total) * 100)}%; background: {listHue(l)};"></div>
      </div>
    {/if}
  </div>
{/snippet}

<style>
  .new-pill input:focus-visible { box-shadow: none; }
  .new-pill:focus-within { box-shadow: var(--sempa-shadow-1), 0 0 0 2px color-mix(in srgb, var(--sempa-accent) 30%, transparent) !important; }
</style>
