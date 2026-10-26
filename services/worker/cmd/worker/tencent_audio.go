package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// 腾讯云 TokenHub 音频适配器。
//
// 背景：TokenHub 只公开了文本/图像/视频/3D 的接口文档，音频（语音合成、音乐生成）
// 没有公开文档；但本平台这几个音频模型的参数 schema 就是 MiniMax 原生参数
// （voice_id / model_version / output_format: hex|url / is_instrumental / lyrics_optimizer），
// 因此按 MiniMax 原生请求体构造，并对响应做规范化：
//
//   - MiniMax 风格：{"data":{"audio":"<hex>","status":2}}（output_format=hex）
//     → hex 解码后转 base64 回填，避免平台把 hex 串误当 base64 解析出乱码；
//   - {"data":{"audio":"<url>"}}（output_format=url）→ 直接回传 URL；
//   - OpenAI 风格：{"audio":{"data":"<base64>"}} 或裸二进制音频 → 转成统一结构。
//
// 规范化后的响应体形如 {"data":[{"b64_json":"...","media_type":"audio/mpeg"}]}，
// 交给平台既有的媒体结果解析流程（storeBase64MediaResult / persistGeneratedMedia）。
//
// 用法：线路 runtime_rule 配 {"upstream":{"adapter":"tencent_audio"}}，
// endpoint 配 /v1/t2a_v2（语音合成）或 /v1/music_generation（音乐生成），
// upstream_model 配 TokenHub 的模型 ID（minimax-speech-2.8-hd / minimax-music-v3.0）。

func isTencentAudioAdapter(runtimeRule map[string]interface{}) bool {
	upstream, _ := runtimeRule["upstream"].(map[string]interface{})
	adapter := strings.ToLower(strings.TrimSpace(stringAny(upstream["adapter"])))
	return adapter == "tencent_audio"
}

// buildTencentAudioPayload 把平台的输入映射成 TokenHub（MiniMax 原生）请求体。
func buildTencentAudioPayload(modelCode, upstreamModel, prompt string, input map[string]interface{}) map[string]interface{} {
	model := nonEmptyWorker(upstreamModel, modelCode)
	text := strings.TrimSpace(prompt)
	if text == "" {
		text = strings.TrimSpace(stringAny(input["text"]))
	}
	if strings.TrimSpace(stringAny(input["model_version"])) != "" {
		model = strings.TrimSpace(stringAny(input["model_version"]))
	}

	if isTencentMusicModel(modelCode, input) {
		payload := map[string]interface{}{
			"model":          model,
			"prompt":         text,
			"output_format":  nonEmptyWorker(stringAny(input["output_format"]), "hex"),
			"stream":         false,
			"audio_setting":  tencentAudioSetting(input),
			"is_instrumental": boolAnyDefault(input["is_instrumental"], false),
			"lyrics_optimizer": boolAnyDefault(input["lyrics_optimizer"], false),
			"aigc_watermark": boolAnyDefault(input["aigc_watermark"], false),
		}
		if lyrics := strings.TrimSpace(stringAny(input["lyrics"])); lyrics != "" {
			payload["lyrics"] = lyrics
		}
		return payload
	}

	voice := map[string]interface{}{}
	if id := strings.TrimSpace(stringAny(input["voice_id"])); id != "" {
		voice["voice_id"] = id
	}
	for _, key := range []string{"speed", "vol", "pitch", "emotion"} {
		if value, ok := input[key]; ok && value != nil && stringAny(value) != "auto" {
			voice[key] = value
		}
	}
	payload := map[string]interface{}{
		"model":         model,
		"text":          text,
		"stream":        false,
		"output_format": nonEmptyWorker(stringAny(input["output_format"]), "hex"),
		"voice_setting": voice,
		"audio_setting": tencentAudioSetting(input),
	}
	if boost := strings.TrimSpace(stringAny(input["language_boost"])); boost != "" && boost != "auto" {
		payload["language_boost"] = boost
	}
	if value, ok := input["subtitle_enable"]; ok {
		payload["subtitle_enable"] = boolAnyDefault(value, false)
	}
	if value, ok := input["aigc_watermark"]; ok {
		payload["aigc_watermark"] = boolAnyDefault(value, false)
	}
	return payload
}

func isTencentMusicModel(modelCode string, input map[string]interface{}) bool {
	lowered := strings.ToLower(modelCode)
	if strings.Contains(lowered, "music") {
		return true
	}
	if _, ok := input["is_instrumental"]; ok {
		return true
	}
	if _, ok := input["lyrics_optimizer"]; ok {
		return true
	}
	return false
}

func tencentAudioSetting(input map[string]interface{}) map[string]interface{} {
	setting := map[string]interface{}{}
	if value, ok := input["sample_rate"]; ok && intAny(value) > 0 {
		setting["sample_rate"] = intAny(value)
	}
	if value, ok := input["bitrate"]; ok && intAny(value) > 0 {
		setting["bitrate"] = intAny(value)
	}
	if format := strings.TrimSpace(stringAny(input["format"])); format != "" {
		setting["format"] = format
	}
	if value, ok := input["channel"]; ok && intAny(value) > 0 {
		setting["channel"] = intAny(value)
	}
	return setting
}

func boolAnyDefault(value interface{}, fallback bool) bool {
	if flag, ok := value.(bool); ok {
		return flag
	}
	return fallback
}

// postTencentAudioUpstream 调用上游并把响应规范化为平台可解析的结构。
func postTencentAudioUpstream(ctx context.Context, conn connectionConfig, endpoint string, payload map[string]interface{}, timeout time.Duration) ([]byte, int, error) {
	target := joinBaseEndpoint(conn.BaseURL, endpoint)
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	applyConnectionHeaders(req, conn)
	req.Header.Set("Content-Type", "application/json")
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 250<<20))
	if readErr != nil {
		return raw, resp.StatusCode, readErr
	}
	if resp.StatusCode >= 400 {
		return raw, resp.StatusCode, nil
	}
	normalized, err := normalizeTencentAudioResponse(raw, resp.Header.Get("Content-Type"), payload)
	if err != nil {
		// 规范化失败时原样返回，让平台的宽容解析器再试一次，并留下原始响应便于排查。
		log.Printf("tencent audio normalize failed (%v), passing raw response through; body=%s", err, truncateWorkerLog(string(raw), 800))
		return raw, resp.StatusCode, nil
	}
	return normalized, resp.StatusCode, nil
}

// normalizeTencentAudioResponse 把上游响应统一成 {"data":[{"b64_json"|"url": ...}]}。
func normalizeTencentAudioResponse(raw []byte, contentType string, payload map[string]interface{}) ([]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty response")
	}
	format := "mp3"
	if setting, ok := payload["audio_setting"].(map[string]interface{}); ok {
		if configured := strings.TrimSpace(stringAny(setting["format"])); configured != "" {
			format = configured
		}
	}
	mime := audioMimeForFormat(format)

	// 裸二进制音频（OpenAI /v1/audio/speech 风格）
	if strings.HasPrefix(strings.ToLower(contentType), "audio/") || (!json.Valid(trimmed) && len(trimmed) > 1024) {
		return json.Marshal(map[string]interface{}{
			"data": []map[string]interface{}{{"b64_json": base64.StdEncoding.EncodeToString(trimmed), "media_type": mime}},
		})
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return nil, err
	}

	// OpenAI 风格：{"audio":{"data":"<base64>"}}
	if audio, ok := parsed["audio"].(map[string]interface{}); ok {
		if data := strings.TrimSpace(stringAny(audio["data"])); data != "" {
			return json.Marshal(map[string]interface{}{
				"data": []map[string]interface{}{{"b64_json": data, "media_type": mime}},
			})
		}
	}

	// MiniMax 风格：{"data":{"audio":"<hex|base64|url>", "status":2}}
	if data, ok := parsed["data"].(map[string]interface{}); ok {
		encoded := strings.TrimSpace(stringAny(data["audio"]))
		if encoded == "" {
			encoded = strings.TrimSpace(stringAny(data["audio_file"]))
		}
		if isHTTPURL(encoded) {
			return json.Marshal(map[string]interface{}{
				"data": []map[string]interface{}{{"url": encoded}},
			})
		}
		if encoded != "" {
			base64Audio, err := decodeTencentAudioPayload(encoded)
			if err != nil {
				return nil, err
			}
			return json.Marshal(map[string]interface{}{
				"data": []map[string]interface{}{{"b64_json": base64Audio, "media_type": mime}},
			})
		}
	}

	// 其它结构交给平台的宽容解析器
	return trimmed, nil
}

// decodeTencentAudioPayload 把上游返回的音频载荷统一转成 base64。
// MiniMax 默认返回 hex（output_format=hex），而 hex 串恰好也是合法 base64，
// 平台若按 base64 解析会得到乱码，所以这里必须先按 hex 解开再转 base64。
func decodeTencentAudioPayload(encoded string) (string, error) {
	if base64Audio, err := hex.DecodeString(encoded); err == nil && len(base64Audio) > 0 {
		return base64.StdEncoding.EncodeToString(base64Audio), nil
	}
	if base64Audio, err := base64.StdEncoding.DecodeString(encoded); err == nil && len(base64Audio) > 0 {
		return base64.StdEncoding.EncodeToString(base64Audio), nil
	}
	return "", fmt.Errorf("unrecognized audio payload (len=%d)", len(encoded))
}

func audioMimeForFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "wav":
		return "audio/wav"
	case "flac":
		return "audio/flac"
	case "pcm":
		return "audio/pcm"
	case "aac":
		return "audio/aac"
	default:
		return "audio/mpeg"
	}
}

func truncateWorkerLog(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}
