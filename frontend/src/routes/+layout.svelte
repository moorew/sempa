<script lang="ts">
  import '../app.css';
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { goto, afterNavigate } from '$app/navigation';
  import { today, weekStart, offsetDate } from '$lib/utils';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { theme } from '$lib/stores/theme.svelte';
  import { prefs } from '$lib/stores/prefs.svelte';
  import { timeTracking } from '$lib/stores/timeTracking.svelte';
  import { timeProfile } from '$lib/stores/timeProfile.svelte';
  import { initFocusNotification } from '$lib/focusTimerNotification';
  import { initShareTarget } from '$lib/shareTarget';
  import { initMobileDeepLinks } from '$lib/deeplinkMobile';
  import { quotes } from '$lib/stores/quotes.svelte';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { projectStore } from '$lib/stores/projects.svelte';
  import { mobile } from '$lib/stores/mobile.svelte';
  import { viewport } from '$lib/stores/viewport.svelte';
  import { hapticTick } from '$lib/haptics';
  import { initPushNotifications } from '$lib/push';
  import { registerServiceWorker, enableWebPush, listenForPushNavigation, isWebPushSupported, notificationPermission } from '$lib/webpush';
  import { routines } from '$lib/stores/routines.svelte';
  import { initLocalReminders, syncLocalReminders } from '$lib/localReminders';
  import RoutineBanner from '$lib/components/RoutineBanner.svelte';
  import ReminderBanner from '$lib/components/ReminderBanner.svelte';
  import TimezoneBanner from '$lib/components/TimezoneBanner.svelte';
  import BackupBanner from '$lib/components/BackupBanner.svelte';
  import { timezone } from '$lib/stores/timezone.svelte';
  import { reminderAlerts } from '$lib/stores/reminderAlerts.svelte';
  import { initDesktopReminderPopup, syncDesktopPopup } from '$lib/desktopReminderPopup';
  import { ensureDesktopNotifyPermission } from '$lib/desktopNotify';
  import { SplashScreen } from '@capacitor/splash-screen';
  import { Capacitor } from '@capacitor/core';
  import { api, getServerUrl, getTauriToken, clearTauriToken, clearNativeToken, resetApiResolver, initTauriToken } from '$lib/api';
  import { isTauri, hasLocalDb, onSyncTrigger } from '$lib/tauri/bridge';
  import { initDeepLinks } from '$lib/tauri/deeplink';
  import { windowChrome } from '$lib/stores/windowChrome.svelte';
  import { cockpit } from '$lib/stores/cockpit.svelte';
  import { clock } from '$lib/stores/clock.svelte';
  import CockpitView from '$lib/components/cockpit/CockpitView.svelte';
  import { shortcutLabel } from '$lib/platform';
  import { startSync, sync as runSync, syncStore } from '$lib/sync.svelte';
  import PomodoroTimer from '$lib/components/PomodoroTimer.svelte';
  import SessionConfirm from '$lib/components/SessionConfirm.svelte';
  import TaskTimeModal from '$lib/components/TaskTimeModal.svelte';
  import TimeAutoLogToast from '$lib/components/TimeAutoLogToast.svelte';
  import TimeTrackingWalkthrough from '$lib/components/TimeTrackingWalkthrough.svelte';
  import BottomSheet from '$lib/components/BottomSheet.svelte';
  import TitleBar from '$lib/components/TitleBar.svelte';
  import SyncIndicator from '$lib/components/SyncIndicator.svelte';
  import ContextMenu from '$lib/components/ContextMenu.svelte';
  import UpdateToast from '$lib/components/UpdateToast.svelte';
  import { updates } from '$lib/stores/updates.svelte';
  import { aiStatus } from '$lib/stores/aiStatus.svelte';
  import { jiraStatus } from '$lib/stores/jiraStatus.svelte';
  import { backupHealth } from '$lib/stores/backupHealth.svelte';
  import { importModal } from '$lib/stores/importModal.svelte';
  import { commandPalette } from '$lib/stores/commandPalette.svelte';
  import AiImportModal from '$lib/components/AiImportModal.svelte';
  import AiSetupBanner from '$lib/components/AiSetupBanner.svelte';
  import CommandPalette from '$lib/components/CommandPalette.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import { realtime } from '$lib/stores/realtime.svelte';
  import { celebrate } from '$lib/celebrate';
  import type { Snippet } from 'svelte';

  // Lucide icons
  import {
    Sun, CalendarDays, ClipboardCheck, Mail, Moon, SlidersHorizontal,
    ChevronLeft, ChevronRight, Plus, RefreshCw, X, Check,
    Target, Timer, Palette, Menu, Layers, BookOpen, Search, Bell, LayoutGrid,
    ArrowUpCircle, Gauge, ListChecks, Sparkles, FolderKanban,
    Pencil,
  } from 'lucide-svelte';
  import { NAV_DESTS, NAV_GROUPS, navDest, type NavDest } from '$lib/nav';
  import { createWidgetWindow } from '$lib/tauri/bridge';

  let { children }: { children: Snippet } = $props();

  const todayDate = today();
  const thisWeek  = weekStart(todayDate);

  let isLoginPage      = $derived((page.url.pathname as string) === '/login');
  let isSetupPage      = $derived((page.url.pathname as string) === '/setup');
  // The reminder popup is a separate, chromeless Tauri window (top-right floating
  // card). It must NOT render the app shell or run the heavy auth/sync onMount.
  let isReminderPopup  = $derived((page.url.pathname as string) === '/reminder-popup');
  // The floating widget and sticky-note windows are likewise standalone, chromeless
  // Tauri windows — they own their entire UI and must never render the app shell
  // (sidebar / mobile bottom nav), which was bleeding into the narrow widget.
  // The standalone Pomodoro widget is its own chromeless Tauri window that owns the
  // whole surface (its own full-window timer + inline confirm); the global corner
  // timer and confirm modal are suppressed on it (see below).
  let isPomodoroWidget = $derived((page.url.pathname as string) === '/pomodoro-widget');
  let isStandaloneWindow = $derived(
    isReminderPopup ||
    isPomodoroWidget ||
    (page.url.pathname as string) === '/widget' ||
    (page.url.pathname as string) === '/sticky' ||
    (page.url.pathname as string) === '/quick-add' ||
    (page.url.pathname as string) === '/dock'
  );
  // Whether any in-app banner (reminder alert or routine prompt) is showing —
  // gates the shared spacing wrapper so there's no empty offset when none are.
  let hasBanners = $derived(
    reminderAlerts.alerts.length > 0 || routines.weeklyPlanDue || routines.shutdownDue
      || timezone.mismatch
      || (backupHealth.failing && !backupHealth.dismissed)
  );
  let shortcutsOpen      = $state(false);
  let userEmail          = $state<string | undefined>(undefined);

  // The account line must show a real email, never a device/platform fallback
  // ("desktop"/"mobile" were placeholders used when offline). Prefer the live
  // userEmail when it looks like an email, else the email persisted at login.
  const accountEmail = $derived.by(() => {
    if (userEmail && userEmail.includes('@')) return userEmail;
    const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('sempa_account_email') : null;
    return stored ?? undefined;
  });
  const accountPicture = $derived(
    typeof localStorage !== 'undefined' ? (localStorage.getItem('sempa_account_picture') ?? undefined) : undefined,
  );
  let moreSheetOpen      = $state(false);
  let showIntroAnimation = $state(false);
  let introFadingOut     = $state(false);
  // App shell root — on native mobile we confine celebrations to it so particles
  // stay inside the phone screen rather than escaping to the whole window.
  let appShell           = $state<HTMLElement | undefined>(undefined);

  // Mobile: is this a task-list page where we show the FAB?
  let isTaskListPage = $derived(
    page.url.pathname.startsWith('/day/') || page.url.pathname.startsWith('/week/')
  );

  const SHORTCUT_HELP = [
    { key: 'n',   desc: 'New task (day view)' },
    { key: 'e',   desc: 'Edit hovered task (day view)' },
    { key: 't',   desc: 'Go to today' },
    { key: 'j',   desc: 'Previous week' },
    { key: 'k',   desc: 'Next week' },
    { key: '?',   desc: 'Show keyboard shortcuts' },
    { key: 'Esc', desc: 'Close this dialog' },
  ];

  function handleKeydown(e: KeyboardEvent) {
    if (isLoginPage) return;
    const tgt = e.target as HTMLElement;
    // Cmd/Ctrl+K opens the command palette from anywhere (its own search
    // fallback preserves the old "type + Enter → search" flow).
    if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
      e.preventDefault();
      commandPalette.toggle();
      return;
    }
    // Cmd/Ctrl+Shift+I opens AI Import (when enabled + a model is reachable).
    if ((e.metaKey || e.ctrlKey) && e.shiftKey && (e.key === 'i' || e.key === 'I')) {
      if (prefs.aiOn('aiImport') && prefs.importEntryOn('shortcut') && aiStatus.reachable) {
        e.preventDefault();
        importModal.show();
      }
      return;
    }
    if (tgt.tagName === 'INPUT' || tgt.tagName === 'TEXTAREA' || tgt.isContentEditable) return;
    if (e.metaKey || e.ctrlKey || e.altKey) return;

    if (e.key === 'Escape') { shortcutsOpen = false; return; }
    if (shortcutsOpen) return;

    const path = page.url.pathname;
    const dayMatch = path.match(/^\/day\/(\d{4}-\d{2}-\d{2})/);
    const curDate = dayMatch?.[1] ?? todayDate;
    const curWs   = weekStart(curDate);

    switch (e.key) {
      case 't': e.preventDefault(); goto('/home'); break;
      case 'j': e.preventDefault(); goto(`/day/${offsetDate(curWs, -7)}`); break;
      case 'k': e.preventDefault(); goto(`/day/${offsetDate(curWs, 7)}`); break;
      case '?': e.preventDefault(); shortcutsOpen = true; break;
    }
  }

  onMount(async () => {
    theme.init();
    // Standalone side windows (reminder popup, widget, sticky) need the theme
    // tokens but must NOT start sync/realtime/auth — those belong to the main
    // window, and the widget queries the local DB directly.
    if (isStandaloneWindow) return;

    prefs.init();
    timeTracking.init();
    initFocusNotification();
    initShareTarget();
    void initMobileDeepLinks((u) => goto(u));
    quotes.init();
    mobile.init();
    cockpit.init();
    clock.init();
    viewport.init();

    // Warm the celebration engine and apply the saved sound preference. On native
    // mobile, confine it to the app shell so particles stay on the phone screen.
    celebrate.preload();
    celebrate.setSound(prefs.celebrateSound);
    if (Capacitor.isNativePlatform() && appShell) celebrate.setRoot(appShell);

    // Background check for a newer release (throttled to ~6h; honours the user's
    // "Automatic checks" preference). Surfaces the rail indicator + update toast.
    updates.maybeAutoCheck();

    // Learn whether the local AI model is reachable so AI-assist buttons only
    // appear when they'll actually work.
    void aiStatus.load();

    // Same idea for Jira: hide its tab/tile/command entirely unless connected.
    void jiraStatus.load();

    // Watch for silently-failing backups (403s harmlessly for non-admin users).
    backupHealth.start();

    // Learned per-activity durations, so the completion prompt can stop asking
    // about kinds of work it already knows.
    void timeProfile.load();

    // Desktop floating reminder card (Tauri only; self-guards). Binds the popup
    // window's action listeners once, in the main window.
    void initDesktopReminderPopup((url) => goto(url));
    // Pre-warm native OS notification permission on desktop so the first fired
    // reminder shows its toast immediately instead of racing a permission prompt.
    void ensureDesktopNotifyPermission();

    // Tray "Sync Now" → run a sync cycle. Listener lives for the app's lifetime.
    if (isTauri()) {
      void onSyncTrigger(() => { void runSync(); });

      // sempa:// deep links (OAuth redirect, .desktop launcher actions, second
      // launches forwarded by single-instance) → route into the app.
      void initDeepLinks((url) => goto(url));

      // Mirror the system window-button layout + restore the "use system title
      // bar" preference (Linux native feel).
      void windowChrome.init();

      // Desktop: suppress the webview's generic right-click menu (Reload /
      // Inspect…). Surfaces that want a real menu call contextMenu.show(), which
      // preventDefault()s first, so this only kills the leftover default. Text
      // fields keep their native menu so copy/paste/spellcheck still work.
      const killNativeMenu = (e: MouseEvent) => {
        if (e.defaultPrevented) return;
        const el = e.target as HTMLElement | null;
        if (el?.closest('input, textarea, [contenteditable=""], [contenteditable="true"]')) return;
        e.preventDefault();
      };
      window.addEventListener('contextmenu', killNativeMenu);
    }

    // Near-real-time reconnect sync (local-first platforms only). The browser
    // 'online' event doesn't fire when a Tailscale tailnet reconnects on
    // already-connected wifi, so we trigger sync on signals that actually mean
    // "the server might be reachable again":
    //   • SSE recovers after a drop (most reliable — proves the server is up)
    //   • the app is brought back to the foreground / window regains focus
    if (hasLocalDb()) {
      realtime.onReconnect(() => { void runSync(); });
      const wake = () => { if (!document.hidden) void runSync(); };
      document.addEventListener('visibilitychange', wake);
      window.addEventListener('focus', wake);
    }

    if (!isLoginPage && !isSetupPage) {
      tagStore.load();
      projectStore.load();

      // In Tauri (desktop), require server URL and token before proceeding.
      if (isTauri()) {
        // Load the bearer token from the OS keyring (and migrate any legacy
        // plaintext token) into the in-memory cache before the auth gate reads it.
        await initTauriToken();
        if (!getServerUrl()) {
          goto('/login?redirect=' + encodeURIComponent(page.url.pathname), { replaceState: true });
          return;
        }
        if (!getTauriToken()) {
          goto('/login?redirect=' + encodeURIComponent(page.url.pathname), { replaceState: true });
          return;
        }
        // Server URL and token present. Verify the token *if* the server is
        // reachable, but never bounce to login just because we're offline — the
        // app is local-first, so a stored token means "stay signed in" and we
        // run on local data until the tailnet comes back. Only a definitive
        // "not authenticated" response (server reachable, token rejected)
        // clears the session.
        try {
          const me = await api.auth.me();
          if (!me.authenticated) {
            clearTauriToken();
            goto('/login?redirect=' + encodeURIComponent(page.url.pathname), { replaceState: true });
            return;
          }
          userEmail = me.email ?? 'desktop';
          if (me.email) localStorage.setItem('sempa_account_email', me.email);
        } catch {
          // Network error → offline. Proceed on local data; sync reconciles later.
          // accountEmail falls back to the email persisted at the last login.
          userEmail = 'desktop';
        }
        startSync();
        realtime.connect();
        showIntroAnimation = true;
        setTimeout(() => { introFadingOut = true; }, 1600);
        setTimeout(() => { showIntroAnimation = false; introFadingOut = false; }, 1800);
        return;
      }

      try {
        const me = await api.auth.me();
        if (!me.authenticated) {
          goto('/login?redirect=' + encodeURIComponent(page.url.pathname), { replaceState: true });
          return;
        }
        userEmail = me.email;
        if (me.email) localStorage.setItem('sempa_account_email', me.email);
        // Register for push notifications after auth confirmed
        initPushNotifications();
        // Web Push (PWA / browser): register the service worker, bridge its
        // deep-link messages to the router, and refresh the subscription if the
        // user already granted notification permission on a previous visit.
        listenForPushNavigation((url) => goto(url));
        void registerServiceWorker();
        if (isWebPushSupported() && notificationPermission() === 'granted') {
          void enableWebPush();
        }
        // Redirect to first-run wizard if setup hasn't been completed
        const setup = await api.setup.status();
        if (!setup.done) {
          goto('/setup', { replaceState: true });
        }
        if (hasLocalDb()) startSync();
        realtime.connect();
      } catch {
        // On Android (local-first) an unreachable server means offline, not
        // signed-out: stay in the app on local data and let sync reconcile.
        // On plain web there's no local store, so fall back to the login page.
        if (hasLocalDb()) {
          userEmail = 'mobile';
          startSync();
        } else {
          goto('/login?redirect=' + encodeURIComponent(page.url.pathname), { replaceState: true });
          realtime.disconnect();
        }
      } finally {
        if (Capacitor.isNativePlatform()) {
          await SplashScreen.hide({ fadeOutDuration: 400 });
        }
        showIntroAnimation = true;
        setTimeout(() => { introFadingOut = true; }, 1600);
        setTimeout(() => { showIntroAnimation = false; introFadingOut = false; }, 1800);
      }
    }
  });

  // Tags are loaded in onMount, but the shared layout does NOT re-mount after a
  // client-side login navigation (login uses goto), so onMount's load never
  // re-runs and tag colours stay grey until a manual reload. afterNavigate
  // fires on every client navigation — including the post-login one and the
  // initial load — so reload tags whenever we land on an authenticated page.
  afterNavigate(({ to }) => {
    const path = to?.url.pathname;
    if (path && path !== '/login' && path !== '/setup') {
      tagStore.load();
      projectStore.load();
      // Same remount problem bites sync: on a FRESH login the onMount sync gate
      // already returned early (no token yet) and never runs again, so the
      // first session after sign-in never pulled and the app stayed empty until
      // a restart. startSync()/realtime.connect() self-guard against double
      // starts, so calling them here on every authenticated landing is safe and
      // guarantees sync runs the moment we're signed in on a local-first client.
      if (hasLocalDb()) {
        startSync();
        realtime.connect();
      }
      // Arm the in-app routine scheduler (weekly planning / daily shutdown
      // prompts). Idempotent: re-calling on every authenticated landing is safe.
      routines.init((url) => goto(url));
      // Watch for the device travelling to a new timezone (routines.init above
      // loads the home zone via notificationSettings). Idempotent.
      timezone.init();
      // Schedule on-device OS alarms for upcoming reminders (Android only —
      // fires even with no server/internet). Coalesced + diff-based.
      initLocalReminders((url) => goto(url));
    }
  });

  // Keep the desktop floating reminder card in sync with fired reminders. Runs
  // in the main window only (the popup window short-circuits onMount), and
  // self-guards to Tauri inside syncDesktopPopup.
  $effect(() => {
    if (isStandaloneWindow) return;
    void syncDesktopPopup(reminderAlerts.alerts);
  });

  // Keep the celebration sound in step with the Settings preference.
  $effect(() => {
    celebrate.setSound(prefs.celebrateSound);
  });

  // Re-load tags from server when a tag:change SSE event arrives
  $effect(() => {
    const ev = realtime.lastEvent;
    if (!ev) return;
    if (ev.type === 'tag:change') tagStore.reload();
    if (ev.type === 'project:change') projectStore.reload();
    // On local-first clients a server-side change (from another device) should
    // pull into the local DB so the UI reflects it. runSync is coalesced/cheap.
    if (hasLocalDb()) void runSync();
  });

  // When a local-first pull writes rows (revision bumps), the local DB has new
  // data but the already-mounted pages still show their initial snapshot. Route
  // the change through the realtime channel the pages already watch so they
  // re-read — this is what makes a freshly-launched app fill in once the first
  // sync lands, instead of staying empty until a manual reload.
  let lastRevision = 0;
  $effect(() => {
    const rev = syncStore.revision;
    if (rev === lastRevision) return;
    lastRevision = rev;
    tagStore.reload();
    projectStore.reload();
    realtime.emitLocal('task:change');
    // New/changed tasks may carry reminders — reschedule on-device alarms.
    void syncLocalReminders();
  });

  function isActive(prefix: string): boolean {
    return page.url.pathname.startsWith(prefix);
  }

  // Desktop nav rail grouping. Items keep their routes/icons — only the order
  // and grouping change, driven by the prefs store (Settings → Appearance).
  // Recomputed via $derived so thisWeek/todayDate stay current across day rollover.
  type NavEntry = [href: string, label: string, icon: any];
  type NavGroup = { label: string | null; items: NavEntry[] };
  const navGroups = $derived.by<NavGroup[]>(() => {
    const schemes: Record<string, NavGroup[]> = {
      spaces: [
        { label: null,      items: [['/home', 'Today', CalendarDays], [`/week/${thisWeek}`, 'This Week', CalendarDays]] },
        { label: 'Rituals', items: [[`/plan/${todayDate}`, 'Plan Day', ClipboardCheck], [`/shutdown/${todayDate}`, 'Shutdown', Moon]] },
        { label: 'Inbox',   items: [['/email', 'Email', Mail], ['/reminders', 'Reminders', Bell]] },
        { label: 'Library', items: [['/projects', 'Projects', FolderKanban], ['/backlog', 'Backlog', Layers], ['/lists', 'Lists', ListChecks], ['/journal', 'Journal', BookOpen], ['/insights', 'Insights', Gauge]] },
      ],
      rhythm: [
        { label: 'Plan',   items: [[`/plan/${todayDate}`, 'Plan Day', ClipboardCheck], [`/week/${thisWeek}`, 'This Week', CalendarDays], ['/projects', 'Projects', FolderKanban]] },
        { label: 'Focus',  items: [['/home', 'Today', CalendarDays], ['/backlog', 'Backlog', Layers], ['/lists', 'Lists', ListChecks]] },
        { label: 'Inbox',  items: [['/email', 'Email', Mail], ['/reminders', 'Reminders', Bell]] },
        { label: 'Review', items: [[`/shutdown/${todayDate}`, 'Shutdown', Moon], ['/journal', 'Journal', BookOpen], ['/insights', 'Insights', Gauge]] },
      ],
      flat: [
        { label: null, items: [
          ['/home', 'Today', CalendarDays], ['/search', 'Search', Search], [`/week/${thisWeek}`, 'This Week', CalendarDays],
          [`/plan/${todayDate}`, 'Plan Day', ClipboardCheck], ['/email', 'Email', Mail], ['/backlog', 'Backlog', Layers],
          ['/reminders', 'Reminders', Bell], [`/shutdown/${todayDate}`, 'Shutdown', Moon], ['/journal', 'Journal', BookOpen],
          ['/insights', 'Insights', Gauge], ['/lists', 'Lists', ListChecks], ['/projects', 'Projects', FolderKanban],
        ] },
      ],
    };
    return schemes[prefs.navGrouping] ?? schemes.spaces;
  });

  // Sign out. Local-first: clear the stored token and leave for /login
  // IMMEDIATELY, then tell the server best-effort. The old handler awaited the
  // logout HTTP POST first, so on Android (or any time the server was slow/
  // unreachable) the await never resolved and the button appeared dead — and
  // the mobile variant never cleared the native token at all.
  async function signOut() {
    moreSheetOpen = false;
    // Tell the server to invalidate the session, best-effort, while the token
    // is still present — but DON'T await it (a hung/offline request must not
    // block the local sign-out).
    void api.auth.logout().catch(() => {});
    clearTauriToken();
    clearNativeToken();
    localStorage.removeItem('sempa_account_email');
    localStorage.removeItem('sempa_account_picture');
    userEmail = undefined;
    resetApiResolver();
    realtime.disconnect();
    await goto('/login', { replaceState: true });
  }

  // Phone chrome (1.28): the bottom bar shows the destinations picked in
  // Settings → Appearance → Bottom bar, and the More sheet lists everything else
  // from the same catalogue, so nothing is ever unreachable.
  const navCtx = { today: todayDate, week: thisWeek };
  const navAvailable = (d: NavDest) => d.needs !== 'jira' || jiraStatus.connected;
  const tabDests = $derived(
    prefs.mobileTabs.map((id) => navDest(id)).filter((d): d is NavDest => !!d && navAvailable(d)),
  );
  const moreGroups = $derived(
    NAV_GROUPS.map((label) => ({
      label,
      items: NAV_DESTS.filter((d) => d.group === label && navAvailable(d) && !prefs.mobileTabs.includes(d.id)),
    })).filter((g) => g.items.length > 0),
  );
  // More lights up while its sheet is open, and also when you're on a page that
  // lives in it (so the bar always shows where you are).
  const moreActive = $derived(
    moreSheetOpen ||
    (!tabDests.some((t) => isActive(t.prefix)) && moreGroups.some((g) => g.items.some((d) => isActive(d.prefix)))),
  );
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Global overlays — mounted once, opened from anywhere via their stores. -->
<AiImportModal />
<CommandPalette />
<ConfirmDialog />

{#if isLoginPage || isSetupPage || isStandaloneWindow}
  {@render children()}
{:else if cockpit.active}
  <!-- Cockpit mode: ultrawide-short geometry override (replaces the normal
       sidebar+content chrome with the horizontal cockpit). Titlebar stays for
       window management on desktop. -->
  <div class="flex flex-col h-screen overflow-hidden" style="background: var(--sempa-bg-main);">
    <TitleBar />
    <div class="flex-1" style="min-height:0;"><CockpitView /></div>
  </div>
{:else}
<div bind:this={appShell} class="flex flex-col h-screen overflow-hidden"
     style="background: var(--sempa-bg-main); --app-titlebar-h: {isTauri() && !windowChrome.useSystemTitlebar ? '38px' : '0px'};">
  <!-- Custom titlebar (Tauri only — hidden on web/mobile) -->
  <TitleBar />
  <div class="flex flex-1 overflow-hidden" style="min-height: 0;">

  <!-- ── Sidebar (hidden on mobile) ───────────────────────────────────── -->
  {#if !mobile.value}
  {@const rail = mobile.rail}
  <aside class="flex shrink-0 flex-col" class:w-56={!rail} class:w-16={rail}
         style="background: var(--sempa-bg-nav);">

    <!-- Logo (Cradle mark) — wordmark hides in the collapsed icon rail. -->
    <div class="flex items-center gap-2.5 pb-5 pt-6" class:px-5={!rail} class:justify-center={rail} class:px-0={rail}
         style="color: var(--sempa-accent);">
      <svg width="28" height="28" viewBox="0 0 100 100" fill="none" aria-hidden="true">
        <path d="M22,40 a28,28 0 0 0 56,0"
          stroke="currentColor" stroke-width="10"
          stroke-linecap="round" stroke-linejoin="round"/>
        <circle cx="50" cy="35" r="8" fill="var(--sempa-amber)"/>
      </svg>
      {#if !rail}
        <span style="font-family: 'Plus Jakarta Sans', sans-serif; font-weight: 700;
                     font-size: 20px; letter-spacing: -0.03em; color: var(--sempa-text);">sempa</span>
      {/if}
    </div>

    <!-- Nav -->
    <nav aria-label="Primary" class="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto pb-4" class:px-4={!rail} class:px-2.5={rail}>

      {#snippet navItem(href: string, label: string, Icon: any)}
        {@const active = isActive(href)}
        <a {href} title={rail ? label : undefined}
           class="group flex min-h-[40px] items-center gap-3 rounded-[14px] py-2 text-[14px] tracking-[-0.01em] transition-colors"
           class:px-3={!rail} class:justify-center={rail} class:px-0={rail}
           style={active
             ? `background: var(--sempa-accent-bg); color: var(--sempa-accent-ink); font-weight: 700;`
             : `color: var(--sempa-text); font-weight: 500;`}
           onmouseenter={(e) => { if (!active) (e.currentTarget as HTMLElement).style.background = 'color-mix(in srgb, var(--sempa-text) 5%, transparent)'; }}
           onmouseleave={(e) => { if (!active) (e.currentTarget as HTMLElement).style.background = ''; }}>
          <span class="shrink-0 transition-colors"
                style={active ? `color: var(--sempa-accent)` : 'color: var(--sempa-text-soft)'}>
            <Icon size={18} strokeWidth={active ? 2.25 : 1.9} />
          </span>
          {#if !rail}{label}{/if}
        </a>
      {/snippet}

      {#snippet navLabel(text: string)}
        {#if !rail}
          <div class="px-3 pb-1.5 pt-4 text-[12px] font-semibold"
               style="color: var(--sempa-text-soft);">{text}</div>
        {/if}
      {/snippet}

      <!-- Pinned Search pill (grouped schemes only — in Flat, Search is a row). -->
      {#if prefs.navGrouping !== 'flat'}
        <button onclick={() => goto('/search')} title={rail ? 'Search' : undefined}
                class="mb-4 flex h-[42px] w-full items-center gap-2.5 rounded-full text-[13.5px] transition-colors"
                class:px-4={!rail} class:justify-center={rail} class:px-0={rail}
                style="background: var(--card-bg); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text-soft);"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text)'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.color = 'var(--sempa-text-soft)'}>
          <Search size={16} strokeWidth={2} />
          {#if !rail}
            <span>Search</span>
            <kbd class="ml-auto rounded-full px-2 py-0.5 font-mono text-[10.5px]"
                 style="background: var(--sempa-fill); color: var(--sempa-text-soft);">{shortcutLabel('K')}</kbd>
          {/if}
        </button>
      {/if}

      <!-- Sectioned nav (grouping + section style from Settings → Appearance). -->
      {#each navGroups as group, gi}
        <div class="flex flex-col gap-0.5" class:mt-3.5={gi > 0 && prefs.navSections === 'labels' && !!group.label}>
          {#if prefs.navSections === 'dividers' && gi > 0}
            <div class="mx-3 mb-2 mt-1.5 h-px" style="background: var(--sempa-border);"></div>
          {/if}
          {#if prefs.navSections === 'labels' && group.label}
            {@render navLabel(group.label)}
          {/if}
          {#each group.items as [href, label, Icon]}
            {@render navItem(href, label, Icon)}
          {/each}
        </div>
      {/each}

      <!-- Pomodoro in-progress -->
      {#if pomodoro.taskId}
        <div class="mt-3 rounded-[18px] px-4 py-3"
             style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
          <p class="truncate text-[12px] font-semibold opacity-70">
            {pomodoro.phaseLabel}
          </p>
          <p class="font-mono text-xl font-bold">{pomodoro.display}</p>
        </div>
      {/if}

      <!-- Rail footer: a compact utility icon row and a full-width account chip
           (avatar + email + Sign out). Sync status now lives in a floating
           bottom-right widget, so the footer stays short. -->
      <div class="mt-auto flex flex-col gap-2 pt-4">
        <!-- Utility icon row -->
        <div class="flex items-center" class:justify-between={!rail} class:flex-col={rail} class:gap-1.5={rail}>
          {#if updates.available}
            <button onclick={() => goto('/settings/accounts')} title="Update available — open About"
                    aria-label="Update available"
                    class="flex h-[36px] w-[36px] items-center justify-center rounded-full transition-colors"
                    style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
              <ArrowUpCircle size={18} strokeWidth={1.9} />
            </button>
          {/if}
          <button onclick={() => goto('/settings/accounts')} title="Settings" aria-label="Settings"
                  class="flex h-[36px] w-[36px] items-center justify-center rounded-full transition-colors"
                  style={isActive('/settings') ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);' : 'color: var(--sempa-text-soft);'}
                  onmouseenter={(e) => { if (!isActive('/settings')) { const t = e.currentTarget as HTMLElement; t.style.background = 'var(--sempa-accent-bg)'; t.style.color = 'var(--sempa-text-soft)'; } }}
                  onmouseleave={(e) => { if (!isActive('/settings')) { const t = e.currentTarget as HTMLElement; t.style.background = ''; t.style.color = 'var(--sempa-text-soft)'; } }}>
            <SlidersHorizontal size={18} strokeWidth={1.75} />
          </button>

          <button onclick={theme.toggle} title={theme.dark ? 'Light mode' : 'Dark mode'}
                  aria-label={theme.dark ? 'Switch to light mode' : 'Switch to dark mode'}
                  class="flex h-[36px] w-[36px] items-center justify-center rounded-full transition-colors"
                  style="color: var(--sempa-text-soft);"
                  onmouseenter={(e) => { const t = e.currentTarget as HTMLElement; t.style.background = 'var(--sempa-accent-bg)'; t.style.color = 'var(--sempa-text-soft)'; }}
                  onmouseleave={(e) => { const t = e.currentTarget as HTMLElement; t.style.background = ''; t.style.color = 'var(--sempa-text-soft)'; }}>
            {#if theme.dark}<Sun size={18} strokeWidth={1.75} />{:else}<Moon size={18} strokeWidth={1.75} />{/if}
          </button>

          {#if isTauri()}
            <button onclick={() => createWidgetWindow()} title="Open Widget" aria-label="Open the floating desktop widget"
                    class="flex h-[36px] w-[36px] items-center justify-center rounded-full transition-colors"
                    style="color: var(--sempa-text-soft);"
                    onmouseenter={(e) => { const t = e.currentTarget as HTMLElement; t.style.background = 'var(--sempa-accent-bg)'; t.style.color = 'var(--sempa-text-soft)'; }}
                    onmouseleave={(e) => { const t = e.currentTarget as HTMLElement; t.style.background = ''; t.style.color = 'var(--sempa-text-soft)'; }}>
              <LayoutGrid size={18} strokeWidth={1.75} />
            </button>
          {/if}
        </div>

        <!-- Account chip — avatar + identity; the "Sign out" line is the affordance. -->
        <button onclick={signOut} title={accountEmail ? `${accountEmail} — sign out` : 'Sign out'}
                aria-label="Sign out"
                class="flex w-full items-center gap-3 rounded-[20px] text-left transition-colors"
                class:justify-center={rail} class:p-2.5={!rail} class:p-1.5={rail}
                style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1);"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'var(--sempa-accent-bg)'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = 'var(--sempa-bg-panel)'}>
          {#if accountPicture}
            <img src={accountPicture} alt="" referrerpolicy="no-referrer"
                 class="h-9 w-9 shrink-0 rounded-full object-cover" />
          {:else}
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full"
                  style="background: var(--sempa-accent-bg);
                         color: var(--sempa-accent-ink); font-size: 13.5px; font-weight: 800;">
              {(accountEmail ?? '?').charAt(0).toUpperCase()}
            </span>
          {/if}
          {#if !rail}
            <span class="min-w-0 flex-1">
              {#if accountEmail}
                <span class="block truncate text-[12.5px] font-semibold" style="color: var(--sempa-text);">{accountEmail}</span>
              {/if}
              <span class="block text-[11.5px]" style="color: var(--sempa-text-soft);">Sign out</span>
            </span>
          {/if}
        </button>
      </div>
    </nav>
  </aside>
  {/if}

  <!-- ── Main content ───────────────────────────────────────────────────── -->
  <div class="flex-1 overflow-auto" style="background: var(--sempa-bg-main);
       {mobile.value ? 'padding-bottom: calc(104px + env(safe-area-inset-bottom, 0px));' : ''}">
    {#if hasBanners}
      <!-- Shared spacing for the reminder/routine banners: one top offset (to
           clear the custom titlebar) and a tight gap between them, instead of
           each banner carrying its own 40px top margin (which left a big gap
           between them when both showed). -->
      <div class="mx-auto flex max-w-3xl flex-col gap-1.5"
           style="margin: max(36px, calc(env(safe-area-inset-top, 0px) + 12px)) 16px 0;">
        <ReminderBanner />
        <RoutineBanner />
        <TimezoneBanner />
        <BackupBanner />
        <AiSetupBanner />
      </div>
    {/if}
    {#key page.url.pathname}
      <main class="animate-page-in">{@render children()}</main>
    {/key}
  </div>
  </div><!-- end inner flex row -->
</div>

<!-- ── Mobile bottom tab bar ────────────────────────────────────────────── -->
{#if mobile.value}
  <!-- Floating tab bar (1.27): an inset rounded surface that clears Android's
       gesture handle via the safe-area inset, with a pill behind the active icon. -->
  <nav aria-label="Tabs"
       style="position: fixed; left: 12px; right: 12px; z-index: 40;
              bottom: calc(env(safe-area-inset-bottom, 0px) + 10px); height: 68px;
              background: var(--sempa-bg-panel); border-radius: 26px; box-shadow: var(--sempa-shadow-2);
              display: flex; align-items: center; padding: 0 6px;">
    {#snippet tabButton(label: string, Icon: any, active: boolean, current: boolean, onclick: () => void)}
      <button {onclick}
        aria-current={current ? 'page' : undefined}
        style="display: flex; flex-direction: column; align-items: center; gap: 4px; flex: 1; min-width: 0;
               min-height: 56px; justify-content: center;
               background: none; border: none; cursor: pointer;
               color: {active ? 'var(--sempa-accent-ink)' : 'var(--sempa-text-soft)'};">
        <span style="display: inline-flex; align-items: center; justify-content: center;
                     width: 52px; height: 30px; border-radius: 999px;
                     background: {active ? 'var(--sempa-accent-bg)' : 'transparent'};
                     transition: background-color 160ms ease;">
          <Icon size={21} strokeWidth={active ? 2.3 : 1.9} />
        </span>
        <span style="font-family: 'Plus Jakarta Sans', sans-serif; font-size: 11px; max-width: 100%;
                     overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
                     font-weight: {active ? '700' : '600'};">{label}</span>
      </button>
    {/snippet}
    {#each tabDests as tab (tab.id)}
      {@const active = isActive(tab.prefix)}
      {@render tabButton(tab.short, tab.icon, active, active, () => { goto(tab.href(navCtx)); moreSheetOpen = false; })}
    {/each}
    {@render tabButton('More', Menu, moreActive, false, () => { moreSheetOpen = true; })}
  </nav>

  <!-- FAB for task creation on task-list pages -->
  {#if isTaskListPage}
    <button
      onclick={() => { hapticTick(); goto(`/day/${todayDate}?new=1`); }}
      aria-label="New task"
      style="position: fixed; bottom: calc(env(safe-area-inset-bottom, 0px) + 10px + 68px + 14px); right: 20px; width: 58px; height: 58px;
             border-radius: 999px; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);
             display: flex; align-items: center; justify-content: center; z-index: 30;
             box-shadow: var(--sempa-shadow-accent), 0 2px 6px rgba(0,0,0,.12); border: none; cursor: pointer;">
      <Plus size={26} strokeWidth={2.4} />
    </button>
  {/if}

  <!-- "More" bottom sheet — sectioned: quick row, grouped destinations, account -->
  <BottomSheet open={moreSheetOpen} onClose={() => moreSheetOpen = false}>
    <div style="padding: 8px 16px 24px;">

      {#snippet sheetTile(d: NavDest)}
        {@const active = isActive(d.prefix)}
        <a href={d.href(navCtx)} onclick={() => moreSheetOpen = false}
           class="flex items-center gap-2.5 rounded-[16px] pl-2 pr-3 transition-colors"
           style="min-height: 50px;
                  {active ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent-ink); font-weight: 700;' : 'background: var(--sempa-fill); color: var(--sempa-text); font-weight: 600;'}">
          <span class="tint flex h-[34px] w-[34px] shrink-0 items-center justify-center rounded-[11px]"
                class:on-fill={!active} style="--tint: {d.hue};">
            <d.icon size={17} strokeWidth={2} />
          </span>
          <span class="truncate" style="font-size: 14px;">{d.label}</span>
        </a>
      {/snippet}

      <!-- Quick row -->
      <div class="flex gap-2">
        <button onclick={() => { goto('/settings/accounts'); moreSheetOpen = false; }}
                aria-label="Settings"
                class="flex flex-1 flex-col items-center justify-center gap-1.5 rounded-[20px] py-3.5 transition-opacity active:opacity-80"
                style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
          <SlidersHorizontal size={20} strokeWidth={1.9} />
          <span style="font-size: 12.5px; font-weight: 600;">Settings</span>
        </button>
        <button onclick={() => theme.toggle()}
                aria-label={theme.dark ? 'Switch to light mode' : 'Switch to dark mode'}
                class="flex flex-1 flex-col items-center justify-center gap-1.5 rounded-[20px] py-3.5 transition-opacity active:opacity-80"
                style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
          {#if theme.dark}<Sun size={20} strokeWidth={1.9} />{:else}<Moon size={20} strokeWidth={1.9} />{/if}
          <span style="font-size: 12.5px; font-weight: 600;">{theme.dark ? 'Light mode' : 'Dark mode'}</span>
        </button>
        {#if prefs.aiOn('aiImport') && prefs.importEntryOn('button') && aiStatus.reachable}
          <button onclick={() => { importModal.show(); moreSheetOpen = false; }}
                  aria-label="Import with AI"
                  class="flex flex-1 flex-col items-center justify-center gap-1.5 rounded-[20px] py-3.5 transition-opacity active:opacity-80"
                  style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
            <Sparkles size={20} strokeWidth={1.9} />
            <span style="font-size: 12.5px; font-weight: 600;">AI Import</span>
          </button>
        {/if}
        {#if isTauri()}
          <button onclick={() => createWidgetWindow()} aria-label="Open widget"
                  class="flex flex-1 flex-col items-center justify-center gap-1.5 rounded-[20px] py-3.5 transition-opacity active:opacity-80"
                  style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
            <LayoutGrid size={20} strokeWidth={1.9} />
            <span style="font-size: 12.5px; font-weight: 600;">Widget</span>
          </button>
        {/if}
      </div>

      <!-- Destinations not pinned to the bar, grouped. "Edit tabs" jumps to the
           Bottom bar picker in Settings. -->
      {#each moreGroups as g, gi (g.label)}
        <div class="mb-2 mt-5 flex items-center justify-between gap-3 px-1">
          <p class="text-[13px] font-bold" style="color: var(--sempa-text-soft);">{g.label}</p>
          {#if gi === 0}
            <a href="/settings/accounts?section=appearance&focus=tabs" onclick={() => moreSheetOpen = false}
               class="flex h-8 items-center gap-1.5 rounded-full px-3 text-[12.5px] font-semibold"
               style="background: var(--sempa-fill); color: var(--sempa-accent-ink);">
              <Pencil size={13} strokeWidth={2.2} /> Edit tabs
            </a>
          {/if}
        </div>
        <div class="grid grid-cols-2 gap-2">
          {#each g.items as d (d.id)}{@render sheetTile(d)}{/each}
        </div>
      {/each}

      <!-- Pomodoro (if running) -->
      {#if pomodoro.taskId}
        <div class="mt-4 rounded-[20px] px-4 py-3"
             style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
          <p class="text-[12px] font-semibold opacity-70">{pomodoro.phaseLabel}</p>
          <p class="font-mono text-xl font-bold">{pomodoro.display}</p>
        </div>
      {/if}

      <!-- Sync status (local-first platforms only) -->
      <div class="mt-3 px-1"><SyncIndicator /></div>

      <!-- Account -->
      <div class="mt-3 flex items-center gap-3 rounded-[20px] px-3 py-3" style="background: var(--sempa-fill);">
        {#if accountPicture}
          <img src={accountPicture} alt="" referrerpolicy="no-referrer"
               class="h-9 w-9 shrink-0 rounded-full object-cover" />
        {:else}
          <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full"
                style="background: var(--sempa-accent-bg);
                       color: var(--sempa-accent-ink); font-size: 13px; font-weight: 800;">
            {(accountEmail ?? '?').charAt(0).toUpperCase()}
          </span>
        {/if}
        <div class="min-w-0 flex-1">
          {#if accountEmail}
            <p class="truncate text-[13px]" style="color: var(--sempa-text-soft);" title={accountEmail}>{accountEmail}</p>
          {/if}
        </div>
        <button onclick={signOut}
                class="shrink-0 rounded-full px-4 py-2 text-[13px] font-bold transition-opacity active:opacity-80"
                style="color: var(--sempa-accent-ink); background: var(--sempa-accent-bg);">
          Sign out
        </button>
      </div>
    </div>
  </BottomSheet>
{/if}

{#if pomodoro.taskId && !isPomodoroWidget}
  <PomodoroTimer />
{/if}

{#if !isPomodoroWidget}
  <SessionConfirm />
{/if}
{#if !isStandaloneWindow}
  <TaskTimeModal />
  <TimeTrackingWalkthrough />
  <!-- Auto-logged-time toast: bottom-centre, above the mobile tab bar. -->
  <div class="pointer-events-none fixed inset-x-0 z-[75] flex justify-center px-4"
       style="bottom: calc(env(safe-area-inset-bottom, 0px) + 76px);">
    <TimeAutoLogToast />
  </div>
{/if}

<!-- ── In-app update toast + floating sync status (main window only) ─────── -->
{#if !isStandaloneWindow}
  <UpdateToast />
  <SyncIndicator />
  <ContextMenu />
{/if}

<!-- ── Intro animation overlay ──────────────────────────────────────────── -->
{#if showIntroAnimation}
  <div class="intro-overlay" style="opacity:{introFadingOut ? '0' : '1'};">
    <svg width="80" height="80" viewBox="0 0 100 100" fill="none" aria-hidden="true">
      <path class="arc" d="M22,40 a28,28 0 0 0 56,0"
            stroke="var(--sempa-accent)" stroke-width="9"
            stroke-linecap="round" stroke-linejoin="round"
            stroke-dasharray="88" stroke-dashoffset="88"/>
      <circle class="dot" cx="50" cy="35" r="7.5" fill="var(--sempa-accent)"/>
    </svg>
    <span class="wordmark">sempa</span>
  </div>
{/if}

<!-- Keyboard shortcuts help modal -->
{#if shortcutsOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-50 bg-black/30 backdrop-blur-sm dark:bg-black/50"
       onclick={() => shortcutsOpen = false}></div>
  <div class="fixed inset-0 z-50 flex items-center justify-center pointer-events-none">
    <div class="w-80 rounded-2xl border border-gray-200 bg-white p-6 shadow-2xl pointer-events-auto
                dark:border-gray-700 dark:bg-gray-900">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-100">Keyboard shortcuts</h3>
        <button onclick={() => shortcutsOpen = false}
                class="text-gray-400 hover:text-gray-600 dark:text-gray-600 dark:hover:text-gray-400">
          <X size={16} />
        </button>
      </div>
      <div class="flex flex-col gap-3">
        {#each SHORTCUT_HELP as s}
          <div class="flex items-center justify-between">
            <span class="text-sm text-gray-600 dark:text-gray-400">{s.desc}</span>
            <kbd class="rounded bg-gray-100 px-2 py-0.5 font-mono text-xs text-gray-700
                        dark:bg-gray-800 dark:text-gray-300">{s.key}</kbd>
          </div>
        {/each}
      </div>
    </div>
  </div>
{/if}
{/if}

<style>
  .intro-overlay {
    position: fixed;
    inset: 0;
    z-index: 100;
    background: var(--sempa-bg-main);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    transition: opacity 200ms ease-out;
  }

  @media (prefers-reduced-motion: no-preference) {
    .intro-overlay :global(.arc) {
      animation: arc-draw 700ms ease-out 0ms forwards;
    }
    .intro-overlay :global(.dot) {
      transform-origin: 50px 35px;
      transform: scale(0);
      animation: dot-pop 350ms cubic-bezier(0.34, 1.56, 0.64, 1) 400ms forwards;
    }
    .intro-overlay :global(.wordmark) {
      opacity: 0;
      transform: translateY(8px);
      animation: wordmark-in 400ms ease-out 600ms forwards;
    }
  }

  .wordmark {
    font-family: 'Plus Jakarta Sans', sans-serif;
    font-weight: 500;
    font-size: 24px;
    letter-spacing: -0.02em;
    color: var(--sempa-text);
  }
  @keyframes arc-draw {
    to { stroke-dashoffset: 0; }
  }
  @keyframes dot-pop {
    to { transform: scale(1); }
  }
  @keyframes wordmark-in {
    to { opacity: 1; transform: translateY(0); }
  }
</style>
