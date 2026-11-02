package main

import (
	"strings"
	"testing"
)

func TestSanitizeUserFacingErrorHidesUpstreamIdentity(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		mustHave    []string
		mustNotHave []string
	}{
		{
			name:        "url.Error 前缀（用户实际遇到的报错）",
			in:          `Post "https://hub.yardai.cc/v1/images/generations": unexpected EOF`,
			mustHave:    []string{"unexpected EOF"},
			mustNotHave: []string{"http", "yardai", "hub.", "images/generations"},
		},
		{
			name:        "裸域名带端口",
			in:          `dial tcp api.openai.com:443: connect: connection refused`,
			mustHave:    []string{"connection refused"},
			mustNotHave: []string{"openai", "api.", ".com"},
		},
		{
			name:        "裸 IP",
			in:          `connect to 43.134.218.25:8080 failed`,
			mustHave:    []string{"failed"},
			mustNotHave: []string{"43.134.218.25"},
		},
		{
			name:        "厂商模型代号",
			in:          `gemini-3-pro-image 该线路不可用`,
			mustHave:    []string{"该线路不可用"},
			mustNotHave: []string{"gemini"},
		},
		{
			name:        "正常文案不受影响",
			in:          `生成内容未通过上游安全审核，请修改提示词`,
			mustHave:    []string{"生成内容未通过上游安全审核"},
			mustNotHave: []string{"上游服务"},
		},
		{
			name:        "空字符串",
			in:          "   ",
			mustNotHave: []string{"上游服务"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeUserFacingError(tc.in)
			for _, want := range tc.mustHave {
				if !strings.Contains(got, want) {
					t.Fatalf("sanitized=%q, want it to contain %q", got, want)
				}
			}
			for _, banned := range tc.mustNotHave {
				if strings.Contains(strings.ToLower(got), strings.ToLower(banned)) {
					t.Fatalf("sanitized=%q, must not contain %q", got, banned)
				}
			}
			if again := sanitizeUserFacingError(got); again != got {
				t.Fatalf("not idempotent: first=%q second=%q", got, again)
			}
		})
	}
}

func TestHumanizeUpstreamFailureStillTranslatesBeforeSanitizing(t *testing.T) {
	got := humanizeUpstreamFailure("upstream HTTP 502: error code: 502")
	if !strings.Contains(got, "上游网关转发失败") {
		t.Fatalf("502 mapping lost: %q", got)
	}
	got = humanizeUpstreamFailure(`Post "https://hub.yardai.cc/v1/chat/completions": dial tcp: connection refused`)
	if strings.Contains(strings.ToLower(got), "yardai") || strings.Contains(got, "http") {
		t.Fatalf("upstream leaked: %q", got)
	}
}

func TestSanitizeUserFacingJSONDropsUpstreamOnlyFields(t *testing.T) {
	outputs := map[string]interface{}{
		"current_step": "result",
		"media_tasks": []interface{}{
			map[string]interface{}{
				"task_no":              "T1",
				"error_message":        `Post "https://hub.yardai.cc/v1/images/generations": unexpected EOF`,
				"upstream_content_url": "https://hub.yardai.cc/v1/videos/abc/content",
				"upstream_model":       "gemini-3-pro-image",
				"output": map[string]interface{}{
					"image_url":      "https://cdn.example.com/a.png",
					"upstream_model": "gpt-image-2",
				},
			},
		},
	}
	sanitizeUserFacingJSON(outputs)

	task := outputs["media_tasks"].([]interface{})[0].(map[string]interface{})
	if _, ok := task["upstream_content_url"]; ok {
		t.Fatal("upstream_content_url should be removed")
	}
	if _, ok := task["upstream_model"]; ok {
		t.Fatal("upstream_model should be removed")
	}
	msg := task["error_message"].(string)
	if strings.Contains(strings.ToLower(msg), "yardai") || strings.Contains(msg, "http") {
		t.Fatalf("error message leaked upstream: %q", msg)
	}
	inner := task["output"].(map[string]interface{})
	if _, ok := inner["upstream_model"]; ok {
		t.Fatal("nested upstream_model should be removed")
	}
	if inner["image_url"] != "https://cdn.example.com/a.png" {
		t.Fatalf("media url must be preserved, got %v", inner["image_url"])
	}
}
