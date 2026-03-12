<script lang="ts">
  export let checked = false;
  export let color: 'green' | 'amber' = 'green';
  export let disabled = false;
  export let onChange: (v: boolean) => void = () => {};
</script>

<button
  type="button"
  role="switch"
  aria-checked={checked}
  {disabled}
  on:click={() => !disabled && onChange(!checked)}
  class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border transition-all duration-200 focus:outline-none disabled:cursor-not-allowed disabled:opacity-40
    {checked
      ? color === 'green'
        ? 'border-emerald-500/40 bg-emerald-500/20'
        : 'border-amber-500/40 bg-amber-500/20'
      : 'border-zinc-700 bg-zinc-800/60'}"
>
  <span
    class="inline-block h-3.5 w-3.5 rounded-full shadow transition-all duration-200
      {checked ? 'translate-x-4' : 'translate-x-0.5'}
      {checked
        ? color === 'green'
          ? 'bg-emerald-400'
          : 'bg-amber-400'
        : 'bg-zinc-500'}"
  ></span>
</button>
