/**
 * Offline-first sync engine (shared by Tauri desktop and Capacitor Android).
 *
 * The local SQLite DB is the source of truth for the UI. Every local mutation is
 * also appended to the `sync_log` outbox. This engine reconciles local and
 * server state in two phases whenever the server is reachable:
 *
 *   1. PUSH — replay queued outbox mutations against the REST API, in order.
 *   2. PULL — fetch everything changed on the server since our cursor and apply
 *      it locally (last-write-wins by updated_at), including deletions.
 *
 * It runs on app start, on reconnect (online event / SSE reopen), after a local
 * write, and on a periodic timer. Concurrent runs are coalesced.
 */

import { isTauri } from './tauri/bridge';
import {
    getPendingMutations,
    markMutationsSynced,
    quarantineMutation,
    clearLocalTombstone,
    getPendingMutationCount,
    getSyncState,
    setSyncState,
    execute,
    query,
    type PendingMutation,
} from './tauri/db';

const CURSOR_KEY = 'changes_cursor';

// localStorage-backed config (same keys as $lib/api). Read directly to avoid a
// circular import (api → local-api → sync → api).
function getServerUrl(): string {
    return typeof localStorage !== 'undefined' ? (localStorage.getItem('sempa_server_url') ?? '') : '';
}
function getTauriToken(): string {
    return typeof localStorage !== 'undefined' ? (localStorage.getItem('sempa_tauri_token') ?? '') : '';
}
function getNativeToken(): string {
    return typeof localStorage !== 'undefined' ? (localStorage.getItem('sempa_native_token') ?? '') : '';
}

// ── Sync status store (Svelte 5 runes) ──────────────────────────────────────

export interface SyncState {
    online: boolean;
    syncing: boolean;
    pending: number;
    lastSyncedAt: string | null;
    lastError: string | null;
    // The server is reachable but rejecting our credentials (401/403) — the
    // session lapsed or was revoked. Distinct from `lastError` because it needs
    // a *different* remedy: no amount of retrying fixes it, the user has to sign
    // in again. Retries continue regardless so queued edits are never dropped.
    needsAuth: boolean;
}

function createSyncStore() {
    let online = $state(false);
    let syncing = $state(false);
    let pending = $state(0);
    let lastSyncedAt = $state<string | null>(null);
    let lastError = $state<string | null>(null);
    let needsAuth = $state(false);
    // Bumped whenever a pull writes ≥1 row to the local DB. The UI reads the
    // local DB once on mount, so without a reactive signal a freshly-pulled
    // dataset would stay invisible until a manual reload. Pages/layout watch
    // this to re-read after the initial (and every subsequent) pull lands.
    let revision = $state(0);

    return {
        get online() { return online; },
        get syncing() { return syncing; },
        get pending() { return pending; },
        get lastSyncedAt() { return lastSyncedAt; },
        get lastError() { return lastError; },
        get needsAuth() { return needsAuth; },
        get revision() { return revision; },
        _set(p: Partial<SyncState>) {
            if (p.online !== undefined) online = p.online;
            if (p.syncing !== undefined) syncing = p.syncing;
            if (p.pending !== undefined) pending = p.pending;
            if (p.lastSyncedAt !== undefined) lastSyncedAt = p.lastSyncedAt;
            if (p.lastError !== undefined) lastError = p.lastError;
            if (p.needsAuth !== undefined) needsAuth = p.needsAuth;
        },
        _bumpRevision() { revision += 1; },
    };
}

export const syncStore = createSyncStore();

// ── HTTP plumbing (talks to the server directly, bypassing the local-first api) ──

function authHeader(): Record<string, string> {
    const token = isTauri() ? getTauriToken() : getNativeToken();
    return token ? { Authorization: `Bearer ${token}` } : {};
}

async function serverFetch(path: string, init?: RequestInit): Promise<Response> {
    const base = getServerUrl();
    const token = isTauri() ? getTauriToken() : getNativeToken();
    const res = await fetch(`${base}${path}`, {
        ...init,
        headers: { 'Content-Type': 'application/json', ...authHeader(), ...(init?.headers ?? {}) },
        credentials: token ? 'omit' : 'include',
    });
    noteAuthOutcome(path, res);
    return res;
}

// Central place every authenticated call passes through, so a lapsed session is
// noticed wherever it first bites (push or pull) instead of only at one caller.
//
// /health is deliberately excluded: it's unauthenticated and answers 200 even
// when we're signed out — that's exactly why a dead session looked "online, but
// erroring" rather than "signed out" (reachable() only ever probed /health).
function noteAuthOutcome(path: string, res: Response): void {
    if (path.startsWith('/api/v1/health')) return;
    const failed = res.status === 401 || res.status === 403;
    if (failed) {
        if (!syncStore.needsAuth) syncStore._set({ needsAuth: true });
    } else if (res.ok && syncStore.needsAuth) {
        syncStore._set({ needsAuth: false });
    }
}

// ── Server change payload (mirror of db.SyncChanges) ─────────────────────────

interface Tombstone { entity_type: string; entity_id: string; deleted_at: string }
interface ServerChanges {
    tasks: Record<string, unknown>[];
    objectives: Record<string, unknown>[];
    plans: Record<string, unknown>[];
    tags: Record<string, unknown>[];
    week_reviews: Record<string, unknown>[];
    lists: Record<string, unknown>[];
    list_items: Record<string, unknown>[];
    deletions: Tombstone[];
    cursor: string;
}

// ── Push: replay the outbox ──────────────────────────────────────────────────

// Outcome of replaying one outbox entry:
//   'done'      — applied (or a harmless no-op like a 404 delete); mark synced.
//   'transient' — network/5xx/auth: keep queued, stop the pass, retry next cycle.
//   'permanent' — a 4xx the server will NEVER accept (bad payload): quarantine it
//                 so it can't wedge every later mutation (notably deletes).
type PushResult = 'done' | 'transient' | 'permanent';

// 4xx statuses that a plain retry can't fix — the payload itself is rejected.
// 401/403 (auth) and 5xx are treated as transient on purpose: they resolve
// without dropping the user's change.
const PERMANENT_STATUSES = new Set([400, 409, 413, 422]);

function classify(res: Response, action: string): PushResult {
    if (res.ok) return 'done';
    // 404 on update/delete = already gone server-side; a no-op, not a failure.
    if (res.status === 404 && action !== 'create') return 'done';
    if (PERMANENT_STATUSES.has(res.status)) return 'permanent';
    return 'transient';
}

// Maps an outbox entry to its REST call.
async function replay(m: PendingMutation): Promise<PushResult> {
    const payload = m.payload ? JSON.parse(m.payload) : {};
    // `shared` lives in local SQLite as an integer (0/1); offline-created tasks and
    // objectives log the full row, so the queued payload carries `shared: 0`. The
    // server field is a Go *bool and json.Decode 400s on a number — wedging the
    // outbox forever. Coerce to a real boolean so it pushes (this also recovers any
    // integer payloads already queued before this fix).
    if (typeof payload.shared === 'number') payload.shared = payload.shared !== 0;
    const path = restPath(m.entity_type);
    if (!path) return 'done'; // unknown entity → drop, don't wedge the queue

    try {
        let res: Response;
        if (m.action === 'create') {
            // Carry the client id so the server row shares it (idempotent).
            res = await serverFetch(path, { method: 'POST', body: JSON.stringify({ ...payload, id: m.entity_id }) });
        } else if (m.action === 'update') {
            res = await serverFetch(`${path}/${encodeURIComponent(m.entity_id)}`, {
                method: 'PATCH', body: JSON.stringify(payload),
            });
        } else {
            res = await serverFetch(`${path}/${encodeURIComponent(m.entity_id)}`, { method: 'DELETE' });
        }
        return classify(res, m.action);
    } catch {
        return 'transient'; // network error — keep queued
    }
}

// Plans use PUT /plans/{date} keyed by date, and reviews PUT /weeks/{ws}/review;
// both are handled by their own upsert paths rather than the generic replay.
function restPath(entityType: string): string | null {
    switch (entityType) {
        case 'tasks': return '/api/v1/tasks';
        case 'objectives': return '/api/v1/objectives';
        case 'tags': return '/api/v1/tags';
        case 'lists': return '/api/v1/lists';
        case 'plans': return '/api/v1/plans';        // create/update both PUT /{date}
        case 'week_reviews': return '/api/v1/weeks';  // PUT /{ws}/review
        default: return null;
    }
}

// Outbox entity_type (as logged by local-api) → the singular entity type used
// for local tombstones. Lets a confirmed delete-push clear its tombstone.
const SYNC_LOG_TO_TOMBSTONE: Record<string, string> = {
    tasks: 'task',
    objectives: 'objective',
    tags: 'tag',
    lists: 'list',
    list_items: 'list_item',
};

// Replays queued mutations in order. Returns human-readable descriptions of any
// mutations that were quarantined (dead-lettered) this pass, for surfacing.
async function pushOutbox(): Promise<string[]> {
    const pending = await getPendingMutations();
    const done: number[] = [];
    const quarantined: string[] = [];
    for (const m of pending) {
        let result: PushResult;
        if (m.entity_type === 'plans') {
            result = await replayPlan(m);
        } else if (m.entity_type === 'week_reviews') {
            result = await replayWeekReview(m);
        } else if (m.entity_type === 'list_items') {
            result = await replayListItem(m);
        } else {
            result = await replay(m);
        }

        if (result === 'transient') break; // preserve order — retry next cycle

        if (result === 'permanent') {
            // The server will never accept this payload. Dead-letter it so it
            // stops blocking every later mutation (this is what stranded deletes
            // on the server and made deleted tasks reappear on other devices).
            await quarantineMutation(m.id);
            quarantined.push(`${m.action} ${m.entity_type}/${m.entity_id}`);
            continue;
        }

        // Applied. A confirmed delete means the server is now authoritative and
        // won't re-send the row, so the local tombstone guard can be retired.
        if (m.action === 'delete') {
            const et = SYNC_LOG_TO_TOMBSTONE[m.entity_type];
            if (et) await clearLocalTombstone(et, m.entity_id);
        }
        done.push(m.id);
    }
    await markMutationsSynced(done);
    return quarantined;
}

// Plans are upserted by date (PUT /plans/{date}); the payload carries plan_date.
async function replayPlan(m: PendingMutation): Promise<PushResult> {
    const payload = m.payload ? JSON.parse(m.payload) : {};
    const date = payload.plan_date ?? m.entity_id;
    try {
        const res = await serverFetch(`/api/v1/plans/${encodeURIComponent(date)}`, {
            method: 'PUT', body: JSON.stringify(payload),
        });
        return classify(res, m.action);
    } catch { return 'transient'; }
}

// List items have a non-uniform REST shape: create is nested under the list
// (POST /lists/{list_id}/items), while update/delete use /list-items/{id}.
// Reorder is replayed as per-item position updates, so it rides the update path.
async function replayListItem(m: PendingMutation): Promise<PushResult> {
    const payload = m.payload ? JSON.parse(m.payload) : {};
    try {
        let res: Response;
        if (m.action === 'create') {
            const listId = payload.list_id;
            res = await serverFetch(`/api/v1/lists/${encodeURIComponent(listId)}/items`, {
                method: 'POST', body: JSON.stringify({ ...payload, id: m.entity_id }),
            });
        } else if (m.action === 'update') {
            res = await serverFetch(`/api/v1/list-items/${encodeURIComponent(m.entity_id)}`, {
                method: 'PATCH', body: JSON.stringify(payload),
            });
        } else {
            res = await serverFetch(`/api/v1/list-items/${encodeURIComponent(m.entity_id)}`, { method: 'DELETE' });
        }
        return classify(res, m.action);
    } catch { return 'transient'; }
}

// Week reviews are upserted by week_start (PUT /weeks/{ws}/review).
async function replayWeekReview(m: PendingMutation): Promise<PushResult> {
    const payload = m.payload ? JSON.parse(m.payload) : {};
    const ws = payload.week_start ?? m.entity_id;
    try {
        const res = await serverFetch(`/api/v1/weeks/${encodeURIComponent(ws)}/review`, {
            method: 'PUT', body: JSON.stringify(payload),
        });
        return classify(res, m.action);
    } catch { return 'transient'; }
}

// ── Pull: apply server changes locally ───────────────────────────────────────

// Order tasks so that any task referenced by another (as parent_task_id or
// recurrence_origin_id) is inserted first. The tasks table has self-referential
// FKs, so a child arriving before its parent fails the FK constraint. A simple
// stable topological pass over in-batch references is enough (chains are short);
// anything still unresolved (e.g. a parent not in this batch) is appended and
// caught per-row below.
function orderTasksByDependency(tasks: Record<string, unknown>[]): Record<string, unknown>[] {
    const ids = new Set(tasks.map(t => t.id as string));
    const byId = new Map(tasks.map(t => [t.id as string, t]));
    const ordered: Record<string, unknown>[] = [];
    const placed = new Set<string>();

    const visit = (t: Record<string, unknown>, stack: Set<string>) => {
        const id = t.id as string;
        if (placed.has(id) || stack.has(id)) return;
        stack.add(id);
        for (const refCol of ['parent_task_id', 'recurrence_origin_id'] as const) {
            const ref = t[refCol] as string | null;
            if (ref && ref !== id && ids.has(ref) && !placed.has(ref)) {
                visit(byId.get(ref)!, stack);
            }
        }
        stack.delete(id);
        if (!placed.has(id)) { placed.add(id); ordered.push(t); }
    };

    for (const t of tasks) visit(t, new Set());
    return ordered;
}

async function pullChanges(): Promise<void> {
    const since = (await getSyncState(CURSOR_KEY)) ?? '';
    const res = await serverFetch(`/api/v1/sync/changes?since=${encodeURIComponent(since)}`);
    if (res.status === 401 || res.status === 403) {
        throw new Error('Your session has expired. Sign in again to resume syncing — nothing queued is lost.');
    }
    if (!res.ok) throw new Error(`pull failed: ${res.status}`);
    const changes: ServerChanges = await res.json();

    // Apply parents-before-children: objectives (and tags/plans/reviews) must
    // exist before tasks that reference them, and parent tasks before child
    // tasks. Each row is upserted defensively — a single FK failure (e.g. a row
    // referencing something not in this batch) must skip that row, NOT abort the
    // whole pull and leave the app empty. That silent total-abort was the
    // "synced but empty" bug; surfaced as a 787 FOREIGN KEY error in the UI.
    let applied = 0;
    let failed = 0;
    const tryUpsert = async (fn: () => Promise<boolean>): Promise<void> => {
        try { if (await fn()) applied += 1; }
        catch (e) { failed += 1; console.error('sync upsert skipped a row:', e); }
    };

    for (const o of changes.objectives) await tryUpsert(() => upsertObjective(o));
    for (const tag of changes.tags) await tryUpsert(() => upsertTag(tag));
    for (const p of changes.plans) await tryUpsert(() => upsertPlan(p));
    for (const r of changes.week_reviews) await tryUpsert(() => upsertWeekReview(r));
    for (const t of orderTasksByDependency(changes.tasks)) await tryUpsert(() => upsertTask(t));
    // Lists before their items (FK), and both can reference tasks (already applied).
    for (const l of changes.lists ?? []) await tryUpsert(() => upsertList(l));
    for (const it of changes.list_items ?? []) await tryUpsert(() => upsertListItem(it));
    for (const d of changes.deletions) await tryUpsert(() => applyDeletion(d));

    // Anything actually changed locally → tell the UI to re-read. Do this BEFORE
    // any partial-failure throw below, so the rows that DID apply show up even
    // when a few couldn't.
    if (applied > 0) syncStore._bumpRevision();

    // Only advance the cursor when everything applied. If some rows failed we
    // keep the old cursor so the next pull retries them (e.g. a parent that
    // arrives in a later batch), rather than skipping them forever.
    if (failed === 0 && changes.cursor) {
        await setSyncState(CURSOR_KEY, changes.cursor);
    } else if (failed > 0) {
        const total = changes.tasks.length + changes.objectives.length + changes.plans.length
            + changes.tags.length + changes.week_reviews.length
            + (changes.lists?.length ?? 0) + (changes.list_items?.length ?? 0);
        throw new Error(`Applied ${applied}, skipped ${failed} of ${total} changes (will retry next sync)`);
    }
}

// Last-write-wins guard: returns true only when the incoming row should be
// written, i.e. there's no local row or the incoming updated_at is strictly
// newer. This protects a not-yet-pushed local edit from being clobbered by a
// stale server copy (the local edit wins on the next push).
//
// keyCol matters: tasks/objectives/tags share their id with the server (client
// ids), but daily_plans and week_reviews are upserted server-side by a natural
// key (plan_date / week_start) and get a *different* server id, so they must be
// matched on that natural key — both here and in the ON CONFLICT target below.
async function lww(table: string, row: Record<string, unknown>, keyCol = 'id'): Promise<boolean> {
    const key = row[keyCol] as string;

    // Tombstone guard: a row deleted locally must not be resurrected by a stale
    // server copy re-sent on pull (e.g. before our delete has pushed, or while
    // the outbox is wedged). Only id-keyed tables carry tombstones. If the
    // incoming row is genuinely NEWER than the delete, it's a legitimate
    // re-creation — retire the stale tombstone and let it through.
    if (keyCol === 'id') {
        const entityType = ENTITY_TYPE_BY_TABLE[table];
        if (entityType) {
            const tomb = await query<{ deleted_at: string }[]>(
                `SELECT deleted_at FROM sync_tombstones WHERE entity_type = ? AND entity_id = ?`,
                [entityType, key],
            );
            if (tomb.length > 0) {
                const remoteTs = (row.updated_at as string) ?? '';
                if (remoteTs <= tomb[0].deleted_at) return false; // local delete wins
                await execute(`DELETE FROM sync_tombstones WHERE entity_type = ? AND entity_id = ?`,
                    [entityType, key]);
            }
        }
    }

    const existing = await query<{ updated_at: string }[]>(
        `SELECT updated_at FROM ${table} WHERE ${keyCol} = ?`, [key],
    );
    if (existing.length > 0) {
        const localTs = existing[0].updated_at ?? '';
        const remoteTs = (row.updated_at as string) ?? '';
        if (remoteTs <= localTs) return false; // local is same-or-newer → keep it
    }
    return true;
}

async function upsertTask(t: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('tasks', t))) return false;
    await execute(
        `INSERT INTO tasks (id, title, description, planned_date, week_start, status, position,
            time_estimate_minutes, time_actual_minutes, parent_task_id, weekly_objective_id,
            source, source_id, source_url, source_metadata, completed_at, archived_at,
            created_at, updated_at, tags, recurrence_rule, recurrence_origin_id, is_customized,
            scheduled_start, scheduled_end, roughly_at, remind_at, shared)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
         ON CONFLICT(id) DO UPDATE SET
            title=excluded.title, description=excluded.description, planned_date=excluded.planned_date,
            week_start=excluded.week_start, status=excluded.status, position=excluded.position,
            time_estimate_minutes=excluded.time_estimate_minutes, time_actual_minutes=excluded.time_actual_minutes,
            parent_task_id=excluded.parent_task_id, weekly_objective_id=excluded.weekly_objective_id,
            source=excluded.source, source_id=excluded.source_id, source_url=excluded.source_url,
            source_metadata=excluded.source_metadata, completed_at=excluded.completed_at,
            archived_at=excluded.archived_at, updated_at=excluded.updated_at, tags=excluded.tags,
            recurrence_rule=excluded.recurrence_rule, recurrence_origin_id=excluded.recurrence_origin_id,
            is_customized=excluded.is_customized, scheduled_start=excluded.scheduled_start,
            scheduled_end=excluded.scheduled_end, roughly_at=excluded.roughly_at,
            remind_at=excluded.remind_at, shared=excluded.shared`,
        [
            t.id, t.title, t.description ?? null, t.planned_date ?? null, t.week_start ?? null,
            t.status, t.position ?? 0, t.time_estimate_minutes ?? null, t.time_actual_minutes ?? null,
            t.parent_task_id ?? null, t.weekly_objective_id ?? null, t.source ?? null, t.source_id ?? null,
            t.source_url ?? null, t.source_metadata ?? null, t.completed_at ?? null, t.archived_at ?? null,
            t.created_at ?? null, t.updated_at ?? null, JSON.stringify(t.tags ?? []),
            t.recurrence_rule ?? null, t.recurrence_origin_id ?? null, t.is_customized ? 1 : 0,
            t.scheduled_start ?? null, t.scheduled_end ?? null, t.roughly_at ?? null, t.remind_at ?? null,
            t.shared ? 1 : 0,
        ],
    );
    return true;
}

async function upsertList(l: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('lists', l))) return false;
    await execute(
        `INSERT INTO lists (id, name, task_id, position, archived_at, archive_on_complete, created_at, updated_at, shared)
         VALUES (?,?,?,?,?,?,?,?,?)
         ON CONFLICT(id) DO UPDATE SET
            name=excluded.name, task_id=excluded.task_id, position=excluded.position,
            archived_at=excluded.archived_at, archive_on_complete=excluded.archive_on_complete,
            updated_at=excluded.updated_at, shared=excluded.shared`,
        [
            l.id, l.name ?? '', l.task_id ?? null, l.position ?? 0, l.archived_at ?? null,
            l.archive_on_complete ? 1 : 0, l.created_at ?? null, l.updated_at ?? null, l.shared ? 1 : 0,
        ],
    );
    return true;
}

async function upsertListItem(it: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('list_items', it))) return false;
    await execute(
        `INSERT INTO list_items (id, list_id, text, position, done, category, created_at, updated_at, shared)
         VALUES (?,?,?,?,?,?,?,?,?)
         ON CONFLICT(id) DO UPDATE SET
            list_id=excluded.list_id, text=excluded.text, position=excluded.position,
            done=excluded.done, category=excluded.category, updated_at=excluded.updated_at, shared=excluded.shared`,
        [
            it.id, it.list_id, it.text ?? '', it.position ?? 0, it.done ? 1 : 0,
            it.category ?? null, it.created_at ?? null, it.updated_at ?? null, it.shared ? 1 : 0,
        ],
    );
    return true;
}

async function upsertObjective(o: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('weekly_objectives', o))) return false;
    await execute(
        `INSERT INTO weekly_objectives (id, week_start, title, description, status, position, created_at, updated_at, shared)
         VALUES (?,?,?,?,?,?,?,?,?)
         ON CONFLICT(id) DO UPDATE SET
            week_start=excluded.week_start, title=excluded.title, description=excluded.description,
            status=excluded.status, position=excluded.position, updated_at=excluded.updated_at, shared=excluded.shared`,
        [o.id, o.week_start, o.title, o.description ?? null, o.status ?? 'active', o.position ?? 0,
         o.created_at ?? null, o.updated_at ?? null, o.shared ? 1 : 0],
    );
    return true;
}

async function upsertPlan(p: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('daily_plans', p, 'plan_date'))) return false;
    // Conflict on plan_date (the natural key): the server row's id differs from
    // any local id, so id-based upsert would hit the UNIQUE(plan_date) constraint.
    await execute(
        `INSERT INTO daily_plans (id, plan_date, status, intention, reflection, wins, shutdown_at, created_at, updated_at)
         VALUES (?,?,?,?,?,?,?,?,?)
         ON CONFLICT(plan_date) DO UPDATE SET
            id=excluded.id, status=excluded.status, intention=excluded.intention,
            reflection=excluded.reflection, wins=excluded.wins, shutdown_at=excluded.shutdown_at,
            updated_at=excluded.updated_at`,
        [p.id, p.plan_date, p.status ?? 'pending', p.intention ?? null, p.reflection ?? null,
         p.wins ?? null, p.shutdown_at ?? null, p.created_at ?? null, p.updated_at ?? null],
    );
    return true;
}

async function upsertTag(tag: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('tag_definitions', tag))) return false;
    await execute(
        `INSERT INTO tag_definitions (id, name, color, created_at, updated_at)
         VALUES (?,?,?,?,?)
         ON CONFLICT(id) DO UPDATE SET
            name=excluded.name, color=excluded.color, updated_at=excluded.updated_at`,
        [tag.id, tag.name, tag.color ?? '#6366f1', tag.created_at ?? null, tag.updated_at ?? null],
    );
    return true;
}

async function upsertWeekReview(r: Record<string, unknown>): Promise<boolean> {
    if (!(await lww('week_reviews', r, 'week_start'))) return false;
    // Conflict on week_start (the natural key) — same reasoning as plans.
    await execute(
        `INSERT INTO week_reviews (id, week_start, wins, challenges, next_focus, created_at, updated_at)
         VALUES (?,?,?,?,?,?,?)
         ON CONFLICT(week_start) DO UPDATE SET
            id=excluded.id, wins=excluded.wins, challenges=excluded.challenges,
            next_focus=excluded.next_focus, updated_at=excluded.updated_at`,
        [r.id, r.week_start, r.wins ?? null, r.challenges ?? null, r.next_focus ?? null,
         r.created_at ?? null, r.updated_at ?? null],
    );
    return true;
}

const TOMBSTONE_TABLE: Record<string, string> = {
    task: 'tasks',
    objective: 'weekly_objectives',
    plan: 'daily_plans',
    tag: 'tag_definitions',
    week_review: 'week_reviews',
    list: 'lists',
    list_item: 'list_items',
};

// Inverse of TOMBSTONE_TABLE (local table name → tombstone entity type), used by
// lww() to look up whether a pulled row was deleted locally.
const ENTITY_TYPE_BY_TABLE: Record<string, string> = Object.fromEntries(
    Object.entries(TOMBSTONE_TABLE).map(([entityType, table]) => [table, entityType]),
);

async function applyDeletion(d: Tombstone): Promise<boolean> {
    const table = TOMBSTONE_TABLE[d.entity_type];
    if (!table) return false;
    const res = await execute(`DELETE FROM ${table} WHERE id = ?`, [d.entity_id]);
    // The server confirmed this delete — retire any local tombstone for it.
    await clearLocalTombstone(d.entity_type, d.entity_id);
    return (res?.rowsAffected ?? 0) > 0;
}

// ── Orchestration ────────────────────────────────────────────────────────────

let running = false;
let queued = false;

async function reachable(): Promise<boolean> {
    if (!getServerUrl()) return false;
    try {
        const res = await serverFetch('/api/v1/health');
        return res.ok;
    } catch {
        return false;
    }
}

/**
 * Self-heal: drop local recurring instances the server no longer has — orphans
 * stranded by pre-tombstone recurrence deletes (phantom duplicate recurring tasks).
 *
 * Why this is safe and not over-aggressive:
 *  • Only rows with recurrence_origin_id are ever considered. Recurring instances
 *    are ALWAYS server-generated and synced down — the client never creates them
 *    — so an offline-created task (origin = null) is structurally never a
 *    candidate. (This is the guarantee against deleting unsynced offline work.)
 *  • Aborts on ANY fetch failure or non-array response — never reconciles against
 *    a missing/partial set (the "server returned [] so I deleted everything" trap).
 *  • Per-(origin,date) bucket: only reconciles a bucket the server actually
 *    reports on, so a future day it hasn't generated yet is left untouched.
 *  • Never deletes an id with a pending outbound mutation (offline edit in flight).
 */
async function reconcileRecurringInstances(): Promise<void> {
    let res: Response;
    try {
        res = await serverFetch('/api/v1/tasks/recurring/instances');
    } catch {
        return; // offline / unreachable — do nothing
    }
    if (!res.ok) return;
    let server: { id: string; origin: string; date: string }[];
    try { server = await res.json(); } catch { return; }
    if (!Array.isArray(server)) return;

    // Authoritative allowed-id set per (origin|date).
    const buckets = new Map<string, Set<string>>();
    for (const r of server) {
        const key = `${r.origin}|${r.date}`;
        let set = buckets.get(key);
        if (!set) { set = new Set(); buckets.set(key, set); }
        set.add(r.id);
    }

    const local = await query<{ id: string; recurrence_origin_id: string; planned_date: string | null }[]>(
        `SELECT id, recurrence_origin_id, planned_date FROM tasks
         WHERE recurrence_origin_id IS NOT NULL AND status != 'cancelled'`);
    const pending = new Set(
        (await query<{ entity_id: string }[]>(
            `SELECT entity_id FROM sync_log WHERE entity_type = 'tasks' AND synced = 0`)).map((r) => r.entity_id));

    const orphans: string[] = [];
    for (const t of local) {
        if (!t.planned_date) continue;
        const allowed = buckets.get(`${t.recurrence_origin_id}|${t.planned_date}`);
        if (!allowed) continue;           // server hasn't spoken about this bucket
        if (allowed.has(t.id)) continue;  // legit, keep
        if (pending.has(t.id)) continue;  // pending upload — hands off
        orphans.push(t.id);
    }
    for (const id of orphans) {
        await execute(`DELETE FROM tasks WHERE id = ?`, [id]);
    }
    if (orphans.length) syncStore._bumpRevision();
}

/**
 * Run one full sync cycle (push then pull). Safe to call often: concurrent calls
 * are coalesced into a single trailing run.
 */
export async function sync(): Promise<void> {
    if (running) { queued = true; return; }
    running = true;
    try {
        syncStore._set({ pending: await getPendingMutationCount() });

        const online = await reachable();
        syncStore._set({ online });
        if (!online) return;

        syncStore._set({ syncing: true, lastError: null });
        const quarantined = await pushOutbox();
        await pullChanges();
        await reconcileRecurringInstances();
        syncStore._set({
            lastSyncedAt: new Date().toISOString(),
            pending: await getPendingMutationCount(),
        });
        // Surface any dead-lettered mutations (pull didn't throw if we got here).
        if (quarantined.length > 0) {
            syncStore._set({
                lastError: `Skipped ${quarantined.length} change(s) the server rejected: ${quarantined.join(', ')}`,
            });
        }
    } catch (e) {
        syncStore._set({ lastError: e instanceof Error ? e.message : String(e) });
    } finally {
        syncStore._set({ syncing: false });
        running = false;
        if (queued) { queued = false; void sync(); }
    }
}

let interval: ReturnType<typeof setInterval> | null = null;
let onlineHandler: (() => void) | null = null;

/** Start background sync: initial cycle, on reconnect, and every 30s. */
export function startSync(): void {
    if (interval) return; // already started
    void sync();
    interval = setInterval(() => void sync(), 30_000);
    onlineHandler = () => void sync();
    if (typeof window !== 'undefined') window.addEventListener('online', onlineHandler);
}

export function stopSync(): void {
    if (interval) { clearInterval(interval); interval = null; }
    if (onlineHandler && typeof window !== 'undefined') {
        window.removeEventListener('online', onlineHandler);
        onlineHandler = null;
    }
}

/** Nudge a sync shortly after a local write so changes propagate promptly. */
let flushTimer: ReturnType<typeof setTimeout> | null = null;
export function flushSoon(): void {
    if (flushTimer) return;
    flushTimer = setTimeout(() => { flushTimer = null; void sync(); }, 800);
}
