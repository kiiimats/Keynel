<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { KeynelAPI } from '$lib/api';
  import type { Tunnel, ServerStatus } from '$lib/types';

  // ─── 状態 ─────────────────────────────────────────────
  let configured = false;
  let inputIp  = '';
  let inputKey = '';
  let setupErr = '';
  let setupLoading = false;

  let api: KeynelAPI;
  let tunnels: Tunnel[] = [];
  let status: ServerStatus = { client_connected: false, tunnel_count: 0, active_conns: 0, banned_ips: 0 };
  let sseOk = false;
  let pageErr = '';

  let showModal = false;
  let newProto: 'tcp' | 'udp' = 'tcp';
  let newLocal  = '';
  let newRemote = '';
  let addErr = '';
  let addLoading = false;

  let es: EventSource | null = null;
  let statusTimer: ReturnType<typeof setInterval>;
  let sseRetry:    ReturnType<typeof setTimeout>;

  // ─── 初期化 ───────────────────────────────────────────
  onMount(() => {
    const t = localStorage.getItem('kn_target');
    const k = localStorage.getItem('kn_key');
    if (t && k) boot(t, k);
  });

  onDestroy(() => { es?.close(); clearInterval(statusTimer); clearTimeout(sseRetry); });

  function normalizeTarget(ip: string) {
    let s = ip.trim().replace(/\/$/, '');
    if (!/^https?:\/\//.test(s)) s = 'http://' + s;
    const h = s.replace(/^https?:\/\//, '');
    if (!h.includes(':')) s += ':7002';
    return s;
  }

  async function setup() {
    setupErr = '';
    if (!inputIp || !inputKey) return;
    setupLoading = true;
    const target = normalizeTarget(inputIp);
    try {
      const r = await fetch(`/proxy?${new URLSearchParams({ target, key: inputKey, path: '/api/status' })}`);
      if (!r.ok) throw new Error(`HTTP ${r.status} — APIキーを確認してください`);
      localStorage.setItem('kn_target', target);
      localStorage.setItem('kn_key', inputKey);
      boot(target, inputKey);
    } catch (e: any) {
      setupErr = e.message?.includes('502') || e.message?.includes('fetch')
        ? 'サーバーに接続できません。IPと ufw 7002/tcp を確認してください。'
        : (e.message ?? '接続エラー');
    }
    setupLoading = false;
  }

  function boot(target: string, key: string) {
    api = new KeynelAPI(target, key);
    configured = true;
    connectSSE();
    pollStatus();
    statusTimer = setInterval(pollStatus, 12000);
  }

  function logout() {
    es?.close(); clearInterval(statusTimer); clearTimeout(sseRetry);
    configured = false; tunnels = []; sseOk = false; pageErr = '';
  }

  // ─── SSE ─────────────────────────────────────────────
  function connectSSE() {
    es?.close();
    es = new EventSource(api.sseUrl());

    es.addEventListener('init', (e: MessageEvent) => {
      const d = JSON.parse(e.data);
      tunnels = d.tunnels ?? [];
      status = { ...status, client_connected: d.client_connected };
      sseOk = true; pageErr = '';
    });
    es.addEventListener('tunnel_add',    (e: MessageEvent) => {
      const t: Tunnel = JSON.parse(e.data);
      if (!tunnels.find(x => x.id === t.id)) tunnels = [...tunnels, t];
    });
    es.addEventListener('tunnel_update', (e: MessageEvent) => {
      const t: Tunnel = JSON.parse(e.data);
      tunnels = tunnels.map(x => x.id === t.id ? t : x);
    });
    es.addEventListener('tunnel_delete', (e: MessageEvent) => {
      tunnels = tunnels.filter(t => t.id !== JSON.parse(e.data).id);
    });
    es.addEventListener('client_status', (e: MessageEvent) => {
      status = { ...status, client_connected: JSON.parse(e.data).connected };
    });
    es.onerror = () => {
      sseOk = false;
      es?.close();
      clearTimeout(sseRetry);
      sseRetry = setTimeout(connectSSE, 5000);
    };
  }

  async function pollStatus() {
    if (!api) return;
    try { status = await api.getStatus(); }
    catch { pageErr = 'サーバーに到達できません'; }
  }

  // ─── トンネル操作 ─────────────────────────────────────
  async function addTunnel() {
    addErr = '';
    const cp = parseInt(newLocal), sp = parseInt(newRemote);
    if (!cp || !sp || cp < 1 || cp > 65535 || sp < 1 || sp > 65535) {
      addErr = 'ポートは 1〜65535 で入力してください'; return;
    }
    addLoading = true;
    try {
      const t = await api.createTunnel(newProto, cp, sp);
      tunnels = [...tunnels, t];
      showModal = false; newLocal = ''; newRemote = '';
    } catch (e: any) { addErr = e.message; }
    addLoading = false;
  }

  async function patch(t: Tunnel, field: 'enabled' | 'rate_limit') {
    const updated = await api.patchTunnel(t.id, {
      enabled:    field === 'enabled'    ? !t.enabled    : t.enabled,
      rate_limit: field === 'rate_limit' ? !t.rate_limit : t.rate_limit,
    });
    tunnels = tunnels.map(x => x.id === updated.id ? updated : x);
  }

  async function del(id: string) {
    if (!confirm('削除しますか？')) return;
    await api.deleteTunnel(id);
    tunnels = tunnels.filter(t => t.id !== id);
  }

  function badge(t: Tunnel) {
    if (t.proto === 'tcp' && t.client_port === 22)  return { label: 'SSH', cls: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400' };
    if (t.proto === 'tcp' && t.client_port === 445) return { label: 'SMB', cls: 'border-amber-500/30  bg-amber-500/10  text-amber-400'  };
    if (t.proto === 'tcp') return { label: 'TCP', cls: 'border-sky-500/30    bg-sky-500/10    text-sky-400'    };
    return { label: 'UDP', cls: 'border-violet-500/30 bg-violet-500/10 text-violet-400' };
  }
</script>

<!-- ══════════════════════════════ SETUP ══════════════════════════════ -->
{#if !configured}
<div class="flex min-h-screen items-center justify-center bg-zinc-950 px-4">
  <div class="pointer-events-none fixed inset-0 bg-[linear-gradient(to_right,#ffffff05_1px,transparent_1px),linear-gradient(to_bottom,#ffffff05_1px,transparent_1px)] bg-[size:48px_48px]"></div>

  <div class="relative w-full max-w-sm">
    <div class="mb-10 text-center">
      <div class="mb-3 inline-flex h-12 w-12 items-center justify-center rounded-xl border border-zinc-800 bg-zinc-900 text-2xl">⬡</div>
      <h1 class="text-xl font-semibold text-white">Keynel</h1>
      <p class="mt-1 text-sm text-zinc-500">Tunnel Dashboard</p>
    </div>

    <div class="rounded-2xl border border-zinc-800 bg-zinc-900/80 p-6 shadow-2xl shadow-black/50">
      <div class="space-y-4">
        <div class="space-y-1.5">
          <label class="text-[11px] font-medium uppercase tracking-widest text-zinc-500">サーバー IP</label>
          <input bind:value={inputIp} on:keydown={e => e.key==='Enter'&&setup()}
            placeholder="220.158.19.132"
            class="w-full rounded-lg border border-zinc-700 bg-zinc-800/50 px-3.5 py-2.5 text-sm text-white placeholder-zinc-600 outline-none focus:border-zinc-500" />
          {#if inputIp}
            <p class="font-mono text-[11px] text-zinc-600">→ {normalizeTarget(inputIp)}</p>
          {/if}
        </div>

        <div class="space-y-1.5">
          <label class="text-[11px] font-medium uppercase tracking-widest text-zinc-500">API キー</label>
          <input type="password" bind:value={inputKey} on:keydown={e => e.key==='Enter'&&setup()}
            placeholder="keynel.json の api_key"
            class="w-full rounded-lg border border-zinc-700 bg-zinc-800/50 px-3.5 py-2.5 font-mono text-sm text-white placeholder-zinc-600 outline-none focus:border-zinc-500" />
        </div>

        {#if setupErr}
          <p class="rounded-lg border border-red-900/40 bg-red-950/30 px-3.5 py-2 text-xs text-red-400">{setupErr}</p>
        {/if}

        <button on:click={setup} disabled={setupLoading || !inputIp || !inputKey}
          class="w-full rounded-lg bg-white py-2.5 text-sm font-semibold text-black hover:bg-zinc-100 disabled:opacity-40">
          {setupLoading ? '接続中...' : '接続する'}
        </button>
      </div>
    </div>
  </div>
</div>

<!-- ══════════════════════════════ DASHBOARD ══════════════════════════════ -->
{:else}
<div class="min-h-screen bg-zinc-950">
  <div class="pointer-events-none fixed inset-0 bg-[linear-gradient(to_right,#ffffff03_1px,transparent_1px),linear-gradient(to_bottom,#ffffff03_1px,transparent_1px)] bg-[size:48px_48px]"></div>

  <!-- Nav -->
  <nav class="sticky top-0 z-40 border-b border-zinc-800/60 bg-zinc-950/80 backdrop-blur-md">
    <div class="mx-auto flex h-14 max-w-4xl items-center justify-between px-6">
      <span class="text-sm font-semibold text-white">⬡ Keynel</span>
      <div class="flex items-center gap-4">
        <!-- SSE dot -->
        <span class="relative inline-flex h-2 w-2 rounded-full {sseOk ? 'bg-emerald-500' : 'bg-zinc-600'}">
          {#if sseOk}<span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60"></span>{/if}
        </span>
        <!-- Client status -->
        <span class="hidden text-xs sm:inline {status.client_connected ? 'text-emerald-400' : 'text-zinc-500'}">
          {status.client_connected ? 'Client Online' : 'Client Offline'}
        </span>
        <button on:click={logout}
          class="rounded-lg border border-zinc-700/60 px-3 py-1.5 text-xs text-zinc-400 hover:border-zinc-600 hover:text-zinc-300">
          設定変更
        </button>
      </div>
    </div>
  </nav>

  <main class="relative mx-auto max-w-4xl px-6 py-10">

    {#if pageErr}
      <div class="mb-6 rounded-xl border border-red-900/40 bg-red-950/20 px-4 py-3 text-xs text-red-400">{pageErr}</div>
    {/if}

    <!-- Stats -->
    <div class="mb-8 grid grid-cols-2 gap-3 sm:grid-cols-4">
      {#each [
        { n: tunnels.length,                            label: 'トンネル',  accent: '' },
        { n: tunnels.filter(t=>t.enabled).length,       label: '有効',      accent: 'text-emerald-400' },
        { n: status.active_conns,                       label: '接続中',    accent: '' },
        { n: status.banned_ips,                         label: 'BAN IP',    accent: status.banned_ips > 0 ? 'text-red-400' : '' },
      ] as s}
        <div class="rounded-xl border border-zinc-800/60 bg-zinc-900/40 px-5 py-4">
          <div class="font-mono text-3xl font-semibold tracking-tight {s.accent || 'text-white'}">{s.n}</div>
          <div class="mt-1 text-[10px] uppercase tracking-widest text-zinc-500">{s.label}</div>
        </div>
      {/each}
    </div>

    <!-- Tunnel card -->
    <div class="overflow-hidden rounded-2xl border border-zinc-800/60 bg-zinc-900/40">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-zinc-800/60 px-6 py-4">
        <h2 class="text-sm font-semibold text-white">トンネル</h2>
        <button on:click={() => { showModal = true; addErr = ''; }}
          class="flex items-center gap-1.5 rounded-lg bg-white px-4 py-2 text-sm font-semibold text-black hover:bg-zinc-100 active:scale-95">
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
          </svg>
          追加
        </button>
      </div>

      <!-- Table head -->
      {#if tunnels.length > 0}
        <div class="grid border-b border-zinc-800/40 px-6 py-2 text-[10px] font-medium uppercase tracking-widest text-zinc-600"
          style="grid-template-columns:64px 1fr 110px 120px 40px">
          <span>Proto</span><span>ルート</span><span>ON/OFF</span><span>Ratelimit</span><span></span>
        </div>
      {/if}

      <!-- Empty -->
      {#if tunnels.length === 0}
        <div class="py-16 text-center text-sm text-zinc-500">
          トンネルがありません。「追加」から作成してください。
        </div>

      {:else}
        {#each tunnels as t (t.id)}
          {@const b = badge(t)}
          <div class="group grid items-center gap-3 border-b border-zinc-800/40 px-6 py-3.5 transition-colors hover:bg-zinc-800/20 last:border-0 {!t.enabled ? 'opacity-40' : ''}"
            style="grid-template-columns:64px 1fr 110px 120px 40px">

            <!-- Badge -->
            <span class="inline-block rounded border px-2 py-0.5 font-mono text-[11px] font-semibold tracking-wider {b.cls}">{b.label}</span>

            <!-- Route -->
            <span class="flex items-center gap-2 font-mono text-sm">
              <span class="text-zinc-400">:{t.client_port}</span>
              <span class="text-zinc-600">→</span>
              <span class="text-zinc-400">:{t.server_port}</span>
            </span>

            <!-- ON/OFF -->
            <span class="flex items-center gap-2">
              <button on:click={() => patch(t, 'enabled')}
                class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border transition-all duration-200
                  {t.enabled ? 'border-emerald-500/40 bg-emerald-500/20' : 'border-zinc-700 bg-zinc-800/60'}">
                <span class="inline-block h-3.5 w-3.5 rounded-full shadow transition-all duration-200
                  {t.enabled ? 'translate-x-4 bg-emerald-400' : 'translate-x-0.5 bg-zinc-500'}"></span>
              </button>
              <span class="text-xs {t.enabled ? 'text-emerald-400' : 'text-zinc-600'}">{t.enabled ? 'ON' : 'OFF'}</span>
            </span>

            <!-- Ratelimit -->
            <span class="flex items-center gap-2">
              <button on:click={() => patch(t, 'rate_limit')}
                class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border transition-all duration-200
                  {t.rate_limit ? 'border-amber-500/40 bg-amber-500/20' : 'border-zinc-700 bg-zinc-800/60'}">
                <span class="inline-block h-3.5 w-3.5 rounded-full shadow transition-all duration-200
                  {t.rate_limit ? 'translate-x-4 bg-amber-400' : 'translate-x-0.5 bg-zinc-500'}"></span>
              </button>
              <span class="text-xs {t.rate_limit ? 'text-amber-400' : 'text-zinc-600'}">{t.rate_limit ? 'ON' : 'OFF'}</span>
            </span>

            <!-- Delete -->
            <button on:click={() => del(t.id)}
              class="flex h-7 w-7 items-center justify-center rounded-md text-zinc-600 opacity-0 transition-all hover:bg-red-950/40 hover:text-red-400 group-hover:opacity-100">
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
              </svg>
            </button>
          </div>
        {/each}
      {/if}
    </div>
  </main>
</div>

<!-- ══════════════════════════════ MODAL ══════════════════════════════ -->
{#if showModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
    on:click|self={() => showModal = false} role="dialog" aria-modal="true">
    <div class="w-full max-w-md rounded-2xl border border-zinc-700/60 bg-zinc-900 shadow-2xl shadow-black/60">

      <div class="flex items-center justify-between border-b border-zinc-800 px-6 py-4">
        <h2 class="text-sm font-semibold text-white">トンネルを追加</h2>
        <button on:click={() => showModal = false} class="text-zinc-500 hover:text-zinc-300">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>

      <div class="space-y-5 px-6 py-5">
        <!-- Proto -->
        <div class="space-y-2">
          <label class="text-[11px] font-medium uppercase tracking-widest text-zinc-500">プロトコル</label>
          <div class="flex gap-2">
            {#each ['tcp', 'udp'] as p}
              <button on:click={() => newProto = p as 'tcp'|'udp'}
                class="flex-1 rounded-lg border py-2 font-mono text-sm font-semibold tracking-wider transition-all
                  {newProto === p
                    ? p === 'tcp' ? 'border-sky-500/50 bg-sky-500/10 text-sky-300' : 'border-violet-500/50 bg-violet-500/10 text-violet-300'
                    : 'border-zinc-700 bg-zinc-800/40 text-zinc-500 hover:border-zinc-600'}">
                {p.toUpperCase()}
              </button>
            {/each}
          </div>
        </div>

        <!-- Ports -->
        <div class="grid grid-cols-[1fr_20px_1fr] items-end gap-2">
          <div class="space-y-1.5">
            <label class="text-[11px] font-medium uppercase tracking-widest text-zinc-500">Local</label>
            <input type="number" bind:value={newLocal} placeholder="25565" min="1" max="65535"
              class="w-full rounded-lg border border-zinc-700 bg-zinc-800/60 px-3 py-2.5 font-mono text-sm text-white placeholder-zinc-600 outline-none focus:border-zinc-500" />
          </div>
          <div class="pb-2.5 text-center text-zinc-600">→</div>
          <div class="space-y-1.5">
            <label class="text-[11px] font-medium uppercase tracking-widest text-zinc-500">Internet</label>
            <input type="number" bind:value={newRemote} placeholder="10000" min="1" max="65535"
              class="w-full rounded-lg border border-zinc-700 bg-zinc-800/60 px-3 py-2.5 font-mono text-sm text-white placeholder-zinc-600 outline-none focus:border-zinc-500" />
          </div>
        </div>

        {#if addErr}
          <p class="rounded-lg border border-red-900/40 bg-red-950/30 px-4 py-2.5 text-xs text-red-400">{addErr}</p>
        {/if}
      </div>

      <div class="flex gap-2 border-t border-zinc-800 px-6 py-4">
        <button on:click={() => showModal = false}
          class="flex-1 rounded-lg border border-zinc-700 bg-zinc-800/50 py-2.5 text-sm text-zinc-400 hover:bg-zinc-800">
          キャンセル
        </button>
        <button on:click={addTunnel} disabled={addLoading || !newLocal || !newRemote}
          class="flex-1 rounded-lg bg-white py-2.5 text-sm font-semibold text-black hover:bg-zinc-100 disabled:opacity-40">
          {addLoading ? '追加中...' : '追加する'}
        </button>
      </div>
    </div>
  </div>
{/if}
{/if}
