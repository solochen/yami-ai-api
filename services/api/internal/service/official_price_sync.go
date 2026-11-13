package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---------- 官方价外部数据源 ----------

type officialPriceQuote struct {
	InputPerM  float64
	OutputPerM float64
	CachePerM  float64
	Source     string
	SourceID   string
	priority   int // 越小越可信：0=厂商直连 id，1=厂商前缀，2=第三方转售
}

var (
	officialCatalogMu    sync.Mutex
	officialCatalogAt    time.Time
	officialCatalogCache map[string]officialPriceQuote
	officialCatalogErr   error
)

const officialCatalogTTL = 30 * time.Minute

// fetchOfficialCatalog 拉取 LiteLLM + OpenRouter 的官方价目录（美元/百万 token），缓存 30 分钟。
func fetchOfficialCatalog(ctx context.Context) (map[string]officialPriceQuote, error) {
	officialCatalogMu.Lock()
	defer officialCatalogMu.Unlock()
	if time.Since(officialCatalogAt) < officialCatalogTTL && officialCatalogCache != nil {
		return officialCatalogCache, officialCatalogErr
	}
	catalog := map[string]officialPriceQuote{}
	client := &http.Client{Timeout: 25 * time.Second}

	// LiteLLM：{model: {input_cost_per_token, output_cost_per_token, cache_read_input_token_cost}}
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json", nil); err == nil {
		if resp, err := client.Do(req); err == nil {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			resp.Body.Close()
			var payload map[string]struct {
				InputCostPerToken      float64 `json:"input_cost_per_token"`
				OutputCostPerToken     float64 `json:"output_cost_per_token"`
				CacheReadInputTokenCost float64 `json:"cache_read_input_token_cost"`
				Provider               string  `json:"litellm_provider"`
			}
			if json.Unmarshal(raw, &payload) == nil {
				for id, item := range payload {
					if item.InputCostPerToken <= 0 && item.OutputCostPerToken <= 0 {
						continue
					}
					quote := officialPriceQuote{
						InputPerM:  item.InputCostPerToken * 1_000_000,
						OutputPerM: item.OutputCostPerToken * 1_000_000,
						CachePerM:  item.CacheReadInputTokenCost * 1_000_000,
						Source:     "litellm",
						SourceID:   id,
					}
					quote.priority = quotePriority(id)
					for _, key := range catalogKeys(id) {
						if existing, exists := catalog[key]; !exists || quote.priority < existing.priority {
							catalog[key] = quote
						}
					}
				}
			}
		}
	}

	// OpenRouter：data[].id + pricing（美元/token）
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/models", nil); err == nil {
		if resp, err := client.Do(req); err == nil {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
			resp.Body.Close()
			var payload struct {
				Data []struct {
					ID      string `json:"id"`
					Pricing struct {
						Prompt        string `json:"prompt"`
						Completion    string `json:"completion"`
						InputCacheRead string `json:"input_cache_read"`
					} `json:"pricing"`
				} `json:"data"`
			}
			if json.Unmarshal(raw, &payload) == nil {
				for _, item := range payload.Data {
					input := parseFloatString(item.Pricing.Prompt) * 1_000_000
					output := parseFloatString(item.Pricing.Completion) * 1_000_000
					if input <= 0 && output <= 0 {
						continue
					}
					quote := officialPriceQuote{
						InputPerM: input, OutputPerM: output,
						CachePerM: parseFloatString(item.Pricing.InputCacheRead) * 1_000_000,
						Source:    "openrouter", SourceID: item.ID,
					}
					quote.priority = quotePriority(item.ID)
					for _, key := range catalogKeys(item.ID) {
						if existing, exists := catalog[key]; !exists || quote.priority < existing.priority {
							catalog[key] = quote
						}
					}
				}
			}
		}
	}

	if len(catalog) == 0 {
		officialCatalogErr = fmt.Errorf("外部官方价目录为空（LiteLLM/OpenRouter 均不可用）")
	} else {
		officialCatalogErr = nil
	}
	officialCatalogCache = catalog
	officialCatalogAt = time.Now()
	return catalog, officialCatalogErr
}

var vendorPrefixes = []string{"openai/", "anthropic/", "google/", "gemini/", "xai/", "deepseek/", "zhipu/", "z-ai/", "moonshot", "minimax", "alibaba/", "qwen/"}

// quotePriority 越接近厂商直连越可信。
func quotePriority(id string) int {
	lower := strings.ToLower(strings.TrimSpace(id))
	if !strings.Contains(lower, "/") {
		return 0
	}
	for _, prefix := range vendorPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return 1
		}
	}
	return 2
}

func parseFloatString(text string) float64 {
	var value float64
	_, _ = fmt.Sscanf(strings.TrimSpace(text), "%f", &value)
	return value
}

// catalogKeys 生成用于匹配的 key：原始 id、去掉厂商前缀、归一化形式。
func catalogKeys(id string) []string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return nil
	}
	keys := map[string]bool{id: true}
	if idx := strings.LastIndex(id, "/"); idx >= 0 && idx+1 < len(id) {
		keys[id[idx+1:]] = true
	}
	out := make([]string, 0, len(keys)*2)
	for key := range keys {
		out = append(out, key)
		out = append(out, normalizeModelKey(key))
	}
	return out
}

func normalizeModelKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("_", "-", ".", "-", " ", "", ":", "-")
	return replacer.Replace(value)
}

// ---------- 官方价校准 ----------

type OfficialPriceRow struct {
	ModelID       int64   `json:"model_id"`
	ModelCode     string  `json:"model_code"`
	DisplayName   string  `json:"display_name"`
	Currency      string  `json:"currency"`
	CurrentInput  float64 `json:"current_input"`
	CurrentOutput float64 `json:"current_output"`
	DerivedUSDIn  float64 `json:"derived_usd_input"`
	DerivedUSDOut float64 `json:"derived_usd_output"`
	DerivedCount  int     `json:"derived_samples"`
	ExternalIn    float64 `json:"external_usd_input"`
	ExternalOut   float64 `json:"external_usd_output"`
	ExternalSrc   string  `json:"external_source"`
	ExternalID    string  `json:"external_id"`
	SuggestedIn   float64 `json:"suggested_usd_input"`
	SuggestedOut  float64 `json:"suggested_usd_output"`
	SuggestedSrc  string  `json:"suggested_source"`
	SpreadPct     float64 `json:"spread_pct"`
	Note          string  `json:"note"`
}

// PreviewOfficialPrices 汇总每个模型的：当前官方价、内部反推美元价、外部目录价与建议值。
func (s *ModelService) PreviewOfficialPrices(ctx context.Context) ([]OfficialPriceRow, error) {
	catalog, catalogErr := fetchOfficialCatalog(ctx)
	settlements := s.loadSettlementRates(ctx)
	rows, err := s.db.Query(ctx, `SELECT m.id, m.code, m.display_name, m.official_price_rule, m.new_api_model
		FROM models m WHERE m.is_enabled = true ORDER BY m.sort_order, m.id`)
	if err != nil {
		return nil, err
	}
	type modelInfo struct {
		id       int64
		code     string
		name     string
		official map[string]interface{}
		upstream string
	}
	models := []modelInfo{}
	for rows.Next() {
		var info modelInfo
		var officialRaw []byte
		if err := rows.Scan(&info.id, &info.code, &info.name, &officialRaw, &info.upstream); err != nil {
			continue
		}
		_ = json.Unmarshal(officialRaw, &info.official)
		models = append(models, info)
	}
	rows.Close()

	// 内部反推：cost / 分组倍率 / 矩阵汇率 = 厂商官方美元价
	derived := map[int64][]float64{}
	derivedOut := map[int64][]float64{}
	routeRows, err := s.db.Query(ctx, `
		SELECT r.model_id, r.provider, r.cost_rule, g.rate
		FROM model_routes r
		JOIN upstream_accounts a ON a.id = r.account_id
		JOIN upstream_group_rates g ON g.provider = r.provider AND g.group_name = a.group_name
		WHERE r.is_enabled = true AND g.rate > 0`)
	if err == nil {
		for routeRows.Next() {
			var modelID int64
			var provider string
			var rate float64
			var costRaw []byte
			if err := routeRows.Scan(&modelID, &provider, &costRaw, &rate); err != nil {
				continue
			}
			if rate <= 0 {
				continue
			}
			settlement, ok := settlements[provider]
			if !ok || settlement.CNY <= 0 {
				continue
			}
			var cost map[string]interface{}
			_ = json.Unmarshal(costRaw, &cost)
			if inCost := floatValue(cost["input_cost_per_m"]); inCost > 0 {
				derived[modelID] = append(derived[modelID], inCost/rate/settlement.CNY)
			}
			if outCost := floatValue(cost["output_cost_per_m"]); outCost > 0 {
				derivedOut[modelID] = append(derivedOut[modelID], outCost/rate/settlement.CNY)
			}
		}
		routeRows.Close()
	}

	out := make([]OfficialPriceRow, 0, len(models))
	for _, model := range models {
		row := OfficialPriceRow{ModelID: model.id, ModelCode: model.code, DisplayName: model.name}
		row.Currency = strings.ToUpper(stringValue(model.official["currency"]))
		if row.Currency == "" {
			row.Currency = "CNY"
		}
		row.CurrentInput = floatValue(model.official["input_price_per_m"])
		row.CurrentOutput = floatValue(model.official["output_price_per_m"])
		if samples := derived[model.id]; len(samples) > 0 {
			row.DerivedUSDIn = medianFloat(samples)
			row.DerivedUSDOut = medianFloat(derivedOut[model.id])
			row.DerivedCount = len(samples)
			if min, max := minMaxFloat(samples); min > 0 {
				row.SpreadPct = (max - min) / min * 100
			}
		}
		for _, key := range []string{normalizeModelKey(model.code), normalizeModelKey(model.upstream)} {
			if quote, ok := catalog[key]; ok && quote.InputPerM > 0 {
				row.ExternalIn = quote.InputPerM
				row.ExternalOut = quote.OutputPerM
				row.ExternalSrc = quote.Source
				row.ExternalID = quote.SourceID
				break
			}
		}
		switch {
		case row.ExternalIn > 0:
			row.SuggestedIn, row.SuggestedOut, row.SuggestedSrc = row.ExternalIn, row.ExternalOut, row.ExternalSrc
		case row.DerivedUSDIn > 0:
			row.SuggestedIn, row.SuggestedOut, row.SuggestedSrc = row.DerivedUSDIn, row.DerivedUSDOut, "derived"
			row.Note = "由上游价格反推，建议人工核对"
		default:
			row.Note = "无外部价、无法反推，需手工填写"
		}
		out = append(out, row)
	}
	if catalogErr != nil && len(out) > 0 {
		out[0].Note = catalogErr.Error() + "；仅展示内部反推结果"
	}
	return out, nil
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64{}, values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func minMaxFloat(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	min, max := values[0], values[0]
	for _, value := range values {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}
	return min, max
}

type OfficialPriceApplyItem struct {
	ModelID       int64   `json:"model_id"`
	InputPerM     float64 `json:"input_per_m"`
	OutputPerM    float64 `json:"output_per_m"`
	CacheReadPerM float64 `json:"cache_read_per_m"`
	Currency      string  `json:"currency"`
	Source        string  `json:"source"`
}

// ApplyOfficialPrices 批量写入厂商官方价（默认 USD）。
func (s *ModelService) ApplyOfficialPrices(ctx context.Context, items []OfficialPriceApplyItem) (int64, error) {
	var updated int64
	for _, item := range items {
		if item.ModelID <= 0 || item.InputPerM < 0 || item.OutputPerM < 0 {
			continue
		}
		currency := strings.ToUpper(strings.TrimSpace(item.Currency))
		if currency == "" {
			currency = "USD"
		}
		rule := map[string]interface{}{
			"currency":          currency,
			"billing_type":      "per_token",
			"input_price_per_m": item.InputPerM,
			"output_price_per_m": item.OutputPerM,
			"source":            item.Source,
			"verified_at":       time.Now().Format("2006-01-02"),
		}
		if item.CacheReadPerM > 0 {
			rule["cache_read_price_per_m"] = item.CacheReadPerM
		}
		encoded, _ := json.Marshal(rule)
		tag, err := s.db.Exec(ctx, `UPDATE models SET official_price_rule=$1, updated_at=now() WHERE id=$2`, encoded, item.ModelID)
		if err != nil {
			return updated, err
		}
		updated += tag.RowsAffected()
	}
	return updated, nil
}
