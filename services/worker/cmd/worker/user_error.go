package main

import (
	"regexp"
	"strings"
)

// upstreamRedaction is what replaces an upstream address in user-facing text.
const upstreamRedaction = "上游服务"

// 上游调用失败时，net/http 的 *url.Error 和网关返回体会把上游地址直接带进文案：
//
//	Post "https://hub.example.com/v1/images/generations": unexpected EOF
//
// 这些文案会写进 tasks.error_message、workflow_projects.error_message、outputs 和站内通知，
// 前端原样展示给终端用户，等于把平台用的是哪家上游暴露出去。所有面向用户的错误
// 必须先经过 sanitizeUserFacingError，日志侧不要用这个函数（排障需要真实地址）。
var (
	// transportErrPrefix 剥掉 *url.Error 的方法+URL 前缀，只留真正的失败原因。
	transportErrPrefix = regexp.MustCompile(`(?i)^\s*(?:post|get|put|patch|delete|head|options)\s+"[^"]*"\s*:\s*`)
	// upstreamURLPattern 匹配带协议的地址。
	upstreamURLPattern = regexp.MustCompile(`(?i)(?:https?|wss?)://[^\s"'<>()，。；、）】]+`)
	// upstreamHostPattern 匹配裸域名（含端口）。
	upstreamHostPattern = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+(?:com|cn|net|org|io|ai|cc|xyz|top|dev|me|info|app|cloud|site|shop|vip|pro|tech|online|link|space|fun)(?::\d{1,5})?\b`)
	// upstreamIPv4Pattern 匹配裸 IP（含端口）。
	upstreamIPv4Pattern = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}(?::\d{1,5})?\b`)
	// upstreamVendorPattern 匹配上游厂商名及其模型代号（如 gemini-3-pro-image）。
	upstreamVendorPattern = regexp.MustCompile(`(?i)\b(?:openrouter|new-?api|newapi|yardai|zexapi|otuapi|bigmodel|volcengine|volces|siliconflow|dashscope|anthropic|openai|azure|gemini|minimax|vidu|runway|luma|kling|doubao|seedream|seedance|qwen|glm|deepseek|moonshot|kimi|hunyuan|replicate|stability)(?:[-_][a-z0-9][a-z0-9._]*)*`)
)

// sanitizeUserFacingError 把上游地址、域名、IP、厂商名从面向用户的错误里抹掉，
// 并压成单行、截断到 200 字。幂等，可以重复调用。
func sanitizeUserFacingError(msg string) string {
	msg = redactUpstreamText(msg)
	if runes := []rune(msg); len(runes) > 200 {
		msg = string(runes[:200]) + "..."
	}
	return msg
}

// redactUpstreamText 只做脱敏与空白压缩，不截断；用于需要保留完整多线路汇总的场合。
func redactUpstreamText(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	msg = transportErrPrefix.ReplaceAllString(msg, "")
	msg = upstreamURLPattern.ReplaceAllString(msg, upstreamRedaction)
	msg = upstreamHostPattern.ReplaceAllString(msg, upstreamRedaction)
	msg = upstreamIPv4Pattern.ReplaceAllString(msg, upstreamRedaction)
	msg = upstreamVendorPattern.ReplaceAllString(msg, upstreamRedaction)
	return strings.Join(strings.Fields(msg), " ")
}

// userFacingErrorKeys 是 outputs / node_runs 等 JSON 里承载错误文案的键，
// 值需要在落库前清洗；其余键（媒体 URL 等）原样保留。
var userFacingErrorKeys = map[string]bool{
	"error":            true,
	"error_message":    true,
	"error_detail":     true,
	"last_error":       true,
	"compose_error":    true,
	"fail_reason":      true,
	"detail":           true,
	"reason":           true,
	"progress_message": true,
	"status_message":   true,
}

// upstreamOnlyKeys 只服务于内部链路，不能让终端用户看到，直接移除。
var upstreamOnlyKeys = map[string]bool{
	"upstream_content_url": true,
	"upstream_model":       true,
}

// sanitizeUserFacingJSON 递归清洗 JSON 结构里的错误文案，并移除仅内部使用的上游字段。
// 原地修改并返回同一个值，便于链式调用。
func sanitizeUserFacingJSON(value interface{}) interface{} {
	switch item := value.(type) {
	case map[string]interface{}:
		for key, raw := range item {
			lower := strings.ToLower(key)
			if upstreamOnlyKeys[lower] {
				delete(item, key)
				continue
			}
			if userFacingErrorKeys[lower] {
				if text, ok := raw.(string); ok {
					item[key] = sanitizeUserFacingError(text)
					continue
				}
			}
			item[key] = sanitizeUserFacingJSON(raw)
		}
		return item
	case []interface{}:
		for i := range item {
			item[i] = sanitizeUserFacingJSON(item[i])
		}
		return item
	default:
		return value
	}
}
