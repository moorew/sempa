/**
 * In-app update checker.
 *
 * LIVE (this file): polls the GitHub Releases feed for the repo, compares the
 * latest published version against the running build (__APP_VERSION__), and
 * exposes whether an update is available plus its release notes and the Windows
 * installer download URL. This powers the rail indicator, the update toast, and
 * the Settings → About panel on both web and desktop — no code signing needed.
 *
 * SCAFFOLD (tryDesktopSilentUpdate): the hook where a real Tauri
 * tauri-plugin-updater silent download+install plugs in. It is a no-op stub
 * today; once the signing key is wired up (see docs/UPDATER.md) it returns true
 * and the desktop app updates in the background instead of opening the browser.
 */

import { isCapacitor, hasLocalDb } from '$lib/platform';

const REPO = 'moorew/sempa';
const LAST_CHECK_KEY = 'sempa_update_last_check';
const DISMISSED_KEY  = 'sempa_update_dismissed';  // release version the user dismissed
const CHANNEL_KEY    = 'sempa_update_channel';     // 'stable' | 'prerelease'
const AUTO_KEY       = 'sempa_update_auto';        // '0' disables background checks
const CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000;       // re-check at most every 6h
// When a new release is published but its installer asset hasn't finished
// uploading yet (the release CI is still building), poll on this shorter cadence
// so the prompt appears soon after the build lands — not up to 6h later.
const ASSET_WAIT_MS = 10 * 60 * 1000;               // re-check every 10 min…
const ASSET_WAIT_MAX_TRIES = 6;                     // …for up to ~1h, then give up

export type UpdateChannel = 'stable' | 'prerelease';

export type UpdateInfo = {
  version: string;      // latest release version, no leading "v"
  notes: string;        // release notes (markdown / plain)
  url: string;          // release page ("What's new")
  downloadUrl: string;  // platform-correct asset (APK on Android, installer on Windows), else the release page
  hasAsset: boolean;    // true when a real installer asset for this platform exists in the release
  publishedAt: string;
};

const CURRENT = typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : '0.0.0';

function ls(): Storage | null {
  return typeof localStorage !== 'undefined' ? localStorage : null;
}

/** Numeric semver compare: returns true when `a` is strictly newer than `b`. */
function isNewer(a: string, b: string): boolean {
  const pa = a.replace(/^v/, '').split(/[.\-+]/).map((n) => parseInt(n, 10) || 0);
  const pb = b.replace(/^v/, '').split(/[.\-+]/).map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const d = (pa[i] || 0) - (pb[i] || 0);
    if (d !== 0) return d > 0;
  }
  return false;
}

type Asset = { name: string; browser_download_url: string };

/** The x64 NSIS installer if present (matches the release workflow names), else
 *  any .exe/.msi. Returns null when the release has no Windows installer yet. */
function findWindowsAsset(assets: Asset[]): string | null {
  const byPref = (re: RegExp) => assets.find((a) => re.test(a.name))?.browser_download_url;
  return (
    byPref(/x64.*setup\.exe$/i) ||
    byPref(/setup\.exe$/i) ||
    byPref(/x64.*\.msi$/i) ||
    byPref(/\.(exe|msi)$/i) ||
    null
  );
}

/** The Android APK if present, else null. (The `.aab` is for the Play Store, not
 *  a sideload — never offer that to the user.) */
function findAndroidAsset(assets: Asset[]): string | null {
  return assets.find((a) => /\.apk$/i.test(a.name))?.browser_download_url ?? null;
}

/** The installer asset for the running platform — Android (Capacitor) gets the
 *  APK, desktop/web the Windows installer — or null if it isn't published yet.
 *  Returning null lets the store hold back the prompt until the asset exists,
 *  so tapping "Download" never dumps the user on the GitHub page mid-build. */
function findPlatformAsset(assets: Asset[]): string | null {
  return isCapacitor() ? findAndroidAsset(assets) : findWindowsAsset(assets);
}

function createUpdateStore() {
  let checking = $state(false);
  let info = $state<UpdateInfo | null>(null);
  let error = $state('');
  let lastChecked = $state<string | null>(ls()?.getItem(LAST_CHECK_KEY) ?? null);
  let channel = $state<UpdateChannel>((ls()?.getItem(CHANNEL_KEY) as UpdateChannel) || 'stable');
  let autoCheck = $state(ls()?.getItem(AUTO_KEY) !== '0');
  // Bump to re-evaluate `available` after a dismiss (dismissal lives in localStorage).
  let dismissTick = $state(0);

  const available = $derived.by(() => {
    void dismissTick;
    if (!info) return false;
    if (!isNewer(info.version, CURRENT)) return false;
    // On desktop/Android, hold the prompt back until the platform installer is
    // actually published — otherwise "Download" would open the GitHub release
    // page while CI is still building the asset. Web has nothing to install, so
    // it's informational either way.
    if (hasLocalDb() && !info.hasAsset) return false;
    return ls()?.getItem(DISMISSED_KEY) !== info.version;
  });

  async function fetchLatest(ch: UpdateChannel): Promise<UpdateInfo | null> {
    const headers = { Accept: 'application/vnd.github+json' };
    if (ch === 'prerelease') {
      const res = await fetch(`https://api.github.com/repos/${REPO}/releases?per_page=10`, { headers });
      if (!res.ok) throw new Error(`GitHub responded ${res.status}`);
      const list = (await res.json()) as any[];
      const rel = list.find((r) => !r.draft);
      return rel ? toInfo(rel) : null;
    }
    const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`, { headers });
    if (res.status === 404) return null; // no published release yet
    if (!res.ok) throw new Error(`GitHub responded ${res.status}`);
    return toInfo(await res.json());
  }

  function toInfo(rel: any): UpdateInfo {
    const version = String(rel.tag_name ?? rel.name ?? '').replace(/^v/, '');
    const releasePage = rel.html_url ?? `https://github.com/${REPO}/releases`;
    const asset = findPlatformAsset(rel.assets ?? []);
    return {
      version,
      notes: (rel.body ?? '').trim(),
      url: releasePage,
      downloadUrl: asset ?? releasePage,
      hasAsset: asset !== null,
      publishedAt: rel.published_at ?? '',
    };
  }

  // While waiting for a published-but-still-building release's installer asset,
  // we poll on the short cadence (capped) instead of the 6h one.
  let assetWaitTimer: ReturnType<typeof setTimeout> | null = null;
  let assetWaitTries = 0;

  /** Check for updates. `force` bypasses the 6h throttle (used by the manual
   *  "Check for updates" button and the asset-wait re-check). */
  async function check(force = false): Promise<void> {
    if (checking) return;
    if (!force && lastChecked) {
      const age = Date.now() - new Date(lastChecked).getTime();
      if (Number.isFinite(age) && age < CHECK_INTERVAL_MS) return;
    }
    checking = true;
    error = '';
    try {
      info = await fetchLatest(channel);
      lastChecked = new Date().toISOString();
      ls()?.setItem(LAST_CHECK_KEY, lastChecked);
      scheduleAssetWait();
    } catch (e) {
      error = (e as Error).message || 'Update check failed';
    } finally {
      checking = false;
    }
  }

  /** If a newer release exists for this platform but its installer asset isn't
   *  up yet, re-check shortly (CI is likely still building it). Otherwise cancel
   *  any pending wait. Bounded so a failed/missing build doesn't poll forever. */
  function scheduleAssetWait(): void {
    if (assetWaitTimer) { clearTimeout(assetWaitTimer); assetWaitTimer = null; }
    const waiting =
      autoCheck &&
      hasLocalDb() &&
      !!info &&
      isNewer(info.version, CURRENT) &&
      !info.hasAsset &&
      ls()?.getItem(DISMISSED_KEY) !== info.version;
    if (!waiting) { assetWaitTries = 0; return; }
    if (assetWaitTries >= ASSET_WAIT_MAX_TRIES) return;
    assetWaitTries++;
    assetWaitTimer = setTimeout(() => { void check(true); }, ASSET_WAIT_MS);
  }

  /** Background check on startup, only when auto-checks are enabled. */
  function maybeAutoCheck(): void {
    if (autoCheck) void check(false);
  }

  function dismiss(): void {
    if (info) ls()?.setItem(DISMISSED_KEY, info.version);
    dismissTick++;
  }

  function setChannel(ch: UpdateChannel): void {
    channel = ch;
    ls()?.setItem(CHANNEL_KEY, ch);
    info = null;
    void check(true);
  }

  function setAutoCheck(on: boolean): void {
    autoCheck = on;
    ls()?.setItem(AUTO_KEY, on ? '1' : '0');
  }

  /**
   * SCAFFOLD — silent desktop self-update. Returns true if it handled the
   * update (so the UI suppresses the browser-download fallback). Today it always
   * returns false; the real implementation lives behind the signing key. See
   * docs/UPDATER.md for the exact tauri-plugin-updater wiring:
   *
   *   const { check } = await import('@tauri-apps/plugin-updater');
   *   const update = await check();
   *   if (update) { await update.downloadAndInstall(onProgress); return true; }
   */
  async function tryDesktopSilentUpdate(_onProgress?: (pct: number) => void): Promise<boolean> {
    return false;
  }

  return {
    get checking() { return checking; },
    get info() { return info; },
    get error() { return error; },
    get available() { return available; },
    get current() { return CURRENT; },
    get lastChecked() { return lastChecked; },
    get channel() { return channel; },
    get autoCheck() { return autoCheck; },
    check,
    maybeAutoCheck,
    dismiss,
    setChannel,
    setAutoCheck,
    tryDesktopSilentUpdate,
  };
}

export const updates = createUpdateStore();
