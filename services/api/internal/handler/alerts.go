package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

func (h *Handler) AdminListAlertChannels(c *gin.Context) {
	items, err := h.admin.ListAlertChannels(c.Request.Context())
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, items)
}

func (h *Handler) AdminCreateAlertChannel(c *gin.Context) {
	var input service.AlertChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "告警通道参数无效")
		return
	}
	item, err := h.admin.CreateAlertChannel(c.Request.Context(), input)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.Created(c, item)
}

func (h *Handler) AdminUpdateAlertChannel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "告警通道 ID 无效")
		return
	}
	var input service.AlertChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "告警通道参数无效")
		return
	}
	item, err := h.admin.UpdateAlertChannel(c.Request.Context(), id, input)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, item)
}

func (h *Handler) AdminDeleteAlertChannel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "告警通道 ID 无效")
		return
	}
	if err := h.admin.DeleteAlertChannel(c.Request.Context(), id); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, nil)
}

func (h *Handler) AdminTestAlertChannel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "告警通道 ID 无效")
		return
	}
	if err := h.admin.TestAlertChannel(c.Request.Context(), id); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, gin.H{"sent": true})
}

func (h *Handler) AdminListUpstreamEvents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, err := h.admin.ListUpstreamEvents(c.Request.Context(), limit)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, items)
}
