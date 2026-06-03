import { api } from '$lib/api';

type Phase = 'work' | 'short_break' | 'long_break';

const PREFS_KEY = 'pomodoro_prefs';

class PomodoroTimer {
  taskId    = $state<string | null>(null);
  taskTitle = $state<string | null>(null);
  phase     = $state<Phase>('work');
  totalSeconds = $state(25 * 60);
  remaining    = $state(25 * 60);
  isRunning    = $state(false);
  completedToday = $state(0);
  lastTimeUpdate = $state<{ taskId: string; newActual: number } | null>(null);

  // Custom durations (minutes)
  workMins       = $state(25);
  shortBreakMins = $state(5);
  longBreakMins  = $state(15);

  #intervalId:   ReturnType<typeof setInterval> | null = null;
  #sessionStart: string | null = null;
  #initialActual = 0;

  constructor() {
    if (typeof window !== 'undefined') {
      try {
        const raw = localStorage.getItem(PREFS_KEY);
        if (raw) {
          const p = JSON.parse(raw);
          if (p.workMins > 0)       this.workMins       = p.workMins;
          if (p.shortBreakMins > 0) this.shortBreakMins = p.shortBreakMins;
          if (p.longBreakMins > 0)  this.longBreakMins  = p.longBreakMins;
        }
      } catch { /* ignore */ }
      // Sync initial totalSeconds/remaining with prefs
      this.totalSeconds = this.workMins * 60;
      this.remaining    = this.workMins * 60;
    }
  }

  setPrefs(workMins: number, shortBreakMins: number, longBreakMins: number) {
    this.workMins       = workMins;
    this.shortBreakMins = shortBreakMins;
    this.longBreakMins  = longBreakMins;
    try {
      localStorage.setItem(PREFS_KEY, JSON.stringify({ workMins, shortBreakMins, longBreakMins }));
    } catch { /* ignore */ }
  }

  get progressPct(): number {
    return Math.round(((this.totalSeconds - this.remaining) / this.totalSeconds) * 100);
  }
  get display(): string {
    const m = Math.floor(this.remaining / 60).toString().padStart(2, '0');
    const s = (this.remaining % 60).toString().padStart(2, '0');
    return `${m}:${s}`;
  }
  get phaseLabel(): string {
    if (this.phase === 'work')        return 'Focus';
    if (this.phase === 'short_break') return 'Short break';
    return 'Long break';
  }

  start(taskId: string, taskTitle: string, currentActualMinutes = 0) {
    this.#clearInterval();
    this.taskId          = taskId;
    this.taskTitle       = taskTitle;
    this.#initialActual  = currentActualMinutes;
    this.phase           = 'work';
    this.totalSeconds    = this.workMins * 60;
    this.remaining       = this.workMins * 60;
    this.#sessionStart   = new Date().toISOString();
    this.#resume();
  }

  togglePause() { this.isRunning ? this.#pause() : this.#resume(); }

  stop() {
    this.#pause();
    this.taskId = null; this.taskTitle = null;
    this.phase = 'work';
    this.totalSeconds = this.workMins * 60;
    this.remaining    = this.workMins * 60;
    this.#sessionStart = null;
  }

  #resume() { this.isRunning = true; this.#intervalId = setInterval(() => this.#tick(), 1000); }
  #pause()  { this.isRunning = false; this.#clearInterval(); }
  #clearInterval() {
    if (this.#intervalId !== null) { clearInterval(this.#intervalId); this.#intervalId = null; }
  }
  #tick() { if (this.remaining > 0) this.remaining--; else void this.#onComplete(); }

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

  async #onComplete() {
    this.#pause();
    if (this.phase === 'work') {
      const durationMins = Math.round(this.totalSeconds / 60);
      if (this.taskId && this.#sessionStart) {
        try {
          await api.pomodoros.create({
            task_id: this.taskId, duration_minutes: durationMins,
            started_at: this.#sessionStart, completed_at: new Date().toISOString(),
            was_completed: true,
          });
          const newActual = this.#initialActual + durationMins;
          await api.tasks.update(this.taskId, { time_actual_minutes: newActual });
          this.lastTimeUpdate = { taskId: this.taskId, newActual };
          this.#initialActual = newActual;
        } catch { /* non-critical */ }
      }

      this.completedToday++;
      const isLongBreak  = this.completedToday % 4 === 0;
      this.phase        = isLongBreak ? 'long_break' : 'short_break';
      this.totalSeconds = isLongBreak ? this.longBreakMins * 60 : this.shortBreakMins * 60;
      this.remaining    = this.totalSeconds;
      this.#sessionStart = null;

      const breakLabel = isLongBreak ? `${this.longBreakMins}-min break` : `${this.shortBreakMins}-min break`;
      void this.#notify('Pomodoro complete!', `Time for a ${breakLabel}.`);
    } else {
      void this.#notify('Break over', 'Time to focus.');
      this.phase        = 'work';
      this.totalSeconds = this.workMins * 60;
      this.remaining    = this.totalSeconds;
      this.#sessionStart = new Date().toISOString();
      this.#resume();
      return;
    }
    this.#resume();
  }
}

export const pomodoro = new PomodoroTimer();
