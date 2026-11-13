package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ---------- CodexAuv ----------

type codexAuvSyncAdapter struct{}

func (codexAuvSyncAdapter) Name() string { return "codexauv" }

func (codexAuvSyncAdapter) headers(cred string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(cred),
		"Accept":        "application/json",
		"User-Agent":    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36",
	}
}

func (a codexAuvSyncAdapter) FetchGroups(ctx context.Context, src UpstreamSyncSource, cred string) ([]syncGroup, error) {
	raw, _, err := syncHTTPDo(ctx, "GET", syncJoinURL(src.SiteBaseURL, "/api/v1/groups/available"), a.headers(cred), nil, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data []struct {
			ID   int64   `json:"id"`
			Name string  `json:"name"`
			Rate float64 `json:"rate_multiplier"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	groups := []syncGroup{}
	for _, g := range payload.Data {
		if strings.TrimSpace(g.Name) == "" {
			continue
		}
		groups = append(groups, syncGroup{ID: strconv.FormatInt(g.ID, 10), Name: strings.TrimSpace(g.Name), Rate: g.Rate})
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("站点未返回任何分组")
	}
	return groups, nil
}

func (a codexAuvSyncAdapter) FetchKeys(ctx context.Context, src UpstreamSyncSource, cred string) ([]syncKey, error) {
	raw, _, err := syncHTTPDo(ctx, "GET", syncJoinURL(src.SiteBaseURL, "/api/v1/keys?page=1&page_size=200&timezone=Asia/Shanghai"), a.headers(cred), nil, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Items []struct {
				ID    int64  `json:"id"`
				Key   string `json:"key"`
				Name  string `json:"name"`
				Group struct {
					Name string `json:"name"`
				} `json:"group"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	keys := make([]syncKey, 0, len(payload.Data.Items))
	for _, item := range payload.Data.Items {
		group := strings.TrimSpace(item.Group.Name)
		keys = append(keys, syncKey{ID: strconv.FormatInt(item.ID, 10), Group: group, Name: item.Name, Key: item.Key})
	}
	return keys, nil
}

func (a codexAuvSyncAdapter) CreateKey(ctx context.Context, src UpstreamSyncSource, cred string, group syncGroup) (*syncKey, error) {
	groupID, _ := strconv.ParseInt(group.ID, 10, 64)
	body, _ := json.Marshal(map[string]interface{}{"name": group.Name, "group_id": groupID})
	raw, _, err := syncHTTPDo(ctx, "POST", syncJoinURL(src.SiteBaseURL, "/api/v1/keys"), a.headers(cred), body, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.Data.Key) == "" {
		return nil, fmt.Errorf("站点未返回 Key 明文")
	}
	return &syncKey{Group: group.Name, Name: group.Name, Key: payload.Data.Key}, nil
}

func (a codexAuvSyncAdapter) FetchModels(ctx context.Context, src UpstreamSyncSource, key string) ([]string, error) {
	raw, _, err := syncHTTPDo(ctx, "GET", syncJoinURL(src.SiteBaseURL, "/v1/models"), map[string]string{"Authorization": "Bearer " + key}, nil, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(payload.Data))
	for _, m := range payload.Data {
		out = append(out, m.ID)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("模型清单为空")
	}
	return out, nil
}

// ---------- new-api 系（PackyCode 等） ----------

type newAPISyncAdapter struct{}

func (newAPISyncAdapter) Name() string { return "new_api" }

func (a newAPISyncAdapter) headers(src UpstreamSyncSource, cred string) map[string]string {
	userID := strings.TrimSpace(fmt.Sprint(src.Config["user_id"]))
	if userID == "" || userID == "<nil>" {
		userID = "0"
	}
	return map[string]string{
		"Cookie":       strings.TrimSpace(cred),
		"New-Api-User": userID,
		"Accept":       "application/json",
	}
}

func (a newAPISyncAdapter) FetchGroups(ctx context.Context, src UpstreamSyncSource, cred string) ([]syncGroup, error) {
	raw, _, err := syncHTTPDo(ctx, "GET", syncJoinURL(src.SiteBaseURL, "/api/user/self/groups"), a.headers(src, cred), nil, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data map[string]struct {
			Ratio   float64 `json:"ratio"`
			Desc    string  `json:"desc"`
			Inactive bool   `json:"inactive"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	groups := []syncGroup{}
	for name, info := range payload.Data {
		if strings.TrimSpace(name) == "" || info.Inactive {
			continue
		}
		groups = append(groups, syncGroup{ID: name, Name: strings.TrimSpace(name), Rate: info.Ratio})
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("站点未返回任何分组")
	}
	return groups, nil
}

func (a newAPISyncAdapter) FetchKeys(ctx context.Context, src UpstreamSyncSource, cred string) ([]syncKey, error) {
	raw, _, err := syncHTTPDo(ctx, "GET", syncJoinURL(src.SiteBaseURL, "/api/token/?p=1&size=100"), a.headers(src, cred), nil, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Items []struct {
				ID     int64  `json:"id"`
				Key    string `json:"key"`
				Name   string `json:"name"`
				Group  string `json:"group"`
				Status string `json:"status"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	keys := make([]syncKey, 0, len(payload.Data.Items))
	for _, item := range payload.Data.Items {
		group := strings.TrimSpace(item.Group)
		if group == "" {
			group = strings.TrimSpace(item.Name)
		}
		keys = append(keys, syncKey{ID: strconv.FormatInt(item.ID, 10), Group: group, Name: item.Name, Key: item.Key})
	}
	return keys, nil
}

func (a newAPISyncAdapter) CreateKey(ctx context.Context, src UpstreamSyncSource, cred string, group syncGroup) (*syncKey, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"name":                 group.Name,
		"group":                group.Name,
		"unlimited_quota":      true,
		"expired_time":         -1,
		"remain_quota":         0,
		"model_limits_enabled": false,
		"model_limits":         "",
		"allow_ips":            "",
	})
	raw, _, err := syncHTTPDo(ctx, "POST", syncJoinURL(src.SiteBaseURL, "/api/token/"), a.headers(src, cred), body, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if !payload.Success || strings.TrimSpace(payload.Data.Key) == "" {
		return nil, fmt.Errorf("站点未返回 Key 明文")
	}
	return &syncKey{Group: group.Name, Name: group.Name, Key: payload.Data.Key}, nil
}

func (a newAPISyncAdapter) FetchModels(ctx context.Context, src UpstreamSyncSource, key string) ([]string, error) {
	raw, _, err := syncHTTPDo(ctx, "GET", syncJoinURL(src.SiteBaseURL, "/v1/models"), map[string]string{"Authorization": "Bearer " + key}, nil, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(payload.Data))
	for _, m := range payload.Data {
		out = append(out, m.ID)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("模型清单为空")
	}
	return out, nil
}

func init() {
	registerSyncAdapter(codexAuvSyncAdapter{})
	registerSyncAdapter(newAPISyncAdapter{})
}
