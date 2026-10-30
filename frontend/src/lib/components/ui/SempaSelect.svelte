<script lang="ts">
  /**
   * SempaSelect — a fully custom, Sempa-styled dropdown.
   *
   * Native <select> elements render as off-brand Android system pickers, so we
   * replace them with this component. On mobile it opens as a bottom sheet with
   * large touch targets; on desktop it opens as an inline absolute dropdown that
   * sits *below* the top nav (z-dropdown < z-top-nav) so it can never cover it.
   *
   * No native <select> is rendered anywhere in the DOM.
   */
  import { mobile } from '$lib/stores/mobile.svelte';
  import { Check, ChevronDown } from 'lucide-svelte';
  import { tick } from 'svelte';

  type OptionValue = string | number | null;
  interface Option {
    value: OptionValue;
    label: string;
    icon?: string;
    color?: string; // renders a small colour dot (e.g. a project's colour)
  }

  let {
    value = $bindable(),
    options,
    placeholder = 'Select…',
    id,
    onchange,
    scrollToValue,
  }: {
    value: OptionValue;
    options: Option[];
    placeholder?: string;
    id?: string;
    onchange?: (value: OptionValue) => void;
    // When nothing is selected yet, scroll the open list to roughly this value
    // (e.g. "now" for a time picker) instead of pinning to the top (midnight).
    scrollToValue?: OptionValue;
  } = $props();

  let open = $state(false);
  let rootEl = $state<HTMLElement | undefined>();
  let scrollEl = $state<HTMLElement | undefined>();

  const selected = $derived(options.find((o) => o.value === value));
  const hasValue = $derived(selected !== undefined && selected.value !== null && selected.value !== '');

  function choose(opt: Option) {
    value = opt.value;
    onchange?.(opt.value);
    open = false;
  }

  // Close on Escape and on outside click/tap (desktop). The mobile scrim handles
  // outside taps for the bottom sheet.
  $effect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') { open = false; }
    }
    function onDown(e: Event) {
      if (rootEl && !rootEl.contains(e.target as Node)) open = false;
    }
    window.addEventListener('keydown', onKey);
    // capture phase so we see the tap before it's swallowed by other handlers
    window.addEventListener('pointerdown', onDown, true);
    return () => {
      window.removeEventListener('keydown', onKey);
      window.removeEventListener('pointerdown', onDown, true);
    };
  });

  // On open, centre the selected option in view (or `scrollToValue` when nothing
  // is selected yet) so a long list — e.g. a 48-slot time picker — lands on the
  // relevant row rather than pinned to the top (which reads as "defaults to 12am").
  $effect(() => {
    if (!open) return;
    const target = hasValue ? value : scrollToValue;
    if (target === undefined || target === null || target === '') return;
    const idx = options.findIndex((o) => o.value === target);
    if (idx < 0) return;
    tick().then(() => {
      const container = scrollEl;
      if (!container) return;
      const el = container.querySelectorAll<HTMLElement>('[role="option"]')[idx];
      if (!el) return;
      const cr = container.getBoundingClientRect();
      const er = el.getBoundingClientRect();
      container.scrollTop += (er.top - cr.top) - (container.clientHeight - el.clientHeight) / 2;
    });
  });
</script>

<div bind:this={rootEl} style="position: relative;">
  <!-- Trigger -->
  <button
    {id}
    type="button"
    onclick={() => (open = !open)}
    aria-haspopup="listbox"
    aria-expanded={open}
    class="sempa-select-trigger flex w-full items-center justify-between gap-2 px-3.5 text-sm outline-none"
    data-open={open}
    style="color: {hasValue ? 'var(--sempa-text)' : 'var(--sempa-text-dim)'};">
    <span class="flex min-w-0 items-center gap-2 truncate">
      {#if selected?.icon}<span class="shrink-0">{selected.icon}</span>{/if}{#if selected?.color}<span class="h-2.5 w-2.5 shrink-0 rounded-full" style="background: {selected.color};"></span>{/if}
      <span class="truncate" style="font-weight: {hasValue ? 600 : 500};">{selected ? selected.label : placeholder}</span>
    </span>
    <span class="shrink-0 transition-transform" style="transform: rotate({open ? 180 : 0}deg); color: var(--sempa-text-dim);">
      <ChevronDown size={16} strokeWidth={2.2} />
    </span>
  </button>

  {#if open}
    {#if mobile.value}
      <!-- Mobile: bottom sheet -->
      <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
      <div class="sempa-select-scrim" role="presentation" onclick={() => (open = false)}></div>
      <div class="sempa-select-sheet" role="listbox" aria-label={placeholder}>
        <!-- Drag handle pill -->
        <div class="flex justify-center pt-3 pb-2">
          <div class="h-1.5 w-10 rounded-full" style="background: var(--sempa-track);"></div>
        </div>
        <div bind:this={scrollEl} class="overflow-y-auto px-2" style="max-height: 60vh;">
          {#each options as opt}
            {@const isSel = opt.value === value}
            <button
              type="button"
              role="option"
              aria-selected={isSel}
              onclick={() => choose(opt)}
              class="sempa-select-row flex w-full items-center gap-3 px-4 text-left text-[15px]"
              style="min-height: 52px; border-radius: 14px;
                     color: {isSel ? 'var(--sempa-accent-ink)' : 'var(--sempa-text)'};
                     font-weight: {isSel ? 700 : 500};
                     {isSel ? 'background: var(--sempa-accent-bg);' : ''}">
              {#if isSel}
                <span class="flex shrink-0 items-center justify-center rounded-full"
                      style="width: 22px; height: 22px; background: var(--sempa-btn-bg); color: var(--sempa-btn-fg);">
                  <Check size={14} strokeWidth={3} />
                </span>
              {:else}
                <span class="shrink-0 rounded-full" style="width: 22px; height: 22px; border: 2px solid var(--sempa-track);"></span>
              {/if}
              {#if opt.icon}<span class="shrink-0">{opt.icon}</span>{/if}{#if opt.color}<span class="h-2.5 w-2.5 shrink-0 rounded-full" style="background: {opt.color};"></span>{/if}
              <span class="truncate">{opt.label}</span>
            </button>
          {/each}
        </div>
      </div>
    {:else}
      <!-- Desktop: inline absolute dropdown (below top nav) -->
      <div
        bind:this={scrollEl}
        class="sempa-select-menu"
        role="listbox"
        aria-label={placeholder}>
        {#each options as opt}
          {@const isSel = opt.value === value}
          <button
            type="button"
            role="option"
            aria-selected={isSel}
            onclick={() => choose(opt)}
            class="sempa-select-row flex w-full items-center gap-2 px-2.5 py-2 text-left text-sm"
            style="color: {isSel ? 'var(--sempa-accent-ink)' : 'var(--sempa-text)'}; font-weight: {isSel ? 700 : 500};
                   {isSel ? 'background: var(--sempa-accent-bg);' : ''}">
            <span class="flex h-4 w-4 shrink-0 items-center justify-center" style="color: var(--sempa-accent);">
              {#if isSel}<Check size={15} strokeWidth={2.5} />{/if}
            </span>
            {#if opt.icon}<span class="shrink-0">{opt.icon}</span>{/if}{#if opt.color}<span class="h-2.5 w-2.5 shrink-0 rounded-full" style="background: {opt.color};"></span>{/if}
            <span class="truncate">{opt.label}</span>
          </button>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  /* Trigger — a filled field (radius 14) with the accent focus ring. */
  .sempa-select-trigger {
    min-height: 40px;
    border-radius: 14px;
    background: var(--sempa-fill);
    border: 1px solid var(--sempa-border);
    transition: border-color 120ms ease, box-shadow 120ms ease;
  }
  .sempa-select-trigger:hover { border-color: color-mix(in srgb, var(--sempa-accent) 35%, var(--sempa-border)); }
  .sempa-select-trigger[data-open="true"],
  .sempa-select-trigger:focus-visible {
    outline: none;
    border-color: var(--sempa-accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--sempa-accent) 18%, transparent);
  }
  @media (pointer: coarse) { .sempa-select-trigger { min-height: 44px; } }

  /* Desktop inline dropdown — absolute, BELOW the top nav (z-dropdown < z-top-nav).
     A raised popover: radius 16, shadow-2, rows inset with their own rounding. */
  .sempa-select-menu {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    right: 0;
    min-width: 160px;
    z-index: var(--z-dropdown, 30);
    max-height: 16rem;
    overflow-y: auto;
    padding: 6px;
    border-radius: 16px;
    background: var(--sempa-bg-panel);
    box-shadow: var(--sempa-shadow-2);
    animation: sempa-fade-in 140ms ease both;
  }
  .sempa-select-row {
    border-radius: 10px;
    transition: background-color 120ms ease;
  }
  .sempa-select-row:hover {
    background: var(--sempa-fill);
  }
  .sempa-select-row[aria-selected="true"]:hover {
    background: var(--sempa-accent-bg);
  }

  /* Mobile bottom sheet — intentionally above everything (z-sheet). */
  .sempa-select-scrim {
    position: fixed;
    inset: 0;
    z-index: var(--z-scrim, 89);
    background: rgba(0, 0, 0, 0.4);
    animation: sempa-fade-in 160ms ease both;
  }
  .sempa-select-sheet {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: var(--z-sheet, 90);
    border-radius: 28px 28px 0 0;
    background: var(--sempa-bg-panel);
    box-shadow: var(--sempa-shadow-2);
    padding-bottom: max(16px, env(safe-area-inset-bottom, 16px));
    animation: sempa-sheet-up 280ms ease-out both;
  }

  @keyframes sempa-sheet-up {
    from { transform: translateY(100%); }
    to   { transform: translateY(0); }
  }
  @media (prefers-reduced-motion: reduce) {
    .sempa-select-menu, .sempa-select-scrim, .sempa-select-sheet { animation: none; }
  }
</style>
