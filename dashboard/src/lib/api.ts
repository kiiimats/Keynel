import type { Tunnel, ServerStatus } from './types';

export class KeynelAPI {
  constructor(
    private baseUrl: string,
    private apiKey: string
  ) {}

  private headers() {
    return {
      'Content-Type': 'application/json',
      'X-API-Key': this.apiKey,
    };
  }

  private url(path: string) {
    return `${this.baseUrl}${path}`;
  }

  async getStatus(): Promise<ServerStatus> {
    const res = await fetch(this.url('/api/status'), { headers: this.headers() });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  }

  async getTunnels(): Promise<Tunnel[]> {
    const res = await fetch(this.url('/api/tunnels'), { headers: this.headers() });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  }

  async createTunnel(proto: 'tcp' | 'udp', clientPort: number, serverPort: number): Promise<Tunnel> {
    const res = await fetch(this.url('/api/tunnels'), {
      method: 'POST',
      headers: this.headers(),
      body: JSON.stringify({ proto, client_port: clientPort, server_port: serverPort }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      throw new Error(err.error ?? 'Unknown error');
    }
    return res.json();
  }

  async patchTunnel(id: string, patch: { enabled: boolean; rate_limit: boolean }): Promise<Tunnel> {
    const res = await fetch(this.url(`/api/tunnels/${id}`), {
      method: 'PATCH',
      headers: this.headers(),
      body: JSON.stringify(patch),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  }

  async deleteTunnel(id: string): Promise<void> {
    const res = await fetch(this.url(`/api/tunnels/${id}`), {
      method: 'DELETE',
      headers: this.headers(),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
  }

  sseUrl(): string {
    return `${this.url('/api/events')}?key=${encodeURIComponent(this.apiKey)}`;
  }
}
