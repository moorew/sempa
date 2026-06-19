# In-app updates

> **Audience: maintainers / people building Sempa from source.** This is a
> developer reference for how the updater works and how to enable signed silent
> auto-update. **If you just use Sempa, you don't need any of this** — updates
> surface automatically in the app; see [Updates](../README.md#updates) in the
> README for what that looks like.

Sempa ships with a **brand-controlled in-app update experience**:

- A subtle **update indicator** in the left rail when a newer version exists.
- An **update toast** (bottom-right) with *Download* / *What's new* / *Later*.
- **Settings → About**: current version, update channel (Stable / Beta),
  automatic-checks toggle, last-checked time, and a manual **Check for updates**.

## How it works today (live — no setup)

The checker polls the GitHub Releases feed for `moorew/sempa`
(`src/lib/stores/updates.svelte.ts`), compares the latest published tag against
the running build (`__APP_VERSION__`, injected from `package.json` by Vite), and
when a newer release exists surfaces the notes and the Windows installer
download link. The *Download* action opens the signed installer from the
GitHub release — the user runs it to update. This works on **web and desktop**
and needs **no code signing**.

The version is single-sourced from `frontend/package.json`; the release tag is
derived from it (`.github/workflows/tag-release.yml`). Keep
`frontend/src-tauri/tauri.conf.json` `version` and `Cargo.toml` `version` in
step when you bump.

## Turning on silent background auto-update (Tauri)

This upgrades desktop from "download + run installer" to "download in the
background → *Restart to apply*", using `tauri-plugin-updater`. It requires a
signing keypair that **only you can create** (the private half lives in CI
secrets). Everything else is already scaffolded and guarded so it has no effect
until you complete these steps.

### 1. Generate an updater signing key

```bash
# from frontend/
npx tauri signer generate -w ~/.sempa-updater.key
# prints a PUBLIC KEY and writes the PRIVATE KEY to the file (set a password)
```

### 2. Add the secrets to GitHub

In **Settings → Secrets and variables → Actions** add:

- `TAURI_SIGNING_PRIVATE_KEY` — contents of `~/.sempa-updater.key`
- `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` — the password you chose

`.github/workflows/windows-release.yml` already wires these env vars and a
guarded **Generate updater manifest (latest.json)** step that activates the
moment the secret is present.

### 3. Configure the app

In `frontend/src-tauri/tauri.conf.json`:

```jsonc
{
  "bundle": { "createUpdaterArtifacts": true },
  "plugins": {
    "updater": {
      "pubkey": "<PASTE THE PUBLIC KEY FROM STEP 1>",
      "endpoints": [
        "https://github.com/moorew/sempa/releases/latest/download/latest.json"
      ]
    }
  }
}
```

Add the Rust plugins in `frontend/src-tauri/Cargo.toml`:

```toml
tauri-plugin-updater = "2"
tauri-plugin-process = "2"
```

…and register them in `frontend/src-tauri/src/lib.rs`:

```rust
.plugin(tauri_plugin_updater::Builder::new().build())
.plugin(tauri_plugin_process::init())
```

Add the JS packages (`frontend/`):

```bash
npm i @tauri-apps/plugin-updater @tauri-apps/plugin-process
```

Grant the capability in `frontend/src-tauri/capabilities/*.json`:

```json
"updater:default", "process:allow-restart"
```

### 4. Flip the silent path on

In `src/lib/stores/updates.svelte.ts`, replace the body of
`tryDesktopSilentUpdate()` (it returns `false` today) with the real flow:

```ts
const { check } = await import('@tauri-apps/plugin-updater');
const { relaunch } = await import('@tauri-apps/plugin-process');
const update = await check();
if (!update) return false;
await update.downloadAndInstall((e) => {
  if (e.event === 'Progress') _onProgress?.(/* derive % from e.data */ 0);
});
await relaunch();
return true;
```

Then call it from the toast's *Download* action on desktop, falling back to the
browser download when it returns `false`.

### 5. Ship

Bump `package.json` (+ `tauri.conf.json` + `Cargo.toml`) version, push to `main`
→ the tag-release workflow tags it → the Windows release workflow builds, signs,
generates `latest.json`, and uploads it. Older desktop installs poll the
endpoint and update silently.

## Linux

The same keypair signs the Linux artifacts. With `createUpdaterArtifacts` on, the
Linux release workflow (`.github/workflows/linux-release.yml`) emits
`*.AppImage.sig` and already attaches it to the release (the `.sig` glob is
present and dormant until then). On Linux:

- **AppImage** is the self-updating target — `linux-x86_64` / `linux-aarch64` keys
  in `latest.json` point at the AppImage + its signature.
- **Flatpak self-updates via Flathub** and does **not** use this updater.
- **.deb / .rpm** update via a distro repo (or the Tauri updater swaps the file).

**One combined `latest.json`.** Windows and Linux build in separate workflows, so
each can only sign its own platforms. Don't let both upload a `latest.json` (they
clobber). Generate per-platform signatures in each workflow, then assemble a
single manifest carrying every key (`windows-*`, `linux-*`) in a final
release-assembly step — or consolidate into one release workflow.

## Channels

`stable` (tags `vX.Y.Z`) and `beta` (tags `vX.Y.Z-beta.N`). The About panel
already exposes the channel toggle; point the beta build's updater endpoint at a
separate `latest-beta.json` so beta testers track pre-releases without affecting
stable installs.
