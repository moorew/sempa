<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api, setServerUrl, getServerUrl, setTauriToken, clearTauriToken, setNativeToken, clearNativeToken, resetApiResolver } from '$lib/api';
  import { isTauri, openExternal } from '$lib/tauri/bridge';

  let isNative = $state(false);
  let needsServerUrl = $derived(isNative || isTauri());
  let Capacitor: any = null;
  let Browser: any = null;
  let App: any = null;

  let authInfo = $state<{ google_enabled: boolean; password_enabled: boolean } | null>(null);
  let username = $state('');
  let password = $state('');
  let serverUrl = $state('');
  let loading  = $state(false);
  let error    = $state('');
  let showPasswordForm = $state(false);
  let showServerUrl = $state(false);
  let appUrlListener: { remove: () => void } | null = null;

  const redirectTarget = $derived(page.url.searchParams.get('redirect') ?? '/');

  /** Derive the base URL for OAuth redirects (matches api.ts logic). */
  function getBase(): string {
    const envUrl = import.meta.env.VITE_API_URL as string | undefined;
    if (envUrl) return envUrl;
    return getServerUrl();
  }

  async function handleAppUrl(event: { url: string }) {
    try {
      const u = new URL(event.url);
      if (u.hostname !== 'login') return;
      const linkToken = u.searchParams.get('link_token');
      const redirect  = u.searchParams.get('redirect') ?? '/';
      if (!linkToken) return;
      await Browser?.close().catch(() => {});
      const result = await api.auth.nativeFinalize(linkToken);
      // Store session as Bearer token (cookie-based auth is unreliable cross-origin on Android)
      if (result.token) setNativeToken(result.token);
      goto(redirect, { replaceState: true });
    } catch {
      error = 'Google sign-in failed. Please try again.';
    }
  }

  // Exchange the one-time OAuth link token for a Bearer session, once. Shared by
  // the cold-start path (onMount) and the warm-start deep-link path ($effect).
  let tauriLoginDone = false;
  async function finalizeTauriLogin(linkToken: string) {
    if (tauriLoginDone) return;
    tauriLoginDone = true;
    loading = true;
    try {
      const result = await api.auth.nativeFinalize(linkToken);
      if (result.token) { setTauriToken(result.token); resetApiResolver(); }
      goto(redirectTarget, { replaceState: true });
    } catch {
      error = 'Google sign-in failed. Please try again.';
      loading = false;
      tauriLoginDone = false; // allow a retry
    }
  }

  // Warm start: the system browser returns via sempa://login?link_token=… while
  // /login is already mounted, so the deep-link router goto()s here with the token
  // in the query — onMount won't re-run, so react to it.
  $effect(() => {
    if (!isTauri()) return;
    const linkToken = page.url.searchParams.get('link_token');
    if (linkToken && !tauriLoginDone) void finalizeTauriLogin(linkToken);
  });

  onMount(async () => {
    // Load Capacitor modules dynamically (only on mobile)
    try {
      const capMod = await import('@capacitor/core');
      Capacitor = capMod.Capacitor;
      isNative = Capacitor.isNativePlatform();
      if (isNative) {
        const [browserMod, appMod] = await Promise.all([
          import('@capacitor/browser'),
          import('@capacitor/app'),
        ]);
        Browser = browserMod.Browser;
        App = appMod.App;
      }
    } catch { /* Capacitor not available */ }

    // Tauri Google OAuth callback. Cold start: the app launched via the sempa://
    // deep link, so the page mounts with ?link_token=X already in the URL. (Warm
    // start — the deep link arrives while /login is already open — is handled by
    // the $effect below.)
    if (isTauri()) {
      const linkToken = page.url.searchParams.get('link_token');
      if (linkToken) { await finalizeTauriLogin(linkToken); return; }
    }

    // Android Capacitor WebView-navigation OAuth callback (fallback when Browser plugin unavailable).
    // Backend redirects to https://localhost/login?link_token=X, which loads this page.
    if (isNative && !isTauri()) {
      const linkToken = page.url.searchParams.get('link_token');
      if (linkToken) {
        loading = true;
        try {
          const result = await api.auth.nativeFinalize(linkToken);
          if (result.token) setNativeToken(result.token);
          goto(redirectTarget, { replaceState: true });
          return;
        } catch {
          error = 'Google sign-in failed. Please try again.';
          loading = false;
        }
      }
    }

    // On native/Tauri platforms, load any previously saved server URL
    if (needsServerUrl) {
      serverUrl = getServerUrl();
      // If no server URL is configured yet, show the URL field and wait
      if (!serverUrl) {
        showServerUrl = true;
        authInfo = { google_enabled: false, password_enabled: true };
        return;
      }
    }

    await continueAuth();
  });

  async function continueAuth() {
    const errParam = page.url.searchParams.get('error');
    if (errParam === 'not_allowed') error = 'Your Google account is not authorised for this Sempa instance.';
    else if (errParam) error = 'Google sign-in was cancelled or failed. Please try again.';

    // Check for existing session first
    try {
      const me = await api.auth.me();
      if (me.authenticated) {
        goto(redirectTarget, { replaceState: true });
        return;
      }
    } catch { /* no session — continue */ }

    if (isNative && App) {
      // Cold start: app was opened via the deep link before the listener was registered
      const launch = await App?.getLaunchUrl();
      if (launch?.url?.startsWith('com.clevercode.sempa://login')) {
        await handleAppUrl({ url: launch.url });
        return;
      }
      // Warm start: listen for the deep link callback while app is already running
      appUrlListener = await App?.addListener('appUrlOpen', handleAppUrl);
    }

    // Load auth config
    try {
      authInfo = await api.auth.config();
    } catch {
      authInfo = { google_enabled: false, password_enabled: true };
    }
  }

  onDestroy(() => { appUrlListener?.remove(); });

  function googleSignIn() {
    const base = getBase();
    const params = new URLSearchParams({ redirect: redirectTarget });
    if (isNative && Browser) {
      // Android primary path: Chrome Custom Tab — deep link fires appUrlOpen on return.
      Browser.open({ url: `${base}/api/v1/auth/google?${params}&native=true` });
    } else if (isNative) {
      // Android fallback: Browser plugin unavailable — navigate the WebView directly.
      // Backend will redirect back to this origin's /login?link_token=X.
      const origin = window.location.origin;
      window.location.href = `${base}/api/v1/auth/google?${params}&capacitor_origin=${encodeURIComponent(origin)}`;
    } else if (isTauri()) {
      // ALL desktop platforms (Windows + Linux/macOS): run OAuth in the SYSTEM
      // browser and return via the sempa:// deep link. Do NOT navigate the in-app
      // webview to Google — Google blocks OAuth inside embedded webviews (WebView2
      // and WebKitGTK alike, "disallowed_useragent"), so it loads then hangs at the
      // account chooser/consent. The system browser is the RFC 8252 native-app
      // pattern; the deep-link router (initDeepLinks → routeForDeepLink) catches
      // sempa://login?link_token=… on return and finalizes the session. This relies
      // on the `sempa://` scheme being registered on every desktop OS — see the
      // deep-link plugin `schemes` in tauri.conf.json (was Linux-overlay-only, which
      // is why Windows previously had to fall back to the broken webview path).
      void openExternal(`${base}/api/v1/auth/google?${params}&desktop_deeplink=true`);
    } else {
      window.location.href = `${base}/api/v1/auth/google?${params}`;
    }
  }

  async function submit() {
    if (!username.trim() || !password) return;
    loading = true; error = '';
    try {
      const result = await api.auth.login(username.trim(), password);
      // In Tauri mode, store Bearer token for subsequent requests
      if (isTauri() && result.token) {
        setTauriToken(result.token);
        resetApiResolver(); // force re-evaluation with token
      }
      goto(redirectTarget, { replaceState: true });
    } catch {
      error = 'Invalid username or password.';
    } finally {
      loading = false;
    }
  }

  async function connectServer() {
    if (!serverUrl.trim()) return;
    loading = true; error = '';
    try {
      // Normalize and save URL
      let url = serverUrl.trim();
      if (!url.startsWith('http://') && !url.startsWith('https://')) {
        url = 'https://' + url;
      }
      setServerUrl(url);
      serverUrl = url;
      resetApiResolver(); // switch from localApi to httpApi now that URL is set

      // Test connectivity. A cold/sleeping backend (e.g. behind Tailscale) can
      // take a while to wake; bound the probe so the button never hangs on
      // "Connecting…" with no feedback — the user reported pressing Connect and
      // seeing nothing happen, which was this request not resolving.
      const me = await withTimeout(api.auth.me(), 12_000);
      // Hide the URL form now that we've reached the server. This MUST be an
      // explicit state change: the template's {#if showServerUrl && !getServerUrl()}
      // gate read getServerUrl() (a non-reactive localStorage call), so after a
      // successful connect Svelte never re-evaluated it and the form just sat
      // there — the "nothing happens on Connect, fixed by restarting" bug.
      showServerUrl = false;
      if (me.authenticated) {
        goto(redirectTarget, { replaceState: true });
        return;
      }
      // Server reachable but not authenticated — show the sign-in options.
      // Clear the placeholder authInfo first so the UI shows the loading spinner
      // (authInfo === null) rather than briefly flashing the password-only form
      // before api.auth.config() reveals the real options (e.g. Google).
      authInfo = null;
      await continueAuth();
    } catch (e) {
      error = e instanceof Error && e.message === 'timeout'
        ? 'Server did not respond. Check the URL and that the server is reachable, then try again.'
        : 'Could not connect to server. Check the URL and try again.';
      // Drop the unverified URL so the field stays visible for another attempt.
      localStorage.removeItem('sempa_server_url');
      resetApiResolver();
      showServerUrl = true;
    } finally {
      loading = false;
    }
  }

  /** Reject with Error('timeout') if the promise doesn't settle in `ms`. */
  function withTimeout<T>(p: Promise<T>, ms: number): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      const t = setTimeout(() => reject(new Error('timeout')), ms);
      p.then(
        (v) => { clearTimeout(t); resolve(v); },
        (e) => { clearTimeout(t); reject(e); },
      );
    });
  }
</script>

<svelte:head><title>Sign in — Sempa</title></svelte:head>

<div class="flex min-h-screen items-center justify-center px-4 py-10" style="background: var(--sempa-bg-main);">

  <div class="w-full max-w-[400px]">

    <div class="surface-raised animate-scale-in px-6 pt-8 pb-7 sm:px-8">

      <!-- Logo -->
      <div class="mb-7 flex flex-col items-center gap-4">
        <div class="flex h-16 w-16 items-center justify-center rounded-[20px]"
             style="background: var(--sempa-btn-bg); box-shadow: var(--sempa-shadow-accent);">
          <svg width="34" height="34" viewBox="0 0 100 100" fill="none" aria-hidden="true">
            <path d="M22,40 a28,28 0 0 0 56,0"
                  stroke="var(--sempa-btn-fg)" stroke-width="10" stroke-linecap="round"/>
            <circle cx="50" cy="35" r="8" fill="var(--sempa-btn-fg)"/>
          </svg>
        </div>
        <div class="text-center">
          <h1 class="login-wordmark" style="color: var(--sempa-text);">sempa</h1>
          <p class="mt-1 text-[13.5px]" style="color: var(--sempa-text-soft);">Your personal task manager</p>
        </div>
      </div>

      <div class="space-y-4">
        {#if error}
          <div class="login-error tint animate-slide-down" role="alert" style="--tint: #d0453a;">
            {error}
          </div>
        {/if}

        {#if showServerUrl}
          <!-- Server URL configuration (native/mobile) -->
          <form onsubmit={(e) => { e.preventDefault(); connectServer(); }} class="space-y-4">
            <div>
              <label for="server-url" class="login-label">Server URL</label>
              <input id="server-url" type="url" bind:value={serverUrl}
                     placeholder="https://sempa.example.com"
                     autofocus
                     class="login-input"
                     onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 3px color-mix(in srgb, var(--sempa-accent) 18%, transparent)'; }}
                     onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }} />
              <p class="mt-2 px-1 text-[12.5px]" style="color: var(--sempa-text-soft);">
                Enter your self-hosted Sempa server address
              </p>
            </div>
            <button type="submit" disabled={loading || !serverUrl.trim()}
                    class="btn-primary login-cta">
              {loading ? 'Connecting…' : 'Connect'}
            </button>
          </form>
        {:else if authInfo === null}
          <!-- Loading -->
          <div class="flex justify-center py-6">
            <div class="h-6 w-6 animate-spin rounded-full border-[2.5px]"
                 style="border-color: var(--sempa-track); border-top-color: var(--sempa-accent);"></div>
          </div>

        {:else if authInfo.google_enabled}
          <!-- Google Sign-In (primary) -->
          <button onclick={googleSignIn}
                  class="login-google"
                  onmousedown={(e) => (e.currentTarget as HTMLElement).style.transform = 'scale(0.97)'}
                  onmouseup={(e) => (e.currentTarget as HTMLElement).style.transform = ''}
                  onmouseleave={(e) => (e.currentTarget as HTMLElement).style.transform = ''}>
            <!-- Google coloured G -->
            <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
              <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
              <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
              <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z"/>
              <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
            </svg>
            Continue with Google
          </button>

          <!-- Password fallback (collapsed by default) -->
          {#if !showPasswordForm}
            <button onclick={() => showPasswordForm = true}
                    class="login-link">
              Sign in with username & password instead
            </button>
          {:else}
            <div class="space-y-3 pt-4" style="border-top: 1px solid var(--sempa-border);">
              <p class="px-1 text-[13px] font-bold" style="color: var(--sempa-text);">Password sign-in</p>
              <form onsubmit={(e) => { e.preventDefault(); submit(); }} class="space-y-3">
                <div>
                  <label for="username" class="login-label">Username</label>
                  <input id="username" type="text" bind:value={username} autocomplete="username"
                         class="login-input"
                         onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 3px color-mix(in srgb, var(--sempa-accent) 18%, transparent)'; }}
                         onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }} />
                </div>
                <div>
                  <label for="password" class="login-label">Password</label>
                  <input id="password" type="password" bind:value={password} autocomplete="current-password"
                         class="login-input"
                         onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 3px color-mix(in srgb, var(--sempa-accent) 18%, transparent)'; }}
                         onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }} />
                </div>
                <button type="submit" disabled={loading || !username || !password}
                        class="btn-primary login-cta">
                  {loading ? 'Signing in…' : 'Sign in'}
                </button>
              </form>
            </div>
          {/if}

        {:else}
          <!-- Password-only (no Google configured) -->
          <form onsubmit={(e) => { e.preventDefault(); submit(); }} class="space-y-4">
            <div>
              <label for="username" class="login-label">Username</label>
              <input id="username" type="text" bind:value={username} autocomplete="username"
                     autofocus
                     class="login-input"
                     onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 3px color-mix(in srgb, var(--sempa-accent) 18%, transparent)'; }}
                     onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }} />
            </div>
            <div>
              <label for="password" class="login-label">Password</label>
              <input id="password" type="password" bind:value={password} autocomplete="current-password"
                     class="login-input"
                     onfocus={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; (e.currentTarget as HTMLElement).style.boxShadow = '0 0 0 3px color-mix(in srgb, var(--sempa-accent) 18%, transparent)'; }}
                     onblur={(e) => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-border)'; (e.currentTarget as HTMLElement).style.boxShadow = ''; }} />
            </div>
            <button type="submit" disabled={loading || !username || !password}
                    class="btn-primary login-cta">
              {loading ? 'Signing in…' : 'Sign in'}
            </button>
          </form>
        {/if}
      </div>
    </div>

    {#if needsServerUrl && getServerUrl()}
      <button onclick={() => { showServerUrl = true; localStorage.removeItem('sempa_server_url'); clearTauriToken(); clearNativeToken(); resetApiResolver(); serverUrl = ''; authInfo = null; error = ''; }}
              class="login-link mt-4">
        Change server ({getServerUrl().replace(/^https?:\/\//, '')})
      </button>
    {/if}

    <p class="mt-6 text-center text-[12.5px]" style="color: var(--sempa-text-dim);">
      Self-hosted · Your data stays yours
    </p>
  </div>
</div>

<style>
  .login-wordmark {
    font-size: 27px; font-weight: 600; letter-spacing: -0.03em; line-height: 1.1;
  }
  .login-label {
    display: block; margin: 0 4px 6px;
    font-size: 12.5px; font-weight: 600; color: var(--sempa-text-soft);
  }
  /* Filled input, radius 14. */
  .login-input {
    width: 100%; height: 46px; padding: 0 15px;
    border-radius: 14px; border: 1px solid var(--sempa-border);
    background: var(--sempa-fill); color: var(--sempa-text);
    font-size: 15px; outline: none;
    transition: border-color 120ms ease, box-shadow 120ms ease;
  }
  .login-input::placeholder { color: var(--sempa-text-dim); }
  .login-input:focus {
    border-color: var(--sempa-accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--sempa-accent) 18%, transparent);
  }
  /* Full-width pill primary (pairs with .btn-primary from app.css). */
  .login-cta { width: 100%; height: 48px; font-size: 15px; }
  .login-cta:disabled { opacity: .45; cursor: default; filter: none; }
  .login-google {
    display: flex; width: 100%; height: 48px; align-items: center; justify-content: center; gap: 10px;
    border: 0; border-radius: 999px; cursor: pointer;
    background: var(--sempa-fill); color: var(--sempa-text);
    box-shadow: inset 0 0 0 1px var(--sempa-border);
    font-size: 15px; font-weight: 700;
    transition: background-color 120ms ease;
  }
  .login-google:hover { background: color-mix(in srgb, var(--sempa-text) 4%, var(--sempa-fill)); }
  .login-link {
    display: block; width: 100%; min-height: 36px; border-radius: 999px;
    text-align: center; font-size: 13px; font-weight: 600; color: var(--sempa-text-soft);
    transition: color 120ms ease;
  }
  .login-link:hover { color: var(--sempa-accent-ink); }
  /* Colour comes from .tint (a red mixed against the theme). */
  .login-error {
    border-radius: 14px; padding: 11px 14px;
    font-size: 13px; font-weight: 600; line-height: 1.45;
  }
</style>
