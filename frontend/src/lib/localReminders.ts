/**
 * On-device scheduled reminders for Android (Capacitor).
 *
 * This is the reliability backbone: it schedules a real OS alarm
 * (@capacitor/local-notifications) for every upcoming task reminder, read
 * straight from the LOCAL database. Because the OS holds the alarm, the
 * reminder fires even when:
 *   • the server is unreachable,
 *   • the device is fully offline, or
 *   • the app is closed.
 *
 * The server-side Web Push / FCM remains as a redundant channel. Per the
 * "prefer no-miss" choice we tolerate a rare duplicate rather than risk a miss,
 * so no cross-channel suppression is attempted.
 *
 * Scheduling is idempotent and diff-based: we keep a map of {taskId → alarm} in
 * localStorage and only (re)schedule what changed, cancelling alarms for tasks
 * that were completed, deleted, or had their reminder cleared/moved.
 */

import { isCapacitor } from '$lib/platform';
import { localApi } from '$lib/tauri/local-api';
import { notificationSettings } from '$lib/stores/notificationSettings.svelte';
import { DEFAULT_SOUND_ID, NOTIFICATION_SOUNDS } from '$lib/sounds';

const MAP_KEY = 'sempa-local-reminder-map';
// Records the app version the OS alarms were last scheduled under. Android cancels
// ALL pending AlarmManager alarms when the app is updated, but our diff map in
// localStorage survives the update — so without this every armed reminder would
// look "unchanged" and never be re-scheduled, and reminders silently stop firing
// after an upgrade. When this differs from the running version we force a full
// re-arm (see syncLocalReminders).
const APP_VERSION_KEY = 'sempa-local-reminder-app-version';

// Android notification channels are IMMUTABLE once created — importance and
// sound are frozen at first creation and silently ignored on later edits. Early
// builds created `snd_<id>` channels with a broken sound reference (bare resource
// name, no extension) which Android couldn't resolve, leaving the channel silent
// and low-priority forever. Bumping this version forces a fresh, correct channel
// on existing installs. Bump again if channel settings ever need to change.
const CHANNEL_VERSION = 'v2';
const soundChannelId = (soundId: string) => `rem_${soundId}_${CHANNEL_VERSION}`;
const SILENT_CHANNEL_ID = `rem_silent_${CHANNEL_VERSION}`;

interface ScheduledEntry {
  notifId: number;
  remindAt: string;
  title: string;
  soundId: string;
}
type ScheduleMap = Record<string, ScheduledEntry>;

let navigate: (url: string) => void = () => {};
let listenersBound = false;
let running = false;
let rerunQueued = false;

// Stable positive 31-bit int from a task UUID (local-notifications needs ints).
function notifIdFor(uuid: string): number {
  let h = 5381;
  for (let i = 0; i < uuid.length; i++) h = ((h << 5) + h + uuid.charCodeAt(i)) | 0;
  return Math.abs(h) % 2147483646 + 1;
}

function readMap(): ScheduleMap {
  try {
    return JSON.parse(localStorage.getItem(MAP_KEY) || '{}');
  } catch {
    return {};
  }
}
function writeMap(m: ScheduleMap) {
  try {
    localStorage.setItem(MAP_KEY, JSON.stringify(m));
  } catch {
    /* ignore */
  }
}

type LocalNotifModule = typeof import('@capacitor/local-notifications');
type LocalNotif = LocalNotifModule['LocalNotifications'];

async function loadPlugin(): Promise<LocalNotif | null> {
  // Prefer the already-registered global bridge instance (same one push.ts uses
  // successfully for PushNotifications). The dynamic import returns an equivalent
  // proxy, but going through the global avoids any chance of resolving the web
  // fallback and matches the proven-working path on this app.
  try {
    const cap = (window as unknown as { Capacitor?: { Plugins?: Record<string, unknown> } }).Capacitor;
    const fromGlobal = cap?.Plugins?.LocalNotifications;
    if (fromGlobal) return fromGlobal as LocalNotif;
  } catch {
    /* fall through to dynamic import */
  }
  try {
    const mod = await import('@capacitor/local-notifications');
    return mod.LocalNotifications;
  } catch {
    return null;
  }
}

// Current app version (e.g. "1.21.2"), or "" if unavailable. Used to detect an
// app update, after which Android has dropped all our scheduled alarms.
async function appVersion(): Promise<string> {
  try {
    const { App } = await import('@capacitor/app');
    return (await App.getInfo()).version || '';
  } catch {
    return '';
  }
}

/**
 * Reject if a native bridge call doesn't settle in `ms`. Capacitor calls
 * occasionally never call back (e.g. a permission dialog whose result plumbing
 * is wedged); without this the awaiting UI hangs forever with no clue why.
 */
function withTimeout<T>(p: Promise<T>, ms: number, label: string): Promise<T> {
  return Promise.race([
    p,
    new Promise<T>((_, reject) => setTimeout(() => reject(new Error(`${label} timed out after ${ms}ms`)), ms)),
  ]);
}

async function ensurePermission(LN: LocalNotif): Promise<boolean> {
  try {
    let perm = await withTimeout(LN.checkPermissions(), 8000, 'checkPermissions');
    if (perm.display !== 'granted') perm = await withTimeout(LN.requestPermissions(), 30000, 'requestPermissions');
    return perm.display === 'granted';
  } catch {
    return false;
  }
}

// Ensure the channel the reminder will post to exists. Android freezes a
// channel's sound + importance at creation, so we both (a) reference the sound
// file by its FULL name including extension — `piano.mp3`, relative to res/raw,
// as the plugin documents — and (b) version the channel id (see CHANNEL_VERSION)
// so corrected settings actually take effect on devices that already had the
// old, broken channel. Returns the channel id the schedule should target.
async function ensureChannel(LN: LocalNotif, soundId: string, soundOn: boolean): Promise<string> {
  if (soundOn) {
    const label = NOTIFICATION_SOUNDS.find((s) => s.id === soundId)?.label ?? soundId;
    const id = soundChannelId(soundId);
    try {
      await LN.createChannel?.({
        id,
        name: `Reminder — ${label}`,
        description: 'Sempa task reminders',
        sound: `${soundId}.mp3`, // res/raw filename WITH extension — required to resolve
        importance: 5, // IMPORTANCE_HIGH — heads-up + sound
        visibility: 1,
        vibration: true,
      });
    } catch {
      /* already exists / unsupported */
    }
    return id;
  }
  // Sound off: still post to a HIGH-importance channel so the reminder shows as
  // a heads-up notification (just silent), rather than the default channel which
  // may be collapsed/low.
  try {
    await LN.createChannel?.({
      id: SILENT_CHANNEL_ID,
      name: 'Reminders (silent)',
      description: 'Sempa task reminders',
      importance: 4,
      visibility: 1,
      vibration: true,
    });
  } catch {
    /* already exists / unsupported */
  }
  return SILENT_CHANNEL_ID;
}

async function bindListeners(LN: LocalNotif) {
  if (listenersBound) return;
  listenersBound = true;
  try {
    await LN.registerActionTypes({
      types: [
        {
          id: 'REMINDER',
          actions: [
            { id: 'done', title: 'Mark done' },
            { id: 'snooze', title: 'Snooze 1h' },
          ],
        },
      ],
    });
    await LN.addListener('localNotificationActionPerformed', async (event) => {
      const extra = (event.notification.extra ?? {}) as { taskId?: string; url?: string };
      const taskId = extra.taskId;
      if (!taskId) return;
      // api.tasks on Capacitor writes the local DB and queues a sync, so these
      // work offline and reconcile with the server on reconnect.
      const { api } = await import('$lib/api');
      if (event.actionId === 'done') {
        await api.tasks.update(taskId, { status: 'done' }).catch(() => {});
        void syncLocalReminders();
      } else if (event.actionId === 'snooze') {
        const at = new Date(Date.now() + 60 * 60 * 1000).toISOString();
        await api.tasks.update(taskId, { remind_at: at }).catch(() => {});
        void syncLocalReminders();
      } else {
        // Body tap → deep-link into the app.
        navigate(extra.url || `/focus/${taskId}`);
      }
    });
  } catch {
    /* plugin without action support — basic notifications still work */
  }
}

/**
 * Reconcile scheduled OS alarms with the current local DB + settings.
 * Safe to call often; coalesces concurrent invocations.
 */
export async function syncLocalReminders(): Promise<void> {
  if (!isCapacitor()) return;
  if (running) {
    rerunQueued = true;
    return;
  }
  running = true;
  try {
    const LN = await loadPlugin();
    if (!LN) return;

    const st = notificationSettings.settings;
    const remindersOn = st.master_enabled; // master gate
    const soundOn = st.master_enabled && st.sound_enabled;
    const soundId = st.sound_id || DEFAULT_SOUND_ID;

    if (!(await ensurePermission(LN))) return;
    await bindListeners(LN);
    // Always post to one of our own channels (high importance) so reminders show
    // as heads-up notifications; pick the per-sound one when sound is enabled.
    const channelId = await ensureChannel(LN, soundId, soundOn);

    // After an app update Android has silently cancelled every alarm we'd
    // scheduled, while our diff map (below) still says they're armed. Detect the
    // version change and re-arm every future reminder regardless of the diff.
    const version = await appVersion();
    const forceReschedule = version !== '' && version !== localStorage.getItem(APP_VERSION_KEY);

    const prev = readMap();
    const next: ScheduleMap = {};
    const toSchedule: Parameters<LocalNotif['schedule']>[0]['notifications'] = [];
    const toCancel: { id: number }[] = [];

    if (remindersOn) {
      const tasks = await localApi.tasks.withReminders();
      const now = Date.now();
      for (const t of tasks) {
        if (!t.remind_at) continue;
        const when = new Date(t.remind_at).getTime();
        if (isNaN(when) || when <= now) continue; // past-due handled by server catch-up
        const notifId = notifIdFor(t.id);
        // soundId field doubles as the channel-change key, so store the channel
        // we'll actually post to — re-schedules if the user flips sound on/off.
        const entry: ScheduledEntry = { notifId, remindAt: t.remind_at, title: t.title, soundId: channelId };
        next[t.id] = entry;

        const unchanged =
          prev[t.id] &&
          prev[t.id].remindAt === entry.remindAt &&
          prev[t.id].title === entry.title &&
          prev[t.id].soundId === entry.soundId;
        if (unchanged && !forceReschedule) continue; // already scheduled correctly

        toSchedule.push({
          id: notifId,
          title: 'Reminder',
          body: t.title,
          schedule: { at: new Date(when), allowWhileIdle: true },
          channelId,
          actionTypeId: 'REMINDER',
          extra: { taskId: t.id, url: `/focus/${t.id}` },
        });
      }
    }

    // Cancel alarms for tasks that disappeared or whose reminder changed (the
    // changed ones are re-added above with the same id, which replaces them).
    for (const taskId of Object.keys(prev)) {
      if (!next[taskId]) toCancel.push({ id: prev[taskId].notifId });
    }

    if (toCancel.length) await LN.cancel({ notifications: toCancel }).catch(() => {});
    if (toSchedule.length) await LN.schedule({ notifications: toSchedule }).catch(() => {});
    writeMap(next);
    // Record the version we just (re)armed under, so the force-reschedule above
    // fires once per update rather than on every sync.
    if (version) localStorage.setItem(APP_VERSION_KEY, version);
  } catch {
    /* best-effort; the server push channel is the backup */
  } finally {
    running = false;
    if (rerunQueued) {
      rerunQueued = false;
      void syncLocalReminders();
    }
  }
}

/**
 * Post an OS notification for a reminder that is due RIGHT NOW. This is the
 * catch-up channel for Android: the in-app poll (routines.svelte.ts) calls this
 * the moment it detects a due reminder, so the user gets a real system-tray
 * notification + channel sound even when the pre-scheduled alarm never fired —
 * e.g. it was wiped by an app update, or the reminder came due while the app was
 * foregrounded (when only the in-app banner would otherwise show).
 *
 * It reuses the task's stable notifId and our high-importance channel, so if the
 * scheduled alarm *did* fire, posting with the same id collapses into it rather
 * than producing a duplicate. Best-effort and Capacitor-only; silent on failure.
 */
export async function fireImmediateReminder(task: { id: string; title: string }): Promise<void> {
  if (!isCapacitor()) return;
  try {
    const LN = await loadPlugin();
    if (!LN) return;
    if (!(await ensurePermission(LN))) return;
    await bindListeners(LN);

    const st = notificationSettings.settings;
    if (!st.master_enabled) return;
    const soundOn = st.master_enabled && st.sound_enabled;
    const soundId = st.sound_id || DEFAULT_SOUND_ID;
    const channelId = await ensureChannel(LN, soundId, soundOn);

    await LN.schedule({
      notifications: [
        {
          // Same id as the scheduled alarm → collapses instead of duplicating.
          id: notifIdFor(task.id),
          title: 'Reminder',
          body: task.title,
          channelId,
          actionTypeId: 'REMINDER',
          extra: { taskId: task.id, url: `/focus/${task.id}` },
        },
      ],
    });
  } catch {
    /* best-effort — the in-app banner still surfaced it */
  }
}

/**
 * Fire a real on-device notification a couple of seconds from now so the user
 * can verify the OS path (permission + channel + sound + heads-up) in isolation,
 * independent of reminder timing/sync. Returns a human-readable result for the
 * settings UI. Capacitor only.
 */
export async function sendTestReminder(): Promise<{ ok: boolean; message: string }> {
  if (!isCapacitor()) return { ok: false, message: 'Test notifications only run on the Android app.' };

  const LN = await loadPlugin();
  if (!LN) return { ok: false, message: 'Notification plugin not found on this device.' };

  // Each native call is bounded so a wedged bridge call surfaces the exact step
  // it stalled on instead of spinning forever.
  try {
    let perm = await withTimeout(LN.checkPermissions(), 8000, 'checkPermissions');
    if (perm.display !== 'granted') {
      perm = await withTimeout(LN.requestPermissions(), 30000, 'requestPermissions');
    }
    if (perm.display !== 'granted') {
      return { ok: false, message: `Notifications are blocked (status: ${perm.display}). Enable them for Sempa in Android settings.` };
    }
  } catch (e) {
    return { ok: false, message: `Permission step failed: ${e instanceof Error ? e.message : String(e)}` };
  }

  const st = notificationSettings.settings;
  const soundOn = st.master_enabled && st.sound_enabled;
  const soundId = st.sound_id || DEFAULT_SOUND_ID;

  let channelId = SILENT_CHANNEL_ID;
  try {
    channelId = await withTimeout(ensureChannel(LN, soundId, soundOn), 8000, 'createChannel');
  } catch (e) {
    return { ok: false, message: `Channel step failed: ${e instanceof Error ? e.message : String(e)}` };
  }

  try {
    await withTimeout(
      // No `schedule` field → fires immediately. This deliberately bypasses the
      // exact-alarm path so the test isolates plain notification display
      // (permission + channel) from alarm scheduling.
      LN.schedule({
        notifications: [
          {
            id: 2000000001,
            title: 'Sempa test reminder',
            body: soundOn ? 'If you can hear this, reminders are working.' : 'Reminders are working (sound is off).',
            channelId,
            extra: { url: '/settings/notifications' },
          },
        ],
      }),
      8000,
      'schedule',
    );
    return { ok: true, message: 'Test sent — it should appear now.' };
  } catch (e) {
    return { ok: false, message: `Schedule failed: ${e instanceof Error ? e.message : String(e)}` };
  }
}

/** Wire deep-link navigation and run an initial schedule. Capacitor only. */
export function initLocalReminders(nav: (url: string) => void): void {
  if (!isCapacitor()) return;
  navigate = nav;
  void syncLocalReminders();
}
