# Jira integration

Sempa syncs Jira issues into tasks you can plan on your board, and lets you act
on them (open, change status) without leaving Sempa. Filtering is offline‑first:
it runs over already‑synced data, so the panel stays fast and works without a
live Jira connection.

## Setup

Settings → Integrations → Jira. You provide:

| Field | Notes |
|-------|-------|
| **Host** | e.g. `https://yourcompany.atlassian.net` |
| **Email** | the account email for the API token |
| **API token** | an Atlassian API token (stored encrypted; redacted on read) |
| **JQL** | which issues to import. Default: `assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC` |

Auth is HTTP Basic (`email:api_token`). Use **Test** to verify, then **Save**.

## How sync works

- Issues matching your JQL are imported as tasks with `source = "jira"` and
  `source_id = <ISSUE-KEY>`.
- Each task stores a **link** to the issue (`source_url` → `…/browse/KEY`) and a
  `source_metadata` blob: key, status, status category, type, priority,
  assignee, "mine" flag, and epic.
- On **first import** the issue **description is copied into the task notes** so
  a dragged‑in task carries real context (offline‑readable and searchable).
  Existing tasks whose notes are still empty are **backfilled** on the next
  sync; notes you've edited are never overwritten.
- Re‑sync updates the title, metadata and link in place (dedupe by issue key),
  so status/priority/assignee drift is kept current.

### When sync runs
- **Automatically** on the background poller (same cadence as the mail inbox),
  so the panel stays fresh on its own.
- **Manually** via the sync button in the Jira panel.
- **After deleting** a Jira‑linked task (see below).

## The Jira panel (right sidebar → Jira tab)

- Lists synced Jira tasks. Drag one onto a day to plan it (creates/links the
  task on that day).
- **Scope** (Sempa‑side): Unplanned / Planned / All.
- **Facet filters** — all **multi‑select** (pick any number of values; within a
  facet the match is OR, across facets it's AND):
  - Toggles: **Open** (hide done), **Assigned to me**.
  - Selects: **Priority**, **Type**, **Status**, **Epic**, **Sprint** — e.g.
    Priority ∈ {P1, P2}, Status ∈ {To Do, Backlog}.
- **Your default view** — whatever filters + scope you set are **saved and
  restored** automatically; they become your default every time you open Sempa.
  "Reset to default view" returns to the built‑in defaults (Open + Assigned to
  me, Unplanned).
- Fuzzy search by key or title (the search box is always transient — never
  saved into your default).

## Working with a Jira task

Open a Jira task to see a live **Jira section** in the editor:

- Current **status, type, priority, assignee, labels**, fetched live.
- A prominent **Open in Jira** link.
- The issue's **real workflow transitions** as buttons (e.g. "Start progress",
  "Done"). Clicking one transitions the issue in Jira and updates the shown
  status — no fragile status mapping; it respects each project's workflow.

The description copied at import time lives in the task **Notes** (so it's there
offline); the Jira section is live (needs a connection).

## Status & lifecycle

- **Completing** a Sempa task that's linked to Jira transitions the ticket to
  its *Done* category transition (best‑effort).
- **Deleting** a Jira‑linked task does **not** touch Jira. The ticket returns to
  the Jira pool: a re‑sync is kicked immediately on delete (and the poller would
  re‑import it anyway), so it reappears in the panel as an unplanned item ready
  to drag again. (This is why a deleted Jira task "comes back".)
- The Sempa status of an imported issue starts as `backlog`; the true Jira
  status is always shown from `source_metadata` / the live section.

## Offline behaviour

Filtering, the issue link, and the imported notes all work offline (they read
local data). The live Jira section and transition buttons need a connection to
your Sempa server (which proxies Jira); offline they simply show "Loading…".

## Troubleshooting

- **An issue isn't showing** — check your saved filters (Reset to default view)
  and that it matches your JQL (`statusCategory != Done` hides done issues).
- **Descriptions missing on old tasks** — trigger a sync; the backfill fills
  empty notes on that pass.
- **A transition isn't offered** — Jira only exposes transitions valid from the
  issue's current status in its workflow.
