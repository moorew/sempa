<script lang="ts">
  import { onMount } from 'svelte';
  import { timeInsights } from '$lib/stores/timeInsights.svelte';
  import { classifyActivity, bucketByKey } from '$lib/activityBuckets';
  import { formatMinutes } from '$lib/utils';
  import { Gauge, RefreshCw } from 'lucide-svelte';

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
    if (mult >= 1.15) return '#b07d18';     // runs over — warm
    if (mult <= 0.85) return 'var(--sempa-success)';
    return 'var(--sempa-text-soft)';
  }
  // 1× → 40% wide, scaling up so overruns read clearly; capped.
  const barWidth = (mult: number) => Math.max(8, Math.min(100, mult * 40));
</script>

<svelte:head><title>Insights — Sempa</title></svelte:head>

<div class="mx-auto flex h-full max-w-2xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <div class="flex items-center justify-between px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <div class="flex items-center gap-2.5">
      <Gauge size={20} style="color: var(--sempa-accent);" />
      <h1 class="text-base font-semibold" style="color: var(--sempa-text);">Time insights</h1>
    </div>
    <button onclick={() => timeInsights.refresh()} aria-label="Refresh"
      class="rounded-lg p-1.5 transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);">
      <RefreshCw size={16} />
    </button>
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-6 pb-24">
    {#if loading && !data}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>

    {:else if !data?.available}
      <!-- Learning state -->
      <div class="rounded-2xl px-5 py-8 text-center" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
        <div class="mb-3 text-4xl">🌱</div>
        <h2 class="mb-1.5 text-lg font-bold" style="color: var(--sempa-text);">Still learning your timing</h2>
        <p class="mx-auto max-w-sm text-sm leading-relaxed" style="color: var(--sempa-text-soft);">
          Sempa needs a few completed tasks that have both an estimate and a logged
          time before it can show your patterns. Keep using the focus timer and the
          “how long did that take?” prompts — you’ve logged
          <strong>{data?.samples ?? 0}</strong> so far.
        </p>
      </div>

    {:else}
      <!-- Headline -->
      {#if headline}
        <div class="mb-6 rounded-2xl px-5 py-6 text-center"
          style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
          <p class="text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">
            Your estimating
          </p>
          <p class="my-1 text-5xl font-bold tabular-nums"
            style="color: {headline.kind === 'over' ? '#b07d18' : headline.kind === 'under' ? 'var(--sempa-success)' : 'var(--sempa-accent)'};">
            {headline.big}
          </p>
          <p class="text-sm" style="color: var(--sempa-text-soft);">{headline.line}</p>
          <p class="mt-2 text-xs" style="color: var(--sempa-text-dim);">Based on {data.samples} completed task{data.samples === 1 ? '' : 's'}</p>
        </div>

        <!-- Planned vs actual totals -->
        <div class="mb-6 rounded-2xl px-5 py-5" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
          <p class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">
            Planned vs. actual (recent)
          </p>
          <div class="flex flex-col gap-2.5">
            <div>
              <div class="mb-1 flex justify-between text-xs" style="color: var(--sempa-text-soft);">
                <span>Planned</span><span class="tabular-nums">{formatMinutes(totals.est)}</span>
              </div>
              <div class="h-2.5 overflow-hidden rounded-full" style="background: var(--sempa-bg);">
                <div class="h-full rounded-full" style="width: {totals.estPct}%; background: var(--sempa-text-dim);"></div>
              </div>
            </div>
            <div>
              <div class="mb-1 flex justify-between text-xs" style="color: var(--sempa-text-soft);">
                <span>Actual</span><span class="tabular-nums">{formatMinutes(totals.act)}</span>
              </div>
              <div class="h-2.5 overflow-hidden rounded-full" style="background: var(--sempa-bg);">
                <div class="h-full rounded-full" style="width: {totals.actPct}%; background: var(--sempa-accent);"></div>
              </div>
            </div>
          </div>
        </div>
      {/if}

      <!-- By activity -->
      {#if activityRows.length}
        <div class="mb-6">
          <p class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">By activity</p>
          <div class="flex flex-col gap-3 rounded-2xl px-5 py-4" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
            {#each activityRows as row}
              <div class="flex items-center gap-3">
                <span class="w-6 text-center text-lg">{row.bucket.emoji}</span>
                <span class="w-28 shrink-0 truncate text-sm" style="color: var(--sempa-text);">{row.bucket.label}</span>
                <div class="h-2 flex-1 overflow-hidden rounded-full" style="background: var(--sempa-bg);">
                  <div class="h-full rounded-full" style="width: {barWidth(row.mult)}%; background: {color(row.mult)};"></div>
                </div>
                <span class="w-10 text-right text-sm font-semibold tabular-nums" style="color: {color(row.mult)};">{row.mult}×</span>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <!-- By tag -->
      {#if data.tags.length}
        <div class="mb-6">
          <p class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">By tag</p>
          <div class="flex flex-col gap-3 rounded-2xl px-5 py-4" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
            {#each data.tags as t}
              <div class="flex items-center gap-3">
                <span class="w-36 shrink-0 truncate text-sm" style="color: var(--sempa-text);">#{t.tag}</span>
                <div class="h-2 flex-1 overflow-hidden rounded-full" style="background: var(--sempa-bg);">
                  <div class="h-full rounded-full" style="width: {barWidth(t.multiplier)}%; background: {color(t.multiplier)};"></div>
                </div>
                <span class="w-10 text-right text-sm font-semibold tabular-nums" style="color: {color(t.multiplier)};">{t.multiplier}×</span>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Recent tasks -->
      {#if recent.length}
        <div class="mb-6">
          <p class="mb-3 text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Recent tasks</p>
          <div class="overflow-hidden rounded-2xl" style="border: 1px solid var(--sempa-border);">
            {#each recent.slice(0, 25) as r, i}
              {@const over = r.actual_minutes - r.estimate_minutes}
              <div class="flex items-center gap-3 px-4 py-2.5"
                style="background: var(--sempa-bg-panel); {i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}">
                <span class="min-w-0 flex-1 truncate text-sm" style="color: var(--sempa-text);" title={r.title}>{r.title}</span>
                <span class="shrink-0 text-xs tabular-nums" style="color: var(--sempa-text-dim);">
                  {formatMinutes(r.estimate_minutes)} → {formatMinutes(r.actual_minutes)}
                </span>
                {#if over !== 0}
                  <span class="w-12 shrink-0 text-right text-xs font-medium tabular-nums"
                    style="color: {over > 0 ? '#b07d18' : 'var(--sempa-success)'};">
                    {over > 0 ? '+' : ''}{over}m
                  </span>
                {:else}
                  <span class="w-12 shrink-0 text-right text-xs" style="color: var(--sempa-text-dim);">on point</span>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {/if}
  </div>
</div>
