# Sempa for Linux — Build Spec (Part I)

The complete desktop app — every screen we already ship — delivered so it looks and
behaves like it belongs on GNOME, KDE Plasma, and everything between. Wayland-first,
HiDPI-perfect, portal-native, beautiful in both themes.

## 1.1 Design principles

Held to the same bar as a hand-built GNOME/KDE app — not a wrapped website.

- **Native, not foreign** — follow system color scheme, accent (optional), font
  scaling, window-button layout, reduced-motion. Nothing feels imported from another OS.
- **Calm by default** — the brand's quiet warmth survives: terracotta, cream, the day
  thread. No neon, no clutter, generous space.
- **Offline-first** — everything works without a network; sync is an enhancement, never
  a dependency.
- **Sandbox-clean** — least-privilege Flatpak; every capability via an XDG portal.
- **Pixel-perfect at any scale** — crisp at 100–200% and fractional scaling, one
  monitor or four. Vector assets only.
- **Keyboard-complete** — every action without a mouse; visible focus; AT-SPI exposed.

## 1.2 Architecture & stack

| Layer | Choice | Notes |
|---|---|---|
| Shell | **Tauri 2** | Reuse the existing Windows/macOS core. Linux webview is `webkit2gtk-4.1` (WebKitGTK 6 / GTK4 where available). |
| Core / logic | **Rust** | Sync engine, offline queue, secret access, background tasks, IPC. Shared across platforms. |
| Frontend | Existing web UI | Same app + design system. No Linux-specific fork. |
| Local store | **SQLite** (`sqlx`) | Cache + offline queue + FTS5 search. XDG/Flatpak data dir. |
| Realtime | WebSocket (`tokio-tungstenite`) | Same protocol as all surfaces; optimistic local writes. |
| Secrets | Secret Service (`oo7`/libsecret) | OAuth refresh tokens & device creds. Never plaintext. |

**Why WebKitGTK, not Chromium:** Tauri uses the system WebView — bundle ~10 MB vs
~150 MB, updates with the OS. Budget for WebKitGTK quirks (§1.13); the trade is worth a
native-weight Linux app.

## 1.3 App identity

| | |
|---|---|
| App ID (reverse-DNS) | `ca.sempa.Sempa` |
| Binary | `sempa` |
| Desktop entry | `ca.sempa.Sempa.desktop` · `StartupWMClass=ca.sempa.Sempa` |
| Icon theme name | `ca.sempa.Sempa` (hicolor + scalable SVG + symbolic) |
| URL scheme | `sempa://` (OAuth redirect + deep links) |
| AppStream | `ca.sempa.Sempa.metainfo.xml` (Flathub: summary, screenshots, releases, content rating, branding colors) |

> **Decision needed:** confirm the reverse-DNS namespace. `ca.sempa.*` assumes you own
> `sempa.ca`. The app ID is effectively permanent once on Flathub — set it before first
> submission.

## 1.4 Packaging & distribution

| Format | Role | Updates | Notes |
|---|---|---|---|
| **Flatpak → Flathub** *(canonical)* | Primary install | Flathub (automatic) | Sandboxed; portal-based; one build runs everywhere. Source of truth. |
| **AppImage** | Portable, no-install | Tauri updater (signed) | Bundle WebKitGTK deps carefully. |
| **.deb** | Debian/Ubuntu/Pop!/Mint | Tauri updater or APT repo | From `tauri build`. |
| **.rpm** | Fedora/RHEL/openSUSE | Tauri updater or DNF repo | From `tauri build`. |
| **AUR** *(community)* | Arch/Manjaro | `pacman` | Ship a `PKGBUILD` (`sempa-bin` or from source). |
| Snap | Optional | Snap Store | Lower priority; confinement fiddlier than Flatpak. |

**Recommendation:** lead with Flatpak/Flathub + the Tauri-built AppImage/deb/rpm from
CI. Add AUR (Arch users will ask). Snap is opt-in only.

## 1.5 Flatpak & portals

Runtime `org.freedesktop.Platform//24.08`. Least set of `finish-args`; everything else
via portals so users see real permission prompts.

```yaml
# ca.sempa.Sempa.yaml — finish-args (least privilege)
--socket=wayland            # Wayland-native; X11 fallback below
--socket=fallback-x11
--share=ipc                 # required with X11 fallback
--device=dri                # GPU compositing
--share=network             # sync + integrations
--talk-name=org.freedesktop.secrets   # Secret Service (tokens)
# NOT granted: no --filesystem=home. File access via portal only.
```

| Capability | Portal / interface |
|---|---|
| Open/save attachments, export | `org.freedesktop.portal.FileChooser` |
| Notifications | `org.freedesktop.portal.Notification` |
| Launch at login / background | `org.freedesktop.portal.Background` + Autostart |
| Open OAuth in browser | `org.freedesktop.portal.OpenURI` |
| Light/dark + accent + reduced-motion | `org.freedesktop.portal.Settings` (`org.freedesktop.appearance`) |
| Global quick-add shortcut | `org.freedesktop.portal.GlobalShortcuts` |
| Token storage | Secret Service (libsecret/oo7) |

## 1.6 Desktop integration

- **Icons:** scalable `ca.sempa.Sempa.svg` + symbolic `ca.sempa.Sempa-symbolic.svg`
  (tray/notifications) + rasterized hicolor 16–512. Reuse `assets/` / `icon-export/`.
- **Tray / status icon:** via `StatusNotifierItem` (KDE native; GNOME needs the
  AppIndicator extension). Tray is an **optional enhancement** (quick-add, today count,
  shutdown ritual) — never required for core use.
- **Single instance:** second launch focuses the window; routes `sempa://` deep links to it.
- **Global quick-add:** system shortcut pops a small centered Quick-Add window
  (mirrors the cockpit quick-add). Registered via GlobalShortcuts portal; rebindable.
- **Autostart:** opt-in "Start Sempa at login" → Background portal; minimize to tray.
- **.desktop actions:** `New task`, `Plan day`, `Shutdown ritual` as launcher actions.

## 1.7 Theming & native feel

**Window decorations.** Custom **in-brand titlebar** (Tauri `decorations:false`) that
behaves natively: read system window-button layout (close/min/max order & side) from
the Settings portal / GNOME `button-layout` and mirror it; double-click-maximize, drag,
edge-snap, right-click window menu. On KDE, respect SSD if the user forces it.

> **Decision:** custom titlebar (consistent brand, recommended) vs. server-side
> decorations (maximally native). Default custom-but-faithful; expose a "Use system
> title bar" toggle.

**Color scheme & accent.**
- Follow `org.freedesktop.appearance color-scheme` → auto dark/light, live-switching;
  manual override (System / Light / Dark).
- Default accent = brand **terracotta** (the identity). Offer "Match system accent"
  (GNOME 47+ / KDE).
- High-contrast: honor the portal flag with a dedicated AAA palette.

**Type & rendering.**
- Bundle **Plus Jakarta Sans**, **Hanken Grotesk** (wordmark), **JetBrains Mono** —
  don't depend on system availability. Offer "Use system UI font".
- Respect system text-scaling on top of display scaling.
- Grayscale/subpixel AA per system; never force a mode.

**Motion & shape.** Honor `prefers-reduced-motion` (disable entrance/thread animations,
keep instant states). Client-side rounded corners + soft shadow on Wayland CSD.

## 1.8 Windowing & responsive

- Remember geometry per-monitor; restore sanely on monitor changes; min size **880×600**.
- **Responsive breakpoints:** below ~1040px the sidebar collapses to an icon rail; below
  ~720px it becomes a bottom/overlay nav. Content reflows — never clips.
- **Cockpit mode:** when very wide-and-short (e.g. dragged onto an ultrawide strip /
  Corsair Xeneon Edge, ~32:9), switch to the horizontal cockpit layout from the companion
  study rather than letterboxing the standard view.

> Already designed — the wide cockpit and the small responsive layouts are mocked and
> interactive in `companion/Sempa Companion Screens.html`.

## 1.9 Data, sync & offline

- **Local SQLite** mirrors today + current week + journal + backlog; FTS5 for instant
  search; schema-versioned migrations.
- **Optimistic writes:** apply locally, enqueue, broadcast over WS; reconcile on ack.
- **Offline queue:** durable; replays on reconnect; **last-write-wins per field** with
  server clock; surfaces conflicts only when truly divergent.
- **Background sync** on a light interval + on focus + on network-up (NetworkManager
  state via D-Bus/portal).

## 1.10 Secrets & integrations

- OAuth (Gmail, Fastmail, Jira) opens the **system browser** via OpenURI; redirect
  returns through `sempa://` or a loopback listener. Refresh tokens → **Secret Service**.
- Calendar feeds (ICS/webcal) sync as today/week context. *Optional:* read system
  calendars via Evolution Data Server for a deeper Linux feel.
- All integration config mirrors the existing Settings → Integrations; no Linux-only
  divergence.

## 1.11 Notifications & background

- Daily plan reminder, calendar event alerts, the evening **shutdown-ritual nudge** at
  the chosen time.
- Respect Do-Not-Disturb / focus modes; coalesce; never nag.
- Background operation only with explicit opt-in via the Background portal; clearly
  indicated in the tray.

## 1.12 Accessibility

- AT-SPI tree via WebKit; every control labeled; landmark roles on nav/main/aside.
- Full keyboard operation; visible `:focus-visible`; logical tab order; shortcut
  cheat-sheet (`?`).
- Text scalable to 200% without breakage; contrast ≥ WCAG AA (AAA in high-contrast);
  reduced-motion respected.

## 1.13 Performance & WebKitGTK

- Cold start < **1.5s** to interactive on a mid-range laptop; idle RSS < ~220 MB.
- Validate the DMABUF renderer per driver; gate behind a setting/env
  (`WEBKIT_DISABLE_DMABUF_RENDERER=1`) as an escape hatch for broken stacks
  (some NVIDIA/old Mesa).
- Lazy-load routes; virtualize long lists (backlog, journal); avoid layout thrash on the
  day timeline.
- Smooth 60fps on day-thread/entrance animations; auto-disable under reduced-motion.

## 1.14 Updates

- **Flatpak:** updates via Flathub — no in-app updater needed.
- **AppImage / deb / rpm:** Tauri updater against a **signed** `latest.json`; reuse the
  existing in-app update UX. Channels `stable` / `beta`.
- Show release notes inline; never force; respect metered connections.

## 1.15 Build & CI

```
# GitHub Actions — produce every artifact per tag
matrix: [ x86_64, aarch64 ]
steps:
  - tauri build            # → .deb, .rpm, .AppImage
  - flatpak-builder        # → ca.sempa.Sempa.flatpak  (+ Flathub PR)
  - sign latest.json       # Tauri updater signing key (secret)
  - publish: GitHub Releases + Flathub + AUR bump
```

Reproducible builds; pin the runtime; updater private key in CI secrets only. The
`aarch64` artifacts double as the base for the Dock (Part II).

## 1.16 QA matrix & acceptance

| Axis | Must cover |
|---|---|
| Desktops | GNOME (Wayland), KDE Plasma 6 (Wayland), + an X11 fallback session |
| Distros | Fedora (current), Ubuntu LTS, Arch |
| Scaling | 100/125/150/175/200% incl. fractional & mixed-DPI multi-monitor |
| Theme | Light, Dark, auto-switch, high-contrast, system-accent on/off |
| Install | Flatpak, AppImage, deb, rpm — each clean-install & update verified |

**Acceptance checklist**

- [ ] Launches < 1.5s; window geometry restored per-monitor.
- [ ] Dark/light follow the system and switch live; reduced-motion honored.
- [ ] No `--filesystem=home`; all file/secret/notify access via portals.
- [ ] Crisp at every tested scale; no blurry text or icons.
- [ ] Full keyboard operation; AT-SPI nodes present; focus visible.
- [ ] Offline: full use; queue replays correctly on reconnect.
- [ ] OAuth round-trips through the system browser; tokens in the keyring.
- [ ] Cockpit mode triggers on ultrawide-short windows.
- [ ] Flathub AppStream validates (screenshots, releases, content rating).
