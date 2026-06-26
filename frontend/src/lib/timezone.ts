/**
 * Timezone helpers.
 *
 * Sempa tasks use *floating* time: "8am" means 8am wherever you are, and a
 * task's date/time is never rewritten when you travel (RFC 5545 floating time,
 * matching every major task app). The only thing a timezone governs is the day
 * boundary — when "today" rolls over — which always follows the *device* (see
 * utils.today(), which reads new Date() in the device zone).
 *
 * The server has a fixed *home* timezone (SEMPA_TIMEZONE / the notifications
 * setting) that anchors its background work (recurrence horizon, morning
 * digest). When the device zone drifts away from home — i.e. you've travelled —
 * we surface a calm prompt to either make the new zone home or keep it just for
 * the trip. Nothing about the tasks themselves changes either way.
 */

/** The device's current IANA timezone, e.g. "America/Vancouver". */
export function deviceTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}

/** A short human label for a zone, e.g. "Vancouver" from "America/Vancouver". */
export function zoneLabel(tz: string): string {
  if (!tz) return '';
  const part = tz.split('/').pop() ?? tz;
  return part.replace(/_/g, ' ');
}

/** The device's current UTC offset as a compact label, e.g. "UTC−7". */
export function offsetLabel(tz: string = deviceTimeZone()): string {
  try {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone: tz,
      timeZoneName: 'shortOffset',
    }).formatToParts(new Date());
    const name = parts.find((p) => p.type === 'timeZoneName')?.value ?? '';
    return name.replace('GMT', 'UTC').replace('-', '−');
  } catch {
    return '';
  }
}
