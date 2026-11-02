/**
 * Open a URL in the user's default browser, cross-platform.
 *
 * - Tauri desktop: hand off to the OS via the `open_external` command so the
 *   link opens in the system default browser (not inside the app webview).
 * - Capacitor (Android): use the in-app browser.
 * - Web: a plain new tab.
 */
import { isTauri, isCapacitor } from '$lib/platform';
import { safeHref } from '$lib/utils';

export async function openExternal(url: string): Promise<void> {
    // Only ever hand a web link to the OS or a new window. Callers pass stored
    // data (task and calendar links); the Tauri command already refuses other
    // schemes, and falling back to window.open would undo that check.
    const href = safeHref(url);
    if (!href) return;
    url = href;

    if (isTauri()) {
        try {
            const { invoke } = await import('@tauri-apps/api/core');
            await invoke('open_external', { url });
            return;
        } catch {
            /* fall through to window.open */
        }
    }

    if (isCapacitor()) {
        try {
            const { Browser } = await import('@capacitor/browser');
            await Browser.open({ url });
            return;
        } catch {
            /* fall through */
        }
    }

    window.open(url, '_blank', 'noopener,noreferrer');
}
