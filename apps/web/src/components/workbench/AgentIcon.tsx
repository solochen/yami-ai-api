import {
  BookOpenText,
  Camera,
  CaptionsOff,
  Clapperboard,
  Film,
  Flame,
  ImagePlus,
  MessageCircleMore,
  MonitorUp,
  MousePointerClick,
  Newspaper,
  PackagePlus,
  PackageSearch,
  PanelsTopLeft,
  Repeat2,
  ScanSearch,
  ShoppingBag,
  SlidersHorizontal,
  WandSparkles,
  Workflow,
  type LucideIcon,
} from "lucide-react";

const AGENT_ICON_BY_CODE: Record<string, LucideIcon> = {
  general_creative_agent: WandSparkles,
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
  const SemanticIcon = code ? AGENT_ICON_BY_CODE[code] : undefined;
  if (SemanticIcon) {
    return (
      <SemanticIcon
        aria-hidden={alt ? undefined : true}
        aria-label={alt || undefined}
        className={className || "h-[52%] w-[52%] min-h-4 min-w-4"}
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
