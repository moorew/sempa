# Changelog

All notable, user-facing changes to Sempa are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/), and Sempa follows
[Semantic Versioning](https://semver.org/). Each release is also tagged in git
(`vX.Y.Z`) with auto-generated notes on the
[Releases page](https://github.com/moorew/sempa/releases).

## [1.0.139] - 2026-06-16

### Changed
- **Daily encouragement is now much more subtle.** Instead of a fixed line on the
  day view, the quote appears with the logo animation when you open Sempa and
  flashes briefly when you complete a task, then fades. (Still toggleable/editable
  in Settings → Accounts.)

## [1.0.138] - 2026-06-16

### Fixed
- **Some Fastmail calendars were missing** (e.g. a shared "Family" calendar). The
  schedule only listed calendars that had an event on the day in view, and events
  were de-duplicated globally by UID across calendars — so a shared calendar whose
  events also live on another calendar could be zeroed out and vanish. Now **all
  discovered calendars are listed** (so you can show/hide and recolour every one,
  even with no events that day), and de-dup is per-calendar so shared calendars
  keep their events.

### Added
- Documentation: today's changes captured in this changelog, the README User
  Guide (schedule, tags export, objectives, calendars, Jira, daily encouragement),
  and a new `docs/JIRA.md`.

## [1.0.137] - 2026-06-16

### Added
- **Daily encouragement.** A quiet, rotating quote appears on the day view (stable
  through the day) to add a little personality. Subtle by design; turn it off or
  edit the list (add/remove/reset) in Settings → Accounts.

## [1.0.136] - 2026-06-16

### Added
- **Jira filters are now multi-select.** Pick any number of values per facet —
  e.g. Priority ∈ {P1, P2}, Status ∈ {To Do, Backlog} (OR within a facet, AND
  across facets) — as toggleable chips.
- **Your default Jira view.** The filters and scope you choose are saved and
  restored automatically, so your configured view is your default. "Reset to
  default view" returns to the built-in defaults.
- **Undo delete.** Deleting a task shows an Undo toast (recreates non-Jira
  tasks; Jira tasks return via re-sync — see below).
- **`docs/JIRA.md`** documents the Jira integration end to end.

### Fixed
- **Deleting a Jira task no longer loses it.** A deleted Jira-linked task now
  re-imports (an immediate re-sync on delete, plus Jira is now on the background
  poller), so it returns to the Jira panel instead of vanishing until a manual
  sync.

## [1.0.135] - 2026-06-16

### Added
- **Richer Jira tasks.** The issue description is copied into the task notes on
  import (offline-readable; existing tasks backfilled when their notes are empty,
  without overwriting edits).
- **Manage Jira from the task.** The task editor shows a live Jira section —
  current status, type, priority, assignee, labels, an "Open in Jira" link, and
  the issue's real workflow transitions as buttons, so you can change status
  without leaving Sempa.

## [1.0.134] - 2026-06-16

### Fixed
- **Email → task showed "task not found".** The task was created then immediately
  updated to the dropped day in a second call that raced the new row. Email-to-task
  now creates the task on the dropped day in a single request — no error, correct day.

## [1.0.133] - 2026-06-16

### Fixed
- **Reordering tasks within a day** did nothing (the drop index was computed
  against a differently-sorted list); it now reorders correctly.
- **Dragging a Jira issue onto a day** now lands it even when the issue isn't yet
  in the loaded board (it's fetched on drop).
- **Failed drops are visible.** Email/Jira/move failures now show a toast instead
  of being silently swallowed.

## [1.0.132] - 2026-06-16

### Fixed
- **Critical: opening a task froze the app.** An internal effect looped
  (`effect_update_depth_exceeded`), wedging clicks, typing (the Add button stayed
  greyed) and drag-drop on web and desktop. Fixed.
- **Duplicate link previews.** A pasted link showed two preview cards in the task
  view; now one.

## [1.0.131] - 2026-06-16

### Fixed
- **Update prompt could open the GitHub page mid-build.** The in-app update
  notice is now held back until the platform's installer asset is actually
  published, and re-checks shortly after so it appears soon after the build lands.

## [1.0.130] - 2026-06-16

### Added
- **Proper right-click menus (desktop).** Right-click a task (Edit · Complete ·
  Focus · Delete), a weekly objective, or empty board space (New task · Go to
  today · Search). The generic webview menu is suppressed on desktop (text fields
  keep native copy/paste).

## [1.0.129] - 2026-06-16

### Fixed
- **Sync badge covered the Save button.** The floating online/offline indicator
  now hides while the task editor is open, on mobile and Windows.

## [1.0.128] - 2026-06-16

### Added
- **Recolour any calendar.** The schedule's Calendars panel now has a colour dot
  per calendar — including each Fastmail sub-calendar — tap to change its hue.

## [1.0.127] - 2026-06-16

### Changed
- **Calendars get distinct colours.** Each calendar is assigned its own brand hue
  (no more two calendars sharing a colour), and event blocks show the source
  calendar name so feeds stay distinguishable.
- **Bigger weekly-goal cards** in the day sidebar — each objective is a bordered
  card with a mini progress bar, easier to read and to drag.

## [1.0.126] - 2026-06-16

### Added
- **Schedule ↔ task sync.** Dropping a task on the calendar moves it into that
  day, snaps the block to the task's planned time (estimate, default 30 min), and
  resizing the block updates the planned time. Edits in the list reflect live on
  the calendar.
- **Tagged task export.** On the Search page, export the filtered task list as CSV
  or clean Markdown.
- **Drag an objective onto a day** to create a linked task; completing an
  objective's linked task(s) auto-completes the objective.

### Fixed
- **Android update download.** The updater now offers the Android APK on Android
  (it previously always handed back the Windows installer).
- **Objective Markdown export** is now a clean single-H1 bulleted list (no bold or
  subheadings).

## [1.0.125] - 2026-06-15

### Added
- **Drag to reorder weekly objectives.** Grab the handle on any objective on the
  Week page and drop it to change the order — it persists and syncs, just like
  reordering tasks. Works on desktop and the web app.

## [1.0.124] - 2026-06-15

### Fixed
- **Windows: grey box around reminder popups.** The floating reminder window was
  transparent, so Windows painted its backing grey wherever the cards didn't —
  showing as a grey box around the whole stack. The window is now opaque and
  painted edge-to-edge as a single dark panel (reminders are rows separated by
  hairlines, not separate floating cards), so there's no bare window area for the
  grey to show. Win11 still rounds the corners.

### Changed
- **In-app reminder & routine banners are far more compact.** Each reminder is now
  a single tidy row (icon + title + Open/Done/Snooze) instead of a tall two-row
  card, the "Plan your week" / "Daily shutdown" prompt is slimmer, and the two no
  longer leave a large gap between them. You still clearly see pending reminders
  without them taking over half the screen.

## [1.0.123] - 2026-06-15

### Fixed
- **AI "Suggest tags" returned nothing.** The prompt included a worked example
  whose tag values (`finance`, `home`, `work`) leaked into small models' answers
  (e.g. `llama3.2:3b` would reply `["work"]` for a Tailscale task). Since those
  aren't your tags, they were filtered out and you saw nothing. The prompt now
  restates *your* allowed tags and forbids inventing any — both `llama3.2:3b` and
  `qwen2.5:1.5b` now pick the right tags. "Suggest" also tells you when no tag
  matches instead of looking like a dead click.
- **AI "Plan my day" gave no visible result.** It reordered your tasks and saved
  the new order, but the Plan screen only showed a task *count*, so the change
  was invisible. It now shows the suggested order as a numbered list, surfaces
  any error, and the button becomes "re-plan" after running.

## [1.0.122] - 2026-06-15

### Fixed
- **AI features that "did nothing" (tidy notes, suggest tags, quick add, etc.).**
  Two causes: (1) the app called Ollama's `/api/generate`, which some models —
  including recent `llama3.2` builds — reject with *"does not support generate"*;
  it now uses `/api/chat`, which works across models. (2) AI failures were
  swallowed silently — the task panel now shows the error instead. If a model
  still errors with *"does not support generate/chat"*, re-pull it
  (`ollama pull <model>`) — a stale manifest from an Ollama upgrade can break the
  old copy.
- **Quick-add extraction** is much better: it now reliably resolves relative days
  ("tomorrow", weekday names), durations, and tags (added a worked example + the
  weekday to the prompt).

## [1.0.121] - 2026-06-15

### Fixed
- **Desktop: connecting Google (Drive backup / Gmail) no longer logs you out or
  makes data "disappear".** The consent flow used to navigate the desktop app's
  main window to a remote page, stranding it on the server's web login where the
  local database is (correctly) blocked — surfacing as
  `Command plugin:sql|load not allowed by ACL`. Desktop now opens OAuth in your
  **OS browser** and re-checks status when you return; the local DB is never left.
  (If you hit this: just relaunch the desktop app — your data is safe on the
  server and re-syncs.)
- **AI suggest-tags and quick-add tag parsing.** The allowed-tag list was passed
  to the model in a malformed shape, so tag suggestions came back empty / merged.
  Tags are now passed as a proper list (with an example), and quick-add gets the
  weekday so relative dates ("Thursday") resolve.

### Added
- **AI: Tidy up notes** — a ✦ on the task Notes field reformats messy / pasted
  text into clean paragraphs and bullet lists, preserving all content and URLs.
  Local-only, toggleable in Settings → Integrations → AI.

## [1.0.120] - 2026-06-14

### Added
- **AI assist (local model) across the app — all opt-in per feature** (Settings →
  Integrations → AI → "AI features"; each also requires the model to be reachable):
  - **Natural-language quick add** — the task title field gets a ✦ that parses
    "lunch w/ Sam thu 1pm 30m #personal" into title, date, time, estimate, and tags.
  - **Suggest tags** — ✦ on the task tag editor proposes tags from your existing set.
  - **Break into subtasks** — ✦ on a task creates a few concrete subtasks.
  - **Email/Jira → task summary + estimate** endpoint (shared helper).
  - **Plan my day** — Plan Day suggests an order for today's tasks around your events.
  - **Draft weekly review** — Week Review fills wins / challenges / next-focus from the week.
  - **Reflection prompts** — Shutdown suggests context-aware end-of-day questions.
  - New backend `internal/ai` client + `/api/v1/ai/*` endpoints; everything runs on
    your local Ollama model — nothing leaves the server.

### Changed
- **Mobile sync status is now hidden at rest.** It only appears while
  syncing/pending/offline/errored (bottom-left, above the tab bar) and disappears
  when synced, so it never sits on top of content. Desktop keeps the permanent
  bottom-right cloud.

## [1.0.119] - 2026-06-14

### Fixed
- **Mobile: the floating sync status no longer covers the "+" button.** On mobile the
  sync widget now sits in the bottom-left (above the tab bar); the bottom-right corner
  is left to the task-creation FAB. Desktop is unchanged (bottom-right).

## [1.0.118] - 2026-06-14

### Fixed
- **Backups failing silently with "refresh returned HTTP 400".** When the Google
  Drive refresh token expires or is revoked (Google returns `invalid_grant` — OAuth
  apps still in "Testing" status expire refresh tokens after 7 days), the error is now
  recognised as **re-auth required**. Backup → Google Drive shows a clear amber
  "Google access expired — Reconnect" banner (with a tip to publish the OAuth app so
  tokens stop expiring weekly) instead of reading as "Connected" while every backup
  fails. The token refresh error also now includes Google's actual message.

## [1.0.117] - 2026-06-14

### Added
- **AI model management.** Settings → Integrations → AI now lists every downloaded
  model with its on-disk **size**, lets you pick the active one, **download a new
  model with a live progress bar**, and **remove** a model (with a confirm that it
  must be re-downloaded). Backed by new server endpoints that proxy Ollama
  pull/delete and report progress.
- **Action feedback in AI settings.** Test/Save and model actions now report clearly
  ("Connected · N models", "AI settings saved", "Nothing to save", "Downloaded X",
  "Removed X") instead of silently doing nothing.

### Changed
- **Sync status is now a floating widget** (bottom-right): a permanent compact cloud
  icon (cloud-off when offline) whose label fades in only on hover, while
  syncing/pending/offline/errored, or briefly after a sync — freeing the left rail.
  The sidebar footer (utility icons + account) is correspondingly shorter.
- **Platform-correct keyboard shortcuts.** The Search hint shows `Ctrl+K` on
  Windows/Linux and `⌘K` on macOS, and the shortcut now actually opens Search.

## [1.0.116] - 2026-06-14

### Fixed
- **More theme-aware highlights.** The right-panel docks weren't fully themed: the
  Inbox/Email tab underline, unread dot and "→ Task" button, the Jira issue keys and
  links, the Jira "Medium" priority marker, and the weekly Goals progress bar/dots used
  fixed blue/yellow/amber that ignored the active theme. They now follow `--sempa-accent`
  (and `--sempa-amber` for the Medium-priority marker), matching the rest of the UI.
- **Local AI connection (deploy).** On the default compose, `OLLAMA_BASE_URL` resolved to
  a bridge hostname the host-networked app container couldn't reach. Combined with the
  1.0.115 compose change, the app now talks to Ollama over loopback; existing servers just
  set `OLLAMA_BASE_URL=http://127.0.0.1:11434`.

### Changed
- **Email view restyled.** The full Inbox now renders each message as a themed card (like
  the Reminders view) instead of a flat divider list, with inline "→ Task" / "Archive"
  actions — all using the theme tokens.

## [1.0.115] - 2026-06-14

### Added
- **In-app updates.** A subtle update indicator in the left rail, a brand-controlled
  "update available" toast (Download · What's new · Later), and **Settings → About**
  showing the current version, update channel (Stable/Beta), automatic-checks toggle,
  last-checked time, and a manual "Check for updates". Works on web and desktop by
  polling GitHub Releases — no signing required. The full silent background
  auto-update path (tauri-plugin-updater) is scaffolded and documented in
  `docs/UPDATER.md`; it activates once an updater signing key is added to CI.
- **Local AI is now opt-in at install.** `install.sh` asks whether you want local
  AI for text processing; if yes it starts Ollama, pulls `qwen2.5:1.5b`, prefills
  the in-app AI fields, and verifies the connection. Otherwise Ollama isn't started.
- **Sectioned navigation rail** with a pinned Search pill and a configurable grouping
  (Settings → Appearance: Spaces / Plan·Focus·Review / Flat, with Labels or Dividers).

### Fixed
- **Local AI connection.** Ollama ran on a bridge network the (host-networked) app
  container couldn't resolve (`http://ollama:11434`), so the AI test returned 404 /
  "not reachable". It now runs on the host namespace bound to loopback and the app
  talks to it over `127.0.0.1` — and only runs when you opt in.
- **Theme-aware highlights.** Orange/amber that ignored the active theme now follows
  it: the Pomodoro timer, overdue/focus task badges, backup warnings, the AI status
  dot, and the Schedule calendar swatches (no longer stuck orange in cool themes).
- **Left rail polish.** Footer icons no longer squash (distorting their highlight),
  the sync status no longer collides with the icons, and the account avatar is now a
  proper chip (avatar + email + Sign out) instead of an orphaned button.

## [1.0.114] - 2026-06-14

### Fixed
- **Installer no longer aborts when you enter a custom value.** `install.sh` ran
  under `set -e`, and its `ask_default` helper returned a non-zero status whenever
  you typed anything other than the default (App URL, host port, or username),
  silently exiting the script right after the first prompt. Accepting every
  default happened to work, which hid the bug.
- Made `install.sh` portable to hosts with BusyBox `grep` (e.g. minimal/Alpine
  systems): replaced `grep -oP` for the Docker version and URL port with
  bash-native parsing.
- Hardened the "update existing install" backup step so it can't abort when only
  one of `.env` / `.env.local` is present.

### Changed
- **Clarified Tailscale setup docs.** The README and installer now explain that
  the bundled `ts-sempa` sidecar joins the tailnet as its own dedicated node
  (`sempa.<your-tailnet>.ts.net`) rather than reusing the host machine's name,
  and document the MagicDNS/HTTPS and `tag:container` prerequisites. Removed the
  incorrect manual `tailscale cert` step (Serve provisions the cert automatically).
- Relicensed under AGPL-3.0; added CONTRIBUTING and this changelog.
- Routine dependency updates.

### Added
- Auto-tagging workflow: bumping the version in `frontend/package.json` on `main`
  now cuts the matching `vX.Y.Z` tag and kicks off the release builds.

## [1.0.113] - 2026-06-14

### Fixed
- **Recurring tasks now appear on future days across all platforms.** Recurring
  instances were generated lazily server-side only when a web client requested a
  given week, so offline-first desktop/Android clients (which read the local DB)
  saw a daily task "end" after the current week. A background poller now
  proactively materialises the current week plus the next two weeks.

### Security
- Updated Go toolchain to 1.25.11, fixing reachable standard-library
  vulnerabilities (GO-2026-5037/5038/5039, GO-2026-4986, GO-2026-4971).
- Updated `go-chi/chi` to v5.2.2, fixing GO-2025-3770 (open redirect).
- Added continuous security scanning (CodeQL, govulncheck, Trivy, gitleaks,
  zizmor, OpenSSF Scorecard) and Dependabot; pinned all GitHub Actions to commit
  SHAs.
