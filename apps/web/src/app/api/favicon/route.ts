import { NextResponse } from "next/server";
import { getPublicSystemConfig } from "@/lib/public-config";

export const dynamic = "force-dynamic";

function staticFallback() {
  return new NextResponse(null, {
    status: 307,
    headers: { Location: "/assets/yihangai-logo.png", "Cache-Control": "no-cache" },
  });
}

export async function GET() {
  const cfg = await getPublicSystemConfig();
  const favicon = String(cfg.site_favicon || cfg.site_logo || "").trim();
  if (favicon) {
    const res = NextResponse.redirect(favicon);
    res.headers.set("Cache-Control", "no-cache");
    return res;
  }
  return staticFallback();
}
