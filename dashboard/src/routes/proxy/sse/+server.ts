import type { RequestHandler } from '@sveltejs/kit';

// SSE を Worker 経由でプロキシする。
// クエリパラメータ:
//   target : 転送先ベースURL (例: http://220.158.19.132:7002)
//   key    : APIキー

export const GET: RequestHandler = async ({ url }) => {
  const target = url.searchParams.get('target');
  const key = url.searchParams.get('key');

  if (!target || !key) {
    return new Response('missing target or key', { status: 400 });
  }

  const sseUrl = `${target}/api/events?key=${encodeURIComponent(key)}`;

  const upstream = await fetch(sseUrl, {
    headers: {
      'X-API-Key': key,
      Accept: 'text/event-stream',
      'Cache-Control': 'no-cache',
    },
  });

  if (!upstream.ok || !upstream.body) {
    return new Response('upstream error', { status: 502 });
  }

  // Cloudflare Workers は ReadableStream のパススルーをサポートしている
  return new Response(upstream.body, {
    status: 200,
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Access-Control-Allow-Origin': '*',
      Connection: 'keep-alive',
    },
  });
};
