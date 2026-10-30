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
  import { tagStore } from '$lib/stores/tags.svelte';
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
  // Timeline block colour: the task's first tag (soft .tint block), else the accent.
  const tintOf = (t: Task) => (t.tags?.length ? tagStore.colorFor(t.tags[0]) : 'var(--sempa-accent)');

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
          class="check-ring" data-state={isDone(t) ? 'done' : undefined}
          style="width:{size}px; height:{size}px; cursor:pointer; padding:0;
                 {t === now ? 'border-color:var(--sempa-amber); background:var(--sempa-bg-panel);' : ''}">
    {#if isDone(t)}
      <svg width={size * 0.6} height={size * 0.6} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
        <path stroke-linecap="round" stroke-linejoin="round" d="M5 12.5l4.5 4.5L19 7.5" />
      </svg>
    {/if}
  </button>
{/snippet}

{#snippet ring(value: number, size: number, stroke: number)}
  {@const r = (size - stroke) / 2}
  {@const c = 2 * Math.PI * r}
  <svg width={size} height={size} viewBox="0 0 {size} {size}" style="flex-shrink:0;">
    <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--sempa-track)" stroke-width={stroke} />
    <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--sempa-accent)" stroke-width={stroke}
            stroke-linecap="round" stroke-dasharray="{c}" stroke-dashoffset="{c * (1 - value)}"
            transform="rotate(-90 {size / 2} {size / 2})" style="transition:stroke-dashoffset .4s;" />
    <text x="50%" y="50%" text-anchor="middle" dominant-baseline="central"
          style="font-family:'JetBrains Mono',monospace; font-size:{size * 0.2}px; font-weight:700; fill:var(--sempa-text);">{doneCount}/{total}</text>
  </svg>
{/snippet}

{#snippet nowCard(big: boolean)}
  <div style="display:flex; flex-direction:column; height:100%; box-sizing:border-box; background:var(--sempa-bg-panel);
              border-radius:28px; padding:{big ? '30px 34px' : '22px 24px'}; box-shadow:var(--sempa-shadow-1);">
    <div style="display:flex; align-items:center; gap:10px;">
      <span style="display:inline-flex; align-items:center; gap:8px; height:28px; padding:0 12px; border-radius:999px;
                   background:var(--cal-amber-bg); color:var(--cal-amber-fg); font-size:12.5px; font-weight:800; letter-spacing:.04em;">
        <span style="width:8px; height:8px; border-radius:999px; background:var(--sempa-amber);
                     box-shadow:0 0 0 3px color-mix(in srgb, var(--sempa-amber) 22%, transparent);"></span>NOW
      </span>
      {#if now}
        <span class="chip chip-time" style="height:26px; padding:0 11px; font-size:12.5px;">
          {#if now.scheduled_start}{hhmm(now.scheduled_start)}{#if now.scheduled_end} – {hhmm(now.scheduled_end)}{/if}{:else}Unscheduled{/if}
        </span>
      {/if}
    </div>
    {#if now}
      <div style="font-weight:800; font-size:{big ? '56px' : '34px'}; line-height:1.06; letter-spacing:-.035em;
                  color:var(--sempa-text); margin-top:16px; text-wrap:balance;">{now.title}</div>
      <div style="flex:1;"></div>
      <div style="display:flex; gap:10px; margin-top:18px;">
        <button onclick={() => toggle(now)} class="btn-primary"
                style="flex:1; height:{big ? 56 : 50}px; font-size:16px;">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>
          Complete
        </button>
      </div>
      {#if next}
        <div style="margin-top:14px; padding:10px 8px 10px 16px; border-radius:999px; background:var(--sempa-fill);
                    display:flex; align-items:center; gap:10px;">
          <span style="font-size:12px; font-weight:600; color:var(--sempa-text-soft); flex-shrink:0;">Next</span>
          <span style="font-size:15px; font-weight:600; color:var(--sempa-text); overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{next.title}</span>
          {#if next.scheduled_start}<span class="chip chip-time" style="margin-left:auto; background:var(--sempa-bg-panel);">{hhmm(next.scheduled_start)}</span>{/if}
        </div>
      {/if}
    {:else}
      <div style="flex:1; display:flex; align-items:center; justify-content:center; color:var(--sempa-text-dim); font-size:18px;">All done — rest easy.</div>
    {/if}
  </div>
{/snippet}

{#snippet timeline(h: number)}
  <div style="height:{h}px; flex-shrink:0; box-sizing:border-box; background:var(--sempa-bg-panel); box-shadow:var(--sempa-shadow-1);
              border-radius:28px; padding:14px 22px 0; overflow:hidden; position:relative;">
    <div style="font-size:15px; font-weight:800; letter-spacing:-.02em; color:var(--sempa-text);">Timeline</div>
    <div style="position:relative; height:{h - 72}px; margin-top:8px;">
      {#each [8,10,12,14,16,18] as hr}
        <div style="position:absolute; left:{xpct(hr*60)}%; top:0; bottom:0; width:1px; background:var(--sempa-border);"></div>
        <!-- edge labels align inward so 08:00 / 18:00 aren't clipped by the card -->
        <div style="position:absolute; left:{xpct(hr*60)}%; bottom:-20px; transform:translateX({hr === 8 ? '0' : hr === 18 ? '-100%' : '-50%'});
                    font-family:'JetBrains Mono',monospace; font-size:11px; color:var(--sempa-text-dim);">{String(hr).padStart(2, '0')}:00</div>
      {/each}
      {#each blocks as b (b.t.id)}
        {@const dur = b.e && b.e > b.s ? b.e - b.s : 30}
        <!-- Soft colour block in the task's tag hue; the current block is the one
             solid btn-bg fill, done blocks fall back to a quiet sage tint. -->
        <div title={b.t.title} class:tint={b.t !== now && !isDone(b.t)}
             style="position:absolute; left:{xpct(b.s)}%; width:max(calc({(dur/SPAN)*100}% - 3px), 1.6%); top:0; height:{h-92}px;
                    border-radius:14px; padding:8px 11px; overflow:hidden; font-size:13px; box-sizing:border-box;
                    {b.t === now
                      ? 'background:var(--sempa-btn-bg); color:var(--sempa-btn-fg); box-shadow:var(--sempa-shadow-accent); z-index:1;'
                      : isDone(b.t)
                        ? 'background:var(--cal-sage-bg); color:var(--cal-sage-fg);'
                        : `--tint:${tintOf(b.t)};`}">
          <div style="font-weight:700; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;
                      {isDone(b.t) ? 'text-decoration:line-through;' : ''}">{b.t.title}</div>
          {#if dur >= 45}
            <div style="font-family:'JetBrains Mono',monospace; font-size:11px; opacity:.8; margin-top:2px; white-space:nowrap;">{hhmm(b.t.scheduled_start)}{#if b.e} – {hhmm(b.t.scheduled_end)}{/if}</div>
          {/if}
        </div>
      {/each}
      <!-- now-line: the cockpit day-thread "now" treatment -->
      <div style="position:absolute; left:{xpct(nowMin())}%; top:-4px; bottom:14px; width:2px; border-radius:999px; background:var(--sempa-amber); z-index:2;">
        <span style="position:absolute; top:-5px; left:-4px; width:10px; height:10px; border-radius:999px; background:var(--sempa-amber);
                     box-shadow:0 0 0 3px color-mix(in srgb, var(--sempa-amber) 25%, transparent);"></span>
      </div>
    </div>
  </div>
{/snippet}

<div style="display:flex; flex-direction:column; height:100vh; background:var(--sempa-bg-main);
            color:var(--sempa-text); font-family:'Plus Jakarta Sans',sans-serif; overflow:hidden;">
  <!-- Header -->
  <div style="height:64px; flex-shrink:0; display:flex; align-items:center; gap:16px; padding:0 24px;">
    <span style="display:flex; align-items:center; gap:9px; color:var(--sempa-accent);">
      <svg width="24" height="24" viewBox="0 0 100 100" fill="none" aria-hidden="true">
        <path d="M22,40 a28,28 0 0 0 56,0" stroke="currentColor" stroke-width="9" stroke-linecap="round" stroke-linejoin="round"/>
        <circle cx="50" cy="35" r="7.5" fill="currentColor"/>
      </svg>
      <span style="font-weight:800; font-size:20px; letter-spacing:-.03em; color:var(--sempa-text);">sempa</span>
    </span>
    <span class="chip" style="background:var(--sempa-track); color:var(--sempa-text-soft); font-size:12px;">Cockpit</span>
    <div style="flex:1;"></div>
    <span style="font-size:14px; font-weight:600; color:var(--sempa-text-soft);"><b style="color:var(--sempa-text); font-weight:800;">{doneCount}</b>/{total} today</span>
    <div style="width:140px; height:6px; border-radius:999px; background:var(--sempa-track); overflow:hidden;">
      <div style="width:{pct}%; height:100%; border-radius:999px; background:var(--sempa-accent); transition:width .4s;"></div>
    </div>
    <span style="font-family:'JetBrains Mono',monospace; font-size:15px; font-weight:600; color:var(--sempa-text);
                 background:var(--sempa-bg-panel); box-shadow:var(--sempa-shadow-1); border-radius:999px; padding:6px 14px;">{clock}</span>
    <button onclick={() => cockpit.toggleMode()} class="btn-surface"
            style="height:36px; padding:0 16px; font-size:13px;">
      {cockpit.mode === 'cockpit' ? 'Glance' : 'Expand'}
    </button>
  </div>

  {#if cockpit.mode === 'cockpit'}
    <!-- Cockpit: 3 columns + timeline -->
    <div style="flex:1; min-height:0; display:flex; flex-direction:column; gap:18px; padding:4px 24px 22px;">
      <div style="flex:1; min-height:0; display:flex; gap:18px;">
        <div style="width:34%; max-width:600px; flex-shrink:0;">{@render nowCard(false)}</div>
        <!-- TodayList -->
        <div style="flex:1; min-width:0; display:flex; flex-direction:column; background:var(--sempa-bg-panel);
                    border-radius:28px; overflow:hidden; box-shadow:var(--sempa-shadow-1);">
          <div style="padding:18px 22px 10px;">
            <div style="display:flex; align-items:center; gap:10px;">
              <span style="font-size:17px; font-weight:800; letter-spacing:-.02em; color:var(--sempa-text);">Today</span>
              <div style="flex:1;"></div>
              <span style="font-size:14px; font-weight:600; color:var(--sempa-text-soft);"><b style="color:var(--sempa-text); font-weight:800;">{doneCount}</b> / {total} done</span>
            </div>
            <div style="display:flex; gap:8px; margin-top:12px;">
              <input bind:value={draft} placeholder="Add a task…" onkeydown={(e) => e.key === 'Enter' && add()}
                     style="flex:1; min-width:0; height:40px; box-sizing:border-box; background:var(--sempa-fill); border:1px solid var(--sempa-border);
                            border-radius:999px; padding:0 18px; font-size:15px; color:var(--sempa-text); outline:none;" />
              <button onclick={add} class="btn-soft" style="height:40px; padding:0 20px; font-size:15px;">Add</button>
            </div>
          </div>
          <div style="flex:1; overflow:auto; padding:4px 12px 14px;">
            {#each active as t (t.id)}
              <div style="display:flex; align-items:center; gap:12px; padding:8px 12px; border-radius:16px;
                          background:{t === now ? 'var(--cal-amber-bg)' : 'transparent'};">
                <span style="font-family:'JetBrains Mono',monospace; font-size:13px; color:{t === now ? 'var(--cal-amber-fg)' : 'var(--sempa-text-soft)'}; width:48px; flex-shrink:0;">{hhmm(t.scheduled_start) || '—'}</span>
                {@render checkCircle(t, 22)}
                <span style="flex:1; font-size:15.5px; font-weight:{t === now ? 700 : 500}; color:var(--sempa-text); overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{t.title}</span>
              </div>
            {/each}
            {#if doneList.length}
              <div style="margin-top:8px; padding:6px 12px; font-size:12px; font-weight:600; color:var(--sempa-text-soft);">{doneCount} done</div>
              {#each doneList as t (t.id)}
                <div style="display:flex; align-items:center; gap:12px; padding:6px 12px;">
                  <span style="width:48px; flex-shrink:0;"></span>
                  {@render checkCircle(t, 20)}
                  <span style="flex:1; font-size:15px; color:var(--sempa-text-dim); text-decoration:line-through; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{t.title}</span>
                </div>
              {/each}
            {/if}
          </div>
        </div>
        <!-- WeekPanel -->
        <div style="width:26%; max-width:470px; flex-shrink:0; box-sizing:border-box; display:flex; flex-direction:column; gap:12px;
                    background:var(--sempa-bg-panel); border-radius:28px;
                    padding:18px 18px; box-shadow:var(--sempa-shadow-1); overflow:hidden;">
          <div style="display:flex; align-items:center; gap:14px; padding:0 4px;">
            {@render ring(total ? doneCount / total : 0, 78, 9)}
            <div>
              <div style="font-size:15px; font-weight:800; letter-spacing:-.02em; color:var(--sempa-text);">Today's progress</div>
              <div style="font-size:12.5px; font-weight:500; color:var(--sempa-text-soft); margin-top:2px;">{active.length} left · {pct}% done</div>
            </div>
          </div>
          {#if objectiveTitle}
            <div style="padding:12px 16px; border-radius:18px; background:var(--sempa-accent-bg);">
              <div style="font-size:12px; font-weight:600; color:var(--sempa-accent-ink);">This week</div>
              <div style="font-size:15px; font-weight:700; color:var(--sempa-text); margin-top:3px;">{objectiveTitle}</div>
            </div>
          {/if}
          <div style="flex:1;"></div>
          {#if intention}
            <div style="padding:12px 16px; border-radius:18px; background:var(--sempa-fill);">
              <div style="font-size:12px; font-weight:600; color:var(--sempa-text-soft);">Intention</div>
              <div style="font-size:14.5px; font-style:italic; color:var(--sempa-text); line-height:1.45; margin-top:3px;">{intention}</div>
            </div>
          {/if}
        </div>
      </div>
      {@render timeline(170)}
    </div>
  {:else}
    <!-- Glance: big now card + summary + timeline -->
    <div style="flex:1; min-height:0; display:flex; flex-direction:column; gap:22px; padding:8px 40px 30px;">
      <div style="flex:1; min-height:0; display:flex; gap:28px;">
        <div style="flex:1.5; min-width:0;">{@render nowCard(true)}</div>
        <div style="flex:1; display:flex; align-items:center; gap:28px; background:var(--sempa-bg-panel);
                    border-radius:28px; padding:0 40px; box-shadow:var(--sempa-shadow-1);">
          {@render ring(total ? doneCount / total : 0, 150, 14)}
          <div style="min-width:0;">
            <div style="font-size:27px; font-weight:800; letter-spacing:-.03em; color:var(--sempa-text);">{active.length} tasks left</div>
            <div style="font-size:15px; font-weight:500; color:var(--sempa-text-soft); margin-top:4px;">{pct}% of today complete</div>
            {#if intention}
              <div style="margin-top:16px; padding:12px 18px; border-radius:18px; background:var(--sempa-fill); font-style:italic; font-size:15px; color:var(--sempa-text); line-height:1.45;">{intention}</div>
            {/if}
          </div>
        </div>
      </div>
      {@render timeline(186)}
    </div>
  {/if}
</div>
