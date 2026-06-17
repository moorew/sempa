// Thin, SSR-safe wrapper over the framework-agnostic celebration engine
// (celebrations-engine.js, which attaches window.SempaCelebrate). The engine
// touches `window` at module load, so it must only ever run in the browser —
// we lazy-load it via dynamic import and queue any calls made before it lands.
//
//   celebrate.task(originEl)        tier 1 · tiny, local to a card
//   celebrate.day(originEl, copy)   tier 2 · a quiet day moment
//   celebrate.week(copy)            tier 3 · the full cradle-mark moment
//
// All tiers are reduced-motion aware inside the engine. Sound defaults off and
// is wired to a Settings preference via setSound().

import { browser } from '$app/environment';

interface Engine {
  task(el?: Element | null): void;
  day(el?: Element | null, copy?: string): void;
  week(copy?: string): void;
  setSound(on: boolean): void;
  setRoot(el: Element | null): void;
}

let engine: Engine | null = null;
let loading: Promise<Engine | null> | null = null;
let pendingSound: boolean | null = null;

function load(): Promise<Engine | null> {
  if (!browser) return Promise.resolve(null);
  if (engine) return Promise.resolve(engine);
  if (!loading) {
    loading = import('./celebrations-engine.js')
      .then(() => {
        engine = (window as unknown as { SempaCelebrate?: Engine }).SempaCelebrate ?? null;
        if (engine && pendingSound !== null) {
          engine.setSound(pendingSound);
          pendingSound = null;
        }
        return engine;
      })
      .catch(() => null);
  }
  return loading;
}

// Run `fn` against the engine, loading it first if needed. Fire-and-forget: the
// first call pays a one-off import cost, later calls run synchronously.
function run(fn: (e: Engine) => void): void {
  if (!browser) return;
  if (engine) { fn(engine); return; }
  void load().then((e) => { if (e) fn(e); });
}

export const celebrate = {
  /** Warm the engine ahead of the first celebration (call once on app mount). */
  preload(): void { void load(); },
  task(el?: Element | null): void { run((e) => e.task(el ?? null)); },
  day(el?: Element | null, copy?: string): void { run((e) => e.day(el ?? null, copy)); },
  week(copy?: string): void { run((e) => e.week(copy)); },
  /** Confine celebrations to a container (mobile phone screen); null resets. */
  setRoot(el: Element | null): void { run((e) => e.setRoot(el)); },
  setSound(on: boolean): void {
    if (engine) engine.setSound(on);
    else { pendingSound = on; void load(); }
  },
};
