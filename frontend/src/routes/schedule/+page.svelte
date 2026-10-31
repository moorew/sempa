<script lang="ts">
  import { api } from '$lib/api';
  import type { Task, UpdateTaskInput } from '$lib/types';
  import { offsetDate, weekStart, formatMinutes } from '$lib/utils';
  import { hapticTick } from '$lib/haptics';
  import { clock } from '$lib/stores/clock.svelte';
  import TimeslotCalendar from '$lib/components/TimeslotCalendar.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import BottomSheet from '$lib/components/BottomSheet.svelte';
  import { ChevronLeft, ChevronRight, CalendarClock, Clock } from 'lucide-svelte';

  // Reactive so the "Today" label / highlight roll over at midnight.
  const todayDate = $derived(clock.today);

  let date    = $state(clock.today);
  let tasks   = $state<Task[]>([]);
  let loading = $state(true);
  let error   = $state('');

  // Task editor (TaskPanel manages its own mobile bottom sheet)
  let panelOpen = $state(false);
  let panelTask = $state<Task | null>(null);

  // "Pick a task" sheet, opened by tapping an empty slot
  let slotSheet  = $state<{ start: string; end: string } | null>(null);

  $effect(() => { date; loadTasks(); });

  async function loadTasks() {
    loading = true; error = '';
    try {
      tasks = await api.tasks.listByDate(date);
    } catch (e: any) {
      error = e.message ?? 'Failed to load tasks';
    } finally { loading = false; }
  }

  function navigateDay(delta: number) {
    hapticTick();
    date = offsetDate(date, delta);
  }

  // ── Date strip (centred, swipeable) ─────────────────────────────────────────
  const STRIP_RANGE = 21;
  const stripDays = $derived(
    Array.from({ length: STRIP_RANGE * 2 + 1 }, (_, i) => {
      const d = offsetDate(date, i - STRIP_RANGE);
      const dt = new Date(d + 'T12:00:00');
      return {
        date: d,
        dayName: dt.toLocaleDateString('en-US', { weekday: 'short' }),
        dayNum: dt.toLocaleDateString('en-US', { day: 'numeric' }),
        isToday: d === todayDate,
        isWeekend: dt.getDay() === 0 || dt.getDay() === 6,
      };
    })
  );

  const headerLabel = $derived.by(() => {
    const dt = new Date(date + 'T12:00:00');
    if (date === todayDate) return 'Today';
    return dt.toLocaleDateString('en-US', { weekday: 'long', day: 'numeric', month: 'short' });
  });

  // Keep the selected day centred whenever the date changes.
  let stripEl = $state<HTMLElement | undefined>();
  $effect(() => {
    date;
    if (!stripEl) return;
    queueMicrotask(() => {
      const el = stripEl?.querySelector<HTMLElement>(`[data-strip-date="${date}"]`);
      if (el && stripEl) stripEl.scrollLeft = el.offsetLeft - stripEl.clientWidth / 2 + el.clientWidth / 2;
    });
  });

  // Unscheduled, plannable tasks for this day → candidates for an empty slot.
  const unscheduledForDay = $derived(
    tasks.filter(t => !t.scheduled_start && t.status !== 'done' && t.status !== 'cancelled' && !t.parent_task_id)
  );

  // ── Schedule / unschedule (same mutations as the desktop panel) ─────────────
  async function handleSchedule(taskId: string, start: string, end: string) {
    const prev = tasks.slice();
    const t = tasks.find(x => x.id === taskId);
    const plannedDate = start.slice(0, 10);
    const durationMin = Math.max(15, Math.round((new Date(end).getTime() - new Date(start).getTime()) / 60000));
    const patch: UpdateTaskInput = {
      scheduled_start: start,
      scheduled_end: end,
      planned_date: plannedDate,
      week_start: weekStart(plannedDate),
      time_estimate_minutes: durationMin,
    };
    if (t?.status === 'backlog') patch.status = 'planned';
    tasks = tasks.map(x => x.id === taskId ? { ...x, ...patch } : x);
    try {
      const updated = await api.tasks.update(taskId, patch);
      tasks = tasks.map(x => x.id === updated.id ? updated : x);
    } catch { tasks = prev; }
  }

  async function handleUnschedule(taskId: string) {
    const prev = tasks.slice();
    tasks = tasks.map(t => t.id === taskId ? { ...t, scheduled_start: null, scheduled_end: null } : t);
    try {
      await api.tasks.update(taskId, { scheduled_start: null, scheduled_end: null });
    } catch { tasks = prev; }
  }

  function openTask(id: string) {
    const t = tasks.find(t => t.id === id);
    if (t) { panelTask = t; panelOpen = true; }
  }

  function onPanelSave(saved: Task) {
    panelOpen = false;
    if (saved.status === 'cancelled') { tasks = tasks.filter(t => t.id !== saved.id); return; }
    const idx = tasks.findIndex(t => t.id === saved.id);
    tasks = idx >= 0 ? tasks.map(t => t.id === saved.id ? saved : t) : [...tasks, saved];
  }

  // ── Tap empty slot → pick a task to schedule there ──────────────────────────
  function onSlotTap(start: string, end: string) {
    hapticTick();
    slotSheet = { start, end };
  }

  async function pickTaskForSlot(taskId: string) {
    if (!slotSheet) return;
    const { start, end } = slotSheet;
    slotSheet = null;
    // Ensure the task is anchored to this day, then schedule it at the tapped time.
    const t = tasks.find(t => t.id === taskId);
    if (t && (t.planned_date !== date || t.status === 'backlog')) {
      await api.tasks.update(taskId, {
        planned_date: date, week_start: weekStart(date),
        status: t.status === 'backlog' ? 'planned' : t.status,
      }).catch(() => {});
    }
    await handleSchedule(taskId, start, end);
  }

  function slotTimeLabel(iso: string): string {
    const d = new Date(iso);
    return d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }

  const scheduledCount = $derived(tasks.filter(t => t.scheduled_start?.startsWith(date)).length);
</script>

<div class="flex h-full flex-col" style="background: var(--sempa-bg-main);">

  <!-- Header (1.27): eyebrow + big day title, round day nav, capsule strip -->
  <header class="sticky top-0 z-[40] px-5 pb-3"
          style="background: var(--sempa-bg-main); padding-top: max(16px, calc(env(safe-area-inset-top, 0px) + 10px));">
    <div class="flex items-center gap-3">
      <div class="min-w-0 flex-1">
        <p class="flex items-center gap-1.5 text-[13.5px] font-semibold" style="color: var(--sempa-accent-ink);">
          <CalendarClock size={15} strokeWidth={2.2} /> Schedule
        </p>
        <h1 class="type-page truncate" style="color: var(--sempa-text);">{headerLabel}</h1>
        {#if scheduledCount > 0}
          <p class="text-[12.5px] font-medium" style="color: var(--sempa-text-soft);">{scheduledCount} scheduled</p>
        {/if}
      </div>
      <button onclick={() => navigateDay(-1)} aria-label="Previous day" class="btn-icon h-11 w-11">
        <ChevronLeft size={20} strokeWidth={2.2} />
      </button>
      <button onclick={() => navigateDay(1)} aria-label="Next day" class="btn-icon h-11 w-11">
        <ChevronRight size={20} strokeWidth={2.2} />
      </button>
    </div>

    <!-- Date strip — capsules -->
    <!-- The strip scrolls sideways, which also clips vertically, so it carries
         extra bottom padding (pulled back with -mb-4) for the selected capsule's
         accent shadow to fall into instead of being cut off. -->
    <div bind:this={stripEl}
         class="no-scrollbar -mx-5 -mb-4 mt-2 flex snap-x snap-proximity gap-1.5 overflow-x-auto scroll-px-5 px-5 pb-6 pt-1"
         style="-webkit-overflow-scrolling: touch; overscroll-behavior-x: contain;">
      {#each stripDays as day (day.date)}
        {@const isSel = day.date === date}
        <button onclick={() => { hapticTick(); date = day.date; }}
                data-strip-date={day.date}
                aria-current={isSel ? 'date' : undefined}
                class="flex h-[68px] w-[46px] shrink-0 snap-center flex-col items-center justify-center gap-0.5 rounded-full transition-colors"
                style={isSel
                  ? 'background: var(--sempa-btn-bg); color: var(--sempa-btn-fg); box-shadow: var(--sempa-shadow-accent);'
                  : day.isToday
                    ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);'
                    : day.isWeekend
                      ? 'color: var(--sempa-text-dim);'
                      : 'color: var(--sempa-text-soft);'}>
          <span class="text-[16px] font-extrabold leading-none">{day.dayNum}</span>
          <span class="text-[10.5px] font-semibold opacity-85">{day.dayName}</span>
          <span class="mt-0.5 h-[5px] w-[5px] rounded-full"
                style="background: {isSel ? 'var(--sempa-btn-fg)' : day.isToday ? 'var(--sempa-accent)' : 'transparent'};"></span>
        </button>
      {/each}
    </div>
  </header>

  <!-- Timeline -->
  <div class="mx-4 mb-4 flex-1 overflow-hidden rounded-[26px] pt-3"
       style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1);">
    {#if error}
      <p class="px-4 py-6 text-sm text-red-500">{error}</p>
    {:else}
      <TimeslotCalendar
        {date}
        {tasks}
        onSchedule={handleSchedule}
        onUnschedule={handleUnschedule}
        onOpenTask={openTask}
        onSlotTap={onSlotTap}
        onEventConverted={(t) => { tasks = [...tasks, t]; }}
      />
    {/if}
  </div>
</div>

<!-- Pick-a-task sheet (tap empty slot) -->
<BottomSheet open={slotSheet !== null} onClose={() => slotSheet = null}>
  {#if slotSheet}
    <div class="px-5 pb-6 pt-1" data-sheet-scroll>
      <div class="flex items-center gap-2">
        <Clock size={16} style="color: var(--sempa-accent);" />
        <h2 class="text-base font-semibold" style="color: var(--sempa-text);">
          Schedule at {slotTimeLabel(slotSheet.start)}
        </h2>
      </div>

      {#if unscheduledForDay.length === 0}
        <p class="mt-4 text-sm" style="color: var(--sempa-text-dim);">
          No unscheduled tasks for this day. Plan a task first, then place it here.
        </p>
      {:else}
        <p class="mt-1 text-xs" style="color: var(--sempa-text-dim);">Pick a task to place on the timeline</p>
        <ul class="mt-3 space-y-2">
          {#each unscheduledForDay as t (t.id)}
            <li>
              <button onclick={() => pickTaskForSlot(t.id)}
                      class="flex w-full items-center gap-3 rounded-[18px] px-4 py-3.5 text-left transition-colors active:opacity-80"
                      style="background: var(--sempa-fill);">
                <span class="min-w-0 flex-1 truncate text-sm" style="color: var(--sempa-text);">{t.title}</span>
                {#if t.time_estimate_minutes}
                  <span class="shrink-0 text-xs" style="color: var(--sempa-text-dim);">{formatMinutes(t.time_estimate_minutes)}</span>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</BottomSheet>

<TaskPanel
  open={panelOpen}
  task={panelTask}
  defaultDate={date}
  onSave={onPanelSave}
  onClose={() => panelOpen = false}
/>
