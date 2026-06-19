/**
 * Unit coverage for the sempa:// deep-link router (Phase 1).
 *
 * routeForDeepLink is pure, so it pins the .desktop launcher actions and the
 * OAuth-redirect carve-out without needing a Tauri runtime.
 */
import { describe, expect, it } from 'vitest';
import { routeForDeepLink } from './deeplink';
import { today } from '$lib/utils';

describe('routeForDeepLink', () => {
    const d = today();

    it('routes the New task action to today with the composer open', () => {
        expect(routeForDeepLink('sempa://new')).toBe(`/day/${d}?new=1`);
        expect(routeForDeepLink('sempa://new-task')).toBe(`/day/${d}?new=1`);
    });

    it('routes Plan day and Shutdown ritual to today', () => {
        expect(routeForDeepLink('sempa://plan')).toBe(`/plan/${d}`);
        expect(routeForDeepLink('sempa://plan-day')).toBe(`/plan/${d}`);
        expect(routeForDeepLink('sempa://shutdown')).toBe(`/shutdown/${d}`);
    });

    it('is case-insensitive on the action', () => {
        expect(routeForDeepLink('sempa://PLAN')).toBe(`/plan/${d}`);
    });

    it('swallows the OAuth redirect (handled in Phase 3, not bounced home)', () => {
        expect(routeForDeepLink('sempa://oauth/callback?code=abc')).toBeNull();
    });

    it('falls back to home for unknown actions', () => {
        expect(routeForDeepLink('sempa://wat')).toBe('/home');
    });

    it('ignores non-sempa schemes and malformed input', () => {
        expect(routeForDeepLink('https://example.com')).toBeNull();
        expect(routeForDeepLink('not a url')).toBeNull();
    });
});
