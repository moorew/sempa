<script lang="ts">
  /**
   * Contextual display of a day's intention and/or reflection, shown inline on
   * the day view. Gated by the caller via prefs.contextualReflections. When a
   * field is empty and it's the relevant day, shows a gentle prompt that links
   * to the Plan Day / Shutdown ritual so the comments are actually reachable.
   */
  import { ClipboardCheck, Moon } from 'lucide-svelte';

  let {
    date,
    intention = null,
    reflection = null,
    show = 'both',
    promptWhenEmpty = false,
  }: {
    date: string;
    intention?: string | null;
    reflection?: string | null;
    show?: 'intention' | 'reflection' | 'both';
    /** Show a "set an intention / add a reflection" link when the field is empty. */
    promptWhenEmpty?: boolean;
  } = $props();

  const showIntention = $derived(show === 'both' || show === 'intention');
  const showReflection = $derived(show === 'both' || show === 'reflection');

  const hasIntention = $derived(!!intention?.trim());
  const hasReflection = $derived(!!reflection?.trim());

  // Only render at all if there's something to show or a prompt to offer.
  const visible = $derived(
    (showIntention && (hasIntention || promptWhenEmpty)) ||
    (showReflection && (hasReflection || promptWhenEmpty))
  );
</script>

{#if visible}
  <div class="flex flex-col gap-2">
    {#if showIntention}
      {#if hasIntention}
        <div class="px-4 py-3.5" style="background: var(--sempa-accent-bg); border-radius: 18px;">
          <p class="flex items-center gap-1.5 type-label" style="color: var(--sempa-accent-ink);">
            <ClipboardCheck size={13} strokeWidth={2.25} />
            Today's intention
          </p>
          <p class="mt-1 leading-relaxed" style="font-size: 14.5px; font-weight: 600; color: var(--sempa-text);">{intention}</p>
        </div>
      {:else if promptWhenEmpty}
        <a href="/plan/{date}" class="rc-prompt">
          <span class="rc-prompt-icon" style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
            <ClipboardCheck size={15} strokeWidth={2} />
          </span>
          Set an intention for the day
        </a>
      {/if}
    {/if}

    {#if showReflection}
      {#if hasReflection}
        <div class="px-4 py-3.5" style="background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1); border-radius: 18px;">
          <p class="flex items-center gap-1.5 type-label" style="color: var(--sempa-text-soft);">
            <Moon size={13} strokeWidth={2.25} />
            Reflection
          </p>
          <p class="mt-1 whitespace-pre-line text-sm leading-relaxed" style="color: var(--sempa-text);">{reflection}</p>
        </div>
      {:else if promptWhenEmpty}
        <a href="/shutdown/{date}" class="rc-prompt">
          <span class="rc-prompt-icon" style="background: var(--sempa-fill); color: var(--sempa-text-soft);">
            <Moon size={15} strokeWidth={2} />
          </span>
          Add an end-of-day reflection
        </a>
      {/if}
    {/if}
  </div>
{/if}

<style>
  /* Empty-state prompt: a quiet surface pill-row that links to the ritual. */
  .rc-prompt {
    display: flex; align-items: center; gap: 10px;
    min-height: 48px; padding: 6px 16px 6px 8px; border-radius: 18px;
    background: var(--sempa-bg-panel); box-shadow: var(--sempa-shadow-1);
    font-size: 14px; font-weight: 600; color: var(--sempa-text-soft);
    transition: color 120ms ease, opacity 120ms ease;
  }
  .rc-prompt:hover { color: var(--sempa-text); }
  .rc-prompt:active { opacity: .75; }
  .rc-prompt-icon {
    display: inline-grid; place-items: center; flex: 0 0 auto;
    width: 32px; height: 32px; border-radius: 11px;
  }
</style>
