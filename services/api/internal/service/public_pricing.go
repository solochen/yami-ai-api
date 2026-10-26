package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/starai/api/internal/billing"
)

// publicPriceRule converts the private route cost rule into a user-facing
// charge rule. The frontend never receives upstream cost fields.
func (s *ModelService) publicPriceRule(
	ctx context.Context,
	model *ModelFull,
) (map[string]interface{}, error) {
	if model == nil {
		return nil, errors.New("model not found")
	}
	if model.Category == "multi_collab" || model.Code == "multi_collab_chat" {
		return map[string]interface{}{
			"billing_type":    "model_actual",
			"settlement_note": "按成员模型实际用量逐项结算",
		}, nil
	}
	cfg, err := billing.LoadPricingConfig(ctx, s.db)
	if err != nil {
		return nil, err
	}
	routes, err := s.RuntimeRoutes(ctx, model)
	if err != nil {
		return nil, err
	}
	var selected *ModelRoute
	for index := range routes {
		availability := routeCostAvailabilityFor(&routes[index])
		if availability.Billable && !availability.Complimentary {
			selected = &routes[index]
			break
		}
	}
	if selected == nil {
		for index := range routes {
			if routeCostAvailabilityFor(&routes[index]).Complimentary {
				return map[string]interface{}{
					"billing_type":    "complimentary",
					"settlement_note": "当前线路明确免费，不扣除算力",
				}, nil
			}
		}
		return nil, fmt.Errorf("%w: %s", billing.ErrProviderCostMissing, model.Code)
	}
	rule := copyMap(selected.CostRule)
	billingType := strings.ToLower(strings.TrimSpace(stringValue(rule["billing_type"])))
	price := map[string]interface{}{"billing_type": billingType}
	convert := func(key, outputKey string) {
		if raw, exists := rule[key]; exists {
			cost := floatValue(raw)
			charge, err := billing.CalculateCharge(cost, cfg.GrossMarginPercent)
			if err == nil {
				price[outputKey] = charge.UserCharge
			}
		}
	}
	switch billingType {
	case "per_token":
		convert("input_cost_per_m", "input_price_per_m")
		convert("output_cost_per_m", "output_price_per_m")
		convert("cache_read_cost_per_m", "cache_read_price_per_m")
		convert("cache_write_cost_per_m", "cache_write_price_per_m")
		convert("audio_input_cost_per_m", "audio_input_price_per_m")
		convert("audio_output_cost_per_m", "audio_output_price_per_m")
		price["input_price"] = microPerMillion(floatValue(price["input_price_per_m"]))
		price["output_price"] = microPerMillion(floatValue(price["output_price_per_m"]))
		price["cache_read_price"] = microPerMillion(floatValue(price["cache_read_price_per_m"]))
		price["cache_write_price"] = microPerMillion(floatValue(price["cache_write_price_per_m"]))
		price["audio_input_price"] = microPerMillion(floatValue(price["audio_input_price_per_m"]))
		price["audio_output_price"] = microPerMillion(floatValue(price["audio_output_price_per_m"]))
	case "per_image", "per_request", "per_second":
		convert("unit_cost", "unit_price")
		for _, tier := range []struct {
			costKey  string
			priceKey string
		}{
			{"unit_cost_by_size", "unit_price_by_size"},
			{"unit_cost_by_resolution", "unit_price_by_resolution"},
			{"unit_cost_by_variant", "unit_price_by_variant"},
		} {
			if rawMap, ok := rule[tier.costKey].(map[string]interface{}); ok {
				converted := map[string]interface{}{}
				for key, raw := range rawMap {
					charge, err := billing.CalculateCharge(floatValue(raw), cfg.GrossMarginPercent)
					if err == nil {
						converted[key] = charge.UserCharge
					}
				}
				price[tier.priceKey] = converted
			}
		}
	case "dynamic":
		for key, raw := range rule {
			if key == "complimentary" || strings.Contains(strings.ToLower(key), "cost") ||
				key == "points_per_cny" || key == "platform_multiplier" || key == "gross_margin_percent" {
				continue
			}
			price[key] = raw
		}
		for _, key := range []string{"rates_per_second", "input_video_rates_per_second", "rates_per_m_tokens", "excess_image_price", "fallback_cost"} {
			raw, exists := rule[key]
			if !exists {
				continue
			}
			nested, ok := raw.(map[string]interface{})
			if !ok {
				if key == "excess_image_price" || key == "fallback_cost" {
					charge, err := billing.CalculateCharge(floatValue(raw), cfg.GrossMarginPercent)
					if err == nil {
						price[key] = charge.UserCharge
					}
				}
				continue
			}
			converted := map[string]interface{}{}
			for resolution, value := range nested {
				if inner, ok := value.(map[string]interface{}); ok {
					row := map[string]interface{}{}
					for innerKey, innerValue := range inner {
						charge, err := billing.CalculateCharge(floatValue(innerValue), cfg.GrossMarginPercent)
						if err == nil {
							row[innerKey] = charge.UserCharge
						}
					}
					converted[resolution] = row
				} else {
					charge, err := billing.CalculateCharge(floatValue(value), cfg.GrossMarginPercent)
					if err == nil {
						converted[resolution] = charge.UserCharge
					}
				}
			}
			price[key] = converted
		}
	}
	price["settlement_note"] = "参考价按当前可计费线路计算，最终按实际线路和用量结算"
	return price, nil
}

func microPerMillion(pricePerMillion float64) float64 {
	if pricePerMillion <= 0 {
		return 0
	}
	return pricePerMillion / 1_000_000
}
