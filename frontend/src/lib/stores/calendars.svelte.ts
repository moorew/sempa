// Client-side calendar display preferences shared between the schedule view and
// the Calendars settings tab: which subscriptions are hidden, each calendar's
// brand-colour override, and the global display toggles. All persisted to
// localStorage so the choices stick across days and sessions. Kept as a single
// rune store (not per-component state) so toggling a calendar in Settings
// updates the schedule live.

export const BRAND_CAL_KEYS = ['terracotta', 'stone', 'sage', 'amber'] as const;
export type BrandCalKey = (typeof BRAND_CAL_KEYS)[number];

export const BRAND_CAL_LABEL: Record<BrandCalKey, string> = {
  terracotta: 'Terracotta', stone: 'Stone', sage: 'Sage', amber: 'Amber',
};

// Inline-style helpers — resolve to the --cal-* CSS vars so light/dark follow
// the theme automatically.
export const calFg = (key: BrandCalKey) => `var(--cal-${key}-fg)`;
export const calBg = (key: BrandCalKey) => `var(--cal-${key}-bg)`;

export type DisplayPrefs = {
  showDeclined: boolean;
  showAllDayWeek: boolean;
  dimPastEvents: boolean;
};
const DEFAULT_DISPLAY: DisplayPrefs = {
  showDeclined: false,
  showAllDayWeek: true,
  dimPastEvents: true,
};

const HIDDEN_KEY  = 'sempa_hidden_calendars';
const COLORS_KEY  = 'sempa_calendar_colors';
const ORDER_KEY   = 'sempa_calendar_order';
const DISPLAY_KEY = 'sempa_calendar_display';

function load<T>(key: string, fallback: T): T {
  if (typeof localStorage === 'undefined') return fallback;
  try { const v = localStorage.getItem(key); return v ? JSON.parse(v) : fallback; } catch { return fallback; }
}
function persist(key: string, v: unknown) {
  if (typeof localStorage !== 'undefined') localStorage.setItem(key, JSON.stringify(v));
}

// Deterministic fallback hue for a calendar we haven't registered an order for
// yet (so it still renders a stable brand colour rather than a blank).
function hashColor(id: string): BrandCalKey {
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) | 0;
  return BRAND_CAL_KEYS[Math.abs(h) % BRAND_CAL_KEYS.length];
}

function createCalendarStore() {
  let hidden  = $state<string[]>(load(HIDDEN_KEY, []));
  let colors  = $state<Record<string, BrandCalKey>>(load(COLORS_KEY, {}));
  // Stable assignment order of calendar ids → each gets a *distinct* brand hue
  // by position (so the first four calendars never share a colour). A plain hash
  // collides far too often with only four hues. Persisted so colours are stable
  // across sessions; new calendars take the next free hue as they appear.
  let order   = $state<string[]>(load(ORDER_KEY, []));
  let display = $state<DisplayPrefs>({ ...DEFAULT_DISPLAY, ...load<Partial<DisplayPrefs>>(DISPLAY_KEY, {}) });

  // The colour currently shown for a calendar: an explicit override, else the
  // distinct hue for its registered position, else a stable hash fallback.
  const resolveKey = (id: string): BrandCalKey => {
    if (colors[id]) return colors[id];
    const idx = order.indexOf(id);
    return idx >= 0 ? BRAND_CAL_KEYS[idx % BRAND_CAL_KEYS.length] : hashColor(id);
  };

  return {
    get display() { return display; },
    isHidden: (id: string) => hidden.includes(id),
    // Record calendars so each gets a distinct, stable default hue. Called by
    // the schedule view and the Calendars settings tab as feeds load.
    register(ids: string[]) {
      const missing = ids.filter((id) => id && !order.includes(id)).sort();
      if (missing.length) { order = [...order, ...missing]; persist(ORDER_KEY, order); }
    },
    colorKey: (id: string): BrandCalKey => resolveKey(id),
    toggleHidden(id: string) {
      hidden = hidden.includes(id) ? hidden.filter((x) => x !== id) : [...hidden, id];
      persist(HIDDEN_KEY, hidden);
    },
    setHidden(id: string, hide: boolean) {
      const has = hidden.includes(id);
      if (hide && !has) hidden = [...hidden, id];
      else if (!hide && has) hidden = hidden.filter((x) => x !== id);
      persist(HIDDEN_KEY, hidden);
    },
    cycleColor(id: string) {
      const cur = resolveKey(id);
      const next = BRAND_CAL_KEYS[(BRAND_CAL_KEYS.indexOf(cur) + 1) % BRAND_CAL_KEYS.length];
      colors = { ...colors, [id]: next };
      persist(COLORS_KEY, colors);
    },
    setDisplay<K extends keyof DisplayPrefs>(k: K, v: DisplayPrefs[K]) {
      display = { ...display, [k]: v };
      persist(DISPLAY_KEY, display);
    },
  };
}

export const calendars = createCalendarStore();
