<script module lang="ts">
  import { api } from '$lib/api';
  import type { LinkUnfurl } from '$lib/types';

  // Dedupe + cache unfurl requests for the session so the same URL shown in
  // several places (or re-rendered) only hits the API once. The backend caches
  // too; this just avoids redundant round-trips within a page.
  const cache = new Map<string, Promise<LinkUnfurl>>();
  export function unfurlCached(url: string): Promise<LinkUnfurl> {
    let p = cache.get(url);
    if (!p) {
      p = api.unfurl(url);
      cache.set(url, p);
    }
    return p;
  }
</script>

<script lang="ts">
  import { prettyUrl } from '$lib/utils';

  let { url }: { url: string } = $props();

  // Hero (og:image) and favicon each load through the server proxy so they work
  // regardless of mixed-content/hotlink/CORS. Either can still fail (404, not an
  // image, offline) — we degrade gracefully each time.
  let heroFailed = $state(false);
  let favFailed = $state(false);

  const host = $derived.by(() => {
    try { return new URL(url).hostname.replace(/^www\./, ''); } catch { return url; }
  });
  const fallbackLabel = $derived.by(() => {
    try { return prettyUrl(new URL(url)); } catch { return url; }
  });
  // First letter of the host for the last-resort monogram tile.
  const monogram = $derived((host.replace(/^[^a-z0-9]+/i, '')[0] || '?').toUpperCase());
  const clamp2 = 'display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;';
</script>

<a href={url} target="_blank" rel="noopener noreferrer"
   onclick={(e) => e.stopPropagation()}
   class="block overflow-hidden rounded-xl no-underline"
   style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-main);">
  {#await unfurlCached(url)}
    <!-- Skeleton while fetching -->
    <div class="flex items-center gap-2.5 px-3 py-3">
      <div class="h-9 w-9 shrink-0 rounded-lg" style="background: var(--sempa-border); opacity:.5;"></div>
      <div class="flex-1 space-y-1.5">
        <div class="h-2.5 w-1/2 rounded" style="background: var(--sempa-border); opacity:.5;"></div>
        <div class="h-2.5 w-3/4 rounded" style="background: var(--sempa-border); opacity:.35;"></div>
      </div>
    </div>
  {:then data}
    {@const hasHero = !!data.image_url && !heroFailed}
    {#if hasHero}
      <!-- Big thumbnail on top (proxied), hides itself if it fails to load. -->
      <img src={api.unfurlImageUrl(data.image_url)} alt="" loading="lazy"
           class="w-full object-cover" style="max-height: 150px; background: var(--sempa-bg-panel);"
           onerror={() => (heroFailed = true)} />
    {/if}
    <div class="flex items-stretch gap-3 px-3 py-2.5">
      {#if !hasHero}
        <!-- No usable hero image → favicon / monogram tile so the card still
             reads as a designed link card rather than bare text. -->
        <div class="flex h-11 w-11 shrink-0 items-center justify-center self-center overflow-hidden rounded-lg"
             style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border);">
          {#if data.favicon_url && !favFailed}
            <img src={api.unfurlImageUrl(data.favicon_url)} alt="" class="h-5 w-5"
                 onerror={() => (favFailed = true)} />
          {:else}
            <span class="text-base font-semibold" style="color: var(--sempa-accent);">{monogram}</span>
          {/if}
        </div>
      {/if}
      <div class="min-w-0 flex-1">
        <div class="mb-0.5 flex items-center gap-1.5">
          {#if hasHero && data.favicon_url && !favFailed}
            <img src={api.unfurlImageUrl(data.favicon_url)} alt="" class="h-3.5 w-3.5 shrink-0 rounded-sm"
                 onerror={() => (favFailed = true)} />
          {/if}
          <span class="truncate text-[11px]" style="color: var(--sempa-text-dim);">{data.site_name || host}</span>
        </div>
        <p class="text-sm font-medium leading-snug" style="color: var(--sempa-text); {clamp2}">
          {data.ok && data.title ? data.title : fallbackLabel}
        </p>
        {#if data.description}
          <p class="mt-0.5 text-xs leading-snug" style="color: var(--sempa-text-soft); {clamp2}">{data.description}</p>
        {/if}
      </div>
    </div>
  {:catch}
    <!-- Unfurl request itself failed → minimal chip so the link is still
         visible and tappable. -->
    <div class="flex items-center gap-2 px-3 py-2.5">
      <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg text-xs font-semibold"
            style="background: var(--sempa-bg-panel); border: 1px solid var(--sempa-border); color: var(--sempa-accent);">{monogram}</span>
      <span class="truncate text-[13px]" style="color: var(--sempa-accent);">{fallbackLabel}</span>
    </div>
  {/await}
</a>
