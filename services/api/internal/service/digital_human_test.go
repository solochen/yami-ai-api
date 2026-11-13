package service

import (
	"math"
	"strings"
	"testing"
)

func TestBuildDigitalHumanPersona(t *testing.T) {
	persona := buildDigitalHumanPersona(DigitalHumanRole{
		Name: "小林", Relation: "朋友", UserTitle: "阿龙", Memory: "喜欢夜跑", Style: "短句", Persona: "温和",
	}, []DigitalHumanMessage{{Speaker: "user", Content: "在吗"}})
	for _, part := range []string{"小林", "朋友", "阿龙", "喜欢夜跑", "短句", "温和", "用户：在吗", "括号里的内容不要念出来"} {
		if !strings.Contains(persona, part) {
			t.Fatalf("persona missing %s: %s", part, persona)
		}
	}
	withAction := buildDigitalHumanPersona(DigitalHumanRole{Name: "小雪"}, []DigitalHumanMessage{
		{Speaker: "assistant", Content: "好哒，老公，我这就蹲下啦～（缓缓屈膝，目光温柔地注视着你）"},
	})
	if strings.Contains(withAction, "缓缓屈膝") {
		t.Fatalf("stage direction leaked into persona: %s", withAction)
	}
	if !strings.Contains(withAction, "好哒，老公，我这就蹲下啦～") {
		t.Fatalf("spoken line missing: %s", withAction)
	}
	if got := stripSpokenStageDirections("好的(smiles)再见"); got != "好的再见" {
		t.Fatalf("halfwidth stage direction: %q", got)
	}
}

func TestDigitalHumanCost(t *testing.T) {
	// 上游成本：语音 61 秒 × 2.8125 元/分钟
	if got := digitalHumanProviderCost("audio", 61, 0, 2.8125, 2.8125, 0.03125); math.Abs(got-2.859375) > 0.0001 {
		t.Fatalf("voice provider cost = %v", got)
	}
	// 视频 90 秒 × 2.8125 元/分钟
	if got := digitalHumanProviderCost("video", 90, 0, 2.8125, 2.8125, 0.03125); math.Abs(got-4.21875) > 0.0001 {
		t.Fatalf("video provider cost = %v", got)
	}
	// 文字会话：Vidu 积分 × 0.03125 元/积分
	if got := digitalHumanProviderCost("text", 0, 414, 2.8125, 2.8125, 0.03125); math.Abs(got-12.9375) > 0.0001 {
		t.Fatalf("text provider cost = %v", got)
	}
	if got := digitalHumanProviderCost("audio", 0, 9, 2.8125, 2.8125, 0.03125); got != 0 {
		t.Fatalf("empty voice cost = %v", got)
	}
	// 售价 = 上游成本 ÷ (1 − 平台佣金率)，向上取整到 6 位小数
	if got := digitalHumanUserCharge(2.8125, 3); math.Abs(got-2.899485) > 0.0001 {
		t.Fatalf("user charge = %v", got)
	}
	if got := digitalHumanUserCharge(2.8125, 0); math.Abs(got-2.8125) > 0.0001 {
		t.Fatalf("zero margin charge = %v", got)
	}
	if got := digitalHumanUserCharge(0, 3); got != 0 {
		t.Fatalf("zero cost charge = %v", got)
	}
}

func TestDigitalHumanSalePrice(t *testing.T) {
	// 配了售价就用售价（文字 / 语音 / 视频同价）。
	configured := digitalHumanSettings{SalePerMinute: 2.49}
	if got := digitalHumanSalePerMinute(configured); math.Abs(got-2.49) > 0.0001 {
		t.Fatalf("configured sale price = %v", got)
	}
	// 没配、或配成 0，一律用默认售价，绝不回落到上游成本价。
	for _, sale := range []float64{0, -1} {
		settings := digitalHumanSettings{SalePerMinute: sale, VoicePerMinute: 2.8125, VideoPerMinute: 2.8125}
		if got := digitalHumanSalePerMinute(settings); math.Abs(got-digitalHumanSaleDefault) > 0.0001 {
			t.Fatalf("sale=%v should fall back to default, got %v", sale, got)
		}
	}
	if digitalHumanSaleDefault != 2.49 {
		t.Fatalf("default sale price = %v", digitalHumanSaleDefault)
	}
}

func TestDigitalHumanSessionCostUsesSalePrice(t *testing.T) {
	settings := digitalHumanSettings{SalePerMinute: 2.49, VoicePerMinute: 1.96875, VideoPerMinute: 1.96875, TextCreditRate: 0.021875}
	// 90 秒视频按 2.49 元/分钟 = 3.735
	if got := math.Round(90.0/60*digitalHumanSalePerMinute(settings)*1e6) / 1e6; math.Abs(got-3.735) > 0.0001 {
		t.Fatalf("90s charge = %v", got)
	}
	// 售价留空也按默认 2.49 计费，不会被算成成本价 2.8125。
	blank := digitalHumanSettings{VideoPerMinute: 2.8125, TextCreditRate: 0.03125}
	if got := digitalHumanSalePerMinute(blank); got != 2.49 {
		t.Fatalf("blank sale price = %v", got)
	}
}

func TestApplyDigitalHumanAutoCost(t *testing.T) {
	// 标准价：1.5 积分/秒 × 60 × 0.03125 = 2.8125 元/分钟
	std := applyDigitalHumanAutoCost(digitalHumanSettings{
		CostAutoFromCredits:     true,
		RealtimeCreditPerSecond: 1.5,
		TextCreditRate:          0.03125,
		VoicePerMinute:          99,
		VideoPerMinute:          99,
	})
	if math.Abs(std.VideoPerMinute-2.8125) > 0.0001 || std.VoicePerMinute != std.VideoPerMinute {
		t.Fatalf("standard auto cost = %v / %v", std.VoicePerMinute, std.VideoPerMinute)
	}
	// 7 折套餐：1.5 × 60 × 0.021875 = 1.96875
	promo := applyDigitalHumanAutoCost(digitalHumanSettings{
		CostAutoFromCredits:     true,
		RealtimeCreditPerSecond: 1.5,
		TextCreditRate:          0.021875,
	})
	if math.Abs(promo.VideoPerMinute-1.96875) > 0.0001 {
		t.Fatalf("promo auto cost = %v", promo.VideoPerMinute)
	}
	// 如果账单证明实时版其实是 1 积分/秒，改这一个数就够：1 × 60 × 0.021875 = 1.3125
	one := applyDigitalHumanAutoCost(digitalHumanSettings{
		CostAutoFromCredits:     true,
		RealtimeCreditPerSecond: 1,
		TextCreditRate:          0.021875,
	})
	if math.Abs(one.VideoPerMinute-1.3125) > 0.0001 {
		t.Fatalf("1 credit/s auto cost = %v", one.VideoPerMinute)
	}
	// 关掉开关就保留手工填写的成本。
	manual := applyDigitalHumanAutoCost(digitalHumanSettings{
		CostAutoFromCredits:     false,
		RealtimeCreditPerSecond: 1.5,
		TextCreditRate:          0.021875,
		VideoPerMinute:          3.5,
	})
	if manual.VideoPerMinute != 3.5 {
		t.Fatalf("manual cost should be kept, got %v", manual.VideoPerMinute)
	}
}

func TestDigitalHumanUpstreamError(t *testing.T) {
	// 鉴权失败：空响应体也要给出可操作提示，且不能带厂商名。
	for _, status := range []int{401, 403} {
		msg := digitalHumanUpstreamError(status, nil)
		if !strings.Contains(msg, "密钥无效") {
			t.Fatalf("status %d should mention invalid key, got %q", status, msg)
		}
		if strings.Contains(strings.ToLower(msg), "vidu") {
			t.Fatalf("user facing message must not leak vendor, got %q", msg)
		}
	}
	// 业务错误：透传脱敏后的原因。
	msg := digitalHumanUpstreamError(400, []byte(`{"message":"vidu 账号余额不足 https://api.vidu.cn/x"}`))
	if strings.Contains(strings.ToLower(msg), "vidu") || strings.Contains(msg, "api.vidu.cn") {
		t.Fatalf("upstream detail not sanitized: %q", msg)
	}
	if !strings.Contains(msg, "余额不足") {
		t.Fatalf("reason dropped: %q", msg)
	}
	// 空响应体 + 非鉴权状态码：保持原来的兜底文案。
	if got := digitalHumanUpstreamError(500, nil); got != "数字人服务拒绝了这次通话" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestDigitalHumanLogBody(t *testing.T) {
	if got := digitalHumanLogBody(nil); got != "<empty>" {
		t.Fatalf("empty body = %q", got)
	}
	long := strings.Repeat("拒", 600)
	got := digitalHumanLogBody([]byte(long))
	if !strings.HasSuffix(got, "...") || len([]rune(got)) != 515 {
		t.Fatalf("unexpected truncation: %d runes", len([]rune(got)))
	}
}

func TestRankDigitalHumanKnowledge(t *testing.T) {
	docs := []knowledgeDoc{{ID: 1, Title: "家庭", Content: "父亲住在杭州，周末一起喝茶。"}, {ID: 2, Title: "工作", Content: "公司在上海做设计。"}}
	hits := rankDigitalHumanKnowledge(docs, "父亲住在哪里", 5)
	if len(hits) == 0 || hits[0].ID != "doc_1" {
		t.Fatalf("hits = %#v", hits)
	}
	if len(rankDigitalHumanKnowledge(docs, "", 5)) != 0 {
		t.Fatal("empty query should not match")
	}
}
