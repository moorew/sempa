# Changelog

All notable, user-facing changes to Sempa are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/), and Sempa follows
[Semantic Versioning](https://semver.org/). Each release is also tagged in git
(`vX.Y.Z`) with auto-generated notes on the
[Releases page](https://github.com/moorew/sempa/releases).

## [1.20.0] - 2026-06-22

### Added
- **Swipe a task left to push it to tomorrow** (mobile), alongside the existing
  swipe-right-to-complete. A quick gesture to clear today without opening the task.

## [1.19.0] - 2026-06-22

### Added
- **Quick actions on the Android app icon.** Long-press Sempa on your home screen
  for shortcuts straight to **New task**, **Plan today**, and **Daily shutdown** —
  no need to open the app and navigate first.

## [1.18.0] - 2026-06-22

### Added
- **Share to Sempa (Android).** Share a link or text from any app — browser, email,
  anything — and pick Sempa: it opens a new task prefilled with what you shared (a
  page's title becomes the task, its URL drops into the notes as a link). Capture
  without leaving the app you're in.

## [1.17.0] - 2026-06-22

### Added
- **Share weekly objectives with your household.** Objectives now have the same
  Private/Shared control as tasks and lists (multi-user installs only): right-click
  an objective → "Share with household" / "Make private". Shared objectives show a
  small people icon. Completes the multi-user sharing model.

## [1.16.1] - 2026-06-22

### Fixed
- **Focus widget resting face.** The face's lines were too thick and the eyes/smile
  merged into one blob; the stroke is lighter now and the eyes and smile are spaced
  apart so it reads as a calm face.

## [1.16.0] - 2026-06-21

### Added
- **Theme-matched app icon (Android, opt-in).** When you switch theme, Sempa asks
  if you'd like to match the home-screen app icon too. Say yes and the launcher icon
  swaps to an accent-coloured variant (Terracotta, Forest, Plum, Slate, OLED, Ocean).
  It's always your choice — nothing changes the icon silently — because Android
  briefly refreshes the launcher icon when it switches.

## [1.15.0] - 2026-06-21

### Added
- **Widgets now follow your theme.** All home-screen widgets pick up the active
  theme's colours — accent preset (Terracotta, Forest, Plum, Slate, OLED, Ocean)
  and light/dark — and update when you change it.
- **Calm resting state for the focus widget.** When there are no tasks, the focus
  widget shows a friendly resting face with "All clear · Tap to plan today", so it
  looks good on your home screen all the time.

## [1.14.4] - 2026-06-21

### Fixed
- **App no longer crashes when finishing/discarding a focus session** (which left
  the "How long did you work?" sheet stuck on every launch). The focus service was
  started with `startForegroundService()` but stopped without `startForeground()`,
  violating Android's contract; it now promotes to the foreground for an instant
  before tearing down, so Done/Discard/Stop are safe and the stuck sheet clears.

### Changed
- **Focus widget rebuilt for a proper, resizable timer.** It's now a native widget
  (not Glance) so it shows a real ticking countdown with the phase, task, and
  Pause/Resume + Done controls — and it works from a small tile up to a large one.
  Idle, it lists today's tasks to start. (Remove and re-add the widget after
  updating.)

## [1.14.3] - 2026-06-21

### Fixed
- **Starting a task from the home-screen widget no longer pops "How long did you
  work on that?".** A leftover, unconfirmed prior session could ambush a fresh
  start; the widget start now supersedes it, ignores any stale notification action,
  and begins cleanly. Also made the timer's single-owner coordination
  desktop-only, so on Android the start always takes effect immediately.
- **Widget picker previews.** Replaced the oversized app-icon previews with proper
  per-widget mock previews (Focus timer, Today's tasks, Weekly overview, Progress)
  so each widget is recognisable before you add it.

## [1.14.2] - 2026-06-21

### Changed
- **The Android focus widget now shows a live, ticking countdown.** Instead of a
  static "X min left", the widget mirrors the running app timer with a real mm:ss
  countdown (a native chronometer that ticks on its own — no battery cost), kept in
  sync with start/pause/resume. It keeps ticking with the app closed.

## [1.14.1] - 2026-06-21

### Fixed
- **Home-screen widget picker now names each widget.** All four widgets showed as
  "sempa" with a blank white preview, so you couldn't tell which was the focus
  timer. They now have distinct labels — "Sempa · Focus timer", "Sempa · Today's
  tasks", "Sempa · Weekly overview", "Sempa · Progress" — and show the app icon as
  a preview instead of a white square.

## [1.14.0] - 2026-06-21

### Added
- **Android home-screen focus widget.** Add the Sempa focus widget to your home
  screen to run the Pomodoro timer without opening the app: when a session is
  active it shows the phase, task, and minutes left with Pause/Resume and Done
  controls; when idle it lists today's tasks so you can tap one to start focusing
  (the app opens briefly to begin, and time is logged as usual on finish). The
  widget mirrors the native focus service, so it keeps working while the app is
  closed.

## [1.13.0] - 2026-06-21

### Added
- **Floating focus-timer widget on the desktop.** Pop the Pomodoro timer out into
  its own compact, always-on-top window that stays above your other apps — start a
  task from today's list, pause/resume, and log time without the main window in
  focus. Open it from the tray ("Focus timer") or the pop-out button on the in-app
  timer. Under the hood the timer now coordinates across windows (a single owner
  drives the clock) so the main window and the widget never double-count.
  _(The Android home-screen widget is coming in a follow-up.)_

## [1.12.4] - 2026-06-21

### Fixed
- **Android/desktop sync no longer gets stuck at "1 to sync".** A task or weekly
  objective created offline logged its `shared` flag as a SQLite integer (`0`/`1`),
  but the server expects a JSON boolean and rejected the push — wedging the whole
  outbox so nothing synced afterwards. The sync engine now sends `shared` as a
  proper boolean, which also clears any mutation already stuck from this. (A
  follow-on to the 1.12.3 sharing fix.)

## [1.12.3] - 2026-06-21

### Fixed
- **Sharing now works on the desktop and Android apps.** The Private/Shared
  toggle wrote a `shared` column that the on-device database didn't have, so it
  errored on tasks and silently did nothing on lists. The column is now mirrored
  through the local schema, the on-open reconcile, the desktop migration, sync,
  and the local API — so the toggle works offline-first and syncs both ways.
  (The web app was unaffected.)

## [1.12.2] - 2026-06-21

### Changed
- **Sharing control is now an on-brand toggle.** The list sharing checkbox was a
  default browser checkbox; it's now the same themed Sempa switch used elsewhere,
  and the label is simply **Private / Shared** (was "Share with household").

## [1.12.1] - 2026-06-20

### Added
- **Invite members with Google — no password stored.** Settings → Account → Users
  now invites people by email as **Google-only accounts**: they sign in with
  Google and Sempa never holds a credential for them. A password account is still
  available behind a toggle if you want one.

### Security
- **Google sign-in is now closed by default.** A Google account can sign in only
  if it was invited, is on the env allow-list, or is the very first account on a
  fresh server. Previously an *empty* allow-list let any Google account create
  itself — that fall-open is fixed.

## [1.12.0] - 2026-06-20

### Added
- **Multi-user: private by default, share what you choose.** Every task, list,
  weekly objective, plan, journal entry and pomodoro session is now scoped to its
  owner — others on the same Sempa never see your data unless you share it. A
  **Private / Shared** toggle on tasks and lists lets the household see and edit
  the things you opt in (sharing a task cascades to its sub-tasks; sharing a list
  to its items). Daily plans, journals, insights and pomodoro stats stay strictly
  personal. The sharing controls only appear once a second account exists.
- Un-sharing immediately removes an item from everyone else (the owner keeps it).

### Changed
- All reads, search, realtime sync and the offline `/sync/changes` pull are
  scoped per user, so each account only ever receives its own + shared data.

### Notes
- Your existing data was assigned to your account in v1.11.2, so nothing changes
  for you — you still see everything you had. A new household member starts with a
  clean, private space and sees only what you share.
- Offline display of shared-state and a Shared toggle for weekly objectives are
  planned follow-ups (the backend already supports objective sharing).

## [1.11.2] - 2026-06-20

### Changed
- **Multi-user: data-ownership foundation (internal).** Every user-owned entity
  (tasks, lists, list items, weekly objectives, daily plans, week reviews,
  pomodoro sessions) now carries an `owner_id`, and a one-time startup backfill
  assigns all your existing data to your account. Behaviour is unchanged — data is
  still shared across accounts — but this is the groundwork for per-user privacy,
  landing next.

## [1.11.1] - 2026-06-20

### Fixed
- **"Users & password" is now under Settings → Account**, where account and
  credential management belongs — it was previously buried under the Tasks
  settings section and hard to find.

## [1.11.0] - 2026-06-20

### Added
- **Multi-user accounts — foundation.** Real per-user identities with credential
  management at **Settings → Users & password**: an admin can add people
  (email + password), reset passwords, and grant admin; everyone can change their
  own password. Google sign-in now creates a user record (the first sign-in — or
  your env login — becomes admin), and password users don't need the email
  allow-list. Your existing env `SEMPA_USERNAME`/`SEMPA_PASSWORD` login keeps
  working as a bootstrap admin, so you can't be locked out.
- **Security hardening:** bcrypt password storage, login rate-limiting,
  constant-time credential checks with timing-uniform "unknown user" handling,
  and admin-gated user management.

### Note
- This is the **identity** layer only. Per-user data ownership and **shared
  lists/tasks/objectives** come next — until then, all data is still shared across
  accounts (anyone who logs in sees everything). After deploying, log out and back
  in once so your session picks up your new user identity.

## [1.10.0] - 2026-06-20

### Added
- **Android focus-timer notification.** While a focus session runs, an ongoing
  notification shows the task and a **live countdown** — and it keeps ticking on
  the lock screen and in the shade even when the app is fully closed (a foreground
  service using Android's native notification chronometer, so the timer never
  freezes). **Pause** and **Done** buttons control it from the shade; the in-app
  timer stays the source of truth and reconciles taps made while the app was shut.

## [1.9.2] - 2026-06-20

### Fixed
- **Adding list items keeps the keyboard up.** Tapping Add (or hitting return) no
  longer dismisses the keyboard — the input stays focused so you can rattle off
  items one after another. Same for creating lists.

## [1.9.1] - 2026-06-20

### Added
- **Lists now work offline.** Lists and their items are local-first like tasks —
  created, edited, reordered, and checked off with no server or internet, syncing
  when you're back online. (Wired `lists`/`list_items` through the local schema,
  Tauri migration, local API, and the sync engine's pull/push/tombstone paths.)
- **Lists in the bottom tab bar** on mobile, for one-tap access (no more digging
  through “More”).

## [1.9.0] - 2026-06-20

### Added
- **Lists.** A new **Lists** view for standalone, undated checklists (e.g. an
  ongoing groceries list). Add items, **drag to reorder**, and check items off
  (they grey out / strike through in place). Lists persist independently and can
  be **linked to a task** from the task editor's new "Lists" section — attach an
  existing list or create one for the task. **Organize with AI** groups a list's
  items into natural categories; **Export Markdown** downloads it (name as the
  title, items as bullets). Lists can be archived/unarchived, with an optional
  per-list "archive when its task is completed" cleanup. *(Currently requires a
  reachable server; full offline sync is a planned follow-up.)*
- **Self-heal for stranded recurring duplicates.** After each sync, local-first
  clients reconcile their recurring instances against the server's authoritative
  set and drop orphans the server no longer has (the lingering phantom duplicate).
  Safe by construction: only ever touches server-generated instances (never
  offline-created items), aborts on any fetch failure, reconciles only per-(origin,
  date) buckets the server reports, and never deletes an edit pending upload.

### Changed
- **Smarter tag suggestions.** "Suggest tags" now auto-adds an existing tag only
  when it clearly applies (its word is in the title/notes, or it's the detected
  activity), and offers everything else — uncertain existing tags, the activity
  itself, and new ideas — as tap-to-add chips. All tag matching is now
  case-insensitive/canonical, so "personal" + "Personal" can't both appear, and
  irrelevant guesses are no longer auto-applied.

## [1.8.1] - 2026-06-20

### Fixed
- **Phantom duplicate recurring tasks on phones/offline.** Recurrence cleanup
  deleted instances with raw SQL that bypassed the sync layer, so local-first
  devices were never told to drop them — leaving a stale copy alongside the
  freshly-generated one. All recurrence deletes now record sync tombstones so
  every device converges to one instance per day. (A duplicate that was stranded
  on a device *before* this fix needs deleting once in the app — it'll stay gone.)

## [1.8.0] - 2026-06-20

### Added
- **Edit recurring tasks.** Settings → Recurring Tasks now has an edit button on
  each template that opens the full task editor — change the title, notes, tags,
  time estimate, *and the schedule* (e.g. daily → weekdays). Edits propagate to
  upcoming occurrences while anything you've already customised or worked on is
  left untouched. Recurring instances also now inherit the template's time
  estimate, so they feed capacity and the time profile.

## [1.7.0] - 2026-06-20

### Added
- **Time insights screen.** A new **Insights** view (in the nav, and linked from
  Settings → Time tracking) shows your planned-vs-actual profile: a plain-language
  calibration headline ("1.8× longer than you plan"), planned-vs-actual totals,
  multipliers **by activity** and **by tag**, and a recent-tasks list with the
  over/under on each. Shows a friendly "still learning" state until there's data.

## [1.6.0] - 2026-06-20

### Added
- **Realistic day capacity.** The capacity check now judges a day by *realistic*
  time — your estimates × how long that kind of work actually takes you — so a
  day of optimistic guesses still trips the limit ("Realistically ~7h — over your
  6h"). With no history yet it matches your estimates. Toggle in Settings → Time
  tracking.
- **Smarter tag suggestions.** "Suggest tags" now proposes a few concise **new**
  tags when your existing ones don't fit, instead of only ever reusing the
  current set.

### Fixed
- **Duplicate recurring tasks.** Once a recurring instance was customised (or
  carried forward from a missed day), the generator created a second, pristine
  copy for the same day. Recurring tasks are now strictly one-per-day, with a
  self-healing pass that removes existing duplicates (keeping the one you edited).
- **First-run walkthrough re-opening on every launch.** A start-up race let it
  read its "already seen" flag before settings finished loading; it now waits.

## [1.5.0] - 2026-06-20

### Added
- **Day capacity indicator.** Set a realistic hours-per-day limit and Sempa
  subtly flags overload: setting a task's time that pushes a day past its limit
  shows a quiet line under the estimate (“That puts this day at 7h — over your
  6h”), and each day's planned total warms when it's over. No pop-ups — just a
  nudge to plan a day you can finish. Toggle and set the limit in
  Settings → Time tracking.

## [1.4.0] - 2026-06-20

### Added
- **Capture time on every completion.** Finish a task without the focus timer and
  Sempa pops a quick "How long did that take?" prompt — one-tap chips or a custom
  value — so even the small tasks you'd forget to track still build your
  planned-vs-actual profile. It's smart about it: skips trivially-quick tasks,
  backs off during a burst of completions, and pauses after repeated skips so it
  never nags. Never double-asks when the focus timer already logged the time.
- **Auto-detected activity types.** Tasks are classified into activities (Email,
  Meeting, Deep work, Admin, Errand…) from their title, each with a sensible
  default time that pre-fills the prompt instantly. When the local AI is on, it
  refines the suggestion from your own history in the background.
- **Time-tracking settings** (Settings → Time tracking) — toggle the completion
  prompt, skip-quick-tasks, edit the default time for each activity, set focus
  timer durations, and replay the intro.
- **First-run walkthrough** — a short, dismissible intro to the time-tracking
  features, re-watchable anytime from the settings above.

## [1.3.1] - 2026-06-19

### Fixed
- **Focus widget: "time spent" now counts up live.** The elapsed readout was
  static while the countdown moved; it now ticks second-by-second so you can see
  time accruing against your estimate.
- **Finishing from the widget now completes the task.** Marking a task done via
  the focus widget logged the time but left the task looking open in the day view
  (so you'd complete it twice); the completed status is now reflected immediately.

## [1.3.0] - 2026-06-19

### Added
- **Focus timer & time tracking, rebuilt for time-blindness.** A persistent
  Pomodoro widget now follows you across the whole app, showing the task, the
  countdown, and how far you're running **past your estimate** so overruns are
  visible in the moment. It measures real elapsed time (and survives reloads and
  navigation) rather than assuming a fixed block.
- **Confirm-and-adjust logging.** When you finish or stop a session, Sempa shows
  the measured time and asks you to confirm or adjust it (including “didn't really
  work on it”) — so the time logged against a task is honest, even if you walked
  away mid-timer.
- **Planned-vs-actual profile.** As you log confirmed times, Sempa learns how your
  estimates compare to reality — overall and per tag — and **nudges** your future
  estimates toward realistic numbers. **Plan my day** is calibrated by the same
  history so the schedule isn't over-optimistic.
- **AI time prediction (local).** An **Estimate with AI** action predicts how long
  a task will take. It works from day one with a clearly-labelled general estimate
  and a “learning your timing” indicator, then becomes **personalized** — grounded
  in your own similar past tasks — once you've logged a few sessions.
- **Reminder → one-tap Focus.** A task reminder's primary action now starts a focus
  session pre-loaded with the task and its planned duration.

## [1.2.0] - 2026-06-19

### Added
- **Sempa for Linux** — a native desktop app (Tauri/WebKitGTK) shipping as
  `.AppImage`, `.deb`, `.rpm` (x86_64 + aarch64) and an AUR `sempa-bin` package,
  with Flatpak/Flathub on the way. App id `ca.sempa.Sempa`, the `sempa://` URL
  scheme, and launcher actions (New task / Plan day / Shutdown ritual).
- **Native Linux feel** — follows the system light/dark scheme (live) and accent,
  mirrors your window-button layout in the in-brand titlebar (with a "Use system
  title bar" option), self-hosted brand fonts (offline, no CDN), reduced-motion
  and high-contrast support, and a responsive icon rail + ultrawide **cockpit mode**.
- **Linux surfaces** — tray (`StatusNotifierItem`), a global **Quick-Add** window
  (`Ctrl/Cmd+Shift+Space`), and a **Launch at login** toggle.
- **The Sempa Dock** — a Raspberry Pi touch appliance (kiosk image scaffold) that
  boots straight into today on the day thread, with an on-screen keyboard and an
  ambient idle face.
- **Device pairing** — pair a Dock (or any device) with a short code approved from
  a signed-in app; it gets a scoped, revocable token and never holds your password.

### Security
- Desktop bearer token now lives in the OS **Secret Service keyring** instead of
  plaintext; tightened the desktop CSP (`font-src 'self'`); least-privilege
  Flatpak (no `--filesystem=home`, portal-brokered).
- Patched a transitive `undici` advisory (dev/test tooling only).

## [1.0.150] - 2026-06-17

### Changed
- **The daily quote no longer makes the top of the day/home views taller.** On
  mobile it greets you under the header and then smoothly eases out of the way;
  on desktop it sits beside the header (no extra row) and collapses out of sight
  when the window is too narrow to spare the room.

### Added
- **Reorder tasks within a day on mobile.** Long-press a task card to pick it up,
  then drag it into the order you want — the list reshuffles under your finger
  and the new priority order is saved.

### Fixed
- **Same-day drag-to-reorder now reliably sticks.** Dropping a card in a new spot
  renormalises the day's ordering, so reordering works even when several tasks
  shared the same stored position (previously the move could silently do nothing).
- **The task editor is far less twitchy on Android.** Scrolling a menu or list
  inside the editor, or a small flick near the top, no longer tears the whole
  sheet away — a dismiss now needs a deliberate pull on the body (or the handle),
  and gestures that begin inside a nested scrollable are left to scroll.

## [1.0.149] - 2026-06-17

### Security
- **The server container now runs as a non-root user** (uid 10001) — resolves
  the last Trivy/Scorecard hardening item (DS-0002). Fresh installs work out of
  the box (the data volume inherits the right ownership); existing installs are
  re-owned automatically by `install.sh` / `deploy/update.sh`, with a documented
  one-liner for manual stacks.

### Changed
- **`install.sh` can now pull the prebuilt GHCR image** instead of building from
  source (option 2 → `docker compose pull sempa`), and `docker-compose.yml`
  carries the `ghcr.io/moorew/sempa` image tag for that path.
- README documents pulling the image, the non-root upgrade note, and verifying
  signed downloads with cosign.

## [1.0.148] - 2026-06-17

### Fixed
- **Release signing is now resilient.** The cosign signing step retries up to 3×
  to ride out transient Sigstore/Fulcio OIDC hiccups (one such hiccup failed the
  1.0.147 signing). This is the version that actually ships cosign-signed
  installers + the GHCR image.

## [1.0.147] - 2026-06-17

### Fixed
- **Release tooling** so the 1.0.146 signing/GHCR work actually ships: pin
  **cosign to v2** (v4 deprecated the `--output-signature`/`--output-certificate`
  flags, which broke the 1.0.146 signing step), and have the tagger dispatch the
  new **publish-image** workflow (bot-created tags don't trigger tag workflows on
  their own). Signed releases + the GHCR image land from this version.

## [1.0.146] - 2026-06-17

### Security
- **Signed releases.** Desktop and Android release artifacts are now signed with
  **cosign** (keyless / Sigstore — no private keys to manage). Each download ships
  with a `.cosign.sig` + `.cosign.pem`; verify with `cosign verify-blob` (see the
  README → *Verifying release downloads*). Satisfies OpenSSF Scorecard
  Signed-Releases.
- **Prebuilt images on GHCR.** Each release now publishes a multi-arch
  (amd64/arm64) server image to `ghcr.io/moorew/sempa`, so you can pull instead of
  building from source. `install.sh` still builds from source by default.
- **Digest-pinned Docker base images** (reproducible builds; Scorecard
  Pinned-Dependencies).
- Documented the remaining OpenSSF Scorecard posture (single-maintainer and
  Tauri-Linux-build-graph items) in `SECURITY.md`.

## [1.0.145] - 2026-06-17

### Security
- **Triaged the GitHub code-scanning backlog to zero open items** — fixed the
  genuinely actionable findings and documented the rest (with rationale) in
  [SECURITY.md](SECURITY.md):
  - The session **logout** cookie now carries `HttpOnly` / `Secure` / `SameSite`,
    matching the active session cookie.
  - **Pinned the Docker base image** (no longer `:latest`) and added a container
    **health check**.
  - **Least-privilege CI**: release workflows are now read-only by default, with
    write scopes granted per-job.
  - Removed a **stray committed build binary** (`backend/server`) and ignored it;
    added a **Dependabot cooldown** to slow supply-chain churn.
  - Documented the accepted/mitigated findings — link-preview & webhook SSRF
    (owner-configured / IP-validated), the Linux-only `glib` advisory,
    container-as-root (deferred), and the single-maintainer Scorecard signals.

## [1.0.144] - 2026-06-17

### Security
- **Dynamic analysis in CI.** Go tests now run under the race detector, and the
  parsers that handle untrusted external input (iCal feeds, imported emails,
  recurrence rules, and local-model output) are fuzzed with Go's native fuzzing —
  a smoke run on every push/PR and a longer weekly sweep.

### Fixed
- **Link-preview parser could crash on certain malformed pages.** Fuzzing
  surfaced a slice-bounds panic in the unfurl HTML parser when a page's `</head>`
  detection ran over a lowercased copy whose byte length differed from the
  original (e.g. due to characters like `İ`); it now matches case-insensitively on
  the original markup. The crashing input is kept as a regression seed.

## [1.0.143] - 2026-06-17

### Security
- **Hardened the dependency supply chain.** Pinned patched versions of vulnerable
  build-time transitive dependencies (`tar`, `uuid`, `minimatch`, `cookie`) via
  npm `overrides` — these tools (Capacitor asset generation, xcode project
  tooling) never ship in the server, web bundle, or apps. Documented the
  dependency- and vulnerability-remediation process in `SECURITY.md`, including
  the one reviewed/accepted finding (`glib`, a Linux-only Tauri build-graph
  dependency that isn't in any shipped artifact).

### Internal
- **CI now runs the full test suite** (Go tests, frontend type-check, Vitest, and
  the UX invariant scripts) on every push and pull request to `main`.
- **GitHub Release notes are now drawn from this CHANGELOG** rather than an
  auto-generated commit list.
- Documentation now links to the full docs at [sempa.ca/docs.html](https://sempa.ca/docs.html).

## [1.0.142] - 2026-06-17

### Added
- **A calm celebration language.** Completing a task now blooms a small, warm
  flourish from the card (bloom + rising embers + a ripple). When you finish the
  last task of a day, that escalates to a quiet "day complete" moment with a
  light sweep across the day's progress bar; finishing the last weekly objective
  draws the cradle mark in a full-screen moment. Everything is on-brand, gated by
  reduced-motion, and silent by default — an optional soft chime lives under
  Settings → Appearance → Celebration sound.

### Changed
- **The daily encouragement is now a single quiet line** under the day/week
  header — it fades in, then settles back so it's present but recedes. It no
  longer flashes unreadably on the splash screen.
- **The task detail is now a roomier centered modal on desktop** (and a bottom
  sheet on mobile) instead of the cramped right-hand drawer. Long notes clamp to
  four lines with a "Show more", and a forwarded email shows as a tidy reader
  card with "Read full message" / "Open in mail" so a pasted thread can't run on.
- **Navigation feels more natural** — pages rise in gently, the day view's right
  panel crossfades between tabs, and the task modal scales/rises in and reverses
  out. All motion respects reduced-motion.

### Fixed
- **Local-AI JSON parsing** tolerates braces inside string values (e.g. a task
  title like `fix render() {}`) instead of truncating the model's response.

## [1.0.141] - 2026-06-17

### Changed
- **Sharper local-AI assists.** Reworked the prompts behind quick-add, task
  summaries, tag suggestions, breakdown, day planning, weekly review, reflection
  questions, tidy-notes, and email task-title cleanup so the small local model
  keeps specific names/projects, stops over-tagging, and respects length limits.
  Email titles now get a short body snippet for context, and the model runs at a
  lower temperature with a bounded token budget for faster, steadier output.

## [1.0.140] - 2026-06-16

### Changed
- **Settings is no longer one long scroll on desktop.** It now shows one section
  at a time (Account · Integrations · Tasks · Appearance · About), selected from
  the sub-nav — matching how mobile settings already worked. The active section is
  reflected in the URL (`?section=…`) so sections are linkable.

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
