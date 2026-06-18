<script lang="ts">
  /**
   * The daily encouragement. Two variants:
   *
   *  - `collapse` (default — mobile): fades in under the header, holds for a few
   *    seconds so it's read as a greeting, then smoothly eases its own height to
   *    zero and gets out of the way so the top section isn't permanently taller.
   *
   *  - `inline` (desktop): a single quiet line that lives *beside* the header
   *    content rather than on its own row, so it adds no vertical height. When
   *    the host is too narrow to fit the full line, it collapses to a quiet
   *    quote-mark button that reveals the whole quote in a hover/focus popover —
   *    so a tight window never just chops the quote mid-word, and the reader
   *    never has to widen the window to see it.
   *
   * Stable for the whole day (quotes.todays is seeded off the date). Honours the
   * Settings toggle (renders nothing when off) and respects prefers-reduced-motion
   * (renders straight at the resting opacity, and never auto-collapses).
   */
  import { quotes } from '$lib/stores/quotes.svelte';
  import { onMount, untrack } from 'svelte';
  import { Quote } from 'lucide-svelte';

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

  /* ── Inline overflow handling ───────────────────────────────────────────────
     Compare the full line's natural width (an always-present, off-screen measurer
     — so the check is stable no matter which branch renders) against the host's
     available width. Too narrow → collapse to the quote-mark button + popover. */
  let hostEl = $state<HTMLElement>();
  let measureEl = $state<HTMLElement>();
  let collapsed = $state(false);
  let popOpen = $state(false);

  $effect(() => {
    if (variant !== 'inline' || !hostEl || !measureEl) return;
    const host = hostEl, meas = measureEl;
    const recheck = () => {
      const next = meas.scrollWidth > host.clientWidth + 1;
      // Read+write `collapsed` untracked so this effect never re-triggers itself
      // (a tracked read-modify-write loops → effect_update_depth_exceeded; see
      // CLAUDE.md "$effect read-modify-write" gotcha).
      untrack(() => { if (next !== collapsed) collapsed = next; });
    };
    recheck();
    const ro = new ResizeObserver(recheck);
    ro.observe(host);
    return () => ro.disconnect();
  });
</script>

{#if q}
  {#if variant === 'inline'}
    <div class="quote-host" bind:this={hostEl}>
      <!-- Always present, off-screen: full line at natural width, so the fit
           check is independent of which branch is currently rendered. -->
      <span class="quote-measure" bind:this={measureEl} aria-hidden="true"
        >“{q.text}”{#if q.author} — {q.author}{/if}</span>

      {#if collapsed}
        <span class="quote-popwrap">
          <button type="button" class="quote-chip"
                  aria-label={q.author ? `Quote of the day: “${q.text}” — ${q.author}` : `Quote of the day: “${q.text}”`}
                  onmouseenter={() => (popOpen = true)}
                  onmouseleave={() => (popOpen = false)}
                  onfocus={() => (popOpen = true)}
                  onblur={() => (popOpen = false)}>
            <Quote size={15} strokeWidth={2} />
          </button>
          {#if popOpen}
            <span class="quote-pop" role="tooltip">
              “{q.text}”{#if q.author}<span class="author"> — {q.author}</span>{/if}
            </span>
          {/if}
        </span>
      {:else}
        <p class="quote-inline" style="opacity: {REST};">
          “{q.text}”{#if q.author}<span class="author"> — {q.author}</span>{/if}
        </p>
      {/if}
    </div>
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

  /* ── Inline → collapsed quote-mark button + popover ─────────────────────── */
  .quote-host {
    position: relative;
    display: flex;
    min-width: 0;
    max-width: 100%;
    align-items: center;
    justify-content: center;
  }
  /* Off-screen measurer: mirrors .quote-inline metrics, never wraps, no layout. */
  .quote-measure {
    position: absolute;
    top: 0;
    left: 0;
    visibility: hidden;
    pointer-events: none;
    white-space: nowrap;
    font-size: 12.5px;
    font-style: italic;
    line-height: 1.4;
  }
  .quote-popwrap {
    position: relative;
    display: inline-flex;
  }
  .quote-chip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: 30px;
    width: 30px;
    border-radius: 9px;
    color: var(--sempa-text-dim);
    background: transparent;
    cursor: pointer;
    transition: color 150ms ease, background 150ms ease;
  }
  .quote-chip:hover,
  .quote-chip:focus-visible {
    color: var(--sempa-accent);
    background: var(--sempa-accent-bg);
    outline: none;
  }
  .quote-pop {
    position: absolute;
    top: calc(100% + 8px);
    left: 50%;
    transform: translateX(-50%);
    z-index: 60;
    display: block;
    width: max-content;
    max-width: min(320px, 70vw);
    padding: 9px 12px;
    border-radius: 10px;
    background: var(--sempa-bg-panel);
    border: 1px solid var(--sempa-border);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
    font-size: 12.5px;
    font-style: italic;
    line-height: 1.45;
    text-align: center;
    color: var(--sempa-text);
    animation: quote-pop-in 140ms ease;
  }
  /* Upward-pointing arrow (border, then panel-fill on top). */
  .quote-pop::before,
  .quote-pop::after {
    content: "";
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
  }
  .quote-pop::before { border-bottom-color: var(--sempa-border); }
  .quote-pop::after { margin-bottom: -1px; border-bottom-color: var(--sempa-bg-panel); }

  @keyframes quote-pop-in {
    from { opacity: 0; transform: translateX(-50%) translateY(-3px); }
    to   { opacity: 1; transform: translateX(-50%) translateY(0); }
  }

  @media (prefers-reduced-motion: reduce) {
    .quote-collapse { transition: none; }
    .quote-pop { animation: none; }
  }
</style>
