<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  let serverUrl = '';
  let apiKey = '';
  let error = '';
  let testing = false;

  function normalizeUrl(input: string): string {
    let url = input.trim().replace(/\/$/, '');
    if (!/^https?:\/\//.test(url)) url = 'http://' + url;
    const hostPart = url.replace(/^https?:\/\//, '');
    if (!hostPart.includes(':')) url = url + ':7002';
    return url;
  }

  async function connect() {
    error = '';
    if (!serverUrl || !apiKey) return;
    testing = true;
    try {
      const url = normalizeUrl(serverUrl);
      const res = await fetch(`${url}/api/status`, {
        headers: { 'X-API-Key': apiKey },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status} — APIキーが違います`);
      dispatch('connect', { serverUrl: url, apiKey });
    } catch (e: any) {
      const msg: string = e.message ?? '';
      if (msg.includes('Failed to fetch') || msg.includes('ERR_CONNECTION') || msg.includes('TIMED_OUT')) {
        error = 'サーバーに到達できません。ufw で 7002/tcp を開けてください。';
      } else {
        error = msg || '接続できませんでした';
      }
    }
    testing = false;
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Enter') connect();
  }

  // 入力中にリアルタイムでプレビュー表示
  $: preview = serverUrl ? normalizeUrl(serverUrl) : '';
</script>

<div class="flex min-h-screen items-center justify-center bg-zinc-950 px-4">
  <div class="pointer-events-none fixed inset-0 bg-[linear-gradient(to_right,#ffffff06_1px,transparent_1px),linear-gradient(to_bottom,#ffffff06_1px,transparent_1px)] bg-[size:48px_48px]"></div>
  <div class="pointer-events-none fixed left-1/2 top-1/3 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-cyan-500/5 blur-3xl"></div>

  <div class="relative w-full max-w-sm">
    <div class="mb-10 text-center">
      <div class="mb-3 inline-flex h-12 w-12 items-center justify-center rounded-xl border border-zinc-700/60 bg-zinc-900 text-2xl shadow-lg shadow-black/40">
        ⬡
      </div>
      <h1 class="text-xl font-semibold tracking-tight text-white">Keynel</h1>
      <p class="mt-1 text-sm text-zinc-500">Tunnel Management Dashboard</p>
    </div>

    <div class="rounded-2xl border border-zinc-800/80 bg-zinc-900/80 p-6 shadow-2xl shadow-black/50 backdrop-blur-sm">
      <div class="space-y-4">
        <div class="space-y-1.5">
          <label class="text-xs font-medium uppercase tracking-widest text-zinc-500">
            サーバー IP
          </label>
          <input
            type="text"
            bind:value={serverUrl}
            on:keydown={onKey}
            placeholder="220.158.19.132"
            autocomplete="off"
            class="w-full rounded-lg border border-zinc-700 bg-zinc-800/50 px-3.5 py-2.5 text-sm text-white placeholder-zinc-600 outline-none transition-all focus:border-zinc-500 focus:ring-2 focus:ring-zinc-500/20"
          />
          {#if preview}
            <p class="font-mono text-[11px] text-zinc-600">→ {preview}</p>
          {/if}
        </div>

        <div class="space-y-1.5">
          <label class="text-xs font-medium uppercase tracking-widest text-zinc-500">
            API キー
          </label>
          <input
            type="password"
            bind:value={apiKey}
            on:keydown={onKey}
            placeholder="keynel.json の api_key"
            autocomplete="off"
            class="w-full rounded-lg border border-zinc-700 bg-zinc-800/50 px-3.5 py-2.5 font-mono text-sm text-white placeholder-zinc-600 outline-none transition-all focus:border-zinc-500 focus:ring-2 focus:ring-zinc-500/20"
          />
        </div>

        {#if error}
          <div class="rounded-lg border border-red-900/40 bg-red-950/30 px-3.5 py-2.5 text-xs text-red-400">
            {error}
          </div>
        {/if}

        <button
          on:click={connect}
          disabled={testing || !serverUrl || !apiKey}
          class="mt-2 w-full rounded-lg bg-white py-2.5 text-sm font-semibold text-black transition-all hover:bg-zinc-100 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-40"
        >
          {testing ? '接続中...' : '接続する'}
        </button>
      </div>
    </div>

    <p class="mt-5 text-center text-xs text-zinc-600">
      サーバーの <code class="font-mono text-zinc-500">keynel.json</code> から api_key をコピーしてください
    </p>
  </div>
</div>
