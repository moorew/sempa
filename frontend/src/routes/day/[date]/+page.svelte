<script lang="ts">
  import { onMount, tick, untrack } from 'svelte';
  import { timeTracking } from '$lib/stores/timeTracking.svelte';
  import { timeInsights } from '$lib/stores/timeInsights.svelte';
  import { plannedMinutes } from '$lib/dayCapacity';
  import { flip } from 'svelte/animate';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import type { Task, TaskStatus, UpdateTaskInput } from '$lib/types';
  import { appendPosition, compareTasksForDay, formatMinutes, isToday, offsetDate, weekStart } from '$lib/utils';
  import { clock } from '$lib/stores/clock.svelte';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import { mobile } from '$lib/stores/mobile.svelte';
  import { hapticTick } from '$lib/haptics';
  import WeekDayColumn from '$lib/components/WeekDayColumn.svelte';
  import TaskPanel from '$lib/components/TaskPanel.svelte';
  import BottomSheet from '$lib/components/BottomSheet.svelte';
  import EmailPanel from '$lib/components/EmailPanel.svelte';
  import MiniCalendar from '$lib/components/MiniCalendar.svelte';
  import TimeslotCalendar from '$lib/components/TimeslotCalendar.svelte';
  import WeeklyObjectivesWidget from '$lib/components/WeeklyObjectivesWidget.svelte';
  import { ChevronLeft, ChevronRight, Plus, Clock, Mail, SlidersHorizontal, Target } from 'lucide-svelte';
  import { tagStore } from '$lib/stores/tags.svelte';
  import TagFilterBar from '$lib/components/TagFilterBar.svelte';
  import JiraPanel from '$lib/components/JiraPanel.svelte';
  import { jiraStatus } from '$lib/stores/jiraStatus.svelte';
  import MobileTaskCard from '$lib/components/MobileTaskCard.svelte';
  import MobileTaskView from '$lib/components/MobileTaskView.svelte';
  import { syncWidgetData } from '$lib/widget-bridge';
  import { realtime } from '$lib/stores/realtime.svelte';
  import { swipeNavigate } from '$lib/actions/swipeNavigate';
  import { prefs } from '$lib/stores/prefs.svelte';
  import ReflectionCard from '$lib/components/ReflectionCard.svelte';
  import IntentionQuoteCard from '$lib/components/IntentionQuoteCard.svelte';
  import { celebrate } from '$lib/celebrate';
  import type { DailyPlan } from '$lib/types';

  // "date" is used to anchor the week and mark today
  let date      = $derived($page.params.date ?? clock.today);
  let ws        = $derived(weekStart(date));
  // Reactive so the "today" highlight, greeting and rollover roll over at
  // midnight instead of staying frozen on the day the page was opened.
  let todayDate = $derived(clock.today);
  // Whether the anchored day is today — drives the always-present "Today" jump.
  const onToday = $derived(date === todayDate);

  let tasks   = $state<Task[]>([]);
  let loading = $state(true);
  let error   = $state<string | null>(null);

  // Rollover
  let rolloverTasks    = $state<Task[]>([]);
  let rolloverDismissed = $state(false);

  let kanbanScroll  = $state<HTMLElement | undefined>();
  let weekGrid      = $state<HTMLElement | undefined>();
  let draggingId    = $state<string | null>(null);
  let dragOverDate  = $state<string | null>(null);
  let emailPanel    = $state<EmailPanel | undefined>(undefined);
  type RightPanel = 'schedule' | 'mail' | 'jira' | 'objectives';
  let rightPanel    = $state<RightPanel>('schedule');

  // The Jira tab only exists when Jira is actually connected — an unconfigured
  // integration shouldn't take up a quarter of the tab bar. If it disappears
  // while it's the active tab (user disconnects in settings), fall back rather
  // than leaving the pane blank.
  const rightPanelTabs = $derived<{ id: RightPanel; label: string }[]>([
    { id: 'schedule',   label: 'Schedule' },
    { id: 'mail',       label: 'Inbox' },
    ...(jiraStatus.connected ? [{ id: 'jira' as const, label: 'Jira' }] : []),
    { id: 'objectives', label: 'Goals' },
  ]);
  // Derived, not an $effect that reassigns rightPanel — an effect that reads and
  // writes the same $state is the read-modify-write loop that wedges the whole app.
  const activePanel = $derived<RightPanel>(
    rightPanel === 'jira' && !jiraStatus.connected ? 'schedule' : rightPanel,
  );

  let panelOpen   = $state(false);
  let panelTask   = $state<Task | null>(null);
  let panelStatus = $state<TaskStatus>('planned');
  let panelDate   = $state(date);
  let panelTitle  = $state('');  // create-mode prefill (e.g. shared-in text)
  let panelNotes  = $state('');

  // Mobile task detail view (read-first, tap Edit to open full panel)
  let mobileViewOpen = $state(false);
  let mobileViewTaskId = $state<string | null>(null);
  // Derived so complete/edit actions update the view in real-time
  const mobileViewTask = $derived(mobileViewTaskId ? (tasks.find(t => t.id === mobileViewTaskId) ?? null) : null);

  function openMobileView(task: Task) {
    mobileViewTaskId = task.id;
    mobileViewOpen = true;
  }

  // Week days: Mon–Sun (used by the mobile header's selected-day lookup).
  const weekDays = $derived(
    Array.from({ length: 7 }, (_, i) => {
      const d = offsetDate(ws, i);
      const dt = new Date(d + 'T12:00:00');
      return {
        date: d,
        dayName: dt.toLocaleDateString('en-US', { weekday: 'short' }),
        dayNum: dt.toLocaleDateString('en-US', { day: 'numeric' }),
        monthName: dt.toLocaleDateString('en-US', { month: 'short' }),
        fullDayName: dt.toLocaleDateString('en-US', { weekday: 'long' }),
        isToday: d === todayDate,
        isWeekend: dt.getDay() === 0 || dt.getDay() === 6,
      };
    })
  );

  // Desktop board: an INFINITE rolling strip of day columns. Rather than a fixed
  // 7-day window that pages (and reloads/flashes) at week boundaries, we render a
  // contiguous range of day-offsets relative to the anchored `date` and grow it
  // on demand as the user scrolls toward either edge. The anchor (offset 0 —
  // today by default) is snapped to the left edge on (re)anchor, with a past
  // buffer scrolled off to the left so you can immediately scroll backwards.
  const WEEK = 7;
  const PAST_BUFFER = 7;     // day columns rendered before the anchor (left runway)
  const FUTURE_BUFFER = 21;  // day columns rendered after the anchor
  const COL_FALLBACK = 236;  // w-56 (224) + gap-3 (12); only used pre-layout

  // Inclusive start / exclusive end day-offsets from `date`. Grown by the scroll
  // handler; reset whenever the anchor changes.
  let colStart = $state(-PAST_BUFFER);
  let colEnd   = $state(FUTURE_BUFFER);
  // Offset of the left-most visible column — drives the header's date label so
  // it tracks what you've scrolled to, not just the anchor.
  let firstVisibleOffset = $state(0);

  const columns = $derived(
    Array.from({ length: colEnd - colStart }, (_, i) => {
      const off = colStart + i;
      const d = offsetDate(date, off);
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

  // Unique Mon–Sun week-starts spanned by a half-open offset range [from, to).
  function weeksForOffsets(from: number, to: number): string[] {
    const set = new Set<string>();
    for (let o = from; o < to; o++) set.add(weekStart(offsetDate(date, o)));
    return [...set];
  }

  // Mobile: current selected day info
  const selectedDay = $derived(weekDays.find(d => d.date === date) ?? weekDays[0]);

  // Mobile: a wide, horizontally-scrollable day strip centred on the selected
  // date so you can swipe left/right and tap any day with your finger.
  const STRIP_RANGE = 21; // days on each side of the selected date
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
        isFirstOfMonth: dt.getDate() === 1,
      };
    })
  );

  // Keep the selected day centred in the strip whenever the date changes.
  let stripEl = $state<HTMLElement | undefined>();
  let stripReady = false;
  $effect(() => {
    date; // re-run when the selected day changes
    if (!stripEl) return;
    const chip = stripEl.querySelector<HTMLElement>(`[data-strip-date="${date}"]`);
    if (!chip) return;
    chip.scrollIntoView({ inline: 'center', block: 'nearest', behavior: stripReady ? 'smooth' : 'auto' });
    stripReady = true;
  });

  // ── Pomodoro update ───────────────────────────────────────────────────────
  // Reflect a just-logged session immediately: the confirmed time, and — when
  // the session was finished via "Done" — the completed status too, so the card
  // doesn't still look open. untrack keeps the effect keyed to lastTimeUpdate
  // only (reading+writing `tasks` untracked avoids the read-write $state loop).
  $effect(() => {
    const upd = pomodoro.lastTimeUpdate;
    if (!upd) return;
    untrack(() => {
      tasks = tasks.map(t => t.id === upd.taskId
        ? {
            ...t,
            time_actual_minutes: upd.newActual,
            ...(upd.done ? { status: 'done' as const, completed_at: t.completed_at ?? new Date().toISOString() } : {}),
          }
        : t);
    });
  });

  // ── Contextual day plan (intention / reflection) ───────────────────────────
  let dayPlan = $state<DailyPlan | null>(null);
  $effect(() => {
    const d = date;
    dayPlan = null;
    if (!prefs.contextualReflections) return;
    api.plans.get(d).then((p) => { if (d === date) dayPlan = p; }).catch(() => {});
  });
  const dayIntention = $derived(dayPlan?.plan_date === date ? dayPlan?.intention : null);
  const dayReflection = $derived(dayPlan?.plan_date === date ? dayPlan?.reflection : null);

  // ── Load ──────────────────────────────────────────────────────────────────
  // Tasks live in a single pool keyed by id; columns filter it by planned_date.
  // Because the pool is independent of the column offsets, changing the anchor
  // (navigating, clicking a calendar day) never has to clear it — that's what
  // kills the per-week "flash". We only fetch weeks we haven't loaded yet.
  let loadedWeeks = new Set<string>();

  // Fetch any of `weeks` not already loaded and merge them into the pool.
  async function ensureWeeks(weeks: string[], initial = false) {
    const missing = weeks.filter(w => !loadedWeeks.has(w));
    if (!missing.length) return;
    missing.forEach(w => loadedWeeks.add(w));
    if (initial) { loading = true; error = null; }
    try {
      const lists = await Promise.all(missing.map(w => api.tasks.listByWeek(w)));
      const byId = new Map(tasks.map(t => [t.id, t]));
      for (const list of lists) for (const t of list) byId.set(t.id, t);
      tasks = [...byId.values()];
    } catch (e) {
      missing.forEach(w => loadedWeeks.delete(w)); // allow a retry
      if (initial) error = e instanceof Error ? e.message : 'Failed';
    } finally {
      if (initial) loading = false;
    }
  }

  // Full reload of every week we've loaded (used by the error-retry button).
  async function loadTasks() {
    const weeks = loadedWeeks.size ? [...loadedWeeks] : weeksForOffsets(colStart, colEnd);
    loading = true; error = null;
    try {
      const lists = await Promise.all(weeks.map(w => api.tasks.listByWeek(w)));
      const byId = new Map<string, Task>();
      for (const list of lists) for (const t of list) byId.set(t.id, t);
      weeks.forEach(w => loadedWeeks.add(w));
      tasks = [...byId.values()];
    }
    catch (e) { error = e instanceof Error ? e.message : 'Failed'; }
    finally { loading = false; }
  }

  // Background refresh (no loading flag → no flash) for realtime/CRUD echoes.
  async function reloadLoaded() {
    const weeks = loadedWeeks.size ? [...loadedWeeks] : weeksForOffsets(colStart, colEnd);
    try {
      const lists = await Promise.all(weeks.map(w => api.tasks.listByWeek(w)));
      const byId = new Map<string, Task>();
      for (const list of lists) for (const t of list) byId.set(t.id, t);
      weeks.forEach(w => loadedWeeks.add(w));
      tasks = [...byId.values()];
    } catch { /* keep stale on failure */ }
  }

  async function loadRollover() {
    try {
      const prev = await api.tasks.listByDate(offsetDate(todayDate, -1));
      rolloverTasks = prev.filter(t => (t.status === 'planned' || t.status === 'in_progress') && !t.recurrence_origin_id);
    } catch { /* ignore */ }
  }

  onMount(() => { loadRollover(); timeInsights.ensure(); });

  // (Re)initialise the rolling board around the anchored date: reset the column
  // range, request its weeks, and flag that the scroll needs re-anchoring. Tasks
  // already in the pool are kept, so this is flash-free after the first load.
  let pendingAnchor = $state(true);
  $effect(() => {
    date; // re-run on anchor change
    colStart = -PAST_BUFFER;
    colEnd   = FUTURE_BUFFER;
    pendingAnchor = true;
    void ensureWeeks(weeksForOffsets(colStart, colEnd), tasks.length === 0);
  });

  // Once the board has rendered, snap the anchor (offset 0) to the left edge,
  // leaving PAST_BUFFER columns scrolled off to the left as backward runway.
  $effect(() => {
    if (mobile.value || loading || !weekGrid || !pendingAnchor) return;
    pendingAnchor = false;
    const stride = colStride();
    weekGrid.scrollLeft = (0 - colStart) * stride;
    firstVisibleOffset = 0;
  });

  // Re-fetch when another platform broadcasts a change
  $effect(() => {
    const ev = realtime.lastEvent;
    if (!ev) return;
    if (ev.type === 'task:change' || ev.type === 'objective:change') reloadLoaded();
  });

  // Handle FAB deep link — runs on mount AND whenever search params change
  // (same-page goto('/day/today?new=1') doesn't re-trigger onMount)
  $effect(() => {
    const newParam = $page.url.searchParams.get('new');
    if (!newParam) return;
    // Optional prefill, e.g. from a Share-to-Sempa intent (title + notes/URL).
    const t = $page.url.searchParams.get('title') ?? '';
    const n = $page.url.searchParams.get('notes') ?? '';
    openCreate(date, t, n);
    history.replaceState({}, '', $page.url.pathname);
  });

  async function rolloverAll() {
    const toRoll = rolloverTasks.filter(t => {
      // Skip recurring tasks where today already has an instance from the same series.
      if (!t.recurrence_origin_id) return true;
      return !tasks.some(
        existing => existing.recurrence_origin_id === t.recurrence_origin_id
                 && existing.planned_date === todayDate
                 && existing.id !== t.id
      );
    });
    await Promise.all(toRoll.map(t =>
      api.tasks.update(t.id, { planned_date: todayDate, week_start: weekStart(todayDate), status: 'planned' })
    ));
    rolloverTasks = []; await reloadLoaded();
  }

  // ── Tasks per day ──────────────────────────────────────────────────────────
  // ── Tag filter (in-place "filter mode") ──────────────────────────────────
  // Persisted so the chosen filter sticks as you move between days/weeks.
  let filterTags  = $state<string[]>([]);
  let filterMatch = $state<'any' | 'all'>('any');
  let showFilter  = $state(false);

  if (typeof localStorage !== 'undefined') {
    try {
      const raw = localStorage.getItem('sempa_tag_filter');
      if (raw) {
        const v = JSON.parse(raw);
        if (Array.isArray(v.tags)) filterTags = v.tags;
        if (v.match === 'all' || v.match === 'any') filterMatch = v.match;
        if (filterTags.length) showFilter = true;
      }
    } catch { /* ignore */ }
  }
  $effect(() => {
    if (typeof localStorage === 'undefined') return;
    localStorage.setItem('sempa_tag_filter', JSON.stringify({ tags: filterTags, match: filterMatch }));
  });

  function passesTagFilter(t: Task): boolean {
    if (filterTags.length === 0) return true;
    const tt = t.tags ?? [];
    return filterMatch === 'all'
      ? filterTags.every(f => tt.includes(f))
      : filterTags.some(f => tt.includes(f));
  }

  function dayTasks(d: string): Task[] {
    return tasks
      .filter(t => t.planned_date === d && t.status !== 'cancelled' && !t.parent_task_id)
      .filter(passesTagFilter)
      .sort(compareTasksForDay);
  }

  // Day stats for header — exclude sub-tasks (they nest inside their parent)
  const totalTasks   = $derived(tasks.filter(t => t.status !== 'cancelled' && !t.parent_task_id));
  const doneTasks    = $derived(totalTasks.filter(t => t.status === 'done').length);
  const estimateMins = $derived(totalTasks.reduce((s, t) => s + (t.time_estimate_minutes ?? 0), 0));
  const actualMins   = $derived(totalTasks.reduce((s, t) => s + (t.time_actual_minutes ?? 0), 0));

  // Today-specific glance stats (desktop header)
  const todayBoard       = $derived(totalTasks.filter(t => t.planned_date === todayDate));
  const todayRemaining   = $derived(todayBoard.filter(t => t.status !== 'done').length);
  const todayRemainMins  = $derived(
    todayBoard.filter(t => t.status !== 'done').reduce((s, t) => s + (t.time_estimate_minutes ?? 0), 0)
  );

  // Sync task data to Android widgets whenever tasks change
  $effect(() => {
    if (tasks.length === 0 && loading) return;
    const todayList = tasks.filter(t => t.planned_date === todayDate && t.status !== 'cancelled');
    const weekCounts = new Map<string, number>();
    for (const t of tasks) {
      if (t.status === 'cancelled') continue;
      if (t.planned_date) weekCounts.set(t.planned_date, (weekCounts.get(t.planned_date) ?? 0) + 1);
    }
    syncWidgetData(todayList, weekCounts);
  });

  // Mobile: stats for selected day
  const mobileDayTasks  = $derived(dayTasks(date));
  const mobileActive    = $derived(mobileDayTasks.filter(t => t.status !== 'done'));
  const mobileDone      = $derived(mobileDayTasks.filter(t => t.status === 'done'));
  const mobileDayEstimate = $derived(mobileDayTasks.reduce((s, t) => s + (t.time_estimate_minutes ?? 0), 0));
  const mobileEffectivePlanned = $derived(plannedMinutes(mobileDayTasks, undefined, timeTracking.capacityRealistic));
  const mobileOverCapacity = $derived(timeTracking.capacityEnabled && mobileEffectivePlanned > timeTracking.capacityMinutes);

  // ── Mobile long-press drag-to-reorder ──────────────────────────────────────
  // While a card is picked up we drive a live order of ids (reorderOrder) so the
  // list reshuffles under the finger; on release we renormalise positions and
  // persist. `mobileActiveOrdered` is what the list renders.
  let mobileListEl  = $state<HTMLElement | undefined>();
  let reorderId     = $state<string | null>(null);
  let reorderOrder  = $state<string[] | null>(null);

  const mobileActiveOrdered = $derived.by(() => {
    if (!reorderOrder) return mobileActive;
    const byId = new Map(mobileActive.map(t => [t.id, t]));
    const ordered = reorderOrder.map(id => byId.get(id)).filter((t): t is Task => !!t);
    // Include any card that appeared mid-drag (shouldn't normally happen) so
    // nothing is dropped from the list.
    for (const t of mobileActive) if (!reorderOrder.includes(t.id)) ordered.push(t);
    return ordered;
  });

  function mobileReorderStart(id: string) {
    reorderId = id;
    reorderOrder = mobileActive.map(t => t.id);
  }

  function mobileReorderMove(clientY: number) {
    if (!reorderId || !reorderOrder || !mobileListEl) return;
    const els = Array.from(mobileListEl.querySelectorAll<HTMLElement>('[data-task-id]'));
    let target = reorderOrder.length - 1;
    for (let i = 0; i < els.length; i++) {
      const r = els[i].getBoundingClientRect();
      if (clientY < r.top + r.height / 2) { target = i; break; }
    }
    const from = reorderOrder.indexOf(reorderId);
    if (from === -1 || from === target) return;
    const next = reorderOrder.slice();
    next.splice(from, 1);
    next.splice(target, 0, reorderId);
    reorderOrder = next;
  }

  async function mobileReorderEnd() {
    const order = reorderOrder;
    reorderId = null;
    reorderOrder = null;
    if (!order) return;

    const newPosOf = new Map<string, number>();
    order.forEach((id, i) => newPosOf.set(id, (i + 1) * 1000));

    const prev = tasks.slice();
    const changed = prev.filter(t => newPosOf.has(t.id) && newPosOf.get(t.id) !== t.position);
    if (changed.length === 0) return;

    tasks = tasks.map(t => newPosOf.has(t.id) ? { ...t, position: newPosOf.get(t.id)! } : t);
    try {
      const updated = await Promise.all(
        changed.map(t => api.tasks.update(t.id, { position: newPosOf.get(t.id)! })),
      );
      const byId = new Map(updated.map(t => [t.id, t]));
      tasks = tasks.map(t => byId.get(t.id) ?? t);
    } catch (e: any) { tasks = prev; showDropError(e?.message || 'Could not reorder tasks'); }
  }

  // ── Infinite board scroll ──────────────────────────────────────────────────
  // Measure the real per-column stride from the DOM so prepend compensation and
  // week jumps stay exact under browser zoom (rem-based widths drift otherwise).
  function colStride(): number {
    const el = weekGrid?.querySelector('[data-daycol]');
    return el instanceof HTMLElement ? el.offsetWidth + 12 : COL_FALLBACK;
  }

  // Grow the rendered range as the user nears either edge. Prepending shifts all
  // columns right, so we add the same number of pixels back to scrollLeft to
  // keep the view visually still — no jump, no flash, infinite in both directions.
  let extending = false;
  function onBoardScroll() {
    if (!weekGrid) return;
    const stride = colStride();
    const { scrollLeft, scrollWidth, clientWidth } = weekGrid;
    firstVisibleOffset = colStart + Math.round(scrollLeft / stride);
    if (extending) return;
    const edge = stride * 3;
    if (scrollWidth - clientWidth - scrollLeft < edge) {
      extending = true;
      const newEnd = colEnd + WEEK;
      void ensureWeeks(weeksForOffsets(colEnd, newEnd));
      colEnd = newEnd;
      tick().then(() => { extending = false; });
    } else if (scrollLeft < edge) {
      extending = true;
      const newStart = colStart - WEEK;
      const added = colStart - newStart;
      void ensureWeeks(weeksForOffsets(newStart, colStart));
      colStart = newStart;
      tick().then(() => {
        if (weekGrid) weekGrid.scrollLeft += added * stride;
        extending = false;
      });
    }
  }

  // Header ‹ › and arrow keys: smooth-scroll the board by a week (no navigation,
  // so no flash). Pre-extend the range first so the smooth scroll has runway.
  function scrollWeeks(dir: 1 | -1) {
    if (mobile.value) { navigateDay(dir); return; }
    if (!weekGrid) return;
    const stride = colStride();
    if (dir > 0) {
      const room = weekGrid.scrollWidth - weekGrid.clientWidth - weekGrid.scrollLeft;
      if (room < WEEK * stride + stride) {
        const ne = colEnd + WEEK * 2;
        void ensureWeeks(weeksForOffsets(colEnd, ne));
        colEnd = ne;
      }
      tick().then(() => weekGrid?.scrollBy({ left: WEEK * stride, behavior: 'smooth' }));
    } else if (weekGrid.scrollLeft < WEEK * stride + stride) {
      const ns = colStart - WEEK * 2;
      const added = colStart - ns;
      void ensureWeeks(weeksForOffsets(ns, colStart));
      colStart = ns;
      tick().then(() => {
        if (!weekGrid) return;
        weekGrid.scrollLeft += added * stride;
        weekGrid.scrollBy({ left: -WEEK * stride, behavior: 'smooth' });
      });
    } else {
      weekGrid.scrollBy({ left: -WEEK * stride, behavior: 'smooth' });
    }
  }

  // "Today": re-anchor (which snaps today to the left) when off-day; if already
  // anchored on today, just smooth-scroll today's column back to the left edge.
  function goToday() {
    if (date !== todayDate) { goto(`/day/${todayDate}`); return; }
    if (!weekGrid) return;
    const stride = colStride();
    weekGrid.scrollTo({ left: (0 - colStart) * stride, behavior: 'smooth' });
  }

  function handleCalendarDateClick(d: string) {
    goto(`/day/${d}`);
  }

  // ── Mobile day navigation ────────────────────────────────────────────────
  function navigateDay(delta: number) {
    goto(`/day/${offsetDate(date, delta)}`);
  }

  // ── Drag & drop between days ───────────────────────────────────────────────
  function handleDragStart(id: string) { draggingId = id; }

  async function handleDrop(targetDate: string, insertIdx?: number) {
    if (!draggingId) return;
    const id = draggingId;
    draggingId = null; dragOverDate = null;

    // The card may not be in the week-loaded pool yet — e.g. an unplanned Jira
    // backlog task dragged in from the Jira panel. Fetch it so the drop still
    // lands (and the task gets added to the board below).
    let task = tasks.find(t => t.id === id);
    const inPool = !!task;
    if (!task) {
      try { task = await api.tasks.get(id); }
      catch (e: any) { showDropError(e?.message || 'Could not add that item'); return; }
    }

    const newStatus = task.status === 'backlog' ? 'planned' : task.status;

    // Reorder among the day's ACTIVE list (the same set the column renders with
    // [data-task-idx]); completed tasks keep their own order. We rebuild that
    // list with the moved card spliced in at the drop point, then hand every
    // card a fresh, evenly-spaced position. Renormalising (rather than computing
    // a single midpoint) is what makes "reorder within a day" actually stick:
    // many tasks ship with position 0, so midpoints between equal neighbours
    // collapsed to the same value and nothing visibly moved.
    const others = tasks
      .filter(t => t.planned_date === targetDate && t.status !== 'cancelled'
                   && t.status !== 'done' && t.id !== id)
      .sort(compareTasksForDay);

    // The drop index is measured against the rendered active list, which on a
    // same-day move still includes the dragged card. Once we remove it, every
    // slot below its old position shifts up by one — so decrement to compensate.
    let idx = insertIdx ?? others.length;
    if (inPool && task.planned_date === targetDate && task.status !== 'done') {
      const prevIdx = tasks
        .filter(t => t.planned_date === targetDate && t.status !== 'cancelled'
                     && t.status !== 'done')
        .sort(compareTasksForDay)
        .findIndex(t => t.id === id);
      if (prevIdx !== -1 && idx > prevIdx) idx -= 1;
    }
    idx = Math.max(0, Math.min(idx, others.length));

    const moved = { ...task, planned_date: targetDate, status: newStatus };
    const orderedIds = [...others.slice(0, idx), moved, ...others.slice(idx)].map(t => t.id);
    const newPosOf = new Map<string, number>();
    orderedIds.forEach((tid, i) => newPosOf.set(tid, (i + 1) * 1000));
    const movedPos = newPosOf.get(id)!;

    // Optimistic: apply the new positions (and the moved card's new day/status)
    // locally so the board reorders instantly.
    const prev = tasks.slice();
    const applyLocal = (t: Task): Task => {
      if (t.id === id) return { ...moved, position: movedPos };
      const p = newPosOf.get(t.id);
      return p !== undefined ? { ...t, position: p } : t;
    };
    tasks = inPool
      ? tasks.map(applyLocal)
      : [...tasks, { ...moved, position: movedPos }];

    try {
      // Persist the moved card (it also changes day/status), plus any sibling
      // whose position actually shifted. Columns are small, so a handful of
      // PATCHes in parallel is cheap and keeps every client's order identical.
      const movedReq = api.tasks.update(id, {
        planned_date: targetDate,
        week_start: weekStart(targetDate),
        position: movedPos,
        status: newStatus,
      });
      const siblingReqs = others
        .filter(t => newPosOf.get(t.id) !== t.position)
        .map(t => api.tasks.update(t.id, { position: newPosOf.get(t.id)! }));

      const [updatedMoved, ...updatedSiblings] = await Promise.all([movedReq, ...siblingReqs]);
      const byId = new Map<string, Task>([updatedMoved, ...updatedSiblings].map(t => [t.id, t]));
      tasks = tasks.map(t => byId.get(t.id) ?? t);
    } catch (e: any) { tasks = prev; showDropError(e?.message || 'Could not move that task'); }
  }

  // ── Complete ──────────────────────────────────────────────────────────────
  // Swipe-left on a mobile card → bump the task to the next day. Optimistic: it
  // leaves this day's board immediately; reverts on failure.
  async function handleReschedule(id: string) {
    const task = tasks.find(t => t.id === id);
    if (!task) return;
    const tomorrow = offsetDate(date, 1);
    const prev = tasks.slice();
    tasks = tasks.filter(t => t.id !== id);
    try {
      await api.tasks.update(id, { planned_date: tomorrow, week_start: weekStart(tomorrow) });
    } catch { tasks = prev; }
  }

  async function handleComplete(id: string) {
    const task = tasks.find(t => t.id === id);
    if (!task) return;
    const newStatus = task.status === 'done' ? 'planned' : 'done';
    const prev = tasks.slice();
    tasks = tasks.map(t => t.id === id ? { ...t, status: newStatus } : t);
    // Calm celebration — Tier 1 on completion, escalating to a Tier 2 "day
    // complete" moment when this was the last open task for its day.
    if (newStatus === 'done') celebrateCompletion(task);
    try {
      const updated = await api.tasks.update(id, {
        status: newStatus,
        completed_at: newStatus === 'done' ? new Date().toISOString() : null,
      });
      tasks = tasks.map(t => t.id === updated.id ? updated : t);
      syncLinkedObjective(task.weekly_objective_id);
    } catch { tasks = prev; }
  }

  // Fire the celebration layer for a freshly-completed task. Reads the live DOM
  // for the card / day-column origin so particles bloom from the right place.
  function celebrateCompletion(task: Task) {
    const cardEl = document.querySelector(`[data-task-id="${task.id}"]`);
    celebrate.task(cardEl);

    const d = task.planned_date;
    if (!d) return;
    // `tasks` already carries the optimistic completion, so this reflects the
    // day's state the user is about to see.
    const dayList = dayTasks(d);
    if (dayList.length === 0 || !dayList.every(t => t.status === 'done')) return;

    const label = new Date(d + 'T12:00:00')
      .toLocaleDateString('en-US', { weekday: 'long' }).toLowerCase();
    const logged = dayList.reduce((s, t) => s + (t.time_actual_minutes ?? 0), 0);
    const copy = `${label} complete · ${dayList.length} done`
      + (logged > 0 ? ` · ${formatMinutes(logged)} logged` : '');

    // Light sweep across that day's progress bar as it reaches 100%.
    const track = document.querySelector(`#day-col-${d} .day-bar-track`);
    if (track) {
      track.classList.add('sc-sweep');
      setTimeout(() => track.classList.remove('sc-sweep'), 760);
    }
    // Let Tier 1 land first, then the day moment.
    const colEl = document.getElementById(`day-col-${d}`);
    setTimeout(() => celebrate.day(colEl, copy), 200);
  }

  // Keep a linked objective in step with its tasks: when every task linked to it
  // is done the objective auto-completes; if one is reopened it returns to
  // active. For an objective dragged in as a single task, completing that task
  // completes the objective — the behaviour the linked-task workflow expects.
  function syncLinkedObjective(objectiveId: string | null) {
    if (!objectiveId) return;
    const linked = tasks.filter(t => t.weekly_objective_id === objectiveId && t.status !== 'cancelled');
    if (linked.length === 0) return;
    const status = linked.every(t => t.status === 'done') ? 'completed' : 'active';
    api.objectives.update(objectiveId, { status }).catch(() => {});
  }

  // ── Drag a weekly objective onto a day → create a linked task there ─────────
  async function handleObjectiveDrop(obj: { id: string; title: string }, targetDate: string) {
    dragOverDate = null;
    const colTasks = tasks.filter(t => t.planned_date === targetDate && t.status !== 'cancelled');
    const pos = appendPosition(colTasks.map(t => t.position));
    try {
      const t = await api.tasks.create({
        title: obj.title,
        weekly_objective_id: obj.id,
        planned_date: targetDate,
        week_start: weekStart(targetDate),
        status: 'planned',
        position: pos,
      });
      tasks = [...tasks, t];
    } catch (e) { error = e instanceof Error ? e.message : 'Failed to add task'; }
  }

  // ── Focus (Pomodoro) ──────────────────────────────────────────────────────
  function handleFocus(id: string, title: string) {
    const t = tasks.find(t => t.id === id);
    pomodoro.start(id, title, t?.time_actual_minutes ?? 0, t?.time_estimate_minutes ?? null);
  }

  // ── Focus mode (full-screen) ───────────────────────────────────────────────
  function handleFocusMode(id: string) {
    goto(`/focus/${id}`);
  }

  // ── Hover tracking for keyboard shortcut ─────────────────────────────────
  let hoveredTaskId = $state<string | null>(null);
  function handleTaskHover(id: string | null) { hoveredTaskId = id; }

  // ── Keyboard shortcut: n = new task, e = edit hovered ────────────────────
  function handleKeydown(e: KeyboardEvent) {
    const tgt = e.target as HTMLElement;
    if (tgt.tagName === 'INPUT' || tgt.tagName === 'TEXTAREA' || tgt.isContentEditable) return;
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (e.key === 'n' && !panelOpen) { e.preventDefault(); openCreate(date); }
    if (e.key === 'e' && !panelOpen && hoveredTaskId) {
      e.preventDefault();
      const t = tasks.find(t => t.id === hoveredTaskId);
      if (t) openEdit(t);
    }
    // Arrow keys scroll the board by a week. Skipped above when focus is in an
    // input/textarea/contenteditable.
    if (e.key === 'ArrowLeft' && !panelOpen)  { e.preventDefault(); scrollWeeks(-1); }
    if (e.key === 'ArrowRight' && !panelOpen) { e.preventDefault(); scrollWeeks(1); }
  }

  // ── Trash (with confirm modal) ──────────────────────────────────────────
  let trashConfirmOpen  = $state(false);
  let trashTaskId       = $state<string | null>(null);
  let trashTaskTitle    = $state('');

  function handleTrashRequest(id: string, title: string) {
    trashTaskId = id;
    trashTaskTitle = title;
    trashConfirmOpen = true;
  }

  async function confirmTrash() {
    if (!trashTaskId) return;
    const id = trashTaskId;
    const removed = tasks.find(t => t.id === id) ?? null;
    const prev = tasks.slice();
    tasks = tasks.filter(t => t.id !== id);
    trashConfirmOpen = false;
    trashTaskId = null;
    try { await api.tasks.delete(id); if (removed) showUndo(removed); }
    catch { tasks = prev; }
  }

  // ── Undo a delete ───────────────────────────────────────────────────────────
  // Non-Jira tasks are recreated from the captured snapshot. Jira tasks aren't
  // recreated here (that would duplicate the ticket) — they re-import via the
  // backend re-sync, so the toast just says so.
  let undoTask = $state<Task | null>(null);
  let undoTimer: ReturnType<typeof setTimeout> | null = null;
  function showUndo(task: Task) {
    undoTask = task;
    if (undoTimer) clearTimeout(undoTimer);
    undoTimer = setTimeout(() => { undoTask = null; }, 8000);
  }
  async function doUndo() {
    const t = undoTask;
    undoTask = null;
    if (!t || t.source === 'jira') return;
    try {
      const recreated = await api.tasks.create({
        title: t.title,
        description: t.description ?? undefined,
        planned_date: t.planned_date ?? undefined,
        week_start: t.week_start ?? undefined,
        status: t.status,
        position: t.position,
        time_estimate_minutes: t.time_estimate_minutes ?? undefined,
        weekly_objective_id: t.weekly_objective_id ?? undefined,
        tags: t.tags,
        scheduled_start: t.scheduled_start ?? undefined,
        scheduled_end: t.scheduled_end ?? undefined,
        roughly_at: t.roughly_at,
      });
      tasks = [...tasks, recreated];
    } catch (e: any) { showDropError(e?.message || 'Could not restore the task'); }
  }

  function cancelTrash() {
    trashConfirmOpen = false;
    trashTaskId = null;
  }

  // ── Panel ─────────────────────────────────────────────────────────────────
  function openCreate(d: string, prefillTitle = '', prefillNotes = '') {
    panelTask = null; panelStatus = 'planned'; panelDate = d;
    panelTitle = prefillTitle; panelNotes = prefillNotes; panelOpen = true;
  }
  function openEdit(task: Task) { panelTask = task; panelOpen = true; }

  async function handlePanelSave(saved: Task) {
    panelOpen = false;
    if (saved.status === 'cancelled') { tasks = tasks.filter(t => t.id !== saved.id); return; }
    if (!panelTask && saved.recurrence_rule) { await reloadLoaded(); return; }
    const idx = tasks.findIndex(t => t.id === saved.id);
    if (idx >= 0) tasks = tasks.map(t => t.id === saved.id ? saved : t);
    else tasks = [...tasks, saved];
  }

  // ── Email drop ────────────────────────────────────────────────────────────
  async function handleEmailDrop(emailData: { id: string; subject: string }, targetDate: string) {
    try {
      // toTask now creates the task already planned for the dropped day, so no
      // fragile follow-up update (which 404'd on the just-created task).
      const task = await api.integrations.fastmail.toTask(emailData.id, emailData.subject, targetDate);
      tasks = tasks.some(t => t.id === task.id) ? tasks.map(t => t.id === task.id ? task : t) : [...tasks, task];
      emailPanel?.removeEmail(emailData.id);
    } catch (e: any) {
      // Surface it: the page-level `error` only renders by replacing the whole
      // board, so on a populated board a failed email→task looked like nothing
      // happened. Show a transient toast instead.
      showDropError(e?.message || 'Could not create a task from that email');
    }
  }

  // ── Transient drop error toast (email/jira drops fail far from the board) ───
  let dropError = $state<string | null>(null);
  let dropErrorTimer: ReturnType<typeof setTimeout> | null = null;
  function showDropError(msg: string) {
    dropError = msg;
    if (dropErrorTimer) clearTimeout(dropErrorTimer);
    dropErrorTimer = setTimeout(() => { dropError = null; }, 6000);
  }

  // ── Calendar schedule / unschedule ────────────────────────────────────────
  // Placing or resizing a block keeps the task and its calendar block in sync:
  // the drop day becomes the task's planned_date (so it moves into that day's
  // column), and the block length is written back as the task's planned time
  // (time_estimate_minutes) so a resize updates the estimate and vice-versa.
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

  // Heading — the 7 days starting at whatever's scrolled to the left edge, so it
  // reflects where you are in the infinite strip rather than a fixed anchor.
  const firstVisibleDate = $derived(offsetDate(date, firstVisibleOffset));
  function weekLabel(): string {
    const start = new Date(firstVisibleDate + 'T00:00:00');
    const endDt = new Date(offsetDate(firstVisibleDate, 6) + 'T00:00:00');
    const mo = (d: Date) => d.toLocaleDateString('en-US', { month: 'short' });
    const dy = (d: Date) => d.getDate();
    if (start.getMonth() === endDt.getMonth()) return `${mo(start)} ${dy(start)}–${dy(endDt)}`;
    return `${mo(start)} ${dy(start)} – ${mo(endDt)} ${dy(endDt)}`;
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<svelte:head>
  <title>{isToday(date) ? 'Today' : weekLabel()} — Sempa</title>
</svelte:head>

<!-- ═══════════════════════════════════════════════════════════════════════ -->
<!-- MOBILE LAYOUT                                                          -->
<!-- ═══════════════════════════════════════════════════════════════════════ -->
{#if mobile.value}

  <!-- Mobile header -->
  <header class="sticky top-0 z-[40] px-5 pt-4 pb-3"
          style="background: var(--sempa-bg-main); padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px));">
    <div class="flex items-center justify-between mb-1">
      <button onclick={() => navigateDay(-1)} aria-label="Previous day"
              class="flex h-10 w-10 items-center justify-center rounded-xl transition-colors
                     active:bg-gray-100 dark:active:bg-gray-800"
              style="color: var(--sempa-text-dim);">
        <ChevronLeft size={20} />
      </button>
      <div class="text-center">
        <h1 style="font-size: 28px; font-weight: 600; letter-spacing: -0.025em; color: var(--sempa-text);">
          {selectedDay.isToday ? 'Today' : selectedDay.fullDayName + ', ' + selectedDay.dayNum}
        </h1>
        {#if !selectedDay.isToday}
          <p class="text-xs" style="color: var(--sempa-text-dim);">{selectedDay.monthName}</p>
        {/if}
      </div>
      <button onclick={() => navigateDay(1)} aria-label="Next day"
              class="flex h-10 w-10 items-center justify-center rounded-xl transition-colors
                     active:bg-gray-100 dark:active:bg-gray-800"
              style="color: var(--sempa-text-dim);">
        <ChevronRight size={20} />
      </button>
    </div>

    <!-- Quick date strip — horizontally scrollable; swipe to browse, tap to pick -->
    <div bind:this={stripEl}
         class="no-scrollbar -mx-5 mt-2 flex snap-x snap-proximity gap-1 overflow-x-auto scroll-px-5 px-5"
         style="-webkit-overflow-scrolling: touch; overscroll-behavior-x: contain;">
      {#each stripDays as day (day.date)}
        {@const isSel = day.date === date}
        <button onclick={() => { hapticTick(); goto(`/day/${day.date}`); }}
                data-strip-date={day.date}
                aria-current={isSel ? 'date' : undefined}
                class="flex shrink-0 snap-center flex-col items-center gap-0.5 rounded-xl px-2.5 py-1.5 transition-colors"
                style={isSel
                  ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                  : day.isToday
                    ? 'color: var(--sempa-accent);'
                    : day.isWeekend
                      ? 'color: var(--sempa-text-dim); opacity: 0.7;'
                      : 'color: var(--sempa-text-dim);'}>
          <span class="text-[10.5px] font-semibold uppercase">{day.dayName}</span>
          <span class="flex h-7 w-7 items-center justify-center rounded-full text-[13px] font-semibold"
                style={day.isToday && !isSel
                  ? 'background: var(--sempa-today-bg); color: var(--sempa-today-fg);'
                  : ''}>
            {day.dayNum}
          </span>
        </button>
      {/each}
    </div>

    <!-- Day stats -->
    {#if mobileDayTasks.length > 0}
      <div class="flex items-center gap-3 mt-2 text-[11px]" style="color: var(--sempa-text-dim);">
        <span>{mobileDone.length}/{mobileDayTasks.length} done</span>
        {#if mobileDayEstimate > 0}<span style={mobileOverCapacity ? 'color:#b07d18;' : ''}>{formatMinutes(mobileDayEstimate)} planned{#if mobileOverCapacity} · over{/if}</span>{/if}
      </div>
    {/if}
  </header>

  <!-- Tag filter (in-place) -->
  {#if tagStore.definitions.length}
    <div class="px-4 pb-1">
      <button onclick={() => showFilter = !showFilter}
              class="inline-flex items-center gap-1.5 rounded-full transition-colors"
              style="font-size: 12px; padding: 4px 10px;
                     {filterTags.length
                       ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                       : 'color: var(--sempa-text-dim); box-shadow: inset 0 0 0 1px var(--sempa-border);'}">
        <SlidersHorizontal size={13} />
        {filterTags.length ? `Filtered · ${filterTags.length}` : 'Filter by tag'}
      </button>
      {#if showFilter}
        <div class="mt-2">
          <TagFilterBar bind:selected={filterTags} bind:match={filterMatch} />
        </div>
      {/if}
    </div>
  {/if}

  <!-- Contextual intention (mobile) -->
  {#if prefs.contextualReflections}
    <div class="px-4 mb-3">
      <ReflectionCard date={date} intention={dayIntention} show="intention" promptWhenEmpty={isToday(date)} />
    </div>
  {/if}

  <!-- Rollover banner (mobile) -->
  {#if rolloverTasks.length > 0 && !rolloverDismissed && isToday(date)}
    <div class="mx-4 mb-3 flex items-center gap-2 rounded-xl px-3 py-2.5 animate-slide-down"
         style="border: 1px solid var(--sempa-amber); background: color-mix(in srgb, var(--sempa-amber) 8%, var(--sempa-bg-main));">
      <p class="flex-1 text-xs" style="color: var(--sempa-amber);">
        <strong>{rolloverTasks.length}</strong> from yesterday
      </p>
      <button onclick={rolloverAll}
              class="rounded-lg px-2.5 py-1 text-[11px] font-medium"
              style="background: var(--sempa-amber); color: var(--sempa-btn-fg);">
        Roll over
      </button>
      <button onclick={() => rolloverDismissed = true} aria-label="Dismiss"
              style="color: var(--sempa-amber); opacity: 0.7;">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>
  {/if}

  <!-- Mobile task list -->
  <main class="px-4 pb-24 animate-fade-in"
        use:swipeNavigate={{ onPrev: () => navigateDay(-1), onNext: () => navigateDay(1) }}>
    {#if loading}
      <div class="flex h-48 items-center justify-center text-sm" style="color: var(--sempa-text-dim);">Loading...</div>
    {:else if error}
      <div class="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-600
                  dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-400">
        {error} <button onclick={loadTasks} class="ml-2 underline">Retry</button>
      </div>
    {:else if mobileDayTasks.length === 0}
      <div class="flex flex-col items-center justify-center py-16 gap-3">
        <div class="h-12 w-12 rounded-full flex items-center justify-center"
             style="background: var(--sempa-accent-bg);">
          <Plus size={20} style="color: var(--sempa-accent);" />
        </div>
        <p class="text-sm" style="color: var(--sempa-text-dim);">No tasks for this day</p>
        <button onclick={() => openCreate(date)}
                class="rounded-[9px] px-4 py-2 text-[13px] font-medium"
                style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
          Add task
        </button>
      </div>
    {:else}
      <!-- Active tasks — long-press a card to drag it into priority order -->
      <div bind:this={mobileListEl} class="flex flex-col gap-2">
        {#each mobileActiveOrdered as task (task.id)}
          <div animate:flip={{ duration: 200 }}>
            <MobileTaskCard
              {task}
              onComplete={handleComplete}
              onReschedule={handleReschedule}
              onTrash={handleTrashRequest}
              onClick={openMobileView}
              onFocusClick={handleFocus}
              onReorderStart={mobileReorderStart}
              onReorderMove={mobileReorderMove}
              onReorderEnd={mobileReorderEnd}
              dragging={reorderId === task.id}
            />
          </div>
        {/each}
      </div>

      <!-- Completed tasks -->
      {#if mobileDone.length > 0}
        <div class="mt-4 pt-3" style="border-top: 1px solid var(--sempa-border);">
          <p class="mb-2 text-[11px] font-medium uppercase tracking-wider" style="color: var(--sempa-text-dim);">
            {mobileDone.length} completed
          </p>
          <div class="flex flex-col gap-1.5">
            {#each mobileDone as task (task.id)}
              <MobileTaskCard
                {task}
                onComplete={handleComplete}
                onTrash={handleTrashRequest}
                onClick={openMobileView}
              />
            {/each}
          </div>
        </div>
      {/if}
    {/if}

    <!-- Contextual reflection (mobile) -->
    {#if prefs.contextualReflections && !loading}
      <div class="mt-4">
        <ReflectionCard date={date} reflection={dayReflection} show="reflection" promptWhenEmpty={isToday(date)} />
      </div>
    {/if}
  </main>

  <!-- Mobile task detail view: read-first, Edit button opens full TaskPanel -->
  <MobileTaskView
    open={mobileViewOpen}
    task={mobileViewTask}
    onClose={() => mobileViewOpen = false}
    onEdit={() => { const t = mobileViewTask; mobileViewOpen = false; if (t) openEdit(t); }}
    onComplete={handleComplete}
    onDelete={handleTrashRequest}
    onFocusStart={handleFocus}
  />

  <!-- TaskPanel handles its own mobile bottom sheet -->
  <TaskPanel open={panelOpen} task={panelTask} defaultStatus={panelStatus} defaultDate={panelDate}
             defaultTitle={panelTitle} defaultNotes={panelNotes}
             onSave={handlePanelSave} onClose={() => panelOpen = false} />

<!-- ═══════════════════════════════════════════════════════════════════════ -->
<!-- DESKTOP LAYOUT (unchanged)                                             -->
<!-- ═══════════════════════════════════════════════════════════════════════ -->
{:else}

<!-- ── Header ─────────────────────────────────────────────────────────────── -->
<header class="sticky top-0 z-[40] backdrop-blur-sm"
        style="background: color-mix(in srgb, var(--sempa-bg-main) 95%, transparent);
               border-bottom: 1px solid var(--sempa-border);
               padding-top: max(12px, calc(env(safe-area-inset-top, 0px) + 8px));">
  <div class="flex items-center justify-between gap-3 px-6 py-3">
    <!-- Week nav — highest priority; never shrinks -->
    <div class="flex shrink-0 items-center gap-2">
      <button onclick={() => scrollWeeks(-1)} aria-label="Previous week"
              class="rounded-lg p-1.5 transition-colors"
              style="color: var(--sempa-text-dim);">
        <ChevronLeft size={16} />
      </button>
      <div>
        <p class="type-date" style="color: var(--sempa-text);">{weekLabel()}</p>
        {#if isToday(date)}
          <p class="type-label" style="color: var(--sempa-accent);">This week</p>
        {/if}
      </div>
      <button onclick={() => scrollWeeks(1)} aria-label="Next week"
              class="rounded-lg p-1.5 transition-colors"
              style="color: var(--sempa-text-dim);">
        <ChevronRight size={16} />
      </button>
    </div>

    <!-- The toolbar holds only nav, stats, and actions — the daily quote moved to
         the intention card (a quiet companion to planning the day), so nothing
         competes for this contested horizontal space. -->
    <div class="min-w-0 flex-auto"></div>

    <!-- Stats — uniform size/colour across the row (spec 4g) -->
    {#if !loading && totalTasks.length > 0}
      <div class="hidden md:flex shrink-0 items-center gap-4" style="font-size: 12.5px; color: var(--sempa-text-soft);">
        {#if isToday(date)}
          <span class="flex items-center gap-1.5" style="color: var(--sempa-text-soft);">
            <span class="inline-flex h-1.5 w-1.5 rounded-full" style="background: var(--sempa-accent);"></span>
            {#if todayRemaining > 0}
              <strong style="color: var(--sempa-text);">{todayRemaining}</strong> left today
              {#if todayRemainMins > 0}· ~{formatMinutes(todayRemainMins)}{/if}
            {:else if todayBoard.length > 0}
              All done today 🎉
            {:else}
              Nothing scheduled today
            {/if}
          </span>
          <span style="opacity:0.4;">|</span>
        {/if}
        <span>{doneTasks}/{totalTasks.length} done this week</span>
        {#if estimateMins > 0}<span>~{formatMinutes(estimateMins)} planned</span>{/if}
        {#if actualMins > 0}<span style="color: var(--sempa-accent);">{formatMinutes(actualMins)} logged</span>{/if}
      </div>
    {/if}

    <!-- Actions — never shrink; Today pill + New task keep full size -->
    <div class="flex shrink-0 items-center gap-2">
      <!-- Always present: jump to (and centre) today. When already on this week
           it re-centres today's column; otherwise it navigates to today. -->
      <button onclick={goToday}
              title={onToday ? 'Jump to today' : 'Go to today'}
              class="flex items-center gap-1.5 font-medium"
              style="border: 1px solid {onToday ? 'var(--sempa-accent)' : 'var(--sempa-border)'};
                     color: {onToday ? 'var(--sempa-accent)' : 'var(--sempa-text-soft)'};
                     background: {onToday ? 'var(--sempa-accent-bg)' : 'transparent'};
                     border-radius: 9px; padding: 6px 12px;
                     font-size: 12px; cursor: pointer; transition: all 150ms ease;"
              onmouseenter={(e) => { if (!onToday) (e.currentTarget as HTMLElement).style.background = 'var(--sempa-accent-bg)'; }}
              onmouseleave={(e) => { if (!onToday) (e.currentTarget as HTMLElement).style.background = 'transparent'; }}>
        <span class="inline-flex h-1.5 w-1.5 rounded-full"
              style="background: {onToday ? 'var(--sempa-accent)' : 'var(--sempa-text-dim)'};"></span>
        Today
      </button>
      <button onclick={() => openCreate(date)}
              class="flex items-center gap-1.5 rounded-[9px] px-3 py-1.5 text-[13px] font-[500]
                     tracking-[-0.01em] transition-colors shadow-sm"
              style="background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);"
              onmouseenter={(e)=>(e.currentTarget as HTMLElement).style.opacity='0.88'}
              onmouseleave={(e)=>(e.currentTarget as HTMLElement).style.opacity='1'}>
        <Plus size={13} strokeWidth={2.5} />
        New task
      </button>
    </div>
  </div>
</header>

<!-- ── Body ───────────────────────────────────────────────────────────────── -->
<!-- Height excludes the desktop header (57px) AND the custom titlebar (--app-titlebar-h,
     38px on the Tauri desktop, 0 on web) so the planner fills the window exactly
     instead of overflowing by the titlebar height (which caused stray v+h scrollbars). -->
<div class="flex overflow-hidden" style="height: calc(100vh - 57px - var(--app-titlebar-h, 0px));">

  <!-- Kanban area. A flex COLUMN that fills the viewport height: the filter /
       banner sit at the top (fixed), and the board below grows to fill the rest
       so its horizontal scrollbar lands at the very BOTTOM of the page rather
       than floating halfway up where the tallest column happens to end. -->
  <main bind:this={kanbanScroll} class="flex-1 flex flex-col overflow-hidden px-4 pt-5 pb-2 animate-fade-in">

    <!-- Intention card — and, until an intention is set, the quiet home for the
         daily quote (reflection paired with planning the day). A set reflection
         still shows beneath it; the end-of-day prompt lives in the shutdown ritual. -->
    {#if prefs.contextualReflections && (dayIntention?.trim() || dayReflection?.trim() || isToday(date))}
      <div class="mb-4 flex shrink-0 flex-col gap-2">
        <IntentionQuoteCard date={date} intention={dayIntention} promptWhenEmpty={isToday(date)} />
        {#if dayReflection?.trim()}
          <ReflectionCard date={date} reflection={dayReflection} show="reflection" />
        {/if}
      </div>
    {/if}

    <!-- Tag filter (in-place) -->
    {#if tagStore.definitions.length}
      <div class="mb-4 flex shrink-0 flex-wrap items-center gap-3">
        <button onclick={() => showFilter = !showFilter}
                class="inline-flex items-center gap-1.5 rounded-full transition-colors"
                style="font-size: 12px; padding: 4px 10px;
                       {filterTags.length
                         ? 'background: var(--sempa-accent-bg); color: var(--sempa-accent);'
                         : 'color: var(--sempa-text-dim); box-shadow: inset 0 0 0 1px var(--sempa-border);'}">
          <SlidersHorizontal size={13} />
          {filterTags.length ? `Filtered · ${filterTags.length}` : 'Filter by tag'}
        </button>
        {#if showFilter}
          <TagFilterBar bind:selected={filterTags} bind:match={filterMatch} />
        {/if}
      </div>
    {/if}

    <!-- Rollover banner -->
    {#if rolloverTasks.length > 0 && !rolloverDismissed}
      <div class="mb-4 flex shrink-0 items-center gap-3 rounded-xl px-4 py-3 animate-slide-down"
           style="border: 1px solid var(--sempa-amber); background: color-mix(in srgb, var(--sempa-amber) 8%, var(--sempa-bg-main));">
        <svg class="h-4 w-4 shrink-0" style="color: var(--sempa-amber);" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
        <p class="flex-1 text-xs" style="color: var(--sempa-amber);">
          <strong>{rolloverTasks.length}</strong> unfinished from yesterday —
          {rolloverTasks.slice(0,2).map(t=>t.title).join(', ')}{rolloverTasks.length > 2 ? '...' : ''}
        </p>
        <button onclick={rolloverAll}
                class="rounded-lg px-3 py-1 text-xs font-medium transition-colors"
                style="background: var(--sempa-amber); color: var(--sempa-btn-fg);">
          Roll over
        </button>
        <button onclick={() => rolloverDismissed = true} aria-label="Dismiss"
                style="color: var(--sempa-amber); opacity: 0.7;">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>
    {/if}

    {#if loading}
      <div class="flex h-64 items-center justify-center text-sm text-gray-300 dark:text-gray-700">Loading...</div>
    {:else if error}
      <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-600
                  dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-400">
        {error} <button onclick={loadTasks} class="ml-2 underline">Retry</button>
      </div>
    {:else}
      <!-- Infinite rolling board: today (anchor) starts at the left edge, with
           past days reachable by scrolling left and the future to the right —
           the range grows on demand at either edge, so there's no week boundary
           to flash through. Weekend columns read a touch softer. -->
      <div bind:this={weekGrid} onscroll={onBoardScroll}
           class="flex flex-1 min-h-0 items-stretch gap-3 overflow-x-auto overflow-y-hidden" data-weekgrid>
        {#each columns as day (day.date)}
          <div id="day-col-{day.date}" data-daycol class="flex h-full w-56 shrink-0" style={day.isWeekend ? 'opacity: 0.92;' : ''}>
            <WeekDayColumn
              date={day.date} dayName={day.dayName} dayNum={day.dayNum}
              isToday={day.isToday} isWeekend={day.isWeekend}
              tasks={dayTasks(day.date)}
              isDragOver={dragOverDate === day.date}
              onTaskDragStart={handleDragStart}
              onTaskFocusClick={handleFocus}
              onTaskFocusMode={handleFocusMode}
              onTaskComplete={handleComplete}
              onTaskTrash={handleTrashRequest}
              onTaskClick={openEdit}
              onTaskHover={handleTaskHover}
              onDrop={handleDrop}
              onEmailDrop={handleEmailDrop}
              onObjectiveDrop={handleObjectiveDrop}
              onDragOver={(d) => (dragOverDate = d)}
              onDragLeave={() => (dragOverDate = null)}
              onAddClick={openCreate}
            />
          </div>
        {/each}
      </div>
    {/if}
  </main>

  <!-- ── Right panel ─────────────────────────────────────────────────────────
       Calendar pinned at the top, then a single tabbed region (Schedule / Inbox
       / Jira / Objectives) that fills the rest of the height. Previously these
       all stacked vertically and pushed each other off-screen on shorter
       windows; now exactly one is shown at full height. -->
  <aside class="w-80 shrink-0 flex flex-col overflow-hidden"
         style="background: var(--sempa-bg-panel); border-left: 1px solid var(--sempa-border);">

    <!-- Pinned: mini calendar -->
    <div class="shrink-0" style="border-bottom: 1px solid var(--sempa-border);">
      <MiniCalendar {date} onDateClick={handleCalendarDateClick} onDateDrop={(d) => handleDrop(d)} dragActive={draggingId !== null} />
    </div>

    <!-- Tab bar -->
    <div class="flex shrink-0 items-stretch" style="border-bottom: 1px solid var(--sempa-border);">
      {#each rightPanelTabs as panel}
        {@const active = activePanel === panel.id}
        <button onclick={() => rightPanel = panel.id}
                title={panel.label}
                class="flex flex-1 flex-col items-center justify-center gap-1 py-2 text-[10.5px] font-medium transition-colors"
                style={active
                  ? 'color: var(--sempa-accent); box-shadow: inset 0 -2px 0 var(--sempa-accent);'
                  : 'color: var(--sempa-text-dim);'}>
          {#if panel.id === 'schedule'}
            <Clock size={15} />
          {:else if panel.id === 'mail'}
            <Mail size={15} />
          {:else if panel.id === 'jira'}
            <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor">
              <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.005-1.005zm5.723-5.756H5.757a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 18.313 18.3V6.763a1.006 1.006 0 0 0-1.019-1.006zM23.277.007H11.749a5.215 5.215 0 0 0 5.214 5.214h2.129v2.058A5.218 5.218 0 0 0 24.282 12.5V1.012A1.005 1.005 0 0 0 23.277.007z"/>
            </svg>
          {:else}
            <Target size={15} />
          {/if}
          {panel.label}
        </button>
      {/each}
    </div>

    <!-- Active tab content (full remaining height). Keyed on the active tab so a
         switch crossfades the pane in (~240ms) rather than snapping. The panels
         already mount/unmount per tab via the if/else, so keying changes only
         the entrance, not data-loading behaviour. -->
    <div class="flex-1 overflow-hidden">
      {#key activePanel}
        <div class="h-full animate-pane-in">
          {#if activePanel === 'schedule'}
            <TimeslotCalendar
              date={date}
              tasks={tasks}
              onSchedule={handleSchedule}
              onUnschedule={handleUnschedule}
              onOpenTask={(id) => { const t = tasks.find(t => t.id === id); if (t) openEdit(t); }}
              onEventConverted={(t) => { tasks = [...tasks, t]; }}
            />
          {:else if activePanel === 'mail'}
            <EmailPanel bind:this={emailPanel} onTaskCreated={(t) => { tasks = [...tasks, t]; }} />
          {:else if activePanel === 'jira'}
            <JiraPanel
              onTaskDragStart={(id) => { draggingId = id; }}
              onTasksReloaded={reloadLoaded}
            />
          {:else}
            <!-- Objectives: weekly goals + a jump to the full planner -->
            <div class="h-full overflow-y-auto">
              <WeeklyObjectivesWidget {date} />
              <a href="/week/{ws}"
                 class="m-3 flex items-center justify-center gap-1.5 rounded-lg px-3 py-2 text-[12px] font-medium transition-colors"
                 style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft);">
                <Target size={13} />
                Open weekly planner
              </a>
            </div>
          {/if}
        </div>
      {/key}
    </div>
  </aside>
</div>

<TaskPanel open={panelOpen} task={panelTask} defaultStatus={panelStatus} defaultDate={panelDate}
           defaultTitle={panelTitle} defaultNotes={panelNotes}
           onSave={handlePanelSave} onClose={() => (panelOpen = false)} />
{/if}

<!-- ── Trash confirm modal ──────────────────────────────────────────────── -->
{#if trashConfirmOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-[60] flex items-center justify-center bg-black/30 backdrop-blur-sm animate-fade-in"
       onclick={cancelTrash}>
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="w-full max-w-sm mx-4 rounded-2xl p-6 shadow-2xl animate-scale-in"
         style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);"
         onclick={(e) => e.stopPropagation()}>
      <!-- Icon -->
      <div class="mx-auto mb-4 flex h-11 w-11 items-center justify-center rounded-full"
           style="background: color-mix(in srgb, #ef4444 12%, var(--sempa-bg-panel));">
        <svg class="h-5 w-5 text-red-500" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
        </svg>
      </div>
      <!-- Text -->
      <h3 class="mb-1 text-center text-sm font-semibold" style="color: var(--sempa-text);">Delete task?</h3>
      <p class="mb-5 text-center text-xs leading-relaxed" style="color: var(--sempa-text-soft);">
        <span class="font-medium" style="color: var(--sempa-text);">"{trashTaskTitle}"</span> will be permanently removed.
      </p>
      <!-- Actions -->
      <div class="flex items-center gap-2">
        <button onclick={cancelTrash}
                class="flex-1 rounded-[9px] px-3 py-2 text-[13px] font-medium transition-colors"
                style="border: 1px solid var(--sempa-border); color: var(--sempa-text-soft); background: transparent;"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.background = 'var(--sempa-accent-bg)'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.background = 'transparent'}>
          Cancel
        </button>
        <button onclick={confirmTrash}
                class="flex-1 rounded-[9px] px-3 py-2 text-[13px] font-medium transition-colors shadow-sm"
                style="background: #ef4444; color: white;"
                onmouseenter={(e) => (e.currentTarget as HTMLElement).style.opacity = '0.88'}
                onmouseleave={(e) => (e.currentTarget as HTMLElement).style.opacity = '1'}>
          Delete
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- ── Undo-delete toast ──────────────────────────────────────────────────────
     On mobile the toast must clear the fixed bottom tab bar (72px + safe-area)
     AND the Android gesture-nav zone — otherwise the swipe-up-to-close gesture
     lands on the Undo button. Desktop keeps the plain bottom-6 inset. -->
{#if undoTask}
  <div class="fixed left-1/2 z-[80] -translate-x-1/2 flex items-center gap-3 rounded-xl px-4 py-2.5 text-sm shadow-2xl animate-slide-down"
       style="bottom: {mobile.value ? 'calc(env(safe-area-inset-bottom, 0px) + 88px)' : '1.5rem'}; background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: var(--sempa-text); max-width: min(92vw, 28rem);"
       role="status">
    <span class="min-w-0 flex-1 truncate">
      {undoTask.source === 'jira' ? 'Deleted — returning to the Jira list' : 'Task deleted'}
    </span>
    {#if undoTask.source !== 'jira'}
      <button onclick={doUndo} class="shrink-0 font-semibold" style="color: var(--sempa-accent);">Undo</button>
    {/if}
    <button onclick={() => undoTask = null} aria-label="Dismiss" class="shrink-0" style="color: var(--sempa-text-dim);">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" d="M6 18 18 6M6 6l12 12"/></svg>
    </button>
  </div>
{/if}

<!-- ── Drop error toast ─────────────────────────────────────────────────────
     Drops (email→task, Jira→task, moves) fail far from the board, where the
     page-level error banner would replace the whole board. This surfaces them. -->
{#if dropError}
  <div class="fixed left-1/2 z-[80] -translate-x-1/2 rounded-xl px-4 py-2.5 text-sm shadow-2xl animate-slide-down"
       style="bottom: {mobile.value ? 'calc(env(safe-area-inset-bottom, 0px) + 88px)' : '1.5rem'}; background: var(--sempa-bg-panel); border: 1px solid #ef4444; color: var(--sempa-text); max-width: min(92vw, 28rem);"
       role="alert">
    <div class="flex items-center gap-2.5">
      <svg class="h-4 w-4 shrink-0 text-red-500" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/>
      </svg>
      <span class="min-w-0 flex-1">{dropError}</span>
      <button onclick={() => dropError = null} aria-label="Dismiss" class="shrink-0" style="color: var(--sempa-text-dim);">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" d="M6 18 18 6M6 6l12 12"/></svg>
      </button>
    </div>
  </div>
{/if}
