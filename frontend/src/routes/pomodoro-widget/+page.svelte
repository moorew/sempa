<script lang="ts">
  // Standalone Pomodoro widget — a self-contained focus timer surface meant to run
  // in its own chromeless Tauri window (always-on-top mini-window), and reusable on
  // the web. It binds to the same `pomodoro` store as the in-app corner timer, so
  // state (and persistence) is shared. Three states: idle (pick today's task),
  // active (countdown + controls), confirm (log the measured time).
  //
  // NOTE: the root layout suppresses the global corner <PomodoroTimer/> and
  // <SessionConfirm/> on this route (isPomodoroWidget) so we own the whole window.
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { api } from '$lib/api';
  import { syncStore } from '$lib/sync.svelte';
  import { formatMinutes } from '$lib/utils';
  import type { Task } from '$lib/types';

  // Local YYYY-MM-DD (matches the day Kanban / api localToday).
  function today(): string {
    const d = new Date();
    const mm = String(d.getMonth() + 1).padStart(2, '0');
    const dd = String(d.getDate()).padStart(2, '0');
    return `${d.getFullYear()}-${mm}-${dd}`;
  }

  let tasks = $state<Task[]>([]);
  let loading = $state(true);
  let confirmMinutes = $state(0);

  const accentColor = $derived(
    pomodoro.phase === 'work' ? 'var(--sempa-accent)' : 'var(--sempa-success)'
  );
  const overBy = $derived(pomodoro.overByMinutes);

  // Today's plannable tasks: not done/cancelled, ordered as on the day board.
  async function loadTasks() {
    loading = true;
    try {
      const all = await api.tasks.listByDate(today());
      tasks = all
        .filter((t) => t.status !== 'done' && t.status !== 'cancelled')
        .sort((a, b) => (a.position ?? 0) - (b.position ?? 0));
    } catch {
      tasks = [];
    } finally {
      loading = false;
    }
  }

  // Refresh the picker on mount, whenever a sync pull lands, and whenever the timer
  // returns to idle (a just-finished task should drop out of the list).
  $effect(() => {
    // track the signals that should trigger a reload
    void syncStore.revision;
    void pomodoro.taskId;
    void pomodoro.pendingConfirm;
    if (!pomodoro.taskId && !pomodoro.pendingConfirm) void loadTasks();
  });

  function startTask(t: Task) {
    pomodoro.start(t.id, t.title, t.time_actual_minutes ?? 0, t.time_estimate_minutes ?? null);
  }

  // When the confirm sheet opens, seed the editable minutes from the measured value.
  $effect(() => {
    const pc = pomodoro.pendingConfirm;
    if (pc) confirmMinutes = pc.elapsedMinutes;
  });
</script>

<div
  class="flex h-screen w-screen flex-col overflow-hidden p-3"
  style="background: var(--sempa-bg-panel); color: var(--sempa-text);"
>
  {#if pomodoro.pendingConfirm}
    <!-- ── Confirm: log the measured time ─────────────────────────────────── -->
    {@const pc = pomodoro.pendingConfirm}
    <p class="mb-1 truncate text-sm font-semibold" title={pc.taskTitle}>{pc.taskTitle}</p>
    <p class="mb-2 text-xs" style="color: var(--sempa-text-dim);">How long did you actually work?</p>

    <div class="mb-2 flex items-center justify-center gap-2">
      <button onclick={() => (confirmMinutes = Math.max(0, confirmMinutes - 5))}
        class="h-8 w-8 rounded-lg text-lg leading-none"
        style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);"
        aria-label="Subtract 5 minutes">−</button>
      <div class="flex items-baseline gap-1">
        <input type="number" min="0" max="600" bind:value={confirmMinutes}
          class="w-16 rounded-lg px-2 py-1 text-center text-2xl font-bold tabular-nums"
          style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
        <span class="text-xs" style="color: var(--sempa-text-dim);">min</span>
      </div>
      <button onclick={() => (confirmMinutes = Math.min(600, confirmMinutes + 5))}
        class="h-8 w-8 rounded-lg text-lg leading-none"
        style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);"
        aria-label="Add 5 minutes">+</button>
    </div>

    {#if pc.plannedMinutes}
      <p class="mb-2 text-center text-[11px]" style="color: var(--sempa-text-dim);">
        {#if confirmMinutes > pc.plannedMinutes}
          {confirmMinutes - pc.plannedMinutes}m over the {formatMinutes(pc.plannedMinutes)} planned
        {:else}
          of {formatMinutes(pc.plannedMinutes)} planned
        {/if}
      </p>
    {/if}

    <div class="mt-auto flex flex-col gap-1.5">
      <button onclick={() => pomodoro.confirmSession(confirmMinutes)}
        class="rounded-lg py-2 text-sm font-medium text-white transition-opacity hover:opacity-90"
        style="background: var(--sempa-accent);">
        Log {confirmMinutes} min{pc.markDone ? ' & mark done' : ''}
      </button>
      <div class="flex gap-1.5">
        <button onclick={() => pomodoro.confirmSession(0)}
          class="flex-1 rounded-lg py-1.5 text-xs transition-opacity hover:opacity-80"
          style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
          Didn't work on it
        </button>
        <button onclick={() => pomodoro.discardSession()}
          class="flex-1 rounded-lg py-1.5 text-xs transition-opacity hover:opacity-80"
          style="color: var(--sempa-text-dim);">
          Discard
        </button>
      </div>
    </div>
  {:else if pomodoro.taskId}
    <!-- ── Active: countdown + controls ───────────────────────────────────── -->
    <div class="mb-1 flex items-center justify-between">
      <span data-tauri-drag-region class="text-[11px] font-semibold uppercase tracking-wider" style="color: {accentColor};">
        {pomodoro.phaseLabel}
      </span>
      <button onclick={() => pomodoro.requestStop()}
        class="rounded p-1 transition-opacity hover:opacity-70"
        style="color: var(--sempa-text-dim);" aria-label="Stop and log time">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>

    {#if pomodoro.taskTitle}
      <p class="mb-1 truncate text-sm font-medium" title={pomodoro.taskTitle}>{pomodoro.taskTitle}</p>
    {/if}

    <div class="text-center">
      <span class="font-mono text-5xl font-bold tabular-nums">{pomodoro.display}</span>
    </div>

    <div class="my-2 h-1.5 overflow-hidden rounded-full" style="background: var(--sempa-border);">
      <div class="h-full rounded-full transition-all duration-500"
        style="width: {pomodoro.progressPct}%; background: {accentColor};"></div>
    </div>

    <div class="mb-2 flex items-center justify-between text-xs" style="color: var(--sempa-text-dim);">
      <span><span class="font-mono tabular-nums" style="color: var(--sempa-text-soft);">{pomodoro.elapsedDisplay}</span> spent</span>
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

    <div class="mt-auto flex gap-2">
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
  {:else}
    <!-- ── Idle: pick a task to focus ─────────────────────────────────────── -->
    <div class="mb-2 flex items-center justify-between">
      <span data-tauri-drag-region class="text-[11px] font-semibold uppercase tracking-wider" style="color: var(--sempa-text-dim);">
        Focus on…
      </span>
      <button onclick={() => loadTasks()}
        class="rounded p-1 transition-opacity hover:opacity-70"
        style="color: var(--sempa-text-dim);" aria-label="Refresh tasks">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      </button>
    </div>

    <div class="-mx-1 flex-1 overflow-y-auto px-1">
      {#if loading}
        <p class="py-6 text-center text-xs" style="color: var(--sempa-text-dim);">Loading…</p>
      {:else if tasks.length === 0}
        <p class="py-6 text-center text-xs" style="color: var(--sempa-text-dim);">No tasks planned for today.</p>
      {:else}
        <ul class="flex flex-col gap-1">
          {#each tasks as t (t.id)}
            <li>
              <button onclick={() => startTask(t)}
                class="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left transition-colors hover:opacity-90"
                style="border: 1px solid var(--sempa-border); background: var(--sempa-bg);">
                <svg class="h-3.5 w-3.5 shrink-0" style="color: {accentColor};" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M8 5v14l11-7z" />
                </svg>
                <span class="flex-1 truncate text-[13px]" style="color: var(--sempa-text);">{t.title}</span>
                {#if t.time_estimate_minutes}
                  <span class="shrink-0 text-[11px] tabular-nums" style="color: var(--sempa-text-dim);">{formatMinutes(t.time_estimate_minutes)}</span>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</div>
