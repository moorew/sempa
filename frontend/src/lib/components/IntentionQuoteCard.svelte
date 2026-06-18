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
          <ClipboardCheck size={15} strokeWidth={1.75} style="color: var(--sempa-accent); flex-shrink: 0;" />
          <span class="intention-text">{intention}</span>
          <Check size={16} strokeWidth={2.5} class="check" />
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
  .intention {
    background: var(--card-bg);
    border: 1px solid var(--sempa-border);
    border-radius: 12px;
    transition: border-color 0.15s;
  }
  .intention:hover { border-color: var(--sempa-text-dim); }
  .intention-inner {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 11px 13px;
  }

  /* unset: compact chip + quote share the line */
  .set-chip {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    flex-shrink: 0;
    border: 1px solid var(--sempa-border);
    border-radius: 9999px;
    padding: 7px 14px;
    font-size: 13px;
    font-weight: 500;
    color: var(--sempa-text-soft);
    background: var(--sempa-bg-panel);
    transition: 0.12s;
    white-space: nowrap;
    text-decoration: none;
  }
  .set-chip:hover { border-color: var(--sempa-accent); color: var(--sempa-accent); }

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
  .intention-display .intention-text {
    font-size: 14px;
    font-weight: 500;
    color: var(--sempa-text);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .intention-display :global(.check) {
    margin-left: auto;
    color: var(--sempa-success);
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
