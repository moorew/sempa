<script lang="ts">
  import { api } from '$lib/api';
  import type { FastmailEmail } from '$lib/types';

  let {
    onTaskCreated,
  }: {
    onTaskCreated?: (task: import('$lib/types').Task) => void;
  } = $props();

  type Tab = 'inbox' | 'archived';
  let tab = $state<Tab>('inbox');

  let inboxEmails    = $state<FastmailEmail[]>([]);
  let archivedEmails = $state<FastmailEmail[]>([]);
  let loading        = $state(true);
  let error          = $state('');
  let connected      = $state(true);

  let converting  = $state<Record<string, boolean>>({});
  let archiving   = $state<Record<string, boolean>>({});
  let unarchiving = $state<Record<string, boolean>>({});
  let done        = $state<Record<string, string>>({});

  $effect(() => { loadInbox(); });

  async function loadInbox() {
    loading = true; error = '';
    try {
      inboxEmails = await api.integrations.fastmail.emails();
    } catch (e: any) {
      if (e.message?.includes('not connected')) connected = false;
      else error = e.message ?? 'Failed';
    } finally { loading = false; }
  }

  async function loadArchived() {
    loading = true; error = '';
    try {
      archivedEmails = await api.integrations.fastmail.archivedEmails();
    } catch (e: any) { error = e.message ?? 'Failed'; }
    finally { loading = false; }
  }

  function switchTab(t: Tab) {
    tab = t;
    if (t === 'archived' && archivedEmails.length === 0 && !loading) {
      loadArchived();
    }
  }

  export async function toTask(email: FastmailEmail) {
    if (converting[email.id]) return;
    converting[email.id] = true;
    try {
      const task = await api.integrations.fastmail.toTask(email.id, email.subject);
      done[email.id] = 'task';
      onTaskCreated?.(task);
      setTimeout(() => { inboxEmails = inboxEmails.filter(e => e.id !== email.id); }, 500);
    } catch (e: any) { error = e.message; }
    finally { converting[email.id] = false; }
  }

  async function archive(email: FastmailEmail) {
    archiving[email.id] = true;
    try {
      await api.integrations.fastmail.archive(email.id);
      inboxEmails = inboxEmails.filter(e => e.id !== email.id);
    } catch { /* ignore */ }
    finally { archiving[email.id] = false; }
  }

  async function unarchive(email: FastmailEmail) {
    unarchiving[email.id] = true;
    try {
      await api.integrations.fastmail.unarchive(email.id);
      archivedEmails = archivedEmails.filter(e => e.id !== email.id);
    } catch (e: any) { error = e.message ?? 'Failed to unarchive'; }
    finally { unarchiving[email.id] = false; }
  }

  export function removeEmail(id: string) {
    inboxEmails = inboxEmails.filter(e => e.id !== id);
  }

  function senderName(e: FastmailEmail) {
    return e.from?.[0]?.name || e.from?.[0]?.email || '?';
  }
  function senderInitial(e: FastmailEmail) {
    return senderName(e).charAt(0).toUpperCase();
  }
  function formatTime(iso: string): string {
    const d = new Date(iso);
    const diffDays = Math.floor((Date.now() - d.getTime()) / 86400000);
    if (diffDays === 0) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7)  return d.toLocaleDateString([], { weekday: 'short' });
    return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
  }
  function onDragStart(e: DragEvent, email: FastmailEmail) {
    e.dataTransfer?.setData('application/x-sempa-email',
      JSON.stringify({ id: email.id, subject: email.subject }));
    e.dataTransfer!.effectAllowed = 'copy';
  }

  const emails = $derived(tab === 'inbox' ? inboxEmails : archivedEmails);
</script>

<div class="flex h-full flex-col">
  <!-- Tabs: a segmented pill on the panel -->
  <div class="flex shrink-0 items-center gap-2 px-3 py-2.5">
    <div class="segmented flex-1" role="tablist" aria-label="Mailbox">
      <button onclick={() => switchTab('inbox')} role="tab" aria-selected={tab === 'inbox'}
              class="py-1.5 text-xs font-semibold transition-colors"
              style="color: {tab === 'inbox' ? 'var(--sempa-text)' : 'var(--sempa-text-soft)'};">
        Inbox
      </button>
      <button onclick={() => switchTab('archived')} role="tab" aria-selected={tab === 'archived'}
              class="py-1.5 text-xs font-semibold transition-colors"
              style="color: {tab === 'archived' ? 'var(--sempa-text)' : 'var(--sempa-text-soft)'};">
        Archived
      </button>
    </div>
    <button onclick={() => tab === 'inbox' ? loadInbox() : loadArchived()}
            disabled={loading}
            aria-label="Refresh"
            class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full transition-colors disabled:opacity-40"
            style="background: var(--sempa-fill); color: var(--sempa-text-soft);">
      <svg class="h-3.5 w-3.5 {loading ? 'animate-spin' : ''}" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
      </svg>
    </button>
  </div>

  <div class="flex-1 overflow-y-auto">
    {#if !connected}
      <div class="flex flex-col items-center justify-center gap-2 p-6 text-center">
        <p class="text-xs" style="color: var(--sempa-text-dim);">Fastmail not connected</p>
        <a href="/settings/accounts" class="btn-soft h-8 px-3.5 text-xs">Set up →</a>
      </div>

    {:else if loading}
      <div class="space-y-px">
        {#each Array(5) as _}
          <div class="flex items-start gap-2.5 px-3 py-3 animate-pulse">
            <div class="h-8 w-8 shrink-0 rounded-full" style="background: var(--sempa-fill);"></div>
            <div class="flex-1 space-y-1.5 pt-0.5">
              <div class="h-2.5 w-20 rounded-full" style="background: var(--sempa-fill);"></div>
              <div class="h-2.5 w-full rounded-full" style="background: var(--sempa-fill);"></div>
            </div>
          </div>
        {/each}
      </div>

    {:else if error}
      <p class="p-4 text-xs" style="color: var(--sempa-danger, #c0392b);">{error}</p>

    {:else if emails.length === 0}
      <div class="flex h-32 items-center justify-center">
        <p class="text-xs" style="color: var(--sempa-text-dim);">
          {tab === 'inbox' ? 'Inbox is empty' : 'No archived emails'}
        </p>
      </div>

    {:else}
      <ul>
        {#each emails as email, i (email.id)}
          <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
          <li class="email-row group relative mx-1.5 rounded-[14px] px-2.5 py-2.5
                     {done[email.id] ? 'opacity-40' : ''}
                     {tab === 'inbox' ? 'cursor-grab active:cursor-grabbing' : ''}"
              style={i > 0 ? 'box-shadow: 0 -1px 0 var(--sempa-border);' : ''}
              draggable={tab === 'inbox'}
              ondragstart={tab === 'inbox' ? (e) => onDragStart(e, email) : null}>

            <div class="flex items-start gap-2.5">
              <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-bold"
                   style={email.is_unread
                     ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);'
                     : 'background: var(--sempa-fill); color: var(--sempa-text-soft);'}>
                {senderInitial(email)}
              </div>
              <div class="min-w-0 flex-1 pr-1">
                <div class="flex items-baseline justify-between gap-1">
                  <span class="truncate text-xs"
                        style="color: {email.is_unread ? 'var(--sempa-text)' : 'var(--sempa-text-soft)'}; font-weight: {email.is_unread ? 700 : 500};">
                    {senderName(email)}
                  </span>
                  <span class="shrink-0 font-mono text-[10.5px]" style="color: var(--sempa-text-dim);">{formatTime(email.received_at)}</span>
                </div>
                <p class="truncate text-xs"
                   style="color: {email.is_unread ? 'var(--sempa-text)' : 'var(--sempa-text-dim)'}; font-weight: {email.is_unread ? 500 : 400};">
                  {email.subject || '(no subject)'}
                </p>
              </div>
            </div>

            <!-- Hover actions -->
            <div class="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1
                        opacity-0 group-hover:opacity-100 transition-opacity">
              {#if tab === 'inbox'}
                <button onclick={() => toTask(email)}
                        disabled={!!converting[email.id] || !!done[email.id]}
                        title="Add as task + archive"
                        class="btn-primary h-7 px-2.5 text-[11px] disabled:opacity-50">
                  {converting[email.id] ? '…' : done[email.id] === 'task' ? '✓' : '→ Task'}
                </button>
                <button onclick={() => archive(email)}
                        disabled={archiving[email.id]}
                        title="Archive"
                        aria-label="Archive"
                        class="flex h-7 w-7 items-center justify-center rounded-full text-xs transition-colors disabled:opacity-50"
                        style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text-soft);">
                  {archiving[email.id] ? '…' : '×'}
                </button>
              {:else}
                <button onclick={() => unarchive(email)}
                        disabled={unarchiving[email.id]}
                        title="Move back to inbox"
                        class="btn-surface h-7 px-2.5 text-[11px] disabled:opacity-50">
                  {unarchiving[email.id] ? '…' : '↩ Inbox'}
                </button>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>

  {#if tab === 'inbox'}
    <div class="shrink-0 px-4 py-2.5" style="border-top: 1px solid var(--sempa-border);">
      <p class="text-[11px]" style="color: var(--sempa-text-dim);">Drag emails onto a task column</p>
    </div>
  {/if}
</div>

<style>
  .email-row { transition: background-color 120ms ease; }
  .email-row:hover { background: var(--sempa-fill); box-shadow: none !important; }
  .email-row:hover + .email-row { box-shadow: none !important; }
</style>
