# Sempa — Builder's Guide

> **Read this first.** This is the starting signpost for anyone — human or AI —
> working on Sempa. It captures the architecture, conventions, and the hard-won
> gotchas that aren't obvious from the code alone. Skim the whole thing once;
> it will save you from re-discovering bugs we've already paid for.
>
> **Maintainers:** operational/security runbook (deploy host, secrets, release
> overrides, keystore) lives in the **private** `sempa-ops` repo (`BUILDERS.md`),
> not here — this repo is public.

---

## What Sempa is

A self-hosted, single-user task manager inspired by Sunsama. Philosophy:
eliminate distractions, plan with intention, unify task sources. Core surfaces:
guided daily planning, a daily Kanban, weekly objectives, timeboxing + Pomodoro,
time tracking + insights, standalone Lists, and a daily shutdown ritual.

- **App name is "Sempa"** throughout code and branding (the repo/dir is "aura"/
  "sempa" interchangeably; prefer Sempa in user-facing text).
- Brand: Plus Jakarta Sans, burnt-terracotta accent `#b3592e`, warm cream/ink
  backgrounds via `--sempa-*` CSS tokens. Terracotta is the default accent preset.

## Tech stack

| Layer | Choice |
|---|---|
| Backend | Go — Chi router, sqlc, golang-migrate |
| Database | SQLite (WAL mode); a PostgreSQL upgrade path is intended |
| Frontend | SvelteKit + TailwindCSS, **Svelte 5 runes** |
| Desktop | Tauri v2 (Rust shell, `frontend/src-tauri`) — Windows + Linux (Flatpak/AppImage/deb/rpm). No macOS build is shipped. |
| Dock | Tauri on Raspberry Pi (aarch64) — the Sempa Dock |
| Android | Capacitor (`frontend/android`) |
| Transport | REST + SSE for realtime |
| Packaging | Docker Compose for self-hosting |

Why: Go gives a tiny image + low RAM; SvelteKit gives small bundles; SQLite WAL
is plenty for single-user with a clean upgrade path. API-first so clients
(desktop/Android) can be added without a rewrite.

## Where things live

```
backend/internal/
  api/         HTTP handlers (Chi). auth.go, events.go (SSE), search.go, …
  db/          sqlc queries + stores. recurrence.go, search.go, …
  poller/      background loops (recurrence horizon, backup scheduler)
  notify/      notification engine (web push, FCM, webhook)
  integrations/ gmail, fastmail (caldav/jmap), jira, unfurl
  backup/      backup/restore engine (zip + AES-GCM + destinations)
frontend/src/
  lib/api.ts           api resolver: httpApi (server) vs localApi (offline)
  lib/tauri/           local-first SQLite: db.ts, local-api.ts, schema.ts
  lib/stores/*.svelte.ts  runes stores (realtime, sync, routines, prefs, …)
  lib/components/       TaskPanel, TaskCard, BottomSheet, calendar, …
  routes/              SvelteKit pages
  src-tauri/           Tauri (Rust) desktop shell + windows.rs
deploy/                install.sh, update.sh (self-host)
docs/                  UPDATER.md, JIRA.md
```

## Architecture essentials

- **Two API backends behind one interface (`lib/api.ts`).** When a server URL is
  configured, the app uses `httpApi` (REST + Bearer/cookie auth). With no server
  (Tauri/Capacitor offline), it falls back to `localApi` reading the device's
  local SQLite. `resolveApi()` switches between them; `LOCAL_CORE` lists the
  tables served locally.
- **Auth** accepts three credentials: session cookie, `Authorization: Bearer`,
  or `?token=` query (the last is for `EventSource`, which can't set headers).
  Sessions are **DB-backed** (table `sessions`) — they must survive a restart
  (see gotchas).
- **Realtime sync (<1s).** Backend SSE hub at `GET /api/v1/events`; every
  mutation broadcasts a `change` event. Frontend `stores/realtime.svelte.ts`
  reconnects with backoff; pages re-fetch via `$effect(() => realtime.lastEvent)`.
- **Local-first clients don't run server logic.** Tauri/Capacitor read rows
  straight from local SQLite. Anything computed server-side reaches them **only
  if it's materialized into rows and synced down** — never lazy-on-read. See the
  recurrence-horizon gotcha.

## Local development & how to validate

The dev/prod host has a limited toolchain. Know what you can actually run:

- **Go 1.25** — build/test/`govulncheck` all work locally (`GOTOOLCHAIN=auto`
  fetches the `go.mod` version).
- **Docker** — full `docker build` works (validate Dockerfile/base-image bumps).
- **Node 20** — `npm run build` and `npm run check` (svelte-check) work and are
  the reliable frontend validation. **The vitest suite does NOT run locally**
  (`frontend/src/lib/sync.test.ts` needs Node 22's `node:sqlite`); CI runs Node 22.
- **No Rust/cargo** — you **cannot** build or type-check the Tauri desktop or
  `windows.rs` locally. Rust changes are validated only by the Windows release
  workflow in CI. Plan accordingly: write Rust carefully, lean on CI.

> Rule of thumb: validate Go + Docker + `npm run check`/`build` locally; trust CI
> for the frontend test suite and anything Tauri/Rust or Android.

## Versioning & releases

**Semantic versioning (adopted 2026-06-17):**

- New user-facing **feature** (`feat:`) → **minor** bump, reset patch to `0`.
- **Fix / chore / ci / docs / tweak** → **patch** bump.
- **Breaking change** (a data/schema migration users must act on, a removed or
  changed API, a new install requirement) → **major** bump — flag it and confirm
  with a maintainer first.

History before this was just an incrementing counter (`1.0.112 → 1.0.151`); from
`1.1.0` onward the number is meant to carry meaning. No retroactive renumbering.

**Release flow** (the version string lives in 5 spots, keep them in lockstep):
`frontend/package.json`, `frontend/package-lock.json` (×2), `tauri.conf.json`,
`src-tauri/Cargo.toml`, and the `sempa` package in `src-tauri/Cargo.lock` (not the
coincidental `serde_json` at the same number). Then commit, push `main`, and push
a lightweight `vX.Y.Z` tag — that triggers the Windows, Android, and GHCR image
release workflows. `frontend/android/app/build.gradle` is *not* bumped (CI derives
the Android version from the tag). The self-hosted Docker container serves both
the Go backend **and** the web app, so a redeploy is needed for nearly any
backend or web change. (Maintainer specifics — deploy host, redeploy command,
CI overrides — are in the private `sempa-ops` guide.)

## Conventions

- **Match the surrounding code.** Comment density, naming, and idiom vary by file;
  follow the file you're in.
- New **DB columns**: add the migration *and* mirror the column into the
  local-first schema in **all** of: `lib/tauri/schema.ts`, `lib/tauri/db.ts`
  `COLUMN_RECONCILE`, `src-tauri/src/db.rs` migration, and `sync.svelte.ts`
  `upsertTask`. Miss one and Android upgrades break silently (see gotchas).
- New **synced entity/table** (e.g. `lists`/`list_items`): the same mirror, wider.
  Wire **all** of: backend migration + store + handlers/routes + `db/sync.go`
  `SyncChanges` (collect rows) + delete handlers recording tombstones; then
  client `schema.ts` CREATE TABLE, `src-tauri/src/db.rs` migration, `local-api.ts`
  namespace (CRUD + `logMutation` + `flushSoon`), `sync.svelte.ts` (`ServerChanges`
  fields, `upsertX` lww, pull-apply order parents-before-children, `TOMBSTONE_TABLE`,
  `restPath`/custom replay for non-uniform REST), and add the namespace to
  `LOCAL_CORE` in `lib/api.ts`. Make create accept a client id (`clientOrNewID`)
  so offline-created rows keep their id on sync. Lists are the worked example.
- **Backend background work** goes through the `poller` pattern (startup +
  interval, idempotent), not ad-hoc goroutines.
- **Notification channels (Android)** are immutable once created — bump the
  channel id (`rem_<sound>_v2`) to change sound/importance, and keep the id in
  sync across `localReminders.ts`, `push.ts`, and backend `fcm.go`.
- **Tags in list cards** render as colour **dots only** (names via tooltip);
  full chips only in the detail/edit views. Don't reintroduce labels in cards.

## Hard-won gotchas

These are real bugs that cost real time. Check here before debugging anything in
the same area.

### Frontend / Svelte 5
- **`$effect` that read-modify-writes the same `$state` loops forever** →
  `effect_update_depth_exceeded`, which **wedges the entire app** (all clicks/
  typing/DnD die, not just one component). Even a store method called inside the
  effect is tracked. Fix: wrap the mutation in `untrack(() => …)`. When the
  whole UI goes inert, check the console for this error *first*.
- **`position:fixed` drawer/modal gets cut off** if any ancestor has a non-`none`
  `transform` (it becomes the containing block, not the viewport). A `transform`
  keyframe with `animation-fill-mode: both` computes to `matrix(...)`, not `none`
  — enough to trigger it. Page wrappers that contain fixed children must use an
  **opacity-only** entrance (`.animate-page-in`); keep transform animations on
  leaf elements only.
- **Flex collapse in max-height columns:** use `flex-[1_1_auto] min-h-0`, not
  Tailwind `flex-1` (= `flex:1 1 0`) — a `flex-basis:0` child in an
  indefinite-height flex column collapses to zero height in Chromium.

### Mobile bottom sheets
- Use the shared `dismissibleSheet` action (`lib/actions/sheet.ts`): mark the
  handle `data-sheet-handle` and scroll area `data-sheet-scroll` so dismiss-drag
  doesn't fight inner scrolling. Don't bind `transform`/`transition` in markup —
  the action owns them.
- **Anchor critical actions (Save/Cancel) at the TOP**, not a bottom footer. On
  Android WebView the bottom of a full-height fixed sheet lands behind the
  keyboard; we burned several releases trying to size it before moving actions to
  a top action bar. Don't size sheets off `visualViewport` JS values — they get
  stuck on keyboard dismiss. Let `adjustResize` + a viewport-anchored fixed
  height do it.

### Local-first (Tauri/Capacitor)
- **Capacitor (Android) has no migration runner.** Local schema is all
  `CREATE TABLE IF NOT EXISTS`, which can't add a column to an existing table. A
  missing column breaks **both** task-save and sync at once (looks like two
  unrelated bugs; sync stalls permanently because the cursor never advances).
  `reconcileColumns()` ALTERs missing columns in on open — keep `COLUMN_RECONCILE`
  in sync with `schema.ts`.
- **Recurrence/derived data must be materialized proactively.** A poller
  (`poller/recurrence.go`, `GenerateHorizon`, +2 weeks) writes instances as rows
  so they sync down. Don't reimplement server logic in `local-api.ts`.
- **HTML5 drag-and-drop silently dies in Tauri** because the webview defaults
  `dragDropEnabled: true` (OS file-drop intercepts it). Set
  `"dragDropEnabled": false` on the main window in `tauri.conf.json`.

### Desktop (Tauri / Windows)
- **You can't compile Rust on the dev host.** Windows CI is the only validation.
- **Grey box around a chromeless popup** = the transparent window's OS backing
  showing through gaps. The robust fix is an **opaque** window
  (`transparent(false)`) painted edge-to-edge and resized to exact content
  height — not just `border-radius: 0`. Don't re-introduce `transparent(true)`.
- **Reminders** use 3 independent surfaces (native OS toast, in-app banner,
  floating card) so at least one always fires; the native toast is the reliable
  background channel on Windows.

### Linux (Tauri / packaging)
- **App id is per-platform.** Linux/Flathub uses `ca.sempa.Sempa` (permanent
  once on Flathub; we own sempa.ca), set via a Linux-only overlay
  `frontend/src-tauri/tauri.linux.conf.json` that Tauri deep-merges over
  `tauri.conf.json` on Linux builds only. Windows keeps `com.sempa.desktop`
  + the nsis/msi targets. **Don't change the Flathub id once submitted.** Dock
  id: `ca.sempa.Dock`.
- **Tauri config is strict JSON — no comments.** A `"//"` key fails the build
  with `Additional properties are not allowed ('//' was unexpected)`. Keep notes
  in `frontend/src-tauri/linux/README.md`, not inline in the config.
- **AppImage in CI (ubuntu-24.04):** linuxdeploy/appimagetool ship as FUSE2
  AppImages and the runner has no FUSE2 — set `APPIMAGE_EXTRACT_AND_RUN=1` +
  `NO_STRIP=true` and install `libfuse2t64`. deb/rpm build without this. Tauri
  swallows linuxdeploy stderr — build with `tauri build --verbose` to see the
  real error.
- **Reading the system window-button layout:** the custom titlebar mirrors
  `gtk-decoration-layout` via a Linux-only Tauri command
  (`window_decoration_layout` in `commands.rs`). It needs the trait
  `gtk::prelude::GtkSettingsExt` (NOT `SettingsExt`), the `gtk = "0.18"` dep
  pinned to match the wry/tao tree, and GTK calls must hop to the main thread
  (`app.run_on_main_thread`).
- **Self-hosted fonts:** the brand fonts (Plus Jakarta Sans, Hanken Grotesk,
  JetBrains Mono) are now self-hosted woff2 in `frontend/static/fonts/` (see
  `src/fonts.css`); the Google Fonts `<link>` is gone and the Tauri CSP is
  `font-src 'self'`. Offline-first + Flatpak-sandbox-clean. Don't re-introduce a
  font CDN.

### Backend
- **Sessions must persist.** They're DB-backed (table `sessions`); an in-memory
  store logged everyone out on every redeploy (symptom: tasks show but all tags
  render **grey**, because `/api/v1/tags` 401s and the color falls back).
- **Inspecting the live DB:** it's WAL mode. Copy `sempa.db` **with** its `-wal`
  and `-shm` files (or query in-process) or you'll read stale state.
- **Fastmail:** calendar reads go over **CalDAV** (app-password Basic auth), not
  JMAP. JMAP was unreliable and is effectively mail-only. Don't send an app
  password as `Bearer`.

## Pointers

- Contributing: `CONTRIBUTING.md`
- Security policy: `SECURITY.md`
- Jira integration: `docs/JIRA.md`
- Desktop updater status/steps: `docs/UPDATER.md`
- **Maintainer ops/security runbook: private `sempa-ops` repo → `BUILDERS.md`**
