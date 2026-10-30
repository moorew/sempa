<script lang="ts">
  /**
   * The Sempa Dock UI (PI_DOCK_SPEC + companion PiCompanion study). A calm,
   * paper-like, touch-first "today" appliance: tasks strung on the day-thread
   * spine, big targets for add / check, an on-screen keyboard, and an ambient
   * idle face. One frontend with the rest of Sempa — the Dock differs by layout.
   * Responsive across 7″ (≥1100) and 5″ panels. Defaults to the light (paper)
   * theme on the appliance.
   *
   * Standalone route (no app chrome). On the appliance the binary launches in
   * dock mode (fullscreen, cursor hidden) and lands here. Reorder lives in the
   * full app; the Dock does add + check (the core kiosk gestures).
   */
  import { onMount } from 'svelte';
  import { api, getTauriToken, setTauriToken, initTauriToken } from '$lib/api';
  import { today } from '$lib/utils';
  import { realtime } from '$lib/stores/realtime.svelte';
  import type { Task } from '$lib/types';

  const date = today();
  let tasks = $state<Task[]>([]);
  let intention = $state('');
  let clock = $state(timeLabel());
  let dateLabel = $state(longDate());
  let w = $state(typeof window !== 'undefined' ? window.innerWidth : 1280);

  // Responsive config — 7″ (≥1100) vs 5″. Big touch targets either way.
  let c = $derived(
    w >= 1100
      ? { pad: 34, gap: 16, title: 38, sub: 15, rowH: 78, check: 52, titleSz: 23, node: 9, line: 3, kbKey: 56, kbFont: 22, showMeta: true, addFont: 20, headGap: 18, card: 28, inset: 12, ring: 36 }
      : { pad: 20, gap: 10, title: 26, sub: 13.5, rowH: 60, check: 42, titleSz: 19, node: 8, line: 2.5, kbKey: 40, kbFont: 16, showMeta: false, addFont: 17, headGap: 12, card: 22, inset: 8, ring: 30 },
  );

  const isDone = (t: Task) => t.status === 'done';
  const isLive = (t: Task) => t.status !== 'done' && t.status !== 'cancelled';
  let active = $derived(tasks.filter(isLive).sort((a, b) => a.position - b.position));
  let done = $derived(tasks.filter(isDone));
  let total = $derived(active.length + done.length);
  let pct = $derived(total ? Math.round((done.length / total) * 100) : 0);
  let nowId = $derived(active[0]?.id ?? null);

  let doneOpen = $state(false);
  let adding = $state(false);
  let draft = $state('');

  // ── Pairing gate ──────────────────────────────────────────────────────────
  // On the appliance the device has no token until paired: show a code, the user
  // approves it from a signed-in app, then we receive + store the device token.
  let needsPairing = $state(false);
  let pairCode = $state('');
  let pairPoll: ReturnType<typeof setInterval> | null = null;

  async function startPairing() {
    needsPairing = true;
    try {
      const { code } = await api.devices.pairStart('Sempa Dock', 'dock');
      pairCode = code;
    } catch { setTimeout(startPairing, 4000); return; }
    if (pairPoll) clearInterval(pairPoll);
    pairPoll = setInterval(async () => {
      try {
        const s = await api.devices.pairStatus(pairCode);
        if (s.status === 'approved' && s.token) {
          if (pairPoll) clearInterval(pairPoll);
          setTauriToken(s.token);
          needsPairing = false;
          await load();
        } else if (s.status === 'expired' || s.status === 'unknown') {
          if (pairPoll) clearInterval(pairPoll);
          void startPairing(); // fresh code
        }
      } catch { /* keep polling */ }
    }, 2500);
  }

  async function load() {
    try {
      const [ts, plan] = await Promise.all([
        api.tasks.listByDate(date),
        api.plans.get(date).catch(() => null),
      ]);
      tasks = ts;
      intention = plan?.intention ?? '';
    } catch { /* offline — keep prior (offline queue resyncs) */ }
  }
  async function toggle(t: Task) {
    const wantDone = !isDone(t);
    tasks = tasks.map((x) => (x.id === t.id ? { ...x, status: wantDone ? 'done' : 'planned' } : x));
    try { await api.tasks.update(t.id, { status: wantDone ? 'done' : 'planned' }); }
    catch { void load(); }
  }
  async function commitAdd() {
    const title = draft.trim();
    draft = ''; adding = false;
    if (!title) return;
    try { await api.tasks.create({ title, planned_date: date, status: 'planned' }); await load(); }
    catch { /* offline */ }
  }

  function timeLabel() {
    const d = new Date();
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  }
  function longDate() {
    return new Date().toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric' });
  }
  function metaOf(t: Task): string {
    if (t.scheduled_start) {
      const d = new Date(t.scheduled_start);
      if (!isNaN(d.getTime())) return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
    }
    return t.roughly_at ?? 'anytime';
  }

  // ── Ambient / idle face (spec §2.9) ───────────────────────────────────────
  let idle = $state(false);
  let idleTimer: ReturnType<typeof setTimeout> | null = null;
  const IDLE_MS = 90_000;
  function poke() {
    if (idle) idle = false;
    if (idleTimer) clearTimeout(idleTimer);
    idleTimer = setTimeout(() => { if (!adding) idle = true; }, IDLE_MS);
  }

  // ── On-screen keyboard ────────────────────────────────────────────────────
  const kbRows = ['qwertyuiop', 'asdfghjkl', 'zxcvbnm'];
  function press(ch: string) { draft += ch; poke(); }
  function backspace() { draft = draft.slice(0, -1); poke(); }

  onMount(() => {
    // Load the device token from the keyring; if there's none, pair first.
    void initTauriToken().then(() => {
      if (getTauriToken()) void load();
      else void startPairing();
    });
    const t = setInterval(() => { clock = timeLabel(); dateLabel = longDate(); }, 30_000);
    const onResize = () => (w = window.innerWidth);
    window.addEventListener('resize', onResize);
    poke();
    return () => {
      clearInterval(t); window.removeEventListener('resize', onResize);
      if (idleTimer) clearTimeout(idleTimer); if (pairPoll) clearInterval(pairPoll);
    };
  });
  let lastSeen: unknown = null;
  $effect(() => { const ev = realtime.lastEvent; if (ev !== lastSeen) { lastSeen = ev; void load(); } });
</script>

<svelte:window onpointerdown={poke} onkeydown={poke} />

<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
<div
  onpointerdown={() => idle && poke()}
  style="position:relative; width:100vw; height:100vh; overflow:hidden; box-sizing:border-box;
         background:var(--sempa-bg-main); color:var(--sempa-text);
         font-family:'Plus Jakarta Sans',sans-serif; padding:{c.pad}px; display:flex; flex-direction:column;
         cursor:none; user-select:none;">

  {#if needsPairing}
    <!-- Pairing: show a code; approve it from a signed-in Sempa app. -->
    <div style="position:absolute; inset:0; display:flex; align-items:center; justify-content:center; padding:{c.pad}px;">
      <div style="display:flex; flex-direction:column; align-items:center; gap:18px; text-align:center;
                  max-width:560px; width:100%; box-sizing:border-box; padding:{c.pad + 6}px {c.pad}px;
                  background:var(--sempa-bg-panel); border-radius:{c.card}px; box-shadow:var(--sempa-shadow-2);">
        <span style="display:flex; align-items:center; justify-content:center; width:64px; height:64px; border-radius:999px;
                     background:var(--sempa-accent-bg); color:var(--sempa-accent);">
          <svg width="36" height="36" viewBox="0 0 100 100" fill="none" aria-hidden="true">
            <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="9" stroke-linecap="round" stroke-linejoin="round"/>
            <circle cx="50" cy="35" r="7.5" fill="currentColor"/>
          </svg>
        </span>
        <div style="font-size:{c.title * 0.7}px; font-weight:800; letter-spacing:-.02em; color:var(--sempa-text);">Pair this Dock</div>
        <div style="font-family:'JetBrains Mono',monospace; font-size:{w >= 1100 ? 52 : 40}px; letter-spacing:.14em; font-weight:700;
                    color:var(--sempa-accent-ink); background:var(--sempa-fill); border-radius:999px; padding:10px 30px;">{pairCode || '········'}</div>
        <div style="font-size:16px; color:var(--sempa-text-soft); max-width:420px; line-height:1.5;">
          In Sempa on your phone or computer, open <b style="color:var(--sempa-text);">Settings → Paired devices</b> and enter this code.
        </div>
      </div>
    </div>
  {:else if idle}
    <!-- Ambient face: dimmed, serene; tap anywhere wakes it. -->
    <div style="position:absolute; inset:0; display:flex; flex-direction:column; align-items:center; justify-content:center;
                gap:18px; background:var(--sempa-bg-main); opacity:0.55; transition:opacity .6s;">
      <div style="font-family:'JetBrains Mono',monospace; font-size:64px; color:var(--sempa-text);">{clock}</div>
      <div style="font-size:20px; color:var(--sempa-text-soft);">{dateLabel}</div>
      <div style="display:flex; align-items:center; gap:14px; margin-top:8px; padding:14px 28px 14px 22px; border-radius:999px;
                  background:var(--sempa-bg-panel); box-shadow:var(--sempa-shadow-1); max-width:80%;">
        <span style="width:14px; height:14px; border-radius:999px; background:var(--sempa-amber); flex-shrink:0;
                     box-shadow:0 0 0 6px color-mix(in srgb, var(--sempa-amber) 18%, transparent);"></span>
        <span style="font-size:22px; font-weight:600; color:var(--sempa-text); overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{active[0]?.title ?? 'All done — rest easy.'}</span>
      </div>
      <div style="font-size:15px; color:var(--sempa-text-dim);">{done.length}/{total} done today</div>
    </div>
  {:else}
    <!-- Header -->
    <div style="display:flex; align-items:flex-start; gap:16px; margin-bottom:{c.headGap}px;">
      <span style="color:var(--sempa-accent); flex-shrink:0; margin-top:{c.title * 0.22}px;">
        <svg width={c.title * 0.7} height={c.title * 0.7} viewBox="0 0 100 100" fill="none" aria-hidden="true">
          <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="9" stroke-linecap="round" stroke-linejoin="round"/>
          <circle cx="50" cy="35" r="7.5" fill="currentColor"/>
        </svg>
      </span>
      <div style="flex:1; min-width:0;">
        <div style="font-weight:800; font-size:{c.title}px; letter-spacing:-.03em; line-height:1.15; color:var(--sempa-text);">{dateLabel}</div>
        {#if intention}<div style="font-style:italic; font-size:{c.sub}px; color:var(--sempa-text-soft); margin-top:4px;">{intention}</div>{/if}
      </div>
      <div style="font-family:'JetBrains Mono',monospace; font-size:{c.title * 0.5}px; font-weight:600; color:var(--sempa-text);
                  background:var(--sempa-bg-panel); box-shadow:var(--sempa-shadow-1); border-radius:999px;
                  padding:{c.title * 0.2}px {c.title * 0.42}px; flex-shrink:0;">{clock}</div>
    </div>

    <!-- Progress strip -->
    <div style="display:flex; align-items:center; gap:14px; margin-bottom:{c.headGap}px;">
      <span style="font-size:{c.sub}px; font-weight:600; color:var(--sempa-text-soft);"><b style="color:var(--sempa-text); font-weight:800;">{done.length}</b> of {total} done today</span>
      <div style="flex:1; height:6px; border-radius:999px; background:var(--sempa-track); overflow:hidden;">
        <div style="width:{pct}%; height:100%; border-radius:999px; background:var(--sempa-accent); transition:width .4s;"></div>
      </div>
      <span style="font-family:'JetBrains Mono',monospace; font-size:{c.sub}px; font-weight:600; color:var(--sempa-text-soft);">{pct}%</span>
    </div>

    <!-- Paper list -->
    <div style="flex:1; min-height:0; overflow:auto; background:var(--sempa-bg-panel);
                border-radius:{c.card}px; box-shadow:var(--sempa-shadow-1); padding:{c.inset}px 0;">
      {#if active.length === 0 && done.length === 0}
        <div style="height:100%; display:flex; align-items:center; justify-content:center; color:var(--sempa-text-dim); font-size:18px;">All done — rest easy.</div>
      {/if}
      {#each active as t, i (t.id)}
        <!-- Row: inset by c.inset so the current task can sit in a soft rounded
             amber block; padding compensates so the spine stays on one axis and
             there's no vertical margin, keeping the thread unbroken. -->
        <div style="display:flex; align-items:stretch; gap:{c.gap}px; min-height:{c.rowH}px;
                    margin:0 {c.inset}px; padding:0 {c.pad - c.inset}px; border-radius:{c.card - c.inset}px;
                    background:{t.id === nowId ? 'var(--cal-amber-bg)' : 'transparent'};">
          <!-- Day-thread spine cell -->
          <div style="width:{c.node * 2 + 10}px; position:relative; display:flex; align-items:center; justify-content:center; flex-shrink:0;">
            {#if i > 0}<div style="position:absolute; top:0; height:50%; width:{c.line}px; background:var(--sempa-thread-future, var(--sempa-track));"></div>{/if}
            {#if i < active.length - 1 || done.length}<div style="position:absolute; bottom:0; height:50%; width:{c.line}px; background:var(--sempa-thread-future, var(--sempa-track));"></div>{/if}
            {#if t.id === nowId}
              <div style="width:{c.node * 2}px; height:{c.node * 2}px; border-radius:50%; background:var(--sempa-amber); z-index:1;
                          box-shadow:0 0 0 {c.node * 0.6}px color-mix(in srgb, var(--sempa-amber) 24%, transparent);"></div>
            {:else}
              <div style="width:{c.node * 2}px; height:{c.node * 2}px; border-radius:50%; background:var(--sempa-bg-panel);
                          border:{c.line}px solid var(--sempa-thread-future, var(--sempa-track)); z-index:1; box-sizing:border-box;"></div>
            {/if}
          </div>
          <!-- Check: full-size touch target, round ring drawn inside it -->
          <div style="display:flex; align-items:center;">
            <button onclick={() => toggle(t)} aria-label="Complete {t.title}"
                    style="width:{c.check}px; height:{c.check}px; border:none; background:none; padding:0; cursor:pointer; touch-action:manipulation;
                           display:flex; align-items:center; justify-content:center;">
              <span class="check-ring" aria-hidden="true"
                    style="width:{c.ring}px; height:{c.ring}px; border-width:{t.id === nowId ? 3 : 2.5}px;
                           border-color:{t.id === nowId ? 'var(--sempa-amber)' : 'color-mix(in srgb, var(--sempa-text-dim) 55%, var(--sempa-track))'};
                           {t.id === nowId ? 'background:var(--sempa-bg-panel);' : ''}"></span>
            </button>
          </div>
          <!-- Title + meta -->
          <div style="flex:1; min-width:0; display:flex; flex-direction:column; justify-content:center; gap:2px;">
            <div style="font-size:{c.titleSz}px; font-weight:{t.id === nowId ? 700 : 600}; letter-spacing:-.01em; color:var(--sempa-text);
                        overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{t.title}</div>
            {#if c.showMeta && (t.scheduled_start || t.roughly_at)}
              <div style="font-family:'JetBrains Mono',monospace; font-size:13.5px; color:var(--sempa-text-soft);">{metaOf(t)}{#if t.id === nowId} · now{/if}</div>
            {/if}
          </div>
          {#if t.id === nowId}
            <div style="display:flex; align-items:center;">
              <span style="display:inline-flex; align-items:center; gap:7px; height:{w >= 1100 ? 30 : 26}px; padding:0 {w >= 1100 ? 13 : 10}px;
                           border-radius:999px; background:var(--sempa-bg-panel); box-shadow:var(--sempa-shadow-1);
                           font-size:{w >= 1100 ? 13 : 12}px; font-weight:800; letter-spacing:.04em;
                           color:var(--cal-amber-fg);">
                <span style="width:7px; height:7px; border-radius:999px; background:var(--sempa-amber);"></span>NOW
              </span>
            </div>
          {/if}
        </div>
      {/each}

      {#if done.length}
        <button onclick={() => (doneOpen = !doneOpen)}
                style="display:flex; align-items:center; gap:{c.gap}px; width:calc(100% - {c.inset * 2}px); min-height:{c.rowH * 0.7}px;
                       margin:0 {c.inset}px; padding:0 {c.pad - c.inset}px; border:none; background:none; cursor:pointer;
                       color:var(--sempa-text-soft); font-size:{c.titleSz - 3}px; font-weight:600;">
          <span style="width:{c.node * 2 + 10}px; display:flex; justify-content:center;">
            <span style="width:{c.node * 2}px; height:{c.node * 2}px; border-radius:50%; background:var(--sempa-success);"></span>
          </span>
          {done.length} done today
          <span aria-hidden="true"
                style="margin-left:auto; display:flex; align-items:center; justify-content:center; width:{c.check * 0.72}px; height:{c.check * 0.72}px;
                       border-radius:999px; background:var(--sempa-fill); color:var(--sempa-text-soft);">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4"
                 style="transform:rotate({doneOpen ? 90 : 0}deg); transition:transform .2s;">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 6l6 6-6 6" />
            </svg>
          </span>
        </button>
        {#if doneOpen}
          {#each done as t (t.id)}
            <div style="display:flex; align-items:center; gap:{c.gap}px; min-height:{c.rowH * 0.7}px; margin:0 {c.inset}px; padding:0 {c.pad - c.inset}px;">
              <span style="width:{c.node * 2 + 10}px; display:flex; justify-content:center;">
                <span style="width:{c.node * 1.2}px; height:{c.node * 1.2}px; border-radius:50%; background:color-mix(in srgb, var(--sempa-success) 55%, var(--sempa-track));"></span>
              </span>
              <button onclick={() => toggle(t)} aria-label="Mark not done"
                      style="width:{c.check}px; height:{c.check * 0.78}px; border:none; background:none; padding:0; cursor:pointer;
                             display:flex; align-items:center; justify-content:center;">
                <span class="check-ring" data-state="done" aria-hidden="true" style="width:{c.ring * 0.82}px; height:{c.ring * 0.82}px;">
                  <svg width={c.ring * 0.5} height={c.ring * 0.5} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M5 12.5l4.5 4.5L19 7.5" />
                  </svg>
                </span>
              </button>
              <span style="flex:1; font-size:{c.titleSz - 2}px; color:var(--sempa-text-dim); text-decoration:line-through;
                           overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{t.title}</span>
            </div>
          {/each}
        {/if}
      {/if}
    </div>

    <!-- Add button -->
    <button onclick={() => { adding = true; poke(); }}
            style="margin-top:{c.gap}px; width:100%; background:var(--sempa-btn-bg); box-shadow: var(--sempa-shadow-accent); color:var(--sempa-btn-fg); border:none;
                   border-radius:999px; padding:{c.rowH * 0.26}px; font-size:{c.addFont}px; font-weight:700; cursor:pointer;
                   display:flex; align-items:center; justify-content:center; gap:10px; touch-action:manipulation;">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4"><path stroke-linecap="round" d="M12 5v14M5 12h14"/></svg>
      Add a task
    </button>
  {/if}

  {#if adding}
    <!-- On-screen keyboard overlay; actions pinned at the TOP (Android-keyboard gotcha). -->
    <div style="position:absolute; left:0; right:0; bottom:0; background:var(--sempa-bg-main);
                border-radius:{c.card}px {c.card}px 0 0; padding:{c.pad * 0.8}px {c.pad}px {c.pad}px;
                box-shadow:0 -18px 44px -12px rgba(0,0,0,0.22), inset 0 1px 0 color-mix(in srgb, var(--sempa-text) 6%, transparent);
                display:flex; flex-direction:column; gap:10px; z-index:5;">
      <div style="display:flex; gap:10px; align-items:center;">
        <div style="flex:1; min-height:{c.kbKey}px; display:flex; align-items:center; padding:0 {c.kbKey * 0.4}px; border-radius:999px;
                    background:var(--sempa-bg-panel); border:1.5px solid var(--sempa-accent); box-sizing:border-box;
                    font-size:{c.kbFont}px; color:var(--sempa-text);">
          {#if draft}{draft}{:else}<span style="color:var(--sempa-text-dim);">New task…</span>{/if}
          <span style="width:2px; height:{c.kbFont}px; background:var(--sempa-accent); margin-left:2px; animation:sempaCaret 1.1s steps(1) infinite;"></span>
        </div>
        <button onclick={() => { adding = false; draft = ''; }}
                style="min-height:{c.kbKey}px; padding:0 {c.kbKey * 0.45}px; border-radius:999px; border:none;
                       background:var(--sempa-bg-panel); box-shadow:var(--sempa-shadow-1); color:var(--sempa-text);
                       font-weight:600; font-size:{c.kbFont - 4}px; cursor:pointer;">Cancel</button>
        <button onclick={commitAdd}
                style="min-height:{c.kbKey}px; padding:0 {c.kbKey * 0.55}px; border-radius:999px; border:none; background:var(--sempa-btn-bg); box-shadow:var(--sempa-shadow-accent);
                       color:var(--sempa-btn-fg); font-weight:700; font-size:{c.kbFont - 4}px; cursor:pointer;">Add</button>
      </div>
      {#each kbRows as row, ri}
        <div style="display:flex; gap:8px; justify-content:center; padding:0 {ri === 1 ? c.kbKey * 0.5 : 0}px;">
          {#if ri === 2}
            <button onpointerdown={(e) => { e.preventDefault(); backspace(); }}
                    style="flex:1.6; height:{c.kbKey}px; border-radius:16px; border:none;
                           background:var(--sempa-track); color:var(--sempa-text-soft); font-size:{c.kbFont}px; cursor:pointer; touch-action:none;">⌫</button>
          {/if}
          {#each row.split('') as ch}
            <button onpointerdown={(e) => { e.preventDefault(); press(ch); }}
                    style="flex:1; min-width:{c.kbKey}px; height:{c.kbKey}px; border-radius:16px; border:none; box-shadow:var(--sempa-shadow-1);
                           background:var(--sempa-bg-panel); color:var(--sempa-text); font-size:{c.kbFont}px; font-weight:600;
                           cursor:pointer; touch-action:none; -webkit-tap-highlight-color:transparent;">{ch}</button>
          {/each}
          {#if ri === 2}
            <button onpointerdown={(e) => { e.preventDefault(); press(' '); }}
                    style="flex:1.6; height:{c.kbKey}px; border-radius:16px; border:none;
                           background:var(--sempa-track); color:var(--sempa-text-soft); font-size:{c.kbFont - 2}px; font-weight:600; cursor:pointer; touch-action:none;">space</button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  @keyframes sempaCaret { 0%, 49% { opacity: 1; } 50%, 100% { opacity: 0; } }
</style>
