<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { ICalSubscription } from '$lib/types';
  import { calendars, calFg, BRAND_CAL_LABEL } from '$lib/stores/calendars.svelte';

  // Connected calendar accounts
  let google   = $state<{ connected: boolean; email?: string; last_synced_at?: string }>({ connected: false });
  let fastmail = $state<{ connected: boolean; enabled?: boolean; last_synced_at?: string | null }>({ connected: false });
  let fmEmail  = $state<string | undefined>();

  // Subscribed calendars (ICS feeds) — the per-calendar entities the schedule
  // colours and show/hide settings key off of (matching ev.subscription_id).
  let subs        = $state<ICalSubscription[]>([]);
  let subsLoading = $state(true);
  let subsError   = $state('');
  // Connected-accounts section loads independently so a slow/unavailable
  // Google/Fastmail probe never blocks the feed list or the Add-feed button.
  let accountsLoading = $state(true);

  async function loadSubs() {
    subsLoading = true; subsError = '';
    try {
      subs = await api.ical.listSubscriptions() ?? [];
      // Register feeds so each is assigned its own distinct brand colour.
      calendars.register(subs.map((s) => s.id));
    } catch (e) {
      subsError = (e as Error).message || 'Could not load feeds.';
    } finally {
      subsLoading = false;
    }
  }

  onMount(() => {
    // Feeds first + on their own promise — this is what Add feed needs, and it
    // must never wait on the account probes below.
    void loadSubs();

    // Accounts in the background; each settles independently.
    void (async () => {
      const [g, f, fm] = await Promise.allSettled([
        api.integrations.calendar.get(),
        api.integrations.fastmail.calendar.get(),
        api.integrations.fastmail.get(),
      ]);
      if (g.status === 'fulfilled')  google   = g.value;
      if (f.status === 'fulfilled')  fastmail = f.value;
      if (fm.status === 'fulfilled') fmEmail  = (fm.value as { email?: string }).email;
      accountsLoading = false;
    })();
  });

  function fmtTime(s?: string | null) {
    if (!s) return 'Never';
    return new Date(s).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }

  type Account = { id: string; name: string; email?: string; synced?: string | null };
  const accounts = $derived.by(() => {
    const out: Account[] = [];
    if (google.connected)   out.push({ id: 'google',   name: 'Google Calendar',   email: google.email,  synced: google.last_synced_at });
    if (fastmail.connected) out.push({ id: 'fastmail', name: 'Fastmail Calendar', email: fmEmail,        synced: fastmail.last_synced_at });
    return out;
  });

  // ── Add feed modal ──────────────────────────────────────────────────────────
  let showAddFeed = $state(false);
  let feedUrl     = $state('');
  let feedName    = $state('');
  let feedError   = $state('');
  let feedSaving  = $state(false);

  function openAddFeed() {
    feedUrl = ''; feedName = ''; feedError = ''; showAddFeed = true;
  }

  // Accept http/https/webcal only; webcal is just https for an ICS feed, so the
  // backend (which fetches http(s)) gets a normalised URL.
  function validFeedUrl(raw: string): boolean {
    return /^(https?|webcal):\/\//i.test(raw.trim());
  }

  async function saveFeed() {
    const raw = feedUrl.trim();
    if (!validFeedUrl(raw)) {
      feedError = 'Enter a URL starting with http, https, or webcal.';
      return;
    }
    feedSaving = true; feedError = '';
    try {
      const normalizedUrl = raw.replace(/^webcal:\/\//i, 'https://');
      const sub = await api.ical.createSubscription({
        name: feedName.trim() || new URL(normalizedUrl).hostname,
        url:  normalizedUrl,
      });
      subs = [...subs, sub];
      showAddFeed = false;
    } catch (e) {
      feedError = (e as Error).message || 'Failed to add feed.';
    } finally {
      feedSaving = false;
    }
  }
</script>

<div class="mx-auto flex h-full max-w-[640px] flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <!-- Header -->
  <div class="px-5 pt-6 pb-2 sm:px-8 sm:pt-8">
    <a href="/settings/accounts?section=integrations" class="set-back">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round" d="m15 18-6-6 6-6"/>
      </svg>
      Settings
    </a>
    <h1 class="set-title type-page">Calendars</h1>
  </div>

  <div class="flex-1 overflow-y-auto px-5 pt-4 pb-16 sm:px-8">

      <!-- ── Connected accounts ─────────────────────────────────────────── -->
      <h2 class="set-heading">Accounts</h2>

      {#if accountsLoading}
        <div class="set-group set-group-pad text-sm" style="color: var(--sempa-text-dim);">
          Loading accounts…
        </div>
      {:else if accounts.length === 0}
        <div class="set-group set-group-pad text-sm" style="color: var(--sempa-text-soft);">
          No calendar accounts connected yet.
          <a href="/settings/accounts" class="font-semibold" style="color: var(--sempa-accent-ink);">Connect one →</a>
        </div>
      {:else}
        <section class="set-group">
          {#each accounts as acct (acct.id)}
            <div class="set-row">
              <!-- Provider icon tile -->
              <span class="set-icon">
                <svg class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
                  <rect x="3" y="4" width="18" height="18" rx="2"/><path stroke-linecap="round" d="M16 2v4M8 2v4M3 10h18"/>
                </svg>
              </span>
              <div class="set-row-main">
                <p class="set-label truncate">{acct.name}</p>
                {#if acct.email}
                  <p class="set-desc truncate">{acct.email}</p>
                {/if}
              </div>
              <span class="set-status">Connected</span>
            </div>
          {/each}
        </section>
      {/if}

      <!-- ── Subscribed calendars ───────────────────────────────────────── -->
      <div class="set-heading-row">
        <h2 class="set-heading">Calendars</h2>
        <button onclick={openAddFeed}
                class="btn-soft set-btn-sm">
          + Add feed
        </button>
      </div>

      {#if subsLoading}
        <div class="set-group set-group-pad text-sm" style="color: var(--sempa-text-dim);">
          Loading feeds…
        </div>
      {:else if subsError}
        <div class="set-group set-group-pad text-sm" style="color: var(--sempa-text-soft);">
          <span class="set-error">{subsError}</span>
          <button onclick={loadSubs} class="ml-1 font-semibold underline" style="color: var(--sempa-accent-ink);">Retry</button>
        </div>
      {:else if subs.length === 0}
        <div class="set-group set-group-pad text-sm" style="color: var(--sempa-text-soft);">
          No subscribed calendars yet.
          <button onclick={openAddFeed} class="font-semibold" style="color: var(--sempa-accent-ink);">Add an iCal or webcal feed →</button>
        </div>
      {:else}
        <section class="set-group">
          {#each subs as sub (sub.id)}
            {@const on = !calendars.isHidden(sub.id)}
            {@const colorKey = calendars.colorKey(sub.id)}
            <div class="set-row set-row-compact">
              <!-- Colour swatch — tap to cycle the four brand colours -->
              <button onclick={() => calendars.cycleColor(sub.id)}
                      class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl transition-transform active:scale-90"
                      style="background: var(--cal-{colorKey}-bg); opacity: {on ? 1 : 0.55};"
                      title="Colour: {BRAND_CAL_LABEL[colorKey]} — tap to change"
                      aria-label="Change calendar colour">
                <span class="h-3.5 w-3.5 rounded-full" style="background: {calFg(colorKey)};"></span>
              </button>
              <div class="set-row-main transition-opacity" style="opacity: {on ? 1 : 0.55};">
                <p class="set-label truncate">{sub.name}</p>
                <p class="set-desc">{BRAND_CAL_LABEL[colorKey]}</p>
              </div>
              <!-- On/off toggle -->
              <button onclick={() => calendars.toggleHidden(sub.id)}
                      class="set-switch" data-on={on}
                      aria-label={on ? 'Hide calendar' : 'Show calendar'} aria-pressed={on}></button>
            </div>
          {/each}
        </section>
        <p class="set-meta mt-2.5 px-1.5">
          Each calendar gets its own colour automatically. Tap a swatch to change it
          (Terracotta · Stone · Sage · Amber).
        </p>
      {/if}

      <!-- ── Display preferences ────────────────────────────────────────── -->
      <h2 class="set-heading">Display preferences</h2>

      <section class="set-group">
        {#each [
          { key: 'showDeclined',   label: 'Show declined events',          desc: 'Include events you’ve declined.' },
          { key: 'showAllDayWeek', label: 'Show all-day in week header',   desc: 'Surface all-day events above the week grid.' },
          { key: 'dimPastEvents',  label: 'Dim past events',               desc: 'Fade events earlier than now on today.' },
        ] as row (row.key)}
          {@const on = calendars.display[row.key as keyof typeof calendars.display]}
          <div class="set-row">
            <div class="set-row-main">
              <p class="set-label">{row.label}</p>
              <p class="set-desc">{row.desc}</p>
            </div>
            <button onclick={() => calendars.setDisplay(row.key as 'showDeclined' | 'showAllDayWeek' | 'dimPastEvents', !on)}
                    class="set-switch" data-on={on}
                    aria-label={row.label} aria-pressed={on}></button>
          </div>
        {/each}
      </section>

  </div>
</div>

<!-- ── Add feed modal ─────────────────────────────────────────────────────── -->
{#if showAddFeed}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[80] flex items-center justify-center bg-black/30 backdrop-blur-sm animate-fade-in"
       onclick={() => (showAddFeed = false)}>
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="surface-raised mx-4 w-full max-w-sm p-6 animate-scale-in"
         onclick={(e) => e.stopPropagation()}>
      <h3 class="type-section mb-1" style="color: var(--sempa-text);">Add calendar feed</h3>
      <p class="mb-5 text-[13px]" style="color: var(--sempa-text-soft);">Subscribe to a read-only ICS or webcal URL.</p>

      <label class="set-field-label" for="feed-url">
        ICS / Webcal URL <span style="color: var(--set-danger);">*</span>
      </label>
      <!-- svelte-ignore a11y_autofocus -->
      <input id="feed-url" type="url" inputmode="url" bind:value={feedUrl} autofocus
             autocomplete="off" autocapitalize="none" spellcheck="false"
             placeholder="https://example.com/calendar.ics  or  webcal://…"
             onkeydown={(e) => { if (e.key === 'Enter') saveFeed(); }}
             class="set-input mb-4 w-full" />

      <label class="set-field-label" for="feed-name">Name (optional)</label>
      <input id="feed-name" type="text" bind:value={feedName}
             placeholder="Work calendar"
             onkeydown={(e) => { if (e.key === 'Enter') saveFeed(); }}
             class="set-input w-full" />

      {#if feedError}<p class="set-error mt-3">{feedError}</p>{/if}

      <div class="mt-6 flex items-center justify-end gap-2">
        <button onclick={() => (showAddFeed = false)}
                class="set-btn-quiet set-btn">
          Cancel
        </button>
        <button onclick={saveFeed} disabled={feedSaving || !feedUrl.trim()}
                class="btn-primary set-btn">
          {feedSaving ? 'Saving…' : 'Save'}
        </button>
      </div>
    </div>
  </div>
{/if}
