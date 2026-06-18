<script lang="ts">
  /**
   * The daily encouragement. Two variants:
   *
   *  - `collapse` (default — mobile): fades in under the header, holds for a few
   *    seconds so it's read as a greeting, then smoothly eases its own height to
   *    zero and gets out of the way so the top section isn't permanently taller.
   *
   *  - `inline` (desktop): a single quiet line that lives *beside* the header
   *    content rather than on its own row, so it adds no vertical height. The
   *    caller hides it (display) when the window is too narrow to spare the room.
   *
   * Stable for the whole day (quotes.todays is seeded off the date). Honours the
   * Settings toggle (renders nothing when off) and respects prefers-reduced-motion
   * (renders straight at the resting opacity, and never auto-collapses).
   */
  import { quotes } from '$lib/stores/quotes.svelte';
  import { onMount } from 'svelte';

  let { variant = 'collapse' }: { variant?: 'collapse' | 'inline' } = $props();

  const REST = 0.42;
  const HOLD_MS = 8400; // visible before it eases away (collapse variant only)

  let shown = $state(false);   // fade-in has begun
  let gone = $state(false);    // collapse variant has eased its height away
  let reduced = $state(false);

  const q = $derived(quotes.todays);

  onMount(() => {
    reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    // Inline (desktop) and reduced-motion both render statically — no entrance,
    // no auto-collapse. The inline variant relies on the caller's media query to
    // hide it on small windows.
    if (variant !== 'collapse' || reduced) { shown = true; return; }
    const raf = requestAnimationFrame(() => { shown = true; });
    const t = setTimeout(() => { gone = true; }, HOLD_MS);
    return () => { cancelAnimationFrame(raf); clearTimeout(t); };
  });

  const collapseOpacity = $derived(reduced ? REST : gone ? 0 : shown ? 1 : 0);
</script>

{#if q}
  {#if variant === 'inline'}
    <p class="quote-inline" style="opacity: {REST};" title={q.author ? `“${q.text}” — ${q.author}` : q.text}>
      “{q.text}”{#if q.author}<span class="author"> — {q.author}</span>{/if}
    </p>
  {:else}
    <!-- Self-managed vertical space: the wrapper owns its padding so that when it
         collapses, the entire band (text + spacing) reclaims to zero height. The
         caller should give only horizontal padding. -->
    <div class="quote-collapse" class:gone style="opacity: {collapseOpacity};">
      <p class="daily-quote">
        “{q.text}”{#if q.author}<span class="author"> — {q.author}</span>{/if}
      </p>
    </div>
  {/if}
{/if}

<style>
  /* ── Collapse variant (mobile) ─────────────────────────────────────────── */
  .quote-collapse {
    max-height: 5rem;
    padding-top: 0.25rem;
    padding-bottom: 0.5rem;
    overflow: hidden;
    transition: max-height 620ms cubic-bezier(0.4, 0, 0.2, 1),
                opacity 600ms ease,
                padding 620ms cubic-bezier(0.4, 0, 0.2, 1);
  }
  .quote-collapse.gone {
    max-height: 0;
    padding-top: 0;
    padding-bottom: 0;
  }
  .daily-quote {
    margin: 0 auto;
    max-width: 640px;
    font-size: 13px;
    font-style: italic;
    line-height: 1.5;
    text-align: center;
    text-wrap: balance;
    color: var(--sempa-text-dim);
  }

  /* ── Inline variant (desktop, to the side of the header) ───────────────── */
  .quote-inline {
    margin: 0;
    min-width: 0;
    max-width: 100%;
    font-size: 12.5px;
    font-style: italic;
    line-height: 1.4;
    text-align: center;
    color: var(--sempa-text-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .author {
    font-style: normal;
    opacity: 0.7;
  }

  @media (prefers-reduced-motion: reduce) {
    .quote-collapse { transition: none; }
  }
</style>
