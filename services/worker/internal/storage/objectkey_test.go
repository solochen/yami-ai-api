package storage

import (
	"net/url"
	"testing"
)

// 迁移 169 之后上传素材存的是 /media/<bucket>/<key> 相对地址；web/admin 靠代理能显示，
// 但 worker 侧必须能反查出对象 key 才能读到原图，否则参考图会整批静默丢失。
func TestLocalObjectKeyFromRelativeMediaURL(t *testing.T) {
	c, err := NewLocal(t.TempDir(), "/media")
	if err != nil {
		t.Fatalf("构造 LocalClient 失败: %v", err)
	}
	cases := []struct{ in, want string }{
		{"/media/assets/7/ast_1/IMG_3641 拷贝.jpg", "assets/7/ast_1/IMG_3641 拷贝.jpg"},
		{"/media/%E4%B8%AD%E6%96%87.png", "中文.png"},
		{"assets/7/ast_1/x.jpg", "assets/7/ast_1/x.jpg"},
		{"", ""},
		{"/etc/passwd", "etc/passwd"},
	}
	for _, tc := range cases {
		if got := c.ObjectKeyFromURL(tc.in); got != tc.want {
			t.Errorf("ObjectKeyFromURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// 无 host 的 publicURL 不接受外部绝对地址，避免把 CDN 地址当成本地 key。
	if got := c.ObjectKeyFromURL("https://cdn.example.com/a.png"); got != "" {
		t.Errorf("外部地址不应映射为对象 key, got %q", got)
	}
}

// MinIO 客户端与 LocalClient 的解析规则保持一致：覆盖相对地址、同源绝对地址、异源地址。
func TestMinioObjectKeyFromURLRules(t *testing.T) {
	hostless := &Client{publicURL: "/media", bucket: "starai-works"}
	cases := []struct{ in, want string }{
		{"/media/starai-works/assets/7/x.jpg", "assets/7/x.jpg"},
		{"/starai-works/assets/7/x.jpg", "assets/7/x.jpg"},
		{"assets/7/x.jpg", "assets/7/x.jpg"},
		{"", ""},
		{"https://cdn.other.com/starai-works/assets/x.jpg", ""},
	}
	for _, tc := range cases {
		if got := hostless.ObjectKeyFromURL(tc.in); got != tc.want {
			t.Errorf("hostless ObjectKeyFromURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	sameOrigin := "https://media.example.com"
	withHost := &Client{publicURL: sameOrigin, bucket: "starai-works"}
	if got := withHost.ObjectKeyFromURL("https://media.example.com/starai-works/assets/7/x.jpg"); got != "assets/7/x.jpg" {
		t.Errorf("同源绝对地址解析错误: %q", got)
	}
	if got := withHost.ObjectKeyFromURL("https://evil.example.com/starai-works/assets/7/x.jpg"); got != "" {
		t.Errorf("异源地址不应解析, got %q", got)
	}
	// 同源绝对地址经 url.Parse 后 Host 匹配，Path 里的空格需要正常解码。
	raw := "https://media.example.com/starai-works/assets/7/" + url.PathEscape("IMG_3641 拷贝.jpg")
	if got := withHost.ObjectKeyFromURL(raw); got != "assets/7/IMG_3641 拷贝.jpg" {
		t.Errorf("同源转义地址解析错误: %q", got)
	}
}
