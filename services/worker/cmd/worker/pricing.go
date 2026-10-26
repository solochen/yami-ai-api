package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultGrossMarginPercent = 3.0
	defaultPaymentComputeRate = 1.0
	defaultUSDToComputeRate   = 7.2
)

var errWorkerProviderCostMissing = errors.New("upstream provider cost is missing")

type workerPricingConfig struct {
	GrossMarginPercent float64
	PaymentComputeRate float64
	USDToComputeRate   float64
	MissingCostPolicy  string
}

type workerChargeBreakdown struct {
	ProviderCost float64
	UserCharge   float64
	Commission   float64
	Config       workerPricingConfig
}

func loadWorkerPricingConfig(ctx context.Context, pool *pgxpool.Pool) workerPricingConfig {
	cfg := workerPricingConfig{
		GrossMarginPercent: defaultGrossMarginPercent,
		PaymentComputeRate: defaultPaymentComputeRate,
		USDToComputeRate:   defaultUSDToComputeRate,
		MissingCostPolicy:  "block",
	}
	if pool == nil {
		return cfg
	}
	rows, err := pool.Query(ctx, `SELECT key,value FROM system_configs WHERE key = ANY($1::text[])`, []string{
		"platform_gross_margin_percent",
		"payment_compute_rate",
		"model_usd_to_compute_rate",
		"missing_cost_policy",
	})
	if err != nil {
		return cfg
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var raw []byte
		if rows.Scan(&key, &raw) != nil {
			continue
		}
		var value interface{}
		if json.Unmarshal(raw, &value) != nil {
			continue
		}
		switch key {
		case "platform_gross_margin_percent":
			if number, ok := workerFiniteNumber(value); ok && number < 100 {
				cfg.GrossMarginPercent = number
			}
		case "payment_compute_rate":
			if number, ok := workerFiniteNumber(value); ok {
				cfg.PaymentComputeRate = number
			}
		case "model_usd_to_compute_rate":
			if number, ok := workerFiniteNumber(value); ok {
				cfg.USDToComputeRate = number
			}
		case "missing_cost_policy":
			if text, ok := value.(string); ok && (text == "block" || text == "legacy") {
				cfg.MissingCostPolicy = text
			}
		}
	}
	return cfg
}

func workerFiniteNumber(value interface{}) (float64, bool) {
	number, ok := value.(float64)
	return number, ok && number > 0 && !math.IsNaN(number) && !math.IsInf(number, 0)
}

func workerCalculateCharge(providerCost, grossMarginPercent float64) (workerChargeBreakdown, error) {
	if math.IsNaN(providerCost) || math.IsInf(providerCost, 0) || providerCost < 0 {
		return workerChargeBreakdown{}, errors.New("invalid provider cost")
	}
	if math.IsNaN(grossMarginPercent) || math.IsInf(grossMarginPercent, 0) ||
		grossMarginPercent < 0 || grossMarginPercent >= 100 {
		return workerChargeBreakdown{}, errors.New("invalid gross margin")
	}
	providerMicros := workerCeilMicrocredits(providerCost)
	userMicros := providerMicros
	if grossMarginPercent > 0 {
		userMicros = int64(math.Ceil(float64(providerMicros) / ((100 - grossMarginPercent) / 100)))
	}
	if userMicros < providerMicros {
		userMicros = providerMicros
	}
	return workerChargeBreakdown{
		ProviderCost: float64(providerMicros) / 1_000_000,
		UserCharge:   float64(userMicros) / 1_000_000,
		Commission:   float64(userMicros-providerMicros) / 1_000_000,
	}, nil
}

func workerChargeForProviderCost(ctx context.Context, pool *pgxpool.Pool, providerCost float64) (workerChargeBreakdown, error) {
	cfg := loadWorkerPricingConfig(ctx, pool)
	charge, err := workerCalculateCharge(providerCost, cfg.GrossMarginPercent)
	if err != nil {
		return workerChargeBreakdown{}, err
	}
	charge.Config = cfg
	return charge, nil
}

func workerRouteCostSource(route workerModelRoute) string {
	if len(route.CostRule) == 0 {
		return "missing_route_cost"
	}
	if value, _ := route.CostRule["complimentary"].(bool); value {
		return "complimentary"
	}
	return "route_cost"
}

func workerProviderCostFromRoute(route workerModelRoute, input map[string]interface{}, promptTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) (float64, bool) {
	switch workerRouteCostSource(route) {
	case "route_cost":
		return workerRouteProviderCost(route, input, promptTokens, outputTokens, cacheReadTokens, cacheWriteTokens), true
	case "complimentary":
		return 0, true
	default:
		return 0, false
	}
}

func workerChargeRouteProviderCost(ctx context.Context, pool *pgxpool.Pool, providerCost float64, source string) (workerChargeBreakdown, error) {
	cfg := loadWorkerPricingConfig(ctx, pool)
	if providerCost < 0 || (providerCost == 0 && source != "complimentary" && source != "local_free" && cfg.MissingCostPolicy == "block") {
		return workerChargeBreakdown{}, errWorkerProviderCostMissing
	}
	charge, err := workerCalculateCharge(providerCost, cfg.GrossMarginPercent)
	if err != nil {
		return workerChargeBreakdown{}, err
	}
	charge.Config = cfg
	return charge, nil
}

type workerSettlement struct {
	ModelID      int64
	RouteID      int64
	RefType      string
	RefID        string
	Sequence     int
	ProviderCost float64
	UserCharge   float64
	Commission   float64
	CostSource   string
	CostComplete bool
	Config       workerPricingConfig
	TxType       string
	Remark       string
	Metadata     map[string]interface{}
	Items        []workerSettlementItem
}

type workerSettlementItem struct {
	ModelID          int64
	RouteID          int64
	Source           string
	RefType          string
	RefID            string
	ProviderCost     float64
	UserCharge       float64
	Commission       float64
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	ImageCount       int
	VideoSeconds     float64
	Metadata         map[string]interface{}
}

type workflowProviderCostItem struct {
	ModelID          int64
	RouteID          int64
	Source           string
	RefID            string
	ProviderCost     float64
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	ImageCount       int
	VideoSeconds     float64
}

type workflowProviderCostSummary struct {
	ProviderCost     float64
	Items            []workflowProviderCostItem
	MissingCostCount int
	IncompleteCost   bool
}

// workflowProviderCosts reads only usage that has a durable,
// route-derived cost. A successful call without verifiable cost marks the
// whole workflow incomplete so the caller can release the reservation rather
// than guessing or silently subsidizing the request.
func workflowProviderCosts(ctx context.Context, pool *pgxpool.Pool, projectID int64) (workflowProviderCostSummary, error) {
	summary := workflowProviderCostSummary{}

	llmRows, err := pool.Query(ctx, `
		SELECT model_id, route_id, status, provider_cost,
		       COALESCE(provider_cost_source,''), provider_cost_complete, request_id
		FROM (
			SELECT DISTINCT ON (request_id)
				model_id, route_id, status, provider_cost,
				provider_cost_source, provider_cost_complete, request_id, id
			FROM model_route_attempts
			WHERE workflow_project_id=$1
			ORDER BY request_id, id DESC
		) latest
		ORDER BY id`, projectID)
	if err != nil {
		return summary, err
	}
	for llmRows.Next() {
		var modelID, routeID *int64
		var status, source, requestID string
		var providerCost float64
		var complete bool
		if err := llmRows.Scan(&modelID, &routeID, &status, &providerCost, &source, &complete, &requestID); err != nil {
			llmRows.Close()
			return summary, err
		}
		if !strings.EqualFold(strings.TrimSpace(status), "success") {
			continue
		}
		if providerCost <= 0 && (strings.EqualFold(source, "complimentary") || strings.EqualFold(source, "local_free")) {
			complete = true
		}
		if !complete || providerCost < 0 {
			summary.MissingCostCount++
			summary.IncompleteCost = true
			continue
		}
		item := workflowProviderCostItem{
			Source:       "llm",
			RefID:        requestID,
			ProviderCost: providerCost,
		}
		if modelID != nil {
			item.ModelID = *modelID
		}
		if routeID != nil {
			item.RouteID = *routeID
		}
		summary.ProviderCost += providerCost
		summary.Items = append(summary.Items, item)
	}
	if err := llmRows.Err(); err != nil {
		llmRows.Close()
		return summary, err
	}
	llmRows.Close()

	mediaRows, err := pool.Query(ctx, `
		SELECT COALESCE(model_id,0),COALESCE(route_id,0),type,status,
		       COALESCE(provider_cost,0),task_no,input,
		       COALESCE(provider_cost_source,''),provider_cost_complete
		FROM tasks
		WHERE workflow_project_id=$1 AND status='succeeded'
		ORDER BY id`, projectID)
	if err != nil {
		return summary, err
	}
	for mediaRows.Next() {
		var modelID, routeID int64
		var taskType, status, taskNo, source string
		var providerCost float64
		var complete bool
		var inputRaw []byte
		if err := mediaRows.Scan(&modelID, &routeID, &taskType, &status, &providerCost, &taskNo, &inputRaw, &source, &complete); err != nil {
			mediaRows.Close()
			return summary, err
		}
		_ = status
		input := map[string]interface{}{}
		_ = json.Unmarshal(inputRaw, &input)
		if providerCost <= 0 && (strings.EqualFold(source, "complimentary") || strings.EqualFold(source, "local_free")) {
			complete = true
		}
		if providerCost < 0 || !complete {
			summary.MissingCostCount++
			summary.IncompleteCost = true
			continue
		}
		summary.ProviderCost += providerCost
		summary.Items = append(summary.Items, workflowProviderCostItem{
			ModelID:      modelID,
			RouteID:      routeID,
			Source:       taskType,
			RefID:        taskNo,
			ProviderCost: providerCost,
			ImageCount:   intAny(input["n"]),
			VideoSeconds: workerDurationSeconds(input),
		})
	}
	if err := mediaRows.Err(); err != nil {
		mediaRows.Close()
		return summary, err
	}
	mediaRows.Close()
	summary.ProviderCost = numericMicrosWorker(summary.ProviderCost)
	return summary, nil
}

func chargeBillingWithSettlement(
	ctx context.Context,
	pool *pgxpool.Pool,
	userID int64,
	freezeAmount float64,
	settlement workerSettlement,
	finalize func(pgx.Tx) error,
) error {
	if settlement.UserCharge < 0 {
		return errors.New("settlement charge cannot be negative")
	}
	settlement.RefType = strings.TrimSpace(settlement.RefType)
	settlement.RefID = strings.TrimSpace(settlement.RefID)
	if settlement.RefType == "" || settlement.RefID == "" {
		return errors.New("settlement reference is required")
	}
	if err := validateWorkerSettlementItems(settlement); err != nil {
		return err
	}
	baseKey := settlement.RefType + ":" + settlement.RefID + ":" + nonEmptyWorker(settlement.TxType, "usage")
	key := baseKey
	if settlement.Sequence > 0 {
		key = fmt.Sprintf("%s:%d", key, settlement.Sequence)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	lockKey := key
	if settlement.RefType == "workflow" {
		lockKey = "workflow-settlement:" + settlement.RefID
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
		return err
	}
	if settlement.RefType == "workflow" && settlement.Sequence <= 0 {
		if err = tx.QueryRow(ctx,
			`SELECT COALESCE(MAX(workflow_sequence),0)+1
			 FROM billing_settlements
			 WHERE ref_type='workflow' AND ref_id=$1 AND status='settled'`,
			settlement.RefID,
		).Scan(&settlement.Sequence); err != nil {
			return err
		}
		key = fmt.Sprintf("%s:%d", baseKey, settlement.Sequence)
	}
	var alreadySettled bool
	if err = tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM billing_settlements WHERE settlement_key=$1 AND status='settled')`,
		key,
	).Scan(&alreadySettled); err != nil {
		return err
	}
	if alreadySettled {
		if finalize != nil {
			if err = finalize(tx); err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	}
	var balance, frozen float64
	if err = tx.QueryRow(ctx,
		`SELECT compute_balance, frozen_compute FROM wallets WHERE user_id=$1 FOR UPDATE`,
		userID,
	).Scan(&balance, &frozen); err != nil {
		return err
	}
	lockedAmount, err := lockedFreezeAmount(ctx, tx, userID, settlement.RefType, settlement.RefID)
	if err != nil {
		return err
	}
	if lockedAmount <= 0 && settlement.UserCharge > 0 {
		return fmt.Errorf("active billing reservation not found for %s/%s", settlement.RefType, settlement.RefID)
	}
	_ = freezeAmount
	newBalance := balance - settlement.UserCharge
	newFrozen := frozen
	if settlement.RefType != "workflow" {
		newFrozen -= lockedAmount
	}
	if newBalance < -1e-6 {
		return errors.New("账户余额不足")
	}
	if newFrozen < 0 {
		newFrozen = 0
	}
	if _, err = tx.Exec(ctx,
		`UPDATE wallets SET compute_balance=$1, frozen_compute=$2, updated_at=now() WHERE user_id=$3`,
		newBalance, newFrozen, userID,
	); err != nil {
		return err
	}
	if settlement.RefType == "workflow" {
		if _, err = tx.Exec(ctx,
			`UPDATE balance_freezes SET amount=GREATEST(amount-$1,0), status=CASE WHEN amount-$1<=0 THEN 'charged' ELSE status END, released_at=CASE WHEN amount-$1<=0 THEN now() ELSE released_at END
			 WHERE user_id=$2 AND ref_type=$3 AND ref_id=$4 AND status='frozen'`,
			settlement.UserCharge, userID, settlement.RefType, settlement.RefID,
		); err != nil {
			return err
		}
	} else {
		if _, err = tx.Exec(ctx,
			`UPDATE balance_freezes SET status='charged', released_at=now()
			 WHERE user_id=$1 AND ref_type=$2 AND ref_id=$3 AND status='frozen'`,
			userID, settlement.RefType, settlement.RefID,
		); err != nil {
			return err
		}
	}
	var walletTxID int64
	if settlement.UserCharge > 0 {
		if err = tx.QueryRow(ctx,
			`INSERT INTO wallet_transactions
			 (user_id,type,direction,amount,balance_after,ref_type,ref_id,remark)
			 VALUES ($1,$2,'out',$3,$4,$5,$6,$7) RETURNING id`,
			userID, nonEmptyWorker(settlement.TxType, "usage"), settlement.UserCharge,
			newBalance, settlement.RefType, settlement.RefID, settlement.Remark,
		).Scan(&walletTxID); err != nil {
			return err
		}
	}
	metadata, _ := json.Marshal(nonNilWorkerMetadata(settlement.Metadata))
	var settlementID int64
	if err = tx.QueryRow(ctx,
		`INSERT INTO billing_settlements
		 (user_id,model_id,route_id,ref_type,ref_id,wallet_transaction_id,
		  provider_cost_compute,user_charge_compute,commission_compute,
		  gross_margin_percent,payment_compute_rate,usd_to_compute_rate,
		  rounding_mode,cost_source,cost_complete,settlement_key,workflow_sequence,metadata)
		 VALUES ($1,NULLIF($2,0),NULLIF($3,0),$4,$5,NULLIF($6,0),$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18::jsonb)
		 RETURNING id`,
		userID, settlement.ModelID, settlement.RouteID, settlement.RefType, settlement.RefID,
		walletTxID, settlement.ProviderCost, settlement.UserCharge, settlement.Commission,
		settlement.Config.GrossMarginPercent, settlement.Config.PaymentComputeRate,
		settlement.Config.USDToComputeRate, "ceil_6dp",
		nonEmptyWorker(settlement.CostSource, "route_cost"), settlement.CostComplete, key,
		settlement.Sequence, metadata,
	).Scan(&settlementID); err != nil {
		return err
	}
	if err = insertWorkerSettlementItems(ctx, tx, settlementID, settlement.Items); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE platform_billing_accounts
		 SET user_charge_compute=user_charge_compute+$1,
		     provider_cost_compute=provider_cost_compute+$2,
		     commission_compute=commission_compute+$3,
		     gross_margin_percent=$4, updated_at=now()
		 WHERE id=1`,
		settlement.UserCharge, settlement.ProviderCost, settlement.Commission,
		settlement.Config.GrossMarginPercent,
	); err != nil {
		return err
	}
	if finalize != nil {
		if err = finalize(tx); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func chargeWorkflowWithSettlement(
	ctx context.Context,
	pool *pgxpool.Pool,
	userID int64,
	publicID string,
	projectID int64,
	freezeAmount float64,
	incrementalCharge float64,
	charge workflowChargeResult,
	remark string,
	finalize func(pgx.Tx) error,
) error {
	var currentActual, currentProvider float64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(actual_cost,0),COALESCE(provider_cost,0) FROM workflow_projects WHERE id=$1`, projectID).Scan(&currentActual, &currentProvider); err != nil {
		return err
	}
	deltaProvider := numericMicrosWorker(charge.ProviderCost - currentProvider)
	deltaCharge := numericMicrosWorker(incrementalCharge)
	if deltaProvider < 0 {
		deltaProvider = 0
	}
	if deltaCharge < 0 {
		deltaCharge = 0
	}
	// The ledger is charged from the aggregate cost. Because both aggregate
	// amounts are independently rounded upward, align the commission with the
	// actual incremental user charge rather than allocating per item again.
	deltaCommission := numericMicrosWorker(deltaCharge - deltaProvider)
	if deltaCommission < 0 {
		deltaCommission = 0
	}
	if deltaCommission > deltaCharge {
		deltaCommission = deltaCharge
	}
	items := []workerSettlementItem{}
	if deltaCharge > 0 || deltaProvider > 0 {
		items = []workerSettlementItem{{
			Source:       "workflow",
			RefType:      "workflow_item",
			RefID:        publicID,
			ProviderCost: deltaProvider,
			UserCharge:   deltaCharge,
			Commission:   deltaCommission,
			Metadata:     map[string]interface{}{"cumulative_user_charge": charge.UserCharge, "cumulative_provider_cost": charge.ProviderCost},
		}}
	}
	settlement := workerSettlement{
		RefType: "workflow", RefID: publicID,
		ProviderCost: deltaProvider, UserCharge: deltaCharge, Commission: deltaCommission,
		CostSource: "workflow_actual", CostComplete: !charge.CostIncomplete,
		Config: charge.Config, TxType: "workflow_usage", Remark: remark,
		Metadata: map[string]interface{}{
			"cumulative_user_charge":   charge.UserCharge,
			"cumulative_provider_cost": charge.ProviderCost,
		},
		Items: items,
	}
	return chargeBillingWithSettlement(ctx, pool, userID, freezeAmount, settlement, finalize)
}

func validateWorkerSettlementItems(settlement workerSettlement) error {
	if len(settlement.Items) == 0 {
		return nil
	}
	provider, userCharge, commission := 0.0, 0.0, 0.0
	for index, item := range settlement.Items {
		if strings.TrimSpace(item.Source) == "" {
			return fmt.Errorf("settlement item %d source is required", index)
		}
		if item.ProviderCost < 0 || item.UserCharge < 0 || item.Commission < 0 {
			return fmt.Errorf("settlement item %d contains a negative amount", index)
		}
		if math.Abs(item.UserCharge-item.ProviderCost-item.Commission) > 0.000001 {
			return fmt.Errorf("settlement item %d is unbalanced", index)
		}
		provider += item.ProviderCost
		userCharge += item.UserCharge
		commission += item.Commission
	}
	if math.Abs(provider-settlement.ProviderCost) > 0.000001 ||
		math.Abs(userCharge-settlement.UserCharge) > 0.000001 ||
		math.Abs(commission-settlement.Commission) > 0.000001 {
		return errors.New("settlement items do not match the aggregate charge")
	}
	return nil
}

func insertWorkerSettlementItems(ctx context.Context, tx pgx.Tx, settlementID int64, items []workerSettlementItem) error {
	for _, item := range items {
		itemType := strings.TrimSpace(item.RefType)
		if itemType == "" {
			itemType = "settlement"
		}
		itemID := strings.TrimSpace(item.RefID)
		if itemID == "" {
			itemID = fmt.Sprintf("%d", settlementID)
		}
		metadata, _ := json.Marshal(nonNilWorkerMetadata(item.Metadata))
		if _, err := tx.Exec(ctx,
			`INSERT INTO billing_settlement_items
			 (settlement_id,model_id,route_id,source,ref_type,ref_id,provider_cost_compute,
			  user_charge_compute,commission_compute,prompt_tokens,completion_tokens,
			  cached_input_tokens,image_count,video_seconds,metadata)
			 VALUES ($1,NULLIF($2,0),NULLIF($3,0),$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::jsonb)`,
			settlementID, item.ModelID, item.RouteID, strings.TrimSpace(item.Source),
			itemType, itemID, item.ProviderCost, item.UserCharge, item.Commission,
			item.PromptTokens, item.CompletionTokens, item.CachedTokens,
			item.ImageCount, item.VideoSeconds, metadata,
		); err != nil {
			return err
		}
	}
	return nil
}

func nonNilWorkerMetadata(value map[string]interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	return value
}

func nonEmptyWorker(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func workerCeilMicrocredits(value float64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(math.Ceil(value*1_000_000 - 1e-9))
}

// workerRouteCostTimeMultiplier 返回线路成本在当前时刻的时段倍数。
//
// 与 API 侧 routeCostTimeMultiplier 同一套约定：cost_rule 里配置
// peak_multiplier（倍数）、peak_ranges（["09:00-12:00", ...]，北京时间）、
// peak_weekdays_only（是否仅工作日，默认 true）。未配置或不在时段内返回 1。
// 容器通常跑在 UTC，这里用固定 +08:00 偏移，避免依赖镜像内的 tzdata。
func workerRouteCostTimeMultiplier(rule map[string]interface{}, now time.Time) float64 {
	if rule == nil {
		return 1
	}
	multiplier := floatAny(rule["peak_multiplier"])
	if multiplier <= 0 || multiplier == 1 {
		return 1
	}
	windows := workerClockRanges(rule["peak_ranges"])
	if len(windows) == 0 {
		return 1
	}
	local := now.In(time.FixedZone("CST", 8*60*60))
	weekdaysOnly := true
	if flag, ok := rule["peak_weekdays_only"].(bool); ok {
		weekdaysOnly = flag
	}
	if weekdaysOnly && (local.Weekday() == time.Saturday || local.Weekday() == time.Sunday) {
		return 1
	}
	minutes := local.Hour()*60 + local.Minute()
	for _, window := range windows {
		if minutes >= window[0] && minutes < window[1] {
			return multiplier
		}
	}
	return 1
}

func workerClockRanges(raw interface{}) [][2]int {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	windows := make([][2]int, 0, len(items))
	for _, item := range items {
		start, end, ok := workerParseClockRange(fmt.Sprint(item))
		if ok {
			windows = append(windows, [2]int{start, end})
		}
	}
	return windows
}

func workerParseClockRange(raw string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, okStart := workerParseClockMinutes(parts[0])
	end, okEnd := workerParseClockMinutes(parts[1])
	if !okStart || !okEnd || end <= start {
		return 0, 0, false
	}
	return start, end, true
}

func workerParseClockMinutes(raw string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || hour < 0 || hour > 24 {
		return 0, false
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}
