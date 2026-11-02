package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

func (h *Handler) AdminListUpstreamProviders(c *gin.Context) {
	items, err := h.admin.ListUpstreamProviders(c.Request.Context())
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, items)
}

func (h *Handler) AdminUpdateUpstreamProvider(c *gin.Context) {
	var in service.UpdateUpstreamProviderInput
	if err := c.ShouldBindJSON(&in); err != nil {
		util.BadRequest(c, "参数无效")
		return
	}
	item, err := h.admin.UpdateUpstreamProvider(c.Request.Context(), c.Param("provider"), in)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "update_upstream_provider", "upstream_provider", c.Param("provider"), map[string]interface{}{
		"source_type": in.SourceType, "is_enabled": in.IsEnabled,
	})
	util.OK(c, item)
}

func (h *Handler) AdminStartPriceSync(c *gin.Context) {
	var in struct {
		Providers []string `json:"providers"`
	}
	_ = c.ShouldBindJSON(&in)
	runID, err := h.admin.StartPriceSync(c.Request.Context(), c.GetInt64("admin_id"), in.Providers)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "sync_upstream_prices", "price_sync_run", strconv.FormatInt(runID, 10), map[string]interface{}{
		"providers": in.Providers,
	})
	util.Created(c, gin.H{"run_id": runID})
}

func (h *Handler) AdminListPriceSyncRuns(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	items, total, err := h.admin.ListPriceSyncRuns(c.Request.Context(), page, pageSize)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminGetPriceSyncRun(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "批次 ID 无效")
		return
	}
	item, err := h.admin.GetPriceSyncRun(c.Request.Context(), id)
	if err != nil {
		util.NotFound(c, "同步批次不存在")
		return
	}
	util.OK(c, item)
}

func (h *Handler) AdminListUpstreamPrices(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	trusted := c.DefaultQuery("trusted", "") == "1" || c.DefaultQuery("trusted", "") == "true"
	items, total, err := h.admin.ListUpstreamPrices(c.Request.Context(), page, pageSize,
		strings.TrimSpace(c.Query("provider")), strings.TrimSpace(c.Query("q")), strings.TrimSpace(c.Query("group")), trusted)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminPriceAnalysis(c *gin.Context) {
	items, err := h.admin.PriceAnalysis(c.Request.Context())
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, gin.H{"items": items})
}

func (h *Handler) AdminPriceModelDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "模型 ID 无效")
		return
	}
	item, err := h.admin.PriceModelDetail(c.Request.Context(), id)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, item)
}

func (h *Handler) AdminUpsertManualPrice(c *gin.Context) {
	var in service.ManualPriceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		util.BadRequest(c, "参数无效")
		return
	}
	item, err := h.admin.UpsertManualPrice(c.Request.Context(), in)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "manual_upstream_price", "upstream_price", in.Provider+"/"+in.UpstreamModel, map[string]interface{}{
		"group_name": in.GroupName, "billing_type": in.BillingType,
	})
	util.OK(c, item)
}

func (h *Handler) AdminListUpstreamMappings(c *gin.Context) {
	unmatched := c.DefaultQuery("unmatched", "1") == "1" || c.DefaultQuery("unmatched", "") == "true"
	items, err := h.admin.ListUpstreamMappings(c.Request.Context(), unmatched)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, gin.H{"items": items})
}

func (h *Handler) AdminUpdateUpstreamMapping(c *gin.Context) {
	var in struct {
		Provider      string `json:"provider"`
		UpstreamModel string `json:"upstream_model"`
		ModelID       int64  `json:"model_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.ModelID <= 0 {
		util.BadRequest(c, "参数无效")
		return
	}
	if err := h.admin.UpdateUpstreamMapping(c.Request.Context(), in.Provider, in.UpstreamModel, in.ModelID); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "confirm_upstream_model_mapping", "upstream_model_mapping", in.Provider+"/"+in.UpstreamModel, map[string]interface{}{
		"model_id": in.ModelID,
	})
	util.OK(c, gin.H{"ok": true})
}

func (h *Handler) AdminModelRouteOverview(c *gin.Context) {
	items, err := h.admin.ModelRouteOverview(c.Request.Context())
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, gin.H{"items": items})
}

func (h *Handler) AdminPromotePreview(c *gin.Context) {
	modelID, err1 := strconv.ParseInt(c.Param("id"), 10, 64)
	routeID, err2 := strconv.ParseInt(c.Param("routeId"), 10, 64)
	if err1 != nil || err2 != nil || modelID <= 0 || routeID <= 0 {
		util.BadRequest(c, "模型或线路 ID 无效")
		return
	}
	row, target, daily, monthly, err := h.admin.PromotePreview(c.Request.Context(), modelID, routeID)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	util.OK(c, gin.H{"model": row, "target": target, "daily_saving": daily, "monthly_saving": monthly})
}

func (h *Handler) AdminPromoteModelRoute(c *gin.Context) {
	modelID, err1 := strconv.ParseInt(c.Param("id"), 10, 64)
	routeID, err2 := strconv.ParseInt(c.Param("routeId"), 10, 64)
	if err1 != nil || err2 != nil || modelID <= 0 || routeID <= 0 {
		util.BadRequest(c, "模型或线路 ID 无效")
		return
	}
	var in struct {
		Confirm bool   `json:"confirm"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || !in.Confirm {
		util.BadRequest(c, "请先确认切换")
		return
	}
	row, _, daily, monthly, err := h.admin.PromotePreview(c.Request.Context(), modelID, routeID)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	result, err := h.models.PromoteModelRoute(c.Request.Context(), modelID, routeID, c.GetInt64("admin_id"), in.Reason, daily, monthly)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "promote_model_route", "model_route", strconv.FormatInt(routeID, 10), map[string]interface{}{
		"model_id": modelID, "model_code": row.Code, "from_route_id": result.FromRouteID,
		"changed": result.Changed, "estimated_daily_saving": daily, "estimated_monthly_saving": monthly,
	})
	util.OK(c, gin.H{"result": result, "model": row, "daily_saving": daily, "monthly_saving": monthly})
}

func (h *Handler) AdminApplyRealCost(c *gin.Context) {
	var in struct {
		ModelID int64 `json:"model_id"`
		RouteID int64 `json:"route_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.ModelID <= 0 || in.RouteID <= 0 {
		util.BadRequest(c, "model_id 和 route_id 不能为空")
		return
	}
	item, err := h.admin.ApplyRealCost(c.Request.Context(), in.ModelID, in.RouteID)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "apply_real_upstream_cost", "model_route", strconv.FormatInt(in.RouteID, 10), map[string]interface{}{
		"model_id": in.ModelID, "source_type": item.SourceType, "upstream_model": item.UpstreamModel,
	})
	util.OK(c, item)
}

func (h *Handler) AdminListRouteSwitches(c *gin.Context) {
	modelID, _ := strconv.ParseInt(c.DefaultQuery("model_id", "0"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	items, total, err := h.admin.ListRouteSwitches(c.Request.Context(), modelID, page, pageSize)
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) AdminRollbackModelRoute(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "切换记录 ID 无效")
		return
	}
	result, err := h.models.RollbackModelRoute(c.Request.Context(), id, c.GetInt64("admin_id"))
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "rollback_model_route", "model_route_switch", strconv.FormatInt(id, 10), map[string]interface{}{
		"model_id": result.ModelID, "new_switch_id": result.SwitchID,
	})
	util.OK(c, result)
}

var _ = http.StatusOK
