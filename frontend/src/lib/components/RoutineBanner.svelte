<script lang="ts">
  /**
   * Non-intrusive in-app prompt for the scheduled routines (weekly planning /
   * daily shutdown). Rendered at the top of the main content by +layout.svelte.
   * This is intentionally a calm banner — NOT an OS alarm.
   */
  import { routines } from '$lib/stores/routines.svelte';
  import { CalendarCheck, Moon, X } from 'lucide-svelte';

  // Weekly planning takes priority if (rarely) both are due at once.
  const mode = $derived(
    routines.weeklyPlanDue ? 'plan' : routines.shutdownDue ? 'shutdown' : null,
  );
</script>

{#if mode}
  <div class="flex items-center gap-2.5 rounded-lg border px-3 py-2"
       style="border-color: var(--sempa-accent-bg); background: var(--sempa-accent-bg);">
    <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg"
         style="background: var(--sempa-bg-panel); color: var(--sempa-accent);">
      {#if mode === 'plan'}
        <CalendarCheck size={15} strokeWidth={2} />
      {:else}
        <Moon size={15} strokeWidth={2} />
      {/if}
    </div>

    <!-- Title + one-line hint on a single row to stay compact. -->
    <div class="flex min-w-0 flex-1 items-baseline gap-2">
      <p class="shrink-0 font-semibold" style="font-size: 13px; color: var(--sempa-text);">
        {mode === 'plan' ? 'Plan your week' : 'Daily shutdown'}
      </p>
      <p class="truncate" style="font-size: 11.5px; color: var(--sempa-text-soft);">
        {mode === 'plan'
          ? 'Schedule what matters this week.'
          : "Close out today and reschedule what's left."}
      </p>
    </div>

    <button
      onclick={() => (mode === 'plan' ? routines.startWeeklyPlan() : routines.startShutdown())}
      class="shrink-0 rounded-md px-2.5 py-1 font-semibold transition-opacity hover:opacity-90"
      style="font-size: 12px; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
      {mode === 'plan' ? 'Start planning' : 'Start shutdown'}
    </button>

    <button
      onclick={() => (mode === 'plan' ? routines.dismissWeeklyPlan() : routines.dismissShutdown())}
      aria-label="Dismiss"
      class="shrink-0 rounded-md p-1 transition-colors"
      style="color: var(--sempa-text-dim);">
      <X size={15} />
    </button>
  </div>
{/if}
