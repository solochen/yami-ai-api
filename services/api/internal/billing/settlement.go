package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Settlement describes the immutable accounting snapshot written together
// with a wallet debit. RefType/RefID match the wallet transaction reference.
type Settlement struct {
	UserID       int64
	ModelID      *int64
	RouteID      *int64
	RefType      string
	RefID        string
	ExternalRef  bool
	ProviderCost float64
	UserCharge   float64
	Commission   float64
	CostSource   string
	CostComplete bool
	Metadata     map[string]interface{}
	Items        []SettlementItem
	Key          string
	Config       PricingConfig
}

// SettlementItem is one independently priced model or media action inside a
// compound settlement. The aggregate settlement remains balanced as well, so
// callers can reconcile a request without knowing how it was decomposed.
type SettlementItem struct {
	ModelID          *int64
	RouteID          *int64
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

// ChargeWithSettlement performs wallet debit, freeze closure, wallet ledger
// insert, settlement snapshot and caller finalization in one transaction.
func (s *Service) ChargeWithSettlement(
	ctx context.Context,
	userID int64,
	freezeAmount float64,
	settlement Settlement,
	txType string,
	remark string,
	finalize func(pgx.Tx) error,
) error {
	settlement.UserID = userID
	settlement.RefType = strings.TrimSpace(settlement.RefType)
	settlement.RefID = strings.TrimSpace(settlement.RefID)
	if settlement.RefType == "" || settlement.RefID == "" {
		return errors.New("settlement reference is required")
	}
	if settlement.Key == "" {
		settlement.Key = settlement.RefType + ":" + settlement.RefID + ":" + txType
	}
	if err := validateSettlementItems(settlement); err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Settlement keys are used by retrying request handlers. Workflow
	// settlements also need one lock across all deferred segments so sequence
	// allocation and balance updates are serialized for the whole workflow.
	lockKey := settlement.Key
	if settlement.RefType == "workflow" {
		lockKey = "workflow-settlement:" + settlement.RefID
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
		return err
	}
	var alreadySettled bool
	if err = tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM billing_settlements WHERE settlement_key=$1 AND status='settled')`,
		settlement.Key,
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
	lockedAmount, err := sumLockedFreezes(ctx, tx, userID, settlement.RefType, settlement.RefID)
	if err != nil {
		return err
	}
	if lockedAmount <= 0 && settlement.UserCharge > 0 {
		return ErrFreezeNotFound
	}
	_ = freezeAmount
	charge := settlement.UserCharge
	if charge < 0 {
		return errors.New("settlement charge cannot be negative")
	}
	newFrozen := frozen
	if settlement.RefType != "workflow" {
		newFrozen -= lockedAmount
	}
	if newFrozen < 0 {
		newFrozen = 0
	}
	newBalance := balance - charge
	if newBalance < -1e-6 {
		return ErrInsufficientBalance
	}
	if _, err = tx.Exec(ctx,
		`UPDATE wallets SET compute_balance=$1, frozen_compute=$2, updated_at=now() WHERE user_id=$3`,
		newBalance, newFrozen, userID,
	); err != nil {
		return err
	}
	if !settlement.ExternalRef && settlement.RefType == "workflow" {
		if _, err = tx.Exec(ctx,
			`UPDATE balance_freezes SET amount=GREATEST(amount-$1,0), status=CASE WHEN amount-$1<=0 THEN 'charged' ELSE status END, released_at=CASE WHEN amount-$1<=0 THEN now() ELSE released_at END
			 WHERE user_id=$2 AND ref_type=$3 AND ref_id=$4 AND status='frozen'`,
			charge, userID, settlement.RefType, settlement.RefID,
		); err != nil {
			return err
		}
	} else if !settlement.ExternalRef {
		if _, err = tx.Exec(ctx,
			`UPDATE balance_freezes SET status='charged', released_at=now()
				 WHERE user_id=$1 AND ref_type=$2 AND ref_id=$3 AND status='frozen'`,
			userID, settlement.RefType, settlement.RefID,
		); err != nil {
			return err
		}
	}

	var walletTxID int64
	if charge > 0 {
		if err = tx.QueryRow(ctx,
			`INSERT INTO wallet_transactions
			 (user_id,type,direction,amount,balance_after,ref_type,ref_id,remark)
			 VALUES ($1,$2,'out',$3,$4,$5,$6,$7) RETURNING id`,
			userID, txType, charge, newBalance, settlement.RefType, settlement.RefID, remark,
		).Scan(&walletTxID); err != nil {
			return err
		}
	}

	metadata, _ := json.Marshal(nonNilMetadata(settlement.Metadata))
	var settlementID int64
	if err = tx.QueryRow(ctx,
		`INSERT INTO billing_settlements
		 (user_id,model_id,route_id,ref_type,ref_id,wallet_transaction_id,
		  provider_cost_compute,user_charge_compute,commission_compute,
		  gross_margin_percent,payment_compute_rate,usd_to_compute_rate,
		  rounding_mode,cost_source,cost_complete,settlement_key,metadata)
		 VALUES ($1,$2,$3,$4,$5,NULLIF($6,0),$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17::jsonb)
		 RETURNING id`,
		userID, settlement.ModelID, settlement.RouteID, settlement.RefType, settlement.RefID,
		walletTxID, settlement.ProviderCost, charge, settlement.Commission,
		settlement.Config.GrossMarginPercent, settlement.Config.PaymentComputeRate,
		settlement.Config.USDToComputeRate, settlement.Config.RoundingMode,
		nonEmpty(settlement.CostSource, "route_cost"), settlement.CostComplete,
		settlement.Key, metadata,
	).Scan(&settlementID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "billing_settlements_settlement_key_key" {
			return tx.Rollback(ctx)
		}
		return err
	}
	if err = insertSettlementItems(ctx, tx, settlementID, settlement.Items); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE platform_billing_accounts
		 SET user_charge_compute=user_charge_compute+$1,
		     provider_cost_compute=provider_cost_compute+$2,
		     commission_compute=commission_compute+$3,
		     gross_margin_percent=$4, updated_at=now()
		 WHERE id=1`,
		charge, settlement.ProviderCost, settlement.Commission, settlement.Config.GrossMarginPercent,
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

func validateSettlementItems(settlement Settlement) error {
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

func insertSettlementItems(ctx context.Context, tx pgx.Tx, settlementID int64, items []SettlementItem) error {
	for _, item := range items {
		metadata, _ := json.Marshal(nonNilMetadata(item.Metadata))
		itemType := strings.TrimSpace(item.RefType)
		if itemType == "" {
			itemType = "settlement"
		}
		itemID := strings.TrimSpace(item.RefID)
		if itemID == "" {
			itemID = fmt.Sprintf("%d", settlementID)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO billing_settlement_items
			 (settlement_id,model_id,route_id,source,ref_type,ref_id,provider_cost_compute,
			  user_charge_compute,commission_compute,prompt_tokens,completion_tokens,
			  cached_input_tokens,image_count,video_seconds,metadata)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::jsonb)`,
			settlementID, item.ModelID, item.RouteID, strings.TrimSpace(item.Source),
			itemType, itemID,
			item.ProviderCost, item.UserCharge, item.Commission,
			item.PromptTokens, item.CompletionTokens, item.CachedTokens,
			item.ImageCount, item.VideoSeconds, metadata,
		); err != nil {
			return err
		}
	}
	return nil
}

func nonNilMetadata(value map[string]interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	return value
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// FinancialSummary is the durable source for the admin finance dashboard.
type FinancialSummary struct {
	GrossMarginPercent     float64 `json:"gross_margin_percent"`
	UserChargeCompute      float64 `json:"user_charge_compute"`
	ProviderCostCompute    float64 `json:"provider_cost_compute"`
	CommissionCompute      float64 `json:"commission_compute"`
	RealizedMarginRate     float64 `json:"realized_margin_rate"`
	SettlementCount        int64   `json:"settlement_count"`
	IncompleteCostCount    int64   `json:"incomplete_cost_count"`
	WalletBalanceCompute   float64 `json:"wallet_balance_compute"`
	FrozenCompute          float64 `json:"frozen_compute"`
	UnfulfilledLiability   float64 `json:"unfulfilled_liability"`
	RechargeCashPaid       float64 `json:"recharge_cash_paid"`
	CashRechargeCredits    float64 `json:"cash_recharge_credits"`
	CardRechargeCredits    float64 `json:"card_recharge_credits"`
	CardFaceValueCNY       float64 `json:"card_face_value_cny"`
	MissingRouteCostModels int64   `json:"missing_route_cost_models"`
}

// WorkflowCharge is the authoritative charge for a workflow project. It is
// derived only from successful LLM attempts and media tasks whose upstream
// cost is complete.
type WorkflowCharge struct {
	ProviderCost float64
	UserCharge   float64
	Commission   float64
	CostComplete bool
	MissingCosts int
	Config       PricingConfig
}

func (s *Service) WorkflowCharge(ctx context.Context, projectID int64) (*WorkflowCharge, error) {
	cfg, err := LoadPricingConfig(ctx, s.db)
	if err != nil {
		return nil, err
	}
	var provider float64
	var missing int64
	if err := s.db.QueryRow(ctx, `
		WITH llm AS (
			SELECT DISTINCT ON (request_id) provider_cost,
			       COALESCE(provider_cost_source,'') AS provider_cost_source,
			       provider_cost_complete
			FROM model_route_attempts
			WHERE workflow_project_id=$1
			  AND status IN ('SUCCESS','success')
			ORDER BY request_id,id DESC
		), media AS (
			SELECT COALESCE(provider_cost,0) AS provider_cost,
			       COALESCE(provider_cost_source,'') AS provider_cost_source,
			       provider_cost_complete
			FROM tasks
			WHERE workflow_project_id=$1 AND status='succeeded'
		)
		SELECT
		  COALESCE((SELECT SUM(provider_cost) FROM llm
		            WHERE provider_cost_complete
		               OR provider_cost_source IN ('complimentary','local_free')),0)
		  + COALESCE((SELECT SUM(provider_cost) FROM media
		              WHERE provider_cost_complete
		                 OR provider_cost_source IN ('complimentary','local_free')),0),
		  (SELECT COUNT(*) FROM llm
		   WHERE NOT provider_cost_complete
		     AND provider_cost_source NOT IN ('complimentary','local_free'))
		  + (SELECT COUNT(*) FROM media
		     WHERE NOT provider_cost_complete
		       AND provider_cost_source NOT IN ('complimentary','local_free'))
		  + (SELECT COUNT(*) FROM llm
		     WHERE provider_cost_complete
		       AND provider_cost_source NOT IN ('complimentary','local_free','route_cost'))
		  + (SELECT COUNT(*) FROM media
		     WHERE provider_cost_complete
		       AND provider_cost_source NOT IN ('complimentary','local_free','route_cost'))`, projectID,
	).Scan(&provider, &missing); err != nil {
		return nil, err
	}
	if missing > 0 {
		return &WorkflowCharge{ProviderCost: provider, CostComplete: false, MissingCosts: int(missing), Config: cfg}, ErrProviderCostMissing
	}
	charge, err := CalculateCharge(provider, cfg.GrossMarginPercent)
	if err != nil {
		return nil, err
	}
	return &WorkflowCharge{
		ProviderCost: charge.ProviderCost,
		UserCharge:   charge.UserCharge,
		Commission:   charge.Commission,
		CostComplete: true,
		Config:       cfg,
	}, nil
}

func (s *Service) FinancialSummary(ctx context.Context) (*FinancialSummary, error) {
	var out FinancialSummary
	if err := s.db.QueryRow(ctx,
		`SELECT gross_margin_percent,user_charge_compute,provider_cost_compute,commission_compute
		 FROM platform_billing_accounts WHERE id=1`,
	).Scan(&out.GrossMarginPercent, &out.UserChargeCompute, &out.ProviderCostCompute, &out.CommissionCompute); err != nil {
		return nil, err
	}
	if out.UserChargeCompute > 0 {
		out.RealizedMarginRate = out.CommissionCompute / out.UserChargeCompute
	}
	_ = s.db.QueryRow(ctx,
		`SELECT COUNT(*),COUNT(*) FILTER (WHERE NOT cost_complete) FROM billing_settlements WHERE status='settled'`,
	).Scan(&out.SettlementCount, &out.IncompleteCostCount)
	_ = s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(compute_balance),0),COALESCE(SUM(frozen_compute),0) FROM wallets`,
	).Scan(&out.WalletBalanceCompute, &out.FrozenCompute)
	// Frozen compute is already part of compute_balance in this wallet model.
	// Adding it would count the same customer liability twice.
	out.UnfulfilledLiability = out.WalletBalanceCompute
	_ = s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM orders WHERE status='paid'`,
	).Scan(&out.RechargeCashPaid)
	_ = s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM wallet_transactions
		 WHERE direction='in' AND type='online_recharge'`,
	).Scan(&out.CashRechargeCredits)
	_ = s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM wallet_transactions
		 WHERE direction='in' AND type='card_recharge'`,
	).Scan(&out.CardRechargeCredits)
	_ = s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(face_value_cny),0) FROM recharge_cards WHERE status='used'`,
	).Scan(&out.CardFaceValueCNY)
	_ = s.db.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM models m
		 WHERE m.is_enabled=true
		   AND EXISTS (
		     SELECT 1 FROM model_routes r
		     WHERE r.model_id=m.id AND r.is_enabled=true
		   )
		   AND NOT EXISTS (
		     SELECT 1 FROM model_routes r
		     WHERE r.model_id=m.id AND r.is_enabled=true
		       AND (
		         COALESCE((r.cost_rule->>'complimentary')::boolean,false)=true
		         OR (
		           r.cost_rule IS NOT NULL
		           AND r.cost_rule<>'{}'::jsonb
		           AND (
		             COALESCE((r.cost_rule->>'input_cost_per_m')::numeric,0)>0
		             OR COALESCE((r.cost_rule->>'output_cost_per_m')::numeric,0)>0
		             OR COALESCE((r.cost_rule->>'cache_read_cost_per_m')::numeric,0)>0
		             OR COALESCE((r.cost_rule->>'cache_write_cost_per_m')::numeric,0)>0
		             OR COALESCE((r.cost_rule->>'audio_input_cost_per_m')::numeric,0)>0
		             OR COALESCE((r.cost_rule->>'audio_output_cost_per_m')::numeric,0)>0
		             OR COALESCE((r.cost_rule->>'unit_cost')::numeric,0)>0
		             OR EXISTS (
		               SELECT 1 FROM jsonb_each_text(COALESCE(r.cost_rule->'unit_cost_by_size','{}'::jsonb)) x
		               WHERE COALESCE(x.value::numeric,0)>0
		             )
		             OR EXISTS (
		               SELECT 1 FROM jsonb_each_text(COALESCE(r.cost_rule->'unit_cost_by_resolution','{}'::jsonb)) x
		               WHERE COALESCE(x.value::numeric,0)>0
		             )
		             OR EXISTS (
		               SELECT 1 FROM jsonb_each_text(COALESCE(r.cost_rule->'unit_cost_by_variant','{}'::jsonb)) x
		               WHERE COALESCE(x.value::numeric,0)>0
		             )
		           )
		         )
		       )
		   )`,
	).Scan(&out.MissingRouteCostModels)
	return &out, nil
}
