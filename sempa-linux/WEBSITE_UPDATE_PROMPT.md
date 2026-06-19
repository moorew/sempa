# Website updates — run this AFTER Sempa for Linux & the Dock ship

> Separate from the build. Paste this into Claude Code **in the website repo** once the
> Linux app is on Flathub / GitHub Releases and the Dock is real. Don't publish any of
> this until the artifacts exist — gate behind a feature flag / draft until release day.

You are updating the **Sempa marketing website** to launch **Sempa for Linux** and the
**Sempa Dock**. Match the existing site's brand, components, and tone (terracotta
`#b3592e`, cream `#f7f3eb`, amber `#da9f62`; Plus Jakarta Sans + Hanken Grotesk wordmark
+ JetBrains Mono). Reuse existing components; don't introduce a new design language.

Work in phases; pause for review after each.

## Phase 1 — Downloads
- Add **Linux** to the downloads page with a **smart OS-detect** primary button: on
  Linux, recommend **Flatpak**; otherwise show the full list. Detect arch (x86_64 /
  aarch64) where possible.
- Formats + canonical install snippets:
  - **Flathub** (primary) — badge + `flatpak install flathub ca.sempa.Sempa`
  - **AppImage** — direct download, "chmod +x & run" note
  - **.deb** (Debian/Ubuntu/Pop!/Mint), **.rpm** (Fedora/openSUSE)
  - **AUR** — `yay -S sempa` (or the exact package name once published)
- Pull live version/links from GitHub Releases / Flathub; never hardcode a version that
  can drift. Show "stable / beta" channel toggle.
- Verify the **Flathub app id** (`ca.sempa.Sempa`) before linking — a wrong id 404s.

## Phase 2 — "Sempa for Linux" story
- A landing section/page selling the native angle: follows your system theme & accent,
  HiDPI-perfect, portal-clean privacy (no broad filesystem access), offline-first,
  keyboard-complete.
- **Screenshots:** light + dark, GNOME + KDE. (Capture from the real build; until then,
  use the mockups in the design project as placeholders behind the flag.)
- Cross-link to the existing feature set — it's the *same app*, now on Linux.

## Phase 3 — The Sempa Dock product page
- New page `/dock`: hero (the calm desk object), "what it is," **how it works**
  (pair with a code from the app → it shows today, always on), the day-thread story
  ("make your tasks tangible").
- **Specs / what you need:** Raspberry Pi 5, official Touch Display 2 (7″) or a 5″ panel,
  power, stand. Be honest that it's a Pi appliance.
- **Buy vs. build:** if you sell a kit, add it; otherwise a clean **"Build your own"**
  guide (flashable image download, BOM, optional STL enclosure files). Link the image +
  setup docs.
- Gallery; an ambient-face shot; light/dark.

## Phase 4 — Plumbing
- **Nav & footer:** add "Linux" (download) and "Dock" (product). Update the platforms /
  compatibility matrix to include Linux and the Dock.
- **System requirements** page: add the Linux row (Wayland/X11, glibc note, WebKitGTK
  comes from the OS) and the Dock hardware.
- **Changelog / release notes:** include the Linux release channel; auto-render from
  releases if the site already does that for other platforms.
- **SEO & social:** per-page `<title>`/meta/OG images for the Linux page and `/dock`;
  add `SoftwareApplication` structured data for the app and `Product` for the Dock.
- **Analytics:** fire a download event per format (`flatpak`/`appimage`/`deb`/`rpm`/`aur`)
  and a `dock_interest` event on the Dock CTA.
- **A11y & perf:** new pages keyboard-navigable, alt text on every screenshot, images
  responsive/lazy, Lighthouse ≥ existing pages.

## Constraints & honesty
- Do **not** announce availability that isn't real yet — keep everything behind a flag /
  in draft until the artifacts are published, then flip one switch.
- Don't fabricate distro support, a hardware kit, or benchmarks. If something's "build
  your own," say so plainly.
- Reuse existing components, tokens, and copy voice; this is an extension of the site,
  not a redesign.
- Flag any existing download/version logic you'd change **before** editing it.

When done, show me the new pages in dev, the OS-detect logic across a few user agents,
and the structured-data validation.
