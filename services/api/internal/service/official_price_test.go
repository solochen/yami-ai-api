package service

import (
	"math"
	"testing"

	"github.com/starai/api/internal/billing"
)

func TestModelOfficialUserChargeCNY(t *testing.T) {
	model := &ModelFull{ModelDTO: ModelDTO{
		PriceMultiplier: 0.6,
		OfficialPriceRule: map[string]interface{}{
			"currency": "CNY", "billing_type": "per_token",
			"input_price_per_m": 9.0, "output_price_per_m": 27.0, "cache_read_price_per_m": 0.3,
		},
	}}
	cfg := billing.PricingConfig{USDToComputeRate: 7.2}
	charge, ok := modelOfficialUserCharge(model, cfg, 1_000_000, 1_000_000, 1_000_000, 0)
	if !ok {
		t.Fatal("expected official pricing to apply")
	}
	// 0.6 * (9 + 27 + 0.3) = 21.78
	if math.Abs(charge-21.78) > 1e-6 {
		t.Fatalf("charge = %v, want 21.78", charge)
	}
}

func TestModelOfficialUserChargeUSD(t *testing.T) {
	model := &ModelFull{ModelDTO: ModelDTO{
		PriceMultiplier: 0.6,
		OfficialPriceRule: map[string]interface{}{
			"currency": "USD", "billing_type": "per_token",
			"input_price_per_m": 2.0, "output_price_per_m": 10.0,
		},
	}}
	cfg := billing.PricingConfig{USDToComputeRate: 7.2}
	charge, ok := modelOfficialUserCharge(model, cfg, 1_000_000, 1_000_000, 0, 0)
	if !ok {
		t.Fatal("expected official pricing to apply")
	}
	// 0.6 * (2*7.2 + 10*7.2) = 0.6 * 86.4 = 51.84
	if math.Abs(charge-51.84) > 1e-6 {
		t.Fatalf("charge = %v, want 51.84", charge)
	}
}

func TestModelOfficialUserChargeDisabled(t *testing.T) {
	model := &ModelFull{ModelDTO: ModelDTO{
		PriceMultiplier: 0,
		OfficialPriceRule: map[string]interface{}{
			"currency": "CNY", "input_price_per_m": 9.0,
		},
	}}
	if _, ok := modelOfficialUserCharge(model, billing.PricingConfig{USDToComputeRate: 7.2}, 100, 100, 0, 0); ok {
		t.Fatal("multiplier 0 should not apply official pricing")
	}
}
