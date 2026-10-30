<script lang="ts">
  /**
   * In-app banner(s) for task reminders that have just come due while the app is
   * open. Rendered at the top of the main content by +layout.svelte, above the
   * routine banner. Unlike the routine prompts this IS an alarm surface — it
   * tells the user exactly which task rang, with quick Open / Done / Snooze
   * actions. It's the cross-platform backstop for when a native OS toast is
   * suppressed (e.g. Windows focus assist) and only the sound was heard.
   */
  import { goto } from '$app/navigation';
  import { reminderAlerts } from '$lib/stores/reminderAlerts.svelte';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { Bell, X } from 'lucide-svelte';

  // The in-app banner is the bulletproof, always-DOM visual: it CANNOT silently
  // fail the way the native toast or the separate floating window can on Windows.
  // So we now show it on every platform whenever a reminder is up. On desktop the
  // floating Granola card additionally covers the case where Sempa is in the
  // background (see $lib/desktopReminderPopup, which only opens the card when the
  // app isn't foregrounded — so there's no double display while you're in-app).
  const show = $derived(reminderAlerts.alerts.length > 0);

  // Turn the reminder into the on-ramp: fetch the task for its planned estimate,
  // start a focus session pre-loaded with it, then open the focus page so the
  // running timer shows alongside the task's context.
  async function startFocus(taskId: string, title: string) {
    reminderAlerts.dismiss(taskId);
    const { api } = await import('$lib/api');
    try {
      const t = await api.tasks.get(taskId);
      pomodoro.start(t.id, t.title, t.time_actual_minutes ?? 0, t.time_estimate_minutes ?? null);
    } catch {
      // Offline or fetch failed — still start with what the alert carried.
      pomodoro.start(taskId, title, 0, null);
    }
    goto(`/focus/${taskId}`);
  }
</script>

{#if show}
  <div class="flex flex-col gap-1.5">
    {#each reminderAlerts.alerts as a (a.taskId)}
      <!-- One compact row per reminder: icon + title on the left, quick actions
           on the right. Wraps to a second line only on very narrow widths. -->
      <div class="banner flex-wrap" style="row-gap: 10px;">
        <div class="flex min-w-0 flex-[1_1_14rem] items-center gap-3">
          <!-- The one "now" thing on screen: a solid tile. -->
          <div class="banner-icon" style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); box-shadow: var(--sempa-shadow-accent);">
            <Bell size={17} strokeWidth={2} />
          </div>
          <div class="min-w-0 flex-1">
            <span class="banner-hint" style="font-weight: 700; color: var(--sempa-accent-ink);">
              Reminder
            </span>
            <p class="banner-title truncate">{a.title}</p>
          </div>
        </div>

        <!-- Quick actions: compact, right-aligned, never grow to fill the row. -->
        <div class="ml-auto flex shrink-0 items-center gap-1.5">
          <button
            onclick={() => startFocus(a.taskId, a.title)}
            class="btn-primary banner-btn">
            Focus
          </button>
          <button
            onclick={() => reminderAlerts.markDone(a.taskId)}
            class="btn-soft banner-btn banner-quiet">
            Done
          </button>
          <button
            onclick={() => reminderAlerts.snooze(a.taskId)}
            class="btn-soft banner-btn banner-quiet"
            title="Snooze 1 hour">
            Snooze
          </button>
          <button
            onclick={() => reminderAlerts.dismiss(a.taskId)}
            aria-label="Dismiss"
            class="banner-x">
            <X size={16} />
          </button>
        </div>
      </div>
    {/each}
  </div>
{/if}

<style>
  /* 1.27: a soft raised surface with a tinted icon tile and pill actions. */
  .banner {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px;
    border-radius: 18px;
    background: var(--sempa-bg-panel);
    box-shadow: var(--sempa-shadow-1);
  }
  .banner-icon {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 38px;
    height: 38px;
    border-radius: 12px;
    background: var(--sempa-accent-bg);
    color: var(--sempa-accent-ink);
  }
  .banner-title { font-size: 13.5px; font-weight: 700; line-height: 1.25; color: var(--sempa-text); }
  .banner-hint  { font-size: 12px; font-weight: 500; line-height: 1.35; color: var(--sempa-text-soft); }
  .banner-btn   { height: 36px; padding: 0 15px; font-size: 12.5px; flex-shrink: 0; }
  .banner-quiet { background: var(--sempa-fill); color: var(--sempa-text-soft); }
  .banner-quiet:hover { color: var(--sempa-text); }
  .banner-x {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: 999px;
    color: var(--sempa-text-dim);
    transition: background-color 120ms ease, color 120ms ease;
  }
  .banner-x:hover { background: var(--sempa-fill); color: var(--sempa-text); }
</style>
