package util

import (
	"strings"
	"testing"
)

func TestSanitizeMessageHidesUpstreamIdentity(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		mustHave    string
		mustNotHave []string
	}{
		{
			name:        "url.Error 前缀",
			in:          `Post "https://hub.yardai.cc/v1/images/generations": unexpected EOF`,
			mustHave:    "unexpected EOF",
			mustNotHave: []string{"http", "yardai", "hub."},
		},
		{
			name:        "裸域名",
			in:          `dial tcp api.openrouter.ai:443: i/o timeout`,
			mustHave:    "i/o timeout",
			mustNotHave: []string{"openrouter", ".ai:"},
		},
		{
			name:        "裸 IP",
			in:          `connect to 10.0.0.12:8080 failed`,
			mustHave:    "failed",
			mustNotHave: []string{"10.0.0.12"},
		},
		{
			name:        "厂商模型代号",
			in:          `deepseek-v4-pro-0813 无可用线路`,
			mustHave:    "无可用线路",
			mustNotHave: []string{"deepseek"},
		},
		{
			name:        "正常业务文案保持原样",
			in:          "余额不足，请先充值",
			mustHave:    "余额不足，请先充值",
			mustNotHave: []string{"上游服务"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeMessage(tc.in)
			if !strings.Contains(got, tc.mustHave) {
				t.Fatalf("sanitized=%q, want it to contain %q", got, tc.mustHave)
			}
			for _, banned := range tc.mustNotHave {
				if strings.Contains(strings.ToLower(got), strings.ToLower(banned)) {
					t.Fatalf("sanitized=%q, must not contain %q", got, banned)
				}
			}
			if again := SanitizeMessage(got); again != got {
				t.Fatalf("not idempotent: first=%q second=%q", got, again)
			}
		})
	}
}

func TestSanitizeJSONDropsUpstreamOnlyFields(t *testing.T) {
	output := map[string]interface{}{
		"video_url":            "https://cdn.example.com/v.mp4",
		"upstream_content_url": "https://hub.yardai.cc/v1/videos/1/content",
		"upstream_model":       "seedance-1.0",
		"error_message":        `Get "https://hub.yardai.cc/v1/x": unexpected EOF`,
	}
	SanitizeJSON(output)

	if _, ok := output["upstream_content_url"]; ok {
		t.Fatal("upstream_content_url should be removed")
	}
	if _, ok := output["upstream_model"]; ok {
		t.Fatal("upstream_model should be removed")
	}
	if output["video_url"] != "https://cdn.example.com/v.mp4" {
		t.Fatalf("media url must survive, got %v", output["video_url"])
	}
	if msg := output["error_message"].(string); strings.Contains(strings.ToLower(msg), "yardai") {
		t.Fatalf("error_message leaked upstream: %q", msg)
	}
}

func TestSanitizeErrorTextHandlesNil(t *testing.T) {
	if got := SanitizeErrorText(nil); got != nil {
		t.Fatalf("nil should stay nil, got %v", *got)
	}
	raw := `Post "https://hub.yardai.cc/v1/x": unexpected EOF`
	got := SanitizeErrorText(&raw)
	if got == nil || strings.Contains(strings.ToLower(*got), "yardai") {
		t.Fatalf("unexpected result: %v", got)
	}
}
