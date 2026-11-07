package handler

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
	"github.com/starai/modelcontract"
)

func (h *Handler) AdminListAgentModels(c *gin.Context) {
	items, err := h.agents.ListAgentModelBindings(c.Request.Context())
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, map[string]interface{}{"items": items, "capability_labels": modelcontract.Labels})
}

func (h *Handler) AdminAgentModelCandidates(c *gin.Context) {
	items, err := h.agents.ListAgentModelBindings(c.Request.Context())
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	for _, item := range items {
		if item.Code != c.Param("code") {
			continue
		}
		for _, slot := range item.Slots {
			if slot.ID == c.Param("slot") {
				util.OK(c, map[string]interface{}{"items": slot.Candidates, "required_capabilities": slot.Required})
				return
			}
		}
	}
	util.BadRequest(c, "模型槽位不存在")
}

func (h *Handler) AdminUpdateAgentModel(c *gin.Context) {
	var req service.AgentModelSlotUpdate
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		util.BadRequest(c, "参数错误，仅允许提交模型编码和原模型编码")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		util.BadRequest(c, "参数错误")
		return
	}
	detail, err := h.agents.UpdateAgentModelSlot(c.Request.Context(), c.Param("code"), c.Param("slot"), req)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "update_agent_model", "workflow", c.Param("code"), detail)
	util.OK(c, detail)
}
