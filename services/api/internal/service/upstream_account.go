package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/starai/api/internal/util"
)

// UpstreamAccount 表示一个上游账号：一个 API Key 只属于一个分组（CodexAuv / PackyCode 等）。
// 线路绑定账号后，api_key 与 price_group 都以账号为准（写入线路时做快照，运行时不改代码）。
type UpstreamAccount struct {
	ID           int64      `json:"id"`
	Provider     string     `json:"provider"`
	Name         string     `json:"name"`
	APIKey       string     `json:"api_key"`
	GroupName    string     `json:"group_name"`
	RateMultiplier float64  `json:"rate_multiplier"`
	LoginAccount string     `json:"login_account"`
	HealthStatus string     `json:"health_status"`
	CooldownUntil *time.Time `json:"cooldown_until,omitempty"`
	LastProbeAt  *time.Time `json:"last_probe_at,omitempty"`
	IsEnabled    bool       `json:"is_enabled"`
	Notes        string     `json:"notes"`
	LastTestAt   *time.Time `json:"last_test_at,omitempty"`
	LastTestOK   *bool      `json:"last_test_ok,omitempty"`
	LastTestErr  string     `json:"last_test_error,omitempty"`
	RouteCount   int64      `json:"route_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type UpstreamAccountInput struct {
	Provider  string `json:"provider"`
	Name      string `json:"name"`
	APIKey    string `json:"api_key"`
	GroupName    string `json:"group_name"`
	LoginAccount string `json:"login_account"`
	IsEnabled    *bool  `json:"is_enabled"`
	Notes     string `json:"notes"`
}

const upstreamAccountColumns = `id, provider, name, api_key, group_name, rate_multiplier, login_account, health_status, cooldown_until, last_probe_at, is_enabled, COALESCE(notes,''),
	last_test_at, last_test_ok, COALESCE(last_test_error,''), created_at, updated_at`

func (s *ModelService) ListUpstreamAccounts(ctx context.Context, provider string) ([]UpstreamAccount, error) {
	q := `SELECT ` + upstreamAccountColumns + `,
		(SELECT count(*) FROM model_routes r WHERE r.account_id = upstream_accounts.id)
		FROM upstream_accounts`
	args := []interface{}{}
	if strings.TrimSpace(provider) != "" {
		q += ` WHERE provider = $1`
		args = append(args, provider)
	}
	q += ` ORDER BY provider, login_account, group_name, id`
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []UpstreamAccount{}
	for rows.Next() {
		var a UpstreamAccount
		if err := rows.Scan(&a.ID, &a.Provider, &a.Name, &a.APIKey, &a.GroupName, &a.RateMultiplier, &a.LoginAccount, &a.HealthStatus, &a.CooldownUntil, &a.LastProbeAt, &a.IsEnabled, &a.Notes,
			&a.LastTestAt, &a.LastTestOK, &a.LastTestErr, &a.CreatedAt, &a.UpdatedAt, &a.RouteCount); err != nil {
			return nil, err
		}
		a.APIKey = s.maskAccountKey(a.APIKey)
		items = append(items, a)
	}
	return items, rows.Err()
}

func (s *ModelService) CreateUpstreamAccount(ctx context.Context, in UpstreamAccountInput) (*UpstreamAccount, error) {
	provider := strings.TrimSpace(in.Provider)
	key := strings.TrimSpace(in.APIKey)
	if provider == "" || key == "" {
		return nil, errors.New("上游与 API Key 不能为空")
	}
	// 账号在未指定名称时自动生成：上游 · 分组-倍率x-登录账号（便于多账号分流时识别）
	rate := s.lookupGroupRate(ctx, provider, strings.TrimSpace(in.GroupName))
	name := strings.TrimSpace(in.Name)
	if name == "" {
		label := provider
		if g := strings.TrimSpace(in.GroupName); g != "" {
			label += " · " + g
			if rate > 0 {
				label += "-" + formatRate(rate) + "x"
			}
		}
		if l := strings.TrimSpace(in.LoginAccount); l != "" {
			label += "-" + l
		}
		name = label
	}
	encrypted, err := util.EncryptSecret(key, s.routeCipherKey)
	if err != nil {
		return nil, errors.New("账号密钥加密失败")
	}
	enabled := true
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}
	var id int64
	err = s.db.QueryRow(ctx, `INSERT INTO upstream_accounts (provider, name, api_key, group_name, rate_multiplier, login_account, is_enabled, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		provider, name, encrypted, strings.TrimSpace(in.GroupName), rate, strings.TrimSpace(in.LoginAccount), enabled, strings.TrimSpace(in.Notes)).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "upstream_accounts_provider_api_key_key") {
			return nil, errors.New("该上游下已存在相同 API Key 的账号")
		}
		return nil, err
	}
	return s.getUpstreamAccount(ctx, id)
}

func (s *ModelService) UpdateUpstreamAccount(ctx context.Context, id int64, in UpstreamAccountInput) (*UpstreamAccount, error) {
	current, err := s.getUpstreamAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = current.Name
	}
	groupName := strings.TrimSpace(in.GroupName)
	enabled := current.IsEnabled
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}
	storedKey := ""
	if err := s.db.QueryRow(ctx, `SELECT api_key FROM upstream_accounts WHERE id=$1`, id).Scan(&storedKey); err != nil {
		return nil, errors.New("账号不存在")
	}
	key := strings.TrimSpace(in.APIKey)
	if key != "" && !isMaskedSecret(key) {
		encrypted, encErr := util.EncryptSecret(key, s.routeCipherKey)
		if encErr != nil {
			return nil, errors.New("账号密钥加密失败")
		}
		storedKey = encrypted
	}
	targetProvider := strings.TrimSpace(in.Provider)
	if targetProvider == "" {
		targetProvider = current.Provider
	}
	rate := s.lookupGroupRate(ctx, targetProvider, groupName)
	if _, err := s.db.Exec(ctx, `UPDATE upstream_accounts SET provider=$1, name=$2, api_key=$3, group_name=$4,
		rate_multiplier=$5, login_account=$6, is_enabled=$7, notes=$8, updated_at=now() WHERE id=$9`,
		targetProvider, name, storedKey, groupName, rate, strings.TrimSpace(in.LoginAccount), enabled, strings.TrimSpace(in.Notes), id); err != nil {
		return nil, err
	}
	if groupName != current.GroupName {
		// 账号分组变化时，同步已绑定线路的分组快照
		_, _ = s.db.Exec(ctx, `UPDATE model_routes SET price_group=$1, updated_at=now() WHERE account_id=$2 AND COALESCE(price_group,'') <> $1`, groupName, id)
	}
	return s.getUpstreamAccount(ctx, id)
}

func (s *ModelService) DeleteUpstreamAccount(ctx context.Context, id int64) error {
	var bound int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM model_routes WHERE account_id=$1`, id).Scan(&bound); err != nil {
		return err
	}
	if bound > 0 {
		return fmt.Errorf("该账号仍绑定 %d 条线路，请先迁移或解绑", bound)
	}
	_, err := s.db.Exec(ctx, `DELETE FROM upstream_accounts WHERE id=$1`, id)
	return err
}

// MigrateUpstreamAccount 把源账号下的所有线路一键切到目标账号（Key 与分组一起替换）。
func (s *ModelService) MigrateUpstreamAccount(ctx context.Context, fromID, toID int64) (int64, error) {
	if fromID == toID {
		return 0, errors.New("源账号与目标账号相同")
	}
	var targetKey, targetGroup, targetProvider string
	if err := s.db.QueryRow(ctx, `SELECT api_key, group_name, provider FROM upstream_accounts WHERE id=$1`, toID).
		Scan(&targetKey, &targetGroup, &targetProvider); err != nil {
		return 0, errors.New("目标账号不存在")
	}
	var fromProvider string
	if err := s.db.QueryRow(ctx, `SELECT provider FROM upstream_accounts WHERE id=$1`, fromID).Scan(&fromProvider); err != nil {
		return 0, errors.New("源账号不存在")
	}
	if fromProvider != targetProvider {
		return 0, errors.New("只能在同一上游内迁移账号")
	}
	tag, err := s.db.Exec(ctx, `UPDATE model_routes
		SET account_id=$1, api_key=$2, price_group=$3, updated_at=now()
		WHERE account_id=$4`, toID, targetKey, targetGroup, fromID)
	if err != nil {
		return 0, err
	}
	affected := tag.RowsAffected()
	// 记录手动批量迁移事件，便于审计与告警通知。
	var fromName, fromGroup, toName, toGroup string
	_ = s.db.QueryRow(ctx, `SELECT name, group_name FROM upstream_accounts WHERE id=$1`, fromID).Scan(&fromName, &fromGroup)
	_ = s.db.QueryRow(ctx, `SELECT name, group_name FROM upstream_accounts WHERE id=$1`, toID).Scan(&toName, &toGroup)
	reason := fmt.Sprintf("手动批量迁移 %d 条线路：%s（%s）→ %s（%s）", affected, fromName, fromGroup, toName, toGroup)
	_, _ = s.db.Exec(ctx, `INSERT INTO upstream_events (event_type, provider, account_id, model_id, model_code, reason)
		VALUES ('manual_migrate', $1, $2, NULL, '', $3)`, targetProvider, toID, reason)
	return affected, nil
}

// TestUpstreamAccount 用账号 Key 请求上游的模型列表接口，返回连通性结果。
func (s *ModelService) TestUpstreamAccount(ctx context.Context, id int64) (map[string]interface{}, error) {
	var provider, storedKey string
	if err := s.db.QueryRow(ctx, `SELECT provider, api_key FROM upstream_accounts WHERE id=$1`, id).Scan(&provider, &storedKey); err != nil {
		return nil, errors.New("账号不存在")
	}
	key, err := util.DecryptSecret(storedKey, s.routeCipherKey)
	if err != nil {
		key = storedKey
	}
	var baseURL string
	_ = s.db.QueryRow(ctx, `SELECT base_url FROM model_routes WHERE provider=$1 AND COALESCE(base_url,'') <> '' ORDER BY id LIMIT 1`, provider).Scan(&baseURL)
	if baseURL == "" {
		return nil, errors.New("该上游还没有已配置的线路，无法推断 Base URL")
	}
	base := strings.TrimRight(baseURL, "/")
	candidates := []string{base + "/v1/models", base + "/models"}
	client := &http.Client{Timeout: 20 * time.Second}
	var lastStatus int
	var lastErr string
	for _, url := range candidates {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if reqErr != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+key)
		resp, doErr := client.Do(req)
		if doErr != nil {
			lastErr = doErr.Error()
			continue
		}
		raw := make([]byte, 2048)
		n, _ := resp.Body.Read(raw)
		resp.Body.Close()
		lastStatus = resp.StatusCode
		body := string(raw[:n])
		ok := resp.StatusCode >= 200 && resp.StatusCode < 300
		errText := ""
		if !ok {
			errText = truncateText(body, 300)
		}
		_, _ = s.db.Exec(ctx, `UPDATE upstream_accounts SET last_test_at=now(), last_test_ok=$1, last_test_error=$2, updated_at=now() WHERE id=$3`,
			ok, errText, id)
		var count int
		var payload map[string]interface{}
		if json.Unmarshal(raw[:n], &payload) == nil {
			if list, ok2 := payload["data"].([]interface{}); ok2 {
				count = len(list)
			}
		}
		return map[string]interface{}{
			"ok": ok, "status_code": resp.StatusCode, "url": url,
			"models": count, "message": truncateText(body, 300),
		}, nil
	}
	_, _ = s.db.Exec(ctx, `UPDATE upstream_accounts SET last_test_at=now(), last_test_ok=false, last_test_error=$1, updated_at=now() WHERE id=$2`,
		lastErr, id)
	return map[string]interface{}{"ok": false, "status_code": lastStatus, "message": lastErr}, nil
}

// formatRate 把倍率格式化成 0.075 / 2 / 2.65 这样的短形式。
func formatRate(rate float64) string {
	text := strconv.FormatFloat(rate, 'f', -1, 64)
	return text
}

// lookupGroupRate 查该上游分组的倍率（来自价格同步矩阵，未同步到时为 0）。
func (s *ModelService) lookupGroupRate(ctx context.Context, provider, group string) float64 {
	if strings.TrimSpace(provider) == "" || strings.TrimSpace(group) == "" {
		return 0
	}
	// 分组倍率表是权威来源（来自上游分组的真实倍率）；价格矩阵作为兜底
	var rate float64
	_ = s.db.QueryRow(ctx, `SELECT COALESCE(rate,0) FROM upstream_group_rates
		WHERE provider=$1 AND group_name=$2`, provider, group).Scan(&rate)
	if rate <= 0 {
		_ = s.db.QueryRow(ctx, `SELECT COALESCE(max(billing_multiplier),0) FROM upstream_price_current
			WHERE provider=$1 AND group_name=$2`, provider, group).Scan(&rate)
	}
	return rate
}

func (s *ModelService) getUpstreamAccount(ctx context.Context, id int64) (*UpstreamAccount, error) {
	var a UpstreamAccount
	err := s.db.QueryRow(ctx, `SELECT `+upstreamAccountColumns+`,
		(SELECT count(*) FROM model_routes r WHERE r.account_id = upstream_accounts.id)
		FROM upstream_accounts WHERE id=$1`, id).
		Scan(&a.ID, &a.Provider, &a.Name, &a.APIKey, &a.GroupName, &a.RateMultiplier, &a.LoginAccount, &a.HealthStatus, &a.CooldownUntil, &a.LastProbeAt, &a.IsEnabled, &a.Notes,
			&a.LastTestAt, &a.LastTestOK, &a.LastTestErr, &a.CreatedAt, &a.UpdatedAt, &a.RouteCount)
	if err != nil {
		return nil, errors.New("账号不存在")
	}
	a.APIKey = s.maskAccountKey(a.APIKey)
	return &a, nil
}

// resolveRouteAccount 返回账号的明文 Key、分组与上游，供创建/更新线路时写入快照。
func (s *ModelService) resolveRouteAccount(ctx context.Context, accountID int64) (string, string, string, error) {
	var storedKey, group, provider string
	if err := s.db.QueryRow(ctx, `SELECT api_key, group_name, provider FROM upstream_accounts WHERE id=$1`, accountID).
		Scan(&storedKey, &group, &provider); err != nil {
		return "", "", "", errors.New("上游账号不存在")
	}
	key, err := util.DecryptSecret(storedKey, s.routeCipherKey)
	if err != nil {
		key = storedKey
	}
	return key, group, provider, nil
}

// maskAccountKey 解密后掩码展示，避免把 enc:v1: 密文直接暴露给前端。
func (s *ModelService) maskAccountKey(stored string) string {
	if stored == "" {
		return ""
	}
	plain, err := util.DecryptSecret(stored, s.routeCipherKey)
	if err != nil {
		plain = stored
	}
	return maskSecret(plain)
}

func truncateText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}

// UpstreamLoginAccountStat 按「上游登录账号」汇总 Key 数、线路数与近期调用质量。
type UpstreamLoginAccountStat struct {
	Provider      string  `json:"provider"`
	LoginAccount  string  `json:"login_account"`
	KeyCount      int64   `json:"key_count"`
	RouteCount    int64   `json:"route_count"`
	Attempts      int64   `json:"attempts"`
	Failures      int64   `json:"failures"`
	FailureRate   float64 `json:"failure_rate"`
	LastFailureAt string  `json:"last_failure_at,omitempty"`
}

// ListUpstreamLoginAccountStats 按登录账号汇总账号与线路用量（默认近 7 天）。
func (s *ModelService) ListUpstreamLoginAccountStats(ctx context.Context, days int) ([]UpstreamLoginAccountStat, error) {
	if days < 1 || days > 90 {
		days = 7
	}
	rows, err := s.db.Query(ctx, `SELECT a.provider, a.login_account,
		count(DISTINCT a.id) AS key_count,
		(SELECT count(*) FROM model_routes r2 JOIN upstream_accounts a2 ON a2.id = r2.account_id
		  WHERE a2.provider = a.provider AND a2.login_account = a.login_account)
		FROM upstream_accounts a
		GROUP BY a.provider, a.login_account
		ORDER BY a.provider, a.login_account`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byKey := map[string]*UpstreamLoginAccountStat{}
	order := []string{}
	for rows.Next() {
		var st UpstreamLoginAccountStat
		if err := rows.Scan(&st.Provider, &st.LoginAccount, &st.KeyCount, &st.RouteCount); err != nil {
			return nil, err
		}
		k := st.Provider + "\x00" + st.LoginAccount
		byKey[k] = &st
		order = append(order, k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	usageRows, err := s.db.Query(ctx, `SELECT r.provider, COALESCE(a.login_account,''),
		count(*) AS attempts,
		count(*) FILTER (WHERE ra.status <> 'success') AS failures,
		max(ra.created_at) FILTER (WHERE ra.status <> 'success')::text
		FROM model_route_attempts ra
		JOIN model_routes r ON r.id = ra.route_id
		LEFT JOIN upstream_accounts a ON a.id = r.account_id
		WHERE ra.created_at >= now() - make_interval(days => $1)
		GROUP BY 1,2`, days)
	if err != nil {
		return nil, err
	}
	defer usageRows.Close()
	for usageRows.Next() {
		var provider, login string
		var attempts, failures int64
		var lastFailure *string
		if err := usageRows.Scan(&provider, &login, &attempts, &failures, &lastFailure); err != nil {
			return nil, err
		}
		k := provider + "\x00" + login
		st, ok := byKey[k]
		if !ok {
			st = &UpstreamLoginAccountStat{Provider: provider, LoginAccount: login}
			byKey[k] = st
			order = append(order, k)
		}
		st.Attempts = attempts
		st.Failures = failures
		if attempts > 0 {
			st.FailureRate = float64(failures) / float64(attempts) * 100
		}
		if lastFailure != nil {
			st.LastFailureAt = *lastFailure
		}
	}
	out := make([]UpstreamLoginAccountStat, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out, usageRows.Err()
}

// AccountRouteItem 是绑定到某账号的一条线路（用于后台弹窗查看）。
type AccountRouteItem struct {
	RouteID       int64  `json:"route_id"`
	ModelID       int64  `json:"model_id"`
	ModelCode     string `json:"model_code"`
	ModelName     string `json:"model_name"`
	RouteName     string `json:"route_name"`
	Provider      string `json:"provider"`
	UpstreamModel string `json:"upstream_model"`
	PriceGroup    string `json:"price_group"`
	Priority      int    `json:"priority"`
	IsEnabled     bool   `json:"is_enabled"`
	HealthStatus  string `json:"health_status"`
}

// ListAccountRoutes 返回绑定到该账号的所有线路。
func (s *ModelService) ListAccountRoutes(ctx context.Context, accountID int64) ([]AccountRouteItem, error) {
	rows, err := s.db.Query(ctx, `SELECT r.id, r.model_id, m.code, m.display_name, r.route_name, r.provider,
		r.upstream_model, COALESCE(r.price_group,''), r.priority, r.is_enabled, COALESCE(r.health_status,'')
		FROM model_routes r JOIN models m ON m.id = r.model_id
		WHERE r.account_id = $1
		ORDER BY r.priority, m.sort_order, r.id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AccountRouteItem{}
	for rows.Next() {
		var it AccountRouteItem
		if err := rows.Scan(&it.RouteID, &it.ModelID, &it.ModelCode, &it.ModelName, &it.RouteName, &it.Provider,
			&it.UpstreamModel, &it.PriceGroup, &it.Priority, &it.IsEnabled, &it.HealthStatus); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
