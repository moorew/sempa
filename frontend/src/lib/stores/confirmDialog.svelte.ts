// A small promise-based confirmation dialog, so destructive actions use a
// Sempa-themed modal instead of the browser's native confirm(). Mount
// <ConfirmDialog /> once in the root layout; call confirmDialog.ask(...) and
// await the boolean.

export interface ConfirmOptions {
  title: string;
  message?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
}

class ConfirmStore {
  open = $state(false);
  opts = $state<ConfirmOptions>({ title: '' });
  private resolver: ((v: boolean) => void) | null = null;

  ask(opts: ConfirmOptions): Promise<boolean> {
    this.opts = opts;
    this.open = true;
    return new Promise<boolean>((resolve) => { this.resolver = resolve; });
  }

  private settle(v: boolean) {
    this.open = false;
    const r = this.resolver;
    this.resolver = null;
    r?.(v);
  }
  confirm() { this.settle(true); }
  cancel() { this.settle(false); }
}

export const confirmDialog = new ConfirmStore();
