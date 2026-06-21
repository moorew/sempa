# Security

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/13228/badge)](https://www.bestpractices.dev/projects/13228)

## Best practices & findings

Sempa follows the [OpenSSF Best Practices](https://www.bestpractices.dev/projects/13228)
criteria — the linked project page documents every criterion and its
justification. Live security findings are published to:

- **OpenSSF Best Practices assessment** — https://www.bestpractices.dev/projects/13228
- **OpenSSF Scorecard** — https://scorecard.dev/viewer/?uri=github.com/moorew/sempa
- **Code scanning (CodeQL / Trivy / Scorecard SARIF)** — the repo's
  [Security → Code scanning](https://github.com/moorew/sempa/security/code-scanning) tab
- **Dependency alerts** — [Security → Dependabot](https://github.com/moorew/sempa/security/dependabot)

## Automated scanning

Continuous checks run in CI via [`.github/workflows/security.yml`](.github/workflows/security.yml)
on every push/PR to `main` and on a weekly schedule:

| Check | Tool | Covers |
|-------|------|--------|
| SAST (code flaws) | **CodeQL** | Go backend + SvelteKit/TS frontend |
| Go dependency CVEs | **govulncheck** | Only CVEs reachable from your code |
| Deps · IaC · secrets | **Trivy** | npm/Go/Cargo CVEs, Dockerfile + compose misconfig, secrets |
| Secret scanning | **gitleaks** | Full git history |

Dependency update PRs are opened weekly by **Dependabot**
([`.github/dependabot.yml`](.github/dependabot.yml)) for Go, npm, Cargo, Docker,
and GitHub Actions.

Results land in the repo's **Security → Code scanning** tab where available, and
always in the workflow logs.

The full test suite (Go + frontend) runs in CI on every push/PR via
[`.github/workflows/test.yml`](.github/workflows/test.yml); `main` is protected,
so changes land only through reviewed PRs with these checks green.

**Dynamic analysis.** The same workflow runs Go tests under the **race detector**
(`go test -race`) and **fuzzes the parsers that consume untrusted external input**
(iCal feeds, imported emails, recurrence rules, and local-model output) with Go's
native fuzzing — a short smoke on every push/PR and a longer weekly sweep. Crashing
inputs are committed under `testdata/fuzz/` as permanent regression seeds. (The
languages in use — Go, TypeScript, Rust — are memory-safe, so memory-sanitizer/
DAST tooling for unsafe languages is not applicable.)

## Dependency & vulnerability remediation

- **Dependencies** are kept current by **Dependabot** (weekly PRs across Go, npm,
  Cargo, Docker, and GitHub Actions) and watched by **govulncheck** (Go,
  reachability-aware) and **Trivy** (npm/Go/Cargo CVEs). Trivy fails the build on
  fixable HIGH/CRITICAL CVEs.
- **Transitive dev-tooling CVEs** that have no upstream fix in the pinned parent
  are pinned forward with npm `overrides` (see `frontend/package.json`). These
  packages (Capacitor asset/icon generation, `xcode` project tooling) are
  build-time only — they ship in neither the server, the web bundle, nor the
  desktop/mobile apps.
- **Remediation targets:** medium-or-higher severity vulnerabilities that are
  reachable in shipped code are fixed promptly (well within 60 days of public
  disclosure); criticals are addressed as fast as practical. Findings that are
  not applicable to shipped artifacts are recorded under *Accepted findings*
  below with a rationale.

## GitHub-native protections

Alongside the CI scanning above, the repository has GitHub's native security
features enabled:

- **Secret scanning** + **push protection** — blocks commits containing known
  secret formats before they land
- **Dependabot alerts** + security updates — flags and patches vulnerable dependencies
- **CodeQL code scanning** — enabled via the workflow above

Results surface in the repo's **Security** tab.

## Handling secrets

- Never commit `.env`, `.env.local`, keystores (`*.keystore` / `*.jks`), or other
  credentials — these are git-ignored.
- Release signing material is injected in CI from repository secrets
  (`KEYSTORE_BASE64`, `KEYSTORE_PASSWORD`, `KEY_ALIAS`, `KEY_PASSWORD`,
  `GOOGLE_SERVICES_JSON`) — see the release workflows.
- If a secret is ever committed, rotate it first, then purge it from history.

## Accepted findings

Some scanner findings are deliberate, reviewed trade-offs rather than bugs. These
are dismissed in the Security tab with a justification and documented here:

- **CodeQL `go/request-forgery` (SSRF) — AI task-title cleanup model-server URL.**
  The AI task-title cleanup feature sends a request to an Ollama endpoint that the
  instance owner configures (Settings → Integrations, or `OLLAMA_BASE_URL`). By
  design that endpoint is a self-hosted/internal address (e.g.
  `http://ollama:11434`), so the usual SSRF mitigation (blocking internal hosts)
  would break the feature. The URL is settable only by the **authenticated owner**
  — who already controls the server — never by untrusted input, and the API
  validates it is a well-formed `http(s)` URL (`validModelServerURL`). The residual
  risk is accepted. See `backend/internal/integrations/fastmail/aititle.go`. If
  Sempa ever gains lower-privilege/multi-user roles, revisit this.

- **gtk-rs / `glib` advisories — Linux-only Tauri build graph.** The gtk-rs GTK3
  binding crates (`glib`, `atk`, `gdk*`, `gtk`, `gdk-pixbuf`, …) are pulled in
  transitively by the `webkit2gtk` webview Tauri uses **only on Linux**. The
  Windows desktop app uses the WebView2 runtime (no gtk/glib) and the Android app
  uses Capacitor, so these never appear there. The Linux desktop **does** ship
  (AppImage/deb/rpm), so the crates are in those artifacts. Two flavours of
  finding, both accepted:
  - *Soundness* (`glib` < 0.20, GHSA-wrw7-89jp-8q8g): a Rust soundness issue in
    the bindings (unsound `Variant`/value iterators), reachable only by specific
    misuse of those APIs in-process — Sempa never calls `glib` directly.
  - *Unmaintained* (e.g. **RUSTSEC-2024-0413** `atk`, and the sibling `gdk*`/`gtk`
    crates): the gtk-rs GTK3 repos were archived. These are *informational*
    "unmaintained" advisories, **not exploitable CVEs**, and **no patched version
    exists**.
  In every case the only real fix is the whole gtk-rs stack moving off GTK3,
  which is gated on Tauri/`wry`/`webkit2gtk` support — we pick it up with the next
  Tauri upgrade. Our gating scanners (`govulncheck`, Trivy, CodeQL) don't flag
  them; they surface only in OSV/Scorecard's posture report. Accepted until then.

- **CodeQL `go/request-forgery` (SSRF) — link-preview / unfurl fetcher.**
  `unfurl.Fetch` / `FetchImage` deliberately fetch a user-supplied URL (to build
  a link preview), but they are **mitigated**: every request is gated by
  `ValidatePublicURL`, which requires an `http(s)` scheme and refuses any host
  that resolves to a loopback/private/link-local/unspecified address — and the
  check is re-run on each redirect hop. (See the `isPrivateIP` table test.)
  CodeQL doesn't model the validator, so it still flags the call; accepted.

- **CodeQL `go/request-forgery` (SSRF) — notification webhook.** The reminder
  webhook posts to `cfg.Endpoint`, which the **instance owner** sets in
  notification settings. Sending to an owner-chosen URL *is* the feature, and on
  a single-user, self-hosted instance the owner already controls the host, so
  this is not a privilege boundary. Accepted (revisit if multi-user roles land).

- **CodeQL `go/unsafe-quoting` — AI quick-add prompt (`api/ai.go`).** The flagged
  string is a **natural-language prompt** sent to the local LLM, not structured
  data: `%q` safely escapes the user's text for display, and the *model's* reply
  is what gets parsed — then validated/filtered before use. Sempa never
  interprets this string as JSON, so there is nothing to "break out of." Not
  applicable.

- **OpenSSF Scorecard.** Most checks pass. Release artifacts are **cosign-signed**
  (keyless/Sigstore — see *Verifying release downloads* in the README), base
  images are **digest-pinned**, a server image is **published to GHCR** on each
  release, and the container **runs as a non-root user** (uid 10001). The
  remaining lower scores reflect this being a small, single-maintainer,
  recently-created project rather than fixable defects, and are accepted:
  - *Code-Review* / *Contributors* — a solo project has no second reviewer; these
    can't be satisfied without additional maintainers.
  - *Branch-Protection* — `main` blocks force-pushes and deletions and requires
    PRs via a ruleset, but "apply to administrators" and required approving
    reviews are intentionally **not** enabled: on a single-maintainer repo they
    would make the project unmergeable. Revisit if more maintainers join.
  - *Binary-Artifacts* — the only checked-in binary is the standard Gradle wrapper
    JAR (a verified Android build file).
  - *Maintained* — time-based ("created in the last 90 days"); resolves on its own.
  - *Vulnerabilities* — the ~18 findings are RUSTSEC advisories in the **Tauri Linux
    desktop build graph** (gtk-rs GTK3 / `webkit2gtk` stack) in `Cargo.lock` —
    overwhelmingly *unmaintained* advisories (e.g. RUSTSEC-2024-0413) with no
    patched versions, not exploitable CVEs. Fixed only by Tauri moving off GTK3.
    See the gtk-rs / `glib` entry above for the full rationale.

## Reporting a vulnerability

Open a private security advisory via **Security → Advisories**, or contact the maintainer directly.
