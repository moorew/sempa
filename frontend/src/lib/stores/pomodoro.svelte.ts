import { api } from '$lib/api';
import { syncFocusNotification } from '$lib/focusTimerNotification';
import { isTauri } from '$lib/tauri/bridge';

type Phase = 'work' | 'short_break' | 'long_break';

const PREFS_KEY = 'pomodoro_prefs';
const STATE_KEY = 'pomodoro_state';

// What the end-of-session confirm sheet needs to show. We move the live session
// into this object when the user stops/finishes so the widget can disappear while
// the confirm sheet stays up until they log (or discard) the time.
export interface PendingConfirm {
  taskId: string;
  taskTitle: string;
  elapsedMinutes: number;   // measured wall-clock (running) time, pre-filled
  plannedMinutes: number | null;
  priorActual: number;      // already-logged minutes on the task
  markDone: boolean;        // true when triggered via "Done" (also set status=done)
}

function todayKey(): string {
  // Local YYYY-MM-DD — used to reset the daily pomodoro counter at midnight.
  const d = new Date();
  return `${d.getFullYear()}-${d.getMonth() + 1}-${d.getDate()}`;
}

// The measurement insight: the countdown is just a focus aid. What we *log* is
// the real wall-clock time the timer spent actually running (paused/break time
// excluded), which the user then confirms or adjusts at the end. This is what
// makes the data honest for time-blindness — both over- and under-runs are
// captured, and walking away without pausing is corrected at confirm time.
class PomodoroTimer {
  taskId    = $state<string | null>(null);
  taskTitle = $state<string | null>(null);
  plannedMinutes = $state<number | null>(null);
  phase     = $state<Phase>('work');
  totalSeconds = $state(25 * 60);
  remaining    = $state(25 * 60);
  isRunning    = $state(false);
  completedToday = $state(0);
  // Running work time in whole seconds — a reactive mirror of #elapsedWorkMs()
  // that the tick refreshes so the "spent" readout visibly counts up.
  elapsedSeconds = $state(0);
  lastTimeUpdate = $state<{ taskId: string; newActual: number; done: boolean } | null>(null);
  pendingConfirm = $state<PendingConfirm | null>(null);

  // Custom durations (minutes)
  workMins       = $state(25);
  shortBreakMins = $state(5);
  longBreakMins  = $state(15);

  #intervalId:   ReturnType<typeof setInterval> | null = null;
  #sessionStart: string | null = null;     // ISO start of the whole focus session
  #initialActual = 0;                       // task.time_actual_minutes at session start
  #completedDate = todayKey();

  // Epoch-based timekeeping so reload, navigation, and background-tab throttling
  // never drift the clock — everything is derived from wall-clock timestamps.
  #phaseEndMs: number | null = null;        // when the current phase hits 0 (running only)
  #workAccumulatedMs = 0;                    // running work time banked from prior segments
  #workSegmentStartMs: number | null = null; // epoch the current running work segment began

  // ── Cross-window coordination ──────────────────────────────────────────────
  // The timer is a module singleton, but on desktop (Tauri) it runs in several
  // webviews at once (main window + the floating Pomodoro widget window), which
  // share localStorage. Without coordination each instance would tick its own
  // interval and double-fire phase completions, notifications and API writes.
  //
  // We elect ONE leader via the Web Locks API (it auto-releases when a window
  // closes or crashes — no heartbeat to get wrong). The leader is the sole owner
  // of the interval, persistence, notifications and API calls; it publishes state
  // snapshots over a BroadcastChannel. Followers never tick — they mirror the
  // leader's snapshots and forward user actions to it as commands, so there's a
  // single source of truth. A lone window (web, Android, desktop-main-only) is
  // trivially its own leader, so single-window behaviour is unchanged.
  #isLeader = false;
  #bc: BroadcastChannel | null = null;
  #releaseLock: (() => void) | null = null;
  #lastBroadcastRemaining = -1;

  constructor() {
    if (typeof window !== 'undefined') {
      this.#loadPrefs();
      this.totalSeconds = this.workMins * 60;
      this.remaining    = this.workMins * 60;
      // Paint last-known state immediately; leadership (and ticking) is resolved
      // asynchronously below so two windows never both drive the clock.
      this.#restoreForDisplay();
      this.#initCoordination();
    }
  }

  // ── Coordination internals ─────────────────────────────────────────────────
  #initCoordination() {
    try { this.#bc = new BroadcastChannel('sempa-pomodoro'); } catch { this.#bc = null; }
    if (this.#bc) this.#bc.onmessage = (ev) => this.#onMessage(ev.data);

    // Leader election only matters where several timer webviews coexist — that's
    // ONLY the Tauri desktop shell (main window + the floating widget window).
    // Capacitor (Android) and a normal browser tab are single-context, so electing
    // a leader asynchronously risks a mutation (e.g. start) firing before the lock
    // is granted and being forwarded to nobody. Become leader immediately there.
    const locks = (isTauri() && typeof navigator !== 'undefined'
      ? (navigator as Navigator & { locks?: LockManager }).locks
      : undefined);
    if (locks?.request) {
      // Hold the exclusive lock for this window's lifetime: the callback's promise
      // resolves only on pagehide, so we keep leadership until the window closes
      // (or crashes — the browser then auto-releases it to the next waiter).
      void locks.request('sempa-pomodoro-leader', { mode: 'exclusive' }, () => {
        this.#becomeLeader();
        return new Promise<void>((resolve) => { this.#releaseLock = resolve; });
      }).catch(() => { /* lock unavailable → stay a follower; a leader exists */ });
      // Nudge whoever currently leads to send us a snapshot so we paint live state.
      this.#post({ type: 'sync-request' });
      window.addEventListener('pagehide', () => this.#releaseLock?.());
    } else {
      // No Web Locks in this webview → act as sole leader. Multi-window would then
      // fall back to pre-coordination behaviour; single-window is unaffected.
      this.#becomeLeader();
    }
  }

  #becomeLeader() {
    if (this.#isLeader) return;
    this.#isLeader = true;
    this.#clearInterval();
    // Re-read the authoritative persisted state and resume ticking if a session
    // was live (epoch anchors make this exact, even across a leader handover).
    this.#restoreForLeader();
    this.#broadcastState();
  }

  #onMessage(data: unknown) {
    if (!data || typeof data !== 'object') return;
    const msg = data as { type?: string; state?: unknown; method?: string; args?: unknown[] };
    if (msg.type === 'state') {
      if (!this.#isLeader && msg.state) this.#applyRemoteState(msg.state as Record<string, unknown>);
    } else if (msg.type === 'command') {
      // Only the leader executes mutations — followers forwarded them here.
      if (this.#isLeader && msg.method) {
        (this as unknown as Record<string, (...a: unknown[]) => unknown>)[msg.method]?.(...(msg.args ?? []));
      }
    } else if (msg.type === 'sync-request') {
      if (this.#isLeader) this.#broadcastState();
    }
  }

  #post(msg: unknown) { try { this.#bc?.postMessage(msg); } catch { /* channel closed */ } }

  /** Forward a mutating call from a follower to the leader (single writer). */
  #forward(method: string, args: unknown[]) { this.#post({ type: 'command', method, args }); }

  #broadcastState() {
    if (!this.#isLeader) return;
    this.#post({ type: 'state', state: this.#snapshot() });
  }

  #snapshot() {
    return {
      taskId: this.taskId, taskTitle: this.taskTitle, plannedMinutes: this.plannedMinutes,
      phase: this.phase, totalSeconds: this.totalSeconds, remaining: this.remaining,
      isRunning: this.isRunning, completedToday: this.completedToday,
      elapsedSeconds: this.elapsedSeconds, pendingConfirm: this.pendingConfirm,
      workMins: this.workMins, shortBreakMins: this.shortBreakMins, longBreakMins: this.longBreakMins,
    };
  }

  #applyRemoteState(s: Record<string, unknown>) {
    this.taskId         = (s.taskId as string | null) ?? null;
    this.taskTitle      = (s.taskTitle as string | null) ?? null;
    this.plannedMinutes = (s.plannedMinutes as number | null) ?? null;
    this.phase          = (s.phase as Phase) ?? 'work';
    this.totalSeconds   = (s.totalSeconds as number) ?? this.workMins * 60;
    this.remaining      = (s.remaining as number) ?? this.totalSeconds;
    this.isRunning      = Boolean(s.isRunning);
    this.completedToday = (s.completedToday as number) ?? 0;
    this.elapsedSeconds = (s.elapsedSeconds as number) ?? 0;
    this.pendingConfirm = (s.pendingConfirm as PendingConfirm | null) ?? null;
    if (typeof s.workMins === 'number')       this.workMins = s.workMins;
    if (typeof s.shortBreakMins === 'number') this.shortBreakMins = s.shortBreakMins;
    if (typeof s.longBreakMins === 'number')  this.longBreakMins = s.longBreakMins;
  }

  // ── Preferences ────────────────────────────────────────────────────────────
  #loadPrefs() {
    try {
      const raw = localStorage.getItem(PREFS_KEY);
      if (raw) {
        const p = JSON.parse(raw);
        if (p.workMins > 0)       this.workMins       = p.workMins;
        if (p.shortBreakMins > 0) this.shortBreakMins = p.shortBreakMins;
        if (p.longBreakMins > 0)  this.longBreakMins  = p.longBreakMins;
      }
    } catch { /* ignore */ }
  }

  setPrefs(workMins: number, shortBreakMins: number, longBreakMins: number) {
    if (!this.#isLeader) { this.#forward('setPrefs', [workMins, shortBreakMins, longBreakMins]); return; }
    this.workMins       = workMins;
    this.shortBreakMins = shortBreakMins;
    this.longBreakMins  = longBreakMins;
    try {
      localStorage.setItem(PREFS_KEY, JSON.stringify({ workMins, shortBreakMins, longBreakMins }));
    } catch { /* ignore */ }
    // If idle, reflect the new focus length immediately.
    if (!this.taskId && this.phase === 'work') {
      this.totalSeconds = workMins * 60;
      this.remaining    = workMins * 60;
    }
  }

  // ── Derived display ──────────────────────────────────────────────────────────
  get progressPct(): number {
    if (this.totalSeconds <= 0) return 0;
    return Math.round(((this.totalSeconds - this.remaining) / this.totalSeconds) * 100);
  }
  get display(): string {
    return this.#fmt(this.remaining);
  }
  get phaseLabel(): string {
    if (this.phase === 'work')        return 'Focus';
    if (this.phase === 'short_break') return 'Short break';
    return 'Long break';
  }

  /** Wall-clock running work time this session, in whole minutes. */
  get elapsedWorkMinutes(): number {
    return Math.round(this.elapsedSeconds / 60);
  }
  /** "12:34" of elapsed running work time — the honest measure, ticks up live. */
  get elapsedDisplay(): string {
    return this.#fmt(this.elapsedSeconds);
  }
  /** Minutes over (positive) or under (negative) the planned estimate; null if no plan. */
  get overByMinutes(): number | null {
    if (!this.plannedMinutes) return null;
    return this.elapsedWorkMinutes - this.plannedMinutes;
  }

  #fmt(totalSec: number): string {
    const sec = Math.max(0, totalSec);
    const m = Math.floor(sec / 60).toString().padStart(2, '0');
    const s = (sec % 60).toString().padStart(2, '0');
    return `${m}:${s}`;
  }

  isFor(taskId: string): boolean {
    return this.taskId === taskId;
  }

  // ── Lifecycle ────────────────────────────────────────────────────────────────
  start(taskId: string, taskTitle: string, currentActualMinutes = 0, plannedMinutes: number | null = null) {
    if (!this.#isLeader) { this.#forward('start', [taskId, taskTitle, currentActualMinutes, plannedMinutes]); return; }
    this.#clearInterval();
    this.#rolloverDay();
    this.taskId          = taskId;
    this.taskTitle       = taskTitle;
    this.plannedMinutes  = plannedMinutes;
    this.#initialActual  = currentActualMinutes;
    this.phase           = 'work';
    this.totalSeconds    = this.workMins * 60;
    this.remaining       = this.workMins * 60;
    this.#sessionStart   = new Date().toISOString();
    this.#workAccumulatedMs  = 0;
    this.#workSegmentStartMs = null;
    this.#resume();
  }

  togglePause() {
    if (!this.#isLeader) { this.#forward('togglePause', []); return; }
    this.isRunning ? this.#pause() : this.#resume();
  }

  /** Stop the session and open the confirm sheet (logs time, leaves status as-is). */
  requestStop() {
    if (!this.#isLeader) { this.#forward('requestStop', []); return; }
    this.#endSession(false);
  }

  /** Mark the task done and open the confirm sheet to log the time spent. */
  finishTask() {
    if (!this.#isLeader) { this.#forward('finishTask', []); return; }
    this.#endSession(true);
  }

  /** Hard reset with no logging (used internally and by "discard"). */
  reset() {
    if (!this.#isLeader) { this.#forward('reset', []); return; }
    this.#clearInterval();
    this.taskId = null; this.taskTitle = null; this.plannedMinutes = null;
    this.phase = 'work';
    this.totalSeconds = this.workMins * 60;
    this.remaining    = this.workMins * 60;
    this.isRunning = false;
    this.#sessionStart = null;
    this.#phaseEndMs = null;
    this.#workAccumulatedMs = 0;
    this.#workSegmentStartMs = null;
    this.elapsedSeconds = 0;
    this.#persist();
    this.#syncNotification();
  }

  #endSession(markDone: boolean) {
    if (!this.taskId) { this.reset(); return; }
    this.#pause(); // banks the active work segment into #workAccumulatedMs
    const elapsed = Math.max(0, this.elapsedWorkMinutes);
    this.pendingConfirm = {
      taskId: this.taskId,
      taskTitle: this.taskTitle ?? 'this task',
      elapsedMinutes: elapsed,
      plannedMinutes: this.plannedMinutes,
      priorActual: this.#initialActual,
      markDone,
    };
    // Hide the live widget; the confirm sheet now owns the flow.
    this.#clearInterval();
    this.taskId = null; this.taskTitle = null; this.plannedMinutes = null;
    this.isRunning = false;
    this.#phaseEndMs = null;
    this.#persist();
    this.#syncNotification();
  }

  /** Persist the confirmed minutes: accumulate into the task and log a session. */
  async confirmSession(minutes: number) {
    if (!this.#isLeader) { this.#forward('confirmSession', [minutes]); return; }
    const pc = this.pendingConfirm;
    if (!pc) return;
    this.pendingConfirm = null;
    const logged = Math.max(0, Math.round(minutes));
    const newActual = pc.priorActual + logged;
    try {
      if (logged > 0) {
        await api.pomodoros.create({
          task_id: pc.taskId,
          duration_minutes: logged,
          started_at: this.#sessionStart ?? new Date().toISOString(),
          completed_at: new Date().toISOString(),
          was_completed: true,
        });
      }
      const update: Record<string, unknown> = { time_actual_minutes: newActual };
      if (pc.markDone) {
        update.status = 'done';
        update.completed_at = new Date().toISOString();
      }
      await api.tasks.update(pc.taskId, update);
      this.lastTimeUpdate = { taskId: pc.taskId, newActual, done: pc.markDone };
      // Fold the new data point into the planned-vs-actual profile.
      void import('$lib/stores/timeInsights.svelte').then((m) => m.timeInsights.refresh());
    } catch { /* non-critical — task update may retry via sync outbox */ }
    this.#sessionStart = null;
    this.#workAccumulatedMs = 0;
    this.#workSegmentStartMs = null;
    this.#persist();
    this.#syncNotification();
  }

  /** Close the confirm sheet without logging anything. */
  discardSession() {
    if (!this.#isLeader) { this.#forward('discardSession', []); return; }
    this.pendingConfirm = null;
    this.#sessionStart = null;
    this.#workAccumulatedMs = 0;
    this.#workSegmentStartMs = null;
    this.#persist();
    this.#syncNotification();
  }

  // ── Timer internals ──────────────────────────────────────────────────────────
  #resume() {
    this.isRunning = true;
    this.#phaseEndMs = Date.now() + this.remaining * 1000;
    if (this.phase === 'work') this.#workSegmentStartMs = Date.now();
    this.#syncElapsed();
    this.#intervalId = setInterval(() => this.#tick(), 250);
    this.#persist();
    this.#syncNotification();
  }

  #pause() {
    if (!this.isRunning) return;
    // Bank any in-flight work segment before stopping the clock.
    if (this.phase === 'work' && this.#workSegmentStartMs !== null) {
      this.#workAccumulatedMs += Date.now() - this.#workSegmentStartMs;
      this.#workSegmentStartMs = null;
    }
    this.isRunning = false;
    this.#clearInterval();
    this.#phaseEndMs = null;
    this.#syncElapsed();
    this.#persist();
    this.#syncNotification();
  }

  #clearInterval() {
    if (this.#intervalId !== null) { clearInterval(this.#intervalId); this.#intervalId = null; }
  }

  // Mirror the live wall-clock work time into reactive state so the "spent"
  // readout counts up second-by-second alongside the countdown.
  #syncElapsed() {
    this.elapsedSeconds = Math.floor(this.#elapsedWorkMs() / 1000);
  }

  // Push state to the native Android ongoing-notification (no-op elsewhere).
  #syncNotification() {
    if (this.taskId) {
      syncFocusNotification({
        title: this.taskTitle ?? 'Focus',
        phase: this.phaseLabel,
        running: this.isRunning,
        endMs: this.#phaseEndMs ?? 0,
        remainingMs: this.remaining * 1000,
      });
    } else {
      syncFocusNotification(null);
    }
  }

  #tick() {
    if (this.#phaseEndMs === null) return;
    this.#syncElapsed();
    const rem = Math.round((this.#phaseEndMs - Date.now()) / 1000);
    if (rem <= 0) {
      this.remaining = 0;
      void this.#onComplete();
    } else {
      this.remaining = rem;
    }
    // Mirror the live countdown to follower windows, but only when the displayed
    // second actually changes (the interval fires 4×/s) to keep the channel quiet.
    if (this.remaining !== this.#lastBroadcastRemaining) {
      this.#lastBroadcastRemaining = this.remaining;
      this.#broadcastState();
    }
  }

  #playChime() {
    try {
      const ctx = new AudioContext();
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.type = 'sine';
      osc.frequency.setValueAtTime(880, ctx.currentTime);
      osc.frequency.exponentialRampToValueAtTime(440, ctx.currentTime + 0.5);
      gain.gain.setValueAtTime(0.25, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.8);
      osc.start(ctx.currentTime);
      osc.stop(ctx.currentTime + 0.8);
      ctx.close().catch(() => {});
    } catch { /* audio not available */ }
  }

  async #notify(title: string, body: string) {
    this.#playChime();
    if (typeof Notification === 'undefined') return;
    if (Notification.permission === 'default') {
      await Notification.requestPermission();
    }
    if (Notification.permission === 'granted') {
      new Notification(title, { body, silent: true });
    }
  }

  // A phase boundary is a *rhythm* cue, not a logging event. We bank the work
  // time, nudge a break, and keep accumulating across pomodoros — nothing is
  // written until the user confirms at the end of the session.
  #onComplete() {
    this.#clearInterval();
    if (this.phase === 'work') {
      // Bank the completed work segment.
      if (this.#workSegmentStartMs !== null) {
        this.#workAccumulatedMs += Date.now() - this.#workSegmentStartMs;
        this.#workSegmentStartMs = null;
      }
      this.#rolloverDay();
      this.completedToday++;
      const isLongBreak  = this.completedToday % 4 === 0;
      this.phase        = isLongBreak ? 'long_break' : 'short_break';
      this.totalSeconds = isLongBreak ? this.longBreakMins * 60 : this.shortBreakMins * 60;
      this.remaining    = this.totalSeconds;

      const breakLabel = isLongBreak ? `${this.longBreakMins}-min break` : `${this.shortBreakMins}-min break`;
      void this.#notify('Focus interval done', `Nice — take a ${breakLabel}. Your time keeps counting toward this task.`);
      this.#resume(); // break starts ticking; work accumulation stays paused
    } else {
      void this.#notify('Break over', 'Back to it.');
      this.phase        = 'work';
      this.totalSeconds = this.workMins * 60;
      this.remaining    = this.totalSeconds;
      this.#resume(); // resuming work starts a fresh accumulation segment
    }
  }

  #elapsedWorkMs(): number {
    let ms = this.#workAccumulatedMs;
    if (this.isRunning && this.phase === 'work' && this.#workSegmentStartMs !== null) {
      ms += Date.now() - this.#workSegmentStartMs;
    }
    return ms;
  }

  #rolloverDay() {
    const t = todayKey();
    if (t !== this.#completedDate) {
      this.#completedDate = t;
      this.completedToday = 0;
    }
  }

  // ── Persistence ──────────────────────────────────────────────────────────────
  // Persisting the live session (with epoch anchors) is what lets the timer
  // survive navigation and reload accurately — fixing the old "timer dies on
  // reload" bug that made it untrustworthy.
  #persist() {
    if (typeof window === 'undefined') return;
    try {
      if (!this.taskId && !this.pendingConfirm) {
        localStorage.removeItem(STATE_KEY);
        return;
      }
      localStorage.setItem(STATE_KEY, JSON.stringify({
        taskId: this.taskId,
        taskTitle: this.taskTitle,
        plannedMinutes: this.plannedMinutes,
        phase: this.phase,
        totalSeconds: this.totalSeconds,
        remaining: this.remaining,
        isRunning: this.isRunning,
        completedToday: this.completedToday,
        completedDate: this.#completedDate,
        sessionStart: this.#sessionStart,
        initialActual: this.#initialActual,
        phaseEndMs: this.#phaseEndMs,
        workAccumulatedMs: this.#workAccumulatedMs,
        workSegmentStartMs: this.#workSegmentStartMs,
        pendingConfirm: this.pendingConfirm,
        savedAt: Date.now(),
      }));
    } catch { /* ignore quota / serialization errors */ }
    // Every structural change (start/pause/resume/complete/confirm/reset) flows
    // through here on the leader — mirror it to follower windows.
    this.#broadcastState();
  }

  // Followers (and the first paint before leadership resolves) read last-known
  // state for display ONLY — never starting the interval, persisting, or firing
  // notifications. A live snapshot from the leader arrives moments later and takes
  // over. Approximate the "spent" readout from the stored anchors to avoid a flash
  // of 00:00 while we wait for it.
  #restoreForDisplay() {
    let raw: string | null = null;
    try { raw = localStorage.getItem(STATE_KEY); } catch { return; }
    if (!raw) return;
    let s: any;
    try { s = JSON.parse(raw); } catch { return; }

    this.#completedDate = s.completedDate ?? todayKey();
    this.completedToday = s.completedDate === todayKey() ? (s.completedToday ?? 0) : 0;

    if (s.pendingConfirm) { this.pendingConfirm = s.pendingConfirm; return; }
    if (!s.taskId) return;

    this.taskId         = s.taskId;
    this.taskTitle      = s.taskTitle ?? null;
    this.plannedMinutes = s.plannedMinutes ?? null;
    this.phase          = s.phase ?? 'work';
    this.totalSeconds   = s.totalSeconds ?? this.workMins * 60;

    if (s.isRunning && typeof s.phaseEndMs === 'number') {
      const rem = Math.round((s.phaseEndMs - Date.now()) / 1000);
      this.remaining = Math.max(0, rem);
      this.isRunning = rem > 0; // if it elapsed while away, the leader settles it
    } else {
      this.remaining = s.remaining ?? this.totalSeconds;
      this.isRunning = false;
    }

    let ms = s.workAccumulatedMs ?? 0;
    if (s.isRunning && s.phase === 'work' && typeof s.workSegmentStartMs === 'number') {
      ms += Math.max(0, Date.now() - s.workSegmentStartMs);
    }
    this.elapsedSeconds = Math.floor(ms / 1000);
  }

  #restoreForLeader() {
    let raw: string | null = null;
    try { raw = localStorage.getItem(STATE_KEY); } catch { return; }
    if (!raw) return;
    let s: any;
    try { s = JSON.parse(raw); } catch { return; }

    this.#completedDate = s.completedDate ?? todayKey();
    this.completedToday = s.completedDate === todayKey() ? (s.completedToday ?? 0) : 0;

    // A confirm sheet was open when we left — restore it so the user can finish.
    if (s.pendingConfirm) {
      this.pendingConfirm = s.pendingConfirm;
      this.#sessionStart  = s.sessionStart ?? null;
      return;
    }
    if (!s.taskId) return;

    this.taskId         = s.taskId;
    this.taskTitle      = s.taskTitle ?? null;
    this.plannedMinutes = s.plannedMinutes ?? null;
    this.phase          = s.phase ?? 'work';
    this.totalSeconds   = s.totalSeconds ?? this.workMins * 60;
    this.#sessionStart  = s.sessionStart ?? null;
    this.#initialActual = s.initialActual ?? 0;
    this.#workAccumulatedMs  = s.workAccumulatedMs ?? 0;
    this.#workSegmentStartMs = s.workSegmentStartMs ?? null;

    if (s.isRunning && typeof s.phaseEndMs === 'number') {
      const rem = Math.round((s.phaseEndMs - Date.now()) / 1000);
      if (rem > 0) {
        // Mid-phase: pick up exactly where the wall clock says we are.
        this.remaining   = rem;
        this.#phaseEndMs = s.phaseEndMs;
        this.isRunning   = true;
        this.#intervalId = setInterval(() => this.#tick(), 250);
      } else {
        // The phase elapsed while we were away — settle it and continue.
        this.remaining = 0;
        // If a work segment was open across the gap, bank up to the phase end.
        if (this.phase === 'work' && this.#workSegmentStartMs !== null) {
          this.#workAccumulatedMs += Math.max(0, s.phaseEndMs - this.#workSegmentStartMs);
          this.#workSegmentStartMs = null;
        }
        this.isRunning = true;
        void this.#onComplete();
      }
    } else {
      this.remaining = s.remaining ?? this.totalSeconds;
      this.isRunning = false;
      this.#phaseEndMs = null;
    }
    this.#syncElapsed();
    this.#syncNotification();
  }
}

export const pomodoro = new PomodoroTimer();
