<script lang="ts">
  /**
   * Floating sync-status widget (bottom-right). A permanent compact cloud icon
   * conveys state at a glance — plain cloud when synced, cloud-off when offline,
   * upload/alert variants for pending/error. The text label stays hidden to keep
   * the corner quiet, and fades in only when there's something to say: on hover,
   * while syncing/pending/offline/errored, or briefly after a sync completes.
   * Clicking syncs now (or shows the error). Only meaningful where a local DB
   * exists (desktop/Android); renders nothing on plain web.
   */
  import { syncStore, sync as runSync } from '$lib/sync.svelte';
  import { hasLocalDb } from '$lib/tauri/bridge';
  import { mobile } from '$lib/stores/mobile.svelte';
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import { RefreshCw, Cloud, CloudOff, CloudUpload, CloudAlert } from 'lucide-svelte';

  let nowMs = $state(Date.now());
  let hovered = $state(false);
  let flash = $state(false); // briefly reveal the label after a sync completes
  onMount(() => {
    const t = setInterval(() => { nowMs = Date.now(); }, 30_000);
    return () => clearInterval(t);
  });

  function ago(iso: string | null, now: number): string {
    if (!iso) return '';
    const secs = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 1000));
    if (secs < 60) return 'just now';
    const mins = Math.floor(secs / 60);
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    return `${Math.floor(hrs / 24)}d ago`;
  }

  const syncState = $derived(
    syncStore.syncing ? 'syncing'
    : syncStore.lastError ? 'error'
    : !syncStore.online ? 'offline'
    : syncStore.pending > 0 ? 'pending'
    : 'synced',
  );

  const label = $derived(
    syncState === 'syncing' ? 'Syncing…'
    : syncState === 'error' ? 'Sync error — tap'
    : syncState === 'offline' ? (syncStore.pending > 0 ? `Offline · ${syncStore.pending} pending` : 'Offline')
    : syncState === 'pending' ? `${syncStore.pending} to sync`
    : syncStore.lastSyncedAt ? `Synced ${ago(syncStore.lastSyncedAt, nowMs)}` : 'Synced',
  );

  const color = $derived(
    syncState === 'error' ? '#dc2626'
    : syncState === 'offline' ? 'var(--sempa-text-dim)'
    : syncState === 'pending' ? 'var(--sempa-accent)'
    : 'var(--sempa-text-soft)',
  );

  // Non-synced states keep the label visible (they need attention). The "synced"
  // resting state hides it — except for a short flash right after a sync.
  const persistent = $derived(
    syncState === 'syncing' || syncState === 'pending' || syncState === 'error' || syncState === 'offline',
  );
  const showLabel = $derived(hovered || persistent || flash);

  // Flash the label for ~2.5s when sync transitions to "synced" so the user gets
  // a moment of "synced just now" feedback, then it quietly fades away.
  let prev = syncState;
  let flashTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    const s = syncState;
    if (s !== prev) {
      const wasSyncing = prev === 'syncing';
      prev = s;
      if (s === 'synced' && wasSyncing) {
        flash = true;
        if (flashTimer) clearTimeout(flashTimer);
        flashTimer = setTimeout(() => { flash = false; }, 2500);
      }
    }
  });

  function onClick() {
    if (syncStore.lastError) {
      alert(`Sync error:\n\n${syncStore.lastError}`);
    } else {
      runSync();
    }
  }
</script>

{#if hasLocalDb()}
  <div class="fixed z-[60]"
       style="right: 16px; bottom: {mobile.value ? 'calc(env(safe-area-inset-bottom, 0px) + 92px)' : '16px'};">
    <button
      onclick={onClick}
      onmouseenter={() => (hovered = true)}
      onmouseleave={() => (hovered = false)}
      onfocus={() => (hovered = true)}
      onblur={() => (hovered = false)}
      disabled={syncStore.syncing}
      title={syncState === 'error'
        ? (syncStore.lastError ?? 'Sync error')
        : syncState === 'offline'
        ? 'No connection to your sempa server. Changes are saved locally and will sync when you reconnect.'
        : 'Click to sync now'}
      class="flex items-center gap-2 rounded-full px-2 py-2 shadow-md transition-colors"
      style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: {color};">
      <span class="flex h-4 w-4 items-center justify-center" class:spin={syncStore.syncing}>
        {#if syncState === 'syncing'}
          <RefreshCw size={15} strokeWidth={1.75} />
        {:else if syncState === 'error'}
          <CloudAlert size={15} strokeWidth={1.75} />
        {:else if syncState === 'offline'}
          <CloudOff size={15} strokeWidth={1.75} />
        {:else if syncState === 'pending'}
          <CloudUpload size={15} strokeWidth={1.75} />
        {:else}
          <Cloud size={15} strokeWidth={1.75} />
        {/if}
      </span>
      {#if showLabel}
        <span class="whitespace-nowrap pr-1 text-[11px]" transition:fade={{ duration: 140 }}>{label}</span>
      {/if}
    </button>
  </div>
{/if}

<style>
  .spin {
    display: inline-flex;
    animation: sync-spin 1s linear infinite;
  }
  @keyframes sync-spin {
    to { transform: rotate(360deg); }
  }
</style>
