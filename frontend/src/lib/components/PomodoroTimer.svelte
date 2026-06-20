<script lang="ts">
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { formatMinutes } from '$lib/utils';

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
  class="fixed bottom-20 right-4 z-50 w-[15.5rem] rounded-2xl p-4 shadow-2xl sm:bottom-6 sm:right-6"
  style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: var(--sempa-text);"
>
  <!-- Header row -->
  <div class="mb-1 flex items-center justify-between">
    <span class="text-xs font-semibold uppercase tracking-wider" style="color: {accentColor};">
      {pomodoro.phaseLabel}
    </span>
    <div class="flex items-center gap-0.5" style="color: var(--sempa-text-dim);">
      <button onclick={() => (collapsed = !collapsed)}
        class="rounded p-1 transition-opacity hover:opacity-70"
        aria-label={collapsed ? 'Expand timer' : 'Collapse timer'}>
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d={collapsed ? 'M5 15l7-7 7 7' : 'M19 9l-7 7-7-7'} />
        </svg>
      </button>
      <button onclick={openSettings}
        class="rounded p-1 transition-opacity hover:opacity-70" aria-label="Timer settings">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </button>
      <button onclick={() => pomodoro.requestStop()}
        class="rounded p-1 transition-opacity hover:opacity-70" aria-label="Stop and log time">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  </div>

  {#if settingsOpen}
    <!-- Settings panel -->
    <div class="mb-1 rounded-xl p-3" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg);">
      <p class="mb-2 text-[10.5px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">
        Durations (min)
      </p>
      <div class="flex flex-col gap-2">
        <label class="flex items-center justify-between text-xs" style="color: var(--sempa-text-soft);">
          Focus
          <input type="number" min="1" max="120" bind:value={workInput}
            class="w-14 rounded px-2 py-1 text-right text-xs"
            style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
        </label>
        <label class="flex items-center justify-between text-xs" style="color: var(--sempa-text-soft);">
          Short break
          <input type="number" min="1" max="60" bind:value={shortBreakInput}
            class="w-14 rounded px-2 py-1 text-right text-xs"
            style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
        </label>
        <label class="flex items-center justify-between text-xs" style="color: var(--sempa-text-soft);">
          Long break
          <input type="number" min="1" max="60" bind:value={longBreakInput}
            class="w-14 rounded px-2 py-1 text-right text-xs"
            style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel); color: var(--sempa-text);" />
        </label>
      </div>
      <div class="mt-2.5 flex gap-2">
        <button onclick={applySettings}
          class="flex-1 rounded-lg py-1.5 text-xs font-medium text-white transition-opacity hover:opacity-90"
          style="background: var(--sempa-accent);">Apply</button>
        <button onclick={() => (settingsOpen = false)}
          class="flex-1 rounded-lg py-1.5 text-xs transition-opacity hover:opacity-80"
          style="background: var(--sempa-bg); color: var(--sempa-text-soft);">Cancel</button>
      </div>
    </div>
  {:else}
    <!-- Task title -->
    {#if pomodoro.taskTitle}
      <p class="mb-2 truncate text-sm font-medium" style="color: var(--sempa-text);" title={pomodoro.taskTitle}>
        {pomodoro.taskTitle}
      </p>
    {/if}

    {#if !collapsed}
      <!-- Countdown (focus aid) -->
      <div class="mb-2 text-center">
        <span class="font-mono text-5xl font-bold tabular-nums" style="color: var(--sempa-text);">
          {pomodoro.display}
        </span>
      </div>

      <!-- Progress bar -->
      <div class="mb-2.5 h-1.5 overflow-hidden rounded-full" style="background: var(--sempa-border);">
        <div class="h-full rounded-full transition-all duration-500"
          style="width: {pomodoro.progressPct}%; background: {accentColor};"></div>
      </div>
    {/if}

    <!-- Elapsed vs planned — the visible time-blindness signal -->
    <div class="mb-3 flex items-center justify-between text-xs" style="color: var(--sempa-text-dim);">
      <span>
        <span class="font-mono tabular-nums" style="color: var(--sempa-text-soft);">{pomodoro.elapsedDisplay}</span> spent
      </span>
      {#if pomodoro.plannedMinutes}
        <span>
          {#if overBy !== null && overBy > 0}
            <span class="font-semibold" style="color: #c0392b;">+{overBy}m over</span>
          {:else}
            of {formatMinutes(pomodoro.plannedMinutes)} planned
          {/if}
        </span>
      {/if}
    </div>

    <!-- Controls -->
    <div class="flex gap-2">
      <button onclick={() => pomodoro.togglePause()}
        class="flex-1 rounded-lg py-2 text-sm font-medium text-white transition-opacity hover:opacity-90"
        style="background: {accentColor};">
        {pomodoro.isRunning ? 'Pause' : 'Resume'}
      </button>
      <button onclick={() => pomodoro.finishTask()}
        class="flex items-center justify-center gap-1.5 rounded-lg px-3 py-2 text-sm font-medium transition-opacity hover:opacity-80"
        style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);"
        aria-label="Mark done and log time">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
        </svg>
        Done
      </button>
    </div>

    <!-- Completed count -->
    {#if pomodoro.completedToday > 0}
      <p class="mt-2 text-center text-xs" style="color: var(--sempa-text-dim);">
        {pomodoro.completedToday} interval{pomodoro.completedToday !== 1 ? 's' : ''} today
      </p>
    {/if}
  {/if}
</div>
