/**
 * Bridge to the native Android focus-timer notification (FocusTimer plugin).
 *
 * The web Pomodoro store is the source of truth; this mirrors its state into an
 * ongoing notification whose countdown ticks natively (survives app close) and
 * routes the notification's Pause/Done/Resume taps back to the store — live when
 * the app is alive, or drained from a prefs stash on next launch.
 *
 * Entirely a no-op off Capacitor/Android (or if the installed app predates the
 * native plugin), so every other platform is unaffected.
 */
import { isCapacitor } from '$lib/platform';

interface FocusTimerPlugin {
  show(opts: { title: string; phase: string; running: boolean; endTime: number; remaining: number }): Promise<void>;
  stop(): Promise<void>;
  consumePendingAction(): Promise<{ action?: string }>;
  consumePendingStart?(): Promise<{ taskId?: string }>;
  addListener?(event: string, cb: (data: { action?: string; taskId?: string }) => void): unknown;
}

function plugin(): FocusTimerPlugin | null {
  if (typeof window === 'undefined' || !isCapacitor()) return null;
  return ((window as Window & { Capacitor?: { Plugins?: Record<string, unknown> } })
    .Capacitor?.Plugins?.FocusTimer as FocusTimerPlugin) ?? null;
}

export interface FocusNotifState {
  title: string;
  phase: string;
  running: boolean;
  endMs: number;
  remainingMs: number;
}

/** Mirror the current timer state into the notification, or clear it (null). */
export function syncFocusNotification(state: FocusNotifState | null): void {
  const p = plugin();
  if (!p) return;
  try {
    if (state) {
      void p.show({
        title: state.title, phase: state.phase, running: state.running,
        endTime: state.endMs, remaining: state.remainingMs,
      });
    } else {
      void p.stop();
    }
  } catch { /* older app without the plugin — ignore */ }
}

let inited = false;
/** Wire notification-button taps to the timer. Call once at app start. */
export function initFocusNotification(): void {
  const p = plugin();
  if (!p || inited) return;
  inited = true;
  try { p.addListener?.('focusAction', (e) => void route(e?.action)); } catch { /* ignore */ }
  // Widget "start this task" taps, delivered live when the app is already running.
  try { p.addListener?.('focusStart', (e) => void startFocusTask(e?.taskId)); } catch { /* ignore */ }
  void drainPending();
  // Re-check on every foreground, for taps made while the app was closed.
  try {
    const App = (window as Window & { Capacitor?: { Plugins?: Record<string, { addListener?: (e: string, cb: (s: { isActive?: boolean }) => void) => void } > } })
      .Capacitor?.Plugins?.App;
    App?.addListener?.('appStateChange', (s) => { if (s?.isActive) void drainPending(); });
  } catch { /* ignore */ }
}

async function drainPending(): Promise<void> {
  const p = plugin();
  if (!p) return;
  // A widget "start this task" tap WINS and is handled first: starting fresh, we
  // must ignore any stale notification action (e.g. a leftover "done" that was
  // never drained) — otherwise it would immediately end the just-started session
  // and pop the "how long did you work?" sheet. Always consume the action stash to
  // clear it, but only route it when we didn't just start a task.
  let started = false;
  try {
    const r = await p.consumePendingStart?.();
    if (r?.taskId) { await startFocusTask(r.taskId); started = true; }
  } catch { /* ignore */ }
  try {
    const r = await p.consumePendingAction();
    if (!started) await route(r?.action);
  } catch { /* ignore */ }
}

/** Start the timer for a task chosen from the home-screen widget. The web store is
 *  the single source of truth, so the widget only ever asks; we start it here. */
async function startFocusTask(taskId?: string): Promise<void> {
  if (!taskId) return;
  const { pomodoro } = await import('$lib/stores/pomodoro.svelte');
  const { api } = await import('$lib/api');
  try {
    const t = await api.tasks.get(taskId);
    if (!t) return;
    // Starting a task from the widget supersedes any unconfirmed prior session:
    // drop its lingering confirm sheet so the new timer starts cleanly instead of
    // asking the user to log time for a task they're no longer on.
    if (pomodoro.pendingConfirm) pomodoro.discardSession();
    pomodoro.start(t.id, t.title, t.time_actual_minutes ?? 0, t.time_estimate_minutes ?? null);
  } catch { /* task gone / offline — ignore */ }
}

async function route(action?: string): Promise<void> {
  if (!action) return;
  const { pomodoro } = await import('$lib/stores/pomodoro.svelte');
  if (action === 'done') {
    pomodoro.finishTask();
  } else if (action === 'pause') {
    if (pomodoro.isRunning) pomodoro.togglePause();
  } else if (action === 'resume') {
    if (pomodoro.taskId && !pomodoro.isRunning) pomodoro.togglePause();
  }
}
