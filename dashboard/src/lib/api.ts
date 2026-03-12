import type { Tunnel, ServerStatus } from './types';

// ダッシュボードはサーバー自身から配信されるので
// 相対URLで直接 /api/* にアクセスできる。
// キーはクエリパラメータで渡す（SSE用）またはヘッダーで渡す。

export class KeynelAPI {
  constructor(private key: string) {}

  private async req(path: string, init: RequestInit = {}): Promise<Response> {
    return fetch(path, {
      ...init,
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': this.key,
        ...(init.headers ?? {}),
      },
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
    if (!r.ok) {
      const e = await r.json().catch(() => ({}));
      throw new Error(e.error ?? `HTTP ${r.status}`);
    }
    return r.json();
  }

  async patchTunnel(id: string, patch: { enabled: boolean; rate_limit: boolean }): Promise<Tunnel> {
    const r = await this.req(`/api/tunnels/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
    return r.json();
  }

  async deleteTunnel(id: string): Promise<void> {
    const r = await this.req(`/api/tunnels/${id}`, { method: 'DELETE' });
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
  }

  // SSE: クエリパラメータでキーを渡す（EventSource はカスタムヘッダー不可）
  sseUrl(): string {
    return `/api/events?key=${encodeURIComponent(this.key)}`;
  }
}
