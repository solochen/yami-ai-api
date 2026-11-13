import assert from "node:assert/strict";
import test from "node:test";

process.env.NEXT_PUBLIC_MEDIA_CDN_HOSTS = "img.example.com, IMG2.example.com";
const { bucketThumbnailWidth, isMediaCDNURL, mediaThumbnailURL } = await import("./media.ts");

test("only configured CDN hosts are treated as CDN images", () => {
  assert.equal(isMediaCDNURL("https://img.example.com/gallery/a.png"), true);
  assert.equal(isMediaCDNURL("https://img2.example.com/gallery/a.png"), true);
  assert.equal(isMediaCDNURL("https://raw.githubusercontent.com/a.png"), false);
  assert.equal(isMediaCDNURL("/media/starai-works/a.png"), false);
  assert.equal(isMediaCDNURL(""), false);
});

test("CDN images get a deterministic thumbnail rule", () => {
  assert.equal(
    mediaThumbnailURL("https://img.example.com/gallery/a.png", 700),
    "https://img.example.com/gallery/a.png?imageMogr2/thumbnail/750x/format/webp/quality/82/ignore-error/1",
  );
  // 同一张图在不同宽度下收敛到同一档位，避免 CDN 缓存碎片化。
  assert.equal(bucketThumbnailWidth(600), 640);
  assert.equal(bucketThumbnailWidth(640), 640);
  assert.equal(bucketThumbnailWidth(5000), 1600);
});

test("non-CDN, relative and already-signed URLs stay untouched", () => {
  assert.equal(mediaThumbnailURL("https://raw.githubusercontent.com/a.png", 640), "https://raw.githubusercontent.com/a.png");
  assert.equal(mediaThumbnailURL("/media/starai-works/a.png", 640), "/media/starai-works/a.png");
  assert.equal(
    mediaThumbnailURL("https://img.example.com/a.png?sign=abc", 640),
    "https://img.example.com/a.png?sign=abc",
  );
  assert.equal(mediaThumbnailURL("https://img.example.com/a.png", 0), "https://img.example.com/a.png");
});
