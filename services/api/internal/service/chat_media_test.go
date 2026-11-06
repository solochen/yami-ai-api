package service

import (
	"testing"

	"github.com/starai/api/internal/runtime"
)

func TestChatMediaRouteCapabilities(t *testing.T) {
	rule := func(caps map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{"capabilities": caps}
	}
	model := &ModelFull{RuntimeRule: rule(map[string]interface{}{"vision": true, "video_analysis": true})}
	messages := []runtime.ChatMessage{{Role: "user", Content: "分析附件"}}
	for _, field := range []string{"reference_images", "reference_videos"} {
		req := chatRequestMessages(messages, map[string]interface{}{field: []string{"https://example.com/media"}})
		if err := chatRouteMediaError(model, &ModelRoute{Protocol: "gemini"}, req); err != nil {
			t.Fatal(err)
		}
		if err := chatRouteMediaError(model, &ModelRoute{RuntimeRule: rule(map[string]interface{}{"vision": false, "image_input": true, "video_analysis": false})}, req); err == nil {
			t.Fatal("disabled route accepted media")
		}
		// An unconfigured capability is unknown, not unsupported. Treating it as
		// "no" silently blocks models that do read images, because a wrong flag
		// cannot be distinguished from a missing one.
		if err := chatRouteMediaError(&ModelFull{}, &ModelRoute{}, req); err != nil {
			t.Fatalf("unknown capabilities must not block media: %v", err)
		}
	}
	video := chatRequestMessages(messages, map[string]interface{}{"reference_videos": []string{"https://example.com/a.mp4"}})
	if chatRouteMediaError(model, &ModelRoute{Protocol: "claude"}, video) == nil {
		t.Fatal("unsupported protocol accepted video")
	}
	if err := chatRouteMediaError(&ModelFull{}, &ModelRoute{}, messages); err != nil {
		t.Fatal("text-only request rejected", err)
	}
	image := chatRequestMessages(messages, map[string]interface{}{"reference_images": []string{"https://example.com/a.png"}})
	if err := chatRouteMediaError(&ModelFull{}, &ModelRoute{RuntimeRule: rule(map[string]interface{}{"image_input": true})}, image); err != nil {
		t.Fatal("route override ignored", err)
	}
}

func TestChatMediaExplicitFalseStillBlocks(t *testing.T) {
	image := chatRequestMessages(
		[]runtime.ChatMessage{{Role: "user", Content: "看图"}},
		map[string]interface{}{"reference_images": []string{"https://example.com/a.png"}},
	)
	explicitFalse := &ModelFull{RuntimeRule: map[string]interface{}{
		"capabilities": map[string]interface{}{"vision": false},
	}}
	if err := chatRouteMediaError(explicitFalse, &ModelRoute{}, image); err == nil {
		t.Fatal("an explicitly disabled model must still reject images")
	}
}

func TestMediaProbeBypassesTheGate(t *testing.T) {
	// The admin capability probe exists precisely to correct a wrong flag, so it
	// must be able to reach an upstream that is configured to reject media.
	image := chatRequestMessages(
		[]runtime.ChatMessage{{Role: "user", Content: "看图"}},
		map[string]interface{}{"reference_images": []string{"https://example.com/a.png"}},
	)
	explicitFalse := &ModelFull{RuntimeRule: map[string]interface{}{
		"capabilities": map[string]interface{}{"vision": false},
	}}
	if err := mediaGateError(WithMediaProbe(t.Context()), explicitFalse, &ModelRoute{}, image); err != nil {
		t.Fatalf("probe must bypass the media gate: %v", err)
	}
	if err := mediaGateError(t.Context(), explicitFalse, &ModelRoute{}, image); err == nil {
		t.Fatal("normal requests must still honour the gate")
	}
}
