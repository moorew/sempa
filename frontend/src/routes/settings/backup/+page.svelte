<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { backupHealth } from '$lib/stores/backupHealth.svelte';
  import { isTauri, openExternal } from '$lib/tauri/bridge';
  import type { BackupDestination, BackupRun, BackupSettings, BackupDestinationType } from '$lib/types';

  let loading = $state(true);
  let saving = $state(false);
  let error = $state('');
  let notice = $state('');

  // Form state
  let enabled = $state(false);
  let scheduleHour = $state(3);
  let retention = $state(7);
  let securityMode = $state<'none' | 'encrypt' | 'exclude_secrets'>('none');
  let passphrase = $state('');           // only sent when non-empty
  let hasPassphrase = $state(false);
  let destinations = $state<BackupDestination[]>([]);

  let runs = $state<BackupRun[]>([]);
  let lastRunAt = $state<string | null>(null);
  let lastStatus = $state<string | null>(null);
  let driveConnected = $state(false);
  let driveNeedsReconnect = $state(false);
  let googleOAuth = $state(false);
  let driveReturnPending = $state(false); // desktop: re-check Drive status on window refocus

  // Manual restore
  let restoreFile = $state<File | null>(null);
  let restorePassphrase = $state('');
  let restoreConfirm = $state(false);
  let restoring = $state(false);
  let restorePct = $state<number | null>(null);

  let runningNow = $state(false);
  let testResults = $state<Record<string, string>>({});

  // Capacitor (native) bits — loaded dynamically, only present on Android.
  let Capacitor: any = null;
  let CapBrowser: any = null;
  let isNative = $state(false);
  let browserListener: { remove: () => void } | null = null;
  let driveAppUrlListener: { remove: () => void } | null = null;
  let messageListener: ((e: MessageEvent) => void) | null = null;
  let focusListener: (() => void) | null = null;

  onMount(() => {
    void load();
    void initNative();

    // Full-redirect fallback (e.g. popup blocked) lands back here with a flag.
    const driveParam = page.url.searchParams.get('drive');
    if (driveParam === 'connected') notice = 'Google Drive connected.';
    else if (driveParam === 'error') error = 'Google Drive connection failed. Please try again.';

    // Web popup notifies us via postMessage when the OAuth window finishes.
    messageListener = (e: MessageEvent) => {
      if (e.data === 'sempa-drive-connected') { notice = 'Google Drive connected.'; void refreshDriveStatus(); }
      else if (e.data === 'sempa-drive-error') { error = 'Google Drive connection failed.'; }
    };
    window.addEventListener('message', messageListener);

    // Desktop: the consent flow runs in the OS browser, so re-check status when
    // the app window regains focus after the user comes back.
    focusListener = () => {
      if (driveReturnPending) { driveReturnPending = false; void refreshDriveStatus().then(() => { if (driveConnected) notice = 'Google Drive connected.'; }); }
    };
    window.addEventListener('focus', focusListener);
  });

  onDestroy(() => {
    if (messageListener) window.removeEventListener('message', messageListener);
    if (focusListener) window.removeEventListener('focus', focusListener);
    browserListener?.remove();
    driveAppUrlListener?.remove();
  });

  async function initNative() {
    try {
      const cap = await import('@capacitor/core');
      Capacitor = cap.Capacitor;
      isNative = Capacitor.isNativePlatform();
      if (isNative) {
        const b = await import('@capacitor/browser');
        CapBrowser = b.Browser;
      }
    } catch { /* not on a Capacitor platform */ }
  }

  async function refreshDriveStatus() {
    try {
      const s = await api.backup.driveStatus();
      driveConnected = s.connected;
      driveNeedsReconnect = !!s.needs_reconnect;
      // Reconnecting here is what dismisses the app-wide backup banner, so pull
      // the health summary back in step rather than waiting out its poll.
      void backupHealth.refresh();
    } catch { /* ignore */ }
  }

  async function load() {
    loading = true; error = '';
    try {
      const r = await api.backup.getSettings();
      const s = r.settings;
      enabled = s.enabled;
      scheduleHour = s.schedule_hour;
      retention = s.retention;
      securityMode = s.security_mode;
      hasPassphrase = s.has_passphrase;
      destinations = parseDestinations(s.destinations);
      runs = r.runs ?? [];
      lastRunAt = s.last_run_at;
      lastStatus = s.last_status;
      driveConnected = r.drive_connected;
      googleOAuth = r.google_oauth;
      // Probe the live Drive token so an expired/revoked one surfaces a reconnect
      // prompt rather than reading as "connected" while every backup fails.
      if (r.drive_connected) void refreshDriveStatus();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load backup settings';
    } finally {
      loading = false;
    }
  }

  function parseDestinations(raw: string): BackupDestination[] {
    try { const v = JSON.parse(raw || '[]'); return Array.isArray(v) ? v : []; }
    catch { return []; }
  }

  const HOURS = Array.from({ length: 24 }, (_, h) => ({
    value: h,
    label: new Date(2000, 0, 1, h).toLocaleTimeString(undefined, { hour: 'numeric' }),
  }));

  const destTypeLabel: Record<BackupDestinationType, string> = {
    drive: 'Google Drive', s3: 'S3-compatible', webdav: 'WebDAV / Nextcloud', local: 'Local folder',
  };

  function addDestination(type: BackupDestinationType) {
    destinations = [...destinations, {
      id: crypto.randomUUID(), type, name: destTypeLabel[type], enabled: true,
      region: type === 's3' ? 'us-east-1' : undefined,
    }];
  }
  function removeDestination(id: string) {
    destinations = destinations.filter(d => d.id !== id);
  }

  async function save() {
    saving = true; error = ''; notice = '';
    try {
      const payload: Parameters<typeof api.backup.updateSettings>[0] = {
        enabled,
        schedule_hour: scheduleHour,
        retention: Math.max(1, retention || 1),
        security_mode: securityMode,
        destinations,
      };
      if (passphrase.trim()) payload.passphrase = passphrase;
      const r = await api.backup.updateSettings(payload);
      const s = r.settings;
      hasPassphrase = s.has_passphrase;
      destinations = parseDestinations(s.destinations);
      passphrase = '';
      notice = 'Settings saved.';
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    } finally {
      saving = false;
    }
  }

  async function testDestination(id: string) {
    testResults = { ...testResults, [id]: '…' };
    try {
      const r = await api.backup.test(id);
      testResults = { ...testResults, [id]: r.ok ? `OK · ${r.existing_backups ?? 0} backups found` : `Failed: ${r.error}` };
    } catch (e) {
      testResults = { ...testResults, [id]: `Failed: ${e instanceof Error ? e.message : 'error'}` };
    }
  }

  async function runNow() {
    runningNow = true; error = ''; notice = '';
    try {
      const r = await api.backup.run();
      if (r.error) error = `Backup ran with errors: ${r.error}`;
      else notice = 'Backup completed.';
      await load();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Backup failed';
    } finally {
      runningNow = false;
    }
  }

  function downloadBackup() {
    window.open(api.backup.downloadUrl(), '_blank');
  }

  async function connectDrive() {
    const url = api.backup.driveAuthUrl(isNative);
    error = ''; notice = '';
    if (isNative && CapBrowser) {
      // Android: open consent in a Chrome Custom Tab (keeps the app's WebView
      // intact). The token is stored server-side; when the server finishes it
      // bounces to a com.clevercode.sempa://drive-backup deep link that
      // re-foregrounds the app — we catch it via appUrlOpen, close the tab, and
      // re-check status. browserFinished covers the manual-cancel path (user
      // backs out of the tab without finishing).
      browserListener?.remove();
      driveAppUrlListener?.remove();
      try {
        const { App } = await import('@capacitor/app');
        driveAppUrlListener = await App.addListener('appUrlOpen', (d: { url?: string }) => {
          if (!d?.url || !d.url.includes('drive-backup')) return;
          void CapBrowser.close?.().catch(() => {});
          if (d.url.includes('drive=error')) error = 'Google Drive connection failed. Please try again.';
          else notice = 'Google Drive connected.';
          void refreshDriveStatus();
          driveAppUrlListener?.remove();
          driveAppUrlListener = null;
        });
      } catch { /* @capacitor/app unavailable */ }
      browserListener = await CapBrowser.addListener('browserFinished', () => {
        void refreshDriveStatus();
      });
      await CapBrowser.open({ url });
      return;
    }
    if (isTauri()) {
      // Desktop: open the consent flow in the OS browser. Navigating the main
      // webview here would strand the app on a remote origin (the SQL plugin is
      // ACL-denied off the local app → "everything disappeared" + re-login). The
      // auth URL already carries the desktop token, so the server completes it;
      // we re-check status when the user returns to the window.
      void openExternal(url);
      driveReturnPending = true;
      return;
    }
    // Web: open a popup so we never navigate away from this page.
    const popup = window.open(url, 'sempa-drive', 'popup,width=520,height=680');
    if (!popup) {
      // Popup blocked — fall back to a full-page redirect (lands back here).
      window.location.href = url;
    }
  }
  async function disconnectDrive() {
    await api.backup.driveDisconnect();
    driveConnected = false;
  }

  function onRestorePick(e: Event) {
    const input = e.target as HTMLInputElement;
    restoreFile = input.files?.[0] ?? null;
    restoreConfirm = false;
  }

  async function doRestore() {
    if (!restoreFile) return;
    restoring = true; error = ''; notice = ''; restorePct = 0;
    try {
      await api.backup.restore(restoreFile, restorePassphrase || undefined, (p) => (restorePct = p));
      notice = 'Restore complete. Reloading…';
      setTimeout(() => location.reload(), 1200);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Restore failed';
    } finally {
      restoring = false;
      restorePct = null;
      restoreConfirm = false;
    }
  }

  function fmtBytes(n: number | null): string {
    if (!n) return '—';
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
    if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
    return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }
  function fmtDate(s: string | null): string {
    if (!s) return 'never';
    const d = new Date(s);
    return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  }
</script>

<div class="mx-auto max-w-[640px] px-5 pt-6 pb-24 sm:px-8 sm:pt-8">
  <a href="/settings/accounts" class="set-back">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" d="m15 18-6-6 6-6"/>
    </svg>
    Settings
  </a>

  <h1 class="set-title type-page">Backup &amp; restore</h1>
  <p class="set-lede mb-6">
    Bundle everything — tasks, objectives, plans, and attached files — into one file you can restore anywhere.
    <a href="https://sempa.ca/docs.html#backup" target="_blank" rel="noopener">Full setup &amp; restore guide →</a>
  </p>

  {#if loading}
    <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
  {:else}
    <!-- Status -->
    <section class="set-group">
      <div class="set-row set-row-wrap" style="min-height: 68px;">
        <div class="set-row-main min-w-[160px]">
          <p class="set-label">Last backup</p>
          <p class="set-desc">
            {#if lastStatus === 'error'}
              <span class="font-bold" style="color: var(--set-danger);">Failed</span> · {fmtDate(lastRunAt)}
            {:else if lastRunAt}
              <span class="font-bold" style="color: var(--sempa-success);">Success</span> · {fmtDate(lastRunAt)}
            {:else}
              No backups yet
            {/if}
          </p>
        </div>
        <div class="flex gap-2">
          <button onclick={downloadBackup}
                  class="set-btn-quiet set-btn">
            Download now
          </button>
          <button onclick={runNow} disabled={runningNow}
                  class="btn-primary set-btn">
            {runningNow ? 'Running…' : 'Back up now'}
          </button>
        </div>
      </div>
    </section>

    <!-- Schedule -->
    <h2 class="set-heading">Schedule</h2>
    <section class="set-group">
      <label class="set-row cursor-pointer">
        <span class="set-row-main">
          <span class="set-label block">Automatic daily backup</span>
          <span class="set-desc block">Runs once a day and keeps the most recent copies.</span>
        </span>
        <input type="checkbox" bind:checked={enabled} class="set-switch-input" />
        <span class="set-switch" aria-hidden="true"></span>
      </label>
      <div class="set-row set-row-compact">
        <label class="set-row-main set-label" for="hour">Run at</label>
        <span class="set-select">
          <select id="hour" bind:value={scheduleHour}
                  class="set-input w-[128px]">
            {#each HOURS as h}<option value={h.value}>{h.label}</option>{/each}
          </select>
        </span>
      </div>
      <div class="set-row set-row-compact">
        <label class="set-row-main set-label" for="ret">Keep last</label>
        <div class="flex items-center gap-2">
          <input id="ret" type="number" min="1" bind:value={retention}
                 class="set-input set-input-num" />
          <span class="set-meta">backups</span>
        </div>
      </div>
    </section>

    <!-- Security -->
    <h2 class="set-heading">Security</h2>
    <section class="set-group">
      {#each [
        { v: 'none', t: 'Standard', d: 'Plain bundle. Includes your integration tokens.' },
        { v: 'encrypt', t: 'Encrypt with passphrase', d: 'AES-256. You need the passphrase to restore — store it safely.' },
        { v: 'exclude_secrets', t: 'Exclude integration secrets', d: 'Everything except Gmail/Jira/Fastmail tokens. Re-link them after a restore.' },
      ] as opt}
        <label class="set-row cursor-pointer"
               style={securityMode === opt.v ? 'background: color-mix(in srgb, var(--sempa-accent-bg) 55%, transparent);' : ''}>
          <input type="radio" name="security" value={opt.v} bind:group={securityMode} class="set-radio" />
          <span class="set-row-main">
            <span class="set-label block">{opt.t}</span>
            <span class="set-desc block">{opt.d}</span>
          </span>
        </label>
      {/each}
      {#if securityMode === 'encrypt'}
        <div class="set-block">
          <label class="set-field-label" for="pass">
            Passphrase {#if hasPassphrase}<span class="font-medium" style="color: var(--sempa-text-dim);">— leave blank to keep current</span>{/if}
          </label>
          <input id="pass" type="password" bind:value={passphrase} autocomplete="new-password"
                 placeholder={hasPassphrase ? '••••••••' : 'Enter a passphrase'}
                 class="set-input w-full" />
        </div>
      {/if}
    </section>

    <!-- Destinations -->
    <h2 class="set-heading">Destinations</h2>
    <p class="set-desc -mt-1 mb-3 px-1.5">Where automatic backups are sent. Manual downloads always work without one.</p>
    {#if destinations.length}
      <div class="flex flex-col gap-3">
        {#each destinations as dest (dest.id)}
          <section class="set-group">
            <div class="set-row set-row-compact">
              <span class="tint chip" style="--tint: var(--sempa-accent);">{destTypeLabel[dest.type]}</span>
              <span class="flex-1"></span>
              <label class="flex cursor-pointer items-center gap-2 text-[12.5px] font-semibold" style="color: var(--sempa-text-soft);">
                Enabled
                <input type="checkbox" bind:checked={dest.enabled} class="set-switch-input" />
                <span class="set-switch" aria-hidden="true"></span>
              </label>
              <button onclick={() => removeDestination(dest.id)} class="set-danger set-btn-sm">Remove</button>
            </div>

            <div class="set-block space-y-2">
              <input bind:value={dest.name} placeholder="Name" aria-label="Destination name"
                     class="set-input w-full" />

              {#if dest.type === 'local'}
                <input bind:value={dest.path} placeholder="/path/on/server (e.g. /data/backups)"
                       class="set-input set-input-mono w-full" />
              {:else if dest.type === 'webdav'}
                <div class="grid gap-2">
                  <input bind:value={dest.url} placeholder="https://nextcloud.example.com/remote.php/dav/files/me/Backups"
                         class="set-input w-full" />
                  <div class="grid grid-cols-2 gap-2">
                    <input bind:value={dest.username} placeholder="Username"
                           class="set-input min-w-0" />
                    <input bind:value={dest.password} type="password" placeholder="Password / app token"
                           class="set-input min-w-0" />
                  </div>
                </div>
              {:else if dest.type === 's3'}
                <div class="grid gap-2">
                  <div class="grid grid-cols-2 gap-2">
                    <input bind:value={dest.bucket} placeholder="Bucket"
                           class="set-input min-w-0" />
                    <input bind:value={dest.region} placeholder="Region (us-east-1)"
                           class="set-input min-w-0" />
                  </div>
                  <input bind:value={dest.endpoint} placeholder="Endpoint (blank for AWS; e.g. https://s3.us-west-002.backblazeb2.com)"
                         class="set-input w-full" />
                  <input bind:value={dest.prefix} placeholder="Key prefix (optional, e.g. sempa/)"
                         class="set-input w-full" />
                  <div class="grid grid-cols-2 gap-2">
                    <input bind:value={dest.access_key_id} placeholder="Access key ID"
                           class="set-input min-w-0" />
                    <input bind:value={dest.secret_access_key} type="password" placeholder="Secret access key"
                           class="set-input min-w-0" />
                  </div>
                </div>
              {:else if dest.type === 'drive'}
                <div class="space-y-2.5">
                  {#if driveConnected && driveNeedsReconnect}
                    <div class="tint rounded-2xl px-4 py-3" style="--tint: var(--sempa-amber);">
                      <p class="text-[13px] font-bold">
                        Google access expired — backups are failing. Reconnect to restore them.
                      </p>
                      <p class="mt-1 text-[12.5px] leading-relaxed" style="color: var(--sempa-text-soft);">
                        This happens because the Google OAuth app is in “Testing”, which expires tokens after 7 days.
                        <a href="https://console.cloud.google.com/auth/audience" target="_blank" rel="noopener"
                           class="font-semibold underline" style="color: var(--sempa-accent-ink);">Publish it to Production</a>
                        to fix it permanently.
                      </p>
                      <button onclick={connectDrive}
                              class="btn-primary set-btn-sm mt-2.5">
                        Reconnect Google Drive
                      </button>
                    </div>
                  {:else if driveConnected}
                    <div class="flex flex-wrap items-center gap-2 text-[13px]" style="color: var(--sempa-text-soft);">
                      <span class="set-status">Connected</span>
                      <span>Backups go to a “Sempa Backups” folder.</span>
                      <button onclick={disconnectDrive} class="set-ghost set-btn-sm" style="color: var(--set-danger);">Disconnect</button>
                    </div>
                  {:else if googleOAuth}
                    <button onclick={connectDrive}
                            class="btn-soft set-btn">
                      Connect Google Drive
                    </button>
                  {:else}
                    <p class="text-[12.5px] font-semibold" style="color: var(--sempa-text-soft);">Google OAuth isn’t configured on this server (set GMAIL_CLIENT_ID/SECRET).</p>
                  {/if}
                  <p class="text-[12px]">
                    <a href="https://sempa.ca/docs.html#backup" target="_blank" rel="noopener"
                       class="font-semibold underline" style="color: var(--sempa-accent-ink);">Google Drive setup &amp; reconnect guide →</a>
                  </p>
                  <input bind:value={dest.folder_id} placeholder="Advanced: specific Drive folder ID (blank = auto “Sempa Backups” folder)"
                         class="set-input w-full" />
                </div>
              {/if}

              <div class="flex flex-wrap items-center gap-3 pt-1">
                <button onclick={() => testDestination(dest.id)} class="set-btn-quiet set-btn-sm">Test connection</button>
                {#if testResults[dest.id]}<span class="set-meta">{testResults[dest.id]}</span>{/if}
              </div>
            </div>
          </section>
        {/each}
      </div>
    {/if}

    <div class="mt-3 flex flex-wrap gap-2">
      {#each (['local','drive','s3','webdav'] as BackupDestinationType[]) as t}
        <button onclick={() => addDestination(t)}
                class="btn-surface set-btn-sm">
          + {destTypeLabel[t]}
        </button>
      {/each}
    </div>

    <!-- Restore -->
    <h2 class="set-heading">Restore</h2>
    <section class="set-group" style="background: color-mix(in srgb, var(--sempa-amber) 9%, var(--sempa-bg-panel));">
      <div class="set-block">
        <p class="set-label">Restore from a backup</p>
        <p class="set-desc">
          This <strong style="color: var(--sempa-text);">erases all current data</strong> and replaces it with the backup. Encrypted backups need their passphrase.
        </p>
        <label class="btn-surface set-btn mt-3 max-w-full cursor-pointer">
          <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 16V4m0 0L8 8m4-4 4 4M4 16v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2"/>
          </svg>
          <span class="truncate">{restoreFile ? restoreFile.name : 'Choose backup file…'}</span>
          <input type="file" accept=".zip,.enc" onchange={onRestorePick} class="hidden" />
        </label>
        {#if restoreFile}
          <input type="password" bind:value={restorePassphrase} placeholder="Passphrase (only if encrypted)"
                 class="set-input set-input-raised mt-3 w-full" />
          {#if restorePct !== null}
            <div class="day-bar-track mt-3">
              <div class="day-bar-fill" style="width: {restorePct}%; background: var(--sempa-amber);"></div>
            </div>
          {/if}
          <div class="mt-3">
            {#if !restoreConfirm}
              <button onclick={() => restoreConfirm = true} disabled={restoring}
                      class="set-danger set-btn">
                Restore “{restoreFile.name}”
              </button>
            {:else}
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-sm font-semibold" style="color: var(--sempa-text);">Erase everything and restore?</span>
                <button onclick={doRestore} disabled={restoring}
                        class="set-btn inline-flex items-center rounded-full font-bold text-white disabled:opacity-50"
                        style="background: var(--set-danger-solid);">
                  {restoring ? 'Restoring…' : 'Yes, restore'}
                </button>
                <button onclick={() => restoreConfirm = false} class="set-ghost set-btn">Cancel</button>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    </section>

    <!-- History -->
    {#if runs.length}
      <h2 class="set-heading">Recent backups</h2>
      <section class="set-group">
        {#each runs as run}
          <div class="set-row set-row-compact" style="min-height: 48px;">
            <span class="h-2 w-2 shrink-0 rounded-full" style="background: {run.status === 'success' ? 'var(--sempa-success)' : 'var(--set-danger)'};"></span>
            <span class="set-row-main text-[13px]">
              <span class="font-semibold" style="color: var(--sempa-text);">{fmtDate(run.started_at)}</span>
              <span style="color: var(--sempa-text-dim);"> · {run.trigger}</span>
            </span>
            <span class="font-mono text-[12px]" style="color: {run.status === 'error' ? 'var(--set-danger)' : 'var(--sempa-text-soft)'};">
              {run.status === 'error' ? (run.error ?? 'error') : fmtBytes(run.size_bytes)}
            </span>
          </div>
        {/each}
      </section>
    {/if}

    <!-- Sticky save bar — feedback shows right next to the button -->
    <div class="sticky bottom-3 mt-6 ml-auto flex w-fit max-w-full items-center gap-3 rounded-full p-1.5 {notice || error ? 'pl-5' : ''}"
         style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-2);">
      {#if notice}
        <span class="set-ok min-w-0 truncate">{notice}</span>
      {:else if error}
        <span class="set-error min-w-0 truncate">{error}</span>
      {/if}
      <button onclick={save} disabled={saving}
              class="btn-primary set-btn h-10 px-5 disabled:opacity-50">
        {saving ? 'Saving…' : 'Save settings'}
      </button>
    </div>
  {/if}
</div>
