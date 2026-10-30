<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { formatDate, formatWeekRange, isToday } from '$lib/utils';
  import type { DailyPlan, WeekReview } from '$lib/types';
  import SempaPattern from '$lib/components/ui/SempaPattern.svelte';
  import { BookOpen, ClipboardCheck } from 'lucide-svelte';

  type DailyEntry = { kind: 'daily'; date: string; plan: DailyPlan };
  type WeekEntry  = { kind: 'week'; date: string; review: WeekReview };
  type Entry = DailyEntry | WeekEntry;

  let plans   = $state<DailyPlan[]>([]);
  let reviews = $state<WeekReview[]>([]);
  let loading = $state(true);

  // Merge daily plans and week reviews into one reverse-chronological timeline.
  const entries = $derived.by<Entry[]>(() => {
    const out: Entry[] = [];
    for (const p of plans) out.push({ kind: 'daily', date: p.plan_date, plan: p });
    for (const r of reviews) out.push({ kind: 'week', date: r.week_start, review: r });
    // Week reviews sort by their week-start; sort the whole list newest first,
    // and when a week review shares a date boundary with a day, show it first.
    out.sort((a, b) => {
      if (a.date === b.date) return a.kind === 'week' ? -1 : 1;
      return a.date < b.date ? 1 : -1;
    });
    return out;
  });

  function parseList(json: string | null): string[] {
    if (!json) return [];
    try {
      const v = JSON.parse(json);
      return Array.isArray(v) ? v.filter((s) => typeof s === 'string' && s.trim()) : [];
    } catch { return []; }
  }

  async function load() {
    [plans, reviews] = await Promise.all([
      api.plans.list().catch(() => []),
      api.weeks.listReviews().catch(() => []),
    ]);
    loading = false;
  }

  onMount(load);
</script>

<div class="relative mx-auto w-full max-w-2xl px-4 py-6 sm:px-6 sm:py-10">
  <!-- Header -->
  <div class="mb-6">
    <h1 class="type-page" style="color: var(--sempa-text);">Journal</h1>
    <p class="mt-1.5 text-sm" style="color: var(--sempa-text-soft);">
      Your intentions, reflections, and weekly reviews over time.
    </p>
  </div>

  {#if loading}
    <div class="flex flex-col gap-3">
      {#each Array(3) as _}
        <div class="h-28 animate-pulse" style="background: var(--sempa-bg-panel); border-radius: 22px; box-shadow: var(--sempa-shadow-1);"></div>
      {/each}
    </div>
  {:else if entries.length === 0}
    <!-- Empty state -->
    <div class="surface relative overflow-hidden py-20 text-center" style="border-radius: 24px;">
      <div class="pointer-events-none absolute inset-0 z-0">
        <SempaPattern motif="garden" class="h-full w-full" opacity={0.9} />
      </div>
      <div class="relative z-10 flex flex-col items-center gap-2 px-6">
        <p class="type-section" style="color: var(--sempa-text);">
          Nothing here yet.
        </p>
        <p class="max-w-sm text-sm" style="color: var(--sempa-text-soft);">
          Set an intention when you plan a day, or jot a reflection at shutdown —
          they'll collect here.
        </p>
      </div>
    </div>
  {:else}
    <div class="flex flex-col gap-3">
      {#each entries as entry (entry.kind + entry.date)}
        {#if entry.kind === 'daily'}
          {@const intention = entry.plan.intention?.trim()}
          {@const reflection = entry.plan.reflection?.trim()}
          <a href="/day/{entry.date}" class="j-card">
            <div class="mb-3 flex items-center gap-2">
              <span class="j-date">
                <BookOpen size={13} strokeWidth={2.25} />
                {formatDate(entry.date)}
              </span>
              {#if isToday(entry.date)}
                <span class="chip" style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">Today</span>
              {/if}
            </div>
            {#if intention}
              <div class="j-intention" class:mb-3={!!reflection}>
                <p class="type-label" style="color: var(--sempa-accent-ink);">Intention</p>
                <p class="mt-1 leading-relaxed" style="font-size: 14.5px; font-weight: 600; color: var(--sempa-text);">{intention}</p>
              </div>
            {/if}
            {#if reflection}
              <div class="px-1">
                <p class="type-label" style="color: var(--sempa-text-soft);">Reflection</p>
                <p class="mt-1 whitespace-pre-line text-sm leading-relaxed" style="color: var(--sempa-text);">{reflection}</p>
              </div>
            {/if}
          </a>
        {:else}
          {@const wins = parseList(entry.review.wins)}
          {@const challenges = parseList(entry.review.challenges)}
          {@const nextFocus = entry.review.next_focus?.trim()}
          <a href="/week/{entry.date}/review" class="j-card is-week">
            <div class="mb-3 flex items-center gap-2">
              <span class="j-date" style="background: var(--sempa-bg-panel); color: var(--sempa-accent-ink);">
                <ClipboardCheck size={13} strokeWidth={2.25} />
                Week review · {formatWeekRange(entry.date)}
              </span>
            </div>
            {#if wins.length}
              <div class="mb-3 px-1">
                <p class="type-label" style="color: var(--sempa-text-soft);">Wins</p>
                <ul class="mt-1 list-disc pl-5 text-sm leading-relaxed" style="color: var(--sempa-text);">
                  {#each wins as w}<li>{w}</li>{/each}
                </ul>
              </div>
            {/if}
            {#if challenges.length}
              <div class="mb-3 px-1">
                <p class="type-label" style="color: var(--sempa-text-soft);">Challenges</p>
                <ul class="mt-1 list-disc pl-5 text-sm leading-relaxed" style="color: var(--sempa-text);">
                  {#each challenges as c}<li>{c}</li>{/each}
                </ul>
              </div>
            {/if}
            {#if nextFocus}
              <div class="px-1">
                <p class="type-label" style="color: var(--sempa-text-soft);">Next focus</p>
                <p class="mt-1 whitespace-pre-line text-sm leading-relaxed" style="color: var(--sempa-text);">{nextFocus}</p>
              </div>
            {/if}
          </a>
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .j-card {
    display: block;
    padding: 16px;
    background: var(--sempa-bg-panel);
    border-radius: 22px;
    box-shadow: var(--sempa-shadow-1);
    transition: box-shadow 150ms ease, transform 80ms ease-out;
  }
  .j-card:hover { box-shadow: var(--sempa-shadow-2); }
  .j-card:active { transform: scale(0.995); }
  /* Week reviews read as a soft accent block rather than a white card. */
  .j-card.is-week { background: var(--sempa-accent-bg); box-shadow: none; }
  .j-card.is-week:hover { box-shadow: inset 0 0 0 1.5px color-mix(in srgb, var(--sempa-accent) 35%, transparent); }

  .j-date {
    display: inline-flex; align-items: center; gap: 6px;
    height: 28px; padding: 0 12px 0 10px; border-radius: 999px;
    background: var(--sempa-fill); color: var(--sempa-text-soft);
    font-size: 12.5px; font-weight: 600;
  }

  /* Soft accent-tinted intention block, nested in the card. */
  .j-intention { background: var(--sempa-accent-bg); border-radius: 16px; padding: 12px 14px; }
</style>
