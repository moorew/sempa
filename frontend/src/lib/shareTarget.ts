/**
 * Share-to-Sempa: when the user shares text/a link into Sempa from another app
 * (Android ACTION_SEND), MainActivity stashes it; we drain it here on app start and
 * on each foreground, then open the day board with a prefilled new-task composer.
 *
 * A browser share usually sends a page title (subject) + URL (text): the title
 * becomes the task title and the URL goes into the notes (where it renders as a
 * link chip). A plain text share with no subject becomes the title.
 *
 * No-op off Capacitor/Android (or on an app build without the plugin method).
 */
import { isCapacitor } from '$lib/platform';
import { goto } from '$app/navigation';
import { today } from '$lib/utils';

interface WidgetBridgePlugin {
  consumePendingShare?(): Promise<{ text?: string; subject?: string }>;
}

function plugin(): WidgetBridgePlugin | null {
  if (typeof window === 'undefined' || !isCapacitor()) return null;
  return ((window as Window & { Capacitor?: { Plugins?: Record<string, unknown> } })
    .Capacitor?.Plugins?.WidgetBridge as WidgetBridgePlugin) ?? null;
}

let inited = false;
/** Wire the share-target drain. Call once at app start. */
export function initShareTarget(): void {
  if (inited || !plugin()) return;
  inited = true;
  void drain();
  // A share usually brings the app to the foreground — re-drain then.
  try {
    const App = (window as Window & { Capacitor?: { Plugins?: Record<string, { addListener?: (e: string, cb: (s: { isActive?: boolean }) => void) => void }> } })
      .Capacitor?.Plugins?.App;
    App?.addListener?.('appStateChange', (s) => { if (s?.isActive) void drain(); });
  } catch { /* ignore */ }
}

async function drain(): Promise<void> {
  const p = plugin();
  if (!p?.consumePendingShare) return;
  try {
    const r = await p.consumePendingShare();
    const text = (r?.text ?? '').trim();
    const subject = (r?.subject ?? '').trim();
    if (!text && !subject) return;
    const title = subject || text.split('\n')[0].slice(0, 200);
    const notes = subject ? text : (text === title ? '' : text);
    const qs = new URLSearchParams({ new: '1' });
    if (title) qs.set('title', title);
    if (notes) qs.set('notes', notes);
    await goto(`/day/${today()}?${qs.toString()}`);
  } catch { /* ignore */ }
}
