package service

import (
	"context"
	"fmt"
	"strings"
)

// UpstreamGroupCapability 描述一个上游分组支持哪些模型（来自价格同步矩阵）。
type UpstreamGroupCapability struct {
	GroupName  string         `json:"group_name"`
	ModelCount int            `json:"model_count"`
	Categories map[string]int `json:"categories"`
	Models     []string       `json:"models"`
}

// UpstreamAccountCompat 是「账号能否服务某条线路」的判定结果。
type UpstreamAccountCompat struct {
	AccountID    int64  `json:"account_id"`
	Status       string `json:"status"` // compatible / incompatible / unknown
	Reason       string `json:"reason,omitempty"`
	GroupName    string `json:"group_name"`
	GroupModelIn bool   `json:"group_model_exact"`
}

// ListUpstreamGroupCapabilities 返回某上游下各分组支持的模型清单与类型分布。
func (s *ModelService) ListUpstreamGroupCapabilities(ctx context.Context, provider string) ([]UpstreamGroupCapability, error) {
	rows, err := s.db.Query(ctx, `SELECT c.group_name, c.upstream_model, COALESCE(m.category,'')
		FROM upstream_price_current c
		LEFT JOIN upstream_model_mappings mp ON mp.provider = c.provider AND mp.upstream_model = c.upstream_model
		LEFT JOIN models m ON m.id = mp.model_id
		WHERE c.provider = $1 AND COALESCE(c.group_name,'') <> ''
		ORDER BY c.group_name, c.upstream_model`, strings.TrimSpace(provider))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byGroup := map[string]*UpstreamGroupCapability{}
	order := []string{}
	for rows.Next() {
		var group, model, category string
		if err := rows.Scan(&group, &model, &category); err != nil {
			return nil, err
		}
		item, ok := byGroup[group]
		if !ok {
			item = &UpstreamGroupCapability{GroupName: group, Categories: map[string]int{}}
			byGroup[group] = item
			order = append(order, group)
		}
		item.Models = append(item.Models, model)
		item.ModelCount++
		key := category
		if key == "" {
			key = "unknown"
		}
		item.Categories[key]++
	}
	out := make([]UpstreamGroupCapability, 0, len(order))
	for _, group := range order {
		out = append(out, *byGroup[group])
	}
	return out, rows.Err()
}

// groupModelMatrix 读取某上游的「分组 → 支持的模型集合」。
func (s *ModelService) groupModelMatrix(ctx context.Context, provider string) (map[string]map[string]bool, error) {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT group_name, upstream_model FROM upstream_price_current
		WHERE provider = $1 AND COALESCE(group_name,'') <> ''`, strings.TrimSpace(provider))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	matrix := map[string]map[string]bool{}
	for rows.Next() {
		var group, model string
		if err := rows.Scan(&group, &model); err != nil {
			return nil, err
		}
		if matrix[group] == nil {
			matrix[group] = map[string]bool{}
		}
		matrix[group][model] = true
	}
	return matrix, rows.Err()
}

func normalizeUpstreamModel(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// CheckAccountCompatibility 判断某上游各账号的分组能否服务指定上游模型。
// compatible：分组矩阵明确包含该模型；incompatible：分组矩阵收录了但这个模型没有；unknown：分组为空或未同步到。
func (s *ModelService) CheckAccountCompatibility(ctx context.Context, provider, upstreamModel string) ([]UpstreamAccountCompat, error) {
	matrix, err := s.groupModelMatrix(ctx, provider)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT id, group_name FROM upstream_accounts WHERE provider = $1 ORDER BY group_name, id`, strings.TrimSpace(provider))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	target := normalizeUpstreamModel(upstreamModel)
	out := []UpstreamAccountCompat{}
	for rows.Next() {
		var id int64
		var group string
		if err := rows.Scan(&id, &group); err != nil {
			return nil, err
		}
		res := UpstreamAccountCompat{AccountID: id, GroupName: group}
		models, known := matrix[group]
		switch {
		case strings.TrimSpace(group) == "":
			res.Status = "unknown"
			res.Reason = "账号未绑定分组，无法判断能力"
		case !known:
			res.Status = "unknown"
			res.Reason = "分组未出现在价格矩阵中（可能未同步）"
		default:
			exact := false
			for model := range models {
				if normalizeUpstreamModel(model) == target {
					exact = true
					break
				}
			}
			if exact {
				res.Status = "compatible"
			} else {
				res.Status = "incompatible"
				res.Reason = fmt.Sprintf("分组「%s」未包含模型 %s", group, upstreamModel)
			}
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

// validateAccountRoutesCompatibility 校验线路能否绑定到目标账号分组（force=true 时跳过）。
func (s *ModelService) validateAccountRoutesCompatibility(ctx context.Context, provider, group string, routeIDs []int64, modelID int64) error {
	if strings.TrimSpace(group) == "" {
		return nil // 未绑定分组：无法判断，放行
	}
	matrix, err := s.groupModelMatrix(ctx, provider)
	if err != nil {
		return err
	}
	models, known := matrix[group]
	if !known {
		return nil // 该分组不在矩阵中，放行（未知）
	}
	rows, err := s.db.Query(ctx, `SELECT id, upstream_model FROM model_routes
		WHERE ($1::bigint[] IS NOT NULL AND id = ANY($1::bigint[]))
		   OR ($2::bigint > 0 AND model_id = $2 AND provider = $3)`,
		routeIDs, modelID, provider)
	if err != nil {
		return err
	}
	defer rows.Close()
	bad := []string{}
	for rows.Next() {
		var id int64
		var upstream string
		if err := rows.Scan(&id, &upstream); err != nil {
			continue
		}
		if !models[upstream] {
			normalized := false
			for m := range models {
				if normalizeUpstreamModel(m) == normalizeUpstreamModel(upstream) {
					normalized = true
					break
				}
			}
			if !normalized {
				bad = append(bad, upstream)
			}
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("目标分组「%s」不支持这些模型：%s", group, strings.Join(dedupeStrings(bad), "、"))
	}
	return nil
}

func dedupeStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

// BulkImportResult 是批量导入账号的单行结果。
type BulkImportResult struct {
	Line    int    `json:"line"`
	Name    string `json:"name"`
	Status  string `json:"status"` // created / skipped / failed
	Message string `json:"message,omitempty"`
}

// BulkImportAccounts 解析「名称, API Key, 分组, 登录账号」多行文本并批量创建账号。
func (s *ModelService) BulkImportAccounts(ctx context.Context, provider, content string) ([]BulkImportResult, error) {
	matrix, err := s.groupModelMatrix(ctx, provider)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	results := []BulkImportResult{}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == '，' || r == '\t' })
		for j := range parts {
			parts[j] = strings.TrimSpace(parts[j])
		}
		res := BulkImportResult{Line: i + 1}
		if len(parts) < 2 {
			res.Status = "failed"
			res.Message = "格式应为：名称, API Key, 分组, 登录账号"
			results = append(results, res)
			continue
		}
		name, key := parts[0], parts[1]
		group, login := "", ""
		if len(parts) >= 3 {
			group = parts[2]
		}
		if len(parts) >= 4 {
			login = parts[3]
		}
		res.Name = name
		if key == "" {
			res.Status = "failed"
			res.Message = "API Key 为空"
			results = append(results, res)
			continue
		}
		if group != "" {
			if _, ok := matrix[group]; !ok {
				res.Status = "failed"
				res.Message = fmt.Sprintf("分组「%s」在该上游的价格矩阵中不存在", group)
				results = append(results, res)
				continue
			}
		}
		enabled := true
		_, createErr := s.CreateUpstreamAccount(ctx, UpstreamAccountInput{
			Provider: provider, Name: name, APIKey: key, GroupName: group, LoginAccount: login, IsEnabled: &enabled,
		})
		if createErr != nil {
			if strings.Contains(createErr.Error(), "已存在") {
				res.Status = "skipped"
				res.Message = createErr.Error()
			} else {
				res.Status = "failed"
				res.Message = createErr.Error()
			}
			results = append(results, res)
			continue
		}
		res.Status = "created"
		results = append(results, res)
	}
	return results, nil
}
