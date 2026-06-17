// @ts-nocheck
/* ───────────────────────────────────────────────────────────────────────────
 * sempa · celebration language
 *
 * One warm visual DNA — BLOOM + rising EMBERS + RIPPLE — scaled across three
 * tiers. The cradle mark is reserved for the weekly moment. Calm by default:
 * slow, gravity-light, ease-out; never confetti, never a hard burst, never an
 * infinite loop. All motion gated by prefers-reduced-motion.
 *
 *   SempaCelebrate.task(originEl)        tier 1 · tiny, local to a card
 *   SempaCelebrate.day(originEl, copy)   tier 2 · medium, a quiet day moment
 *   SempaCelebrate.week(copy)            tier 3 · full, the cradle mark blooms
 *
 * Reads colours from CSS custom properties on :root so it follows the theme.
 * ───────────────────────────────────────────────────────────────────────── */
(function () {
  const REDUCED = window.matchMedia('(prefers-reduced-motion: reduce)');
  let soundOn = false; // toggled by the host

  /* ── overlay layers ──────────────────────────────────────────────────── */
  let canvas, ctx, fx, dpr = window.devicePixelRatio || 1;
  let root = null, W = window.innerWidth, H = window.innerHeight;
  function dims() {
    if (root) return { w: root.clientWidth, h: root.clientHeight };
    return { w: window.innerWidth, h: window.innerHeight };
  }
  // Mount celebrations inside a specific element (e.g. a phone screen). Pass
  // null to return to full-window. The element is made a positioning context.
  function setRoot(el) {
    if (canvas) { canvas.remove(); fx.remove(); canvas = fx = ctx = null; }
    root = el || null;
    if (root && getComputedStyle(root).position === 'static') root.style.position = 'relative';
  }

  function ensureLayers() {
    if (canvas) return;
    const parent = root || document.body;
    const pos = root ? 'absolute' : 'fixed';
    canvas = document.createElement('canvas');
    canvas.className = 'sc-canvas'; canvas.style.position = pos;
    fx = document.createElement('div');
    fx.className = 'sc-fx'; fx.style.position = pos;
    parent.appendChild(canvas);
    parent.appendChild(fx);
    ctx = canvas.getContext('2d');
    resize();
    window.addEventListener('resize', resize);
  }
  function resize() {
    if (!canvas) return;
    const d = dims(); W = d.w; H = d.h;
    dpr = window.devicePixelRatio || 1;
    canvas.width = W * dpr;
    canvas.height = H * dpr;
    canvas.style.width = W + 'px';
    canvas.style.height = H + 'px';
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  function tok(name, fallback) {
    const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return v || fallback;
  }
  // warm ember palette pulled from the live theme
  function palette() {
    return [
      tok('--sempa-accent', '#cc6e3a'),
      tok('--sempa-amber', '#da9f62'),
      mix(tok('--sempa-amber', '#da9f62'), '#fbeede', 0.5),
    ];
  }
  function mix(a, b, t) {
    const pa = hex(a), pb = hex(b);
    const r = Math.round(pa[0] + (pb[0] - pa[0]) * t);
    const g = Math.round(pa[1] + (pb[1] - pa[1]) * t);
    const bl = Math.round(pa[2] + (pb[2] - pa[2]) * t);
    return `rgb(${r},${g},${bl})`;
  }
  function hex(c) {
    c = c.replace('#', '');
    if (c.length === 3) c = c.split('').map(x => x + x).join('');
    return [parseInt(c.slice(0, 2), 16), parseInt(c.slice(2, 4), 16), parseInt(c.slice(4, 6), 16)];
  }

  function center(el) {
    if (!el) return { x: W / 2, y: H / 2 };
    const r = el.getBoundingClientRect();
    if (!root) return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    // scale-invariant: derive the live CSS scale from the root's rendered size
    const rr = root.getBoundingClientRect();
    const sx = rr.width / root.clientWidth || 1;
    const sy = rr.height / root.clientHeight || 1;
    return { x: (r.left + r.width / 2 - rr.left) / sx, y: (r.top + r.height / 2 - rr.top) / sy };
  }

  /* ── ember field (canvas) ─────────────────────────────────────────────── */
  let embers = [];
  let raf = null;

  function spawnEmbers(opts) {
    const { x, y, count, spread, rise, life, sizeMax, spawnMs } = opts;
    const cols = palette();
    const start = performance.now();
    let made = 0;
    const per = spawnMs / count;
    function emit(now) {
      const due = Math.min(count, Math.floor((now - start) / per) + 1);
      while (made < due) {
        const a = (Math.random() - 0.5);
        embers.push({
          x: x + (Math.random() - 0.5) * spread,
          y: y + (Math.random() - 0.5) * spread * 0.35,
          vx: a * 0.35,
          vy: -(0.30 + Math.random() * rise),
          r: 1.4 + Math.random() * (sizeMax - 1.4),
          c: cols[(Math.random() * cols.length) | 0],
          born: now,
          life: life * (0.8 + Math.random() * 0.4),
          sway: 0.4 + Math.random() * 0.9,
          phase: Math.random() * Math.PI * 2,
        });
        made++;
      }
      if (made < count) requestAnimationFrame(emit);
    }
    requestAnimationFrame(emit);
    loop();
  }

  function loop() {
    if (raf) return;
    const step = (now) => {
      ctx.clearRect(0, 0, W, H);
      embers = embers.filter(e => now - e.born < e.life);
      for (const e of embers) {
        const t = (now - e.born) / e.life;        // 0..1
        e.vy *= 0.992;                              // gentle deceleration — floaty
        e.x += e.vx + Math.sin(now / 600 + e.phase) * e.sway * 0.18;
        e.y += e.vy;
        // opacity: quick ease-in, long ease-out
        const op = t < 0.12 ? t / 0.12 : 1 - ((t - 0.12) / 0.88);
        const rr = e.r * (1 - t * 0.45);
        const grad = ctx.createRadialGradient(e.x, e.y, 0, e.x, e.y, rr * 3.2);
        grad.addColorStop(0, e.c);
        grad.addColorStop(1, 'rgba(0,0,0,0)');
        ctx.globalAlpha = Math.max(0, op) * 0.9;
        ctx.fillStyle = grad;
        ctx.beginPath();
        ctx.arc(e.x, e.y, rr * 3.2, 0, Math.PI * 2);
        ctx.fill();
        // bright core
        ctx.globalAlpha = Math.max(0, op);
        ctx.fillStyle = e.c;
        ctx.beginPath();
        ctx.arc(e.x, e.y, rr, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.globalAlpha = 1;
      if (embers.length) { raf = requestAnimationFrame(step); }
      else { raf = null; ctx.clearRect(0, 0, W, H); }
    };
    raf = requestAnimationFrame(step);
  }

  /* ── ripple + bloom (DOM) ─────────────────────────────────────────────── */
  function ripple(x, y, size, ms) {
    const el = document.createElement('div');
    el.className = 'sc-ripple';
    el.style.left = x + 'px';
    el.style.top = y + 'px';
    el.style.width = el.style.height = size + 'px';
    el.style.setProperty('--sc-dur', ms + 'ms');
    el.style.borderColor = tok('--sempa-accent', '#cc6e3a');
    fx.appendChild(el);
    setTimeout(() => el.remove(), ms + 60);
  }
  function bloom(x, y, size, ms) {
    const el = document.createElement('div');
    el.className = 'sc-bloom';
    el.style.left = x + 'px';
    el.style.top = y + 'px';
    el.style.width = el.style.height = size + 'px';
    el.style.setProperty('--sc-dur', ms + 'ms');
    const a = tok('--sempa-accent', '#cc6e3a');
    el.style.background = `radial-gradient(circle, ${withAlpha(a, 0.5)} 0%, ${withAlpha(a, 0.18)} 35%, rgba(0,0,0,0) 70%)`;
    fx.appendChild(el);
    setTimeout(() => el.remove(), ms + 60);
  }
  function withAlpha(c, a) {
    const p = hex(c);
    return `rgba(${p[0]},${p[1]},${p[2]},${a})`;
  }

  function toast(text, x, y, ms) {
    const el = document.createElement('div');
    el.className = 'sc-toast';
    el.textContent = text;
    if (x != null) { el.style.left = x + 'px'; el.style.top = y + 'px'; el.classList.add('sc-toast--anchored'); }
    el.style.setProperty('--sc-dur', ms + 'ms');
    fx.appendChild(el);
    setTimeout(() => el.remove(), ms + 120);
  }

  /* ── sound (soft, optional) ──────────────────────────────────────────── */
  let actx = null;
  function chime(notes, gain) {
    if (!soundOn) return;
    try {
      actx = actx || new (window.AudioContext || window.webkitAudioContext)();
      const t0 = actx.currentTime;
      notes.forEach((f, i) => {
        const o = actx.createOscillator();
        const g = actx.createGain();
        o.type = 'sine';
        o.frequency.value = f;
        const start = t0 + i * 0.10;
        g.gain.setValueAtTime(0, start);
        g.gain.linearRampToValueAtTime(gain, start + 0.04);
        g.gain.exponentialRampToValueAtTime(0.0001, start + 1.2);
        o.connect(g); g.connect(actx.destination);
        o.start(start); o.stop(start + 1.3);
      });
    } catch (e) { /* no-op */ }
  }

  /* ── haptics (mobile) ────────────────────────────────────────────────── */
  function haptic(pattern) {
    if (navigator.vibrate) { try { navigator.vibrate(pattern); } catch (e) {} }
  }

  /* ── reduced-motion fallback ─────────────────────────────────────────── */
  function quietGlow(x, y, size) {
    bloom(x, y, size, 520);
  }

  /* ── TIER 1 · task ───────────────────────────────────────────────────── */
  function task(originEl) {
    ensureLayers();
    const { x, y } = center(originEl);
    haptic(12);
    chime([523.25, 659.25], 0.04); // soft C–E
    if (REDUCED.matches) { quietGlow(x, y, 90); return; }
    bloom(x, y, 120, 420);
    ripple(x, y, 26, 560);
    spawnEmbers({ x, y, count: 5, spread: 16, rise: 0.5, life: 1500, sizeMax: 3, spawnMs: 220 });
  }

  /* ── TIER 2 · day ────────────────────────────────────────────────────── */
  function day(originEl, copy) {
    ensureLayers();
    const { x, y } = center(originEl);
    haptic([0, 14, 60, 22]);
    chime([523.25, 659.25, 783.99], 0.05); // C–E–G
    if (REDUCED.matches) { quietGlow(x, y, 200); if (copy) toast(copy, x, y - 40, 2400); return; }
    bloom(x, y, 320, 700);
    ripple(x, y, 60, 900);
    setTimeout(() => ripple(x, y, 60, 900), 180);
    spawnEmbers({ x, y: y + 10, count: 18, spread: 120, rise: 0.7, life: 1900, sizeMax: 3.6, spawnMs: 760 });
    if (copy) toast(copy, x, y - 46, 2600);
  }

  /* ── TIER 3 · week (the cradle mark) ─────────────────────────────────── */
  function week(copy) {
    ensureLayers();
    haptic([0, 18, 70, 26, 70, 34]);
    chime([392.0, 523.25, 659.25, 783.99], 0.06); // G–C–E–G, rising
    const cx = W / 2, cy = H * 0.42;

    // dim veil + centred mark
    const veil = document.createElement('div');
    veil.className = 'sc-veil';
    if (root) veil.style.position = 'absolute';
    veil.innerHTML = markSVG() +
      (copy ? `<div class="sc-week-copy">${copy}</div>` : '');
    fx.appendChild(veil);
    requestAnimationFrame(() => veil.classList.add('sc-veil--in'));

    if (REDUCED.matches) {
      quietGlow(cx, cy, 360);
      setTimeout(() => { veil.classList.remove('sc-veil--in'); }, 2400);
      setTimeout(() => veil.remove(), 2900);
      return;
    }

    bloom(cx, cy, 520, 1400);
    setTimeout(() => ripple(cx, cy, 120, 1400), 350);
    setTimeout(() => ripple(cx, cy, 120, 1500), 650);
    // a generous, slow field rising across the lower screen
    spawnEmbers({ x: W / 2, y: H * 0.82, count: 46, spread: W * 0.86, rise: 0.85, life: 2600, sizeMax: 4.2, spawnMs: 1500 });

    setTimeout(() => { veil.classList.remove('sc-veil--in'); }, 2700);
    setTimeout(() => veil.remove(), 3300);
  }

  function markSVG() {
    const a = tok('--sempa-accent', '#cc6e3a');
    return `
    <svg class="sc-mark" width="92" height="92" viewBox="0 0 100 100" fill="none" aria-hidden="true">
      <path class="sc-arc" d="M22,40 a28,28 0 0 0 56,0" stroke="${a}" stroke-width="9"
            stroke-linecap="round" stroke-linejoin="round" stroke-dasharray="88" stroke-dashoffset="88"/>
      <circle class="sc-dot" cx="50" cy="35" r="7.5" fill="${a}"/>
    </svg>`;
  }

  window.SempaCelebrate = {
    task, day, week, setRoot,
    setSound(v) { soundOn = !!v; },
    get soundOn() { return soundOn; },
  };
})();

// Side-effecting module: the IIFE above attaches window.SempaCelebrate. This
// empty export marks the file as a module so it can be dynamically imported.
export {};
