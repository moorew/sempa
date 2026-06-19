# Linux Tauri configuration

This directory holds the Linux-specific Tauri bundling inputs.

## `../tauri.linux.conf.json` (the overlay)

Tauri **deep-merges** `tauri.<platform>.conf.json` over `tauri.conf.json` on that
platform only. So `tauri.linux.conf.json` applies on Linux builds **exclusively** —
Windows/macOS keep `identifier: com.sempa.desktop` and the `nsis`/`msi` targets,
untouched.

On Linux it sets:

- `identifier: ca.sempa.Sempa` — the canonical reverse-DNS id for Flathub (we own
  `sempa.ca`). **Permanent once submitted to Flathub** — do not change it.
- bundle targets `deb` / `rpm` / `appimage`
- the `sempa://` deep-link scheme
- the Linux icon set + the `.desktop` template and extra installed files below

> Tauri's config is strict JSON with **no comment support** — a `//` key fails the
> build with `Additional properties are not allowed`. Keep notes here, not inline.

## `ca.sempa.Sempa.desktop` (Handlebars template)

The bundler renders this per package format, substituting `{{exec}}`. Brand-critical
fields (`Icon`, `StartupWMClass`, `MimeType`, launcher `Actions`) are hardcoded so
they're deterministic regardless of package format. A rendered standalone copy for
Flatpak/manual installs lives at `sempa-linux/desktop/ca.sempa.Sempa.desktop`.

## Icons & AppStream

The canonical desktop-integration files (hicolor tree, scalable SVG, `-symbolic`,
and `ca.sempa.Sempa.metainfo.xml`) live under `sempa-linux/desktop/` and are
installed into the package via the `files` map in the overlay.
