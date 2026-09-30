package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/starai/worker/internal/breakdown"
)

func breakdownFakeTimeoutErr() error {
	return fmt.Errorf("上游拒绝请求：HTTP 200 context deadline exceeded (Client.Timeout or context cancellation while reading body)")
}

func TestRunBreakdownSegmentsHalvesTimedOutSegments(t *testing.T) {
	segments := breakdown.PlanSegments(240000, nil)
	if len(segments) != 8 {
		t.Fatalf("segments = %#v", segments)
	}
	var calls atomic.Int64
	parts, notes := runBreakdownSegments(segments, 240000, nil, func(task breakdownSegmentTask) (map[string]interface{}, error) {
		calls.Add(1)
		if task.root == 3 && task.depth == 0 {
			return nil, breakdownFakeTimeoutErr()
		}
		return map[string]interface{}{"style": "分段结果"}, nil
	})
	// 8 个根片段 + 根 3 超时后拆出的 2 个半段。
	if calls.Load() != 10 {
		t.Fatalf("calls = %d", calls.Load())
	}
	if len(parts) != 9 {
		t.Fatalf("parts = %d", len(parts))
	}
	for i := 1; i < len(parts); i++ {
		if intAny(parts[i]["start_ms"]) < intAny(parts[i-1]["start_ms"]) {
			t.Fatalf("parts not sorted: %#v", parts)
		}
	}
	if intAny(parts[0]["start_ms"]) != 0 || intAny(parts[len(parts)-1]["end_ms"]) != 240000 {
		t.Fatalf("coverage = %#v", parts)
	}
	if !containsNote(notes, "1 段超时片段拆成小片段重试后补全。") {
		t.Fatalf("notes = %#v", notes)
	}
	if containsNote(notes, "视频理解未完成") {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestRunBreakdownSegmentsRetriesDeeperUntilDepthLimit(t *testing.T) {
	segments := breakdown.PlanSegments(240000, nil)
	var calls atomic.Int64
	parts, notes := runBreakdownSegments(segments, 240000, nil, func(task breakdownSegmentTask) (map[string]interface{}, error) {
		calls.Add(1)
		if task.root == 4 {
			return nil, breakdownFakeTimeoutErr()
		}
		return map[string]interface{}{"style": "分段结果"}, nil
	})
	// 根 4 每层都超时：1 + 2 + 4 = 7 次调用后到深度上限放弃。
	if calls.Load() != 7+7 {
		t.Fatalf("calls = %d", calls.Load())
	}
	if len(parts) != 7 {
		t.Fatalf("parts = %d", len(parts))
	}
	if !containsNote(notes, "1 段视频理解未完成，已用其余片段和关键帧合并。") {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestRunBreakdownSegmentsRecoversAtSecondDepth(t *testing.T) {
	segments := breakdown.PlanSegments(240000, nil)
	parts, notes := runBreakdownSegments(segments, 240000, nil, func(task breakdownSegmentTask) (map[string]interface{}, error) {
		if task.root == 2 && task.depth < 2 {
			return nil, breakdownFakeTimeoutErr()
		}
		return map[string]interface{}{"style": "分段结果"}, nil
	})
	if len(parts) != 7+4 {
		t.Fatalf("parts = %d", len(parts))
	}
	if !containsNote(notes, "1 段超时片段拆成小片段重试后补全。") {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestRunBreakdownSegmentsSkipsNonTimeoutFailures(t *testing.T) {
	segments := breakdown.PlanSegments(240000, nil)
	var calls atomic.Int64
	parts, notes := runBreakdownSegments(segments, 240000, nil, func(task breakdownSegmentTask) (map[string]interface{}, error) {
		calls.Add(1)
		if task.root == 2 {
			return nil, fmt.Errorf("模型没有返回 JSON")
		}
		return map[string]interface{}{"style": "分段结果"}, nil
	})
	if calls.Load() != 8 {
		t.Fatalf("calls = %d", calls.Load())
	}
	if len(parts) != 7 {
		t.Fatalf("parts = %d", len(parts))
	}
	if !containsNote(notes, "1 段视频理解未完成，已用其余片段和关键帧合并。") {
		t.Fatalf("notes = %#v", notes)
	}
}

func TestRunBreakdownSegmentsReportsPartiallyRecoveredRootAsIncomplete(t *testing.T) {
	segments := breakdown.PlanSegments(240000, nil)
	parts, notes := runBreakdownSegments(segments, 240000, nil, func(task breakdownSegmentTask) (map[string]interface{}, error) {
		if task.root == 2 && (task.depth == 0 || task.segment.StartMS >= 45000) {
			return nil, breakdownFakeTimeoutErr()
		}
		return map[string]interface{}{"style": "分段结果"}, nil
	})
	if len(parts) == 0 {
		t.Fatal("expected partial results")
	}
	if !containsNote(notes, "1 段视频理解未完成，已用其余片段和关键帧合并。") {
		t.Fatalf("notes = %#v", notes)
	}
	if containsNote(notes, "超时片段拆成小片段重试后补全") {
		t.Fatalf("partial root must not be reported as recovered: %#v", notes)
	}
}

func TestBreakdownShouldHalve(t *testing.T) {
	if !breakdownShouldHalve(breakdownFakeTimeoutErr()) {
		t.Fatal("timeout should halve")
	}
	if !breakdownShouldHalve(fmt.Errorf("片段超过模型可读大小")) {
		t.Fatal("oversized clip should halve")
	}
	if breakdownShouldHalve(fmt.Errorf("模型没有返回 JSON")) {
		t.Fatal("parse error must not halve")
	}
	if breakdownShouldHalve(nil) {
		t.Fatal("nil must not halve")
	}
}

func TestBreakdownTimeouts(t *testing.T) {
	if got := breakdownSegmentTimeout(33000); got != 8*time.Minute+24*time.Second {
		t.Fatalf("33s clip timeout = %s", got)
	}
	if got := breakdownSegmentTimeout(0); got != 4*time.Minute+8*time.Second {
		t.Fatalf("minimal clip timeout = %s", got)
	}
	if got := breakdownSegmentTimeout(600000); got != 12*time.Minute {
		t.Fatalf("timeout cap = %s", got)
	}
	if got := breakdownScriptTimeout(8); got != 7*time.Minute {
		t.Fatalf("script timeout = %s", got)
	}
	if got := breakdownScriptTimeout(40); got != 12*time.Minute {
		t.Fatalf("script timeout cap = %s", got)
	}
}

func TestBreakdownTimeoutFloorOverridesShortChatRouteTimeout(t *testing.T) {
	floor := breakdownSegmentTimeout(33000)
	ctx := context.WithValue(context.Background(), breakdownTimeoutKey{}, floor)
	if got := workerLLMRequestTimeout(ctx, 4*time.Minute, 90); got != floor {
		t.Fatalf("breakdown timeout = %s, want %s", got, floor)
	}
	if got := workerLLMRequestTimeout(context.Background(), 4*time.Minute, 90); got != 90*time.Second {
		t.Fatalf("ordinary route timeout = %s", got)
	}
}

func TestBreakdownRootCoverageAcceptsSplitSegments(t *testing.T) {
	roots := []breakdown.Segment{{Index: 1, StartMS: 0, EndMS: 30000}, {Index: 2, StartMS: 30000, EndMS: 60000}}
	parts := []map[string]interface{}{
		{"start_ms": 0, "end_ms": 15000}, {"start_ms": 15000, "end_ms": 30000}, {"start_ms": 30000, "end_ms": 60000},
	}
	total, done := breakdownRootCoverage(parts, roots)
	if total != 2 || done != 2 {
		t.Fatalf("coverage = %d/%d", done, total)
	}
	parts = parts[:2]
	_, done = breakdownRootCoverage(parts, roots)
	if done != 1 {
		t.Fatalf("partial coverage = %d", done)
	}
}

func TestMergeBreakdownCuesPrefersSubtitleAndEnrichesSpeaker(t *testing.T) {
	soft := []breakdown.Cue{{StartMS: 1000, EndMS: 2400, Text: "既应了", Source: "soft"}}
	audio := []breakdown.Cue{{StartMS: 1050, EndMS: 2450, Text: "既应了", Speaker: "黑衣男子", Emotion: "冷静", Source: "audio", Confidence: 0.92}}
	merged := mergeBreakdownCues(soft, audio)
	if len(merged) != 1 || merged[0].Source != "soft" || merged[0].Speaker != "黑衣男子" || merged[0].Confidence != 0.92 {
		t.Fatalf("cues = %#v", merged)
	}
}

func TestCachedBreakdownSegmentRequiresCurrentModelAndVersion(t *testing.T) {
	segment := breakdown.Segment{StartMS: 0, EndMS: 30000}
	outputs := map[string]interface{}{"segment_checkpoints": map[string]interface{}{
		"0-30000": map[string]interface{}{"status": "done", "version": breakdownAnalysisVersion, "model_code": "vision-v2", "result": map[string]interface{}{"style": "雨夜"}},
	}}
	if _, ok := cachedBreakdownSegment(outputs, segment, "vision-v2"); !ok {
		t.Fatal("current checkpoint should be reusable")
	}
	if _, ok := cachedBreakdownSegment(outputs, segment, "vision-v3"); ok {
		t.Fatal("checkpoint from another model must not be reused")
	}
	outputs["segment_checkpoints"].(map[string]interface{})["0-30000"].(map[string]interface{})["version"] = "old"
	if _, ok := cachedBreakdownSegment(outputs, segment, "vision-v2"); ok {
		t.Fatal("old checkpoint must not be reused")
	}
}

func TestBreakdownScriptWindowsAvoidTinyTail(t *testing.T) {
	tests := []struct {
		duration int
		want     [][2]int
	}{
		{61000, [][2]int{{0, 61000}}},
		{121000, [][2]int{{0, 60000}, {60000, 121000}}},
		{600000, [][2]int{{0, 60000}, {60000, 120000}, {120000, 180000}, {180000, 240000}, {240000, 300000}, {300000, 360000}, {360000, 420000}, {420000, 480000}, {480000, 540000}, {540000, 600000}}},
	}
	for _, tt := range tests {
		got := breakdownScriptWindows(tt.duration)
		if len(got) != len(tt.want) {
			t.Fatalf("duration %d windows = %#v", tt.duration, got)
		}
		for index := range got {
			if got[index] != tt.want[index] {
				t.Fatalf("duration %d window %d = %#v, want %#v", tt.duration, index, got[index], tt.want[index])
			}
		}
	}
}

func TestResolveBreakdownScriptTaskSplitsTimeoutWithoutSamePayloadRetry(t *testing.T) {
	checkpoints := map[string]interface{}{}
	var calls []string
	run := func(task breakdownScriptTask, _ []breakdown.Script, compact bool) (breakdownScriptChunkResult, error) {
		calls = append(calls, fmt.Sprintf("%d-%d:%t", task.StartMS, task.EndMS, compact))
		if task.StartMS == 0 && task.EndMS == 60000 {
			return breakdownScriptChunkResult{}, breakdownFakeTimeoutErr()
		}
		return breakdownTestScriptResult(task.StartMS, task.EndMS), nil
	}
	resolved, err := resolveBreakdownScriptTask(
		breakdownScriptTask{StartMS: 0, EndMS: 60000}, nil, "script-v1", checkpoints, run, breakdownTestScriptSaver(checkpoints, "script-v1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved = %d", len(resolved))
	}
	want := []string{"0-60000:false", "0-30000:false", "30000-60000:false"}
	if fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
	if status := stringAny(checkpoints["0-60000"].(map[string]interface{})["status"]); status != "split_done" {
		t.Fatalf("parent status = %s", status)
	}
}

func TestResolveBreakdownScriptTaskSplitsThirtySecondsToFifteen(t *testing.T) {
	checkpoints := map[string]interface{}{}
	var calls []string
	run := func(task breakdownScriptTask, _ []breakdown.Script, compact bool) (breakdownScriptChunkResult, error) {
		calls = append(calls, fmt.Sprintf("%d-%d:%t", task.StartMS, task.EndMS, compact))
		if task.StartMS == 0 && (task.EndMS == 60000 || task.EndMS == 30000) {
			return breakdownScriptChunkResult{}, breakdownFakeTimeoutErr()
		}
		return breakdownTestScriptResult(task.StartMS, task.EndMS), nil
	}
	resolved, err := resolveBreakdownScriptTask(
		breakdownScriptTask{StartMS: 0, EndMS: 60000}, nil, "script-v1", checkpoints, run, breakdownTestScriptSaver(checkpoints, "script-v1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 3 {
		t.Fatalf("resolved = %d", len(resolved))
	}
	want := []string{"0-60000:false", "0-30000:false", "0-15000:false", "15000-30000:false", "30000-60000:false"}
	if fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestResolveBreakdownScriptTaskUsesCompletedCheckpoint(t *testing.T) {
	result := breakdownTestScriptResult(0, 60000)
	checkpoints := map[string]interface{}{
		"0-60000": map[string]interface{}{
			"status": "done", "version": breakdownAnalysisVersion, "model_code": "script-v1", "result": result.Raw,
		},
	}
	calls := 0
	run := func(task breakdownScriptTask, _ []breakdown.Script, compact bool) (breakdownScriptChunkResult, error) {
		calls++
		return breakdownTestScriptResult(task.StartMS, task.EndMS), nil
	}
	resolved, err := resolveBreakdownScriptTask(
		breakdownScriptTask{StartMS: 0, EndMS: 60000}, nil, "script-v1", checkpoints, run, breakdownTestScriptSaver(checkpoints, "script-v1"),
	)
	if err != nil || len(resolved) != 1 || calls != 0 {
		t.Fatalf("resolved=%d calls=%d err=%v", len(resolved), calls, err)
	}
}

func TestResolveBreakdownScriptTaskSplitsLegacyTimeoutCheckpointDirectly(t *testing.T) {
	checkpoints := map[string]interface{}{
		"0-60000": map[string]interface{}{
			"status": "failed", "version": breakdownAnalysisVersion, "model_code": "script-v1", "attempts": 2,
			"error": breakdownFakeTimeoutErr().Error(),
		},
	}
	var calls []string
	run := func(task breakdownScriptTask, _ []breakdown.Script, compact bool) (breakdownScriptChunkResult, error) {
		calls = append(calls, fmt.Sprintf("%d-%d", task.StartMS, task.EndMS))
		return breakdownTestScriptResult(task.StartMS, task.EndMS), nil
	}
	_, err := resolveBreakdownScriptTask(
		breakdownScriptTask{StartMS: 0, EndMS: 60000}, nil, "script-v1", checkpoints, run, breakdownTestScriptSaver(checkpoints, "script-v1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"0-30000", "30000-60000"}
	if fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestResolveBreakdownScriptTaskRetriesOnlyFailedChild(t *testing.T) {
	left := breakdownTestScriptResult(0, 15000)
	checkpoints := map[string]interface{}{
		"0-30000": map[string]interface{}{
			"status": "split_failed", "version": breakdownAnalysisVersion, "model_code": "script-v1",
		},
		"0-15000": map[string]interface{}{
			"status": "done", "version": breakdownAnalysisVersion, "model_code": "script-v1", "result": left.Raw,
		},
		"15000-30000": map[string]interface{}{
			"status": "failed", "version": breakdownAnalysisVersion, "model_code": "script-v1", "error": breakdownFakeTimeoutErr().Error(),
		},
	}
	var calls []string
	run := func(task breakdownScriptTask, _ []breakdown.Script, compact bool) (breakdownScriptChunkResult, error) {
		calls = append(calls, fmt.Sprintf("%d-%d", task.StartMS, task.EndMS))
		return breakdownTestScriptResult(task.StartMS, task.EndMS), nil
	}
	resolved, err := resolveBreakdownScriptTask(
		breakdownScriptTask{StartMS: 0, EndMS: 30000}, nil, "script-v1", checkpoints, run, breakdownTestScriptSaver(checkpoints, "script-v1"),
	)
	if err != nil || len(resolved) != 2 {
		t.Fatalf("resolved=%d err=%v", len(resolved), err)
	}
	if fmt.Sprint(calls) != fmt.Sprint([]string{"15000-30000"}) {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestResolveBreakdownScriptTaskCompactRetriesValidationFailure(t *testing.T) {
	checkpoints := map[string]interface{}{}
	var compactCalls []bool
	run := func(task breakdownScriptTask, _ []breakdown.Script, compact bool) (breakdownScriptChunkResult, error) {
		compactCalls = append(compactCalls, compact)
		if !compact {
			return breakdownScriptChunkResult{}, fmt.Errorf("模型没有返回有效 JSON")
		}
		return breakdownTestScriptResult(task.StartMS, task.EndMS), nil
	}
	_, err := resolveBreakdownScriptTask(
		breakdownScriptTask{StartMS: 0, EndMS: 60000}, nil, "script-v1", checkpoints, run, breakdownTestScriptSaver(checkpoints, "script-v1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(compactCalls) != fmt.Sprint([]bool{false, true}) {
		t.Fatalf("compact calls = %#v", compactCalls)
	}
}

func TestResolveBreakdownScriptTaskCompactRetriesTimeoutAtMinimumWindow(t *testing.T) {
	checkpoints := map[string]interface{}{}
	var compactCalls []bool
	run := func(task breakdownScriptTask, _ []breakdown.Script, compact bool) (breakdownScriptChunkResult, error) {
		compactCalls = append(compactCalls, compact)
		if !compact {
			return breakdownScriptChunkResult{}, breakdownFakeTimeoutErr()
		}
		return breakdownTestScriptResult(task.StartMS, task.EndMS), nil
	}
	_, err := resolveBreakdownScriptTask(
		breakdownScriptTask{StartMS: 0, EndMS: breakdownScriptMinWindowMS}, nil, "script-v1", checkpoints, run, breakdownTestScriptSaver(checkpoints, "script-v1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(compactCalls) != fmt.Sprint([]bool{false, true}) {
		t.Fatalf("compact calls = %#v", compactCalls)
	}
}

func breakdownTestScriptResult(startMS, endMS int) breakdownScriptChunkResult {
	startSec := startMS / 1000
	endSec := (endMS + 999) / 1000
	script := breakdown.Script{Style: "测试风格", Scenes: []string{"测试场景"}}
	for start := startSec; start < endSec; {
		end := start + 5
		if end > endSec || endSec-end < 2 {
			end = endSec
		}
		script.Units = append(script.Units, breakdown.Unit{
			StartSec: start, EndSec: end, Scene: "测试场景",
			Shots: []breakdown.Take{{Camera: "中景", Action: "人物完成动作", Dialogue: "无对白"}},
		})
		start = end
	}
	payload, _ := json.Marshal(script)
	raw := map[string]interface{}{}
	_ = json.Unmarshal(payload, &raw)
	return breakdownScriptChunkResult{Script: script, Raw: raw}
}

func breakdownTestScriptSaver(checkpoints map[string]interface{}, modelCode string) breakdownScriptCheckpointSaver {
	return func(task breakdownScriptTask, status string, result *breakdownScriptChunkResult, err error, requestAttempts int) {
		key := breakdownScriptTaskKey(task)
		previous, _ := checkpoints[key].(map[string]interface{})
		row := map[string]interface{}{
			"status": status, "version": breakdownAnalysisVersion, "model_code": modelCode,
			"attempts": intAny(previous["attempts"]) + requestAttempts,
		}
		if result != nil {
			row["result"] = result.Raw
		}
		if err != nil {
			row["error"] = err.Error()
		}
		checkpoints[key] = row
	}
}

func containsNote(notes []string, want string) bool {
	for _, note := range notes {
		if strings.Contains(note, want) {
			return true
		}
	}
	return false
}
