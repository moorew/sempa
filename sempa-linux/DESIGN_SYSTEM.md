# Sempa — Design System reference (for the Linux app & Dock)

One design system across every surface. Don't invent Linux-only styling — reuse the
existing frontend tokens. This is the quick reference.

## Brand palette

| Token | Hex | Use |
|---|---|---|
| Terracotta | `#b3592e` | Primary accent, app tile, theme color, filled primary buttons |
| Paper | `#f5efe7` | Light canvas (page background) |
| Card | `#ffffff` | Light raised surfaces (cards, panels, sheets) |
| Ink | `#2b221b` | Text, dark UI base |
| Amber | `#da9f62` | The "now" node on the day thread (light; `#e6a65a` dark) |

## App UI themes (terracotta, from `frontend/src/themes.css`, 1.27)

Six themes ship (terracotta default, forest, plum, slate, oled, ocean), each light +
dark; OLED is dark-only. All values below are the terracotta set. Components read the
`--sempa-*` tokens, never these hexes, so every theme follows automatically.

| Token | Light | Dark | Use |
|---|---|---|---|
| `--sempa-bg-main` / `-nav` / `--sempa-bg` | `#f5efe7` | `#14100d` | Canvas everything sits on |
| `--sempa-bg-rail` | `#e4dbcd` | `#0f0b08` | Icon rail |
| `--sempa-bg-panel` / `--card-bg` | `#ffffff` | `#221b16` | Raised surfaces |
| `--sempa-fill` | `#f6f0e8` | `#2b231d` | Inset fill on a surface (chips, inputs, segmented track) |
| `--sempa-track` | `#ebe2d6` | `#3a2f26` | Progress tracks, empty check rings |
| `--sempa-border` | `#e6ddd1` | `#33281f` | Row dividers inside a surface |
| `--sempa-text` | `#2b221b` | `#f4ede5` | Primary text |
| `--sempa-text-soft` | `#6f5f53` | `#bcab9c` | Secondary text, section labels |
| `--sempa-text-dim` | `#8a7b6e` | `#958476` | Meta text (holds ≥3.5:1) |
| `--sempa-accent` | `#b3592e` | `#e0814f` | Accent fills and icons |
| `--sempa-accent-ink` | `#8f411d` | `#f3a57d` | Accent-coloured text (AA on surfaces and accent-bg) |
| `--sempa-accent-bg` | `#f6e5d9` | `#3a2519` | Soft accent tint: active nav, soft buttons, selected rows |
| `--sempa-btn-bg` / `--sempa-btn-fg` | `#b3592e` / `#fffaf5` | `#b3592e` / `#fffaf5` | Filled primary buttons |
| `--sempa-amber` | `#da9f62` | `#e6a65a` | The "now" line and node |
| `--sempa-success` / `-bg` | `#4a8656` / `#e8f2ea` | `#6db27a` / `#1d2e22` | Done states |
| `--cal-terracotta-fg/-bg` | `#8a3c1a` / `#f8e4d7` | `#f3a57d` / `#3a2519` | Soft calendar/timeline block |
| `--cal-stone-fg/-bg` | `#6f5f53` / `#f1ebe2` | `#bcab9c` / `#2b231d` | ″ |
| `--cal-sage-fg/-bg` | `#2f5c3a` / `#e3efe4` | `#a3d3ad` / `#22342a` | ″ (also done blocks) |
| `--cal-amber-fg/-bg` | `#745214` / `#f7ecd3` | `#ecc985` / `#362c19` | ″ (also the current row on the Dock/cockpit) |

Elevation tokens: `--sempa-shadow-1` (cards), `--sempa-shadow-2` (sheets, popovers,
floating bars), `--sempa-shadow-accent` (primary buttons, the "now" block).

> Linux follows the **system** color-scheme by default (auto dark/light). Default accent
> is terracotta; "Match system accent" is opt-in.

## 1.27 surface model

"Cards on paper": calm, raised, rounded. The same rules apply to the app, the Dock,
cockpit mode, the desktop widgets and the Android home-screen widgets.

- **Cards on paper.** The page is the canvas (`--sempa-bg-main`). Cards, panels and
  sheets are `--sempa-bg-panel` with radius 18–28px. Nested blocks inside a card use
  `--sempa-fill` with no shadow. Dividers (`--sempa-border`) only separate rows inside
  a card.
- **Elevation, not borders.** A raised surface carries `--sempa-shadow-1` (or `-2` for
  sheets and popovers) instead of a 1px outline. Light themes tint the shadow with the
  theme's own ink; dark themes lift with lightness plus a faint top highlight.
- **OLED lifts with grey and a 1px edge.** Shadows vanish on true black, so OLED
  surfaces are a hair of grey (`#0e0e0e`) with a 1px `#1d1d1d` ring as their
  "shadow", and `--sempa-shadow-accent` is `none`.
- **Pills.** Every button is fully rounded (999px). Primary = `.btn-primary`
  (`--sempa-btn-bg` + `--sempa-shadow-accent`); secondary = `.btn-surface` (raised) or
  `.btn-soft` (accent tint); icon buttons are round. Min height 36px desktop, 44px
  touch (the Dock keeps its larger targets). Search fields are pills too.
- **Tints via `color-mix`.** Tags, projects and calendars render as soft blocks: set
  `--tint` to the source colour and use `.tint`, which mixes it into the surface (bg)
  and into the text (fg) at per-theme ratios. The one current/"now" block may be solid
  `--sempa-btn-bg` with `--sempa-shadow-accent`.
- **`--sempa-accent-ink` for accent text.** `--sempa-accent` is a fill colour; small
  accent-coloured text uses `--sempa-accent-ink` so it holds AA. Filled controls with
  light text always use `--sempa-btn-bg`, never `--sempa-accent` (in dark themes the
  accent is lighter and fails under white text).
- **Progress and checks.** Progress bars are 6px, fully rounded, track `--sempa-track`,
  fill accent. Checkboxes are round `.check-ring`s (22px; larger on the Dock); done is
  filled success with a check.
- **Type.** Sentence case everywhere: no tracked ALL-CAPS mono eyebrows (a content
  word like "NOW" is fine). Section labels 12px/600 in `--sempa-text-soft`. Times and
  durations stay JetBrains Mono.

## Type

| Role | Family |
|---|---|
| UI | **Plus Jakarta Sans** (400/500/600/700) |
| Wordmark | **Hanken Grotesk** (the "sempa" lockup) |
| Labels / figures / mono | **JetBrains Mono** |

Bundle all three with the app — never depend on system availability. Offer "Use system
UI font" as an option.

## The mark & the day thread

- **Cradle mark:** an open arc holding a single node — "a mind spacious enough to hold the
  day." SVG in `assets/mark.svg`.
- **Day thread:** tasks strung along a vertical/horizontal spine, the current one lit
  **amber** (the "now"). It's the core motif — used in the app sidebar, the cockpit
  timeline, and (centrally) the Dock list. Make tasks feel held, tangible. In 1.27 the
  current task also sits in a soft amber block (`--cal-amber-bg`) on the Dock and in the
  cockpit list, and the current timeline block is the one solid `--sempa-btn-bg` fill;
  the amber node and now-line stay as they are.

## Assets in this bundle (`assets/`)

| File | Use |
|---|---|
| `icon.svg` | Full-color rounded app tile: the day thread, first node checked off (1.29) |
| `mark.svg` | Bare cradle mark (logo, splash screens, installer art) |
| `logo.svg` / `logo-reversed.svg` | Wordmark lockup, dark-on-light / light-on-dark |
| `icon.ico` | Multi-size icon |
| `icon-256.png` / `icon-512.png` | Linux/desktop raster source |

Every app icon (these, the hicolor set and its `-symbolic` icon, Tauri, Windows tiles,
Android launcher incl. the theme-matched variants, web/PWA, iOS) is generated from one
definition by `frontend/scripts/icons.py`. Change the geometry or colours there and
re-run it; don't edit the PNGs by hand.

Fuller sets live in the design project: `icon-export/` (svg, desktop, web, android) and
`Sempa Brand Guidelines.html`.

## Reuse map

| Need | Existing artifact |
|---|---|
| Desktop responsive + cockpit layouts | `companion/Sempa Companion Screens.html` |
| Dock UI (paper list, thread, keyboard) | `companion/` — "Raspberry Pi · Desk companion" |
| In-app updater UX | `installer/` + `sempa-installer-handoff/` |
| Icons (all densities, .ico, svg) | `icon-export/` |
| Brand (color, type, voice) | `Sempa Brand Guidelines.html` |

## IDs, paths & env

| | |
|---|---|
| App ID | `ca.sempa.Sempa` · Dock: `ca.sempa.Dock` |
| Data (native) | `$XDG_DATA_HOME/sempa` |
| Data (Flatpak) | `~/.var/app/ca.sempa.Sempa/data` |
| Dock app | `/opt/sempa-dock/` · data on writable partition |
| GPU escape hatch | `WEBKIT_DISABLE_DMABUF_RENDERER=1` (broken GPU stacks) |
