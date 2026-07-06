# Sempa Roadmap & Backlog

A running list of shipped work and ideas we've discussed but not built yet, so
nothing gets lost between sessions. Grouped by theme. Newest thinking at the top
of each list. This is a living doc — prune and reorder freely.

---

## Time tracking & time-blindness

The throughline: Sempa helps people who chronically under-estimate time. Capture
honest data with minimal friction, reflect it back, and use it to plan days that
are actually finishable.

### Shipped
- **Focus timer & honest capture** (v1.3.x) — persistent cross-platform widget,
  live planned-vs-elapsed, wall-clock measurement, end-of-session confirm/adjust.
- **Planned-vs-actual profile + AI prediction** (v1.3.0) — per-tag/global
  multipliers (median), estimate nudge, plan-day calibration, hybrid
  `predict-time` with cold-start "general estimate" + "learning your timing".
- **Capture on every completion** (v1.4.0) — quick modal on any completion
  without tracked time; instant keyword activity buckets + background AI refine;
  smart/adaptive suppression; settings page; first-run walkthrough.
- **Day capacity indicator** (v1.5.0) — subtle over-capacity hint in the editor
  and a quiet per-day total that warms when over; configurable hours/day.
- **Realistic (calibrated) capacity** (v1.6.0) — capacity judged against
  estimate × your history multiplier, not raw estimates ("realistically ~7h").
- **Insights screen** (v1.7.0) — calibration headline, planned-vs-actual totals,
  multipliers by activity & by tag, recent-tasks over/under; "still learning" state.
- **Edit recurring templates** (v1.8.0) — full editor (title/notes/tags/estimate/
  schedule) from Settings → Recurring Tasks; edits propagate to upcoming
  occurrences; instances now inherit the template's estimate.

### Next up / ideas
- **Insights: trend over time** — "you're getting better at estimating X";
  per-activity sparklines. (Headline view shipped in v1.7.0.)
- **Phase 5: Android notification-shade timer — shipped v1.10.0.** Foreground
  service (`FocusTimerService`) posts an ongoing notification whose countdown
  ticks natively via the notification chronometer (survives app close), with
  Pause/Done buttons. `FocusTimerPlugin` bridges to the web timer (source of
  truth); taps relay live or drain from prefs on next launch. iOS equivalent
  (Live Activity) would be a separate native build.
- **Native "Start focus" on reminders** — add the Focus action to the desktop
  floating card and the Android push notification (today only the in-app banner
  has it).
- **Default cold-start multiplier** — optionally seed ~1.3–1.5× before personal
  data exists (most time-blind folks under-estimate). Held back to avoid feeling
  presumptuous; revisit once the honest version has been lived with.
- **Idle auto-pause** — auto-pause the focus timer when the device is idle /
  backgrounded for a while, for even more accurate capture (mobile reliability TBD).
- **Per-task session history** — show the individual focus sessions logged
  against a task.
- **Tune capture cadence after real use** — the burst limit, consecutive-skip
  pause, and skip-quick threshold are easy dials if the prompt feels off.

---

## AI assist
- **AI Import — SHIPPED v1.22.0.** `POST /api/v1/ai/import` turns a URL or pasted
  text into a typed task with ordered steps (→ sub-tasks) plus a companion list of
  items. Deterministic schema.org/Recipe JSON-LD fast path, SSRF-safe full-body
  fetch (`unfurl.FetchContent`), LLM fallback for everything else. Editable preview
  (steps→subtasks, items→list, today/backlog placement). Entry points: Lists
  button, `Ctrl/⌘+Shift+I`, command palette, mobile More menu — each toggleable;
  gated on a reachable model. v1.23.2–1.23.3 refined step titles (short title +
  full detail, server-derived so 1.5–3B models can't dump/truncate).
- **Suggest tags now proposes new tags** (shipped) — was existing-only; now also
  offers a few concise new tags when nothing fits.
- Idea: surface AI suggestions inline (chips to accept) rather than auto-applying.

## Navigation & mobile
- **Command palette — SHIPPED v1.23.0.** `Cmd/Ctrl+K` keyboard-first launcher for
  navigation + actions (incl. Import with AI), with a search fallback so any query
  has a home. `source_url` became a first-class create field in the same release.
- **Share-to-Sempa — SHIPPED v1.18.0.** Android share-target: share a link/text
  from any app to turn it into a task.
- **Long-press app-icon quick actions — SHIPPED v1.19.0.** New task / Plan day /
  Shutdown ritual shortcuts from the launcher icon (desktop desktop-entry actions
  shipped alongside the Linux app).
- **Swipe-to-reschedule — SHIPPED v1.20.0.** Swipe a task left on mobile to push
  it to tomorrow.

---

## Lists
- **Shipped v1.9.0** — standalone checklists, drag-reorder, check-off (grey/strike
  in place), task linking (from the task editor), Organize-with-AI, Markdown
  export, archive/unarchive, optional archive-on-task-complete.
- **Offline sync — shipped v1.9.1.** Lists are now local-first like tasks: wired
  `lists`/`list_items` through `schema.ts`, `db.rs` (migration v8), `local-api.ts`
  CRUD, `sync.svelte` (upsert + pull apply + `TOMBSTONE_TABLE` + `replayListItem`),
  and `LOCAL_CORE`. List/item create accept a client id (`clientOrNewID`) so
  offline-created rows keep their id on sync. Reorder replays as per-item position
  updates. Lists also added to the mobile bottom tab bar.
- Ideas: reorder within AI-grouped view; move an item between lists; list templates.

## iOS
- **CI build green (feat/ios → main).** `@capacitor/ios` added; iOS Xcode project
  generated in CI (not committed; `cap add ios --packagemanager Cocoapods`), built
  unsigned for the simulator on a macOS runner — no Mac needed. Forced CocoaPods
  (SPM/core-8.4 plugin skew) and Xcode 16 (sqlcipher needs Swift-6 tools).
  `.github/workflows/ios-release.yml` + `docs/IOS.md`.
- **Next (needs your Apple account):** add the 4 App Store Connect API-key secrets
  (see docs/IOS.md) → the same workflow archives + uploads to **TestFlight** on a
  `v*` tag.
- **iOS feature parity follow-ons:** push (APNs), haptics (`@capacitor/haptics`),
  home-screen widgets (WidgetKit), focus-timer Live Activity (ActivityKit).

## Multi-user (household)
Decision: one server, **household model** — items are private or shared (both see/
edit); private-by-default; shareable = tasks, lists, weekly objectives. (Not
container-per-user — federation is the wrong tool for 2 people.)
- **Phase 1A — identity + credentials (shipped v1.11.0).** `users` table + bcrypt
  credentials; Google→user records; `user_id` on sessions; admin user-management
  + self password change (Settings → Users); env login preserved as bootstrap
  admin; first post-deploy login becomes admin; hardening (bcrypt, login throttle,
  timing-uniform checks, admin gating). Backend `db/users.go`, `api/users.go`,
  `auth.go`; frontend `/settings/users`.
- **Phase 1B — data ownership (in progress).**
  - **1B-1 ownership foundation (shipped v1.11.2).** Migration 024 adds `owner_id`
    to tasks, lists, list_items, weekly_objectives, daily_plans, week_reviews,
    pomodoro_sessions (+ indexes). Startup `db.BackfillOwnership` claims every
    still-unowned (`owner_id=''`) row for the primary (oldest) user — idempotent,
    no-op with no users. **Behaviour-neutral**: nothing is scoped yet, so data is
    still shared; this just attributes existing rows so the flip can't hide them.
  - **1B-2 scoping + Phase 2 sharing — SHIPPED v1.12.0.** Per-user scoping across
    every read, search, realtime + `/sync/changes` (owner resolved once in
    `requireAuth`, `db.SystemScope` sentinel for background callers, empty owner
    fails closed). `shared` flag (migration 025) on tasks/lists/list_items/
    objectives with a visibility rule `owner_id = me OR shared = 1`; personal
    tables (plans/reviews/pomodoro) are owner-only (migration 026 gives them a
    composite `(owner, date)` unique). Private/Shared toggle on tasks + lists
    (cascades to sub-tasks / items). Un-share records a `revoke` tombstone that
    drops the item from peers but not the owner. SSE stays a global ping →
    scoped refetch. Isolation tests in `db/scoping_test.go`.
  - **1B-3 objectives Private/Shared — SHIPPED v1.17.0.** The last piece of the
    sharing model: weekly objectives now carry the same Private/Shared toggle as
    tasks and lists, completing per-item sharing across all three shareable types.
  - **Follow-ups:** offline (local-first) display of the shared flag (not mirrored
    into the device SQLite schema — the toggle is online-only for now, which is why
    it's hidden when offline).
- **Phase 2 — sharing.** `shared` flag + Private/Shared toggle on tasks/lists/
  objectives; sync + realtime include shared items for both; cascade to subtasks/
  list-items. Tags global; reminders to owner (v1).

## Known issues / recently fixed
- **Timezone-aware recurrence — SHIPPED v1.21.0.** Server home timezone
  (`SEMPA_TIMEZONE`, editable in Settings → Notifications), floating task times,
  a non-destructive `SeedHorizon` poller that never deletes a still-current
  instance at the container's midnight, client device-`?today=` for exact rollover,
  and a travel-detection banner (device zone vs home zone). See CLAUDE.md gotchas.
- **Duplicate recurring instances heal everywhere** (fixed v1.23.5–1.23.7) — a
  non-atomic `instanceExistsForDate` seed race (poller vs parallel client
  week-fetches) could leave two pristine copies on a day; dedup only ran for
  "today". Now `dedupeRecurringInstances` runs on every seeded day (per-day) plus a
  global all-dates pass in `SeedHorizon` each tick, collapsing open **and** done
  pristine duplicates by an explicit preference rank while never emptying a day
  (timezone-safe on the poller).
- **Month calendars start on Monday** (fixed v1.23.8) — the pop-up date picker and
  mini month calendar were Sunday-first, out of step with the app's Monday-first
  `weekStart()` convention.
- **Recurrence deletes now tombstone** (fixed v1.8.1) — raw-SQL recurrence
  deletes bypassed the sync layer, stranding stale instances on local-first
  devices (phantom duplicates). All recurrence deletes now record sync tombstones.
- **Client sync-reconcile** (shipped v1.9.0) — `reconcileRecurringInstances` in
  `sync.svelte` drops local recurring instances the server no longer lists (heals
  devices stranded *before* the tombstone fix), backed by
  `GET /tasks/recurring/instances`. Safe: instance-scoped, aborts on fetch
  failure, per-(origin,date) buckets, skips ids pending upload. Remaining idea:
  prune old tombstones over time.
- **Duplicate recurring instances** (fixed v1.6.0) — once an instance was
  customised (or carried forward), the dedup ignored it and a fresh pristine
  duplicate was created. Now: one instance per template per day, with a self-heal
  pass. (Deletes tombstone since v1.8.1 and devices self-heal since v1.9.0, so
  stranded duplicates no longer linger.)
- **First-run walkthrough re-opening every launch** (fixed v1.6.0) — mount-order
  race read the "seen" flag before the store loaded it; gated on an `initialized`
  flag.
