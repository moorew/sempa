// Every place you can navigate to on a phone, in one catalogue. The bottom tab
// bar shows the ones you've picked (Settings → Appearance → Bottom bar) and the
// More sheet lists everything else, so a destination is never unreachable.
//
// The desktop sidebar keeps its own grouping schemes in +layout.svelte; this
// catalogue only drives the phone chrome.

import {
  CalendarCheck, CalendarRange, ClipboardCheck, CalendarClock, FolderKanban, Layers,
  ListChecks, BookOpen, Search, Mail, Bell, SquareKanban, Moon, Gauge,
} from 'lucide-svelte';

export const NAV_IDS = [
  'today', 'week', 'plan', 'schedule', 'projects', 'backlog', 'lists', 'search',
  'email', 'reminders', 'jira',
  'shutdown', 'journal', 'insights',
] as const;
export type NavId = (typeof NAV_IDS)[number];

export type NavGroupName = 'Plan' | 'Inbox' | 'Review';

export interface NavDest {
  id: NavId;
  /** Full name, used in the More sheet and Settings. */
  label: string;
  /** Tab bar label: kept short so five fit on a narrow phone. */
  short: string;
  /** A lucide-svelte icon (they all share CalendarCheck's component type). */
  icon: typeof CalendarCheck;
  /** One of the fixed 1.27.1 hues, used with `.tint` for the icon square. */
  hue: string;
  group: NavGroupName;
  /** Pathname prefix that marks this destination as the current page. */
  prefix: string;
  href: (d: { today: string; week: string }) => string;
  /** Only offered when this integration is connected. */
  needs?: 'jira';
}

// The six fixed hues from the 1.27.1 colour pass.
const CLAY = '#d27a4c', SKY = '#5d8ccc', SAGE = '#5f9a6c', LILAC = '#9a7cc8', HONEY = '#d3a24a', ROSE = '#c76a86';

export const NAV_DESTS: NavDest[] = [
  { id: 'today',     label: 'Today',     short: 'Today',     icon: CalendarCheck,  hue: CLAY,  group: 'Plan',   prefix: '/home',      href: () => '/home' },
  { id: 'week',      label: 'This Week', short: 'Week',      icon: CalendarRange,  hue: SKY,   group: 'Plan',   prefix: '/week/',     href: (d) => `/week/${d.week}` },
  { id: 'plan',      label: 'Plan Day',  short: 'Plan',      icon: ClipboardCheck, hue: HONEY, group: 'Plan',   prefix: '/plan/',     href: (d) => `/plan/${d.today}` },
  { id: 'schedule',  label: 'Schedule',  short: 'Schedule',  icon: CalendarClock,  hue: SAGE,  group: 'Plan',   prefix: '/schedule',  href: () => '/schedule' },
  { id: 'projects',  label: 'Projects',  short: 'Projects',  icon: FolderKanban,   hue: LILAC, group: 'Plan',   prefix: '/projects',  href: () => '/projects' },
  { id: 'backlog',   label: 'Backlog',   short: 'Backlog',   icon: Layers,         hue: ROSE,  group: 'Plan',   prefix: '/backlog',   href: () => '/backlog' },
  { id: 'lists',     label: 'Lists',     short: 'Lists',     icon: ListChecks,     hue: SAGE,  group: 'Plan',   prefix: '/lists',     href: () => '/lists' },
  { id: 'search',    label: 'Search',    short: 'Search',    icon: Search,         hue: SKY,   group: 'Plan',   prefix: '/search',    href: () => '/search' },
  { id: 'email',     label: 'Email',     short: 'Email',     icon: Mail,           hue: SKY,   group: 'Inbox',  prefix: '/email',     href: () => '/email' },
  { id: 'reminders', label: 'Reminders', short: 'Reminders', icon: Bell,           hue: HONEY, group: 'Inbox',  prefix: '/reminders', href: () => '/reminders' },
  { id: 'jira',      label: 'Jira',      short: 'Jira',      icon: SquareKanban,   hue: LILAC, group: 'Inbox',  prefix: '/jira',      href: () => '/jira', needs: 'jira' },
  { id: 'shutdown',  label: 'Shutdown',  short: 'Shutdown',  icon: Moon,           hue: LILAC, group: 'Review', prefix: '/shutdown/', href: (d) => `/shutdown/${d.today}` },
  { id: 'journal',   label: 'Journal',   short: 'Journal',   icon: BookOpen,       hue: ROSE,  group: 'Review', prefix: '/journal',   href: () => '/journal' },
  { id: 'insights',  label: 'Insights',  short: 'Insights',  icon: Gauge,          hue: CLAY,  group: 'Review', prefix: '/insights',  href: () => '/insights' },
];

export const NAV_GROUPS: NavGroupName[] = ['Plan', 'Inbox', 'Review'];

const BY_ID = new Map(NAV_DESTS.map((d) => [d.id, d]));
export function navDest(id: NavId): NavDest | undefined { return BY_ID.get(id); }

/** What the tab bar showed before it was customisable (and still the default). */
export const DEFAULT_TABS: NavId[] = ['today', 'week', 'journal', 'lists'];
/** The bar always ends with More, so this many slots are left for destinations. */
export const MAX_TABS = 4;
export const MIN_TABS = 2;

/** Clean a stored list: known ids only, no repeats, within the slot limits. */
export function sanitizeTabs(raw: unknown): NavId[] {
  if (!Array.isArray(raw)) return [...DEFAULT_TABS];
  const seen = new Set<NavId>();
  for (const v of raw) {
    if (typeof v === 'string' && (NAV_IDS as readonly string[]).includes(v)) seen.add(v as NavId);
    if (seen.size === MAX_TABS) break;
  }
  return seen.size >= MIN_TABS ? [...seen] : [...DEFAULT_TABS];
}
