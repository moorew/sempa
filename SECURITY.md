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

- **`glib` < 0.20 unsoundness (GHSA-wrw7-89jp-8q8g) — Linux-only Tauri build
  graph.** `glib` 0.18 is pulled in transitively by the gtk-rs / `webkit2gtk`
  stack that Tauri uses **only on Linux**. Sempa ships a **Windows** desktop app
  (which uses the WebView2 runtime — no gtk/glib) and an **Android** app; there
  is **no Linux desktop release**, so `glib` never appears in a shipped artifact.
  The advisory is a Rust *soundness* issue in the gtk-rs `glib` bindings (unsound
  `Variant`/value iterators), reachable only by specific misuse of those APIs
  within the process — Sempa's own code never calls `glib` directly. A fix
  requires the whole gtk-rs stack to move to 0.20, which is gated on Tauri/`wry`
  support; we will pick it up with the next Tauri upgrade. Accepted until then.

## Reporting a vulnerability

Open a private security advisory via **Security → Advisories**, or contact the maintainer directly.
