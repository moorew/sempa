// Day-capacity math: how much planned work a given day is carrying versus the
// humane daily limit the user set. Powers the subtle "this is too much for that
// day" signal in the task editor and the quiet total on the day view.
import type { Task } from '$lib/types';
import { timeTracking } from '$lib/stores/timeTracking.svelte';

/**
 * Sum of planned estimate minutes for a day's tasks. Cancelled tasks and
 * subtasks are excluded — subtask time rolls up under its parent's estimate, so
 * counting both would double-count. Pass excludeId to omit the task being edited.
 */
export function plannedMinutes(tasks: Task[], excludeId?: string): number {
  let total = 0;
  for (const t of tasks) {
    if (t.id === excludeId) continue;
    if (t.status === 'cancelled') continue;
    if (t.parent_task_id) continue;
    total += t.time_estimate_minutes ?? 0;
  }
  return total;
}

export interface CapacityState {
  total: number;
  capacity: number;
  over: boolean;
  overBy: number;
}

/**
 * Capacity snapshot for a day. `extraMinutes` lets a caller fold in an estimate
 * that isn't part of `tasks` yet (e.g. a value being typed in the editor).
 */
export function capacityState(tasks: Task[], extraMinutes = 0, excludeId?: string): CapacityState {
  const total = plannedMinutes(tasks, excludeId) + Math.max(0, extraMinutes);
  const capacity = timeTracking.capacityMinutes;
  return { total, capacity, over: total > capacity, overBy: total - capacity };
}
