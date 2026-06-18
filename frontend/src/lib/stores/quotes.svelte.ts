// Daily encouragement — a single, quiet quote surfaced on the day view to add a
// little personality without getting in the way. Purely client-side
// (localStorage), like the other display prefs.
//
// Quotes come from curated "packs" (business, history, science, …) that the user
// turns on or off and freely combines, plus any of their own custom quotes. See
// quotePacks.ts for the curated, fact-checked content. The day's quote is drawn
// from the union of every enabled pack plus the custom list.

import { today } from '$lib/utils';
import { QUOTE_PACKS, DEFAULT_ENABLED_PACKS, packById, type Quote, type QuotePack } from './quotePacks';

export type { Quote, QuotePack } from './quotePacks';
export { QUOTE_PACKS, packById } from './quotePacks';

const ENABLED_KEY = 'sempa.quotes.enabled';   // master on/off (existing)
const PACKS_KEY = 'sempa.quotes.packs';        // JSON string[] of enabled pack ids
const CUSTOM_KEY = 'sempa.quotes.custom';      // JSON Quote[] of user's own quotes
const LEGACY_LIST_KEY = 'sempa.quotes.list';   // pre-packs single list (for migration)

// The quotes that shipped as defaults before packs existed. Used only to tell a
// returning user's *custom* additions apart from the old built-ins during the
// one-time migration, so we keep what they typed and drop what was built in.
const LEGACY_DEFAULT_TEXTS = new Set<string>([
  'Let us pick up our books and our pens. They are our most powerful weapons.',
  'The challenge is in the moment; the time is always now.',
  'The time is always right to do what is right.',
  "If you're offered a seat on a rocket ship, don't ask what seat. Just get on.",
  "Winning isn't everything, but wanting to win is.",
  'You have within you, right now, everything you need to deal with whatever the world can throw at you.',
  'Character is power.',
  "Never be limited by other people's limited imaginations.",
  'Only a life lived for others is a life worthwhile.',
  'The only impossible journey is the one you never begin.',
  'If you fell down yesterday, stand up today.',
]);

// Stable hash so the same day always shows the same quote (no flicker), but it
// rotates day to day. Avoids Math.random so it's deterministic.
function hashDay(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
  return Math.abs(h);
}

function isQuote(v: unknown): v is Quote {
  return !!v && typeof (v as Quote).text === 'string' && typeof (v as Quote).author === 'string';
}

function createQuotesStore() {
  let enabled = $state(true);
  let enabledPacks = $state<Set<string>>(new Set(DEFAULT_ENABLED_PACKS));
  let custom = $state<Quote[]>([]);

  // A brief, transient quote shown on certain moments (app open, completing a
  // task). It auto-clears, so it's a quiet flourish rather than fixed UI.
  let momentQuote = $state<Quote | null>(null);
  let momentIdx = 0;
  let momentTimer: ReturnType<typeof setTimeout> | null = null;

  function persistPacks() {
    if (typeof localStorage !== 'undefined') localStorage.setItem(PACKS_KEY, JSON.stringify([...enabledPacks]));
  }
  function persistCustom() {
    if (typeof localStorage !== 'undefined') localStorage.setItem(CUSTOM_KEY, JSON.stringify(custom));
  }

  // One-time move from the old single editable list to packs + custom. We keep
  // anything the user typed themselves (not in the old built-in set) as custom
  // quotes, and start them on the default pack selection.
  function migrateLegacy() {
    if (typeof localStorage === 'undefined') return;
    try {
      const raw = localStorage.getItem(LEGACY_LIST_KEY);
      if (raw) {
        const v = JSON.parse(raw);
        if (Array.isArray(v)) {
          custom = v.filter(isQuote).filter((q) => !LEGACY_DEFAULT_TEXTS.has(q.text));
        }
      }
    } catch { /* ignore — just start clean */ }
    enabledPacks = new Set(DEFAULT_ENABLED_PACKS);
    persistPacks();
    persistCustom();
  }

  function init() {
    if (typeof localStorage === 'undefined') return;
    const e = localStorage.getItem(ENABLED_KEY);
    if (e !== null) enabled = e === '1';

    const packsRaw = localStorage.getItem(PACKS_KEY);
    if (packsRaw === null) {
      // First run on the packs-aware build: bring across any custom quotes.
      migrateLegacy();
    } else {
      try {
        const ids = JSON.parse(packsRaw);
        if (Array.isArray(ids)) enabledPacks = new Set(ids.filter((x) => typeof x === 'string'));
      } catch { enabledPacks = new Set(DEFAULT_ENABLED_PACKS); }
      try {
        const customRaw = localStorage.getItem(CUSTOM_KEY);
        if (customRaw) {
          const v = JSON.parse(customRaw);
          if (Array.isArray(v)) custom = v.filter(isQuote);
        }
      } catch { /* keep empty */ }
    }
  }

  /** The full pool the daily quote is drawn from: every enabled pack + custom. */
  function pool(): Quote[] {
    const out: Quote[] = [];
    for (const p of QUOTE_PACKS) if (enabledPacks.has(p.id)) out.push(...p.quotes);
    out.push(...custom);
    return out;
  }

  return {
    get enabled() { return enabled; },
    /** All packs, for the settings UI. */
    get packs(): QuotePack[] { return QUOTE_PACKS; },
    /** The user's own quotes. */
    get custom(): Quote[] { return custom; },
    /** Whether a given pack is currently switched on. */
    packEnabled(id: string) { return enabledPacks.has(id); },
    /** How many quotes are in the active pool (enabled packs + custom). */
    get poolSize() { return pool().length; },
    /** The quote to show today (stable through the day), or null when off/empty. */
    get todays(): Quote | null {
      if (!enabled) return null;
      const p = pool();
      if (p.length === 0) return null;
      return p[hashDay(today()) % p.length];
    },
    /** The current transient "moment" quote (or null). Cleared automatically. */
    get moment(): Quote | null { return momentQuote; },
    /** Briefly surface a quote (rotates through the pool) on a small action,
     *  e.g. completing a task. No-op when disabled or empty. */
    flash() {
      if (!enabled) return;
      const p = pool();
      if (p.length === 0) return;
      momentQuote = p[momentIdx % p.length];
      momentIdx++;
      if (momentTimer) clearTimeout(momentTimer);
      momentTimer = setTimeout(() => { momentQuote = null; }, 2600);
    },
    init,
    setEnabled(v: boolean) {
      enabled = v;
      if (typeof localStorage !== 'undefined') localStorage.setItem(ENABLED_KEY, v ? '1' : '0');
    },
    setPackEnabled(id: string, on: boolean) {
      const next = new Set(enabledPacks);
      if (on) next.add(id); else next.delete(id);
      enabledPacks = next;
      persistPacks();
    },
    togglePack(id: string) { this.setPackEnabled(id, !enabledPacks.has(id)); },
    /** Restore the default pack selection (does not touch custom quotes). */
    resetPacks() {
      enabledPacks = new Set(DEFAULT_ENABLED_PACKS);
      persistPacks();
    },
    /** Add a custom quote. Author is optional (blank → shown with no attribution). */
    addCustom(text: string, author: string) {
      const t = text.trim();
      if (!t) return;
      custom = [...custom, { text: t, author: author.trim() }];
      persistCustom();
    },
    removeCustom(index: number) {
      custom = custom.filter((_, i) => i !== index);
      persistCustom();
    },
  };
}

export const quotes = createQuotesStore();
