<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { GmailIntegrationConfig, JiraIntegrationConfig } from '$lib/types';

  let jira = $state<JiraIntegrationConfig | null>(null);
  let gmail = $state<GmailIntegrationConfig | null>(null);

  onMount(async () => {
    [jira, gmail] = await Promise.all([
      api.integrations.jira.get(),
      api.integrations.gmail.get(),
    ]);
  });
</script>

<div class="mx-auto max-w-2xl px-6 py-8">
  <h1 class="mb-1 text-xl font-semibold text-gray-900 dark:text-gray-50">Settings</h1>
  <p class="mb-8 text-sm text-gray-500 dark:text-gray-400">Manage integrations, tags, and recurring tasks.</p>

  <!-- Section: Tasks -->
  <p class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">Tasks</p>
  <div class="mb-6 flex flex-col gap-2">
    <a href="/settings/tags"
       class="flex items-center gap-4 rounded-xl border border-gray-200 bg-white px-5 py-4 transition-colors hover:border-gray-300 hover:shadow-sm dark:border-gray-700 dark:bg-gray-800 dark:hover:border-gray-600">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-violet-50 dark:bg-violet-950">
        <svg class="h-5 w-5 text-violet-500" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 0 1 0 2.828l-7 7a2 2 0 0 1-2.828 0l-7-7A2 2 0 0 1 3 12V7a4 4 0 0 1 4-4z"/>
        </svg>
      </div>
      <div class="flex-1">
        <p class="font-medium text-gray-900 dark:text-gray-50">Tags</p>
        <p class="text-sm text-gray-400 dark:text-gray-500">Colour-coded labels for your tasks</p>
      </div>
      <svg class="h-4 w-4 text-gray-400" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
      </svg>
    </a>

    <a href="/settings/recurring"
       class="flex items-center gap-4 rounded-xl border border-gray-200 bg-white px-5 py-4 transition-colors hover:border-gray-300 hover:shadow-sm dark:border-gray-700 dark:bg-gray-800 dark:hover:border-gray-600">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-amber-50 dark:bg-amber-950">
        <span class="text-xl text-amber-500">↺</span>
      </div>
      <div class="flex-1">
        <p class="font-medium text-gray-900 dark:text-gray-50">Recurring Tasks</p>
        <p class="text-sm text-gray-400 dark:text-gray-500">Daily, weekly, and monthly templates</p>
      </div>
      <svg class="h-4 w-4 text-gray-400" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
      </svg>
    </a>
  </div>

  <!-- Section: Integrations -->
  <p class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">Integrations</p>
  <div class="flex flex-col gap-2">
    <a href="/settings/integrations/gmail"
       class="flex items-center gap-4 rounded-xl border border-gray-200 bg-white px-5 py-4 transition-colors hover:border-gray-300 hover:shadow-sm dark:border-gray-700 dark:bg-gray-800 dark:hover:border-gray-600">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-red-50 dark:bg-red-950">
        <svg class="h-5 w-5 text-red-500" viewBox="0 0 24 24" fill="currentColor">
          <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <p class="font-medium text-gray-900 dark:text-gray-50">Gmail</p>
        {#if gmail?.connected}
          <p class="text-sm text-gray-500 truncate dark:text-gray-400">{gmail.email ?? 'Connected'}</p>
        {:else}
          <p class="text-sm text-gray-400 dark:text-gray-500">Not connected</p>
        {/if}
      </div>
      <div class="flex items-center gap-2">
        {#if gmail?.connected}
          <span class="inline-flex items-center gap-1 rounded-full bg-green-50 px-2.5 py-0.5 text-xs font-medium text-green-700 dark:bg-green-950 dark:text-green-400">
            <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>Connected
          </span>
        {/if}
        <svg class="h-4 w-4 text-gray-400" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
        </svg>
      </div>
    </a>

    <a href="/settings/integrations/jira"
       class="flex items-center gap-4 rounded-xl border border-gray-200 bg-white px-5 py-4 transition-colors hover:border-gray-300 hover:shadow-sm dark:border-gray-700 dark:bg-gray-800 dark:hover:border-gray-600">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50 dark:bg-blue-950">
        <svg class="h-5 w-5 text-blue-500" viewBox="0 0 24 24" fill="currentColor">
          <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
        </svg>
      </div>
      <div class="flex-1 min-w-0">
        <p class="font-medium text-gray-900 dark:text-gray-50">Jira</p>
        {#if jira?.connected}
          <p class="text-sm text-gray-500 truncate dark:text-gray-400">{jira.config?.host ?? 'Connected'}</p>
        {:else}
          <p class="text-sm text-gray-400 dark:text-gray-500">Not connected</p>
        {/if}
      </div>
      <div class="flex items-center gap-2">
        {#if jira?.connected}
          <span class="inline-flex items-center gap-1 rounded-full bg-green-50 px-2.5 py-0.5 text-xs font-medium text-green-700 dark:bg-green-950 dark:text-green-400">
            <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>Connected
          </span>
        {/if}
        <svg class="h-4 w-4 text-gray-400" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
        </svg>
      </div>
    </a>
  </div>
</div>
