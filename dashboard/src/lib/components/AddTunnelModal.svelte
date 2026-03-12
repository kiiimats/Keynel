<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  export let onAdd: (proto: 'tcp' | 'udp', clientPort: number, serverPort: number) => Promise<void>;

  const dispatch = createEventDispatcher();

  let proto: 'tcp' | 'udp' = 'tcp';
  let clientPort = '';
  let serverPort = '';
  let error = '';
  let loading = false;

  async function submit() {
    error = '';
    const cp = parseInt(clientPort);
    const sp = parseInt(serverPort);
    if (!cp || !sp || cp < 1 || cp > 65535 || sp < 1 || sp > 65535) {
      error = 'ポート番号は 1〜65535 で指定してください';
      return;
    }
    loading = true;
    try {
      await onAdd(proto, cp, sp);
      dispatch('close');
    } catch (e: any) {
      error = e.message ?? 'エラーが発生しました';
    }
    loading = false;
  }

  function handleKey(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('close');
  }
</script>

<svelte:window on:keydown={handleKey} />

<!-- Backdrop -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
  on:click|self={() => dispatch('close')}
  role="dialog"
  aria-modal="true"
>
  <div class="w-full max-w-md rounded-2xl border border-zinc-700/60 bg-zinc-900 shadow-2xl shadow-black/60">
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-zinc-800 px-6 py-4">
      <h2 class="text-sm font-semibold text-white">トンネルを追加</h2>
      <button
        on:click={() => dispatch('close')}
        class="flex h-7 w-7 items-center justify-center rounded-md text-zinc-500 hover:bg-zinc-800 hover:text-zinc-300 transition-colors"
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>

    <!-- Body -->
    <div class="space-y-5 px-6 py-5">
      <!-- Protocol -->
      <div class="space-y-2">
        <label class="text-xs font-medium uppercase tracking-widest text-zinc-500">プロトコル</label>
        <div class="flex gap-2">
          {#each ['tcp', 'udp'] as p}
            <button
              type="button"
              on:click={() => proto = p as 'tcp' | 'udp'}
              class="flex-1 rounded-lg border py-2 font-mono text-sm font-semibold tracking-wider transition-all duration-150
                {proto === p
                  ? p === 'tcp'
                    ? 'border-sky-500/50 bg-sky-500/10 text-sky-300'
                    : 'border-violet-500/50 bg-violet-500/10 text-violet-300'
                  : 'border-zinc-700 bg-zinc-800/40 text-zinc-500 hover:border-zinc-600 hover:text-zinc-400'}"
            >
              {p.toUpperCase()}
            </button>
          {/each}
        </div>
      </div>

      <!-- Ports -->
      <div class="grid grid-cols-[1fr_28px_1fr] items-end gap-2">
        <div class="space-y-2">
          <label class="text-xs font-medium uppercase tracking-widest text-zinc-500">Local ポート</label>
          <input
            type="number"
            bind:value={clientPort}
            placeholder="25565"
            min="1" max="65535"
            class="w-full rounded-lg border border-zinc-700 bg-zinc-800/60 px-3 py-2.5 font-mono text-sm text-white placeholder-zinc-600 outline-none transition-colors focus:border-zinc-500 focus:ring-1 focus:ring-zinc-500/30"
          />
        </div>
        <div class="flex items-center justify-center pb-2.5 text-zinc-600">→</div>
        <div class="space-y-2">
          <label class="text-xs font-medium uppercase tracking-widest text-zinc-500">Internet ポート</label>
          <input
            type="number"
            bind:value={serverPort}
            placeholder="10000"
            min="1" max="65535"
            class="w-full rounded-lg border border-zinc-700 bg-zinc-800/60 px-3 py-2.5 font-mono text-sm text-white placeholder-zinc-600 outline-none transition-colors focus:border-zinc-500 focus:ring-1 focus:ring-zinc-500/30"
          />
        </div>
      </div>

      <!-- Preview -->
      {#if clientPort || serverPort}
        <div class="rounded-lg border border-zinc-800 bg-zinc-800/30 px-4 py-2.5">
          <p class="font-mono text-xs text-zinc-400">
            <span class="text-zinc-600">localhost:</span><span class="text-white">{clientPort || '?'}</span>
            <span class="mx-2 text-zinc-600">→</span>
            <span class="text-zinc-600">server:</span><span class="text-white">{serverPort || '?'}</span>
            <span class="ml-2 text-zinc-600">({proto.toUpperCase()})</span>
          </p>
        </div>
      {/if}

      {#if error}
        <p class="rounded-lg border border-red-900/40 bg-red-950/30 px-4 py-2.5 text-xs text-red-400">{error}</p>
      {/if}
    </div>

    <!-- Footer -->
    <div class="flex gap-2 border-t border-zinc-800 px-6 py-4">
      <button
        on:click={() => dispatch('close')}
        class="flex-1 rounded-lg border border-zinc-700 bg-zinc-800/50 py-2.5 text-sm text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-zinc-300"
      >
        キャンセル
      </button>
      <button
        on:click={submit}
        disabled={loading || !clientPort || !serverPort}
        class="flex-1 rounded-lg bg-white py-2.5 text-sm font-semibold text-black transition-all hover:bg-zinc-100 disabled:cursor-not-allowed disabled:opacity-40"
      >
        {loading ? '追加中...' : '追加する'}
      </button>
    </div>
  </div>
</div>
