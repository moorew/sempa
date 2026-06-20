// Day-capacity math: how much planned work a given day is carrying versus the
// humane daily limit the user set. Powers the subtle "this is too much for that
// day" signal in the task editor and the quiet total on the day view.
//
// "Realistic" mode multiplies each estimate by the user's history multiplier
// (per-tag, else global) — so a day of optimistic 20-min guesses is judged by
// what those tasks actually tend to take. With no logged history the multiplier
// is 1×, so realistic mode is a safe default.
import type { Task } from '$lib/types';
import { timeTracking } from '$lib/stores/timeTracking.svelte';
import { timeInsights } from '$lib/stores/timeInsights.svelte';

/** Estimate adjusted by the history multiplier when realistic mode is on. */
export function effortMinutes(estimate: number, tags: string[], realistic: boolean): number {
  if (!realistic || estimate <= 0) return estimate;
  const m = timeInsights.multiplierFor(tags ?? []);
  return m ? Math.round(estimate * m.mult) : estimate;
}

/**
 * Sum of planned minutes for a day's tasks. Cancelled tasks and subtasks are
 * excluded — subtask time rolls up under its parent's estimate. `realistic`
 * applies the history multiplier per task. Pass excludeId to omit a task.
 */
export function plannedMinutes(tasks: Task[], excludeId?: string, realistic = false): number {
  let total = 0;
  for (const t of tasks) {
    if (t.id === excludeId) continue;
    if (t.status === 'cancelled') continue;
    if (t.parent_task_id) continue;
    total += effortMinutes(t.time_estimate_minutes ?? 0, t.tags ?? [], realistic);
  }
  return total;
}

export interface CapacityState {
  total: number;     // effective minutes (calibrated when realistic)
  rawTotal: number;  // raw estimate sum (for "X planned (~Y real)" wording)
  capacity: number;
  over: boolean;
  overBy: number;
  realistic: boolean;
}

/**
 * Capacity snapshot for a day. `extraMinutes`/`extraTags` fold in an estimate
 * not yet part of `tasks` (e.g. a value being typed in the editor). Reads the
 * realistic-mode toggle from settings.
 */
export function capacityState(
  tasks: Task[],
  extraMinutes = 0,
  excludeId?: string,
  extraTags: string[] = [],
): CapacityState {
  const realistic = timeTracking.capacityRealistic;
  const base = plannedMinutes(tasks, excludeId, realistic);
  const rawBase = plannedMinutes(tasks, excludeId, false);
  const extra = Math.max(0, extraMinutes);
  const total = base + effortMinutes(extra, extraTags, realistic);
  const rawTotal = rawBase + extra;
  const capacity = timeTracking.capacityMinutes;
  return { total, rawTotal, capacity, over: total > capacity, overBy: total - capacity, realistic };
}
