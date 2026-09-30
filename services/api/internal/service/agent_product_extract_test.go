package service

import "testing"

func TestParseProductURL(t *testing.T) {
	cases := []struct {
		raw      string
		platform string
		id       string
	}{
		{"https://item.jd.com/10041037225218.html", "jd", "10041037225218"},
		{"https://item.m.jd.com/product/10041037225218.html", "jd", "10041037225218"},
	}
	for _, tc := range cases {
		link, err := parseProductURL(tc.raw)
		if err != nil || link.Platform != tc.platform || link.ID != tc.id {
			t.Fatalf("%s => %+v %v", tc.raw, link, err)
		}
	}
	for _, raw := range []string{
		"https://example.com/item.htm?id=739370932035",
		"https://item.jd.com/abc.html",
		"https://item.taobao.com/item.htm?id=739370932035",
		"https://detail.tmall.com/item.htm?id=744983869996",
		"https://haohuo.jinritemai.com/ecommerce/trade/detail/index.html?id=3821314999977115743&origin_type=2631",
	} {
		if _, err := parseProductURL(raw); err == nil {
			t.Fatalf("expected rejection for %s", raw)
		}
	}
}
