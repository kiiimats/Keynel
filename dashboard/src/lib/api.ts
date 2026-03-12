import type { Tunnel, ServerStatus } from './types';

export class KeynelAPI {
  constructor(
    private targetUrl: string,
    private apiKey: string
  ) {}

  // target と key をクエリパラメータで渡す
  private proxyUrl(path: string): string {
    const params = new URLSearchParams({
      target: this.targetUrl,
      key: this.apiKey,
      path,
    });
    return `/proxy?${params.toString()}`;
  }

  private async request(path: string, options: RequestInit = {}): Promise<Response> {
    return fetch(this.proxyUrl(path), {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...(options.headers ?? {}),
      },
    });
  }

  async getStatus(): Promise<ServerStatus> {
    const res = await this.request('/api/status');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  }

  async getTunnels(): Promise<Tunnel[]> {
    const res = await this.request('/api/tunnels');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  }

  async createTunnel(proto: 'tcp' | 'udp', clientPort: number, serverPort: number): Promise<Tunnel> {
    const res = await this.request('/api/tunnels', {
      method: 'POST',
      body: JSON.stringify({ proto, client_port: clientPort, server_port: serverPort }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      throw new Error(err.error ?? 'Unknown error');
    }
    return res.json();
  }

  async patchTunnel(id: string, patch: { enabled: boolean; rate_limit: boolean }): Promise<Tunnel> {
    const res = await this.request(`/api/tunnels/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  }

  async deleteTunnel(id: string): Promise<void> {
    const res = await this.request(`/api/tunnels/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
  }

  sseProxyUrl(): string {
    const params = new URLSearchParams({
      target: this.targetUrl,
      key: this.apiKey,
    });
    return `/proxy/sse?${params.toString()}`;
  }

  getTargetUrl(): string {
    return this.targetUrl;
  }
}
