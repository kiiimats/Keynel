import { writable, derived } from 'svelte/store';
import type { Tunnel, ServerStatus } from './types';

export const tunnels = writable<Tunnel[]>([]);
export const status = writable<ServerStatus>({
  client_connected: false,
  tunnel_count: 0,
  active_conns: 0,
  banned_ips: 0,
});
export const sseConnected = writable(false);

export const enabledCount = derived(tunnels, ($t) => $t.filter((t) => t.enabled).length);
export const totalCount = derived(tunnels, ($t) => $t.length);
