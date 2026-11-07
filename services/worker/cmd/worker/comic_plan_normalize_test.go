package main

import "testing"

// 同一镜同时有对白和旁白时，按叙事模式保留真正会被朗读的一条，
// 避免整条付费任务在口型校验阶段直接失败。
func TestNormalizeComicDramaPlanPrefersReadableSpeech(t *testing.T) {
	cases := []struct {
		perspective   string
		wantDialogue  string
		wantNarration string
		wantMarker    string
	}{
		{"third_person", "", "她想起从前", "narration_preferred"},
		{"smart", "别怕", "", "dialogue_preferred"},
	}
	for _, tc := range cases {
		plan := map[string]interface{}{"storyboards": []interface{}{
			map[string]interface{}{"id": "S01", "scene": "雨夜街道", "dialogue": "别怕", "narration": "她想起从前"},
			map[string]interface{}{"id": "S02", "scene": "天台", "dialogue": "", "narration": "她终于找到了它"},
		}}
		inputs := map[string]interface{}{"narration_perspective": tc.perspective, "storyboard_grid": 2}
		out := normalizeComicDramaPlan(plan, inputs, nil)
		shots := comicStoryboards(out, nil)
		if len(shots) != 2 {
			t.Fatalf("%s: storyboards=%d", tc.perspective, len(shots))
		}
		if got := stringAny(shots[0]["dialogue"]); got != tc.wantDialogue {
			t.Fatalf("%s: dialogue=%q want %q", tc.perspective, got, tc.wantDialogue)
		}
		if got := stringAny(shots[0]["narration"]); got != tc.wantNarration {
			t.Fatalf("%s: narration=%q want %q", tc.perspective, got, tc.wantNarration)
		}
		if got := stringAny(shots[0]["speech_normalized"]); got != tc.wantMarker {
			t.Fatalf("%s: marker=%q want %q", tc.perspective, got, tc.wantMarker)
		}
		// 单一类型的镜头不能被改动。
		if got := stringAny(shots[1]["narration"]); got != "她终于找到了它" {
			t.Fatalf("%s: clean narration changed: %q", tc.perspective, got)
		}
	}
}
