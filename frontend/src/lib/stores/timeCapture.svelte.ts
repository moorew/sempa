// Completion time-capture: the engine behind "how long did that take?" prompts.
//
// Hooked once at api.tasks.update (see lib/api.ts): any task completed *without*
// tracked time triggers maybePrompt(). The point is to gather data even for the
// tasks you never ran the focus timer on — the ones time-blindness makes you
// forget — so the planned-vs-actual profile reflects real life, not just the
// sessions you remembered to track.
//
// Smart/adaptive: it skips trivially-quick tasks, backs off during a burst of
// completions, and pauses for the session after a few skips, so it never nags.
import type { Task } from '$lib/types';
import { api } from '$lib/api';
import { prefs } from '$lib/stores/prefs.svelte';
import { aiStatus } from '$lib/stores/aiStatus.svelte';
import { timeTracking } from '$lib/stores/timeTracking.svelte';
import { classifyActivity } from '$lib/activityBuckets';
import { timeProfile } from '$lib/stores/timeProfile.svelte';

interface CaptureItem {
  taskId: string;
  title: string;
  tags: string[];
  bucketKey: string;
  defaultMinutes: number;
}

const BATCH_WINDOW_MS = 60_000;
const BATCH_LIMIT = 4;       // ≥ this many completions in the window → stay quiet
const SKIP_PAUSE_AFTER = 3;  // consecutive skips → pause prompts for the session

class TimeCapture {
  pending = $state<CaptureItem | null>(null);
  // A better, AI-grounded suggestion that arrives asynchronously (non-blocking).
  aiSuggestion = $state<number | null>(null);
  // Set when a learned bucket was logged without asking — drives the undo toast.
  autoLogged = $state<(CaptureItem & { minutes: number }) | null>(null);

  #queue: CaptureItem[] = [];
  #recent: number[] = [];
  #consecutiveSkips = 0;
  #pausedForSession = false;

  /** Decide whether to prompt for time on a just-completed task. */
  maybePrompt(task: Task | null | undefined) {
    if (!task || task.status !== 'done') return;
    if (!timeTracking.promptOnComplete || this.#pausedForSession) return;
    // Subtasks/checklist items are completed in bursts — don't prompt for those.
    if (task.parent_task_id) return;
    // Already has logged time (e.g. via the focus timer) — don't re-ask.
    if ((task.time_actual_minutes ?? 0) > 0) return;
    // Trivially-quick tasks aren't worth interrupting for.
    const est = task.time_estimate_minutes ?? 0;
    if (timeTracking.skipQuick && est > 0 && est <= 5) return;

    const bucket = classifyActivity(task.title, task.tags ?? []);
    const item: CaptureItem = {
      taskId: task.id,
      title: task.title,
      tags: task.tags ?? [],
      bucketKey: bucket.key,
      defaultMinutes: est > 0 ? est : timeTracking.defaultMinutesFor(bucket.key),
    };

    // Graduation: once this kind of work has been taught enough times AND the
    // durations are consistent, stop asking. Log the learned figure and say so,
    // rather than interrupting to ask a question we can already answer.
    const learned = timeProfile.learnedMinutes(bucket.key);
    if (learned !== null) {
      void this.#autoLog(item, learned);
      return;
    }

    // Burst back-off: if you're rapid-firing completions, stay out of the way.
    const now = Date.now();
    this.#recent = this.#recent.filter((t) => now - t < BATCH_WINDOW_MS);
    if (this.#recent.length >= BATCH_LIMIT) return;
    this.#recent.push(now);

    if (this.pending) this.#queue.push(item);
    else this.#open(item);
  }

  /**
   * Write the learned duration without a prompt, and surface it.
   *
   * Not silent on purpose: time appearing on tasks you never confirmed is hard to
   * notice and harder to trust. The toast is non-blocking (needs no action) but
   * makes the guess visible and one tap correctable — and a correction feeds a
   * fresh sample, so the profile keeps adjusting instead of freezing at whatever
   * it first learned.
   */
  async #autoLog(item: CaptureItem, minutes: number) {
    this.autoLogged = { ...item, minutes };
    try {
      await api.tasks.update(item.taskId, { time_actual_minutes: minutes });
      void import('$lib/stores/timeInsights.svelte').then((m) => m.timeInsights.refresh());
      timeProfile.scheduleRefresh();
    } catch { /* queues via the sync outbox when offline */ }
  }

  /** Dismiss the auto-log toast. */
  clearAutoLogged() { this.autoLogged = null; }

  /** "Change" on the toast: reopen the normal prompt, pre-filled with the guess. */
  correctAutoLogged() {
    const a = this.autoLogged;
    if (!a) return;
    this.autoLogged = null;
    this.#open({ ...a, defaultMinutes: a.minutes });
  }

  #open(item: CaptureItem) {
    this.pending = item;
    this.aiSuggestion = null;
    void this.#refine(item);
  }

  // Non-blocking AI refinement: the modal opens instantly on the keyword default;
  // if the local model is available it quietly offers a history-grounded number.
  async #refine(item: CaptureItem) {
    if (!prefs.aiOn('predictTime') || !aiStatus.reachable) return;
    try {
      const res = await api.ai.predictTime(item.title, item.tags);
      if (this.pending?.taskId === item.taskId && res.available && res.minutes) {
        this.aiSuggestion = res.minutes;
      }
    } catch { /* offline / model busy — keep the keyword default */ }
  }

  /** Log the confirmed minutes against the task and advance the queue. */
  async log(minutes: number) {
    const p = this.pending;
    if (!p) return;
    this.#consecutiveSkips = 0;
    this.#advance();
    const mins = Math.max(0, Math.round(minutes));
    if (mins <= 0) return;
    try {
      await api.tasks.update(p.taskId, { time_actual_minutes: mins });
      void import('$lib/stores/timeInsights.svelte').then((m) => m.timeInsights.refresh());
      // Every confirmed number is a teaching signal — refresh so the bucket can
      // graduate (or, after a correction, revise what it thought it knew).
      timeProfile.scheduleRefresh();
    } catch { /* queues via sync outbox offline */ }
  }

  /** Dismiss without logging; pause for the session after repeated skips. */
  skip() {
    if (!this.pending) return;
    this.#consecutiveSkips++;
    if (this.#consecutiveSkips >= SKIP_PAUSE_AFTER) this.#pausedForSession = true;
    this.#advance();
  }

  /** "Don't ask again" — turn the feature off (re-enableable in settings). */
  disable() {
    timeTracking.setPromptOnComplete(false);
    this.pending = null;
    this.#queue = [];
  }

  /** Clear a session pause (e.g. when the user revisits time-tracking settings). */
  resume() {
    this.#pausedForSession = false;
    this.#consecutiveSkips = 0;
  }

  #advance() {
    const next = this.#queue.shift();
    if (next) this.#open(next);
    else this.pending = null;
  }
}

export const timeCapture = new TimeCapture();
