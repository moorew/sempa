# iOS build (Capacitor + GitHub CI)

Sempa's iOS app is built on a **macOS GitHub Actions runner** — no Mac purchase
required. The Xcode project is **not committed**; it's generated in CI each run
(`cap add ios --packagemanager Cocoapods`), so iOS config lives in
`frontend/capacitor.config.ts` plus the PlistBuddy steps in
`.github/workflows/ios-release.yml`.

**Build notes (why the workflow is shaped the way it is):**
- **CocoaPods, not SPM.** Capacitor 8 defaults `cap add ios` to Swift Package
  Manager, but the official plugins' SPM packages don't resolve against core 8.4
  (sqlcipher.swift wants Swift-6 tools, and `CAPPluginCall`/`PluginConfig` APIs
  skew). Forcing CocoaPods builds the `.xcworkspace` with compatible plugin pods.
- **Xcode 16.** The workflow selects the newest Xcode 16 on the runner (sqlcipher
  + the toolchain need Swift 6 tools; the default Xcode 15.4 fails).
- **`@capacitor-community/sqlite`** ships an iOS pod, so the offline-first DB
  works on iOS out of the box.

## Two phases

1. **Simulator build** — runs always, needs no Apple account. Compiles the app
   unsigned for the iOS Simulator, proving the codebase builds on iOS. This is the
   green check that says "Sempa builds for iOS."
2. **TestFlight** — runs only on a `v*` tag **and** only when the App Store
   Connect API-key secrets below are set. Archives a signed build with automatic
   signing and uploads it to TestFlight.

## What you need for TestFlight (one-time, ~20 min)

1. **Apple Developer Program** membership ($99/yr).
2. An **App Store Connect API key** (App Store Connect → Users and Access → Integrations → App Store Connect API → generate a key with **App Manager** access). Download the `.p8` (you can only download it once).
3. Register the app's bundle id **`com.clevercode.sempa`** (Certificates, IDs & Profiles → Identifiers) and create the app record in App Store Connect. (Automatic signing with `-allowProvisioningUpdates` can create the signing profile for you.)

Then add these **repository secrets** (Settings → Secrets and variables → Actions):

| Secret | Value |
|---|---|
| `IOS_ASC_KEY_ID` | the API key's Key ID |
| `IOS_ASC_ISSUER_ID` | the API Issuer ID (top of the Keys page) |
| `IOS_ASC_API_KEY_P8` | the `.p8` file, base64-encoded (`base64 -i AuthKey_XXXX.p8 | pbcopy`) |
| `IOS_TEAM_ID` | your Apple Developer Team ID (Membership page) |

With those set, push a `vX.Y.Z` tag and the **iOS Release** workflow archives,
exports, and uploads to TestFlight. Without them, the workflow still runs the
simulator build and skips TestFlight.

## Decisions baked in
- **Bundle id / name:** `com.clevercode.sempa` / "Sempa" (matches Android).
- **App Transport Security:** arbitrary loads allowed — Sempa connects to a
  user's self-hosted server, often plain `http://` over a private tailnet, which
  iOS blocks by default. (Scope this to your domain later if you move to https.)
- **Deep links:** `com.clevercode.sempa://` URL scheme (mirrors Android).
- **SQLite / offline:** `@capacitor-community/sqlite` ships an iOS pod, so the
  local-first DB works on iOS out of the box.

## Not yet ported (Android-only today; iOS follow-ons)
- **Push notifications** — needs APNs: a Firebase iOS app + `GoogleService-Info.plist`,
  an APNs key in Firebase, the `@capacitor/push-notifications` iOS setup, and Push
  + Background-Modes capabilities.
- **Home-screen widgets** — Android Glance has no iOS equivalent; would need a
  Swift WidgetKit extension.
- **Focus-timer notification** — the Android foreground-service chronometer
  (v1.10.0) has no iOS analog; the iOS version is a **Live Activity** (ActivityKit,
  Swift). The in-app focus timer works on iOS already; `focusTimerNotification.ts`
  safely no-ops there.
- **Haptics** — the Android `SempaHaptics` JS bridge is Android-only; iOS would
  use `@capacitor/haptics`. Haptics simply don't fire on iOS until then.
