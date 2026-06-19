# Sempa on Linux — capability → XDG portal map

How each OS capability is brokered under the Flatpak sandbox. The rule (LINUX_APP_SPEC §1.5):
least-privilege `finish-args`, everything else through a portal so the user sees a
real permission prompt. **No `--filesystem=home`.**

| Capability | How it's brokered | Status |
|---|---|---|
| Open/save attachments, backup/restore, export | `org.freedesktop.portal.FileChooser` — the app uses a plain `<input type="file">` (backup/restore) which WebKitGTK routes through the portal automatically under Flatpak. | ✅ no app change |
| Notifications (reminders, shutdown nudge) | `org.freedesktop.portal.Notification` via `tauri-plugin-notification` (`lib/desktopNotify.ts`). Portal-backed under Flatpak. | ✅ no app change |
| Open OAuth / external links in the browser | `org.freedesktop.portal.OpenURI` via `tauri-plugin-shell` `open` (`bridge.ts openExternal`). OAuth round-trips server-side; the device only opens the consent URL. | ✅ no app change |
| Light/dark + accent + reduced-motion + contrast | `org.freedesktop.portal.Settings` (`org.freedesktop.appearance`) — WebKitGTK maps these to the `prefers-color-scheme` / `prefers-reduced-motion` / `prefers-contrast` / `AccentColor` CSS features the frontend already consumes (Phase 2). | ✅ Phase 2 |
| Token storage (bearer token) | **Secret Service** (`org.freedesktop.secrets`) via the `keyring` crate + `secret_*` commands. The one host service we `--talk-name`. | ✅ Phase 3 |
| Launch at login / run in background | `org.freedesktop.portal.Background` (RequestBackground with autostart). `tauri-plugin-autostart` writes `~/.config/autostart`, which is sandboxed under Flatpak — the Background portal is the correct path. | ⏳ Phase 4 (autostart opt-in + tray) |
| Global quick-add shortcut | `org.freedesktop.portal.GlobalShortcuts` | ⏳ Phase 4 |

## Why these `finish-args` and nothing more

- `--socket=wayland` + `--socket=fallback-x11` + `--share=ipc` — display; ipc is required only for the X11 fallback.
- `--device=dri` — GPU compositing for WebKitGTK (60fps day-thread).
- `--share=network` — sync + integrations (REST + SSE to the Sempa server).
- `--talk-name=org.freedesktop.secrets` — the keyring. Portals themselves need no
  talk-name (the `org.freedesktop.portal.Desktop` bus name is always reachable in
  the sandbox), which is why FileChooser/Notification/OpenURI/Background/Settings
  appear nowhere in `finish-args`.

## Data, sync & offline (already shared across all surfaces — not reimplemented)

- **Local SQLite** via `tauri-plugin-sql` in the Flatpak data dir
  (`~/.var/app/ca.sempa.Sempa/data`). Same schema as every client.
- **Offline queue + last-write-wins per field**: `sync.svelte.ts` (`sync_log`
  table, PUSH/PULL with an `updated_at` cursor) + `src-tauri/src/sync.rs`.
- **Realtime**: Server-Sent Events (`/api/v1/events`, `stores/realtime.svelte.ts`),
  not WebSocket — functionally equivalent (<1s), reconnect-driven resync on
  network-up. We deliberately do **not** swap transports (don't reinvent the sync
  engine).
- **Search**: server-side today (LIKE-based). Local FTS5 is a noted future
  enhancement; the current local search works, so it's not on the Phase 3 path.
