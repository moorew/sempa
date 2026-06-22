/**
 * Mobile deep-link routing (Capacitor/Android). The home-screen long-press App
 * Shortcuts launch the app with a `com.clevercode.sempa://<action>` URL; we map it
 * to an in-app route with the shared router and navigate. Covers both cold start
 * (getLaunchUrl) and warm (appUrlOpen). No-op off Capacitor.
 */
import { isCapacitor } from '$lib/platform';
import { routeForDeepLink } from '$lib/tauri/deeplink';

interface AppPlugin {
  getLaunchUrl?(): Promise<{ url?: string } | null>;
  addListener?(event: string, cb: (data: { url?: string }) => void): unknown;
}

function appPlugin(): AppPlugin | null {
  if (typeof window === 'undefined' || !isCapacitor()) return null;
  return ((window as Window & { Capacitor?: { Plugins?: Record<string, unknown> } })
    .Capacitor?.Plugins?.App as AppPlugin) ?? null;
}

let inited = false;
/** Wire mobile deep links. `navigate` is the app router (goto). Call once at start. */
export async function initMobileDeepLinks(navigate: (url: string) => void): Promise<void> {
  const App = appPlugin();
  if (!App || inited) return;
  inited = true;

  const handle = (url?: string) => {
    if (!url) return;
    const route = routeForDeepLink(url);
    if (route) navigate(route);
  };

  try { handle((await App.getLaunchUrl?.())?.url); } catch { /* ignore */ }
  try { App.addListener?.('appUrlOpen', (d) => handle(d?.url)); } catch { /* ignore */ }
}
