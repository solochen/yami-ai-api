package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/starai/api/internal/runtime"
)

const RenovationAgentCode = "renovation_budget_agent"

// RenovationRunConfig 来自 workflow_definitions.runtime_config，
// 可在后台直接改库调整，无需重启。
type RenovationRunConfig struct {
	ChatModelCode        string
	EmbeddingModelCode   string
	MaxRounds            int
	MaxToolCallsPerRound int
}

func defaultRenovationRunConfig() RenovationRunConfig {
	return RenovationRunConfig{
		ChatModelCode:        "chat_demo_v1",
		EmbeddingModelCode:   "",
		MaxRounds:            8,
		MaxToolCallsPerRound: 6,
	}
}

// LoadRenovationRunConfig 读取装修预算助理的运行配置。
func (s *RenovationService) LoadRenovationRunConfig(ctx context.Context) (RenovationRunConfig, error) {
	cfg := defaultRenovationRunConfig()
	var runtimeCfg []byte
	err := s.db.QueryRow(ctx,
		`SELECT runtime_config FROM workflow_definitions WHERE code=$1 AND is_enabled=true`, RenovationAgentCode).Scan(&runtimeCfg)
	if err != nil {
		return cfg, fmt.Errorf("装修预算助理未配置或未启用: %w", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(runtimeCfg, &raw); err != nil {
		return cfg, fmt.Errorf("runtime_config 解析失败: %w", err)
	}
	if v, ok := raw["chat_model_code"].(string); ok && v != "" {
		cfg.ChatModelCode = v
	}
	if v, ok := raw["embedding_model_code"].(string); ok {
		cfg.EmbeddingModelCode = v
	}
	if v, ok := raw["max_rounds"].(float64); ok && v > 0 && v <= 20 {
		cfg.MaxRounds = int(v)
	}
	if v, ok := raw["max_tool_calls_per_round"].(float64); ok && v > 0 && v <= 12 {
		cfg.MaxToolCallsPerRound = int(v)
	}
	return cfg, nil
}

const renovationSystemPrompt = `你是"装修预算助理"，一位专业的家装预算与软装顾问，用中文服务用户。

工作纪律：
1. 凡涉及材料价格、面积、用量、费用的数字，必须先调用工具查询或计算，禁止凭记忆报价、禁止心算。
2. 用户提供的尺寸不完整或含糊时（如"大概五米多"），先礼貌追问确认，不要自行假设。
3. 回答装修风格、搭配建议、材料选购类问题前，先调用 search_knowledge 检索知识库，结合检索内容作答。
4. 当本轮所有询价与算量完成后，必须调用 generate_report 生成 Markdown 预算报告交付用户，报告用表格列明细。
5. 只讨论家装装修相关话题；无关请求（写代码、闲聊等）礼貌拒绝并引导回装修主题。
6. 工具返回 error 字段时，读取错误信息调整参数重试，或向用户澄清。`

// RenovationRunResult 一次完整 Agent 运行的结果。
type RenovationRunResult struct {
	ConversationID string
	Rounds         int
	Content        string
	Report         *RenovationReport
}

type RenovationReport struct {
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// RunAgentStream 执行装修预算助理的完整工具调用循环。
// sink 用于向 SSE 层发事件：status / tool_result / delta / round。
func (s *RenovationService) RunAgentStream(ctx context.Context, userID int64, convPublicID, userMessage string, cfg RenovationRunConfig, sink func(event string, payload interface{})) (*RenovationRunResult, error) {
	if strings.TrimSpace(userMessage) == "" {
		return nil, errors.New("消息不能为空")
	}

	// 会话：复用 conversations 体系，标题固定前缀用于 scope 过滤
	if convPublicID == "" {
		conv, err := s.chat.CreateConversation(ctx, userID, cfg.ChatModelCode, "装修预算助理")
		if err != nil {
			return nil, err
		}
		convPublicID = conv.PublicID
	}
	sink("meta", map[string]interface{}{"conversation_id": convPublicID, "chat_model": cfg.ChatModelCode, "max_rounds": cfg.MaxRounds})

	// 历史恢复：仅取 user/assistant 显示消息（最终答案中已包含算量结论）
	history, _ := s.chat.GetConversation(ctx, userID, convPublicID, 40)

	messages := []map[string]interface{}{
		{"role": "system", "content": renovationSystemPrompt},
	}
	if histRows, ok := history["messages"].([]map[string]string); ok {
		for _, row := range histRows {
			if row["role"] == "user" || row["role"] == "assistant" {
				messages = append(messages, map[string]interface{}{"role": row["role"], "content": row["content"]})
			}
		}
	}
	messages = append(messages, map[string]interface{}{"role": "user", "content": userMessage})

	params := map[string]interface{}{
		"tools":       RenovationToolSchemas(),
		"tool_choice": "auto",
	}

	result := &RenovationRunResult{ConversationID: convPublicID}
	for round := 1; round <= cfg.MaxRounds; round++ {
		result.Rounds = round
		sink("round", map[string]interface{}{"round": round, "max_rounds": cfg.MaxRounds})

		requestID, ch, estimated, callErr := s.chat.CompletionStreamRaw(ctx, userID, cfg.ChatModelCode, messages, params)
		if callErr != nil {
			return result, callErr
		}

		var content, reasoning strings.Builder
		var usage *runtime.ChatUsage
		var streamErr error
		pendingCalls := map[int]*streamToolCall{}
		var callOrder []int
		for chunk := range ch {
			if chunk.Error != nil {
				streamErr = chunk.Error
				break
			}
			if chunk.Content != "" {
				content.WriteString(chunk.Content)
				sink("delta", map[string]interface{}{"text": chunk.Content, "round": round})
			}
			reasoning.WriteString(chunk.ReasoningContent)
			if len(chunk.ToolCalls) > 0 {
				accumulateToolCalls(pendingCalls, &callOrder, chunk.ToolCalls)
			}
			if chunk.Usage != nil {
				usage = chunk.Usage
			}
		}
		toolCalls := materializeToolCalls(pendingCalls, callOrder)
		if streamErr != nil {
			_ = s.chat.UnfreezeStream(ctx, userID, requestID, estimated)
			return result, streamErr
		}
		if err := ctx.Err(); err != nil {
			_ = s.chat.UnfreezeStream(ctx, userID, requestID, estimated)
			return result, err
		}
		// 本轮结算（Ephemeral，不自动落库）
		finalizeInput := CompletionInput{ModelCode: cfg.ChatModelCode, Params: params, Stream: true, Ephemeral: true}
		if _, err := s.chat.FinalizeStream(ctx, userID, requestID, finalizeInput, content.String(), reasoning.String(), usage, estimated); err != nil {
			return result, fmt.Errorf("第 %d 轮结算失败: %w", round, err)
		}

		// 没有工具调用 → 任务完成
		if len(toolCalls) == 0 {
			result.Content = content.String()
			break
		}

		// 记录 assistant(tool_calls) 消息
		assistantMsg := map[string]interface{}{"role": "assistant", "tool_calls": toolCalls}
		if content.Len() > 0 {
			assistantMsg["content"] = content.String()
		} else {
			assistantMsg["content"] = nil
		}
		messages = append(messages, assistantMsg)

		if len(toolCalls) > cfg.MaxToolCallsPerRound {
			toolCalls = toolCalls[:cfg.MaxToolCallsPerRound]
		}
		for _, call := range toolCalls {
			name, arguments, callID := decodeToolCall(call)
			sink("status", map[string]interface{}{"message": fmt.Sprintf("正在调用工具 %s…", name), "round": round})
			toolResult, execErr := s.ExecuteRenovationTool(ctx, name, arguments, cfg.EmbeddingModelCode)
			ok := execErr == nil
			if execErr != nil {
				toolResult = toolErrorf("工具执行失败：%v", execErr)
			}
			if ok && name == "generate_report" {
				if report := captureReport(arguments); report != nil {
					result.Report = report
				}
			}
			var resultPayload interface{} = toolResult
			var argsPayload interface{} = arguments
			if json.Valid([]byte(toolResult)) {
				resultPayload = json.RawMessage(toolResult)
			}
			if json.Valid([]byte(arguments)) {
				argsPayload = json.RawMessage(arguments)
			}
			sink("tool_result", map[string]interface{}{
				"round": round, "name": name, "arguments": argsPayload, "result": resultPayload, "ok": ok,
			})
			messages = append(messages, map[string]interface{}{
				"role": "tool", "tool_call_id": callID, "content": toolResult,
			})
		}
	}

	// 轮次耗尽仍未收敛：强制不带工具收尾一次
	if result.Content == "" {
		messages = append(messages, map[string]interface{}{
			"role": "user", "content": "（系统提示：已达到工具调用轮次上限，请基于已获得的信息直接总结当前结论，不要再调用工具。）",
		})
		requestID, ch, estimated, callErr := s.chat.CompletionStreamRaw(ctx, userID, cfg.ChatModelCode, messages, nil)
		if callErr == nil {
			var content strings.Builder
			var usage *runtime.ChatUsage
			var streamErr error
			for chunk := range ch {
				if chunk.Error != nil {
					streamErr = chunk.Error
					break
				}
				if chunk.Content != "" {
					content.WriteString(chunk.Content)
					sink("delta", map[string]interface{}{"text": chunk.Content, "round": result.Rounds + 1})
				}
				if chunk.Usage != nil {
					usage = chunk.Usage
				}
			}
			if streamErr == nil {
				finalizeInput := CompletionInput{ModelCode: cfg.ChatModelCode, Stream: true, Ephemeral: true}
				_, _ = s.chat.FinalizeStream(ctx, userID, requestID, finalizeInput, content.String(), "", usage, estimated)
				result.Content = content.String()
			} else {
				_ = s.chat.UnfreezeStream(ctx, userID, requestID, estimated)
			}
		}
	}

	// 落库：user 行 + 最终 assistant 行
	if result.Content != "" {
		_ = s.chat.AppendConversationMessage(ctx, userID, convPublicID, "user", userMessage)
		_ = s.chat.AppendConversationMessage(ctx, userID, convPublicID, "assistant", result.Content)
	}

	return result, nil
}

func decodeToolCall(call map[string]interface{}) (name, arguments, callID string) {
	callID, _ = call["id"].(string)
	if fn, ok := call["function"].(map[string]interface{}); ok {
		name, _ = fn["name"].(string)
		arguments, _ = fn["arguments"].(string)
	}
	return
}

// streamToolCall 流式 tool_call 分片的按 index 聚合体。
type streamToolCall struct {
	id        string
	name      strings.Builder
	arguments strings.Builder
}

func accumulateToolCalls(pending map[int]*streamToolCall, order *[]int, calls []map[string]interface{}) {
	for _, call := range calls {
		idx := 0
		switch v := call["index"].(type) {
		case float64:
			idx = int(v)
		case int:
			idx = v
		}
		tc, exists := pending[idx]
		if !exists {
			tc = &streamToolCall{}
			pending[idx] = tc
			*order = append(*order, idx)
		}
		if id, ok := call["id"].(string); ok && id != "" {
			tc.id = id
		}
		if fn, ok := call["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				tc.name.WriteString(name)
			}
			if args, ok := fn["arguments"].(string); ok {
				tc.arguments.WriteString(args)
			}
		}
	}
}

func materializeToolCalls(pending map[int]*streamToolCall, order []int) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(order))
	for _, idx := range order {
		tc := pending[idx]
		out = append(out, map[string]interface{}{
			"id":   tc.id,
			"type": "function",
			"function": map[string]interface{}{
				"name":      tc.name.String(),
				"arguments": tc.arguments.String(),
			},
		})
	}
	return out
}

func captureReport(arguments string) *RenovationReport {
	var args struct {
		Title           string `json:"title"`
		MarkdownContent string `json:"markdown_content"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil || args.MarkdownContent == "" {
		return nil
	}
	return &RenovationReport{
		Title:    args.Title,
		Filename: fmt.Sprintf("装修预算报告_%s.md", time.Now().Format("20060102_150405")),
		Content:  args.MarkdownContent,
	}
}
