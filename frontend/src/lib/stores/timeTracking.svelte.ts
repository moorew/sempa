// Time-tracking preferences (client-only, localStorage). Kept separate from the
// general prefs store so the feature owns its own settings surface.
import { bucketByKey } from '$lib/activityBuckets';

const PROMPT_KEY = 'sempa.tt.promptOnComplete';
const SKIPQUICK_KEY = 'sempa.tt.skipQuick';
const BUCKET_KEY = 'sempa.tt.bucketMinutes';
const WALKTHROUGH_KEY = 'sempa.tt.walkthroughSeen';
const CAPACITY_ON_KEY = 'sempa.tt.capacityEnabled';
const CAPACITY_MIN_KEY = 'sempa.tt.capacityMinutes';
const CAPACITY_REAL_KEY = 'sempa.tt.capacityRealistic';

const DEFAULT_CAPACITY_MINUTES = 360; // 6h of focused work — a humane default

function createTimeTrackingStore() {
  // Ask "how long did that take?" when a task is completed without tracked time.
  let promptOnComplete = $state(true);
  // Skip the prompt for trivially-quick tasks (estimate ≤ 5 min).
  let skipQuick = $state(true);
  // Per-bucket default-minute overrides (key → minutes). Empty = use baseline.
  let bucketMinutes = $state<Record<string, number>>({});
  // First-run walkthrough seen flag, and live open state (so settings can replay).
  let walkthroughSeen = $state(false);
  let walkthroughOpen = $state(false);
  // Set once init() has read localStorage, so consumers (e.g. the first-run
  // walkthrough) don't act on default values during the mount-order race.
  let initialized = $state(false);
  // Day-capacity indicator: warn (subtly) when a day's planned time runs over.
  let capacityEnabled = $state(true);
  let capacityMinutes = $state(DEFAULT_CAPACITY_MINUTES);
  // Compare against estimates × your history multiplier ("realistically ~7h"),
  // not raw estimates. Safe to default on: with no data the multiplier is 1×.
  let capacityRealistic = $state(true);

  function init() {
    if (typeof localStorage === 'undefined') return;
    const p = localStorage.getItem(PROMPT_KEY);
    if (p !== null) promptOnComplete = p === '1';
    const sq = localStorage.getItem(SKIPQUICK_KEY);
    if (sq !== null) skipQuick = sq === '1';
    const w = localStorage.getItem(WALKTHROUGH_KEY);
    if (w !== null) walkthroughSeen = w === '1';
    const ce = localStorage.getItem(CAPACITY_ON_KEY);
    if (ce !== null) capacityEnabled = ce === '1';
    const cm = localStorage.getItem(CAPACITY_MIN_KEY);
    if (cm !== null) { const n = parseInt(cm, 10); if (n > 0) capacityMinutes = n; }
    const cr = localStorage.getItem(CAPACITY_REAL_KEY);
    if (cr !== null) capacityRealistic = cr === '1';
    try {
      const raw = localStorage.getItem(BUCKET_KEY);
      if (raw) bucketMinutes = JSON.parse(raw);
    } catch { /* keep defaults */ }
    initialized = true;
  }

  function setPromptOnComplete(on: boolean) {
    promptOnComplete = on;
    if (typeof localStorage !== 'undefined') localStorage.setItem(PROMPT_KEY, on ? '1' : '0');
  }
  function setSkipQuick(on: boolean) {
    skipQuick = on;
    if (typeof localStorage !== 'undefined') localStorage.setItem(SKIPQUICK_KEY, on ? '1' : '0');
  }
  function setBucketMinutes(key: string, minutes: number) {
    bucketMinutes = { ...bucketMinutes, [key]: minutes };
    if (typeof localStorage !== 'undefined') localStorage.setItem(BUCKET_KEY, JSON.stringify(bucketMinutes));
  }
  function markWalkthroughSeen() {
    walkthroughSeen = true;
    if (typeof localStorage !== 'undefined') localStorage.setItem(WALKTHROUGH_KEY, '1');
  }
  function setCapacityEnabled(on: boolean) {
    capacityEnabled = on;
    if (typeof localStorage !== 'undefined') localStorage.setItem(CAPACITY_ON_KEY, on ? '1' : '0');
  }
  function setCapacityMinutes(minutes: number) {
    capacityMinutes = Math.max(30, Math.min(1440, minutes));
    if (typeof localStorage !== 'undefined') localStorage.setItem(CAPACITY_MIN_KEY, String(capacityMinutes));
  }
  function setCapacityRealistic(on: boolean) {
    capacityRealistic = on;
    if (typeof localStorage !== 'undefined') localStorage.setItem(CAPACITY_REAL_KEY, on ? '1' : '0');
  }
  function openWalkthrough() { walkthroughOpen = true; }
  function dismissWalkthrough() {
    walkthroughOpen = false;
    markWalkthroughSeen();
  }

  /** Default minutes for a bucket: the user override if set, else the baseline. */
  function defaultMinutesFor(key: string): number {
    return bucketMinutes[key] ?? bucketByKey(key).defaultMinutes;
  }

  return {
    get promptOnComplete() { return promptOnComplete; },
    get skipQuick() { return skipQuick; },
    get bucketMinutes() { return bucketMinutes; },
    get initialized() { return initialized; },
    get walkthroughSeen() { return walkthroughSeen; },
    get walkthroughOpen() { return walkthroughOpen; },
    get capacityEnabled() { return capacityEnabled; },
    get capacityMinutes() { return capacityMinutes; },
    get capacityRealistic() { return capacityRealistic; },
    init,
    setCapacityEnabled,
    toggleCapacityEnabled: () => setCapacityEnabled(!capacityEnabled),
    setCapacityMinutes,
    setCapacityRealistic,
    toggleCapacityRealistic: () => setCapacityRealistic(!capacityRealistic),
    openWalkthrough,
    dismissWalkthrough,
    setPromptOnComplete,
    togglePromptOnComplete: () => setPromptOnComplete(!promptOnComplete),
    setSkipQuick,
    toggleSkipQuick: () => setSkipQuick(!skipQuick),
    setBucketMinutes,
    markWalkthroughSeen,
    defaultMinutesFor,
  };
}

export const timeTracking = createTimeTrackingStore();
