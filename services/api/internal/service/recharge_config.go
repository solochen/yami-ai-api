package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultPaymentComputeRate = 1.0
	DefaultPaymentCurrency    = "CNY"
	DefaultRechargeShopURL    = "https://catfk.com/shop/yardai"
)

func paymentComputeRate(ctx context.Context, db *pgxpool.Pool) float64 {
	var raw []byte
	if err := db.QueryRow(ctx, `SELECT value FROM system_configs WHERE key='payment_compute_rate'`).Scan(&raw); err != nil {
		return DefaultPaymentComputeRate
	}
	var rate float64
	if err := json.Unmarshal(raw, &rate); err != nil || !validPositiveNumber(rate) {
		return DefaultPaymentComputeRate
	}
	return rate
}

func validPositiveNumber(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func configNumber(value interface{}) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, validPositiveNumber(number)
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil && validPositiveNumber(parsed)
	default:
		return 0, false
	}
}

func ValidateRechargeConfig(values map[string]interface{}) error {
	if rate, ok := configNumber(values["payment_compute_rate"]); !ok || rate > 1_000_000_000 {
		return errors.New("人民币充值倍率必须是大于 0 且不超过 1000000000 的数字")
	}

	enabled := parseConfigBool(values["recharge_shop_enabled"], false)
	rawURL, _ := values["recharge_shop_url"].(string)
	rawURL = strings.TrimSpace(rawURL)
	if !enabled && rawURL == "" {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "catfk.com") || parsed.User != nil {
		return errors.New("卡密店铺地址必须是 https://catfk.com 下的完整地址")
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return errors.New("卡密店铺地址不能使用非标准端口")
	}
	return nil
}
