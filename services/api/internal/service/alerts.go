package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AlertChannel 是外部告警通道（钉钉 / 飞书机器人）。
type AlertChannel struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	WebhookURL   string    `json:"webhook_url"`
	Secret       string    `json:"secret"`
	IsEnabled    bool      `json:"is_enabled"`
	NotifyEvents string    `json:"notify_events"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AlertChannelInput struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	WebhookURL   string `json:"webhook_url"`
	Secret       string `json:"secret"`
	IsEnabled    *bool  `json:"is_enabled"`
	NotifyEvents string `json:"notify_events"`
}

func (s *AdminService) ListAlertChannels(ctx context.Context) ([]AlertChannel, error) {
	rows, err := s.db.Query(ctx, `SELECT id,name,type,webhook_url,COALESCE(secret,''),is_enabled,
		COALESCE(notify_events,''),created_at,updated_at FROM alert_channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AlertChannel{}
	for rows.Next() {
		var c AlertChannel
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.WebhookURL, &c.Secret, &c.IsEnabled, &c.NotifyEvents, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if c.Secret != "" {
			c.Secret = "••••••"
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *AdminService) CreateAlertChannel(ctx context.Context, in AlertChannelInput) (*AlertChannel, error) {
	if strings.TrimSpace(in.WebhookURL) == "" {
		return nil, fmt.Errorf("webhook 地址不能为空")
	}
	channelType := strings.ToLower(strings.TrimSpace(in.Type))
	if channelType != "dingtalk" && channelType != "feishu" {
		return nil, fmt.Errorf("类型只能是 dingtalk 或 feishu")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = channelType
	}
	enabled := true
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}
	events := strings.TrimSpace(in.NotifyEvents)
	if events == "" {
		events = "route_failover,account_down,all_routes_down,manual_migrate"
	}
	var id int64
	if err := s.db.QueryRow(ctx, `INSERT INTO alert_channels (name,type,webhook_url,secret,is_enabled,notify_events)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		name, channelType, strings.TrimSpace(in.WebhookURL), strings.TrimSpace(in.Secret), enabled, events).Scan(&id); err != nil {
		return nil, err
	}
	return s.getAlertChannel(ctx, id)
}

func (s *AdminService) UpdateAlertChannel(ctx context.Context, id int64, in AlertChannelInput) (*AlertChannel, error) {
	current, err := s.getAlertChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	enabled := current.IsEnabled
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = current.Name
	}
	secret := strings.TrimSpace(in.Secret)
	if secret == "" {
		secret = current.Secret
	}
	if secret == "••••••" {
		var stored string
		_ = s.db.QueryRow(ctx, `SELECT COALESCE(secret,'') FROM alert_channels WHERE id=$1`, id).Scan(&stored)
		secret = stored
	}
	events := strings.TrimSpace(in.NotifyEvents)
	if events == "" {
		events = current.NotifyEvents
	}
	channelType := strings.ToLower(strings.TrimSpace(in.Type))
	if channelType == "" {
		channelType = current.Type
	}
	if _, err := s.db.Exec(ctx, `UPDATE alert_channels SET name=$1,type=$2,webhook_url=$3,secret=$4,is_enabled=$5,notify_events=$6,updated_at=now() WHERE id=$7`,
		name, channelType, strings.TrimSpace(in.WebhookURL), secret, enabled, events, id); err != nil {
		return nil, err
	}
	return s.getAlertChannel(ctx, id)
}

func (s *AdminService) DeleteAlertChannel(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM alert_channels WHERE id=$1`, id)
	return err
}

func (s *AdminService) getAlertChannel(ctx context.Context, id int64) (*AlertChannel, error) {
	var c AlertChannel
	if err := s.db.QueryRow(ctx, `SELECT id,name,type,webhook_url,COALESCE(secret,''),is_enabled,
		COALESCE(notify_events,''),created_at,updated_at FROM alert_channels WHERE id=$1`, id).
		Scan(&c.ID, &c.Name, &c.Type, &c.WebhookURL, &c.Secret, &c.IsEnabled, &c.NotifyEvents, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, fmt.Errorf("告警通道不存在")
	}
	if c.Secret != "" {
		c.Secret = "••••••"
	}
	return &c, nil
}

// TestAlertChannel 发送一条测试消息。
func (s *AdminService) TestAlertChannel(ctx context.Context, id int64) error {
	var channelType, webhook, secret string
	if err := s.db.QueryRow(ctx, `SELECT type,webhook_url,COALESCE(secret,'') FROM alert_channels WHERE id=$1`, id).
		Scan(&channelType, &webhook, &secret); err != nil {
		return fmt.Errorf("告警通道不存在")
	}
	return sendAlertMessage(ctx, channelType, webhook, secret, "告警通道测试", "这是一条来自 **上游告警** 的测试消息。")
}

// RunAlertDispatcher 每 30 秒扫描未通知的切换事件，推送到启用的钉钉/飞书机器人。
func (s *AdminService) RunAlertDispatcher(ctx context.Context) {
	run := func() {
		rows, err := s.db.Query(ctx, `SELECT e.id,e.event_type,e.provider,COALESCE(e.account_id,0),COALESCE(e.model_id,0),e.model_code,
			COALESCE(e.from_route_id,0),COALESCE(e.to_route_id,0),e.reason,COALESCE(e.status_code,0),e.created_at,
			COALESCE(a.name,''),COALESCE(a.group_name,''),COALESCE(a.login_account,''),
			COALESCE(m.display_name,''),COALESCE(rf.route_name,''),COALESCE(rt.route_name,'')
			FROM upstream_events e
			LEFT JOIN upstream_accounts a ON a.id = e.account_id
			LEFT JOIN models m ON m.id = e.model_id
			LEFT JOIN model_routes rf ON rf.id = e.from_route_id
			LEFT JOIN model_routes rt ON rt.id = e.to_route_id
			WHERE e.notified=false AND e.created_at > now()-interval '1 day'
			ORDER BY e.id LIMIT 50`)
		if err != nil {
			return
		}
		type eventRow struct {
			id, accountID, modelID, fromRoute, toRoute          int64
			eventType, provider, modelCode, reason              string
			accountName, accountGroup, accountLogin             string
			modelName, fromRouteName, toRouteName               string
			status                                              int
			createdAt                                           time.Time
		}
		events := []eventRow{}
		for rows.Next() {
			var e eventRow
			if err := rows.Scan(&e.id, &e.eventType, &e.provider, &e.accountID, &e.modelID, &e.modelCode,
				&e.fromRoute, &e.toRoute, &e.reason, &e.status, &e.createdAt,
				&e.accountName, &e.accountGroup, &e.accountLogin, &e.modelName, &e.fromRouteName, &e.toRouteName); err != nil {
				continue
			}
			events = append(events, e)
		}
		rows.Close()
		if len(events) == 0 {
			return
		}
		channels, err := s.ListAlertChannels(ctx)
		if err != nil || len(channels) == 0 {
			return
		}
		enabled := []AlertChannel{}
		for _, c := range channels {
			if c.IsEnabled {
				enabled = append(enabled, c)
			}
		}
		if len(enabled) == 0 {
			return
		}
		for _, e := range events {
			for _, c := range enabled {
				if !strings.Contains(c.NotifyEvents, e.eventType) {
					continue
				}
				title, text := buildAlertMessage(alertMessageInput{
					EventType: e.eventType, Provider: e.provider, ModelCode: e.modelCode, ModelName: e.modelName,
					AccountName: e.accountName, AccountGroup: e.accountGroup, AccountLogin: e.accountLogin,
					FromRouteName: e.fromRouteName, ToRouteName: e.toRouteName,
					Reason: e.reason, Status: e.status, At: e.createdAt,
				})
				var secret string
				_ = s.db.QueryRow(ctx, `SELECT COALESCE(secret,'') FROM alert_channels WHERE id=$1`, c.ID).Scan(&secret)
				if err := sendAlertMessage(ctx, c.Type, c.WebhookURL, secret, title, text); err != nil {
					log.Printf("alert send failed channel=%d: %v", c.ID, err)
				}
			}
			_, _ = s.db.Exec(ctx, `UPDATE upstream_events SET notified=true, notified_at=now() WHERE id=$1`, e.id)
		}
	}
	go func() {
		run()
		ticker := time.NewTicker(30 * time.Second)
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

type alertMessageInput struct {
	EventType, Provider, ModelCode, ModelName       string
	AccountName, AccountGroup, AccountLogin          string
	FromRouteName, ToRouteName                       string
	Reason                                           string
	Status                                           int
	At                                               time.Time
}

func buildAlertMessage(in alertMessageInput) (string, string) {
	var title string
	switch in.EventType {
	case "route_failover":
		title = "线路切换告警"
	case "account_down":
		title = "上游账号不可用"
	case "all_routes_down":
		title = "所有线路不可用"
	case "manual_migrate":
		title = "手动批量迁移"
	default:
		title = "上游告警"
	}
	lines := []string{"### ⚠️ " + title}
	if in.AccountName != "" {
		lines = append(lines, "- 账号："+in.AccountName)
	}
	groupLine := ""
	if in.AccountGroup != "" {
		groupLine = "- 分组：" + in.AccountGroup
		if in.AccountLogin != "" {
			groupLine += "（登录账号 " + in.AccountLogin + "）"
		}
	} else if in.AccountLogin != "" {
		groupLine = "- 登录账号：" + in.AccountLogin
	}
	if groupLine != "" {
		lines = append(lines, groupLine)
	} else if in.Provider != "" {
		lines = append(lines, "- 上游："+in.Provider)
	}
	if in.ModelCode != "" {
		model := "- 模型：" + in.ModelCode
		if in.ModelName != "" && in.ModelName != in.ModelCode {
			model += "（" + in.ModelName + "）"
		}
		lines = append(lines, model)
	} else if in.Provider != "" && len(lines) < 3 {
		lines = append(lines, "- 上游："+in.Provider)
	}
	if in.FromRouteName != "" && in.ToRouteName != "" && in.FromRouteName != in.ToRouteName {
		lines = append(lines, "- 线路："+in.FromRouteName+" → "+in.ToRouteName)
	} else if in.FromRouteName != "" {
		lines = append(lines, "- 线路："+in.FromRouteName)
	}
	if in.Status > 0 {
		lines = append(lines, "- 状态码：`"+strconv.Itoa(in.Status)+"`")
	}
	if in.Reason != "" {
		lines = append(lines, "- 原因："+in.Reason)
	}
	lines = append(lines, "- 时间："+in.At.Format("2006-01-02 15:04:05"))
	return title, strings.Join(lines, "\n")
}

func sendAlertMessage(ctx context.Context, channelType, webhook, secret, title, markdown string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	var body map[string]interface{}
	switch channelType {
	case "dingtalk":
		body = map[string]interface{}{
			"msgtype":  "markdown",
			"markdown": map[string]string{"title": title, "text": markdown},
		}
		if secret != "" {
			timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
			sign := dingTalkSign(timestamp, secret)
			webhook = webhook + "&timestamp=" + timestamp + "&sign=" + url.QueryEscape(sign)
		}
	case "feishu":
		// 飞书：interactive 卡片 + lark_md，能正确渲染 markdown
		body = map[string]interface{}{
			"msg_type": "interactive",
			"card": map[string]interface{}{
				"config": map[string]interface{}{"wide_screen_mode": true},
				"header": map[string]interface{}{
					"title":    map[string]string{"tag": "plain_text", "content": title},
					"template": "orange",
				},
				"elements": []interface{}{
					map[string]interface{}{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": markdown}},
				},
			},
		}
		if secret != "" {
			timestamp := strconv.FormatInt(time.Now().Unix(), 10)
			body["timestamp"] = timestamp
			body["sign"] = feishuSign(timestamp, secret)
		}
	default:
		return fmt.Errorf("不支持的告警类型 %s", channelType)
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func dingTalkSign(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func feishuSign(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	mac.Write([]byte{})
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
