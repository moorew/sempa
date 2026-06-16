/**
 * Tracks whether a blocking overlay (the task editor panel, etc.) is currently
 * open, so ambient floating widgets — chiefly the bottom-corner SyncIndicator —
 * can step out of the way instead of covering the panel's action buttons.
 *
 * A counter rather than a boolean so nested/overlapping opens don't clear the
 * flag early: each opener push()es on open and pop()s on close.
 */
function createOverlayStore() {
  let count = $state(0);
  return {
    get open() {
      return count > 0;
    },
    push() {
      count++;
    },
    pop() {
      count = Math.max(0, count - 1);
    },
  };
}

export const overlay = createOverlayStore();
