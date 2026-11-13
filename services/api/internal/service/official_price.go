package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/starai/api/internal/billing"
)

// officialPriceCNY 把模型官方价统一换算成「人民币 / 百万 tokens」。
// USD 官方价乘以 system_configs.model_usd_to_compute_rate（当前 7.2）；CNY 官方价原样使用。
func officialPriceCNY(rule map[string]interface{}, fx float64) map[string]float64 {
	out := map[string]float64{}
	if len(rule) == 0 {
		return out
	}
	multiplier := 1.0
	if strings.EqualFold(stringValue(rule["currency"]), "USD") {
		if fx <= 0 {
			fx = 7.2
		}
		multiplier = fx
	}
	for _, key := range []string{
		"input_price_per_m", "output_price_per_m",
		"cache_read_price_per_m", "cache_write_price_per_m",
	} {
		if value := floatValue(rule[key]); value > 0 {
			out[key] = value * multiplier
		}
	}
	return out
}

// officialPriceNative 返回官方价（保持原币种，不做汇率换算）。
func officialPriceNative(rule map[string]interface{}) map[string]float64 {
	out := map[string]float64{}
	for _, key := range []string{
		"input_price_per_m", "output_price_per_m",
		"cache_read_price_per_m", "cache_write_price_per_m",
	} {
		if value := floatValue(rule[key]); value > 0 {
			out[key] = value
		}
	}
	return out
}

// loadSettlementRates 读取各上游结算汇率与矩阵汇率。
func (s *ModelService) loadSettlementRates(ctx context.Context) map[string]struct{ CNY, MatrixFX float64 } {
	out := map[string]struct{ CNY, MatrixFX float64 }{}
	rows, err := s.db.Query(ctx, `SELECT provider, cny_per_unit, matrix_fx FROM upstream_settlements`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var provider string
		var cny, matrixFX float64
		if err := rows.Scan(&provider, &cny, &matrixFX); err != nil {
			continue
		}
		if matrixFX <= 0 {
			matrixFX = 1
		}
		out[provider] = struct{ CNY, MatrixFX float64 }{cny, matrixFX}
	}
	return out
}

// modelOfficialUserCharge 按「官方价 × 售价倍率」计算用户应付金额。
// 目前仅支持 per_token；其它计费类型仍走原有成本加成逻辑。
func modelOfficialUserCharge(model *ModelFull, cfg billing.PricingConfig, promptTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) (float64, bool) {
	if model == nil || model.PriceMultiplier <= 0 || len(model.OfficialPriceRule) == 0 {
		return 0, false
	}
	billingType := strings.ToLower(strings.TrimSpace(stringValue(model.OfficialPriceRule["billing_type"])))
	if billingType != "" && billingType != "per_token" {
		return 0, false
	}
	if promptTokens <= 0 && outputTokens <= 0 {
		return 0, false
	}
	prices := officialPriceCNY(model.OfficialPriceRule, cfg.USDToComputeRate)
	if len(prices) == 0 {
		return 0, false
	}
	charge := 0.0
	charge += prices["input_price_per_m"] / 1_000_000 * float64(maxNonNegative(promptTokens))
	charge += prices["output_price_per_m"] / 1_000_000 * float64(maxNonNegative(outputTokens))
	charge += prices["cache_read_price_per_m"] / 1_000_000 * float64(maxNonNegative(cacheReadTokens))
	charge += prices["cache_write_price_per_m"] / 1_000_000 * float64(maxNonNegative(cacheWriteTokens))
	return roundCredits(charge * model.PriceMultiplier), true
}

// modelOfficialPublicPrice 生成对外展示的售价（官方价 × 倍率）。
func modelOfficialPublicPrice(model *ModelFull, cfg billing.PricingConfig) map[string]interface{} {
	if model == nil || model.PriceMultiplier <= 0 || len(model.OfficialPriceRule) == 0 {
		return nil
	}
	billingType := strings.ToLower(strings.TrimSpace(stringValue(model.OfficialPriceRule["billing_type"])))
	if billingType != "" && billingType != "per_token" {
		return nil
	}
	prices := officialPriceCNY(model.OfficialPriceRule, cfg.USDToComputeRate)
	if len(prices) == 0 {
		return nil
	}
	price := map[string]interface{}{"billing_type": "per_token", "currency": "CNY", "price_multiplier": model.PriceMultiplier}
	for key, value := range prices {
		price[key] = roundCredits(value * model.PriceMultiplier)
	}
	return price
}

func maxNonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

// ---------- 线路成本自动校准 ----------

type RouteCostChange struct {
	RouteID     int64                  `json:"route_id"`
	Provider    string                 `json:"provider"`
	RouteName   string                 `json:"route_name"`
	ModelCode   string                 `json:"model_code"`
	GroupName   string                 `json:"group_name"`
	GroupRate   float64                `json:"group_rate"`
	OldCostRule map[string]interface{} `json:"old_cost_rule,omitempty"`
	NewCostRule map[string]interface{} `json:"new_cost_rule,omitempty"`
	Reason      string                 `json:"reason,omitempty"`
}

// RecalibrateRouteCosts 重算线路成本。
// 优先使用上游价格矩阵里「该上游 / 该分组 / 该模型」的真实报价（× 结算汇率）；
// 矩阵缺失时才回退到「厂商官方价 × 分组倍率 × 结算汇率」。
func (s *ModelService) RecalibrateRouteCosts(ctx context.Context, provider string, routeIDs []int64, dryRun bool) ([]RouteCostChange, error) {
	q := `SELECT r.id, r.provider, r.route_name, m.code, COALESCE(a.group_name,''), COALESCE(g.rate,0), r.cost_rule, m.official_price_rule,
		       COALESCE(c.cny_input_per_m,0), COALESCE(c.cny_output_per_m,0),
		       COALESCE(c.cny_cache_read_per_m,0), COALESCE(c.cny_cache_write_per_m,0)
		FROM model_routes r
		JOIN models m ON m.id = r.model_id
		LEFT JOIN upstream_accounts a ON a.id = r.account_id
		LEFT JOIN upstream_group_rates g ON g.provider = r.provider AND g.group_name = a.group_name
		LEFT JOIN LATERAL (
			SELECT p.cny_input_per_m, p.cny_output_per_m, p.cny_cache_read_per_m, p.cny_cache_write_per_m
			FROM upstream_price_current p
			WHERE p.provider = r.provider AND p.upstream_model = r.upstream_model
			  AND p.billing_type = 'per_token'
			  AND (p.group_name = COALESCE(a.group_name,'') OR p.group_name = '')
			ORDER BY (p.group_name = COALESCE(a.group_name,'')) DESC
			LIMIT 1
		) c ON true
		WHERE ($1 = '' OR r.provider = $1)`
	args := []interface{}{provider}
	if len(routeIDs) > 0 {
		q += ` AND r.id = ANY($2)`
		args = append(args, routeIDs)
	}
	q += ` ORDER BY r.id`
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	settlements := s.loadSettlementRates(ctx)
	changes := []RouteCostChange{}
	updates := []struct {
		id   int64
		rule []byte
	}{}
	for rows.Next() {
		var id int64
		var providerName, routeName, modelCode, group string
		var rate float64
		var costRaw, officialRaw []byte
		var matrixIn, matrixOut, matrixCacheRead, matrixCacheWrite float64
		if err := rows.Scan(&id, &providerName, &routeName, &modelCode, &group, &rate, &costRaw, &officialRaw,
			&matrixIn, &matrixOut, &matrixCacheRead, &matrixCacheWrite); err != nil {
			continue
		}
		var official map[string]interface{}
		_ = json.Unmarshal(officialRaw, &official)
		var oldCost map[string]interface{}
		_ = json.Unmarshal(costRaw, &oldCost)
		change := RouteCostChange{RouteID: id, Provider: providerName, RouteName: routeName, ModelCode: modelCode,
			GroupName: group, GroupRate: rate, OldCostRule: oldCost}
		settlement, ok := settlements[providerName]
		if !ok || settlement.CNY <= 0 {
			settlement = struct{ CNY, MatrixFX float64 }{1, 1}
		}
		prices := officialPriceNative(official)
		switch {
		case group == "":
			change.Reason = "线路未绑定上游账号/分组"
		case matrixIn > 0 && !matrixPriceImplausible(official, matrixIn, settlement.CNY):
			// 上游矩阵有真实报价且数值合理：直接用它的价格 × 结算汇率
			newCost := map[string]interface{}{}
			for k, v := range oldCost {
				newCost[k] = v
			}
			newCost["billing_type"] = "per_token"
			newCost["source_currency"] = "CNY"
			newCost["price_unit_normalized"] = true
			newCost["group_rate"] = rate
			newCost["settlement_rate"] = settlement.CNY
			newCost["cost_source"] = "upstream_matrix"
			newCost["input_cost_per_m"] = roundCredits(matrixIn * settlement.CNY)
			newCost["output_cost_per_m"] = roundCredits(matrixOut * settlement.CNY)
			if matrixCacheRead > 0 {
				newCost["cache_read_cost_per_m"] = roundCredits(matrixCacheRead * settlement.CNY)
			}
			if matrixCacheWrite > 0 {
				newCost["cache_write_cost_per_m"] = roundCredits(matrixCacheWrite * settlement.CNY)
			}
			change.NewCostRule = newCost
			encoded, _ := json.Marshal(newCost)
			updates = append(updates, struct {
				id   int64
				rule []byte
			}{id: id, rule: encoded})
		case len(prices) > 0 && rate > 0:
			// 兜底：官方价 × 结算汇率 × 分组倍率
			if matrixIn > 0 {
				change.Reason = "上游矩阵价格异常（疑似匹配到其它规格），已回退官方价 × 倍率"
			}
			newCost := map[string]interface{}{}
			for k, v := range oldCost {
				newCost[k] = v
			}
			newCost["billing_type"] = "per_token"
			newCost["source_currency"] = "CNY"
			newCost["price_unit_normalized"] = true
			newCost["group_rate"] = rate
			newCost["settlement_rate"] = settlement.CNY
			newCost["cost_source"] = "official_x_ratio"
			for key, value := range prices {
				costKey := strings.Replace(key, "_price_per_m", "_cost_per_m", 1)
				newCost[costKey] = roundCredits(value * settlement.CNY * rate)
			}
			change.NewCostRule = newCost
			encoded, _ := json.Marshal(newCost)
			updates = append(updates, struct {
				id   int64
				rule []byte
			}{id: id, rule: encoded})
		case len(prices) == 0:
			change.Reason = "缺少官方价，且上游矩阵没有该模型报价"
		default:
			change.Reason = "缺少分组倍率（upstream_group_rates）"
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if dryRun {
		return changes, nil
	}
	for _, item := range updates {
		if _, err := s.db.Exec(ctx, `UPDATE model_routes SET cost_rule=$1, updated_at=now() WHERE id=$2`, item.rule, item.id); err != nil {
			return changes, fmt.Errorf("更新线路 %d 成本失败：%w", item.id, err)
		}
	}
	return changes, nil
}

// ---------- 定价审计（售价 vs 成本 vs 毛利） ----------

type PricingAuditRow struct {
	ModelID      int64   `json:"model_id"`
	ModelCode    string  `json:"model_code"`
	DisplayName  string  `json:"display_name"`
	Category     string  `json:"category"`
	Currency     string  `json:"currency"`
	Multiplier   float64 `json:"price_multiplier"`
	OfficialIn   float64 `json:"official_input"`
	OfficialOut  float64 `json:"official_output"`
	SaleIn       float64 `json:"sale_input"`
	SaleOut      float64 `json:"sale_output"`
	CostIn       float64 `json:"cost_input"`
	CostOut      float64 `json:"cost_output"`
	MarginPct    float64 `json:"margin_pct"`
	Status       string  `json:"status"` // ok / low / loss / missing
}

// PricingAudit 汇总每个模型的官方价、倍率售价、最低线路成本与毛利率。
func (s *ModelService) PricingAudit(ctx context.Context) ([]PricingAuditRow, error) {
	fx := 7.2
	if cfg, err := billing.LoadPricingConfig(ctx, s.db); err == nil && cfg.USDToComputeRate > 0 {
		fx = cfg.USDToComputeRate
	}
	rows, err := s.db.Query(ctx, `
		SELECT m.id, m.code, m.display_name, m.category, m.official_price_rule, m.price_multiplier,
		       MIN(NULLIF((r.cost_rule->>'input_cost_per_m')::numeric, 0)) AS min_in,
		       MIN(NULLIF((r.cost_rule->>'output_cost_per_m')::numeric, 0)) AS min_out
		FROM models m
		LEFT JOIN model_routes r ON r.model_id = m.id AND r.is_enabled = true
		WHERE m.is_enabled = true
		GROUP BY m.id, m.code, m.display_name, m.category, m.official_price_rule, m.price_multiplier
		ORDER BY m.sort_order, m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PricingAuditRow{}
	for rows.Next() {
		var row PricingAuditRow
		var officialRaw []byte
		var minIn, minOut *float64
		if err := rows.Scan(&row.ModelID, &row.ModelCode, &row.DisplayName, &row.Category, &officialRaw, &row.Multiplier, &minIn, &minOut); err != nil {
			continue
		}
		var official map[string]interface{}
		_ = json.Unmarshal(officialRaw, &official)
		row.Currency = strings.ToUpper(stringValue(official["currency"]))
		if row.Currency == "" {
			row.Currency = "CNY"
		}
		prices := officialPriceCNY(official, fx)
		row.OfficialIn = prices["input_price_per_m"]
		row.OfficialOut = prices["output_price_per_m"]
		if minIn != nil {
			row.CostIn = *minIn
		}
		if minOut != nil {
			row.CostOut = *minOut
		}
		if row.Multiplier > 0 {
			row.SaleIn = roundCredits(row.OfficialIn * row.Multiplier)
			row.SaleOut = roundCredits(row.OfficialOut * row.Multiplier)
		}
		switch {
		case row.Multiplier <= 0:
			row.Status = "disabled"
		case row.SaleIn <= 0 || row.CostIn <= 0:
			row.Status = "missing"
		default:
			row.MarginPct = math.Round((row.SaleIn-row.CostIn)/row.SaleIn*1000) / 10
			switch {
			case row.MarginPct < 0:
				row.Status = "loss"
			case row.MarginPct < 20:
				row.Status = "low"
			default:
				row.Status = "ok"
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// BulkSetPriceMultiplier 批量设置售价倍率（modelIDs 为空表示所有模型）。
func (s *ModelService) BulkSetPriceMultiplier(ctx context.Context, modelIDs []int64, multiplier float64) (int64, error) {
	if multiplier < 0 || multiplier > 10 {
		return 0, fmt.Errorf("倍率需在 0–10 之间")
	}
	var tag interface{ RowsAffected() int64 }
	var err error
	if len(modelIDs) > 0 {
		tag, err = s.db.Exec(ctx, `UPDATE models SET price_multiplier=$1, updated_at=now() WHERE id = ANY($2)`, multiplier, modelIDs)
	} else {
		tag, err = s.db.Exec(ctx, `UPDATE models SET price_multiplier=$1, updated_at=now() WHERE official_price_rule <> '{}'::jsonb`, multiplier)
	}
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// SetModelPriceMultiplier 设置单个模型的售价倍率（0 表示关闭倍率定价）。
func (s *ModelService) SetModelPriceMultiplier(ctx context.Context, modelID int64, multiplier float64) (int64, error) {
	if multiplier < 0 || multiplier > 10 {
		return 0, fmt.Errorf("倍率需在 0–10 之间")
	}
	tag, err := s.db.Exec(ctx, `UPDATE models SET price_multiplier=$1, updated_at=now() WHERE id=$2`, multiplier, modelID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// matrixPriceImplausible 判断上游矩阵价格是否明显不合理（防止匹配到不同规格/按次计费的行）。
// 合理区间：官方价（人民币等值）的 1% ~ 200%。
func matrixPriceImplausible(official map[string]interface{}, matrixIn, settlementCNY float64) bool {
	if matrixIn <= 0 {
		return true
	}
	prices := officialPriceNative(official)
	officialIn := prices["input_price_per_m"]
	if officialIn <= 0 {
		return false
	}
	currency := strings.ToUpper(stringValue(official["currency"]))
	base := officialIn * settlementCNY
	if currency == "USD" {
		base = officialIn * 6.98
	}
	if base <= 0 {
		return false
	}
	return matrixIn > base*2 || matrixIn < base*0.01
}
