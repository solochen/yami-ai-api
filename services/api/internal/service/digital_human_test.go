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
