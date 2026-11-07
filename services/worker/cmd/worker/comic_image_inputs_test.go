package main

import "testing"

// 成片分辨率不能透传给图片模型；图片任务必须使用模型自己的质量档位。
func TestComicImageTaskInputsDropsVideoOnlyParams(t *testing.T) {
	inputs := map[string]interface{}{
		"quality":        "480P",
		"resolution":     "480p",
		"duration":       5,
		"duration_sec":   5,
		"generate_audio": true,
		"orientation":    "landscape",
		"count":          3,
	}
	out := comicImageTaskInputs(inputs, nil)
	for _, key := range []string{"quality", "resolution", "duration", "duration_sec", "generate_audio"} {
		if _, ok := out[key]; ok {
			t.Fatalf("%s should not be forwarded to image models", key)
		}
	}
	if out["aspect_ratio"] != "16:9" || out["ratio"] != "16:9" {
		t.Fatalf("comic keyframes must follow the film aspect ratio: %v", out)
	}
	if out["count"] != 3 {
		t.Fatalf("unrelated inputs were dropped: %v", out)
	}
	if inputs["quality"] != "480P" {
		t.Fatal("input map must not be mutated")
	}
}

// 迁移 169 后站内素材是 /media/... 相对地址；成功落库的结果不能被判为空。
func TestComicMediaURLAcceptsRelativeManagedMedia(t *testing.T) {
	output := map[string]interface{}{"image_url": "/media/starai-works/works/image/task_1/1.jpg"}
	if got := comicMediaURL(output, "image_url", "url"); got != "/media/starai-works/works/image/task_1/1.jpg" {
		t.Fatalf("relative media rejected: %q", got)
	}
	if got := comicMediaURL(map[string]interface{}{"video_url": "https://cdn.example.com/a.mp4"}, "video_url"); got != "https://cdn.example.com/a.mp4" {
		t.Fatalf("absolute media rejected: %q", got)
	}
	if got := comicMediaURL(map[string]interface{}{"image_url": "not-a-url"}, "image_url"); got != "" {
		t.Fatalf("invalid media accepted: %q", got)
	}
	for _, m := range []map[string]interface{}{nil, {}, {"image_url": nil}} {
		if got := comicMediaURL(m, "image_url", "url"); got != "" {
			t.Fatalf("unexpected media URL: %q", got)
		}
	}
}

// 语音合成等按量计费的媒体任务依赖字符数核算上游成本，内部标记仍应被过滤。
func TestAgentMediaTaskInputKeepsBillingEstimates(t *testing.T) {
	in := map[string]interface{}{
		"_estimated_input_tokens":  21,
		"_estimated_output_tokens": 0,
		"_mode":                    "auto",
		"_shot_index":              1,
		"voice_id":                 "female-shaonv",
	}
	out := agentMediaTaskInput(in, "一句旁白", "wfp_test")
	if out["_estimated_input_tokens"] != 21 {
		t.Fatalf("billing estimate stripped: %v", out)
	}
	for _, key := range []string{"_mode", "_shot_index"} {
		if _, ok := out[key]; ok {
			t.Fatalf("internal field %s leaked into media task", key)
		}
	}
	if out["voice_id"] != "female-shaonv" || out["prompt"] != "一句旁白" {
		t.Fatalf("regular inputs missing: %v", out)
	}
}
