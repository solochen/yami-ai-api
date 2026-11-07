package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starai/api/internal/runtime"
	"github.com/starai/api/internal/util"
)

// PromptAgentMode 是「提示词对话智能体」的 runtime_config.agent_mode。
// 这一类智能体只有一份系统提示词 + 多轮对话，不跑工具循环，也不需要 worker。
const PromptAgentMode = "prompt_chat"

// PromptAgentConfig 由 workflow_definitions 一行 + Go 侧提示词包合成。
// 展示文案（开场白/快捷问题/占位符/免责声明）以 DB 为准，提示词正文以 Go 为准，
// 这样文案可以在后台改库热更，行为逻辑仍然可测试、可版本化。
type PromptAgentConfig struct {
	Code          string
	Name          string
	Description   string
	ChatModelCode string
	Greeting      string
	QuickPrompts  []string
	Placeholder   string
	Disclaimer    string
	MaxInputChars int
	MaxTokens     int
	Temperature   float64
	HistoryLimit  int
	SystemPrompt  string
}

// PromptAgentTurnResult 一轮对话的落库结果，随 SSE done 事件返回。
type PromptAgentTurnResult struct {
	ConversationID string
	UserMessageID  int64
	MessageID      int64
	Content        string
	PromptTokens   int
	OutputTokens   int
	Cost           float64
}

// PromptAgentConversationMessage 是对外暴露的消息结构（带 id，供编辑/重新生成使用）。
type PromptAgentConversationMessage struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type PromptAgentConversation struct {
	PublicID     string                           `json:"public_id"`
	Title        string                           `json:"title"`
	AgentCode    string                           `json:"agent_code"`
	Messages     []PromptAgentConversationMessage `json:"messages"`
	CreatedAt    string                           `json:"created_at"`
	UpdatedAt    string                           `json:"updated_at"`
	MessageCount int                              `json:"message_count"`
}

type PromptAgentConversationSummary struct {
	PublicID     string `json:"public_id"`
	Title        string `json:"title"`
	UpdatedAt    string `json:"updated_at"`
	MessageCount int    `json:"message_count"`
}

type PromptAgentService struct {
	db   *pgxpool.Pool
	chat *ChatService
}

func NewPromptAgentService(db *pgxpool.Pool, chat *ChatService) *PromptAgentService {
	return &PromptAgentService{db: db, chat: chat}
}

// LoadConfig 读取并校验一个提示词智能体的运行配置。
func (s *PromptAgentService) LoadConfig(ctx context.Context, code string) (*PromptAgentConfig, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("智能体编码不能为空")
	}
	var name string
	var description *string
	var runtimeCfg []byte
	err := s.db.QueryRow(ctx, `
		SELECT name, description, runtime_config
		FROM workflow_definitions
		WHERE code=$1 AND is_enabled=true`, code).Scan(&name, &description, &runtimeCfg)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("该智能体不存在或已停用")
	}
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if len(runtimeCfg) > 0 {
		if err := json.Unmarshal(runtimeCfg, &raw); err != nil {
			return nil, fmt.Errorf("智能体配置解析失败: %w", err)
		}
	}
	if stringValue(raw["agent_mode"]) != PromptAgentMode {
		return nil, errors.New("该智能体不支持对话模式")
	}
	pack, hasPack := PromptPackByCode(code)
	if !hasPack && stringValue(raw["system_prompt"]) == "" {
		return nil, errors.New("该智能体的提示词尚未配置")
	}

	cfg := &PromptAgentConfig{
		Code:          code,
		Name:          name,
		ChatModelCode: stringValue(raw["chat_model_code"]),
		Greeting:      stringValue(raw["greeting"]),
		Placeholder:   stringValue(raw["input_placeholder"]),
		Disclaimer:    stringValue(raw["disclaimer"]),
		HistoryLimit:  intValue(raw["history_limit"], 40, 2, 80),
		MaxInputChars: intValue(raw["max_input_chars"], 4000, 100, 20000),
		MaxTokens:     intValue(raw["max_tokens"], 3000, 256, 16000),
		Temperature:   floatValueOrDefault(raw["temperature"], 0.7),
		SystemPrompt:  strings.TrimSpace(stringValue(raw["system_prompt"])),
	}
	if description != nil {
		cfg.Description = strings.TrimSpace(*description)
	}
	cfg.QuickPrompts = stringSlice(raw["quick_prompts"])
	if hasPack {
		if cfg.SystemPrompt == "" {
			cfg.SystemPrompt = pack.SystemPrompt
		}
		if cfg.ChatModelCode == "" {
			cfg.ChatModelCode = pack.DefaultChatModelCode
		}
	}
	if cfg.ChatModelCode == "" {
		return nil, errors.New("智能体尚未配置对话模型")
	}
	if cfg.SystemPrompt == "" {
		return nil, errors.New("智能体提示词为空")
	}
	if err := (&AgentService{db: s.db}).validateModelOverrides(ctx, WorkflowDTO{Code: code, RuntimeConfig: raw}, map[string]interface{}{"chat_model_code": cfg.ChatModelCode}); err != nil {
		return nil, err
	}
	cfg.SystemPrompt = composePromptAgentSystemPrompt(cfg.SystemPrompt)
	return cfg, nil
}

// composePromptAgentSystemPrompt 统一追加输出纪律；已被管理员写入的不重复追加。
func composePromptAgentSystemPrompt(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || strings.Contains(prompt, "## 通用输出纪律") {
		return prompt
	}
	return prompt + "\n\n" + promptAgentCommonRules
}

// StreamTurn 处理一轮普通发送：可新建会话，也可续写已有会话。
func (s *PromptAgentService) StreamTurn(ctx context.Context, userID int64, code, convPublicID, userMessage string, sink func(string, interface{})) (*PromptAgentTurnResult, error) {
	cfg, err := s.LoadConfig(ctx, code)
	if err != nil {
		return nil, err
	}
	message := strings.TrimSpace(userMessage)
	if err := validatePromptAgentInput(message, cfg.MaxInputChars); err != nil {
		return nil, err
	}

	convPublicID, created, history, err := s.prepareConversation(ctx, userID, cfg, convPublicID, message)
	if err != nil {
		return nil, err
	}

	result, err := s.runCompletion(ctx, userID, cfg, history, message, sink)
	if err != nil {
		if created {
			s.discardEmptyConversation(ctx, userID, convPublicID)
		}
		return nil, err
	}

	userMessageID, err := s.appendMessage(ctx, userID, convPublicID, "user", message)
	if err != nil {
		return nil, err
	}
	messageID, err := s.appendMessage(ctx, userID, convPublicID, "assistant", result.Content)
	if err != nil {
		return nil, err
	}
	result.ConversationID = convPublicID
	result.UserMessageID = userMessageID
	result.MessageID = messageID
	return result, nil
}

// RerunTurn 从指定用户消息重新生成：可携带新内容（编辑并重跑），
// 该消息之后的所有对话会被截断，保证上下文与界面一致。
func (s *PromptAgentService) RerunTurn(ctx context.Context, userID int64, code, convPublicID string, messageID int64, newContent string, sink func(string, interface{})) (*PromptAgentTurnResult, error) {
	cfg, err := s.LoadConfig(ctx, code)
	if err != nil {
		return nil, err
	}
	convID, err := s.conversationID(ctx, userID, convPublicID, code)
	if err != nil {
		return nil, err
	}
	var role, content string
	err = s.db.QueryRow(ctx, `
		SELECT role, content FROM conversation_messages
		WHERE id=$1 AND conversation_id=$2`, messageID, convID).Scan(&role, &content)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("消息不存在")
	}
	if err != nil {
		return nil, err
	}
	if role != "user" {
		return nil, errors.New("只能从用户消息重新生成")
	}
	if strings.TrimSpace(newContent) != "" {
		content = newContent
	}
	content = strings.TrimSpace(content)
	if err := validatePromptAgentInput(content, cfg.MaxInputChars); err != nil {
		return nil, err
	}

	history, err := s.historyBefore(ctx, convID, messageID, cfg.HistoryLimit)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(ctx, `
		DELETE FROM conversation_messages
		WHERE conversation_id=$1 AND id>$2 AND role IN ('user','assistant')`, convID, messageID); err != nil {
		return nil, err
	}
	if newContent != "" {
		if _, err := s.db.Exec(ctx, `
			UPDATE conversation_messages SET content=$3
			WHERE id=$1 AND conversation_id=$2`, messageID, convID, content); err != nil {
			return nil, err
		}
	}

	result, err := s.runCompletion(ctx, userID, cfg, history, content, sink)
	if err != nil {
		return nil, err
	}
	assistantID, err := s.appendMessage(ctx, userID, convPublicID, "assistant", result.Content)
	if err != nil {
		return nil, err
	}
	result.ConversationID = convPublicID
	result.UserMessageID = messageID
	result.MessageID = assistantID
	return result, nil
}

// runCompletion 拼 system + 历史 + 本轮输入，流式输出并结算。
func (s *PromptAgentService) runCompletion(ctx context.Context, userID int64, cfg *PromptAgentConfig, history []map[string]interface{}, userMessage string, sink func(string, interface{})) (*PromptAgentTurnResult, error) {
	messages := make([]map[string]interface{}, 0, len(history)+2)
	messages = append(messages, map[string]interface{}{"role": "system", "content": cfg.SystemPrompt})
	messages = append(messages, history...)
	messages = append(messages, map[string]interface{}{"role": "user", "content": userMessage})

	params := map[string]interface{}{}
	if cfg.MaxTokens > 0 {
		params["max_tokens"] = cfg.MaxTokens
	}
	if cfg.Temperature >= 0 {
		params["temperature"] = cfg.Temperature
	}

	sink("meta", map[string]interface{}{
		"agent_code": cfg.Code,
		"model":      cfg.ChatModelCode,
	})

	requestID, ch, estimated, err := s.chat.CompletionStreamRaw(ctx, userID, cfg.ChatModelCode, messages, params)
	if err != nil {
		log.Printf("prompt agent %s stream start failed: %v", cfg.Code, err)
		return nil, friendlyPromptAgentError(err)
	}

	var content, reasoning strings.Builder
	var usage *runtime.ChatUsage
	var streamErr error
	for chunk := range ch {
		if chunk.Error != nil {
			streamErr = chunk.Error
			break
		}
		if chunk.Content != "" {
			content.WriteString(chunk.Content)
			sink("delta", map[string]interface{}{"text": chunk.Content})
		}
		if chunk.ReasoningContent != "" {
			reasoning.WriteString(chunk.ReasoningContent)
			// 推理内容单独下发，前端折叠展示，避免等待首 token 时页面毫无反应。
			sink("thinking", map[string]interface{}{"text": chunk.ReasoningContent})
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}
	if err := ctx.Err(); err != nil {
		_ = s.chat.UnfreezeStream(context.WithoutCancel(ctx), userID, requestID, estimated)
		return nil, err
	}
	if streamErr != nil {
		_ = s.chat.UnfreezeStream(context.WithoutCancel(ctx), userID, requestID, estimated)
		log.Printf("prompt agent %s stream failed: %v", cfg.Code, streamErr)
		return nil, friendlyPromptAgentError(streamErr)
	}
	if strings.TrimSpace(content.String()) == "" {
		_ = s.chat.UnfreezeStream(context.WithoutCancel(ctx), userID, requestID, estimated)
		return nil, errors.New("模型没有返回内容，请重试或换个说法")
	}

	finalizeInput := CompletionInput{
		ModelCode:    cfg.ChatModelCode,
		Params:       params,
		Stream:       true,
		Ephemeral:    true,
		BillingLabel: cfg.Name,
	}
	if _, err := s.chat.FinalizeStream(ctx, userID, requestID, finalizeInput, content.String(), reasoning.String(), usage, estimated); err != nil {
		return nil, err
	}

	result := &PromptAgentTurnResult{Content: content.String()}
	if usage != nil {
		result.PromptTokens = usage.PromptTokens
		result.OutputTokens = usage.CompletionTokens
	}
	return result, nil
}

// prepareConversation 校验会话归属；为空时按首条消息新建。
func (s *PromptAgentService) prepareConversation(ctx context.Context, userID int64, cfg *PromptAgentConfig, convPublicID, firstMessage string) (string, bool, []map[string]interface{}, error) {
	if strings.TrimSpace(convPublicID) != "" {
		convID, err := s.conversationID(ctx, userID, convPublicID, cfg.Code)
		if err != nil {
			return "", false, nil, err
		}
		history, err := s.historyBefore(ctx, convID, 0, cfg.HistoryLimit)
		if err != nil {
			return "", false, nil, err
		}
		return convPublicID, false, history, nil
	}

	publicID := util.NewPublicID("conv")
	title := truncate(firstMessage, 24)
	var modelID *int64
	var resolved int64
	if err := s.db.QueryRow(ctx, `SELECT id FROM models WHERE code=$1`, cfg.ChatModelCode).Scan(&resolved); err == nil {
		modelID = &resolved
	}
	if _, err := s.db.Exec(ctx, `
		INSERT INTO conversations (public_id, user_id, model_id, title, agent_code)
		VALUES ($1,$2,$3,$4,$5)`, publicID, userID, modelID, title, cfg.Code); err != nil {
		return "", false, nil, err
	}
	return publicID, true, nil, nil
}

// discardEmptyConversation 清理失败/中断时留下的空会话，避免历史列表积累垃圾。
func (s *PromptAgentService) discardEmptyConversation(ctx context.Context, userID int64, convPublicID string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	_, _ = s.db.Exec(cleanupCtx, `
		DELETE FROM conversations c
		WHERE c.public_id=$1 AND c.user_id=$2
		  AND NOT EXISTS (SELECT 1 FROM conversation_messages m WHERE m.conversation_id=c.id)`, convPublicID, userID)
}

func (s *PromptAgentService) conversationID(ctx context.Context, userID int64, convPublicID, agentCode string) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `
		SELECT id FROM conversations
		WHERE public_id=$1 AND user_id=$2 AND agent_code=$3`, convPublicID, userID, agentCode).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, errors.New("会话不存在")
	}
	return id, err
}

// historyBefore 取指定消息之前的最近 N 条 user/assistant 消息；beforeID=0 表示取最新。
func (s *PromptAgentService) historyBefore(ctx context.Context, convID int64, beforeID int64, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 40
	}
	var rows pgx.Rows
	var err error
	if beforeID > 0 {
		rows, err = s.db.Query(ctx, `
			SELECT role, content FROM (
				SELECT id, role, content FROM conversation_messages
				WHERE conversation_id=$1 AND role IN ('user','assistant') AND id<$2
				ORDER BY id DESC LIMIT $3
			) recent ORDER BY id ASC`, convID, beforeID, limit)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT role, content FROM (
				SELECT id, role, content FROM conversation_messages
				WHERE conversation_id=$1 AND role IN ('user','assistant')
				ORDER BY id DESC LIMIT $2
			) recent ORDER BY id ASC`, convID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := make([]map[string]interface{}, 0, limit)
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err != nil {
			return nil, err
		}
		history = append(history, map[string]interface{}{"role": role, "content": content})
	}
	return history, rows.Err()
}

// appendMessage 落库一条消息并刷新会话时间，保证历史列表按最近活动排序。
func (s *PromptAgentService) appendMessage(ctx context.Context, userID int64, convPublicID, role, content string) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `
		WITH target AS (
			SELECT id FROM conversations WHERE public_id=$1 AND user_id=$2
		)
		INSERT INTO conversation_messages (conversation_id, role, content)
		SELECT id, $3, $4 FROM target
		RETURNING id`, convPublicID, userID, role, content).Scan(&id)
	if err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE conversations SET updated_at=now() WHERE public_id=$1 AND user_id=$2`, convPublicID, userID); err != nil {
		return id, err
	}
	return id, nil
}

func (s *PromptAgentService) ListConversations(ctx context.Context, userID int64, agentCode string, limit int) ([]PromptAgentConversationSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT c.public_id, COALESCE(c.title,''), c.updated_at,
		       (SELECT COUNT(*) FROM conversation_messages m WHERE m.conversation_id=c.id AND m.role IN ('user','assistant'))
		FROM conversations c
		WHERE c.user_id=$1 AND c.agent_code=$2
		  AND EXISTS (SELECT 1 FROM conversation_messages m WHERE m.conversation_id=c.id)
		ORDER BY c.updated_at DESC
		LIMIT $3`, userID, agentCode, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PromptAgentConversationSummary, 0, limit)
	for rows.Next() {
		var item PromptAgentConversationSummary
		var updated time.Time
		if err := rows.Scan(&item.PublicID, &item.Title, &updated, &item.MessageCount); err != nil {
			return nil, err
		}
		item.UpdatedAt = updated.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PromptAgentService) GetConversation(ctx context.Context, userID int64, agentCode, convPublicID string) (*PromptAgentConversation, error) {
	convID, err := s.conversationID(ctx, userID, convPublicID, agentCode)
	if err != nil {
		return nil, err
	}
	out := &PromptAgentConversation{PublicID: convPublicID, AgentCode: agentCode, Messages: []PromptAgentConversationMessage{}}
	var created, updated time.Time
	var title *string
	if err := s.db.QueryRow(ctx, `SELECT title, created_at, updated_at FROM conversations WHERE id=$1`, convID).
		Scan(&title, &created, &updated); err != nil {
		return nil, err
	}
	if title != nil {
		out.Title = *title
	}
	out.CreatedAt = created.Format(time.RFC3339)
	out.UpdatedAt = updated.Format(time.RFC3339)

	rows, err := s.db.Query(ctx, `
		SELECT id, role, content, created_at FROM conversation_messages
		WHERE conversation_id=$1 AND role IN ('user','assistant')
		ORDER BY id ASC`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var msg PromptAgentConversationMessage
		var createdAt time.Time
		if err := rows.Scan(&msg.ID, &msg.Role, &msg.Content, &createdAt); err != nil {
			return nil, err
		}
		msg.CreatedAt = createdAt.Format(time.RFC3339)
		out.Messages = append(out.Messages, msg)
	}
	out.MessageCount = len(out.Messages)
	return out, rows.Err()
}

func (s *PromptAgentService) RenameConversation(ctx context.Context, userID int64, agentCode, convPublicID, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("标题不能为空")
	}
	if len([]rune(title)) > 60 {
		title = string([]rune(title)[:60])
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE conversations SET title=$3
		WHERE public_id=$1 AND user_id=$2 AND agent_code=$4`, convPublicID, userID, title, agentCode)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("会话不存在")
	}
	return nil
}

func (s *PromptAgentService) DeleteConversation(ctx context.Context, userID int64, agentCode, convPublicID string) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM conversations
		WHERE public_id=$1 AND user_id=$2 AND agent_code=$3`, convPublicID, userID, agentCode)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("会话不存在")
	}
	return nil
}

func (s *PromptAgentService) DeleteMessage(ctx context.Context, userID int64, agentCode, convPublicID string, messageID int64) error {
	convID, err := s.conversationID(ctx, userID, convPublicID, agentCode)
	if err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx, `
		DELETE FROM conversation_messages WHERE id=$1 AND conversation_id=$2`, messageID, convID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("消息不存在")
	}
	return nil
}

// friendlyPromptAgentError 把上游传输层的英文错误换成用户能看懂的提示；
// 业务侧已经写好的中文错误（额度、限流、长度上限等）原样透出。
func friendlyPromptAgentError(err error) error {
	if err == nil {
		return nil
	}
	var balanceErr *BalanceError
	if errors.As(err, &balanceErr) {
		return err
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return errors.New("模型服务暂时不可用，请稍后重试")
	}
	for _, r := range message {
		if r > 127 {
			return err
		}
	}
	return errors.New("模型服务暂时不可用，请稍后重试")
}

func validatePromptAgentInput(message string, maxChars int) error {
	if strings.TrimSpace(message) == "" {
		return errors.New("消息不能为空")
	}
	if maxChars > 0 && len([]rune(message)) > maxChars {
		return fmt.Errorf("消息过长，请控制在 %d 字以内", maxChars)
	}
	return nil
}

func intValue(value interface{}, fallback, min, max int) int {
	n := fallback
	switch v := value.(type) {
	case float64:
		n = int(v)
	case int:
		n = v
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			n = int(parsed)
		}
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func floatValueOrDefault(value interface{}, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		if parsed, err := v.Float64(); err == nil {
			return parsed
		}
	}
	return fallback
}
