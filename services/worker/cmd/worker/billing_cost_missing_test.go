package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMarkTaskBillingCostMissingTerminatesRunningTask 回归：
// 上游未返回可核验成本时必须把任务置为 failed。
// 历史缺陷：UPDATE 把 NOT NULL 的 tasks.output 写成 NULL 并丢弃错误，
// 导致任务永久停留在 running，画布节点一直转圈。
func TestMarkTaskBillingCostMissingTerminatesRunningTask(t *testing.T) {
	dsn := os.Getenv("BILLING_COST_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("set BILLING_COST_TEST_DATABASE_URL or DATABASE_URL for isolated SQL regression")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var userID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&userID); err != nil {
		t.Skipf("no user row available: %v", err)
	}
	taskNo := "task_qa_billingcost_" + time.Now().Format("150405.000000")
	if _, err := pool.Exec(ctx, `
		INSERT INTO tasks (task_no, user_id, type, status, input, estimated_cost)
		VALUES ($1,$2,'image','running','{}'::jsonb,0.1)`, taskNo, userID); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tasks WHERE task_no=$1`, taskNo) }()

	if err := markTaskBillingCostMissing(ctx, pool, taskNo, 0.012345, "上游未返回可核验成本，已停止扣费：test"); err != nil {
		t.Fatalf("markTaskBillingCostMissing returned error: %v", err)
	}

	var status, errorCode, errorMessage string
	var providerCostSource string
	var finished bool
	var rawOutput []byte
	if err := pool.QueryRow(ctx, `
		SELECT status, coalesce(error_code,''), coalesce(error_message,''), provider_cost_source,
		       finished_at IS NOT NULL, output
		FROM tasks WHERE task_no=$1`, taskNo).
		Scan(&status, &errorCode, &errorMessage, &providerCostSource, &finished, &rawOutput); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	if errorCode != "BILLING_COST_MISSING" {
		t.Fatalf("error_code = %q", errorCode)
	}
	if errorMessage == "" || !finished {
		t.Fatalf("error_message/finished_at not recorded: %q %v", errorMessage, finished)
	}
	if providerCostSource != "missing_route_cost" {
		t.Fatalf("provider_cost_source = %q", providerCostSource)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(rawOutput, &decoded); err != nil {
		t.Fatalf("tasks.output is not valid JSON: %v (%s)", err, string(rawOutput))
	}
}
