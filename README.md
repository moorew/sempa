
<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 280 70">
  
  <rect width="280" height="70" fill="#2b221b"></rect>
  
  <svg x="5" y="5" width="60" height="60" viewBox="0 0 100 100">
    <path d="M22,40 a28,28 0 0 0 56,0" stroke="#f7f3eb" stroke-width="9" stroke-linecap="round" stroke-linejoin="round" fill="none"></path>
    <circle cx="50" cy="35" r="7.5" fill="#f7f3eb"></circle>
  </svg>

<img width="300" height="75" alt="sempa-logo-reversed" src="https://github.com/user-attachments/assets/e2683027-3cfa-40f3-91a7-fa4a68ef5cfa" />
<img width="1813" height="664" alt="image" src="https://github.com/user-attachments/assets/ed332d31-7805-48ff-b8b4-663db73e69e0" />


A self-hosted personal task manager for everyone.

Plan your day, track focused work, and end each day with intention — with your email and calendar pulled in automatically.

## Full details, docs and downloads available at [sempa.ca](https://sempa.ca)

---

## Features

- **Daily Kanban** — drag tasks across a week view, plan each day
- **Email → Tasks** — import starred Gmail or Fastmail emails as tasks
- **Schedule panel** — see calendar events alongside your tasks
- **Pomodoro + timeboxing** — schedule focused blocks, track sessions per task
- **Weekly review** — set objectives, review what shipped, plan ahead
- **Shutdown ritual** — guided end-of-day reflection
- **Jira sync** — bi-directional: import assigned issues, mark done in Sempa to close the ticket
- **Reminders & notifications** — per-task reminders delivered by Web Push, Android, or a webhook, with selectable alert sounds
- **Recurring tasks** — daily, weekly, and monthly templates
- **Local AI assist** — an optional on-server model (Ollama) powers quick-add parsing, task summaries, tag suggestions, subtask breakdown, day planning, weekly-review drafts and reflection prompts. **100% local & private — nothing ever leaves your server**, every feature is individually toggleable, and it's off until you turn it on. [See the AI section ↓](#ai-assist--local--private)
- **In-app updates** — notices new releases, shows what's new, and points you to the installer (silent desktop self-update is opt-in)
- **Six themes** — Terracotta, Forest, Plum, Slate, OLED Black, and Ocean, each in light + dark
- **Keyboard shortcuts** — `n` new task, `t` today, `j/k` prev/next week, `?` help

📖 **New here? Jump to the [User Guide](#user-guide) for how to use every feature.**

### Apps

| Platform | How to get it |
|----------|--------------|
| **Web** | Self-host with Docker (see below) |
| **Android** | APK from [GitHub Releases](../../releases) or build from source |
| **Windows** | Sempa-branded `.exe` setup (NSIS) or `.msi` from [GitHub Releases](../../releases) (x64 + ARM64) |
| **PWA** | Install from your browser when visiting your Sempa instance |

All apps connect to your self-hosted server — your data stays on your machine.

---

## Quick start

**Prerequisites:** Docker and Docker Compose (v2).

```bash
git clone https://github.com/moorew/sempa.git
cd sempa
bash install.sh
```

The installer asks a few questions (URL, auth method, and optional extras like Tailscale, email-to-task, and local AI for text processing), writes your config, builds the image, and starts the container. The whole process takes about 2 minutes. Everything else — email, calendar, and Jira accounts — is connected later inside the app under **Settings**.

Open the URL it prints and follow the in-app setup wizard to connect your email and calendar.

---

## Self-hosting with Tailscale (recommended)

Tailscale is the easiest way to access Sempa securely from all your devices without exposing it to the public internet.

### Why Tailscale?

- **No port forwarding** — access your server from anywhere on your tailnet
- **Automatic HTTPS** — Tailscale provides TLS certificates via MagicDNS
- **Zero-trust networking** — only your devices can reach the server
- **Works on all platforms** — desktop, mobile, and headless servers

### Setup

> **Important:** The bundled `ts-sempa` sidecar joins your tailnet as its **own dedicated
> node named `sempa`** — it does *not* reuse the hostname of the machine it runs on. So your
> Sempa URL is **`https://sempa.<your-tailnet>.ts.net`**, regardless of what the host box is
> called. (`<your-tailnet>` is your tailnet's MagicDNS name, e.g. `tail1234.ts.net`, shown at
> the top of [the admin console](https://login.tailscale.com/admin/machines).)

1. **Enable the tailnet features it relies on** (one-time, in the admin console):
   - **MagicDNS** and **HTTPS Certificates** — [DNS settings](https://login.tailscale.com/admin/dns).
     Tailscale Serve uses these to provision the TLS cert for the `sempa` node automatically.
   - **An ACL tag `tag:container`** — the sidecar advertises this tag so the node never expires
     (servers shouldn't drop off the tailnet after 90 days). Add it under `tagOwners` in your
     [ACL policy](https://login.tailscale.com/admin/acls), e.g. `"tag:container": ["autogroup:admin"]`.

2. **Install Tailscale** on every device you want to reach Sempa from (phone, laptop, etc.):
   [tailscale.com/download](https://tailscale.com/download). The *server* does **not** need
   Tailscale installed on the host — the sidecar container provides it.

3. **Generate an auth key** at [Tailscale Admin → Keys](https://login.tailscale.com/admin/settings/keys).
   Create it as a **tagged key** with `tag:container` (so the sidecar can advertise that tag).
   You'll paste it when the installer asks for `TS_AUTHKEY`.

4. **Run the installer:**
   ```bash
   bash install.sh
   ```
   When asked for the **App URL**, enter your Sempa *node* address — not the host machine:
   ```
   https://sempa.<your-tailnet>.ts.net
   ```
   Paste the auth key from step 3 when prompted.

5. **HTTPS is automatic.** The `ts-sempa` container runs Tailscale Serve (`ts-sempa/config/sempa.json`),
   which provisions the cert for `sempa.<your-tailnet>.ts.net` and proxies `:443` → the app on `127.0.0.1:9001`.
   There's no manual `tailscale cert` step. Give it a minute on first boot, then check it's serving:
   ```bash
   docker compose logs ts-sempa     # look for "Serve started"
   ```

6. **Connect your phone/desktop app**: Open the app, enter `https://sempa.<your-tailnet>.ts.net`
   in the server field, and sign in.

### Alternative: any reverse proxy

Sempa works behind any reverse proxy (Caddy, nginx, Traefik). Set `APP_URL` to your public URL and configure the proxy to forward to port 9001. If you go this route, **make sure you have authentication enabled** (Google OAuth or username/password).

---

## Manual setup

If you prefer to configure things by hand:

**1. Clone the repo**

```bash
git clone https://github.com/moorew/sempa.git
cd sempa
```

**2. Create `.env`** (Docker Compose variable substitution)

```bash
cp .env.example .env
# Edit .env and set APP_URL to wherever Sempa will live
```

**3. Create `.env.local`** (secrets — never committed)

```bash
cp .env.local.example .env.local
# Fill in your credentials (see Configuration below)
```

**4. Build and start**

```bash
docker compose build
docker compose up -d
```

**5. Open the app**

Navigate to `APP_URL` in your browser. The first-run wizard will guide you through connecting integrations.

---

## Configuration

All configuration is in two files that you create locally:

| File | Purpose |
|------|---------|
| `.env` | Infrastructure (URL, port) — Docker Compose reads this for variable substitution |
| `.env.local` | Secrets (API keys, credentials) — loaded into the container |

### `.env`

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_URL` | `http://localhost:9001` | The URL where Sempa is accessible (no trailing slash) |
| `HOST_PORT` | `9001` | The port to expose on the host |

### `.env.local`

#### Authentication

Sempa supports two auth methods. You can enable one or both.

**Google Sign-In (recommended)**

Uses OAuth — you sign in with your Google account, no password to manage.

```dotenv
GMAIL_CLIENT_ID=your-client-id.apps.googleusercontent.com
GMAIL_CLIENT_SECRET=your-secret
# Comma-separated list of allowed Google emails.
# Leave unset to allow any Google account.
SEMPA_ALLOWED_EMAILS=you@gmail.com
```

Setup steps:
1. Go to [Google Cloud Console → Credentials](https://console.cloud.google.com/apis/credentials)
2. Create an OAuth 2.0 Client ID (Web application)
3. Add an Authorised redirect URI: `{APP_URL}/api/v1/auth/google/callback`
4. Copy the Client ID and Secret into `.env.local`

> The same credentials are used for Gmail integration — you only need one OAuth client for everything.

**Username & password**

```dotenv
SEMPA_USERNAME=admin
SEMPA_PASSWORD=your-strong-password
```

If `SEMPA_PASSWORD` is not set, auth is disabled entirely (fine for local-only installs on a trusted network like Tailscale).

#### Tailscale (optional)

If you use the bundled Tailscale sidecar (`ts-sempa` service in `docker-compose.yml`), add your auth key:

```dotenv
TS_AUTHKEY=tskey-auth-...
```

Generate one at [Tailscale Admin → Keys](https://login.tailscale.com/admin/settings/keys). The key is read by the `ts-sempa` container to join your tailnet.

#### Optional

| Variable | Description |
|----------|-------------|
| `TS_AUTHKEY` | Auth key for the Tailscale sidecar container |
| `EMAIL_FORWARD_TOKEN` | Secret token for the Cloudflare email-to-task webhook |
| `SMTP_ALLOWED_SENDERS` | Restrict email-to-task senders (comma-separated emails or `@domain`; empty = accept all) |
| `SMTP_PORT` | Port for the built-in inbound SMTP server (default: `2525`) |
| `VAPID_SUBJECT` | Web Push contact address (e.g. `mailto:you@example.com`); the VAPID key pair auto-generates |
| `FCM_KEY_PATH` | Path to a Firebase service-account JSON key for native Android push |
| `OLLAMA_BASE_URL` | Ollama endpoint for AI task-title cleanup. **Empty by default** (feature off). `install.sh` sets it to `http://127.0.0.1:11434` when you opt into local AI; it can also be set manually or changed in **Settings → Integrations**. |
| `OLLAMA_MODEL` | Local model for AI task-title cleanup (default: `qwen2.5:1.5b`) |
| `COMPOSE_PROFILES` | Set to `ai` to start the optional `ollama` service (written automatically when you choose local AI at install). |
| `INBOX_POLL_INTERVAL` | How often to poll the email inbox (default: `1m`) |
| `CALENDAR_POLL_INTERVAL` | How often to refresh ICS subscriptions + the Fastmail calendar (default: `15m`; empty disables) |

---

## Integrations

All integrations are optional and configured through the Settings UI after first login.

| Integration | What it does |
|-------------|-------------|
| **Gmail** | Imports starred emails as tasks. Uses the same OAuth app as sign-in. |
| **Google Calendar** | Shows today's events in the Schedule panel. Enabled via the Gmail settings page. |
| **Fastmail** | Imports starred emails as tasks via IMAP. App password required. |
| **Fastmail Calendar** | Syncs JMAP calendar events into the Schedule panel. |
| **Jira** | Imports assigned issues as tasks. Marking a Jira-sourced task done closes the ticket. |
| **Calendar feeds (ICS)** | Subscribe to any `.ics` / webcal URL for read-only events. |
| **Email inbox** | Forward any email to a Fastmail address to auto-create a task. |
| **AI assist (local)** | A local language model (Ollama, default `qwen2.5:1.5b`) powers title cleanup plus quick-add parsing, tag suggestions, subtask breakdown, day planning, weekly-review drafts and reflection prompts. **Runs entirely on your server — no data leaves it, no API key.** Opt-in (during `install.sh` or by setting `OLLAMA_BASE_URL`); manage models and toggle each feature in Settings → Integrations → AI. [Details ↓](#ai-assist--local--private) |

> **Note on the model-server URL (AI task-title cleanup).** The Ollama endpoint
> is configurable in Settings → Integrations and may point at an internal /
> loopback address — that's by design, because the model server is self-hosted
> (e.g. `http://ollama:11434`). Static analysis (CodeQL `go/request-forgery`)
> flags this as a possible SSRF because a configured URL drives a server-side
> request. It's a **deliberate, accepted trade-off**: the URL is settable only
> by the authenticated instance owner (who already controls the server), is
> validated to be a well-formed `http(s)` URL, and restricting it to public
> hosts would defeat the feature. See `SECURITY.md`.

---

## Connecting mobile & desktop apps

The Android app and Windows desktop app connect to your self-hosted server:

1. **Install the app** from [GitHub Releases](../../releases)
2. **Open the app** — you'll see a "Server URL" field
3. **Enter your server address** (e.g. `https://sempa.tail1234.ts.net`)
4. **Sign in** with your Google account or username/password

Both your phone and server must be on the same Tailscale network (or the server must be reachable from your phone's network).

> **Tip:** Install Tailscale on your phone to access your server from anywhere, even on mobile data.

---

## User Guide

Everything below is how to *use* Sempa day to day. Features work the same on web, the Windows desktop app, and Android, except where noted.

### First run

After signing in, a short setup wizard helps you connect email and calendar (all optional — you can skip and add them later in **Settings**). You land on **Today**.

### Getting around

- **Desktop / web:** a left sidebar with a pinned **Search** pill and a **sectioned nav rail** — by default grouped into Today/This Week, **Rituals** (Plan Day, Shutdown), **Inbox** (Email, Reminders), and **Library** (Backlog, Journal). You can change the grouping (Spaces · Plan·Focus·Review · Flat) and section style (Labels · Dividers) in **Settings → Appearance**. The footer holds a utility icon row (Settings, light/dark, desktop Widget) — plus an **update indicator** when a new version is available — the sync status, and an account chip (avatar + email + Sign out). The day view's right panel is a tabbed dock — **Schedule · Inbox · Jira · Goals** — under a mini-calendar.
- **Mobile:** a bottom tab bar — **Today**, **Week**, **Journal**, and **More**. The **More** sheet is grouped: a quick row (Settings, light/dark, Widget), a **Plan** group (Plan Day, Schedule, Backlog, Search), an **Inbox** group (Email, Reminders, Jira, Shutdown), and your account row. A **+** button creates a task on list screens.

### Tasks

The core unit. Open any task to edit it in a panel (desktop) or bottom sheet (mobile).

- **Create:** the **+** button, the “Add task” box, or press `n` on a day view.
- **Title & notes:** notes support pasted URLs, which render as tidy **link preview chips** (title, site, thumbnail) instead of raw links.
- **Status:** `backlog → planned → in progress → done` (plus `cancelled`). On the week board you change status by dragging between columns; in a task you toggle it done with the checkbox.
- **Due date & time estimate:** pick a due date with the styled date picker and an estimate (15 min – 8 h) used for planning.
- **Tags:** type to add colour-coded tags; they show as coloured dots on compact cards.
- **Sub-tasks:** break a task into a checklist of smaller items.
- **Time-blocking:** give a task a scheduled start/end so it appears as a block on the Schedule panel next to your calendar events.
- **“Roughly at”:** a soft time hint (e.g. “around 2pm”) that orders a task in the day without committing to a hard block.
- **Reminders:** set **Remind me** (date + time) for a hard alert — see [Reminders & notifications](#reminders-notifications--routines).
- **Attachments:** attach files to a task (or objective); stored on your server.
- **Right-click menu (desktop):** right-click a task for quick actions (Edit · Complete · Focus · Delete), a weekly objective for its actions, or empty board space for New task / Go to today / Search.
- **Undo delete:** deleting a task shows an **Undo** toast; non-Jira tasks are restored, and a deleted Jira task returns to the Jira list automatically.

Press `e` to edit the hovered task on a day view.

### Plan your day

**Plan Day** (`/plan`) is a guided morning ritual: write your **intention** for the day, see what carried over **from yesterday**, and pull tasks from your backlog into today. Your previous day’s note is shown for continuity.

### Week view

The **Week** board is a Kanban across the seven days. Drag tasks between days and statuses, set **Weekly Objectives** (the handful of outcomes that matter this week), and link tasks to an objective so progress is visible. You can **drag an objective from the sidebar onto a day** to create a task already linked to it; completing an objective's linked task(s) **auto-completes the objective**. "Copy as Markdown" exports a clean, single-heading bulleted list.

### Weekly planning & review

- **Plan the week:** review your backlog and schedule the week ahead. This is also surfaced automatically as a gentle in-app prompt (see Routines).
- **Weekly review:** capture **Wins**, **Challenges**, and your **Next focus**. Reviews are saved per week and searchable.

### Daily shutdown

**Shutdown** (`/shutdown`) is an end-of-day ritual: tick off what’s done, **reschedule** anything unfinished, record a **win** and an optional reflection, and close the day cleanly. Like weekly planning, it can prompt you automatically at a time you choose.

### Backlog

A single list of everything not yet scheduled. Use it as your inbox of ideas and pull items into days as you plan.

### Journal

The **Journal** collects your daily **intentions** and **reflections** and your weekly **wins / challenges / next focus** in one timeline. You can have intentions and reflections also appear inline on the day and week screens (toggle in **Settings → Appearance**, “contextual reflections”).

### Daily encouragement

A quiet, rotating quote appears on the day view (stable through the day) for a little personality. It's subtle by design — turn it off or edit the list (add/remove/reset) in **Settings → Accounts**.

### Focus & Pomodoro

Open a task in **Focus** mode to work distraction-free with a built-in **Pomodoro** timer. Completed sessions are logged per task, so you can see time actually spent vs. your estimate.

### Search & tag filters

**Search** looks across tasks, objectives, and journal entries. On list views you can switch into **tag filter mode** to show only tasks with a given tag. When filtering by a tag, **export** the resulting task list as **CSV** or clean **Markdown** from the Search page.

### Recurring tasks

Create **daily, weekly, or monthly** templates (managed in **Settings → Recurring Tasks**). Instances are generated automatically; editing one instance customises just that occurrence while the series rolls forward.

### Calendars & schedule

See calendar events beside your tasks in the **Schedule** tab of the day view's right-hand dock (alongside Inbox, Jira, and Goals):

- **Google Calendar** and **Fastmail Calendar** — connect in **Settings → Integrations / Accounts**.
- **CalDAV** — connect a CalDAV server and optionally push your time-blocks back to it.
- **ICS / webcal feeds** — subscribe to any read-only calendar URL.

**Scheduling tasks onto the calendar:**

- **Drag a task** onto the timeline to time-block it. The block moves the task into that day, snaps to the task's **planned time** (its estimate, or a 30-minute default), and edits in the list reflect live on the block.
- **Resize** a block's top/bottom edge to change its length — this updates the task's planned time.

**Calendar colours:** each calendar gets its own distinct brand hue automatically (including each individual Fastmail calendar — Family, personal, shared, etc.), and event blocks show the source calendar name so feeds stay distinguishable. In the schedule's **Calendars** panel (and **Settings → Calendars** for feeds) you can **show/hide** each calendar and **tap its colour dot to recolour** it. All your calendars are listed there, even ones with no events that day.

### Email → tasks

Turn email into tasks several ways:

- **Gmail / Fastmail:** star an email and it imports as a task (same OAuth app as sign-in for Gmail; an app password for Fastmail).
- **Task inbox:** forward (or auto-forward) mail to a dedicated address to create tasks; allow-list senders in settings.
- **AI title cleanup:** imported subjects are tidied into clean task titles by your local model — no API key, no data leaves your server. Part of the broader [AI assist](#ai-assist--local--private) suite; opt in during `install.sh` or in Settings → Integrations → AI.

The **Email** screen lets you triage incoming mail and convert messages to tasks, with the original linked.

### AI assist — local & private

Sempa can use a small language model to take friction out of capture and review.
**Everything runs on a model you host yourself (Ollama), on your own server. No
data is ever sent to any third party, there is no API key, and there are no usage
costs or rate limits.** If the model is unavailable, every feature simply falls
back to the manual path — nothing breaks.

**You are in complete control:**

- **Off by default.** AI does nothing until you enable it. Turn it on during
  `install.sh` (which installs Ollama, pulls the model, and tests the connection)
  or in **Settings → Integrations → AI**.
- **Per-feature toggles.** Each AI feature below has its own on/off switch in
  **Settings → Integrations → AI → “AI features”**. Turn off any you don't want and
  it disappears from the app. Features only appear when the model is reachable.
- **Model management.** See your downloaded models and their sizes, switch the
  active model, **download a new model with a live progress bar**, or **remove** one
  to reclaim disk — all from Settings. The default is `qwen2.5:1.5b` (~1 GB,
  CPU-friendly); browse alternatives at [ollama.com/library](https://ollama.com/library).
  Use any **chat/instruct** model (Sempa talks to Ollama's chat API). If a model
  errors with *"does not support generate/chat"*, its local copy is stale from an
  Ollama upgrade — re-pull it: `ollama pull <model>` (or remove + re-download in Settings).

Nothing is automatic or destructive — the model **suggests**, you approve.

**What each feature does:**

| Feature | Where | What it does |
|---------|-------|--------------|
| **Natural-language quick add** | ✦ on a task's title | Type something like *“lunch with Sam thu 1pm 30m #personal”* and it fills in the title, date, time, time-estimate, and tags. |
| **Suggest tags** | ✦ on the tag editor | Recommends tags for a task **only from your existing tag set**, so your taxonomy stays consistent. |
| **Break into subtasks** | ✦ on a task | Splits a task into a few concrete, ordered subtasks you can keep or edit. |
| **Title cleanup & summary** | Email / Jira import | Tidies imported email subjects (and issues) into concise, action-oriented task titles with a rough time estimate. |
| **Plan my day** | Plan Day | Suggests a focused order for today's tasks around your calendar events, with a one-line rationale — applied to your board only when you click. |
| **Draft weekly review** | Week Review | Drafts your **wins / challenges / next focus** from the week's completed tasks and objectives. You edit before saving. |
| **Reflection prompts** | Shutdown | Offers a couple of context-aware end-of-day questions based on what you did and didn't finish. |

### Jira

Connect Jira to import your assigned issues as tasks. See **[docs/JIRA.md](docs/JIRA.md)** for the full guide. In brief:

- **Rich import:** issues sync as tasks with a link to the ticket and the issue description copied into the task notes (offline-readable); re-sync keeps title/status/metadata current.
- **Manage from the task:** the editor shows a live Jira section — status, type, priority, assignee, labels, an "Open in Jira" link, and the issue's real workflow **transitions as buttons**. Marking a Jira task **done** also transitions the ticket.
- **Multi-select filters + your default view:** filter the Jira panel by any combination of Priority / Type / Status / Epic / Sprint (e.g. Status ∈ {To Do, Backlog}); your chosen filters and scope are saved as your default, with a Reset.
- **Delete returns it to Jira:** deleting a Jira-linked task re-imports it (immediate re-sync + background poller), so it goes back to the Jira list rather than disappearing.

### Reminders, notifications & routines

Configured in **Settings → Notifications**.

**Per-task reminders.** Set **Remind me** (date + time) on any task for a hard alert. It fires with two quick actions — **Mark done** and **Snooze 1h** — and tapping the notification opens the app on that task. Reminders are deduplicated, so re-arming or snoozing won’t double-fire.

**Delivery channels** (toggle each independently):

- **Web Push** — native OS notifications on Windows/Android browsers and installed PWAs. Click **Enable** to grant permission; it subscribes this device.
- **Native Android** — push to the installed Android app.
- **Custom webhook** — POST notifications to a self-hosted service such as **ntfy** or **Gotify**. Enter the endpoint URL, optional topic, HTTP method, and an auth header/token, then use **Send test notification** to verify the handshake.

**Alert sound.** Turn the custom sound on and pick from **10 calm tones** (Carbon Piano, Handpan, Kalimba, Waterside, and more) — each row has a **▶ preview**. The choice applies to in-app alerts, Android notification channels, and the desktop reminder.

**In-app routines.** Set the day/time for the **Weekly planning** prompt and the time + workdays for the **Daily shutdown** prompt. These appear as calm in-app banners (not OS alarms) that guide you into the matching workflow.

**Will reminders fire if I’m offline?** Yes, within reason:

| Situation | What happens |
|-----------|--------------|
| Server briefly down | The reminder fires when the server returns (late, not lost) |
| Android device fully offline / app closed | An **on-device OS alarm** still fires (scheduled locally from your tasks) |
| Settings changed offline | Saved locally and synced to the server on reconnect |
| Windows desktop, app running | Fires in-app with your chosen sound |
| Windows desktop, app closed | Reminders need the app running (no background push in the desktop shell) |

> Custom notification *sounds* play for the in-app/desktop reminder, the settings preview, and Android channels. A **background** web-push notification on a plain browser uses the OS default sound — a browser platform limit.

### Backup & restore

In **Settings → Backup & Restore**:

- **Automatic daily backups** at an hour you choose, with a retention count.
- **Encryption** — optionally protect backups with a passphrase.
- **Destinations** — keep backups locally or push to **S3**, **WebDAV**, or **Google Drive**. Use **Test** to verify a destination.
- **Run now** for an on-demand backup, **Download** the latest archive, or **Restore** from a file.

Database migrations run automatically on startup, and your data lives in the `sempa_data` Docker volume.

### Today board

The **Today** view is a rolling board of day columns with today anchored at the left edge. Scroll left into the past and right into the future **continuously** — there are no week boundaries to page through. The ‹ › header buttons and the `j`/`k` keys jump a week at a time; **Today** re-anchors on the current day.

### Desktop widget

The Windows desktop app includes a floating **Widget** — a compact, always-on-top panel showing what's up next with quick checkboxes and a quick-add box. Open it from the sidebar icon rail (or a single click on the system-tray icon); the tray's double-click opens the main window.

### Offline & sync

The desktop and Android apps are **local-first**: they keep a local copy of your data, so the app stays fully usable with no connection. Changes queue and **sync automatically** when the server is reachable again. A sync indicator shows status. Plain web (in a browser, not installed) talks directly to the server.

### Themes & appearance

In **Settings → Appearance** you pick from **six full-interface themes** — **Terracotta** (default), **Forest**, **Plum**, **Slate**, **OLED Black**, and **Ocean** — each with a live preview. Every theme has a light and a dark mode, except **OLED Black**, which is dark-only (the mode toggle hides for it). The same page has a **text-size** slider, the **sidebar grouping** + **section style** controls for the desktop nav rail, and the **contextual reflections** toggle. Your choice is remembered per device and applied before first paint (no flash on load).

### Updates

Sempa checks GitHub for newer releases and surfaces them in-app: a subtle indicator in the nav rail, an update toast (Download · What's new · Later), and **Settings → About**, which shows the current version, update channel (Stable/Beta), an automatic-checks toggle, when it last checked, and a manual **Check for updates**. On web and desktop this points you to the installer download — no setup required. Optional silent background self-update for the desktop app (download + restart-to-apply via `tauri-plugin-updater`) is documented in [`docs/UPDATER.md`](docs/UPDATER.md) and activates once an updater signing key is added to CI.

### Settings overview

| Section | What you configure |
|---------|--------------------|
| **Account** | Sign-in, profile |
| **Integrations** | Gmail, Fastmail, Jira, CalDAV, task inbox, and **AI** (local model: connection, model download/remove, and a per-feature on/off for each AI assist) |
| **Calendars** | Connected calendars, ICS/webcal feeds, show/hide, colours |
| **Tags** | Create/rename/recolour tags |
| **Recurring Tasks** | Daily/weekly/monthly templates |
| **Notifications** | Reminders, delivery channels, sounds, routines |
| **Backup & Restore** | Schedule, encryption, destinations |
| **Appearance** | Theme (six options), light/dark mode, text size, sidebar grouping, contextual reflections |
| **About** | App version, update channel, automatic update checks, "Check for updates" |

### Keyboard shortcuts

| Key | Action |
|-----|--------|
| `n` | New task (day view) |
| `e` | Edit the hovered task (day view) |
| `t` | Go to today |
| `j` | Previous week |
| `k` | Next week |
| `?` | Show shortcuts help |
| `Esc` | Close the open dialog |

---

## Upgrading

```bash
git pull
docker compose build
docker compose up -d
```

Database migrations run automatically on startup. Your data is in a Docker volume (`sempa_data`) and is preserved across rebuilds.

---

## Development

**Requirements:** Go 1.25+, Node.js 20+

```bash
# Backend (runs on :9001)
cd backend
go run ./cmd/server/

# Frontend (runs on :5173, proxies API to :9001)
cd frontend
npm install
npm run dev
```

The frontend dev server sets `VITE_API_URL=http://localhost:9001` automatically via `.env.development`. You can set `SEMPA_PASSWORD=dev` in your shell to enable auth locally.

### Building native apps

```bash
# Android (requires Android SDK)
cd frontend
npx cap sync android
npx cap open android   # opens in Android Studio

# Windows (requires Rust toolchain)
cd frontend
npm run tauri build
```

### Project structure

```
backend/
  cmd/server/        Entry point
  internal/
    api/             HTTP handlers
    config/          Environment-based config
    db/              SQLite stores + migrations
    integrations/    External service clients (Gmail, Fastmail, Jira, iCal)
frontend/
  src/
    routes/          SvelteKit pages
    lib/
      components/    Reusable UI components
      stores/        Svelte runes-based state
      api.ts         Typed API client
  src-tauri/         Tauri (Windows/macOS/Linux) desktop app
  android/           Capacitor Android wrapper
install.sh           Guided first-time setup (prereqs → config → build → start)
deploy/
  update.sh          Pull + rebuild script
```

---

## Philosophy

- **Single-user per instance.** Each person runs their own copy — like Gitea or Vaultwarden. Your data stays on your server.
- **No cloud dependency.** Runs fully offline once configured. External services (Gmail, Jira) are optional integrations.
- **Small footprint.** ~10 MB Docker image, ~20 MB RAM. SQLite database — no separate database server.
- **API-first.** Everything the frontend does goes through the REST API.

---

## Roadmap

- [x] Android app (Capacitor)
- [x] Windows desktop app (Tauri)
- [ ] Slack integration
- [ ] CalDAV write-back (create Sempa tasks as calendar events)
- [ ] Public Docker image on GitHub Container Registry

---

## Contributing

Bug reports, feature requests, and pull requests are welcome — see
[CONTRIBUTING.md](CONTRIBUTING.md) for the process, coding standards, and how to
run the tests. For security issues, please follow [SECURITY.md](SECURITY.md).

---

## License

Sempa is free and open-source software, licensed under the **GNU Affero General
Public License v3.0 or later** (AGPL-3.0-or-later) — see [LICENSE](LICENSE).

Copyright (C) 2026 William Moore

This program is free software: you can redistribute it and/or modify it under the
terms of the GNU Affero General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version. It is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU AGPL for more details.
