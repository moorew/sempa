<script lang="ts">
  /**
   * AI Import — paste a URL or text (a recipe, itinerary, brief…) and the local
   * model turns it into one task whose steps become subtasks, plus a companion
   * list (ingredients → groceries, packing items, materials…). An editable
   * preview stands between the model and your data: a small local model will
   * occasionally mis-extract, and a silent wrong task+list is worse than none.
   *
   * Mounted once in the root layout; opened from anywhere via the importModal
   * store (the Lists button and the keyboard quick-launch both call show()).
   */
  import { api } from '$lib/api';
  import { importModal } from '$lib/stores/importModal.svelte';
  import { today } from '$lib/utils';
  import { goto } from '$app/navigation';
  import { Sparkles, X, Trash2, Loader2, ListChecks, ListTodo } from 'lucide-svelte';

  type Phase = 'input' | 'loading' | 'preview' | 'creating';

  let phase = $state<Phase>('input');
  let input = $state('');
  let error = $state('');

  // Preview state (all editable before creating).
  let kind = $state('generic');
  let title = $state('');
  let notes = $state('');
  let sourceUrl = $state('');
  let steps = $state<string[]>([]);
  let items = $state<{ text: string; include: boolean }[]>([]);
  let listName = $state('');
  let makeList = $state(true);
  let placement = $state<'today' | 'backlog'>('today');

  const looksLikeUrl = (s: string) => /^https?:\/\/\S+$/i.test(s.trim());

  // Reset to a clean input phase whenever the modal (re)opens.
  $effect(() => {
    if (importModal.open) {
      phase = 'input';
      input = importModal.seed;
      error = '';
    }
  });

  function reset() {
    phase = 'input';
    error = '';
    steps = [];
    items = [];
  }

  function close() {
    if (phase === 'creating') return; // don't abandon a write in flight
    importModal.close();
  }

  async function generate() {
    const text = input.trim();
    if (!text) return;
    phase = 'loading';
    error = '';
    try {
      const res = await api.ai.import(looksLikeUrl(text) ? { url: text } : { text });
      if (!res.available) {
        error = 'AI isn’t set up. Connect a local model in Settings → Accounts.';
        phase = 'input';
        return;
      }
      kind = res.type ?? 'generic';
      title = (res.title ?? '').trim();
      notes = (res.notes ?? '').trim();
      sourceUrl = res.source_url ?? '';
      steps = (res.steps ?? []).map((s) => s.trim()).filter(Boolean);
      items = (res.items ?? []).map((t) => ({ text: t.trim(), include: true })).filter((i) => i.text);
      listName = (res.list_name ?? '').trim();
      makeList = items.length > 0;
      if (!title && steps.length === 0 && items.length === 0) {
        error = 'Couldn’t find anything to import from that. Try pasting the text directly.';
        phase = 'input';
        return;
      }
      phase = 'preview';
    } catch (e) {
      error = e instanceof Error ? e.message : 'Import failed. Please try again.';
      phase = 'input';
    }
  }

  function removeStep(i: number) { steps = steps.filter((_, idx) => idx !== i); }
  function removeItem(i: number) { items = items.filter((_, idx) => idx !== i); }

  const chosenItems = $derived(items.filter((i) => i.include && i.text.trim()));
  const canCreate = $derived(title.trim().length > 0);

  async function create() {
    if (!canCreate) return;
    phase = 'creating';
    error = '';
    try {
      const plannedDate = placement === 'today' ? today() : undefined;
      const status = placement === 'today' ? 'planned' : 'backlog';

      const description = notes.trim();

      const parent = await api.tasks.create({
        title: title.trim(),
        description: description || undefined,
        source_url: sourceUrl || undefined,
        planned_date: plannedDate,
        status,
        position: 0,
      });

      // Steps → subtasks, in order, sharing the parent's placement.
      let pos = 0;
      for (const s of steps.map((x) => x.trim()).filter(Boolean)) {
        await api.tasks.create({
          title: s,
          parent_task_id: parent.id,
          planned_date: plannedDate,
          status,
          position: pos++,
        });
      }

      // Ingredients / gather items → a new list linked to the task.
      let createdListId = '';
      const picked = chosenItems;
      if (makeList && picked.length > 0) {
        const list = await api.lists.create({
          name: listName.trim() || `${title.trim()} — items`,
          task_id: parent.id,
        });
        createdListId = list.id;
        for (const it of picked) await api.lists.addItem(list.id, it.text.trim());
      }

      importModal.close();
      // Land somewhere useful: the shopping list if we made one, else the task.
      if (createdListId) await goto(`/lists/${createdListId}`);
      else if (plannedDate) await goto(`/day/${plannedDate}`);
      else await goto('/backlog');
    } catch (e) {
      error = e instanceof Error ? e.message : 'Couldn’t create everything. Please try again.';
      phase = 'preview';
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }
</script>

<svelte:window onkeydown={importModal.open ? onKeydown : undefined} />

{#if importModal.open}
  <!-- Backdrop -->
  <div class="fixed inset-0 z-[100] flex items-start justify-center p-4 sm:p-6"
       style="background: rgba(0,0,0,0.45);"
       onclick={(e) => { if (e.target === e.currentTarget) close(); }}
       role="presentation">
    <div class="mt-[6vh] flex w-full max-w-lg flex-col overflow-hidden rounded-2xl shadow-2xl"
         style="max-height: 84vh; background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);"
         role="dialog" aria-modal="true" aria-label="Import with AI">

      <!-- Header (actions anchored at the top for mobile keyboard safety) -->
      <div class="flex items-center gap-2.5 px-5 py-3.5" style="border-bottom: 1px solid var(--sempa-border);">
        <Sparkles size={18} style="color: var(--sempa-accent);" />
        <h2 class="flex-1 text-sm font-semibold" style="color: var(--sempa-text);">
          {phase === 'preview' || phase === 'creating' ? 'Review before creating' : 'Import with AI'}
        </h2>
        <button onclick={close} disabled={phase === 'creating'} aria-label="Close"
                class="rounded-lg p-1 transition-opacity hover:opacity-70 disabled:opacity-40" style="color: var(--sempa-text-dim);">
          <X size={18} />
        </button>
      </div>

      <div class="flex-1 overflow-y-auto px-5 py-4">
        {#if phase === 'input' || phase === 'loading'}
          <p class="mb-2 text-xs" style="color: var(--sempa-text-soft);">
            Paste a link or text — a recipe, an itinerary, a brief. Sempa will draft a task with
            steps as subtasks and, when there are things to buy or gather, a shopping list.
          </p>
          <textarea bind:value={input} rows={6} disabled={phase === 'loading'}
                    placeholder="https://example.com/recipe   —or—   paste the full text here…"
                    class="w-full resize-none rounded-xl px-3 py-2.5 text-sm outline-none disabled:opacity-60"
                    style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);"></textarea>
          {#if error}
            <p class="mt-2 text-xs" style="color: #dc2626;">{error}</p>
          {/if}
        {:else if phase === 'preview' || phase === 'creating'}
          {#if error}
            <p class="mb-3 text-xs" style="color: #dc2626;">{error}</p>
          {/if}

          <!-- Task title -->
          <label class="mb-1 block text-[11px] font-medium uppercase tracking-wide" style="color: var(--sempa-text-dim);" for="imp-title">Task</label>
          <input id="imp-title" bind:value={title} placeholder="Task title"
                 class="mb-4 w-full rounded-xl px-3 py-2.5 text-sm font-medium outline-none"
                 style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />

          <!-- Placement -->
          <div class="mb-4 flex gap-2">
            <button onclick={() => (placement = 'today')}
                    class="flex flex-1 items-center justify-center gap-1.5 rounded-lg px-3 py-2 text-xs font-medium transition-colors"
                    style="border: 1px solid {placement === 'today' ? 'var(--sempa-accent)' : 'var(--sempa-border)'};
                           background: {placement === 'today' ? 'var(--sempa-accent-bg)' : 'transparent'};
                           color: {placement === 'today' ? 'var(--sempa-accent)' : 'var(--sempa-text-soft)'};">
              <ListTodo size={14} /> Today
            </button>
            <button onclick={() => (placement = 'backlog')}
                    class="flex flex-1 items-center justify-center gap-1.5 rounded-lg px-3 py-2 text-xs font-medium transition-colors"
                    style="border: 1px solid {placement === 'backlog' ? 'var(--sempa-accent)' : 'var(--sempa-border)'};
                           background: {placement === 'backlog' ? 'var(--sempa-accent-bg)' : 'transparent'};
                           color: {placement === 'backlog' ? 'var(--sempa-accent)' : 'var(--sempa-text-soft)'};">
              Backlog
            </button>
          </div>

          <!-- Steps → subtasks -->
          {#if steps.length > 0}
            <p class="mb-1.5 text-[11px] font-medium uppercase tracking-wide" style="color: var(--sempa-text-dim);">
              Steps ({steps.length} subtasks)
            </p>
            <div class="mb-4 flex flex-col gap-1.5">
              {#each steps as _, i (i)}
                <div class="flex items-center gap-2">
                  <span class="w-5 shrink-0 text-right text-xs" style="color: var(--sempa-text-dim);">{i + 1}.</span>
                  <input bind:value={steps[i]}
                         class="flex-1 rounded-lg px-2.5 py-1.5 text-sm outline-none"
                         style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
                  <button onclick={() => removeStep(i)} aria-label="Remove step"
                          class="shrink-0 rounded p-1 transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);">
                    <Trash2 size={14} />
                  </button>
                </div>
              {/each}
            </div>
          {/if}

          <!-- Items → list -->
          {#if items.length > 0}
            <div class="mb-1.5 flex items-center justify-between">
              <p class="text-[11px] font-medium uppercase tracking-wide" style="color: var(--sempa-text-dim);">
                <ListChecks size={12} class="mb-0.5 inline" /> Shopping list ({chosenItems.length})
              </p>
              <label class="flex items-center gap-1.5 text-xs" style="color: var(--sempa-text-soft);">
                <input type="checkbox" bind:checked={makeList} style="accent-color: var(--sempa-accent);" />
                Create list
              </label>
            </div>
            {#if makeList}
              <input bind:value={listName} placeholder="List name"
                     class="mb-2 w-full rounded-lg px-2.5 py-1.5 text-xs outline-none"
                     style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text);" />
              <div class="flex flex-col gap-1.5">
                {#each items as item, i (i)}
                  <div class="flex items-center gap-2">
                    <input type="checkbox" bind:checked={item.include} style="accent-color: var(--sempa-accent);" />
                    <input bind:value={item.text}
                           class="flex-1 rounded-lg px-2.5 py-1.5 text-sm outline-none"
                           style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main); color: var(--sempa-text); {item.include ? '' : 'opacity:0.5;'}" />
                    <button onclick={() => removeItem(i)} aria-label="Remove item"
                            class="shrink-0 rounded p-1 transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);">
                      <Trash2 size={14} />
                    </button>
                  </div>
                {/each}
              </div>
            {/if}
          {/if}
        {/if}
      </div>

      <!-- Footer actions -->
      <div class="flex items-center justify-end gap-2 px-5 py-3.5" style="border-top: 1px solid var(--sempa-border);">
        {#if phase === 'input' || phase === 'loading'}
          <button onclick={generate} disabled={!input.trim() || phase === 'loading'}
                  class="flex items-center gap-1.5 rounded-xl px-4 py-2 text-sm font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-40"
                  style="background: var(--sempa-accent);">
            {#if phase === 'loading'}<Loader2 size={15} class="animate-spin" /> Reading…{:else}<Sparkles size={15} /> Generate{/if}
          </button>
        {:else}
          <button onclick={reset} disabled={phase === 'creating'}
                  class="rounded-xl px-3 py-2 text-sm font-medium transition-opacity hover:opacity-70 disabled:opacity-40"
                  style="color: var(--sempa-text-soft);">
            Start over
          </button>
          <button onclick={create} disabled={!canCreate || phase === 'creating'}
                  class="flex items-center gap-1.5 rounded-xl px-4 py-2 text-sm font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-40"
                  style="background: var(--sempa-accent);">
            {#if phase === 'creating'}<Loader2 size={15} class="animate-spin" /> Creating…{:else}Create task{makeList && chosenItems.length > 0 ? ' + list' : ''}{/if}
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}
