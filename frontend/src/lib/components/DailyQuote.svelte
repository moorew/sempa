<script lang="ts">
  /**
   * The daily encouragement — a quiet companion line. It fades in on mount, then
   * settles to a low resting opacity so it adds personality without competing for
   * attention. The quote WRAPS freely (2–3 lines is fine) and is never truncated:
   * the whole point is reading the whole thought.
   *
   * Stable for the whole day (quotes.todays is seeded off the date), honours the
   * Settings toggle (renders nothing when off) and respects prefers-reduced-motion
   * (renders straight at the resting opacity, with no fade).
   */
  import { quotes } from '$lib/stores/quotes.svelte';
  import { onMount } from 'svelte';

  const REST = 0.42;

  let shown = $state(false);
  let reduced = $state(false);

  const q = $derived(quotes.todays);
  const opacity = $derived(shown ? REST : 0);

  onMount(() => {
    reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduced) { shown = true; return; }
    const raf = requestAnimationFrame(() => { shown = true; });
    return () => cancelAnimationFrame(raf);
  });
</script>

{#if q}
  <p class="mquote" class:reduced style="opacity: {opacity};">
    “{q.text}”{#if q.author}<span class="author"> — {q.author}</span>{/if}
  </p>
{/if}

<style>
  .mquote {
    margin: 0.25rem auto 0.5rem;
    max-width: 640px;
    font-size: 12.75px;
    font-style: italic;
    line-height: 1.5;
    text-align: center;
    text-wrap: balance; /* wraps — never truncates */
    color: var(--sempa-text-dim);
    transition: opacity 900ms ease;
  }
  .mquote.reduced { transition: none; }
  .author {
    font-style: normal;
    opacity: 0.8;
  }
</style>
