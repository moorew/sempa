/**
 * Desktop deep-link routing for the `sempa://` URL scheme.
 *
 * Two sources of links, both handled here:
 *   • Cold start  — the OS launches Sempa with a sempa:// URL in argv; the
 *     deep-link plugin exposes it via getCurrent().
 *   • Warm        — Sempa is already running; the single-instance plugin forwards
 *     the second launch's URL and the plugin fires onOpenUrl().
 *
 * Routes (Phase 1):
 *   sempa://new        → today's board, compose open      (.desktop "New task")
 *   sempa://plan       → today's guided plan              (.desktop "Plan day")
 *   sempa://shutdown   → today's shutdown ritual          (.desktop "Shutdown ritual")
 *   sempa://oauth/...  → reserved for the OAuth redirect handler (Phase 3)
 *
 * Tauri-only; a no-op in the browser/Capacitor.
 */
import { isTauri } from '$lib/platform';
import { today } from '$lib/utils';

/** Translate a sempa:// URL into an in-app route, or null to ignore it. */
export function routeForDeepLink(raw: string): string | null {
    let url: URL;
    try {
        url = new URL(raw);
    } catch {
        return null;
    }
    // Desktop uses the `sempa://` scheme; Android registers `com.clevercode.sempa://`
    // (Capacitor's custom scheme) — accept both so the same router serves both.
    if (url.protocol !== 'sempa:' && url.protocol !== 'com.clevercode.sempa:') return null;

    // sempa://<host><path> — the host is the action (URL drops the leading //).
    const action = (url.hostname || url.pathname.replace(/^\/+/, '')).toLowerCase();
    const d = today();

    switch (action) {
        case 'new':
        case 'new-task':
            return `/day/${d}?new=1`;
        case 'plan':
        case 'plan-day':
            return `/plan/${d}`;
        case 'shutdown':
            return `/shutdown/${d}`;
        case 'login':
        case 'oauth':
            // OAuth return from the system browser: sempa://login?link_token=…&redirect=…
            // Forward to the login page WITH the query so it can finalize the token.
            return `/login${url.search}`;
        case 'drive':
        case 'drive-backup':
            // Google Drive backup OAuth return (Android Custom Tab / system browser):
            // com.clevercode.sempa://drive-backup?drive=connected — land on the backup
            // page WITH the query so it can show the result and re-check status.
            return `/settings/backup${url.search}`;
        default:
            return '/home';
    }
}

/**
 * Wire deep-link handling. `navigate` is the app router (goto). Returns a
 * cleanup function (or null off-desktop).
 */
export async function initDeepLinks(
    navigate: (url: string) => void,
): Promise<(() => void) | null> {
    if (!isTauri()) return null;

    const handle = (urls: string[] | null) => {
        if (!urls) return;
        for (const raw of urls) {
            const route = routeForDeepLink(raw);
            if (route) navigate(route);
        }
    };

    try {
        const { onOpenUrl, getCurrent } = await import('@tauri-apps/plugin-deep-link');
        // Any URL the app was cold-launched with.
        handle(await getCurrent());
        // Subsequent URLs (forwarded by single-instance, or OS open-url).
        return await onOpenUrl(handle);
    } catch (e) {
        console.warn('[deeplink] init failed', e);
        return null;
    }
}
