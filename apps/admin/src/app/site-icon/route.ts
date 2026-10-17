import { NextResponse } from "next/server";
import { API_URL } from "@/lib/api";

export const dynamic = "force-dynamic";

// 容器内走 compose 服务名，浏览器端继续用 NEXT_PUBLIC_API_URL（宿主端口）
const INTERNAL_API_URL = process.env.INTERNAL_API_URL || API_URL;

function staticFallback() {
  return new NextResponse(null, {
    status: 307,
    headers: { Location: "/assets/yihangai-logo.png", "Cache-Control": "no-cache" },
  });
}

export async function GET() {
  try {
    const res = await fetch(`${INTERNAL_API_URL}/api/system-configs/public`, { cache: "no-store" });
    if (res.ok) {
      const json = await res.json();
      const cfg = json?.data || {};
      const favicon = String(cfg.site_favicon || cfg.site_logo || "").trim();
      if (favicon) {
        const redirect = NextResponse.redirect(favicon);
        redirect.headers.set("Cache-Control", "no-cache");
        return redirect;
      }
      return staticFallback();
    }
  } catch {
    /* fallback below */
  }
  return staticFallback();
}
