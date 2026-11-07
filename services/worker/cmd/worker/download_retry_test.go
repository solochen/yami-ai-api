package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestDownloadMediaNotFoundBudgetFallsThroughQuickly 回归：
// 上游结果 CDN 用裸 404 表示「该地址不存在」时，不能按 45 次退避重试，
// 否则会白等约 20 分钟，并让前端画布视频节点先报「生成超时」。
func TestDownloadMediaNotFoundBudgetFallsThroughQuickly(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		http.NotFound(w, r) // 裸 404，不带 task not found 标记
	}))
	defer server.Close()

	_, _, err := downloadMediaWithNotFoundBudget(context.Background(), connectionConfig{}, server.URL+"/v1/videos/x/content", 1<<20, maxDownloadAttemptsNotFound)
	if err == nil {
		t.Fatal("expected an error for a 404 media URL")
	}
	if got := atomic.LoadInt64(&hits); got != maxDownloadAttemptsNotFound {
		t.Fatalf("download attempts = %d, want %d", got, maxDownloadAttemptsNotFound)
	}
}

// TestDownloadMediaDefaultBudgetStillRetriesTransient 确认真实结果地址仍保留完整重试预算。
func TestDownloadMediaDefaultBudgetStillRetriesTransient(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&hits, 1) < 4 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n0123456789"))
	}))
	defer server.Close()

	data, contentType, err := downloadAuthenticatedMedia(context.Background(), connectionConfig{}, server.URL+"/img.png", 1<<20)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if len(data) == 0 || contentType == "" {
		t.Fatalf("empty download: %d bytes, content-type %q", len(data), contentType)
	}
	if got := atomic.LoadInt64(&hits); got != 4 {
		t.Fatalf("download attempts = %d, want 4", got)
	}
}
