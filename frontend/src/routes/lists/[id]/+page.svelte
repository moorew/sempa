<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { List, ListItem, Task } from '$lib/types';
  import { realtime } from '$lib/stores/realtime.svelte';
  import { aiStatus } from '$lib/stores/aiStatus.svelte';
  import { prefs } from '$lib/stores/prefs.svelte';
  import { Sparkles, Download, Trash2, GripVertical, X } from 'lucide-svelte';

  const listId = $derived($page.params.id ?? '');

  let list = $state<List | null>(null);
  let items = $state<ListItem[]>([]);
  let linkedTask = $state<Task | null>(null);
  let loading = $state(true);
  let newText = $state('');
  let organizing = $state(false);

  async function load() {
    try {
      const [l, its] = await Promise.all([api.lists.get(listId), api.lists.items(listId)]);
      list = l;
      items = its;
      linkedTask = l.task_id ? await api.tasks.get(l.task_id).catch(() => null) : null;
    } catch { /* ignore */ }
    finally { loading = false; }
  }
  onMount(load);
  $effect(() => { void realtime.lastEvent; void listId; load(); });

  const aiOrganizeOn = $derived(prefs.aiOn('organizeList') && aiStatus.reachable);
  const grouped = $derived(items.some((i) => i.category));
  // Category → items, in first-seen order; falls back to one unnamed group.
  const groups = $derived.by(() => {
    const map = new Map<string, ListItem[]>();
    for (const it of items) {
      const key = it.category ?? '';
      if (!map.has(key)) map.set(key, []);
      map.get(key)!.push(it);
    }
    return [...map.entries()];
  });

  async function addItem() {
    const text = newText.trim();
    if (!text) return;
    newText = '';
    try { const it = await api.lists.addItem(listId, text); items = [...items, it]; } catch { /* ignore */ }
  }
  async function toggleDone(it: ListItem) {
    const done = !it.done;
    items = items.map((x) => (x.id === it.id ? { ...x, done } : x));
    try { await api.lists.updateItem(it.id, { done }); } catch { /* ignore */ }
  }
  async function removeItem(it: ListItem) {
    items = items.filter((x) => x.id !== it.id);
    try { await api.lists.deleteItem(it.id); } catch { /* ignore */ }
  }
  async function rename(name: string) {
    if (!list || name === list.name) return;
    try { list = await api.lists.update(list.id, { name }); } catch { /* ignore */ }
  }
  async function toggleArchiveOnComplete() {
    if (!list) return;
    try { list = await api.lists.update(list.id, { archive_on_complete: !list.archive_on_complete }); } catch { /* ignore */ }
  }
  async function unlinkTask() {
    if (!list) return;
    try { list = await api.lists.update(list.id, { task_id: '' }); linkedTask = null; } catch { /* ignore */ }
  }

  // ── Drag reorder (flat view only) ────────────────────────────────────────────
  let dragId = $state<string | null>(null);
  function onDrop(targetId: string) {
    if (!dragId || dragId === targetId) { dragId = null; return; }
    const ids = items.map((i) => i.id);
    const from = ids.indexOf(dragId), to = ids.indexOf(targetId);
    if (from < 0 || to < 0) { dragId = null; return; }
    const reordered = [...items];
    const [moved] = reordered.splice(from, 1);
    reordered.splice(to, 0, moved);
    items = reordered;
    dragId = null;
    api.lists.reorder(listId, reordered.map((i) => i.id)).catch(() => {});
  }

  async function organize() {
    if (organizing) return;
    const texts = items.map((i) => i.text);
    if (!texts.length) return;
    organizing = true;
    try {
      const res = await api.ai.organizeList(texts);
      if (res.available && res.groups?.length) {
        // Map each item's category by verbatim (case-insensitive) text match.
        const cat = new Map<string, string>();
        for (const g of res.groups) for (const t of g.items) cat.set(t.trim().toLowerCase(), g.category);
        await Promise.all(items.map((it) => {
          const c = cat.get(it.text.trim().toLowerCase());
          return c && c !== it.category ? api.lists.updateItem(it.id, { category: c }) : null;
        }).filter(Boolean) as Promise<unknown>[]);
        await load();
      }
    } catch { /* ignore */ }
    finally { organizing = false; }
  }
  async function clearGrouping() {
    await Promise.all(items.filter((i) => i.category).map((i) => api.lists.updateItem(i.id, { category: '' })));
    await load();
  }

  function exportMarkdown() {
    if (!list) return;
    const lines = [`# ${list.name || 'List'}`, ''];
    if (grouped) {
      for (const [cat, its] of groups) {
        lines.push(`## ${cat || 'Other'}`, '');
        for (const it of its) lines.push(`- [${it.done ? 'x' : ' '}] ${it.text}`);
        lines.push('');
      }
    } else {
      for (const it of items) lines.push(`- [${it.done ? 'x' : ' '}] ${it.text}`);
    }
    const blob = new Blob([lines.join('\n')], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${(list.name || 'list').replace(/[^\w- ]+/g, '').trim() || 'list'}.md`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }
</script>

<svelte:head><title>{list?.name ?? 'List'} — Sempa</title></svelte:head>

<div class="mx-auto flex h-full max-w-2xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <!-- Header -->
  <div class="flex items-center gap-3 px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <a href="/lists" class="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm transition-colors" style="color: var(--sempa-accent);">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" d="M19 12H5m7-7-7 7 7 7"/></svg>
      Lists
    </a>
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-6 pb-24">
    {#if loading && !list}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else if !list}
      <p class="text-sm" style="color: var(--sempa-text-dim);">List not found.</p>
    {:else}
      <!-- Name -->
      <input value={list.name} onblur={(e) => rename((e.currentTarget as HTMLInputElement).value.trim())}
        onkeydown={(e) => { if (e.key === 'Enter') (e.currentTarget as HTMLInputElement).blur(); }}
        placeholder="List name"
        class="mb-2 w-full bg-transparent text-2xl font-bold outline-none" style="color: var(--sempa-text);" />

      <!-- Linked task + options -->
      <div class="mb-4 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs" style="color: var(--sempa-text-dim);">
        {#if linkedTask}
          <span class="inline-flex items-center gap-1.5">
            Linked to <span style="color: var(--sempa-text-soft);">“{linkedTask.title}”</span>
            <button onclick={unlinkTask} aria-label="Unlink" class="transition-opacity hover:opacity-70"><X size={12} /></button>
          </span>
          <label class="inline-flex cursor-pointer items-center gap-1.5">
            <input type="checkbox" checked={list.archive_on_complete} onchange={toggleArchiveOnComplete} />
            Archive this list when the task is done
          </label>
        {:else}
          <span>Not linked to a task — attach it from a task's “Lists” section.</span>
        {/if}
      </div>

      <!-- Actions -->
      <div class="mb-4 flex flex-wrap gap-2">
        {#if aiOrganizeOn}
          <button onclick={organize} disabled={organizing || items.length === 0}
            class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-opacity hover:opacity-80 disabled:opacity-40"
            style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
            <Sparkles size={13} /> {organizing ? 'Organizing…' : 'Organize with AI'}
          </button>
        {/if}
        {#if grouped}
          <button onclick={clearGrouping} class="rounded-lg px-3 py-1.5 text-xs transition-opacity hover:opacity-70"
            style="color: var(--sempa-text-soft); border: 1px solid var(--sempa-border);">Clear grouping</button>
        {/if}
        <button onclick={exportMarkdown} disabled={items.length === 0}
          class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs transition-opacity hover:opacity-80 disabled:opacity-40"
          style="color: var(--sempa-text-soft); border: 1px solid var(--sempa-border);">
          <Download size={13} /> Export Markdown
        </button>
      </div>

      <!-- Add item -->
      <form class="mb-4 flex gap-2" onsubmit={(e) => { e.preventDefault(); addItem(); }}>
        <input bind:value={newText} placeholder="Add an item…"
          class="flex-1 rounded-xl px-3 py-2.5 text-sm outline-none"
          style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
        <button type="submit" disabled={!newText.trim()}
          class="rounded-xl px-4 py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-40"
          style="background: var(--sempa-accent);">Add</button>
      </form>

      <!-- Items -->
      {#if items.length === 0}
        <p class="text-sm" style="color: var(--sempa-text-dim);">No items yet.</p>
      {:else if grouped}
        {#each groups as [cat, its]}
          <p class="mb-2 mt-4 text-xs font-semibold uppercase tracking-wider first:mt-0" style="color: var(--sempa-text-dim);">{cat || 'Other'}</p>
          <div class="flex flex-col gap-1.5">
            {#each its as it (it.id)}{@render itemRow(it, false)}{/each}
          </div>
        {/each}
      {:else}
        <div class="flex flex-col gap-1.5">
          {#each items as it (it.id)}{@render itemRow(it, true)}{/each}
        </div>
      {/if}
    {/if}
  </div>
</div>

{#snippet itemRow(it: ListItem, draggable: boolean)}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="flex items-center gap-2.5 rounded-xl px-3 py-2.5"
    style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); {dragId === it.id ? 'opacity:0.4;' : ''}"
    draggable={draggable}
    ondragstart={() => (dragId = it.id)}
    ondragover={(e) => { if (draggable) e.preventDefault(); }}
    ondrop={() => onDrop(it.id)}>
    {#if draggable}
      <span class="cursor-grab" style="color: var(--sempa-text-dim);"><GripVertical size={15} /></span>
    {/if}
    <button onclick={() => toggleDone(it)} aria-label="Toggle done"
      class="flex h-5 w-5 shrink-0 items-center justify-center rounded-md border transition-colors"
      style="border-color: {it.done ? 'var(--sempa-success)' : 'var(--sempa-border)'}; background: {it.done ? 'var(--sempa-success)' : 'transparent'};">
      {#if it.done}
        <svg class="h-3 w-3 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>
      {/if}
    </button>
    <span class="min-w-0 flex-1 text-sm" style="color: var(--sempa-text); {it.done ? 'text-decoration: line-through; opacity: 0.5;' : ''}">{it.text}</span>
    <button onclick={() => removeItem(it)} aria-label="Delete item" class="shrink-0 rounded p-1 transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);">
      <Trash2 size={14} />
    </button>
  </div>
{/snippet}
