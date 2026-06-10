<script lang="ts">
  // Reusable tag filter: a row of selectable tag chips plus an AND/OR toggle
  // (shown once 2+ tags are picked). Bindable `selected` + `match` so callers
  // can react. Used by the Search page and the inline Day/Week filter.
  import { tagStore } from '$lib/stores/tags.svelte';

  let {
    selected = $bindable<string[]>([]),
    match = $bindable<'any' | 'all'>('any'),
  }: {
    selected?: string[];
    match?: 'any' | 'all';
  } = $props();

  function toggle(name: string) {
    selected = selected.includes(name)
      ? selected.filter((t) => t !== name)
      : [...selected, name];
  }
</script>

{#if tagStore.definitions.length}
  <div class="flex flex-wrap items-center gap-1.5">
    {#each tagStore.definitions as tag (tag.id)}
      {@const active = selected.includes(tag.name)}
      <button onclick={() => toggle(tag.name)}
              class="inline-flex items-center gap-1.5 rounded-full transition-colors"
              style="font-size: 12px; padding: 4px 10px;
                     {active
                       ? `background: color-mix(in srgb, ${tag.color} 18%, transparent); color: ${tag.color}; box-shadow: inset 0 0 0 1px ${tag.color};`
                       : 'color: var(--sempa-text-soft); box-shadow: inset 0 0 0 1px var(--sempa-border);'}">
        <span class="h-2 w-2 shrink-0 rounded-full" style="background: {tag.color};"></span>
        {tag.name}
      </button>
    {/each}

    {#if selected.length >= 2}
      <!-- AND/OR toggle for combining the selected tags. -->
      <div class="ml-1 inline-flex overflow-hidden rounded-full" style="box-shadow: inset 0 0 0 1px var(--sempa-border);">
        {#each [{ k: 'any', l: 'Any' }, { k: 'all', l: 'All' }] as opt}
          {@const on = match === opt.k}
          <button onclick={() => (match = opt.k as 'any' | 'all')}
                  style="font-size: 11px; padding: 4px 9px;
                         {on ? 'background: var(--sempa-accent); color: var(--sempa-btn-fg);' : 'color: var(--sempa-text-dim);'}">
            {opt.l}
          </button>
        {/each}
      </div>
    {/if}

    {#if selected.length}
      <button onclick={() => (selected = [])}
              class="ml-0.5 rounded-full transition-colors"
              style="font-size: 12px; padding: 4px 8px; color: var(--sempa-text-dim);">
        Clear
      </button>
    {/if}
  </div>
{/if}
