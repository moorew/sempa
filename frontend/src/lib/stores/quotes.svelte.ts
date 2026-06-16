// Daily encouragement — a single, quiet quote surfaced on the day view to add a
// little personality without getting in the way. Purely client-side
// (localStorage), like the other display prefs. Users can turn it off and edit
// the list in Settings.

import { today } from '$lib/utils';

export interface Quote {
  text: string;
  author: string;
}

// A small, non-cheesy starter set (seeded from the user's picks). The list is
// fully editable in Settings; Reset restores this.
export const DEFAULT_QUOTES: Quote[] = [
  { text: 'Let us pick up our books and our pens. They are our most powerful weapons.', author: 'Malala Yousafzai' },
  { text: 'The challenge is in the moment; the time is always now.', author: 'James Baldwin' },
  { text: 'The time is always right to do what is right.', author: 'Martin Luther King Jr.' },
  { text: "If you're offered a seat on a rocket ship, don't ask what seat. Just get on.", author: 'Sheryl Sandberg' },
  { text: "Winning isn't everything, but wanting to win is.", author: 'Vince Lombardi' },
  { text: 'You have within you, right now, everything you need to deal with whatever the world can throw at you.', author: 'Brian Tracy' },
  { text: 'Character is power.', author: 'Booker T. Washington' },
  { text: "Never be limited by other people's limited imaginations.", author: 'Mae C. Jemison' },
  { text: 'Only a life lived for others is a life worthwhile.', author: 'Albert Einstein' },
  { text: 'The only impossible journey is the one you never begin.', author: 'Tony Robbins' },
  { text: 'If you fell down yesterday, stand up today.', author: 'H.G. Wells' },
];

const ENABLED_KEY = 'sempa.quotes.enabled';
const LIST_KEY = 'sempa.quotes.list';

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
  let list = $state<Quote[]>([...DEFAULT_QUOTES]);

  function init() {
    if (typeof localStorage === 'undefined') return;
    const e = localStorage.getItem(ENABLED_KEY);
    if (e !== null) enabled = e === '1';
    try {
      const raw = localStorage.getItem(LIST_KEY);
      if (raw) {
        const v = JSON.parse(raw);
        if (Array.isArray(v)) list = v.filter(isQuote);
      }
    } catch { /* keep defaults */ }
  }

  function persist() {
    if (typeof localStorage !== 'undefined') localStorage.setItem(LIST_KEY, JSON.stringify(list));
  }

  return {
    get enabled() { return enabled; },
    get list() { return list; },
    /** The quote to show today (stable through the day), or null when off/empty. */
    get todays(): Quote | null {
      if (!enabled || list.length === 0) return null;
      return list[hashDay(today()) % list.length];
    },
    init,
    setEnabled(v: boolean) {
      enabled = v;
      if (typeof localStorage !== 'undefined') localStorage.setItem(ENABLED_KEY, v ? '1' : '0');
    },
    add(text: string, author: string) {
      const t = text.trim();
      if (!t) return;
      list = [...list, { text: t, author: author.trim() }];
      persist();
    },
    remove(index: number) {
      list = list.filter((_, i) => i !== index);
      persist();
    },
    reset() {
      list = [...DEFAULT_QUOTES];
      persist();
    },
  };
}

export const quotes = createQuotesStore();
