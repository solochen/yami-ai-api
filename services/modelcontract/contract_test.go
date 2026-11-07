package modelcontract

import (
	"strings"
	"testing"
)

func chat(caps map[string]interface{}) Model {
	return Model{Code: "chat", Category: "chat", Mode: "chat_completions", Enabled: true, Rule: map[string]interface{}{"capabilities": caps}}
}
func slot(t *testing.T, code, key string) Slot {
	t.Helper()
	s, ok := FieldSlot(Definition{Code: code}, key)
	if !ok {
		t.Fatal(key)
	}
	return s
}
func TestSlotCapabilityBoundaries(t *testing.T) {
	visionVideo := chat(map[string]interface{}{"vision": true, "video_analysis": true, "audio_analysis": true})
	image := Model{Code: "image", Category: "image", Mode: "images", Enabled: true, Rule: map[string]interface{}{"image": map[string]interface{}{"max_reference_images": 3}}}
	video := Model{Code: "video", Category: "video", Mode: "video", Enabled: true, Schema: map[string]interface{}{"properties": map[string]interface{}{"generation_mode": map[string]interface{}{"enum": []string{"text", "image"}}}}}
	tts := Model{Code: "tts", Category: "audio", Mode: "audio", Enabled: true, Endpoint: "/v1/audio/speech"}
	music := tts
	music.Code = "music"
	music.Endpoint = "/v1/music_generation"
	asr := tts
	asr.Code = "asr"
	asr.Endpoint = "/v1/audio/transcriptions"
	plainImage := image
	plainImage.Rule = nil
	plainVideo := video
	plainVideo.Schema = nil
	cases := []struct {
		name, workflow, key string
		model               Model
		want                bool
	}{
		{"viral plain text", "viral_video_breakdown", "analysis_model_code", chat(nil), false},
		{"viral image only", "viral_video_breakdown", "analysis_model_code", chat(map[string]interface{}{"vision": true}), false},
		{"viral video only", "viral_video_breakdown", "analysis_model_code", chat(map[string]interface{}{"video_analysis": true}), false},
		{"viral superset", "viral_video_breakdown", "analysis_model_code", visionVideo, true},
		{"viral fallback", "viral_video_breakdown", "fallback_model_code", chat(nil), false},
		{"keyframe text image", "ai_comic_drama", "image_model_code", plainImage, false},
		{"keyframe reference", "ai_comic_drama", "image_model_code", image, true},
		{"video text only", "ai_comic_drama", "video_model_code", plainVideo, false},
		{"video image input", "ai_comic_drama", "video_model_code", video, true},
		{"voice tts", "ai_comic_drama", "narration_model_code", tts, true},
		{"voice music", "ai_comic_drama", "narration_model_code", music, false},
		{"voice asr", "ai_comic_drama", "narration_model_code", asr, false},
		{"voice chat", "ai_comic_drama", "narration_model_code", visionVideo, false},
		{"music tts", "general_creative_agent", "music_model_code", tts, false},
		{"music endpoint", "general_creative_agent", "music_model_code", music, true},
		{"visual analysis", "ai_photo_studio", "analysis_model_code", chat(nil), false},
		{"vision backup", "product_refine", "fallback_model_code", chat(nil), false},
		{"multi ref tryon", "ai_virtual_tryon", "generation_model_code", image, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(slot(t, tc.workflow, tc.key), tc.model)
			if (err == nil) != tc.want {
				t.Fatalf("pass=%v want=%v error=%v", err == nil, tc.want, err)
			}
		})
	}
}

func TestExplicitFalseAndRoutesCannotGrantMissingCapability(t *testing.T) {
	s := slot(t, "viral_video_breakdown", "analysis_model_code")
	m := chat(map[string]interface{}{"vision": false, "image_input": true, "video_analysis": true})
	if Validate(s, m) == nil {
		t.Fatal("legacy alias bypassed explicit false")
	}
	m.Rule = map[string]interface{}{"capabilities": map[string]interface{}{"vision": true, "video_analysis": true}}
	m.Routes = []Route{{Enabled: true, Rule: map[string]interface{}{"capabilities": map[string]interface{}{"image_input": false}}}}
	if Validate(s, m) == nil {
		t.Fatal("route override bypassed")
	}
	m.Routes = []Route{{Enabled: true}, {Enabled: true, Rule: map[string]interface{}{"capabilities": map[string]interface{}{"video_analysis": false}}}}
	if Validate(s, m) == nil {
		t.Fatal("incompatible enabled failover route accepted")
	}
	m.Routes[1].Enabled = false
	if err := Validate(s, m); err != nil {
		t.Fatal(err)
	}
	m.Routes[0].Enabled = false
	if Validate(s, m) == nil {
		t.Fatal("all routes disabled")
	}
}

func TestEmbeddingSlotAcceptsVectorModels(t *testing.T) {
	slot := slot(t, "renovation_budget_agent", "embedding_model_code")
	model := Model{Code: "qwen3-7-text-embedding", Name: "Qwen3.7 Text Embedding", Category: "embedding", Mode: "embeddings", Enabled: true, Routes: []Route{{Enabled: true}}}
	if err := Validate(slot, model); err != nil {
		t.Fatal(err)
	}
	chatModel := chat(nil)
	if Validate(slot, chatModel) == nil {
		t.Fatal("chat model accepted as embedding")
	}
}

func TestSchemaModeAndReferenceRestrictions(t *testing.T) {
	m := Model{Code: "video", Category: "video", Mode: "video", Enabled: true, Rule: map[string]interface{}{"video": map[string]interface{}{"upload_profile": "first_last", "max_reference_images": 3}}, Schema: map[string]interface{}{"properties": map[string]interface{}{"generation_mode": map[string]interface{}{"enum": []string{"text"}}, "reference_images": map[string]interface{}{}}}}
	if Validate(slot(t, "ai_comic_drama", "video_model_code"), m) == nil {
		t.Fatal("text-only mode accepted via stale reference metadata")
	}
	m = Model{Code: "image", Category: "image", Mode: "images", Enabled: true, Rule: map[string]interface{}{"image": map[string]interface{}{"max_reference_images": 0}}, Schema: map[string]interface{}{"properties": map[string]interface{}{"reference_images": map[string]interface{}{}}}}
	if Validate(slot(t, "ai_comic_drama", "image_model_code"), m) == nil {
		t.Fatal("explicit zero reference limit ignored")
	}
}

func TestBuiltInRequirementsCannotBeLowered(t *testing.T) {
	d := Definition{Code: "viral_video_breakdown", Runtime: map[string]interface{}{"agent_mode": "prompt_chat", "analysis_model_code": "plain"}}
	m := chat(nil)
	m.Code = "plain"
	if ValidateDefinition(d, []Model{m}) == nil {
		t.Fatal("agent mode weakened viral requirements")
	}
	d = Definition{Code: "ai_comic_drama", Nodes: []Node{{ID: "keyframes", Name: "Fake", Type: "llm", ModelCode: "plain"}}}
	if ValidateDefinition(d, []Model{m}) == nil {
		t.Fatal("node type weakened keyframe requirements")
	}
	d = Definition{Runtime: map[string]interface{}{"dialogue_model_codes": []interface{}{true}}}
	if err := ValidateDefinition(d, nil); err == nil || !strings.Contains(err.Error(), "数组") {
		t.Fatal(err)
	}
}

func TestAliasAndEffectiveOverride(t *testing.T) {
	m := chat(nil)
	m.Alias = "public"
	if got, ok := Find([]Model{m}, "public"); !ok || got.Code != m.Code {
		t.Fatal("alias not resolved")
	}
	d := Definition{Code: "ai_comic_drama", Runtime: map[string]interface{}{"video_model_code": "old", "generation_model_code": "old"}, Nodes: []Node{{ID: "video_segments", ModelCode: "old"}}}
	effective := Effective(d, map[string]interface{}{"video_model_code": "new"})
	if effective.Runtime["generation_model_code"] != "new" || len(effective.Nodes) != 0 || d.Runtime["video_model_code"] != "old" {
		t.Fatal("override did not resolve mirrors without mutating definition")
	}
}

func TestDescriptiveNodesUseRuntimeRequirements(t *testing.T) {
	for _, tc := range []struct{ code, id, field, capability string }{
		{"video_upscale", "upscale", "generation_model_code", "video_upscale"},
		{"video_redraw", "redraw", "generation_model_code", "video_redraw"},
		{"subtitle_remove", "subtitle_remove", "generation_model_code", "subtitle_remove"},
		{"subtitle_remove", "remove_subtitle", "generation_model_code", "subtitle_remove"},
	} {
		t.Run(tc.code+"/"+tc.id, func(t *testing.T) {
			m := Model{Code: "specialized", Category: "video", Mode: "video", Enabled: true, Rule: map[string]interface{}{"capabilities": map[string]interface{}{tc.capability: true}}}
			d := Definition{Code: tc.code, Runtime: map[string]interface{}{tc.field: m.Code}, Nodes: []Node{{ID: tc.id, Type: "video", ModelCode: m.Code}}}
			if err := ValidateDefinition(d, []Model{m}); err != nil {
				t.Fatalf("specialized operation wrongly required image-to-video: %v", err)
			}
			n := d.Nodes[0]
			n.Type = "image"
			if err := Validate(NodeSlot(d, n), Model{Code: "image", Category: "image", Mode: "images", Enabled: true}); err == nil {
				t.Fatal("changing descriptive node type weakened the operation")
			}
		})
	}
	for _, tc := range []struct{ code, id, field string }{
		{"ai_photo_studio", "styling", "analysis_model_code"},
		{"ai_photo_studio", "generate", "generation_model_code"},
		{"ai_virtual_tryon", "try_on", "generation_model_code"},
		{"ai_novel_workshop", "planning", "analysis_model_code"},
		{"ai_novel_workshop", "writing", "generation_model_code"},
		{"ai_novel_workshop", "polishing", "generation_model_code"},
		{"ai_novel_workshop", "archiving", "generation_model_code"},
		{"general_creative_agent", "generate", "image_model_code"},
	} {
		if got := NodeRuntimeField(Definition{Code: tc.code}, Node{ID: tc.id}); got != tc.field {
			t.Fatalf("%s/%s=%s, want %s", tc.code, tc.id, got, tc.field)
		}
	}
	if NodeRuntimeField(Definition{Runtime: map[string]interface{}{"agent_mode": "custom_nodes"}}, Node{ID: "generate"}) != "" {
		t.Fatal("custom node became a runtime mirror")
	}
}

func TestMissingCallableNodesRemainEditableAndCompositionIsLocal(t *testing.T) {
	slots := Slots(Definition{Runtime: map[string]interface{}{"agent_mode": "custom_nodes"}, Nodes: []Node{{ID: "empty-chat", Type: "llm"}, {ID: "local", Type: "tool"}}})
	if len(slots) != 1 || slots[0].ID != "node.empty-chat" || slots[0].Optional {
		t.Fatalf("missing model slot hidden: %+v", slots)
	}
	for _, s := range Slots(Definition{Code: "ai_comic_drama", Nodes: []Node{{ID: "compose", Type: "video"}, {ID: "narrations", Type: "audio"}}}) {
		if s.ID == "node.compose" || s.ID == "node.narrations" && !s.Optional {
			t.Fatalf("local composition or optional narration became a required call: %+v", s)
		}
	}
}

func TestLegacyRequestModelAliasesCannotBypass(t *testing.T) {
	plainImage := Model{Code: "image", Category: "image", Mode: "images", Enabled: true}
	for _, code := range []string{"ai_photo_studio", "ai_virtual_tryon", "ai_novel_workshop"} {
		d := Effective(Definition{Code: code}, map[string]interface{}{"model_code": "image"})
		if err := ValidateDefinition(d, []Model{plainImage}); err == nil {
			t.Fatalf("%s accepted an incompatible legacy model override", code)
		}
	}
	d := Effective(Definition{Code: "ai_comic_drama"}, map[string]interface{}{"dialogue_model_code": "image"})
	if codes := Strings(d.Runtime["dialogue_model_codes"]); len(codes) != 1 || codes[0] != "image" {
		t.Fatal("singular dialogue alias did not resolve")
	}
}

func TestComicExecutionValidatesOnlyRemainingCalls(t *testing.T) {
	d := Definition{Code: "ai_comic_drama", Runtime: map[string]interface{}{"image_model_code": "retired", "video_model_code": "retired", "narration_model_code": "retired", "analysis_model_code": "retired"}}
	inputs := map[string]interface{}{"audio_strategy": "tts_only"}
	outputs := map[string]interface{}{"comic_drama": map[string]interface{}{"storyboards": []interface{}{map[string]interface{}{"id": "S01", "dialogue": "你好"}}}, "keyframes": []interface{}{map[string]interface{}{"id": "S01", "image_url": "one.jpg"}}, "segments": []interface{}{map[string]interface{}{"id": "S01", "video_url": "one.mp4"}}, "narrations": []interface{}{map[string]interface{}{"id": "S01", "audio_url": "one.wav"}}}
	work := RemainingComicWork(inputs, outputs, d.Runtime)
	if work != (ComicWork{}) {
		t.Fatalf("completed calls counted: %+v", work)
	}
	if err := ValidateComicWork(d, inputs, work, nil); err != nil {
		t.Fatalf("local composition blocked: %v", err)
	}
	delete(outputs, "segments")
	work = RemainingComicWork(inputs, outputs, d.Runtime)
	if work.Videos != 1 || work.Images != 0 || work.Narrations != 0 {
		t.Fatal(work)
	}
	if ValidateComicWork(d, inputs, work, nil) == nil {
		t.Fatal("missing video accepted unavailable model")
	}
	video := Model{Code: "new", Category: "video", Mode: "video", Enabled: true, Rule: map[string]interface{}{"video": map[string]interface{}{"upload_profile": "first_last"}}}
	inputs["video_model_code"] = "new"
	if err := ValidateComicWork(d, inputs, work, []Model{video}); err != nil {
		t.Fatal(err)
	}
	video.Rule = nil
	if ValidateComicWork(d, inputs, work, []Model{video}) == nil {
		t.Fatal("retry accepted text-only video model")
	}
}
