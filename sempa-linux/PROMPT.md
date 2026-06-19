You are building **two native Linux products** for Sempa from our existing Tauri
monorepo: the **full desktop app** and the **Sempa Dock** (a Raspberry Pi touch
appliance). Everything you need is in the `sempa-linux/` folder I've added to the
repo: `LINUX_APP_SPEC.md`, `PI_DOCK_SPEC.md`, `DESIGN_SYSTEM.md`, and `assets/`.

**Read `LINUX_APP_SPEC.md` and `PI_DOCK_SPEC.md` in full before writing code.** Both
build from the same Rust core and web UI we already ship on Windows/macOS — do **not**
fork the frontend or reinvent the sync engine. The goal is a Linux app a discerning
GNOME/KDE user is proud to run, and a dock that feels like a real product, not a
browser in fullscreen.

Work in these phases and **pause for my review after each**:

### Phase 1 — Desktop app foundation
- Add the Linux Tauri target (`webkit2gtk-4.1`). App id `ca.sempa.Sempa`
  (confirm the reverse-DNS namespace with me first — it's permanent on Flathub).
- Wire identity: `.desktop` entry with `StartupWMClass`, `sempa://` URL scheme,
  icons from `assets/` (install hicolor sizes + scalable SVG; derive a `-symbolic`
  from `mark.svg`), and a stub `ca.sempa.Sempa.metainfo.xml` (AppStream).
- Single-instance + deep-link routing.

### Phase 2 — Native feel
- Custom in-brand titlebar that **behaves natively**: read window-button layout from
  the Settings portal and mirror order/side; double-click-maximize, drag, snap,
  right-click window menu. Add a "Use system title bar" toggle.
- Follow `org.freedesktop.appearance` color-scheme (live light/dark) with a manual
  override. Default accent = brand terracotta; add "Match system accent".
- Bundle the brand fonts; respect text-scaling, fractional/HiDPI scaling, and
  `prefers-reduced-motion`. High-contrast palette behind the portal flag.

### Phase 3 — Portals, data, secrets
- Least-privilege Flatpak manifest (`finish-args` per the spec — **no
  `--filesystem=home`**). Route file access, notifications, autostart/background,
  OpenURI, global shortcut, and settings through XDG portals.
- Local SQLite cache + offline queue + FTS5; optimistic writes; WS realtime;
  last-write-wins per field; resync on network-up.
- OAuth via the system browser → `sempa://` / loopback redirect; store refresh
  tokens in the **Secret Service** keyring. Mirror the existing Settings → Integrations.

### Phase 4 — Surfaces & polish
- Notifications (daily plan, calendar, evening shutdown nudge) respecting DND.
- Optional tray via `StatusNotifierItem`; global **quick-add** window via the
  GlobalShortcuts portal. Autostart opt-in via Background portal.
- Responsive breakpoints (sidebar → icon rail → overlay) and **cockpit mode** for
  ultrawide-short windows. Use `companion/Sempa Companion Screens.html` as the
  visual reference (it's in the design project; I'll paste screenshots if you need).
- Accessibility pass: AT-SPI labels, full keyboard nav, visible focus, 200% text.

### Phase 5 — Packaging & updates
- Produce **Flatpak** (Flathub-ready, with valid AppStream + screenshots),
  **AppImage**, **.deb**, **.rpm** via `tauri build`; ship an AUR `PKGBUILD`.
- Tauri updater against a **signed** `latest.json` for AppImage/deb/rpm (reuse our
  existing in-app update UX; Flatpak self-updates). Channels `stable`/`beta`.
- GitHub Actions matrix (`x86_64`, `aarch64`) emitting every artifact per tag;
  keep the updater private key in CI secrets.

### Phase 6 — The Sempa Dock (`sempa-dock`)
- A dedicated **`aarch64` Tauri build** sharing crates with the desktop app: touch-first
  UI (the Dock design in the companion file), fullscreen, undecorated, cursor hidden,
  on-screen keyboard, offline SQLite + keyring, signed OTA.
- Appliance image: Raspberry Pi OS Lite (Bookworm 64-bit), **read-only root +
  overlayfs**, **Cage** kiosk launched by **greetd** autologin, quiet boot + custom
  Plymouth, backlight day/night + idle dim + tap-to-wake, ambient idle face, systemd
  auto-recovery, scoped device-token pairing. Build a reproducible image via
  `rpi-image-gen`/`pi-gen`.

## Constraints & honesty
- Reuse the existing Tauri core, IPC/preload patterns, and design tokens
  (`DESIGN_SYSTEM.md`). One frontend for all surfaces — Dock differs by layout, not a
  rewrite.
- Be upfront about **WebKitGTK** quirks: validate the DMABUF renderer per driver and
  expose `WEBKIT_DISABLE_DMABUF_RENDERER=1` as an escape hatch. Don't pretend a quirk
  is fixed if you're working around it.
- The Dock is **native**, not a kiosk browser, so we keep crisp fonts, offline,
  secrets, the updater, and low input latency.
- Flag anything in the repo that conflicts (existing app id, icon paths, updater
  config, bundle targets) **before** overwriting.

When each phase is done, build it and walk me through verifying that spec's
acceptance checklist.
