/**
 * Travel-aware timezone store.
 *
 * Compares the device's current zone against the server's *home* zone (carried
 * on the synced notification settings) and decides whether to prompt. Because
 * tasks float and the day boundary already follows the device, the choices are:
 *
 *   - Update home   → re-anchor the server (recurrence poller + morning digest)
 *                     to the device zone. Persisted via the settings document.
 *   - Just the trip → stop prompting for this zone; auto-clears the moment the
 *                     device returns home. Nothing server-side changes.
 *
 * The acknowledgement is keyed by the device zone, so arriving in a *new* zone
 * prompts again, and flying home silently resets.
 */

import { notificationSettings } from './notificationSettings.svelte';
import { deviceTimeZone, zoneLabel, offsetLabel } from '$lib/timezone';

const ACK_KEY = 'sempa.tzTripAck'; // the device zone the user accepted "for this trip"

function createTimezoneStore() {
  let device = $state('UTC');
  let ack = $state<string | null>(null);
  let started = false;

  function readAck() {
    try {
      return localStorage.getItem(ACK_KEY);
    } catch {
      return null;
    }
  }

  const home = $derived(notificationSettings.settings.timezone ?? '');

  // Prompt when the device differs from home and the user hasn't already waved
  // off this particular zone for the trip.
  const mismatch = $derived(
    notificationSettings.loaded && !!home && !!device && device !== home && ack !== device,
  );

  /** Re-read device zone + ack. Safe to call repeatedly (e.g. on window focus). */
  function refresh() {
    device = deviceTimeZone();
    ack = readAck();
    // Returned home — drop any stale trip acknowledgement so a future trip prompts.
    if (home && device === home && ack) {
      ack = null;
      try {
        localStorage.removeItem(ACK_KEY);
      } catch {
        /* ignore */
      }
    }
  }

  function init() {
    if (started) {
      refresh();
      return;
    }
    started = true;
    refresh();
    if (typeof window !== 'undefined') {
      window.addEventListener('focus', refresh);
      document.addEventListener('visibilitychange', () => {
        if (!document.hidden) refresh();
      });
    }
  }

  /** "Just for this trip" — dismiss for the current zone without touching home. */
  function keepForTrip() {
    ack = device;
    try {
      localStorage.setItem(ACK_KEY, device);
    } catch {
      /* ignore */
    }
  }

  /** "Update home" — persist the device zone as the server's home timezone. */
  async function updateHome() {
    await notificationSettings.save({ ...notificationSettings.settings, timezone: device });
    ack = null;
    try {
      localStorage.removeItem(ACK_KEY);
    } catch {
      /* ignore */
    }
  }

  return {
    get device() {
      return device;
    },
    get deviceLabel() {
      return zoneLabel(device);
    },
    get deviceOffset() {
      return offsetLabel(device);
    },
    get home() {
      return home;
    },
    get homeLabel() {
      return zoneLabel(home);
    },
    get mismatch() {
      return mismatch;
    },
    init,
    refresh,
    keepForTrip,
    updateHome,
  };
}

export const timezone = createTimezoneStore();
