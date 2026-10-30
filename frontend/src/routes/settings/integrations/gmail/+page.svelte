<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { api } from '$lib/api';

  let connected  = $state(false);
  let email      = $state('');
  let labels     = $state<string[]>([]);
  let lastSynced = $state<string | null>(null);

  let syncing   = $state(false);
  let syncResult = $state<{ total: number; new: number; updated: number; errors: number } | null>(null);
  let syncError  = $state('');

  // Inline label editor state
  let labelInput = $state('');
  let savingLabels = $state(false);

  onMount(async () => {
    const cfg = await api.integrations.gmail.get();
    connected  = cfg.connected;
    email      = cfg.email ?? '';
    labels     = cfg.labels ?? ['STARRED'];
    lastSynced = cfg.last_synced_at ?? null;

    // Show success banner if returning from OAuth flow
    const connected_param = page.url.searchParams.get('connected');
    if (connected_param === '1' && !connected) {
      // reload after redirect
      const fresh = await api.integrations.gmail.get();
      connected  = fresh.connected;
      email      = fresh.email ?? '';
      labels     = fresh.labels ?? ['STARRED'];
    }
  });

  async function sync() {
    syncing = true;
    syncResult = null;
    syncError = '';
    try {
      syncResult = await api.integrations.gmail.sync();
    } catch (e) {
      syncError = (e as Error).message;
    } finally {
      syncing = false;
    }
  }

  async function saveLabels() {
    savingLabels = true;
    try {
      await api.integrations.gmail.updateLabels(labels);
    } finally {
      savingLabels = false;
    }
  }

  function addLabel() {
    const v = labelInput.trim().toUpperCase();
    if (v && !labels.includes(v)) {
      labels = [...labels, v];
      labelInput = '';
    }
  }

  function removeLabel(l: string) {
    labels = labels.filter((x) => x !== l);
  }

  const COMMON_LABELS = ['STARRED', 'INBOX', 'IMPORTANT'];

  async function disconnect() {
    if (!confirm('Disconnect Gmail? Imported tasks will be kept.')) return;
    await api.integrations.gmail.delete();
    connected = false;
    email = '';
  }
</script>

<div class="mx-auto max-w-[640px] px-5 pt-6 pb-16 sm:px-8 sm:pt-8">
  <!-- Back -->
  <a href="/settings/integrations" class="set-back">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" d="m15 18-6-6 6-6"/>
    </svg>
    Integrations
  </a>

  <div class="mt-5 mb-6 flex items-center gap-3.5">
    <span class="set-icon tint" style="--tint: #ea4335; width: 48px; height: 48px; border-radius: 16px;">
      <svg class="h-6 w-6" viewBox="0 0 24 24" fill="currentColor">
        <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
      </svg>
    </span>
    <div class="min-w-0">
      <h1 class="type-page" style="color: var(--sempa-text);">Gmail</h1>
      {#if connected}
        <p class="set-desc truncate">{email}</p>
      {:else}
        <p class="set-desc">Not connected</p>
      {/if}
    </div>
  </div>

  {#if !connected}
    <!-- Connect flow -->
    <div class="set-group px-6 py-8 text-center">
      <p class="mx-auto mb-5 max-w-sm text-[13.5px] leading-relaxed" style="color: var(--sempa-text-soft);">
        Connect your Gmail account to automatically import starred emails as tasks.
        Sempa only requests read-only access.
      </p>
      <a href={api.integrations.gmail.authUrl()}
         class="btn-primary set-btn h-10 px-5">
        <svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
          <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
        </svg>
        Connect with Google
      </a>
    </div>
  {:else}
    <!-- Label configuration -->
    <h2 class="set-heading">Labels to sync</h2>
    <div class="set-group">
      <div class="set-block">
        <p class="set-desc" style="margin-top: 0;">
          Only emails with these Gmail labels will be imported as tasks.
        </p>

        <!-- Quick-add common labels -->
        <div class="mt-3 flex flex-wrap gap-2">
          {#each COMMON_LABELS as l}
            <button type="button"
                    onclick={() => { if (!labels.includes(l)) labels = [...labels, l]; }}
                    aria-pressed={labels.includes(l)}
                    class="h-8 rounded-full px-3.5 text-[12.5px] font-bold transition-colors"
                    style={labels.includes(l)
                      ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);'
                      : 'background: var(--sempa-fill); color: var(--sempa-text-soft);'}>
              {l}
            </button>
          {/each}
        </div>

        <!-- Current labels (custom ones too) -->
        {#if labels.some((l) => !COMMON_LABELS.includes(l))}
          <div class="mt-2 flex flex-wrap gap-2">
            {#each labels as l}
              {#if !COMMON_LABELS.includes(l)}
                <span class="set-chip">
                  {l}
                  <button type="button" onclick={() => removeLabel(l)} aria-label="Remove {l}">
                    <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2.4" viewBox="0 0 24 24">
                      <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
                    </svg>
                  </button>
                </span>
              {/if}
            {/each}
          </div>
        {/if}

        <!-- Custom label input -->
        <div class="mt-3 flex gap-2">
          <input type="text" bind:value={labelInput}
                 onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), addLabel())}
                 placeholder="Custom label ID (e.g. Label_12345)"
                 aria-label="Custom label ID"
                 class="set-input min-w-0 flex-1" />
          <button type="button" onclick={addLabel}
                  class="set-btn-quiet set-btn">
            Add
          </button>
        </div>

        <div class="mt-4 flex flex-wrap items-center gap-2">
          <button type="button" onclick={saveLabels} disabled={savingLabels}
                  class="btn-primary set-btn">
            {savingLabels ? 'Saving…' : 'Save labels'}
          </button>

          <button type="button" onclick={sync} disabled={syncing}
                  class="set-btn-quiet set-btn">
            {syncing ? 'Syncing…' : 'Sync now'}
          </button>
        </div>
      </div>
    </div>

    <!-- Sync result -->
    {#if syncResult}
      <div class="mt-4 rounded-2xl px-4 py-3 text-[13px] font-semibold" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
        Sync complete — {syncResult.new} new, {syncResult.updated} updated, {syncResult.errors} errors
        (total: {syncResult.total})
      </div>
    {/if}

    {#if syncError}
      <div class="set-banner-error mt-4">{syncError}</div>
    {/if}

    {#if lastSynced}
      <p class="set-meta mt-4 px-1.5">Last synced: {new Date(lastSynced).toLocaleString()}</p>
    {/if}

    <!-- Danger zone -->
    <h2 class="set-heading">Danger zone</h2>
    <div class="set-group">
      <div class="set-row">
        <div class="set-row-main">
          <p class="set-label">Disconnect Gmail</p>
          <p class="set-desc">Imported tasks will be kept.</p>
        </div>
        <button onclick={disconnect}
                class="set-danger set-btn">
          Disconnect
        </button>
      </div>
    </div>
  {/if}
</div>
