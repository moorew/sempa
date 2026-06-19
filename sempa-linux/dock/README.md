# The Sempa Dock — appliance (Phase 6)

An always-on Raspberry Pi touch appliance that boots straight into today's tasks
on the day thread. **Native, not a kiosk browser**: it's the same Sempa binary
(aarch64) launched in **dock mode**, so it keeps crisp bundled fonts, offline
SQLite, the Secret Service keyring, the signed updater and low input latency.

> One frontend, two surfaces. The Dock differs by **layout**, not a rewrite — the
> touch UI is the `/dock` route (`frontend/src/routes/dock/+page.svelte`), built
> from the companion `PiCompanion` study (see `COCKPIT_AND_DOCK_LAYOUTS.md`).

## What's in this folder

| Path | Role |
|---|---|
| `greetd/config.toml` | Autologin → Cage → the Dock (no login screen; respawns on crash) |
| `bin/sempa-dock-session` | Session wrapper: appliance env + `cage -s -- sempa --dock` |
| `boot/config.txt.snippet` · `boot/cmdline.txt.snippet` | KMS driver, DSI panel, quiet/cursor-free boot |
| `udev/99-sempa-backlight.rules` | Let the `sempa` user write `/sys/class/backlight` (dim/wake) |
| `plymouth/sempa/` | Boot splash — the cream cradle mark breathing on terracotta |
| `pi-gen/stage-dock/` | Reproducible image stage (packages + install script) |

## App side (already in the main app)

- **Dock mode** (`src-tauri/src/lib.rs`): `--dock` / `SEMPA_DOCK=1` → fullscreen,
  decorations off, navigate to `/dock`. The cursor is hidden by the `/dock` CSS.
- **`/dock` UI**: paper list on the day-thread spine, big touch targets
  (rowH 78/60, check 52/42), add + check, an on-screen keyboard (QWERTY, actions
  pinned at the **top** — the Android-keyboard gotcha), a collapsible done
  cluster, and an **ambient idle face** after 90s (date, clock, current task,
  progress; tap to wake). Responsive 7″ (≥1100) / 5″. Light (paper) theme.
- **Offline + sync + secrets**: reuses the shipping local SQLite + offline queue
  + last-write-wins, and the Secret Service keyring for the device token.

## OS image (build on a Linux host)

1. Build the **aarch64 `.deb`** (the Linux release matrix produces it) and copy it
   to `pi-gen/stage-dock/01-sempa-dock/files/sempa-dock.deb`. Copy this repo's
   `sempa-linux/dock/` tree to `…/files/dock/`.
2. Clone **pi-gen** (Raspberry Pi OS image builder), add `stage-dock` after the
   Lite stage, base = **Raspberry Pi OS Lite (Bookworm, 64-bit)**, and build.
   The stage installs the app + kiosk stack, enables greetd autologin, sets the
   Plymouth theme, and applies the outbound-only firewall.
3. **Read-only root + overlayfs** with a writable **`/data`** partition (SQLite,
   token, logs) — apply via `raspi-config nonint enable_overlayfs` in a final
   `on_chroot` step, or the image post-process. The session wrapper points
   `XDG_DATA_HOME` at `/data/sempa`. This survives yanked power (PI_DOCK_SPEC §2.3).
4. Flash the image; it boots to a calm Sempa first frame.

> Not buildable on this dev host (no `pi-gen`/ARM/flatpak-builder, no display), so
> these configs are **un-runtime-tested** — they encode the spec and the standard
> Cage/greetd/Plymouth/pi-gen patterns. Verify on a real Pi 5 + Touch Display 2.

## Pairing (scoped, revocable device token) — implemented

No account password ever lives on the device (PI_DOCK_SPEC §2.7, §2.12):

1. First run with no token → the Dock shows a short **pairing code** (`/dock`
   pairing gate; it calls `POST /api/v1/devices/pair/start` and polls
   `GET /api/v1/devices/pair/status`).
2. You approve it from a signed-in app — **Settings → System → Paired devices**,
   enter the code (`POST /api/v1/devices/pair/approve`). Approval mints a normal
   **session token with an 8-day (current-week) TTL**, handed to the device
   **exactly once** via the status poll; the Dock stores it in the keyring.
3. Revoke any device from the same Settings panel (`DELETE /api/v1/devices/{id}`),
   which deletes the underlying session immediately.

Backend: migration `020_device_pairing.sql` + `db.PairingStore` +
`api/pairing.go` (covered by `pairing_test.go`). **Follow-up:** the device session
is currently TTL-bounded + revocable + token-only; fine-grained *path* scoping
(restricting the device to today/week endpoints) is a documented next step.

## Acceptance checklist (verify on hardware)

- [ ] Flash → calm Sempa first frame < 15s, no console/splash flashes.
- [ ] Cannot escape to a desktop; crash auto-recovers within seconds (greetd).
- [ ] Pairs with a scoped token; no account password on device; token revocable.
- [ ] Add / check all work by touch; on-screen keyboard usable on 7″ and 5″.
- [ ] Survives power loss with no corruption (read-only root verified).
- [ ] Backlight day/night + idle dim + tap-to-wake behave.
- [ ] Ambient face appears on idle and returns on touch.
- [ ] Offline edits queue and resync on reconnect.
- [ ] OTA app update applies atomically overnight and restarts clean.
- [ ] 60fps UI; touch latency feels immediate (< ~80ms).
