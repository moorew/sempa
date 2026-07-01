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
  import { theme } from '$lib/stores/theme.svelte';
  import { today, weekStart } from '$lib/utils';
  import { goto } from '$app/navigation';
  import {
    Sparkles, Search, Home, Layers, ListChecks, CalendarClock, ClipboardCheck,
    Moon, Bell, Mail, SquareKanban, BarChart3, BookOpen, Settings, SunMoon,
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
      { id: 'schedule', label: 'Schedule',        icon: CalendarClock, keywords: 'calendar timebox', run: () => go('/schedule') },
      { id: 'plan',     label: 'Plan today',      icon: ClipboardCheck, keywords: 'planning', run: () => go(`/plan/${td}`) },
      { id: 'shutdown', label: 'Shutdown',        icon: Moon,          keywords: 'reflect end of day', run: () => go(`/shutdown/${td}`) },
      { id: 'review',   label: 'Weekly review',   icon: BookOpen,      keywords: 'week wins', run: () => go(`/week/${ws}/review`) },
      { id: 'insights', label: 'Insights',        icon: BarChart3,     keywords: 'stats time tracking', run: () => go('/insights') },
      { id: 'journal',  label: 'Journal',         icon: BookOpen,      keywords: 'intentions reflections', run: () => go('/journal') },
      { id: 'reminders', label: 'Reminders',      icon: Bell,          keywords: 'notifications', run: () => go('/reminders') },
      { id: 'email',    label: 'Email',           icon: Mail,          keywords: 'inbox gmail fastmail', run: () => go('/email') },
      { id: 'jira',     label: 'Jira',            icon: SquareKanban,  keywords: 'issues', run: () => go('/jira') },
      { id: 'settings', label: 'Settings',        icon: Settings,      keywords: 'preferences config account', run: () => go('/settings/accounts') },
      { id: 'theme',    label: 'Toggle dark mode', icon: SunMoon,      keywords: 'light dark appearance', run: () => theme.toggle() },
    );
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
       style="background: rgba(0,0,0,0.45);"
       onclick={(e) => { if (e.target === e.currentTarget) commandPalette.close(); }}
       role="presentation">
    <div class="mt-[10vh] flex w-full max-w-lg flex-col overflow-hidden rounded-2xl shadow-2xl"
         style="max-height: 70vh; background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);"
         role="dialog" aria-modal="true" aria-label="Command palette">
      <div class="flex items-center gap-2.5 px-4 py-3" style="border-bottom: 1px solid var(--sempa-border);">
        <Search size={16} style="color: var(--sempa-text-dim);" />
        <!-- svelte-ignore a11y_autofocus -->
        <input bind:this={inputEl} bind:value={query} onkeydown={onKeydown}
               placeholder="Search commands… (↑↓ to move, ↵ to run)"
               class="flex-1 bg-transparent text-sm outline-none" style="color: var(--sempa-text);" />
      </div>
      <div class="flex-1 overflow-y-auto py-1.5">
        {#each shown as c, i (c.id)}
          <button onclick={c.run} onmouseenter={() => (selected = i)}
                  class="flex w-full items-center gap-3 px-4 py-2 text-left text-sm transition-colors"
                  style="background: {i === selected ? 'var(--sempa-accent-bg)' : 'transparent'};
                         color: {i === selected ? 'var(--sempa-accent)' : 'var(--sempa-text)'};">
            <c.icon size={16} class="shrink-0" style="opacity: 0.85;" />
            <span class="flex-1 truncate">{c.label}</span>
            {#if c.hint}<span class="shrink-0 text-[11px]" style="color: var(--sempa-text-dim);">{c.hint}</span>{/if}
          </button>
        {/each}
      </div>
    </div>
  </div>
{/if}
