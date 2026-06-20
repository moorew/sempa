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
- **Phase 5: Android notification-shade timer** — OS-level foreground-service
  timer so the countdown lives in the shade/lock screen even when the app's
  closed. Biggest mobile win; real Capacitor/native lift, CI-validated only.
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
- **Next: offline sync for Lists.** Currently server-backed (httpApi). To make
  them local-first like tasks, wire the 5 spots: `lists`/`list_items` into
  `schema.ts`, `db.rs` migration, `local-api.ts` CRUD, `sync.svelte` upsert +
  apply + TOMBSTONE_TABLE (`list`/`list_item`), and add `lists` to `LOCAL_CORE`.
  Backend already emits them in `/sync/changes`, so this is additive.
- Ideas: reorder within AI-grouped view; move an item between lists; list templates.

## Known issues / recently fixed
- **Recurrence deletes now tombstone** (fixed v1.8.1) — raw-SQL recurrence
  deletes bypassed the sync layer, stranding stale instances on local-first
  devices (phantom duplicates). All recurrence deletes now record sync
  tombstones. Idea: a client-side **sync reconcile** that drops local recurring
  instances the server no longer has for a day, to heal devices stranded *before*
  the fix without a manual delete. Also: prune old tombstones over time.
- **Duplicate recurring instances** (fixed v1.6.0) — once an instance was
  customised (or carried forward), the dedup ignored it and a fresh pristine
  duplicate was created. Now: one instance per template per day, with a self-heal
  pass. Note: server-side recurring deletes don't emit sync tombstones, so a
  pre-existing duplicate may linger on an offline device until a full resync.
- **First-run walkthrough re-opening every launch** (fixed v1.6.0) — mount-order
  race read the "seen" flag before the store loaded it; gated on an `initialized`
  flag.
