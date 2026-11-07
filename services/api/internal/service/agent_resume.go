package service

import (
	"context"

	"github.com/starai/modelcontract"
)

type comicRemainingWork = modelcontract.ComicWork

func comicResumeItems(raw interface{}) []interface{} {
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

// Match the Worker's ID-based reuse checks (not raw array lengths, which can
// include duplicates or failed items). Missing checkpoints are never success.
func remainingComicWork(inputs, outputs, runtimeCfg map[string]interface{}) comicRemainingWork {
	return modelcontract.RemainingComicWork(inputs, outputs, runtimeCfg)
}

func (s *AgentService) validateComicRemainingModels(ctx context.Context, inputs map[string]interface{}, work comicRemainingWork) error {
	if work == (comicRemainingWork{}) {
		return nil
	}
	models, err := modelcontract.LoadModels(ctx, s.db)
	if err != nil {
		return err
	}
	return modelcontract.ValidateComicWork(modelcontract.Definition{Code: "ai_comic_drama", Runtime: inputs}, inputs, work, models)
}

// Resolve the same missing-code defaults as the Worker, without replacing any
// existing selection or modifying completed checkpoints.
func fillComicResumeModelDefaults(inputs, runtimeCfg map[string]interface{}) {
	for _, key := range []string{"image_model_code", "video_model_code", "narration_model_code"} {
		if stringValue(inputs[key]) == "" {
			code := stringValue(runtimeCfg[key])
			if code == "" && key != "narration_model_code" {
				code = stringValue(runtimeCfg["generation_model_code"])
			}
			inputs[key] = code
		}
	}
	if len(agentStringSlice(inputs["dialogue_model_codes"], nil)) == 0 {
		codes := agentStringSlice(runtimeCfg["dialogue_model_codes"], nil)
		if len(codes) == 0 {
			if code := stringValue(runtimeCfg["analysis_model_code"]); code != "" {
				codes = []string{code}
			}
		}
		inputs["dialogue_model_codes"] = codes
	}
}

func (s *AgentService) estimateComicRemainingCost(ctx context.Context, runtimeCfg, inputs map[string]interface{}, work comicRemainingWork) float64 {
	attempts := float64(min(5, max(0, intFromAgentAny(firstAgentNonNil(inputs["max_retry"], runtimeCfg["max_retry"])))) + 1)
	total := 0.0
	if work.Plan {
		codes := agentStringSlice(inputs["dialogue_model_codes"], nil)
		code := stringValue(runtimeCfg["analysis_model_code"])
		if len(codes) > 0 {
			code = codes[0]
		}
		total += s.estimateModelCostByCode(ctx, code, inputs, 1200, 2500)
	}
	if work.Images+work.Assets > 0 {
		total += attempts * s.estimateModelCostByCode(ctx, stringValue(inputs["image_model_code"]), map[string]interface{}{"n": work.Images + work.Assets}, 0, 0)
	}
	if work.Videos > 0 {
		duration := comicSegmentDurationSeconds(inputs, runtimeCfg)
		params := map[string]interface{}{"count": 1, "n": 1, "duration": duration, "duration_sec": duration, "resolution": normalizeComicVideoResolution(firstAgentString(stringValue(inputs["quality"]), stringValue(runtimeCfg["quality"]))), "generation_mode": "image", "reference_images": []string{"comic-keyframe"}}
		total += attempts * float64(work.Videos) * s.estimateModelCostByCode(ctx, stringValue(inputs["video_model_code"]), params, 0, 0)
	}
	if work.Narrations > 0 {
		total += float64(work.Narrations) * s.estimateModelCostByCode(ctx, stringValue(inputs["narration_model_code"]), map[string]interface{}{"count": 1}, 0, 200)
	}
	if code := stringValue(runtimeCfg["quality_model_code"]); code != "" {
		total += attempts * float64(work.Images+work.Videos+work.Assets) * s.estimateModelCostByCode(ctx, code, inputs, 6000, 500)
	}
	return total
}
