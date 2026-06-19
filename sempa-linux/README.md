# Sempa for Linux & the Dock — handoff for Claude Code

Everything needed to build **two native Linux products** from your existing Tauri
monorepo:

1. **Sempa for Linux** — the full desktop app, native on GNOME/KDE/everything.
2. **The Sempa Dock** — an always-on Raspberry Pi touch appliance.

Hand the whole folder to Claude Code in your repo.

```
sempa-linux-handoff/
├─ PROMPT.md                 ← paste this to Claude Code to kick off the BUILD
├─ WEBSITE_UPDATE_PROMPT.md  ← separate prompt: website changes once it ships
├─ LINUX_APP_SPEC.md         ← full spec, desktop app (Claude reads this)
├─ PI_DOCK_SPEC.md           ← full spec, Pi dock appliance
├─ DESIGN_SYSTEM.md          ← colors, type, assets, reuse map
└─ assets/
   ├─ icon.svg / mark.svg            (app tile + bare cradle mark)
   ├─ logo.svg / logo-reversed.svg   (wordmark, dark-on-light / light-on-dark)
   ├─ icon.ico                       (multi-size)
   └─ icon-256.png / icon-512.png    (Linux/desktop source)
```

## How to use it

**1. Drop the folder into your app repo**, e.g. as `sempa-linux/` at the root.
(Rename to taste — just keep the prompt's paths consistent.)

**2. Run Claude Code in the repo and paste the build prompt:**
```bash
cd /path/to/your/app-repo
claude
```
then paste the contents of `sempa-linux/PROMPT.md` (or `claude < sempa-linux/PROMPT.md`).

**3. Let it work in phases** (the prompt is staged so you can review between steps),
then verify against the acceptance checklists at the bottom of each spec.

**4. When it ships,** run `WEBSITE_UPDATE_PROMPT.md` against your **website** repo to
add Linux downloads and the Dock product page.

## Prerequisites Claude will ask about

- The release URL / update server for the Tauri updater (AppImage/deb/rpm), and the
  **updater signing keypair** (keep the private key in CI secrets only).
- Your canonical **reverse-DNS app id** — this spec assumes `ca.sempa.Sempa`
  (i.e. you control `sempa.ca`). It is effectively permanent once on Flathub.
- The **device-pairing API** for the Dock (mint a scoped, revocable device token).

## Visual references (in this design project, not the bundle)

- `Sempa Linux & Dock — Build Spec.html` — the same spec, formatted & printable.
- `companion/Sempa Companion Screens.html` — **interactive** reference for the
  desktop responsive/cockpit layouts **and** the entire Dock UI.
- `Sempa Brand Guidelines.html`, `icon-export/` — brand + full icon set.

## The payoff

One design system, two builds a Linux user is proud to run: a portal-clean,
HiDPI-perfect desktop app that follows the system theme — and a calm, tangible
desk dock that boots straight into your day.
