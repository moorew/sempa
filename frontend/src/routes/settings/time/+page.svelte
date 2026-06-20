<script lang="ts">
  import { onMount } from 'svelte';
  import { timeTracking } from '$lib/stores/timeTracking.svelte';
  import { timeCapture } from '$lib/stores/timeCapture.svelte';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { ACTIVITY_BUCKETS } from '$lib/activityBuckets';
  import { Clock, Play } from 'lucide-svelte';

  // Visiting settings clears any "paused after repeated skips" state, so toggling
  // here always takes effect immediately.
  onMount(() => timeCapture.resume());

  // Pomodoro durations (mirror of the widget's settings, editable here too).
  let workMins = $state(pomodoro.workMins);
  let shortMins = $state(pomodoro.shortBreakMins);
  let longMins = $state(pomodoro.longBreakMins);
  function savePomo() {
    pomodoro.setPrefs(
      Math.max(1, Math.min(120, workMins || 25)),
      Math.max(1, Math.min(60, shortMins || 5)),
      Math.max(1, Math.min(60, longMins || 15)),
    );
  }
</script>

<svelte:head><title>Time tracking — Sempa</title></svelte:head>

<div class="mx-auto flex h-full max-w-xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <!-- Header -->
  <div class="flex items-center gap-3 px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <a href="/settings/accounts"
       class="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm transition-colors"
       style="color: var(--sempa-accent);">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" d="M19 12H5m7-7-7 7 7 7"/>
      </svg>
      Settings
    </a>
    <h1 class="text-base font-semibold" style="color: var(--sempa-text);">Time tracking</h1>
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-6 pb-20">
    {#snippet sectionLabel(text: string)}
      <p class="mb-3 mt-7 first:mt-0" style="font-family:monospace; font-size:10.5px; font-weight:700;
         letter-spacing:0.12em; text-transform:uppercase; color:var(--sempa-text-dim)">{text}</p>
    {/snippet}

    {#snippet toggleRow(label: string, desc: string, value: boolean, onChange: () => void)}
      <button onclick={onChange}
        class="flex w-full items-center gap-3 rounded-xl px-4 py-3.5 text-left transition-colors"
        style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium" style="color: var(--sempa-text);">{label}</p>
          <p class="mt-0.5 text-xs" style="color: var(--sempa-text-dim);">{desc}</p>
        </div>
        <span class="relative h-6 w-10 shrink-0 rounded-full transition-colors"
          style="background: {value ? 'var(--sempa-accent)' : 'var(--sempa-border)'};">
          <span class="absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all"
            style="left: {value ? '1.25rem' : '0.125rem'};"></span>
        </span>
      </button>
    {/snippet}

    <!-- Completion capture -->
    {@render sectionLabel('Completion capture')}
    <div class="flex flex-col gap-2">
      {@render toggleRow(
        'Ask how long a task took',
        'When you complete a task without the focus timer, pop a quick prompt to log the time. This builds your planned-vs-actual profile.',
        timeTracking.promptOnComplete,
        timeTracking.togglePromptOnComplete,
      )}
      {#if timeTracking.promptOnComplete}
        {@render toggleRow(
          'Skip very quick tasks',
          'Don’t prompt for tasks estimated at 5 minutes or less.',
          timeTracking.skipQuick,
          timeTracking.toggleSkipQuick,
        )}
      {/if}
    </div>

    <!-- Day capacity -->
    {@render sectionLabel('Day capacity')}
    <div class="flex flex-col gap-2">
      {@render toggleRow(
        'Warn when a day is overloaded',
        'A subtle hint appears when a day’s planned time runs over your limit — when you set a task’s time, and on the day view. No pop-ups.',
        timeTracking.capacityEnabled,
        timeTracking.toggleCapacityEnabled,
      )}
      {#if timeTracking.capacityEnabled}
        <div class="flex items-center justify-between rounded-xl px-4 py-3.5"
          style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium" style="color: var(--sempa-text);">Hours per day</p>
            <p class="mt-0.5 text-xs" style="color: var(--sempa-text-dim);">How much focused work fits in a realistic day.</p>
          </div>
          <input type="number" min="0.5" max="24" step="0.5"
            value={(timeTracking.capacityMinutes / 60).toString()}
            onchange={(e) => timeTracking.setCapacityMinutes(Math.round((+(e.currentTarget as HTMLInputElement).value || 6) * 60))}
            class="w-16 rounded px-2 py-1 text-right text-sm"
            style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
        </div>
        {@render toggleRow(
          'Judge by realistic time',
          'Compare against how long tasks actually take you (your history multiplier), not just your estimates. With no data yet this matches your estimates.',
          timeTracking.capacityRealistic,
          timeTracking.toggleCapacityRealistic,
        )}
      {/if}
    </div>

    <!-- Default times per activity -->
    {@render sectionLabel('Default times by activity')}
    <p class="-mt-1 mb-3 text-xs" style="color: var(--sempa-text-dim);">
      Sempa auto-detects the kind of work from a task’s title and pre-fills these
      durations. Adjust any that don’t match how you work.
    </p>
    <div class="overflow-hidden rounded-xl" style="border: 1px solid var(--sempa-border);">
      {#each ACTIVITY_BUCKETS.filter(b => b.key !== 'other') as b, i}
        <div class="flex items-center gap-3 px-4 py-2.5"
          style="background: var(--sempa-bg-panel); {i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}">
          <span class="text-lg">{b.emoji}</span>
          <span class="flex-1 text-sm" style="color: var(--sempa-text);">{b.label}</span>
          <input type="number" min="1" max="480" value={timeTracking.defaultMinutesFor(b.key)}
            onchange={(e) => timeTracking.setBucketMinutes(b.key, Math.max(1, Math.min(480, +(e.currentTarget as HTMLInputElement).value || b.defaultMinutes)))}
            class="w-16 rounded px-2 py-1 text-right text-sm"
            style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
          <span class="text-xs" style="color: var(--sempa-text-dim);">min</span>
        </div>
      {/each}
    </div>

    <!-- Pomodoro durations -->
    {@render sectionLabel('Focus timer')}
    <div class="flex flex-col gap-3 rounded-xl px-4 py-4"
      style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
      <label class="flex items-center justify-between text-sm" style="color: var(--sempa-text-soft);">
        Focus length
        <input type="number" min="1" max="120" bind:value={workMins} onchange={savePomo}
          class="w-16 rounded px-2 py-1 text-right text-sm"
          style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
      </label>
      <label class="flex items-center justify-between text-sm" style="color: var(--sempa-text-soft);">
        Short break
        <input type="number" min="1" max="60" bind:value={shortMins} onchange={savePomo}
          class="w-16 rounded px-2 py-1 text-right text-sm"
          style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
      </label>
      <label class="flex items-center justify-between text-sm" style="color: var(--sempa-text-soft);">
        Long break
        <input type="number" min="1" max="60" bind:value={longMins} onchange={savePomo}
          class="w-16 rounded px-2 py-1 text-right text-sm"
          style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
      </label>
    </div>

    <!-- Walkthrough -->
    {@render sectionLabel('Help')}
    <button onclick={() => timeTracking.openWalkthrough()}
      class="flex w-full items-center gap-3 rounded-xl px-4 py-3.5 text-left transition-colors"
      style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
      <Clock size={18} style="color: var(--sempa-accent);" />
      <div class="flex-1">
        <p class="text-sm font-medium" style="color: var(--sempa-text);">Replay the intro</p>
        <p class="mt-0.5 text-xs" style="color: var(--sempa-text-dim);">Watch the time-tracking walkthrough again.</p>
      </div>
      <Play size={16} style="color: var(--sempa-text-dim);" />
    </button>
  </div>
</div>
