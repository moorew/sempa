<script lang="ts">
  import type { TaskStatus } from '$lib/types';
  import { tagStore } from '$lib/stores/tags.svelte';

  const TIME_OPTIONS = [
    { label: 'No estimate',  value: null },
    { label: '15 min',       value: 15   },
    { label: '30 min',       value: 30   },
    { label: '45 min',       value: 45   },
    { label: '1 hour',       value: 60   },
    { label: '1.5 hours',    value: 90   },
    { label: '2 hours',      value: 120  },
    { label: '2.5 hours',    value: 150  },
    { label: '3 hours',      value: 180  },
    { label: '4 hours',      value: 240  },
    { label: '6 hours',      value: 360  },
    { label: '8 hours',      value: 480  },
  ];

  const DAYS = ['Sunday','Monday','Tuesday','Wednesday','Thursday','Friday','Saturday'];
  const ordinal = (n: number) => {
    const s = ['th','st','nd','rd'], v = n % 100;
    return s[(v - 20) % 10] || s[v] || s[0];
  };

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
      description: string | null;
      status: TaskStatus;
      estimateMinutes: number | null;
      tags: string[];
      recurrenceRule: string | null;
      plannedDate: string | null;
    }) => void;
    onClose: () => void;
  } = $props();

  // Form state
  let title = $state('');
  let description = $state('');
  let plannedDate = $state('');
  let estimateMinutes = $state<number | null>(null);
  let recurrenceRule = $state('');
  let selectedTags = $state<string[]>([]);
  let tagSearch = $state('');
  let tagDropdownOpen = $state(false);
  let titleInput: HTMLInputElement | undefined = $state();

  const recurrenceOptions = $derived.by(() => {
    const d = new Date((plannedDate || defaultDate) + 'T12:00:00');
    const wd = d.getDay(), dom = d.getDate();
    return [
      { value: '',                 label: "Doesn't repeat" },
      { value: 'daily',            label: 'Every day' },
      { value: 'weekdays',         label: 'Every weekday (Mon–Fri)' },
      { value: `weekly:${wd}`,     label: `Weekly on ${DAYS[wd]}` },
      { value: `monthly:${dom}`,   label: `Monthly on the ${dom}${ordinal(dom)}` },
    ];
  });

  $effect(() => {
    if (open) {
      title = ''; description = ''; plannedDate = defaultDate;
      estimateMinutes = null; recurrenceRule = ''; selectedTags = []; tagSearch = '';
      tagDropdownOpen = false;
      setTimeout(() => titleInput?.focus(), 30);
    }
  });

  // Tags
  const filteredTags = $derived(
    tagStore.definitions.filter(t =>
      !selectedTags.includes(t.name) &&
      t.name.toLowerCase().includes(tagSearch.toLowerCase())
    )
  );

  function toggleTag(name: string) {
    if (selectedTags.includes(name)) {
      selectedTags = selectedTags.filter(t => t !== name);
    } else {
      selectedTags = [...selectedTags, name];
    }
    tagSearch = '';
  }

  function removeTag(name: string) { selectedTags = selectedTags.filter(t => t !== name); }

  function handleTagKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') { tagDropdownOpen = false; return; }
    if (e.key === 'Enter' && tagSearch.trim()) {
      e.preventDefault();
      const match = filteredTags[0];
      if (match) toggleTag(match.name);
      else if (tagSearch.trim()) { toggleTag(tagSearch.trim().toLowerCase()); }
    }
    if (e.key === 'Backspace' && tagSearch === '' && selectedTags.length) {
      selectedTags = selectedTags.slice(0, -1);
    }
  }

  function handleSubmit() {
    if (!title.trim()) return;
    onSubmit({
      title: title.trim(),
      description: description.trim() || null,
      status: defaultStatus,
      estimateMinutes,
      tags: selectedTags,
      recurrenceRule: recurrenceRule || null,
      plannedDate: recurrenceRule ? null : (plannedDate || null),
    });
  }
</script>

{#if open}
  <!-- Backdrop -->
  <div role="presentation"
       class="fixed inset-0 z-40 bg-black/30 backdrop-blur-sm dark:bg-black/50"
       onclick={onClose}></div>

  <!-- Panel — slides in from the right -->
  <aside role="dialog" aria-modal="true" aria-label="Add task"
         class="fixed right-0 top-0 z-50 flex h-full w-full max-w-md flex-col
                border-l border-gray-200 bg-white shadow-2xl
                dark:border-gray-700 dark:bg-gray-900">

    <!-- Header -->
    <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-gray-800">
      <h2 class="text-sm font-semibold text-gray-800 dark:text-gray-100">New task</h2>
      <button onclick={onClose}
              class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 transition-colors
                     dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-300">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>

    <!-- Body — scrollable -->
    <div class="flex-1 overflow-y-auto px-5 py-4 space-y-4">

      <!-- Title -->
      <div>
        <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-title">
          Title <span class="text-red-400">*</span>
        </label>
        <input
          id="task-title"
          bind:this={titleInput}
          bind:value={title}
          onkeydown={(e) => e.key === 'Escape' && onClose()}
          type="text"
          placeholder="What needs to get done?"
          class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
                 text-gray-800 placeholder-gray-400 outline-none
                 focus:border-blue-400 focus:bg-white focus:ring-2 focus:ring-blue-100
                 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-600
                 dark:focus:border-blue-500 dark:focus:bg-gray-800 dark:focus:ring-blue-900/40"
        />
      </div>

      <!-- Notes / Description -->
      <div>
        <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-notes">
          Notes <span class="text-xs font-normal text-gray-400 dark:text-gray-600">— markdown supported</span>
        </label>
        <textarea
          id="task-notes"
          bind:value={description}
          rows="4"
          placeholder="Add details, links, or context...&#10;&#10;Supports **bold**, _italic_, [links](https://...)"
          class="w-full resize-none rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
                 text-gray-800 placeholder-gray-400 outline-none leading-relaxed
                 focus:border-blue-400 focus:bg-white focus:ring-2 focus:ring-blue-100
                 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-600
                 dark:focus:border-blue-500 dark:focus:bg-gray-800"
        ></textarea>
      </div>

      <!-- Due date + Time estimate (side by side) -->
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-date">
            Due date
          </label>
          <input
            id="task-date"
            type="date"
            bind:value={plannedDate}
            disabled={!!recurrenceRule}
            class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
                   text-gray-800 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100
                   disabled:opacity-50 disabled:cursor-not-allowed
                   dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-blue-500
                   dark:disabled:opacity-30 [color-scheme:light] dark:[color-scheme:dark]"
          />
          {#if recurrenceRule}
            <p class="mt-1 text-xs text-gray-400 dark:text-gray-600">Set by recurrence</p>
          {/if}
        </div>

        <div>
          <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-estimate">
            Time estimate
          </label>
          <select
            id="task-estimate"
            bind:value={estimateMinutes}
            class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
                   text-gray-800 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100
                   dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-blue-500">
            {#each TIME_OPTIONS as opt}
              <option value={opt.value}>{opt.label}</option>
            {/each}
          </select>
        </div>
      </div>

      <!-- Tags -->
      <div>
        <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400">Tags</label>

        <!-- Selected chips + search input -->
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="flex min-h-[42px] flex-wrap gap-1.5 items-center rounded-lg border border-gray-200 bg-gray-50 px-3 py-2
                    focus-within:border-blue-400 focus-within:ring-2 focus-within:ring-blue-100
                    dark:border-gray-700 dark:bg-gray-800 dark:focus-within:border-blue-500"
             onclick={() => { tagDropdownOpen = true; }}>
          {#each selectedTags as tag}
            <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium text-white shrink-0"
                  style="background-color: {tagStore.colorFor(tag)}">
              {tag}
              <button type="button" onclick={(e) => { e.stopPropagation(); removeTag(tag); }}
                      class="opacity-75 hover:opacity-100 leading-none ml-0.5">×</button>
            </span>
          {/each}
          <input
            bind:value={tagSearch}
            onfocus={() => (tagDropdownOpen = true)}
            onblur={() => setTimeout(() => (tagDropdownOpen = false), 150)}
            onkeydown={handleTagKeydown}
            type="text"
            placeholder={selectedTags.length ? '' : 'Search or add tags…'}
            class="flex-1 min-w-[80px] bg-transparent text-sm text-gray-700 placeholder-gray-400 outline-none
                   dark:text-gray-200 dark:placeholder-gray-600"
          />
        </div>

        <!-- Dropdown -->
        {#if tagDropdownOpen}
          <div class="relative z-10">
            <div class="absolute top-1 left-0 right-0 rounded-lg border border-gray-200 bg-white shadow-lg
                        dark:border-gray-700 dark:bg-gray-800 max-h-44 overflow-y-auto">
              {#if filteredTags.length}
                {#each filteredTags as tag}
                  <button type="button"
                          onmousedown={(e) => { e.preventDefault(); toggleTag(tag.name); }}
                          class="flex w-full items-center gap-2.5 px-3 py-2 text-sm text-left
                                 hover:bg-gray-50 transition-colors dark:hover:bg-gray-700">
                    <span class="h-3 w-3 rounded-full shrink-0" style="background-color: {tag.color}"></span>
                    <span class="text-gray-700 dark:text-gray-200">{tag.name}</span>
                  </button>
                {/each}
              {:else if tagSearch.trim()}
                <button type="button"
                        onmousedown={(e) => { e.preventDefault(); toggleTag(tagSearch.trim().toLowerCase()); tagSearch = ''; }}
                        class="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-500
                               hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-700">
                  <span class="text-blue-500">+</span> Create tag "<strong>{tagSearch.trim()}</strong>"
                </button>
              {:else}
                <p class="px-3 py-2 text-sm text-gray-400 dark:text-gray-600">No tags yet — type to create one</p>
              {/if}
            </div>
          </div>
        {/if}
      </div>

      <!-- Recurrence -->
      <div>
        <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400" for="task-recurrence">
          Repeat
        </label>
        <select
          id="task-recurrence"
          bind:value={recurrenceRule}
          class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 text-sm
                 text-gray-700 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100
                 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:focus:border-blue-500">
          {#each recurrenceOptions as opt}
            <option value={opt.value}>{opt.label}</option>
          {/each}
        </select>
        {#if recurrenceRule}
          <p class="mt-1.5 flex items-center gap-1 text-xs text-violet-600 dark:text-violet-400">
            <span>↺</span> Creates a recurring template — first instance generates for today
          </p>
        {/if}
      </div>

    </div>

    <!-- Footer -->
    <div class="flex items-center justify-end gap-2 border-t border-gray-100 px-5 py-4 dark:border-gray-800">
      <button onclick={onClose}
              class="rounded-lg px-4 py-2 text-sm text-gray-500 hover:bg-gray-100 transition-colors
                     dark:text-gray-400 dark:hover:bg-gray-800">
        Cancel
      </button>
      <button onclick={handleSubmit}
              disabled={!title.trim()}
              class="rounded-lg bg-blue-500 px-5 py-2 text-sm font-medium text-white
                     hover:bg-blue-600 disabled:opacity-40 disabled:cursor-not-allowed transition-colors">
        {recurrenceRule ? 'Create recurring' : 'Add task'}
      </button>
    </div>

  </aside>
{/if}
