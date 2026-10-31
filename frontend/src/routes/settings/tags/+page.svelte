<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { tagStore } from '$lib/stores/tags.svelte';
  import type { TagDefinition } from '$lib/types';

  const PALETTE = [
    '#3b82f6','#10b981','#f59e0b','#ef4444',
    '#8b5cf6','#ec4899','#06b6d4','#84cc16',
  ];

  let tags = $state<TagDefinition[]>([]);
  let newName = $state('');
  let newColor = $state(PALETTE[0]);
  let saving = $state(false);
  let error = $state('');

  onMount(async () => {
    tags = await api.tags.list();
  });

  async function create() {
    if (!newName.trim()) return;
    saving = true; error = '';
    try {
      const tag = await api.tags.create(newName.trim(), newColor);
      tags = [...tags, tag];
      tagStore.add(tag);
      newName = '';
      newColor = PALETTE[tags.length % PALETTE.length];
    } catch (e) { error = (e as Error).message; }
    finally { saving = false; }
  }

  async function updateColor(tag: TagDefinition, color: string) {
    const updated = await api.tags.update(tag.id, color);
    tags = tags.map(t => t.id === updated.id ? updated : t);
    tagStore.add(updated);
  }

  async function remove(id: string) {
    await api.tags.delete(id);
    tags = tags.filter(t => t.id !== id);
    tagStore.remove(id);
  }
</script>

<div class="mx-auto max-w-[640px] px-5 pt-6 pb-16 sm:px-8 sm:pt-8">
  <a href="/settings/accounts?section=tasks" class="set-back">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" d="m15 18-6-6 6-6"/>
    </svg>
    Settings
  </a>

  <h1 class="set-title type-page">Tags</h1>
  <p class="set-lede mb-6">
    Create tags to categorise tasks. Use them in task titles as <code class="rounded-md px-1.5 py-0.5 text-[12.5px]" style="background: var(--sempa-fill); color: var(--sempa-text);">#work</code> or pick from the list.
  </p>

  <!-- Existing tags -->
  {#if tags.length}
    <div class="set-group mb-6">
      {#each tags as tag (tag.id)}
        <div class="set-row set-row-compact">
          <input type="color" value={tag.color}
                 onchange={(e) => updateColor(tag, (e.target as HTMLInputElement).value)}
                 aria-label="Colour for {tag.name}"
                 class="set-color" />
          <span class="set-row-main set-label truncate">{tag.name}</span>
          <span class="tint chip" style="--tint: {tag.color};">{tag.name}</span>
          <button onclick={() => remove(tag.id)} aria-label="Delete {tag.name}"
                  class="set-icon-btn danger">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" d="M6 18 18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>
      {/each}
    </div>
  {:else}
    <p class="set-meta mb-6 px-1.5">No tags yet.</p>
  {/if}

  <!-- Create new tag -->
  <h2 class="set-heading">New tag</h2>
  <div class="set-group">
    <div class="set-block">
      <div class="flex items-center gap-3">
        <input type="color" bind:value={newColor} aria-label="New tag colour"
               class="set-color" style="width: 34px; height: 34px;" />
        <input type="text" bind:value={newName}
               onkeydown={(e) => e.key === 'Enter' && create()}
               placeholder="Tag name (e.g. work, personal)"
               class="set-input min-w-0 flex-1" />
        <button onclick={create} disabled={saving || !newName.trim()}
                class="btn-primary set-btn">
          Add
        </button>
      </div>
      {#if error}<p class="set-error mt-2">{error}</p>{/if}
    </div>
  </div>
</div>
