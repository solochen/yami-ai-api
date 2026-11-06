package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/starai/api/internal/runtime"
	"github.com/starai/api/internal/service"
	"github.com/starai/api/internal/util"
)

// visionProbe is one synthetic image with an unambiguous expected answer.
type visionProbe struct {
	label    string
	expected []string
	rgb      [3]uint8
}

var visionProbes = []visionProbe{
	{label: "红", expected: []string{"红", "red", "crimson", "scarlet", "赤"}, rgb: [3]uint8{220, 20, 20}},
	{label: "蓝", expected: []string{"蓝", "blue", "azure", "靛", "青"}, rgb: [3]uint8{20, 40, 220}},
}

// visionProbeResult is one probe's outcome, returned to the admin verbatim so a
// wrong verdict can always be checked against what the model actually said.
type visionProbeResult struct {
	Label    string `json:"label"`
	Answer   string `json:"answer"`
	Matched  bool   `json:"matched"`
	Refused  bool   `json:"refused"`
	ErrorMsg string `json:"error,omitempty"`
}

var visionRefusalMarkers = []string{
	"无法查看", "无法看到", "无法读取", "无法识别", "无法解析", "无法处理", "看不到", "看不见",
	"不能查看", "不能看到", "不能读取", "没有看到", "未提供图片", "没有图片", "不含图片", "不包含图片",
	"无法直接查看", "抱歉",
	"cannot view", "can't view", "cannot see", "can't see", "unable to view", "unable to see",
	"no image", "not able to view", "i'm unable",
}

// AdminProbeModelVision sends a synthetic colour image to a model and decides
// whether it can actually read images. Manual `runtime_rule.capabilities.vision`
// configuration is unreliable in both directions, and a non-vision upstream
// answers with a polite refusal instead of an error, so this probe is the only
// dependable way to fill the flag in.
func (h *Handler) AdminProbeModelVision(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		util.BadRequest(c, "模型 ID 无效")
		return
	}
	var req struct {
		Apply *bool `json:"apply"`
	}
	_ = c.ShouldBindJSON(&req)
	apply := req.Apply == nil || *req.Apply

	model, err := h.models.GetFullByIDForAdmin(c.Request.Context(), id)
	if err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	if service.IsMultiCollabModel(model) {
		util.BadRequest(c, "多模型协作入口没有独立上游，请对成员模型单独检测")
		return
	}

	results := make([]visionProbeResult, 0, len(visionProbes))
	matched, refused, failed := 0, 0, 0

	for _, probe := range visionProbes {
		answer, probeErr := h.runVisionProbe(c.Request.Context(), model, probe)
		item := visionProbeResult{Label: probe.label, Answer: answer}
		if probeErr != nil {
			item.ErrorMsg = probeErr.Error()
			failed++
			results = append(results, item)
			continue
		}
		item.Refused = looksLikeRefusal(answer, probe.expected)
		item.Matched = !item.Refused && containsAny(answer, probe.expected)
		if item.Matched {
			matched++
		} else if item.Refused {
			refused++
		}
		results = append(results, item)
	}

	verdict := "inconclusive"
	switch {
	case failed == len(visionProbes):
		verdict = "error"
	case matched == len(visionProbes):
		verdict = "supported"
	case matched == 0 && refused > 0:
		// It never named either colour and explicitly said it could not see at
		// least one of them.
		verdict = "unsupported"
	case matched == 0 && answersIndistinguishable(results, visionProbes):
		// Two different colours producing the identical answer that names
		// neither colour means the model is not reading the images at all —
		// here it confidently hallucinated instead of refusing.
		verdict = "unsupported"
	}

	applied := false
	message := ""
	if apply && (verdict == "supported" || verdict == "unsupported") {
		want := verdict == "supported"
		if err := h.models.SetModelCapability(c.Request.Context(), model.ID, "vision", want); err != nil {
			util.InternalError(c, err.Error())
			return
		}
		if want {
			// image_input is the legacy alias for the same capability; keep them
			// consistent so older checks agree with the probe.
			_ = h.models.SetModelCapability(c.Request.Context(), model.ID, "image_input", true)
		}
		applied = true
	}
	switch verdict {
	case "supported":
		message = "模型能正确识别图片内容，已标记为支持图片理解"
	case "unsupported":
		message = "模型无法识别图片内容，已标记为不支持图片理解"
	case "error":
		message = "所有线路都调用失败，无法判断；请先确认模型线路可用"
	default:
		message = "结果不确定，未修改配置；请查看下面的原始回答人工确认"
	}

	util.OK(c, map[string]interface{}{
		"verdict": verdict,
		"applied": applied,
		"message": message,
		"probes":  results,
	})
}

// runVisionProbe renders a solid-colour image and asks the model to name it.
func (h *Handler) runVisionProbe(ctx context.Context, model *service.ModelFull, probe visionProbe) (string, error) {
	data, err := solidColorPNG(probe.rgb, 96)
	if err != nil {
		return "", err
	}
	messages := []map[string]interface{}{{
		"role": "user",
		"content": []map[string]interface{}{
			{"type": "text", "text": "这张图片主要是什么颜色？只回答颜色名称，不要解释。"},
			{"type": "image_url", "image_url": map[string]interface{}{
				"url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(data),
			}},
		},
	}}
	requestID := util.NewRequestID() + "-vision-probe"
	probeCtx, cancel := context.WithTimeout(service.WithMediaProbe(ctx), 60*time.Second)
	defer cancel()
	resp, _, callErr := h.chat.CompleteWithFailover(probeCtx, requestID, model, runtime.ChatRequest{
		Model:       model.NewAPIModel,
		Messages:    messages,
		Temperature: runtime.Float64Ptr(0),
	}, map[string]interface{}{"temperature": 0.0})
	if callErr != nil {
		var pe *runtime.PlatformError
		if errors.As(callErr, &pe) {
			return "", errors.New(pe.Message)
		}
		return "", callErr
	}
	if resp == nil || len(resp.Choices) == 0 {
		return "", errors.New("模型没有返回内容")
	}
	answer := strings.TrimSpace(resp.Choices[0].Message.Content)
	if answer == "" {
		answer = strings.TrimSpace(resp.Choices[0].Message.ReasoningContent)
	}
	if answer == "" {
		return "", errors.New("模型返回了空正文")
	}
	return answer, nil
}

func solidColorPNG(rgb [3]uint8, size int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill := color.RGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: 255}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("生成探测图片失败: %w", err)
	}
	return buf.Bytes(), nil
}

func containsAny(text string, needles []string) bool {
	lowered := strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(lowered, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func looksLikeRefusal(text string, expected []string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	// Naming the expected colour makes it an answer, even when it opens with an
	// apology ("抱歉让您久等了。这张图整体是红色…").
	if containsAny(trimmed, expected) {
		return false
	}
	// A long non-answer is not a clean refusal; report it as inconclusive rather
	// than guessing.
	if len([]rune(trimmed)) > 60 {
		return false
	}
	return containsAny(trimmed, visionRefusalMarkers)
}

// answersIndistinguishable reports whether every probe came back with the same
// answer and that answer names none of the probe colours. A model that is
// actually looking at the images cannot return the identical non-colour answer
// for two different solid colours; one that hallucinates or parrots a template
// does.
func answersIndistinguishable(results []visionProbeResult, probes []visionProbe) bool {
	if len(results) != len(probes) || len(results) < 2 {
		return false
	}
	normalized := make([]string, 0, len(results))
	for _, item := range results {
		value := strings.ToLower(strings.Join(strings.Fields(item.Answer), ""))
		if value == "" {
			return false
		}
		normalized = append(normalized, value)
	}
	for _, value := range normalized[1:] {
		if value != normalized[0] {
			return false
		}
	}
	var allExpected []string
	for _, probe := range probes {
		allExpected = append(allExpected, probe.expected...)
	}
	return !containsAny(normalized[0], allExpected)
}
