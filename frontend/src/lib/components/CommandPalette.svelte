<script lang="ts">
  /**
   * Command palette (Cmd/Ctrl+K) — a fast, keyboard-first launcher for
   * navigation and actions. This is the general surface the AI Import command
   * lives in (alongside the Lists button and the direct Ctrl/⌘+Shift+I shortcut).
   * A non-matching query falls back to a full-text Search command, preserving
   * the old Cmd+K → search muscle memory (one extra Enter).
   *
   * Mounted once in the root layout; opened via the commandPalette store.
   */
  import { commandPalette } from '$lib/stores/commandPalette.svelte';
  import { importModal } from '$lib/stores/importModal.svelte';
  import { prefs } from '$lib/stores/prefs.svelte';
  import { aiStatus } from '$lib/stores/aiStatus.svelte';
  import { jiraStatus } from '$lib/stores/jiraStatus.svelte';
  import { theme } from '$lib/stores/theme.svelte';
  import { today, weekStart } from '$lib/utils';
  import { goto } from '$app/navigation';
  import {
    Sparkles, Search, Home, Layers, ListChecks, CalendarClock, ClipboardCheck,
    Moon, Bell, Mail, SquareKanban, BarChart3, BookOpen, Settings, SunMoon, FolderKanban,
  } from 'lucide-svelte';

  interface Command {
    id: string;
    label: string;
    hint?: string;
    icon: typeof Home;
    keywords?: string;
    run: () => void;
  }

  let query = $state('');
  let selected = $state(0);
  let inputEl = $state<HTMLInputElement | undefined>();

  function go(path: string) {
    commandPalette.close();
    goto(path);
  }

  // Built reactively so gated commands (AI Import) appear/disappear correctly.
  const commands = $derived.by<Command[]>(() => {
    const td = today();
    const ws = weekStart(td);
    const list: Command[] = [];

    if (prefs.aiOn('aiImport') && aiStatus.reachable) {
      list.push({
        id: 'ai-import', label: 'Import with AI', hint: 'URL or text → task + list',
        icon: Sparkles, keywords: 'recipe import ai generate ingredients steps',
        run: () => { commandPalette.close(); importModal.show(); },
      });
    }

    list.push(
      { id: 'today',    label: 'Today',           icon: Home,          keywords: 'home day board', run: () => go('/home') },
      { id: 'backlog',  label: 'Backlog',         icon: Layers,        keywords: 'inbox unscheduled', run: () => go('/backlog') },
      { id: 'lists',    label: 'Lists',           icon: ListChecks,    keywords: 'checklist groceries', run: () => go('/lists') },
      { id: 'projects', label: 'Projects',        icon: FolderKanban,  keywords: 'epic initiative long term goal', run: () => go('/projects') },
      { id: 'schedule', label: 'Schedule',        icon: CalendarClock, keywords: 'calendar timebox', run: () => go('/schedule') },
      { id: 'plan',     label: 'Plan today',      icon: ClipboardCheck, keywords: 'planning', run: () => go(`/plan/${td}`) },
      { id: 'shutdown', label: 'Shutdown',        icon: Moon,          keywords: 'reflect end of day', run: () => go(`/shutdown/${td}`) },
      { id: 'review',   label: 'Weekly review',   icon: BookOpen,      keywords: 'week wins', run: () => go(`/week/${ws}/review`) },
      { id: 'insights', label: 'Insights',        icon: BarChart3,     keywords: 'stats time tracking', run: () => go('/insights') },
      { id: 'journal',  label: 'Journal',         icon: BookOpen,      keywords: 'intentions reflections', run: () => go('/journal') },
      { id: 'reminders', label: 'Reminders',      icon: Bell,          keywords: 'notifications', run: () => go('/reminders') },
      { id: 'email',    label: 'Email',           icon: Mail,          keywords: 'inbox gmail fastmail', run: () => go('/email') },
      { id: 'settings', label: 'Settings',        icon: Settings,      keywords: 'preferences config account', run: () => go('/settings/accounts') },
      { id: 'theme',    label: 'Toggle dark mode', icon: SunMoon,      keywords: 'light dark appearance', run: () => theme.toggle() },
    );

    // Jira is optional — only offer it once the integration is actually connected.
    if (jiraStatus.connected) {
      list.push({
        id: 'jira', label: 'Jira', icon: SquareKanban, keywords: 'issues',
        run: () => go('/jira'),
      });
    }
    return list;
  });

  const q = $derived(query.trim().toLowerCase());
  const filtered = $derived.by<Command[]>(() => {
    if (!q) return commands;
    return commands.filter((c) => (c.label + ' ' + (c.keywords ?? '')).toLowerCase().includes(q));
  });

  // A search fallback so any query is never a dead end.
  const searchItem = $derived<Command | null>(
    query.trim()
      ? { id: 'search', label: `Search for “${query.trim()}”`, icon: Search, run: () => go(`/search?q=${encodeURIComponent(query.trim())}`) }
      : { id: 'search', label: 'Search everything', icon: Search, run: () => go('/search') },
  );

  const shown = $derived<Command[]>(searchItem ? [...filtered, searchItem] : filtered);

  // Reset + focus each time it opens; keep selection in range as the list shrinks.
  $effect(() => {
    if (commandPalette.open) {
      query = '';
      selected = 0;
      setTimeout(() => inputEl?.focus(), 20);
    }
  });
  $effect(() => {
    if (selected >= shown.length) selected = Math.max(0, shown.length - 1);
  });

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') { e.preventDefault(); commandPalette.close(); return; }
    if (e.key === 'ArrowDown') { e.preventDefault(); selected = Math.min(selected + 1, shown.length - 1); return; }
    if (e.key === 'ArrowUp') { e.preventDefault(); selected = Math.max(selected - 1, 0); return; }
    if (e.key === 'Enter') { e.preventDefault(); shown[selected]?.run(); return; }
  }
</script>

{#if commandPalette.open}
  <div class="fixed inset-0 z-[110] flex items-start justify-center p-4"
       style="background: color-mix(in srgb, var(--sempa-bg-rail) 30%, rgba(0,0,0,0.42));"
       onclick={(e) => { if (e.target === e.currentTarget) commandPalette.close(); }}
       role="presentation">
    <div class="surface-raised mt-[10vh] flex w-full max-w-lg flex-col overflow-hidden p-2.5"
         style="max-height: 70vh;"
         role="dialog" aria-modal="true" aria-label="Command palette">
      <div class="palette-field flex items-center gap-2.5 rounded-full px-4" style="height: 46px; background: var(--sempa-fill);">
        <Search size={17} style="color: var(--sempa-text-dim);" />
        <!-- svelte-ignore a11y_autofocus -->
        <input bind:this={inputEl} bind:value={query} onkeydown={onKeydown}
               placeholder="Search commands… (↑↓ to move, ↵ to run)"
               class="min-w-0 flex-1 bg-transparent text-sm outline-none" style="color: var(--sempa-text);" />
        <kbd class="hidden shrink-0 rounded-full px-2 py-0.5 text-[10.5px] sm:inline-block"
             style="background: var(--sempa-bg-panel); color: var(--sempa-text-dim); box-shadow: var(--sempa-shadow-1);">Esc</kbd>
      </div>
      <div class="flex-1 overflow-y-auto px-0.5 pb-0.5 pt-2">
        {#each shown as c, i (c.id)}
          <button onclick={c.run} onmouseenter={() => (selected = i)}
                  class="flex w-full items-center gap-3 rounded-[14px] px-3 py-2 text-left text-sm transition-colors"
                  style="background: {i === selected ? 'var(--sempa-accent-bg)' : 'transparent'};
                         color: {i === selected ? 'var(--sempa-accent-ink)' : 'var(--sempa-text)'};
                         font-weight: {i === selected ? 600 : 500};">
            <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-[10px]"
                  style="background: {i === selected ? 'var(--sempa-bg-panel)' : 'var(--sempa-fill)'};
                         color: {i === selected ? 'var(--sempa-accent-ink)' : 'var(--sempa-text-soft)'};">
              <c.icon size={16} />
            </span>
            <span class="flex-1 truncate">{c.label}</span>
            {#if c.hint}<span class="shrink-0 text-[11.5px] font-medium" style="color: var(--sempa-text-dim);">{c.hint}</span>{/if}
          </button>
        {/each}
      </div>
    </div>
  </div>
{/if}

<style>
  .palette-field input:focus-visible { box-shadow: none; }
</style>
