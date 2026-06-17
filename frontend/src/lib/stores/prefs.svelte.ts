// User-facing display preferences that live purely on the client (localStorage).
// Keep this separate from theme.svelte so feature toggles don't bloat the theme
// store. Mirrors the same runes + init() pattern.

const CONTEXTUAL_KEY = 'sempa-contextual-reflections';
const NAV_GROUPING_KEY = 'sempa.navGrouping';
const NAV_SECTIONS_KEY = 'sempa.navSections';
const AI_FEATURES_KEY = 'sempa.aiFeatures';
const CELEBRATE_SOUND_KEY = 'sempa.celebrateSound';

// Per-feature toggles for the AI-assist features, so users are in full control of
// where the local model is used. All default ON, but every feature is also gated
// by AI being enabled + reachable on the server, so these only matter once AI is set up.
export type AiFeature =
  | 'quickAdd' | 'summarize' | 'suggestTags' | 'breakdown' | 'tidyNotes'
  | 'planDay' | 'weeklyReview' | 'reflection';

export const AI_FEATURE_META: { key: AiFeature; label: string; hint: string }[] = [
  { key: 'quickAdd',     label: 'Natural-language quick add', hint: 'Type "lunch with Sam thu 1pm 30m #personal" → a structured task.' },
  { key: 'summarize',    label: 'Email/Jira → task summary',  hint: 'Tidy imported items into a concise title with a time estimate.' },
  { key: 'suggestTags',  label: 'Suggest tags',               hint: 'Recommend tags for a task from your existing set.' },
  { key: 'breakdown',    label: 'Break into subtasks',        hint: 'Split a task into a few concrete subtasks.' },
  { key: 'tidyNotes',    label: 'Tidy up notes',              hint: 'Reformat messy notes into clean paragraphs and lists.' },
  { key: 'planDay',      label: 'Plan my day',                hint: 'Suggest an order for today’s tasks around your events.' },
  { key: 'weeklyReview', label: 'Draft weekly review',        hint: 'Draft wins / challenges / next focus from the week.' },
  { key: 'reflection',   label: 'Reflection prompts',         hint: 'Context-aware end-of-day questions in Shutdown.' },
];

type AiFeatures = Record<AiFeature, boolean>;
const AI_FEATURES_DEFAULT: AiFeatures = {
  quickAdd: true, summarize: true, suggestTags: true, breakdown: true, tidyNotes: true,
  planDay: true, weeklyReview: true, reflection: true,
};

// How the desktop navigation rail is organised. 'spaces' (default) groups by
// place; 'rhythm' groups by plan→focus→review; 'flat' is the original one-list.
export type NavGrouping = 'spaces' | 'rhythm' | 'flat';
// Whether grouped rails show mono-caps section names or quiet dividers.
export type NavSections = 'labels' | 'dividers';

const isGrouping = (v: string | null): v is NavGrouping =>
  v === 'spaces' || v === 'rhythm' || v === 'flat';
const isSections = (v: string | null): v is NavSections =>
  v === 'labels' || v === 'dividers';

function createPrefsStore() {
  // When true, intentions/reflections/week-review summaries are surfaced inline
  // on the day and week screens. The Journal page is unaffected by this toggle.
  let contextualReflections = $state(true);
  let navGrouping = $state<NavGrouping>('spaces');
  let navSections = $state<NavSections>('labels');
  let aiFeatures = $state<AiFeatures>({ ...AI_FEATURES_DEFAULT });
  // Soft chime on the day/week celebration moments. Off by default — the
  // celebrations are visual-first; sound is opt-in.
  let celebrateSound = $state(false);

  function init() {
    if (typeof localStorage === 'undefined') return;
    const saved = localStorage.getItem(CONTEXTUAL_KEY);
    if (saved !== null) contextualReflections = saved === '1';
    const g = localStorage.getItem(NAV_GROUPING_KEY);
    if (isGrouping(g)) navGrouping = g;
    const s = localStorage.getItem(NAV_SECTIONS_KEY);
    if (isSections(s)) navSections = s;
    const cs = localStorage.getItem(CELEBRATE_SOUND_KEY);
    if (cs !== null) celebrateSound = cs === '1';
    try {
      const raw = localStorage.getItem(AI_FEATURES_KEY);
      if (raw) aiFeatures = { ...AI_FEATURES_DEFAULT, ...JSON.parse(raw) };
    } catch { /* keep defaults */ }
  }

  function setContextualReflections(on: boolean) {
    contextualReflections = on;
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(CONTEXTUAL_KEY, on ? '1' : '0');
    }
  }

  function setNavGrouping(v: NavGrouping) {
    navGrouping = v;
    if (typeof localStorage !== 'undefined') localStorage.setItem(NAV_GROUPING_KEY, v);
  }

  function setNavSections(v: NavSections) {
    navSections = v;
    if (typeof localStorage !== 'undefined') localStorage.setItem(NAV_SECTIONS_KEY, v);
  }

  function setAiFeature(key: AiFeature, on: boolean) {
    aiFeatures = { ...aiFeatures, [key]: on };
    if (typeof localStorage !== 'undefined') localStorage.setItem(AI_FEATURES_KEY, JSON.stringify(aiFeatures));
  }

  function setCelebrateSound(on: boolean) {
    celebrateSound = on;
    if (typeof localStorage !== 'undefined') localStorage.setItem(CELEBRATE_SOUND_KEY, on ? '1' : '0');
  }

  return {
    get contextualReflections() { return contextualReflections; },
    get navGrouping() { return navGrouping; },
    get navSections() { return navSections; },
    get aiFeatures() { return aiFeatures; },
    get celebrateSound() { return celebrateSound; },
    /** True when a given AI feature is switched on by the user. */
    aiOn(key: AiFeature) { return aiFeatures[key]; },
    init,
    setContextualReflections,
    toggleContextualReflections: () => setContextualReflections(!contextualReflections),
    setNavGrouping,
    setNavSections,
    setAiFeature,
    setCelebrateSound,
    toggleCelebrateSound: () => setCelebrateSound(!celebrateSound),
  };
}

export const prefs = createPrefsStore();
