<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';

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
    host = email = apiToken = jql = '';
  }
</script>

<div class="mx-auto max-w-xl px-6 py-8">
  <!-- Back -->
  <a href="/settings/integrations" class="mb-6 inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700">
    <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
      <path stroke-linecap="round" d="m15 18-6-6 6-6"/>
    </svg>
    Integrations
  </a>

  <div class="mb-6 flex items-center gap-3">
    <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50">
      <svg class="h-5 w-5 text-blue-500" viewBox="0 0 24 24" fill="currentColor">
        <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
      </svg>
    </div>
    <div>
      <h1 class="text-xl font-semibold text-gray-900">Jira</h1>
      {#if connected}
        <p class="text-sm text-green-600">Connected</p>
      {:else}
        <p class="text-sm text-gray-400">Not connected</p>
      {/if}
    </div>
  </div>

  <form onsubmit={(e) => { e.preventDefault(); save(); }} class="flex flex-col gap-4">
    <div>
      <label class="mb-1.5 block text-sm font-medium text-gray-700" for="host">Jira Host</label>
      <input id="host" type="url" bind:value={host} placeholder="https://yourcompany.atlassian.net"
             class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100" />
    </div>

    <div>
      <label class="mb-1.5 block text-sm font-medium text-gray-700" for="email">Email</label>
      <input id="email" type="email" bind:value={email} placeholder="you@company.com"
             class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100" />
    </div>

    <div>
      <label class="mb-1.5 block text-sm font-medium text-gray-700" for="token">API Token</label>
      <input id="token" type="password" bind:value={apiToken} placeholder="Paste your Atlassian API token"
             class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100" />
      <p class="mt-1 text-xs text-gray-400">
        Generate at <span class="font-mono">id.atlassian.com/manage-profile/security/api-tokens</span>
      </p>
    </div>

    <div>
      <label class="mb-1.5 block text-sm font-medium text-gray-700" for="jql">
        JQL Filter <span class="font-normal text-gray-400">(optional)</span>
      </label>
      <input id="jql" type="text" bind:value={jql}
             placeholder="assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC"
             class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100" />
    </div>

    {#if saveError}
      <p class="text-sm text-red-600">{saveError}</p>
    {/if}

    <div class="flex items-center gap-3 pt-1">
      <button type="submit" disabled={saving}
              class="rounded-lg bg-blue-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-600 disabled:opacity-50">
        {saving ? 'Saving…' : 'Save'}
      </button>

      {#if connected}
        <button type="button" onclick={test} disabled={testing}
                class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50">
          {testing ? 'Testing…' : 'Test connection'}
        </button>

        <button type="button" onclick={sync} disabled={syncing}
                class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50">
          {syncing ? 'Syncing…' : 'Sync now'}
        </button>
      {/if}
    </div>
  </form>

  <!-- Test / sync feedback -->
  {#if testStatus !== 'idle'}
    <div class="mt-4 rounded-lg px-4 py-3 text-sm
                {testStatus === 'ok' ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}">
      {testMsg}
    </div>
  {/if}

  {#if syncResult}
    <div class="mt-4 rounded-lg bg-blue-50 px-4 py-3 text-sm text-blue-700">
      Sync complete — {syncResult.new} new, {syncResult.updated} updated, {syncResult.errors} errors
      (total: {syncResult.total})
    </div>
  {/if}

  {#if lastSynced}
    <p class="mt-4 text-xs text-gray-400">Last synced: {new Date(lastSynced).toLocaleString()}</p>
  {/if}

  <!-- Danger zone -->
  {#if connected}
    <div class="mt-10 rounded-xl border border-red-100 bg-red-50 px-5 py-4">
      <p class="mb-3 text-sm font-medium text-red-800">Danger zone</p>
      <button onclick={disconnect}
              class="rounded-lg border border-red-300 px-3 py-1.5 text-sm text-red-700 transition-colors hover:bg-red-100">
        Disconnect Jira
      </button>
    </div>
  {/if}
</div>
