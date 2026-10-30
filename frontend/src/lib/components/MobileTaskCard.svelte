<script lang="ts">
  import type { Task } from '$lib/types';
  import { formatMinutes, bareUrl, prettyUrl } from '$lib/utils';
  import { tagStore } from '$lib/stores/tags.svelte';
  import { projectStore } from '$lib/stores/projects.svelte';
  import { hapticClick, hapticTick } from '$lib/haptics';

  let {
    task,
    onComplete,
    onReschedule,
    onTrash,
    onClick,
    onFocusClick,
    onReorderStart,
    onReorderMove,
    onReorderEnd,
    dragging = false,
  }: {
    task: Task;
    onComplete?: (id: string) => void;
    /** Swipe left → reschedule (to tomorrow). Only enabled when supplied. */
    onReschedule?: (id: string) => void;
    onTrash?: (id: string, title: string) => void;
    onClick?: (task: Task) => void;
    onFocusClick?: (id: string, title: string) => void;
    // Long-press drag-to-reorder. When these are supplied the card enters a
    // reorder gesture after a brief hold; the parent owns the live ordering.
    onReorderStart?: (id: string) => void;
    onReorderMove?: (clientY: number) => void;
    onReorderEnd?: () => void;
    /** True when THIS card is the one currently being dragged (lifts it). */
    dragging?: boolean;
  } = $props();
  const project = $derived(projectStore.get(task.project_id));

  const isDone      = $derived(task.status === 'done');
  const isRecurring = $derived(!!task.recurrence_origin_id);
  const isActive    = $derived(task.status === 'in_progress');
  // Share of the estimate already logged, for the in-progress ring (null = no estimate).
  const ringFrac    = $derived(task.time_estimate_minutes ? (task.time_actual_minutes ?? 0) / task.time_estimate_minutes : null);
  const sourceLabel: Record<string, string> = {
    gmail: 'Gmail', fastmail: 'Mail', jira: 'Jira', google_calendar: 'Cal',
  };

  // Reminder marker — mirrors TaskCard so the planner shows at a glance which
  // tasks will ring. Bell + short time; only while the task is still open.
  const hasReminder = $derived(!!task.remind_at && !isDone);
  const remindLabel = $derived.by(() => {
    if (!task.remind_at) return '';
    const dt = new Date(task.remind_at);
    if (isNaN(dt.getTime())) return '';
    const now = new Date();
    const sameDay = dt.toDateString() === now.toDateString();
    const time = dt.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
    return sameDay ? time : `${dt.toLocaleDateString([], { month: 'short', day: 'numeric' })} ${time}`;
  });

  // Swipe state
  let startX = $state(0);
  let startY = $state(0);
  let deltaX = $state(0);
  let swiping = $state(false);
  let locked = $state<null | 'h' | 'v'>(null); // gesture direction, decided once
  let suppressClick = $state(false);
  const SWIPE_THRESHOLD = 60;
  const MAX_SWIPE = 80;
  const TRIGGER = SWIPE_THRESHOLD * 0.4;
  const LONG_PRESS_MS = 380;   // hold this long (without moving) to pick up
  const MOVE_SLOP = 8;         // finger travel that cancels the long-press

  let longPressTimer: ReturnType<typeof setTimeout> | null = null;
  let reorderActive = $state(false); // this card is being dragged to reorder

  function cancelLongPress() {
    if (longPressTimer) { clearTimeout(longPressTimer); longPressTimer = null; }
  }

  function handleTouchStart(e: TouchEvent) {
    startX = e.touches[0].clientX;
    startY = e.touches[0].clientY;
    deltaX = 0;
    locked = null;
    swiping = true;
    reorderActive = false;
    // Arm long-press → drag-to-reorder, only where the parent wired it up
    // (active list, never the completed section).
    cancelLongPress();
    if (onReorderStart) {
      longPressTimer = setTimeout(() => {
        longPressTimer = null;
        reorderActive = true;
        swiping = false;   // hand the gesture over to reorder; cancel any swipe
        deltaX = 0;
        hapticClick();
        onReorderStart?.(task.id);
      }, LONG_PRESS_MS);
    }
  }

  function handleTouchMove(e: TouchEvent) {
    // Reorder owns the gesture: feed the parent the finger position and block
    // the page from scrolling underneath the pick-up.
    if (reorderActive) {
      e.preventDefault();
      onReorderMove?.(e.touches[0].clientY);
      return;
    }
    if (!swiping) return;
    const dx = e.touches[0].clientX - startX;
    const dy = e.touches[0].clientY - startY;

    // Any real movement before the hold completes means this is a scroll/swipe,
    // not a pick-up — disarm the long-press.
    if (longPressTimer && (Math.abs(dx) > MOVE_SLOP || Math.abs(dy) > MOVE_SLOP)) {
      cancelLongPress();
    }

    // Decide once whether this is a horizontal swipe or a vertical scroll, so
    // list scrolling never fights the swipe-to-complete gesture.
    if (locked === null) {
      if (Math.abs(dx) < 8 && Math.abs(dy) < 8) return;
      locked = Math.abs(dx) > Math.abs(dy) ? 'h' : 'v';
    }
    if (locked !== 'h') return;
    // Right = complete, left = reschedule; each direction only if its handler exists
    // (e.g. completed-section cards have neither extra). Damp + clamp both ways.
    if (dx > 0 && !onComplete) return;
    if (dx < 0 && !onReschedule) return;

    const prev = deltaX;
    deltaX = Math.max(-MAX_SWIPE, Math.min(dx * 0.4, MAX_SWIPE));
    if (Math.abs(prev) < TRIGGER && Math.abs(deltaX) >= TRIGGER) hapticTick();
  }

  function handleTouchEnd() {
    cancelLongPress();
    if (reorderActive) {
      reorderActive = false;
      onReorderEnd?.();
      // Suppress the trailing synthetic click so the drop doesn't also open the
      // task detail.
      suppressClick = true;
      setTimeout(() => { suppressClick = false; }, 350);
      return;
    }
    if (!swiping) return;
    swiping = false;
    if (deltaX > TRIGGER) {
      hapticClick();
      onComplete?.(task.id);
    } else if (deltaX < -TRIGGER) {
      hapticClick();
      onReschedule?.(task.id);
    }
    // Suppress the synthetic click that follows any real horizontal swipe so a
    // swipe doesn't also open the task detail.
    if (locked === 'h' && Math.abs(deltaX) > 4) {
      suppressClick = true;
      setTimeout(() => { suppressClick = false; }, 350);
    }
    deltaX = 0;
    locked = null;
  }

  function handleClick() {
    if (suppressClick) return;
    onClick?.(task);
  }
</script>

<div class="relative overflow-hidden rounded-[22px]" data-task-id={task.id}>
  <!-- Swipe-right reveal (complete — green check) -->
  {#if deltaX > 0}
    <div class="absolute inset-y-0 left-0 flex items-center pl-4"
         style="color: var(--sempa-success); width: {deltaX}px;">
      <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
      </svg>
    </div>
  {/if}

  <!-- Swipe-left reveal (reschedule to tomorrow — calendar + arrow) -->
  {#if deltaX < 0}
    <div class="absolute inset-y-0 right-0 flex items-center justify-end pr-4"
         style="color: var(--sempa-accent); width: {-deltaX}px;">
      <svg class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M8 7V3m8 4V3M4 11h16M5 5h14a1 1 0 011 1v13a1 1 0 01-1 1H5a1 1 0 01-1-1V6a1 1 0 011-1z"/>
        <path stroke-linecap="round" stroke-linejoin="round" d="M11 15h4m0 0l-2-2m2 2l-2 2"/>
      </svg>
    </div>
  {/if}

  <!-- Card -->
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div
    class="relative flex items-center gap-3 rounded-[22px] py-3.5 pl-4 pr-2.5"
    class:reordering={dragging}
    style="background: {isDone ? 'transparent' : 'var(--card-bg)'};
           box-shadow: {dragging
             ? 'var(--sempa-shadow-2)'
             : isDone
               ? 'inset 0 0 0 1px var(--sempa-border)'
               : isActive
                 ? 'var(--sempa-shadow-1), inset 0 0 0 2px var(--sempa-accent)'
                 : 'var(--sempa-shadow-1)'};
           transform: {dragging ? 'scale(1.02)' : `translateX(${deltaX}px)`};
           transition: {swiping || dragging ? 'none' : 'transform 200ms ease-out'};
           {dragging ? 'z-index: 5;' : ''}
           -webkit-touch-callout: none; -webkit-user-select: none; user-select: none;"
    ontouchstart={handleTouchStart}
    ontouchmove={handleTouchMove}
    ontouchend={handleTouchEnd}
    ontouchcancel={handleTouchEnd}
    onclick={handleClick}
  >
    <!-- Complete circle -->
    <button
      type="button"
      onclick={(e) => { e.stopPropagation(); hapticClick(); onComplete?.(task.id); }}
      class="check-ring self-start mt-0.5"
      style="width: 24px; height: 24px;"
      data-state={isDone ? 'done' : isActive ? 'active' : undefined}
      aria-label="Complete task"
    >
      {#if isDone}
        <svg class="h-3 w-3" fill="none" stroke="currentColor" stroke-width="3.2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
        </svg>
      {/if}
    </button>

    <!-- Content -->
    <div class="min-w-0 flex-1">
      <p class="text-[15px] leading-snug tracking-[-0.01em]
                {isDone ? 'line-through opacity-50 font-medium' : 'font-bold'}"
         style="color: var(--sempa-text); overflow-wrap: anywhere; word-break: break-word;">
        {#if bareUrl(task.title)}
          <span class="inline-flex max-w-full items-center gap-1 align-middle">
            <svg class="h-3.5 w-3.5 shrink-0 opacity-60" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M10 13a5 5 0 007.07 0l3-3a5 5 0 00-7.07-7.07l-1.72 1.71M14 11a5 5 0 00-7.07 0l-3 3a5 5 0 007.07 7.07l1.71-1.71"/>
            </svg>
            <span class="truncate">{prettyUrl(bareUrl(task.title)!)}</span>
          </span>
        {:else}
          {task.title}
        {/if}
      </p>

      <!-- Meta row -->
      {#if project || task.tags?.length || task.time_estimate_minutes || (task.source && task.source !== 'manual') || isRecurring || hasReminder}
        <div class="flex flex-wrap items-center gap-1.5 mt-2">
          {#if project}
            <!-- Project as a small colour SQUARE (tags are round dots). -->
            <span class="shrink-0" title={`Project: ${project.title || 'Untitled project'}`}
                  style="width: 8px; height: 8px; border-radius: 2px; background: {project.color};"></span>
          {/if}
          {#if (task.tags ?? []).length}
            <!-- First tag as a tinted chip with its name (matches TaskCard); any
                 further tags stay as dots. Title lists them all on long-press. -->
            {@const first = (task.tags ?? [])[0]}
            <span class="tint chip min-w-0" style="--tint: {tagStore.colorFor(first)}; max-width: 10rem;"
                  title={(task.tags ?? []).join(', ')}>
              <span class="shrink-0 rounded-full" style="width: 6px; height: 6px; background: {tagStore.colorFor(first)};"></span>
              <span class="truncate">{first}</span>
            </span>
            {#if (task.tags ?? []).length > 1}
              <span class="inline-flex items-center gap-1" title={(task.tags ?? []).slice(1).join(', ')}>
                {#each (task.tags ?? []).slice(1) as tag}
                  <span class="shrink-0 rounded-full" style="width: 7px; height: 7px; background-color: {tagStore.colorFor(tag)};"></span>
                {/each}
              </span>
            {/if}
          {/if}
          {#if task.time_estimate_minutes}
            <span class="chip chip-time">
              {isActive && task.time_actual_minutes ? `${formatMinutes(task.time_actual_minutes)} / ` : ''}{formatMinutes(task.time_estimate_minutes)}
            </span>
          {/if}
          {#if hasReminder}
            <span class="chip"
                  style="gap: 4px; background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);"
                  title="Reminder {remindLabel}">
              <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.25" style="flex: 0 0 auto;">
                <path stroke-linecap="round" stroke-linejoin="round" d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.73 21a2 2 0 0 1-3.46 0"/>
              </svg>
              {remindLabel}
            </span>
          {/if}
          {#if task.source && task.source !== 'manual'}
            <span class="chip"
                  style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
              {sourceLabel[task.source] ?? task.source}
            </span>
          {/if}
          {#if isRecurring}
            <span class="chip"
                  style="background: var(--sempa-fill); color: var(--sempa-text-soft);"
                  title="Recurring">&#8634;</span>
          {/if}
        </div>
      {/if}
    </div>

    <!-- Time-used ring for the task in progress (actual vs estimate) -->
    {#if isActive && ringFrac !== null}
      {@const C = 2 * Math.PI * 14}
      <svg width="36" height="36" viewBox="0 0 36 36" role="img" class="shrink-0"
           aria-label="{Math.round(ringFrac * 100)} percent of estimate used">
        <circle cx="18" cy="18" r="14" fill="none" stroke="var(--sempa-track)" stroke-width="4"/>
        <circle cx="18" cy="18" r="14" fill="none" stroke="var(--sempa-accent)" stroke-width="4" stroke-linecap="round"
                stroke-dasharray="{(Math.min(ringFrac, 1) * C).toFixed(1)} {C.toFixed(1)}" transform="rotate(-90 18 18)"/>
        <text x="18" y="21.5" text-anchor="middle" font-size="9.5" font-weight="800" fill="var(--sempa-text)"
              font-family="Plus Jakarta Sans, sans-serif">{Math.round(ringFrac * 100)}%</text>
      </svg>
    {/if}

    <!-- Actions (visible on mobile, no hover needed) -->
    <div class="flex shrink-0 items-center">
      {#if onFocusClick && !isDone}
        <button onclick={(e) => { e.stopPropagation(); onFocusClick?.(task.id, task.title); }}
                class="flex h-9 w-9 items-center justify-center rounded-full" style="color: var(--sempa-text-soft);"
                title="Start focus timer" aria-label="Start focus timer">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 7v5l3 3"/>
          </svg>
        </button>
      {/if}
      {#if onTrash}
        <button onclick={(e) => { e.stopPropagation(); onTrash?.(task.id, task.title); }}
                class="flex h-9 w-9 items-center justify-center rounded-full" style="color: var(--sempa-text-soft);"
                title="Delete task" aria-label="Delete task">
          <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
          </svg>
        </button>
      {/if}
    </div>
  </div>
</div>
