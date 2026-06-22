/**
 * Bridge to push task data + the active theme's colours to the Android Glance/native
 * widgets via the WidgetBridge Capacitor plugin. Falls back silently on web/iOS where
 * the plugin is not available.
 */

import type { Task } from './types';
import { today, offsetDate, weekStart } from './utils';

interface WidgetThemeColors {
  primary?: string;
  accent_bg?: string;
  surface?: string;
  on_surface?: string;
  on_surface_dim?: string;
  outline?: string;
  green?: string;
}

interface WidgetBridgePlugin {
  // All fields optional so a theme-only push (no task list) leaves task data intact.
  updateWidgetData(opts: {
    todayTotal?: number;
    todayDone?: number;
    tasks?: { id: string; title: string; done: boolean }[];
    week?: { date: string; count: number }[];
    theme?: WidgetThemeColors;
  }): Promise<void>;
  setAppIcon?(opts: { theme: string }): Promise<void>;
}

function getPlugin(): WidgetBridgePlugin | null {
  try {
    // Capacitor registers plugins on window.Capacitor.Plugins
    const cap = (window as any).Capacitor;
    if (cap?.Plugins?.WidgetBridge) {
      return cap.Plugins.WidgetBridge as WidgetBridgePlugin;
    }
  } catch {}
  return null;
}

/** Normalise a CSS colour value to #rrggbb (drops alpha); '' if not resolvable. */
function toHex(v: string): string {
  if (!v) return '';
  if (v.startsWith('#')) {
    if (v.length === 4) return '#' + v.slice(1).split('').map((c) => c + c).join('');
    return v.slice(0, 7);
  }
  const m = v.match(/rgba?\(([^)]+)\)/i);
  if (m) {
    const [r, g, b] = m[1].split(',').map((s) => parseFloat(s.trim()));
    const h = (n: number) => Math.max(0, Math.min(255, Math.round(n || 0))).toString(16).padStart(2, '0');
    return `#${h(r)}${h(g)}${h(b)}`;
  }
  return ''; // unknown keyword (e.g. AccentColor) → let the widget keep its fallback
}

/** Read the active theme's resolved colours from the live CSS custom properties. */
function readThemeColors(): WidgetThemeColors | undefined {
  if (typeof document === 'undefined' || typeof getComputedStyle === 'undefined') return undefined;
  try {
    const cs = getComputedStyle(document.documentElement);
    const c = (v: string) => toHex(cs.getPropertyValue(v).trim());
    const t: WidgetThemeColors = {
      primary: c('--sempa-accent'),
      accent_bg: c('--sempa-accent-bg'),
      surface: c('--sempa-bg-panel'),
      on_surface: c('--sempa-text'),
      on_surface_dim: c('--sempa-text-dim'),
      outline: c('--sempa-border'),
      green: c('--sempa-success'),
    };
    const out: WidgetThemeColors = {};
    for (const [k, v] of Object.entries(t)) if (v) (out as Record<string, string>)[k] = v;
    return Object.keys(out).length ? out : undefined;
  } catch {
    return undefined;
  }
}

/**
 * Sync current task data (and the active theme colours) to the Android widgets.
 * Call after task list changes (create, complete, delete, reorder).
 */
export function syncWidgetData(todayTasks: Task[], weekTaskCounts?: Map<string, number>) {
  const plugin = getPlugin();
  if (!plugin) return;

  const todayDate = today();
  const total = todayTasks.length;
  const done = todayTasks.filter((t) => t.status === 'done').length;
  const tasks = todayTasks.slice(0, 10).map((t) => ({
    id: t.id,
    title: t.title,
    done: t.status === 'done',
  }));

  const ws = weekStart(todayDate);
  const week: { date: string; count: number }[] = [];
  for (let i = 0; i < 7; i++) {
    const d = offsetDate(ws, i);
    week.push({ date: d, count: weekTaskCounts?.get(d) ?? 0 });
  }

  plugin.updateWidgetData({ todayTotal: total, todayDone: done, tasks, week, theme: readThemeColors() }).catch(() => {});
}

/** Push only the active theme's colours (called when the theme/appearance changes). */
export function syncWidgetTheme() {
  const plugin = getPlugin();
  if (!plugin) return;
  const theme = readThemeColors();
  if (!theme) return;
  plugin.updateWidgetData({ theme }).catch(() => {});
}

/** Whether the native launcher-icon switch is available (Android with the plugin). */
export function canThemeAppIcon(): boolean {
  return !!getPlugin()?.setAppIcon;
}

/** Switch the Android launcher icon to the variant for `theme` (terracotta, forest,
 *  plum, slate, oled, ocean). User-initiated only — the launcher briefly refreshes. */
export function setAppIcon(theme: string) {
  getPlugin()?.setAppIcon?.({ theme }).catch(() => {});
}
