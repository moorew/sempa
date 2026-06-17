<script lang="ts">
  /**
   * The daily encouragement, as a single quiet line under the day/week header.
   * Fades in on mount, then after a few seconds eases down to a resting opacity
   * so it stays present but recedes. Stable for the whole day (quotes.todays is
   * seeded off the date). Honours the Settings toggle (renders nothing when off)
   * and respects prefers-reduced-motion (renders straight at the resting opacity).
   */
  import { quotes } from '$lib/stores/quotes.svelte';
  import { onMount } from 'svelte';

  const REST = 0.42;

  let shown = $state(false);   // fade-in has begun
  let settled = $state(false); // eased down to the resting opacity
  let reduced = $state(false);

  const q = $derived(quotes.todays);

  onMount(() => {
    reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduced) return;
    const raf = requestAnimationFrame(() => { shown = true; });
    const t = setTimeout(() => { settled = true; }, 6000);
    return () => { cancelAnimationFrame(raf); clearTimeout(t); };
  });

  const opacity = $derived(reduced ? REST : !shown ? 0 : settled ? REST : 1);
</script>

{#if q}
  <p class="daily-quote" class:settled style="opacity: {opacity};">
    “{q.text}”<span class="author"> — {q.author}</span>
  </p>
{/if}

<style>
  .daily-quote {
    margin: 0 auto;
    max-width: 640px;
    font-size: 13px;
    font-style: italic;
    line-height: 1.5;
    text-align: center;
    text-wrap: balance;
    color: var(--sempa-text-dim);
    transition: opacity 900ms ease;
  }
  .daily-quote.settled {
    transition: opacity 1200ms ease;
  }
  .author {
    font-style: normal;
    opacity: 0.7;
  }
  @media (prefers-reduced-motion: reduce) {
    .daily-quote { transition: none; }
  }
</style>
