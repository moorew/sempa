# The Sempa Dock — Build Spec (Part II)

An always-on touch appliance for the desk — a Raspberry Pi running a purpose-built
fullscreen Sempa. Calm like a paper list, tangible, instant. **Not a browser pointed at
a URL** — a real product you boot into.

## 2.1 Product intent

The Dock makes tasks physical. It sits on your desk, always showing today, strung on the
day thread. You touch it to add, check, and reorder — no login dance, no app to open. It
should feel like a beautiful object, not a tablet running software.

> Already designed — the entire Dock UI (calm paper layout, day-thread spine, big touch
> targets, on-screen keyboard, 7″ and 5″ responsive) is built and interactive in
> `companion/Sempa Companion Screens.html` ("Raspberry Pi · Desk companion"). This spec
> turns that design into a shipping appliance.

## 2.2 Hardware

| Part | Recommended | Notes |
|---|---|---|
| Compute | **Raspberry Pi 5** (4 GB) | Pi 4 supported; Pi 5 is noticeably snappier for the WebView. Zero 2 W too weak. |
| Display (primary) | **Official Touch Display 2** — 7″ 1280×720 | 5-pt touch, DSI, no extra cabling. Reference panel. |
| Display (compact) | 5″ 800×480 DSI/HDMI touch | Supported via the responsive Dock layout. |
| Storage | A2 microSD 32 GB *or* USB-SSD | SSD preferred for longevity with logs/updates. |
| Power | Official 27W (Pi 5) / 15W (Pi 4) | Under-power causes touch glitches — spec the real PSU. |
| Extras | RTC module, stand/case | RTC keeps time across power loss without network. Stand angle ≈ 20–30°. |

## 2.3 OS image strategy

- **Base:** Raspberry Pi OS *Lite* (Bookworm, 64-bit) — no desktop environment.
- **Reproducible image** built with `rpi-image-gen` (or `pi-gen`): pre-installs the Dock
  app, kiosk stack, services, splash, and config so flashing a card yields a finished
  product.
- **Read-only root + overlayfs** with a small writable data partition (SQLite, tokens,
  logs). Survives yanked power — essential for an appliance.

## 2.4 Display server & kiosk

Wayland via **Cage** (single-app kiosk compositor) started by **greetd** autologin. Cage
launches the Dock app fullscreen — no chrome, no escape to a desktop.

```ini
# greetd → cage → sempa-dock (one app, fullscreen, Wayland)
[default_session]
command = "cage -d -- /opt/sempa-dock/sempa-dock"
user = "sempa"
```

> **Alternative:** **labwc** if you ever need more than one surface (e.g. an OS-update
> overlay). Cage is the right default: simplest, most robust, single-purpose. Avoid a full DE.

## 2.5 The Dock app (`sempa-dock`)

A dedicated **Tauri build** sharing the design system and sync core with the desktop
app, but touch-first with appliance behaviors. **Native, not a kiosk browser**, because
we need:

- Crisp bundled fonts & GPU compositing (V3D, dmabuf) for 60fps.
- Offline SQLite + Secret Service token storage on-device.
- The signed OTA updater (§2.11) and low input latency.

Boots straight into the day list; the on-screen keyboard is the one already designed; no
settings sprawl — just add, check, reorder, and a small paired-device status.

> **Build target:** compile for `aarch64` from the same monorepo (a second Tauri
> binary/target sharing crates with the desktop app). Window: fullscreen, undecorated,
> cursor hidden, no context menu.

## 2.6 Boot experience

- Quiet boot: `quiet loglevel=0 vt.global_cursor_default=0 logo.nologo`; hide the rainbow
  splash & console text.
- Custom **Plymouth** theme — the sempa mark on terracotta, gently breathing.
- Budget: power-on → calm first frame < **15s**. No flashes of console or white.

## 2.7 First-run & pairing

- **Wi-Fi:** headless via `firstrun.sh`/`cloud-init` baked at flash time, *or* an
  on-device first-run screen (pick network, on-screen keyboard) using NetworkManager.
- **Pairing:** the Dock shows a short code; you approve it from the web/desktop app,
  which mints a **scoped, revocable device token** (today + current week only). No account
  password ever lives on the device.
- **Time:** NTP when online; RTC otherwise.

## 2.8 Touch, display & backlight

- **Orientation:** `display_lcd_rotate` / Wayland output transform; landscape default,
  portrait supported.
- **Touch calibration:** map the touchscreen to the output transform; verify multi-touch
  & edge accuracy.
- **Backlight:** control via `/sys/class/backlight/*/brightness`. Day/night schedule,
  gentle auto-dim on idle, soft night floor; **tap-to-wake** to full.
- **No blanking:** disable DPMS/console blanking; the app owns dimming so the screen
  never hard-cuts to black.

## 2.9 Ambient & idle

After inactivity the Dock eases into an **ambient face**: date, current/next task on the
day thread, and progress — dimmed and serene, like a desk clock. A touch returns to the
full list. This is the "calm, reassuring, always-on" behavior the device is for.

## 2.10 Services & reliability

| | |
|---|---|
| `greetd` | Autologin → launches Cage → Dock app. |
| `sempa-dock` (via Cage) | `Restart=always`, watchdog, journald logging, memory cap. Auto-recovers within seconds. |
| Storage | Read-only root (overlayfs); writable data partition; logs to tmpfs/journald with size caps. |
| Network | NetworkManager auto-reconnect; offline queue keeps working; resync on link-up. |

## 2.11 Updates

- **App:** signed OTA via the Tauri updater on a quiet schedule (e.g. overnight), applied
  atomically with auto-restart. Same signing-key discipline as Part I.
- **OS:** `unattended-upgrades` for security, *or* A/B image updates (**RAUC**/Mender)
  for fleet-grade robustness.

> **Decision:** app-level OTA + apt security is simplest and fine for personal/small use.
> Choose RAUC A/B only if you'll run many docks and want guaranteed-rollback image updates.

## 2.12 Security & networking

- Scoped device token; revoke from the account anytime; the device holds no broad creds.
- SSH disabled by default (or key-only) on shipped images; outbound-only firewall
  (nftables/ufw).
- Secrets in the keyring or an encrypted file on the data partition; signed updates only.
- Optional LAN discovery (mDNS) for direct desktop↔dock sync when off the internet.

## 2.13 Physical & power

- Stand at a readable desk angle; tidy single-cable run; optional 3D-printed enclosure
  (ship STL files in the repo).
- Safe shutdown on the power button (systemd-logind); RTC preserves time; low idle draw &
  thermal headroom (no throttling at idle).

## 2.14 Acceptance

- [ ] Flash image → boots to a calm Sempa first frame in < 15s, no console/splash flashes.
- [ ] Cannot escape to a desktop; crash auto-recovers within seconds.
- [ ] Pairs with a scoped token; no account password on device; token revocable.
- [ ] Add / check / reorder all work by touch; on-screen keyboard usable.
- [ ] Survives power loss with no corruption (read-only root verified).
- [ ] Backlight day/night schedule + idle dim + tap-to-wake behave.
- [ ] Ambient face appears on idle and returns on touch.
- [ ] Offline edits queue and resync on reconnect.
- [ ] OTA app update applies atomically overnight and restarts clean.
- [ ] 60fps UI; touch latency feels immediate (< ~80ms).
