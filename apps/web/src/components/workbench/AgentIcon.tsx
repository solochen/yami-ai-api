import { BrandIcon, hasBrandIcon } from "@/components/icons/brandIcons";

import {
  BookOpenText,
  Calculator,
  CalendarDays,
  Camera,
  CaptionsOff,
  Clapperboard,
  Film,
  Flame,
  Handshake,
  ImagePlus,
  MessageCircleMore,
  MessageSquareQuote,
  Mic,
  MonitorUp,
  MousePointerClick,
  Newspaper,
  PackagePlus,
  PackageSearch,
  Palette,
  PanelsTopLeft,
  ReceiptText,
  Repeat2,
  ScanSearch,
  ShoppingBag,
  SlidersHorizontal,
  Sparkles,
  WandSparkles,
  Workflow,
  type LucideIcon,
} from "lucide-react";

const AGENT_ICON_BY_CODE: Record<string, LucideIcon> = {
  general_creative_agent: WandSparkles,
  renovation_budget_agent: Calculator,
  infinite_canvas: Workflow,
  one_click_viral_remake: MousePointerClick,
  viral_remake: Flame,
  video_remake: Repeat2,
  ecommerce_video: ShoppingBag,
  ecommerce_image: PackagePlus,
  general_image: ImagePlus,
  video_creation: Clapperboard,
  content_image_post: Newspaper,
  video_creation_v2: Film,
  ai_comic_drama: PanelsTopLeft,
  realtime_avatar: MessageCircleMore,
  viral_video_breakdown: ScanSearch,
  extract_product_images: PackageSearch,
  video_upscale: MonitorUp,
  product_refine: SlidersHorizontal,
  subtitle_remove: CaptionsOff,
  ai_photo_studio: Camera,
  ai_novel_workshop: BookOpenText,
  video_script_writer: Clapperboard,
  ecom_sales_copy: ShoppingBag,
  spoken_copy: Mic,
  wechat_article: Newspaper,
  moments_copy: MessageSquareQuote,
  event_plan: CalendarDays,
  franchise_copy: Handshake,
  quotation_copy: ReceiptText,
  prompt_engineer: Sparkles,
  visual_asset_extractor: Palette,
};

export function AgentIcon({
  code,
  value,
  fallback = "🤖",
  alt = "",
  className,
}: {
  code?: string;
  value?: string;
  fallback?: string;
  alt?: string;
  className?: string;
}) {
  // 品牌图标优先：双色（品牌橙实心面 + 继承文字色的线稿），随容器文字色自动适配明暗主题。
  if (code && hasBrandIcon(code)) {
    return <BrandIcon name={code} title={alt || undefined} className={className || "h-[70%] w-[70%] min-h-4 min-w-4"} />;
  }

  const SemanticIcon = code ? AGENT_ICON_BY_CODE[code] : undefined;
  if (SemanticIcon) {
    return (
      <SemanticIcon
        aria-hidden={alt ? undefined : true}
        aria-label={alt || undefined}
        className={className || "h-[70%] w-[70%] min-h-4 min-w-4"}
        role={alt ? "img" : undefined}
        strokeWidth={1.9}
      />
    );
  }

  const icon = value?.trim() || fallback;
  if (/^(https?:\/\/|\/|data:image\/|blob:)/i.test(icon)) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={icon} alt={alt} loading="lazy" decoding="async" className={className || "h-full w-full object-cover"} />;
  }

  return <>{icon}</>;
}
