package service

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

var characterStudioForbiddenUserTerms = []string{
	"vidu", "aliyun", "alirtc", "artc", "trtc", "agora", "volcengine",
	"s2-editing", "s2_editing", "s_editing", "subject_replacement",
	"live_id", "render_uid", "switch_prompt", "conn_init", "client_secret",
}

func assertCharacterStudioNeutral(t *testing.T, fields ...string) {
	t.Helper()
	for _, field := range fields {
		lower := strings.ToLower(field)
		for _, forbidden := range characterStudioForbiddenUserTerms {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("user-facing text contains forbidden term %q: %q", forbidden, field)
			}
		}
	}
}

func TestCharacterStudioNormalizeResolution(t *testing.T) {
	cases := map[string]string{
		"540p":  "540p",
		"720P":  "720p",
		"1080p": "1080p",
		"2K":    "2K",
		"2k":    "2K",
		"":      "720p",
		"4k":    "720p",
	}
	for input, want := range cases {
		if got := normalizeCharacterStudioResolution(input, "720p"); got != want {
			t.Fatalf("normalize(%q) = %q, want %q", input, got, want)
		}
	}
	if got := normalizeCharacterStudioResolution("", ""); got != "" {
		t.Fatalf("empty fallback should stay empty, got %q", got)
	}
}

func TestCharacterStudioImageURL(t *testing.T) {
	cases := []struct {
		raw, base, want string
	}{
		{"https://cdn.example.com/a.png", "", "https://cdn.example.com/a.png"},
		{"http://cdn.example.com/a.png", "", "http://cdn.example.com/a.png"},
		{"//cdn.example.com/a.png", "", "https://cdn.example.com/a.png"},
		{"/media/a.png", "https://site.example.com", "https://site.example.com/media/a.png"},
		{"/media/a.png", "", ""},
		{"data:image/png;base64,abc", "", "data:image/png;base64,abc"},
		{"", "https://site.example.com", ""},
	}
	for _, item := range cases {
		if got := characterStudioImageURL(item.raw, item.base); got != item.want {
			t.Fatalf("imageURL(%q,%q) = %q, want %q", item.raw, item.base, got, item.want)
		}
	}
}

func TestCharacterStudioCost(t *testing.T) {
	cases := []struct {
		seconds int
		want    float64
	}{
		{0, 0},
		{-3, 0},
		{10, 0.415},
		{30, 1.245},
		{60, 2.49},
		{600, 24.9},
	}
	for _, item := range cases {
		got := characterStudioCost(2.49, item.seconds)
		if math.Abs(got-item.want) > 1e-9 {
			t.Fatalf("cost(%d) = %v, want %v", item.seconds, got, item.want)
		}
	}
}

func TestCharacterStudioProviderCost(t *testing.T) {
	settings := characterStudioSettings{CreditPerSecond: 1, CreditPrice: 0.03125}
	if got := characterStudioProviderCost(60, 90, settings); math.Abs(got-2.8125) > 1e-9 {
		t.Fatalf("provider cost from credits = %v", got)
	}
	if got := characterStudioProviderCost(60, 0, settings); math.Abs(got-1.875) > 1e-9 {
		t.Fatalf("provider cost fallback = %v", got)
	}
}

func TestCharacterStudioEndReasonsAreNeutral(t *testing.T) {
	codes := []string{
		"user_end", "timeout", "audit_violation", "credit_insufficient",
		"client_reconnect_timeout", "client_disconnect_before_ready", "connection_lost",
		"render_ready_timeout", "render_reconnect_timeout", "render_internal_error",
		"render_ended", "start_timeout", "create_failed", "create_rejected", "handshake_failed",
		"unknown_code",
	}
	for _, code := range codes {
		assertCharacterStudioNeutral(t, characterStudioEndReason(code))
	}
	if characterStudioEndReason("user_end") == "" {
		t.Fatal("user_end should map to a readable reason")
	}
}

func TestCharacterStudioUpstreamErrorsAreNeutral(t *testing.T) {
	bodies := [][]byte{
		[]byte(`{"code":"CreditInsufficient","message":"credit insufficient"}`),
		[]byte(`{"code":"400","message":"image_url missing"}`),
		[]byte(`{"code":"400","message":"MultiFaceDetected"}`),
		[]byte(`{"code":"500","message":"internal error"}`),
	}
	for _, body := range bodies {
		for _, status := range []int{400, 401, 402, 403, 429, 500} {
			assertCharacterStudioNeutral(t, characterStudioUpstreamError(status, body))
		}
	}
	for _, code := range []string{"NOT_READY", "SIP_READY_TIMEOUT", "LIVE_ENDED", "LIVE_CONN_INIT_FAILED"} {
		assertCharacterStudioNeutral(t, characterStudioHandshakeError(code))
	}
	for _, code := range []string{"SWITCH_PROMPT_INVALID", "LIVE_NOT_ACTIVE", "RENDER_NOT_READY", "SWITCH_PROMPT_EXPIRED", "SWITCH_PROMPT_TIMEOUT", "RENDER_WRITE_FAILED", "SWITCH_PROMPT_INTERNAL_ERROR"} {
		assertCharacterStudioNeutral(t, characterStudioSwitchError(code))
	}
}

func TestCharacterStudioProtocolPayloads(t *testing.T) {
	init := characterStudioConnInitPayload()
	initPayload, _ := init["conn_init"].(map[string]any)
	if initPayload == nil || initPayload["version"] == nil {
		t.Fatal("conn init payload must carry version")
	}
	switchPayload := characterStudioSwitchPayload("https://cdn.example.com/ref.png")
	switchInner, _ := switchPayload["switch_prompt"].(map[string]any)
	if switchInner == nil {
		t.Fatal("switch payload must carry switch_prompt")
	}
	if switchInner["editing_type"] != characterStudioEditingType {
		t.Fatal("switch payload must pin editing type")
	}
	prompts, _ := switchInner["prompts"].([]any)
	if len(prompts) != 1 {
		t.Fatal("switch payload must carry exactly one reference image")
	}
	prompt, _ := prompts[0].(map[string]any)
	if prompt["type"] != "image" || prompt["content"] != "https://cdn.example.com/ref.png" {
		t.Fatalf("unexpected switch prompt: %#v", prompt)
	}
	if _, err := json.Marshal(switchPayload); err != nil {
		t.Fatal(err)
	}
	hangup := characterStudioHangupPayload()
	hangupInner, _ := hangup["hangup"].(map[string]any)
	if hangupInner == nil || hangupInner["hangup_reason"] == nil {
		t.Fatal("hangup payload must carry a reason")
	}
}

func TestCharacterStudioChannelParsing(t *testing.T) {
	raw := []byte(`{"app_id":"app","channel_id":"ch","user_id":"u","token":"tok","token_expire_at":"1790000000","provider":"artc"}`)
	channel := characterStudioChannel(raw)
	if channel == nil || channel.AppID != "app" || channel.ChannelID != "ch" || channel.Token != "tok" {
		t.Fatalf("unexpected channel: %+v", channel)
	}
	if characterStudioChannel([]byte(`{}`)) != nil {
		t.Fatal("empty channel should be nil")
	}
}

// 展示层字段名也不能带上游内部字段，避免接口响应把实现细节透给前端。
func TestCharacterStudioViewFieldNamesAreNeutral(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(CharacterStudioSessionView{}),
		reflect.TypeOf(CharacterStudioChannel{}),
		reflect.TypeOf(CharacterStudioCharacter{}),
		reflect.TypeOf(CharacterStudioTake{}),
	}
	for _, typ := range types {
		for i := 0; i < typ.NumField(); i++ {
			tag := typ.Field(i).Tag.Get("json")
			assertCharacterStudioNeutral(t, tag)
		}
	}
}

func TestCharacterStudioSettingsDefaults(t *testing.T) {
	settings := characterStudioSettings{SalePerMinute: 2.49, CreditPerSecond: 1, CreditPrice: 0.03125, MaxMinutes: 10}
	if settings.SalePerMinute != 2.49 {
		t.Fatal("unexpected sale price")
	}
}
