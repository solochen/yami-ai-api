import { NextResponse } from "next/server";
import { getPublicSystemConfig } from "@/lib/public-config";

export const dynamic = "force-dynamic";

function staticFallback() {
  return new NextResponse(null, {
    status: 307,
    headers: { Location: "/assets/yihangai-logo.png" },
  });
}

export async function GET() {
  const cfg = await getPublicSystemConfig();
  const favicon = String(cfg.site_favicon || cfg.site_logo || "").trim();
  if (favicon) {
    return NextResponse.redirect(favicon);
  }
  return staticFallback();
}
