package handler

import (
	"net/http"
	"strconv"
	"strings"

	"errors"
	"github.com/gin-gonic/gin"

	"github.com/starai/api/internal/billing"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

func (h *Handler) DigitalHumanPrices(c *gin.Context) {
	voice, video := h.chat.DigitalHumanPrices(c.Request.Context())
	util.OK(c, gin.H{"voice_price_per_minute": voice, "video_price_per_minute": video})
}

func (h *Handler) ListDigitalHumanRoles(c *gin.Context) {
	items, err := h.chat.ListDigitalHumanRoles(c.Request.Context(), c.GetInt64("user_id"))
	if err != nil {
		h.writeDigitalHumanError(c, err)
		return
	}
	util.OK(c, items)
}

func (h *Handler) GetDigitalHumanRole(c *gin.Context) {
	item, err := h.chat.GetDigitalHumanRole(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"))
	if err != nil {
		h.writeDigitalHumanError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *Handler) SaveDigitalHumanRole(c *gin.Context) {
	var req service.DigitalHumanInput
	if c.ShouldBindJSON(&req) != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	item, err := h.chat.SaveDigitalHumanRole(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"), req)
	if err != nil {
		h.writeDigitalHumanError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *Handler) DeleteDigitalHumanRole(c *gin.Context) {
	if err := h.chat.DeleteDigitalHumanRole(c.Request.Context(), c.GetInt64("user_id"), c.Param("id")); err != nil {
		h.writeDigitalHumanError(c, err)
		return
	}
	util.OK(c, gin.H{"ok": true})
}

func (h *Handler) AddDigitalHumanKnowledge(c *gin.Context) {
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if c.ShouldBindJSON(&req) != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	item, err := h.chat.AddDigitalHumanKnowledge(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"), req.Title, req.Content)
	if err != nil {
		h.writeDigitalHumanError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *Handler) DeleteDigitalHumanKnowledge(c *gin.Context) {
	docID, _ := strconv.ParseInt(c.Param("docId"), 10, 64)
	if err := h.chat.DeleteDigitalHumanKnowledge(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"), docID); err != nil {
		h.writeDigitalHumanError(c, err)
		return
	}
	util.OK(c, gin.H{"ok": true})
}

func (h *Handler) StartDigitalHumanSession(c *gin.Context) {
	var req struct {
		Mode string `json:"mode"`
	}
	if c.ShouldBindJSON(&req) != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	item, err := h.chat.StartDigitalHumanSession(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"), req.Mode, scheme+"://"+c.Request.Host)
	if err != nil {
		h.writeDigitalHumanError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *Handler) SendDigitalHumanText(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if c.ShouldBindJSON(&req) != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	userMessage, assistant, err := h.chat.SendDigitalHumanText(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"), req.Content)
	if err != nil {
		h.writeDigitalHumanError(c, err)
		return
	}
	util.OK(c, gin.H{"user": userMessage, "assistant": assistant})
}

func (h *Handler) EndDigitalHumanSession(c *gin.Context) {
	item, err := h.chat.EndDigitalHumanSession(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"))
	if err != nil {
		h.writeDigitalHumanError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *Handler) SearchDigitalHumanKnowledge(c *gin.Context) {
	var req struct {
		LiveID     string `json:"live_id"`
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if c.ShouldBindJSON(&req) != nil {
		util.BadRequest(c, "参数错误")
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	hits, err := h.chat.SearchDigitalHumanKnowledge(c.Request.Context(), token, req.LiveID, req.Query, req.MaxResults)
	if err != nil {
		if msg, ok := service.AsDigitalHumanError(err); ok && msg == "unauthorized" {
			util.Unauthorized(c, "知识库凭证无效")
			return
		}
		util.InternalError(c, "知识库检索失败")
		return
	}
	if hits == nil {
		hits = []service.KnowledgeHit{}
	}
	c.JSON(http.StatusOK, gin.H{"knowledge": hits})
}

func (h *Handler) writeDigitalHumanError(c *gin.Context, err error) {
	if msg, ok := service.AsDigitalHumanError(err); ok {
		util.BadRequest(c, msg)
		return
	}
	if errors.Is(err, billing.ErrInsufficientBalance) {
		util.Fail(c, http.StatusPaymentRequired, 402, billing.InsufficientBalanceMsg)
		return
	}
	util.InternalError(c, "数字人服务暂时不可用")
}
