package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

func (h *Handler) AdminListUpstreamSyncSources(c *gin.Context) {
	items, err := h.models.ListUpstreamSyncSources(c.Request.Context())
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, items)
}

func (h *Handler) AdminCreateUpstreamSyncSource(c *gin.Context) {
	var input service.UpstreamSyncSourceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "参数无效")
		return
	}
	item, err := h.models.CreateUpstreamSyncSource(c.Request.Context(), input)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "create_upstream_sync_source", "upstream_sync_source",
		strconv.FormatInt(item.ID, 10), map[string]interface{}{"provider": item.Provider})
	util.Created(c, item)
}

func (h *Handler) AdminUpdateUpstreamSyncSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "同步源 ID 无效")
		return
	}
	var input service.UpstreamSyncSourceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "参数无效")
		return
	}
	item, updateErr := h.models.UpdateUpstreamSyncSource(c.Request.Context(), id, input)
	if updateErr != nil {
		util.BadRequest(c, updateErr.Error())
		return
	}
	util.OK(c, item)
}

func (h *Handler) AdminDeleteUpstreamSyncSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "同步源 ID 无效")
		return
	}
	if delErr := h.models.DeleteUpstreamSyncSource(c.Request.Context(), id); delErr != nil {
		util.BadRequest(c, delErr.Error())
		return
	}
	util.OK(c, nil)
}

func (h *Handler) AdminPreviewUpstreamSync(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "同步源 ID 无效")
		return
	}
	var input struct {
		Rebuild bool `json:"rebuild"`
	}
	_ = c.ShouldBindJSON(&input)
	preview, previewErr := h.models.PreviewUpstreamSync(c.Request.Context(), id, input.Rebuild)
	if previewErr != nil {
		util.BadRequest(c, previewErr.Error())
		return
	}
	util.OK(c, preview)
}

func (h *Handler) AdminRunUpstreamSync(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "同步源 ID 无效")
		return
	}
	var input struct {
		Rebuild     bool `json:"rebuild"`
		BindUnbound bool `json:"bind_unbound"`
	}
	_ = c.ShouldBindJSON(&input)
	result, runErr := h.models.RunUpstreamSync(c.Request.Context(), id, input.Rebuild, input.BindUnbound)
	if runErr != nil {
		util.BadRequest(c, runErr.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "run_upstream_sync", "upstream_sync_source",
		strconv.FormatInt(id, 10), map[string]interface{}{
			"keys_created": result.KeysCreated, "accounts_created": result.AccountsCreated,
			"accounts_updated": result.AccountsUpdated, "routes_bound": result.RoutesBound,
		})
	util.OK(c, result)
}

func (h *Handler) AdminListUpstreamSyncRuns(c *gin.Context) {
	sourceID, _ := strconv.ParseInt(c.Query("source_id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.models.ListUpstreamSyncRuns(c.Request.Context(), sourceID, limit)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, items)
}

func (h *Handler) AdminGetUpstreamSyncRunItems(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || runID <= 0 {
		util.BadRequest(c, "运行 ID 无效")
		return
	}
	items, listErr := h.models.GetUpstreamSyncRunItems(c.Request.Context(), runID)
	if listErr != nil {
		util.InternalError(c, listErr.Error())
		return
	}
	util.OK(c, items)
}
