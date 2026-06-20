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
- **Suggest tags now proposes new tags** (shipped) — was existing-only; now also
  offers a few concise new tags when nothing fits.
- Idea: surface AI suggestions inline (chips to accept) rather than auto-applying.

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
  - **1B-2 scoping (next).** Set `owner_id` on create from the session user; scope
    every read + `/sync/changes` + SSE by owner; map device/Dock sessions to the
    primary user. Isolation tests (user A never sees user B's rows). The privacy flip.
- **Phase 2 — sharing.** `shared` flag + Private/Shared toggle on tasks/lists/
  objectives; sync + realtime include shared items for both; cascade to subtasks/
  list-items. Tags global; reminders to owner (v1).

## Known issues / recently fixed
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
