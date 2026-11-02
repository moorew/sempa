/**
 * The device's local store must belong to one account at a time: a different
 * account (or server) signing in wipes it before any sync, and sign-out wipes it.
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { makeSqliteDriver, type TestDb } from '../test/sqlite-driver';
import { tauriSchemaSql } from '../test/schemas';
import { __setTestDriver } from '$lib/tauri/db';

(globalThis as any).window = globalThis.window ?? globalThis;
(globalThis as any).window.__TAURI__ = { core: { invoke: async () => {} }, event: {} };

const { bindLocalAccount, clearLocalAccount } = await import('./localAccount');

let db: TestDb;

function seed() {
  db.raw.exec(`INSERT INTO tasks (id, title, status, position, tags, created_at, updated_at)
               VALUES ('a-private', 'A PRIVATE', 'planned', 0, '[]', '2026-11-01', '2026-11-01')`);
  db.raw.exec(`INSERT INTO sync_log (entity_type, entity_id, action, payload, synced)
               VALUES ('task', 'a-offline', 'create', '{}', 0)`);
  db.raw.exec(`INSERT INTO sync_state (key, value) VALUES ('last_synced_at', '2026-11-01 08:00:00')`);
}
const count = (t: string) => (db.raw.prepare(`SELECT COUNT(*) AS n FROM ${t}`).get() as { n: number }).n;

beforeEach(() => {
  db = makeSqliteDriver();
  db.raw.exec(tauriSchemaSql());
  __setTestDriver(db.asDriver());
  localStorage.clear();
  localStorage.setItem('sempa_server_url', 'https://sempa.test');
  // Offline: the sign-out flush must not reach anything.
  vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('offline'); }));
});

describe('local account binding', () => {
  it('adopts existing data on first bind, keeps it for the same account', async () => {
    seed();
    await bindLocalAccount('user-a');
    await bindLocalAccount('user-a');
    expect(count('tasks')).toBe(1);
    expect(count('sync_log')).toBe(1);
  });

  it('wipes rows, outbox and cursor when another account signs in', async () => {
    seed();
    await bindLocalAccount('user-a');
    await bindLocalAccount('user-b');
    expect(count('tasks')).toBe(0);
    expect(count('sync_log')).toBe(0);
    expect(count('sync_state')).toBe(0);
  });

  it('wipes when the same account id comes from another server', async () => {
    seed();
    await bindLocalAccount('user-a');
    localStorage.setItem('sempa_server_url', 'https://other.test');
    await bindLocalAccount('user-a');
    expect(count('tasks')).toBe(0);
  });

  it('sign-out leaves nothing behind', async () => {
    seed();
    await bindLocalAccount('user-a');
    await clearLocalAccount();
    expect(count('tasks')).toBe(0);
    expect(count('sync_log')).toBe(0);
    expect(localStorage.getItem('sempa_local_account')).toBeNull();
  });
});
