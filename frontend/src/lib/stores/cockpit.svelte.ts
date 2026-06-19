/**
 * Cockpit mode — the ultrawide-short layout (e.g. a 32:9 strip / Corsair Xeneon
 * Edge). A geometry override that sits ABOVE the normal responsive ladder: when
 * the window is very wide and short we switch to the horizontal cockpit instead
 * of letterboxing the standard view.
 *
 * Trigger (spec §1.8 + companion study, which targets 2560×720 ≈ 3.56:1):
 *   aspect ratio ≥ 2.4  AND  height ≤ 900px
 * Expressed as `(min-aspect-ratio: 12/5) and (max-height: 900px)` (12/5 = 2.4).
 *
 * Two sub-modes once active: `glance` (calm, read-mostly) and `cockpit` (full
 * interactive). User-togglable; the choice persists.
 */
export type CockpitMode = 'glance' | 'cockpit';

const MODE_KEY = 'sempa-cockpit-mode';

class CockpitStore {
  /** True when the window geometry is ultrawide-short. */
  active = $state(false);
  /** Sub-layout once active. */
  mode = $state<CockpitMode>('cockpit');
  private mq: MediaQueryList | null = null;

  init() {
    if (typeof window === 'undefined') return;
    const saved = localStorage.getItem(MODE_KEY);
    if (saved === 'glance' || saved === 'cockpit') this.mode = saved;

    this.mq = window.matchMedia('(min-aspect-ratio: 12/5) and (max-height: 900px)');
    this.active = this.mq.matches;
    this.mq.onchange = (e) => { this.active = e.matches; };
  }

  setMode(m: CockpitMode) {
    this.mode = m;
    if (typeof localStorage !== 'undefined') localStorage.setItem(MODE_KEY, m);
  }

  toggleMode() {
    this.setMode(this.mode === 'cockpit' ? 'glance' : 'cockpit');
  }
}

export const cockpit = new CockpitStore();
