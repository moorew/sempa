<script lang="ts">
  /**
   * A brief, quiet quote that fades in for a moment on small wins (e.g. completing
   * a task) and fades away — driven by quotes.flash(). Mounted once in the layout.
   * Deliberately understated and click-through (pointer-events: none).
   */
  import { quotes } from '$lib/stores/quotes.svelte';
  import { fade } from 'svelte/transition';

  const q = $derived(quotes.moment);
</script>

{#if q}
  <div class="pointer-events-none fixed bottom-8 left-1/2 z-[55] max-w-[80vw] -translate-x-1/2 text-center"
       transition:fade={{ duration: 400 }}>
    <p class="text-[12px] italic leading-snug" style="color: var(--sempa-text-dim);">
      “{q.text}”<span class="not-italic" style="opacity: 0.65;"> — {q.author}</span>
    </p>
  </div>
{/if}
