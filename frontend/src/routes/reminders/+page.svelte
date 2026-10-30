<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import { localApi } from '$lib/tauri/local-api';
  import { hasLocalDb } from '$lib/platform';
  import { reminderAlerts } from '$lib/stores/reminderAlerts.svelte';
  import type { Task } from '$lib/types';
  import { Bell, BellRing, Check, Clock, X } from 'lucide-svelte';

  let tasks = $state<Task[]>([]);
  let loading = $state(true);
  let now = $state(Date.now());

  // Tick every 30s so countdowns stay fresh and a reminder slides from
  // "upcoming" to "rang" without a manual reload.
  let tick: ReturnType<typeof setInterval> | null = null;

  async function load() {
    try {
      tasks = hasLocalDb()
        ? await localApi.tasks.withReminders()
        : await api.tasks.listWithReminders();
    } catch {
      tasks = [];
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void load();
    tick = setInterval(() => (now = Date.now()), 30_000);
  });
  onDestroy(() => {
    if (tick) clearInterval(tick);
  });

  const withTime = $derived(
    tasks
      .filter((t) => t.remind_at)
      .map((t) => ({ task: t, at: new Date(t.remind_at as string).getTime() }))
      .filter((r) => !isNaN(r.at)),
  );
  const upcoming = $derived(withTime.filter((r) => r.at > now).sort((a, b) => a.at - b.at));
  const rang     = $derived(withTime.filter((r) => r.at <= now).sort((a, b) => b.at - a.at));

  function relUpcoming(at: number): string {
    const mins = Math.round((at - now) / 60000);
    if (mins < 1) return 'in <1 min';
    if (mins < 60) return `in ${mins} min`;
    const hrs = Math.round(mins / 60);
    if (hrs < 24) return `in ${hrs}h`;
    const days = Math.round(hrs / 24);
    return `in ${days}d`;
  }
  function relPast(at: number): string {
    const mins = Math.round((now - at) / 60000);
    if (mins < 1) return 'just now';
    if (mins < 60) return `${mins} min ago`;
    const hrs = Math.round(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    const days = Math.round(hrs / 24);
    return `${days}d ago`;
  }
  function clockTime(at: number): string {
    const d = new Date(at);
    return d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }
  function dayLabel(at: number): string {
    const d = new Date(at);
    const today = new Date();
    const t0 = new Date(today.getFullYear(), today.getMonth(), today.getDate()).getTime();
    const d0 = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
    const diff = Math.round((d0 - t0) / 86400000);
    if (diff === 0) return 'Today';
    if (diff === 1) return 'Tomorrow';
    if (diff === -1) return 'Yesterday';
    return d.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' });
  }

  function open(taskId: string) {
    goto(`/focus/${taskId}`);
  }
  async function markDone(taskId: string) {
    tasks = tasks.filter((t) => t.id !== taskId);
    reminderAlerts.dismiss(taskId);
    await api.tasks.update(taskId, { status: 'done' }).catch(() => {});
  }
  async function clearReminder(taskId: string) {
    tasks = tasks.filter((t) => t.id !== taskId);
    reminderAlerts.dismiss(taskId);
    await api.tasks.update(taskId, { remind_at: '' }).catch(() => {});
  }
</script>

<div class="mx-auto flex h-full max-w-xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <!-- Header -->
  <div class="flex items-end gap-3 px-5 pb-2 pt-7">
    <div class="min-w-0 flex-1">
      <h1 class="type-page" style="color: var(--sempa-text);">Reminders</h1>
      {#if !loading && withTime.length > 0}
        <p class="type-meta mt-1" style="color: var(--sempa-text-dim);">
          {upcoming.length} upcoming{rang.length ? ` · ${rang.length} rang` : ''}
        </p>
      {/if}
    </div>
    <a href="/settings/notifications" class="btn-surface h-9 px-4 text-[12.5px]">
      Settings
    </a>
  </div>

  <div class="flex-1 overflow-y-auto px-5 pb-20 pt-4">
    {#if loading}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else if withTime.length === 0}
      <div class="flex flex-col items-center justify-center py-20 text-center">
        <div class="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl"
             style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text-dim);">
          <Bell size={22} strokeWidth={1.75} />
        </div>
        <p class="font-bold" style="font-size: 15px; color: var(--sempa-text);">No reminders set</p>
        <p class="type-meta mt-1 max-w-xs" style="color: var(--sempa-text-soft);">
          Set "Remind me" on any task and it'll show up here.
        </p>
      </div>
    {:else}

      {#snippet sectionLabel(text: string, count: number)}
        <div class="mb-2.5 flex items-center gap-2 px-1">
          <p style="font-size: 15px; font-weight: 800; letter-spacing: -.015em; color: var(--sempa-text);">{text}</p>
          <span class="chip" style="height: 20px; padding: 0 7px; background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); color: var(--sempa-text-soft);">{count}</span>
        </div>
      {/snippet}

      {#snippet row(task: Task, at: number, fired: boolean, i: number)}
        <div class="flex items-center gap-3 px-4 py-3"
             style={i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}>
          <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-[12px]"
               style="background: {fired ? 'var(--sempa-accent-bg)' : 'var(--sempa-fill)'};
                      color: {fired ? 'var(--sempa-accent-ink)' : 'var(--sempa-text-soft)'};">
            {#if fired}<BellRing size={16} strokeWidth={2} />{:else}<Clock size={16} strokeWidth={2} />{/if}
          </div>

          <button class="min-w-0 flex-1 text-left" onclick={() => open(task.id)}>
            <p class="truncate font-semibold" style="font-size: 14px; color: var(--sempa-text);">{task.title}</p>
            <p class="type-meta mt-0.5 flex flex-wrap items-center gap-x-1.5" style="color: {fired ? 'var(--sempa-accent-ink)' : 'var(--sempa-text-soft)'};">
              <span class="font-mono text-[11.5px]">{clockTime(at)}</span>
              <span aria-hidden="true">·</span>
              <span>{dayLabel(at)}</span>
              <span aria-hidden="true">·</span>
              <span>{fired ? `rang ${relPast(at)}` : relUpcoming(at)}</span>
            </p>
          </button>

          {#if fired}
            <button onclick={() => markDone(task.id)} aria-label="Mark done"
                    class="btn-soft h-9 w-9 shrink-0 p-0"
                    title="Mark done">
              <Check size={16} strokeWidth={2.25} />
            </button>
          {/if}
          <button onclick={() => clearReminder(task.id)} aria-label="Clear reminder"
                  class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full transition-colors hover:bg-[var(--sempa-fill)]"
                  style="color: var(--sempa-text-dim);"
                  title="Clear reminder">
            <X size={16} />
          </button>
        </div>
      {/snippet}

      {#if rang.length > 0}
        {@render sectionLabel('Rang', rang.length)}
        <section class="surface mb-7 overflow-hidden" style="border-radius: 22px;">
          {#each rang as r, i (r.task.id)}
            {@render row(r.task, r.at, true, i)}
          {/each}
        </section>
      {/if}

      {#if upcoming.length > 0}
        {@render sectionLabel('Upcoming', upcoming.length)}
        <section class="surface mb-7 overflow-hidden" style="border-radius: 22px;">
          {#each upcoming as r, i (r.task.id)}
            {@render row(r.task, r.at, false, i)}
          {/each}
        </section>
      {/if}

    {/if}
  </div>
</div>
