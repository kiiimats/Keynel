<script lang="ts">
  export let value: number | string;
  export let label: string;
  export let accent: 'default' | 'green' | 'amber' | 'red' = 'default';
</script>

<div class="rounded-xl border border-zinc-800/60 bg-zinc-900/40 px-5 py-4 backdrop-blur-sm">
  <div
    class="font-mono text-3xl font-semibold tracking-tight
      {accent === 'green' ? 'text-emerald-400' :
       accent === 'amber' ? 'text-amber-400' :
       accent === 'red'   ? 'text-red-400' :
                            'text-white'}"
  >
    {value}
  </div>
  <div class="mt-1 text-xs uppercase tracking-widest text-zinc-500">{label}</div>
</div>
