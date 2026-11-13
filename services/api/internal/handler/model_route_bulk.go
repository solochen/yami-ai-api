package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

func (h *Handler) AdminListModelRoutesBulk(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	modelID, _ := strconv.ParseInt(c.Query("model_id"), 10, 64)
	items, total, err := h.models.ListModelRoutesForAdmin(c.Request.Context(), service.ModelRouteListFilter{
		Provider:  strings.TrimSpace(c.Query("provider")),
		Keyword:   strings.TrimSpace(c.Query("q")),
		AccountID: strings.TrimSpace(c.Query("account_id")),
		Enabled:   strings.TrimSpace(c.Query("enabled")),
		ModelID:   modelID,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminBindModelRoutes(c *gin.Context) {
	var input struct {
		RouteIDs  []int64 `json:"route_ids"`
		ModelID   int64   `json:"model_id"`
		AccountID int64   `json:"account_id"`
		Force     bool    `json:"force"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "参数无效")
		return
	}
	affected, err := h.models.BindRoutesToAccount(c.Request.Context(), input.RouteIDs, input.ModelID, input.AccountID, input.Force)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "bind_model_routes", "model_route",
		strconv.FormatInt(input.AccountID, 10), map[string]interface{}{"routes": affected, "model_id": input.ModelID})
	util.OK(c, gin.H{"affected": affected})
}

// AdminBindSingleModelRoute 单条线路切换账号（行内下拉即时保存）。
func (h *Handler) AdminBindSingleModelRoute(c *gin.Context) {
	routeID, err := strconv.ParseInt(c.Param("routeId"), 10, 64)
	if err != nil || routeID <= 0 {
		util.BadRequest(c, "线路 ID 无效")
		return
	}
	var input struct {
		AccountID int64 `json:"account_id"`
		Force     bool  `json:"force"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "参数无效")
		return
	}
	affected, bindErr := h.models.BindRoutesToAccount(c.Request.Context(), []int64{routeID}, 0, input.AccountID, input.Force)
	if bindErr != nil {
		util.BadRequest(c, bindErr.Error())
		return
	}
	if affected == 0 {
		util.BadRequest(c, "线路未更新")
		return
	}
	util.OK(c, gin.H{"affected": affected})
}
