package service

import (
	"context"
	"os"
	"testing"
	"time"
)

// 本地联调用：真实拉取七牛市场与 aiboli（预期 401，仅验证链路不崩）。
func TestLiveQiniuFetch(t *testing.T) {
	if os.Getenv("LIVE_PRICE_TEST") == "" {
		t.Skip("需要网络，设置 LIVE_PRICE_TEST=1 运行")
	}
	s := &AdminService{}
	ctx := context.Background()
	prov := UpstreamProvider{Provider: "qiniu", PriceEndpoint: "https://api.qnaigc.com/v1", Currency: "CNY", RechargeRate: 1}
	items, code, err := s.fetchQiniuPrices(ctx, prov, 30*time.Second)
	if err != nil {
		t.Fatalf("qiniu fetch err: %v (code %d)", err, code)
	}
	if len(items) < 50 {
		t.Fatalf("qiniu items too few: %d", len(items))
	}
	token, perSec, perImg := 0, 0, 0
	for _, it := range items {
		switch it.BillingType {
		case "per_token":
			token++
		case "per_second":
			perSec++
		case "per_image":
			perImg++
		}
		if it.CNYInputPerM < 0 {
			t.Fatalf("bad price for %s: %+v", it.UpstreamModel, it)
		}
	}
	t.Logf("qiniu items=%d token=%d per_second=%d per_image=%d", len(items), token, perSec, perImg)
}

func TestLiveAiboliAuthPath(t *testing.T) {
	if os.Getenv("LIVE_PRICE_TEST") == "" {
		t.Skip("需要网络，设置 LIVE_PRICE_TEST=1 运行")
	}
	s := &AdminService{}
	ctx := context.Background()
	prov := UpstreamProvider{Provider: "aiboli", PriceEndpoint: "https://api.aiboli.xyz", Currency: "CNY", RechargeRate: 1, Config: map[string]interface{}{"api_key": ""}}
	_, code, err := s.fetchNewAPIPrices(ctx, prov, 20*time.Second)
	if err == nil {
		t.Fatalf("expected auth failure without key, got code %d", code)
	}
	t.Logf("aiboli without key -> %d %v（预期 401，链路正常）", code, err)
}
