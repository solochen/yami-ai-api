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
	if got := digitalHumanCost("audio", 61, 0, 100, 3500, 1); math.Abs(got-101.666667) > 0.001 {
		t.Fatalf("voice cost = %v", got)
	}
	if got := digitalHumanCost("video", 90, 0, 100, 3500, 1); got != 5250 {
		t.Fatalf("video cost = %v", got)
	}
	if got := digitalHumanCost("text", 0, 2.5, 100, 3500, 1); got != 2.5 {
		t.Fatalf("text cost = %v", got)
	}
	if got := digitalHumanCost("audio", 0, 9, 100, 3500, 1); got != 0 {
		t.Fatalf("empty voice cost = %v", got)
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
