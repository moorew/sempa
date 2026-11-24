<script lang="ts">
  import type { Project, Task } from '$lib/types';
  import { formatShortDate } from '$lib/utils';

  // Open projects drawn as bars across the desktop board's day columns, from
  // start date to target date, so a multi-week project is visible while you
  // plan days. It sits inside the board's horizontal scroller and uses the same
  // column track (15rem columns, 0.875rem gap = w-60 / gap-3.5), so each bar
  // lines up with its days without measuring anything.
  //
  // A project with no start runs from when it was created; one with no target
  // runs open-ended off the right edge. An open project past its target keeps
  // running to today, marked overdue, rather than vanishing from view.
  let {
    dates,
    projects,
    projectTasks,
    today,
    selectedId = null,
    onSelect,
  }: {
    dates: string[];                          // the board's contiguous column dates
    projects: Project[];                      // open projects only
    projectTasks: Record<string, Task[]>;     // top-level tasks per project
    today: string;
    selectedId?: string | null;
    onSelect: (id: string) => void;
  } = $props();

  function dayIndex(from: string, to: string): number {
    return Math.round(
      (new Date(to + 'T12:00:00').getTime() - new Date(from + 'T12:00:00').getTime()) / 86_400_000,
    );
  }

  interface Bar {
    p: Project;
    col: number;        // 1-based grid column
    span: number;
    lane: number;       // 1-based grid row
    clipStart: boolean; // starts before the first rendered column
    openEnd: boolean;   // no target date, or target past the last column
    overdue: boolean;
    endsHere: boolean;  // the target day is inside the range (draw the flag)
    toPlan: number;     // open tasks with no day yet
    open: number;       // open tasks in total
  }

  const bars = $derived.by<Bar[]>(() => {
    if (dates.length === 0) return [];
    const first = dates[0];
    const last = dates[dates.length - 1];
    const raw = [];
    for (const p of projects) {
      const start = p.start_date || p.created_at.slice(0, 10);
      const overdue = !!p.target_date && p.target_date < today;
      const end = p.target_date ? (overdue ? today : p.target_date) : last;
      if (end < first || start > last || end < start) continue;
      const s = start < first ? first : start;
      const e = end > last ? last : end;
      const ts = (projectTasks[p.id] ?? []).filter((t) => t.status !== 'done' && t.status !== 'cancelled');
      raw.push({
        p,
        col: dayIndex(first, s) + 1,
        span: dayIndex(s, e) + 1,
        clipStart: start < first,
        openEnd: !p.target_date || p.target_date > last,
        overdue,
        endsHere: !!p.target_date && !overdue && p.target_date >= first && p.target_date <= last,
        toPlan: ts.filter((t) => !t.planned_date).length,
        open: ts.length,
      });
    }
    // Greedy lane packing: longest-running first so short projects tuck under.
    raw.sort((a, b) => a.col - b.col || b.span - a.span);
    const laneEnds: number[] = [];
    return raw.map((b) => {
      let lane = laneEnds.findIndex((endCol) => endCol < b.col);
      if (lane === -1) { lane = laneEnds.length; laneEnds.push(0); }
      laneEnds[lane] = b.col + b.span - 1;
      return { ...b, lane: lane + 1 };
    });
  });

  function tip(b: Bar): string {
    const p = b.p;
    const range = [
      p.start_date ? `from ${formatShortDate(p.start_date)}` : '',
      p.target_date ? `due ${formatShortDate(p.target_date)}` : 'no target date',
    ].filter(Boolean).join(', ');
    const work = b.open === 0 ? 'no open tasks' : `${b.open} open task${b.open === 1 ? '' : 's'}${b.toPlan ? `, ${b.toPlan} not on a day yet` : ''}`;
    return `${p.title || 'Untitled project'} — ${range}${b.overdue ? ' (overdue)' : ''} · ${work}`;
  }
</script>

{#if bars.length}
  <div class="timeline shrink-0" style="grid-template-columns: repeat({dates.length}, 15rem);"
       aria-label="Projects on this timeline">
    {#each bars as b (b.p.id)}
      <button type="button"
              class="bar"
              class:clip-start={b.clipStart}
              class:open-end={b.openEnd}
              class:overdue={b.overdue}
              class:selected={selectedId === b.p.id}
              style="grid-column: {b.col} / span {b.span}; grid-row: {b.lane}; --pc: {b.p.color};"
              title={tip(b)}
              onclick={() => onSelect(b.p.id)}>
        <!-- Sticky so a long bar's label stays in view as the board scrolls. -->
        <span class="label">
          <span class="dot"></span>
          <span class="truncate">{b.p.title || 'Untitled project'}</span>
          {#if b.toPlan}
            <span class="count" title="{b.toPlan} not on a day yet">{b.toPlan} to plan</span>
          {/if}
          {#if b.overdue}<span class="count due">overdue</span>{/if}
        </span>
        {#if b.endsHere}<span class="flag" aria-hidden="true"></span>{/if}
      </button>
    {/each}
  </div>
{/if}

<style>
  .timeline {
    display: grid;
    width: max-content;
    column-gap: 0.875rem;
    row-gap: 4px;
    grid-auto-rows: 26px;
    margin-bottom: 10px;
  }
  .bar {
    position: relative;
    display: flex;
    align-items: center;
    min-width: 0;
    border-radius: 999px;
    background: color-mix(in srgb, var(--pc) 16%, var(--sempa-bg-panel));
    border: 1px solid color-mix(in srgb, var(--pc) 34%, transparent);
    color: var(--sempa-text);
    font-size: 12px;
    font-weight: 700;
    text-align: left;
    transition: background 0.15s, box-shadow 0.15s;
  }
  .bar:hover { background: color-mix(in srgb, var(--pc) 24%, var(--sempa-bg-panel)); }
  .bar.selected { box-shadow: 0 0 0 2px var(--pc); }
  /* Runs on past the rendered range: square + fade that end. */
  .bar.clip-start { border-top-left-radius: 4px; border-bottom-left-radius: 4px; border-left-style: dashed; }
  .bar.open-end   { border-top-right-radius: 4px; border-bottom-right-radius: 4px; border-right-style: dashed; }
  .bar.overdue    { border-color: color-mix(in srgb, #c0392b 55%, transparent); }
  .label {
    position: sticky;
    left: 0;
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    max-width: 100%;
    padding: 0 10px;
  }
  .bar:has(.flag) .label { padding-right: 22px; }
  .dot { width: 8px; height: 8px; border-radius: 2px; background: var(--pc); flex-shrink: 0; }
  .count {
    flex-shrink: 0;
    padding: 1px 7px;
    border-radius: 999px;
    font-size: 10.5px;
    font-weight: 700;
    background: color-mix(in srgb, var(--pc) 22%, transparent);
    color: var(--sempa-text-soft);
  }
  .count.due { background: color-mix(in srgb, #c0392b 18%, transparent); color: #c0392b; }
  /* Target-date flag on the bar's right end. */
  .flag {
    position: absolute;
    right: 8px;
    width: 8px;
    height: 8px;
    transform: rotate(45deg);
    border-radius: 1px;
    background: var(--pc);
  }
</style>
