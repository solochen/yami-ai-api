import type { CSSProperties, ReactNode } from "react";

// 品牌图标系统：双色线面结合（B 档）。
// 规则：每枚图标 = 1 个品牌橙实心面 + 若干 currentColor 线稿；
// 线宽 1.9、圆头圆角、24×24 视图框、四周留白 ≥ 3。
// 颜色全部继承父级文字色，因此浅色/深色模式各自由容器的 text 色决定，
// 无需为两套主题准备两份资源。
const ACCENT = "#FF8711";
const S = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.9,
  strokeLinecap: "round",
  strokeLinejoin: "round",
} as const;

const ACCENT_SHAPE = {
  fill: ACCENT,
  stroke: "currentColor",
  strokeWidth: 1.9,
  strokeLinecap: "round",
  strokeLinejoin: "round",
} as const;

const ICONS: Record<string, ReactNode> = {
  // ===== 一级导航 =====
  "nav.models": (
    <>
      <rect x="8.4" y="3.2" width="10.6" height="10.6" rx="3" {...S} />
      <rect x="5" y="9.6" width="10.6" height="10.6" rx="3" {...ACCENT_SHAPE} />
      <path d="M8.6 14.9h3.4" {...S} strokeWidth={1.6} />
    </>
  ),
  "nav.agents": (
    <>
      <path d="M12 2.8v2.4" {...S} />
      <rect x="5.6" y="6.4" width="12.8" height="12" rx="4.2" {...ACCENT_SHAPE} />
      <circle cx="9.5" cy="12.4" r="1.15" fill="currentColor" stroke="none" />
      <circle cx="14.5" cy="12.4" r="1.15" fill="currentColor" stroke="none" />
      <path d="M9.6 16.2h4.8" {...S} strokeWidth={1.6} />
    </>
  ),
  "nav.gallery": (
    <>
      <circle cx="12" cy="12" r="9" {...S} />
      <ellipse cx="12" cy="12" rx="9" ry="3.6" {...S} strokeWidth={1.5} opacity="0.55" transform="rotate(-28 12 12)" />
      <circle cx="12" cy="12" r="2.8" fill={ACCENT} stroke="none" />
      <path d="M17.4 6.6l-2.6 2.6" {...S} strokeWidth={1.5} />
    </>
  ),
  "nav.api": (
    <>
      <circle cx="12" cy="12" r="3.1" {...ACCENT_SHAPE} />
      <path d="M8.2 5.6H6.4A2.4 2.4 0 004 8v8a2.4 2.4 0 002.4 2.4h1.8" {...S} />
      <path d="M15.8 5.6h1.8A2.4 2.4 0 0120 8v8a2.4 2.4 0 01-2.4 2.4h-1.8" {...S} />
    </>
  ),

  // ===== 二级分类 =====
  "chip.all": (
    <>
      <rect x="4" y="4" width="6.4" height="6.4" rx="2" fill={ACCENT} stroke="none" />
      <rect x="13.6" y="4" width="6.4" height="6.4" rx="2" {...S} strokeWidth={1.7} />
      <rect x="4" y="13.6" width="6.4" height="6.4" rx="2" {...S} strokeWidth={1.7} />
      <rect x="13.6" y="13.6" width="6.4" height="6.4" rx="2" {...S} strokeWidth={1.7} />
    </>
  ),
  "chip.video": (
    <>
      <rect x="3.4" y="5.4" width="17.2" height="13.2" rx="3.4" {...S} />
      <path d="M10.4 9.6l4.4 2.4-4.4 2.4z" fill={ACCENT} stroke="none" />
    </>
  ),
  "chip.image": (
    <>
      <rect x="3.4" y="4.6" width="17.2" height="14.8" rx="3.4" {...S} />
      <circle cx="9" cy="9.8" r="1.9" fill={ACCENT} stroke="none" />
      <path d="M4.6 16.6l4.2-4.2 3.2 3.2 2.6-2.4 3.8 3.4" {...S} strokeWidth={1.7} />
    </>
  ),
  "chip.content": (
    <>
      <path d="M6.2 3.6h7.6l4 4v12.2a1.6 1.6 0 01-1.6 1.6H6.2a1.6 1.6 0 01-1.6-1.6V5.2a1.6 1.6 0 011.6-1.6z" {...S} />
      <path d="M13.4 3.8V8h4.2" {...S} strokeWidth={1.6} />
      <rect x="8" y="12.4" width="5.6" height="3.4" rx="1.2" fill={ACCENT} stroke="none" />
      <path d="M8 18.2h7.6" {...S} strokeWidth={1.6} />
    </>
  ),
  "chip.aiCreate": (
    <>
      <path d="M12 3.2l1.9 4.6 4.6 1.9-4.6 1.9-1.9 4.6-1.9-4.6-4.6-1.9 4.6-1.9z" fill={ACCENT} stroke="none" />
      <path d="M17.8 15.2l.9 2.1 2.1.9-2.1.9-.9 2.1-.9-2.1-2.1-.9 2.1-.9z" {...S} strokeWidth={1.5} />
    </>
  ),
  "chip.assistant": (
    <>
      <path d="M4 6.6A2.6 2.6 0 016.6 4h10.8A2.6 2.6 0 0120 6.6v6.2a2.6 2.6 0 01-2.6 2.6H10l-4.2 3.4v-3.4H6.6A2.6 2.6 0 014 12.8z" {...S} />
      <circle cx="9.2" cy="9.7" r="1.2" fill={ACCENT} stroke="none" />
      <circle cx="14.8" cy="9.7" r="1.2" fill={ACCENT} stroke="none" />
    </>
  ),
  "chip.mine": (
    <>
      <circle cx="12" cy="8.6" r="3.9" {...ACCENT_SHAPE} />
      <path d="M4.8 20.4c0-3.6 3.2-5.8 7.2-5.8s7.2 2.2 7.2 5.8" {...S} />
    </>
  ),

  // ===== 智能体 =====
  general_creative_agent: (
    <>
      <path d="M5 19l9.4-9.4" {...S} />
      <path d="M15.6 3.4l1.5 3.5 3.5 1.5-3.5 1.5-1.5 3.5-1.5-3.5-3.5-1.5 3.5-1.5z" fill={ACCENT} stroke="none" />
      <path d="M6.6 16.4l1.2 2.8" {...S} strokeWidth={1.5} opacity="0.6" />
    </>
  ),
  infinite_canvas: (
    <>
      <path d="M8.4 8.2a5.2 5.2 0 100 7.6l7.2-7.6a5.2 5.2 0 110 7.6l-2.4-2.6" {...S} />
      <circle cx="12" cy="12" r="2.1" fill={ACCENT} stroke="none" />
    </>
  ),
  one_click_viral_remake: (
    <>
      <circle cx="11" cy="11" r="6.6" {...S} />
      <circle cx="11" cy="11" r="2.4" fill={ACCENT} stroke="none" />
      <path d="M16 16l4.4 4.4" {...S} />
      <path d="M17.6 4.4l.9 2 2 .9-2 .9-.9 2-.9-2-2-.9 2-.9z" {...S} strokeWidth={1.4} />
    </>
  ),
  viral_remake: (
    <>
      <path d="M12.6 3.6c2.6 2.6 2 4.8.6 6.4 2.2-0.6 3.4 0.6 3.4 2.4 1.8 1.2 2.4 3 1.4 5.2-1 2.2-3 3.4-6 3.4s-5.4-1.4-6.4-3.9c-1-2.4 0-4.7 2-6.6 0 1.6.4 2.6 1.2 3.2.6-4 2.2-7 3.8-10.1z" fill={ACCENT} stroke="none" />
      <path d="M12 18.6c1.4 0 2.2-.8 2.2-2s-.9-2-1.6-3c-.7 1-1.4 1.9-1.4 3s.5 2 1.4 2z" fill="currentColor" stroke="none" opacity="0.55" />
    </>
  ),
  video_remake: (
    <>
      <rect x="3.4" y="6.6" width="14" height="10.8" rx="3" {...S} />
      <path d="M20.6 9.4v5.2l-3.2-2.6z" fill={ACCENT} stroke="none" />
      <path d="M6.4 4.4l-2 2.2M12.4 4.4l-2 2.2" {...S} strokeWidth={1.5} opacity="0.6" />
    </>
  ),
  ecommerce_video: (
    <>
      <path d="M4.6 8h14.8l-1.2 11.2a1.8 1.8 0 01-1.8 1.6H7.6a1.8 1.8 0 01-1.8-1.6z" {...S} />
      <path d="M8.6 8.4V6.6a3.4 3.4 0 016.8 0v1.8" {...S} />
      <path d="M10.6 13.2l3.4 1.9-3.4 1.9z" fill={ACCENT} stroke="none" />
    </>
  ),
  ecommerce_image: (
    <>
      <path d="M4.4 8.4L12 4l7.6 4.4v7.2L12 20l-7.6-4.4z" {...S} />
      <path d="M4.8 8.6L12 13l7.2-4.4M12 13v6.8" {...S} strokeWidth={1.5} opacity="0.5" />
      <path d="M17.8 3.2l.9 2 2 .9-2 .9-.9 2-.9-2-2-.9 2-.9z" fill={ACCENT} stroke="none" />
    </>
  ),
  ecom_factory: (
    <>
      <rect x="3.4" y="6.6" width="9.2" height="7.4" rx="2" {...S} strokeWidth={1.7} />
      <rect x="6.4" y="10.4" width="9.2" height="7.4" rx="2" {...ACCENT_SHAPE} strokeWidth={1.7} />
      <path d="M17.6 7.4h3M19.1 5.9v3" {...S} strokeWidth={1.6} opacity="0.7" />
      <path d="M15.8 18.6h4.6" {...S} strokeWidth={1.6} opacity="0.7" />
    </>
  ),
  general_image: (
    <>
      <rect x="3.4" y="5" width="13.4" height="14" rx="3.2" {...S} />
      <circle cx="8.4" cy="10" r="1.7" fill={ACCENT} stroke="none" />
      <path d="M4.6 16.6l3.6-3.6 2.8 2.8" {...S} strokeWidth={1.6} />
      <path d="M18 5.6v5M15.5 8.1h5" {...S} />
    </>
  ),
  video_creation: (
    <>
      <rect x="3.4" y="6.4" width="13.4" height="11.2" rx="2.8" {...S} />
      <path d="M17 10.4h1.2l2.4-2v7.2l-2.4-2H17z" {...ACCENT_SHAPE} strokeWidth={1.5} />
      <path d="M6.6 4.6l-1.4 1.8M11 4.6l-1.4 1.8M15.4 4.6l-1.4 1.8" {...S} strokeWidth={1.5} opacity="0.6" />
    </>
  ),
  video_creation_v2: (
    <>
      <rect x="3.2" y="5.6" width="17.6" height="12.8" rx="3.2" {...S} />
      <path d="M3.6 8.6h16.8M8.4 5.8v3M14 5.8v3" {...S} strokeWidth={1.5} opacity="0.55" />
      <path d="M10.4 13.4l4 2.2-4 2.2z" fill={ACCENT} stroke="none" />
    </>
  ),
  content_image_post: (
    <>
      <rect x="3.4" y="4.4" width="11" height="15.2" rx="2.8" {...S} />
      <path d="M6.2 8.4h5.4M6.2 11.6h5.4" {...S} strokeWidth={1.5} opacity="0.6" />
      <rect x="13.4" y="11" width="7.2" height="8.6" rx="2.2" fill={ACCENT} stroke="none" />
      <path d="M15 17.2l1.8-1.8 1.4 1.4" stroke="currentColor" strokeWidth={1.5} fill="none" strokeLinecap="round" />
    </>
  ),
  ai_comic_drama: (
    <>
      <rect x="3.4" y="4.4" width="17.2" height="15.2" rx="3.2" {...S} />
      <path d="M12 4.6v14.8M3.6 12h16.8" {...S} strokeWidth={1.5} opacity="0.55" />
      <path d="M6.2 8.6l2.2 2.2M8.4 8.6l-2.2 2.2" stroke={ACCENT} strokeWidth={2} strokeLinecap="round" />
      <circle cx="16.2" cy="15.6" r="1.8" fill={ACCENT} stroke="none" />
    </>
  ),
  realtime_avatar: (
    <>
      <path d="M4 7.4A2.8 2.8 0 016.8 4.6h10.4A2.8 2.8 0 0120 7.4v6a2.8 2.8 0 01-2.8 2.8h-6.6L6 19.8v-3.6H6.8A2.8 2.8 0 014 13.4z" {...S} />
      <circle cx="12" cy="9.2" r="2.4" fill={ACCENT} stroke="none" />
      <path d="M8.6 14.4c.8-1.4 2-2 3.4-2s2.6.6 3.4 2" {...S} strokeWidth={1.6} />
    </>
  ),
  viral_video_breakdown: (
    <>
      <circle cx="10.6" cy="10.6" r="6.4" {...S} />
      <path d="M15.4 15.4l4.6 4.6" {...S} />
      <rect x="7.6" y="8.6" width="6" height="4" rx="1.2" fill={ACCENT} stroke="none" />
    </>
  ),
  extract_product_images: (
    <>
      <path d="M3.8 9.6L12 5.2l8.2 4.4-8.2 4.4z" {...ACCENT_SHAPE} strokeWidth={1.7} />
      <path d="M3.8 9.6v5.2L12 19l8.2-4.2V9.6" {...S} strokeWidth={1.6} />
      <path d="M9 14.6l1.8 1.8 4-4" {...S} strokeWidth={1.6} />
    </>
  ),
  video_upscale: (
    <>
      <rect x="3.4" y="5.4" width="17.2" height="13.2" rx="3.2" {...S} />
      <path d="M12 16V9.4M9.4 12L12 9.4l2.6 2.6" {...S} strokeWidth={1.8} />
      <circle cx="12" cy="12" r="6" fill={ACCENT} stroke="none" opacity="0.16" />
    </>
  ),
  product_refine: (
    <>
      <rect x="3.4" y="4.4" width="17.2" height="15.2" rx="3.2" {...S} />
      <path d="M7.2 8.4v7.2M12 6.6v10.8M16.8 9.6v4.8" {...S} />
      <circle cx="7.2" cy="10.6" r="1.5" fill={ACCENT} stroke="none" />
      <circle cx="12" cy="13.4" r="1.5" fill={ACCENT} stroke="none" />
      <circle cx="16.8" cy="11.2" r="1.5" fill={ACCENT} stroke="none" />
    </>
  ),
  subtitle_remove: (
    <>
      <rect x="3.4" y="5.6" width="17.2" height="12.8" rx="3.2" {...S} />
      <path d="M6.4 15h4M12.6 15h2.2" {...S} strokeWidth={1.6} opacity="0.6" />
      <path d="M8 8.6h8" stroke={ACCENT} strokeWidth={2.4} strokeLinecap="round" />
      <path d="M5.6 18.4L18.4 5.6" {...S} strokeWidth={1.7} opacity="0.75" />
    </>
  ),
  ai_photo_studio: (
    <>
      <path d="M3.6 9.6A2.8 2.8 0 016.4 6.8h1.8l1.2-2h5.2l1.2 2h1.8a2.8 2.8 0 012.8 2.8v7.6a2.8 2.8 0 01-2.8 2.8H6.4a2.8 2.8 0 01-2.8-2.8z" {...S} />
      <circle cx="12" cy="13.2" r="3.4" fill={ACCENT} stroke="none" />
    </>
  ),
  ai_virtual_tryon: (
    <>
      <circle cx="12" cy="4.6" r="1.6" {...S} strokeWidth={1.6} />
      <path d="M4.6 8.2l7.4 2 7.4-2" {...S} />
      <path d="M6.6 20.4l1.6-8.6M17.4 20.4l-1.6-8.6" {...S} strokeWidth={1.6} />
      <path d="M12 10.4v4.2" stroke={ACCENT} strokeWidth={2.6} strokeLinecap="round" />
    </>
  ),
  ai_novel_workshop: (
    <>
      <path d="M4 5.4A2.4 2.4 0 016.4 3h11.2A2.4 2.4 0 0120 5.4v13.2a2.4 2.4 0 01-2.4 2.4H6.4A2.4 2.4 0 014 18.6z" {...S} />
      <path d="M12 3.2v17.6" {...S} strokeWidth={1.5} opacity="0.55" />
      <path d="M7 7.6h2.6M7 11.2h2.6M14.4 7.6H17M14.4 11.2H17" {...S} strokeWidth={1.5} opacity="0.75" />
      <path d="M15 20l4.6-4.6" stroke={ACCENT} strokeWidth={2.2} strokeLinecap="round" />
    </>
  ),
  video_redraw: (
    <>
      <rect x="3.4" y="6.4" width="13" height="11.2" rx="3" {...S} />
      <path d="M16.4 10.6l4.2-2.2v7.2l-4.2-2.2z" fill={ACCENT} stroke="none" />
      <path d="M7.4 3.4l2.4 2.4-2.4 2.4" {...S} strokeWidth={1.5} opacity="0.7" />
    </>
  ),
  renovation_budget_agent: (
    <>
      <rect x="4.4" y="3.2" width="15.2" height="17.6" rx="3.2" {...S} />
      <rect x="7.2" y="6" width="9.6" height="3.6" rx="1.4" fill={ACCENT} stroke="none" />
      <path d="M7.6 12.6h1.8M11 12.6h1.8M14.4 12.6h1.8M7.6 16.4h1.8M11 16.4h1.8M14.4 16.4h1.8" {...S} strokeWidth={1.7} opacity="0.75" />
    </>
  ),
};

export type BrandIconName = keyof typeof ICONS | string;

export function BrandIcon({
  name,
  className,
  style,
  title,
}: {
  name: BrandIconName;
  className?: string;
  style?: CSSProperties;
  title?: string;
}) {
  const body = ICONS[name];
  if (!body) return null;
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      style={style}
      role={title ? "img" : undefined}
      aria-label={title || undefined}
      aria-hidden={title ? undefined : true}
      focusable="false"
    >
      {title ? <title>{title}</title> : null}
      {body}
    </svg>
  );
}

export function hasBrandIcon(name: string) {
  return Boolean(ICONS[name]);
}
