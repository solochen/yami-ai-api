package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/runtime"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

type renovationChatRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
}

// RenovationBudgetChat 装修预算助理 Agent 的 SSE 入口。
// 事件协议：meta / round / status / tool_result / delta / done / error
func (h *Handler) RenovationBudgetChat(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req renovationChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		util.BadRequest(c, "消息不能为空")
		return
	}
	cfg, err := h.renovation.LoadRenovationRunConfig(c.Request.Context())
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(200)
	flusher, _ := c.Writer.(http.Flusher)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	sink := func(event string, payload interface{}) {
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

	result, runErr := h.renovation.RunAgentStream(ctx, userID, req.ConversationID, req.Message, cfg, sink)
	if runErr != nil {
		var balanceErr *service.BalanceError
		if errors.As(runErr, &balanceErr) {
			sink("error", map[string]interface{}{"code": "INSUFFICIENT_BALANCE", "message": runErr.Error()})
			return
		}
		sink("error", map[string]interface{}{"code": "AGENT_RUN_FAILED", "message": runErr.Error()})
		return
	}
	done := map[string]interface{}{
		"conversation_id": result.ConversationID,
		"content":         result.Content,
		"rounds":          result.Rounds,
	}
	if result.Report != nil {
		done["report"] = result.Report
	}
	sink("done", done)
}
