package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/starai/api/internal/util"
)

// RunAccountProbeLoop 定时主动探测上游账号（每 10 分钟），失败按错误类型熔断账号。
// 探测必须发一次最小推理请求：多数中转站的 /v1/models 不校验余额。
func (s *ModelService) RunAccountProbeLoop(ctx context.Context) {
	run := func() {
		rows, err := s.db.Query(ctx, `SELECT a.id, a.provider, a.api_key, a.group_name
			FROM upstream_accounts a
			WHERE a.is_enabled = true
			  AND (a.cooldown_until IS NULL OR a.cooldown_until <= now())
			  AND (a.last_probe_at IS NULL OR a.last_probe_at < now() - interval '10 minutes')
			  AND EXISTS (SELECT 1 FROM model_routes r WHERE r.account_id = a.id AND r.is_enabled = true)
			ORDER BY a.last_probe_at NULLS FIRST LIMIT 20`)
		if err != nil {
			return
		}
		type accountRow struct {
			id                  int64
			provider, storedKey string
			group               string
		}
		accounts := []accountRow{}
		for rows.Next() {
			var a accountRow
			if err := rows.Scan(&a.id, &a.provider, &a.storedKey, &a.group); err != nil {
				continue
			}
			accounts = append(accounts, a)
		}
		rows.Close()
		for _, account := range accounts {
			s.probeOneAccount(ctx, account.id, account.provider, account.storedKey)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
	go func() {
		time.Sleep(30 * time.Second)
		run()
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

// probeOneAccount 用账号下最便宜的若干条线路各发一次最小请求。
// 按线路协议构造请求：claude → /v1/messages（x-api-key），其余按 OpenAI 兼容 chat 请求。
func (s *ModelService) probeOneAccount(ctx context.Context, accountID int64, provider, storedKey string) {
	type candidate struct {
		routeID, modelID                          int64
		protocol, baseURL, endpoint, upstreamModel string
		apiKeyHeader, modelCode, routeName         string
	}
	rows, err := s.db.Query(ctx, `SELECT r.id, r.model_id, COALESCE(NULLIF(r.protocol,''),'openai'), r.base_url, r.endpoint,
		r.upstream_model, COALESCE(NULLIF(r.api_key_header,''),''), m.code, r.route_name
		FROM model_routes r JOIN models m ON m.id = r.model_id
		WHERE r.account_id = $1 AND r.is_enabled = true
		  AND (m.request_mode = 'chat_completions' OR lower(COALESCE(r.protocol,'')) = 'claude')
		ORDER BY COALESCE((r.cost_rule->>'input_cost_per_m')::numeric, 999999) ASC NULLS LAST, r.id
		LIMIT 3`, accountID)
	if err != nil {
		return
	}
	candidates := []candidate{}
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.routeID, &c.modelID, &c.protocol, &c.baseURL, &c.endpoint, &c.upstreamModel,
			&c.apiKeyHeader, &c.modelCode, &c.routeName); err == nil {
			candidates = append(candidates, c)
		}
	}
	rows.Close()
	key, decErr := util.DecryptSecret(storedKey, s.routeCipherKey)
	if decErr != nil {
		key = storedKey
	}
	fatalReason := ""
	fatalStatus := 0
	fatalRouteID := int64(0)
	fatalModelID := int64(0)
	fatalModelCode := ""
	fatalRouteName := ""
	for _, c := range candidates {
		base := strings.TrimRight(strings.TrimSpace(c.baseURL), "/")
		if base == "" {
			continue
		}
		endpoint := strings.TrimSpace(c.endpoint)
		isClaude := strings.EqualFold(c.protocol, "claude")
		if isClaude && (endpoint == "" || strings.Contains(endpoint, "chat/completions")) {
			endpoint = "/v1/messages"
		}
		if endpoint == "" {
			endpoint = "/v1/chat/completions"
		}
		url := joinProbeURL(base, endpoint)
		var payload map[string]interface{}
		headers := map[string]string{"Content-Type": "application/json"}
		if isClaude {
			payload = map[string]interface{}{
				"model":      c.upstreamModel,
				"max_tokens": 8,
				"messages":   []map[string]string{{"role": "user", "content": "ping"}},
			}
			headers["anthropic-version"] = "2023-06-01"
			headers["x-api-key"] = key
		} else {
			payload = map[string]interface{}{
				"model":      c.upstreamModel,
				"messages":   []map[string]string{{"role": "user", "content": "ping"}},
				"max_tokens": 8,
				"stream":     false,
			}
			headers["Authorization"] = "Bearer " + key
		}
		raw, _ := json.Marshal(payload)
		reqCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		req, reqErr := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(raw))
		if reqErr != nil {
			cancel()
			continue
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, doErr := (&http.Client{Timeout: 25 * time.Second}).Do(req)
		if doErr != nil {
			cancel()
			fatalReason = doErr.Error()
			fatalStatus = 0
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		cancel()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			s.recordProbeResult(ctx, accountID, provider, c.routeID, resp.StatusCode, true, "", c.modelID, c.modelCode, c.routeName)
			return
		}
		reason := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateText(string(body), 200))
		lowerBody := strings.ToLower(string(body))
		modelScoped := strings.Contains(lowerBody, "model_not_found") ||
			strings.Contains(lowerBody, "not supported") ||
			strings.Contains(lowerBody, "无可用渠道") ||
			strings.Contains(lowerBody, "no available channel") ||
			strings.Contains(lowerBody, "protocol_not_supported")
		if resp.StatusCode == 404 || resp.StatusCode == 400 || resp.StatusCode == 422 || modelScoped {
			continue
		}
		fatalReason = reason
		fatalStatus = resp.StatusCode
		fatalRouteID = c.routeID
		fatalModelID = c.modelID
		fatalModelCode = c.modelCode
		fatalRouteName = c.routeName
		if resp.StatusCode == 401 || resp.StatusCode == 402 || resp.StatusCode == 403 {
			break
		}
	}
	if fatalReason == "" {
		_, _ = s.db.Exec(ctx, `UPDATE upstream_accounts SET last_probe_at=now(), updated_at=now() WHERE id=$1`, accountID)
		return
	}
	s.recordProbeResult(ctx, accountID, provider, fatalRouteID, fatalStatus, false, fatalReason, fatalModelID, fatalModelCode, fatalRouteName)
}

// joinProbeURL 拼接 base 与 endpoint，避免重复的 /v1 前缀。
func joinProbeURL(base, endpoint string) string {
	b := strings.TrimRight(base, "/")
	e := "/" + strings.TrimLeft(endpoint, "/")
	if strings.HasSuffix(b, "/v1") && strings.HasPrefix(e, "/v1/") {
		return b + e[3:]
	}
	return b + e
}

func (s *ModelService) recordProbeResult(ctx context.Context, accountID int64, provider string, routeID int64, status int, ok bool, reason string, modelID int64, modelCode, routeName string) {
	_, _ = s.db.Exec(ctx, `UPDATE upstream_accounts SET last_probe_at=now(), updated_at=now() WHERE id=$1`, accountID)
	if ok {
		_, _ = s.db.Exec(ctx, `UPDATE upstream_accounts SET health_status='healthy', consecutive_failures=0, cooldown_until=NULL WHERE id=$1`, accountID)
		return
	}
	// 账号级熔断：额度/鉴权类立即熔断 30 分钟；其它错误累计 3 次再熔断 10 分钟。
	fatal := status == 401 || status == 402 || status == 403 || strings.Contains(strings.ToLower(reason), "insufficient")
	cooldown := "10 minutes"
	increment := "consecutive_failures + 1"
	if fatal {
		cooldown = "30 minutes"
	}
	_, _ = s.db.Exec(ctx, `UPDATE upstream_accounts SET
		consecutive_failures = `+increment+`,
		health_status = CASE WHEN `+increment+` >= 3 OR $2 THEN 'open' ELSE 'degraded' END,
		cooldown_until = CASE WHEN `+increment+` >= 3 OR $2 THEN now() + interval '`+cooldown+`' ELSE cooldown_until END,
		updated_at = now()
		WHERE id=$1`, accountID, fatal)
	if routeName != "" {
		reason = "探测线路：" + routeName + "；" + reason
	}
	_, _ = s.db.Exec(ctx, `INSERT INTO upstream_events (event_type, provider, account_id, model_id, model_code, from_route_id, reason, status_code)
		SELECT 'account_down',$1,$2,NULLIF($6,0),$7,$3,$4,$5
		WHERE NOT EXISTS (SELECT 1 FROM upstream_events e WHERE e.event_type='account_down' AND e.account_id=$2 AND e.created_at > now()-interval '15 minutes')`,
		provider, accountID, routeID, truncateText(reason, 400), status, modelID, modelCode)
	log.Printf("upstream account %d probe failed: %s", accountID, truncateText(reason, 200))
}

// ListUpstreamEvents 返回最近的线路切换/账号告警事件，供后台排查。
func (s *AdminService) ListUpstreamEvents(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `SELECT e.id,e.event_type,e.provider,COALESCE(e.account_id,0),e.model_code,
		e.reason,COALESCE(e.status_code,0),e.notified,e.created_at,COALESCE(a.name,''),COALESCE(a.group_name,'')
		FROM upstream_events e LEFT JOIN upstream_accounts a ON a.id = e.account_id
		ORDER BY e.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]interface{}{}
	for rows.Next() {
		var id, accountID int64
		var eventType, provider, modelCode, reason, accountName, accountGroup string
		var status int
		var notified bool
		var createdAt time.Time
		if err := rows.Scan(&id, &eventType, &provider, &accountID, &modelCode, &reason, &status, &notified, &createdAt, &accountName, &accountGroup); err != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"id": id, "event_type": eventType, "provider": provider, "account_id": accountID,
			"account_name": accountName, "account_group": accountGroup,
			"model_code": modelCode, "reason": reason, "status_code": status,
			"notified": notified, "created_at": createdAt,
		})
	}
	return items, rows.Err()
}
