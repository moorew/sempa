# Sempa for Linux — packaging & release (Phase 5)

How the Linux artifacts are built, published, and updated. Maintainer-facing.

## Formats & roles

| Format | Built by | Updates |
|---|---|---|
| **Flatpak → Flathub** *(canonical)* | `flatpak-builder` (manifest in `flatpak/`) | Flathub (automatic) |
| **AppImage** | `tauri build` (CI) | Tauri updater (signed `latest.json`) |
| **.deb** | `tauri build` (CI) | Tauri updater, or an APT repo |
| **.rpm** | `tauri build` (CI) | Tauri updater, or a DNF repo |
| **AUR** (`sempa-bin`) | `aur/sempa-bin/PKGBUILD` | `pacman` (re-pull on version bump) |

## CI matrix

`.github/workflows/linux-release.yml` builds **x86_64** (`ubuntu-24.04`) and
**aarch64** (`ubuntu-24.04-arm`, free on public repos) natively, emitting
deb/rpm/AppImage for each. PRs that touch the app run the matrix as validation;
a `vX.Y.Z` tag attaches all artifacts to the GitHub Release. The aarch64
artifacts also seed the Sempa Dock (Part II).

## Updater (AppImage/deb/rpm)

The in-app update UX already exists (`stores/updates.svelte.ts`). Turning on the
**signed Tauri updater** is the maintainer key-gated procedure in
[`docs/UPDATER.md`](../docs/UPDATER.md) — the same keypair covers Windows and
Linux. Linux specifics:

- **AppImage** is the self-updating Linux target: with `createUpdaterArtifacts`
  on, the build emits `*.AppImage.sig`; the CI release step already attaches it.
- **Flatpak self-updates via Flathub** — it does NOT use the Tauri updater.
- **.deb/.rpm**: prefer a distro repo; the Tauri updater can also swap them.
- The updater manifest `latest.json` must be a **single combined file** carrying
  every platform key (`windows-x86_64`, `windows-aarch64`, `linux-x86_64`,
  `linux-aarch64`, …). Because Windows and Linux build in separate workflows,
  generate the per-platform signatures in each, then assemble one `latest.json`
  in a final release-assembly step (or move all builds into one workflow) — do
  **not** let two workflows each upload their own `latest.json` (they'd clobber).
- **Channels**: `stable` (tags `vX.Y.Z`) and `beta` (tags `vX.Y.Z-beta.N`). Point
  the beta build's endpoint at a `latest-beta.json`.

## Flathub submission (once, then automatic)

1. Finalize `flatpak/ca.sempa.Sempa.yaml`: pin a **git source** at the release
   tag (replace the `dir` source) and add generated **offline** sources, since
   Flathub builds with no network:
   - `flatpak-cargo-generator.py Cargo.lock -o cargo-sources.json`
   - `flatpak-node-generator npm frontend/package-lock.json -o node-sources.json`
   and reference both in the module.
2. Add **screenshots** to `desktop/ca.sempa.Sempa.metainfo.xml` (Flathub requires
   reachable `<image>` URLs) and a release entry per version.
3. Validate locally: `flatpak run org.flatpak.Builder --force-clean build-dir
   flatpak/ca.sempa.Sempa.yaml` and `appstreamcli validate` the metainfo.
4. Fork **`flathub/flathub`**, add `ca.sempa.Sempa` (the manifest + sources), open
   the submission PR. The app id `ca.sempa.Sempa` is **permanent** once accepted.
5. After acceptance, Flathub rebuilds on each manifest bump (a small PR to the
   `flathub/ca.sempa.Sempa` repo per release).

## AUR

`aur/sempa-bin/` holds the `PKGBUILD`. Per release: bump `pkgver`, refresh
`sha256sums` (`updpkgsums`), regenerate `.SRCINFO`
(`makepkg --printsrcinfo > .SRCINFO`), and push to the `ssh://aur@aur.archlinux.org/sempa-bin.git`
remote. See `aur/README.md`.
