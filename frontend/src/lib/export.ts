/**
 * Export helpers for task lists (e.g. the result of a tag filter).
 *
 * Two formats are offered to the user:
 *  - CSV      → tasksToCSV()      for spreadsheets / archiving
 *  - Markdown → tasksToMarkdown() for a clean, readable bulleted list
 *
 * downloadFile() turns a string into a browser download. On the desktop (Tauri)
 * and Android (Capacitor) shells an anchor download still works because the
 * webview honours blob: URLs, so a single code path covers every platform.
 */
import type { Task } from '$lib/types';

/** Trigger a client-side download of `content` as `filename`. */
export function downloadFile(filename: string, content: string, mime: string): void {
  if (typeof document === 'undefined') return;
  const blob = new Blob([content], { type: `${mime};charset=utf-8` });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Revoke on the next tick so the click has a chance to start the download.
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

/** Quote a CSV field, doubling embedded quotes per RFC 4180. */
function csvCell(value: string | number | null | undefined): string {
  const s = value == null ? '' : String(value);
  return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/** A CSV with a header row and one row per task. */
export function tasksToCSV(tasks: Task[]): string {
  const header = ['Title', 'Status', 'Planned date', 'Scheduled start', 'Estimate (min)', 'Tags'];
  const rows = tasks.map((t) => [
    csvCell(t.title),
    csvCell(t.status),
    csvCell(t.planned_date),
    csvCell(t.scheduled_start),
    csvCell(t.time_estimate_minutes),
    csvCell((t.tags ?? []).join(', ')),
  ].join(','));
  return [header.map(csvCell).join(','), ...rows].join('\r\n');
}

/** A clean Markdown document: a single H1 title then a checkbox bullet list. */
export function tasksToMarkdown(tasks: Task[], title: string): string {
  const lines = [`# ${title}`, ''];
  for (const t of tasks) {
    const box = t.status === 'done' ? '[x]' : '[ ]';
    const date = t.planned_date ? ` (${t.planned_date})` : '';
    lines.push(`- ${box} ${t.title}${date}`);
  }
  if (tasks.length === 0) lines.push('- _No tasks_');
  return lines.join('\n');
}

/** Filesystem-safe slug for building export filenames from a tag/label. */
export function slugify(s: string): string {
  return (s || 'tasks').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'tasks';
}
