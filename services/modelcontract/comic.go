package modelcontract

import "fmt"

type ComicWork struct {
	Assets     int  `json:"assets"`
	Plan       bool `json:"plan"`
	Images     int  `json:"images"`
	Videos     int  `json:"videos"`
	Narrations int  `json:"narrations"`
}

func firstString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
func audioStrategy(inputs, runtime map[string]interface{}) string {
	strategy := firstString(String(inputs["audio_strategy"]), String(runtime["audio_strategy"]))
	if strategy != "" {
		return strategy
	}
	if firstString(String(inputs["narration_model_code"]), String(runtime["narration_model_code"])) != "" {
		return "hybrid"
	}
	return "video_native"
}
func gridSize(inputs, runtime map[string]interface{}) int {
	v := inputs["storyboard_grid"]
	if v == nil {
		v = runtime["storyboard_grid"]
	}
	n := int(number(v))
	if n <= 0 {
		n = 6
	}
	return n
}

func items(raw interface{}) []interface{} {
	if items, ok := raw.([]interface{}); ok {
		return items
	}
	if items, ok := raw.([]map[string]interface{}); ok {
		out := make([]interface{}, len(items))
		for i := range items {
			out[i] = items[i]
		}
		return out
	}
	return nil
}

func RemainingComicWork(inputs, outputs, runtimeCfg map[string]interface{}) ComicWork {
	if String(outputs["final_video_url"]) != "" && String(outputs["current_step"]) == "result" {
		return ComicWork{}
	}
	plan, ok := outputs["comic_drama"].(map[string]interface{})
	shots := items(plan["storyboards"])
	audio := audioStrategy(inputs, runtimeCfg) != "video_native"
	if !ok || len(shots) == 0 {
		grid := gridSize(inputs, runtimeCfg)
		work := ComicWork{Plan: true, Assets: grid * 3, Images: grid, Videos: grid}
		if audio {
			work.Narrations = grid
		}
		return work
	}
	index := func(key string) map[string]map[string]interface{} {
		result := map[string]map[string]interface{}{}
		for _, raw := range items(outputs[key]) {
			if item, ok := raw.(map[string]interface{}); ok {
				result[String(item["id"])] = item
			}
		}
		return result
	}
	images, videos, voices := index("keyframes"), index("segments"), index("narrations")
	missing := func(item map[string]interface{}, key string) bool {
		return String(item[key]) == "" || String(item["status"]) == "failed"
	}
	work := ComicWork{}
	for i, raw := range shots {
		shot, _ := raw.(map[string]interface{})
		id := firstString(String(shot["id"]), fmt.Sprintf("S%02d", i+1))
		if missing(images[id], "image_url") {
			work.Images++
		}
		if missing(videos[id], "video_url") {
			work.Videos++
		}
		if audio && (String(shot["dialogue"]) != "" || String(shot["narration"]) != "") && missing(voices[id], "audio_url") {
			work.Narrations++
		}
	}
	used := map[string]bool{}
	for _, raw := range shots {
		shot, _ := raw.(map[string]interface{})
		for _, key := range []string{"character_codes", "prop_codes"} {
			for _, code := range Strings(shot[key]) {
				used[code] = true
			}
		}
		if code := String(shot["location_code"]); code != "" {
			used[code] = true
		}
	}
	for _, raw := range append(items(inputs["comic_assets"]), items(outputs["consistency_assets"])...) {
		asset, _ := raw.(map[string]interface{})
		metadata, _ := asset["metadata"].(map[string]interface{})
		if len(Strings(metadata["reference_urls"])) > 0 || len(Strings(metadata["reference_images"])) > 0 {
			delete(used, firstString(String(asset["asset_code"]), String(asset["code"])))
		}
	}
	work.Assets = len(used)
	return work
}

func ValidateComicWork(d Definition, inputs map[string]interface{}, work ComicWork, models []Model) error {
	d = Effective(d, inputs)
	keys := []string{}
	if work.Images+work.Assets > 0 {
		keys = append(keys, "image_model_code", "quality_model_code")
	}
	if work.Videos > 0 {
		keys = append(keys, "video_model_code")
	}
	if work.Narrations > 0 {
		keys = append(keys, "narration_model_code")
	}
	if work.Plan {
		keys = append(keys, "analysis_model_code", "dialogue_model_codes", "fallback_model_code")
	}
	for _, key := range keys {
		slot, _ := FieldSlot(d, key)
		codes := []string{String(d.Runtime[key])}
		if key == "dialogue_model_codes" {
			codes = Strings(d.Runtime[key])
		}
		if key == "analysis_model_code" && codes[0] == "" && len(Strings(d.Runtime["dialogue_model_codes"])) > 0 {
			continue
		}
		for _, code := range codes {
			if code == "" {
				if slot.Optional && key != "narration_model_code" {
					continue
				}
				return fmt.Errorf("剩余阶段缺少%s模型", slot.Label)
			}
			m, ok := Find(models, code)
			if !ok {
				return fmt.Errorf("%s：模型不存在：%s；已完成素材保留", slot.Label, code)
			}
			if err := Validate(slot, m); err != nil {
				return err
			}
		}
	}
	return nil
}
