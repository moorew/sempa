<script lang="ts">
  import { onMount } from 'svelte';
  import { timeInsights } from '$lib/stores/timeInsights.svelte';
  import { classifyActivity, bucketByKey } from '$lib/activityBuckets';
  import { formatMinutes } from '$lib/utils';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { Gauge, RefreshCw, Sprout, ListChecks, Clock, Users, Mail, Phone, Eye, PenLine, Brain,
           FolderOpen, ShoppingBasket, Leaf, CalendarDays, Circle } from 'lucide-svelte';

  // Activity bucket → icon (the bucket data carries an emoji; the refresh keeps
  // UI chrome emoji-free, so map keys to line icons here, display-only).
  const BUCKET_ICON: Record<string, typeof Circle> = {
    meeting: Users, email: Mail, call: Phone, review: Eye, writing: PenLine,
    deepwork: Brain, admin: FolderOpen, errand: ShoppingBasket, personal: Leaf,
    planning: CalendarDays, other: Circle,
  };

  let loading = $state(true);
  onMount(async () => { await timeInsights.refresh(); loading = false; });

  const data = $derived(timeInsights.data);
  const recent = $derived(data?.recent ?? []);

  function median(xs: number[]): number {
    if (!xs.length) return 0;
    const s = [...xs].sort((a, b) => a - b);
    const n = s.length;
    return n % 2 ? s[(n - 1) / 2] : (s[n / 2 - 1] + s[n / 2]) / 2;
  }
  const round1 = (x: number) => Math.round(x * 10) / 10;

  // Headline calibration, in plain language.
  const headline = $derived.by(() => {
    if (!data?.available) return null;
    const m = data.global_multiplier;
    if (m >= 1.15) return { kind: 'over', big: `${m}×`, line: 'longer than you plan, on average' };
    if (m <= 0.85) return { kind: 'under', big: `${m}×`, line: 'of your estimate — you finish early' };
    return { kind: 'ok', big: 'On point', line: 'your estimates closely match reality' };
  });

  // Planned vs actual totals across the recent window (a concrete gut-check).
  const totals = $derived.by(() => {
    let est = 0, act = 0;
    for (const r of recent) { est += r.estimate_minutes; act += r.actual_minutes; }
    const max = Math.max(est, act, 1);
    return { est, act, estPct: (est / max) * 100, actPct: (act / max) * 100 };
  });

  // Per-activity multipliers, classified client-side from recent tasks.
  const activityRows = $derived.by(() => {
    const groups = new Map<string, number[]>();
    for (const r of recent) {
      if (r.estimate_minutes <= 0 || r.actual_minutes <= 0) continue;
      const key = classifyActivity(r.title, r.tags).key;
      const arr = groups.get(key) ?? [];
      arr.push(r.actual_minutes / r.estimate_minutes);
      groups.set(key, arr);
    }
    const rows = [...groups.entries()]
      .filter(([, rs]) => rs.length >= 2)
      .map(([key, rs]) => ({ bucket: bucketByKey(key), mult: round1(median(rs)), samples: rs.length }));
    rows.sort((a, b) => b.samples - a.samples);
    return rows;
  });

  function color(mult: number): string {
    if (mult >= 1.15) return 'var(--cal-amber-fg)';     // runs over — warm
    if (mult <= 0.85) return 'var(--sempa-success)';
    return 'var(--sempa-text-soft)';
  }
  // Soft bar fill for the same three states (a tint, not a hard line).
  function barFill(mult: number): string {
    if (mult >= 1.15) return 'color-mix(in srgb, var(--cal-amber-fg) 55%, var(--cal-amber-bg))';
    if (mult <= 0.85) return 'color-mix(in srgb, var(--sempa-success) 65%, var(--sempa-bg-panel))';
    return 'color-mix(in srgb, var(--sempa-accent) 55%, var(--sempa-bg-panel))';
  }
  // 1× → 40% wide, scaling up so overruns read clearly; capped.
  const barWidth = (mult: number) => Math.max(8, Math.min(100, mult * 40));
</script>

<svelte:head><title>Insights — Sempa</title></svelte:head>

<div class="mx-auto flex h-full max-w-2xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <div class="flex items-start justify-between gap-3 px-4 pb-2 pt-6 sm:px-6 sm:pt-10">
    <div class="min-w-0">
      <h1 class="type-page" style="color: var(--sempa-text);">Time insights</h1>
      <p class="mt-1.5 text-sm" style="color: var(--sempa-text-soft);">How your estimates compare with the time things really take.</p>
    </div>
    <button onclick={() => timeInsights.refresh()} aria-label="Refresh" class="btn-icon shrink-0">
      <RefreshCw size={16} />
    </button>
  </div>

  <div class="flex-1 overflow-y-auto px-4 pb-24 pt-4 sm:px-6">
    {#if loading && !data}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>

    {:else if !data?.available}
      <!-- Learning state -->
      <div class="surface px-5 py-9 text-center" style="border-radius: 24px;">
        <span class="stat-icon mx-auto" style="background: var(--sempa-success-bg); color: var(--sempa-success);">
          <Sprout size={22} strokeWidth={2} />
        </span>
        <h2 class="mb-1.5 type-section" style="color: var(--sempa-text);">Still learning your timing</h2>
        <p class="mx-auto max-w-sm text-sm leading-relaxed" style="color: var(--sempa-text-soft);">
          Sempa needs a few completed tasks that have both an estimate and a logged
          time before it can show your patterns. Keep using the focus timer and the
          “how long did that take?” prompts — you’ve logged
          <strong style="color: var(--sempa-text);">{data?.samples ?? 0}</strong> so far.
        </p>
      </div>

    {:else}
      <!-- Headline: stat tiles -->
      {#if headline}
        <div class="mb-5 grid gap-2.5 sm:gap-3 {recent.length ? 'grid-cols-2 sm:grid-cols-3' : 'grid-cols-2'}">
          <div class="stat-tile">
            <span class="stat-icon"
                  style={headline.kind === 'over'
                    ? 'background: var(--cal-amber-bg); color: var(--cal-amber-fg);'
                    : headline.kind === 'under'
                      ? 'background: var(--sempa-success-bg); color: var(--sempa-success);'
                      : 'background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);'}>
              <Gauge size={20} strokeWidth={2} />
            </span>
            <p class="stat-num tabular-nums">{headline.big}</p>
            <p class="stat-label">{headline.line}</p>
          </div>
          <div class="stat-tile">
            <span class="stat-icon" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
              <ListChecks size={20} strokeWidth={2} />
            </span>
            <p class="stat-num tabular-nums">{data.samples}</p>
            <p class="stat-label">completed task{data.samples === 1 ? '' : 's'} measured</p>
          </div>
          {#if recent.length}
            <div class="stat-tile col-span-2 sm:col-span-1">
              <span class="stat-icon" style="background: var(--cal-sage-bg); color: var(--cal-sage-fg);">
                <Clock size={20} strokeWidth={2} />
              </span>
              <p class="stat-num tabular-nums">{formatMinutes(totals.act) || '0m'}</p>
              <p class="stat-label">logged on recent tasks</p>
            </div>
          {/if}
        </div>

        <!-- Planned vs actual totals (only once there are recent samples to sum) -->
        {#if recent.length}
        <div class="surface mb-5 px-5 py-5" style="border-radius: 22px;">
          <h2 class="mb-4 section-title">Planned vs. actual <span style="color: var(--sempa-text-dim); font-weight: 600;">(recent)</span></h2>
          <div class="flex flex-col gap-3.5">
            <div>
              <div class="mb-1.5 flex justify-between type-meta" style="color: var(--sempa-text-soft);">
                <span>Planned</span><span class="font-mono tabular-nums" style="font-size: 12px;">{formatMinutes(totals.est) || '0m'}</span>
              </div>
              <div class="soft-track">
                <div class="soft-fill" style="width: {totals.estPct}%; background: color-mix(in srgb, var(--sempa-text-soft) 45%, var(--sempa-bg-panel));"></div>
              </div>
            </div>
            <div>
              <div class="mb-1.5 flex justify-between type-meta" style="color: var(--sempa-text-soft);">
                <span>Actual</span><span class="font-mono tabular-nums" style="font-size: 12px;">{formatMinutes(totals.act) || '0m'}</span>
              </div>
              <div class="soft-track">
                <div class="soft-fill" style="width: {totals.actPct}%; background: var(--sempa-accent);"></div>
              </div>
            </div>
          </div>
        </div>
        {/if}
      {/if}

      <!-- By activity -->
      {#if activityRows.length}
        <div class="mb-5">
          <h2 class="mb-2.5 px-1 type-label" style="color: var(--sempa-text-soft);">By activity</h2>
          <div class="surface flex flex-col gap-3 px-4 py-4 sm:px-5" style="border-radius: 22px;">
            {#each activityRows as row}
              {@const Icon = BUCKET_ICON[row.bucket.key] ?? Circle}
              <div class="flex items-center gap-3">
                <span class="row-icon"><Icon size={15} strokeWidth={2} /></span>
                <span class="w-24 shrink-0 truncate text-sm font-medium sm:w-32" style="color: var(--sempa-text);">{row.bucket.label}</span>
                <div class="soft-track flex-1">
                  <div class="soft-fill" style="width: {barWidth(row.mult)}%; background: {barFill(row.mult)};"></div>
                </div>
                <span class="w-11 text-right tabular-nums" style="font-size: 14px; font-weight: 800; color: {color(row.mult)};">{row.mult}×</span>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <!-- By tag -->
      {#if data.tags.length}
        <div class="mb-5">
          <h2 class="mb-2.5 px-1 type-label" style="color: var(--sempa-text-soft);">By tag</h2>
          <div class="surface flex flex-col gap-3 px-4 py-4 sm:px-5" style="border-radius: 22px;">
            {#each data.tags as t}
              <div class="flex items-center gap-3">
                <span class="w-28 shrink-0 sm:w-36">
                  <span class="chip tint max-w-full" style="--tint: {tagStore.colorFor(t.tag)}; height: 26px; padding: 0 11px; font-size: 12.5px;">
                    <span class="truncate">{t.tag}</span>
                  </span>
                </span>
                <div class="soft-track flex-1">
                  <div class="soft-fill" style="width: {barWidth(t.multiplier)}%; background: {barFill(t.multiplier)};"></div>
                </div>
                <span class="w-11 text-right tabular-nums" style="font-size: 14px; font-weight: 800; color: {color(t.multiplier)};">{t.multiplier}×</span>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Recent tasks -->
      {#if recent.length}
        <div class="mb-5">
          <h2 class="mb-2.5 px-1 type-label" style="color: var(--sempa-text-soft);">Recent tasks</h2>
          <div class="surface overflow-hidden" style="border-radius: 22px;">
            {#each recent.slice(0, 25) as r, i}
              {@const over = r.actual_minutes - r.estimate_minutes}
              <div class="flex items-center gap-3 px-4 py-3"
                style={i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}>
                <span class="min-w-0 flex-1 truncate text-sm" style="color: var(--sempa-text);" title={r.title}>{r.title}</span>
                <span class="shrink-0 font-mono tabular-nums" style="font-size: 11.5px; color: var(--sempa-text-dim);">
                  {formatMinutes(r.estimate_minutes)} → {formatMinutes(r.actual_minutes)}
                </span>
                {#if over !== 0}
                  <span class="chip shrink-0 justify-center tabular-nums"
                    style="min-width: 52px; {over > 0
                      ? 'background: var(--cal-amber-bg); color: var(--cal-amber-fg);'
                      : 'background: var(--sempa-success-bg); color: var(--sempa-success);'}">
                    {over > 0 ? '+' : ''}{over}m
                  </span>
                {:else}
                  <span class="chip shrink-0 justify-center" style="min-width: 52px; background: var(--sempa-fill); color: var(--sempa-text-soft);">on point</span>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>
  /* Stat tile: surface, radius 22, tinted 46px icon tile, big number, soft label. */
  .stat-tile {
    background: var(--sempa-bg-panel);
    border-radius: var(--radius-tile);
    box-shadow: var(--sempa-shadow-1);
    padding: 16px;
    min-width: 0;
  }
  .stat-icon {
    display: grid; place-items: center;
    width: 46px; height: 46px; border-radius: 14px;
    margin-bottom: 12px;
  }
  .stat-num   { font-size: 20px; font-weight: 800; letter-spacing: -.02em; line-height: 1.15; color: var(--sempa-text); }
  .stat-label { margin-top: 3px; font-size: 12.5px; font-weight: 500; line-height: 1.35; color: var(--sempa-text-soft); }
  @media (max-width: 420px) {
    .stat-tile { padding: 12px; }
    .stat-icon { width: 40px; height: 40px; border-radius: 12px; margin-bottom: 10px; }
  }

  .section-title { font-size: 15px; font-weight: 800; letter-spacing: -.01em; color: var(--sempa-text); }

  /* Soft tinted bars: a rounded track with a soft fill. */
  .soft-track { height: 10px; border-radius: 999px; background: var(--sempa-track); overflow: hidden; }
  .soft-fill  { height: 100%; border-radius: 999px; transition: width 400ms ease; }

  .row-icon {
    display: inline-grid; place-items: center; flex: 0 0 auto;
    width: 30px; height: 30px; border-radius: 10px;
    background: var(--sempa-fill); color: var(--sempa-text-soft);
  }
</style>
