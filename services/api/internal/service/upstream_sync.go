package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/starai/api/internal/util"
)

// ---------- 数据模型 ----------

type UpstreamSyncSource struct {
	ID             int64                  `json:"id"`
	Provider       string                 `json:"provider"`
	Adapter        string                 `json:"adapter"`
	Name           string                 `json:"name"`
	SiteBaseURL    string                 `json:"site_base_url"`
	Credential     string                 `json:"credential,omitempty"` // 只写不读
	CredentialHint string                 `json:"credential_hint"`
	LoginAccount   string                 `json:"login_account"`
	Config         map[string]interface{} `json:"config"`
	IsEnabled      bool                   `json:"is_enabled"`
	LastSyncAt     *time.Time             `json:"last_sync_at,omitempty"`
	LastStatus     string                 `json:"last_status,omitempty"`
	LastError      string                 `json:"last_error,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type UpstreamSyncSourceInput struct {
	Provider     string                 `json:"provider"`
	Adapter      string                 `json:"adapter"`
	Name         string                 `json:"name"`
	SiteBaseURL  string                 `json:"site_base_url"`
	Credential   string                 `json:"credential"`
	LoginAccount string                 `json:"login_account"`
	Config       map[string]interface{} `json:"config"`
	IsEnabled    *bool                  `json:"is_enabled"`
}

type syncGroup struct {
	ID   string
	Name string
	Rate float64
}

type syncKey struct {
	ID    string
	Group string
	Name  string
	Key   string
}

// UpstreamSyncAdapter 抽象不同上游的同步接口。
type UpstreamSyncAdapter interface {
	Name() string
	FetchGroups(ctx context.Context, src UpstreamSyncSource, plainCredential string) ([]syncGroup, error)
	FetchKeys(ctx context.Context, src UpstreamSyncSource, plainCredential string) ([]syncKey, error)
	CreateKey(ctx context.Context, src UpstreamSyncSource, plainCredential string, group syncGroup) (*syncKey, error)
	FetchModels(ctx context.Context, src UpstreamSyncSource, key string) ([]string, error)
}

// ---------- 同步源 CRUD ----------

const upstreamSyncSourceColumns = `id, provider, adapter, name, site_base_url, credential, credential_hint,
	login_account, config, is_enabled, last_sync_at, COALESCE(last_status,''), COALESCE(last_error,''), created_at, updated_at`

func (s *ModelService) ListUpstreamSyncSources(ctx context.Context) ([]UpstreamSyncSource, error) {
	rows, err := s.db.Query(ctx, `SELECT `+upstreamSyncSourceColumns+` FROM upstream_sync_sources ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []UpstreamSyncSource{}
	for rows.Next() {
		var it UpstreamSyncSource
		var cred, config []byte
		if err := rows.Scan(&it.ID, &it.Provider, &it.Adapter, &it.Name, &it.SiteBaseURL, &cred, &it.CredentialHint,
			&it.LoginAccount, &config, &it.IsEnabled, &it.LastSyncAt, &it.LastStatus, &it.LastError, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(config, &it.Config)
		if it.Config == nil {
			it.Config = map[string]interface{}{}
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *ModelService) CreateUpstreamSyncSource(ctx context.Context, in UpstreamSyncSourceInput) (*UpstreamSyncSource, error) {
	provider := strings.TrimSpace(in.Provider)
	base := strings.TrimRight(strings.TrimSpace(in.SiteBaseURL), "/")
	if provider == "" || base == "" {
		return nil, errors.New("上游与站点地址不能为空")
	}
	adapter := strings.TrimSpace(in.Adapter)
	if adapter == "" {
		adapter = defaultSyncAdapter(provider)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = provider
	}
	enabled := true
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}
	cred, hint, err := s.encryptSyncCredential(in.Credential)
	if err != nil {
		return nil, err
	}
	config, _ := json.Marshal(in.Config)
	var id int64
	if err := s.db.QueryRow(ctx, `INSERT INTO upstream_sync_sources
		(provider, adapter, name, site_base_url, credential, credential_hint, login_account, config, is_enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		provider, adapter, name, base, cred, hint, strings.TrimSpace(in.LoginAccount), config, enabled).Scan(&id); err != nil {
		if strings.Contains(err.Error(), "upstream_sync_sources_provider_site_base_url_key") {
			return nil, errors.New("该上游站点已存在同步源")
		}
		return nil, err
	}
	return s.getUpstreamSyncSource(ctx, id)
}

func (s *ModelService) UpdateUpstreamSyncSource(ctx context.Context, id int64, in UpstreamSyncSourceInput) (*UpstreamSyncSource, error) {
	current, err := s.getUpstreamSyncSource(ctx, id)
	if err != nil {
		return nil, err
	}
	cred := ""
	hint := current.CredentialHint
	if strings.TrimSpace(in.Credential) != "" {
		cred, hint, err = s.encryptSyncCredential(in.Credential)
		if err != nil {
			return nil, err
		}
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = current.Name
	}
	enabled := current.IsEnabled
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}
	config := in.Config
	if config == nil {
		config = current.Config
	}
	cfg, _ := json.Marshal(config)
	if strings.TrimSpace(cred) == "" {
		if _, err := s.db.Exec(ctx, `UPDATE upstream_sync_sources SET name=$1, login_account=$2, config=$3,
			is_enabled=$4, updated_at=now() WHERE id=$5`, name, strings.TrimSpace(in.LoginAccount), cfg, enabled, id); err != nil {
			return nil, err
		}
	} else {
		if _, err := s.db.Exec(ctx, `UPDATE upstream_sync_sources SET name=$1, credential=$2, credential_hint=$3,
			login_account=$4, config=$5, is_enabled=$6, updated_at=now() WHERE id=$7`,
			name, cred, hint, strings.TrimSpace(in.LoginAccount), cfg, enabled, id); err != nil {
			return nil, err
		}
	}
	return s.getUpstreamSyncSource(ctx, id)
}

func (s *ModelService) DeleteUpstreamSyncSource(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM upstream_sync_sources WHERE id=$1`, id)
	return err
}

func (s *ModelService) getUpstreamSyncSource(ctx context.Context, id int64) (*UpstreamSyncSource, error) {
	var it UpstreamSyncSource
	var cred, config []byte
	if err := s.db.QueryRow(ctx, `SELECT `+upstreamSyncSourceColumns+` FROM upstream_sync_sources WHERE id=$1`, id).
		Scan(&it.ID, &it.Provider, &it.Adapter, &it.Name, &it.SiteBaseURL, &cred, &it.CredentialHint,
			&it.LoginAccount, &config, &it.IsEnabled, &it.LastSyncAt, &it.LastStatus, &it.LastError, &it.CreatedAt, &it.UpdatedAt); err != nil {
		return nil, errors.New("同步源不存在")
	}
	_ = json.Unmarshal(config, &it.Config)
	if it.Config == nil {
		it.Config = map[string]interface{}{}
	}
	return &it, nil
}

func (s *ModelService) encryptSyncCredential(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", nil
	}
	encrypted, err := util.EncryptSecret(raw, s.routeCipherKey)
	if err != nil {
		return "", "", errors.New("凭证加密失败")
	}
	hint := raw
	if len(hint) > 10 {
		hint = hint[:6] + "***" + hint[len(hint)-4:]
	}
	return encrypted, hint, nil
}

func (s *ModelService) decryptSyncCredential(stored string) string {
	if stored == "" {
		return ""
	}
	plain, err := util.DecryptSecret(stored, s.routeCipherKey)
	if err != nil {
		return stored
	}
	return plain
}

func defaultSyncAdapter(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "codexauv":
		return "codexauv"
	default:
		return "new_api"
	}
}

// ---------- 预览 ----------

type SyncPreviewItem struct {
	Group         string  `json:"group"`
	Rate          float64 `json:"rate"`
	HasKey        bool    `json:"has_key"`
	HasAccount    bool    `json:"has_account"`
	Action        string  `json:"action"` // reuse / create_key / create_account / update / rebuild
	AccountID     int64   `json:"account_id,omitempty"`
	AccountName   string  `json:"account_name,omitempty"`
	Message       string  `json:"message,omitempty"`
}

type SyncPreview struct {
	Source            UpstreamSyncSource `json:"source"`
	Rebuild           bool               `json:"rebuild"`
	Groups            int                `json:"groups"`
	KeysExisting      int                `json:"keys_existing"`
	KeysToCreate      int                `json:"keys_to_create"`
	AccountsExisting  int                `json:"accounts_existing"`
	AccountsToCreate  int                `json:"accounts_to_create"`
	Items             []SyncPreviewItem  `json:"items"`
}

// PreviewUpstreamSync 拉取站点分组并与项目账号做 diff（不写库）。
func (s *ModelService) PreviewUpstreamSync(ctx context.Context, sourceID int64, rebuild bool) (*SyncPreview, error) {
	src, cred, adapter, err := s.loadSyncSource(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	groups, err := adapter.FetchGroups(ctx, *src, cred)
	if err != nil {
		return nil, fmt.Errorf("拉取分组失败：%w", err)
	}
	keys, keyErr := adapter.FetchKeys(ctx, *src, cred)
	if keyErr != nil {
		keys = nil // 拉不到已有 Key 时不阻塞预览（全部视作需创建）
	}
	keyByGroup := map[string]syncKey{}
	for _, k := range keys {
		if k.Group != "" {
			keyByGroup[k.Group] = k
		}
	}
	accounts, _ := s.listAccountsByGroup(ctx, src.Provider)
	preview := &SyncPreview{Source: *src, Rebuild: rebuild, Groups: len(groups)}
	for _, g := range groups {
		item := SyncPreviewItem{Group: g.Name, Rate: g.Rate}
		if acc, ok := accounts[g.Name]; ok {
			_ = acc
			item.HasKey = true
			preview.KeysExisting++
		} else if k, ok := keyByGroup[g.Name]; ok && k.Key != "" {
			item.HasKey = true
			preview.KeysExisting++
		} else {
			preview.KeysToCreate++
			item.Action = "create_key"
		}
		if acc, ok := accounts[g.Name]; ok {
			item.HasAccount = true
			item.AccountID = acc.ID
			item.AccountName = acc.Name
			preview.AccountsExisting++
			if rebuild {
				item.Action = "rebuild"
			} else if item.Action == "" {
				item.Action = "reuse"
			}
		} else {
			preview.AccountsToCreate++
			if item.Action == "" {
				item.Action = "create_account"
			}
		}
		preview.Items = append(preview.Items, item)
	}
	return preview, nil
}

// ---------- 执行 ----------

type SyncRunResult struct {
	RunID             int64 `json:"run_id"`
	GroupsTotal       int   `json:"groups_total"`
	KeysCreated       int   `json:"keys_created"`
	KeysReused        int   `json:"keys_reused"`
	AccountsCreated   int   `json:"accounts_created"`
	AccountsUpdated   int   `json:"accounts_updated"`
	RoutesBound       int   `json:"routes_bound"`
	VerifyPassed      int   `json:"verify_passed"`
	VerifyFailed      int   `json:"verify_failed"`
}

// RunUpstreamSync 执行同步：为缺失分组创建 Key、建/更新账号、回填倍率、校验，可选绑定未绑定线路。
func (s *ModelService) RunUpstreamSync(ctx context.Context, sourceID int64, rebuild, bindUnbound bool) (*SyncRunResult, error) {
	src, cred, adapter, err := s.loadSyncSource(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	groups, err := adapter.FetchGroups(ctx, *src, cred)
	if err != nil {
		_, _ = s.db.Exec(ctx, `UPDATE upstream_sync_sources SET last_status='failed', last_error=$1, updated_at=now() WHERE id=$2`,
			truncateText(err.Error(), 500), sourceID)
		return nil, fmt.Errorf("拉取分组失败：%w", err)
	}
	keys, _ := adapter.FetchKeys(ctx, *src, cred)
	keyByGroup := map[string]syncKey{}
	for _, k := range keys {
		if k.Group != "" {
			keyByGroup[k.Group] = k
		}
	}
	accounts, _ := s.listAccountsByGroup(ctx, src.Provider)
	runID, err := s.startSyncRun(ctx, sourceID, len(groups))
	if err != nil {
		return nil, err
	}
	result := &SyncRunResult{RunID: runID, GroupsTotal: len(groups)}
	failedMsg := ""
	for _, g := range groups {
		// 1) Key：复用站点明文 → 复用账号已存 Key → 创建新 Key
		var key syncKey
		siteKey, hasSiteKey := keyByGroup[g.Name]
		existingAcc, hasAccount := accounts[g.Name]
		reuse := false
		if !rebuild {
			if hasSiteKey && siteKey.Key != "" {
				key, reuse = siteKey, true
			} else if hasAccount {
				if plain, plainErr := s.accountPlainKey(ctx, existingAcc.ID); plainErr == nil && plain != "" {
					key, reuse = syncKey{Group: g.Name, Key: plain}, true
				}
			}
		}
		if reuse {
			result.KeysReused++
		} else {
			created, createErr := adapter.CreateKey(ctx, *src, cred, g)
			if createErr != nil {
				result.VerifyFailed++
				_ = s.addSyncRunItem(ctx, runID, g, "failed", "创建 Key 失败："+truncateText(createErr.Error(), 300), 0)
				failedMsg = "部分分组创建 Key 失败"
				continue
			}
			key = *created
			result.KeysCreated++
		}
		// 2) 账号：建或更新
		name := syncAccountName(src.Provider, g.Name, g.Rate, src.LoginAccount)
		acc, exists := existingAcc, hasAccount
		if exists {
			if _, upErr := s.updateSyncAccount(ctx, src.Provider, acc.ID, name, key.Key, g.Name, src.LoginAccount); upErr != nil {
				_ = s.addSyncRunItem(ctx, runID, g, "failed", "更新账号失败："+truncateText(upErr.Error(), 300), acc.ID)
				continue
			}
			result.AccountsUpdated++
			_ = s.addSyncRunItem(ctx, runID, g, "updated", "已更新账号 Key/名称/登录账号", acc.ID)
		} else {
			newAcc, createErr := s.createSyncAccount(ctx, src.Provider, name, key.Key, g.Name, src.LoginAccount)
			if createErr != nil {
				_ = s.addSyncRunItem(ctx, runID, g, "failed", "创建账号失败："+truncateText(createErr.Error(), 300), 0)
				continue
			}
			result.AccountsCreated++
			acc = *newAcc
			_ = s.addSyncRunItem(ctx, runID, g, "created", "已创建账号并写入 Key", acc.ID)
		}
		// 3) 倍率
		_, _ = s.db.Exec(ctx, `INSERT INTO upstream_group_rates (provider, group_name, rate)
			VALUES ($1,$2,$3) ON CONFLICT (provider, group_name) DO UPDATE SET rate=EXCLUDED.rate, updated_at=now()`,
			src.Provider, g.Name, g.Rate)
		_, _ = s.db.Exec(ctx, `UPDATE upstream_accounts SET rate_multiplier=$1, updated_at=now() WHERE id=$2`, g.Rate, acc.ID)
		// 4) 校验
		models, verifyErr := adapter.FetchModels(ctx, *src, key.Key)
		if verifyErr != nil {
			result.VerifyFailed++
			_ = s.addSyncRunItem(ctx, runID, g, "verify_failed", "连通性校验失败："+truncateText(verifyErr.Error(), 200), acc.ID)
		} else {
			result.VerifyPassed++
		}
		// 5) 可选：绑定未绑定线路
		if bindUnbound && len(models) > 0 {
			bound, bindErr := s.bindUnboundRoutes(ctx, src.Provider, g.Name, acc.ID, models)
			if bindErr == nil {
				result.RoutesBound += int(bound)
			}
		}
	}
	status := "success"
	if failedMsg != "" {
		status = "partial"
	}
	_, _ = s.db.Exec(ctx, `UPDATE upstream_sync_runs SET status=$1, keys_created=$2, keys_reused=$3,
		accounts_created=$4, accounts_updated=$5, routes_bound=$6, verify_passed=$7, verify_failed=$8,
		finished_at=now(), summary=$9 WHERE id=$10`,
		status, result.KeysCreated, result.KeysReused, result.AccountsCreated, result.AccountsUpdated,
		result.RoutesBound, result.VerifyPassed, result.VerifyFailed, failedMsg, runID)
	_, _ = s.db.Exec(ctx, `UPDATE upstream_sync_sources SET last_sync_at=now(), last_status=$1, last_error=$2, updated_at=now() WHERE id=$3`,
		status, failedMsg, sourceID)
	return result, nil
}

// ---------- 内部工具 ----------

type syncAccountRef struct {
	ID   int64
	Name string
}

func (s *ModelService) loadSyncSource(ctx context.Context, id int64) (*UpstreamSyncSource, string, UpstreamSyncAdapter, error) {
	src, err := s.getUpstreamSyncSource(ctx, id)
	if err != nil {
		return nil, "", nil, err
	}
	var stored string
	if err := s.db.QueryRow(ctx, `SELECT credential FROM upstream_sync_sources WHERE id=$1`, id).Scan(&stored); err != nil {
		return nil, "", nil, errors.New("同步源不存在")
	}
	cred := s.decryptSyncCredential(stored)
	adapter, ok := upstreamSyncAdapters[src.Adapter]
	if !ok {
		return nil, "", nil, fmt.Errorf("不支持的适配器：%s", src.Adapter)
	}
	return src, cred, adapter, nil
}

func (s *ModelService) listAccountsByGroup(ctx context.Context, provider string) (map[string]syncAccountRef, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, group_name FROM upstream_accounts WHERE provider=$1`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]syncAccountRef{}
	for rows.Next() {
		var ref syncAccountRef
		var group string
		if err := rows.Scan(&ref.ID, &ref.Name, &group); err != nil {
			continue
		}
		out[group] = ref
	}
	return out, rows.Err()
}

// accountPlainKey 读取账号已保存的明文 Key（用于站点无法返回明文时复用）。
func (s *ModelService) accountPlainKey(ctx context.Context, accountID int64) (string, error) {
	var stored string
	if err := s.db.QueryRow(ctx, `SELECT api_key FROM upstream_accounts WHERE id=$1`, accountID).Scan(&stored); err != nil {
		return "", err
	}
	if plain, err := util.DecryptSecret(stored, s.routeCipherKey); err == nil {
		return plain, nil
	}
	return stored, nil
}

func (s *ModelService) startSyncRun(ctx context.Context, sourceID int64, groups int) (int64, error) {
	var runID int64
	err := s.db.QueryRow(ctx, `INSERT INTO upstream_sync_runs (source_id, status, groups_total) VALUES ($1,'running',$2) RETURNING id`,
		sourceID, groups).Scan(&runID)
	return runID, err
}

func (s *ModelService) addSyncRunItem(ctx context.Context, runID int64, g syncGroup, action, message string, accountID int64) error {
	_, err := s.db.Exec(ctx, `INSERT INTO upstream_sync_run_items (run_id, group_name, rate, action, message, account_id)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,0))`, runID, g.Name, g.Rate, action, message, accountID)
	return err
}

func (s *ModelService) createSyncAccount(ctx context.Context, provider, name, key, group, login string) (*syncAccountRef, error) {
	enabled := true
	acc, err := s.CreateUpstreamAccount(ctx, UpstreamAccountInput{
		Provider: provider, Name: name, APIKey: key, GroupName: group, LoginAccount: login, IsEnabled: &enabled,
	})
	if err != nil {
		return nil, err
	}
	return &syncAccountRef{ID: acc.ID, Name: acc.Name}, nil
}

func (s *ModelService) updateSyncAccount(ctx context.Context, provider string, id int64, name, key, group, login string) (*syncAccountRef, error) {
	_, err := s.UpdateUpstreamAccount(ctx, id, UpstreamAccountInput{
		Provider: provider, Name: name, APIKey: key, GroupName: group, LoginAccount: login,
	})
	if err != nil {
		return nil, err
	}
	return &syncAccountRef{ID: id, Name: name}, nil
}

func syncAccountName(provider, group string, rate float64, login string) string {
	label := providerDisplayName(provider) + " · " + group
	if rate > 0 {
		label += "-" + formatRate(rate) + "x"
	}
	if login != "" {
		label += "-" + login
	}
	return label
}

func providerDisplayName(provider string) string {
	switch strings.ToLower(provider) {
	case "codexauv":
		return "CodexAuv"
	case "packycode":
		return "PackyCode"
	case "qiniu":
		return "七牛云"
	case "new_api":
		return "yardai 中转"
	default:
		return provider
	}
}

// bindUnboundRoutes 把该上游下「未绑定账号」且模型在分组清单里的线路绑到该账号。
func (s *ModelService) bindUnboundRoutes(ctx context.Context, provider, group string, accountID int64, models []string) (int64, error) {
	modelSet := map[string]bool{}
	for _, m := range models {
		modelSet[strings.ToLower(m)] = true
	}
	rows, err := s.db.Query(ctx, `SELECT r.id, r.upstream_model FROM model_routes r
		WHERE r.provider=$1 AND r.account_id IS NULL`, provider)
	if err != nil {
		return 0, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		var upstream string
		if err := rows.Scan(&id, &upstream); err != nil {
			continue
		}
		if modelSet[strings.ToLower(upstream)] {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := s.db.Exec(ctx, `UPDATE model_routes SET account_id=$1, updated_at=now() WHERE id = ANY($2)`, accountID, ids)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ---------- 运行记录查询 ----------

func (s *ModelService) ListUpstreamSyncRuns(ctx context.Context, sourceID int64, limit int) ([]map[string]interface{}, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	q := `SELECT id, source_id, status, groups_total, keys_created, keys_reused, accounts_created, accounts_updated,
		routes_bound, verify_passed, verify_failed, started_at, finished_at, COALESCE(summary,'')
		FROM upstream_sync_runs`
	args := []interface{}{}
	if sourceID > 0 {
		args = append(args, sourceID)
		q += ` WHERE source_id=$1`
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY id DESC LIMIT $%d`, len(args))
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]interface{}{}
	for rows.Next() {
		var id, source, groups, kc, kr, ac, au, rb, vp, vf int64
		var status, summary string
		var started time.Time
		var finished *time.Time
		if err := rows.Scan(&id, &source, &status, &groups, &kc, &kr, &ac, &au, &rb, &vp, &vf, &started, &finished, &summary); err != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"id": id, "source_id": source, "status": status, "groups_total": groups,
			"keys_created": kc, "keys_reused": kr, "accounts_created": ac, "accounts_updated": au,
			"routes_bound": rb, "verify_passed": vp, "verify_failed": vf,
			"started_at": started, "finished_at": finished, "summary": summary,
		})
	}
	return items, rows.Err()
}

func (s *ModelService) GetUpstreamSyncRunItems(ctx context.Context, runID int64) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(ctx, `SELECT group_name, rate, action, message, COALESCE(account_id,0)
		FROM upstream_sync_run_items WHERE run_id=$1 ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]interface{}{}
	for rows.Next() {
		var group, action, message string
		var rate float64
		var accountID int64
		if err := rows.Scan(&group, &rate, &action, &message, &accountID); err != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"group": group, "rate": rate, "action": action, "message": message, "account_id": accountID,
		})
	}
	return items, rows.Err()
}

// ---------- 适配器注册 ----------

var upstreamSyncAdapters = map[string]UpstreamSyncAdapter{}

func registerSyncAdapter(adapter UpstreamSyncAdapter) {
	upstreamSyncAdapters[adapter.Name()] = adapter
}

// ---------- 通用 HTTP 工具 ----------

func syncHTTPDo(ctx context.Context, method, target string, headers map[string]string, body []byte, timeout time.Duration) ([]byte, int, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, target, reader)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return raw, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateText(string(raw), 200))
	}
	return raw, resp.StatusCode, nil
}

func syncJoinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}
