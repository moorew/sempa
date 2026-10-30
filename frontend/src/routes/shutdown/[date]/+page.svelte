<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { formatDate, formatMinutes, isToday, today } from '$lib/utils';
  import { Moon, Star, MessageSquare, Sparkles, ArrowLeft, ArrowRight, Check, Plus, ChevronRight } from 'lucide-svelte';
  import { prefs } from '$lib/stores/prefs.svelte';
  import { aiStatus } from '$lib/stores/aiStatus.svelte';

  let date = $derived(page.params.date ?? today());

  let step       = $state<1 | 2 | 3>(1);
  let doneTasks  = $state<Task[]>([]);
  let pendingTasks = $state<Task[]>([]);
  let wins       = $state<string[]>(['']);
  let reflection = $state('');
  let saving     = $state(false);
  let error      = $state<string | null>(null);

  let winInputs: (HTMLInputElement | undefined)[] = $state([]);
  let pendingOpen = $state(false);

  // ── AI: context-aware reflection prompts ────────────────────────────────────
  let aiPrompts = $state<string[]>([]);
  let aiPromptsLoading = $state(false);
  const showAiPrompts = $derived(prefs.aiOn('reflection') && aiStatus.reachable);
  async function suggestPrompts() {
    if (aiPromptsLoading) return;
    aiPromptsLoading = true;
    try {
      const res = await api.ai.reflectionPrompts(
        doneTasks.map(t => t.title),
        pendingTasks.map(t => t.title),
      );
      if (res.available && res.prompts) aiPrompts = res.prompts;
    } catch { /* ignore */ }
    finally { aiPromptsLoading = false; }
  }
  function usePrompt(p: string) {
    reflection = reflection.trim() ? `${reflection.trim()}\n\n${p} ` : `${p} `;
  }

  onMount(async () => {
    try {
      const tasks = await api.tasks.listByDate(date);
      doneTasks    = tasks.filter(t => t.status === 'done');
      pendingTasks = tasks.filter(t => t.status !== 'done' && t.status !== 'cancelled');

      // Load existing plan data if any
      try {
        const plan = await api.plans.get(date);
        if (plan.reflection) reflection = plan.reflection;
        if (plan.wins) {
          const parsed = JSON.parse(plan.wins) as string[];
          wins = parsed.length > 0 ? parsed : [''];
        }
      } catch { /* 404 expected */ }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    }
  });

  function updateWin(i: number, val: string) {
    wins = wins.map((w, idx) => idx === i ? val : w);
  }

  function addWin() {
    wins = [...wins, ''];
    setTimeout(() => winInputs[wins.length - 1]?.focus(), 0);
  }

  function handleWinKeydown(e: KeyboardEvent, i: number) {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (i === wins.length - 1) {
        addWin();
      } else {
        winInputs[i + 1]?.focus();
      }
    }
    if (e.key === 'Backspace' && wins[i] === '' && wins.length > 1) {
      e.preventDefault();
      wins = wins.filter((_, idx) => idx !== i);
      setTimeout(() => winInputs[Math.max(0, i - 1)]?.focus(), 0);
    }
  }

  const totalMinutes = $derived(doneTasks.reduce((s, t) => s + (t.time_actual_minutes ?? t.time_estimate_minutes ?? 0), 0));

  async function finish() {
    saving = true;
    error  = null;
    try {
      const cleanWins = wins.filter(w => w.trim());
      await api.plans.upsert(date, {
        status:     'shutdown_complete',
        reflection: reflection.trim() || null,
        wins:       cleanWins.length > 0 ? JSON.stringify(cleanWins) : null,
        shutdown_at: new Date().toISOString(),
      });
      goto(`/day/${date}`);
    } catch (e) {
      error  = e instanceof Error ? e.message : 'Failed to save';
      saving = false;
    }
  }
</script>

<svelte:head><title>Shutdown {isToday(date) ? 'Today' : date} — Sempa</title></svelte:head>

<div class="flex min-h-full flex-col items-center justify-center px-4 py-10 sm:py-12 animate-page-in">
  <!-- Step indicators: a quiet pill stepper -->
  <ol class="ritual-steps mb-8" aria-label="Shutdown steps">
    {#each ['Review', 'Wins', 'Reflect'] as label, i}
      {@const n = i + 1}
      <li class="ritual-step" data-state={n < step ? 'done' : n === step ? 'current' : 'todo'}
          aria-current={n === step ? 'step' : undefined}>
        <span class="ritual-step-dot">
          {#if n < step}
            <Check size={12} strokeWidth={3.25} />
          {:else}
            {n}
          {/if}
        </span>
        <span class="ritual-step-label">{label}</span>
      </li>
    {/each}
  </ol>

  <div class="w-full max-w-md">
    <!-- ── Step 1: Review ───────────────────────────────────────────────── -->
    {#if step === 1}
      {#key step}
      <div class="ritual-card animate-fade-in">
        <div class="ritual-head">
          <span class="ritual-icon" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
            <Moon size={22} strokeWidth={2} />
          </span>
          <h1 class="type-date" style="color: var(--sempa-text);">
            {doneTasks.length > 0 ? 'Great work today!' : 'Wrapping up…'}
          </h1>
          <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">
            {doneTasks.length} task{doneTasks.length !== 1 ? 's' : ''} completed
            {#if totalMinutes > 0}· {formatMinutes(totalMinutes)} logged{/if}
          </p>
        </div>

        {#if doneTasks.length > 0}
          <div class="mb-4 flex flex-col gap-2">
            {#each doneTasks as t (t.id)}
              <div class="done-row">
                <span class="check-ring" data-state="done" style="width: 20px; height: 20px;" aria-hidden="true">
                  <svg class="h-2.5 w-2.5" fill="none" stroke="currentColor" stroke-width="3.5" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                  </svg>
                </span>
                <span class="min-w-0 flex-1 text-sm font-medium" style="color: var(--sempa-text);">{t.title}</span>
                {#if t.time_actual_minutes}
                  <span class="chip chip-time shrink-0" style="background: var(--sempa-bg-panel);">{formatMinutes(t.time_actual_minutes)}</span>
                {/if}
              </div>
            {/each}
          </div>
        {/if}

        {#if pendingTasks.length > 0}
          <div class="mb-5">
            <button type="button" onclick={() => pendingOpen = !pendingOpen}
                    aria-expanded={pendingOpen}
                    class="pending-toggle">
              <ChevronRight size={14} strokeWidth={2.25} style="transition: transform 150ms ease; transform: rotate({pendingOpen ? 90 : 0}deg);" />
              {pendingTasks.length} task{pendingTasks.length !== 1 ? 's' : ''} not completed
            </button>
            {#if pendingOpen}
              <div class="mt-1 flex flex-col pl-3">
                {#each pendingTasks as t (t.id)}
                  <p class="flex items-center gap-2.5 py-1.5" style="color: var(--sempa-text-soft); font-size: 13px;">
                    <span class="check-ring" style="width: 16px; height: 16px;" aria-hidden="true"></span>
                    {t.title}
                  </p>
                {/each}
              </div>
            {/if}
          </div>
        {/if}

        <div class="flex justify-end">
          <button onclick={() => step = 2} class="btn-primary ritual-cta">
            Continue
            <ArrowRight size={16} strokeWidth={2.25} />
          </button>
        </div>
      </div>
      {/key}

    <!-- ── Step 2: Wins ──────────────────────────────────────────────────── -->
    {:else if step === 2}
      {#key step}
      <div class="ritual-card animate-fade-in">
        <div class="ritual-head">
          <span class="ritual-icon" style="background: var(--cal-amber-bg); color: var(--cal-amber-fg);">
            <Star size={22} strokeWidth={2} />
          </span>
          <h1 class="type-date" style="color: var(--sempa-text);">What went well?</h1>
          <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">Write down your wins — big or small.</p>
        </div>

        <div class="mb-2 flex flex-col gap-2">
          {#each wins as win, i}
            <input
              bind:this={winInputs[i]}
              value={win}
              oninput={(e) => updateWin(i, (e.target as HTMLInputElement).value)}
              onkeydown={(e) => handleWinKeydown(e, i)}
              onfocus={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'}
              onblur={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'transparent'}
              type="text"
              aria-label="Win {i + 1}"
              placeholder="e.g. Fixed the auth bug faster than expected"
              class="ritual-input"
              style="height: 46px; font-size: 14px; border-radius: 14px; padding: 0 14px;"
            />
          {/each}
        </div>

        <button onclick={addWin} class="add-link">
          <Plus size={13} strokeWidth={2.5} />
          Add win (or press Enter)
        </button>

        <div class="mt-6 flex items-center justify-between">
          <button onclick={() => step = 1} class="ritual-back">
            <ArrowLeft size={16} strokeWidth={2.25} />
            Back
          </button>
          <button onclick={() => step = 3} class="btn-primary ritual-cta">
            Continue
            <ArrowRight size={16} strokeWidth={2.25} />
          </button>
        </div>
      </div>
      {/key}

    <!-- ── Step 3: Reflect ───────────────────────────────────────────────── -->
    {:else}
      {#key step}
      <div class="ritual-card animate-fade-in">
        <div class="ritual-head">
          <span class="ritual-icon" style="background: var(--cal-stone-bg); color: var(--cal-stone-fg);">
            <MessageSquare size={22} strokeWidth={2} />
          </span>
          <h1 class="type-date" style="color: var(--sempa-text);">Any reflections?</h1>
          <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">Blockers, learnings, or things to improve tomorrow.</p>
        </div>

        <textarea
          bind:value={reflection}
          rows="4"
          aria-label="Reflection"
          placeholder="Optional — what would you do differently?"
          onfocus={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'}
          onblur={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'transparent'}
          class="ritual-input"
          style="font-size: 14px; line-height: 1.6;"
        ></textarea>

        {#if showAiPrompts}
          <div class="mt-3 text-left">
            {#if aiPrompts.length === 0}
              <button onclick={suggestPrompts} disabled={aiPromptsLoading}
                      class="btn-soft disabled:opacity-50"
                      style="height: 34px; padding: 0 14px; font-size: 12.5px;"
                      title="Suggest reflection questions from today">
                <Sparkles size={13} strokeWidth={2} />
                {aiPromptsLoading ? 'Thinking…' : 'Suggest prompts'}
              </button>
            {:else}
              <p class="mb-1.5 type-label" style="color: var(--sempa-text-soft);">Tap a prompt to add it</p>
              <div class="flex flex-col gap-1.5">
                {#each aiPrompts as p}
                  <button onclick={() => usePrompt(p)} class="prompt-chip">
                    {p}
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        {/if}

        {#if error}<p class="mt-2 text-xs text-red-500">{error}</p>{/if}

        <div class="mt-6 flex items-center justify-between">
          <button onclick={() => step = 2} class="ritual-back">
            <ArrowLeft size={16} strokeWidth={2.25} />
            Back
          </button>
          <button onclick={finish} disabled={saving} class="btn-primary ritual-cta disabled:opacity-50">
            {saving ? 'Saving…' : 'Finish day'}
            <Check size={16} strokeWidth={2.5} />
          </button>
        </div>
      </div>
      {/key}
    {/if}
  </div>
</div>

<style>
  /* ── Ritual shell (shared look with the plan-day ritual) ───────────────── */
  .ritual-steps { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; justify-content: center; list-style: none; padding: 0; margin-top: 0; }
  .ritual-step {
    display: inline-flex; align-items: center; gap: 7px;
    height: 34px; padding: 0 14px 0 5px; border-radius: 999px;
    font-size: 13px; font-weight: 600; color: var(--sempa-text-soft);
  }
  .ritual-step-dot {
    display: inline-grid; place-items: center; width: 24px; height: 24px; border-radius: 999px;
    font-size: 12px; font-weight: 700; background: var(--sempa-track); color: var(--sempa-text-soft);
  }
  .ritual-step[data-state="current"] { background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text); font-weight: 700; }
  .ritual-step[data-state="current"] .ritual-step-dot { background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); }
  .ritual-step[data-state="done"] { color: var(--sempa-success); }
  .ritual-step[data-state="done"] .ritual-step-dot { background: var(--sempa-success); color: var(--sempa-on-accent); }
  @media (max-width: 400px) {
    .ritual-step { padding-right: 11px; font-size: 12.5px; }
  }

  .ritual-card {
    background: var(--sempa-bg-panel);
    border-radius: var(--radius-sheet);
    box-shadow: var(--sempa-shadow-1);
    padding: 32px 28px 24px;
  }
  @media (max-width: 480px) { .ritual-card { padding: 26px 20px 20px; } }

  .ritual-head { display: flex; flex-direction: column; align-items: center; text-align: center; margin-bottom: 22px; }
  .ritual-icon {
    display: grid; place-items: center; width: 52px; height: 52px; border-radius: 16px;
    margin-bottom: 14px;
  }

  .ritual-input {
    display: block; width: 100%; resize: none; outline: none;
    background: var(--sempa-fill); border: 1px solid transparent; border-radius: 16px;
    padding: 14px 16px; color: var(--sempa-text);
    transition: border-color 120ms ease;
  }

  .ritual-cta { height: 44px; padding: 0 22px; font-size: 14px; }
  .ritual-back {
    display: inline-flex; align-items: center; gap: 6px;
    height: 44px; padding: 0 16px 0 12px; border-radius: 999px;
    font-size: 14px; font-weight: 600; color: var(--sempa-text-soft);
    transition: background-color 120ms ease, color 120ms ease;
  }
  .ritual-back:hover { background: var(--sempa-fill); color: var(--sempa-text); }

  /* Completed task: a soft success block. */
  .done-row {
    display: flex; align-items: center; gap: 12px;
    min-height: 48px; padding: 9px 14px; border-radius: 16px;
    background: var(--sempa-success-bg);
  }

  .pending-toggle {
    display: inline-flex; align-items: center; gap: 6px;
    height: 32px; padding: 0 12px 0 8px; border-radius: 999px;
    font-size: 13px; font-weight: 600; color: var(--sempa-text-soft);
    transition: background-color 120ms ease, color 120ms ease;
  }
  .pending-toggle:hover { background: var(--sempa-fill); color: var(--sempa-text); }

  .add-link {
    display: inline-flex; align-items: center; gap: 5px;
    height: 32px; padding: 0 12px 0 8px; border-radius: 999px;
    font-size: 12.5px; font-weight: 600; color: var(--sempa-accent-ink);
  }
  .add-link:hover { background: var(--sempa-accent-bg); }

  .prompt-chip {
    text-align: left; padding: 10px 14px; border-radius: 14px;
    font-size: 13px; line-height: 1.45; color: var(--sempa-text-soft);
    background: var(--sempa-fill);
    transition: background-color 120ms ease, color 120ms ease;
  }
  .prompt-chip:hover { background: var(--sempa-accent-bg); color: var(--sempa-accent-ink); }
</style>
