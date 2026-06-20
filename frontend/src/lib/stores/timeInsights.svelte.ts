/**
 * Planned-vs-actual time profile, loaded once and shared.
 *
 * The headline multipliers come from plain server-side statistics over the
 * user's completed tasks (estimate vs logged actual). It's a server-only
 * endpoint, so on a pure-offline client with no server configured the fetch
 * simply fails and the profile stays unavailable — callers tolerate that.
 */
import { api } from '$lib/api';
import type { TimeInsights } from '$lib/types';

class TimeInsightsStore {
  data = $state<TimeInsights | null>(null);
  #loaded = false;
  #loading = false;

  /** Load once (idempotent); safe to call from any component that needs it. */
  async ensure() {
    if (this.#loaded || this.#loading) return;
    this.#loading = true;
    try {
      this.data = await api.insights.time();
      this.#loaded = true;
    } catch {
      /* offline / no server — leave unavailable */
    } finally {
      this.#loading = false;
    }
  }

  /** Re-fetch after new time is logged so the profile reflects it. */
  async refresh() {
    try {
      this.data = await api.insights.time();
      this.#loaded = true;
    } catch { /* ignore */ }
  }

  /**
   * Multiplier to apply for a task with these tags: the best-evidenced matching
   * tag wins, otherwise the global multiplier. null when there isn't enough data.
   */
  multiplierFor(tags: string[]): { mult: number; tag: string | null } | null {
    const d = this.data;
    if (!d?.available) return null;
    let best: { tag: string; samples: number; multiplier: number } | null = null;
    for (const t of d.tags) {
      if (tags.includes(t.tag) && (!best || t.samples > best.samples)) best = t;
    }
    if (best) return { mult: best.multiplier, tag: best.tag };
    if (d.global_multiplier) return { mult: d.global_multiplier, tag: null };
    return null;
  }
}

export const timeInsights = new TimeInsightsStore();
