import type { NextRequest } from "next/server";

// 媒体同源代理：把 /media/<bucket>/<key> 转发到对象存储。
// 目的：媒体地址在库里只存相对路径，浏览器和 Next 图片优化器都只访问自己的域名，
// 对象存储不必对外暴露，容器内也能解析到（MEDIA_PROXY_TARGET=http://minio:9000）。
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
    // 不要把内部存储地址回给浏览器，排障信息留在服务端日志。
    console.error("media proxy failed", target, error);
    return new Response("media proxy failed", { status: 502 });
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
