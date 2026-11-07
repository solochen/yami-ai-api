package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

// Douyin blocks anonymous requests to its web detail API behind signed
// parameters, so a plain link (especially a v.douyin.com share link) cannot be
// turned into a playable file by fetching the page or by yt-dlp alone. The
// self-hostable Douyin_TikTok_Download_API service (also used by the worker)
// resolves share links to direct media URLs; when it is unreachable we still try
// the public share page first, then the caller falls back to yt-dlp.
const defaultDouyinAPIBase = "https://api.douyin.wtf"

const douyinParsePollInterval = time.Second
const douyinParsePollTimeout = 45 * time.Second

func douyinAPIBase() string {
	if base := strings.TrimSpace(os.Getenv("DOUYIN_API_BASE")); base != "" {
		return strings.TrimRight(base, "/")
	}
	// Legacy setting used by older deployments: a full resolver endpoint such as
	// https://host/api/douyin/web/fetch_one_video. Only the origin is reusable.
	if legacy := strings.TrimSpace(os.Getenv("DOUYIN_RESOLVER_URL")); legacy != "" {
		if parsed, err := url.Parse(legacy); err == nil && parsed.Host != "" {
			return parsed.Scheme + "://" + parsed.Host
		}
	}
	return defaultDouyinAPIBase
}

func douyinAPIKey() string {
	return strings.TrimSpace(os.Getenv("DOUYIN_API_KEY"))
}

// resolveDouyinVideoURL asks the configured parser service for a direct media
// URL. It logs in with the service demo account when no API key is configured,
// which is how the public instance is meant to be used.
func resolveDouyinVideoURL(ctx context.Context, client *http.Client, shareURL string) (string, error) {
	base := douyinAPIBase()
	endpoint, err := validateImportURL(base + "/api/v1/parse")
	if err != nil {
		return "", fmt.Errorf("抖音解析服务地址无效")
	}
	headers := map[string]string{"Content-Type": "application/json"}
	if key := douyinAPIKey(); key != "" {
		headers["X-API-Key"] = key
	}
	body, err := json.Marshal(map[string]string{"url": shareURL})
	if err != nil {
		return "", fmt.Errorf("抖音解析请求无效")
	}
	status, payload, err := douyinServiceJSON(ctx, client, http.MethodPost, endpoint.String(), body, headers)
	if err != nil {
		return "", fmt.Errorf("抖音解析服务不可用")
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		if douyinAPIKey() != "" {
			return "", fmt.Errorf("抖音解析服务鉴权失败，请检查 DOUYIN_API_KEY")
		}
		if err := douyinDemoLogin(ctx, client, base); err != nil {
			return "", err
		}
		status, payload, err = douyinServiceJSON(ctx, client, http.MethodPost, endpoint.String(), body, headers)
		if err != nil {
			return "", fmt.Errorf("抖音解析服务不可用")
		}
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("抖音解析服务返回 HTTP %d", status)
	}
	if link := douyinMediaURL(payload); link != "" {
		return link, nil
	}
	taskID := nestedStringValue(payload, "data", "task_id")
	if taskID == "" {
		return "", fmt.Errorf("抖音解析服务没有返回视频地址")
	}
	return pollDouyinParseTask(ctx, client, base, taskID, headers)
}

func douyinDemoLogin(ctx context.Context, client *http.Client, base string) error {
	endpoint, err := validateImportURL(base + "/api/v1/auth/demo")
	if err != nil {
		return fmt.Errorf("抖音解析服务地址无效")
	}
	status, payload, err := douyinServiceJSON(ctx, client, http.MethodGet, endpoint.String(), nil, nil)
	if err != nil || status < 200 || status >= 300 {
		return fmt.Errorf("抖音解析服务不可用")
	}
	data, _ := payload["data"].(map[string]interface{})
	username := nestedStringValue(data, "username")
	password := nestedStringValue(data, "password")
	if username == "" || password == "" {
		return fmt.Errorf("抖音解析服务需要配置 DOUYIN_API_KEY")
	}
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	loginEndpoint, err := validateImportURL(base + "/api/v1/auth/login")
	if err != nil {
		return fmt.Errorf("抖音解析服务地址无效")
	}
	status, _, err = douyinServiceJSON(ctx, client, http.MethodPost, loginEndpoint.String(), body, map[string]string{"Content-Type": "application/json"})
	if err != nil || status < 200 || status >= 300 {
		return fmt.Errorf("抖音解析服务登录失败，请配置 DOUYIN_API_KEY")
	}
	return nil
}

func pollDouyinParseTask(ctx context.Context, client *http.Client, base, taskID string, headers map[string]string) (string, error) {
	endpoint, err := validateImportURL(base + "/api/v1/tasks/" + url.PathEscape(taskID))
	if err != nil {
		return "", fmt.Errorf("抖音解析服务地址无效")
	}
	deadline := time.Now().Add(douyinParsePollTimeout)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		status, payload, err := douyinServiceJSON(ctx, client, http.MethodGet, endpoint.String(), nil, headers)
		if err != nil {
			return "", fmt.Errorf("抖音解析服务不可用")
		}
		if status < 200 || status >= 300 {
			return "", fmt.Errorf("抖音解析服务返回 HTTP %d", status)
		}
		if link := douyinMediaURL(payload); link != "" {
			return link, nil
		}
		state := strings.ToLower(nestedStringValue(payload, "data", "state"))
		if state == "failed" || state == "error" || state == "done" || state == "succeeded" {
			return "", fmt.Errorf("抖音解析服务没有返回视频地址")
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("抖音解析超时")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(douyinParsePollInterval):
		}
	}
}

func douyinServiceJSON(ctx context.Context, client *http.Client, method, endpoint string, body []byte, headers map[string]string) (int, map[string]interface{}, error) {
	var reader io.Reader
	if len(body) > 0 {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", importBrowserUserAgent)
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxImportedPageBytes+1))
	if err != nil || int64(len(payload)) > maxImportedPageBytes {
		return resp.StatusCode, nil, fmt.Errorf("抖音解析结果无效")
	}
	var value map[string]interface{}
	if err := json.Unmarshal(payload, &value); err != nil {
		return resp.StatusCode, nil, fmt.Errorf("抖音解析结果无效")
	}
	return resp.StatusCode, value, nil
}

// douyinMediaURL reads the media URL out of the parser response. The service
// returns either a finished parse ({media:{video:{url}}}) or a task wrapper, and
// both shapes are searched because the schema differs between versions.
func douyinMediaURL(value interface{}) string {
	if item, ok := value.(map[string]interface{}); ok {
		if media, ok := item["media"].(map[string]interface{}); ok {
			if video, ok := media["video"].(map[string]interface{}); ok {
				if link := firstVideoHTTPURL(video["url"]); link != "" {
					return link
				}
				if urls, ok := video["urls"].([]interface{}); ok {
					for _, candidate := range urls {
						if link := firstVideoHTTPURL(candidate); link != "" {
							return link
						}
					}
				}
			}
		}
		for _, key := range []string{"play_addr", "playAddr", "play_addr_h264", "video_url", "videoUrl"} {
			if link := keyedVideoURL(item, key); link != "" {
				return link
			}
		}
		for _, key := range []string{"data", "result", "task", "items", "video"} {
			if nested, exists := item[key]; exists {
				if link := douyinMediaURL(nested); link != "" {
					return link
				}
			}
		}
	}
	if items, ok := value.([]interface{}); ok {
		for _, item := range items {
			if link := douyinMediaURL(item); link != "" {
				return link
			}
		}
	}
	return ""
}

func keyedVideoURL(value interface{}, wanted string) string {
	switch item := value.(type) {
	case map[string]interface{}:
		for key, nested := range item {
			if strings.EqualFold(key, wanted) {
				if link := firstVideoHTTPURL(nested); link != "" {
					return link
				}
			}
		}
		for _, nested := range item {
			if link := keyedVideoURL(nested, wanted); link != "" {
				return link
			}
		}
	case []interface{}:
		for _, nested := range item {
			if link := keyedVideoURL(nested, wanted); link != "" {
				return link
			}
		}
	}
	return ""
}

// firstVideoHTTPURL walks candidate values and ignores image URLs: parser
// responses also carry covers and avatars, which must not be mistaken for the
// playable file.
func firstVideoHTTPURL(value interface{}) string {
	switch item := value.(type) {
	case string:
		for _, candidate := range append([]string{item}, strings.Fields(item)...) {
			if parsed, err := validateImportURL(candidate); err == nil && !imageImportPath(parsed.Path) {
				return parsed.String()
			}
		}
	case []interface{}:
		for _, nested := range item {
			if candidate := firstVideoHTTPURL(nested); candidate != "" {
				return candidate
			}
		}
	case map[string]interface{}:
		for _, key := range []string{"UrlList", "urlList", "url_list", "url", "src"} {
			if nested, ok := item[key]; ok {
				if candidate := firstVideoHTTPURL(nested); candidate != "" {
					return candidate
				}
			}
		}
	}
	return ""
}

func imageImportPath(rawPath string) bool {
	switch strings.ToLower(path.Ext(rawPath)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".avif", ".heic", ".bmp", ".svg":
		return true
	}
	return false
}

func nestedStringValue(value interface{}, keys ...string) string {
	var current interface{} = value
	for _, key := range keys {
		item, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = item[key]
	}
	text, _ := current.(string)
	return strings.TrimSpace(text)
}

var douyinRouterDataPattern = regexp.MustCompile(`(?s)window\._ROUTER_DATA\s*=\s*(\{.*?\})\s*;?\s*</script>`)
var douyinSSRDataPattern = regexp.MustCompile(`(?s)window\._SSR_DATA\s*=\s*(\{.*?\})\s*;?\s*</script>`)
var douyinRenderDataPattern = regexp.MustCompile(`(?s)<script[^>]*id="RENDER_DATA"[^>]*>(.*?)</script>`)

// extractDouyinSharePageVideoURL parses the server-rendered share page. It works
// for older share links that still embed the item; newer pages often omit the
// playable address, in which case the parser service path is used instead.
func extractDouyinSharePageVideoURL(page []byte) string {
	source := string(page)
	for _, pattern := range []*regexp.Regexp{douyinRouterDataPattern, douyinSSRDataPattern} {
		match := pattern.FindStringSubmatch(source)
		if len(match) != 2 {
			continue
		}
		var value interface{}
		if json.Unmarshal([]byte(match[1]), &value) != nil {
			continue
		}
		if link := douyinPageVideoURL(value); link != "" {
			return link
		}
	}
	if match := douyinRenderDataPattern.FindStringSubmatch(source); len(match) == 2 {
		if decoded, err := url.QueryUnescape(strings.TrimSpace(match[1])); err == nil {
			var value interface{}
			if json.Unmarshal([]byte(decoded), &value) == nil {
				if link := douyinPageVideoURL(value); link != "" {
					return link
				}
			}
		}
	}
	return ""
}

func douyinPageVideoURL(value interface{}) string {
	if item, ok := value.(map[string]interface{}); ok {
		if video, ok := item["video"].(map[string]interface{}); ok {
			for _, key := range []string{"play_addr", "playAddr", "play_addr_h264", "playApi", "play_api"} {
				if link := firstVideoHTTPURL(video[key]); link != "" {
					return link
				}
			}
			if link := keyedVideoURL(video, "play_addr"); link != "" {
				return link
			}
		}
		for _, nested := range item {
			if link := douyinPageVideoURL(nested); link != "" {
				return link
			}
		}
	}
	if items, ok := value.([]interface{}); ok {
		for _, item := range items {
			if link := douyinPageVideoURL(item); link != "" {
				return link
			}
		}
	}
	return ""
}

func resolveDouyinSharePageVideo(ctx context.Context, client *http.Client, awemeID string) (string, error) {
	pageURL := "https://www.iesdouyin.com/share/video/" + url.PathEscape(awemeID) + "/"
	resp, err := getImportURL(ctx, client, pageURL, "https://www.douyin.com/")
	if err != nil {
		return "", fmt.Errorf("抖音分享页不可用")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("抖音分享页返回 HTTP %d", resp.StatusCode)
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, maxImportedPageBytes+1))
	if err != nil || int64(len(page)) > maxImportedPageBytes {
		return "", fmt.Errorf("抖音分享页读取失败")
	}
	if link := extractDouyinSharePageVideoURL(page); link != "" {
		return link, nil
	}
	return "", fmt.Errorf("抖音分享页没有内嵌视频地址")
}
