<script lang="ts">
  import type { Task } from '$lib/types';
  import { formatMinutes } from '$lib/utils';
  import { tagStore } from '$lib/stores/tags.svelte';

  let {
    task,
    accent,
    onDragStart,
    onFocusClick,
    onComplete,
    onClick,
  }: {
    task: Task;
    accent: string;
    onDragStart: (id: string) => void;
    onFocusClick?: (id: string, title: string) => void;
    onComplete?: (id: string) => void;
    onClick?: (task: Task) => void;
  } = $props();

  const sourceLabel: Record<string, string> = {
    gmail: 'Gmail', fastmail: 'Mail', jira: 'Jira', google_calendar: '📅'
  };
  const isDone = $derived(task.status === 'done');
  const isRecurring = $derived(!!task.recurrence_origin_id);
</script>

<div
  draggable="true"
  role="listitem"
  ondragstart={() => onDragStart(task.id)}
  class="group flex items-start gap-2 rounded-lg border border-gray-200 bg-white p-3 shadow-xs
         cursor-grab active:cursor-grabbing active:opacity-50 active:shadow-md transition-shadow
         dark:border-gray-700 dark:bg-gray-800"
>
  <!-- Quick-complete circle -->
  <button
    type="button"
    onclick={(e) => { e.stopPropagation(); onComplete?.(task.id); }}
    title={isDone ? 'Completed' : 'Mark complete'}
    class="mt-0.5 h-4 w-4 shrink-0 rounded-full border-2 flex items-center justify-center
           transition-all cursor-pointer
           {isDone
             ? 'border-green-500 bg-green-500'
             : 'border-gray-300 hover:border-green-400 hover:bg-green-50 dark:border-gray-600 dark:hover:border-green-500 dark:hover:bg-green-950'}"
    aria-label="Complete task"
  >
    {#if isDone}
      <svg class="h-2.5 w-2.5 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
      </svg>
    {/if}
  </button>

  <!-- Card body — click to edit -->
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="min-w-0 flex-1 cursor-pointer" onclick={() => onClick?.(task)}>
    <p class="text-sm font-medium leading-snug
              {isDone ? 'line-through text-gray-400 dark:text-gray-600' : 'text-gray-800 dark:text-gray-100'}">
      {task.title}
    </p>

    {#if task.tags?.length || task.time_estimate_minutes || (task.source && task.source !== 'manual') || isRecurring}
      <div class="mt-1.5 flex flex-wrap gap-1">
        {#each (task.tags ?? []) as tag}
          <span class="rounded-full px-2 py-0.5 text-xs font-medium text-white"
                style="background-color: {tagStore.colorFor(tag)}">{tag}</span>
        {/each}
        {#if task.time_estimate_minutes}
          <span class="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500 font-mono
                       dark:bg-gray-700 dark:text-gray-400">
            {formatMinutes(task.time_estimate_minutes)}
          </span>
        {/if}
        {#if task.source && task.source !== 'manual'}
          <span class="rounded bg-indigo-50 px-1.5 py-0.5 text-xs text-indigo-600
                       dark:bg-indigo-950 dark:text-indigo-400">
            {sourceLabel[task.source] ?? task.source}
          </span>
        {/if}
        {#if isRecurring}
          <span class="rounded bg-violet-50 px-1.5 py-0.5 text-xs text-violet-600
                       dark:bg-violet-950 dark:text-violet-400" title="Recurring">↺</span>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Hover actions -->
  <div class="flex shrink-0 items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
    {#if onFocusClick && !isDone}
      <button onclick={(e) => { e.stopPropagation(); onFocusClick?.(task.id, task.title); }}
              class="rounded p-1 text-gray-300 hover:text-amber-500 transition-colors
                     dark:text-gray-600 dark:hover:text-amber-400"
              title="Start focus timer">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <circle cx="12" cy="12" r="9"/><path stroke-linecap="round" d="M12 7v5l3 3"/>
        </svg>
      </button>
    {/if}
    <div class="text-gray-300 p-1 dark:text-gray-600">
      <svg class="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 20 20">
        <path d="M7 2a2 2 0 110 4 2 2 0 010-4zm6 0a2 2 0 110 4 2 2 0 010-4zM7 8a2 2 0 110 4 2 2 0 010-4zm6 0a2 2 0 110 4 2 2 0 010-4zM7 14a2 2 0 110 4 2 2 0 010-4zm6 0a2 2 0 110 4 2 2 0 010-4z"/>
      </svg>
    </div>
  </div>
</div>
