<script lang="ts">
  /**
   * The day's intention card — and, until an intention is set, the quiet home for
   * the daily quote. Reflection paired with the act of planning the day:
   *
   *  - No intention yet → a compact "+ Set intention" chip shares the row with the
   *    day's quote. The quote takes the remaining space and WRAPS (never truncates)
   *    — the whole value of a quote is reading the whole thought.
   *  - Intention set → your own words take over: clipboard icon + the intention
   *    text + a success check. No quote is shown once an intention exists.
   *
   * On a narrow viewport (≤640px) the chip and quote stop sharing a line and stack.
   * The quote is drawn from the same daily-stable pool as elsewhere (quotes.todays),
   * honours the Settings off-toggle (renders nothing when off) and uses only the
   * --sempa-* / --card-* tokens.
   */
  import { quotes } from '$lib/stores/quotes.svelte';
  import { ClipboardCheck, Check, Plus } from 'lucide-svelte';

  let {
    date,
    intention = null,
    promptWhenEmpty = false,
  }: {
    date: string;
    intention?: string | null;
    promptWhenEmpty?: boolean;
  } = $props();

  const hasIntention = $derived(!!intention?.trim());
  const q = $derived(quotes.todays);

  // Only render at all when there's an intention to show or it's the relevant day.
  const visible = $derived(hasIntention || promptWhenEmpty);
</script>

{#if visible}
  <div class="intention" class:intention-set={hasIntention}>
    <div class="intention-inner">
      {#if hasIntention}
        <!-- State B: the committed intention takes the full row. -->
        <a class="intention-display" href="/plan/{date}" title="Edit today's plan">
          <span class="intention-icon"><ClipboardCheck size={15} strokeWidth={2} /></span>
          <span class="intention-copy">
            <span class="intention-label">Today's intention</span>
            <span class="intention-text">{intention}</span>
          </span>
          <span class="check-ring check" data-state="done" style="width: 22px; height: 22px;" aria-hidden="true">
            <Check size={13} strokeWidth={3} />
          </span>
        </a>
      {:else}
        <!-- State A: compact chip + quote share the line (quote wraps, never clips). -->
        <a class="set-chip" href="/plan/{date}">
          <Plus size={14} strokeWidth={2.25} />
          Set intention
        </a>
        {#if q}
          <p class="intention-quote">
            <span class="q-mark" aria-hidden="true">“</span>
            <span class="q-text">{q.text}{#if q.author}<span class="q-author"> — {q.author}</span>{/if}</span>
          </p>
        {/if}
      {/if}
    </div>
  </div>
{/if}

<style>
  /* Unset: a raised surface holding the chip + quote. Set: the soft
     accent-tinted "intention" card (accent-bg, accent-ink label). */
  .intention {
    background: var(--card-bg);
    border-radius: var(--radius-card);
    box-shadow: var(--sempa-shadow-1);
    transition: box-shadow 0.15s, background-color 0.15s;
  }
  .intention.intention-set {
    background: var(--sempa-accent-bg);
    box-shadow: none;
  }
  .intention.intention-set:hover {
    box-shadow: inset 0 0 0 1.5px color-mix(in srgb, var(--sempa-accent) 35%, transparent);
  }
  .intention-inner {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 10px 14px 10px 10px;
  }

  /* unset: compact chip + quote share the line */
  .set-chip {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    flex-shrink: 0;
    height: 36px;
    border-radius: 9999px;
    padding: 0 16px 0 12px;
    font-size: 13px;
    font-weight: 700;
    color: var(--sempa-accent-ink);
    background: var(--sempa-accent-bg);
    transition: 0.12s;
    white-space: nowrap;
    text-decoration: none;
  }
  .set-chip:hover { filter: brightness(0.97); }

  .intention-quote {
    display: flex;
    gap: 9px;
    align-items: baseline;
    flex: 1 1 auto;
    min-width: 0;
    margin: 0;
  }
  .intention-quote .q-mark {
    color: var(--sempa-accent);
    font-weight: 700;
    flex-shrink: 0;
    opacity: 0.9;
  }
  .intention-quote .q-text {
    font-style: italic;
    font-size: 13.5px;
    line-height: 1.5;
    color: var(--sempa-text-soft);
    text-wrap: pretty; /* wraps — never truncates */
  }
  .intention-quote .q-author {
    font-style: normal;
    color: var(--sempa-text-dim);
    white-space: nowrap;
  }

  /* set: committed intention replaces chip + quote */
  .intention-display {
    display: flex;
    align-items: center;
    gap: 12px;
    flex: 1 1 auto;
    min-width: 0;
    text-decoration: none;
  }
  .intention-icon {
    display: inline-grid; place-items: center; flex: 0 0 auto;
    width: 34px; height: 34px; border-radius: 12px;
    background: var(--sempa-bg-panel); color: var(--sempa-accent-ink);
  }
  .intention-copy { display: flex; flex-direction: column; min-width: 0; flex: 1 1 auto; }
  .intention-label { font-size: 12px; font-weight: 600; line-height: 1.3; color: var(--sempa-accent-ink); }
  .intention-display .intention-text {
    font-size: 14.5px;
    font-weight: 600;
    line-height: 1.35;
    color: var(--sempa-text);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .intention-display .check {
    margin-left: auto;
    flex-shrink: 0;
  }

  /* Narrow: stop sharing the line — vertical space is the cheap dimension here. */
  @media (max-width: 640px) {
    .intention-inner {
      flex-direction: column;
      align-items: stretch;
      gap: 10px;
    }
    .set-chip { align-self: flex-start; }
  }
</style>
