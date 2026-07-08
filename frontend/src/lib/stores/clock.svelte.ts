/**
 * Reactive "today" store — the single source of truth for the current calendar
 * day across the whole UI.
 *
 * Why this exists: `utils.today()` reads `new Date()` once. A `const todayDate =
 * today()` (or a `$derived(today())`, which has no reactive dependency to
 * invalidate it) is captured at mount and then FROZEN. Any surface left open
 * across midnight keeps showing yesterday — the day board highlights the wrong
 * column, the greeting names the wrong weekday, and the always-on Pi Dock
 * cockpit shows yesterday's tasks until it's restarted. That's the
 * "it thinks it's Tuesday when it's Wednesday" bug.
 *
 * `clock.today` is a real `$state` string (YYYY-MM-DD, device-local, same value
 * `utils.today()` returns) that ticks over at the day boundary, so every
 * `$derived`/`$effect` reading it re-runs exactly when the date changes. It only
 * reassigns when the date actually changes, so downstream work fires once per
 * rollover, not every poll.
 */
import { today as computeToday } from '$lib/utils';

class ClockStore {
  /** Current device-local date as YYYY-MM-DD. Reactive; rolls over at midnight. */
  today = $state(computeToday());

  private timer: ReturnType<typeof setInterval> | null = null;

  init() {
    if (typeof window === 'undefined') return;
    // A cheap 30s heartbeat is the primary mechanism for always-on surfaces
    // (the Pi Dock never regains focus, so it has no other rollover signal).
    // Up to 30s of lag at the boundary is imperceptible for a date change.
    this.timer ??= setInterval(() => this.refresh(), 30_000);
    // Waking a backgrounded tab / desktop window / phone: correct immediately
    // instead of waiting for the next heartbeat.
    document.addEventListener('visibilitychange', () => { if (!document.hidden) this.refresh(); });
    window.addEventListener('focus', () => this.refresh());
  }

  /** Recompute; only reassign (and thus invalidate readers) on an actual change. */
  refresh() {
    const d = computeToday();
    if (d !== this.today) this.today = d;
  }
}

export const clock = new ClockStore();
