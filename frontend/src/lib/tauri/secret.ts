/**
 * Desktop bearer-token storage backed by the OS secret store (Secret Service /
 * libsecret on Linux, via the secret_* Tauri commands).
 *
 * The token used to live in plaintext localStorage. These helpers move it into
 * the keyring, but keep localStorage as a *fallback only* so a host without a
 * Secret Service daemon (or a transient keyring error) can never lock the user
 * out — the invariant is "keyring when available, never plaintext when it is".
 *
 * api.ts owns the synchronous in-memory cache that the request hot-path reads;
 * this module does the async keyring I/O behind it.
 */
import { secretGet, secretSet, secretDelete } from '$lib/tauri/bridge';

// Legacy plaintext key — also the fallback key when the keyring is unavailable.
const LS_KEY = 'sempa_tauri_token';

function ls(): Storage | null {
    return typeof localStorage !== 'undefined' ? localStorage : null;
}

/**
 * Resolve the token at startup: prefer the keyring; migrate a legacy plaintext
 * token into it (then delete the plaintext). Throws only if the keyring call
 * itself throws — api.ts catches and falls back to the plaintext value.
 */
export async function loadKeyringToken(key: string): Promise<string> {
    const fromRing = await secretGet(key);
    if (fromRing) {
        ls()?.removeItem(LS_KEY); // keyring is source of truth; drop any stale copy
        return fromRing;
    }
    // Migrate a pre-keyring plaintext token, if present.
    const legacy = ls()?.getItem(LS_KEY) ?? null;
    if (legacy) {
        try {
            await secretSet(key, legacy);
            ls()?.removeItem(LS_KEY);
        } catch {
            /* keyring write failed — keep the plaintext copy as fallback */
        }
        return legacy;
    }
    return '';
}

/** Persist the token to the keyring; only drop the plaintext copy on success. */
export async function saveKeyringToken(key: string, token: string): Promise<void> {
    try {
        await secretSet(key, token);
        ls()?.removeItem(LS_KEY);
    } catch {
        // Keyring unavailable → fall back to plaintext so login survives a reload.
        ls()?.setItem(LS_KEY, token);
    }
}

/** Remove the token from both the keyring and any plaintext fallback. */
export async function deleteKeyringToken(key: string): Promise<void> {
    try {
        await secretDelete(key);
    } catch {
        /* ignore */
    }
    ls()?.removeItem(LS_KEY);
}
