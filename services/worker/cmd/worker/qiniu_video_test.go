package main

import (
	"context"
	"encoding/json"
	"github.com/starai/worker/videoparams"
	"testing"
)

func TestBuildQiniuVideoRequestViduTurboTextToVideo(t *testing.T) {
	runtimeRule := map[string]interface{}{
		"upstream": map[string]interface{}{
			"adapter": "qiniu_video",
			"family":  "vidu",
			"tier":    "turbo",
			"endpoints": map[string]interface{}{
				"text":        "/queue/fal-ai/vidu/q3/text-to-video/turbo",
				"first_frame": "/queue/fal-ai/vidu/q3/image-to-video/turbo",
			},
		},
	}
	input := map[string]interface{}{
		"prompt":         "一只白猫在草地上奔跑，阳光",
		"duration":       float64(3),
		"resolution":     "720p",
		"aspect_ratio":   "16:9",
		"generate_audio": true,
	}

	endpoint, payload := buildQiniuVideoRequest(context.Background(), "viduq3-turbo", "viduq3-turbo", "一只白猫在草地上奔跑，阳光", runtimeRule, input)
	raw, _ := json.Marshal(payload)
	t.Logf("endpoint=%s body=%s", endpoint, raw)

	if endpoint != "/queue/fal-ai/vidu/q3/text-to-video/turbo" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
	if payload["duration"] != 3 {
		t.Fatalf("duration missing/wrong: %#v", payload["duration"])
	}
	if payload["resolution"] != "720p" {
		t.Fatalf("resolution wrong: %#v", payload["resolution"])
	}
	if _, exists := payload["generate_audio"]; exists {
		t.Fatalf("vidu must not receive generate_audio")
	}
}

func TestBuildQiniuVideoRequestMinimaxResolutionCasing(t *testing.T) {
	runtimeRule := map[string]interface{}{
		"upstream": map[string]interface{}{
			"adapter": "qiniu_video",
			"family":  "minimax",
			"endpoints": map[string]interface{}{
				"text": "/queue/minimax/h3/text-to-video",
			},
		},
	}
	input := map[string]interface{}{"duration": float64(5), "resolution": "720p", "aspect_ratio": "16:9"}
	endpoint, payload := buildQiniuVideoRequest(context.Background(), "minimax/minimax-h3", "hailuo-3", "test", runtimeRule, input)
	raw, _ := json.Marshal(payload)
	t.Logf("endpoint=%s body=%s", endpoint, raw)
	if endpoint != "/queue/minimax/h3/text-to-video" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
	if payload["resolution"] != "768P" {
		t.Fatalf("720p should map to minimax 768P, got %#v", payload["resolution"])
	}
	if got := qiniuVideoResolution("minimax", map[string]interface{}{"resolution": "1080p"}); got != "2K" {
		t.Fatalf("1080p should map to minimax 2K, got %s", got)
	}
	if got := qiniuVideoResolution("vidu", map[string]interface{}{"resolution": "720p"}); got != "720p" {
		t.Fatalf("vidu keeps lowercase tiers, got %s", got)
	}
	// SanitizeUpstreamPayload 会按 _preserve_video_params 决定是否保留 duration。
	sanitized := videoparams.SanitizeUpstreamPayload(payload, endpoint)
	if sanitized["duration"] != 5 {
		t.Fatalf("duration must survive sanitizer, got %#v", sanitized["duration"])
	}
	if _, exists := sanitized["_preserve_video_params"]; exists {
		t.Fatalf("internal marker must not reach upstream")
	}
}

func TestBuildQiniuVideoRequestKlingFirstFrame(t *testing.T) {
	runtimeRule := map[string]interface{}{
		"upstream": map[string]interface{}{
			"adapter": "qiniu_video",
			"family":  "kling",
			"endpoints": map[string]interface{}{
				"text":        "/queue/fal-ai/kling-video/v3/pro/text-to-video",
				"first_frame": "/queue/fal-ai/kling-video/v3/pro/image-to-video",
				"first_last":  "/queue/fal-ai/kling-video/v3/pro/image-to-video",
			},
		},
	}
	input := map[string]interface{}{
		"duration":        float64(5),
		"aspect_ratio":    "16:9",
		"generate_audio":  true,
		"generation_mode": "first_last",
		"first_frame":     "https://cdn.example.com/a.jpg",
		"last_frame":      "https://cdn.example.com/b.jpg",
	}
	endpoint, payload := buildQiniuVideoRequest(context.Background(), "kling-v3", "kling-v3", "test", runtimeRule, input)
	raw, _ := json.Marshal(payload)
	t.Logf("endpoint=%s body=%s", endpoint, raw)
	if endpoint != "/queue/fal-ai/kling-video/v3/pro/image-to-video" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
	if payload["start_image_url"] != "https://cdn.example.com/a.jpg" {
		t.Fatalf("start_image_url wrong: %#v", payload["start_image_url"])
	}
	if payload["end_image_url"] != "https://cdn.example.com/b.jpg" {
		t.Fatalf("end_image_url wrong: %#v", payload["end_image_url"])
	}
	if payload["generate_audio"] != true {
		t.Fatalf("kling should carry generate_audio: %#v", payload["generate_audio"])
	}
}

func TestQiniuVideoModeKeyFallsBackToText(t *testing.T) {
	if got := qiniuVideoModeKey(map[string]interface{}{"prompt": "x"}); got != "text" {
		t.Fatalf("expected text mode, got %s", got)
	}
	if got := qiniuVideoModeKey(map[string]interface{}{"reference_images": []interface{}{"https://a/b.jpg"}}); got != "reference" {
		t.Fatalf("expected reference mode, got %s", got)
	}
}

func TestExtractMediaItemsFromQiniuFalStatus(t *testing.T) {
	body := []byte(`{"status":"COMPLETED","request_id":"qvideo-1","result":{"video":{"url":"https://cdn.qiniu.com/x.mp4","content_type":"video/mp4","duration":3}}}`)
	items, id := parseUpstreamMedia(body)
	if id != "qvideo-1" {
		t.Fatalf("request_id should be picked up, got %q", id)
	}
	if len(items) != 1 || items[0].URL != "https://cdn.qiniu.com/x.mp4" {
		t.Fatalf("nested result.video.url must be extracted, got %#v", items)
	}
}

func TestFalQueueEndpointDetection(t *testing.T) {
	cases := map[string]bool{
		"/queue/fal-ai/vidu/q3/text-to-video/turbo": true,
		"/queue/minimax/h3/image-to-video":          true,
		"/v1/videos":                                false,
		"/videos":                                   false,
		"":                                          false,
	}
	for endpoint, want := range cases {
		if got := isFalQueueEndpoint(endpoint); got != want {
			t.Fatalf("isFalQueueEndpoint(%q)=%v want %v", endpoint, got, want)
		}
	}
}

func TestBuildQiniuVideoRequestViduFirstFrame(t *testing.T) {
	runtimeRule := map[string]interface{}{
		"upstream": map[string]interface{}{
			"adapter": "qiniu_video",
			"family":  "vidu",
			"endpoints": map[string]interface{}{
				"text":        "/queue/fal-ai/vidu/q3/text-to-video/turbo",
				"first_frame": "/queue/fal-ai/vidu/q3/image-to-video/turbo",
			},
		},
	}
	dataURL := "data:image/png;base64,iVBORw0KGgo="
	input := map[string]interface{}{
		"duration":        float64(3),
		"resolution":      "540p",
		"aspect_ratio":    "16:9",
		"generation_mode": "first_frame",
		"first_frame":     dataURL,
	}
	endpoint, payload := buildQiniuVideoRequest(context.Background(), "viduq3-turbo", "viduq3-turbo", "动起来", runtimeRule, input)
	if endpoint != "/queue/fal-ai/vidu/q3/image-to-video/turbo" {
		t.Fatalf("first_frame must switch to image-to-video, got %s", endpoint)
	}
	if payload["image_url"] != dataURL {
		t.Fatalf("vidu image-to-video expects image_url, got %#v", payload["image_url"])
	}
	if !isFalQueueEndpoint(endpoint) {
		t.Fatalf("endpoint should be detected as fal queue")
	}
}
