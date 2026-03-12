export interface Tunnel {
  id: string;
  proto: 'tcp' | 'udp';
  client_port: number;
  server_port: number;
  enabled: boolean;
  rate_limit: boolean;
  created_at: string;
}

export interface ServerStatus {
  client_connected: boolean;
  tunnel_count: number;
  active_conns: number;
  banned_ips: number;
}
