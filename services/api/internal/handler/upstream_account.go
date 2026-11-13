package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

func (h *Handler) AdminListUpstreamAccounts(c *gin.Context) {
	items, err := h.models.ListUpstreamAccounts(c.Request.Context(), c.Query("provider"))
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, items)
}

func (h *Handler) AdminCreateUpstreamAccount(c *gin.Context) {
	var input service.UpstreamAccountInput
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "账号参数无效")
		return
	}
	item, err := h.models.CreateUpstreamAccount(c.Request.Context(), input)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "create_upstream_account", "upstream_account", strconv.FormatInt(item.ID, 10), map[string]interface{}{"provider": item.Provider})
	util.Created(c, item)
}

func (h *Handler) AdminUpdateUpstreamAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "账号 ID 无效")
		return
	}
	var input service.UpstreamAccountInput
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "账号参数无效")
		return
	}
	item, err := h.models.UpdateUpstreamAccount(c.Request.Context(), id, input)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "update_upstream_account", "upstream_account", strconv.FormatInt(id, 10), nil)
	util.OK(c, item)
}

func (h *Handler) AdminDeleteUpstreamAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "账号 ID 无效")
		return
	}
	if err := h.models.DeleteUpstreamAccount(c.Request.Context(), id); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "delete_upstream_account", "upstream_account", strconv.FormatInt(id, 10), nil)
	util.OK(c, nil)
}

func (h *Handler) AdminTestUpstreamAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "账号 ID 无效")
		return
	}
	result, err := h.models.TestUpstreamAccount(c.Request.Context(), id)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, result)
}

// AdminMigrateUpstreamAccount 把源账号下的线路一键迁移到目标账号（Key + 分组一起换）。
func (h *Handler) AdminMigrateUpstreamAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "账号 ID 无效")
		return
	}
	var input struct {
		ToID int64 `json:"to_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.ToID <= 0 {
		util.BadRequest(c, "缺少目标账号")
		return
	}
	affected, err := h.models.MigrateUpstreamAccount(c.Request.Context(), id, input.ToID)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "migrate_upstream_account", "upstream_account", strconv.FormatInt(id, 10), map[string]interface{}{"to_id": input.ToID, "routes": affected})
	util.OK(c, gin.H{"affected": affected})
}

func (h *Handler) AdminListUpstreamLoginStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	items, err := h.models.ListUpstreamLoginAccountStats(c.Request.Context(), days)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, items)
}

func (h *Handler) AdminListUpstreamAccountRoutes(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "账号 ID 无效")
		return
	}
	items, listErr := h.models.ListAccountRoutes(c.Request.Context(), id)
	if listErr != nil {
		util.InternalError(c, listErr.Error())
		return
	}
	util.OK(c, items)
}

// AdminUpstreamGroupCapabilities 返回某上游各分组支持的模型清单（分组能力矩阵）。
func (h *Handler) AdminUpstreamGroupCapabilities(c *gin.Context) {
	provider := strings.TrimSpace(c.Query("provider"))
	items, err := h.models.ListUpstreamGroupCapabilities(c.Request.Context(), provider)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, items)
}

// AdminAccountCompatibility 判断某上游下各账号能否服务指定上游模型。
func (h *Handler) AdminAccountCompatibility(c *gin.Context) {
	items, err := h.models.CheckAccountCompatibility(c.Request.Context(), strings.TrimSpace(c.Query("provider")), strings.TrimSpace(c.Query("model")))
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, items)
}

// AdminBulkImportAccounts 批量导入上游账号（一行：名称, Key, 分组, 登录账号）。
func (h *Handler) AdminBulkImportAccounts(c *gin.Context) {
	var input struct {
		Provider string `json:"provider"`
		Content  string `json:"content"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Provider) == "" || strings.TrimSpace(input.Content) == "" {
		util.BadRequest(c, "缺少上游或导入内容")
		return
	}
	results, err := h.models.BulkImportAccounts(c.Request.Context(), strings.TrimSpace(input.Provider), input.Content)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, gin.H{"items": results})
}
