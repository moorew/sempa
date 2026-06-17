/**
 * `dismissibleSheet` — Svelte action for drag-to-dismiss bottom sheets that
 * coexists with scrollable content.
 *
 * The previous implementation attached touch handlers to the whole sheet and
 * translated it on any downward swipe, which fought with the inner scroll area:
 * scrolling back up dragged (and often dismissed) the sheet. This action only
 * starts a dismiss drag when:
 *   - the gesture begins on the drag handle (`data-sheet-handle`), OR
 *   - the scroll container is already at the top AND the pull is a deliberate,
 *     mostly-vertical downward drag that did NOT begin inside a nested
 *     scrollable area (a menu/list/picker) or an opted-out region.
 *
 * The deliberate-pull guard (a larger slop + verticality test for content
 * drags) is what stops the sheet feeling "touchy" on Android: scrolling a menu
 * or flicking the body near the top no longer dismisses the whole editor.
 * Mark any region that should never start a dismiss with `data-no-sheet-drag`.
 *
 * Otherwise the touch is left alone so native scrolling works normally. The
 * action owns the element's `transform`/`transition`, so the component should
 * not also bind those.
 */
export interface SheetOptions {
  onClose: () => void;
  /** CSS selector (within the node) for the scrollable content container. */
  scrollSelector?: string;
  /** Drag distance in px past which the sheet dismisses. Default 110. */
  threshold?: number;
  /** Optional haptic callback fired when a drag actually dismisses the sheet. */
  onDismissHaptic?: () => void;
}

const SLOP = 8;               // handle drags react quickly
const CONTENT_SLOP = 30;      // content pulls must travel further before they count
const VERTICAL_RATIO = 1.3;   // dy must dominate dx by this much for a content pull
// Keep the `bottom` transition (sheets animate `bottom` to ride above the soft
// keyboard) alongside the transform spring so settling a drag doesn't disable
// the keyboard-follow animation.
const SPRING = 'transform 320ms cubic-bezier(0.32, 0.72, 0, 1), bottom 180ms ease-out';

export function dismissibleSheet(node: HTMLElement, options: SheetOptions) {
  let opts = options;
  let startY = 0;
  let startX = 0;
  let delta = 0;
  let allow = false;       // was the gesture eligible to become a drag?
  let fromHandle_ = false; // did the gesture start on the drag handle?
  let decided = false;     // have we committed to scroll vs drag?
  let dragging = false;    // are we actively translating the sheet?
  let dragOrigin = 0;      // dy at the moment we committed to a drag

  function scrollEl(): HTMLElement | null {
    return opts.scrollSelector
      ? node.querySelector<HTMLElement>(opts.scrollSelector)
      : null;
  }

  function fromHandle(target: EventTarget | null): boolean {
    return target instanceof Element && !!target.closest('[data-sheet-handle]');
  }

  // The nearest vertically-scrollable ancestor of the touch (up to the sheet).
  // Used to tell "pulling the sheet body at the top" apart from "scrolling a
  // menu/list that lives inside the sheet".
  function nearestScrollable(target: EventTarget | null): HTMLElement | null {
    const gcs = typeof getComputedStyle === 'function' ? getComputedStyle : null;
    if (!gcs) return null;
    let el = target instanceof Element ? (target as HTMLElement) : null;
    while (el && el !== node) {
      const oy = gcs(el).overflowY;
      if ((oy === 'auto' || oy === 'scroll') && el.scrollHeight > el.clientHeight + 1) return el;
      el = el.parentElement;
    }
    return null;
  }

  /** Drag may start from the handle, or from the sheet body scrolled to top. */
  function eligible(target: EventTarget | null): boolean {
    if (fromHandle(target)) return true;
    // Explicit opt-out for menus/pickers that must never trigger a dismiss.
    if (target instanceof Element && target.closest('[data-no-sheet-drag]')) return false;
    const sc = scrollEl();
    if (!sc) return false; // no wired scroller → only the handle dismisses
    // If the touch began inside a DIFFERENT scrollable (a nested menu/list),
    // leave it to scroll — never dismiss the sheet from there.
    const nearest = nearestScrollable(target);
    if (nearest && nearest !== sc) return false;
    return sc.scrollTop <= 0;
  }

  function onStart(e: TouchEvent) {
    if (e.touches.length !== 1) return;
    startY = e.touches[0].clientY;
    startX = e.touches[0].clientX;
    delta = 0;
    decided = false;
    dragging = false;
    fromHandle_ = fromHandle(e.target);
    allow = eligible(e.target);
    // Defensive reset: if a previous gesture was interrupted (e.g. the keyboard
    // opened mid-drag and the webview swallowed the touchend), a stale transform
    // could be left in place, making the sheet look stuck/frozen. Clear it so
    // every new touch starts from a clean, fully-interactive state. We do NOT
    // touch `transition` here — only onMove suppresses it once we truly drag, so
    // plain taps and scrolls never alter the element's styles.
    if (node.style.transform && node.style.transform !== 'translateY(0px)') {
      node.style.transition = 'none';
      node.style.transform = 'translateY(0px)';
    }
  }

  function onMove(e: TouchEvent) {
    if (e.touches.length !== 1) return;
    const dy = e.touches[0].clientY - startY;
    const dx = e.touches[0].clientX - startX;

    if (!decided) {
      // Handle drags react at a small slop; content pulls must be a deliberate,
      // mostly-vertical downward travel before they're allowed to grab the sheet
      // — this is what keeps scrolling/flicking near the top from dismissing it.
      const slop = fromHandle_ ? SLOP : CONTENT_SLOP;
      if (Math.abs(dy) < slop && Math.abs(dx) < slop) return;
      decided = true;
      dragging = allow && dy > 0 &&
        (fromHandle_ ? Math.abs(dy) > Math.abs(dx)
                     : Math.abs(dy) > Math.abs(dx) * VERTICAL_RATIO);
      // Suppress the spring transition only now that we own a drag, so the
      // translate tracks the finger 1:1.
      if (dragging) {
        // Anchor the translate at the slop boundary so the sheet eases from 0
        // rather than snapping by the (possibly large) content slop. A fixed
        // offset (not the raw dy at commit) keeps the feel identical regardless
        // of how coarse the incoming touchmove samples are.
        dragOrigin = slop;
        node.style.transition = 'none';
        // Retract the soft keyboard as soon as a dismiss drag starts so the
        // sheet glides smoothly instead of jumping when focus is lost mid-gesture.
        const active = document.activeElement;
        if (active instanceof HTMLElement &&
            active.matches('input, textarea, [contenteditable]')) {
          active.blur();
        }
      }
    }

    if (!dragging) return;
    delta = Math.max(0, dy - dragOrigin);
    e.preventDefault(); // suppress content scroll while we own the gesture
    // Light resistance so it feels physical rather than 1:1 sticky.
    node.style.transform = `translateY(${delta}px)`;
  }

  function onEnd() {
    if (dragging) {
      node.style.transition = SPRING;
      if (delta > (opts.threshold ?? 110)) {
        opts.onDismissHaptic?.();
        node.style.transform = 'translateY(100%)';
        opts.onClose();
      } else {
        node.style.transform = 'translateY(0px)';
      }
    }
    decided = false;
    dragging = false;
    delta = 0;
    dragOrigin = 0;
  }

  node.addEventListener('touchstart', onStart, { passive: true });
  node.addEventListener('touchmove', onMove, { passive: false });
  node.addEventListener('touchend', onEnd, { passive: true });
  node.addEventListener('touchcancel', onEnd, { passive: true });

  return {
    update(next: SheetOptions) {
      opts = next;
    },
    destroy() {
      node.removeEventListener('touchstart', onStart);
      node.removeEventListener('touchmove', onMove);
      node.removeEventListener('touchend', onEnd);
      node.removeEventListener('touchcancel', onEnd);
    },
  };
}
