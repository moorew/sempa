<script lang="ts">
  import type { TaskStatus } from '$lib/types';
  import { tagStore } from '$lib/stores/tags.svelte';

  let {
    open,
    defaultStatus,
    defaultDate,
    onSubmit,
    onClose,
  }: {
    open: boolean;
    defaultStatus: TaskStatus;
    defaultDate: string;
    onSubmit: (params: {
      title: string;
      status: TaskStatus;
      estimateMinutes: number | null;
      tags: string[];
      recurrenceRule: string | null;
    }) => void;
    onClose: () => void;
  } = $props();

  let title = $state('');
  let estimateRaw = $state('');
  let recurrenceRule = $state('');
  let tagInput = $state('');
  let selectedTags = $state<string[]>([]);
  let titleInput: HTMLInputElement | undefined = $state();

  // Weekday number for "weekly on X" option
  const DAYS = ['Sunday','Monday','Tuesday','Wednesday','Thursday','Friday','Saturday'];

  const recurrenceOptions = $derived.by(() => {
    const d = new Date(defaultDate + 'T12:00:00');
    const wd = d.getDay();
    const dom = d.getDate();
    return [
      { value: '',           label: "Doesn't repeat" },
      { value: 'daily',      label: 'Every day' },
      { value: 'weekdays',   label: 'Every weekday (Mon–Fri)' },
      { value: `weekly:${wd}`, label: `Weekly on ${DAYS[wd]}` },
      { value: `monthly:${dom}`, label: `Monthly on the ${dom}${ordinal(dom)}` },
    ];
  });

  function ordinal(n: number) {
    const s = ['th','st','nd','rd'];
    const v = n % 100;
    return s[(v - 20) % 10] || s[v] || s[0];
  }

  $effect(() => {
    if (open) {
      title = ''; estimateRaw = ''; recurrenceRule = ''; tagInput = ''; selectedTags = [];
      setTimeout(() => titleInput?.focus(), 0);
    }
  });

  function parseMinutes(raw: string): number | null {
    const t = raw.trim().toLowerCase();
    if (!t) return null;
    const h = t.match(/^(\d+(?:\.\d+)?)\s*h$/);
    if (h) return Math.round(parseFloat(h[1]) * 60);
    const m = t.match(/^(\d+)\s*m?$/);
    if (m) return parseInt(m[1], 10);
    return null;
  }

  function addTag(name: string) {
    const n = name.trim().toLowerCase().replace(/^#/, '');
    if (n && !selectedTags.includes(n)) selectedTags = [...selectedTags, n];
    tagInput = '';
  }

  function removeTag(name: string) {
    selectedTags = selectedTags.filter(t => t !== name);
  }

  function handleTagKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ',' || e.key === ' ') {
      e.preventDefault();
      addTag(tagInput);
    }
    if (e.key === 'Backspace' && tagInput === '' && selectedTags.length) {
      selectedTags = selectedTags.slice(0, -1);
    }
  }

  function handleSubmit() {
    if (!title.trim()) return;
    onSubmit({
      title: title.trim(),
      status: defaultStatus,
      estimateMinutes: parseMinutes(estimateRaw),
      tags: selectedTags,
      recurrenceRule: recurrenceRule || null,
    });
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && e.target === titleInput) handleSubmit();
    if (e.key === 'Escape') onClose();
  }

  const existingTags = $derived(tagStore.definitions.map(t => t.name));
  const tagSuggestions = $derived(
    tagInput.trim()
      ? existingTags.filter(t =>
          t.toLowerCase().startsWith(tagInput.toLowerCase().replace(/^#/, '')) &&
          !selectedTags.includes(t)
        ).slice(0, 5)
      : existingTags.filter(t => !selectedTags.includes(t)).slice(0, 8)
  );
</script>

{#if open}
  <div role="presentation"
       class="fixed inset-0 z-40 bg-black/30 backdrop-blur-sm dark:bg-black/50"
       onclick={onClose}></div>

  <div role="dialog" aria-modal="true" aria-label="Add task"
       class="fixed left-1/2 top-1/3 z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2
              rounded-2xl border border-gray-200 bg-white shadow-2xl p-5
              dark:border-gray-700 dark:bg-gray-800">
    <h2 class="mb-4 text-sm font-semibold text-gray-700 dark:text-gray-200">New task</h2>

    <!-- Title -->
    <input
      bind:this={titleInput}
      bind:value={title}
      onkeydown={handleKeydown}
      type="text"
      placeholder="What needs to get done?"
      class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
             text-gray-800 placeholder-gray-400 outline-none
             focus:border-blue-400 focus:bg-white focus:ring-2 focus:ring-blue-100
             dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 dark:placeholder-gray-500
             dark:focus:border-blue-500 dark:focus:bg-gray-700 dark:focus:ring-blue-900"
    />

    <!-- Time estimate -->
    <input
      bind:value={estimateRaw}
      onkeydown={(e) => e.key === 'Escape' && onClose()}
      type="text"
      placeholder="Time estimate — 30m, 1h, 90 (mins)"
      class="mt-2 w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
             text-gray-800 placeholder-gray-400 outline-none
             focus:border-blue-400 focus:bg-white focus:ring-2 focus:ring-blue-100
             dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 dark:placeholder-gray-500
             dark:focus:border-blue-500 dark:focus:bg-gray-700"
    />

    <!-- Tags -->
    <div class="mt-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2
                dark:border-gray-600 dark:bg-gray-700">
      <div class="flex flex-wrap gap-1.5 items-center">
        {#each selectedTags as tag}
          <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium text-white"
                style="background-color: {tagStore.colorFor(tag)}">
            {tag}
            <button type="button" onclick={() => removeTag(tag)}
                    class="opacity-80 hover:opacity-100 leading-none">×</button>
          </span>
        {/each}
        <input
          bind:value={tagInput}
          onkeydown={handleTagKeydown}
          type="text"
          placeholder={selectedTags.length ? '' : 'Add tags…'}
          class="flex-1 min-w-16 bg-transparent text-sm text-gray-700 placeholder-gray-400 outline-none
                 dark:text-gray-200 dark:placeholder-gray-500"
        />
      </div>
      {#if tagSuggestions.length && tagInput.length >= 0}
        <div class="mt-1.5 flex flex-wrap gap-1">
          {#each tagSuggestions as suggestion}
            <button type="button"
                    onclick={() => addTag(suggestion)}
                    class="rounded-full border border-gray-200 px-2 py-0.5 text-xs text-gray-500
                           hover:border-gray-300 hover:bg-white transition-colors
                           dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-600">
              {suggestion}
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Recurrence -->
    <select
      bind:value={recurrenceRule}
      class="mt-2 w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
             text-gray-700 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100
             dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 dark:focus:border-blue-500">
      {#each recurrenceOptions as opt}
        <option value={opt.value}>{opt.label}</option>
      {/each}
    </select>

    {#if recurrenceRule}
      <p class="mt-1.5 text-xs text-violet-600 dark:text-violet-400">
        ↺ This will become a recurring task template
      </p>
    {/if}

    <div class="mt-4 flex justify-end gap-2">
      <button onclick={onClose}
              class="rounded-lg px-4 py-2 text-sm text-gray-500 hover:bg-gray-100 transition-colors
                     dark:text-gray-400 dark:hover:bg-gray-700">
        Cancel
      </button>
      <button onclick={handleSubmit}
              disabled={!title.trim()}
              class="rounded-lg bg-blue-500 px-4 py-2 text-sm font-medium text-white
                     hover:bg-blue-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors">
        {recurrenceRule ? 'Create recurring' : 'Add task'}
      </button>
    </div>
  </div>
{/if}
