package main

import (
	"context"
	"testing"
)

// new-api（hub）只透传 `image` 字段，input_references 会被丢弃；
// 直连 OpenRouter 反之。字段名由线路的 reference_field 声明。
func TestOpenRouterReferenceFieldSelection(t *testing.T) {
	cases := []struct {
		name string
		rule map[string]interface{}
		want string
	}{
		{"默认直连 OpenRouter", map[string]interface{}{}, "input_references"},
		{"显式 input_references", map[string]interface{}{"upstream": map[string]interface{}{"reference_field": "input_references"}}, "input_references"},
		{"hub 走 image", map[string]interface{}{"upstream": map[string]interface{}{"reference_field": "image"}}, "image"},
	}
	for _, tc := range cases {
		if got := openRouterReferenceField(tc.rule); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}

	payload := buildOpenRouterImagePayload(context.Background(), "google/gemini-3-pro-image", "gemini-3-pro-image",
		"prompt", 1, map[string]interface{}{"upstream": map[string]interface{}{"reference_field": "image"}},
		map[string]interface{}{"reference_images": []string{"data:image/png;base64,aGVsbG8="}})
	urls, ok := payload["image"].([]string)
	if !ok || len(urls) != 1 {
		t.Fatalf("hub 线路应把参考图放在 image 字段: %#v", payload["image"])
	}
	if _, exists := payload["input_references"]; exists {
		t.Fatal("hub 线路不应再发 input_references")
	}

	direct := buildOpenRouterImagePayload(context.Background(), "google/gemini-3-pro-image", "gemini-3-pro-image",
		"prompt", 1, map[string]interface{}{},
		map[string]interface{}{"reference_images": []string{"data:image/png;base64,aGVsbG8="}})
	if _, ok := direct["input_references"].([]map[string]interface{}); !ok {
		t.Fatalf("直连线路应使用 input_references: %#v", direct)
	}
	if _, exists := direct["image"]; exists {
		t.Fatal("直连线路不应夹带 image 字段")
	}
}
