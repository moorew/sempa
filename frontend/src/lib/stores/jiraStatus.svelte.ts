// Tracks whether the Jira integration is actually configured, so Jira surfaces
// (day-board tab, command palette, backlog filter, mobile tile) only appear when
// they'd do something. Loaded once app-wide; refresh() re-checks after the user
// connects or disconnects Jira in settings.
//
// `loaded` exists so callers can wait for the first check instead of flashing a
// tab that's about to vanish — the same reason aiStatus has it.
import { api } from '$lib/api';

function createJiraStatus() {
  let connected = $state(false);
  let loaded = $state(false);

  async function load() {
    if (loaded) return;
    try {
      const c = await api.integrations.jira.get();
      connected = !!c.connected;
      loaded = true;
    } catch {
      // Offline / no server configured (pure local-first client): leave
      // connected=false so Jira stays hidden, and retry on the next refresh.
    }
  }

  async function refresh() {
    loaded = false;
    await load();
  }

  return {
    get connected() { return connected; },
    get loaded() { return loaded; },
    load,
    refresh,
  };
}

export const jiraStatus = createJiraStatus();
