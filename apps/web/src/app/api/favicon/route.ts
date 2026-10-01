import { NextResponse } from "next/server";
import { getPublicSystemConfig } from "@/lib/public-config";

export const dynamic = "force-dynamic";

function svgFallback() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="16" fill="#ff8711"/><path d="M32 9c1.7 9.4 3.6 14.9 6.6 17.9 2.9 2.9 8.4 4.9 17.9 6.5-9.5 1.6-15 3.6-17.9 6.6-3 2.9-4.9 8.4-6.6 17.8-1.6-9.4-3.6-14.9-6.5-17.8-3-3-8.5-5-18-6.6 9.5-1.6 15-3.6 18-6.5 2.9-3 4.9-8.5 6.5-17.9Z" fill="#241006"/><circle cx="45.5" cy="17.5" r="3.5" fill="#fff7ed"/></svg>`;
}

export async function GET() {
  const cfg = await getPublicSystemConfig();
  const favicon = String(cfg.site_favicon || cfg.site_logo || "").trim();
  if (favicon) {
    return NextResponse.redirect(favicon);
  }
  return new NextResponse(svgFallback(), {
    headers: {
      "Content-Type": "image/svg+xml; charset=utf-8",
      "Cache-Control": "public, max-age=300",
    },
  });
}
