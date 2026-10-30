<script lang="ts">
  import { onMount } from 'svelte';
  import { RefreshCw } from 'lucide-svelte';
  import { api } from '$lib/api';
  import type { FastmailEmail } from '$lib/types';

  let emails    = $state<FastmailEmail[]>([]);
  let loading   = $state(true);
  let error     = $state('');
  let connected = $state(true);

  // Per-email action state
  let converting = $state<Record<string, boolean>>({});
  let archiving  = $state<Record<string, boolean>>({});
  let done       = $state<Record<string, boolean>>({});

  onMount(load);

  async function load() {
    loading = true; error = '';
    try {
      emails = await api.integrations.fastmail.emails();
    } catch (e: any) {
      if (e.message?.includes('not connected')) {
        connected = false;
      } else {
        error = e.message ?? 'Failed to load emails';
      }
    } finally {
      loading = false;
    }
  }

  async function toTask(email: FastmailEmail) {
    converting[email.id] = true;
    try {
      await api.integrations.fastmail.toTask(email.id, email.subject);
      done[email.id] = true;
      // Fade out after a moment
      setTimeout(() => { emails = emails.filter(e => e.id !== email.id); }, 600);
    } catch (e: any) {
      error = e.message;
    } finally {
      converting[email.id] = false;
    }
  }

  async function archive(email: FastmailEmail) {
    archiving[email.id] = true;
    try {
      await api.integrations.fastmail.archive(email.id);
      emails = emails.filter(e => e.id !== email.id);
    } catch (e: any) {
      error = e.message;
    } finally {
      archiving[email.id] = false;
    }
  }

  function senderName(email: FastmailEmail): string {
    if (!email.from?.length) return '?';
    return email.from[0].name || email.from[0].email;
  }

  function senderInitial(email: FastmailEmail): string {
    return senderName(email).charAt(0).toUpperCase();
  }

  function formatTime(iso: string): string {
    const d = new Date(iso);
    const now = new Date();
    const diffDays = Math.floor((now.getTime() - d.getTime()) / 86400000);
    if (diffDays === 0) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7)  return d.toLocaleDateString([], { weekday: 'short' });
    return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
  }
</script>

<svelte:head><title>Inbox — Sempa</title></svelte:head>

<div class="flex h-full flex-col" style="background: var(--sempa-bg-main);">
  <!-- Header — sticky + safe-area padding so the title clears the status bar
       (clock) on mobile, matching the other full-page mobile headers. -->
  <header class="sticky top-0 z-[40]"
          style="background: var(--sempa-bg-main);
                 padding-top: max(16px, calc(env(safe-area-inset-top, 0px) + 12px));">
    <div class="mx-auto flex max-w-3xl items-end justify-between gap-3 px-4 pb-3 pt-3 sm:px-6">
      <div class="min-w-0">
        <h1 class="type-page" style="color: var(--sempa-text);">Fastmail inbox</h1>
        {#if !loading}
          <p class="type-meta mt-1" style="color: var(--sempa-text-dim);">
            {emails.length} message{emails.length !== 1 ? 's' : ''}{emails.some((e) => e.is_unread) ? ` · ${emails.filter((e) => e.is_unread).length} unread` : ''}
          </p>
        {/if}
      </div>
      <button onclick={load} disabled={loading}
              class="btn-surface h-9 shrink-0 gap-1.5 px-4 text-[12.5px] disabled:opacity-40">
        <RefreshCw size={13} class={loading ? 'animate-spin' : ''} />
        {loading ? 'Loading…' : 'Refresh'}
      </button>
    </div>
  </header>

  <!-- Body -->
  <div class="flex-1 overflow-y-auto">
    {#if !connected}
      <div class="flex h-full flex-col items-center justify-center gap-3 text-center px-6">
        <div class="flex h-12 w-12 items-center justify-center rounded-2xl" style="background: var(--sempa-accent-bg);">
          <svg class="h-6 w-6" style="color: var(--sempa-accent-ink);" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
          </svg>
        </div>
        <p class="font-bold" style="font-size: 15px; color: var(--sempa-text);">Fastmail not connected</p>
        <p class="type-meta" style="color: var(--sempa-text-soft);">Connect your Fastmail account in Settings → Accounts.</p>
        <a href="/settings/accounts" class="btn-primary mt-1 h-10 px-5 text-sm">
          Go to Settings
        </a>
      </div>

    {:else if loading}
      <div class="mx-auto max-w-3xl px-4 py-3 sm:px-6">
        <div class="surface overflow-hidden" style="border-radius: 22px;">
          {#each Array(6) as _, i}
            <div class="flex items-start gap-3 px-4 py-4 animate-pulse"
                 style={i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}>
              <div class="h-9 w-9 shrink-0 rounded-full" style="background: var(--sempa-fill);"></div>
              <div class="flex-1 space-y-2 pt-0.5">
                <div class="h-3 w-32 rounded-full" style="background: var(--sempa-fill);"></div>
                <div class="h-3 w-64 max-w-full rounded-full" style="background: var(--sempa-fill);"></div>
                <div class="h-3 w-48 max-w-full rounded-full" style="background: var(--sempa-fill);"></div>
              </div>
            </div>
          {/each}
        </div>
      </div>

    {:else if error}
      <div class="mx-auto max-w-3xl px-4 py-3 sm:px-6">
        <div class="surface flex items-center gap-3 px-4 py-3 text-sm" style="color: var(--sempa-danger, #c0392b);">
          <span class="flex-1">{error}</span>
          <button onclick={load} class="btn-soft h-8 px-3.5 text-[12px]">Retry</button>
        </div>
      </div>

    {:else if emails.length === 0}
      <div class="flex h-full flex-col items-center justify-center gap-3 text-center">
        <span class="flex h-12 w-12 items-center justify-center rounded-2xl"
              style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text-dim);">
          <svg class="h-6 w-6" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
          </svg>
        </span>
        <p class="type-meta" style="color: var(--sempa-text-soft);">Inbox is empty</p>
      </div>

    {:else}
      <!-- One raised surface; each message is a row divided by a hairline. -->
      <div class="mx-auto max-w-3xl px-4 pb-6 pt-3 sm:px-6">
        <div class="surface overflow-hidden" style="border-radius: 22px;">
          {#each emails as email, i (email.id)}
            <div class="flex items-start gap-3 px-4 py-3.5 transition-opacity"
                 style="{i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}
                        {done[email.id] ? 'opacity: 0.5;' : ''}">

              <!-- Avatar -->
              <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-[13px] font-bold"
                   style={email.is_unread
                     ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);'
                     : 'background: var(--sempa-fill); color: var(--sempa-text-soft);'}>
                {senderInitial(email)}
              </div>

              <!-- Content -->
              <div class="min-w-0 flex-1">
                <div class="flex items-baseline justify-between gap-2">
                  <span class="flex min-w-0 items-center gap-1.5 text-sm" style="color: var(--sempa-text); font-weight: {email.is_unread ? 700 : 500};">
                    {#if email.is_unread}<span class="h-2 w-2 shrink-0 rounded-full" style="background: var(--sempa-accent);" aria-label="Unread"></span>{/if}
                    <span class="truncate">{senderName(email)}</span>
                  </span>
                  <span class="shrink-0 font-mono text-[11px]" style="color: var(--sempa-text-dim);">{formatTime(email.received_at)}</span>
                </div>
                <p class="truncate text-[13.5px]" style="color: {email.is_unread ? 'var(--sempa-text)' : 'var(--sempa-text-soft)'}; font-weight: {email.is_unread ? 600 : 400};">
                  {email.subject || '(no subject)'}
                </p>
                {#if email.preview}
                  <p class="type-meta mt-0.5 line-clamp-2" style="color: var(--sempa-text-dim); font-weight: 400;">{email.preview}</p>
                {/if}

                <!-- Actions -->
                <div class="mt-2.5 flex items-center gap-2">
                  <button onclick={() => toTask(email)}
                          disabled={converting[email.id] || done[email.id]}
                          title="Add to today's tasks and archive"
                          class="btn-primary h-8 gap-1 px-3.5 text-[12px] disabled:opacity-50">
                    {#if converting[email.id]}
                      <svg class="h-3 w-3 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"/>
                      </svg>
                      Adding…
                    {:else if done[email.id]}
                      ✓ Added
                    {:else}
                      → Task
                    {/if}
                  </button>
                  <button onclick={() => archive(email)}
                          disabled={archiving[email.id]}
                          title="Archive"
                          class="btn-soft h-8 px-3.5 text-[12px] disabled:opacity-50"
                          style="background: var(--sempa-fill); color: var(--sempa-text-soft);">
                    {archiving[email.id] ? '…' : 'Archive'}
                  </button>
                </div>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>
</div>
