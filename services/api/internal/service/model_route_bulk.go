package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/starai/api/internal/util"
)

// ModelRouteListItem 是「线路批量管理」里的一行线路（含账号信息）。
type ModelRouteListItem struct {
	RouteID       int64  `json:"route_id"`
	ModelID       int64  `json:"model_id"`
	ModelCode     string `json:"model_code"`
	ModelName     string `json:"model_name"`
	Category      string `json:"category"`
	RouteName     string `json:"route_name"`
	Provider      string `json:"provider"`
	UpstreamModel string `json:"upstream_model"`
	PriceGroup    string `json:"price_group"`
	Priority      int    `json:"priority"`
	IsEnabled     bool   `json:"is_enabled"`
	HealthStatus  string `json:"health_status"`
	AccountID     *int64 `json:"account_id,omitempty"`
	AccountName   string `json:"account_name,omitempty"`
	AccountGroup  string `json:"account_group,omitempty"`
	LoginAccount  string `json:"login_account,omitempty"`
}

type ModelRouteListFilter struct {
	Provider  string
	Keyword   string
	AccountID string // "" 全部；"unbound" 未绑定；数字 指定账号
	Enabled   string // "" 全部；"enabled" / "disabled"
	ModelID   int64
	Page      int
	PageSize  int
}

// ListModelRoutesForAdmin 返回全局线路列表（分页 + 筛选），供批量管理页使用。
func (s *ModelService) ListModelRoutesForAdmin(ctx context.Context, f ModelRouteListFilter) ([]ModelRouteListItem, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 50
	}
	where := []string{"1=1"}
	args := []interface{}{}
	if provider := strings.TrimSpace(f.Provider); provider != "" {
		args = append(args, provider)
		where = append(where, fmt.Sprintf("r.provider = $%d", len(args)))
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		n := len(args)
		where = append(where, fmt.Sprintf("(m.code ILIKE $%d OR m.display_name ILIKE $%d OR r.route_name ILIKE $%d OR r.upstream_model ILIKE $%d)", n, n, n, n))
	}
	switch strings.TrimSpace(f.AccountID) {
	case "":
	case "unbound":
		where = append(where, "r.account_id IS NULL")
	default:
		args = append(args, strings.TrimSpace(f.AccountID))
		where = append(where, fmt.Sprintf("r.account_id = $%d::bigint", len(args)))
	}
	switch strings.TrimSpace(f.Enabled) {
	case "enabled":
		where = append(where, "r.is_enabled = true")
	case "disabled":
		where = append(where, "r.is_enabled = false")
	}
	if f.ModelID > 0 {
		args = append(args, f.ModelID)
		where = append(where, fmt.Sprintf("r.model_id = $%d", len(args)))
	}
	cond := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM model_routes r JOIN models m ON m.id = r.model_id WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := s.db.Query(ctx, `SELECT r.id, r.model_id, m.code, m.display_name, m.category, r.route_name, r.provider,
		r.upstream_model, COALESCE(r.price_group,''), r.priority, r.is_enabled, COALESCE(r.health_status,''),
		r.account_id, COALESCE(a.name,''), COALESCE(a.group_name,''), COALESCE(a.login_account,'')
		FROM model_routes r
		JOIN models m ON m.id = r.model_id
		LEFT JOIN upstream_accounts a ON a.id = r.account_id
		WHERE `+cond+`
		ORDER BY m.sort_order, m.code, r.priority, r.id
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []ModelRouteListItem{}
	for rows.Next() {
		var it ModelRouteListItem
		if err := rows.Scan(&it.RouteID, &it.ModelID, &it.ModelCode, &it.ModelName, &it.Category, &it.RouteName, &it.Provider,
			&it.UpstreamModel, &it.PriceGroup, &it.Priority, &it.IsEnabled, &it.HealthStatus,
			&it.AccountID, &it.AccountName, &it.AccountGroup, &it.LoginAccount); err != nil {
			return nil, 0, err
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

// BindRoutesToAccount 把指定线路批量绑定到账号（Key 与分组一起变成账号快照）。
// routeIDs 为空时按 modelID+provider 命中该模型的全部线路。
func (s *ModelService) BindRoutesToAccount(ctx context.Context, routeIDs []int64, modelID int64, accountID int64, force bool) (int64, error) {
	if accountID <= 0 {
		return 0, errors.New("缺少目标账号")
	}
	var key, group, provider string
	if err := s.db.QueryRow(ctx, `SELECT api_key, group_name, provider FROM upstream_accounts WHERE id=$1`, accountID).
		Scan(&key, &group, &provider); err != nil {
		return 0, errors.New("目标账号不存在")
	}
	if !force {
		// 分组能力校验：账号分组必须支持要绑定的模型，避免把文本分组配到图片模型上
		if err := s.validateAccountRoutesCompatibility(ctx, provider, group, routeIDs, modelID); err != nil {
			return 0, err
		}
	}
	// 账号里的 Key 可能是密文或明文：解密后统一按线路的加密方式重新写入
	plainKey := key
	if decrypted, decErr := util.DecryptSecret(key, s.routeCipherKey); decErr == nil {
		plainKey = decrypted
	}
	storedKey, encErr := util.EncryptSecret(plainKey, s.routeCipherKey)
	if encErr != nil {
		return 0, errors.New("线路密钥加密失败")
	}
	var tag interface{ RowsAffected() int64 }
	if len(routeIDs) > 0 {
		mismatch, err := s.routesProviderMismatch(ctx, routeIDs, provider)
		if err != nil {
			return 0, err
		}
		if mismatch > 0 {
			return 0, fmt.Errorf("有 %d 条线路不属于上游 %s，请先按上游筛选", mismatch, provider)
		}
		t, err := s.db.Exec(ctx, `UPDATE model_routes
			SET account_id=$1, api_key=$2, price_group=$3, updated_at=now()
			WHERE id = ANY($4)`, accountID, storedKey, group, routeIDs)
		if err != nil {
			return 0, err
		}
		tag = t
	} else if modelID > 0 {
		t, err := s.db.Exec(ctx, `UPDATE model_routes
			SET account_id=$1, api_key=$2, price_group=$3, updated_at=now()
			WHERE model_id=$4 AND provider=$5`, accountID, storedKey, group, modelID, provider)
		if err != nil {
			return 0, err
		}
		tag = t
	} else {
		return 0, errors.New("缺少要绑定的线路")
	}
	affected := tag.RowsAffected()
	if affected > 0 {
		// 绑定分组后按「官方价 × 分组倍率」自动校准线路成本，保证利润口径正确。
		if len(routeIDs) > 0 {
			_, _ = s.RecalibrateRouteCosts(ctx, "", routeIDs, false)
		} else if modelID > 0 {
			_, _ = s.db.Exec(ctx, `UPDATE model_routes SET updated_at=now() WHERE model_id=$1 AND provider=$2`, modelID, provider)
		}
		reason := fmt.Sprintf("线路批量绑定 %d 条到账号 %s（%s）", affected, provider, group)
		_, _ = s.db.Exec(ctx, `INSERT INTO upstream_events (event_type, provider, account_id, model_id, model_code, reason)
			VALUES ('manual_migrate', $1, $2, NULLIF($3,0), '', $4)`, provider, accountID, modelID, reason)
	}
	return affected, nil
}

func (s *ModelService) routesProviderMismatch(ctx context.Context, routeIDs []int64, provider string) (int, error) {
	var count int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM model_routes WHERE id = ANY($1) AND provider <> $2`, routeIDs, provider).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
