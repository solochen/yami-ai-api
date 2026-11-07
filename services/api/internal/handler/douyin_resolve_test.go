package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
)

func TestDouyinMediaURLExtractionIgnoresCovers(t *testing.T) {
	raw := `{"success":true,"data":{"task_id":"t1","state":"done","data":{"kind":"video","music":{"play_url":"https://cdn.example.com/audio.mp3"},"media":{"video":{"url":"https://cdn.example.com/video.mp4","cover":"https://cdn.example.com/cover.jpg"},"images":[{"url":"https://cdn.example.com/slide.jpg"}]}}}}`
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatal(err)
	}
	if got := douyinMediaURL(value); got != "https://cdn.example.com/video.mp4" {
		t.Fatalf("unexpected media URL: %q", got)
	}
	var audioOnly interface{}
	if err := json.Unmarshal([]byte(`{"data":{"music":{"play_url":"https://cdn.example.com/audio.mp3"}}}`), &audioOnly); err != nil {
		t.Fatal(err)
	}
	if got := douyinMediaURL(audioOnly); got != "" {
		t.Fatalf("audio-only payload must not resolve to a video URL, got %q", got)
	}
}

func TestExtractDouyinSharePageVideoURL(t *testing.T) {
	page := []byte(`<html><script>window._ROUTER_DATA = {"loaderData":{"video_(id)/page":{"videoInfoRes":{"item_list":[{"video":{"play_addr":{"url_list":["https://cdn.example.com/play.mp4","https://cdn.example.com/play-fallback.mp4"]},"cover":{"url_list":["https://cdn.example.com/cover.jpg"]}}}]}}}};</script></html>`)
	if got := extractDouyinSharePageVideoURL(page); got != "https://cdn.example.com/play.mp4" {
		t.Fatalf("unexpected share page media URL: %q", got)
	}
	if got := extractDouyinSharePageVideoURL([]byte(`<html><script>window._ROUTER_DATA = {"loaderData":{"video_(id)/page":{"itemId":"1"}}};</script></html>`)); got != "" {
		t.Fatalf("expected empty result for pages without media, got %q", got)
	}
}

func TestResolveDouyinVideoURLViaParserService(t *testing.T) {
	var parseCalls, loginCalls, demoCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/demo":
			demoCalls++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true,"data":{"enabled":true,"username":"demo","password":"secret"}}`))
		case "/api/v1/auth/login":
			loginCalls++
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		case "/api/v1/parse":
			parseCalls++
			if parseCalls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHENTICATED"}}`))
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"success":true,"data":{"task_id":"task-1","state":"queued"}}`))
		case "/api/v1/tasks/task-1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true,"data":{"task_id":"task-1","state":"done","data":{"media":{"video":{"url":"https://cdn.example.com/douyin.mp4"}}}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("DOUYIN_API_BASE", server.URL)
	t.Setenv("DOUYIN_API_KEY", "")
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	link, err := resolveDouyinVideoURL(context.Background(), &http.Client{Jar: jar}, "https://v.douyin.com/qN3g8SUlMR0/")
	if err != nil {
		t.Fatalf("resolve Douyin video: %v", err)
	}
	if link != "https://cdn.example.com/douyin.mp4" {
		t.Fatalf("unexpected resolved link: %q", link)
	}
	if demoCalls != 1 || loginCalls != 1 || parseCalls != 2 {
		t.Fatalf("unexpected parser call counts: demo=%d login=%d parse=%d", demoCalls, loginCalls, parseCalls)
	}
}

func TestResolveDouyinVideoURLUsesConfiguredAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/parse" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"media":{"video":{"url":"https://cdn.example.com/key.mp4"}}}}`))
	}))
	defer server.Close()
	t.Setenv("DOUYIN_API_BASE", server.URL)
	t.Setenv("DOUYIN_API_KEY", "test-key")
	jar, _ := cookiejar.New(nil)
	link, err := resolveDouyinVideoURL(context.Background(), &http.Client{Jar: jar}, "https://v.douyin.com/abc/")
	if err != nil || link != "https://cdn.example.com/key.mp4" {
		t.Fatalf("unexpected result: link=%q err=%v", link, err)
	}
}

func TestDouyinAPIBaseDerivesOriginFromLegacyResolver(t *testing.T) {
	t.Setenv("DOUYIN_API_BASE", "")
	t.Setenv("DOUYIN_RESOLVER_URL", "https://resolver.example.com/api/douyin/web/fetch_one_video")
	if got := douyinAPIBase(); got != "https://resolver.example.com" {
		t.Fatalf("unexpected base: %q", got)
	}
	t.Setenv("DOUYIN_API_BASE", "https://api.example.com/")
	if got := douyinAPIBase(); got != "https://api.example.com" {
		t.Fatalf("unexpected configured base: %q", got)
	}
}
