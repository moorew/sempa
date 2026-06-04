/**
 * Local SQLite database access via tauri-plugin-sql.
 * Provides the offline-first data layer — all reads/writes hit local SQLite
 * instantly, and mutations are queued in sync_log for background sync.
 */

import { isTauri } from './bridge';

interface SqlPlugin {
    load(path: string): Promise<Database>;
}

interface Database {
    execute(query: string, bindValues?: unknown[]): Promise<{ rowsAffected: number; lastInsertId: number }>;
    select<T = unknown[]>(query: string, bindValues?: unknown[]): Promise<T>;
}

let db: Database | null = null;

async function getDb(): Promise<Database> {
    if (db) return db;
    if (!isTauri()) throw new Error('Not running in Tauri');

    // Tauri v2: import the SQL plugin module
    const mod = await import('@tauri-apps/plugin-sql');
    const instance = await mod.default.load('sqlite:sempa.db');
    db = instance as unknown as Database;
    return db!;
}

// ── Sync log helpers ────────────────────────────────────────────────────────

export async function logMutation(
    entityType: string,
    entityId: string,
    action: 'create' | 'update' | 'delete',
    payload: Record<string, unknown>,
): Promise<void> {
    const d = await getDb();
    await d.execute(
        `INSERT INTO sync_log (entity_type, entity_id, action, payload) VALUES (?, ?, ?, ?)`,
        [entityType, entityId, action, JSON.stringify(payload)],
    );
}

export async function getPendingMutationCount(): Promise<number> {
    const d = await getDb();
    const rows = await d.select<{ count: number }[]>(
        `SELECT COUNT(*) as count FROM sync_log WHERE synced = 0`,
    );
    return rows[0]?.count ?? 0;
}

export async function markMutationsSynced(ids: number[]): Promise<void> {
    if (ids.length === 0) return;
    const d = await getDb();
    const placeholders = ids.map(() => '?').join(',');
    await d.execute(
        `UPDATE sync_log SET synced = 1 WHERE id IN (${placeholders})`,
        ids,
    );
}

// ── Generic query helpers ───────────────────────────────────────────────────

export async function query<T = unknown[]>(sql: string, params?: unknown[]): Promise<T> {
    const d = await getDb();
    return d.select<T>(sql, params);
}

export async function execute(
    sql: string,
    params?: unknown[],
): Promise<{ rowsAffected: number; lastInsertId: number }> {
    const d = await getDb();
    return d.execute(sql, params);
}
