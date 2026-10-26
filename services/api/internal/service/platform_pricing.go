package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/starai/api/internal/billing"
)

type routeCostAvailability struct {
	Billable      bool
	Complimentary bool
	Source        string
}

func routeCostAvailabilityFor(route *ModelRoute) routeCostAvailability {
	if route == nil {
		return routeCostAvailability{Source: "legacy_model_price"}
	}
	rule := route.CostRule
	if len(rule) == 0 {
		return routeCostAvailability{Source: "missing_route_cost"}
	}
	if complimentary, _ := rule["complimentary"].(bool); complimentary {
		return routeCostAvailability{Billable: true, Complimentary: true, Source: "complimentary"}
	}
	billingType := strings.ToLower(strings.TrimSpace(stringValue(rule["billing_type"])))
	switch billingType {
	case "per_token":
		if positiveAny(rule["input_cost_per_m"], rule["output_cost_per_m"],
			rule["cache_read_cost_per_m"], rule["cache_write_cost_per_m"],
			rule["audio_input_cost_per_m"], rule["audio_output_cost_per_m"]) {
			return routeCostAvailability{Billable: true, Source: "route_cost"}
		}
	case "per_image", "per_request", "per_second":
		if positiveAny(rule["unit_cost"]) || positiveCostMap(rule["unit_cost_by_size"]) ||
			positiveCostMap(rule["unit_cost_by_resolution"]) ||
			positiveCostMap(rule["unit_cost_by_variant"]) {
			return routeCostAvailability{Billable: true, Source: "route_cost"}
		}
	}
	return routeCostAvailability{Source: "missing_route_cost"}
}

func positiveAny(values ...interface{}) bool {
	for _, value := range values {
		if floatValue(value) > 0 {
			return true
		}
	}
	return false
}

func positiveCostMap(value interface{}) bool {
	costs, ok := value.(map[string]interface{})
	if !ok {
		return false
	}
	for _, cost := range costs {
		if floatValue(cost) > 0 {
			return true
		}
	}
	return false
}

func chargeRouteProviderCost(providerCost float64, source string, cfg billing.PricingConfig) (billing.ChargeBreakdown, error) {
	if providerCost < 0 || (providerCost == 0 && source != "complimentary" && source != "local_free" && cfg.MissingCostPolicy == "block") {
		return billing.ChargeBreakdown{}, billing.ErrProviderCostMissing
	}
	return billing.CalculateCharge(providerCost, cfg.GrossMarginPercent)
}

func (s *ModelService) RouteBillingEstimate(
	ctx context.Context,
	model *ModelFull,
	params map[string]interface{},
	promptTokens, outputTokens int,
) (float64, bool, error) {
	if model == nil {
		return 0, false, errors.New("model not found")
	}
	cfg, err := billing.LoadPricingConfig(ctx, s.db)
	if err != nil {
		return 0, false, err
	}
	routes, err := s.RuntimeRoutes(ctx, model)
	if err != nil {
		if strings.Contains(err.Error(), "all model routes are cooling down") {
			return 0, false, err
		}
		return 0, false, err
	}
	if len(routes) == 0 {
		// Legacy rows without model_routes still bill through the model's
		// price_rule. Legacy compatibility is explicit, not silently free.
		if cfg.MissingCostPolicy == "block" {
			return 0, false, fmt.Errorf("%w: %s", billing.ErrProviderCostMissing, model.Code)
		}
		return s.EstimateCost(model, params, promptTokens, outputTokens), false, nil
	}
	complete := true
	var best float64
	hasPricedRoute := false
	for i := range routes {
		route := &routes[i]
		availability := routeCostAvailabilityFor(route)
		if !availability.Billable {
			complete = false
			continue
		}
		providerCost := EstimateRouteProviderCostWithTokenDetails(route, params, promptTokens, outputTokens, 0, 0)
		if availability.Complimentary {
			providerCost = 0
		}
		charge, err := chargeRouteProviderCost(providerCost, availability.Source, cfg)
		if err != nil {
			continue
		}
		hasPricedRoute = true
		if charge.UserCharge > best {
			best = charge.UserCharge
		}
	}
	if !hasPricedRoute {
		return 0, false, fmt.Errorf("%w: %s", billing.ErrProviderCostMissing, model.Code)
	}
	return roundCredits(best), complete, nil
}

func (s *ModelService) RouteProviderCharge(
	ctx context.Context,
	model *ModelFull,
	route *ModelRoute,
	params map[string]interface{},
	promptTokens, outputTokens, cacheReadTokens, cacheWriteTokens int,
) (billing.ChargeBreakdown, bool, billing.PricingConfig, error) {
	cfg, err := billing.LoadPricingConfig(ctx, s.db)
	if err != nil {
		return billing.ChargeBreakdown{}, false, billing.PricingConfig{}, err
	}
	if route == nil || len(route.CostRule) == 0 {
		if cfg.MissingCostPolicy == "block" {
			return billing.ChargeBreakdown{}, false, cfg, billing.ErrProviderCostMissing
		}
		legacy := s.EstimateCostWithTokenDetails(model, params, promptTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
		charge, err := billing.CalculateCharge(legacy, cfg.GrossMarginPercent)
		return charge, false, cfg, err
	}
	availability := routeCostAvailabilityFor(route)
	if !availability.Billable {
		return billing.ChargeBreakdown{}, false, cfg, billing.ErrProviderCostMissing
	}
	providerCost := EstimateRouteProviderCostWithTokenDetails(route, params, promptTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
	if availability.Complimentary {
		providerCost = 0
	}
	charge, err := chargeRouteProviderCost(providerCost, availability.Source, cfg)
	return charge, true, cfg, err
}

func (s *ModelService) EnabledRoutesWithCost(ctx context.Context, model *ModelFull) ([]ModelRoute, error) {
	if model == nil {
		return nil, errors.New("model not found")
	}
	routes, err := s.RuntimeRoutes(ctx, model)
	if err != nil {
		return nil, err
	}
	cfg, err := billing.LoadPricingConfig(ctx, s.db)
	if err != nil {
		return nil, err
	}
	if cfg.MissingCostPolicy == "block" {
		billable := make([]ModelRoute, 0, len(routes))
		for i := range routes {
			if routeCostAvailabilityFor(&routes[i]).Billable {
				billable = append(billable, routes[i])
			}
		}
		if len(billable) == 0 {
			return nil, fmt.Errorf("%w: %s has no enabled route with upstream cost", billing.ErrProviderCostMissing, model.Code)
		}
		routes = billable
	}
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Priority == routes[j].Priority {
			return routes[i].ID < routes[j].ID
		}
		return routes[i].Priority < routes[j].Priority
	})
	return routes, nil
}

func (s *ModelService) BillingPricingConfig(ctx context.Context) (pricing billing.PricingConfig, err error) {
	return billing.LoadPricingConfig(ctx, s.db)
}

func routeIDPointer(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	return &id
}

func modelIDPointer(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	return &id
}

func settlementMetadata(values map[string]interface{}) map[string]interface{} {
	if values == nil {
		return map[string]interface{}{}
	}
	return values
}

func numericMicros(value float64) float64 {
	return math.Ceil(value*1_000_000) / 1_000_000
}
