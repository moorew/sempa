<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { routines as routinesStore } from '$lib/stores/routines.svelte';
  import { notificationSettings } from '$lib/stores/notificationSettings.svelte';
  import { syncLocalReminders, sendTestReminder } from '$lib/localReminders';
  import { isCapacitor } from '$lib/platform';
  import {
    isWebPushSupported, enableWebPush, disableWebPush, isWebPushSubscribed, notificationPermission,
  } from '$lib/webpush';
  import type { NotificationSettings } from '$lib/types';
  import { NOTIFICATION_SOUNDS, playSound, DEFAULT_SOUND_ID } from '$lib/sounds';
  import { deviceTimeZone, zoneLabel } from '$lib/timezone';
  import { Bell, Send, Play, Plane } from 'lucide-svelte';

  // A curated set of common IANA zones; the current home + this device are always
  // merged in so the picker can represent whatever is actually configured.
  const COMMON_ZONES = [
    'UTC',
    'America/Vancouver', 'America/Los_Angeles', 'America/Denver', 'America/Chicago',
    'America/Toronto', 'America/New_York', 'America/Halifax', 'America/St_Johns',
    'America/Sao_Paulo', 'Europe/London', 'Europe/Dublin', 'Europe/Paris',
    'Europe/Berlin', 'Europe/Madrid', 'Europe/Athens', 'Africa/Johannesburg',
    'Asia/Dubai', 'Asia/Kolkata', 'Asia/Bangkok', 'Asia/Singapore',
    'Asia/Hong_Kong', 'Asia/Shanghai', 'Asia/Tokyo', 'Australia/Sydney',
    'Pacific/Auckland',
  ];
  const deviceZone = deviceTimeZone();
  const zoneOptions = $derived.by(() => {
    const set = new Set(COMMON_ZONES);
    if (settings?.timezone) set.add(settings.timezone);
    set.add(deviceZone);
    return [...set].sort();
  });

  let settings = $state<NotificationSettings | null>(null);
  let loading = $state(true);
  let saving = $state(false);
  let savedFlash = $state(false);

  // Web Push UI state
  let webPushSupported = $state(false);
  let webPushSubscribed = $state(false);
  let webPushBusy = $state(false);
  let webPushError = $state<string | null>(null);

  // Webhook test state
  let testState = $state<'idle' | 'sending' | 'ok' | 'error'>('idle');
  let testError = $state<string | null>(null);

  // On-device (Android) test-notification state
  const onAndroid = isCapacitor();
  let deviceTestBusy = $state(false);
  let deviceTestMsg = $state<{ ok: boolean; text: string } | null>(null);

  async function sendDeviceTest() {
    deviceTestBusy = true;
    deviceTestMsg = null;
    try {
      const res = await sendTestReminder();
      deviceTestMsg = { ok: res.ok, text: res.message };
    } finally {
      deviceTestBusy = false;
      setTimeout(() => (deviceTestMsg = null), 6000);
    }
  }

  const DAYS = [
    { v: 1, label: 'Mon' }, { v: 2, label: 'Tue' }, { v: 3, label: 'Wed' },
    { v: 4, label: 'Thu' }, { v: 5, label: 'Fri' }, { v: 6, label: 'Sat' }, { v: 7, label: 'Sun' },
  ];

  onMount(async () => {
    webPushSupported = isWebPushSupported();
    // Local-first: show cached settings immediately (works offline, never hangs).
    // init() reconciles with the server in the background for next time.
    await notificationSettings.init();
    // The picker edits the server's home timezone, so make sure we're working
    // from the authoritative value (the GET resolves it to a concrete zone)
    // rather than an older cache that predates the field.
    await notificationSettings.refreshFromServer().catch(() => {});
    settings = structuredClone($state.snapshot(notificationSettings.settings));
    if (!settings.timezone) settings.timezone = deviceZone;
    loading = false;
    isWebPushSubscribed().then((v) => (webPushSubscribed = v)).catch(() => {});
  });

  async function save() {
    if (!settings) return;
    saving = true;
    try {
      await notificationSettings.save(settings);
      settings = structuredClone($state.snapshot(notificationSettings.settings));
      savedFlash = true;
      setTimeout(() => (savedFlash = false), 1800);
      // Re-arm the in-app routine scheduler + reschedule on-device alarms.
      void routinesStore.refresh();
      void syncLocalReminders();
    } catch (e) {
      console.warn('save notification settings failed', e);
    } finally {
      saving = false;
    }
  }

  async function toggleWebPush() {
    if (!settings) return;
    webPushBusy = true;
    webPushError = null;
    try {
      if (webPushSubscribed) {
        await disableWebPush();
        webPushSubscribed = false;
        settings.webpush_enabled = false;
      } else {
        const res = await enableWebPush();
        if (!res.ok) {
          webPushError =
            res.error === 'denied' ? 'Permission denied in the browser.'
            : res.error === 'unsupported' ? 'Not supported on this device.'
            : 'Could not enable web push.';
        } else {
          webPushSubscribed = true;
          settings.webpush_enabled = true;
        }
      }
      await save();
    } finally {
      webPushBusy = false;
    }
  }

  async function sendTest() {
    if (!settings) return;
    testState = 'sending';
    testError = null;
    try {
      await api.notifications.testWebhook(settings.webhook);
      testState = 'ok';
      setTimeout(() => (testState = 'idle'), 2500);
    } catch (e) {
      testState = 'error';
      testError = e instanceof Error ? e.message.replace(/^\d+\s/, '') : 'Test failed';
    }
  }

  function toggleWorkday(d: number) {
    if (!settings) return;
    const set = new Set(settings.routines.workdays);
    if (set.has(d)) set.delete(d); else set.add(d);
    settings.routines.workdays = [...set].sort((a, b) => a - b);
  }
</script>

<div class="mx-auto flex h-full max-w-[640px] flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <!-- Header -->
  <div class="px-5 pt-6 pb-2 sm:px-8 sm:pt-8">
    <a href="/settings/accounts" class="set-back">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round" d="m15 18-6-6 6-6"/>
      </svg>
      Settings
    </a>
    <h1 class="set-title type-page">Notifications</h1>
  </div>

  <div class="flex-1 overflow-y-auto px-5 pt-4 pb-20 sm:px-8">
    {#if loading || !settings}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else}

      {#snippet sectionLabel(text: string)}
        <h2 class="set-heading">{text}</h2>
      {/snippet}

      {#snippet toggleRow(label: string, desc: string, value: boolean, onChange: (v: boolean) => void, disabled: boolean)}
        <div class="set-row" style:opacity={disabled ? 0.5 : 1}>
          <div class="set-row-main">
            <p class="set-label">{label}</p>
            <p class="set-desc">{desc}</p>
          </div>
          <button role="switch" aria-checked={value} aria-label={label} {disabled}
                  onclick={() => onChange(!value)}
                  class="set-switch" data-on={value}></button>
        </div>
      {/snippet}

      <!-- ── Master ──────────────────────────────────────────────────────── -->
      {@render sectionLabel('General')}
      <section class="set-group">
        {@render toggleRow('All notifications', 'Master switch — pause or enable everything.',
          settings.master_enabled, (v) => { settings!.master_enabled = v; void save(); }, false)}
        {@render toggleRow('Custom alert sound', 'Play a calm tone with reminders.',
          settings.sound_enabled, (v) => { settings!.sound_enabled = v; void save(); }, !settings.master_enabled)}
      </section>

      <!-- ── Timezone ────────────────────────────────────────────────────── -->
      {@render sectionLabel('Timezone')}
      <section class="set-group">
        <div class="set-block">
          <p class="set-label">Home timezone</p>
          <p class="set-desc">
            Sempa anchors recurring tasks and the morning digest to this zone. Task
            times stay put wherever you travel — only the day boundary follows your
            device.
          </p>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <span class="set-select">
              <select bind:value={settings.timezone} aria-label="Home timezone"
                      class="set-input min-w-[220px]">
                {#each zoneOptions as z}
                  <option value={z}>{z.replace(/_/g, ' ')}</option>
                {/each}
              </select>
            </span>
            {#if deviceZone !== settings.timezone}
              <button onclick={() => { settings!.timezone = deviceZone; }}
                      class="btn-soft set-btn">
                <Plane size={14} /> Use {zoneLabel(deviceZone)}
              </button>
            {/if}
          </div>
          <p class="set-meta mt-2.5 px-0.5">
            This device: {deviceZone}{deviceZone !== settings.timezone ? ' — currently away from home' : ''}
          </p>
        </div>
      </section>

      <!-- ── Sound picker ────────────────────────────────────────────────── -->
      {#if settings.sound_enabled && settings.master_enabled}
        {@const selected = settings.sound_id || DEFAULT_SOUND_ID}
        {@render sectionLabel('Alert sound')}
        <section class="set-group">
          {#each NOTIFICATION_SOUNDS as snd (snd.id)}
            {@const active = selected === snd.id}
            <div class="set-row set-row-compact" style="padding-left: 12px;">
              <!-- Preview -->
              <button aria-label={'Preview ' + snd.label} onclick={() => playSound(snd.id)}
                      class="btn-soft h-9 w-9 shrink-0" style="padding: 0;">
                <Play size={14} />
              </button>
              <!-- Select (radio row) -->
              <button onclick={() => { settings!.sound_id = snd.id; playSound(snd.id); void save(); }}
                      class="flex min-h-9 min-w-0 flex-1 items-center justify-between text-left">
                <span class="set-label truncate">{snd.label}</span>
                <span class="set-radio ml-3" data-on={active}></span>
              </button>
            </div>
          {/each}
        </section>
      {/if}

      <!-- ── Delivery channels ───────────────────────────────────────────── -->
      {@render sectionLabel('Delivery channels')}
      <section class="set-group">
        <!-- Web Push -->
        <div class="set-row">
          <div class="set-row-main">
            <p class="set-label">Web Push</p>
            <p class="set-desc">
              {#if !webPushSupported}
                Open this app in a browser/PWA to enable.
              {:else if webPushSubscribed}
                Enabled on this device.
              {:else}
                Native notifications on Windows & Android.
              {/if}
            </p>
            {#if webPushError}
              <p class="set-error mt-1">{webPushError}</p>
            {/if}
          </div>
          <button disabled={!webPushSupported || webPushBusy || !settings.master_enabled}
                  onclick={toggleWebPush}
                  class="{webPushSubscribed ? 'set-btn-quiet' : 'btn-primary'} set-btn disabled:opacity-40">
            {webPushBusy ? '…' : webPushSubscribed ? 'Disable' : 'Enable'}
          </button>
        </div>
        {@render toggleRow('Native (Android FCM)', 'Push to the installed Android app.',
          settings.fcm_enabled, (v) => { settings!.fcm_enabled = v; void save(); }, !settings.master_enabled)}
        {@render toggleRow('Custom webhook (ntfy / Gotify)', 'POST notifications to a self-hosted service.',
          settings.webhook_enabled, (v) => { settings!.webhook_enabled = v; void save(); }, !settings.master_enabled)}
      </section>

      <!-- ── On-device test (Android) ────────────────────────────────────── -->
      {#if onAndroid}
        <section class="set-group mt-3">
          <div class="set-block">
            <p class="set-label">Test this device</p>
            <p class="set-desc">
              Fires a real notification in a few seconds so you can confirm sound and pop-up work on this phone.
            </p>
            <div class="mt-3 flex flex-wrap items-center gap-3">
              <button onclick={sendDeviceTest} disabled={deviceTestBusy}
                      class="btn-primary set-btn">
                <Bell size={14} />
                {deviceTestBusy ? 'Sending…' : 'Send test notification'}
              </button>
              {#if deviceTestMsg}
                <span class={deviceTestMsg.ok ? 'set-ok' : 'set-error'}>
                  {deviceTestMsg.text}
                </span>
              {/if}
            </div>
          </div>
        </section>
      {/if}

      <!-- ── Webhook config ──────────────────────────────────────────────── -->
      {#if settings.webhook_enabled}
        <section class="set-group mt-3">
          <div class="set-block">
            <label class="set-field-label" for="wh-endpoint">Endpoint URL</label>
            <input id="wh-endpoint" type="url" placeholder="https://ntfy.sh/ or https://gotify/message?token=…"
                   bind:value={settings.webhook.endpoint}
                   class="set-input mb-3 w-full" />

            <div class="mb-3 grid grid-cols-2 gap-3">
              <div>
                <label class="set-field-label" for="wh-topic">Topic (ntfy)</label>
                <input id="wh-topic" type="text" placeholder="my-topic" bind:value={settings.webhook.topic}
                       class="set-input w-full" />
              </div>
              <div>
                <label class="set-field-label" for="wh-method">Method</label>
                <input id="wh-method" type="text" placeholder="POST" bind:value={settings.webhook.method}
                       class="set-input w-full" />
              </div>
            </div>

            <div class="mb-4 grid grid-cols-2 gap-3">
              <div>
                <label class="set-field-label" for="wh-ah">Auth header</label>
                <input id="wh-ah" type="text" placeholder="Authorization" bind:value={settings.webhook.auth_header}
                       class="set-input w-full" />
              </div>
              <div>
                <label class="set-field-label" for="wh-av">Token / value</label>
                <input id="wh-av" type="text" placeholder="Bearer tk_…" bind:value={settings.webhook.auth_value}
                       class="set-input w-full" />
              </div>
            </div>

            <div class="flex flex-wrap items-center gap-3">
              <button onclick={async () => { await save(); await sendTest(); }}
                      disabled={testState === 'sending' || !settings.webhook.endpoint}
                      class="btn-primary set-btn">
                <Send size={14} />
                {testState === 'sending' ? 'Sending…' : 'Send test notification'}
              </button>
              {#if testState === 'ok'}
                <span class="set-ok">Sent ✓</span>
              {:else if testState === 'error'}
                <span class="set-error truncate">{testError}</span>
              {/if}
            </div>
          </div>
        </section>
      {/if}

      <!-- ── Routines ────────────────────────────────────────────────────── -->
      {@render sectionLabel('In-app routines')}
      <section class="set-group">
        <!-- Weekly planning -->
        <div class="set-row set-row-wrap">
          <div class="set-row-main min-w-[180px]">
            <p class="set-label">Weekly planning prompt</p>
            <p class="set-desc">When to nudge you to plan the week.</p>
          </div>
          <div class="flex items-center gap-2">
            <label class="sr-only" for="wp-day">Day</label>
            <span class="set-select">
              <select id="wp-day" bind:value={settings.routines.weekly_plan_day}
                      class="set-input w-[96px]">
                {#each DAYS as d}
                  <option value={d.v}>{d.label}</option>
                {/each}
              </select>
            </span>
            <label class="sr-only" for="wp-time">Time</label>
            <input id="wp-time" type="time" bind:value={settings.routines.weekly_plan_time}
                   class="set-input set-input-mono w-[124px]" />
          </div>
        </div>

        <!-- Daily shutdown -->
        <div class="set-row set-row-wrap">
          <div class="set-row-main min-w-[180px]">
            <p class="set-label">Daily shutdown review</p>
            <p class="set-desc">When to start wrapping up the day.</p>
          </div>
          <label class="sr-only" for="sd-time">Time</label>
          <input id="sd-time" type="time" bind:value={settings.routines.daily_shutdown_time}
                 class="set-input set-input-mono w-[124px]" />
        </div>

        <!-- Workdays -->
        <div class="set-block">
          <p class="set-label">Workdays</p>
          <p class="set-desc">Routines only fire on these days.</p>
          <div class="mt-3 flex flex-wrap gap-1.5">
            {#each DAYS as d}
              {@const on = settings.routines.workdays.includes(d.v)}
              <button onclick={() => toggleWorkday(d.v)}
                      aria-pressed={on}
                      class="h-9 min-w-[52px] rounded-full px-3 text-[13px] font-bold transition-colors"
                      style="background: {on ? 'var(--sempa-btn-bg)' : 'var(--sempa-fill)'};
                             color: {on ? 'var(--sempa-btn-fg)' : 'var(--sempa-text-soft)'};">
                {d.label}
              </button>
            {/each}
          </div>
        </div>
      </section>

      <!-- Save -->
      <div class="mt-6 flex items-center gap-3">
        <button onclick={save} disabled={saving}
                class="btn-primary h-11 px-5 text-sm disabled:opacity-50">
          <Bell size={15} />
          {saving ? 'Saving…' : 'Save settings'}
        </button>
        {#if savedFlash}
          <span class="set-ok">Saved ✓</span>
        {/if}
      </div>

    {/if}
  </div>
</div>
