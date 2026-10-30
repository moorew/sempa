import { api } from '$lib/api';
import type { Project, ProjectStatus, ProjectCategory } from '$lib/types';

// Projects + their (user-customisable) statuses, shared by the project picker,
// card colour dots and the weekly-planning reference panel. Reloaded from the
// layout on `project:change` and after a local-first sync pull, like tagStore.
function createProjectStore() {
  let projects = $state<Project[]>([]);
  let statuses = $state<ProjectStatus[]>([]);
  let loaded = $state(false);
  let loading = false;

  async function fetchAll() {
    const [p, s] = await Promise.all([api.projects.list(), api.projectStatuses.list()]);
    projects = p;
    statuses = s;
    loaded = true;
  }

  // Load once; safe to call repeatedly (see tagStore.load for why `loaded` is
  // only set on success).
  async function load() {
    if (loaded || loading) return;
    loading = true;
    try { await fetchAll(); } catch { /* non-fatal */ } finally { loading = false; }
  }

  async function reload() {
    if (loading) return;
    loading = true;
    try { await fetchAll(); } catch { /* non-fatal */ } finally { loading = false; }
  }

  function get(id: string | null | undefined): Project | undefined {
    return id ? projects.find((p) => p.id === id) : undefined;
  }

  function status(id: string | null | undefined): ProjectStatus | undefined {
    return id ? statuses.find((s) => s.id === id) : undefined;
  }

  // A project whose status has gone missing (e.g. not synced yet) counts as active.
  function categoryOf(p: Project): ProjectCategory {
    return status(p.status_id)?.category ?? 'active';
  }

  // Projects you'd reasonably file new work into: everything not done/archived.
  const open = $derived(projects.filter((p) => {
    const c = categoryOf(p);
    return c !== 'done' && c !== 'archived';
  }));

  return {
    get projects() { return projects; },
    get statuses() { return statuses; },
    get open() { return open; },
    get loaded() { return loaded; },
    load,
    reload,
    get,
    status,
    categoryOf,
  };
}

export const projectStore = createProjectStore();

// The fixed categories in display order, with friendly labels for Settings.
export const PROJECT_CATEGORIES: { id: ProjectCategory; label: string; hint: string }[] = [
  { id: 'planning', label: 'Planning', hint: 'Not started yet' },
  { id: 'active',   label: 'Active',   hint: 'Shows in weekly planning' },
  { id: 'on_hold',  label: 'On hold',  hint: 'Paused' },
  { id: 'done',     label: 'Done',     hint: 'Finished — hidden by default' },
  { id: 'archived', label: 'Archived', hint: 'Put away — hidden by default' },
];

// Palette offered when picking a project colour (brand terracotta first).
export const PROJECT_COLORS = [
  '#b3592e', '#c99a2e', '#4f8a5b', '#2e7d8c', '#3f63b3', '#7a4fb3', '#b34f86', '#8a8178',
];
