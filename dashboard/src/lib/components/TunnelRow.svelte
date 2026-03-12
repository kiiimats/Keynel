<script lang="ts">
  import Toggle from './Toggle.svelte';
  import type { Tunnel } from '../types';

  export let tunnel: Tunnel;
  export let onToggle: (id: string, patch: { enabled: boolean; rate_limit: boolean }) => Promise<void>;
  export let onDelete: (id: string) => Promise<void>;

  let deleting = false;
  let patching = false;

  async function handleDelete() {
    if (!confirm('このトンネルを削除しますか？')) return;
    deleting = true;
    await onDelete(tunnel.id);
    deleting = false;
  }

  async function toggle(field: 'enabled' | 'rate_limit') {
    patching = true;
    await onToggle(tunnel.id, {
      enabled: field === 'enabled' ? !tunnel.enabled : tunnel.enabled,
      rate_limit: field === 'rate_limit' ? !tunnel.rate_limit : tunnel.rate_limit,
    });
    patching = false;
  }
</script>

<div
  class="group grid items-center gap-4 border-b border-zinc-800/50 px-6 py-3.5 transition-colors duration-150 hover:bg-zinc-800/20
    {!tunnel.enabled ? 'opacity-40' : ''}"
  style="grid-template-columns: 60px 1fr 120px 130px 44px"
>
  <!-- Proto badge -->
  <span>
    <span
      class="inline-block rounded px-2 py-0.5 font-mono text-[11px] font-semibold tracking-wider
        {tunnel.proto === 'tcp'
          ? 'border border-sky-500/30 bg-sky-500/10 text-sky-400'
          : 'border border-violet-500/30 bg-violet-500/10 text-violet-400'}"
    >
      {tunnel.proto.toUpperCase()}
    </span>
  </span>

  <!-- Route -->
  <span class="flex items-center gap-2 text-sm">
    <span class="font-mono text-zinc-300">
      Local :<span class="text-white">{tunnel.client_port}</span>
    </span>
    <span class="text-zinc-600">→</span>
    <span class="font-mono text-zinc-300">
      Internet :<span class="text-white">{tunnel.server_port}</span>
    </span>
  </span>

  <!-- ON / OFF -->
  <span class="flex items-center gap-2.5">
    <Toggle
      checked={tunnel.enabled}
      color="green"
      disabled={patching}
      onChange={() => toggle('enabled')}
    />
    <span class="text-xs {tunnel.enabled ? 'text-emerald-400' : 'text-zinc-600'}">
      {tunnel.enabled ? 'ON' : 'OFF'}
    </span>
  </span>

  <!-- Ratelimit -->
  <span class="flex items-center gap-2.5">
    <Toggle
      checked={tunnel.rate_limit}
      color="amber"
      disabled={patching}
      onChange={() => toggle('rate_limit')}
    />
    <span class="text-xs {tunnel.rate_limit ? 'text-amber-400' : 'text-zinc-600'}">
      {tunnel.rate_limit ? 'ON' : 'OFF'}
    </span>
  </span>

  <!-- Delete -->
  <span>
    <button
      on:click={handleDelete}
      disabled={deleting}
      class="flex h-7 w-7 items-center justify-center rounded-md border border-transparent text-zinc-600 opacity-0 transition-all duration-150 hover:border-red-900/60 hover:bg-red-950/40 hover:text-red-400 group-hover:opacity-100 disabled:cursor-not-allowed"
      title="削除"
    >
      {#if deleting}
        <svg class="h-3.5 w-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"/>
        </svg>
      {:else}
        <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
        </svg>
      {/if}
    </button>
  </span>
</div>
