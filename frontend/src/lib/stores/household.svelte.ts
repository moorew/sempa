// Household awareness for the multi-user model. A single rune store, loaded once
// from GET /auth/me, that tells the UI whether this instance has more than one
// account — the signal for whether to surface the Private/Shared controls. On a
// solo install there's nothing to share with, so the controls stay hidden.

import { api } from '$lib/api';

class HouseholdStore {
  multiUser = $state(false);
  userId = $state<string | undefined>(undefined);
  isAdmin = $state(false);
  loaded = $state(false);

  async load() {
    try {
      const me = await api.auth.me();
      this.multiUser = me.multi_user === true;
      this.userId = me.user_id;
      this.isAdmin = me.is_admin === true;
    } catch {
      /* offline / no auth — leave defaults (controls hidden) */
    } finally {
      this.loaded = true;
    }
  }

  // Lazy single-flight load, safe to call from any editor's onMount.
  #pending: Promise<void> | null = null;
  ensure() {
    if (this.loaded) return Promise.resolve();
    if (!this.#pending) this.#pending = this.load();
    return this.#pending;
  }
}

export const household = new HouseholdStore();
