<script lang="ts">
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { appendPosition } from '$lib/utils';

  let { parentId, parentDate }: { parentId: string; parentDate?: string } = $props();

  let subtasks  = $state<Task[]>([]);
  let loading   = $state(true);
  let newTitle  = $state('');
  let adding    = $state(false);
  let inputEl   = $state<HTMLInputElement | undefined>();

  $effect(() => {
    parentId; void load();
  });

  async function load() {
    loading = true;
    try { subtasks = await api.tasks.listByParent(parentId); }
    catch { /* ignore */ }
    finally { loading = false; }
  }

  async function add() {
    const t = newTitle.trim();
    if (!t) return;
    adding = true;
    try {
      const newPos = appendPosition(subtasks.map(s => s.position));
      const sub = await api.tasks.create({
        title: t,
        parent_task_id: parentId,
        status: 'planned',
        planned_date: parentDate,
        position: newPos,
      });
      subtasks = [...subtasks, sub];
      newTitle = '';
      inputEl?.focus();
    } catch { /* ignore */ }
    finally { adding = false; }
  }

  async function toggleDone(sub: Task) {
    const newStatus = sub.status === 'done' ? 'planned' : 'done';
    subtasks = subtasks.map(s => s.id === sub.id ? { ...s, status: newStatus } : s);
    try {
      const updated = await api.tasks.update(sub.id, {
        status: newStatus,
        completed_at: newStatus === 'done' ? new Date().toISOString() : null,
      });
      subtasks = subtasks.map(s => s.id === updated.id ? updated : s);
    } catch { await load(); }
  }

  async function remove(id: string) {
    subtasks = subtasks.filter(s => s.id !== id);
    await api.tasks.delete(id);
  }

  const done  = $derived(subtasks.filter(s => s.status === 'done').length);
  const total = $derived(subtasks.length);
</script>

<div class="space-y-2">
  <div class="flex items-center justify-between">
    <span class="text-xs font-medium" style="color: var(--sempa-text-soft);">
      Sub-tasks {#if total > 0}<span style="color: var(--sempa-text-dim);">({done}/{total})</span>{/if}
    </span>
    {#if total > 0}
      <div class="h-1 w-20 overflow-hidden rounded-full" style="background: var(--sempa-border);">
        <div class="h-full rounded-full transition-all"
             style="width: {total ? (done / total) * 100 : 0}%; background: var(--sempa-accent);"></div>
      </div>
    {/if}
  </div>

  {#if loading}
    <div class="space-y-1.5">
      {#each Array(2) as _}
        <div class="h-7 animate-pulse rounded-lg" style="background: var(--sempa-bg-panel);"></div>
      {/each}
    </div>
  {:else}
    <ul class="space-y-1">
      {#each subtasks as sub (sub.id)}
        <li class="subrow group flex items-start gap-2 rounded-lg px-2 py-1.5">
          <button onclick={() => toggleDone(sub)}
                  aria-label={sub.status === 'done' ? 'Mark not done' : 'Mark done'}
                  class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border-2 transition-all"
                  style="border-color: {sub.status === 'done' ? 'var(--sempa-accent)' : 'var(--sempa-border)'};
                         background: {sub.status === 'done' ? 'var(--sempa-accent)' : 'transparent'};">
            {#if sub.status === 'done'}
              <svg class="h-2.5 w-2.5" style="color: #fff;" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
              </svg>
            {/if}
          </button>
          <div class="min-w-0 flex-1">
            <p class="text-sm" style="color: {sub.status === 'done' ? 'var(--sempa-text-dim)' : 'var(--sempa-text)'};
                      {sub.status === 'done' ? 'text-decoration: line-through;' : ''}
                      text-wrap: pretty; overflow-wrap: anywhere;">
              {sub.title}
            </p>
            {#if sub.description}
              <p class="mt-0.5 text-xs" style="color: var(--sempa-text-dim); white-space: pre-wrap;
                        text-wrap: pretty; overflow-wrap: anywhere;">{sub.description}</p>
            {/if}
          </div>
          <button onclick={() => remove(sub.id)}
                  aria-label="Delete sub-task"
                  class="mt-0.5 shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
                  style="color: var(--sempa-text-dim);">
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </li>
      {/each}
    </ul>
  {/if}

  <!-- Add sub-task input -->
  <div class="addrow flex items-center gap-2 rounded-lg border border-dashed px-2 py-1.5"
       style="border-color: var(--sempa-border);">
    <svg class="h-3.5 w-3.5 shrink-0" style="color: var(--sempa-text-dim);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="M12 4v16m8-8H4"/>
    </svg>
    <input bind:this={inputEl}
           bind:value={newTitle}
           onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void add(); } }}
           type="text"
           placeholder="Add a sub-task…"
           class="flex-1 bg-transparent text-xs outline-none"
           style="color: var(--sempa-text);" />
    {#if newTitle.trim()}
      <button onclick={add} disabled={adding}
              class="text-xs disabled:opacity-40" style="color: var(--sempa-accent);">
        Add
      </button>
    {/if}
  </div>
</div>

<style>
  .subrow:hover { background: var(--sempa-accent-bg); }
  .addrow:focus-within { border-color: var(--sempa-accent); }
</style>
