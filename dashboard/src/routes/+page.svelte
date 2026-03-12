<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import SetupScreen from '$lib/components/SetupScreen.svelte';
  import StatCard from '$lib/components/StatCard.svelte';
  import TunnelRow from '$lib/components/TunnelRow.svelte';
  import AddTunnelModal from '$lib/components/AddTunnelModal.svelte';
  import { KeynelAPI } from '$lib/api';
  import { tunnels, status, sseConnected, enabledCount, totalCount } from '$lib/store';
  import type { Tunnel } from '$lib/types';

  // ─── 設定 ────────────────────────────────────
  let configured = false;
  let api: KeynelAPI;

  // ─── UI 状態 ─────────────────────────────────
  let showModal = false;
  let statusError = '';
  let es: EventSource | null = null;
  let statusTimer: ReturnType<typeof setInterval>;

  // ─── 初期化 ───────────────────────────────────
  onMount(() => {
    const savedUrl = localStorage.getItem('keynel_server');
    const savedKey = localStorage.getItem('keynel_apikey');
    if (savedUrl && savedKey) {
      init(savedUrl, savedKey);
    }
  });

  onDestroy(() => {
    es?.close();
    clearInterval(statusTimer);
  });

  function init(serverUrl: string, apiKey: string) {
    api = new KeynelAPI(serverUrl, apiKey);
    localStorage.setItem('keynel_server', serverUrl);
    localStorage.setItem('keynel_apikey', apiKey);
    configured = true;
    connectSSE();
    pollStatus();
    statusTimer = setInterval(pollStatus, 12000);
  }

  function disconnect() {
    es?.close();
    clearInterval(statusTimer);
    configured = false;
    tunnels.set([]);
    sseConnected.set(false);
    statusError = '';
  }

  // ─── SSE ─────────────────────────────────────
  let sseRetryTimeout: ReturnType<typeof setTimeout>;

  function connectSSE() {
    es?.close();
    es = new EventSource(api.sseUrl());

    es.addEventListener('init', (e) => {
      const data = JSON.parse(e.data);
      tunnels.set(data.tunnels ?? []);
      status.update((s) => ({ ...s, client_connected: data.client_connected }));
      sseConnected.set(true);
      statusError = '';
    });

    es.addEventListener('tunnel_add', (e) => {
      const t: Tunnel = JSON.parse(e.data);
      tunnels.update((ts) => (ts.find((x) => x.id === t.id) ? ts : [...ts, t]));
    });

    es.addEventListener('tunnel_update', (e) => {
      const t: Tunnel = JSON.parse(e.data);
      tunnels.update((ts) => ts.map((x) => (x.id === t.id ? t : x)));
    });

    es.addEventListener('tunnel_delete', (e) => {
      const { id } = JSON.parse(e.data);
      tunnels.update((ts) => ts.filter((t) => t.id !== id));
    });

    es.addEventListener('client_status', (e) => {
      const data = JSON.parse(e.data);
      status.update((s) => ({ ...s, client_connected: data.connected }));
    });

    es.onerror = () => {
      sseConnected.set(false);
      statusError = 'リアルタイム接続が切断されました。再接続中...';
      es?.close();
      clearTimeout(sseRetryTimeout);
      sseRetryTimeout = setTimeout(connectSSE, 5000);
    };
  }

  // ─── ポーリング ───────────────────────────────
  async function pollStatus() {
    if (!api) return;
    try {
      const s = await api.getStatus();
      status.set(s);
      if (statusError.startsWith('サーバー')) statusError = '';
    } catch {
      statusError = 'サーバーに到達できません';
    }
  }

  // ─── トンネル操作 ─────────────────────────────
  async function handleAdd(proto: 'tcp' | 'udp', clientPort: number, serverPort: number) {
    const t = await api.createTunnel(proto, clientPort, serverPort);
    tunnels.update((ts) => [...ts, t]);
  }

  async function handlePatch(id: string, patch: { enabled: boolean; rate_limit: boolean }) {
    const updated = await api.patchTunnel(id, patch);
    tunnels.update((ts) => ts.map((t) => (t.id === id ? updated : t)));
  }

  async function handleDelete(id: string) {
    await api.deleteTunnel(id);
    tunnels.update((ts) => ts.filter((t) => t.id !== id));
  }
</script>

{#if !configured}
  <SetupScreen on:connect={(e) => init(e.detail.serverUrl, e.detail.apiKey)} />

{:else}
  <div class="min-h-screen bg-zinc-950 text-white">
    <!-- Background grid -->
    <div class="pointer-events-none fixed inset-0 bg-[linear-gradient(to_right,#ffffff04_1px,transparent_1px),linear-gradient(to_bottom,#ffffff04_1px,transparent_1px)] bg-[size:48px_48px]"></div>

    <!-- Nav -->
    <nav class="sticky top-0 z-40 border-b border-zinc-800/60 bg-zinc-950/80 backdrop-blur-md">
      <div class="mx-auto flex h-14 max-w-5xl items-center justify-between px-6">
        <div class="flex items-center gap-3">
          <span class="text-base font-semibold tracking-tight text-white">⬡ Keynel</span>
          <span class="hidden h-4 w-px bg-zinc-700 sm:block"></span>
          <span class="hidden text-xs text-zinc-500 sm:block">Tunnel Dashboard</span>
        </div>

        <div class="flex items-center gap-4">
          <!-- SSE indicator -->
          <div class="flex items-center gap-2">
            <span
              class="relative inline-flex h-2 w-2 rounded-full
                {$sseConnected ? 'bg-emerald-500' : 'bg-zinc-600'}"
            >
              {#if $sseConnected}
                <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60"></span>
              {/if}
            </span>
            <span class="text-xs text-zinc-500">{$sseConnected ? 'Live' : 'Connecting...'}</span>
          </div>

          <!-- Client status -->
          <div
            class="hidden items-center gap-1.5 rounded-full border px-3 py-1 text-xs sm:flex
              {$status.client_connected
                ? 'border-emerald-500/20 bg-emerald-500/5 text-emerald-400'
                : 'border-zinc-700 bg-zinc-900 text-zinc-500'}"
          >
            <span
              class="h-1.5 w-1.5 rounded-full
                {$status.client_connected ? 'bg-emerald-400' : 'bg-zinc-600'}"
            ></span>
            {$status.client_connected ? 'Client Online' : 'Client Offline'}
          </div>

          <button
            on:click={disconnect}
            class="rounded-lg border border-zinc-700/60 bg-zinc-900/50 px-3 py-1.5 text-xs text-zinc-400 transition-colors hover:border-zinc-600 hover:text-zinc-300"
          >
            設定変更
          </button>
        </div>
      </div>
    </nav>

    <!-- Main -->
    <main class="relative mx-auto max-w-5xl px-6 py-10">

      <!-- Error banner -->
      {#if statusError}
        <div class="mb-6 flex items-center gap-3 rounded-xl border border-red-900/40 bg-red-950/20 px-4 py-3">
          <svg class="h-4 w-4 shrink-0 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/>
          </svg>
          <span class="text-xs text-red-400">{statusError}</span>
        </div>
      {/if}

      <!-- Stats -->
      <div class="mb-8 grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard value={$totalCount} label="トンネル" />
        <StatCard value={$enabledCount} label="有効" accent="green" />
        <StatCard value={$status.active_conns} label="接続中" />
        <StatCard value={$status.banned_ips} label="BAN IP" accent={$status.banned_ips > 0 ? 'red' : 'default'} />
      </div>

      <!-- Tunnel section -->
      <div class="overflow-hidden rounded-2xl border border-zinc-800/60 bg-zinc-900/40 backdrop-blur-sm">
        <!-- Section header -->
        <div class="flex items-center justify-between border-b border-zinc-800/60 px-6 py-4">
          <div>
            <h2 class="text-sm font-semibold text-white">トンネル</h2>
            {#if $totalCount > 0}
              <p class="mt-0.5 text-xs text-zinc-500">{$enabledCount} / {$totalCount} 有効</p>
            {/if}
          </div>
          <button
            on:click={() => (showModal = true)}
            class="flex items-center gap-1.5 rounded-lg bg-white px-4 py-2 text-sm font-semibold text-black transition-all hover:bg-zinc-100 active:scale-95"
          >
            <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4"/>
            </svg>
            追加
          </button>
        </div>

        <!-- Table header -->
        {#if $tunnels.length > 0}
          <div
            class="grid border-b border-zinc-800/40 px-6 py-2.5 text-[10px] font-medium uppercase tracking-widest text-zinc-600"
            style="grid-template-columns: 60px 1fr 120px 130px 44px"
          >
            <span>Proto</span>
            <span>ルート</span>
            <span>ON / OFF</span>
            <span>Ratelimit</span>
            <span></span>
          </div>
        {/if}

        <!-- Rows -->
        {#if $tunnels.length === 0}
          <div class="flex flex-col items-center justify-center gap-2 px-6 py-16 text-center">
            <div class="flex h-12 w-12 items-center justify-center rounded-full border border-zinc-800 bg-zinc-900">
              <svg class="h-5 w-5 text-zinc-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M8.288 15.038a5.25 5.25 0 017.424 0M5.106 11.856c3.807-3.808 9.98-3.808 13.788 0M1.924 8.674c5.565-5.565 14.587-5.565 20.152 0M12.53 18.22l-.53.53-.53-.53a.75.75 0 011.06 0z"/>
              </svg>
            </div>
            <p class="text-sm text-zinc-400">トンネルがありません</p>
            <p class="text-xs text-zinc-600">「追加」ボタンからトンネルを作成してください</p>
          </div>
        {:else}
          {#each $tunnels as tunnel (tunnel.id)}
            <TunnelRow
              {tunnel}
              onToggle={handlePatch}
              onDelete={handleDelete}
            />
          {/each}
        {/if}
      </div>

    </main>
  </div>

  <!-- Modal -->
  {#if showModal}
    <AddTunnelModal
      onAdd={handleAdd}
      on:close={() => (showModal = false)}
    />
  {/if}
{/if}
