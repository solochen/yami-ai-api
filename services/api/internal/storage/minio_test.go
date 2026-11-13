package storage

import (
	"testing"

	"github.com/minio/minio-go/v7"
)

func testClient(style, publicURL, bucket string) *Client {
	return &Client{bucket: bucket, publicURL: publicURL, urlStyle: style}
}

func TestPublicURLKeepsBucketForDirectStorage(t *testing.T) {
	c := testClient(URLStyleBucket, "https://cdn.example.com/", "starai-works")
	if got, want := c.PublicURL("uploads/1/a b.png"), "https://cdn.example.com/starai-works/uploads/1/a%20b.png"; got != want {
		t.Fatalf("PublicURL() = %q, want %q", got, want)
	}
}

func TestPublicURLDropsBucketForCDNDomain(t *testing.T) {
	c := testClient(URLStyleObject, "https://img.example.com", "yami-media-1250000000")
	if got, want := c.PublicURL("/works/2026/中文 图.png"), "https://img.example.com/works/2026/%E4%B8%AD%E6%96%87%20%E5%9B%BE.png"; got != want {
		t.Fatalf("PublicURL() = %q, want %q", got, want)
	}
}

func TestObjectKeyFromURLCustomCDNDomain(t *testing.T) {
	c := testClient(URLStyleObject, "https://img.example.com", "yami-media-1250000000")
	cases := map[string]string{
		"https://img.example.com/gallery/reference/case-1.png":                        "gallery/reference/case-1.png",
		"https://img.example.com/gallery/a.jpg?imageMogr2/thumbnail/480x/format/webp": "gallery/a.jpg",
		"https://img.example.com/%E4%B8%AD%E6%96%87/x.png":                            "中文/x.png",
		"https://other.example.com/gallery/a.jpg":                                     "",
		"https://other.example.com/yami-media-1250000000/gallery/a.jpg":               "",
	}
	for raw, want := range cases {
		if got := c.ObjectKeyFromURL(raw); got != want {
			t.Fatalf("ObjectKeyFromURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestObjectKeyFromURLLegacyProxyPath(t *testing.T) {
	c := testClient(URLStyleBucket, "/media", "starai-works")
	cases := map[string]string{
		"/media/starai-works/works/1/a.png":                 "works/1/a.png",
		"/media/starai-works/works/1/a.png?x-oss-process=1": "works/1/a.png",
		"starai-works/works/1/a.png":                        "works/1/a.png",
		"https://www.orbit-e.com/minio-store/works/1/a.png": "",
	}
	for raw, want := range cases {
		if got := c.ObjectKeyFromURL(raw); got != want {
			t.Fatalf("ObjectKeyFromURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestObjectKeyFromURLLegacyAbsolutePathPrefix(t *testing.T) {
	// 旧部署的公开地址是 https://www.orbit-e.com/minio-store（带路径前缀），
	// 迁移前的历史数据仍要靠它反查对象 key。
	c := testClient(URLStyleBucket, "https://www.orbit-e.com/minio-store", "starai-works")
	if got, want := c.ObjectKeyFromURL("https://www.orbit-e.com/minio-store/starai-works/works/1/a.png"), "works/1/a.png"; got != want {
		t.Fatalf("ObjectKeyFromURL() = %q, want %q", got, want)
	}
}

func TestObjectKeyFromURLRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		style     string
		publicURL string
	}{
		{URLStyleBucket, "https://cdn.example.com/starai-works"},
		{URLStyleObject, "https://img.example.com"},
	} {
		c := testClient(tc.style, tc.publicURL, "starai-works")
		key := "uploads/42/图片 1.webp"
		url := c.PublicURL(key)
		if got := c.ObjectKeyFromURL(url); got != key {
			t.Fatalf("[%s] round trip key = %q, want %q (url %q)", tc.style, got, key, url)
		}
	}
}

func TestBucketLookupType(t *testing.T) {
	cases := []struct {
		value    string
		endpoint string
		want     minio.BucketLookupType
	}{
		// 腾讯云 COS 强制虚拟主机风格
		{"", "cos.ap-hongkong.myqcloud.com", minio.BucketLookupDNS},
		{"", "COS.AP-HONGKONG.MYQCLOUD.COM", minio.BucketLookupDNS},
		{"", "cos.ap-hongkong.myqcloud.com:443", minio.BucketLookupDNS},
		{"", "https://cos.ap-hongkong.myqcloud.com/", minio.BucketLookupDNS},
		// 显式配置优先于自动识别
		{"path", "cos.ap-hongkong.myqcloud.com", minio.BucketLookupPath},
		{"dns", "localhost:9000", minio.BucketLookupDNS},
		// 其他 S3 兼容存储保持 auto
		{"", "localhost:9000", minio.BucketLookupAuto},
		{"", "s3.amazonaws.com", minio.BucketLookupAuto},
	}
	for _, tc := range cases {
		if got := bucketLookupType(tc.value, tc.endpoint); got != tc.want {
			t.Fatalf("bucketLookupType(%q, %q) = %v, want %v", tc.value, tc.endpoint, got, tc.want)
		}
	}
}
