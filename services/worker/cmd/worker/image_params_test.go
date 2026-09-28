package main

import (
	"context"
	"strings"
	"testing"
)

func TestResolveImageGenerationInputFallsBackUnsupportedBananaRatio(t *testing.T) {
	input := map[string]interface{}{
		"aspect_ratio": "4:5",
		"image_size":   "4K",
	}

	resolveImageGenerationInput(input, nil, "/v1/videos", "nano_banana_pro-2K")

	if got := input["aspect_ratio"]; got != "1:1" {
		t.Fatalf("aspect_ratio = %v, want 1:1", got)
	}
	if got := input["image_size"]; got != "4K" {
		t.Fatalf("image_size = %v, want 4K", got)
	}
	if got := input["size"]; got != "2880x2880" {
		t.Fatalf("size = %v, want 2880x2880", got)
	}
}

func TestImageModelForSizeMapsAsyncImageFamilies(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		model    string
		tier     string
		want     string
	}{
		{name: "banana 1k", endpoint: "/v1/videos", model: "nano_banana_2", tier: "1K", want: "nano_banana_pro-1K"},
		{name: "banana 4k", endpoint: "/v1/videos", model: "nano_banana_pro-1K", tier: "4K", want: "nano_banana_pro-4K"},
		{name: "gpt image keeps configured model", endpoint: "/v1/videos", model: "gpt-image-2", tier: "4K", want: "gpt-image-2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := imageModelForSize(nil, tt.endpoint, tt.model, "", tt.tier); got != tt.want {
				t.Fatalf("model = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestImageModelForSizePrefersRuntimeRule(t *testing.T) {
	rule := map[string]interface{}{
		"image": map[string]interface{}{
			"model_by_size": map[string]interface{}{
				"2K": "custom-image-2k",
			},
		},
	}

	if got := imageModelForSize(rule, "/v1/videos", "gpt-image-2", "", "2K"); got != "custom-image-2k" {
		t.Fatalf("model = %s, want custom-image-2k", got)
	}
}

func TestGeminiNativePayloadImageSizeOnlyForFlash(t *testing.T) {
	input := map[string]interface{}{
		"aspect_ratio": "16:9",
		"image_size":   "4K",
	}

	flash := buildGeminiNativeImagePayload(nil, "gemini-3.1-flash-image-preview", "", "prompt", input)
	flashCfg := flash["generationConfig"].(map[string]interface{})["imageConfig"].(map[string]interface{})
	if got := flashCfg["imageSize"]; got != "4K" {
		t.Fatalf("flash imageSize = %v, want 4K", got)
	}

	pro := buildGeminiNativeImagePayload(nil, "gemini-3-pro-image-preview", "", "prompt", input)
	proCfg := pro["generationConfig"].(map[string]interface{})["imageConfig"].(map[string]interface{})
	if _, ok := proCfg["imageSize"]; ok {
		t.Fatalf("pro payload should not include imageSize: %#v", proCfg)
	}
}

func TestBuildVideoImagePayloadIncludesBananaReferenceImages(t *testing.T) {
	input := map[string]interface{}{
		"reference_images": []string{
			"data:image/png;base64,Zmlyc3Q=",
			"data:image/jpeg;base64,c2Vjb25k",
		},
	}

	payload := buildVideoImagePayload(context.Background(), nil, "/v1/videos", "nano_banana_2", "", "prompt", input)
	images, ok := payload["images"].([]string)
	if !ok {
		t.Fatalf("images type = %T, want []string; payload=%#v", payload["images"], payload)
	}
	if len(images) != 2 || images[0] != "data:image/png;base64,Zmlyc3Q=" || images[1] != "data:image/jpeg;base64,c2Vjb25k" {
		t.Fatalf("images = %#v, want both uploaded references", images)
	}
}

func TestBuildVideoImagePayloadIncludesGPTImageSizeWithoutChangingModel(t *testing.T) {
	input := map[string]interface{}{
		"size":             "1456x624",
		"image_size":       "4K",
		"reference_images": []string{"https://star-ai.example/product.jpg"},
	}

	payload := buildVideoImagePayload(context.Background(), nil, "/v1/videos", "gpt-image-2", "", "prompt", input)
	if payload["model"] != "gpt-image-2" {
		t.Fatalf("model = %v, want gpt-image-2", payload["model"])
	}
	if payload["size"] != "1456x624" {
		t.Fatalf("size = %v, want 1456x624", payload["size"])
	}
	if _, exists := payload["aspect_ratio"]; exists {
		t.Fatalf("GPT Image async payload should use size only: %#v", payload)
	}
	images, ok := payload["images"].([]string)
	if !ok || len(images) != 1 || images[0] != "https://star-ai.example/product.jpg" {
		t.Fatalf("public image URL was not preserved: %#v", payload["images"])
	}
}

func TestGPTImage25TransportOptions(t *testing.T) {
	input := map[string]interface{}{"aspect_ratio": "auto", "image_size": "4K"}
	rule := map[string]interface{}{"image": map[string]interface{}{"supported_size_tiers": []interface{}{"1K", "2K", "4K"}}}
	resolveImageGenerationInput(input, rule, "/v1/videos", "gpt-image-2.5-flare")
	payload := buildVideoImagePayload(context.Background(), rule, "/v1/videos", "gpt-image-2.5-flare", "", "prompt", input)
	if input["aspect_ratio"] != "auto" || payload["image_size"] != "4K" {
		t.Fatalf("auto ratio or tier lost: input=%#v payload=%#v", input, payload)
	}
	refs := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}
	payload = buildVideoImagePayload(context.Background(), nil, "/v1/videos", "gpt-image-2.5-sunburst", "", "prompt", map[string]interface{}{"reference_images": refs})
	if images, ok := payload["images"].([]string); !ok || len(images) != 8 {
		t.Fatalf("images = %#v, want first 8 references", payload["images"])
	}
	endpoint, err := resolveImageRequestEndpoint(map[string]interface{}{"upstream": map[string]interface{}{"adapter": "otuapi_image", "edit_endpoint": "/v1/images/edits"}}, "/v1/images/generations", "gpt-image2", map[string]interface{}{"reference_images": []string{"https://example.com/ref.png"}})
	if err != nil || endpoint != "/v1/images/edits" {
		t.Fatalf("sync edit endpoint = %q, err=%v", endpoint, err)
	}
}

func TestBuildVideoImagePayloadFallsBackToImageURL(t *testing.T) {
	input := map[string]interface{}{
		"image_url": "data:image/png;base64,cGhvbmU=",
	}

	payload := buildVideoImagePayload(context.Background(), nil, "/v1/videos", "nano_banana_2", "", "prompt", input)
	images, ok := payload["images"].([]string)
	if !ok || len(images) != 1 || images[0] != "data:image/png;base64,cGhvbmU=" {
		t.Fatalf("image_url was not forwarded as Nano Banana images: %#v", payload)
	}
}

func TestNormalizePayloadMediaPreservesPublicAsyncImageURLs(t *testing.T) {
	payload := map[string]interface{}{
		"images": []string{"https://star-ai.example/product.jpg"},
	}
	if err := normalizePayloadMedia(context.Background(), payload, "/v1/videos"); err != nil {
		t.Fatal(err)
	}
	images, ok := payload["images"].([]string)
	if !ok || len(images) != 1 || images[0] != "https://star-ai.example/product.jpg" {
		t.Fatalf("public image URL was rewritten: %#v", payload["images"])
	}
}

func TestAgentPromptLocksUploadedReferenceSubject(t *testing.T) {
	prompt := agentPromptWithScene("create a premium product shot", map[string]interface{}{
		"image_url": "https://cdn.example/phone.png",
	})

	for _, required := range []string{"REFERENCE IMAGE GUIDANCE", "authoritative subject", "features the user has not explicitly requested to edit", "user-confirmed design, pose and viewpoint edits override these defaults"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt does not contain %q: %s", required, prompt)
		}
	}
}

func TestImagePoliciesBindPrimaryAndAuxiliaryReferences(t *testing.T) {
	prompt := applyImageGenerationPolicies("保持鞋子后视角，鞋两边合理显示能观察到的文字部分", map[string]interface{}{
		"language":         "中文",
		"reference_images": []string{"https://cdn.example/product.jpg", "https://cdn.example/pose.jpg"},
	})
	for _, required := range []string{"Reference image 1 is the authoritative source", "References 2 and later are auxiliary", "Do not copy their product shape", "TEXT RENDERING POLICY:"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("image policy does not contain %q: %s", required, prompt)
		}
	}
	for _, forbidden := range []string{"Generate all visible text", "LANGUAGE HARD REQUIREMENT:"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("product-marking request incorrectly enabled generated copy %q: %s", forbidden, prompt)
		}
	}
	if got := applyImageGenerationPolicies(prompt, map[string]interface{}{"language": "中文", "reference_images": []string{"https://cdn.example/product.jpg", "https://cdn.example/pose.jpg"}}); got != prompt {
		t.Fatal("image policies were duplicated on the worker's second pass")
	}
}

func TestImagePoliciesAllowOnlyExplicitPosterCopy(t *testing.T) {
	prompt := applyImageGenerationPolicies("添加文案：夏日上新", map[string]interface{}{
		"language":       "中文",
		"creative_scene": "marketing_poster",
	})
	if !strings.Contains(prompt, "LANGUAGE HARD REQUIREMENT:") || !strings.Contains(prompt, "only the text explicitly requested") {
		t.Fatalf("explicit poster copy lost its language policy: %s", prompt)
	}
	if strings.Contains(prompt, "Generate all visible text") {
		t.Fatalf("old generate-everything instruction survived: %s", prompt)
	}
}

func TestAgentPromptDoesNotAddReferenceLockWithoutReference(t *testing.T) {
	prompt := agentPromptWithScene("create a premium product shot", map[string]interface{}{})
	if strings.Contains(prompt, "REFERENCE IMAGE GUIDANCE") {
		t.Fatalf("reference lock added without a reference: %s", prompt)
	}
}

func TestBuildOpenRouterImagePayload(t *testing.T) {
	rule := map[string]interface{}{
		"upstream": map[string]interface{}{"adapter": "openrouter_image"},
		"image":    map[string]interface{}{"supported_size_tiers": []interface{}{"1K", "2K"}, "count_max": float64(1)},
	}
	if !isOpenRouterImageAdapter(rule) {
		t.Fatal("adapter not recognized")
	}
	if got := clampOpenRouterImageCount(rule, 4); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
	payload := buildOpenRouterImagePayload(context.Background(), "bytedance-seed/seedream-5-0-pro", "seedream-5-0-pro", "一只猫", 1, rule, map[string]interface{}{
		"aspect_ratio":     "16:9",
		"image_size":       "2K",
		"size":             "2560x1440",
		"reference_images": []string{"https://example.com/a.png"},
	})
	if payload["resolution"] != "2K" || payload["aspect_ratio"] != "16:9" {
		t.Fatalf("payload = %#v", payload)
	}
	if _, ok := payload["size"]; ok {
		t.Fatal("pixel size must not be sent to OpenRouter")
	}
	refs, _ := payload["input_references"].([]map[string]interface{})
	if len(refs) != 1 || refs[0]["type"] != "image_url" {
		t.Fatalf("references = %#v", payload["input_references"])
	}
	noTier := map[string]interface{}{"upstream": map[string]interface{}{"adapter": "openrouter_image"}, "image": map[string]interface{}{}}
	plain := buildOpenRouterImagePayload(context.Background(), "openai/gpt-image-2", "gpt-image-2", "一只猫", 1, noTier, map[string]interface{}{"image_size": "1K", "quality": "high"})
	if _, ok := plain["resolution"]; ok {
		t.Fatal("resolution sent to a model that does not accept it")
	}
	if plain["quality"] != "high" {
		t.Fatalf("quality = %v", plain["quality"])
	}
}

func TestBuildOpenRouterVideoPayloadUsesOpeningFrame(t *testing.T) {
	rule := map[string]interface{}{
		"upstream": map[string]interface{}{"adapter": "openrouter_video"},
		"video":    map[string]interface{}{"supported_frame_images": []interface{}{"first_frame", "last_frame"}},
	}
	payload := buildOpenRouterVideoPayload(context.Background(), "google/veo-3.1", "veo-3-1", "雨夜", rule, map[string]interface{}{
		"duration":         float64(8),
		"resolution":       "720p",
		"aspect_ratio":     "16:9",
		"reference_images": []string{"https://example.com/start.png"},
	})
	if payload["duration"] != 8 || payload["resolution"] != "720p" || payload["aspect_ratio"] != "16:9" {
		t.Fatalf("payload = %#v", payload)
	}
	frames, _ := payload["frame_images"].([]map[string]interface{})
	if len(frames) != 1 || frames[0]["frame_type"] != "first_frame" {
		t.Fatalf("frames = %#v", payload["frame_images"])
	}
}

func TestBuildOpenRouterVideoPayloadReferenceMix(t *testing.T) {
	rule := map[string]interface{}{
		"upstream": map[string]interface{}{"adapter": "openrouter_video"},
		"video":    map[string]interface{}{"supported_frame_images": []interface{}{"first_frame", "last_frame"}},
	}
	payload := buildOpenRouterVideoPayload(context.Background(), "minimax/hailuo-3", "hailuo-3", "走路", rule, map[string]interface{}{
		"generation_mode":  "reference",
		"reference_images": []string{"https://example.com/a.png", "https://example.com/b.png"},
		"reference_videos": []string{"https://example.com/move.mp4"},
		"reference_audios": []string{"https://example.com/voice.mp3"},
		"generate_audio":   true,
	})
	if _, ok := payload["frame_images"]; ok {
		t.Fatal("reference mode must not send frame images")
	}
	refs, _ := payload["input_references"].([]map[string]interface{})
	if len(refs) != 4 || refs[2]["type"] != "video_url" || refs[3]["type"] != "audio_url" {
		t.Fatalf("references = %#v", payload["input_references"])
	}
	if payload["generate_audio"] != true {
		t.Fatalf("audio flag = %#v", payload["generate_audio"])
	}
}

func TestFirstUnsignedVideoURL(t *testing.T) {
	raw := map[string]interface{}{"unsigned_urls": []interface{}{"https://openrouter.ai/api/v1/videos/abc/content?index=0"}}
	if got := firstUnsignedVideoURL(raw); !strings.Contains(got, "/videos/abc/content") {
		t.Fatalf("url = %q", got)
	}
}

func TestUpstreamUSDCost(t *testing.T) {
	if got := upstreamUSDCost([]byte(`{"data":[{"b64_json":"abc"}],"usage":{"cost":0.045}}`)); got != 0.045 {
		t.Fatalf("cost = %v", got)
	}
}

func TestAgentPromptDoesNotTreatComicStyleCoverAsSubjectReference(t *testing.T) {
	prompt := agentPromptWithScene("create a premium product shot", map[string]interface{}{
		"comic_style": map[string]interface{}{"cover_url": "https://cdn.example/style-cover.png"},
	})
	if strings.Contains(prompt, "REFERENCE IMAGE GUIDANCE") {
		t.Fatalf("style cover incorrectly locked as the generated subject: %s", prompt)
	}
}
