package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/runtime"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

type promptAgentChatRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
}

type promptAgentRerunRequest struct {
	Content string `json:"content"`
}

// PromptAgentChat 提示词智能体的一轮对话（SSE）。
// 事件协议：meta / delta / done / error
func (h *Handler) PromptAgentChat(c *gin.Context) {
	var req promptAgentChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		util.BadRequest(c, "消息不能为空")
		return
	}
	code := c.Param("code")
	userID := c.GetInt64("user_id")
	cfg, err := h.promptAgents.LoadConfig(c.Request.Context(), code)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	if cfg.MaxInputChars > 0 && len([]rune(strings.TrimSpace(req.Message))) > cfg.MaxInputChars {
		util.BadRequest(c, fmt.Sprintf("消息过长，请控制在 %d 字以内", cfg.MaxInputChars))
		return
	}
	h.streamPromptAgent(c, func(sink func(string, interface{})) (*service.PromptAgentTurnResult, error) {
		return h.promptAgents.StreamTurn(c.Request.Context(), userID, code, req.ConversationID, req.Message, sink)
	})
}

// RerunPromptAgentMessage 从某条用户消息重新生成；带 content 时是"编辑并重跑"。
func (h *Handler) RerunPromptAgentMessage(c *gin.Context) {
	messageID, err := strconv.ParseInt(c.Param("message_id"), 10, 64)
	if err != nil || messageID <= 0 {
		util.BadRequest(c, "消息参数错误")
		return
	}
	var req promptAgentRerunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	code := c.Param("code")
	userID := c.GetInt64("user_id")
	cfg, err := h.promptAgents.LoadConfig(c.Request.Context(), code)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	if cfg.MaxInputChars > 0 && len([]rune(strings.TrimSpace(req.Content))) > cfg.MaxInputChars {
		util.BadRequest(c, fmt.Sprintf("消息过长，请控制在 %d 字以内", cfg.MaxInputChars))
		return
	}
	h.streamPromptAgent(c, func(sink func(string, interface{})) (*service.PromptAgentTurnResult, error) {
		return h.promptAgents.RerunTurn(c.Request.Context(), userID, code, c.Param("id"), messageID, req.Content, sink)
	})
}

func (h *Handler) streamPromptAgent(c *gin.Context, run func(sink func(string, interface{})) (*service.PromptAgentTurnResult, error)) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// 部分上游会把整段回复攒完才下发，等待期可能长达数十秒。
	// 心跳事件让前端能显示耗时，也避免中间代理因空闲超时断开连接。
	var writeMu sync.Mutex
	sink := func(event string, payload interface{}) {
		writeMu.Lock()
		defer writeMu.Unlock()
		if ctx.Err() != nil {
			return
		}
		if _, err := c.Writer.Write([]byte(runtime.FormatSSE(event, payload))); err != nil {
			cancel()
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	started := time.Now()
	stopHeartbeat := make(chan struct{})
	var heartbeatOnce sync.Once
	stopPing := func() { heartbeatOnce.Do(func() { close(stopHeartbeat) }) }
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopHeartbeat:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				sink("ping", map[string]interface{}{"elapsed_seconds": int(time.Since(started).Seconds())})
			}
		}
	}()

	result, runErr := run(sink)
	stopPing()
	if runErr != nil {
		if errors.Is(runErr, context.Canceled) || ctx.Err() != nil {
			return
		}
		var balanceErr *service.BalanceError
		if errors.As(runErr, &balanceErr) {
			sink("error", map[string]interface{}{"code": "INSUFFICIENT_BALANCE", "message": util.SanitizeMessage(runErr.Error())})
			return
		}
		sink("error", map[string]interface{}{"code": "AGENT_RUN_FAILED", "message": util.SanitizeMessage(runErr.Error())})
		return
	}
	sink("done", map[string]interface{}{
		"conversation_id": result.ConversationID,
		"message_id":      result.MessageID,
		"user_message_id": result.UserMessageID,
		"content":         result.Content,
		"usage": map[string]interface{}{
			"prompt_tokens":     result.PromptTokens,
			"completion_tokens": result.OutputTokens,
		},
	})
}

func (h *Handler) ListPromptAgentConversations(c *gin.Context) {
	items, err := h.promptAgents.ListConversations(c.Request.Context(), c.GetInt64("user_id"), c.Param("code"), 50)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, map[string]interface{}{"items": items})
}

func (h *Handler) GetPromptAgentConversation(c *gin.Context) {
	item, err := h.promptAgents.GetConversation(c.Request.Context(), c.GetInt64("user_id"), c.Param("code"), c.Param("id"))
	if err != nil {
		util.NotFound(c, "会话不存在")
		return
	}
	util.OK(c, item)
}

func (h *Handler) RenamePromptAgentConversation(c *gin.Context) {
	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	if err := h.promptAgents.RenameConversation(c.Request.Context(), c.GetInt64("user_id"), c.Param("code"), c.Param("id"), req.Title); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, nil)
}

func (h *Handler) DeletePromptAgentConversation(c *gin.Context) {
	if err := h.promptAgents.DeleteConversation(c.Request.Context(), c.GetInt64("user_id"), c.Param("code"), c.Param("id")); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, nil)
}

func (h *Handler) DeletePromptAgentMessage(c *gin.Context) {
	messageID, err := strconv.ParseInt(c.Param("message_id"), 10, 64)
	if err != nil || messageID <= 0 {
		util.BadRequest(c, "消息参数错误")
		return
	}
	if err := h.promptAgents.DeleteMessage(c.Request.Context(), c.GetInt64("user_id"), c.Param("code"), c.Param("id"), messageID); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, nil)
}
