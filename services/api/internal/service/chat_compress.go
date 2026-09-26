package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"strings"

	"github.com/starai/api/internal/runtime"
)

type chatCompressionSettings struct {
	Enabled      bool
	ModelCode    string
	MinChars     int
	MinMessages  int
	KeepMessages int
}

func (s *ChatService) chatCompressionSettings(ctx context.Context) chatCompressionSettings {
	settings := chatCompressionSettings{Enabled: true, MinChars: 6000, MinMessages: 8, KeepMessages: 6}
	values := map[string]*string{}
	rows, err := s.db.Query(ctx, `SELECT key, value #>> '{}' FROM system_configs WHERE key LIKE 'chat_compression_%'`)
	if err != nil {
		return settings
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if rows.Scan(&key, &value) == nil {
			values[key] = &value
		}
	}
	if raw := values["chat_compression_enabled"]; raw != nil && (*raw == "false" || *raw == "0") {
		settings.Enabled = false
	}
	if raw := values["chat_compression_model_code"]; raw != nil {
		settings.ModelCode = strings.TrimSpace(*raw)
	}
	if n := configPositiveInt(values["chat_compression_min_chars"]); n > 0 {
		settings.MinChars = n
	}
	if n := configPositiveInt(values["chat_compression_min_messages"]); n > 0 {
		settings.MinMessages = n
	}
	if n := configPositiveInt(values["chat_compression_keep_messages"]); n > 0 {
		settings.KeepMessages = n
	}
	return settings
}

func configPositiveInt(raw *string) int {
	if raw == nil {
		return 0
	}
	n := 0
	for _, r := range strings.TrimSpace(*raw) {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func splitChatForCompression(messages []runtime.ChatMessage, minChars, minMessages, keep int) (system, older, recent []runtime.ChatMessage, ok bool) {
	if keep < 2 {
		keep = 6
	}
	var dialogue []runtime.ChatMessage
	var chars int
	for _, message := range messages {
		if strings.EqualFold(message.Role, "system") {
			system = append(system, message)
			continue
		}
		dialogue = append(dialogue, message)
		chars += len([]rune(message.Content))
	}
	if len(dialogue) <= keep || (chars < minChars && len(dialogue) <= minMessages) {
		return nil, nil, nil, false
	}
	cut := len(dialogue) - keep
	return system, dialogue[:cut], dialogue[cut:], true
}

func compressionHash(messages []runtime.ChatMessage) string {
	hash := sha256.New()
	for _, message := range messages {
		hash.Write([]byte(message.Role))
		hash.Write([]byte{0})
		hash.Write([]byte(message.Content))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (s *ChatService) loadCompressionSummary(ctx context.Context, userID int64, publicID, hash string) string {
	if publicID == "" || hash == "" {
		return ""
	}
	var summary string
	err := s.db.QueryRow(ctx, `SELECT compression_summary FROM conversations WHERE public_id=$1 AND user_id=$2 AND compression_hash=$3`, publicID, userID, hash).Scan(&summary)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(summary)
}

func (s *ChatService) saveCompressionSummary(ctx context.Context, userID int64, publicID, hash, summary string) {
	if publicID == "" || hash == "" || strings.TrimSpace(summary) == "" {
		return
	}
	_, _ = s.db.Exec(ctx, `UPDATE conversations SET compression_hash=$3, compression_summary=$4 WHERE public_id=$1 AND user_id=$2`, publicID, userID, hash, summary)
}

// MaybeCompressContext replaces older chat turns with a summary before the main model call.
// A compression failure leaves the original messages in place.
func (s *ChatService) MaybeCompressContext(ctx context.Context, userID int64, input CompletionInput) CompletionInput {
	if input.Params["channel_key"] != nil || strings.TrimSpace(stringValue(input.Params["compression_model"])) == "off" {
		return input
	}
	settings := s.chatCompressionSettings(ctx)
	if !settings.Enabled {
		return input
	}
	system, older, recent, ok := splitChatForCompression(input.Messages, settings.MinChars, settings.MinMessages, settings.KeepMessages)
	if !ok {
		return input
	}
	modelCode := settings.ModelCode
	if chosen := strings.TrimSpace(stringValue(input.Params["compression_model"])); chosen != "" && chosen != "system" {
		modelCode = chosen
	}
	if modelCode == "" || modelCode == input.ModelCode {
		return input
	}
	model, err := s.models.GetFullByCode(ctx, modelCode)
	if err != nil || model == nil || !model.IsEnabled || model.Category != "chat" || model.Code == "multi_collab_chat" {
		return input
	}
	if main, err := s.ResolveInputModel(ctx, &input); err == nil && (main.Category == "multi_collab" || main.Code == "multi_collab_chat") {
		return input
	}
	hash := compressionHash(older)
	summary := s.loadCompressionSummary(ctx, userID, input.ConversationID, hash)
	if summary == "" {
		var transcript strings.Builder
		for _, message := range older {
			transcript.WriteString(message.Role)
			transcript.WriteString("：")
			transcript.WriteString(message.Content)
			transcript.WriteString("\n")
		}
		result, err := s.Completion(ctx, userID, CompletionInput{
			ModelCode:    model.Code,
			Messages:     []runtime.ChatMessage{{Role: "user", Content: "请把下面的历史对话压缩成一段简洁中文摘要，保留用户目标、关键事实、决定和未完成事项。不要寒暄。\n\n" + transcript.String()}},
			Params:       map[string]interface{}{"deep_think": false, "auto_continue": false},
			Ephemeral:    true,
			BillingLabel: "上下文压缩",
		})
		if err != nil || result == nil || strings.TrimSpace(result.Content) == "" {
			log.Printf("chat compression skipped: %v", err)
			return input
		}
		summary = strings.TrimSpace(result.Content)
		s.saveCompressionSummary(ctx, userID, input.ConversationID, hash, summary)
	}
	messages := append([]runtime.ChatMessage{}, system...)
	messages = append(messages, runtime.ChatMessage{Role: "system", Content: "以下是更早对话的摘要，供继续对话时参考：\n" + summary})
	input.Messages = append(messages, recent...)
	return input
}
