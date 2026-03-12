export const onRequest = async (context: {
  request: Request;
  env: Record<string, unknown>;
}): Promise<Response> => {
  const { request } = context;

  if (request.method === 'OPTIONS') {
    return new Response(null, { status: 204, headers: cors() });
  }

  const url    = new URL(request.url);
  const target = url.searchParams.get('target');
  const key    = url.searchParams.get('key');
  const path   = url.searchParams.get('path') ?? '/';

  if (!target || !key) {
    return Response.json({ error: 'missing target or key' }, { status: 400, headers: cors() });
  }

  const isSSE = path === '/api/events';
  const upstreamUrl = `${target}${path}`;

  const upstreamHeaders: Record<string, string> = { 'X-API-Key': key };
  if (isSSE) {
    upstreamHeaders['Accept'] = 'text/event-stream';
    upstreamHeaders['Cache-Control'] = 'no-cache';
  } else {
    upstreamHeaders['Content-Type'] = 'application/json';
  }

  const body = ['GET', 'HEAD'].includes(request.method) ? undefined : await request.text();

  let upstream: Response;
  try {
    upstream = await fetch(upstreamUrl, {
      method: request.method,
      headers: upstreamHeaders,
      body,
    });
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'unknown error';
    return Response.json({ error: msg }, { status: 502, headers: cors() });
  }

  const contentType = isSSE
    ? 'text/event-stream'
    : (upstream.headers.get('Content-Type') ?? 'application/json');

  return new Response(upstream.body, {
    status: upstream.status,
    headers: { 'Content-Type': contentType, 'Cache-Control': 'no-cache', ...cors() },
  });
};

function cors(): Record<string, string> {
  return {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, POST, PATCH, DELETE, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type, X-API-Key',
  };
}
