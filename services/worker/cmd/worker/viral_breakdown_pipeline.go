package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starai/worker/internal/breakdown"
)

const (
	breakdownAnalysisVersion   = "breakdown-production-v2"
	breakdownScriptWindowMS    = 60000
	breakdownScriptMinWindowMS = 15000
)

func breakdownCheckpointMap(outputs map[string]interface{}, key string) map[string]interface{} {
	if outputs == nil {
		return map[string]interface{}{}
	}
	if items, ok := outputs[key].(map[string]interface{}); ok {
		return items
	}
	items := map[string]interface{}{}
	outputs[key] = items
	return items
}

func breakdownSegmentCheckpointKey(segment breakdown.Segment) string {
	return fmt.Sprintf("%d-%d", segment.StartMS, segment.EndMS)
}

func cachedBreakdownSegment(outputs map[string]interface{}, segment breakdown.Segment, modelCode string) (map[string]interface{}, bool) {
	row, _ := breakdownCheckpointMap(outputs, "segment_checkpoints")[breakdownSegmentCheckpointKey(segment)].(map[string]interface{})
	if stringAny(row["status"]) != "done" || stringAny(row["version"]) != breakdownAnalysisVersion || stringAny(row["model_code"]) != modelCode {
		return nil, false
	}
	result, ok := row["result"].(map[string]interface{})
	return result, ok && result != nil
}

func saveBreakdownSegmentCheckpoint(ctx context.Context, mu *sync.Mutex, pool *pgxpool.Pool, projectID int64, outputs map[string]interface{}, modelCode string, task breakdownSegmentTask, result map[string]interface{}, err error) {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	checkpoints := breakdownCheckpointMap(outputs, "segment_checkpoints")
	key := breakdownSegmentCheckpointKey(task.segment)
	previous, _ := checkpoints[key].(map[string]interface{})
	attempts := intAny(previous["attempts"]) + 1
	row := map[string]interface{}{
		"root": task.root, "depth": task.depth, "start_ms": task.segment.StartMS, "end_ms": task.segment.EndMS,
		"clip_start_ms": task.segment.ClipStartMS, "clip_end_ms": task.segment.ClipEndMS, "attempts": attempts,
		"model_code": modelCode, "version": breakdownAnalysisVersion,
	}
	if err == nil && result != nil {
		row["status"] = "done"
		row["result"] = result
	} else {
		row["status"] = "failed"
		if err != nil {
			row["error"] = truncateText(err.Error(), 240)
		}
	}
	checkpoints[key] = row
	outputs["segment_checkpoints"] = checkpoints
	outputs["worker_version"] = breakdownAnalysisVersion
	saveWorkflowOutputs(ctx, pool, projectID, outputs)
}

type breakdownScriptTask struct {
	Root    int
	Depth   int
	StartMS int
	EndMS   int
}

type breakdownScriptChunkResult struct {
	Script breakdown.Script
	Raw    map[string]interface{}
}

type breakdownScriptChunkRunner func(task breakdownScriptTask, previous []breakdown.Script, compact bool) (breakdownScriptChunkResult, error)
type breakdownScriptCheckpointSaver func(task breakdownScriptTask, status string, result *breakdownScriptChunkResult, err error, requestAttempts int)

func breakdownScriptTaskKey(task breakdownScriptTask) string {
	return fmt.Sprintf("%d-%d", task.StartMS, task.EndMS)
}

func breakdownScriptCanSplit(task breakdownScriptTask) bool {
	return task.EndMS-task.StartMS >= 2*breakdownScriptMinWindowMS
}

func breakdownScriptCheckpointNeedsSplit(row map[string]interface{}, task breakdownScriptTask) bool {
	if !breakdownScriptCanSplit(task) {
		return false
	}
	switch stringAny(row["status"]) {
	case "split", "split_failed", "split_done":
		return true
	case "failed":
		return breakdownCallTimeout(fmt.Errorf("%s", stringAny(row["error"])))
	default:
		return false
	}
}

func resolveBreakdownScriptTask(task breakdownScriptTask, previous []breakdown.Script, modelCode string, checkpoints map[string]interface{}, run breakdownScriptChunkRunner, save breakdownScriptCheckpointSaver) ([]breakdown.Script, error) {
	key := breakdownScriptTaskKey(task)
	row, _ := checkpoints[key].(map[string]interface{})
	currentCheckpoint := stringAny(row["version"]) == breakdownAnalysisVersion && stringAny(row["model_code"]) == modelCode
	if currentCheckpoint && stringAny(row["status"]) == "done" {
		if raw, ok := row["result"].(map[string]interface{}); ok {
			decoded := breakdown.DecodeScript(raw)
			if validateBreakdownScriptChunk(decoded, task.StartMS, task.EndMS) == nil {
				return []breakdown.Script{decoded}, nil
			}
		}
	}
	if currentCheckpoint && breakdownScriptCheckpointNeedsSplit(row, task) {
		status := stringAny(row["status"])
		if status == "failed" || status == "split_failed" {
			var checkpointErr error
			if message := strings.TrimSpace(stringAny(row["error"])); message != "" {
				checkpointErr = fmt.Errorf("%s", message)
			}
			save(task, "split", nil, checkpointErr, 0)
		}
		return resolveBreakdownScriptSplit(task, previous, modelCode, checkpoints, run, save)
	}

	result, err := run(task, previous, false)
	attempts := 1
	if err == nil {
		save(task, "done", &result, nil, attempts)
		return []breakdown.Script{result.Script}, nil
	}
	// Transport timeouts almost always repeat for the same payload. Split immediately
	// when possible. At the 15-second floor, a compact retry is the final fallback.
	if !breakdownCallTimeout(err) || !breakdownScriptCanSplit(task) {
		result, err = run(task, previous, true)
		attempts++
		if err == nil {
			save(task, "done", &result, nil, attempts)
			return []breakdown.Script{result.Script}, nil
		}
	}
	if breakdownScriptCanSplit(task) {
		save(task, "split", nil, err, attempts)
		return resolveBreakdownScriptSplit(task, previous, modelCode, checkpoints, run, save)
	}
	save(task, "failed", nil, err, attempts)
	return nil, err
}

func resolveBreakdownScriptSplit(task breakdownScriptTask, previous []breakdown.Script, modelCode string, checkpoints map[string]interface{}, run breakdownScriptChunkRunner, save breakdownScriptCheckpointSaver) ([]breakdown.Script, error) {
	midpoint := task.StartMS + (task.EndMS-task.StartMS)/2
	leftTask := breakdownScriptTask{Root: task.Root, Depth: task.Depth + 1, StartMS: task.StartMS, EndMS: midpoint}
	rightTask := breakdownScriptTask{Root: task.Root, Depth: task.Depth + 1, StartMS: midpoint, EndMS: task.EndMS}
	left, err := resolveBreakdownScriptTask(leftTask, previous, modelCode, checkpoints, run, save)
	if err != nil {
		save(task, "split_failed", nil, err, 0)
		return left, err
	}
	rightPrevious := append(append([]breakdown.Script(nil), previous...), left...)
	right, err := resolveBreakdownScriptTask(rightTask, rightPrevious, modelCode, checkpoints, run, save)
	resolved := append(append([]breakdown.Script(nil), left...), right...)
	if err != nil {
		save(task, "split_failed", nil, err, 0)
		return resolved, err
	}
	save(task, "split_done", nil, nil, 0)
	return resolved, nil
}

func buildBreakdownScriptChunks(ctx context.Context, pool *pgxpool.Pool, projectID int64, baseURL, token, publicID, modelCode string, durationMS int, evidence map[string]interface{}, notes []string, outputs map[string]interface{}, stages map[string]string) (breakdown.Script, error) {
	if durationMS < 1 {
		return breakdown.Script{}, fmt.Errorf("视频时长无效")
	}
	windows := breakdownScriptWindows(durationMS)
	chunkCount := len(windows)
	parts := make([]breakdown.Script, 0, chunkCount)
	checkpoints := breakdownCheckpointMap(outputs, "script_checkpoints")
	run := func(task breakdownScriptTask, previous []breakdown.Script, compact bool) (breakdownScriptChunkResult, error) {
		window := breakdownEvidenceWindow(evidence, task.StartMS, task.EndMS)
		contextText := breakdownScriptIdentityContext(previous)
		windowNote := fmt.Sprintf("当前只整理全片 %d–%d 毫秒。必须从 %d 毫秒覆盖到 %d 毫秒，按 3–8 秒拆成可独立生成的单镜头；一个 unit 只写一个主要动作。%s", task.StartMS, task.EndMS, task.StartMS, task.EndMS, contextText)
		user := breakdownScriptUser(window, append(notes, windowNote))
		timeout := breakdownScriptTimeout(maxInt(1, breakdownVideoEvidenceCount(window)))
		system := breakdownScriptSystem
		suffix := ""
		if compact {
			system = breakdownScriptRetrySystem
			suffix = "_compact"
		}
		requestID := fmt.Sprintf("%s_script_%02d_%d_%d%s", publicID, task.Root+1, task.StartMS, task.EndMS, suffix)
		text, err := breakdownLLM(ctx, pool, baseURL, token, requestID, modelCode, timeout, system, user, nil, nil)
		if err != nil {
			return breakdownScriptChunkResult{}, err
		}
		raw, err := breakdown.ParseJSONObject(text)
		if err != nil {
			return breakdownScriptChunkResult{}, err
		}
		decoded := breakdown.DecodeScript(raw)
		if err = validateBreakdownScriptChunk(decoded, task.StartMS, task.EndMS); err != nil {
			return breakdownScriptChunkResult{}, err
		}
		return breakdownScriptChunkResult{Script: decoded, Raw: raw}, nil
	}
	save := func(task breakdownScriptTask, status string, result *breakdownScriptChunkResult, callErr error, requestAttempts int) {
		key := breakdownScriptTaskKey(task)
		previous, _ := checkpoints[key].(map[string]interface{})
		row := map[string]interface{}{
			"root": task.Root, "depth": task.Depth,
			"start_ms": task.StartMS, "end_ms": task.EndMS,
			"attempts":   intAny(previous["attempts"]) + requestAttempts,
			"model_code": modelCode, "version": breakdownAnalysisVersion,
			"status": status,
		}
		if strings.HasPrefix(status, "split") {
			row["split_ms"] = task.StartMS + (task.EndMS-task.StartMS)/2
		}
		if result != nil && result.Raw != nil {
			row["result"] = result.Raw
		}
		if callErr != nil {
			row["error"] = truncateText(callErr.Error(), 240)
		}
		checkpoints[key] = row
		outputs["script_checkpoints"] = checkpoints
		outputs["worker_version"] = breakdownAnalysisVersion
		if status == "failed" || status == "split_failed" {
			outputs["completion_status"] = "degraded"
		}
		switch status {
		case "split":
			outputs["progress_message"] = fmt.Sprintf("剧本第 %d/%d 段未完成，正在把 %d–%d 秒拆小后续跑...", task.Root+1, chunkCount, task.StartMS/1000, task.EndMS/1000)
			log.Printf("viral breakdown script window split: project=%s root=%d range=%d-%d depth=%d error=%v", publicID, task.Root+1, task.StartMS, task.EndMS, task.Depth, callErr)
		case "split_done":
			log.Printf("viral breakdown script window recovered: project=%s root=%d range=%d-%d depth=%d", publicID, task.Root+1, task.StartMS, task.EndMS, task.Depth)
		case "split_failed":
			log.Printf("viral breakdown script window split failed: project=%s root=%d range=%d-%d depth=%d error=%v", publicID, task.Root+1, task.StartMS, task.EndMS, task.Depth, callErr)
		}
		saveWorkflowOutputs(ctx, pool, projectID, outputs)
	}

	for index, windowRange := range windows {
		task := breakdownScriptTask{Root: index, StartMS: windowRange[0], EndMS: windowRange[1]}
		resolved, err := resolveBreakdownScriptTask(task, parts, modelCode, checkpoints, run, save)
		parts = append(parts, resolved...)
		if err != nil {
			return breakdown.MergeScripts(parts), fmt.Errorf("剧本分层整理 %d/%d 未完成：%w", index+1, chunkCount, err)
		}
		percent := 84 + (index+1)*12/maxInt(chunkCount, 1)
		if percent > 96 {
			percent = 96
		}
		markBreakdown(ctx, pool, projectID, outputs, "script", fmt.Sprintf("正在分层整理剧本 %d/%d...", index+1, chunkCount), percent, stages)
	}
	return breakdown.MergeScripts(parts), nil
}

func breakdownScriptWindows(durationMS int) [][2]int {
	if durationMS <= 0 {
		return nil
	}
	var windows [][2]int
	for startMS := 0; startMS < durationMS; {
		endMS := startMS + breakdownScriptWindowMS
		if endMS >= durationMS || durationMS-endMS < 12000 {
			endMS = durationMS
		}
		windows = append(windows, [2]int{startMS, endMS})
		startMS = endMS
	}
	return windows
}

func validateBreakdownScriptChunk(script breakdown.Script, startMS, endMS int) error {
	if len(script.Units) == 0 {
		return fmt.Errorf("模型没有返回有效单镜头")
	}
	if strings.TrimSpace(script.Style) == "" {
		return fmt.Errorf("缺少统一视频风格")
	}
	startSec, endSec := startMS/1000, (endMS+999)/1000
	first, last := script.Units[0].StartSec, script.Units[len(script.Units)-1].EndSec
	if first > startSec+1 || last < endSec-1 {
		return fmt.Errorf("时间线未覆盖当前分段：得到 %d–%d 秒，需要 %d–%d 秒", first, last, startSec, endSec)
	}
	cursor := startSec
	for _, unit := range script.Units {
		if unit.EndSec <= unit.StartSec || unit.StartSec < startSec-2 || unit.EndSec > endSec+2 {
			return fmt.Errorf("镜头时间范围无效：%d–%d 秒", unit.StartSec, unit.EndSec)
		}
		if unit.StartSec > cursor+1 {
			return fmt.Errorf("镜头时间线在 %d–%d 秒之间存在缺口", cursor, unit.StartSec)
		}
		if unit.EndSec > cursor {
			cursor = unit.EndSec
		}
		duration := unit.EndSec - unit.StartSec
		shortWholeVideo := endSec-startSec < 2 && duration > 0
		if (!shortWholeVideo && duration < 2) || duration > 9 || len(unit.Shots) != 1 {
			return fmt.Errorf("镜头 %d–%d 秒必须是 2–9 秒且只包含一个 shot", unit.StartSec, unit.EndSec)
		}
		if strings.TrimSpace(unit.Scene) == "" || strings.TrimSpace(unit.Shots[0].Camera) == "" || strings.TrimSpace(unit.Shots[0].Action) == "" {
			return fmt.Errorf("镜头 %d–%d 秒缺少场景、景别或动作", unit.StartSec, unit.EndSec)
		}
		take := unit.Shots[0]
		if strings.TrimSpace(take.Dialogue) != "" && take.Dialogue != "无对白" && strings.TrimSpace(take.Speaker) == "" {
			return fmt.Errorf("镜头 %d–%d 秒有对白但缺少说话人", unit.StartSec, unit.EndSec)
		}
	}
	return nil
}

func invalidateBreakdownScriptCheckpoints(outputs map[string]interface{}, reason string) {
	for key, value := range breakdownCheckpointMap(outputs, "script_checkpoints") {
		row, _ := value.(map[string]interface{})
		if stringAny(row["status"]) != "done" {
			continue
		}
		row["status"] = "needs_review"
		row["error"] = truncateText(reason, 240)
		breakdownCheckpointMap(outputs, "script_checkpoints")[key] = row
	}
}

func breakdownEvidenceWindow(evidence map[string]interface{}, startMS, endMS int) map[string]interface{} {
	out := map[string]interface{}{}
	if shots, ok := evidence["shots"].([]breakdown.Shot); ok {
		var selected []breakdown.Shot
		for _, shot := range shots {
			if overlapsBreakdownRange(shot.StartMS, shot.EndMS, startMS, endMS) {
				selected = append(selected, shot)
			}
		}
		out["shots"] = selected
	} else {
		out["shots"] = evidence["shots"]
	}
	if cues, ok := evidence["cues"].([]breakdown.Cue); ok {
		var selected []breakdown.Cue
		for _, cue := range cues {
			if overlapsBreakdownRange(cue.StartMS, cue.EndMS, startMS, endMS) {
				selected = append(selected, cue)
			}
		}
		out["cues"] = selected
	} else {
		out["cues"] = evidence["cues"]
	}
	var frames []map[string]interface{}
	for _, frame := range breakdownMapSlice(evidence["frames"]) {
		at := intAny(frame["t_ms"])
		if at >= startMS && at < endMS {
			frames = append(frames, frame)
		}
	}
	out["frames"] = frames
	video, _ := evidence["video"].(map[string]interface{})
	var segments []map[string]interface{}
	for _, segment := range breakdownMapSlice(video["segments"]) {
		if overlapsBreakdownRange(intAny(segment["start_ms"]), intAny(segment["end_ms"]), startMS, endMS) {
			segments = append(segments, segment)
		}
	}
	out["video"] = map[string]interface{}{"segments": segments}
	return out
}

func breakdownVideoEvidenceCount(evidence map[string]interface{}) int {
	video, _ := evidence["video"].(map[string]interface{})
	return len(breakdownMapSlice(video["segments"]))
}

func breakdownMapSlice(value interface{}) []map[string]interface{} {
	switch items := value.(type) {
	case []map[string]interface{}:
		return items
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			if row, ok := item.(map[string]interface{}); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func reusableBreakdownFrames(outputs map[string]interface{}) []map[string]interface{} {
	frames := breakdownMapSlice(outputs["keyframes"])
	if len(frames) == 0 {
		return nil
	}
	for _, frame := range frames {
		if intAny(frame["t_ms"]) < 0 || strings.TrimSpace(stringAny(frame["url"])) == "" {
			return nil
		}
	}
	sort.SliceStable(frames, func(i, j int) bool { return intAny(frames[i]["t_ms"]) < intAny(frames[j]["t_ms"]) })
	return frames
}

func breakdownFrameDescriptionComplete(frame map[string]interface{}) bool {
	return strings.TrimSpace(stringAny(frame["scene"])) != "" &&
		strings.TrimSpace(stringAny(frame["action"])) != "" &&
		strings.TrimSpace(stringAny(frame["camera"])) != ""
}

func breakdownUndescribedFrameCount(frames []map[string]interface{}) int {
	missing := 0
	for _, frame := range frames {
		if !breakdownFrameDescriptionComplete(frame) {
			missing++
		}
	}
	return missing
}

func breakdownRootCoverage(parts []map[string]interface{}, roots []breakdown.Segment) (int, int) {
	done := 0
	for _, root := range roots {
		type span struct{ start, end int }
		var spans []span
		for _, part := range parts {
			start, end := intAny(part["start_ms"]), intAny(part["end_ms"])
			if !overlapsBreakdownRange(start, end, root.StartMS, root.EndMS) {
				continue
			}
			if start < root.StartMS {
				start = root.StartMS
			}
			if end > root.EndMS {
				end = root.EndMS
			}
			spans = append(spans, span{start: start, end: end})
		}
		sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
		cursor := root.StartMS
		for _, item := range spans {
			if item.start > cursor {
				break
			}
			if item.end > cursor {
				cursor = item.end
			}
		}
		if cursor >= root.EndMS {
			done++
		}
	}
	return len(roots), done
}

func overlapsBreakdownRange(start, end, wantedStart, wantedEnd int) bool {
	if end <= start {
		end = start + 1
	}
	return end > wantedStart && start < wantedEnd
}

func breakdownScriptIdentityContext(parts []breakdown.Script) string {
	if len(parts) == 0 {
		return ""
	}
	merged := breakdown.MergeScripts(parts)
	var people []string
	for _, person := range merged.Characters {
		people = append(people, strings.TrimSpace(person.Name+"："+person.Appearance))
	}
	var scenes []string
	if len(merged.Scenes) > 0 {
		scenes = merged.Scenes
	}
	return "沿用此前身份，不要改名。已确认人物：" + strings.Join(people, "；") + "。已确认场景：" + strings.Join(scenes, "；") + "。"
}

func breakdownTranscriptCues(parts []map[string]interface{}) []breakdown.Cue {
	var cues []breakdown.Cue
	for _, part := range parts {
		transcript, _ := part["transcript"].(map[string]interface{})
		for _, row := range breakdownMapSlice(transcript["cues"]) {
			text := strings.TrimSpace(stringAny(row["text"]))
			if text == "" {
				continue
			}
			cues = append(cues, breakdown.Cue{
				StartMS: intAny(row["start_ms"]), EndMS: intAny(row["end_ms"]), Text: text,
				Speaker: strings.TrimSpace(stringAny(row["speaker"])), Emotion: strings.TrimSpace(stringAny(row["emotion"])),
				Source: "audio", Confidence: floatAny(row["confidence"]),
			})
		}
	}
	return cues
}

func mergeBreakdownCues(groups ...[]breakdown.Cue) []breakdown.Cue {
	var merged []breakdown.Cue
	seen := map[string]int{}
	for _, group := range groups {
		for _, cue := range group {
			cue.Text = strings.TrimSpace(cue.Text)
			if cue.Text == "" {
				continue
			}
			key := fmt.Sprintf("%d:%d:%s", cue.StartMS/500, cue.EndMS/500, strings.Join(strings.Fields(cue.Text), ""))
			if index, ok := seen[key]; ok {
				if merged[index].Speaker == "" && cue.Speaker != "" {
					merged[index].Speaker = cue.Speaker
				}
				if merged[index].Emotion == "" && cue.Emotion != "" {
					merged[index].Emotion = cue.Emotion
				}
				if cue.Confidence > merged[index].Confidence {
					merged[index].Confidence = cue.Confidence
				}
				continue
			}
			seen[key] = len(merged)
			merged = append(merged, cue)
		}
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].StartMS == merged[j].StartMS {
			return cueSourcePriority(merged[i].Source) < cueSourcePriority(merged[j].Source)
		}
		return merged[i].StartMS < merged[j].StartMS
	})
	return merged
}

func cueSourcePriority(source string) int {
	switch source {
	case "soft":
		return 0
	case "audio":
		return 1
	case "ocr":
		return 2
	default:
		return 3
	}
}

func breakdownQualitySummary(pkg breakdown.ProductionPackage, segmentTotal, segmentDone int) map[string]interface{} {
	return map[string]interface{}{
		"status": pkg.Quality.Status, "coverage_percent": pkg.Quality.CoveragePercent, "max_gap_sec": pkg.Quality.MaxGapSec,
		"issues": pkg.Quality.Issues, "segment_total": segmentTotal, "segment_done": segmentDone,
		"segment_coverage_percent": float64(segmentDone) / float64(maxInt(segmentTotal, 1)) * 100,
		"generated_at":             time.Now().UTC().Format(time.RFC3339), "worker_version": breakdownAnalysisVersion,
	}
}
