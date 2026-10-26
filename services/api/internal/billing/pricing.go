package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultGrossMarginPercent = 3.0
	DefaultUSDToComputeRate   = 7.2
	DefaultRoundingMode       = "ceil_6dp"
)

var ErrProviderCostMissing = errors.New("上游成本缺失，已停止计费")

// PricingConfig is a point-in-time snapshot used by a single settlement.
// Persisting these values with the settlement keeps historical books stable
// when administrators later change the platform margin.
type PricingConfig struct {
	GrossMarginPercent float64
	PaymentComputeRate float64
	USDToComputeRate   float64
	MissingCostPolicy  string
	RoundingMode       string
}

type ChargeBreakdown struct {
	ProviderCost float64
	UserCharge   float64
	Commission   float64
}

func LoadPricingConfig(ctx context.Context, db *pgxpool.Pool) (PricingConfig, error) {
	cfg := PricingConfig{
		GrossMarginPercent: DefaultGrossMarginPercent,
		PaymentComputeRate: 1,
		USDToComputeRate:   DefaultUSDToComputeRate,
		MissingCostPolicy:  "block",
		RoundingMode:       DefaultRoundingMode,
	}
	if db == nil {
		return cfg, nil
	}
	rows, err := db.Query(ctx, `SELECT key,value FROM system_configs WHERE key = ANY($1::text[])`, []string{
		"platform_gross_margin_percent",
		"payment_compute_rate",
		"model_usd_to_compute_rate",
		"missing_cost_policy",
		"billing_rounding_mode",
	})
	if err != nil {
		return PricingConfig{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var raw []byte
		if err := rows.Scan(&key, &raw); err != nil {
			return PricingConfig{}, err
		}
		var value interface{}
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		switch key {
		case "platform_gross_margin_percent":
			if number, ok := nonNegativeFinite(value); ok && number < 100 {
				cfg.GrossMarginPercent = number
			}
		case "payment_compute_rate":
			if number, ok := positiveFinite(value); ok {
				cfg.PaymentComputeRate = number
			}
		case "model_usd_to_compute_rate":
			if number, ok := positiveFinite(value); ok {
				cfg.USDToComputeRate = number
			}
		case "missing_cost_policy":
			if text, ok := value.(string); ok && (text == "block" || text == "legacy") {
				cfg.MissingCostPolicy = text
			}
		case "billing_rounding_mode":
			if text, ok := value.(string); ok && text == DefaultRoundingMode {
				cfg.RoundingMode = text
			}
		}
	}
	if err := rows.Err(); err != nil {
		return PricingConfig{}, err
	}
	return cfg, nil
}

func nonNegativeFinite(value interface{}) (float64, bool) {
	number, ok := value.(float64)
	return number, ok && number >= 0 && !math.IsNaN(number) && !math.IsInf(number, 0)
}

func positiveFinite(value interface{}) (float64, bool) {
	number, ok := value.(float64)
	return number, ok && number > 0 && !math.IsNaN(number) && !math.IsInf(number, 0)
}

// CalculateCharge applies the platform's gross-margin definition:
//
//	provider cost / (1 - gross margin)
//
// For example, a 3% gross margin turns 970 credits of provider cost into
// exactly 1,000 user credits. Amounts are held to six decimal places and
// rounded upward so floating-point loss can never consume the configured
// margin.
func CalculateCharge(providerCost float64, grossMarginPercent float64) (ChargeBreakdown, error) {
	if math.IsNaN(providerCost) || math.IsInf(providerCost, 0) || providerCost < 0 {
		return ChargeBreakdown{}, fmt.Errorf("invalid provider cost: %v", providerCost)
	}
	if math.IsNaN(grossMarginPercent) || math.IsInf(grossMarginPercent, 0) ||
		grossMarginPercent < 0 || grossMarginPercent >= 100 {
		return ChargeBreakdown{}, fmt.Errorf("invalid gross margin: %v", grossMarginPercent)
	}
	providerMicros := ceilMicrocredits(providerCost)
	userMicros := providerMicros
	if grossMarginPercent > 0 {
		denominator := (100 - grossMarginPercent) / 100
		userMicros = int64(math.Ceil(float64(providerMicros) / denominator))
	}
	if userMicros < providerMicros {
		userMicros = providerMicros
	}
	commissionMicros := userMicros - providerMicros
	return ChargeBreakdown{
		ProviderCost: float64(providerMicros) / 1_000_000,
		UserCharge:   float64(userMicros) / 1_000_000,
		Commission:   float64(commissionMicros) / 1_000_000,
	}, nil
}

func ceilMicrocredits(value float64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(math.Ceil(value*1_000_000 - 1e-9))
}
