import type { NextRequest } from "next/server";

// 与 web 端一致的媒体同源代理：后台也需要展示模型图标、画廊封面与站点 Logo。
const TARGET = (process.env.MEDIA_PROXY_TARGET || "http://localhost:9000").replace(/\/+$/, "");

export const dynamic = "force-dynamic";

const PASS_HEADERS = [
  "content-type",
  "content-length",
  "content-range",
  "accept-ranges",
  "etag",
  "last-modified",
];

async function proxy(request: NextRequest, rawPath: string[]) {
  const segments = (rawPath || []).map((segment) => encodeURIComponent(decodeURIComponent(segment)));
  const target = `${TARGET}/${segments.join("/")}`;
  const headers: Record<string, string> = {};
  const range = request.headers.get("range");
  if (range) headers.Range = range;
  const ifNoneMatch = request.headers.get("if-none-match");
  if (ifNoneMatch) headers["If-None-Match"] = ifNoneMatch;

  let upstream: Response;
  try {
    upstream = await fetch(target, { headers, cache: "no-store", signal: AbortSignal.timeout(15_000) });
  } catch (error) {
    return new Response(`media proxy failed: ${(error as Error).message}`, { status: 502 });
  }

  const out = new Headers();
  for (const name of PASS_HEADERS) {
    const value = upstream.headers.get(name);
    if (value) out.set(name, value);
  }
  out.set("cache-control", "public, max-age=31536000, immutable");
  out.set("x-content-type-options", "nosniff");

  return new Response(upstream.body, { status: upstream.status, headers: out });
}

export async function GET(request: NextRequest, { params }: { params: Promise<{ path?: string[] }> }) {
  const { path } = await params;
  return proxy(request, path || []);
}

export async function HEAD(request: NextRequest, { params }: { params: Promise<{ path?: string[] }> }) {
  const { path } = await params;
  const response = await proxy(request, path || []);
  return new Response(null, { status: response.status, headers: response.headers });
}
