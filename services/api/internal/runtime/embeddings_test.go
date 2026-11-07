package runtime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddingsChunksOpenAIRequestsAndPinsDimensions(t *testing.T) {
	var calls int
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("path = %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"data":[{"index":1,"embedding":[2]},{"index":0,"embedding":[1]}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "token", 5, 5)
	vectors, err := client.Embeddings(context.Background(), "qwen3.7-text-embedding", []string{"a", "b"}, map[string]interface{}{
		"runtime_rule": map[string]interface{}{
			"embedding": map[string]interface{}{"batch_size": float64(2), "dimensions": float64(1024)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(vectors) != 2 || vectors[0][0] != 1 || vectors[1][0] != 2 {
		t.Fatalf("calls=%d vectors=%v", calls, vectors)
	}
	if body["dimensions"] != float64(1024) {
		t.Fatalf("dimensions = %v", body["dimensions"])
	}
}

func TestEmbeddingsUsesTokenHubMultimodalTextInput(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings/multimodal" {
			t.Errorf("path = %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.5,0.5]}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "token", 5, 5)
	vectors, err := client.Embeddings(context.Background(), "kinfra-vl-embedding-2b", []string{"奶油风"}, map[string]interface{}{
		"runtime_rule": map[string]interface{}{
			"embedding": map[string]interface{}{
				"adapter": "tokenhub_multimodal", "endpoint": "/embeddings/multimodal", "batch_size": float64(1),
			},
		},
	})
	if err != nil || len(vectors) != 1 || len(vectors[0]) != 2 {
		t.Fatalf("vectors=%v err=%v", vectors, err)
	}
	input, _ := body["input"].([]interface{})
	item, _ := input[0].(map[string]interface{})
	if item["type"] != "text" || item["text"] != "奶油风" {
		t.Fatalf("input = %#v", body["input"])
	}
}

func TestEmbeddingsDecodesDashScopeIndependentVectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"output":{"embeddings":[{"text_index":1,"embedding":[2]},{"text_index":0,"embedding":[1]}]}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token", 5, 5)
	vectors, err := client.Embeddings(context.Background(), "qwen3-vl-embedding", []string{"一", "二"}, map[string]interface{}{
		"runtime_rule": map[string]interface{}{
			"embedding": map[string]interface{}{
				"adapter":    "dashscope_multimodal",
				"endpoint":   "/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding",
				"batch_size": float64(8),
			},
		},
	})
	if err != nil || vectors[0][0] != 1 || vectors[1][0] != 2 {
		t.Fatalf("vectors=%v err=%v", vectors, err)
	}
}

func TestEmbeddingsRejectsOverlongTokenHubText(t *testing.T) {
	client := NewClient("https://tokenhub.example/v1", "token", 5, 5)
	_, err := client.Embeddings(context.Background(), "kinfra-text-embedding-0.6b", []string{"一二三"}, map[string]interface{}{
		"runtime_rule": map[string]interface{}{
			"embedding": map[string]interface{}{"max_input_runes": float64(2)},
		},
	})
	if err == nil {
		t.Fatal("expected length error")
	}
}
