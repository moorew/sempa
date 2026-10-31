<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { page } from '$app/state';
  import { api, getServerUrl, clearTauriToken, clearNativeToken, resetApiResolver } from '$lib/api';
  import type { AiTitleConfig, AiProvider } from '$lib/api';
  import { theme } from '$lib/stores/theme.svelte';
  import type { ThemeName } from '$lib/stores/theme.svelte';
  import { windowChrome } from '$lib/stores/windowChrome.svelte';
  import { prefs, AI_FEATURE_META, IMPORT_ENTRY_META } from '$lib/stores/prefs.svelte';
  import { quotes } from '$lib/stores/quotes.svelte';

  let newQuoteText = $state('');
  let newQuoteAuthor = $state('');
  function addQuote() {
    if (!newQuoteText.trim()) return;
    quotes.addCustom(newQuoteText, newQuoteAuthor);
    newQuoteText = ''; newQuoteAuthor = '';
  }
  import { aiStatus as aiAvail } from '$lib/stores/aiStatus.svelte';
  import { isTauri, openExternal } from '$lib/tauri/bridge';

  // OAuth connect: on desktop open in the OS browser (navigating the main webview
  // to a remote origin would strand the app off the local DB); on web follow the
  // link normally.
  function oauthConnect(e: MouseEvent, url: string) {
    if (isTauri()) { e.preventDefault(); void openExternal(url); }
  }
  import { mobile } from '$lib/stores/mobile.svelte';
  import { NAV_DESTS, DEFAULT_TABS, MAX_TABS, MIN_TABS, navDest, type NavDest, type NavId } from '$lib/nav';
  import { jiraStatus } from '$lib/stores/jiraStatus.svelte';
  import { ChevronUp, ChevronDown, Minus, Plus as PlusIcon, Menu as MenuIcon } from 'lucide-svelte';
  import { realtime } from '$lib/stores/realtime.svelte';
  import { goto } from '$app/navigation';
  import UpdatesPanel from '$lib/components/UpdatesPanel.svelte';
  import { canThemeAppIcon, setAppIcon } from '$lib/widget-bridge';

  // ── Themed app icon (Android) ──────────────────────────────────────────────
  // Switching the launcher icon refreshes it in the launcher, so we never do it
  // silently: after the user picks a theme we ASK whether to match the icon. The
  // chosen icon is remembered so we only prompt on an actual change.
  const APP_ICON_KEY = 'sempa_app_icon';
  let iconPromptTheme = $state<{ id: string; label: string } | null>(null);

  function chooseTheme(id: ThemeName, label: string) {
    theme.setTheme(id);
    if (!canThemeAppIcon()) return;
    const current = (typeof localStorage !== 'undefined' && localStorage.getItem(APP_ICON_KEY)) || 'terracotta';
    if (current !== id) iconPromptTheme = { id, label };
  }

  function confirmAppIcon() {
    const t = iconPromptTheme;
    if (!t) return;
    setAppIcon(t.id);
    try { localStorage.setItem(APP_ICON_KEY, t.id); } catch { /* ignore */ }
    iconPromptTheme = null;
  }

  // ── Account / profile ──────────────────────────────────────────────────────
  let me = $state<{ authenticated: boolean; auth_enabled?: boolean; google_enabled?: boolean; email?: string }>({ authenticated: false });
  let accountPicture = $state<string | undefined>();
  let serverUrl = $state('');
  let signingOut = $state(false);

  // The signed-in email: prefer the live session, fall back to the value
  // persisted at login (so it still shows while offline / local-first).
  const accountEmail = $derived.by(() => {
    if (me.email && me.email.includes('@')) return me.email;
    if (typeof localStorage === 'undefined') return undefined;
    return localStorage.getItem('sempa_account_email') ?? undefined;
  });

  const authMethod = $derived(
    me.auth_enabled === false ? 'Open access — no sign-in required'
    : accountPicture            ? 'Signed in with Google'
    : accountEmail              ? 'Signed in with email & password'
    : 'Signed in'
  );

  async function doSignOut() {
    signingOut = true;
    // Best-effort server logout (don't await — a hung/offline request must not
    // block the local sign-out), then clear all local session state.
    void api.auth.logout().catch(() => {});
    clearTauriToken();
    clearNativeToken();
    localStorage.removeItem('sempa_account_email');
    localStorage.removeItem('sempa_account_picture');
    resetApiResolver();
    realtime.disconnect();
    await goto('/login', { replaceState: true });
  }

  type AccountStatus = { connected: boolean; email?: string; last_synced_at?: string | null; enabled?: boolean };

  let gmail    = $state<AccountStatus>({ connected: false });
  let calendar = $state<{ connected: boolean; email?: string; last_synced_at?: string }>({ connected: false });
  let fastmail = $state<AccountStatus>({ connected: false });
  let fmCal    = $state<{ connected: boolean; enabled: boolean; last_synced_at?: string | null }>({ connected: false, enabled: false });
  // CalDAV — push scheduled tasks to a calendar (reuses Fastmail credentials)
  let caldav = $state<{ connected: boolean; enabled?: boolean; calendar_href?: string; calendar_name?: string; last_synced_at?: string | null }>({ connected: false });
  let caldavCalendars = $state<{ href: string; name: string; color?: string }[]>([]);
  let caldavPickerOpen = $state(false);
  let caldavLoading = $state(false);
  let caldavError = $state('');
  let jira     = $state<{ connected: boolean; last_synced_at?: string | null }>({ connected: false });
  let taskInbox = $state<{
    connected: boolean; email?: string; inbox_address?: string;
    allowed_senders?: string[]; last_synced_at?: string;
  }>({ connected: false });

  // AI: the model behind every AI feature (and the email-title cleanup). It runs
  // either on an Ollama host on this server or behind a Tailscale Aperture
  // gateway, which serves any model it grants this server's tailnet identity.
  const AI_EMPTY: AiTitleConfig = {
    enabled: false, provider: 'ollama', base_url: '', model: '', gateway_url: '', gateway_model: '',
    reachable: false, available_models: [], gateway: { reachable: false, models: [] },
  };
  // Fill gaps so a server from before Aperture support still renders cleanly.
  const normAi = (c: Partial<AiTitleConfig>): AiTitleConfig => ({
    ...AI_EMPTY, ...c,
    provider: c.provider === 'aperture' ? 'aperture' : 'ollama',
    gateway_url: c.gateway_url ?? '', gateway_model: c.gateway_model ?? '',
    available_models: c.available_models ?? [], gateway: c.gateway ?? AI_EMPTY.gateway,
  });
  let aiTitle = $state<AiTitleConfig>({ ...AI_EMPTY });
  let aiSaving = $state(false);
  let aiTesting = $state(false);
  let aiError = $state('');
  let aiStatus = $state('');                 // transient success/info feedback
  type AiSaved = Pick<AiTitleConfig, 'enabled' | 'provider' | 'base_url' | 'model' | 'gateway_url' | 'gateway_model'>;
  const aiSnapshot = (c: AiTitleConfig = aiTitle): AiSaved => ({
    enabled: c.enabled, provider: c.provider, base_url: c.base_url.trim(), model: c.model.trim(),
    gateway_url: c.gateway_url.trim(), gateway_model: c.gateway_model.trim(),
  });
  let aiBaseline: AiSaved | null = null;
  let aiGwTesting = $state(false);
  let aiTrying = $state(false);
  let aiTryResult = $state<{ ok: boolean; text: string } | null>(null);
  // Gateway models: the usable ones to pick from, and the rest (with why) tucked away.
  const aiGwUsable = $derived(aiTitle.gateway.models.filter((m) => m.usable));
  const aiGwBlocked = $derived(aiTitle.gateway.models.filter((m) => !m.usable));
  // Ready for the provider selected here (which may not be saved yet), not the
  // server's answer for the last saved one.
  const aiReady = $derived(aiTitle.provider === 'aperture'
    ? aiGwUsable.some((m) => m.id === aiTitle.gateway_model)
    : (aiTitle.ollama_reachable ?? aiTitle.reachable));
  const fmtMs = (ms = 0) => (ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`);
  let aiPullModel = $state('');              // model name to download
  let aiPulling = $state(false);
  let aiPullPct = $state(0);
  let aiPullLabel = $state('');
  let aiRemoving = $state('');               // model currently being removed
  let aiStatusTimer: ReturnType<typeof setTimeout> | null = null;

  function fmtBytes(n: number): string {
    if (!n) return '';
    const u = ['B', 'KB', 'MB', 'GB', 'TB'];
    let v = n, i = 0;
    while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
    return `${v.toFixed(v < 10 && i > 0 ? 1 : 0)} ${u[i]}`;
  }

  function flashAiStatus(msg: string) {
    aiStatus = msg; aiError = '';
    if (aiStatusTimer) clearTimeout(aiStatusTimer);
    aiStatusTimer = setTimeout(() => { aiStatus = ''; }, 4000);
  }

  // Fastmail connect form
  let fmEmail = $state('');
  let fmPassword = $state('');
  let fmSaving = $state(false);
  let fmError = $state('');
  let fmShowForm = $state(false);

  // Email inbox connect form
  let tiEmail = $state('');
  let tiPassword = $state('');
  let tiAddress = $state('tasks@sempa.ca');
  let tiSaving = $state(false);
  let tiError = $state('');
  let tiShowForm = $state(false);

  // Allowed senders
  let senderInput = $state('');
  let senderSaving = $state(false);

  let syncing     = $state<Record<string, boolean>>({});
  let syncResults = $state<Record<string, string>>({});

  // Sub-nav active section tracking
  let scrollContainer = $state<HTMLElement | undefined>();

  // Desktop: show ONE section at a time (like mobile) instead of one long scroll.
  // Seeded from ?section= so each settings section is linkable; kept in the URL.
  const SECTION_IDS = ['profile', 'appearance', 'personal', 'tasks', 'integrations', 'system', 'about'];
  const sectionParam = (() => {
    const s = page.url.searchParams.get('section');
    return s && SECTION_IDS.includes(s) ? s : null;
  })();
  const initialSection = sectionParam ?? 'profile';
  let desktopSection = $state(initialSection);
  function selectSection(id: string) {
    desktopSection = id;
    if (typeof window !== 'undefined') {
      const u = new URL(window.location.href);
      u.searchParams.set('section', id);
      window.history.replaceState({}, '', u);
    }
  }

  // Mobile section navigation. Seeded from ?section= too, so a link into a
  // section (the Tags and Project statuses back links, the More sheet's
  // "Edit tabs") lands inside it on a phone instead of on the section list.
  let mobileSection = $state<string | null>(sectionParam !== 'integrations' ? sectionParam : null);
  function openMobileSection(id: string | null) {
    mobileSection = id;
    if (typeof window !== 'undefined') {
      const u = new URL(window.location.href);
      if (id) u.searchParams.set('section', id); else u.searchParams.delete('section');
      u.searchParams.delete('focus');
      window.history.replaceState({}, '', u);
    }
  }

  // Bottom bar picker (phones, 1.28). The bar keeps its order; "Under More" is
  // everything else, in catalogue order. Jira is only offered once connected.
  const tabAvailable = (d: NavDest) => d.needs !== 'jira' || jiraStatus.connected;
  const pinnedTabs = $derived(prefs.mobileTabs.map((id) => navDest(id)).filter((d): d is NavDest => !!d));
  const unpinnedTabs = $derived(NAV_DESTS.filter((d) => tabAvailable(d) && !prefs.mobileTabs.includes(d.id)));
  const tabsAreDefault = $derived(prefs.mobileTabs.join() === DEFAULT_TABS.join());
  function moveTab(i: number, delta: number) {
    const next = [...prefs.mobileTabs];
    const j = i + delta;
    if (j < 0 || j >= next.length) return;
    [next[i], next[j]] = [next[j], next[i]];
    prefs.setMobileTabs(next);
  }
  function addTab(id: NavId) {
    if (prefs.mobileTabs.length < MAX_TABS) prefs.setMobileTabs([...prefs.mobileTabs, id]);
  }
  function removeTab(id: NavId) {
    if (prefs.mobileTabs.length > MIN_TABS) prefs.setMobileTabs(prefs.mobileTabs.filter((t) => t !== id));
  }
  // Follow ?section= / ?focus= on every navigation, not just the first load:
  // "Edit tabs" in the More sheet can be tapped while Settings is already open,
  // which reuses this component. The writes are untracked so this can't loop.
  $effect(() => {
    const s = page.url.searchParams.get('section');
    const focus = page.url.searchParams.get('focus');
    untrack(() => {
      if (s && SECTION_IDS.includes(s)) {
        desktopSection = s;
        if (s !== 'integrations') mobileSection = s;
      }
      if (focus === 'tabs') {
        setTimeout(() => document.getElementById('settings-tabs')?.scrollIntoView({ block: 'start', behavior: 'smooth' }), 150);
      }
    });
  });

  // Grouped so each section has one clear purpose: visual theming (Appearance)
  // is kept separate from the personality/behaviour toggles (Personalization),
  // and app-level concerns — notifications & backup — live under System rather
  // than being tucked inside Tasks.
  const NAV_SECTIONS = [
    { id: 'profile', label: 'Account' },
    { id: 'appearance', label: 'Appearance' },
    { id: 'personal', label: 'Personalization' },
    { id: 'tasks', label: 'Tasks' },
    { id: 'integrations', label: 'Integrations' },
    { id: 'system', label: 'System' },
    { id: 'about', label: 'About' },
  ] as const;

  // Integrations (Gmail / Fastmail / Jira / calendar OAuth) are managed on
  // desktop/web; they're hidden in the mobile settings to keep it focused.
  const MOBILE_SECTIONS = NAV_SECTIONS.filter((s) => s.id !== 'integrations');

  // Surfaces a clear banner when the device can't reach the backend at all
  // (the common Android failure mode — wrong/missing server URL or auth). Without
  // this the section just renders every integration as "disconnected", which
  // reads as "nothing loaded".
  let serverUnreachable = $state(false);

  // "Launch at login" (desktop). Backed by tauri-plugin-autostart; under Flatpak
  // this requests the Background portal's autostart. Loaded on mount (Tauri only).
  let autostartOn = $state(false);
  let autostartBusy = $state(false);
  async function loadAutostart() {
    if (!isTauri()) return;
    try {
      const { isEnabled } = await import('@tauri-apps/plugin-autostart');
      autostartOn = await isEnabled();
    } catch { /* plugin unavailable */ }
  }
  async function toggleAutostart(on: boolean) {
    if (!isTauri() || autostartBusy) return;
    autostartBusy = true;
    try {
      const { enable, disable, isEnabled } = await import('@tauri-apps/plugin-autostart');
      if (on) await enable(); else await disable();
      autostartOn = await isEnabled();
    } catch {
      autostartOn = !on; // revert the optimistic flip on failure
    } finally {
      autostartBusy = false;
    }
  }

  // ── Paired devices (Dock) ──
  let pairedDevices = $state<Array<{ id: string; device_name: string; platform: string; approved_at: string | null }>>([]);
  let pairCodeInput = $state('');
  let pairBusy = $state(false);
  let pairError = $state('');
  async function loadPairedDevices() {
    try { pairedDevices = await api.devices.list(); } catch { /* offline */ }
  }
  async function approveDevice() {
    const code = pairCodeInput.trim().toUpperCase();
    if (!code || pairBusy) return;
    pairBusy = true; pairError = '';
    try {
      await api.devices.pairApprove(code);
      pairCodeInput = '';
      await loadPairedDevices();
    } catch {
      pairError = 'That code is invalid or expired.';
    } finally { pairBusy = false; }
  }
  async function revokeDevice(id: string) {
    try { await api.devices.revoke(id); await loadPairedDevices(); } catch { /* ignore */ }
  }

  onMount(async () => {
    const connected = page.url.searchParams.get('connected');
    if (connected === '1') window.history.replaceState({}, '', '/settings/accounts');

    void loadAutostart();
    void loadPairedDevices();

    // allSettled so one failing endpoint never rejects the batch (which would
    // leave the whole Integrations section blank). Track how many failed so we
    // can tell "server unreachable" apart from "genuinely disconnected".
    const results = await Promise.allSettled([
      api.integrations.gmail.get(),
      api.integrations.calendar.get(),
      api.integrations.fastmail.get(),
      api.integrations.fastmail.calendar.get(),
      api.integrations.taskInbox.get(),
      api.integrations.jira.get(),
      api.integrations.caldav.get(),
      api.integrations.aiTitle.get(),
    ]);
    const val = <T,>(i: number, fallback: T): T =>
      results[i].status === 'fulfilled' ? (results[i] as PromiseFulfilledResult<T>).value : fallback;

    gmail     = val(0, { connected: false });
    calendar  = val(1, { connected: false });
    fastmail  = val(2, { connected: false });
    fmCal     = val(3, { connected: false, enabled: false });
    taskInbox = val(4, { connected: false });
    jira      = val(5, { connected: false });
    caldav    = val(6, { connected: false });
    aiTitle   = normAi(val(7, AI_EMPTY));
    aiBaseline = aiSnapshot();

    serverUnreachable = results.every((r) => r.status === 'rejected');

    // Account / profile details
    serverUrl = getServerUrl();
    accountPicture = typeof localStorage !== 'undefined' ? (localStorage.getItem('sempa_account_picture') ?? undefined) : undefined;
    try { me = await api.auth.me(); } catch { /* offline — accountEmail falls back to stored value */ }
  });

  async function syncService(name: string, fn: () => Promise<{ new: number; updated: number; errors: number }>) {
    syncing[name] = true; syncResults[name] = '';
    try {
      const r = await fn();
      syncResults[name] = `${r.new} new, ${r.updated} updated${r.errors ? `, ${r.errors} errors` : ''}`;
    } catch (e) {
      syncResults[name] = 'Error: ' + (e as Error).message;
    } finally { syncing[name] = false; }
  }

  async function connectFastmail() {
    if (!fmEmail.trim() || !fmPassword.trim()) return;
    fmSaving = true; fmError = '';
    try {
      await api.integrations.fastmail.save(fmEmail.trim(), fmPassword.trim());
      fastmail = await api.integrations.fastmail.get();
      fmCal = await api.integrations.fastmail.calendar.get().catch(() => ({ connected: false, enabled: false }));
      caldav = await api.integrations.caldav.get().catch(() => ({ connected: false }));
      fmShowForm = false; fmEmail = ''; fmPassword = '';
    } catch (e) { fmError = (e as Error).message; }
    finally { fmSaving = false; }
  }

  async function connectTaskInbox() {
    if (!tiEmail.trim() || !tiPassword.trim() || !tiAddress.trim()) return;
    tiSaving = true; tiError = '';
    try {
      taskInbox = await api.integrations.taskInbox.save(tiEmail.trim(), tiPassword.trim(), tiAddress.trim());
      tiShowForm = false; tiEmail = ''; tiPassword = '';
    } catch (e) { tiError = (e as Error).message; }
    finally { tiSaving = false; }
  }

  async function addSender() {
    const v = senderInput.trim().toLowerCase();
    if (!v) return;
    const current = taskInbox.allowed_senders ?? [];
    if (current.includes(v)) { senderInput = ''; return; }
    senderSaving = true;
    try {
      const res = await api.integrations.taskInbox.setSenders([...current, v]);
      taskInbox = { ...taskInbox, allowed_senders: res.allowed_senders };
      senderInput = '';
    } finally { senderSaving = false; }
  }

  async function removeSender(s: string) {
    const updated = (taskInbox.allowed_senders ?? []).filter(x => x !== s);
    const res = await api.integrations.taskInbox.setSenders(updated);
    taskInbox = { ...taskInbox, allowed_senders: res.allowed_senders };
  }

  async function toggleCalendar(enabled: boolean) {
    await api.integrations.calendar.toggle(enabled);
    calendar = await api.integrations.calendar.get();
  }

  async function toggleFastmailCalendar(enabled: boolean) {
    syncing['fmcal-toggle'] = true;
    try {
      await api.integrations.fastmail.calendar.toggle(enabled);
      fmCal = { ...fmCal, enabled };
    } catch (e) {
      syncResults['fmcal'] = 'Error: ' + (e as Error).message;
    } finally {
      syncing['fmcal-toggle'] = false;
    }
  }

  async function syncFastmailCalendar() {
    syncing['fmcal'] = true; syncResults['fmcal'] = '';
    try {
      const r = await api.integrations.fastmail.calendar.sync();
      syncResults['fmcal'] = `Synced ${r.synced} events`;
    } catch (e) {
      syncResults['fmcal'] = 'Error: ' + (e as Error).message;
    } finally { syncing['fmcal'] = false; }
  }

  // ── CalDAV: push scheduled tasks to a calendar ──────────────────────────────
  async function openCaldavPicker() {
    caldavPickerOpen = true; caldavError = ''; caldavLoading = true;
    try {
      caldavCalendars = await api.integrations.caldav.calendars();
    } catch (e) {
      caldavError = (e as Error).message;
    } finally { caldavLoading = false; }
  }

  async function selectCaldavCalendar(href: string, name: string) {
    caldavLoading = true; caldavError = '';
    try {
      await api.integrations.caldav.select(href, name);
      caldav = await api.integrations.caldav.get();
      caldavPickerOpen = false;
    } catch (e) {
      caldavError = (e as Error).message;
    } finally { caldavLoading = false; }
  }

  async function toggleCaldav(enabled: boolean) {
    syncing['caldav-toggle'] = true;
    try {
      await api.integrations.caldav.toggle(enabled);
      caldav = { ...caldav, enabled };
    } catch (e) {
      syncResults['caldav'] = 'Error: ' + (e as Error).message;
    } finally { syncing['caldav-toggle'] = false; }
  }

  async function syncCaldav() {
    syncing['caldav'] = true; syncResults['caldav'] = '';
    try {
      const r = await api.integrations.caldav.sync();
      syncResults['caldav'] = `Pushed ${r.synced} tasks`;
      caldav = await api.integrations.caldav.get();
    } catch (e) {
      syncResults['caldav'] = 'Error: ' + (e as Error).message;
    } finally { syncing['caldav'] = false; }
  }

  async function disconnectGmail() {
    if (!confirm('Disconnect Gmail? Imported tasks will be kept.')) return;
    await api.integrations.gmail.delete();
    gmail = { connected: false }; calendar = { connected: false };
  }

  async function disconnectFastmail() {
    if (!confirm('Disconnect Fastmail? Imported tasks will be kept.')) return;
    await api.integrations.fastmail.delete();
    fastmail = { connected: false }; fmCal = { connected: false, enabled: false };
    caldav = { connected: false }; caldavPickerOpen = false;
    taskInbox = { connected: false };
  }

  async function disconnectTaskInbox() {
    if (!confirm('Remove email inbox? Imported tasks will be kept.')) return;
    await api.integrations.taskInbox.delete();
    taskInbox = { connected: false };
  }

  // ── AI task-title cleanup ──────────────────────────────────────────────────
  async function saveAiTitle(successMsg = 'AI settings saved') {
    // Skip a no-op save so the button always gives clear feedback.
    const next = aiSnapshot();
    if (aiBaseline && JSON.stringify(aiBaseline) === JSON.stringify(next)) { flashAiStatus('Nothing to save'); return; }
    aiSaving = true;
    aiError = '';
    try {
      aiTitle = normAi(await api.integrations.aiTitle.save(next));
      aiBaseline = aiSnapshot();
      void aiAvail.refresh();
      flashAiStatus(successMsg);
    } catch (e) {
      aiError = e instanceof Error ? e.message : 'Failed to save';
    } finally {
      aiSaving = false;
    }
  }

  // Switch where the model runs. Saved straight away when the destination is
  // already set up, so the app moves over without hunting for Save.
  async function setAiProvider(p: AiProvider) {
    if (aiTitle.provider === p) return;
    aiTitle.provider = p;
    aiTryResult = null;
    aiError = '';
    const ready = p === 'ollama' ? !!aiTitle.base_url.trim() : !!aiTitle.gateway_url.trim();
    if (ready) await saveAiTitle(p === 'aperture' ? 'Using Aperture' : 'Using the model on this server');
  }

  // Connect = save the gateway URL; the server then lists its models fresh.
  // Until a model is picked, the saved provider stays as it was, so AI keeps
  // working on the old setup while you finish this one.
  async function connectAiGateway() {
    const url = aiTitle.gateway_url.trim();
    if (!url) { aiError = 'Enter your Aperture URL first'; return; }
    aiGwTesting = true;
    aiError = '';
    aiTryResult = null;
    try {
      const keepProvider = aiTitle.gateway_model ? aiTitle.provider : (aiBaseline?.provider ?? 'ollama');
      const saved = normAi(await api.integrations.aiTitle.save({ ...aiSnapshot(), provider: keepProvider }));
      aiTitle = { ...saved, provider: aiTitle.provider };
      aiBaseline = { ...aiSnapshot(saved) };
      void aiAvail.refresh();
      if (saved.gateway.reachable) {
        const n = saved.gateway.models.filter((m) => m.usable).length;
        flashAiStatus(n ? `Connected · ${n} model${n === 1 ? '' : 's'} you can use` : 'Connected, but no models are usable yet');
      }
      // On failure the card shows the gateway's own error under the URL field.
    } catch (e) {
      aiError = e instanceof Error ? e.message : 'Couldn’t connect';
    } finally {
      aiGwTesting = false;
    }
  }

  async function pickGatewayModel(id: string) {
    aiTitle.gateway_model = id;
    aiTryResult = null;
    await saveAiTitle('Model updated');
  }

  // One tiny prompt, end to end: proves the model really answers this server.
  // The server only tries saved settings, so save any edits first.
  async function tryAiModel() {
    aiTrying = true;
    aiTryResult = null;
    aiError = '';
    try {
      if (!aiBaseline || JSON.stringify(aiBaseline) !== JSON.stringify(aiSnapshot())) {
        await saveAiTitle('Saved');
        if (aiError) return;
      }
      const res = await api.integrations.aiTitle.try();
      aiTryResult = res.ok
        ? { ok: true, text: `Answered “${res.reply ?? ''}” in ${fmtMs(res.ms)}` }
        : { ok: false, text: res.error ?? 'No answer' };
    } catch (e) {
      aiTryResult = { ok: false, text: e instanceof Error ? e.message : 'Try failed' };
    } finally {
      aiTrying = false;
    }
  }

  // Flip the feature on/off with explicit feedback (disabling keeps any
  // downloaded model — remove it from the model list if you want the disk back).
  async function toggleAi() {
    aiTitle.enabled = !aiTitle.enabled;
    await saveAiTitle(aiTitle.enabled ? 'AI enabled' : 'AI disabled');
  }

  async function testAiTitle() {
    aiTesting = true;
    aiError = '';
    try {
      const res = await api.integrations.aiTitle.test(aiTitle.base_url.trim());
      aiTitle = { ...aiTitle, available_models: res.models ?? [], ollama_reachable: res.reachable };
      if (res.reachable) {
        void aiAvail.refresh();
        const n = res.models?.length ?? 0;
        flashAiStatus(`Connected · ${n} model${n === 1 ? '' : 's'} available`);
      } else {
        aiError = res.error ? `Not reachable: ${res.error}` : 'Not reachable';
      }
    } catch (e) {
      aiError = e instanceof Error ? e.message : 'Test failed';
    } finally {
      aiTesting = false;
    }
  }

  // Download a model, polling progress so the UI can show a bar. On success the
  // model list refreshes and the new model becomes the active one.
  async function pullAiModel() {
    const name = aiPullModel.trim();
    if (!name || aiPulling) return;
    aiPulling = true; aiError = ''; aiStatus = ''; aiPullPct = 0; aiPullLabel = 'Starting…';
    try {
      const base = aiTitle.base_url.trim();
      await api.integrations.aiTitle.pull(name, base);
      // Poll until done or error.
      for (;;) {
        await new Promise((r) => setTimeout(r, 800));
        const st = await api.integrations.aiTitle.pullStatus(name);
        aiPullLabel = st.status || 'Downloading…';
        if (st.total > 0) aiPullPct = Math.min(100, Math.round((st.completed / st.total) * 100));
        if (st.error) { aiError = `Download failed: ${st.error}`; break; }
        if (st.done) { aiPullPct = 100; break; }
      }
      if (!aiError) {
        const cfg = normAi(await api.integrations.aiTitle.get());
        aiTitle = { ...cfg, base_url: base, model: name };
        aiTitle = normAi(await api.integrations.aiTitle.save(aiSnapshot()));
        aiBaseline = aiSnapshot();
        flashAiStatus(`Downloaded ${name}`);
        aiPullModel = '';
      }
    } catch (e) {
      aiError = e instanceof Error ? e.message : 'Download failed';
    } finally {
      aiPulling = false;
    }
  }

  // Delete a downloaded model to reclaim disk (with a confirm, since it must be
  // re-downloaded to use again).
  async function removeAiModel(name: string) {
    if (!confirm(`Remove "${name}"?\n\nIt will be deleted from the server and must be downloaded again to use it.`)) return;
    aiRemoving = name; aiError = ''; aiStatus = '';
    try {
      aiTitle = normAi(await api.integrations.aiTitle.remove(name, aiTitle.base_url.trim()));
      if (aiTitle.model === name) aiTitle.model = aiTitle.available_models[0]?.name ?? '';
      aiBaseline = aiSnapshot();
      flashAiStatus(`Removed ${name}`);
    } catch (e) {
      aiError = e instanceof Error ? e.message : 'Remove failed';
    } finally {
      aiRemoving = '';
    }
  }

  function formatDate(s?: string | null) {
    if (!s) return 'Never';
    return new Date(s).toLocaleString();
  }

  function formatTime(s?: string | null) {
    if (!s) return '';
    return new Date(s).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }
</script>


{#snippet sectionIcon(id: string)}
  {#if id === 'profile'}
    <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
      <circle cx="12" cy="8" r="4"/><path stroke-linecap="round" d="M4 21v-1a6 6 0 0 1 6-6h4a6 6 0 0 1 6 6v1"/>
    </svg>
  {:else if id === 'integrations'}
    <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="M13 10V3L4 14h7v7l9-11h-7z"/>
    </svg>
  {:else if id === 'tasks'}
    <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2m-6 9 2 2 4-4"/>
    </svg>
  {:else if id === 'about'}
    <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
      <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 8h.01M11 12h1v4h1"/>
    </svg>
  {:else if id === 'personal'}
    <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" d="M12 3l1.9 4.6L18.5 9l-4.6 1.9L12 15l-1.9-4.1L5.5 9l4.6-1.4L12 3zM18 14l.9 2.1L21 17l-2.1.9L18 20l-.9-2.1L15 17l2.1-.9L18 14z"/>
    </svg>
  {:else if id === 'system'}
    <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2M8 11h.01M12 11h4"/>
    </svg>
  {:else}
    <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
      <circle cx="12" cy="12" r="3"/><path stroke-linecap="round" d="M19.07 4.93A10 10 0 1 0 4.93 19.07M12 2v2m0 18v-2m8-8h2M2 12h2m13.66-7.07 1.41-1.41M4.93 19.07l1.41-1.41M19.07 19.07l1.41 1.41M4.93 4.93 3.51 3.51"/>
    </svg>
  {/if}
{/snippet}

{#snippet sectionDesc(id: string)}
  {#if id === 'profile'}{accountEmail ?? 'Signed in'} · sign out
  {:else if id === 'integrations'}Gmail, Fastmail, Jira, Calendars
  {:else if id === 'tasks'}Tags, recurring templates
  {:else if id === 'personal'}Reflections, quotes, sounds
  {:else if id === 'system'}Notifications &amp; backup
  {:else if id === 'about'}Version &amp; updates
  {:else}Theme, text size, mode
  {/if}
{/snippet}

<!-- Row chevron for link rows. -->
{#snippet chevron()}
  <svg class="set-chevron h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" aria-hidden="true">
    <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
  </svg>
{/snippet}

{#if mobile.value}
  <!-- ══ MOBILE SETTINGS ═══════════════════════════════════════════════════ -->
  {#if !mobileSection}
    <!-- State A: section list -->
    <div class="px-4 pb-8" style="padding-top: calc(env(safe-area-inset-top, 0px) + 20px);">
      <h1 class="type-page mb-5 px-1" style="color: var(--sempa-text);">Settings</h1>

      <div class="set-group">
        {#each MOBILE_SECTIONS as section}
          <button onclick={() => openMobileSection(section.id)}
                  class="set-row set-row-link">
            <span class="set-icon">{@render sectionIcon(section.id)}</span>
            <span class="set-row-main">
              <span class="set-label block">{section.label}</span>
              <span class="set-desc block truncate">{@render sectionDesc(section.id)}</span>
            </span>
            {@render chevron()}
          </button>
        {/each}
      </div>

      <!-- Theme toggle -->
      <div class="set-group mt-3">
        <div class="set-row">
          <p class="set-row-main set-label">{theme.dark ? 'Dark mode' : 'Light mode'}</p>
          <button onclick={theme.toggle}
                  role="switch" aria-checked={theme.dark} aria-label="Dark mode"
                  class="set-switch" data-on={theme.dark}></button>
        </div>
      </div>

      <!-- Version -->
      <p class="mt-8 text-center text-[12px]" style="color: var(--sempa-text-dim);">Sempa · self-hosted</p>
    </div>

  {:else}
    <!-- State B: section detail -->
    <div style="padding-top: env(safe-area-inset-top, 0px); display:flex; flex-direction:column; height:100%;">
      <!-- Back header -->
      <div class="px-4 pt-4 pb-1">
        <button onclick={() => openMobileSection(null)} class="set-back">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="m15 18-6-6 6-6"/>
          </svg>
          Settings
        </button>
      </div>
      <!-- Section content -->
      <div class="flex-1 overflow-y-auto">
        <div class="px-4 pt-3 pb-16">
          <h1 class="type-page mb-5 px-1" style="color: var(--sempa-text);">
            {NAV_SECTIONS.find(s => s.id === mobileSection)?.label}
          </h1>
          {#if mobileSection === 'profile'}
            {@render profileContent()}
          {:else if mobileSection === 'integrations'}
            {@render integrationsContent()}
          {:else if mobileSection === 'tasks'}
            {@render tasksContent()}
          {:else if mobileSection === 'appearance'}
            {@render appearanceContent()}
          {:else if mobileSection === 'personal'}
            {@render personalContent()}
          {:else if mobileSection === 'system'}
            {@render systemContent()}
          {:else if mobileSection === 'about'}
            {@render aboutContent()}
          {/if}
        </div>
      </div>
    </div>
  {/if}

{:else}
  <!-- ══ DESKTOP SETTINGS ══════════════════════════════════════════════════ -->
  <div class="flex h-full">

    <!-- ── Settings sub-nav ──────────────────────────────────────────────── -->
    <nav class="flex w-[200px] shrink-0 flex-col gap-1 px-4 pt-8" aria-label="Settings sections"
         style="background: var(--sempa-bg-nav);">
      <h1 class="type-date mb-4 px-3" style="color: var(--sempa-text);">Settings</h1>
      {#each NAV_SECTIONS as section}
        <button onclick={() => selectSection(section.id)}
                aria-current={desktopSection === section.id ? 'page' : undefined}
                class="set-nav-item">
          {section.label}
        </button>
      {/each}
    </nav>

    <!-- ── Active section (one at a time, not a long scroll) ─────────────────── -->
    <div bind:this={scrollContainer} class="flex-1 overflow-y-auto">
      <div class="mx-auto max-w-[640px] px-8 pt-9 pb-16">
        <h2 class="type-page mb-6 px-1" style="color: var(--sempa-text);">
          {NAV_SECTIONS.find(s => s.id === desktopSection)?.label}
        </h2>
        {#if desktopSection === 'profile'}
          {@render profileContent()}
        {:else if desktopSection === 'integrations'}
          {@render integrationsContent()}
        {:else if desktopSection === 'tasks'}
          {@render tasksContent()}
        {:else if desktopSection === 'appearance'}
          {@render appearanceContent()}
        {:else if desktopSection === 'personal'}
          {@render personalContent()}
        {:else if desktopSection === 'system'}
          {@render systemContent()}
        {:else if desktopSection === 'about'}
          {@render aboutContent()}
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- ══ CONTENT SNIPPETS (accessible from both mobile and desktop) ══════════ -->

{#snippet profileContent()}
  <!-- ═══════════════════════════════════════════════════════════════════════
       SECTION: Account / profile
  ════════════════════════════════════════════════════════════════════════ -->
  <div id="settings-profile" class="scroll-mt-6">
    <section class="set-group">
      <!-- Identity row: avatar · email · auth method -->
      <div class="set-row" style="min-height: 72px;">
        {#if accountPicture}
          <img src={accountPicture} alt="" referrerpolicy="no-referrer"
               class="h-11 w-11 shrink-0 rounded-full object-cover" />
        {:else}
          <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full text-base font-extrabold"
               style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
            {(accountEmail ?? '?').charAt(0).toUpperCase()}
          </div>
        {/if}
        <div class="set-row-main">
          {#if accountEmail}
            <p class="set-label truncate" style="font-size: 15px;" title={accountEmail}>{accountEmail}</p>
          {:else}
            <p class="set-label" style="font-size: 15px;">Signed in</p>
          {/if}
          <p class="set-desc flex items-center gap-1.5">
            {#if accountPicture}
              <svg class="h-3.5 w-3.5 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
                <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.27-4.74 3.27-8.1Z"/>
                <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84A11 11 0 0 0 12 23Z"/>
                <path fill="#FBBC05" d="M5.84 14.1a6.6 6.6 0 0 1 0-4.2V7.06H2.18a11 11 0 0 0 0 9.88l3.66-2.84Z"/>
                <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1A11 11 0 0 0 2.18 7.06l3.66 2.84C6.71 7.31 9.14 5.38 12 5.38Z"/>
              </svg>
            {/if}
            {authMethod}
          </p>
        </div>
      </div>

      <!-- Server URL -->
      {#if serverUrl}
        <div class="set-row set-row-compact">
          <span class="set-row-main set-label">Server</span>
          <span class="truncate font-mono text-xs" style="color: var(--sempa-text-soft);" title={serverUrl}>{serverUrl}</span>
        </div>
      {/if}

      <!-- Sign out -->
      <div class="set-row">
        <div class="set-row-main">
          <p class="set-label">Sign out</p>
          <p class="set-desc">Local data stays on this device until you sign back in.</p>
        </div>
        <button onclick={doSignOut} disabled={signingOut}
                class="set-danger set-btn">
          {signingOut ? 'Signing out…' : 'Sign out'}
        </button>
      </div>
    </section>

    <!-- Users & credentials -->
    <div class="set-group">
      <a href="/settings/users" class="set-row set-row-link">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M17 20h5v-2a4 4 0 0 0-3-3.87M9 20H4v-2a4 4 0 0 1 3-3.87m6-1.13a4 4 0 1 0-4-4 4 4 0 0 0 4 4Z"/>
          </svg>
        </span>
        <span class="set-row-main">
          <span class="set-label block">Users &amp; password</span>
          <span class="set-desc block">Add people, manage credentials, change your password</span>
        </span>
        {@render chevron()}
      </a>
    </div>
  </div>
{/snippet}

{#snippet integrationsContent()}
  <!-- ═══════════════════════════════════════════════════════════════════════
       SECTION: Integrations
  ════════════════════════════════════════════════════════════════════════ -->
  <div id="settings-integrations">

    {#if serverUnreachable}
      <div class="tint mb-5 rounded-2xl px-4 py-3" style="--tint: var(--sempa-amber);">
        <p class="text-sm font-bold">Can't reach your server</p>
        <p class="mt-1 text-[12.5px] leading-relaxed" style="color: var(--sempa-text-soft);">
          Integration status couldn't load. Check your connection and that the server URL is
          correct, then pull to refresh or reopen this screen.
        </p>
      </div>
    {/if}

    <!-- ── Email & calendar ──────────────────────────────────────── -->
    <h3 class="set-heading">Email &amp; calendar</h3>

    <!-- ── Gmail ─────────────────────────────────────────────────── -->
    <section class="set-group">
      <!-- Header -->
      <div class="set-row" style="min-height: 64px;">
        <span class="set-icon tint" style="--tint: #ea4335;">
          <svg class="h-[18px] w-[18px]" viewBox="0 0 24 24" fill="currentColor">
            <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
          </svg>
        </span>
        <div class="set-row-main">
          <p class="set-label">Gmail</p>
          {#if gmail.connected}
            <p class="set-desc truncate">
              {gmail.email} &middot; Last synced {formatTime(gmail.last_synced_at)}
            </p>
          {:else}
            <p class="set-desc">Import starred emails as tasks</p>
          {/if}
        </div>
        {#if gmail.connected}
          <span class="set-status">Connected</span>
        {:else}
          <a href={api.integrations.gmail.authUrl(false)}
             onclick={(e) => oauthConnect(e, api.integrations.gmail.authUrl(false))}
             class="btn-soft set-btn-sm">
            Connect &rarr;
          </a>
        {/if}
      </div>

      {#if gmail.connected}
        <!-- Sub-features -->
        <div class="set-row set-row-compact">
          <span class="set-row-main set-label">Starred emails</span>
          {#if syncResults['gmail']}
            <span class="set-meta" style="color: var(--sempa-accent-ink);">{syncResults['gmail']}</span>
          {/if}
          <button onclick={() => syncService('gmail', api.integrations.gmail.sync)}
                  disabled={syncing['gmail']}
                  class="set-btn-quiet set-btn-sm">
            {syncing['gmail'] ? 'Syncing...' : 'Sync'}
          </button>
        </div>

        <!-- Google Calendar sub-feature -->
        <div class="set-row set-row-compact">
          <span class="set-row-main set-label">Google Calendar</span>
          {#if syncResults['calendar']}
            <span class="set-meta" style="color: var(--sempa-accent-ink);">{syncResults['calendar']}</span>
          {/if}
          {#if calendar.connected}
            <button onclick={() => syncService('calendar', () => api.integrations.calendar.sync())}
                    disabled={syncing['calendar']}
                    class="set-btn-quiet set-btn-sm">
              {syncing['calendar'] ? 'Syncing...' : 'Sync today'}
            </button>
            <button onclick={() => toggleCalendar(false)}
                    aria-label="Disable Google Calendar"
                    class="set-switch" data-on="true"></button>
          {:else}
            <a href={api.integrations.gmail.authUrl(true)}
               onclick={(e) => oauthConnect(e, api.integrations.gmail.authUrl(true))}
               class="btn-soft set-btn-sm">
              Enable
            </a>
          {/if}
        </div>

        <!-- Gmail labels link -->
        <a href="/settings/integrations/gmail" class="set-row set-row-compact set-row-link">
          <span class="set-row-main set-label">Label filters</span>
          <span class="set-meta">Configure</span>
          {@render chevron()}
        </a>

        <!-- Disconnect -->
        <div class="set-row set-row-compact">
          <button onclick={disconnectGmail} class="set-ghost set-btn-sm -ml-3" style="color: var(--set-danger);">
            Disconnect Gmail
          </button>
        </div>
      {/if}
    </section>

    <!-- ── Fastmail ──────────────────────────────────────────────── -->
    <section class="set-group">
      <!-- Header -->
      <div class="set-row" style="min-height: 64px;">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
          </svg>
        </span>
        <div class="set-row-main">
          <p class="set-label">Fastmail</p>
          {#if fastmail.connected}
            <p class="set-desc truncate">
              {fastmail.email} &middot; Last synced {formatTime(fastmail.last_synced_at)}
            </p>
          {:else}
            <p class="set-desc">Sync starred emails and calendar via JMAP</p>
          {/if}
        </div>
        {#if fastmail.connected}
          <span class="set-status">Connected</span>
        {:else if !fmShowForm}
          <button onclick={() => fmShowForm = true}
                  class="btn-soft set-btn-sm">
            Connect &rarr;
          </button>
        {/if}
      </div>

      {#if fastmail.connected}
        <!-- Starred emails -->
        <div class="set-row set-row-compact">
          <span class="set-row-main set-label">Starred emails</span>
          {#if syncResults['fastmail']}
            <span class="set-meta" style="color: var(--sempa-accent-ink);">{syncResults['fastmail']}</span>
          {/if}
          <button onclick={() => syncService('fastmail', api.integrations.fastmail.sync)}
                  disabled={syncing['fastmail']}
                  class="set-btn-quiet set-btn-sm">
            {syncing['fastmail'] ? 'Syncing...' : 'Sync'}
          </button>
        </div>

        <!-- Fastmail Calendar -->
        {#if fmCal.connected}
          <div class="set-row set-row-compact">
            <div class="set-row-main">
              <p class="set-label">Calendar</p>
              {#if fmCal.enabled && fmCal.last_synced_at}
                <p class="set-desc">Synced {formatTime(fmCal.last_synced_at)}</p>
              {/if}
            </div>
            {#if syncResults['fmcal']}
              <span class="set-meta" style="color: var(--sempa-accent-ink);">{syncResults['fmcal']}</span>
            {/if}
            {#if fmCal.enabled}
              <button onclick={syncFastmailCalendar}
                      disabled={syncing['fmcal']}
                      class="set-btn-quiet set-btn-sm">
                {syncing['fmcal'] ? 'Syncing...' : 'Sync'}
              </button>
            {/if}
            <button onclick={() => toggleFastmailCalendar(!fmCal.enabled)}
                    disabled={syncing['fmcal-toggle']}
                    aria-label="Toggle Fastmail Calendar"
                    class="set-switch" data-on={fmCal.enabled}></button>
          </div>
        {/if}

        <!-- CalDAV — push scheduled tasks to a calendar -->
        {#if caldav.connected}
          <div>
            <div class="set-row set-row-compact set-row-wrap">
              <div class="set-row-main">
                <p class="set-label">Push tasks to calendar</p>
                {#if caldav.calendar_name || (caldav.enabled && caldav.last_synced_at)}
                  <p class="set-desc">
                    {caldav.calendar_name ?? ''}{#if caldav.calendar_name && caldav.enabled && caldav.last_synced_at}&nbsp;&middot;&nbsp;{/if}{#if caldav.enabled && caldav.last_synced_at}{formatTime(caldav.last_synced_at)}{/if}
                  </p>
                {/if}
              </div>
              <div class="ml-auto flex items-center gap-2">
                {#if syncResults['caldav']}
                  <span class="set-meta" style="color: var(--sempa-accent-ink);">{syncResults['caldav']}</span>
                {/if}
                {#if caldav.calendar_href}
                  {#if caldav.enabled}
                    <button onclick={syncCaldav} disabled={syncing['caldav']}
                            class="set-btn-quiet set-btn-sm">
                      {syncing['caldav'] ? 'Pushing...' : 'Sync now'}
                    </button>
                  {/if}
                  <button onclick={openCaldavPicker} disabled={caldavLoading}
                          class="set-btn-quiet set-btn-sm">Change</button>
                  <button onclick={() => toggleCaldav(!caldav.enabled)}
                          disabled={syncing['caldav-toggle']}
                          aria-label="Toggle CalDAV task push"
                          class="set-switch" data-on={!!caldav.enabled}></button>
                {:else}
                  <button onclick={openCaldavPicker} disabled={caldavLoading}
                          class="btn-soft set-btn-sm">
                    {caldavLoading ? 'Loading...' : 'Set up'}
                  </button>
                {/if}
              </div>
            </div>

            {#if caldavPickerOpen}
              <div class="space-y-1.5 px-[18px] pb-4 text-[13px]">
                {#if caldavError}
                  <p class="set-error">{caldavError}</p>
                {/if}
                {#if caldavLoading}
                  <p class="set-meta">Loading calendars…</p>
                {:else if caldavCalendars.length === 0 && !caldavError}
                  <p class="set-meta">No writable calendars found.</p>
                {:else}
                  <p class="set-meta mb-1">Choose a calendar to write task time-blocks into:</p>
                  {#each caldavCalendars as cal (cal.href)}
                    {@const selected = caldav.calendar_href === cal.href}
                    <button onclick={() => selectCaldavCalendar(cal.href, cal.name)}
                            disabled={caldavLoading}
                            class="flex min-h-10 w-full items-center gap-2.5 rounded-xl px-3 py-2 text-left transition-colors disabled:opacity-50"
                            style="background: {selected ? 'var(--sempa-accent-bg)' : 'var(--sempa-fill)'};
                                   {selected ? 'box-shadow: inset 0 0 0 1.5px var(--sempa-accent);' : ''}">
                      <span class="h-2.5 w-2.5 shrink-0 rounded-full"
                            style="background: {cal.color || '#6b7280'};"></span>
                      <span class="flex-1 min-w-0 truncate font-semibold" style="color: var(--sempa-text);">{cal.name}</span>
                      {#if selected}
                        <span class="font-bold" style="color: var(--sempa-accent-ink);">Selected</span>
                      {/if}
                    </button>
                  {/each}
                {/if}
                <button onclick={() => { caldavPickerOpen = false; caldavError = ''; }}
                        class="set-ghost set-btn-sm -ml-3">Cancel</button>
              </div>
            {/if}
          </div>
        {/if}

        <!-- Email Inbox (Fastmail sub-feature) -->
        {#if taskInbox.connected}
          <div class="set-row set-row-compact">
            <div class="set-row-main">
              <p class="set-label">Email inbox</p>
              {#if taskInbox.inbox_address}
                <p class="set-desc truncate font-mono" style="font-size: 12px;">{taskInbox.inbox_address}</p>
              {/if}
            </div>
            {#if syncResults['task-inbox']}
              <span class="set-meta" style="color: var(--sempa-accent-ink);">{syncResults['task-inbox']}</span>
            {/if}
            <button onclick={() => syncService('task-inbox', api.integrations.taskInbox.sync)}
                    disabled={syncing['task-inbox']}
                    class="set-btn-quiet set-btn-sm">
              {syncing['task-inbox'] ? 'Syncing...' : 'Sync'}
            </button>
          </div>

          <!-- Allowed senders -->
          <div class="set-block space-y-2.5">
            <div class="flex items-center justify-between">
              <p class="set-label">Allowed senders</p>
              {#if (taskInbox.allowed_senders ?? []).length === 0}
                <p class="set-meta">All senders allowed</p>
              {/if}
            </div>
            {#if (taskInbox.allowed_senders ?? []).length > 0}
              <div class="flex flex-wrap gap-1.5">
                {#each (taskInbox.allowed_senders ?? []) as sender}
                  <span class="set-chip">
                    {sender}
                    <button onclick={() => removeSender(sender)} aria-label="Remove {sender}">
                      <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2.4" viewBox="0 0 24 24">
                        <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
                      </svg>
                    </button>
                  </span>
                {/each}
              </div>
            {/if}
            <form onsubmit={(e) => { e.preventDefault(); addSender(); }} class="flex gap-2">
              <input bind:value={senderInput}
                     placeholder="@company.com or user@example.com"
                     class="set-input min-w-0 flex-1 text-[13px]" />
              <button type="submit" disabled={senderSaving || !senderInput.trim()}
                      class="set-btn-quiet set-btn">
                Add
              </button>
            </form>
          </div>

          <!-- Disconnect inbox -->
          <div class="set-row set-row-compact">
            <button onclick={disconnectTaskInbox} class="set-ghost set-btn-sm -ml-3" style="color: var(--set-danger);">
              Remove email inbox
            </button>
          </div>
        {:else}
          <!-- Email inbox not set up yet -->
          <div class="set-row set-row-compact">
            <div class="set-row-main">
              <p class="set-label">Email inbox</p>
              <p class="set-desc">Forward emails to create tasks</p>
            </div>
            <button onclick={() => tiShowForm = true}
                    class="btn-soft set-btn-sm">
              Set up &rarr;
            </button>
          </div>
        {/if}

        <!-- Email inbox setup form (inline) -->
        {#if tiShowForm && !taskInbox.connected}
          <div class="set-block space-y-3">
            <div>
              <label class="set-field-label" for="ti-email">Fastmail email</label>
              <input id="ti-email" type="email" inputmode="email" autocapitalize="none" spellcheck="false" bind:value={tiEmail} placeholder="you@fastmail.com"
                     class="set-input w-full" />
            </div>
            <div>
              <label class="set-field-label" for="ti-pass">App password</label>
              <input id="ti-pass" type="password" bind:value={tiPassword}
                     placeholder="Generate at Fastmail -> Settings -> Privacy & Security"
                     class="set-input w-full" />
            </div>
            <div>
              <label class="set-field-label" for="ti-addr">Forwarding address</label>
              <input id="ti-addr" type="email" inputmode="email" autocapitalize="none" spellcheck="false" bind:value={tiAddress} placeholder="tasks@sempa.ca"
                     class="set-input w-full" />
            </div>
            {#if tiError}<p class="set-error">{tiError}</p>{/if}
            <div class="flex gap-2 pt-1">
              <button onclick={connectTaskInbox} disabled={tiSaving || !tiEmail || !tiPassword || !tiAddress}
                      class="btn-primary set-btn">
                {tiSaving ? 'Connecting...' : 'Connect'}
              </button>
              <button onclick={() => { tiShowForm = false; tiError = ''; }}
                      class="set-btn-quiet set-btn">
                Cancel
              </button>
            </div>
          </div>
        {/if}

        <!-- Disconnect Fastmail -->
        <div class="set-row set-row-compact">
          <button onclick={disconnectFastmail} class="set-ghost set-btn-sm -ml-3" style="color: var(--set-danger);">
            Disconnect Fastmail
          </button>
        </div>

      {:else if fmShowForm}
        <!-- Fastmail connect form -->
        <div class="set-block space-y-3">
          <div>
            <label class="set-field-label" for="fm-email">Email</label>
            <input id="fm-email" type="email" inputmode="email" autocapitalize="none" spellcheck="false" bind:value={fmEmail} placeholder="you@fastmail.com"
                   class="set-input w-full" />
          </div>
          <div>
            <label class="set-field-label" for="fm-pass">App password</label>
            <input id="fm-pass" type="password" bind:value={fmPassword}
                   placeholder="Generate at fastmail.com -> Settings -> Security"
                   class="set-input w-full" />
            <p class="set-meta mt-1.5 px-0.5">
              Create at fastmail.com -> Settings -> Privacy & Security -> App Passwords
            </p>
          </div>
          {#if fmError}<p class="set-error">{fmError}</p>{/if}
          <div class="flex gap-2 pt-1">
            <button onclick={connectFastmail} disabled={fmSaving || !fmEmail || !fmPassword}
                    class="btn-primary set-btn">
              {fmSaving ? 'Connecting...' : 'Connect'}
            </button>
            <button onclick={() => { fmShowForm = false; fmError = ''; }}
                    class="set-btn-quiet set-btn">
              Cancel
            </button>
          </div>
        </div>
      {/if}
    </section>

    <!-- ── Calendars ─────────────────────────────────────────────────
         Subscriptions (ICS/webcal feeds), per-calendar colours and the
         connected Google/Fastmail accounts all live on the dedicated
         /settings/calendars page (also reachable from the schedule sidebar),
         so this is a single entry point rather than a second, divergent copy. -->
    <div class="set-group">
      <a href="/settings/calendars" class="set-row set-row-link">
        <span class="set-icon tint" style="--tint: var(--sempa-success);">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <rect x="3" y="4" width="18" height="18" rx="2" ry="2"/>
            <line x1="16" y1="2" x2="16" y2="6"/>
            <line x1="8" y1="2" x2="8" y2="6"/>
            <line x1="3" y1="10" x2="21" y2="10"/>
          </svg>
        </span>
        <span class="set-row-main">
          <span class="set-label block">Calendars</span>
          <span class="set-desc block">Subscriptions, colours, and connected calendars</span>
        </span>
        {@render chevron()}
      </a>
    </div>

    <!-- ── Project management ────────────────────────────────────── -->
    <h3 class="set-heading">Project management</h3>

    <div class="set-group">
      <a href="/settings/integrations/jira" class="set-row set-row-link">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" viewBox="0 0 24 24" fill="currentColor">
            <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
          </svg>
        </span>
        <span class="set-row-main">
          <span class="set-label block">Jira</span>
          {#if jira.connected}
            <span class="set-desc block">Syncs assigned issues</span>
          {:else}
            <span class="set-desc block">Sync assigned Jira issues as tasks</span>
          {/if}
        </span>
        {#if jira.connected}
          <span class="set-status">Connected</span>
        {:else}
          <span class="set-meta font-semibold" style="color: var(--sempa-accent-ink);">Connect</span>
        {/if}
        {@render chevron()}
      </a>
    </div>

    <!-- ── AI task-title cleanup (local Ollama) ──────────────────── -->
    <h3 class="set-heading">AI</h3>
    <section class="set-group">
      <div class="set-row set-row-top" style="padding-top: 16px; padding-bottom: 16px;">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 3l1.8 4.4L18 9l-4.2 1.6L12 15l-1.8-4.4L6 9l4.2-1.6L12 3z"/>
          </svg>
        </span>
        <div class="set-row-main">
          <p class="set-label">AI assist</p>
          <p class="set-desc">
            {#if aiTitle.provider === 'aperture'}
              Quick-add, planning, summaries and email-title cleanup use a model reached through your
              Aperture gateway, which routes to whichever provider serves it. If it's unavailable, AI
              features hide and imported emails keep their original subject.
            {:else}
              Quick-add, planning, summaries and email-title cleanup use a language model that runs on
              your own server. Nothing is sent to any third party; if it's unavailable, AI features hide
              and imported emails keep their original subject.
            {/if}
          </p>
        </div>
        <button onclick={toggleAi}
                disabled={aiSaving}
                aria-label="Toggle AI assist"
                class="set-switch mt-1" data-on={aiTitle.enabled}></button>
      </div>

      {#if aiTitle.enabled}
        <div class="set-block flex flex-col gap-4">
          <!-- Where the model runs -->
          <div>
            <p class="set-field-label">Where the model runs</p>
            <div class="segmented" role="tablist" aria-label="Where the model runs">
              <button role="tab" aria-selected={aiTitle.provider === 'ollama'} onclick={() => setAiProvider('ollama')}
                      disabled={aiSaving} class="flex h-9 items-center justify-center text-[12.5px] font-semibold">
                On this server
              </button>
              <button role="tab" aria-selected={aiTitle.provider === 'aperture'} onclick={() => setAiProvider('aperture')}
                      disabled={aiSaving} class="flex h-9 items-center justify-center text-[12.5px] font-semibold">
                Aperture
              </button>
            </div>
          </div>

          <!-- Reachability of the model in use -->
          <div class="flex items-center gap-2 text-[12.5px] font-semibold">
            <span class="h-2 w-2 rounded-full" style="background: {aiReady ? 'var(--sempa-success)' : 'var(--sempa-amber)'};"></span>
            <span style="color: var(--sempa-text-soft);">
              {#if aiTitle.provider === 'aperture'}
                {aiReady ? `Ready · ${aiTitle.gateway_model}`
                  : !aiTitle.gateway_url ? 'Add your Aperture URL to get started'
                  : !aiTitle.gateway.reachable ? 'Aperture not reachable from this server'
                  : !aiTitle.gateway_model ? 'Connected · pick a model'
                  : 'The chosen model isn’t available to this server'}
              {:else}
                {aiReady ? 'Model server reachable' : 'Not reachable · AI features stay hidden'}
              {/if}
            </span>
          </div>

          {#if aiTitle.provider === 'aperture'}
            <!-- Aperture gateway -->
            <div>
              <label class="set-field-label" for="ai-gw-url">Aperture URL</label>
              <div class="flex items-center gap-2">
                <input id="ai-gw-url" bind:value={aiTitle.gateway_url} placeholder="https://ai.your-tailnet.ts.net" spellcheck="false"
                       onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); connectAiGateway(); } }}
                       class="set-input set-input-mono min-w-0 flex-1" />
                <button onclick={connectAiGateway} disabled={aiGwTesting || !aiTitle.gateway_url.trim()} class="set-btn-quiet set-btn">
                  {aiGwTesting ? 'Connecting…' : 'Connect'}
                </button>
              </div>
              <p class="set-meta mt-1.5 px-0.5">
                No key needed: Aperture knows this server by its tailnet identity and lists the models it's allowed to use.
              </p>
            </div>

            {#if aiTitle.gateway.error}
              <p class="set-error">Couldn't reach Aperture: {aiTitle.gateway.error}</p>
            {/if}

            {#if aiTitle.gateway.reachable}
              <div>
                <p class="set-field-label">Model</p>
                {#if aiGwUsable.length}
                  <div class="set-fill-list">
                    {#each aiGwUsable as m (m.id)}
                      <label class="flex min-h-12 cursor-pointer items-center gap-3 px-3.5 py-2">
                        <input type="radio" name="ai-gateway-model" value={m.id} checked={aiTitle.gateway_model === m.id}
                               onchange={() => pickGatewayModel(m.id)} disabled={aiSaving}
                               class="set-radio" />
                        <span class="min-w-0 flex-1">
                          <span class="block truncate text-[13px] font-mono" style="color: var(--sempa-text);">{m.name}</span>
                          {#if m.provider}
                            <span class="block truncate text-[11.5px]" style="color: var(--sempa-text-dim);">{m.provider}</span>
                          {/if}
                        </span>
                        {#if m.context_window}
                          <span class="shrink-0 text-[11.5px] font-mono" style="color: var(--sempa-text-dim);">{Math.round(m.context_window / 1000)}k</span>
                        {/if}
                      </label>
                    {/each}
                  </div>
                {:else}
                  <p class="set-meta px-0.5">Aperture answered, but none of its models are usable by this server yet.</p>
                {/if}
                {#if aiGwBlocked.length}
                  <details class="mt-2">
                    <summary class="cursor-pointer px-0.5 text-[12.5px] font-semibold" style="color: var(--sempa-text-soft);">
                      {aiGwBlocked.length} more need setting up in Aperture
                    </summary>
                    <div class="set-fill-list mt-2">
                      {#each aiGwBlocked as m (m.id)}
                        <div class="px-3.5 py-2.5">
                          <p class="truncate text-[13px] font-mono" style="color: var(--sempa-text-soft);">
                            {m.name}{#if m.provider}<span class="font-sans" style="color: var(--sempa-text-dim);"> · {m.provider}</span>{/if}
                          </p>
                          <p class="set-meta mt-0.5">{m.reason}</p>
                        </div>
                      {/each}
                    </div>
                  </details>
                {/if}
              </div>
            {/if}
          {:else}
            <!-- Downloaded models: pick the active one, see its size, remove it -->
            {#if aiTitle.available_models.length > 0}
              <div>
                <p class="set-field-label">Model</p>
                <div class="set-fill-list">
                  {#each aiTitle.available_models as m (m.name)}
                    <label class="flex min-h-12 cursor-pointer items-center gap-3 px-3.5 py-2">
                      <input type="radio" name="ai-active-model" value={m.name} checked={aiTitle.model === m.name}
                             onchange={() => { aiTitle.model = m.name; saveAiTitle('Active model updated'); }}
                             class="set-radio" />
                      <span class="flex-1 truncate text-[13px] font-mono" style="color: var(--sempa-text);">{m.name}</span>
                      <span class="shrink-0 text-[11.5px] font-mono" style="color: var(--sempa-text-dim);">{fmtBytes(m.size)}</span>
                      <button onclick={() => removeAiModel(m.name)} disabled={aiRemoving === m.name}
                              class="set-icon-btn danger"
                              aria-label="Remove {m.name}" title="Remove model">
                        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="1.9" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                        </svg>
                      </button>
                    </label>
                  {/each}
                </div>
              </div>
            {/if}

            <!-- Download a model (with progress) -->
            <div>
              <label class="set-field-label" for="ai-pull">Download a model</label>
              {#if aiPulling}
                <div class="set-fill p-3.5">
                  <div class="mb-2 flex items-center justify-between text-[12px] font-semibold" style="color: var(--sempa-text-soft);">
                    <span class="truncate pr-2">{aiPullLabel}</span>
                    <span class="font-mono">{aiPullPct}%</span>
                  </div>
                  <div class="day-bar-track">
                    <div class="day-bar-fill" style="width: {aiPullPct}%; background: var(--sempa-accent);"></div>
                  </div>
                </div>
              {:else}
                <div class="flex items-center gap-2">
                  <input id="ai-pull" bind:value={aiPullModel} placeholder="e.g. qwen2.5:1.5b" spellcheck="false"
                         onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); pullAiModel(); } }}
                         class="set-input set-input-mono min-w-0 flex-1" />
                  <button onclick={pullAiModel} disabled={!aiPullModel.trim()}
                          class="btn-primary set-btn">
                    Download
                  </button>
                </div>
                <p class="set-meta mt-1.5 px-0.5">Find model names at ollama.com/library (e.g. qwen2.5:1.5b, llama3.2:3b).</p>
              {/if}
            </div>

            <!-- Model server URL -->
            <div>
              <label class="set-field-label" for="ai-url">Model server URL (Ollama)</label>
              <input id="ai-url" bind:value={aiTitle.base_url} placeholder="http://127.0.0.1:11434" spellcheck="false"
                     class="set-input set-input-mono w-full" />
            </div>

          {/if}

          <!-- Feedback -->
          {#if aiError}
            <p class="set-error">{aiError}</p>
          {:else if aiStatus}
            <p class="set-ok">{aiStatus}</p>
          {/if}
          {#if aiTryResult}
            <p class={aiTryResult.ok ? 'set-ok' : 'set-error'}>{aiTryResult.text}</p>
          {/if}

          <div class="flex flex-wrap items-center gap-2">
            <button onclick={() => saveAiTitle()} disabled={aiSaving}
                    class="btn-primary set-btn">
              {aiSaving ? 'Saving…' : 'Save'}
            </button>
            {#if aiTitle.provider === 'ollama'}
              <button onclick={testAiTitle} disabled={aiTesting}
                      class="set-btn-quiet set-btn">
                {aiTesting ? 'Testing…' : 'Test connection'}
              </button>
            {/if}
            <button onclick={tryAiModel}
                    disabled={aiTrying || (aiTitle.provider === 'aperture' ? !aiTitle.gateway_model : !aiTitle.base_url.trim())}
                    class="set-btn-quiet set-btn" title="Send one short prompt to the model and time the answer">
              {aiTrying ? 'Asking…' : 'Try it'}
            </button>
          </div>
        </div>

        <!-- Per-feature controls — full control over where the model is used.
             Each is also gated by reachability above. -->
        <div class="set-block pb-1">
          <p class="set-label">AI features</p>
          <p class="set-desc">Choose where Sempa uses the model. Turn any off to hide it across the app.</p>
        </div>
        {#each AI_FEATURE_META as f (f.key)}
          <div class="set-row">
            <div class="set-row-main">
              <p class="set-label" style="font-weight: 600;">{f.label}</p>
              <p class="set-desc">{f.hint}</p>
            </div>
            <button onclick={() => prefs.setAiFeature(f.key, !prefs.aiOn(f.key))}
                    role="switch" aria-checked={prefs.aiOn(f.key)} aria-label={f.label}
                    class="set-switch" data-on={prefs.aiOn(f.key)}></button>
          </div>
        {/each}

        <!-- Where the Import feature is exposed (only meaningful when the
             "Import from URL or text" feature above is on). -->
        {#if prefs.aiOn('aiImport')}
          <div class="set-block pb-1" style="background: var(--sempa-fill);">
            <p class="set-label">Import entry points</p>
          </div>
          {#each IMPORT_ENTRY_META as e (e.key)}
            <div class="set-row" style="background: var(--sempa-fill);">
              <div class="set-row-main pl-3">
                <p class="set-label">{e.label}</p>
                <p class="set-desc">{e.hint}</p>
              </div>
              <button onclick={() => prefs.setImportEntry(e.key, !prefs.importEntryOn(e.key))}
                      role="switch" aria-checked={prefs.importEntryOn(e.key)} aria-label={e.label}
                      class="set-switch" data-on={prefs.importEntryOn(e.key)}></button>
            </div>
          {/each}
        {/if}
      {/if}
    </section>
  </div>
{/snippet}

{#snippet tasksContent()}
  <!-- ═══════════════════════════════════════════════════════════════════════
       SECTION: Tasks
  ════════════════════════════════════════════════════════════════════════ -->
  <div id="settings-tasks">
    <div class="set-group">
      <a href="/settings/tags" class="set-row set-row-link">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 0 1 0 2.828l-7 7a2 2 0 0 1-2.828 0l-7-7A2 2 0 0 1 3 12V7a4 4 0 0 1 4-4z"/>
          </svg>
        </span>
        <span class="set-row-main">
          <span class="set-label block">Tags</span>
          <span class="set-desc block">Colour-coded labels for your tasks</span>
        </span>
        {@render chevron()}
      </a>

      <a href="/settings/projects" class="set-row set-row-link">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7zM8 11v4M12 11v2M16 11v5"/>
          </svg>
        </span>
        <span class="set-row-main">
          <span class="set-label block">Project statuses</span>
          <span class="set-desc block">Name, colour and order the stages your projects move through</span>
        </span>
        {@render chevron()}
      </a>

      <a href="/settings/recurring" class="set-row set-row-link">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
          </svg>
        </span>
        <span class="set-row-main">
          <span class="set-label block">Recurring tasks</span>
          <span class="set-desc block">Daily, weekly, and monthly templates</span>
        </span>
        {@render chevron()}
      </a>

      <a href="/settings/time" class="set-row set-row-link">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" stroke-linejoin="round" d="M12 7v5l3 3"/>
          </svg>
        </span>
        <span class="set-row-main">
          <span class="set-label block">Time tracking</span>
          <span class="set-desc block">Completion prompts, default activity times, focus timer</span>
        </span>
        {@render chevron()}
      </a>
    </div>
  </div>
{/snippet}

{#snippet systemContent()}
  <!-- ═══════════════════════════════════════════════════════════════════════
       SECTION: System — app-level concerns (notifications, backup)
  ════════════════════════════════════════════════════════════════════════ -->
  <div id="settings-system">
    <div class="set-group">
      <a href="/settings/notifications" class="set-row set-row-link">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0 1 18 14.158V11a6.002 6.002 0 0 0-4-5.659V5a2 2 0 1 0-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 1 1-6 0v-1m6 0H9"/>
          </svg>
        </span>
        <span class="set-row-main">
          <span class="set-label block">Notifications</span>
          <span class="set-desc block">Reminders, push channels, sound, and routines</span>
        </span>
        {@render chevron()}
      </a>

      <a href="/settings/backup" class="set-row set-row-link">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 7v10a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V7M3 7l2-3h14l2 3M3 7h18M9 12h6"/>
          </svg>
        </span>
        <span class="set-row-main">
          <span class="set-label block">Backup &amp; restore</span>
          <span class="set-desc block">Automatic backups, encryption, and recovery</span>
        </span>
        {@render chevron()}
      </a>

      <!-- Launch at login (desktop only) -->
      {#if isTauri()}
      <label class="set-row cursor-pointer">
        <span class="set-icon">
          <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M13 3 4 14h7l-1 7 9-11h-7l1-7Z"/>
          </svg>
        </span>
        <span class="set-row-main">
          <span class="set-label block">Launch at login</span>
          <span class="set-desc block">Start Sempa automatically and open minimized to the tray</span>
        </span>
        <input type="checkbox" checked={autostartOn} disabled={autostartBusy}
               onchange={(e) => toggleAutostart((e.target as HTMLInputElement).checked)}
               class="set-switch-input" />
        <span class="set-switch" aria-hidden="true"></span>
      </label>
      {/if}
    </div>

    <!-- Paired devices (the Sempa Dock): approve a code, see + revoke devices. -->
    <h3 class="set-heading">Paired devices</h3>
    <div class="set-group">
      <div class="set-block">
        <p class="set-desc" style="margin-top: 0;">
          Pair a Sempa Dock: enter the code it shows. The device gets a scoped, revocable token — no password ever leaves this app.
        </p>
        <div class="mt-3 flex gap-2">
          <input bind:value={pairCodeInput} placeholder="Pairing code" maxlength="8"
                 onkeydown={(e) => e.key === 'Enter' && approveDevice()}
                 class="set-input set-input-mono min-w-0 flex-1 uppercase tracking-[0.12em]" />
          <button onclick={approveDevice} disabled={pairBusy}
                  class="btn-primary set-btn">
            {pairBusy ? '…' : 'Pair'}
          </button>
        </div>
        {#if pairError}<p class="set-error mt-2">{pairError}</p>{/if}
      </div>
      {#each pairedDevices as dev (dev.id)}
        <div class="set-row set-row-compact">
          <div class="set-row-main">
            <p class="set-label truncate">{dev.device_name || 'Device'}</p>
            <p class="set-desc">{dev.platform}{dev.approved_at ? ' · paired ' + dev.approved_at.slice(0, 10) : ''}</p>
          </div>
          <button onclick={() => revokeDevice(dev.id)}
                  class="set-danger set-btn-sm">Revoke</button>
        </div>
      {/each}
    </div>
  </div>
{/snippet}

<!-- Sempa-styled on/off switch — the brand toggle used across settings. -->
{#snippet toggleSwitch(on: boolean, toggle: () => void, label: string)}
  <button onclick={toggle} role="switch" aria-checked={on} aria-label={label}
          class="set-switch" data-on={on}></button>
{/snippet}

{#snippet appearanceContent()}
  <!-- ═══════════════════════════════════════════════════════════════════════
       SECTION: Appearance
  ════════════════════════════════════════════════════════════════════════ -->
  <div id="settings-appearance">

    <!-- Theme gallery -->
    <h3 class="set-heading">Theme</h3>
    <section class="set-group">
      <div class="set-block">
        <p class="set-desc" style="margin-top: 0;">
          A full palette for the whole interface. Calm and refined — pick what feels right;
          you can switch any time.
        </p>
        <div class="mt-3.5" style="display:grid; grid-template-columns:repeat(auto-fill,minmax(150px,1fr)); gap:12px;">
          {#each theme.THEMES as t (t.id)}
            {@const selected = theme.theme === t.id}
            {@const previewDark = t.darkOnly || theme.dark}
            <button onclick={() => chooseTheme(t.id, t.label)}
                    aria-pressed={selected}
                    title={t.label}
                    class="text-left transition-shadow"
                    style="border-radius:18px; padding:6px 6px 10px; cursor:pointer; background: var(--sempa-bg-panel);
                           box-shadow: {selected
                             ? '0 0 0 2px var(--sempa-accent), 0 0 0 6px var(--sempa-accent-bg)'
                             : 'inset 0 0 0 1px var(--sempa-border)'};">
              <!-- Live mini-preview rendered in THIS theme's tokens (data-theme
                   re-declares the --sempa-* vars for this subtree): the theme's
                   canvas, a raised card on it, and its accent pill. -->
              <div data-theme={t.id} class:dark={previewDark}
                   style="display:flex; flex-direction:column; gap:8px; height:84px; padding:10px;
                          border-radius:13px; background: var(--sempa-bg-main);">
                <div style="display:flex; align-items:center; gap:7px; padding:8px 9px; border-radius:10px;
                            background: var(--card-bg); box-shadow: var(--sempa-shadow-1);">
                  <span style="width:10px; height:10px; flex-shrink:0; border-radius:50%; border:2px solid var(--sempa-accent);"></span>
                  <span style="flex:1; height:4px; border-radius:2px; background: var(--sempa-text-soft); opacity:.75;"></span>
                  <span style="width:20px; height:8px; flex-shrink:0; border-radius:999px; background: var(--sempa-fill);"></span>
                </div>
                <div style="display:flex; align-items:center; gap:7px; padding:0 2px;">
                  <span style="display:inline-flex; align-items:center; justify-content:center; width:46px; height:18px;
                               border-radius:999px; background: var(--sempa-btn-bg); box-shadow: var(--sempa-shadow-accent);">
                    <span style="width:22px; height:3px; border-radius:2px; background: var(--sempa-btn-fg);"></span>
                  </span>
                  <span style="flex:1; height:6px; border-radius:999px; background: var(--sempa-track);"></span>
                </div>
              </div>
              <div class="mt-2 flex items-center justify-between gap-2 px-1.5">
                <div class="min-w-0">
                  <p style="font-size:13.5px; font-weight:700; color: var(--sempa-text);">{t.label}</p>
                  <p style="font-size:12px; color: var(--sempa-text-dim);">{t.sublabel}</p>
                </div>
                {#if selected}
                  <span aria-hidden="true"
                        style="display:flex; align-items:center; justify-content:center; width:20px; height:20px;
                               border-radius:50%; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); flex-shrink:0;">
                    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                    </svg>
                  </span>
                {/if}
              </div>
            </button>
          {/each}
        </div>

        {#if iconPromptTheme}
          <div class="mt-3.5 flex flex-wrap items-center justify-between gap-3 rounded-2xl p-3 pl-4"
               style="background: var(--sempa-accent-bg);">
            <p class="text-[13px] leading-snug" style="color: var(--sempa-text);">
              Match your app icon to <strong>{iconPromptTheme.label}</strong>?
              <span style="color: var(--sempa-text-soft);">The home-screen icon will refresh.</span>
            </p>
            <div class="flex shrink-0 items-center gap-1.5">
              <button onclick={() => (iconPromptTheme = null)}
                      class="set-ghost set-btn-sm">Not now</button>
              <button onclick={confirmAppIcon}
                      class="btn-primary set-btn-sm">Update icon</button>
            </div>
          </div>
        {/if}
      </div>
    </section>

    <!-- Display -->
    <h3 class="set-heading">Display</h3>
    <section class="set-group">
      <!-- Text size -->
      <div class="set-block">
        <div class="flex items-center justify-between gap-3">
          <p class="set-label">Text size</p>
          <div class="flex items-center gap-1">
            <span class="font-mono text-[12px]" style="color: var(--sempa-text-soft);">{theme.textScale}%</span>
            <button onclick={() => theme.setScale(100)}
                    class="set-ghost set-btn-sm">
              Reset to default
            </button>
          </div>
        </div>
        <div class="mt-2" style="display:flex; align-items:center; gap:14px;">
          <span style="font-size:12px; font-weight:600; color:var(--sempa-text-dim)">A</span>
          <input type="range" min="80" max="130" step="5"
                 value={theme.textScale}
                 oninput={(e) => theme.setScale(parseInt((e.target as HTMLInputElement).value, 10))}
                 aria-label="Text size"
                 style="flex:1; accent-color: var(--sempa-accent);" />
          <span style="font-size:19px; font-weight:700; color:var(--sempa-text)">A</span>
        </div>
      </div>

      <!-- Mode: System / Light / Dark. System follows the OS colour scheme live
           (org.freedesktop.appearance on Linux). Disabled for dark-only OLED. -->
      <div class="set-row set-row-wrap">
        <div class="set-row-main">
          <p class="set-label">Mode</p>
          {#if theme.darkOnly}
            <p class="set-desc">OLED Black is a dark-only theme.</p>
          {:else}
            <p class="set-desc">System follows your device.</p>
          {/if}
        </div>
        <div class="set-seg" style={theme.darkOnly ? 'opacity:0.5; pointer-events:none;' : ''}>
          {#each [['system', 'System'], ['light', 'Light'], ['dark', 'Dark']] as [val, label] (val)}
            <button onclick={() => theme.setMode(val as 'system' | 'light' | 'dark')}
                    disabled={theme.darkOnly} aria-disabled={theme.darkOnly}
                    aria-pressed={theme.mode === val}
                    data-active={theme.mode === val}>
              {label}
            </button>
          {/each}
        </div>
      </div>

      <!-- Match system accent — opt-in, only when the platform exposes an accent
           colour to the webview (Linux GNOME 47+/KDE; also Windows). -->
      {#if theme.canMatchAccent}
      <div class="set-row">
        <div class="set-row-main">
          <p class="set-label">Match system accent</p>
          <p class="set-desc">Use your desktop's accent colour instead of the theme's. Off keeps the Sempa accent.</p>
        </div>
        {@render toggleSwitch(theme.matchAccent, () => theme.setMatchAccent(!theme.matchAccent), 'Match system accent')}
      </div>
      {/if}

      <!-- Use system UI font — swap the brand sans for the native UI font. -->
      <div class="set-row">
        <div class="set-row-main">
          <p class="set-label">Use system UI font</p>
          <p class="set-desc">Render the interface in your device's font instead of Plus Jakarta Sans.</p>
        </div>
        {@render toggleSwitch(theme.systemFont, () => theme.setUiFont(theme.systemFont ? 'brand' : 'system'), 'Use system UI font')}
      </div>

      <!-- Use system title bar — hand window decorations back to the WM (desktop). -->
      {#if isTauri()}
      <div class="set-row">
        <div class="set-row-main">
          <p class="set-label">Use system title bar</p>
          <p class="set-desc">
            Let your desktop draw the window frame instead of Sempa's. Off keeps the in-app title bar,
            which mirrors your system's window-button layout.
          </p>
        </div>
        {@render toggleSwitch(windowChrome.useSystemTitlebar, () => windowChrome.setUseSystemTitlebar(!windowChrome.useSystemTitlebar), 'Use system title bar')}
      </div>
      {/if}
    </section>

    <!-- Bottom bar (phones, 1.28): pick which places sit in the tab bar.
         Plain buttons rather than drag, so it works one-handed and with a
         screen reader. -->
    {#if mobile.value}
    <h3 class="set-heading" id="settings-tabs" style="scroll-margin-top: 16px;">Bottom bar</h3>
    <section class="set-group">
      <div class="set-block">
        <p class="set-desc" style="margin-top: 0;">
          Pick up to {MAX_TABS} places for the tab bar. Everything else stays one tap away under More.
        </p>
        <div class="tabs-preview mt-3" aria-hidden="true">
          {#each pinnedTabs as d, i (d.id)}
            <span class="tabs-preview-item" data-active={i === 0}>
              <span class="tabs-preview-pill"><d.icon size={18} strokeWidth={i === 0 ? 2.3 : 1.9} /></span>
              <span class="tabs-preview-label">{d.short}</span>
            </span>
          {/each}
          <span class="tabs-preview-item">
            <span class="tabs-preview-pill"><MenuIcon size={18} strokeWidth={1.9} /></span>
            <span class="tabs-preview-label">More</span>
          </span>
        </div>
      </div>
      {#each pinnedTabs as d, i (d.id)}
        <div class="set-row set-row-compact" style="gap: 12px; padding-right: 10px;">
          <span class="set-icon tint" style="--tint: {d.hue}; width: 34px; height: 34px; border-radius: 11px;">
            <d.icon size={17} strokeWidth={2} />
          </span>
          <p class="set-row-main set-label truncate">{d.label}{#if d.needs === 'jira' && !jiraStatus.connected}<span class="set-meta"> · hidden until Jira is connected</span>{/if}</p>
          <span class="flex shrink-0 items-center">
            <button class="set-icon-btn" aria-label={`Move ${d.label} up`} disabled={i === 0} onclick={() => moveTab(i, -1)}>
              <ChevronUp size={18} strokeWidth={2.2} />
            </button>
            <button class="set-icon-btn" aria-label={`Move ${d.label} down`} disabled={i === pinnedTabs.length - 1} onclick={() => moveTab(i, 1)}>
              <ChevronDown size={18} strokeWidth={2.2} />
            </button>
            <button class="set-icon-btn danger" aria-label={`Remove ${d.label} from the bar`} disabled={pinnedTabs.length <= MIN_TABS} onclick={() => removeTab(d.id)}>
              <Minus size={18} strokeWidth={2.2} />
            </button>
          </span>
        </div>
      {/each}
    </section>
    <div class="mt-2 flex items-center justify-between gap-3 px-1.5">
      <p class="set-meta">
        {#if pinnedTabs.length >= MAX_TABS}The bar is full. Remove one to make room.
        {:else if pinnedTabs.length <= MIN_TABS}Keep at least {MIN_TABS} in the bar.
        {:else}Room for {MAX_TABS - pinnedTabs.length} more.{/if}
      </p>
      <button class="set-btn-quiet set-btn-sm" onclick={prefs.resetMobileTabs} disabled={tabsAreDefault}>Reset</button>
    </div>

    <h3 class="set-heading">Under More</h3>
    <section class="set-group">
      {#each unpinnedTabs as d (d.id)}
        <div class="set-row set-row-compact" style="gap: 12px; padding-right: 12px;">
          <span class="set-icon tint" style="--tint: {d.hue}; width: 34px; height: 34px; border-radius: 11px;">
            <d.icon size={17} strokeWidth={2} />
          </span>
          <p class="set-row-main set-label truncate">{d.label}</p>
          <button class="set-btn-quiet set-btn-sm" aria-label={`Add ${d.label} to the bar`}
                  disabled={pinnedTabs.length >= MAX_TABS} onclick={() => addTab(d.id)}>
            <PlusIcon size={14} strokeWidth={2.4} /> Add
          </button>
        </div>
      {/each}
    </section>
    {/if}

    <!-- Sidebar navigation grouping (desktop rail only) -->
    {#if !mobile.value}
    <h3 class="set-heading">Sidebar</h3>
    <section class="set-group">
      <div class="set-block">
        <p class="set-label">Sidebar grouping</p>
        <p class="set-desc">How the left navigation rail is organised.</p>
        <div class="set-seg mt-3">
          {#each [['spaces', 'Spaces'], ['rhythm', 'Plan · Focus · Review'], ['flat', 'Flat']] as [val, label]}
            <button onclick={() => prefs.setNavGrouping(val as 'spaces' | 'rhythm' | 'flat')}
                    aria-pressed={prefs.navGrouping === val}
                    data-active={prefs.navGrouping === val}>
              {label}
            </button>
          {/each}
        </div>
      </div>

      <div class="set-block">
        <p class="set-label">Section style</p>
        {#if prefs.navGrouping === 'flat'}
          <p class="set-desc">Section labels apply to grouped layouts.</p>
        {:else}
          <p class="set-desc">Name each group, or separate them with a line.</p>
        {/if}
        <div class="set-seg mt-3" style={prefs.navGrouping === 'flat' ? 'opacity:0.5; pointer-events:none;' : ''}>
          {#each [['labels', 'Labels'], ['dividers', 'Dividers']] as [val, label]}
            <button onclick={() => prefs.setNavSections(val as 'labels' | 'dividers')}
                    aria-pressed={prefs.navSections === val}
                    data-active={prefs.navSections === val}>
              {label}
            </button>
          {/each}
        </div>
      </div>
    </section>
    {/if}
  </div>
{/snippet}

{#snippet personalContent()}
  <!-- ═══════════════════════════════════════════════════════════════════════
       SECTION: Personalization — the personality/behaviour toggles that used to
       crowd Appearance (reflections, daily quote, celebration sound).
  ════════════════════════════════════════════════════════════════════════ -->
  <div id="settings-personal">
    <section class="set-group">

      <!-- Contextual reflections toggle -->
      <div class="set-row">
        <div class="set-row-main">
          <p class="set-label">Show reflections in context</p>
          <p class="set-desc">
            Surface your daily intention &amp; reflection on the day view, and the week-review
            summary on the week view. Turn off to keep them only in the Journal.
          </p>
        </div>
        <button onclick={prefs.toggleContextualReflections}
                role="switch" aria-checked={prefs.contextualReflections}
                aria-label="Show reflections in context"
                class="set-switch" data-on={prefs.contextualReflections}></button>
      </div>

      <!-- Daily encouragement (rotating quote) -->
      <div>
        <div class="set-row">
          <div class="set-row-main">
            <p class="set-label">Daily encouragement</p>
            <p class="set-desc">
              A quiet, rotating quote on the day view — subtle by design. Choose which packs to draw from,
              add your own, or turn it off.
            </p>
          </div>
          <button onclick={() => quotes.setEnabled(!quotes.enabled)}
                  role="switch" aria-checked={quotes.enabled} aria-label="Daily encouragement"
                  class="set-switch" data-on={quotes.enabled}></button>
        </div>

        {#if quotes.enabled}
          <div class="px-[18px] pb-4">
            <!-- Quote packs: toggle on/off and combine freely -->
            <div class="mb-2 flex items-center justify-between px-0.5">
              <p class="text-[12.5px] font-semibold" style="color: var(--sempa-text-soft);">Packs</p>
              <span class="set-meta">{quotes.poolSize} quote{quotes.poolSize === 1 ? '' : 's'} in rotation</span>
            </div>
            <div class="set-fill-list">
              {#each quotes.packs as p (p.id)}
                <button onclick={() => quotes.togglePack(p.id)}
                        aria-pressed={quotes.packEnabled(p.id)}
                        class="flex w-full items-center justify-between gap-3 px-3.5 py-3 text-left">
                  <span class="min-w-0">
                    <span class="block text-[13px] font-semibold" style="color: var(--sempa-text);">{p.name}
                      <span class="font-normal" style="color: var(--sempa-text-dim);">· {p.quotes.length}</span>
                    </span>
                    <span class="mt-0.5 block text-[12px] leading-relaxed" style="color: var(--sempa-text-soft);">{p.description}</span>
                  </span>
                  <span class="set-switch" role="switch" aria-checked={quotes.packEnabled(p.id)}
                        data-on={quotes.packEnabled(p.id)}></span>
                </button>
              {/each}
            </div>
            <button onclick={() => quotes.resetPacks()} class="set-ghost set-btn-sm -ml-1 mt-1.5">
              Reset packs to defaults
            </button>

            <!-- Your own quotes -->
            <p class="mb-2 mt-3 px-0.5 text-[12.5px] font-semibold" style="color: var(--sempa-text-soft);">Your own quotes</p>
            {#if quotes.custom.length}
              <div class="set-fill-list mb-2">
                {#each quotes.custom as q, i (q.text + i)}
                  <div class="flex items-start gap-2 py-2 pl-3.5 pr-1.5">
                    <span class="min-w-0 flex-1 pt-1 text-[12.5px] leading-relaxed" style="color: var(--sempa-text);">
                      “{q.text}”{#if q.author} <span style="color: var(--sempa-text-soft);">— {q.author}</span>{/if}
                    </span>
                    <button onclick={() => quotes.removeCustom(i)} aria-label="Remove quote"
                            class="set-icon-btn danger">
                      <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" d="M6 18 18 6M6 6l12 12"/></svg>
                    </button>
                  </div>
                {/each}
              </div>
            {/if}

            <div class="flex flex-wrap items-center gap-2">
              <input bind:value={newQuoteText} placeholder="Quote"
                     onkeydown={(e) => { if (e.key === 'Enter') addQuote(); }}
                     class="set-input min-w-0 flex-1 text-[13px]" />
              <input bind:value={newQuoteAuthor} placeholder="Author (optional)"
                     onkeydown={(e) => { if (e.key === 'Enter') addQuote(); }}
                     class="set-input w-36 text-[13px]" />
              <button onclick={addQuote} disabled={!newQuoteText.trim()}
                      class="btn-primary set-btn">Add</button>
            </div>
          </div>
        {/if}
      </div>

      <!-- Celebration sound -->
      <div class="set-row">
        <div class="set-row-main">
          <p class="set-label">Celebration sound</p>
          <p class="set-desc">
            A soft chime on the day &amp; week celebration moments. The visuals always play; this only adds sound.
          </p>
        </div>
        <button onclick={prefs.toggleCelebrateSound}
                role="switch" aria-checked={prefs.celebrateSound} aria-label="Celebration sound"
                class="set-switch" data-on={prefs.celebrateSound}></button>
      </div>
    </section>
  </div>
{/snippet}

{#snippet aboutContent()}
  <!-- ═══════════════════════════════════════════════════════════════════════
       SECTION: About / Updates
  ════════════════════════════════════════════════════════════════════════ -->
  <div id="settings-about" class="scroll-mt-6">
    <UpdatesPanel />
  </div>
{/snippet}
