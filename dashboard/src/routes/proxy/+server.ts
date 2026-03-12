import type { RequestHandler } from '@sveltejs/kit';

// クエリパラメータ:
//   path   : 転送先パス (例: /api/status)
//   target : 転送先ベースURL (例: http://220.158.19.132:7002)
//   key    : APIキー

async function proxyRequest(request: Request): Promise<Response> {
  const url = new URL(request.url);
  const target = url.searchParams.get('target');
  const key    = url.searchParams.get('key');
  const path   = url.searchParams.get('path') ?? '/';

  if (!target || !key) {
    return new Response(JSON.stringify({ error: 'missing target or key' }), {
      status: 400,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  const targetUrl = `${target}${path}`;
  const body = ['GET', 'HEAD'].includes(request.method) ? undefined : await request.text();

  try {
    const res = await fetch(targetUrl, {
      method: request.method,
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': key,
      },
      body,
    });

    const resBody = await res.text();
    return new Response(resBody, {
      status: res.status,
      headers: {
        'Content-Type': res.headers.get('Content-Type') ?? 'application/json',
        'Access-Control-Allow-Origin': '*',
      },
    });
  } catch (e: any) {
    return new Response(JSON.stringify({ error: e.message ?? 'proxy error' }), {
      status: 502,
      headers: { 'Content-Type': 'application/json' },
    });
  }
}

export const GET: RequestHandler    = ({ request }) => proxyRequest(request);
export const POST: RequestHandler   = ({ request }) => proxyRequest(request);
export const PATCH: RequestHandler  = ({ request }) => proxyRequest(request);
export const DELETE: RequestHandler = ({ request }) => proxyRequest(request);
export const OPTIONS: RequestHandler = () =>
  new Response(null, {
    status: 204,
    headers: {
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, POST, PATCH, DELETE, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type, X-API-Key',
    },
  });
