/**
 * Unit coverage for parseDecorationLayout — maps GTK's gtk-decoration-layout to
 * the titlebar's control side + order (Phase 2 native feel).
 */
import { describe, expect, it } from 'vitest';
import { parseDecorationLayout } from './windowChrome.svelte';

describe('parseDecorationLayout', () => {
    it('defaults to right-side min/max/close when unset', () => {
        expect(parseDecorationLayout(null)).toEqual({ side: 'right', order: ['minimize', 'maximize', 'close'] });
        expect(parseDecorationLayout('')).toEqual({ side: 'right', order: ['minimize', 'maximize', 'close'] });
    });

    it('reads a GNOME-style right layout, ignoring appmenu/icon/spacer', () => {
        expect(parseDecorationLayout('appmenu:minimize,maximize,close')).toEqual({
            side: 'right',
            order: ['minimize', 'maximize', 'close'],
        });
        expect(parseDecorationLayout('icon:close')).toEqual({ side: 'right', order: ['close'] });
    });

    it('reads a left-side layout (controls before the colon)', () => {
        expect(parseDecorationLayout('close,minimize,maximize:')).toEqual({
            side: 'left',
            order: ['close', 'minimize', 'maximize'],
        });
    });

    it('preserves the system order', () => {
        expect(parseDecorationLayout(':close,minimize,maximize').order).toEqual([
            'close',
            'minimize',
            'maximize',
        ]);
    });

    it('falls back when no real controls are present', () => {
        expect(parseDecorationLayout('appmenu:spacer')).toEqual({
            side: 'right',
            order: ['minimize', 'maximize', 'close'],
        });
    });
});
