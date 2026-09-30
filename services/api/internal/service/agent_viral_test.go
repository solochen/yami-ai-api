package service

import "testing"

func TestNormalizeViralShareText(t *testing.T) {
	inputs := map[string]interface{}{"share_text": "2.56 L@W.MJ pqR:/ :3pm 08/31   https://v.douyin.com/yE121DSSYh8/ 复制此链接，打开Dou音搜索，直接观看视频！"}
	if err := normalizeViralVideoBreakdownInputs(inputs); err != nil {
		t.Fatal(err)
	}
	if inputs["video_url"] != "https://v.douyin.com/yE121DSSYh8/" || inputs["source_url"] != "https://v.douyin.com/yE121DSSYh8/" {
		t.Fatalf("inputs = %#v", inputs)
	}
}

func TestNormalizeViralRequiresVideo(t *testing.T) {
	if err := normalizeViralVideoBreakdownInputs(map[string]interface{}{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestDurationSecondWorkflowBillingIsAllowed(t *testing.T) {
	if err := validateWorkflowPriceRule(map[string]interface{}{"billing_type": "duration_second", "unit_price": float64(0)}); err != nil {
		t.Fatal(err)
	}
}
