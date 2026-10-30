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

  // ── Invite / add a person (admin) ────────────────────────────────────────────
  // Default is a Google-only invite (no password held). A password account is an
  // opt-in fallback behind the toggle.
  let nEmail = $state(''); let nName = $state(''); let nPass = $state(''); let nAdmin = $state(false);
  let nUsePassword = $state(false);
  let adding = $state(false);
  const canAdd = $derived(!!nEmail.trim() && (!nUsePassword || nPass.length >= 8));
  async function addUser() {
    if (adding || !canAdd) return;
    adding = true;
    try {
      const u = await api.auth.users.create({
        email: nEmail.trim(), name: nName.trim(), is_admin: nAdmin,
        ...(nUsePassword ? { password: nPass } : {}),  // omitted = Google-only invite
      });
      users = [...users, u];
      const wasGoogle = !nUsePassword;
      nEmail = ''; nName = ''; nPass = ''; nAdmin = false; nUsePassword = false;
      notify(true, wasGoogle ? `Invited ${u.email} — they can now Sign in with Google` : 'Account created');
    } catch (e) { notify(false, e instanceof Error ? e.message : 'Failed to add person'); }
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

<div class="mx-auto flex h-full max-w-[640px] flex-col" style="padding-top: env(safe-area-inset-top, 0px);">
  <div class="px-5 pt-6 pb-2 sm:px-8 sm:pt-8">
    <a href="/settings/accounts" class="set-back">
      <svg class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="m15 18-6-6 6-6"/></svg>
      Settings
    </a>
    <h1 class="set-title type-page flex items-center gap-2.5"><Users size={24} strokeWidth={2.2} /> Users</h1>
  </div>

  <div class="flex-1 overflow-y-auto px-5 pt-4 pb-20 sm:px-8">
    {#if flash}
      <div class="mb-4 rounded-2xl px-4 py-3 text-[13px] font-semibold"
           style="background: {flash.ok ? 'var(--sempa-success-bg)' : 'color-mix(in srgb, var(--set-danger) 10%, var(--sempa-bg-panel))'};
                  color: {flash.ok ? 'var(--sempa-success)' : 'var(--set-danger)'};">{flash.text}</div>
    {/if}

    {#if loading}
      <p class="text-sm" style="color: var(--sempa-text-dim);">Loading…</p>
    {:else}
      {#if me?.is_admin}
        <h2 class="set-heading">People</h2>
        <div class="set-group">
          {#each users as u}
            <div class="set-row set-row-compact">
              <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-[13px] font-extrabold"
                   style="background: var(--sempa-accent-bg); color: var(--sempa-accent-ink);">
                {(u.name || u.email).charAt(0).toUpperCase()}
              </div>
              <div class="set-row-main">
                <p class="set-label flex items-center gap-1.5 truncate">
                  {u.name || u.email}
                  {#if u.is_admin}<Shield size={13} style="color: var(--sempa-accent);" />{/if}
                </p>
                <p class="set-desc truncate">{u.email}{u.has_password ? '' : ' · Google'}</p>
              </div>
              {#if u.has_password}
                <button onclick={() => resetPassword(u)} aria-label="Reset password" class="set-icon-btn"><KeyRound size={15} /></button>
              {/if}
              {#if u.id !== me?.user_id}
                <button onclick={() => removeUser(u)} aria-label="Delete user" class="set-icon-btn danger"><Trash2 size={15} /></button>
              {/if}
            </div>
          {/each}
        </div>

        <h2 class="set-heading">Invite a person</h2>
        <div class="set-group">
          <div class="set-block flex flex-col gap-2.5">
            <p class="set-desc" style="margin-top: 0;">
              They sign in with their Google account — no password is stored. Just enter
              their email and tell them to open Sempa and choose “Sign in with Google”.
            </p>
            <input bind:value={nEmail} type="email" placeholder="Google email" autocomplete="off" aria-label="Email"
              class="set-input" />
            <input bind:value={nName} placeholder="Name (optional)" aria-label="Name"
              class="set-input" />
          </div>
          <label class="set-row set-row-compact cursor-pointer">
            <span class="set-row-main set-label">Administrator</span>
            <input type="checkbox" bind:checked={nAdmin} class="set-switch-input" />
            <span class="set-switch" aria-hidden="true"></span>
          </label>
          <label class="set-row set-row-compact cursor-pointer">
            <span class="set-row-main">
              <span class="set-label block">Set a password instead</span>
              <span class="set-desc block">For someone without a Google account.</span>
            </span>
            <input type="checkbox" bind:checked={nUsePassword} class="set-switch-input" />
            <span class="set-switch" aria-hidden="true"></span>
          </label>
          <div class="set-block flex flex-col gap-2.5">
            {#if nUsePassword}
              <input bind:value={nPass} type="password" placeholder="Password (min 8)" autocomplete="new-password" aria-label="Password"
                class="set-input" />
            {/if}
            <button onclick={addUser} disabled={adding || !canAdd}
              class="btn-primary set-btn h-10 w-full">
              {adding ? 'Adding…' : (nUsePassword ? 'Create account' : 'Invite via Google')}
            </button>
          </div>
        </div>
      {/if}

      <h2 class="set-heading">Change my password</h2>
      <div class="set-group">
        <div class="set-block flex flex-col gap-2.5">
          <input bind:value={curPass} type="password" placeholder="Current password" autocomplete="current-password" aria-label="Current password"
            class="set-input" />
          <input bind:value={newPass} type="password" placeholder="New password (min 8)" autocomplete="new-password" aria-label="New password"
            class="set-input" />
          <button onclick={changeMyPassword} disabled={changing || newPass.length < 8}
            class="btn-primary set-btn mt-1 h-10 w-full">
            {changing ? 'Saving…' : 'Change password'}
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
