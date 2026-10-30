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
              aria-pressed={active}
              class="inline-flex h-8 items-center gap-1.5 rounded-full px-3 font-semibold transition-colors"
              style="font-size: 12.5px;
                     {active
                       ? `background: color-mix(in srgb, ${tag.color} var(--sempa-tint-bg), var(--sempa-bg-panel)); color: color-mix(in srgb, ${tag.color} var(--sempa-tint-fg), var(--sempa-text)); box-shadow: inset 0 0 0 1.5px ${tag.color};`
                       : 'background: var(--sempa-bg-panel); color: var(--sempa-text-soft); box-shadow: var(--sempa-shadow-1);'}">
        <span class="h-2 w-2 shrink-0 rounded-full" style="background: {tag.color};"></span>
        {tag.name}
      </button>
    {/each}

    {#if selected.length >= 2}
      <!-- AND/OR toggle for combining the selected tags. -->
      <div class="ml-1 inline-flex gap-0.5 overflow-hidden rounded-full p-0.5" style="background: var(--sempa-fill);">
        {#each [{ k: 'any', l: 'Any' }, { k: 'all', l: 'All' }] as opt}
          {@const on = match === opt.k}
          <button onclick={() => (match = opt.k as 'any' | 'all')}
                  class="rounded-full font-semibold"
                  style="font-size: 11.5px; padding: 5px 11px;
                         {on ? 'background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text);' : 'color: var(--sempa-text-soft);'}">
            {opt.l}
          </button>
        {/each}
      </div>
    {/if}

    {#if selected.length}
      <button onclick={() => (selected = [])}
              class="ml-0.5 h-8 rounded-full px-3 font-semibold transition-colors"
              style="font-size: 12.5px; color: var(--sempa-accent-ink);">
        Clear
      </button>
    {/if}
  </div>
{/if}
