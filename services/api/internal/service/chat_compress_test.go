package service

import (
	"strings"
	"testing"

	"github.com/starai/api/internal/runtime"
)

func TestSplitChatForCompressionKeepsRecentTurns(t *testing.T) {
	messages := []runtime.ChatMessage{{Role: "system", Content: "规则"}}
	for i := 0; i < 10; i++ {
		messages = append(messages, runtime.ChatMessage{Role: "user", Content: strings.Repeat("字", 1000)})
	}
	system, older, recent, ok := splitChatForCompression(messages, 6000, 8, 6)
	if !ok || len(system) != 1 || len(older) != 4 || len(recent) != 6 {
		t.Fatalf("split = %d %d %d %v", len(system), len(older), len(recent), ok)
	}
}

func TestSplitChatForCompressionSkipsShortHistory(t *testing.T) {
	messages := []runtime.ChatMessage{{Role: "user", Content: "你好"}, {Role: "assistant", Content: "在"}}
	if _, _, _, ok := splitChatForCompression(messages, 6000, 8, 6); ok {
		t.Fatal("short history should not be compressed")
	}
}
