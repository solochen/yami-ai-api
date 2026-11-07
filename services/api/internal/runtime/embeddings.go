package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type embeddingCallSettings struct {
	Adapter       string
	Endpoint      string
	BatchSize     int
	Dimensions    int
	MaxInputRunes int
}

type embeddingVector struct {
	Index     int       `json:"index"`
	TextIndex int       `json:"text_index"`
	Embedding []float32 `json:"embedding"`
}

type embeddingResponse struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Data    []embeddingVector `json:"data"`
	Output  struct {
		Embeddings []embeddingVector `json:"embeddings"`
	} `json:"output"`
	Embeddings []embeddingVector `json:"embeddings"`
}

// Embeddings 把文本编码为向量。默认走 OpenAI 兼容的 /embeddings；
// runtime_rule.embedding 可切换腾讯云多模态向量和百炼多模态向量协议。
func (c *Client) Embeddings(ctx context.Context, model string, inputs []string, cfg map[string]interface{}) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("embeddings: empty input")
	}
	settings := embeddingSettings(cfg)
	if settings.MaxInputRunes > 0 {
		for _, input := range inputs {
			if len([]rune(input)) > settings.MaxInputRunes {
				return nil, fmt.Errorf("embeddings: input exceeds %d characters", settings.MaxInputRunes)
			}
		}
	}
	out := make([][]float32, 0, len(inputs))
	for start := 0; start < len(inputs); start += settings.BatchSize {
		end := start + settings.BatchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		part, err := c.embedBatch(ctx, model, inputs[start:end], cfg, settings)
		if err != nil {
			return nil, err
		}
		out = append(out, part...)
	}
	return out, nil
}

func (c *Client) embedBatch(ctx context.Context, model string, inputs []string, cfg map[string]interface{}, settings embeddingCallSettings) ([][]float32, error) {
	body, err := embeddingRequestBody(model, inputs, settings)
	if err != nil {
		return nil, err
	}
	requestCfg := c.resolveConfig(cfg)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, joinEndpoint(requestCfg.BaseURL, settings.Endpoint), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	applyAuthHeaders(httpReq, requestCfg)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, mapError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, normalizeHTTPError(resp)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	return decodeEmbeddingResponse(raw, len(inputs))
}

func embeddingRequestBody(model string, inputs []string, settings embeddingCallSettings) ([]byte, error) {
	switch settings.Adapter {
	case "tokenhub_multimodal":
		items := make([]map[string]string, len(inputs))
		for i, text := range inputs {
			items[i] = map[string]string{"type": "text", "text": text}
		}
		return json.Marshal(map[string]interface{}{
			"model":           model,
			"input":           items,
			"encoding_format": "float",
		})
	case "dashscope_multimodal":
		contents := make([]map[string]string, len(inputs))
		for i, text := range inputs {
			contents[i] = map[string]string{"text": text}
		}
		body := map[string]interface{}{
			"model": model,
			"input": map[string]interface{}{"contents": contents},
		}
		if settings.Dimensions > 0 {
			body["parameters"] = map[string]interface{}{"dimension": settings.Dimensions}
		}
		return json.Marshal(body)
	default:
		body := map[string]interface{}{
			"model":           model,
			"input":           inputs,
			"encoding_format": "float",
		}
		if settings.Dimensions > 0 {
			body["dimensions"] = settings.Dimensions
		}
		return json.Marshal(body)
	}
}

func decodeEmbeddingResponse(raw []byte, count int) ([][]float32, error) {
	var parsed embeddingResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("embeddings: decode response: %w", err)
	}
	items := parsed.Data
	if len(items) == 0 {
		items = parsed.Output.Embeddings
	}
	if len(items) == 0 {
		items = parsed.Embeddings
	}
	if len(items) == 0 && parsed.Code != "" {
		return nil, fmt.Errorf("embeddings: %s: %s", parsed.Code, parsed.Message)
	}
	return alignEmbeddingVectors(items, count)
}

func alignEmbeddingVectors(items []embeddingVector, count int) ([][]float32, error) {
	if len(items) != count {
		return nil, fmt.Errorf("embeddings: got %d vectors for %d inputs", len(items), count)
	}
	for _, item := range items {
		if len(item.Embedding) == 0 {
			return nil, fmt.Errorf("embeddings: empty vector")
		}
	}
	if count == 1 {
		return [][]float32{items[0].Embedding}, nil
	}
	indexes := make([]int, count)
	seen := map[int]struct{}{}
	for i, item := range items {
		idx := item.Index
		if item.TextIndex > idx {
			idx = item.TextIndex
		}
		if idx < 0 || idx >= count {
			return vectorsInResponseOrder(items), nil
		}
		if _, exists := seen[idx]; exists {
			return vectorsInResponseOrder(items), nil
		}
		seen[idx] = struct{}{}
		indexes[i] = idx
	}
	if len(seen) != count {
		return vectorsInResponseOrder(items), nil
	}
	out := make([][]float32, count)
	for i, item := range items {
		out[indexes[i]] = item.Embedding
	}
	return out, nil
}

func vectorsInResponseOrder(items []embeddingVector) [][]float32 {
	out := make([][]float32, len(items))
	for i, item := range items {
		out[i] = item.Embedding
	}
	return out
}

func embeddingSettings(cfg map[string]interface{}) embeddingCallSettings {
	settings := embeddingCallSettings{Adapter: "openai", Endpoint: "/embeddings", BatchSize: 16}
	rule, _ := cfg["runtime_rule"].(map[string]interface{})
	raw, _ := rule["embedding"].(map[string]interface{})
	if raw == nil {
		return settings
	}
	if adapter := strings.TrimSpace(embeddingString(raw["adapter"])); adapter != "" {
		settings.Adapter = adapter
	}
	if endpoint := strings.TrimSpace(embeddingString(raw["endpoint"])); endpoint != "" {
		settings.Endpoint = endpoint
	}
	if batch := embeddingInt(raw["batch_size"]); batch > 0 {
		settings.BatchSize = batch
	}
	if dimensions := embeddingInt(raw["dimensions"]); dimensions > 0 {
		settings.Dimensions = dimensions
	}
	if limit := embeddingInt(raw["max_input_runes"]); limit > 0 {
		settings.MaxInputRunes = limit
	}
	if settings.BatchSize < 1 {
		settings.BatchSize = 1
	}
	return settings
}

func embeddingString(value interface{}) string {
	text, _ := value.(string)
	return text
}

func embeddingInt(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		n, _ := typed.Int64()
		return int(n)
	default:
		return 0
	}
}
