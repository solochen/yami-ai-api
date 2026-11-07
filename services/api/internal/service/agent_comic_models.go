package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/starai/modelcontract"
)

// AI 漫剧把模型选择同时存在两处：workflow_definitions.runtime_config（后台
// 默认值）和 workflow_projects.inputs（用户本次提交/重试的取值）。模型代号
// 会随线路调整被停用或下线（例如演示模型 image_fast_v1 / video_demo_v1），
// 旧配置不应让整个智能体永久不可用，因此在真正校验和入队之前做一次自愈：
// 仍然可用的选择保持不变，只替换缺失、已停用或类型不匹配的代号。

type comicModelSlot struct {
	inputKey string
	category string
	label    string
	prefer   []string
}

func comicModelRequestModes(category string) []string {
	switch category {
	case "image":
		return []string{"images"}
	case "video":
		return []string{"video"}
	case "audio":
		return []string{"audio"}
	case "chat":
		return []string{"chat_completions", "responses"}
	default:
		return nil
	}
}

var comicModelSlots = []comicModelSlot{
	{
		inputKey: "image_model_code",
		category: "image",
		label:    "图片",
		prefer:   []string{"seedream-5-0-pro", "seedream-4-5", "seedream-5-0-lite"},
	},
	{
		inputKey: "video_model_code",
		category: "video",
		label:    "视频",
		prefer:   []string{"seedance-2-5", "seedance-2-0-fast", "kling-v3-0-std", "wan-2-6"},
	},
	{
		inputKey: "narration_model_code",
		category: "audio",
		label:    "配音",
		prefer:   []string{"audio_minimax_speech_28_hd"},
	},
}

// 这些早期演示模型即使在某些历史库里仍被标记为启用，也指向占位上游，
// 不能作为 AI 漫剧的真实生产模型。新库/回滚库都按不可用处理。
var retiredComicModelCodes = map[string]bool{
	"image_fast_v1": true,
	"video_demo_v1": true,
	"chat_demo_v1":  true,
}

// Fallbacks must satisfy the same slot and route requirements as admin writes.
// A model without explicit routes may still use the platform default gateway.
func (s *AgentService) comicModelUsable(ctx context.Context, code, category string) bool {
	code = strings.TrimSpace(code)
	if code == "" || retiredComicModelCodes[strings.ToLower(code)] {
		return false
	}
	models, err := modelcontract.LoadModels(ctx, s.db)
	if err != nil {
		return false
	}
	m, ok := modelcontract.Find(models, code)
	if !ok {
		return false
	}
	key := map[string]string{"image": "image_model_code", "video": "video_model_code", "audio": "narration_model_code", "chat": "analysis_model_code"}[category]
	slot, ok := modelcontract.FieldSlot(modelcontract.Definition{Code: "ai_comic_drama"}, key)
	return ok && modelcontract.Validate(slot, m) == nil
}

// comicModelFallback 依次尝试：通用创作智能体的同槽位配置 → 各槽位的稳定
// 首选代号 → 该分类下第一个满足槽位能力的模型。全部失败返回空串，
// 由调用方保留原值并交给显式校验报出可读错误。
func (s *AgentService) comicModelFallback(ctx context.Context, category string, prefer []string) string {
	var raw []byte
	if err := s.db.QueryRow(ctx, `SELECT runtime_config FROM workflow_definitions WHERE code='general_creative_agent'`).Scan(&raw); err == nil {
		var cfg map[string]interface{}
		if json.Unmarshal(raw, &cfg) == nil {
			keys := []string{}
			switch category {
			case "image":
				keys = []string{"image_model_code"}
			case "video":
				keys = []string{"video_model_code"}
			case "audio":
				keys = []string{"speech_model_code", "narration_model_code"}
			case "chat":
				keys = []string{"analysis_model_code", "fallback_model_code"}
			}
			for _, key := range keys {
				if code := strings.TrimSpace(stringValue(cfg[key])); code != "" && s.comicModelUsable(ctx, code, category) {
					return code
				}
			}
		}
	}
	for _, code := range prefer {
		if s.comicModelUsable(ctx, code, category) {
			return code
		}
	}
	modes := comicModelRequestModes(category)
	if len(modes) == 0 {
		return ""
	}
	models, err := modelcontract.LoadModels(ctx, s.db)
	if err != nil {
		return ""
	}
	key := map[string]string{"image": "image_model_code", "video": "video_model_code", "audio": "narration_model_code", "chat": "analysis_model_code"}[category]
	slot, ok := modelcontract.FieldSlot(modelcontract.Definition{Code: "ai_comic_drama"}, key)
	if !ok {
		return ""
	}
	for _, model := range models {
		if model.Category == category && modelcontract.Validate(slot, model) == nil {
			return model.Code
		}
	}
	return ""
}

// healComicDramaModels 只修复不可用/缺失的槽位，并让音频策略与最终是否
// 真的有配音模型保持一致，避免无意义地卡在“需要配置配音模型”。
func (s *AgentService) healComicDramaModels(ctx context.Context, inputs map[string]interface{}) {
	if inputs == nil {
		return
	}
	for _, slot := range comicModelSlots {
		if s.comicModelUsable(ctx, stringValue(inputs[slot.inputKey]), slot.category) {
			continue
		}
		if code := s.comicModelFallback(ctx, slot.category, slot.prefer); code != "" {
			inputs[slot.inputKey] = code
		}
	}

	usable := make([]string, 0, 2)
	seen := map[string]bool{}
	for _, code := range agentStringSlice(inputs["dialogue_model_codes"], nil) {
		code = strings.TrimSpace(code)
		if code == "" || seen[code] || !s.comicModelUsable(ctx, code, "chat") {
			continue
		}
		seen[code] = true
		usable = append(usable, code)
	}
	if len(usable) == 0 {
		if code := strings.TrimSpace(stringValue(inputs["analysis_model_code"])); s.comicModelUsable(ctx, code, "chat") {
			usable = append(usable, code)
		}
	}
	if len(usable) == 0 {
		if code := s.comicModelFallback(ctx, "chat", []string{"deepseek-v4-1-flash-1017", "deepseek-v4-pro-0813"}); code != "" {
			usable = append(usable, code)
		}
	}
	if len(usable) > 0 {
		inputs["dialogue_model_codes"] = usable
	}

	if !s.comicModelUsable(ctx, stringValue(inputs["narration_model_code"]), "audio") {
		if strings.ToLower(strings.TrimSpace(stringValue(inputs["audio_strategy"]))) != "video_native" {
			inputs["audio_strategy"] = "video_native"
		}
	}
}
