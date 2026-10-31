<script lang="ts">
  import { onMount } from 'svelte';
  import { timeTracking } from '$lib/stores/timeTracking.svelte';
  import { timeCapture } from '$lib/stores/timeCapture.svelte';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { ACTIVITY_BUCKETS, bucketByKey } from '$lib/activityBuckets';
  import { timeProfile, LEARN_MIN_SAMPLES } from '$lib/stores/timeProfile.svelte';
  import { formatMinutes } from '$lib/utils';
  import { Clock, Play, Gauge } from 'lucide-svelte';

  // Visiting settings clears any "paused after repeated skips" state, so toggling
  // here always takes effect immediately.
  onMount(() => {
    timeCapture.resume();
    void timeProfile.load();
  });

  // Split so the settled buckets read as an achievement and the rest as progress.
  const learnedBuckets  = $derived(timeProfile.all.filter((b) => b.learned));
  const learningBuckets = $derived(timeProfile.all.filter((b) => !b.learned));

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

<div class="mx-auto flex h-full max-w-[640px] flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <!-- Header -->
  <div class="px-5 pt-6 pb-2 sm:px-8 sm:pt-8">
    <a href="/settings/accounts?section=tasks" class="set-back">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round" d="m15 18-6-6 6-6"/>
      </svg>
      Settings
    </a>
    <h1 class="set-title type-page">Time tracking</h1>
  </div>

  <div class="flex-1 overflow-y-auto px-5 pt-4 pb-20 sm:px-8">
    {#snippet sectionLabel(text: string)}
      <h2 class="set-heading">{text}</h2>
    {/snippet}

    {#snippet toggleRow(label: string, desc: string, value: boolean, onChange: () => void)}
      <button onclick={onChange} role="switch" aria-checked={value}
        class="set-row set-row-link">
        <span class="set-row-main">
          <span class="set-label block">{label}</span>
          <span class="set-desc block">{desc}</span>
        </span>
        <span class="set-switch" data-on={value}></span>
      </button>
    {/snippet}

    <!-- Completion capture -->
    {@render sectionLabel('Completion capture')}
    <div class="set-group">
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

    <!-- What Sempa has learned -->
    {@render sectionLabel('Learned times')}
    <div class="set-group">
      <div class="set-block">
        <p class="set-desc" style="margin-top: 0;">
          After you’ve logged a kind of task {LEARN_MIN_SAMPLES} times and the durations agree,
          Sempa stops asking and fills the time in for you. Kinds of work that vary a
          lot keep asking — an average of 10 minutes and 4 hours isn’t worth guessing.
        </p>

        {#if learnedBuckets.length === 0 && learningBuckets.length === 0}
          <p class="set-meta mt-3">
            Nothing logged yet. Answer a few completion prompts and they’ll show up here.
          </p>
        {/if}
      </div>

      {#each learnedBuckets as b (b.key)}
        {@const meta = bucketByKey(b.key)}
        <div class="set-row set-row-compact">
          <span class="text-base leading-none">{meta.emoji}</span>
          <div class="set-row-main">
            <p class="set-label truncate">
              {meta.label} · <span class="font-mono" style="font-size: 13px;">{formatMinutes(b.median)}</span>
            </p>
            <p class="set-desc">
              Learned from {b.samples} logged {b.samples === 1 ? 'task' : 'tasks'}
            </p>
          </div>
          <button onclick={() => timeProfile.forget(b.key)}
                  class="set-btn-quiet set-btn-sm">
            Ask again
          </button>
        </div>
      {/each}

      {#each learningBuckets as b (b.key)}
        {@const meta = bucketByKey(b.key)}
        <div class="set-row set-row-compact">
          <span class="text-base leading-none opacity-50">{meta.emoji}</span>
          <div class="set-row-main">
            <p class="set-label truncate" style="color: var(--sempa-text-soft);">{meta.label}</p>
            <p class="set-desc">
              {#if timeProfile.isForgotten(b.key)}
                Re-teaching — will stop asking once it’s consistent again
              {:else if b.samples < LEARN_MIN_SAMPLES}
                Still learning · {b.samples} of {LEARN_MIN_SAMPLES}
              {:else}
                Too variable to predict · still asking
              {/if}
            </p>
          </div>
          {#if timeProfile.isForgotten(b.key)}
            <button onclick={() => timeProfile.relearn(b.key)}
                    class="set-btn-quiet set-btn-sm">
              Undo
            </button>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Day capacity -->
    {@render sectionLabel('Day capacity')}
    <div class="set-group">
      {@render toggleRow(
        'Warn when a day is overloaded',
        'A subtle hint appears when a day’s planned time runs over your limit — when you set a task’s time, and on the day view. No pop-ups.',
        timeTracking.capacityEnabled,
        timeTracking.toggleCapacityEnabled,
      )}
      {#if timeTracking.capacityEnabled}
        <div class="set-row">
          <div class="set-row-main">
            <p class="set-label">Hours per day</p>
            <p class="set-desc">How much focused work fits in a realistic day.</p>
          </div>
          <input type="number" min="0.5" max="24" step="0.5"
            value={(timeTracking.capacityMinutes / 60).toString()}
            onchange={(e) => timeTracking.setCapacityMinutes(Math.round((+(e.currentTarget as HTMLInputElement).value || 6) * 60))}
            aria-label="Hours per day"
            class="set-input set-input-num" />
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
    <p class="set-desc -mt-1 mb-3 px-1.5">
      Sempa auto-detects the kind of work from a task’s title and pre-fills these
      durations. Adjust any that don’t match how you work.
    </p>
    <div class="set-group">
      {#each ACTIVITY_BUCKETS.filter(b => b.key !== 'other') as b}
        <div class="set-row set-row-compact">
          <span class="text-lg leading-none">{b.emoji}</span>
          <span class="set-row-main set-label">{b.label}</span>
          <input type="number" min="1" max="480" value={timeTracking.defaultMinutesFor(b.key)}
            onchange={(e) => timeTracking.setBucketMinutes(b.key, Math.max(1, Math.min(480, +(e.currentTarget as HTMLInputElement).value || b.defaultMinutes)))}
            aria-label="{b.label} minutes"
            class="set-input set-input-num" />
          <span class="set-meta w-7">min</span>
        </div>
      {/each}
    </div>

    <!-- Pomodoro durations -->
    {@render sectionLabel('Focus timer')}
    <div class="set-group">
      <label class="set-row set-row-compact">
        <span class="set-row-main set-label">Focus length</span>
        <input type="number" min="1" max="120" bind:value={workMins} onchange={savePomo}
          class="set-input set-input-num" />
        <span class="set-meta w-7">min</span>
      </label>
      <label class="set-row set-row-compact">
        <span class="set-row-main set-label">Short break</span>
        <input type="number" min="1" max="60" bind:value={shortMins} onchange={savePomo}
          class="set-input set-input-num" />
        <span class="set-meta w-7">min</span>
      </label>
      <label class="set-row set-row-compact">
        <span class="set-row-main set-label">Long break</span>
        <input type="number" min="1" max="60" bind:value={longMins} onchange={savePomo}
          class="set-input set-input-num" />
        <span class="set-meta w-7">min</span>
      </label>
    </div>

    <!-- Insights + walkthrough -->
    {@render sectionLabel('Help')}
    <div class="set-group">
      <a href="/insights" class="set-row set-row-link">
        <span class="set-icon"><Gauge size={18} /></span>
        <span class="set-row-main">
          <span class="set-label block">View your time insights</span>
          <span class="set-desc block">See your planned-vs-actual patterns by activity and tag.</span>
        </span>
        <svg class="set-chevron h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" aria-hidden="true">
          <path stroke-linecap="round" d="m9 18 6-6-6-6"/>
        </svg>
      </a>
      <button onclick={() => timeTracking.openWalkthrough()} class="set-row set-row-link">
        <span class="set-icon"><Clock size={18} /></span>
        <span class="set-row-main">
          <span class="set-label block">Replay the intro</span>
          <span class="set-desc block">Watch the time-tracking walkthrough again.</span>
        </span>
        <span class="set-chevron"><Play size={16} /></span>
      </button>
    </div>
  </div>
</div>
