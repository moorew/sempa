// Open-state for the global command palette (Cmd/Ctrl+K). Mounted once in the
// root layout; the palette itself builds its command list reactively.

class CommandPaletteStore {
  open = $state(false);

  show() { this.open = true; }
  close() { this.open = false; }
  toggle() { this.open = !this.open; }
}

export const commandPalette = new CommandPaletteStore();
