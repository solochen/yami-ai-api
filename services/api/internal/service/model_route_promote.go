package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type RoutePrioritySnapshot struct {
	RouteID   int64                  `json:"route_id"`
	Priority  int                    `json:"priority"`
	IsEnabled bool                   `json:"is_enabled"`
	CostRule  map[string]interface{} `json:"cost_rule,omitempty"`
}

type RouteSwitchResult struct {
	Changed                bool                    `json:"changed"`
	SwitchID               int64                   `json:"switch_id"`
	ModelID                int64                   `json:"model_id"`
	FromRouteID            *int64                  `json:"from_route_id,omitempty"`
	ToRouteID              int64                   `json:"to_route_id"`
	Before                 []RoutePrioritySnapshot `json:"before"`
	After                  []RoutePrioritySnapshot `json:"after"`
	EstimatedDailySaving   float64                 `json:"estimated_daily_saving"`
	EstimatedMonthlySaving float64                 `json:"estimated_monthly_saving"`
}

// PromoteModelRoute 把目标线路设为该模型的主线路。
// 语义：把目标线路排到第一位，其余线路保持原有相对顺序，重新编号为 10,20,30...
// 整个过程在一个事务里锁定该模型的全部启用线路，并写 model_route_switch_logs。
func (s *ModelService) PromoteModelRoute(ctx context.Context, modelID, routeID, adminID int64, reason string, dailySaving, monthlySaving float64) (*RouteSwitchResult, error) {
	if modelID <= 0 || routeID <= 0 {
		return nil, errors.New("模型或线路 ID 无效")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	type row struct {
		id       int64
		priority int
		enabled  bool
		costRule map[string]interface{}
	}
	rows, err := tx.Query(ctx, `SELECT id,priority,is_enabled,cost_rule FROM model_routes
		WHERE model_id=$1 AND is_enabled=true ORDER BY priority,id FOR UPDATE`, modelID)
	if err != nil {
		return nil, err
	}
	var list []row
	for rows.Next() {
		var r row
		var costRaw []byte
		if err := rows.Scan(&r.id, &r.priority, &r.enabled, &costRaw); err != nil {
			rows.Close()
			return nil, err
		}
		_ = json.Unmarshal(costRaw, &r.costRule)
		list = append(list, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errors.New("模型没有启用的线路")
	}
	targetIdx := -1
	for i := range list {
		if list[i].id == routeID {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return nil, errors.New("目标线路不存在或未启用")
	}
	before := make([]RoutePrioritySnapshot, 0, len(list))
	for _, r := range list {
		before = append(before, RoutePrioritySnapshot{RouteID: r.id, Priority: r.priority, IsEnabled: r.enabled, CostRule: r.costRule})
	}
	if targetIdx == 0 {
		return &RouteSwitchResult{Changed: false, ModelID: modelID, ToRouteID: routeID, Before: before, After: before}, nil
	}
	currentPrimaryID := list[0].id
	ordered := make([]row, 0, len(list))
	ordered = append(ordered, list[targetIdx])
	for i, r := range list {
		if i == targetIdx {
			continue
		}
		ordered = append(ordered, r)
	}
	after := make([]RoutePrioritySnapshot, 0, len(ordered))
	for i := range ordered {
		ordered[i].priority = (i + 1) * 10
		after = append(after, RoutePrioritySnapshot{RouteID: ordered[i].id, Priority: ordered[i].priority, IsEnabled: ordered[i].enabled, CostRule: ordered[i].costRule})
		if _, err := tx.Exec(ctx, `UPDATE model_routes SET priority=$2, updated_at=now() WHERE id=$1`, ordered[i].id, ordered[i].priority); err != nil {
			return nil, err
		}
	}
	// 用该线路最新同步价更新成本规则，避免切换后仍按旧成本记账。
	var provider, upstreamModel, priceGroup string
	if err := tx.QueryRow(ctx, `SELECT provider,upstream_model,COALESCE(price_group,'') FROM model_routes WHERE id=$1`, routeID).Scan(&provider, &upstreamModel, &priceGroup); err == nil && priceGroup != "" {
		var bt, sourceCurrency, sourceUnit string
		var inCost, outCost, cacheRead, cacheWrite, unitCost float64
		if err := tx.QueryRow(ctx, `SELECT billing_type,source_currency,source_unit,
			COALESCE(cny_input_per_m,0),COALESCE(cny_output_per_m,0),COALESCE(cny_cache_read_per_m,0),
			COALESCE(cny_cache_write_per_m,0),COALESCE(cny_unit_price,0)
			FROM upstream_price_current WHERE provider=$1 AND upstream_model=$2 AND group_name=$3
			ORDER BY observed_at DESC LIMIT 1`, provider, upstreamModel, priceGroup).Scan(
			&bt, &sourceCurrency, &sourceUnit, &inCost, &outCost, &cacheRead, &cacheWrite, &unitCost); err == nil {
			rule := map[string]interface{}{
				"currency": "CREDITS", "billing_type": bt, "compute_per_cny": 1,
				"source_currency": "CNY", "price_unit_normalized": true,
			}
			if bt == "per_token" {
				rule["input_cost_per_m"] = inCost
				rule["output_cost_per_m"] = outCost
				rule["cache_read_cost_per_m"] = cacheRead
				rule["cache_write_cost_per_m"] = cacheWrite
				rule["source_input_cost_per_m"] = inCost
				rule["source_output_cost_per_m"] = outCost
				rule["source_cache_read_cost_per_m"] = cacheRead
				rule["source_cache_write_cost_per_m"] = cacheWrite
			} else {
				rule["unit_cost"] = unitCost
				rule["source_unit_cost"] = unitCost
			}
			if raw, err := json.Marshal(rule); err == nil {
				_, _ = tx.Exec(ctx, `UPDATE model_routes SET cost_rule=$2,updated_at=now() WHERE id=$1`, routeID, raw)
				for i := range after {
					if after[i].RouteID == routeID {
						after[i].CostRule = rule
					}
				}
			}
		}
	}

	beforeRaw, _ := json.Marshal(before)
	afterRaw, _ := json.Marshal(after)
	if strings.TrimSpace(reason) == "" {
		reason = "manual"
	}
	var switchID int64
	if err := tx.QueryRow(ctx, `INSERT INTO model_route_switch_logs
		(model_id,from_route_id,to_route_id,before_snapshot,after_snapshot,reason,estimated_daily_saving,estimated_monthly_saving,admin_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		modelID, currentPrimaryID, routeID, beforeRaw, afterRaw, reason, dailySaving, monthlySaving, adminID).Scan(&switchID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	// 主线路变更后同步 models 表的冗余连接信息；失败不影响已经提交的线路顺序。
	_ = s.syncModelFromPrimaryRoute(ctx, modelID, nil)
	from := currentPrimaryID
	return &RouteSwitchResult{
		Changed: true, SwitchID: switchID, ModelID: modelID, FromRouteID: &from, ToRouteID: routeID,
		Before: before, After: after, EstimatedDailySaving: dailySaving, EstimatedMonthlySaving: monthlySaving,
	}, nil
}

// RollbackModelRoute 按切换日志恢复切换前的优先级。
// 只允许回滚该模型最新一次未回滚的切换，避免覆盖后续人工操作。
func (s *ModelService) RollbackModelRoute(ctx context.Context, switchID, adminID int64) (*RouteSwitchResult, error) {
	if switchID <= 0 {
		return nil, errors.New("切换记录 ID 无效")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var modelID, toRouteID int64
	var fromRouteID *int64
	var beforeRaw, afterRaw []byte
	var rolledBackAt *time.Time
	var reason string
	if err := tx.QueryRow(ctx, `SELECT model_id,from_route_id,to_route_id,before_snapshot,after_snapshot,rolled_back_at,reason
		FROM model_route_switch_logs WHERE id=$1 FOR UPDATE`, switchID).Scan(&modelID, &fromRouteID, &toRouteID, &beforeRaw, &afterRaw, &rolledBackAt, &reason); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("切换记录不存在")
		}
		return nil, err
	}
	if rolledBackAt != nil {
		return nil, errors.New("该切换已经回滚过")
	}
	var newer int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM model_route_switch_logs
		WHERE model_id=$1 AND id>$2 AND rolled_back_at IS NULL`, modelID, switchID).Scan(&newer); err != nil {
		return nil, err
	}
	if newer > 0 {
		return nil, errors.New("该模型之后还有新的切换，请先回滚最新一次")
	}
	var target []RoutePrioritySnapshot
	if err := json.Unmarshal(beforeRaw, &target); err != nil || len(target) == 0 {
		return nil, errors.New("切换快照无法解析")
	}
	// 当前状态，用于写新的回滚日志。
	currentRows, err := tx.Query(ctx, `SELECT id,priority,is_enabled FROM model_routes WHERE model_id=$1 AND is_enabled=true ORDER BY priority,id FOR UPDATE`, modelID)
	if err != nil {
		return nil, err
	}
	var current []RoutePrioritySnapshot
	for currentRows.Next() {
		var snap RoutePrioritySnapshot
		if err := currentRows.Scan(&snap.RouteID, &snap.Priority, &snap.IsEnabled); err != nil {
			currentRows.Close()
			return nil, err
		}
		current = append(current, snap)
	}
	currentRows.Close()
	if err := currentRows.Err(); err != nil {
		return nil, err
	}
	existing := map[int64]bool{}
	for _, c := range current {
		existing[c.RouteID] = true
	}
	var missing []string
	for _, snap := range target {
		if !existing[snap.RouteID] {
			missing = append(missing, fmt.Sprintf("%d", snap.RouteID))
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("快照中的线路已被删除或停用：%s", strings.Join(missing, ", "))
	}
	for _, snap := range target {
		var costRaw []byte
		if snap.CostRule != nil {
			costRaw, _ = json.Marshal(snap.CostRule)
		}
		if _, err := tx.Exec(ctx, `UPDATE model_routes SET priority=$2,is_enabled=$3,
			cost_rule=COALESCE($5::jsonb,cost_rule),updated_at=now() WHERE id=$1 AND model_id=$4`,
			snap.RouteID, snap.Priority, snap.IsEnabled, modelID, costRaw); err != nil {
			return nil, err
		}
	}
	now := time.Now()
	if _, err := tx.Exec(ctx, `UPDATE model_route_switch_logs SET rolled_back_at=$2 WHERE id=$1`, switchID, now); err != nil {
		return nil, err
	}
	currentRaw, _ := json.Marshal(current)
	targetRaw, _ := json.Marshal(target)
	var newID int64
	if err := tx.QueryRow(ctx, `INSERT INTO model_route_switch_logs
		(model_id,from_route_id,to_route_id,before_snapshot,after_snapshot,reason,rollback_of_id,admin_id)
		VALUES($1,$2,$3,$4,$5,'rollback',$6,$7) RETURNING id`,
		modelID, toRouteID, firstRouteID(target), currentRaw, targetRaw, switchID, adminID).Scan(&newID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = s.syncModelFromPrimaryRoute(ctx, modelID, nil)
	return &RouteSwitchResult{
		Changed: true, SwitchID: newID, ModelID: modelID, ToRouteID: firstRouteID(target),
		Before: current, After: target,
	}, nil
}

func firstRouteID(list []RoutePrioritySnapshot) int64 {
	if len(list) == 0 {
		return 0
	}
	best := list[0]
	for _, item := range list[1:] {
		if item.Priority < best.Priority {
			best = item
		}
	}
	return best.RouteID
}

func (s *AdminService) ListRouteSwitches(ctx context.Context, modelID int64, page, pageSize int) ([]RouteSwitchLog, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where := "1=1"
	args := []interface{}{}
	if modelID > 0 {
		args = append(args, modelID)
		where = "l.model_id=$1"
	}
	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM model_route_switch_logs l WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(ctx, `SELECT l.id,l.model_id,COALESCE(m.code,''),l.from_route_id,l.to_route_id,
		COALESCE(rf.route_name,''),COALESCE(rt.route_name,''),l.reason,l.estimated_daily_saving,l.estimated_monthly_saving,
		l.admin_id,l.created_at,l.rolled_back_at,l.rollback_of_id
		FROM model_route_switch_logs l
		JOIN models m ON m.id=l.model_id
		LEFT JOIN model_routes rf ON rf.id=l.from_route_id
		LEFT JOIN model_routes rt ON rt.id=l.to_route_id
		WHERE `+where+` ORDER BY l.created_at DESC LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []RouteSwitchLog{}
	for rows.Next() {
		var l RouteSwitchLog
		if err := rows.Scan(&l.ID, &l.ModelID, &l.ModelCode, &l.FromRouteID, &l.ToRouteID,
			&l.FromRouteName, &l.ToRouteName, &l.Reason, &l.EstimatedDailySaving, &l.EstimatedMonthlySaving,
			&l.AdminID, &l.CreatedAt, &l.RolledBackAt, &l.RollbackOfID); err != nil {
			return nil, 0, err
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}
