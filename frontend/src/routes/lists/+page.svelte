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
  <div class="flex items-center gap-2.5 px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <ListChecks size={20} style="color: var(--sempa-accent);" />
    <h1 class="text-base font-semibold" style="color: var(--sempa-text);">Lists</h1>
    {#if showImport}
      <button onclick={() => importModal.show()}
        class="ml-auto flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors"
        style="border: 1px solid var(--sempa-border); color: var(--sempa-accent);"
        title="Turn a recipe, itinerary or brief into a task + list">
        <Sparkles size={14} /> Import with AI
      </button>
    {/if}
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-6 pb-24">
    <!-- New list -->
    <form class="mb-5 flex gap-2" onsubmit={(e) => { e.preventDefault(); create(); }}>
      <input bind:value={newName} bind:this={newNameInput} placeholder="New list (e.g. Groceries)…"
        class="flex-1 rounded-xl px-3 py-2.5 text-sm outline-none"
        style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
      <button type="submit" disabled={!newName.trim()}
        class="flex items-center gap-1.5 rounded-xl px-4 py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-40"
        style="background: var(--sempa-accent);">
        <Plus size={16} /> Add
      </button>
    </form>

    {#if loading && lists.length === 0}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else if active.length === 0 && archived.length === 0}
      <div class="rounded-2xl border border-dashed p-8 text-center" style="border-color: var(--sempa-border);">
        <p class="text-sm" style="color: var(--sempa-text-soft);">No lists yet.</p>
        <p class="mt-1 text-xs" style="color: var(--sempa-text-dim);">Create an ongoing list like Groceries — it sticks around and can be attached to tasks.</p>
      </div>
    {:else}
      <div class="flex flex-col gap-2">
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
        class="mt-6 text-xs font-medium transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);">
        {showArchived ? 'Hide' : 'Show'} archived
      </button>
      {#if showArchived}
        <div class="mt-2 flex flex-col gap-2">
          {#each archived as l (l.id)}
            {@render listRow(l, true)}
          {/each}
          {#if archived.length === 0}
            <p class="text-xs" style="color: var(--sempa-text-dim);">No archived lists.</p>
          {/if}
        </div>
      {/if}
    {/if}
  </div>
</div>

{#snippet listRow(l: List, isArchived: boolean)}
  <div class="flex items-center gap-3 rounded-xl px-4 py-3"
    style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); {isArchived ? 'opacity:0.65;' : ''}">
    <a href={`/lists/${l.id}`} class="min-w-0 flex-1">
      <p class="truncate text-sm font-medium" style="color: var(--sempa-text);">{l.name || 'Untitled list'}</p>
      {#if l.task_id}
        <p class="text-xs" style="color: var(--sempa-text-dim);">Linked to a task</p>
      {/if}
    </a>
    <button onclick={() => setArchived(l, !isArchived)} aria-label={isArchived ? 'Unarchive' : 'Archive'}
      class="rounded-lg p-1.5 transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);">
      {#if isArchived}<ArchiveRestore size={16} />{:else}<Archive size={16} />{/if}
    </button>
    <button onclick={() => remove(l)} aria-label="Delete list"
      class="rounded-lg p-1.5 transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);">
      <Trash2 size={16} />
    </button>
  </div>
{/snippet}
