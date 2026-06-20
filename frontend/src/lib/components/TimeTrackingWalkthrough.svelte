<script lang="ts">
  import { page } from '$app/stores';
  import { timeTracking } from '$lib/stores/timeTracking.svelte';

  // A short, dismissible intro to time tracking. Opens once on the first app
  // page a new user sees, and is replayable from Settings → Time tracking.
  const STEPS = [
    {
      emoji: '⏱️',
      title: 'Focus, and track honestly',
      body: 'Hit Focus on any task and the timer follows you everywhere, showing time spent vs. your estimate. When you finish, you confirm how long it really took — so a wandering mind never corrupts your data.',
    },
    {
      emoji: '✅',
      title: 'Nothing slips through',
      body: 'Completed something without the timer? Sempa quietly asks “how long did that take?” — even the small tasks you’d normally forget become useful data. One tap to log, or skip.',
    },
    {
      emoji: '📈',
      title: 'It learns your timing',
      body: 'Over time Sempa builds your planned-vs-actual profile (e.g. “email runs 1.8× longer for you”), nudges your estimates toward reality, and the local AI predicts how long new tasks will take.',
    },
    {
      emoji: '⚙️',
      title: 'Make it yours',
      body: 'Turn the prompts on or off, set default times per activity, and replay this intro anytime from Settings → Time tracking.',
    },
  ];

  let step = $state(0);
  let triggered = false;

  // First-run trigger: open once on a real app page (not login/setup).
  $effect(() => {
    const path = $page.url.pathname;
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
    <div class="w-full max-w-md rounded-2xl p-6 shadow-2xl"
      style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: var(--sempa-text);">

      <div class="mb-3 flex items-start justify-between">
        <span class="text-xs font-semibold uppercase tracking-wider" style="color: var(--sempa-accent);">
          Time tracking
        </span>
        <button onclick={finish} aria-label="Dismiss"
          class="rounded p-1 transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div class="mb-1 text-4xl">{current.emoji}</div>
      <h2 class="mb-2 text-lg font-bold" style="color: var(--sempa-text);">{current.title}</h2>
      <p class="text-sm leading-relaxed" style="color: var(--sempa-text-soft);">{current.body}</p>

      <!-- Step dots -->
      <div class="mt-5 flex items-center justify-center gap-1.5">
        {#each STEPS as _, i}
          <span class="h-1.5 rounded-full transition-all"
            style="width: {i === step ? '18px' : '6px'}; background: {i === step ? 'var(--sempa-accent)' : 'var(--sempa-border)'};"></span>
        {/each}
      </div>

      <div class="mt-5 flex items-center justify-between">
        <button onclick={back} disabled={step === 0}
          class="rounded-lg px-3 py-2 text-sm transition-opacity disabled:opacity-0"
          style="color: var(--sempa-text-soft);">Back</button>
        <div class="flex items-center gap-2">
          {#if !isLast}
            <button onclick={finish} class="rounded-lg px-3 py-2 text-sm transition-opacity hover:opacity-70"
              style="color: var(--sempa-text-dim);">Skip</button>
          {/if}
          <button onclick={next}
            class="rounded-xl px-5 py-2 text-sm font-semibold text-white transition-opacity hover:opacity-90"
            style="background: var(--sempa-accent);">
            {isLast ? 'Got it' : 'Next'}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
