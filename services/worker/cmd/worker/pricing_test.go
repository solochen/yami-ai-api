package main

import (
	"testing"
	"time"
)

func TestWorkerCalculateChargeUsesGrossMargin(t *testing.T) {
	got, err := workerCalculateCharge(970, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderCost != 970 || got.UserCharge != 1000 || got.Commission != 30 {
		t.Fatalf("breakdown = %#v", got)
	}
}

func TestWorkerFinalPriceCanRecordLossWithoutChangingUserCharge(t *testing.T) {
	charge, err := workerChargeRouteProviderCost(nil, nil, 0.324, 0.30, "route_cost", true)
	if err != nil {
		t.Fatal(err)
	}
	if charge.UserCharge != 0.30 || charge.Commission >= 0 {
		t.Fatalf("user charge or loss commission incorrect: %#v", charge)
	}
}

func TestWorkerSettlementPreservesNegativeOfficialPriceCommission(t *testing.T) {
	providerCost := 0.387138
	userCharge := 0.292979
	commission := signedMicrosWorker(userCharge - providerCost)
	if commission != -0.094159 {
		t.Fatalf("loss commission = %.6f, want -0.094159", commission)
	}

	settlement := workerSettlement{
		ProviderCost: providerCost,
		UserCharge:   userCharge,
		Commission:   commission,
		Items: []workerSettlementItem{{
			Source:       "workflow",
			ProviderCost: providerCost,
			UserCharge:   userCharge,
			Commission:   commission,
		}},
	}
	if err := validateWorkerSettlementItems(settlement); err != nil {
		t.Fatalf("official-price settlement rejected: %v", err)
	}
}

func TestWorkerCalculateChargeRoundsUp(t *testing.T) {
	got, err := workerCalculateCharge(0.000001, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserCharge != 0.000002 || got.Commission != 0.000001 {
		t.Fatalf("breakdown = %#v", got)
	}
}

func TestWorkerRouteUserPriceUsesOfficialOpenAIImageTokenRates(t *testing.T) {
	rule := map[string]interface{}{
		"billing_type": "per_token", "input_cost_per_m": float64(54),
		"text_input_cost_per_m": float64(33.75), "image_input_cost_per_m": float64(54),
		"output_cost_per_m": float64(202.5), "cache_read_cost_per_m": float64(13.5),
		"text_cache_read_cost_per_m": float64(8.4375),
	}
	got := workerRouteProviderCostRule(rule, map[string]interface{}{
		"_estimated_input_tokens": float64(1000), "_estimated_output_tokens": float64(1367),
		"_actual_image_input_tokens": float64(400),
	}, 1000, 1367, 0, 0)
	want := (600*33.75 + 400*54 + 1367*202.5) / 1_000_000
	if got != want {
		t.Fatalf("official image user price = %.10f, want %.10f", got, want)
	}
}

func TestWorkerOfficialImageOutputEstimateUsesQualityShape(t *testing.T) {
	rule := map[string]interface{}{
		"default_quality":                    "medium",
		"estimated_output_tokens_by_variant": map[string]interface{}{"medium_1k_portrait": float64(1367)},
	}
	if got := workerEstimatedImageOutputTokens(rule, map[string]interface{}{"image_size": "1K", "aspect_ratio": "3:4"}); got != 1367 {
		t.Fatalf("estimated output tokens = %d, want 1367", got)
	}
}

func TestWorkerImageOutputEstimateScalesWithRequestedImageCount(t *testing.T) {
	rule := map[string]interface{}{
		"default_quality": "medium", "image_output_token_estimator": "gpt_image_2_5",
	}
	params := map[string]interface{}{
		"count": float64(3), "image_size": "1K", "aspect_ratio": "3:4",
	}
	if got := workerEstimatedImageOutputTokensForRequest(rule, params); got != 1029 {
		t.Fatalf("three-image output token estimate = %d, want 1029", got)
	}
}

func TestWorkerOfficialImageOutputEstimateUsesModelSpecificFormula(t *testing.T) {
	rule := map[string]interface{}{
		"default_quality": "medium", "image_output_token_estimator": "gpt_image_2_5",
	}
	if got := workerEstimatedImageOutputTokens(rule, map[string]interface{}{"image_size": "1K", "aspect_ratio": "1:1"}); got != 439 {
		t.Fatalf("GPT Image 2.5 square medium estimate = %d, want 439", got)
	}
	if got := workerEstimatedImageOutputTokens(rule, map[string]interface{}{"image_size": "1K", "aspect_ratio": "3:4"}); got != 343 {
		t.Fatalf("GPT Image 2.5 portrait medium estimate = %d, want 343", got)
	}
	if got := workerEstimatedImageOutputTokens(rule, map[string]interface{}{"image_size": "1K", "aspect_ratio": "1:1", "quality": "xhigh"}); got != 3122 {
		t.Fatalf("GPT Image 2.5 square xhigh estimate = %d, want 3122", got)
	}
	if got := workerEstimatedImageOutputTokens(rule, map[string]interface{}{"image_size": "1K", "aspect_ratio": "1:1", "quality": "max"}); got != 7024 {
		t.Fatalf("GPT Image 2.5 square max estimate = %d, want 7024", got)
	}
	rule["image_output_token_estimator"] = "gpt_image_2"
	if got := workerEstimatedImageOutputTokens(rule, map[string]interface{}{"image_size": "1K", "aspect_ratio": "1:1"}); got != 1756 {
		t.Fatalf("GPT Image 2 square medium estimate = %d, want 1756", got)
	}
}

func TestWorkerWorkflowImageUsageDoesNotDoubleCountImageTokensAsText(t *testing.T) {
	rule := map[string]interface{}{
		"billing_type": "per_token", "output_cost_per_m": float64(202.5),
		"image_output_cost_per_m": float64(202.5), "text_output_cost_per_m": float64(67.5),
	}
	params := map[string]interface{}{
		"_billing_item_count": float64(1), "_actual_image_output_tokens": float64(100),
		"_actual_image_output_tokens_included": true,
	}
	got := estimatePriceRuleCostWorker(rule, params, 0, 150, 0, 0)
	want := (100*202.5 + 50*67.5) / 1_000_000
	delta := got - want
	if delta < 0 {
		delta = -delta
	}
	if delta > 1e-12 {
		t.Fatalf("workflow image usage charge = %.10f, want %.10f", got, want)
	}
}

func TestWorkerRouteCostTimeMultiplier(t *testing.T) {
	rule := map[string]interface{}{
		"peak_multiplier":    2.0,
		"peak_ranges":        []interface{}{"09:00-12:00", "14:00-18:00"},
		"peak_weekdays_only": true,
	}
	beijing := time.FixedZone("CST", 8*60*60)
	// 2026-10-26 是周一。
	monday := func(hour, minute int) time.Time {
		return time.Date(2026, 10, 26, hour, minute, 0, 0, beijing)
	}
	if got := workerRouteCostTimeMultiplier(rule, monday(10, 0)); got != 2 {
		t.Fatalf("weekday peak = %v, want 2", got)
	}
	if got := workerRouteCostTimeMultiplier(rule, monday(12, 30)); got != 1 {
		t.Fatalf("weekday off-peak = %v, want 1", got)
	}
	if got := workerRouteCostTimeMultiplier(rule, time.Date(2026, 10, 31, 10, 0, 0, 0, beijing)); got != 1 {
		t.Fatalf("weekend = %v, want 1", got)
	}
	// UTC 01:00 = 北京 09:00，峰时。
	if got := workerRouteCostTimeMultiplier(rule, time.Date(2026, 10, 26, 1, 0, 0, 0, time.UTC)); got != 2 {
		t.Fatalf("utc conversion = %v, want 2", got)
	}
	if got := workerRouteCostTimeMultiplier(nil, monday(10, 0)); got != 1 {
		t.Fatalf("nil rule = %v, want 1", got)
	}
}
