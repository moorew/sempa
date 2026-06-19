# Sempa — Design System reference (for the Linux app & Dock)

One design system across every surface. Don't invent Linux-only styling — reuse the
existing frontend tokens. This is the quick reference.

## Brand palette

| Token | Hex | Use |
|---|---|---|
| Terracotta | `#b3592e` | Primary accent, app tile, theme color |
| Cream | `#f7f3eb` | Light surfaces, reversed text |
| Ink | `#2b2620` | Text, dark UI base |
| Amber | `#da9f62` | The "now" node on the day thread |
| Sand | `#efe6d6` | Warm light background |

## App UI themes (from the shipping frontend)

**Dark (default)**
```
bg #181310   nav/panel #1d1510   rail #130e09   border #2e2016
text #f0ece4 soft #b8a89a        dim #7a6a5e
accent #cc6e3a  abg #2d1d12      amber #da9f62  success #5e9e6b
```

**Light**
```
bg #ece2d1   panel #fbf7f0   nav #eae0cf   border #e0d3bd   hair #e8dcc9
text #2b2620 soft #6f6151    dim #a08c75
accent #b3592e  abg #f1e3d2   amber #bd7f2c  success #4e8a5b
```

> Linux follows the **system** color-scheme by default (auto dark/light). Default accent
> is terracotta; "Match system accent" is opt-in.

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
  timeline, and (centrally) the Dock list. Make tasks feel held, tangible.

## Assets in this bundle (`assets/`)

| File | Use |
|---|---|
| `icon.svg` | Full-color rounded app tile (source for hicolor PNGs) |
| `mark.svg` | Bare cradle mark — derive the `-symbolic` icon from this |
| `logo.svg` / `logo-reversed.svg` | Wordmark lockup, dark-on-light / light-on-dark |
| `icon.ico` | Multi-size icon |
| `icon-256.png` / `icon-512.png` | Linux/desktop raster source |

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
