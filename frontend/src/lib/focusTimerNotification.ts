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
  addListener?(event: string, cb: (data: { action?: string }) => void): unknown;
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
  try { const r = await p.consumePendingAction(); await route(r?.action); } catch { /* ignore */ }
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
