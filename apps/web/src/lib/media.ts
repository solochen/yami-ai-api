// 灵感广场列表原先直接加载原图（单张常见 2–8 MB）。配置图片 CDN 后，
// 列表卡片改用 CDN 侧实时缩放/转码的缩略图，详情弹窗仍加载原图。
//
// NEXT_PUBLIC_MEDIA_CDN_HOSTS 为空时所有函数原样返回，行为与改造前一致；
// 只有命中配置域名的地址才会追加腾讯云数据万象（CI）图片处理参数。
const CDN_HOSTS = (process.env.NEXT_PUBLIC_MEDIA_CDN_HOSTS ?? "")
  .split(",")
  .map((host) => host.trim().toLowerCase())
  .filter(Boolean);

// 缩略图宽度收敛到固定档位，避免同一张图因组件宽度不同产生大量 CDN 缓存副本。
const WIDTH_BUCKETS = [320, 480, 640, 750, 960, 1280, 1600];
const THUMBNAIL_TEMPLATE = "?imageMogr2/thumbnail/{width}x/format/webp/quality/82/ignore-error/1";

export function isMediaCDNURL(url: string): boolean {
  const value = String(url || "").trim();
  if (!value || CDN_HOSTS.length === 0 || !/^https?:\/\//i.test(value)) return false;
  try {
    return CDN_HOSTS.includes(new URL(value).hostname.toLowerCase());
  } catch {
    return false;
  }
}

export function bucketThumbnailWidth(width: number): number {
  const target = Math.max(1, Math.round(width || 0));
  return WIDTH_BUCKETS.find((bucket) => bucket >= target) ?? WIDTH_BUCKETS[WIDTH_BUCKETS.length - 1];
}

export function mediaThumbnailURL(url: string, width: number): string {
  const value = String(url || "").trim();
  if (!isMediaCDNURL(value) || width <= 0) return value;
  // 已经带查询参数（签名、已有处理规则）时不再拼接，避免破坏原地址。
  if (value.includes("?")) return value;
  const size = bucketThumbnailWidth(width);
  return `${value}${THUMBNAIL_TEMPLATE.replace("{width}", String(size))}`;
}
