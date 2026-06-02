const DARK_KEY   = 'sempa-theme';
const ACCENT_KEY = 'sempa-accent';

export type AccentName = 'blue' | 'purple' | 'teal' | 'rose' | 'amber' | 'slate';

export const ACCENT_PRESETS: Record<AccentName, {
  label: string; swatch: string;
  a50: string; a100: string; a200: string; a400: string;
  a500: string; a600: string; a700: string; a900: string; a950: string;
}> = {
  blue: {
    label: 'Blue', swatch: '#3b82f6',
    a50:'#eff6ff', a100:'#dbeafe', a200:'#bfdbfe', a400:'#60a5fa',
    a500:'#3b82f6', a600:'#2563eb', a700:'#1d4ed8', a900:'#1e3a8a', a950:'#172554',
  },
  purple: {
    label: 'Purple', swatch: '#a855f7',
    a50:'#faf5ff', a100:'#f3e8ff', a200:'#e9d5ff', a400:'#c084fc',
    a500:'#a855f7', a600:'#9333ea', a700:'#7e22ce', a900:'#581c87', a950:'#3b0764',
  },
  teal: {
    label: 'Teal', swatch: '#14b8a6',
    a50:'#f0fdfa', a100:'#ccfbf1', a200:'#99f6e4', a400:'#2dd4bf',
    a500:'#14b8a6', a600:'#0d9488', a700:'#0f766e', a900:'#134e4a', a950:'#042f2e',
  },
  rose: {
    label: 'Rose', swatch: '#f43f5e',
    a50:'#fff1f2', a100:'#ffe4e6', a200:'#fecdd3', a400:'#fb7185',
    a500:'#f43f5e', a600:'#e11d48', a700:'#be123c', a900:'#881337', a950:'#4c0519',
  },
  amber: {
    label: 'Amber', swatch: '#f59e0b',
    a50:'#fffbeb', a100:'#fef3c7', a200:'#fde68a', a400:'#fbbf24',
    a500:'#f59e0b', a600:'#d97706', a700:'#b45309', a900:'#78350f', a950:'#451a03',
  },
  slate: {
    label: 'Slate', swatch: '#475569',
    a50:'#f8fafc', a100:'#f1f5f9', a200:'#e2e8f0', a400:'#94a3b8',
    a500:'#64748b', a600:'#475569', a700:'#334155', a900:'#0f172a', a950:'#020617',
  },
};

function createThemeStore() {
  let dark       = $state(false);
  let accentName = $state<AccentName>('blue');

  function init() {
    if (typeof localStorage === 'undefined') return;
    const savedDark   = localStorage.getItem(DARK_KEY);
    const prefersDark = typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches;
    dark = savedDark === 'dark' || (savedDark === null && prefersDark);
    applyDark();

    const savedAccent = localStorage.getItem(ACCENT_KEY) as AccentName | null;
    if (savedAccent && ACCENT_PRESETS[savedAccent]) accentName = savedAccent;
    applyAccent(accentName);
  }

  function applyDark() {
    if (typeof document === 'undefined') return;
    document.documentElement.classList.toggle('dark', dark);
  }

  function applyAccent(name: AccentName) {
    if (typeof document === 'undefined') return;
    const p = ACCENT_PRESETS[name];
    const r = document.documentElement.style;
    r.setProperty('--a50',  p.a50);
    r.setProperty('--a100', p.a100);
    r.setProperty('--a200', p.a200);
    r.setProperty('--a400', p.a400);
    r.setProperty('--a500', p.a500);
    r.setProperty('--a600', p.a600);
    r.setProperty('--a700', p.a700);
    r.setProperty('--a900', p.a900);
    r.setProperty('--a950', p.a950);
  }

  function toggleDark() {
    dark = !dark;
    localStorage.setItem(DARK_KEY, dark ? 'dark' : 'light');
    applyDark();
  }

  function setAccent(name: AccentName) {
    accentName = name;
    localStorage.setItem(ACCENT_KEY, name);
    applyAccent(name);
  }

  return {
    get dark()   { return dark; },
    get accent() { return accentName; },
    init,
    toggle: toggleDark,
    setAccent,
  };
}

export const theme = createThemeStore();
