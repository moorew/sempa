<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api';
  import { COLUMNS, type Task, type TaskStatus } from '$lib/types';
  import { appendPosition, formatDate, isToday, offsetDate, today, weekStart } from '$lib/utils';
  import { pomodoro } from '$lib/stores/pomodoro.svelte';
  import KanbanColumn from '$lib/components/KanbanColumn.svelte';
  import AddTaskModal from '$lib/components/AddTaskModal.svelte';

  let date = $derived($page.params.date ?? today());
  let tasks = $state<Task[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  let draggingId = $state<string | null>(null);
  let dragOverStatus = $state<TaskStatus | null>(null);
  let modalOpen = $state(false);
  let modalStatus = $state<TaskStatus>('planned');

  async function loadTasks() {
    loading = true; error = null;
    try { tasks = await api.tasks.listByDate(date); }
    catch (e) { error = e instanceof Error ? e.message : 'Failed to load tasks'; }
    finally { loading = false; }
  }

  onMount(loadTasks);
  $effect(() => { date; loadTasks(); });

  function columnTasks(status: TaskStatus): Task[] {
    return tasks.filter(t => t.status === status).sort((a, b) => a.position - b.position);
  }

  function handleDragStart(id: string) { draggingId = id; }

  async function handleDrop(targetStatus: TaskStatus) {
    if (!draggingId || !dragOverStatus) return;
    const id = draggingId;
    draggingId = null; dragOverStatus = null;
    const task = tasks.find(t => t.id === id);
    if (!task || task.status === targetStatus) return;
    const newPos = appendPosition(tasks.filter(t => t.status === targetStatus).map(t => t.position));
    const prev = tasks.slice();
    tasks = tasks.map(t => t.id === id ? { ...t, status: targetStatus, position: newPos } : t);
    try {
      const updated = await api.tasks.update(id, {
        status: targetStatus, position: newPos,
        ...(task.planned_date === null && targetStatus !== 'backlog'
          ? { planned_date: date, week_start: weekStart(date) } : {}),
      });
      tasks = tasks.map(t => t.id === updated.id ? updated : t);
    } catch { tasks = prev; }
  }

  function handleFocus(id: string, title: string) { pomodoro.start(id, title); }

  function openModal(status: TaskStatus) { modalStatus = status; modalOpen = true; }

  async function handleCreate(params: {
    title: string; status: TaskStatus; estimateMinutes: number | null;
    tags: string[]; recurrenceRule: string | null;
  }) {
    modalOpen = false;
    const { title, status, estimateMinutes, tags, recurrenceRule } = params;
    const newPos = appendPosition(tasks.filter(t => t.status === status).map(t => t.position));
    try {
      const task = await api.tasks.create({
        title, tags,
        ...(recurrenceRule
          ? { recurrence_rule: recurrenceRule }
          : {
              status,
              position: newPos,
              planned_date: status !== 'backlog' ? date : undefined,
              week_start: status !== 'backlog' ? weekStart(date) : undefined,
            }),
        time_estimate_minutes: estimateMinutes ?? undefined,
      });
      // If it was a recurring template creation, reload to pick up the generated instance
      if (recurrenceRule) { await loadTasks(); }
      else { tasks = [...tasks, task]; }
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to create task';
    }
  }

  function navigate(delta: number) { goto(`/day/${offsetDate(date, delta)}`); }
  function goToday() { goto(`/day/${today()}`); }
</script>

<svelte:head><title>{isToday(date) ? 'Today' : date} — Aura</title></svelte:head>

<header class="sticky top-0 z-10 border-b border-gray-200 bg-white/90 backdrop-blur-sm
               dark:border-gray-800 dark:bg-gray-900/90">
  <div class="mx-auto flex max-w-[1400px] items-center justify-between px-6 py-3">
    <div class="flex items-center gap-3">
      <button onclick={() => navigate(-1)}
              class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-300">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M15 19l-7-7 7-7"/>
        </svg>
      </button>
      <div class="text-center">
        <p class="text-sm font-semibold text-gray-800 dark:text-gray-100">{formatDate(date)}</p>
        {#if isToday(date)}<p class="text-xs text-blue-500 font-medium dark:text-blue-400">Today</p>{/if}
      </div>
      <button onclick={() => navigate(1)}
              class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-300">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M9 5l7 7-7 7"/>
        </svg>
      </button>
    </div>
    <div class="flex items-center gap-2">
      {#if !isToday(date)}
        <button onclick={goToday}
                class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-600
                       hover:bg-gray-50 transition-colors dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800">
          Today
        </button>
      {/if}
      <button onclick={() => openModal('planned')}
              class="flex items-center gap-1.5 rounded-lg bg-blue-500 px-3 py-1.5 text-xs font-medium
                     text-white hover:bg-blue-600 transition-colors">
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M12 4v16m8-8H4"/>
        </svg>
        New task
      </button>
    </div>
  </div>
</header>

<main class="mx-auto max-w-[1400px] px-6 py-6">
  {#if loading}
    <div class="flex h-64 items-center justify-center text-sm text-gray-400 dark:text-gray-600">Loading…</div>
  {:else if error}
    <div class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-400">
      {error} <button onclick={loadTasks} class="ml-2 underline">Retry</button>
    </div>
  {:else}
    <div class="flex gap-4 overflow-x-auto pb-4">
      {#each COLUMNS as col (col.status)}
        <KanbanColumn
          label={col.label} status={col.status} tasks={columnTasks(col.status)}
          accent={col.accent} bg={col.bg} border={col.border}
          isDragOver={dragOverStatus === col.status}
          onTaskDragStart={handleDragStart} onTaskFocusClick={handleFocus}
          onDrop={handleDrop} onDragOver={(s) => (dragOverStatus = s)}
          onDragLeave={() => (dragOverStatus = null)} onAddClick={openModal}
        />
      {/each}
    </div>
  {/if}
</main>

<AddTaskModal open={modalOpen} defaultStatus={modalStatus} defaultDate={date}
              onSubmit={handleCreate} onClose={() => (modalOpen = false)} />
