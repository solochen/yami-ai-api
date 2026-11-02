package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"errors"
	"image/jpeg"
	"log"
	"strings"
)

const (
	// 参考图发给上游前的尺寸/体积上限。实测 8 张手机原图共 62.6MB，转 base64
	// 后请求体 83.5MB，上游网关收不下会一直等到客户端超时且回不到任何错误。
	referenceImageMaxEdge = 1536
	// base64 文本累计上限，超出后丢弃后续参考图（保留前几张主图）。
	referenceImageBudget = 20 << 20
)

// compressReferenceImage 把大图缩到 1536px 并转 JPEG；小图原样返回，避免无谓的画质损失。
func compressReferenceImage(data []byte, contentType string) ([]byte, string) {
	im, err := decodeProductImage(data)
	if err != nil {
		return data, contentType
	}
	bounds := im.Bounds()
	if maxInt(bounds.Dx(), bounds.Dy()) <= referenceImageMaxEdge && len(data) <= 1<<20 {
		return data, contentType
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resizeProductImage(im, referenceImageMaxEdge, false), &jpeg.Options{Quality: 88}); err != nil {
		return data, contentType
	}
	if buf.Len() >= len(data) {
		return data, contentType
	}
	return buf.Bytes(), "image/jpeg"
}

// loadReferenceImageBytes 读取参考图原始字节并压缩，供 multipart 图像编辑接口使用。
func loadReferenceImageBytes(ctx context.Context, source string) ([]byte, string, error) {
	normalized := normalizeReferenceImage(ctx, source)
	if normalized == "" {
		return nil, "", errors.New("素材不可读")
	}
	data, contentType, err := loadMediaBytes(ctx, normalized)
	if err != nil {
		return nil, "", err
	}
	compressed, compressedType := compressReferenceImage(data, contentType)
	return compressed, compressedType, nil
}

// referenceDataURL 把参考图转成上游可接收的 data URL；大图先缩到 1536px 再转 JPEG。
func referenceDataURL(data []byte, contentType string) string {
	data, contentType = compressReferenceImage(data, contentType)
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// openRouterReferenceField 决定参考图放在哪个字段：
// 直连 OpenRouter 用 input_references；经 new-api（hub）中转只能用 image，
// 因为 new-api 会丢弃不认识的字段（实测 input_references 到不了下游 bridge）。
func openRouterReferenceField(runtimeRule map[string]interface{}) string {
	upstream, _ := runtimeRule["upstream"].(map[string]interface{})
	if field := strings.TrimSpace(fmt.Sprint(upstream["reference_field"])); field != "" && field != "<nil>" {
		return field
	}
	return "input_references"
}

// imageReferenceLimit 读取模型声明的参考图上限；0 表示未声明。
func imageReferenceLimit(runtimeRule map[string]interface{}) int {
	imageRule, _ := runtimeRule["image"].(map[string]interface{})
	if imageRule == nil {
		return 0
	}
	if limit := intAny(imageRule["max_reference_images"]); limit > 0 {
		return limit
	}
	return 0
}

// capReferenceInput 按模型上限截断参考图，避免上游因数量超限直接拒绝
// （例如 qwen-image-3 只接受 4 张，传 8 张会报 input_references 参数不支持）。
func capReferenceInput(input map[string]interface{}, limit int) {
	if limit <= 0 {
		return
	}
	sources := referenceImageSources(input["reference_images"])
	if len(sources) <= limit {
		return
	}
	log.Printf("reference images capped %d -> %d by model limit", len(sources), limit)
	trimmed := make([]string, 0, limit)
	trimmed = append(trimmed, sources[:limit]...)
	input["reference_images"] = trimmed
}

// normalizeReferenceList 逐张归一化，并对累计体积设上限，避免整包请求体撑爆上游。
func normalizeReferenceList(ctx context.Context, sources []string) []string {
	out := make([]string, 0, len(sources))
	total := 0
	for _, src := range sources {
		normalized := normalizeReferenceImage(ctx, src)
		if normalized == "" {
			continue
		}
		if !strings.HasPrefix(normalized, "data:") {
			out = append(out, normalized)
			continue
		}
		if total+len(normalized) > referenceImageBudget {
			log.Printf("normalizeReferenceImage: reference budget exceeded, dropping src=%s", src)
			continue
		}
		total += len(normalized)
		out = append(out, normalized)
	}
	if len(sources) > 0 {
		log.Printf("normalizeReferenceImages: %d/%d references kept, %.1f MB base64", len(out), len(sources), float64(total)/(1<<20))
	}
	return out
}
