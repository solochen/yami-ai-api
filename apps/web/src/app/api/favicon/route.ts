import { NextResponse } from "next/server";
import { getPublicSystemConfig } from "@/lib/public-config";

export const dynamic = "force-dynamic";

function staticFallback() {
  return new NextResponse(null, {
    status: 307,
    headers: { Location: "/assets/yihangai-logo.png", "Cache-Control": "no-cache" },
  });
}

export async function GET(req: Request) {
  const cfg = await getPublicSystemConfig();
  const favicon = String(cfg.site_favicon || cfg.site_logo || "").trim();
  if (favicon) {
    // 站点图标可能是站内相对路径（/media/...）。NextResponse.redirect 只接受绝对地址，
    // 直接传相对路径会抛错并让 /site-icon 返回 500。
    let target = favicon;
    try {
      target = new URL(favicon, req.url).toString();
    } catch {
      return staticFallback();
    }
    const res = NextResponse.redirect(target);
    res.headers.set("Cache-Control", "no-cache");
    return res;
  }
  return staticFallback();
}
