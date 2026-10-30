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
  <div class="banner flex-wrap">
    <div class="flex min-w-0 flex-[1_1_15rem] items-center gap-3">
      <div class="banner-icon">
        {#if mode === 'plan'}
          <CalendarCheck size={17} strokeWidth={2} />
        {:else}
          <Moon size={17} strokeWidth={2} />
        {/if}
      </div>

      <!-- Title over a one-line hint. -->
      <div class="min-w-0 flex-1">
        <p class="banner-title truncate">
          {mode === 'plan' ? 'Plan your week' : 'Daily shutdown'}
        </p>
        <p class="banner-hint truncate">
          {mode === 'plan'
            ? 'Schedule what matters this week.'
            : "Close out today and reschedule what's left."}
        </p>
      </div>
    </div>

    <div class="ml-auto flex shrink-0 items-center gap-1.5">
      <button
        onclick={() => (mode === 'plan' ? routines.startWeeklyPlan() : routines.startShutdown())}
        class="btn-primary banner-btn">
        {mode === 'plan' ? 'Start planning' : 'Start shutdown'}
      </button>

      <button
        onclick={() => (mode === 'plan' ? routines.dismissWeeklyPlan() : routines.dismissShutdown())}
        aria-label="Dismiss"
        class="banner-x">
        <X size={16} />
      </button>
    </div>
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
