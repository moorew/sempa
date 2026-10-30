<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import { today } from '$lib/utils';
  import { CalendarCheck, MailCheck, Timer, Sunset, Sparkles } from 'lucide-svelte';

  let step = $state(1);
  const TOTAL = 3;

  // Integration connection state (read from the API)
  let gmailConnected   = $state(false);
  let fastmailConnected = $state(false);
  let jiraConnected    = $state(false);
  let loading          = $state(true);

  onMount(async () => {
    const [gmail, fastmail, jira] = await Promise.all([
      api.integrations.gmail.get().catch(() => ({ connected: false })),
      api.integrations.fastmail.get().catch(() => ({ connected: false })),
      api.integrations.jira.get().catch(() => ({ connected: false })),
    ]);
    gmailConnected    = gmail.connected;
    fastmailConnected = fastmail.connected;
    jiraConnected     = jira.connected;
    loading = false;
  });

  async function finish() {
    await api.setup.complete();
    goto('/day/' + today());
  }

  const progressPct = $derived(((step - 1) / TOTAL) * 100);
</script>

<svelte:head><title>Welcome to Sempa</title></svelte:head>

<div class="flex min-h-screen flex-col items-center justify-center px-4 py-10"
     style="background: var(--sempa-bg-main);">

  <div class="surface-raised w-full max-w-[460px] px-6 pt-6 pb-7 sm:px-8 sm:pt-7">

    <!-- Progress bar -->
    <div class="mb-7 flex items-center gap-3">
      <div class="day-bar-track flex-1">
        <div class="day-bar-fill duration-500"
             style="width: {progressPct}%; background: var(--sempa-accent);"></div>
      </div>
      <p class="shrink-0 text-[12.5px] font-semibold" style="color: var(--sempa-text-soft);">Step {step} of {TOTAL}</p>
    </div>

    <!-- ── Step 1: Welcome ───────────────────────────────────────────────── -->
    {#if step === 1}
      <div class="text-center">
        <div class="mb-5 inline-flex h-16 w-16 items-center justify-center rounded-[20px]"
             style="background: var(--sempa-btn-bg); box-shadow: var(--sempa-shadow-accent);">
          <svg width="36" height="36" viewBox="0 0 100 100" fill="none" aria-hidden="true">
            <path d="M22,40 a28,28 0 0 0 56,0" stroke="var(--sempa-btn-fg)" stroke-width="10" stroke-linecap="round"/>
            <circle cx="50" cy="35" r="8" fill="var(--sempa-btn-fg)"/>
          </svg>
        </div>

        <h1 class="type-page mb-2" style="color: var(--sempa-text);">
          Welcome to Sempa
        </h1>
        <p class="mx-auto mb-6 max-w-[34ch] text-[14px] leading-relaxed" style="color: var(--sempa-text-soft);">
          Your personal daily planner — plan your day, track your work,
          and end each day with intention.
        </p>

        <div class="setup-list mb-7 text-left">
          {#each [
            { Icon: CalendarCheck, title: 'Daily Kanban', desc: 'Plan what you\'re working on each day with a simple board.' },
            { Icon: MailCheck, title: 'Email → Tasks', desc: 'Import starred emails from Gmail or Fastmail directly as tasks.' },
            { Icon: Timer, title: 'Pomodoro + timeboxing', desc: 'Schedule focused blocks and track sessions per task.' },
            { Icon: Sunset, title: 'Shutdown ritual', desc: 'End your day deliberately — review what got done, plan tomorrow.' },
          ] as { Icon, title, desc }}
            <div class="flex items-start gap-3 px-4 py-3">
              <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl"
                    style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
                <Icon size={18} strokeWidth={2} />
              </span>
              <div class="min-w-0 pt-0.5">
                <p class="text-[14px] font-semibold" style="color: var(--sempa-text);">{title}</p>
                <p class="text-[12.5px] leading-snug" style="color: var(--sempa-text-soft);">{desc}</p>
              </div>
            </div>
          {/each}
        </div>

        <button onclick={() => step = 2}
                class="btn-primary setup-cta">
          Get started →
        </button>
      </div>

    <!-- ── Step 2: Connect tools ─────────────────────────────────────────── -->
    {:else if step === 2}
      <div>
        <h2 class="type-page mb-1.5" style="color: var(--sempa-text);">Connect your tools</h2>
        <p class="mb-6 text-[14px] leading-relaxed" style="color: var(--sempa-text-soft);">
          All integrations are optional — you can set them up any time in Settings.
        </p>

        {#if loading}
          <div class="flex justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-[2.5px]"
                 style="border-color: var(--sempa-track); border-top-color: var(--sempa-accent);"></div>
          </div>
        {:else}
          <div class="setup-list">

            <!-- Gmail -->
            <div class="flex items-center gap-3 px-4 py-3" style="min-height: 64px;">
              <span class="tint flex h-10 w-10 shrink-0 items-center justify-center rounded-xl" style="--tint: #ea4335;">
                <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4-8 5-8-5V6l8 5 8-5v2z"/>
                </svg>
              </span>
              <div class="min-w-0 flex-1">
                <p class="text-[14px] font-semibold" style="color: var(--sempa-text);">Gmail</p>
                <p class="text-[12.5px]" style="color: var(--sempa-text-soft);">Import starred emails as tasks</p>
              </div>
              {#if gmailConnected}
                <span class="setup-connected">Connected</span>
              {:else}
                <a href={api.integrations.gmail.authUrl(false)}
                   class="btn-primary setup-btn">
                  Connect
                </a>
              {/if}
            </div>

            <!-- Fastmail -->
            <div class="flex items-center gap-3 px-4 py-3" style="min-height: 64px;">
              <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl"
                    style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
                <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
                  <path stroke-linecap="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
                </svg>
              </span>
              <div class="min-w-0 flex-1">
                <p class="text-[14px] font-semibold" style="color: var(--sempa-text);">Fastmail</p>
                <p class="text-[12.5px]" style="color: var(--sempa-text-soft);">Email + calendar sync</p>
              </div>
              {#if fastmailConnected}
                <span class="setup-connected">Connected</span>
              {:else}
                <a href="/settings/accounts"
                   class="btn-soft setup-btn">
                  Set up →
                </a>
              {/if}
            </div>

            <!-- Jira -->
            <div class="flex items-center gap-3 px-4 py-3" style="min-height: 64px;">
              <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl"
                    style="background: var(--sempa-accent-bg); color: var(--sempa-accent);">
                <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
                </svg>
              </span>
              <div class="min-w-0 flex-1">
                <p class="text-[14px] font-semibold" style="color: var(--sempa-text);">Jira</p>
                <p class="text-[12.5px]" style="color: var(--sempa-text-soft);">Sync assigned issues as tasks</p>
              </div>
              {#if jiraConnected}
                <span class="setup-connected">Connected</span>
              {:else}
                <a href="/settings/integrations/jira"
                   class="btn-soft setup-btn">
                  Set up →
                </a>
              {/if}
            </div>

          </div>
        {/if}

        <div class="mt-7 flex gap-2.5">
          <button onclick={() => step = 1}
                  class="setup-back">
            ← Back
          </button>
          <button onclick={() => step = 3}
                  class="btn-primary setup-cta flex-1">
            Continue →
          </button>
        </div>
        <button onclick={finish}
                class="setup-link mt-2">
          Skip and go to the app
        </button>
      </div>

    <!-- ── Step 3: All set ───────────────────────────────────────────────── -->
    {:else if step === 3}
      <div class="text-center">
        <div class="mb-5 inline-flex h-16 w-16 items-center justify-center rounded-[20px]"
             style="background: var(--sempa-success-bg); color: var(--sempa-success);">
          <Sparkles size={30} strokeWidth={1.9} />
        </div>

        <h2 class="type-page mb-2" style="color: var(--sempa-text);">You're all set!</h2>
        <p class="mb-6 text-[14px]" style="color: var(--sempa-text-soft);">
          Here are a few things to try first.
        </p>

        <div class="setup-list mb-7 text-left">
          {#each [
            ['n', 'Press  to create a new task on the Today view'],
            ['?', 'Press  to see all keyboard shortcuts'],
            ['→', 'Drag emails from the right panel onto a day column to create tasks'],
            ['⚙', 'Visit Settings to connect more integrations any time'],
          ] as [key, desc]}
            <div class="flex items-center gap-3 px-4 py-3">
              <kbd class="inline-flex h-7 min-w-7 shrink-0 items-center justify-center rounded-lg px-1.5 font-mono text-[12px] font-bold"
                   style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">{key}</kbd>
              <span class="text-[13.5px]" style="color: var(--sempa-text-soft);">{desc}</span>
            </div>
          {/each}
        </div>

        <button onclick={finish}
                class="btn-primary setup-cta">
          Start planning →
        </button>

        <button onclick={() => step = 2}
                class="setup-link mt-2">
          ← Back to integrations
        </button>
      </div>
    {/if}

  </div>
</div>

<style>
  /* Nested list on the raised card: a fill block with divider rows. */
  .setup-list { background: var(--sempa-fill); border-radius: 18px; }
  .setup-list > * + * { border-top: 1px solid var(--sempa-border); }
  .setup-cta { width: 100%; height: 48px; font-size: 15px; }
  .setup-btn { height: 34px; padding: 0 14px; font-size: 12.5px; flex-shrink: 0; }
  .setup-back {
    display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0;
    height: 48px; padding: 0 20px; border-radius: 999px;
    background: var(--sempa-fill); color: var(--sempa-text);
    box-shadow: inset 0 0 0 1px var(--sempa-border);
    font-size: 14px; font-weight: 600;
  }
  .setup-link {
    display: block; width: 100%; min-height: 36px; border-radius: 999px; text-align: center;
    font-size: 13px; font-weight: 600; color: var(--sempa-text-soft);
    transition: color 120ms ease;
  }
  .setup-link:hover { color: var(--sempa-accent-ink); }
  .setup-connected {
    display: inline-flex; align-items: center; gap: 6px; flex-shrink: 0;
    height: 26px; padding: 0 10px; border-radius: 999px;
    background: var(--sempa-success-bg); color: var(--sempa-success);
    font-size: 12px; font-weight: 700;
  }
  .setup-connected::before { content: ''; width: 7px; height: 7px; border-radius: 999px; background: currentColor; }
</style>
