// Tracks whether the local AI model is actually usable (enabled + reachable on
// the server), so AI-assist buttons only appear when they'll work. Loaded once
// app-wide; refresh() re-checks after the user changes AI settings.
import { api } from '$lib/api';

function createAiStatus() {
  let reachable = $state(false);
  // Reactive so UI (e.g. the setup nudge) can wait for the first check to land
  // rather than flashing while reachable is still its default false.
  let loaded = $state(false);

  async function load() {
    if (loaded) return;
    try {
      const c = await api.integrations.aiTitle.get();
      reachable = !!c.enabled && !!c.reachable;
      loaded = true;
    } catch {
      /* leave reachable=false; will retry on next refresh */
    }
  }

  async function refresh() {
    loaded = false;
    await load();
  }

  return {
    get reachable() { return reachable; },
    get loaded() { return loaded; },
    load,
    refresh,
  };
}

export const aiStatus = createAiStatus();
