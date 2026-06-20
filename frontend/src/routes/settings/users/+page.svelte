<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { AppUser } from '$lib/types';
  import { Users, Trash2, KeyRound, Shield } from 'lucide-svelte';

  let me = $state<{ user_id?: string; is_admin?: boolean; email?: string } | null>(null);
  let users = $state<AppUser[]>([]);
  let loading = $state(true);
  let flash = $state<{ ok: boolean; text: string } | null>(null);

  function notify(ok: boolean, text: string) { flash = { ok, text }; setTimeout(() => (flash = null), 5000); }

  async function loadUsers() {
    if (!me?.is_admin) return;
    try { users = await api.auth.users.list(); } catch { /* ignore */ }
  }
  onMount(async () => {
    try { me = await api.auth.me(); await loadUsers(); }
    finally { loading = false; }
  });

  // ── Add user (admin) ─────────────────────────────────────────────────────────
  let nEmail = $state(''); let nName = $state(''); let nPass = $state(''); let nAdmin = $state(false);
  let adding = $state(false);
  async function addUser() {
    if (adding || !nEmail.trim() || nPass.length < 8) return;
    adding = true;
    try {
      const u = await api.auth.users.create({ email: nEmail.trim(), name: nName.trim(), password: nPass, is_admin: nAdmin });
      users = [...users, u];
      nEmail = ''; nName = ''; nPass = ''; nAdmin = false;
      notify(true, 'User created');
    } catch (e) { notify(false, e instanceof Error ? e.message : 'Failed to create user'); }
    finally { adding = false; }
  }
  async function removeUser(u: AppUser) {
    if (!confirm(`Delete ${u.email}? Their account is removed (data handling comes with sharing).`)) return;
    try { await api.auth.users.delete(u.id); users = users.filter(x => x.id !== u.id); }
    catch (e) { notify(false, e instanceof Error ? e.message : 'Failed to delete'); }
  }
  async function resetPassword(u: AppUser) {
    const pw = prompt(`New password for ${u.email} (min 8 chars):`);
    if (!pw) return;
    if (pw.length < 8) { notify(false, 'Password must be at least 8 characters'); return; }
    try { await api.auth.users.setPassword(u.id, pw); notify(true, `Password reset for ${u.email}`); }
    catch (e) { notify(false, e instanceof Error ? e.message : 'Failed to reset'); }
  }

  // ── Change my password ───────────────────────────────────────────────────────
  let curPass = $state(''); let newPass = $state(''); let changing = $state(false);
  async function changeMyPassword() {
    if (changing || newPass.length < 8) return;
    changing = true;
    try {
      await api.auth.changePassword(curPass, newPass);
      curPass = ''; newPass = '';
      notify(true, 'Password changed');
    } catch (e) { notify(false, e instanceof Error ? e.message : 'Failed to change password'); }
    finally { changing = false; }
  }
</script>

<svelte:head><title>Users — Sempa</title></svelte:head>

<div class="mx-auto flex h-full max-w-xl flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <div class="flex items-center gap-3 px-5 py-4" style="border-bottom: 1px solid var(--sempa-border);">
    <a href="/settings/accounts" class="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm transition-colors" style="color: var(--sempa-accent);">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" d="M19 12H5m7-7-7 7 7 7"/></svg>
      Settings
    </a>
    <h1 class="flex items-center gap-2 text-base font-semibold" style="color: var(--sempa-text);"><Users size={18} /> Users</h1>
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-6 pb-20">
    {#if flash}
      <div class="mb-4 rounded-lg px-3 py-2 text-sm" style="background: var(--sempa-bg-panel); border: 1px solid {flash.ok ? 'var(--sempa-success)' : '#c0392b'}; color: {flash.ok ? 'var(--sempa-success)' : '#c0392b'};">{flash.text}</div>
    {/if}

    {#if loading}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else}
      {#if me?.is_admin}
        <p class="mb-3 text-[10.5px] font-bold uppercase tracking-wider" style="color: var(--sempa-text-dim);">People</p>
        <div class="mb-4 overflow-hidden rounded-xl" style="border: 1px solid var(--sempa-border);">
          {#each users as u, i}
            <div class="flex items-center gap-3 px-4 py-3" style="background: var(--sempa-bg-panel); {i > 0 ? 'border-top: 1px solid var(--sempa-border);' : ''}">
              <div class="min-w-0 flex-1">
                <p class="flex items-center gap-1.5 truncate text-sm font-medium" style="color: var(--sempa-text);">
                  {u.name || u.email}
                  {#if u.is_admin}<Shield size={12} style="color: var(--sempa-accent);" />{/if}
                </p>
                <p class="truncate text-xs" style="color: var(--sempa-text-dim);">{u.email}{u.has_password ? '' : ' · Google'}</p>
              </div>
              <button onclick={() => resetPassword(u)} aria-label="Reset password" class="rounded p-1.5 transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);"><KeyRound size={15} /></button>
              {#if u.id !== me?.user_id}
                <button onclick={() => removeUser(u)} aria-label="Delete user" class="rounded p-1.5 transition-opacity hover:opacity-70" style="color: var(--sempa-text-dim);"><Trash2 size={15} /></button>
              {/if}
            </div>
          {/each}
        </div>

        <p class="mb-2 text-[10.5px] font-bold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Add a person</p>
        <div class="mb-8 flex flex-col gap-2 rounded-xl px-4 py-4" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
          <input bind:value={nEmail} type="email" placeholder="Email" autocomplete="off"
            class="rounded-lg px-3 py-2 text-sm outline-none" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
          <input bind:value={nName} placeholder="Name (optional)"
            class="rounded-lg px-3 py-2 text-sm outline-none" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
          <input bind:value={nPass} type="password" placeholder="Temporary password (min 8)" autocomplete="new-password"
            class="rounded-lg px-3 py-2 text-sm outline-none" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
          <label class="flex items-center gap-2 text-sm" style="color: var(--sempa-text-soft);">
            <input type="checkbox" bind:checked={nAdmin} /> Administrator
          </label>
          <button onclick={addUser} disabled={adding || !nEmail.trim() || nPass.length < 8}
            class="mt-1 rounded-lg py-2 text-sm font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-40" style="background: var(--sempa-accent);">
            {adding ? 'Adding…' : 'Add user'}
          </button>
        </div>
      {/if}

      <p class="mb-2 text-[10.5px] font-bold uppercase tracking-wider" style="color: var(--sempa-text-dim);">Change my password</p>
      <div class="flex flex-col gap-2 rounded-xl px-4 py-4" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg-panel);">
        <input bind:value={curPass} type="password" placeholder="Current password" autocomplete="current-password"
          class="rounded-lg px-3 py-2 text-sm outline-none" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
        <input bind:value={newPass} type="password" placeholder="New password (min 8)" autocomplete="new-password"
          class="rounded-lg px-3 py-2 text-sm outline-none" style="border: 1px solid var(--sempa-border); background: var(--sempa-bg); color: var(--sempa-text);" />
        <button onclick={changeMyPassword} disabled={changing || newPass.length < 8}
          class="mt-1 rounded-lg py-2 text-sm font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-40" style="background: var(--sempa-accent);">
          {changing ? 'Saving…' : 'Change password'}
        </button>
      </div>
    {/if}
  </div>
</div>
