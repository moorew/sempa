<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import type { Objective, Task, WeekReview } from '$lib/types';
  import { formatMinutes, formatWeekRange, today, weekStart as calcWeekStart } from '$lib/utils';
  import { moveAllUnfinishedToNextWeek, unfinishedObjectives } from '$lib/objectives';
  import { CalendarArrowDown, Sparkles, ChartColumn, MessageCircle, Target, ArrowLeft, ArrowRight,
           Check, CircleCheck, Clock, Trophy, Mountain, Plus } from 'lucide-svelte';
  import { prefs } from '$lib/stores/prefs.svelte';
  import { aiStatus } from '$lib/stores/aiStatus.svelte';

  let ws = $derived(page.params.weekStart ?? calcWeekStart(today()));

  let step = $state(1); // 1=stats, 2=reflection, 3=next-week
  let loading = $state(true);
  let saving  = $state(false);
  let error   = $state('');

  let tasks      = $state<Task[]>([]);
  let objectives = $state<Objective[]>([]);
  let review     = $state<WeekReview | null>(null);

  // Step 2 inputs (wins / challenges as bullet arrays)
  let wins       = $state<string[]>(['']);
  let challenges = $state<string[]>(['']);
  let nextFocus  = $state('');

  async function load() {
    loading = true;
    try {
      [tasks, objectives, review] = await Promise.all([
        api.tasks.listByWeek(ws),
        api.objectives.listByWeek(ws),
        api.weeks.getReview(ws).catch(() => null),
      ]);
      if (review) {
        wins       = review.wins       ? JSON.parse(review.wins)       : [''];
        challenges = review.challenges ? JSON.parse(review.challenges) : [''];
        nextFocus  = review.next_focus ?? '';
      }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  // ── Carry unfinished objectives forward to next week ─────────────────────────
  const unfinished = $derived(unfinishedObjectives(objectives));
  let carrying    = $state(false);
  let carriedCount = $state<number | null>(null);

  async function carryForward() {
    if (carrying || unfinished.length === 0) return;
    carrying = true;
    const n = unfinished.length;
    try {
      await moveAllUnfinishedToNextWeek(objectives, tasks);
      // Reflect the move locally so the prompt collapses into its done state.
      objectives = objectives.filter(o => o.status === 'completed' || o.status === 'cancelled');
      carriedCount = n;
    } catch {
      error = 'Could not move objectives — please try again.';
    } finally {
      carrying = false;
    }
  }

  // Stats
  const doneTasks  = $derived(tasks.filter(t => t.status === 'done'));
  const totalTasks = $derived(tasks.filter(t => t.status !== 'cancelled'));
  const totalMins  = $derived(doneTasks.reduce((s, t) => s + (t.time_actual_minutes ?? t.time_estimate_minutes ?? 0), 0));
  const doneObjs   = $derived(objectives.filter(o => o.status === 'completed').length);

  // ── AI: draft the review from the week's completed work ──────────────────────
  let aiDrafting = $state(false);
  const showAiDraft = $derived(prefs.aiOn('weeklyReview') && aiStatus.reachable);
  async function aiDraftReview() {
    if (aiDrafting) return;
    aiDrafting = true;
    try {
      const res = await api.ai.weeklyReview(
        doneTasks.map(t => t.title),
        objectives.map(o => ({ title: o.title, status: o.status })),
      );
      if (res.available) {
        if (res.wins?.length) wins = res.wins;
        if (res.challenges?.length) challenges = res.challenges;
        if (res.next_focus) nextFocus = res.next_focus;
      }
    } catch { /* keep manual entry on failure */ }
    finally { aiDrafting = false; }
  }

  function addBullet(arr: string[], setter: (v: string[]) => void) {
    setter([...arr, '']);
  }
  function updateBullet(arr: string[], i: number, val: string, setter: (v: string[]) => void) {
    const copy = [...arr]; copy[i] = val; setter(copy);
  }
  function removeBullet(arr: string[], i: number, setter: (v: string[]) => void) {
    if (arr.length <= 1) { setter(['']); return; }
    setter(arr.filter((_, j) => j !== i));
  }
  function handleBulletKey(e: KeyboardEvent, arr: string[], i: number, setter: (v: string[]) => void) {
    if (e.key === 'Enter') { e.preventDefault(); addBullet(arr, setter); }
    if (e.key === 'Backspace' && (e.target as HTMLInputElement).value === '' && arr.length > 1) {
      e.preventDefault(); removeBullet(arr, i, setter);
    }
  }

  async function save() {
    saving = true;
    try {
      const w = wins.filter(s => s.trim());
      const c = challenges.filter(s => s.trim());
      await api.weeks.upsertReview(ws, {
        wins:       w.length ? JSON.stringify(w) : null,
        challenges: c.length ? JSON.stringify(c) : null,
        next_focus: nextFocus.trim() || null,
      });
      goto(`/week/${ws}`);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    } finally {
      saving = false;
    }
  }

  const STEPS = [
    { n: 1, icon: ChartColumn,   label: 'Stats' },
    { n: 2, icon: MessageCircle, label: 'Reflect' },
    { n: 3, icon: Target,        label: 'Next week' },
  ];
</script>

<svelte:head><title>Weekly Review — {formatWeekRange(ws)}</title></svelte:head>

<div class="mx-auto max-w-xl px-4 pb-28 pt-8 sm:px-6 sm:py-10 space-y-6 animate-page-in"
     style="padding-top: calc(env(safe-area-inset-top, 0px) + 32px);">

  <!-- Header -->
  <div>
    <a href="/week/{ws}" class="back-link">
      <ArrowLeft size={14} strokeWidth={2.25} />
      Back to week
    </a>
    <h1 class="mt-3 type-page" style="color: var(--sempa-text);">Weekly review</h1>
    <p class="mt-1 type-meta" style="color: var(--sempa-text-soft);">{formatWeekRange(ws)}</p>
  </div>

  <!-- Step indicators: a segmented control -->
  <div class="segmented on-canvas" role="tablist" aria-label="Review steps">
    {#each STEPS as s}
      <button onclick={() => step = s.n}
              role="tab"
              aria-selected={step === s.n}
              class="seg-step"
              style="color: {step === s.n ? 'var(--sempa-text)' : step > s.n ? 'var(--sempa-success)' : 'var(--sempa-text-soft)'};">
        {#if s.n < step}
          <Check size={14} strokeWidth={3} />
        {:else}
          <s.icon size={14} strokeWidth={2.25} />
        {/if}
        {s.label}
      </button>
    {/each}
  </div>

  {#if loading}
    <div class="flex h-40 items-center justify-center text-sm" style="color: var(--sempa-text-dim);">Loading…</div>
  {:else if error}
    <p class="rounded-xl bg-red-50 p-4 text-sm text-red-600 dark:bg-red-950 dark:text-red-400">{error}</p>

  <!-- Step 1: Stats -->
  {:else if step === 1}
    <div class="space-y-5">
      <h2 class="type-section" style="color: var(--sempa-text);">This week at a glance</h2>

      <div class="grid grid-cols-3 gap-2.5 sm:gap-3">
        <div class="stat-tile">
          <span class="stat-icon" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
            <CircleCheck size={20} strokeWidth={2} />
          </span>
          <p class="stat-num tabular-nums">{doneTasks.length}</p>
          <p class="stat-label">of {totalTasks.length} tasks done</p>
        </div>
        <div class="stat-tile">
          <span class="stat-icon" style="background: var(--sempa-success-bg); color: var(--sempa-success);">
            <Target size={20} strokeWidth={2} />
          </span>
          <p class="stat-num tabular-nums">{doneObjs}</p>
          <p class="stat-label">of {objectives.length} objectives</p>
        </div>
        <div class="stat-tile">
          <span class="stat-icon" style="background: var(--cal-amber-bg); color: var(--cal-amber-fg);">
            <Clock size={20} strokeWidth={2} />
          </span>
          <p class="stat-num tabular-nums">{totalMins > 0 ? formatMinutes(totalMins) : '—'}</p>
          <p class="stat-label">time logged</p>
        </div>
      </div>

      {#if doneTasks.length > 0}
        <div class="surface" style="border-radius: 20px; padding: 14px 16px 10px;">
          <p class="mb-1 type-label" style="color: var(--sempa-text-soft);">Completed tasks</p>
          <ul>
            {#each doneTasks.slice(0, 8) as t, i}
              <li class="flex items-center gap-2.5 py-2 text-sm" style="color: var(--sempa-text); {i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}">
                <span class="check-ring" data-state="done" style="width: 18px; height: 18px;" aria-hidden="true">
                  <svg class="h-2.5 w-2.5" fill="none" stroke="currentColor" stroke-width="3.5" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                  </svg>
                </span>
                <span class="truncate">{t.title}</span>
              </li>
            {/each}
            {#if doneTasks.length > 8}
              <li class="py-2 pl-7 type-meta" style="color: var(--sempa-text-dim); border-top: 1px solid var(--sempa-border);">+{doneTasks.length - 8} more</li>
            {/if}
          </ul>
        </div>
      {/if}

      <button onclick={() => step = 2}
              class="btn-primary w-full"
              style="height: 46px; font-size: 14px;">
        Continue
        <ArrowRight size={16} strokeWidth={2.25} />
      </button>
    </div>

  <!-- Step 2: Reflection -->
  {:else if step === 2}
    <div class="space-y-5">
      <div class="flex items-center justify-between gap-3">
        <h2 class="type-section" style="color: var(--sempa-text);">How did the week go?</h2>
        {#if showAiDraft}
          <button onclick={aiDraftReview} disabled={aiDrafting}
                  class="btn-soft disabled:opacity-50"
                  style="height: 34px; padding: 0 14px; font-size: 12.5px;"
                  title="Draft this review from your completed work">
            <Sparkles size={13} strokeWidth={2} />
            {aiDrafting ? 'Drafting…' : 'AI draft'}
          </button>
        {/if}
      </div>

      <div class="surface" style="border-radius: 20px; padding: 16px;" role="group" aria-labelledby="wins-label">
        <p id="wins-label" class="field-title">
          <span class="field-icon" style="background: var(--cal-amber-bg); color: var(--cal-amber-fg);"><Trophy size={14} strokeWidth={2.25} /></span>
          What went well?
        </p>
        <div class="space-y-1.5">
          {#each wins as w, i}
            <input value={w}
                   oninput={(e) => updateBullet(wins, i, (e.target as HTMLInputElement).value, v => wins = v)}
                   onkeydown={(e) => handleBulletKey(e, wins, i, v => wins = v)}
                   type="text"
                   placeholder="Something that went well…"
                   aria-label="Win {i + 1}"
                   class="bullet-input" />
          {/each}
        </div>
        <button onclick={() => addBullet(wins, v => wins = v)} class="add-link">
          <Plus size={13} strokeWidth={2.5} /> Add
        </button>
      </div>

      <div class="surface" style="border-radius: 20px; padding: 16px;" role="group" aria-labelledby="challenges-label">
        <p id="challenges-label" class="field-title">
          <span class="field-icon" style="background: var(--cal-stone-bg); color: var(--cal-stone-fg);"><Mountain size={14} strokeWidth={2.25} /></span>
          What was challenging?
        </p>
        <div class="space-y-1.5">
          {#each challenges as c, i}
            <input value={c}
                   oninput={(e) => updateBullet(challenges, i, (e.target as HTMLInputElement).value, v => challenges = v)}
                   onkeydown={(e) => handleBulletKey(e, challenges, i, v => challenges = v)}
                   type="text"
                   placeholder="Something that was hard…"
                   aria-label="Challenge {i + 1}"
                   class="bullet-input" />
          {/each}
        </div>
        <button onclick={() => addBullet(challenges, v => challenges = v)} class="add-link">
          <Plus size={13} strokeWidth={2.5} /> Add
        </button>
      </div>

      <div class="flex gap-2">
        <button onclick={() => step = 1}
                class="btn-surface flex-1"
                style="height: 46px; font-size: 14px; color: var(--sempa-text-soft);">
          <ArrowLeft size={16} strokeWidth={2.25} />
          Back
        </button>
        <button onclick={() => step = 3}
                class="btn-primary flex-1"
                style="height: 46px; font-size: 14px;">
          Continue
          <ArrowRight size={16} strokeWidth={2.25} />
        </button>
      </div>
    </div>

  <!-- Step 3: Next week -->
  {:else if step === 3}
    <div class="space-y-5">
      <h2 class="type-section" style="color: var(--sempa-text);">Looking ahead</h2>

      <!-- Carry unfinished objectives into next week -->
      {#if carriedCount !== null}
        <div class="flex items-center gap-2.5 px-4 py-3 text-sm font-medium"
             style="border-radius: 16px; background: var(--sempa-success-bg); color: var(--sempa-success);">
          <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
          </svg>
          Carried {carriedCount} {carriedCount === 1 ? 'objective' : 'objectives'} to next week.
        </div>
      {:else if unfinished.length > 0}
        <div class="surface" style="border-radius: 20px; padding: 16px;">
          <p class="mb-2 text-sm font-semibold" style="color: var(--sempa-text);">
            {unfinished.length} unfinished {unfinished.length === 1 ? 'objective' : 'objectives'} this week
          </p>
          <ul class="mb-3 space-y-1.5">
            {#each unfinished as o}
              <li class="flex items-center gap-2.5 text-sm" style="color: var(--sempa-text-soft);">
                <span class="check-ring" data-state="active" style="width: 16px; height: 16px;" aria-hidden="true"></span>
                <span class="truncate">{o.title}</span>
              </li>
            {/each}
          </ul>
          <button onclick={carryForward} disabled={carrying}
                  class="btn-soft w-full disabled:opacity-50"
                  style="height: 40px; font-size: 13px;">
            <CalendarArrowDown size={15} />
            {carrying ? 'Moving…' : 'Move them to next week'}
          </button>
        </div>
      {/if}

      <!-- The intention for next week: the soft accent "intention" card -->
      <div class="intention-card">
        <label class="mb-2 flex items-center gap-2 type-label" style="color: var(--sempa-accent-ink);" for="next-focus">
          <Target size={14} strokeWidth={2.25} />
          Your focus for next week
        </label>
        <textarea id="next-focus" bind:value={nextFocus} rows="4"
                  placeholder="Your intention for the week ahead…"
                  class="focus-area"></textarea>
      </div>

      {#if error}
        <p class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-950 dark:text-red-400">{error}</p>
      {/if}

      <div class="flex gap-2">
        <button onclick={() => step = 2}
                class="btn-surface flex-1"
                style="height: 46px; font-size: 14px; color: var(--sempa-text-soft);">
          <ArrowLeft size={16} strokeWidth={2.25} />
          Back
        </button>
        <button onclick={save} disabled={saving}
                class="btn-primary flex-1 disabled:opacity-40"
                style="height: 46px; font-size: 14px;">
          {saving ? 'Saving…' : 'Complete review'}
          {#if !saving}<Check size={16} strokeWidth={2.5} />{/if}
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .back-link {
    display: inline-flex; align-items: center; gap: 6px;
    font-size: 13px; font-weight: 600; color: var(--sempa-text-soft);
    transition: color 120ms ease;
  }
  .back-link:hover { color: var(--sempa-accent-ink); }

  /* The segmented track normally sits on a surface; here it sits on the
     canvas, so recess it a notch so the raised selected pill reads. */
  .on-canvas { background: color-mix(in srgb, var(--sempa-text) 6%, var(--sempa-bg-main)); }

  .seg-step {
    display: inline-flex; align-items: center; justify-content: center; gap: 6px;
    height: 36px; padding: 0 10px; font-size: 13px; font-weight: 600; white-space: nowrap;
  }
  .seg-step[aria-selected="true"] { font-weight: 700; }

  /* Stat tile: surface, radius 22, tinted 46px icon tile, big number, soft label. */
  .stat-tile {
    background: var(--sempa-bg-panel);
    border-radius: var(--radius-tile);
    box-shadow: var(--sempa-shadow-1);
    padding: 14px;
    min-width: 0;
  }
  .stat-icon {
    display: grid; place-items: center;
    width: 46px; height: 46px; border-radius: 14px;
    margin-bottom: 12px;
  }
  .stat-num   { font-size: 20px; font-weight: 800; letter-spacing: -.02em; line-height: 1.15; color: var(--sempa-text); }
  .stat-label { margin-top: 2px; font-size: 12.5px; font-weight: 500; color: var(--sempa-text-soft); line-height: 1.3; }
  @media (max-width: 420px) {
    .stat-tile { padding: 12px; }
    .stat-icon { width: 40px; height: 40px; border-radius: 12px; margin-bottom: 10px; }
  }

  .field-title {
    display: flex; align-items: center; gap: 10px; margin-bottom: 12px;
    font-size: 15px; font-weight: 700; letter-spacing: -.01em; color: var(--sempa-text);
  }
  .field-icon {
    display: inline-grid; place-items: center; width: 28px; height: 28px; border-radius: 10px;
  }
  .bullet-input {
    display: block; width: 100%;
    background: var(--sempa-fill); border: 1px solid transparent; border-radius: 12px;
    padding: 9px 13px; font-size: 14px; color: var(--sempa-text); outline: none;
    transition: border-color 120ms ease;
  }
  .bullet-input:focus { border-color: var(--sempa-accent); }
  .add-link {
    display: inline-flex; align-items: center; gap: 5px; margin-top: 8px;
    height: 30px; padding: 0 10px 0 6px; border-radius: 999px;
    font-size: 12.5px; font-weight: 600; color: var(--sempa-accent-ink);
  }
  .add-link:hover { background: var(--sempa-accent-bg); }

  /* Soft accent-tinted intention card. */
  .intention-card {
    background: var(--sempa-accent-bg);
    border-radius: 22px;
    padding: 16px;
  }
  .focus-area {
    display: block; width: 100%; resize: none;
    background: var(--sempa-bg-panel); border: 1px solid transparent; border-radius: 16px;
    padding: 14px; font-size: 14px; line-height: 1.65; color: var(--sempa-text); outline: none;
    transition: border-color 120ms ease;
  }
  .focus-area:focus { border-color: var(--sempa-accent); }
</style>
