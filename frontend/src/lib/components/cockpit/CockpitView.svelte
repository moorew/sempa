<script lang="ts">
  /**
   * Cockpit mode — the ultrawide-short "today" cockpit (companion study /
   * LINUX_APP_SPEC §1.8). Self-contained: fetches today's tasks, intention and
   * the current weekly objective, derives now/next, and renders the horizontal
   * cockpit (NowCard | TodayList + quick-add | WeekPanel) with a day-thread
   * timeline below — or the calmer Glance variant. Tokens are the shipping
   * --sempa-* vars. Reorder lives in the normal day view; cockpit does
   * complete + quick-add. Mounted by +layout when cockpit.active.
   */
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { cockpit } from '$lib/stores/cockpit.svelte';
  import { clock as dayClock } from '$lib/stores/clock.svelte';
  import { realtime } from '$lib/stores/realtime.svelte';
  import type { Task } from '$lib/types';

  let tasks = $state<Task[]>([]);
  let intention = $state<string>('');
  let objectiveTitle = $state<string>('');
  let clock = $state(nowLabel());
  // Reactive so the always-on Dock rolls its "today" over at midnight instead of
  // showing yesterday until it's restarted. The $effect below reloads on change.
  const date = $derived(dayClock.today);

  const isDone = (t: Task) => t.status === 'done';
  const isLive = (t: Task) => t.status !== 'done' && t.status !== 'cancelled';

  let active = $derived(tasks.filter(isLive).sort((a, b) => a.position - b.position));
  let doneList = $derived(tasks.filter(isDone));
  let total = $derived(active.length + doneList.length);
  let doneCount = $derived(doneList.length);
  let pct = $derived(total ? Math.round((doneCount / total) * 100) : 0);
  let now = $derived(active[0] ?? null);
  let next = $derived(active[1] ?? null);

  async function load() {
    try {
      const [ts, plan] = await Promise.all([
        api.tasks.listByDate(date),
        api.plans.get(date).catch(() => null),
      ]);
      tasks = ts;
      intention = plan?.intention ?? '';
    } catch { /* offline — keep prior */ }
    // Current weekly objective (best-effort; cockpit still renders without it).
    try {
      const { weekStart } = await import('$lib/utils');
      const objs = await api.objectives.listByWeek(weekStart(date));
      objectiveTitle = objs?.[0]?.title ?? '';
    } catch { /* none */ }
  }

  async function toggle(t: Task) {
    const wantDone = !isDone(t);
    // optimistic
    tasks = tasks.map((x) => (x.id === t.id ? { ...x, status: wantDone ? 'done' : 'planned' } : x));
    try { await api.tasks.update(t.id, { status: wantDone ? 'done' : 'planned' }); }
    catch { void load(); }
  }

  let draft = $state('');
  async function add() {
    const title = draft.trim();
    if (!title) return;
    draft = '';
    try {
      await api.tasks.create({ title, planned_date: date, status: 'planned' });
      await load();
    } catch { /* offline */ }
  }

  function nowLabel() {
    const d = new Date();
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  }
  function minutesOf(iso: string | null): number | null {
    if (!iso) return null;
    const d = new Date(iso);
    if (isNaN(d.getTime())) return null;
    return d.getHours() * 60 + d.getMinutes();
  }
  function hhmm(iso: string | null): string {
    const m = minutesOf(iso);
    if (m == null) return '';
    return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`;
  }

  // Timeline window 08:00–18:00.
  const A = 480, B = 1080, SPAN = B - A;
  const xpct = (m: number) => Math.max(0, Math.min(100, ((m - A) / SPAN) * 100));
  let nowMin = $derived(() => { const d = new Date(); return d.getHours() * 60 + d.getMinutes(); });
  let blocks = $derived(
    tasks
      .map((t) => ({ t, s: minutesOf(t.scheduled_start), e: minutesOf(t.scheduled_end) }))
      .filter((b): b is { t: Task; s: number; e: number | null } => b.s != null),
  );

  onMount(() => {
    void load();
    const tick = setInterval(() => (clock = nowLabel()), 30_000);
    return () => clearInterval(tick);
  });
  // Re-fetch when realtime signals a change.
  let lastSeen: unknown = null;
  $effect(() => {
    const ev = realtime.lastEvent;
    if (ev !== lastSeen) { lastSeen = ev; void load(); }
  });
  // Reload when the day rolls over at midnight (`date` tracks dayClock.today).
  let lastDate = date;
  $effect(() => {
    if (date !== lastDate) { lastDate = date; void load(); }
  });
</script>

{#snippet checkCircle(t: Task, size: number)}
  <button onclick={() => toggle(t)} aria-label={isDone(t) ? 'Mark not done' : 'Complete'}
          style="width:{size}px; height:{size}px; border:none; background:none; cursor:pointer; flex-shrink:0; padding:0;">
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none"
         stroke={t === now ? 'var(--sempa-amber)' : isDone(t) ? 'var(--sempa-success)' : 'var(--sempa-text-dim)'}
         stroke-width="2">
      <circle cx="12" cy="12" r="10" />
      {#if isDone(t)}<path stroke-linecap="round" stroke-linejoin="round" d="M7 12.5l3.2 3.2L17 8.5" />{/if}
    </svg>
  </button>
{/snippet}

{#snippet ring(value: number, size: number, stroke: number)}
  {@const r = (size - stroke) / 2}
  {@const c = 2 * Math.PI * r}
  <svg width={size} height={size} viewBox="0 0 {size} {size}" style="flex-shrink:0;">
    <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--sempa-accent-bg)" stroke-width={stroke} />
    <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--sempa-accent)" stroke-width={stroke}
            stroke-linecap="round" stroke-dasharray="{c}" stroke-dashoffset="{c * (1 - value)}"
            transform="rotate(-90 {size / 2} {size / 2})" style="transition:stroke-dashoffset .4s;" />
    <text x="50%" y="50%" text-anchor="middle" dominant-baseline="central"
          style="font-family:'JetBrains Mono',monospace; font-size:{size * 0.2}px; fill:var(--sempa-text);">{doneCount}/{total}</text>
  </svg>
{/snippet}

{#snippet nowCard(big: boolean)}
  <div style="display:flex; flex-direction:column; height:100%; background:var(--sempa-bg-panel);
              border:1px solid var(--sempa-border); border-radius:20px; padding:{big ? '34px 38px' : '24px 26px'};
              box-shadow:0 12px 30px rgba(0,0,0,0.10);">
    <div style="display:flex; align-items:center; gap:10px;">
      <span style="width:9px; height:9px; border-radius:5px; background:var(--sempa-amber);
                   box-shadow:0 0 0 4px color-mix(in srgb, var(--sempa-amber) 18%, transparent);"></span>
      <span style="font-family:'JetBrains Mono',monospace; font-size:13.5px; letter-spacing:.16em;
                   color:var(--sempa-amber); font-weight:600;">NOW</span>
    </div>
    {#if now}
      <div style="font-weight:600; font-size:{big ? '60px' : '38px'}; line-height:1.04; letter-spacing:-.02em;
                  color:var(--sempa-text); margin-top:14px; text-wrap:balance;">{now.title}</div>
      <div style="font-family:'JetBrains Mono',monospace; font-size:14px; color:var(--sempa-text-soft); margin-top:10px;">
        {#if now.scheduled_start}{hhmm(now.scheduled_start)}{#if now.scheduled_end} – {hhmm(now.scheduled_end)}{/if}{:else}Unscheduled{/if}
      </div>
      <div style="flex:1;"></div>
      <div style="display:flex; gap:10px; margin-top:18px;">
        <button onclick={() => toggle(now)}
                style="flex:1; display:flex; align-items:center; justify-content:center; gap:8px;
                       background:var(--sempa-btn-bg); color:var(--sempa-btn-fg); border:none; border-radius:12px;
                       padding:14px; font-size:16px; font-weight:600; cursor:pointer;">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>
          Complete
        </button>
      </div>
      {#if next}
        <div style="margin-top:16px; padding-top:14px; border-top:1px solid var(--sempa-border); display:flex; align-items:center; gap:8px;">
          <span style="font-family:'JetBrains Mono',monospace; font-size:12.5px; letter-spacing:.12em; color:var(--sempa-text-dim);">NEXT</span>
          <span style="font-size:15px; color:var(--sempa-text-soft); overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{next.title}</span>
          {#if next.scheduled_start}<span style="margin-left:auto; font-family:'JetBrains Mono',monospace; font-size:13px; color:var(--sempa-text-dim);">{hhmm(next.scheduled_start)}</span>{/if}
        </div>
      {/if}
    {:else}
      <div style="flex:1; display:flex; align-items:center; justify-content:center; color:var(--sempa-text-dim); font-size:18px;">All done — rest easy.</div>
    {/if}
  </div>
{/snippet}

{#snippet timeline(h: number)}
  <div style="height:{h}px; flex-shrink:0; background:var(--sempa-bg-panel); border:1px solid var(--sempa-border);
              border-radius:18px; padding:12px 20px 0; overflow:hidden; position:relative;">
    <div style="font-family:'JetBrains Mono',monospace; font-size:12.5px; letter-spacing:.14em; color:var(--sempa-text-soft);">TIMELINE</div>
    <div style="position:relative; height:{h - 56}px; margin-top:8px;">
      {#each [8,10,12,14,16,18] as hr}
        <div style="position:absolute; left:{xpct(hr*60)}%; top:0; bottom:0; width:1px; background:var(--sempa-border);"></div>
        <div style="position:absolute; left:{xpct(hr*60)}%; bottom:-18px; transform:translateX(-50%); font-family:'JetBrains Mono',monospace; font-size:11px; color:var(--sempa-text-soft);">{hr}</div>
      {/each}
      {#each blocks as b (b.t.id)}
        {@const dur = b.e && b.e > b.s ? b.e - b.s : 30}
        <div title={b.t.title}
             style="position:absolute; left:{xpct(b.s)}%; width:max({(dur/SPAN)*100}%, 1.6%); top:0; height:{h-78}px;
                    border-radius:9px; padding:6px 9px; overflow:hidden; font-size:13px; box-sizing:border-box;
                    background:{isDone(b.t) ? 'var(--sempa-success-bg)' : b.t === now ? 'var(--sempa-amber)' : 'var(--sempa-accent-bg)'};
                    color:{b.t === now ? '#2b1c0e' : isDone(b.t) ? 'var(--sempa-success)' : 'var(--sempa-accent)'};
                    border:1px solid {b.t === now ? 'var(--sempa-amber)' : isDone(b.t) ? 'var(--sempa-success)' : 'var(--sempa-accent)'};">
          <div style="font-weight:600; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{b.t.title}</div>
        </div>
      {/each}
      <!-- now-line: the cockpit day-thread "now" treatment -->
      <div style="position:absolute; left:{xpct(nowMin())}%; top:-4px; bottom:14px; width:2px; background:var(--sempa-amber);">
        <span style="position:absolute; top:-5px; left:-4px; width:10px; height:10px; border-radius:5px; background:var(--sempa-amber);"></span>
      </div>
    </div>
  </div>
{/snippet}

<div style="display:flex; flex-direction:column; height:100vh; background:var(--sempa-bg-main);
            color:var(--sempa-text); font-family:'Plus Jakarta Sans',sans-serif; overflow:hidden;">
  <!-- Header -->
  <div style="height:64px; flex-shrink:0; display:flex; align-items:center; gap:18px; padding:0 24px;
              border-bottom:1px solid var(--sempa-border);">
    <span style="display:flex; align-items:center; gap:9px; color:var(--sempa-accent);">
      <svg width="24" height="24" viewBox="0 0 100 100" fill="none" aria-hidden="true">
        <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="9" stroke-linecap="round" stroke-linejoin="round"/>
        <circle cx="50" cy="35" r="7.5" fill="currentColor"/>
      </svg>
      <span style="font-weight:600; font-size:20px; letter-spacing:-.02em; color:var(--sempa-text);">sempa</span>
    </span>
    <span style="font-family:'JetBrains Mono',monospace; font-size:11.5px; letter-spacing:.14em; color:var(--sempa-text-soft);
                 border:1px solid var(--sempa-border); border-radius:6px; padding:3px 7px;">COCKPIT</span>
    <div style="flex:1;"></div>
    <span style="font-size:14.5px; color:var(--sempa-text-soft);">{doneCount}/{total} today</span>
    <div style="width:140px; height:6px; border-radius:3px; background:var(--sempa-accent-bg);">
      <div style="width:{pct}%; height:100%; border-radius:3px; background:var(--sempa-accent); transition:width .4s;"></div>
    </div>
    <span style="font-family:'JetBrains Mono',monospace; font-size:15px; color:var(--sempa-text-soft);">{clock}</span>
    <button onclick={() => cockpit.toggleMode()}
            style="border:1px solid var(--sempa-border); background:none; color:var(--sempa-text-soft);
                   border-radius:9px; padding:6px 12px; font-size:13px; cursor:pointer;">
      {cockpit.mode === 'cockpit' ? 'Glance' : 'Expand'}
    </button>
  </div>

  {#if cockpit.mode === 'cockpit'}
    <!-- Cockpit: 3 columns + timeline -->
    <div style="flex:1; min-height:0; display:flex; flex-direction:column; gap:18px; padding:18px 24px 20px;">
      <div style="flex:1; min-height:0; display:flex; gap:18px;">
        <div style="width:34%; max-width:600px; flex-shrink:0;">{@render nowCard(false)}</div>
        <!-- TodayList -->
        <div style="flex:1; min-width:0; display:flex; flex-direction:column; background:var(--sempa-bg-panel);
                    border:1px solid var(--sempa-border); border-radius:20px; overflow:hidden; box-shadow:0 12px 30px rgba(0,0,0,0.10);">
          <div style="padding:18px 22px 12px; border-bottom:1px solid var(--sempa-border);">
            <div style="display:flex; align-items:center; gap:10px;">
              <span style="font-family:'JetBrains Mono',monospace; font-size:13px; letter-spacing:.14em; color:var(--sempa-text-soft);">TODAY</span>
              <div style="flex:1;"></div>
              <span style="font-size:15px; color:var(--sempa-text-soft);"><b style="color:var(--sempa-text);">{doneCount}</b> / {total} done</span>
            </div>
            <div style="display:flex; gap:8px; margin-top:12px;">
              <input bind:value={draft} placeholder="Add a task…" onkeydown={(e) => e.key === 'Enter' && add()}
                     style="flex:1; background:var(--sempa-bg-main); border:1px solid var(--sempa-border); border-radius:10px;
                            padding:9px 12px; font-size:15px; color:var(--sempa-text); outline:none;" />
              <button onclick={add} style="background:var(--sempa-accent-bg); color:var(--sempa-accent); border:none;
                            border-radius:10px; padding:0 16px; font-size:15px; font-weight:600; cursor:pointer;">Add</button>
            </div>
          </div>
          <div style="flex:1; overflow:auto; padding:8px 12px 14px;">
            {#each active as t (t.id)}
              <div style="display:flex; align-items:center; gap:12px; padding:9px 10px; border-radius:12px;
                          background:{t === now ? 'color-mix(in srgb, var(--sempa-amber) 14%, transparent)' : 'transparent'};
                          border-left:3px solid {t === now ? 'var(--sempa-amber)' : 'transparent'};">
                <span style="font-family:'JetBrains Mono',monospace; font-size:13.5px; color:var(--sempa-text-soft); width:52px;">{hhmm(t.scheduled_start) || '—'}</span>
                {@render checkCircle(t, 24)}
                <span style="flex:1; font-size:16px; font-weight:{t === now ? 600 : 500}; color:var(--sempa-text); overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{t.title}</span>
              </div>
            {/each}
            {#if doneList.length}
              <div style="margin-top:8px; padding:6px 10px; font-family:'JetBrains Mono',monospace; font-size:12px; color:var(--sempa-text-dim);">{doneCount} done</div>
              {#each doneList as t (t.id)}
                <div style="display:flex; align-items:center; gap:12px; padding:7px 10px; opacity:.6;">
                  <span style="width:52px;"></span>
                  {@render checkCircle(t, 22)}
                  <span style="flex:1; font-size:15px; color:var(--sempa-text-dim); text-decoration:line-through; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{t.title}</span>
                </div>
              {/each}
            {/if}
          </div>
        </div>
        <!-- WeekPanel -->
        <div style="width:26%; max-width:470px; flex-shrink:0; display:flex; flex-direction:column; gap:16px;
                    background:var(--sempa-bg-panel); border:1px solid var(--sempa-border); border-radius:20px;
                    padding:20px 22px; box-shadow:0 12px 30px rgba(0,0,0,0.10);">
          <div style="display:flex; align-items:center; gap:14px;">
            {@render ring(total ? doneCount / total : 0, 78, 9)}
            <div>
              <div style="font-size:14px; color:var(--sempa-text);">Today's progress</div>
              <div style="font-size:13px; color:var(--sempa-text-soft);">{active.length} left · {pct}% done</div>
            </div>
          </div>
          {#if objectiveTitle}
            <div style="padding-top:14px; border-top:1px solid var(--sempa-border);">
              <div style="font-family:'JetBrains Mono',monospace; font-size:12px; letter-spacing:.14em; color:var(--sempa-accent);">THIS WEEK</div>
              <div style="font-size:16px; font-weight:600; color:var(--sempa-text); margin-top:6px;">{objectiveTitle}</div>
            </div>
          {/if}
          <div style="flex:1;"></div>
          {#if intention}
            <div style="padding-top:14px; border-top:1px solid var(--sempa-border);">
              <div style="font-family:'JetBrains Mono',monospace; font-size:12px; letter-spacing:.14em; color:var(--sempa-text-dim);">INTENTION</div>
              <div style="font-size:15.5px; font-style:italic; color:var(--sempa-text-soft); line-height:1.45; margin-top:6px;">{intention}</div>
            </div>
          {/if}
        </div>
      </div>
      {@render timeline(170)}
    </div>
  {:else}
    <!-- Glance: big now card + summary + timeline -->
    <div style="flex:1; min-height:0; display:flex; flex-direction:column; gap:22px; padding:30px 40px 34px;">
      <div style="flex:1; min-height:0; display:flex; gap:40px;">
        <div style="flex:1.5; min-width:0;">{@render nowCard(true)}</div>
        <div style="flex:1; display:flex; align-items:center; gap:28px; background:var(--sempa-bg-panel);
                    border:1px solid var(--sempa-border); border-radius:20px; padding:0 40px; box-shadow:0 12px 30px rgba(0,0,0,0.10);">
          {@render ring(total ? doneCount / total : 0, 150, 14)}
          <div style="min-width:0;">
            <div style="font-size:24px; font-weight:600; color:var(--sempa-text);">{active.length} tasks left</div>
            <div style="font-size:15px; color:var(--sempa-text-soft); margin-top:4px;">{pct}% of today complete</div>
            {#if intention}
              <div style="margin-top:16px; padding-top:14px; border-top:1px solid var(--sempa-border); font-style:italic; font-size:15px; color:var(--sempa-text-soft); line-height:1.45;">{intention}</div>
            {/if}
          </div>
        </div>
      </div>
      {@render timeline(186)}
    </div>
  {/if}
</div>
