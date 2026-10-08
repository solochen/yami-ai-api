import { NextResponse } from "next/server";
import { API_URL } from "@/lib/api";

export const dynamic = "force-dynamic";

function staticFallback() {
  return new NextResponse(null, {
    status: 307,
    headers: { Location: "/assets/yihangai-logo.png" },
  });
}

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/api/system-configs/public`, { cache: "no-store" });
    if (res.ok) {
      const json = await res.json();
      const cfg = json?.data || {};
      const favicon = String(cfg.site_favicon || cfg.site_logo || "").trim();
      if (favicon) return NextResponse.redirect(favicon);
      return staticFallback();
    }
  } catch {
    /* fallback below */
  }
  return staticFallback();
}
