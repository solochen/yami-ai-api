package service

import (
	"encoding/json"
	"strings"

	"github.com/starai/api/internal/runtime"
)

// An explicit canonical switch wins over legacy aliases, including false.
func mediaCapability(rule map[string]interface{}, keys ...string) (bool, bool) {
	caps, _ := rule["capabilities"].(map[string]interface{})
	for _, key := range keys {
		if value, ok := caps[key].(bool); ok {
			return value, true
		}
	}
	return false, false
}

func chatRouteMediaError(model *ModelFull, route *ModelRoute, messages interface{}) error {
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	var items []struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	for _, item := range items {
		var parts []struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(item.Content, &parts) != nil {
			continue
		}
		for _, part := range parts {
			var keys []string
			label := "图片"
			switch part.Type {
			case "image_url":
				keys = []string{"vision", "image_input", "multimodal"}
			case "video_url":
				keys = []string{"video_analysis", "video_input", "video_understanding"}
				label = "视频"
			case "audio_url", "input_audio":
				keys = []string{"audio_analysis", "audio_input", "audio_understanding"}
				label = "音频"
			default:
				continue
			}
			allowed, configured := mediaCapability(model.RuntimeRule, keys...)
			if override, exists := mediaCapability(route.RuntimeRule, keys...); exists {
				allowed, configured = override, true
			}
			protocol := strings.ToLower(strings.TrimSpace(route.Protocol))
			videoUnsupported := protocol == "claude" || protocol == "anthropic" || protocol == "anthropic_messages" || protocol == "claude_messages" || strings.Contains(route.Endpoint, "/responses")
			// Only an EXPLICIT false blocks. A capability that was never
			// configured is unknown, not unsupported: manual configuration is
			// demonstrably unreliable (models that do read images ship with no
			// vision flag at all), and a non-vision upstream answers in-band
			// with a refusal instead of erroring — so treating "unknown" as
			// "no" silently breaks capable models with no way for the user to
			// tell. The admin "检测图片理解能力" probe is what fills the flag in.
			if (configured && !allowed) || ((part.Type == "video_url" || part.Type == "audio_url" || part.Type == "input_audio") && videoUnsupported) {
				if label == "图片" {
					return &runtime.PlatformError{Code: "MEDIA_INPUT_UNSUPPORTED", StatusCode: 400, Message: "当前模型已配置为不支持图片理解，请改用支持图片的模型，或移除附件后重试"}
				}
				return &runtime.PlatformError{Code: "MEDIA_INPUT_UNSUPPORTED", StatusCode: 400, Message: "模型线路未启用或协议不支持" + label + "理解，请检查模型及线路的媒体能力配置"}
			}
		}
	}
	return nil
}
