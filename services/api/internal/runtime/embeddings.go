package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embeddings 调用 OpenAI 兼容的 /embeddings 接口，返回按输入顺序对齐的向量。
func (c *Client) Embeddings(ctx context.Context, model string, inputs []string, cfg map[string]interface{}) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("embeddings: empty input")
	}
	requestCfg := c.resolveConfig(cfg)
	body, err := json.Marshal(embeddingsRequest{Model: model, Input: inputs})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", joinEndpoint(requestCfg.BaseURL, "/embeddings"), bytes.NewReader(body))
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
	var parsed embeddingsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("embeddings: decode response: %w", err)
	}
	if len(parsed.Data) != len(inputs) {
		return nil, fmt.Errorf("embeddings: got %d vectors for %d inputs", len(parsed.Data), len(inputs))
	}
	sort.Slice(parsed.Data, func(i, j int) bool { return parsed.Data[i].Index < parsed.Data[j].Index })
	vectors := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		vectors[i] = d.Embedding
	}
	return vectors, nil
}
