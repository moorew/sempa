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
      <div class="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border px-3 py-2"
           style="border-color: var(--sempa-accent); background: var(--sempa-accent-bg);">
        <div class="flex min-w-0 flex-1 items-center gap-2.5">
          <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg"
               style="background: var(--sempa-bg-panel); color: var(--sempa-accent);">
            <Bell size={15} strokeWidth={2} />
          </div>
          <div class="min-w-0 flex-1">
            <span class="font-semibold" style="font-size: 9.5px; letter-spacing: 0.06em; text-transform: uppercase; color: var(--sempa-accent);">
              Reminder
            </span>
            <p class="truncate" style="font-size: 13px; line-height: 1.25; color: var(--sempa-text);">{a.title}</p>
          </div>
        </div>

        <!-- Quick actions: compact, right-aligned, never grow to fill the row. -->
        <div class="flex shrink-0 items-center gap-1.5">
          <button
            onclick={() => startFocus(a.taskId, a.title)}
            class="rounded-md px-2.5 py-1 font-semibold transition-opacity hover:opacity-90"
            style="font-size: 12px; background: var(--sempa-accent); color: #fff;">
            Focus
          </button>
          <button
            onclick={() => reminderAlerts.markDone(a.taskId)}
            class="rounded-md px-2 py-1 font-medium transition-colors"
            style="font-size: 12px; color: var(--sempa-text-soft); border: 1px solid var(--sempa-border);">
            Done
          </button>
          <button
            onclick={() => reminderAlerts.snooze(a.taskId)}
            class="rounded-md px-2 py-1 font-medium transition-colors"
            style="font-size: 12px; color: var(--sempa-text-soft); border: 1px solid var(--sempa-border);"
            title="Snooze 1 hour">
            Snooze
          </button>
          <button
            onclick={() => reminderAlerts.dismiss(a.taskId)}
            aria-label="Dismiss"
            class="ml-0.5 shrink-0 rounded-md p-1 transition-colors"
            style="color: var(--sempa-text-dim);">
            <X size={15} />
          </button>
        </div>
      </div>
    {/each}
  </div>
{/if}
