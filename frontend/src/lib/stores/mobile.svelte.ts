/**
 * Reactive layout-breakpoint store.
 *
 *  • mobile (≤767px)            → bottom / overlay nav
 *  • rail   (768–1040px)        → sidebar collapses to an icon-only rail
 *  • (>1040px)                  → full sidebar
 *
 * Cockpit mode (ultrawide-short) is a separate geometry override handled elsewhere.
 */
class MobileStore {
  /** ≤767px — bottom nav. */
  value = $state(false);
  /** 768–1040px — collapsed icon rail (desktop sidebar, labels hidden). */
  rail = $state(false);
  private mqMobile: MediaQueryList | null = null;
  private mqRail: MediaQueryList | null = null;

  init() {
    if (typeof window === 'undefined') return;
    this.mqMobile = window.matchMedia('(max-width: 767px)');
    this.value = this.mqMobile.matches;
    this.mqMobile.onchange = (e) => { this.value = e.matches; };

    this.mqRail = window.matchMedia('(min-width: 768px) and (max-width: 1040px)');
    this.rail = this.mqRail.matches;
    this.mqRail.onchange = (e) => { this.rail = e.matches; };
  }
}

export const mobile = new MobileStore();
