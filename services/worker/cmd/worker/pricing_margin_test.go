package main

import (
	"context"

	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestWorkerPricingConfigAcceptsZeroGrossMargin 回归：
// 后台把 platform_gross_margin_percent 配成 0（平台不加毛利）时，worker 必须按 0 结算。
// 历史缺陷：解析用了「必须大于 0」，0 被当成未配置，于是回落默认 3%；
// 而 API 侧冻结按 0% 估算，造成 estimated_cost < actual_cost，
// 余额刚好落在两者之间时会出现「上游已生成、结算失败、结果被丢弃」。
func TestWorkerPricingConfigAcceptsZeroGrossMargin(t *testing.T) {
	dsn := os.Getenv("BILLING_COST_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("set BILLING_COST_TEST_DATABASE_URL or DATABASE_URL for isolated pricing regression")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var original []byte
	if err := pool.QueryRow(ctx, `SELECT value FROM system_configs WHERE key='platform_gross_margin_percent'`).Scan(&original); err != nil {
		t.Skipf("platform_gross_margin_percent not configured: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `UPDATE system_configs SET value=$1 WHERE key='platform_gross_margin_percent'`, original)
	}()

	if _, err := pool.Exec(ctx, `UPDATE system_configs SET value='0'::jsonb WHERE key='platform_gross_margin_percent'`); err != nil {
		t.Fatal(err)
	}
	cfg := loadWorkerPricingConfig(ctx, pool)
	if cfg.GrossMarginPercent != 0 {
		t.Fatalf("gross margin = %v, want 0 (explicitly configured zero margin must be honoured)", cfg.GrossMarginPercent)
	}

	// 0% 毛利时用户扣费必须等于上游成本，冻结与结算才一致。
	charge, err := workerChargeRouteProviderCost(ctx, pool, 0.1, 0.1, "route_cost", false)
	if err != nil {
		t.Fatalf("charge failed: %v", err)
	}
	if charge.UserCharge != 0.1 {
		t.Fatalf("user charge = %v, want 0.1 with 0%% margin", charge.UserCharge)
	}

	if _, err := pool.Exec(ctx, `UPDATE system_configs SET value='3'::jsonb WHERE key='platform_gross_margin_percent'`); err != nil {
		t.Fatal(err)
	}
	cfg = loadWorkerPricingConfig(ctx, pool)
	if cfg.GrossMarginPercent != 3 {
		t.Fatalf("gross margin = %v, want 3", cfg.GrossMarginPercent)
	}
}
