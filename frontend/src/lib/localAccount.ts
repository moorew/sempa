/**
 * Binds the device's local-first store (one SQLite file per device) to the
 * account and server it was synced from.
 *
 * Without this, signing out and letting another household member sign in left
 * the first member's private rows readable, reused their sync cursor, and
 * replayed their unsynced edits under the new member's token.
 */
import { getServerUrl } from '$lib/api';
import { hasLocalDb, resetLocalStore, getPendingMutationCount } from '$lib/tauri/db';
import { stopSync, sync } from '$lib/sync.svelte';

const KEY = 'sempa_local_account';

function bindingFor(userId: string): string {
  return `${getServerUrl() || 'same-origin'}|${userId}`;
}

/**
 * Call once the server has confirmed who is signed in. If the local store was
 * last synced for a different account or server, wipe it before any sync runs.
 * A first run after upgrading (no binding yet) adopts the current data.
 */
export async function bindLocalAccount(userId: string | undefined | null): Promise<void> {
  if (!hasLocalDb() || !userId) return;
  const want = bindingFor(userId);
  let have: string | null = null;
  try { have = localStorage.getItem(KEY); } catch { /* storage unavailable */ }
  if (have && have !== want) {
    stopSync();
    await resetLocalStore();
  }
  try { localStorage.setItem(KEY, want); } catch { /* ignore */ }
}

/**
 * Sign-out half: push any queued edits while the departing user's token is
 * still valid (best effort, bounded), then wipe the local store so nothing of
 * theirs is left for the next person on this device.
 */
export async function clearLocalAccount(): Promise<void> {
  if (!hasLocalDb()) return;
  stopSync();
  try {
    if ((await getPendingMutationCount()) > 0) {
      await Promise.race([sync(), new Promise((r) => setTimeout(r, 5000))]);
    }
  } catch { /* offline — their unsynced edits are discarded with the store */ }
  await resetLocalStore();
  try { localStorage.removeItem(KEY); } catch { /* ignore */ }
}
