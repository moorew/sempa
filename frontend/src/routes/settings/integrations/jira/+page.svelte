<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { jiraStatus } from '$lib/stores/jiraStatus.svelte';

  let host      = $state('');
  let email     = $state('');
  let apiToken  = $state('');
  let jql       = $state('');
  let connected = $state(false);
  let lastSynced = $state<string | null>(null);

  let saving   = $state(false);
  let testing  = $state(false);
  let syncing  = $state(false);

  let testStatus = $state<'idle' | 'ok' | 'error'>('idle');
  let testMsg    = $state('');
  let syncResult = $state<{ total: number; new: number; updated: number; errors: number } | null>(null);
  let saveError  = $state('');

  onMount(async () => {
    const cfg = await api.integrations.jira.get();
    connected  = cfg.connected;
    lastSynced = cfg.last_synced_at ?? null;
    if (cfg.config) {
      host     = cfg.config.host;
      email    = cfg.config.email;
      apiToken = cfg.config.api_token ?? '';
      jql      = cfg.config.jql ?? '';
    }
  });

  async function save() {
    saving = true;
    saveError = '';
    try {
      await api.integrations.jira.save({ host, email, api_token: apiToken, jql: jql || undefined });
      connected = true;
      // Reveal the Jira tab / command / chip app-wide without a reload.
      await jiraStatus.refresh();
    } catch (e) {
      saveError = (e as Error).message;
    } finally {
      saving = false;
    }
  }

  async function test() {
    testing = true;
    testStatus = 'idle';
    testMsg = '';
    try {
      await api.integrations.jira.test();
      testStatus = 'ok';
      testMsg = 'Connection successful';
    } catch (e) {
      testStatus = 'error';
      testMsg = (e as Error).message;
    } finally {
      testing = false;
    }
  }

  async function sync() {
    syncing = true;
    syncResult = null;
    try {
      syncResult = await api.integrations.jira.sync();
    } catch (e) {
      testStatus = 'error';
      testMsg = (e as Error).message;
    } finally {
      syncing = false;
    }
  }

  async function disconnect() {
    if (!confirm('Disconnect Jira? Imported tasks will be kept.')) return;
    await api.integrations.jira.delete();
    connected = false;
    await jiraStatus.refresh();
    host = email = apiToken = jql = '';
  }
</script>

<div class="mx-auto max-w-[640px] px-5 pt-6 pb-16 sm:px-8 sm:pt-8">
  <!-- Back -->
  <a href="/settings/accounts" class="set-back">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" d="m15 18-6-6 6-6"/>
    </svg>
    Settings
  </a>

  <div class="mt-5 mb-6 flex items-center gap-3.5">
    <span class="set-icon" style="width: 48px; height: 48px; border-radius: 16px;">
      <svg class="h-6 w-6" viewBox="0 0 24 24" fill="currentColor">
        <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
      </svg>
    </span>
    <div>
      <h1 class="type-page" style="color: var(--sempa-text);">Jira</h1>
      {#if connected}
        <span class="set-status mt-1">Connected</span>
      {:else}
        <p class="set-desc">Not connected</p>
      {/if}
    </div>
  </div>

  <form onsubmit={(e) => { e.preventDefault(); save(); }} class="set-group">
    <div class="set-block flex flex-col gap-4" style="padding-top: 18px;">
      <div>
        <label class="set-field-label" for="host">Jira host</label>
        <input id="host" type="url" bind:value={host} placeholder="https://yourcompany.atlassian.net"
               class="set-input w-full" />
      </div>

      <div>
        <label class="set-field-label" for="email">Email</label>
        <input id="email" type="email" bind:value={email} placeholder="you@company.com"
               class="set-input w-full" />
      </div>

      <div>
        <label class="set-field-label" for="token">API token</label>
        <input id="token" type="password" bind:value={apiToken} placeholder="Paste your Atlassian API token"
               class="set-input w-full" />
        <p class="set-meta mt-1.5 px-0.5">
          Generate at <span class="font-mono text-[12px]">id.atlassian.com/manage-profile/security/api-tokens</span>
        </p>
      </div>

      <div>
        <label class="set-field-label" for="jql">
          JQL filter <span class="font-medium" style="color: var(--sempa-text-dim);">(optional)</span>
        </label>
        <input id="jql" type="text" bind:value={jql}
               placeholder="assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC"
               class="set-input set-input-mono w-full" />
      </div>

      {#if saveError}
        <p class="set-error">{saveError}</p>
      {/if}

      <div class="flex flex-wrap items-center gap-2 pt-1">
        <button type="submit" disabled={saving}
                class="btn-primary set-btn">
          {saving ? 'Saving…' : 'Save'}
        </button>

        {#if connected}
          <button type="button" onclick={test} disabled={testing}
                  class="set-btn-quiet set-btn">
            {testing ? 'Testing…' : 'Test connection'}
          </button>

          <button type="button" onclick={sync} disabled={syncing}
                  class="set-btn-quiet set-btn">
            {syncing ? 'Syncing…' : 'Sync now'}
          </button>
        {/if}
      </div>
    </div>
  </form>

  {#if testStatus !== 'idle'}
    <div class="mt-4 rounded-2xl px-4 py-3 text-[13px] font-semibold"
         style="background: {testStatus === 'ok' ? 'var(--sempa-success-bg)' : 'color-mix(in srgb, var(--set-danger) 10%, var(--sempa-bg-panel))'};
                color: {testStatus === 'ok' ? 'var(--sempa-success)' : 'var(--set-danger)'};">
      {testMsg}
    </div>
  {/if}

  {#if syncResult}
    <div class="mt-4 rounded-2xl px-4 py-3 text-[13px] font-semibold" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
      Sync complete — {syncResult.new} new, {syncResult.updated} updated, {syncResult.errors} errors
      (total: {syncResult.total})
    </div>
  {/if}

  {#if lastSynced}
    <p class="set-meta mt-4 px-1.5">Last synced: {new Date(lastSynced).toLocaleString()}</p>
  {/if}

  {#if connected}
    <h2 class="set-heading">Danger zone</h2>
    <div class="set-group">
      <div class="set-row">
        <div class="set-row-main">
          <p class="set-label">Disconnect Jira</p>
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
