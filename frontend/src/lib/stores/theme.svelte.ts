const DARK_KEY       = 'sempa-theme';        // 'dark' | 'light' | absent (System)
const THEME_NAME_KEY = 'sempa-theme-name';   // which of the six interface themes
const SCALE_KEY      = 'sempa-text-scale';   // root font-size percent
const ACCENT_KEY     = 'sempa-accent';       // legacy (15-swatch accent) — migrated away
const UIFONT_KEY     = 'sempa-ui-font';      // 'brand' | 'system' (Use system UI font)
const MATCH_ACCENT_KEY = 'sempa-match-accent'; // '1' → accent follows the system (Linux)

export type ColorMode = 'system' | 'light' | 'dark';
export type UiFont = 'brand' | 'system';

export type ThemeName = 'terracotta' | 'forest' | 'plum' | 'slate' | 'oled' | 'ocean';

export type ThemeMeta = {
  id: ThemeName;
  label: string;
  sublabel: string;
  /** OLED is a dark-only theme — the light/dark toggle is hidden/disabled for it. */
  darkOnly?: boolean;
};

/** Curated full-interface themes (colour values live in src/themes.css). */
export const THEMES: ThemeMeta[] = [
  { id: 'terracotta', label: 'Terracotta', sublabel: 'Warm clay' },
  { id: 'forest',     label: 'Forest',     sublabel: 'Pine green' },
  { id: 'plum',       label: 'Plum',       sublabel: 'Aubergine' },
  { id: 'slate',      label: 'Slate',      sublabel: 'Graphite' },
  { id: 'oled',       label: 'OLED Black', sublabel: 'Dark only', darkOnly: true },
  { id: 'ocean',      label: 'Ocean',      sublabel: 'Marine blue' },
];

const THEME_IDS = THEMES.map((t) => t.id);
const isTheme = (v: string | null): v is ThemeName => !!v && (THEME_IDS as string[]).includes(v);

function createThemeStore() {
  let dark      = $state(false);
  let themeName = $state<ThemeName>('terracotta');
  let textScale = $state(100); // percent, e.g. 90 / 100 / 110
  let uiFont      = $state<UiFont>('brand');
  let matchAccent = $state(false);

  /** Whether the system currently prefers a dark scheme. */
  function systemPrefersDark(): boolean {
    return typeof window !== 'undefined'
      && !!window.matchMedia?.('(prefers-color-scheme: dark)').matches;
  }

  function init() {
    if (typeof localStorage === 'undefined') return;

    // ── One-time migration: the old 15-swatch accent picker → themes. Any
    // legacy accent maps to the terracotta default (the themes aren't 1:1 with
    // the old accents). Then drop the legacy key for good.
    if (!localStorage.getItem(THEME_NAME_KEY) && localStorage.getItem(ACCENT_KEY)) {
      localStorage.setItem(THEME_NAME_KEY, 'terracotta');
    }
    localStorage.removeItem(ACCENT_KEY);

    const savedTheme = localStorage.getItem(THEME_NAME_KEY);
    if (isTheme(savedTheme)) themeName = savedTheme;
    applyTheme(themeName);

    const savedDark = localStorage.getItem(DARK_KEY);
    const prefersDark = typeof window !== 'undefined'
      && window.matchMedia?.('(prefers-color-scheme: dark)').matches;
    // OLED forces dark regardless of the saved light/dark preference (which is
    // preserved untouched so it returns when the user picks another theme).
    dark = themeName === 'oled' || savedDark === 'dark' || (savedDark === null && !!prefersDark);
    applyDark();

    const savedScale = localStorage.getItem(SCALE_KEY);
    if (savedScale) {
      const n = parseInt(savedScale, 10);
      if (n >= 80 && n <= 130) textScale = n;
    }
    applyScale(textScale);

    uiFont = localStorage.getItem(UIFONT_KEY) === 'system' ? 'system' : 'brand';
    applyUiFont();
    matchAccent = localStorage.getItem(MATCH_ACCENT_KEY) === '1';
    applyAccent();
    applyThemeColor();

    // Live-follow the system scheme: while in System mode (no explicit light/dark
    // pref) and not OLED, flip when the OS theme changes — no restart needed.
    if (typeof window !== 'undefined' && window.matchMedia) {
      const mq = window.matchMedia('(prefers-color-scheme: dark)');
      mq.addEventListener?.('change', (e) => {
        if (localStorage.getItem(DARK_KEY) === null && themeName !== 'oled') {
          dark = e.matches;
          applyDark();
          applyThemeColor();
        }
      });
    }
  }

  function applyUiFont() {
    if (typeof document === 'undefined') return;
    document.documentElement.classList.toggle('system-font', uiFont === 'system');
  }

  // "Match system accent" (Linux/GNOME 47+/KDE): when the webview exposes the
  // platform accent via the CSS AccentColor system colour, drive --sempa-accent
  // from it (inline style wins over the per-theme stylesheet value). Guarded by
  // CSS.supports so an engine without AccentColor cleanly keeps brand terracotta.
  function applyAccent() {
    if (typeof document === 'undefined') return;
    const el = document.documentElement;
    const supported = typeof CSS !== 'undefined' && CSS.supports?.('color', 'AccentColor');
    if (matchAccent && supported) {
      el.style.setProperty('--sempa-accent', 'AccentColor');
    } else {
      el.style.removeProperty('--sempa-accent');
    }
  }

  function applyTheme(name: ThemeName) {
    if (typeof document === 'undefined') return;
    document.documentElement.dataset.theme = name;
  }

  function applyDark() {
    if (typeof document === 'undefined') return;
    document.documentElement.classList.toggle('dark', dark);
  }

  function applyScale(pct: number) {
    if (typeof document === 'undefined') return;
    document.documentElement.style.fontSize = `${pct}%`;
  }

  // Keep the browser/PWA chrome (address bar, Android status bar) in step with
  // the active theme by syncing <meta name="theme-color"> to the surface colour.
  // Runs after the theme/dark attrs are set, so the computed var is current.
  function applyThemeColor() {
    if (typeof document === 'undefined') return;
    const meta = document.querySelector('meta[name="theme-color"]');
    if (!meta) return;
    const c = getComputedStyle(document.documentElement).getPropertyValue('--sempa-bg-main').trim();
    if (c) meta.setAttribute('content', c);
    // Keep the home-screen widgets in step with the active theme (Android only).
    void import('$lib/widget-bridge').then((m) => m.syncWidgetTheme()).catch(() => {});
  }

  /** True for the active light/dark preference, ignoring an OLED override. */
  function savedDarkPref(): boolean {
    const saved = localStorage.getItem(DARK_KEY);
    const prefersDark = typeof window !== 'undefined'
      && window.matchMedia?.('(prefers-color-scheme: dark)').matches;
    return saved === 'dark' || (saved === null && !!prefersDark);
  }

  function setTheme(name: ThemeName) {
    themeName = name;
    localStorage.setItem(THEME_NAME_KEY, name);
    applyTheme(name);

    if (name === 'oled') {
      // Dark-only — force dark for the session WITHOUT clobbering the saved
      // light/dark preference, so it's restored when leaving OLED.
      if (!dark) { dark = true; applyDark(); }
    } else {
      // Restore whatever light/dark preference was saved before (OLED never
      // overwrote it).
      const wantDark = savedDarkPref();
      if (dark !== wantDark) { dark = wantDark; applyDark(); }
    }
    applyThemeColor();
  }

  function setScale(pct: number) {
    textScale = Math.min(130, Math.max(80, pct));
    localStorage.setItem(SCALE_KEY, String(textScale));
    applyScale(textScale);
  }

  function toggleDark() {
    if (themeName === 'oled') return; // OLED is dark-only — toggle is a no-op
    dark = !dark;
    localStorage.setItem(DARK_KEY, dark ? 'dark' : 'light');
    applyDark();
    applyThemeColor();
  }

  /** Explicit appearance mode: System (follow OS, live), Light, or Dark. */
  function setMode(mode: ColorMode) {
    if (themeName === 'oled') return; // dark-only theme ignores the mode switch
    if (mode === 'system') {
      localStorage.removeItem(DARK_KEY);
      dark = systemPrefersDark();
    } else {
      localStorage.setItem(DARK_KEY, mode);
      dark = mode === 'dark';
    }
    applyDark();
    applyThemeColor();
  }

  function setUiFont(font: UiFont) {
    uiFont = font;
    localStorage.setItem(UIFONT_KEY, font);
    applyUiFont();
  }

  function setMatchAccent(on: boolean) {
    matchAccent = on;
    localStorage.setItem(MATCH_ACCENT_KEY, on ? '1' : '0');
    applyAccent();
  }

  return {
    get dark()      { return dark; },
    get theme()     { return themeName; },
    get textScale() { return textScale; },
    /** 'system' | 'light' | 'dark' — the explicit appearance mode. */
    get mode(): ColorMode {
      const saved = typeof localStorage !== 'undefined' ? localStorage.getItem(DARK_KEY) : null;
      return saved === 'dark' ? 'dark' : saved === 'light' ? 'light' : 'system';
    },
    get systemFont() { return uiFont === 'system'; },
    /** True when the accent is being driven from the system accent colour. */
    get matchAccent() { return matchAccent; },
    /** Whether this engine can follow the system accent (CSS AccentColor). */
    get canMatchAccent() {
      return typeof CSS !== 'undefined' && !!CSS.supports?.('color', 'AccentColor');
    },
    /** True when the active theme can't switch modes (OLED). */
    get darkOnly()  { return themeName === 'oled'; },
    THEMES,
    init,
    toggle: toggleDark,
    setMode,
    setTheme,
    setScale,
    setUiFont,
    setMatchAccent,
  };
}

export const theme = createThemeStore();
