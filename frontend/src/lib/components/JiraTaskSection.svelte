<script lang="ts">
  /**
   * Live Jira controls inside the task editor, shown for source === 'jira' tasks.
   * Fetches the current issue + its available workflow transitions, shows the
   * key fields and a prominent link, and lets the user move the issue's status
   * via the project's real transitions (no fragile status mapping). The notes
   * already carry the description (copied on import), so this stays focused on
   * status/fields management.
   */
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { parseJiraMeta } from '$lib/jira/filters';
  import { openExternal } from '$lib/external';

  let { task }: { task: Task } = $props();

  type Transition = { id: string; name: string; to?: { statusCategory?: { key?: string } } };

  const meta = $derived(parseJiraMeta(task.source_metadata));
  let detail = $state<any>(null);
  let transitions = $state<Transition[]>([]);
  let loading = $state(false);
  let err = $state('');
  let working = $state(false);

  // (Re)load whenever the issue key changes.
  let loadedKey = '';
  $effect(() => {
    const key = meta?.key;
    if (!key || key === loadedKey) return;
    loadedKey = key;
    void load(key);
  });

  async function load(key: string) {
    loading = true; err = '';
    try {
      const [d, t] = await Promise.all([
        api.integrations.jira.getIssue(key),
        api.integrations.jira.getTransitions(key),
      ]);
      detail = d;
      transitions = t;
    } catch (e: any) {
      err = e?.message ?? 'Failed to load the Jira issue';
    } finally {
      loading = false;
    }
  }

  const statusName = $derived(detail?.fields?.status?.name ?? meta?.status ?? '');
  const issueType  = $derived(detail?.fields?.issuetype?.name ?? meta?.issueType ?? '');
  const priority   = $derived(detail?.fields?.priority?.name ?? meta?.priority ?? '');
  const assignee   = $derived(detail?.fields?.assignee?.displayName ?? meta?.assignee ?? '');
  const labels     = $derived<string[]>(detail?.fields?.labels ?? []);

  async function move(t: Transition) {
    if (!meta?.key || working) return;
    working = true; err = '';
    try {
      await api.integrations.jira.transition(meta.key, t.id);
      // Reflect the new status immediately, then refresh the transition list for
      // the new state (the available moves change once the status changes).
      if (detail) detail = { ...detail, fields: { ...detail.fields, status: { ...detail.fields?.status, name: t.name } } };
      transitions = await api.integrations.jira.getTransitions(meta.key).catch(() => transitions);
    } catch (e: any) {
      err = e?.message ?? 'Could not move the issue';
    } finally {
      working = false;
    }
  }
</script>

{#if meta?.key}
  <div class="overflow-hidden rounded-xl" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);">
    <!-- Header: key + open link -->
    <div class="flex items-center justify-between gap-2 px-3 py-2" style="border-bottom: 1px solid var(--sempa-border);">
      <span class="inline-flex items-center gap-1.5 text-xs font-semibold" style="color: var(--sempa-text-soft);">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor" style="color: var(--sempa-accent);">
          <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
        </svg>
        {meta.key}
      </span>
      {#if task.source_url}
        <button onclick={() => openExternal(task.source_url!)}
                class="inline-flex items-center gap-1 text-xs font-medium" style="color: var(--sempa-accent);">
          Open in Jira
          <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6M15 3h6v6M10 14L21 3"/>
          </svg>
        </button>
      {/if}
    </div>

    <div class="px-3 py-2.5 text-xs" style="color: var(--sempa-text-soft);">
      {#if loading && !detail}
        <p style="color: var(--sempa-text-dim);">Loading Jira issue…</p>
      {:else}
        <!-- Fields -->
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span><span style="color: var(--sempa-text-dim);">Status:</span> <span style="color: var(--sempa-text);">{statusName || '—'}</span></span>
          {#if issueType}<span><span style="color: var(--sempa-text-dim);">Type:</span> {issueType}</span>{/if}
          {#if priority}<span><span style="color: var(--sempa-text-dim);">Priority:</span> {priority}</span>{/if}
          {#if assignee}<span><span style="color: var(--sempa-text-dim);">Assignee:</span> {assignee}</span>{/if}
        </div>
        {#if labels.length}
          <div class="mt-1.5 flex flex-wrap gap-1">
            {#each labels as l}
              <span class="rounded-full px-2 py-0.5 text-[10.5px]" style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">{l}</span>
            {/each}
          </div>
        {/if}

        <!-- Transitions -->
        {#if transitions.length}
          <p class="mb-1.5 mt-3" style="color: var(--sempa-text-dim);">Move to</p>
          <div class="flex flex-wrap gap-1.5">
            {#each transitions as t (t.id)}
              <button onclick={() => move(t)} disabled={working}
                      class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors disabled:opacity-50"
                      style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
                {t.name}
              </button>
            {/each}
          </div>
        {/if}

        {#if err}
          <p class="mt-2 text-red-500">{err}</p>
        {/if}
      {/if}
    </div>
  </div>
{/if}
