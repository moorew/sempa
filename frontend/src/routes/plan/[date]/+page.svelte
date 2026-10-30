<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import type { Task } from '$lib/types';
  import { compareTasksForDay, formatDate, formatMinutes, isToday, offsetDate, today, weekStart } from '$lib/utils';
  import { Sparkles, Sunrise, CornerUpLeft, CircleCheck, ArrowLeft, ArrowRight, Check, Quote } from 'lucide-svelte';
  import { prefs } from '$lib/stores/prefs.svelte';
  import { aiStatus } from '$lib/stores/aiStatus.svelte';

  let date       = $derived(page.params.date ?? today());
  let yesterday  = $derived(offsetDate(date, -1));

  let step                  = $state<1 | 2 | 3>(1);
  let intention             = $state('');
  let carryover             = $state<Task[]>([]);
  let todayTasks            = $state<Task[]>([]);
  let selected              = $state(new Set<string>());
  let saving                = $state(false);
  let error                 = $state<string | null>(null);
  let intentionEl: HTMLTextAreaElement | undefined = $state();
  let yesterdayReflection   = $state<string | null>(null);

  let totalEstimate = $derived(
    todayTasks.reduce((sum, t) => sum + (t.time_estimate_minutes ?? 0), 0)
  );

  onMount(async () => {
    try {
      // Load existing plan if any + yesterday's reflection
      try {
        const plan = await api.plans.get(date);
        if (plan.intention) intention = plan.intention;
      } catch { /* 404 is expected */ }
      try {
        const yPlan = await api.plans.get(yesterday);
        if (yPlan.reflection) yesterdayReflection = yPlan.reflection;
      } catch { /* ignore */ }

      const [yTasks, dTasks] = await Promise.all([
        api.tasks.listByDate(yesterday),
        api.tasks.listByDate(date),
      ]);

      carryover  = yTasks.filter(t => t.status !== 'done' && t.status !== 'cancelled');
      todayTasks = dTasks;
      selected   = new Set(carryover.map(t => t.id)); // pre-select all
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    }

    setTimeout(() => intentionEl?.focus(), 100);
  });

  function toggleSelect(id: string) {
    const s = new Set(selected);
    s.has(id) ? s.delete(id) : s.add(id);
    selected = s;
  }

  // ── AI: suggest an order for today's tasks (around calendar events) ──────────
  let aiPlanning = $state(false);
  let aiPlanNote = $state('');
  let aiPlanError = $state('');
  let aiPlanned  = $state(false); // true once an order has been applied (shows the list)
  const showAiPlan = $derived(prefs.aiOn('planDay') && aiStatus.reachable);
  // The active tasks in their current (AI-applied or natural) order — rendered in
  // step 3 so the user can actually SEE what "plan my day" did.
  let planActive = $derived(todayTasks.filter(t => t.status !== 'done' && t.status !== 'cancelled'));
  async function aiPlanMyDay() {
    if (aiPlanning) return;
    // Imported calendar meetings (Google Calendar) ride along as tasks; they're
    // fixed commitments, not work to reorder — feed them to the planner as events.
    const active = todayTasks.filter(
      t => t.status !== 'done' && t.status !== 'cancelled' && t.source !== 'google_calendar',
    );
    if (active.length < 2) return;
    aiPlanning = true;
    aiPlanError = '';
    try {
      // Calendar awareness: subscription feeds (iCal + Fastmail) via the events
      // endpoint, plus Google Calendar meetings (imported as scheduled tasks).
      // Resolve event times to local HH:MM HERE — the browser knows the user's
      // timezone; the server's container may not, so a UTC-stored ("…Z") feed
      // would otherwise be packed against the wrong hour.
      const localHM = (iso: string): string | null => {
        const d = new Date(iso);
        if (isNaN(d.getTime())) return null;
        return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
      };
      const events: { title: string; start: string; end: string }[] = [];
      const pushEvent = (title: string, start: string, end: string) => {
        const s = localHM(start), e = localHM(end);
        if (s && e) events.push({ title, start: s, end: e });
      };
      try {
        const evs = await api.ical.listEvents(date);
        for (const e of evs ?? []) {
          if (!e.all_day && e.start_time && e.end_time) pushEvent(e.summary, e.start_time, e.end_time);
        }
      } catch { /* events optional */ }
      for (const t of todayTasks) {
        if (t.source === 'google_calendar' && t.scheduled_start && t.scheduled_end) {
          pushEvent(t.title, t.scheduled_start, t.scheduled_end);
        }
      }

      const res = await api.ai.planDay(
        date,
        active.map(t => ({ id: t.id, title: t.title, minutes: t.time_estimate_minutes ?? 0 })),
        events,
      );
      if (!res.available) {
        aiPlanError = 'AI is off or unreachable.';
        return;
      }
      if (res.order?.length) {
        // The schedule carries a soft start time (roughly_at) + effort estimate
        // per task. roughly_at is the daily board's primary sort key, so applying
        // it makes the plan actually land on the board — not just show here once.
        const sched = new Map((res.schedule ?? []).map(s => [s.id, s]));
        const patched = new Map<string, Partial<Task>>();
        for (const t of active) {
          const s = sched.get(t.id);
          if (!s) continue;
          const patch: { roughly_at: string; time_estimate_minutes?: number } = { roughly_at: s.roughly_at };
          // Don't clobber an estimate the user set; only fill blanks.
          if (!t.time_estimate_minutes && s.minutes) patch.time_estimate_minutes = s.minutes;
          patched.set(t.id, patch);
        }

        // Apply locally (roughly_at drives compareTasksForDay), then persist.
        todayTasks = todayTasks
          .map(t => ({ ...t, ...(patched.get(t.id) ?? {}) }))
          .sort(compareTasksForDay);
        aiPlanNote = res.note ?? '';
        aiPlanned = true;

        await Promise.all(
          [...patched.entries()].map(([id, patch]) => api.tasks.update(id, patch).catch(() => {})),
        );
      }
    } catch (e) {
      aiPlanError = e instanceof Error ? e.message.replace(/^\d+\s/, '') : 'Planning failed';
    }
    finally { aiPlanning = false; }
  }

  async function goToStep2() {
    step = 2;
    if (carryover.length === 0) await goToStep3();
  }

  async function goToStep3() {
    // Move selected carryover tasks to today
    saving = true;
    try {
      const moves = [...selected].map(id =>
        api.tasks.update(id, { planned_date: date, week_start: weekStart(date), status: 'planned' })
      );
      const moved = await Promise.all(moves);
      // Refresh today's tasks
      todayTasks = await api.tasks.listByDate(date);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to move tasks';
    } finally {
      saving = false;
    }
    step = 3;
  }

  async function startFocusing() {
    saving = true;
    try {
      await api.plans.upsert(date, {
        status: 'active',
        intention: intention.trim() || null,
      });
      goto(`/day/${date}`);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save plan';
      saving = false;
    }
  }

  const stepLabels = ['Intention', 'Carryover', 'Ready'];
</script>

<svelte:head><title>Plan {isToday(date) ? 'Today' : date} — Sempa</title></svelte:head>

<div class="flex min-h-full flex-col items-center justify-center px-4 py-10 sm:py-12 animate-page-in">
  <!-- Step indicators: a quiet pill stepper -->
  <ol class="ritual-steps mb-8" aria-label="Planning steps">
    {#each stepLabels as label, i}
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
    <!-- ── Step 1: Intention ───────────────────────────────────────────── -->
    {#if step === 1}
      {#key step}
      <div class="ritual-card animate-fade-in">
        <div class="ritual-head">
          <span class="ritual-icon" style="background: var(--cal-amber-bg); color: var(--cal-amber-fg);">
            <Sunrise size={22} strokeWidth={2} />
          </span>
          <h1 class="type-date" style="color: var(--sempa-text);">
            {isToday(date) ? 'Good morning!' : formatDate(date)}
          </h1>
          <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">What's your theme or focus for today?</p>
        </div>

        {#if yesterdayReflection}
          <div class="mb-4 px-4 py-3" style="background: var(--sempa-fill); border-radius: 16px;">
            <p class="mb-1 flex items-center gap-1.5 type-label" style="color: var(--sempa-text-soft);">
              <Quote size={12} strokeWidth={2.5} /> Yesterday you noted
            </p>
            <p class="text-sm italic" style="color: var(--sempa-text-soft);">"{yesterdayReflection}"</p>
          </div>
        {/if}

        <textarea
          bind:this={intentionEl}
          bind:value={intention}
          onkeydown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); goToStep2(); } }}
          onfocus={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'}
          onblur={(e) => (e.currentTarget as HTMLElement).style.borderColor = 'transparent'}
          rows="3"
          aria-label="Today's intention"
          placeholder="e.g. 'Ship the auth feature and prep for standup'"
          class="ritual-input"
          style="min-height: 104px; font-size: 15px; line-height: 1.6;"
        ></textarea>

        <div class="mt-5 flex justify-end">
          <button onclick={goToStep2} class="btn-primary ritual-cta">
            Continue
            <ArrowRight size={16} strokeWidth={2.25} />
          </button>
        </div>
      </div>
      {/key}

    <!-- ── Step 2: Carryover ───────────────────────────────────────────── -->
    {:else if step === 2}
      {#key step}
      <div class="ritual-card animate-fade-in">
        <div class="ritual-head">
          <span class="ritual-icon" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
            <CornerUpLeft size={22} strokeWidth={2} />
          </span>
          <h1 class="type-date" style="color: var(--sempa-text);">From yesterday</h1>
          <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">
            {carryover.length === 0
              ? "You cleared your board yesterday. Nicely done."
              : `${carryover.length} task${carryover.length !== 1 ? 's' : ''} didn't get done. Move them to today?`}
          </p>
        </div>

        {#if carryover.length > 0}
          <div class="mb-6 flex flex-col gap-2">
            {#each carryover as task (task.id)}
              {@const isSel = selected.has(task.id)}
              <div class="carry-row" class:is-selected={isSel}>
                <button type="button" role="checkbox" aria-checked={isSel}
                        onclick={() => toggleSelect(task.id)}
                        aria-label="Move “{task.title}” to today"
                        class="carry-check"
                        onmouseenter={(e) => { if (!isSel) (e.currentTarget as HTMLElement).style.borderColor = 'var(--sempa-accent)'; }}
                        onmouseleave={(e) => { if (!isSel) (e.currentTarget as HTMLElement).style.borderColor = ''; }}>
                  {#if isSel}
                    <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="3.25" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                    </svg>
                  {/if}
                </button>
                <p class="min-w-0 flex-1 text-sm font-medium" style="color: var(--sempa-text);">{task.title}</p>
                {#if task.time_estimate_minutes}
                  <span class="chip chip-time shrink-0">{formatMinutes(task.time_estimate_minutes)}</span>
                {/if}
              </div>
            {/each}
          </div>
        {/if}

        {#if error}<p class="mb-4 text-xs text-red-500">{error}</p>{/if}

        <div class="flex items-center justify-between">
          <button onclick={() => step = 1} class="ritual-back">
            <ArrowLeft size={16} strokeWidth={2.25} />
            Back
          </button>
          <button onclick={goToStep3} disabled={saving} class="btn-primary ritual-cta disabled:opacity-50">
            {saving ? 'Moving…' : 'Continue'}
            <ArrowRight size={16} strokeWidth={2.25} />
          </button>
        </div>
      </div>
      {/key}

    <!-- ── Step 3: Ready ───────────────────────────────────────────────── -->
    {:else}
      {#key step}
      <div class="ritual-card animate-fade-in">
        <div class="ritual-head">
          <span class="ritual-icon" style="background: var(--sempa-success-bg); color: var(--sempa-success);">
            <CircleCheck size={22} strokeWidth={2} />
          </span>
          <h1 class="type-date" style="color: var(--sempa-text);">You're set for today!</h1>
          <p class="mt-1 text-sm" style="color: var(--sempa-text-soft);">
            {todayTasks.filter(t => t.status !== 'done').length} tasks on your board
            {#if totalEstimate > 0}· {formatMinutes(totalEstimate)} estimated{/if}
          </p>
        </div>

        {#if intention}
          <div class="intention-card mb-4">
            <p class="type-label" style="color: var(--sempa-accent-ink);">Today's intention</p>
            <p class="mt-1 leading-relaxed" style="font-size: 15px; font-weight: 600; color: var(--sempa-text);">{intention}</p>
          </div>
        {/if}

        {#if showAiPlan && planActive.length >= 2}
          <div class="mt-1">
            <button onclick={aiPlanMyDay} disabled={aiPlanning}
                    class="btn-soft disabled:opacity-50"
                    style="height: 36px; padding: 0 14px; font-size: 13px;"
                    title="Suggest an order for today's tasks">
              <Sparkles size={14} strokeWidth={2} />
              {aiPlanning ? 'Planning…' : aiPlanned ? 'AI: re-plan' : 'AI: plan my day'}
            </button>
            {#if aiPlanNote}
              <p class="mt-2 px-3 py-2 text-left text-xs" style="background: var(--sempa-fill); border-radius: 12px; color: var(--sempa-text-soft);">{aiPlanNote}</p>
            {/if}
            {#if aiPlanError}
              <p class="mt-2 text-left text-xs text-red-500">{aiPlanError}</p>
            {/if}
            {#if aiPlanned}
              <!-- Show the applied order so the reorder is actually visible. -->
              <ol class="mt-3 flex flex-col gap-1.5 text-left">
                {#each planActive as t, i (t.id)}
                  <li class="flex items-center gap-2.5 px-3 py-2" style="background: var(--sempa-fill); border-radius: 14px;">
                    <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full tabular-nums"
                          style="font-size: 11.5px; font-weight: 700; background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">{i + 1}</span>
                    <span class="flex-1 text-sm" style="color: var(--sempa-text);">{t.title}</span>
                    {#if t.time_estimate_minutes}
                      <span class="chip chip-time" style="background: var(--sempa-bg-panel);">{formatMinutes(t.time_estimate_minutes)}</span>
                    {/if}
                  </li>
                {/each}
              </ol>
            {/if}
          </div>
        {/if}

        {#if error}<p class="mb-4 mt-3 text-xs text-red-500">{error}</p>{/if}

        <div class="mt-6 flex items-center justify-between">
          <button onclick={() => step = 2} class="ritual-back">
            <ArrowLeft size={16} strokeWidth={2.25} />
            Back
          </button>
          <button onclick={startFocusing} disabled={saving} class="btn-primary ritual-cta disabled:opacity-50">
            {saving ? 'Saving…' : 'Start focusing'}
            <ArrowRight size={16} strokeWidth={2.25} />
          </button>
        </div>
      </div>
      {/key}
    {/if}
  </div>
</div>

<style>
  /* ── Ritual shell (shared look with the shutdown ritual) ───────────────── */
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

  /* Soft accent-tinted intention card. */
  .intention-card { background: var(--sempa-accent-bg); border-radius: 18px; padding: 14px 16px; }

  /* Carry-over rows: a fill block; selected rows take the soft accent tint. */
  .carry-row {
    display: flex; align-items: center; gap: 12px;
    min-height: 52px; padding: 10px 14px; border-radius: 16px;
    background: var(--sempa-fill);
    box-shadow: inset 0 0 0 1px transparent;
    transition: background-color 120ms ease, box-shadow 120ms ease;
  }
  .carry-row.is-selected {
    background: var(--sempa-accent-bg);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--sempa-accent) 35%, transparent);
  }
  .carry-check {
    display: inline-grid; place-items: center; flex: 0 0 auto;
    width: 22px; height: 22px; border-radius: 999px;
    border: 2px solid var(--sempa-track); background: var(--sempa-bg-panel);
    transition: background-color 120ms ease, border-color 120ms ease;
  }
  .is-selected .carry-check { border-color: var(--sempa-btn-bg); background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); }
</style>
