// Watches whether backups are actually working, so a silent failure can't sit
// unnoticed for days.
//
// A backup that has stopped running is the worst thing to find out about late,
// and until now the only signals were the server log and a flag on the
// Settings → Backup page. This store feeds an app-wide banner instead.
//
// It polls `/backup/health`, which is a pure DB read of the last recorded run —
// NOT `/backup/drive`, which round-trips to Google on every call and would be
// abusive to poll.
import { api } from '$lib/api';
import type { BackupHealth } from '$lib/types';

// Backups run daily, so there is nothing to gain from tight polling.
const REFRESH_MS = 30 * 60 * 1000;

function createBackupHealth() {
  let data = $state<BackupHealth | null>(null);
  let dismissed = $state(false);
  let timer: ReturnType<typeof setInterval> | null = null;

  async function load() {
    try {
      data = await api.backup.health();
    } catch {
      // 403 for non-admin users, or offline / no server on a local-first client.
      // Either way there's nothing actionable to show, so stay silent.
      data = null;
    }
  }

  function start() {
    if (timer) return;
    void load();
    timer = setInterval(() => void load(), REFRESH_MS);
  }

  function stop() {
    if (timer) { clearInterval(timer); timer = null; }
  }

  return {
    get data() { return data; },
    /** Backups are enabled but the last run failed. */
    get failing() { return !!data && data.enabled && !data.ok; },
    /** The failure is an expired OAuth token — only the user can clear it. */
    get needsReconnect() { return !!data?.needs_reconnect; },
    /**
     * Dismissal is per-session only, on purpose: a broken backup shouldn't be
     * permanently hideable. It comes back next launch until it's actually fixed.
     */
    get dismissed() { return dismissed; },
    dismiss() { dismissed = true; },
    start,
    stop,
    refresh: load,
  };
}

export const backupHealth = createBackupHealth();
