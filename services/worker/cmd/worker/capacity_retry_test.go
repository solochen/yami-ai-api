package main

import (
	"strings"
	"testing"
)

// 上游（OpenRouter）按在途请求总额度限流，返回的错误必须被识别为「并发不足」，
// 而不是被当成参数错误直接永久失败。
func TestUpstreamCapacityLimitedDetection(t *testing.T) {
	body := []byte(`{"error":{"message":"This request would exceed your available credits given your current in-flight requests. Retry after in-flight requests settle, or add credits.","code":402}}`)
	if !upstreamCapacityLimited(402, body) {
		t.Fatal("未识别 OpenRouter 在途额度不足")
	}
	if !upstreamCapacityLimited(429, []byte(`{"error":"rate limit"}`)) {
		t.Fatal("429 未识别为并发限制")
	}
	if upstreamCapacityLimited(400, []byte(`{"error":"invalid size"}`)) {
		t.Fatal("参数错误被误判为并发限制")
	}
	msg := upstreamCapacityMessage(upstreamCapacityConcurrency)
	if !strings.HasPrefix(msg, "并发不足") {
		t.Fatalf("提示文案应以「并发不足」开头: %s", msg)
	}
	// 走 humanize 兜底路径时同样给出并发不足文案。
	if got := humanizeUpstreamFailure(`upstream HTTP 402: {"error":{"message":"This request would exceed your available credits given your current in-flight requests."}}`); !strings.HasPrefix(got, "并发不足") {
		t.Fatalf("humanize 未输出并发不足: %s", got)
	}
}

// OpenRouter 另一种拒法：本次请求的最高预估费用超过账户余额（与实扣无关）。
// 图片和视频都会这样拒绝，重试无意义，必须给出「充额度或降低规格」的提示。
func TestUpstreamCreditsExhaustedDetection(t *testing.T) {
	body := []byte(`{"error":{"message":"This request's maximum cost exceeds your available credits. Add credits, or request fewer images.","code":402}}`)
	if got := classifyUpstreamCapacity(402, body); got != upstreamCapacityCredits {
		t.Fatalf("未识别额度不足: %v", got)
	}
	msg := upstreamCapacityMessage(upstreamCapacityCredits)
	if !strings.HasPrefix(msg, "上游额度不足") {
		t.Fatalf("应提示上游额度不足: %s", msg)
	}
	if got := humanizeUpstreamFailure(`This request's maximum cost exceeds your available credits. Add credits, or request fewer images.`); !strings.HasPrefix(got, "上游额度不足") {
		t.Fatalf("humanize 未输出额度不足: %s", got)
	}
	video := []byte(`{"error":{"message":"This request's maximum cost exceeds your available credits. Add credits, or request a shorter or lower-resolution video.","code":402}}`)
	if got := classifyUpstreamCapacity(402, video); got != upstreamCapacityCredits {
		t.Fatalf("视频额度不足未识别: %v", got)
	}
	plain := []byte(`{"error":{"message":"Insufficient credits. Add more using https://openrouter.ai/settings/credits","code":402}}`)
	if got := classifyUpstreamCapacity(402, plain); got != upstreamCapacityCredits {
		t.Fatalf("账户余额不足未识别: %v", got)
	}
	if !workerStatusCanFailover(402) {
		t.Fatal("402 应切换到下一条供应商线路")
	}
}
