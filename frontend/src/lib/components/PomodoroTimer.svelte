<script lang="ts">
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { formatMinutes } from '$lib/utils';
  import { isTauri, openPomodoroWidget } from '$lib/tauri/bridge';

  // Phase accent follows the active theme: work uses the brand accent; breaks
  // use the success hue so the timer matches whichever theme the user picked.
  const accentColor = $derived(
    pomodoro.phase === 'work' ? 'var(--sempa-accent)' : 'var(--sempa-success)'
  );

  // The honest signal for time-blindness: how far past (or under) the estimate
  // the live session has run. Positive = over plan.
  const overBy = $derived(pomodoro.overByMinutes);

  let settingsOpen = $state(false);
  let collapsed    = $state(false);
  let workInput       = $state(pomodoro.workMins);
  let shortBreakInput = $state(pomodoro.shortBreakMins);
  let longBreakInput  = $state(pomodoro.longBreakMins);

  function openSettings() {
    workInput       = pomodoro.workMins;
    shortBreakInput = pomodoro.shortBreakMins;
    longBreakInput  = pomodoro.longBreakMins;
    settingsOpen    = true;
  }

  function applySettings() {
    const w  = Math.max(1, Math.min(120, workInput       || 25));
    const sb = Math.max(1, Math.min(60,  shortBreakInput || 5));
    const lb = Math.max(1, Math.min(60,  longBreakInput  || 15));
    pomodoro.setPrefs(w, sb, lb);
    settingsOpen = false;
  }
</script>

<div
  class="pomo fixed bottom-20 right-4 z-50 w-[16rem] p-4 sm:bottom-6 sm:right-6"
  style="color: var(--sempa-text);"
>
  <!-- Header row -->
  <div class="mb-1.5 flex items-center justify-between">
    <span class="chip" style="background: {pomodoro.phase === 'work' ? 'var(--sempa-accent-bg)' : 'var(--sempa-success-bg)'};
                               color: {pomodoro.phase === 'work' ? 'var(--sempa-accent-ink)' : 'var(--sempa-success)'};">
      <span class="h-1.5 w-1.5 rounded-full" style="background: {accentColor};"></span>
      {pomodoro.phaseLabel}
    </span>
    <div class="-mr-1.5 flex items-center" style="color: var(--sempa-text-dim);">
      <button onclick={() => (collapsed = !collapsed)}
        class="pomo-icon"
        aria-label={collapsed ? 'Expand timer' : 'Collapse timer'}>
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d={collapsed ? 'M5 15l7-7 7 7' : 'M19 9l-7 7-7-7'} />
        </svg>
      </button>
      {#if isTauri()}
        <button onclick={() => openPomodoroWidget()}
          class="pomo-icon" aria-label="Pop out timer to a floating window">
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M14 5h5m0 0v5m0-5l-7 7M9 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-3" />
          </svg>
        </button>
      {/if}
      <button onclick={openSettings}
        class="pomo-icon" aria-label="Timer settings">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </button>
      <button onclick={() => pomodoro.requestStop()}
        class="pomo-icon" aria-label="Stop and log time">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2.25" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  </div>

  {#if settingsOpen}
    <!-- Settings panel -->
    <div class="mb-1 mt-2 p-3" style="background: var(--sempa-fill); border-radius: 16px;">
      <p class="mb-2 type-label" style="color: var(--sempa-text-soft);">
        Durations (min)
      </p>
      <div class="flex flex-col gap-2">
        <label class="flex items-center justify-between text-xs font-medium" style="color: var(--sempa-text-soft);">
          Focus
          <input type="number" min="1" max="120" bind:value={workInput} class="pomo-num" />
        </label>
        <label class="flex items-center justify-between text-xs font-medium" style="color: var(--sempa-text-soft);">
          Short break
          <input type="number" min="1" max="60" bind:value={shortBreakInput} class="pomo-num" />
        </label>
        <label class="flex items-center justify-between text-xs font-medium" style="color: var(--sempa-text-soft);">
          Long break
          <input type="number" min="1" max="60" bind:value={longBreakInput} class="pomo-num" />
        </label>
      </div>
      <div class="mt-3 flex gap-2">
        <button onclick={applySettings}
          class="btn-primary flex-1"
          style="height: 34px; font-size: 12.5px;">Apply</button>
        <button onclick={() => (settingsOpen = false)}
          class="btn-surface flex-1"
          style="height: 34px; font-size: 12.5px; color: var(--sempa-text-soft);">Cancel</button>
      </div>
    </div>
  {:else}
    <!-- Task title -->
    {#if pomodoro.taskTitle}
      <p class="mb-1 truncate" style="font-size: 14px; font-weight: 600; color: var(--sempa-text);" title={pomodoro.taskTitle}>
        {pomodoro.taskTitle}
      </p>
    {/if}

    {#if !collapsed}
      <!-- Countdown (focus aid) -->
      <div class="mb-2.5 mt-1 text-center">
        <span class="font-mono tabular-nums" style="font-size: 46px; font-weight: 700; letter-spacing: -.03em; line-height: 1.05; color: var(--sempa-text);">
          {pomodoro.display}
        </span>
      </div>

      <!-- Progress bar -->
      <div class="day-bar-track mb-2.5">
        <div class="day-bar-fill"
          style="width: {pomodoro.progressPct}%; background: {accentColor}; transition-duration: 500ms;"></div>
      </div>
    {/if}

    <!-- Elapsed vs planned — the visible time-blindness signal -->
    <div class="mb-3 flex items-center justify-between type-meta" style="color: var(--sempa-text-dim);">
      <span>
        <span class="font-mono tabular-nums" style="color: var(--sempa-text-soft);">{pomodoro.elapsedDisplay}</span> spent
      </span>
      {#if pomodoro.plannedMinutes}
        <span>
          {#if overBy !== null && overBy > 0}
            <span class="chip" style="height: 20px; padding: 0 8px; background: var(--cal-amber-bg); color: var(--cal-amber-fg);">+{overBy}m over</span>
          {:else}
            of {formatMinutes(pomodoro.plannedMinutes)} planned
          {/if}
        </span>
      {/if}
    </div>

    <!-- Controls -->
    <div class="flex gap-2">
      <button onclick={() => pomodoro.togglePause()}
        class="btn-primary flex-1"
        style="height: 40px; font-size: 14px;">
        {pomodoro.isRunning ? 'Pause' : 'Resume'}
      </button>
      <button onclick={() => pomodoro.finishTask()}
        class="btn-surface"
        style="height: 40px; padding: 0 14px; font-size: 14px; background: var(--sempa-fill); box-shadow: none; color: var(--sempa-text-soft);"
        aria-label="Mark done and log time">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
        </svg>
        Done
      </button>
    </div>

    <!-- Completed count -->
    {#if pomodoro.completedToday > 0}
      <p class="mt-2.5 text-center type-meta" style="color: var(--sempa-text-dim);">
        {pomodoro.completedToday} interval{pomodoro.completedToday !== 1 ? 's' : ''} today
      </p>
    {/if}
  {/if}
</div>

<style>
  /* Floating timer: a raised sheet-like card (shadow-2) above the page. */
  .pomo {
    background: var(--sempa-bg-panel);
    border-radius: var(--radius-2xl, 24px);
    box-shadow: var(--sempa-shadow-2);
  }
  .pomo-icon {
    display: inline-grid; place-items: center;
    width: 30px; height: 30px; border-radius: 999px;
    transition: background-color 120ms ease, color 120ms ease;
  }
  .pomo-icon:hover { background: var(--sempa-fill); color: var(--sempa-text); }
  .pomo-num {
    width: 60px; height: 30px; padding: 0 10px; text-align: right;
    font-size: 12.5px; border-radius: 10px;
    border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);
  }
</style>
