<script lang="ts">
  import { page } from '$app/state';
  import { timeTracking } from '$lib/stores/timeTracking.svelte';
  import { Timer, CircleCheck, TrendingUp, SlidersHorizontal } from 'lucide-svelte';

  // A short, dismissible intro to time tracking. Opens once on the first app
  // page a new user sees, and is replayable from Settings → Time tracking.
  const STEPS = [
    {
      icon: Timer, tint: 'accent',
      title: 'Focus, and track honestly',
      body: 'Hit Focus on any task and the timer follows you everywhere, showing time spent vs. your estimate. When you finish, you confirm how long it really took — so a wandering mind never corrupts your data.',
    },
    {
      icon: CircleCheck, tint: 'success',
      title: 'Nothing slips through',
      body: 'Completed something without the timer? Sempa quietly asks “how long did that take?” — even the small tasks you’d normally forget become useful data. One tap to log, or skip.',
    },
    {
      icon: TrendingUp, tint: 'amber',
      title: 'It learns your timing',
      body: 'Over time Sempa builds your planned-vs-actual profile (e.g. “email runs 1.8× longer for you”), nudges your estimates toward reality, and the local AI predicts how long new tasks will take.',
    },
    {
      icon: SlidersHorizontal, tint: 'stone',
      title: 'Make it yours',
      body: 'Turn the prompts on or off, set default times per activity, and replay this intro anytime from Settings → Time tracking.',
    },
  ];

  let step = $state(0);
  let triggered = false;

  // First-run trigger: open once on a real app page (not login/setup). Wait for
  // the store to finish loading from localStorage, or a mount-order race makes
  // `walkthroughSeen` read its default (false) and the intro reappears every launch.
  $effect(() => {
    if (!timeTracking.initialized) return;
    const path = page.url.pathname;
    const appPage = !path.startsWith('/login') && !path.startsWith('/setup');
    if (appPage && !timeTracking.walkthroughSeen && !triggered) {
      triggered = true;
      timeTracking.openWalkthrough();
    }
  });

  const open = $derived(timeTracking.walkthroughOpen);
  const current = $derived(STEPS[step]);
  const isLast = $derived(step === STEPS.length - 1);

  function next() { if (isLast) finish(); else step++; }
  function back() { if (step > 0) step--; }
  function finish() { step = 0; timeTracking.dismissWalkthrough(); }
</script>

{#if open}
  <div class="fixed inset-0 z-[70] flex items-end justify-center p-4 sm:items-center"
    style="background: rgba(0,0,0,0.5);" role="dialog" aria-modal="true">
    <div class="surface-raised w-full max-w-md p-6" style="color: var(--sempa-text);">

      <div class="mb-4 flex items-start justify-between">
        <span class="chip" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
          Time tracking · {step + 1} of {STEPS.length}
        </span>
        <button onclick={finish} aria-label="Dismiss"
          class="-mr-1.5 -mt-1.5 flex h-9 w-9 items-center justify-center rounded-full transition-colors hover:bg-[var(--sempa-fill)]"
          style="color: var(--sempa-text-dim);">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <span class="wt-icon"
            style={current.tint === 'success'
              ? 'background: var(--sempa-success-bg); color: var(--sempa-success);'
              : current.tint === 'amber'
                ? 'background: var(--cal-amber-bg); color: var(--cal-amber-fg);'
                : current.tint === 'stone'
                  ? 'background: var(--cal-stone-bg); color: var(--cal-stone-fg);'
                  : 'background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);'}>
        <current.icon size={24} strokeWidth={2} />
      </span>
      <h2 class="mb-2 type-date" style="font-size: 20px; color: var(--sempa-text);">{current.title}</h2>
      <p class="text-sm leading-relaxed" style="color: var(--sempa-text-soft);">{current.body}</p>

      <!-- Step dots -->
      <div class="mt-6 flex items-center justify-center gap-1.5">
        {#each STEPS as _, i}
          <span class="h-1.5 rounded-full transition-all"
            style="width: {i === step ? '20px' : '6px'}; background: {i === step ? 'var(--sempa-accent)' : 'var(--sempa-track)'};"></span>
        {/each}
      </div>

      <div class="mt-5 flex items-center justify-between">
        <button onclick={back} disabled={step === 0}
          class="wt-ghost disabled:opacity-0"
          style="color: var(--sempa-text-soft);">Back</button>
        <div class="flex items-center gap-1.5">
          {#if !isLast}
            <button onclick={finish} class="wt-ghost"
              style="color: var(--sempa-text-dim);">Skip</button>
          {/if}
          <button onclick={next}
            class="btn-primary"
            style="height: 42px; padding: 0 22px; font-size: 14px;">
            {isLast ? 'Got it' : 'Next'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .wt-icon {
    display: grid; place-items: center;
    width: 52px; height: 52px; border-radius: 16px; margin-bottom: 14px;
  }
  .wt-ghost {
    height: 42px; padding: 0 14px; border-radius: 999px;
    font-size: 14px; font-weight: 600;
    transition: background-color 120ms ease, opacity 120ms ease;
  }
  .wt-ghost:hover { background: var(--sempa-fill); }
</style>
