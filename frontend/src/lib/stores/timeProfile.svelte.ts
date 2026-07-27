// Learned durations per activity bucket — the "I know how long this takes now"
// layer that stops the completion prompt asking forever.
//
// Before this, timeCapture asked after *every* completed task with no logged
// time. It had back-off heuristics but no memory: teaching it that email takes
// ~12 minutes twenty times over changed nothing about the twenty-first prompt.
//
// Bucketing happens here on the client rather than server-side because
// classifyActivity() (lib/activityBuckets.ts) is the same classifier the
// completion prompt uses. Re-implementing it in Go would leave two classifiers
// to drift apart, so the server just hands over raw logged durations.
import { api } from '$lib/api';
import { classifyActivity } from '$lib/activityBuckets';
import type { DurationSample } from '$lib/types';

/** Samples needed in a bucket before it can be considered learned. */
export const LEARN_MIN_SAMPLES = 5;

/**
 * How spread out a bucket's durations may be and still count as learned,
 * measured as interquartile range / median.
 *
 * Sample count alone isn't enough. "Deep work" might have fifty samples ranging
 * from 10 minutes to 4 hours — a median there is a number, not a prediction, and
 * auto-filling it would quietly poison the profile with fiction. Buckets that
 * genuinely vary keep asking, which is the honest outcome.
 */
export const LEARN_MAX_SPREAD = 0.5;

export interface BucketStat {
  key: string;
  samples: number;
  median: number;
  /** IQR / median. 0 = perfectly consistent. */
  spread: number;
  learned: boolean;
}

function quantile(sorted: number[], q: number): number {
  if (sorted.length === 0) return 0;
  const pos = (sorted.length - 1) * q;
  const lo = Math.floor(pos);
  const hi = Math.ceil(pos);
  if (lo === hi) return sorted[lo];
  return sorted[lo] + (sorted[hi] - sorted[lo]) * (pos - lo);
}

/** Compute per-bucket stats from raw samples. Exported for tests. */
export function summarise(samples: DurationSample[], forgotten: Set<string> = new Set()): Map<string, BucketStat> {
  const byBucket = new Map<string, number[]>();
  for (const s of samples) {
    if (!(s.minutes > 0)) continue;
    const key = classifyActivity(s.title, s.tags ?? []).key;
    const list = byBucket.get(key);
    if (list) list.push(s.minutes);
    else byBucket.set(key, [s.minutes]);
  }

  const out = new Map<string, BucketStat>();
  for (const [key, mins] of byBucket) {
    const sorted = [...mins].sort((a, b) => a - b);
    const median = quantile(sorted, 0.5);
    const iqr = quantile(sorted, 0.75) - quantile(sorted, 0.25);
    const spread = median > 0 ? iqr / median : Number.POSITIVE_INFINITY;
    out.set(key, {
      key,
      samples: sorted.length,
      median: Math.round(median),
      spread,
      learned:
        !forgotten.has(key) &&
        sorted.length >= LEARN_MIN_SAMPLES &&
        spread <= LEARN_MAX_SPREAD &&
        median > 0,
    });
  }
  return out;
}

// Buckets the user has explicitly asked to re-teach. Lives beside the other
// sempa.tt.* client-only preferences.
const FORGET_KEY = 'sempa.tt.forgottenBuckets';

function loadForgotten(): Set<string> {
  if (typeof localStorage === 'undefined') return new Set();
  try {
    const raw = localStorage.getItem(FORGET_KEY);
    return new Set<string>(raw ? JSON.parse(raw) : []);
  } catch {
    return new Set();
  }
}

function createTimeProfile() {
  let stats = $state<Map<string, BucketStat>>(new Map());
  let forgotten = $state<Set<string>>(new Set());
  let loaded = $state(false);
  let loading = false;
  // Raw samples are retained so forget/relearn can recompute instantly instead of
  // round-tripping to the server just to flip one boolean.
  let raw: DurationSample[] = [];

  async function load() {
    if (loaded || loading) return;
    loading = true;
    forgotten = loadForgotten();
    try {
      raw = await api.insights.durations();
      stats = summarise(raw, forgotten);
      loaded = true;
    } catch {
      // Offline or no server configured — leave the profile empty, which means
      // "nothing learned", so the prompt keeps its existing behaviour.
    } finally {
      loading = false;
    }
  }

  function persistForgotten(next: Set<string>) {
    forgotten = next;
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(FORGET_KEY, JSON.stringify([...next]));
    }
    stats = summarise(raw, next);
  }

  async function refresh() {
    loaded = false;
    await load();
  }

  // Every logged minute is a teaching signal, but the profile only shifts
  // meaningfully over many of them — and completions arrive in bursts. Coalesce
  // so clearing ten tasks costs one refetch, not ten.
  let refreshTimer: ReturnType<typeof setTimeout> | null = null;
  function scheduleRefresh() {
    if (refreshTimer) clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => { refreshTimer = null; void refresh(); }, 5000);
  }

  return {
    get loaded() { return loaded; },
    /** Every bucket with at least one sample, learned or not. */
    get all() { return [...stats.values()].sort((a, b) => b.samples - a.samples); },
    get learned() { return this.all.filter((b) => b.learned); },

    stat(key: string): BucketStat | null { return stats.get(key) ?? null; },

    /** Minutes to auto-log for this bucket, or null while it's still learning. */
    learnedMinutes(key: string): number | null {
      const s = stats.get(key);
      return s?.learned ? s.median : null;
    },

    /** Re-enter the learning period for a bucket (Settings → Time tracking). */
    forget(key: string) {
      const next = new Set(forgotten);
      next.add(key);
      persistForgotten(next);
    },

    /** Undo a forget — the bucket becomes learned again if it still qualifies. */
    relearn(key: string) {
      const next = new Set(forgotten);
      next.delete(key);
      persistForgotten(next);
    },

    isForgotten(key: string) { return forgotten.has(key); },

    load,
    refresh,
    scheduleRefresh,
  };
}

export const timeProfile = createTimeProfile();
