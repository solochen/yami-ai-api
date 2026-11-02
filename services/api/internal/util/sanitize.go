package util

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
// 这类文本一旦经 API 返回给终端用户，就等于暴露平台用的是哪家上游。所有面向用户的
// 错误必须先经过 SanitizeMessage；日志与后台排障继续用原始错误，不要在这里清洗。
var (
	transportErrPrefix    = regexp.MustCompile(`(?i)^\s*(?:post|get|put|patch|delete|head|options)\s+"[^"]*"\s*:\s*`)
	upstreamURLPattern    = regexp.MustCompile(`(?i)(?:https?|wss?)://[^\s"'<>()，。；、）】]+`)
	upstreamHostPattern   = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+(?:com|cn|net|org|io|ai|cc|xyz|top|dev|me|info|app|cloud|site|shop|vip|pro|tech|online|link|space|fun)(?::\d{1,5})?\b`)
	upstreamIPv4Pattern   = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}(?::\d{1,5})?\b`)
	upstreamVendorPattern = regexp.MustCompile(`(?i)\b(?:openrouter|new-?api|newapi|yardai|zexapi|otuapi|bigmodel|volcengine|volces|siliconflow|dashscope|anthropic|openai|azure|gemini|minimax|vidu|runway|luma|kling|doubao|seedream|seedance|qwen|glm|deepseek|moonshot|kimi|hunyuan|replicate|stability)(?:[-_][a-z0-9][a-z0-9._]*)*`)
)

// SanitizeMessage 把上游地址、域名、IP、厂商名从面向用户的错误里抹掉。幂等。
func SanitizeMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	msg = transportErrPrefix.ReplaceAllString(msg, "")
	msg = upstreamURLPattern.ReplaceAllString(msg, upstreamRedaction)
	msg = upstreamHostPattern.ReplaceAllString(msg, upstreamRedaction)
	msg = upstreamIPv4Pattern.ReplaceAllString(msg, upstreamRedaction)
	msg = upstreamVendorPattern.ReplaceAllString(msg, upstreamRedaction)
	msg = strings.Join(strings.Fields(msg), " ")
	if runes := []rune(msg); len(runes) > 200 {
		msg = string(runes[:200]) + "..."
	}
	return msg
}

// userFacingErrorKeys 是 outputs / node_runs 等 JSON 里承载错误文案的键。
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

// upstreamOnlyKeys 只服务于内部链路，不能回给终端用户。
var upstreamOnlyKeys = map[string]bool{
	"upstream_content_url": true,
	"upstream_model":       true,
}

// SanitizeJSON 递归清洗 JSON 结构里的错误文案，并移除仅内部使用的上游字段。
// 用于兜底历史脏数据：已被写入 DB 的旧错误文案在这里再洗一次。
func SanitizeJSON(value interface{}) interface{} {
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
					item[key] = SanitizeMessage(text)
					continue
				}
			}
			item[key] = SanitizeJSON(raw)
		}
		return item
	case map[string]string:
		for key, raw := range item {
			lower := strings.ToLower(key)
			if upstreamOnlyKeys[lower] {
				delete(item, key)
				continue
			}
			if userFacingErrorKeys[lower] {
				item[key] = SanitizeMessage(raw)
			}
		}
		return item
	case []interface{}:
		for i := range item {
			item[i] = SanitizeJSON(item[i])
		}
		return item
	default:
		return value
	}
}

// SanitizeErrorText 处理 *string 形式的错误字段（DTO 里常见）。
func SanitizeErrorText(msg *string) *string {
	if msg == nil {
		return nil
	}
	clean := SanitizeMessage(*msg)
	return &clean
}
