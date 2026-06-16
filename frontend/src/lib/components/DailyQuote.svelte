<script lang="ts">
  /**
   * A quiet line of daily encouragement. Subtle by design — small, dim, and
   * easy to ignore — so it adds personality without nagging. Renders nothing
   * when disabled or the list is empty. The quote is stable through the day.
   */
  import { quotes } from '$lib/stores/quotes.svelte';

  const q = $derived(quotes.todays);
</script>

{#if q}
  <p class="select-none text-center text-[12px] italic leading-snug" style="color: var(--sempa-text-dim);">
    <span style="color: var(--sempa-accent); opacity: 0.6;">“</span>{q.text}<span style="color: var(--sempa-accent); opacity: 0.6;">”</span>
    <span class="not-italic" style="opacity: 0.75;"> — {q.author}</span>
  </p>
{/if}
