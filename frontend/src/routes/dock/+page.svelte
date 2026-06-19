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
  import { api } from '$lib/api';
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
      ? { pad: 34, gap: 16, title: 38, sub: 15, rowH: 78, check: 52, titleSz: 23, node: 9, line: 3, kbKey: 56, kbFont: 22, showMeta: true, addFont: 20, headGap: 18 }
      : { pad: 20, gap: 10, title: 26, sub: 13.5, rowH: 60, check: 42, titleSz: 19, node: 8, line: 2.5, kbKey: 40, kbFont: 16, showMeta: false, addFont: 17, headGap: 12 },
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
    void load();
    const t = setInterval(() => { clock = timeLabel(); dateLabel = longDate(); }, 30_000);
    const onResize = () => (w = window.innerWidth);
    window.addEventListener('resize', onResize);
    poke();
    return () => { clearInterval(t); window.removeEventListener('resize', onResize); if (idleTimer) clearTimeout(idleTimer); };
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

  {#if idle}
    <!-- Ambient face: dimmed, serene; tap anywhere wakes it. -->
    <div style="position:absolute; inset:0; display:flex; flex-direction:column; align-items:center; justify-content:center;
                gap:18px; background:var(--sempa-bg-main); opacity:0.55; transition:opacity .6s;">
      <div style="font-family:'JetBrains Mono',monospace; font-size:64px; color:var(--sempa-text);">{clock}</div>
      <div style="font-size:20px; color:var(--sempa-text-soft);">{dateLabel}</div>
      <div style="display:flex; align-items:center; gap:12px; margin-top:8px;">
        <span style="width:14px; height:14px; border-radius:7px; background:var(--sempa-amber);
                     box-shadow:0 0 0 6px color-mix(in srgb, var(--sempa-amber) 18%, transparent);"></span>
        <span style="font-size:22px; color:var(--sempa-text);">{active[0]?.title ?? 'All done — rest easy.'}</span>
      </div>
      <div style="font-size:15px; color:var(--sempa-text-dim);">{done.length}/{total} done today</div>
    </div>
  {:else}
    <!-- Header -->
    <div style="display:flex; align-items:flex-start; gap:16px; margin-bottom:{c.headGap}px;">
      <span style="color:var(--sempa-accent); flex-shrink:0;">
        <svg width={c.title * 0.7} height={c.title * 0.7} viewBox="0 0 100 100" fill="none" aria-hidden="true">
          <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="9" stroke-linecap="round" stroke-linejoin="round"/>
          <circle cx="50" cy="35" r="7.5" fill="currentColor"/>
        </svg>
      </span>
      <div style="flex:1; min-width:0;">
        <div style="font-weight:700; font-size:{c.title}px; letter-spacing:-.02em; color:var(--sempa-text);">{dateLabel}</div>
        {#if intention}<div style="font-style:italic; font-size:{c.sub}px; color:var(--sempa-text-soft); margin-top:2px;">{intention}</div>{/if}
      </div>
      <div style="font-family:'JetBrains Mono',monospace; font-size:{c.title * 0.5}px; color:var(--sempa-text);">{clock}</div>
    </div>

    <!-- Progress strip -->
    <div style="display:flex; align-items:center; gap:14px; margin-bottom:{c.headGap}px;">
      <span style="font-size:{c.sub}px; color:var(--sempa-text-soft);">{done.length} of {total} done today</span>
      <div style="flex:1; height:6px; border-radius:3px; background:var(--sempa-accent-bg);">
        <div style="width:{pct}%; height:100%; border-radius:3px; background:var(--sempa-accent); transition:width .4s;"></div>
      </div>
      <span style="font-family:'JetBrains Mono',monospace; font-size:{c.sub}px; color:var(--sempa-text-dim);">{pct}%</span>
    </div>

    <!-- Paper list -->
    <div style="flex:1; min-height:0; overflow:auto; background:var(--sempa-bg-panel); border:1px solid var(--sempa-border);
                border-radius:20px; box-shadow:0 12px 30px rgba(80,55,25,0.10); padding:4px 0;">
      {#if active.length === 0 && done.length === 0}
        <div style="height:100%; display:flex; align-items:center; justify-content:center; color:var(--sempa-text-dim); font-size:18px;">All done — rest easy.</div>
      {/if}
      {#each active as t, i (t.id)}
        <div style="display:flex; align-items:stretch; gap:{c.gap}px; min-height:{c.rowH}px; padding:0 {c.pad}px;
                    border-bottom:1px solid var(--sempa-border);">
          <!-- Day-thread spine cell -->
          <div style="width:{c.node * 2 + 10}px; position:relative; display:flex; align-items:center; justify-content:center; flex-shrink:0;">
            {#if i > 0}<div style="position:absolute; top:0; height:50%; width:{c.line}px; background:var(--sempa-thread-future, var(--sempa-border));"></div>{/if}
            {#if i < active.length - 1 || done.length}<div style="position:absolute; bottom:0; height:50%; width:{c.line}px; background:var(--sempa-thread-future, var(--sempa-border));"></div>{/if}
            {#if t.id === nowId}
              <div style="width:{c.node * 2}px; height:{c.node * 2}px; border-radius:50%; background:var(--sempa-amber); z-index:1;
                          box-shadow:0 0 0 4px color-mix(in srgb, var(--sempa-amber) 18%, transparent);"></div>
            {:else}
              <div style="width:{c.node * 2}px; height:{c.node * 2}px; border-radius:50%; background:var(--sempa-bg-main);
                          border:{c.line}px solid var(--sempa-thread-future, var(--sempa-border)); z-index:1; box-sizing:border-box;"></div>
            {/if}
          </div>
          <!-- Check -->
          <div style="display:flex; align-items:center;">
            <button onclick={() => toggle(t)} aria-label="Complete {t.title}"
                    style="width:{c.check}px; height:{c.check}px; border:none; background:none; padding:0; cursor:pointer; touch-action:manipulation;">
              <svg width={c.check} height={c.check} viewBox="0 0 24 24" fill="none"
                   stroke={t.id === nowId ? 'var(--sempa-amber)' : 'var(--sempa-text-dim)'} stroke-width="2">
                <circle cx="12" cy="12" r="10" />
              </svg>
            </button>
          </div>
          <!-- Title + meta -->
          <div style="flex:1; min-width:0; display:flex; flex-direction:column; justify-content:center;">
            <div style="font-size:{c.titleSz}px; font-weight:{t.id === nowId ? 700 : 600}; color:var(--sempa-text);
                        overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{t.title}</div>
            {#if c.showMeta && (t.scheduled_start || t.roughly_at)}
              <div style="font-family:'JetBrains Mono',monospace; font-size:13.5px; color:var(--sempa-text-soft);">{metaOf(t)}{#if t.id === nowId} · now{/if}</div>
            {/if}
          </div>
          {#if t.id === nowId}
            <div style="display:flex; align-items:center;">
              <span style="font-family:'JetBrains Mono',monospace; font-size:11px; letter-spacing:.12em; color:var(--sempa-amber);
                           background:color-mix(in srgb, var(--sempa-amber) 16%, transparent); border-radius:6px; padding:3px 8px;">NOW</span>
            </div>
          {/if}
        </div>
      {/each}

      {#if done.length}
        <button onclick={() => (doneOpen = !doneOpen)}
                style="display:flex; align-items:center; gap:{c.gap}px; width:100%; min-height:{c.rowH * 0.7}px; padding:0 {c.pad}px;
                       border:none; background:none; cursor:pointer; color:var(--sempa-success); font-size:{c.titleSz - 3}px;">
          <span style="width:{c.node * 2 + 10}px; display:flex; justify-content:center;">
            <span style="width:{c.node * 2}px; height:{c.node * 2}px; border-radius:50%; background:var(--sempa-success);"></span>
          </span>
          {done.length} done today
          <span style="margin-left:auto; font-family:'JetBrains Mono',monospace;">{doneOpen ? '▾' : '▸'}</span>
        </button>
        {#if doneOpen}
          {#each done as t (t.id)}
            <div style="display:flex; align-items:center; gap:{c.gap}px; min-height:{c.rowH * 0.7}px; padding:0 {c.pad}px; opacity:.75;">
              <span style="width:{c.node * 2 + 10}px; display:flex; justify-content:center;">
                <span style="width:{c.node * 1.6}px; height:{c.node * 1.6}px; border-radius:50%; background:var(--sempa-accent);"></span>
              </span>
              <button onclick={() => toggle(t)} aria-label="Mark not done"
                      style="width:{c.check * 0.78}px; height:{c.check * 0.78}px; border:none; background:none; padding:0; cursor:pointer;">
                <svg width={c.check * 0.78} height={c.check * 0.78} viewBox="0 0 24 24" fill="none" stroke="var(--sempa-success)" stroke-width="2">
                  <circle cx="12" cy="12" r="10" /><path stroke-linecap="round" stroke-linejoin="round" d="M7 12.5l3.2 3.2L17 8.5" />
                </svg>
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
            style="margin-top:{c.gap}px; width:100%; background:var(--sempa-btn-bg); color:var(--sempa-btn-fg); border:none;
                   border-radius:16px; padding:{c.rowH * 0.26}px; font-size:{c.addFont}px; font-weight:700; cursor:pointer;
                   display:flex; align-items:center; justify-content:center; gap:10px; touch-action:manipulation;">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4"><path stroke-linecap="round" d="M12 5v14M5 12h14"/></svg>
      Add a task
    </button>
  {/if}

  {#if adding}
    <!-- On-screen keyboard overlay; actions pinned at the TOP (Android-keyboard gotcha). -->
    <div style="position:absolute; left:0; right:0; bottom:0; background:var(--sempa-bg-panel); border-top:1px solid var(--sempa-border);
                padding:{c.pad}px; box-shadow:0 -20px 40px rgba(0,0,0,0.18); display:flex; flex-direction:column; gap:10px; z-index:5;">
      <div style="display:flex; gap:10px; align-items:center;">
        <div style="flex:1; min-height:{c.kbKey}px; display:flex; align-items:center; padding:0 14px; border-radius:12px;
                    background:var(--sempa-bg-main); border:1px solid var(--sempa-border); font-size:{c.kbFont}px; color:var(--sempa-text);">
          {#if draft}{draft}{:else}<span style="color:var(--sempa-text-dim);">New task…</span>{/if}
          <span style="width:2px; height:{c.kbFont}px; background:var(--sempa-accent); margin-left:2px; animation:sempaCaret 1.1s steps(1) infinite;"></span>
        </div>
        <button onclick={() => { adding = false; draft = ''; }}
                style="min-height:{c.kbKey}px; padding:0 18px; border-radius:12px; border:1px solid var(--sempa-border);
                       background:none; color:var(--sempa-text-soft); font-size:{c.kbFont - 4}px; cursor:pointer;">Cancel</button>
        <button onclick={commitAdd}
                style="min-height:{c.kbKey}px; padding:0 22px; border-radius:12px; border:none; background:var(--sempa-btn-bg);
                       color:var(--sempa-btn-fg); font-weight:700; font-size:{c.kbFont - 4}px; cursor:pointer;">Add</button>
      </div>
      {#each kbRows as row, ri}
        <div style="display:flex; gap:8px; justify-content:center; padding:0 {ri === 1 ? c.kbKey * 0.5 : 0}px;">
          {#if ri === 2}
            <button onpointerdown={(e) => { e.preventDefault(); backspace(); }}
                    style="flex:1.6; height:{c.kbKey}px; border-radius:10px; border:1px solid var(--sempa-border);
                           background:var(--sempa-bg-main); color:var(--sempa-text-soft); font-size:{c.kbFont}px; cursor:pointer; touch-action:none;">⌫</button>
          {/if}
          {#each row.split('') as ch}
            <button onpointerdown={(e) => { e.preventDefault(); press(ch); }}
                    style="flex:1; min-width:{c.kbKey}px; height:{c.kbKey}px; border-radius:10px; border:1px solid var(--sempa-border);
                           background:var(--sempa-bg-panel); color:var(--sempa-text); font-size:{c.kbFont}px; font-weight:600;
                           cursor:pointer; touch-action:none; -webkit-tap-highlight-color:transparent;">{ch}</button>
          {/each}
          {#if ri === 2}
            <button onpointerdown={(e) => { e.preventDefault(); press(' '); }}
                    style="flex:1.6; height:{c.kbKey}px; border-radius:10px; border:1px solid var(--sempa-border);
                           background:var(--sempa-bg-main); color:var(--sempa-text-soft); font-size:{c.kbFont}px; cursor:pointer; touch-action:none;">space</button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  @keyframes sempaCaret { 0%, 49% { opacity: 1; } 50%, 100% { opacity: 0; } }
</style>
