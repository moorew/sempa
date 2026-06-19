/**
 * Window chrome (Linux native feel): the custom titlebar mirrors the system
 * window-button layout, and the user can hand the titlebar back to the WM.
 *
 *  • controlsSide / controlsOrder — parsed from GTK's gtk-decoration-layout so
 *    minimise/maximise/close sit where (and in the order) the desktop puts them
 *    (GNOME on the right, some setups on the left, etc.).
 *  • useSystemTitlebar — when on, we re-enable native (server-side) decorations
 *    and render no custom bar, for users who prefer maximally-native windows.
 *
 * Tauri-only; inert in the browser/Capacitor.
 */
import { isTauri } from '$lib/platform';
import { getDecorationLayout } from '$lib/tauri/bridge';

export type ControlKind = 'minimize' | 'maximize' | 'close';
export type ControlsSide = 'left' | 'right';

const SYS_TITLEBAR_KEY = 'sempa-system-titlebar';
const CONTROLS: ControlKind[] = ['minimize', 'maximize', 'close'];

/**
 * Parse a GTK decoration-layout string into the side + ordered controls.
 * Format: "<left>:<right>" where each side is a comma list of button names
 * (menu/appmenu/icon/spacer/minimize/maximize/close). We keep only the real
 * window controls, and treat the side that holds them as their placement.
 * Pure — unit-tested in windowChrome.test.ts.
 */
export function parseDecorationLayout(
    layout: string | null | undefined,
): { side: ControlsSide; order: ControlKind[] } {
    const fallback = { side: 'right' as ControlsSide, order: [...CONTROLS] };
    if (!layout) return fallback;
    const [left = '', right = ''] = layout.split(':');
    const pick = (s: string): ControlKind[] =>
        s
            .split(',')
            .map((x) => x.trim())
            .filter((x): x is ControlKind => (CONTROLS as string[]).includes(x));
    const rightControls = pick(right);
    if (rightControls.length) return { side: 'right', order: rightControls };
    const leftControls = pick(left);
    if (leftControls.length) return { side: 'left', order: leftControls };
    return fallback;
}

function createWindowChromeStore() {
    let useSystemTitlebar = $state(false);
    let controlsSide = $state<ControlsSide>('right');
    let controlsOrder = $state<ControlKind[]>([...CONTROLS]);

    async function setDecorations(on: boolean) {
        try {
            const { getCurrentWindow } = await import('@tauri-apps/api/window');
            await getCurrentWindow().setDecorations(on);
        } catch {
            /* not in Tauri / API unavailable */
        }
    }

    async function init() {
        if (!isTauri() || typeof localStorage === 'undefined') return;

        useSystemTitlebar = localStorage.getItem(SYS_TITLEBAR_KEY) === '1';
        // Reconcile the actual window decorations with the saved preference: the
        // window ships decorations:false, so only act when the user opted in.
        if (useSystemTitlebar) await setDecorations(true);

        try {
            const { side, order } = parseDecorationLayout(await getDecorationLayout());
            controlsSide = side;
            controlsOrder = order;
        } catch {
            /* keep defaults */
        }
    }

    async function setUseSystemTitlebar(on: boolean) {
        useSystemTitlebar = on;
        localStorage.setItem(SYS_TITLEBAR_KEY, on ? '1' : '0');
        await setDecorations(on);
    }

    return {
        get useSystemTitlebar() { return useSystemTitlebar; },
        get controlsSide() { return controlsSide; },
        get controlsOrder() { return controlsOrder; },
        init,
        setUseSystemTitlebar,
    };
}

export const windowChrome = createWindowChromeStore();
