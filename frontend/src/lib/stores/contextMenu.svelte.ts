/**
 * App-wide right-click menu. Any surface (task card, objective, board) calls
 * `contextMenu.show(event, items)` from an `oncontextmenu` handler; a single
 * <ContextMenu> instance mounted in the root layout renders it. Centralising it
 * means only one menu is ever open and every surface gets the same styling.
 */
export type MenuItem =
  | { label: string; onClick: () => void; danger?: boolean; disabled?: boolean }
  | 'separator';

function createContextMenuStore() {
  let current = $state<{ x: number; y: number; items: MenuItem[] } | null>(null);

  return {
    get current() { return current; },
    /** Open the menu at the cursor. Suppresses the native menu and stops the
     *  event bubbling to ancestor handlers (so a card's menu wins over the
     *  board's). A no-op if there are no actionable items. */
    show(e: MouseEvent, items: MenuItem[]) {
      const actionable = items.filter((i) => i === 'separator' || !i.disabled);
      if (actionable.every((i) => i === 'separator')) return;
      e.preventDefault();
      e.stopPropagation();
      current = { x: e.clientX, y: e.clientY, items };
    },
    close() { current = null; },
  };
}

export const contextMenu = createContextMenuStore();
