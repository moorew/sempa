// Global open-state for the AI Import modal, so both entry points (the dedicated
// button and the keyboard quick-launch) can open the single modal mounted in the
// root layout, from anywhere in the app.

class ImportModalStore {
  open = $state(false);
  // Optional seed for the input (e.g. a shared URL).
  seed = $state('');

  show(seed = '') {
    this.seed = seed;
    this.open = true;
  }
  close() {
    this.open = false;
    this.seed = '';
  }
}

export const importModal = new ImportModalStore();
