package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

// 电商图片工厂：反推提示词（免费+频控）、items 级重跑、ZIP 导出。

var (
	ecomReverseMu    sync.Mutex
	ecomReverseStats = map[int64][]time.Time{}
)

func ecomReverseAllowed(userID int64) bool {
	ecomReverseMu.Lock()
	defer ecomReverseMu.Unlock()
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	recent := ecomReverseStats[userID][:0]
	for _, ts := range ecomReverseStats[userID] {
		if ts.After(dayStart) {
			recent = append(recent, ts)
		}
	}
	if len(recent) >= 20 {
		ecomReverseStats[userID] = recent
		return false
	}
	recent = append(recent, now)
	ecomReverseStats[userID] = recent
	return true
}

func (h *Handler) EcomFactoryReverse(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req struct {
		ImageURL  string `json:"image_url"`
		ModelCode string `json:"model_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ImageURL == "" {
		util.BadRequest(c, "请上传要反推的图片")
		return
	}
	if !ecomReverseAllowed(userID) {
		util.BadRequest(c, "反推次数已达今日上限（20 次），请明天再试")
		return
	}
	if h.chat == nil || h.agents == nil {
		util.InternalError(c, "反推服务未配置")
		return
	}
	modelCode, err := h.agents.ResolveEcomReverseModel(c.Request.Context(), req.ModelCode)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	prompt, err := h.chat.EcomFactoryReversePrompt(c.Request.Context(), modelCode, req.ImageURL)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, gin.H{"prompt": prompt})
}

func (h *Handler) EcomFactoryItemAction(c *gin.Context) {
	var req service.EcomItemActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "请求参数无效")
		return
	}
	if err := h.agents.EcomFactoryItemAction(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"), req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, nil)
}

func (h *Handler) EcomFactoryExport(c *gin.Context) {
	buf, zipName, err := h.agents.EcomFactoryExportZip(c.Request.Context(), c.GetInt64("user_id"), c.Param("id"))
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+zipName+`"`)
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}
