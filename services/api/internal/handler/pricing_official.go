package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

// AdminRecalibrateRouteCosts 按「官方价 × 分组倍率」重算线路成本（支持 dry_run 预览）。
func (h *Handler) AdminRecalibrateRouteCosts(c *gin.Context) {
	var input struct {
		Provider string  `json:"provider"`
		RouteIDs []int64 `json:"route_ids"`
		DryRun   bool    `json:"dry_run"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "参数无效")
		return
	}
	changes, err := h.models.RecalibrateRouteCosts(c.Request.Context(), input.Provider, input.RouteIDs, input.DryRun)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	updated := 0
	for _, change := range changes {
		if change.NewCostRule != nil {
			updated++
		}
	}
	util.OK(c, gin.H{"items": changes, "updatable": updated, "total": len(changes), "dry_run": input.DryRun})
}

// AdminPricingAudit 返回每个模型的售价/成本/毛利审计结果。
func (h *Handler) AdminPricingAudit(c *gin.Context) {
	rows, err := h.models.PricingAudit(c.Request.Context())
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.Status]++
	}
	util.OK(c, gin.H{"items": rows, "counts": counts, "total": len(rows)})
}

// AdminBulkSetPriceMultiplier 批量设置售价倍率。
func (h *Handler) AdminBulkSetPriceMultiplier(c *gin.Context) {
	var input struct {
		ModelIDs   []int64 `json:"model_ids"`
		Multiplier float64 `json:"multiplier"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "参数无效")
		return
	}
	affected, err := h.models.BulkSetPriceMultiplier(c.Request.Context(), input.ModelIDs, input.Multiplier)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "bulk_set_price_multiplier", "model",
		"", map[string]interface{}{"multiplier": input.Multiplier, "affected": affected})
	util.OK(c, gin.H{"affected": affected})
}

var _ = strconv.Itoa

// AdminSetModelMultiplier 单独设置某个模型的售价倍率。
func (h *Handler) AdminSetModelMultiplier(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "模型 ID 无效")
		return
	}
	var input struct {
		Multiplier float64 `json:"multiplier"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "参数无效")
		return
	}
	affected, setErr := h.models.SetModelPriceMultiplier(c.Request.Context(), id, input.Multiplier)
	if setErr != nil {
		util.BadRequest(c, setErr.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "set_model_multiplier", "model",
		strconv.FormatInt(id, 10), map[string]interface{}{"multiplier": input.Multiplier})
	util.OK(c, gin.H{"affected": affected})
}

// AdminPreviewOfficialPrices 汇总外部目录价 + 内部反推价，供人工核对。
func (h *Handler) AdminPreviewOfficialPrices(c *gin.Context) {
	rows, err := h.models.PreviewOfficialPrices(c.Request.Context())
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	matched, derived, manual := 0, 0, 0
	for _, row := range rows {
		switch row.SuggestedSrc {
		case "litellm", "openrouter":
			matched++
		case "derived":
			derived++
		default:
			manual++
		}
	}
	util.OK(c, gin.H{"items": rows, "total": len(rows), "external_matched": matched, "derived": derived, "manual": manual})
}

// AdminApplyOfficialPrices 应用选中的官方价。
func (h *Handler) AdminApplyOfficialPrices(c *gin.Context) {
	var input struct {
		Items []service.OfficialPriceApplyItem `json:"items"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		util.BadRequest(c, "参数无效")
		return
	}
	updated, err := h.models.ApplyOfficialPrices(c.Request.Context(), input.Items)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	h.admin.LogOperation(c.Request.Context(), c.GetInt64("admin_id"), "apply_official_prices", "model",
		"", map[string]interface{}{"updated": updated})
	util.OK(c, gin.H{"updated": updated})
}
