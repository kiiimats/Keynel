import type { Tunnel, ServerStatus } from './types';

export class KeynelAPI {
  constructor(private target: string, private key: string) {}

  private url(path: string) {
    return `/proxy?${new URLSearchParams({ target: this.target, key: this.key, path })}`;
  }

  private async req(path: string, init: RequestInit = {}) {
    return fetch(this.url(path), {
      ...init,
      headers: { 'Content-Type': 'application/json', ...(init.headers ?? {}) },
    });
  }

  async getStatus(): Promise<ServerStatus> {
    const r = await this.req('/api/status');
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    return r.json();
  }

  async createTunnel(proto: 'tcp' | 'udp', clientPort: number, serverPort: number): Promise<Tunnel> {
    const r = await this.req('/api/tunnels', {
      method: 'POST',
      body: JSON.stringify({ proto, client_port: clientPort, server_port: serverPort }),
    });
    if (!r.ok) { const e = await r.json().catch(() => ({})); throw new Error(e.error ?? `HTTP ${r.status}`); }
    return r.json();
  }

  async patchTunnel(id: string, patch: { enabled: boolean; rate_limit: boolean }): Promise<Tunnel> {
    const r = await this.req(`/api/tunnels/${id}`, { method: 'PATCH', body: JSON.stringify(patch) });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    return r.json();
  }

  async deleteTunnel(id: string) {
    const r = await this.req(`/api/tunnels/${id}`, { method: 'DELETE' });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
  }

  sseUrl() {
    return `/proxy?${new URLSearchParams({ target: this.target, key: this.key, path: '/api/events' })}`;
  }
}
