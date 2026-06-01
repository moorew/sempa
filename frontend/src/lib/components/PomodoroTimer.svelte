<script lang="ts">
  import { pomodoro } from '$lib/stores/pomodoro.svelte';

  const accentClass = $derived(
    pomodoro.phase === 'work' ? 'bg-amber-500' : 'bg-green-400'
  );
</script>

<div
  class="fixed bottom-6 right-6 z-50 w-64 rounded-2xl border border-gray-200 bg-white p-4 shadow-2xl
         dark:border-gray-700 dark:bg-gray-800"
>
  <!-- Header row -->
  <div class="mb-1 flex items-center justify-between">
    <span class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
      {pomodoro.phaseLabel}
    </span>
    <button
      onclick={() => pomodoro.stop()}
      class="rounded p-0.5 text-gray-300 hover:text-gray-500 transition-colors"
      aria-label="Stop timer"
    >
      <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
      </svg>
    </button>
  </div>

  <!-- Task title -->
  {#if pomodoro.taskTitle}
    <p class="mb-3 truncate text-sm font-medium text-gray-700" title={pomodoro.taskTitle}>
      {pomodoro.taskTitle}
    </p>
  {/if}

  <!-- Clock -->
  <div class="mb-3 text-center">
    <span class="font-mono text-5xl font-bold tabular-nums text-gray-900 dark:text-gray-50">
      {pomodoro.display}
    </span>
  </div>

  <!-- Progress bar -->
  <div class="mb-3 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
    <div
      class="h-full rounded-full transition-all duration-1000 {accentClass}"
      style="width: {pomodoro.progressPct}%"
    ></div>
  </div>

  <!-- Controls -->
  <button
    onclick={() => pomodoro.togglePause()}
    class="w-full rounded-lg py-2 text-sm font-medium text-white transition-colors
           {pomodoro.phase === 'work' ? 'bg-amber-500 hover:bg-amber-600' : 'bg-green-500 hover:bg-green-600'}"
  >
    {pomodoro.isRunning ? 'Pause' : 'Resume'}
  </button>

  <!-- Completed count -->
  {#if pomodoro.completedToday > 0}
    <p class="mt-2 text-center text-xs text-gray-400">
      {pomodoro.completedToday} pomodoro{pomodoro.completedToday !== 1 ? 's' : ''} today
    </p>
  {/if}
</div>
