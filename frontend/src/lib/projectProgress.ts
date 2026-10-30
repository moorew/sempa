import type { Task } from '$lib/types';

// Progress summary for a project, computed client-side from its top-level tasks
// (api.projects.tasks). Pure + synchronous so list cards and the detail page
// agree, and it works identically on local-first clients.
export interface ProjectProgress {
  total: number;
  done: number;
  pct: number;              // 0–100, rounded
  estimateMinutes: number;  // sum of estimates on all tasks
  actualMinutes: number;    // sum of tracked time on all tasks
  overdue: boolean;         // past the target date with work left
}

export function projectProgress(tasks: Task[], targetDate?: string | null, todayStr?: string): ProjectProgress {
  const counted = tasks.filter((t) => t.status !== 'cancelled');
  const done = counted.filter((t) => t.status === 'done').length;
  const total = counted.length;
  let estimateMinutes = 0;
  let actualMinutes = 0;
  for (const t of counted) {
    estimateMinutes += t.time_estimate_minutes ?? 0;
    actualMinutes += t.time_actual_minutes ?? 0;
  }
  return {
    total,
    done,
    pct: total === 0 ? 0 : Math.round((done / total) * 100),
    estimateMinutes,
    actualMinutes,
    overdue: !!targetDate && !!todayStr && targetDate < todayStr && done < total,
  };
}

// "in 3 weeks" / "2 days ago"-style label for a target date.
export function relativeDue(target: string, todayStr: string): string {
  const days = Math.round(
    (new Date(target + 'T12:00:00').getTime() - new Date(todayStr + 'T12:00:00').getTime()) / 86_400_000,
  );
  if (days === 0) return 'due today';
  const abs = Math.abs(days);
  const unit = abs >= 60 ? `${Math.round(abs / 30)} months` : abs >= 14 ? `${Math.round(abs / 7)} weeks` : `${abs} day${abs === 1 ? '' : 's'}`;
  return days > 0 ? `due in ${unit}` : `${unit} overdue`;
}
