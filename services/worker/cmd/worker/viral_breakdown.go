package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starai/worker/internal/breakdown"
)

const breakdownMaxBytes int64 = 500 << 20
const breakdownProxyBytes int64 = 16 << 20

func processViralVideoBreakdownWorkflow(ctx context.Context, pool *pgxpool.Pool, baseURL, token string, p WorkflowTaskPayload, publicID string, estimated float64, inputs, runtimeCfg map[string]interface{}) error {
	outputs := loadWorkflowOutputs(ctx, pool, p.ProjectID)
	stages := map[string]string{"understand": "pending", "subtitle": "pending", "frames": "pending", "verify": "pending", "script": "pending"}
	outputs["worker_version"] = breakdownAnalysisVersion
	outputs["analysis_version"] = breakdownAnalysisVersion
	outputs["completion_status"] = "running"
	for _, key := range []string{"production_package", "production_markdown", "quality", "notes", "rewrites"} {
		delete(outputs, key)
	}
	markBreakdown(ctx, pool, p.ProjectID, outputs, "resolve", "正在解析视频来源...", 6, stages)
	if objectStore == nil {
		return failBreakdown(ctx, pool, p, publicID, estimated, outputs, "对象存储未配置，无法保存关键帧")
	}
	dir, err := os.MkdirTemp("", "starai-breakdown-*")
	if err != nil {
		return failBreakdown(ctx, pool, p, publicID, estimated, outputs, "创建临时目录失败")
	}
	defer os.RemoveAll(dir)

	sourceURL := strings.TrimSpace(firstNonEmpty(stringAny(inputs["video_url"]), stringAny(inputs["source_url"])))
	localPath, err := downloadBreakdownVideo(ctx, sourceURL, filepath.Join(dir, "source.mp4"))
	if err != nil {
		return failBreakdown(ctx, pool, p, publicID, estimated, outputs, err.Error())
	}
	info, err := probeBreakdownMedia(ctx, localPath)
	if err != nil {
		return failBreakdown(ctx, pool, p, publicID, estimated, outputs, err.Error())
	}
	maxMS := intAny(runtimeCfg["max_duration_sec"]) * 1000
	if maxMS <= 0 {
		maxMS = breakdown.MaxDurationMS
	}
	if info.DurationMS > maxMS {
		return failBreakdown(ctx, pool, p, publicID, estimated, outputs, fmt.Sprintf("视频超过 %d 秒，请裁短后再分析", maxMS/1000))
	}
	outputs["duration_ms"] = info.DurationMS
	outputs["width"] = info.Width
	outputs["height"] = info.Height
	outputs["source_url"] = sourceURL
	markBreakdown(ctx, pool, p.ProjectID, outputs, "understand", "正在整理角色、场景、道具和节奏线索...", 18, stages)

	nodeID := insertWorkflowNodeRun(ctx, pool, p.ProjectID, "breakdown", "爆款视频拆解", "tool", map[string]interface{}{"source_url": sourceURL}, 0)
	started := time.Now()
	failNode := func(message string) error {
		pool.Exec(ctx, `UPDATE workflow_node_runs SET status='failed', error=$1, duration_ms=$2 WHERE id=$3 AND status='running'`, message, time.Since(started).Milliseconds(), nodeID)
		return failBreakdown(ctx, pool, p, publicID, estimated, outputs, message)
	}
	if stopped, stopErr := stopWorkflowIfRequested(ctx, pool, p, publicID, estimated); stopped {
		return stopErr
	}
	modelCode := strings.TrimSpace(stringAny(runtimeCfg["analysis_model_code"]))
	if modelCode == "" {
		outputs["completion_status"] = "failed"
		return failNode("后台未配置分析模型，无法生成完整拆解")
	}

	stages["understand"] = "running"
	stages["subtitle"] = "running"
	stages["frames"] = "running"
	markBreakdown(ctx, pool, p.ProjectID, outputs, "media", "正在并行执行预剪辑、字幕提取与参考关键帧提取...", 30, stages)
	cuts := detectBreakdownCuts(ctx, localPath, info.DurationMS)
	shots := breakdown.BuildShots(info.DurationMS, cuts)
	cues := []breakdown.Cue{}
	if info.HasSubtitle {
		if raw, readErr := extractBreakdownSRT(ctx, localPath, filepath.Join(dir, "subs.srt")); readErr == nil {
			cues = breakdown.ParseSRT(raw)
		}
	}
	frames := reusableBreakdownFrames(outputs)
	var frameErr error
	if len(frames) == 0 {
		frames, frameErr = extractBreakdownFrames(ctx, publicID, localPath, dir, info.DurationMS, shots)
	}
	if frameErr != nil && len(frames) == 0 {
		outputs["completion_status"] = "failed"
		return failNode(frameErr.Error())
	}
	stages["subtitle"] = "done"
	stages["frames"] = "running"
	outputs["keyframes"] = frames
	outputs["frame_total"] = len(frames)
	outputs["shot_count"] = len(shots)
	markBreakdown(ctx, pool, p.ProjectID, outputs, "frames", fmt.Sprintf("正在分析至 0/%d 帧画面...", len(frames)), 40, stages)

	evidence := map[string]interface{}{"shots": shots, "cues": cues}
	var notes []string
	stages["understand"] = "running"
	var progressMu sync.Mutex
	segments := breakdown.PlanSegments(info.DurationMS, shots)
	var videoParts []map[string]interface{}
	var videoNotes []string
	var videoWG sync.WaitGroup
	videoWG.Add(1)
	go func() {
		defer videoWG.Done()
		videoParts, videoNotes = understandBreakdownSegments(ctx, &progressMu, pool, p.ProjectID, baseURL, token, publicID, modelCode, localPath, dir, info.DurationMS, info.HasAudio, shots, cues, segments, outputs, stages)
	}()
	described := describeBreakdownFrames(ctx, &progressMu, pool, p.ProjectID, baseURL, token, publicID, modelCode, frames, outputs, stages)
	videoWG.Wait()
	notes = append(notes, videoNotes...)
	evidence["video"] = map[string]interface{}{"segments": videoParts}
	evidence["frames"] = described
	outputs["keyframes"] = described
	var ocrCues []breakdown.Cue
	for _, frame := range described {
		if text := strings.TrimSpace(stringAny(frame["on_screen_text"])); text != "" {
			ocrCues = append(ocrCues, breakdown.Cue{StartMS: intAny(frame["t_ms"]), EndMS: intAny(frame["t_ms"]) + 800, Text: text, Source: "ocr"})
		}
	}
	cues = mergeBreakdownCues(cues, breakdownTranscriptCues(videoParts), ocrCues)
	evidence["cues"] = cues
	outputs["cues"] = cues
	outputs["notes"] = notes
	segmentTotal, segmentDone := breakdownRootCoverage(videoParts, segments)
	outputs["segment_checkpoint_summary"] = map[string]interface{}{
		"total": segmentTotal, "done": segmentDone, "failed": segmentTotal - segmentDone,
		"coverage_percent": float64(segmentDone) / float64(maxInt(segmentTotal, 1)) * 100,
	}
	stages["frames"] = "done"
	stages["understand"] = "done"

	fallback := breakdown.FallbackScript(info.DurationMS, shots, cues)
	if facts := breakdownFrameFacts(described); len(facts) > 0 {
		fallback = breakdown.ScriptFromFrames(info.DurationMS, facts, cues)
	}
	savePreview := func(script breakdown.Script) {
		outputs["script"] = script
		outputs["script_markdown"] = breakdown.RenderMarkdown(script)
		outputs["frame_kept"] = len(described)
		outputs["notes"] = notes
	}
	if missing := breakdownUndescribedFrameCount(described); missing > 0 {
		message := fmt.Sprintf("%d 个关键帧理解未完成，已保存成功结果；请点击仅重试失败片段", missing)
		notes = append(notes, message)
		stages["frames"] = "failed"
		stages["understand"] = "failed"
		outputs["completion_status"] = "degraded"
		outputs["quality"] = map[string]interface{}{"status": "failed", "issues": []string{message}, "segment_total": segmentTotal, "segment_done": segmentDone}
		savePreview(fallback)
		markBreakdown(ctx, pool, p.ProjectID, outputs, "failed", message, 78, stages)
		return failNode(message)
	}
	if segmentDone != segmentTotal {
		message := fmt.Sprintf("视频理解仍有 %d/%d 段未补全，已保存成功片段；请点击仅重试失败片段", segmentTotal-segmentDone, segmentTotal)
		notes = append(notes, message)
		stages["understand"] = "failed"
		outputs["completion_status"] = "degraded"
		outputs["quality"] = map[string]interface{}{"status": "failed", "issues": []string{message}, "segment_total": segmentTotal, "segment_done": segmentDone}
		savePreview(fallback)
		markBreakdown(ctx, pool, p.ProjectID, outputs, "failed", message, 80, stages)
		return failNode(message)
	}

	stages["verify"] = "running"
	markBreakdown(ctx, pool, p.ProjectID, outputs, "verify", "正在进行分句与镜头密度校验...", 82, stages)
	if stopped, stopErr := stopWorkflowIfRequested(ctx, pool, p, publicID, estimated); stopped {
		return stopErr
	}

	stages["script"] = "running"
	script, scriptErr := buildBreakdownScriptChunks(ctx, pool, p.ProjectID, baseURL, token, publicID, modelCode, info.DurationMS, evidence, notes, outputs, stages)
	if scriptErr != nil {
		message := truncateText(scriptErr.Error(), 240)
		notes = append(notes, message)
		stages["script"] = "failed"
		outputs["completion_status"] = "degraded"
		outputs["quality"] = map[string]interface{}{"status": "failed", "issues": []string{message}, "segment_total": segmentTotal, "segment_done": segmentDone}
		if len(script.Units) == 0 {
			script = fallback
		}
		savePreview(script)
		markBreakdown(ctx, pool, p.ProjectID, outputs, "failed", message+"；成功分段已保存，可仅重试失败分段", 90, stages)
		return failNode(message)
	}
	production := breakdown.BuildProductionPackage(script, info.DurationMS)
	quality := breakdownQualitySummary(production, segmentTotal, segmentDone)
	outputs["production_package"] = production
	outputs["production_markdown"] = breakdown.RenderProductionMarkdown(production)
	outputs["quality"] = quality
	outputs["script_markdown"] = breakdown.RenderMarkdown(script)
	outputs["script"] = script
	outputs["frame_kept"] = len(described)
	outputs["notes"] = notes
	if production.Quality.Status != "passed" {
		message := "生成包质量门禁未通过：" + strings.Join(production.Quality.Issues, "；")
		invalidateBreakdownScriptCheckpoints(outputs, message)
		stages["verify"] = "failed"
		stages["script"] = "done"
		outputs["completion_status"] = "degraded"
		markBreakdown(ctx, pool, p.ProjectID, outputs, "failed", message, 98, stages)
		return failNode(message)
	}
	stages["verify"] = "done"
	stages["script"] = "done"
	outputs["completion_status"] = "complete"
	markBreakdown(ctx, pool, p.ProjectID, outputs, "result", "完整拆解与可生成镜头包已生成", 100, stages)
	cost := breakdownDurationCost(ctx, pool, p.ProjectID, info.DurationMS, estimated)
	updateNodeRunSuccess(ctx, pool, nodeID, map[string]interface{}{"frame_kept": len(described), "shot_count": len(shots), "production_shot_count": len(production.Shots), "quality": production.Quality}, cost, int(time.Since(started).Milliseconds()))
	return completeSimpleAgentWorkflow(ctx, pool, p, publicID, estimated, outputs)
}

func markBreakdown(ctx context.Context, pool *pgxpool.Pool, projectID int64, outputs map[string]interface{}, step, message string, percent int, stages map[string]string) {
	outputs["current_step"] = step
	outputs["progress_message"] = message
	outputs["progress_percent"] = percent
	copied := map[string]string{}
	for key, value := range stages {
		copied[key] = value
	}
	outputs["stages"] = copied
	saveWorkflowOutputs(ctx, pool, projectID, outputs)
}

func failBreakdown(ctx context.Context, pool *pgxpool.Pool, p WorkflowTaskPayload, publicID string, estimated float64, outputs map[string]interface{}, message string) error {
	if stringAny(outputs["completion_status"]) == "" || stringAny(outputs["completion_status"]) == "running" {
		outputs["completion_status"] = "failed"
	}
	outputs["worker_version"] = breakdownAnalysisVersion
	outputs["analysis_version"] = breakdownAnalysisVersion
	outputs["current_step"] = "failed"
	outputs["progress_message"] = message
	saveWorkflowOutputs(ctx, pool, p.ProjectID, outputs)
	return failWorkflow(ctx, pool, p, publicID, estimated, message)
}

func breakdownDurationCost(ctx context.Context, pool *pgxpool.Pool, projectID int64, durationMS int, estimated float64) float64 {
	rate := floatAny(workflowPriceRule(ctx, pool, projectID)["unit_price"])
	if rate <= 0 || durationMS <= 0 {
		return 0
	}
	cost := math.Ceil(float64(durationMS)/1000) * rate
	if estimated > 0 && cost > estimated {
		return estimated
	}
	return cost
}

type breakdownMedia struct {
	DurationMS  int
	Width       int
	Height      int
	HasAudio    bool
	HasSubtitle bool
}

func probeBreakdownMedia(ctx context.Context, path string) (breakdownMedia, error) {
	ffprobe := "ffprobe"
	if ffmpegPath, err := ffmpegBinaryPath(); err == nil {
		candidate := filepath.Join(filepath.Dir(ffmpegPath), ffprobeExecutableName())
		if _, statErr := os.Stat(candidate); statErr == nil {
			ffprobe = candidate
		}
	}
	cmd := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_entries", "format=duration:stream=codec_type,width,height", "-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		return breakdownMedia{}, fmt.Errorf("无法读取视频，请确认文件可以播放")
	}
	var raw struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if json.Unmarshal(out, &raw) != nil {
		return breakdownMedia{}, fmt.Errorf("无法读取视频，请确认文件可以播放")
	}
	seconds := floatAny(raw.Format.Duration)
	info := breakdownMedia{DurationMS: int(math.Round(seconds * 1000))}
	for _, stream := range raw.Streams {
		switch stream.CodecType {
		case "video":
			if info.Width == 0 {
				info.Width, info.Height = stream.Width, stream.Height
			}
		case "audio":
			info.HasAudio = true
		case "subtitle":
			info.HasSubtitle = true
		}
	}
	if info.Width == 0 || info.DurationMS <= 0 {
		return breakdownMedia{}, fmt.Errorf("文件里没有可分析的视频画面")
	}
	return info, nil
}

func detectBreakdownCuts(ctx context.Context, path string, durationMS int) []int {
	cuts := runSceneDetect(ctx, path, 0.30)
	shots := breakdown.BuildShots(durationMS, cuts)
	if len(shots) > 40 {
		cuts = runSceneDetect(ctx, path, 0.45)
		shots = breakdown.BuildShots(durationMS, cuts)
	}
	if len(shots) == 1 && durationMS > 8000 {
		if retry := runSceneDetect(ctx, path, 0.18); len(retry) > 0 {
			cuts = retry
		}
	}
	return cuts
}

func runSceneDetect(ctx context.Context, path string, threshold float64) []int {
	ffmpegPath, err := ffmpegBinaryPath()
	if err != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, ffmpegPath, "-i", path, "-vf", fmt.Sprintf("select='gt(scene,%.2f)',showinfo", threshold), "-an", "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run()
	return breakdown.ParseSceneTimes(stderr.String())
}

func extractBreakdownSRT(ctx context.Context, video, dest string) (string, error) {
	if err := runFFmpeg(ctx, "-y", "-i", video, "-map", "0:s:0", dest); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(dest)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func extractBreakdownFrames(ctx context.Context, publicID, video, dir string, durationMS int, shots []breakdown.Shot) ([]map[string]interface{}, error) {
	times := map[int]bool{}
	for _, at := range breakdown.SampleTimes(durationMS) {
		times[at] = true
	}
	for _, shot := range shots {
		times[breakdown.KeyframeMS(shot.StartMS, shot.EndMS)] = true
	}
	ordered := make([]int, 0, len(times))
	for at := range times {
		ordered = append(ordered, at)
	}
	sortInts(ordered)
	kept := make([]map[string]interface{}, 0, len(ordered))
	var last int
	for index, at := range ordered {
		if index > 0 && at-last < breakdown.MinShotMS {
			continue
		}
		name := filepath.Join(dir, fmt.Sprintf("frame-%04d.jpg", len(kept)+1))
		if err := runFFmpeg(ctx, "-y", "-ss", fmt.Sprintf("%.3f", float64(at)/1000), "-i", video, "-frames:v", "1", "-vf", "scale=768:-2", "-q:v", "5", name); err != nil {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil || len(body) < 6000 {
			continue
		}
		objectName := fmt.Sprintf("works/breakdown/%s/frame-%04d.jpg", publicID, len(kept)+1)
		imageURL, err := objectStore.Upload(ctx, objectName, "image/jpeg", bytes.NewReader(body), int64(len(body)))
		if err != nil {
			return kept, fmt.Errorf("关键帧上传失败")
		}
		kept = append(kept, map[string]interface{}{"t_ms": at, "url": imageURL, "label": formatBreakdownClock(at)})
		last = at
		if len(kept) >= breakdown.MaxSampleFrames {
			break
		}
	}
	if len(kept) == 0 {
		return nil, fmt.Errorf("没有抽出可用关键帧")
	}
	return kept, nil
}

func sortInts(values []int) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func mustRead(path string) []byte {
	body, _ := os.ReadFile(path)
	return body
}

func describeBreakdownFrames(ctx context.Context, progressMu *sync.Mutex, pool *pgxpool.Pool, projectID int64, baseURL, token, publicID, modelCode string, frames []map[string]interface{}, outputs map[string]interface{}, stages map[string]string) []map[string]interface{} {
	const batchSize = 4
	described := make([]map[string]interface{}, len(frames))
	missing := make([]int, 0, len(frames))
	for index, frame := range frames {
		copyFrame := map[string]interface{}{}
		for key, value := range frame {
			copyFrame[key] = value
		}
		described[index] = copyFrame
		if !breakdownFrameDescriptionComplete(copyFrame) {
			missing = append(missing, index)
		}
	}
	for start := 0; start < len(missing); start += batchSize {
		end := start + batchSize
		if end > len(missing) {
			end = len(missing)
		}
		indices := missing[start:end]
		urls := make([]string, 0, len(indices))
		lines := make([]string, 0, len(indices))
		for localIndex, frameIndex := range indices {
			frame := described[frameIndex]
			urls = append(urls, stringAny(frame["url"]))
			lines = append(lines, fmt.Sprintf("%d. 时间 %s", localIndex, stringAny(frame["label"])))
		}
		percent := 40 + end*35/maxInt(len(missing), 1)
		if percent > 78 {
			percent = 78
		}
		if progressMu != nil {
			progressMu.Lock()
		}
		markBreakdown(ctx, pool, projectID, outputs, "frames", fmt.Sprintf("正在补全 %d/%d 个关键帧...", end, len(missing)), percent, stages)
		if progressMu != nil {
			progressMu.Unlock()
		}
		var parsed map[string]interface{}
		for attempt := 0; attempt < 2; attempt++ {
			text, err := breakdownLLM(ctx, pool, baseURL, token, fmt.Sprintf("%s_f%d_%d", publicID, start, attempt+1), modelCode, 4*time.Minute, breakdownFrameSystem, strings.Join(lines, "\n"), urls, nil)
			if err != nil {
				continue
			}
			parsed, err = breakdown.ParseJSONObject(text)
			if err == nil {
				rows := breakdownMapSlice(parsed["frames"])
				complete := len(rows) == len(indices)
				for _, row := range rows {
					complete = complete && breakdownFrameDescriptionComplete(row)
				}
				if complete {
					break
				}
			}
		}
		rows := breakdownMapSlice(parsed["frames"])
		for localIndex, frameIndex := range indices {
			if localIndex < len(rows) {
				for _, key := range []string{"scene", "action", "camera", "on_screen_text"} {
					described[frameIndex][key] = strings.TrimSpace(stringAny(rows[localIndex][key]))
				}
			}
		}
		if progressMu != nil {
			progressMu.Lock()
		}
		outputs["keyframes"] = described
		saveWorkflowOutputs(ctx, pool, projectID, outputs)
		if progressMu != nil {
			progressMu.Unlock()
		}
	}
	return described
}

func formatBreakdownClock(ms int) string {
	if ms < 0 {
		ms = 0
	}
	total := ms / 1000
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const breakdownVideoSystem = `你是短视频画面拆解编辑。你看到的是全片中的一段，不是独立短片。只写这一段正文时间内看得见的内容，不凭画面猜测台词，不补写没出现的剧情。时间一律用全片绝对毫秒，从 0 开始，不要改成片段内的相对时间。片段前后多出来的一两秒只为看清边界，不要把正文时间之外的镜头写进 shots。用 JSON 返回：{"style":"这一段的色调、光线、服装","scenes":["地点、内外景、昼夜和光线"],"characters":[{"name":"稳定称呼，不确定则写人物1","appearance":"年龄感、脸、发型、服装颜色和识别特征"}],"shots":[{"start_ms":0,"end_ms":1000,"scene":"","characters":["人物名"],"action":"一个可见主要动作","camera":"景别和运动"}]}。同一人物使用同一套外观描述，并沿用给出的字幕称呼。`

const breakdownTranscriptSystem = `你是音频逐句转写员。只听提供视频中的声音，不根据画面、常识或剧情补写。忽略背景音乐和纯音效；对白、旁白、画外音逐句记录原话。时间必须换算成全片绝对毫秒，并且只写正文时间范围内的语句。说话人无法确定时写“未知角色”，不要猜姓名。用 JSON 返回：{"cues":[{"start_ms":0,"end_ms":1000,"speaker":"","emotion":"平静/激动/哭泣等，无法判断则空","text":"原话","confidence":0.0}]}。听不清的字用[听不清]，不要改写成通顺句子。`

const breakdownScriptSystem = `你是短视频剧本与视频生产分镜编辑。只使用提供的切点、音频逐句转写、烧录字幕、分段画面理解和关键帧描述。证据按全片绝对时间给出；按时间合并，不要重复重叠边界镜头，不得补写证据中不存在的剧情。音频原话优先，烧录字幕用于校对；不一致时写入 conflicts，不要擅自合成正确台词。同一人物跨镜使用同一名称和外观。按 3–8 秒拆分，每个 unit 只含一个主要动作和一个可独立生成的镜头。用 JSON 返回：{"style":"","scenes":[],"characters":[{"code":"CHAR_01","name":"","appearance":""}],"units":[{"start_sec":0,"end_sec":6,"scene":"地点/内外景/昼夜","shots":[{"camera":"景别和运镜","characters":["人物名"],"action":"一个连续动作及明确终态","speaker":"无对白则空","emotion":"","dialogue":"无对白时写无对白","continuity":"承接上一镜的站位、持物、服装和方向","evidence_start_ms":0,"evidence_end_ms":6000,"keyframe_prompt":"单一时刻的竖屏关键帧提示词，不生成字幕文字","video_prompt":"从关键帧开始的动作、表演、运镜、终态和环境运动，不生成额外镜头"}]}],"conflicts":[]}。必须覆盖给定时间范围，不要把多个切镜塞进同一个 action。`

const breakdownScriptRetrySystem = breakdownScriptSystem + `这是超时后的紧凑重试。必须覆盖完整时间线，每个 unit 只写一个 shot；字段只保留事实短句，不写分析过程、解释、Markdown 或重复描述。style、scene、appearance、camera、action 每项尽量不超过 40 个汉字。`

const breakdownFrameSystem = `你是画面核对员。按给出的顺序描述每一帧，只写看得见的内容。用 JSON 返回：{"frames":[{"scene":"","action":"这一帧里正在发生的变化","camera":"景别","on_screen_text":"画面上的字幕原文，没有则空字符串"}]}。数组长度必须和帧数一致。`

const (
	breakdownSegmentConcurrency = 3
	breakdownMaxHalveDepth      = 2
)

type breakdownSegmentTask struct {
	root    int // 所属根片段的编号，重试拆出的片段沿用
	depth   int // 对半拆分层级，0 表示计划阶段的根片段
	segment breakdown.Segment
}

type breakdownSegmentOutcome struct {
	task   breakdownSegmentTask
	parsed map[string]interface{}
	err    error
}

type breakdownSegmentRunner func(task breakdownSegmentTask) (map[string]interface{}, error)

// runBreakdownSegments 并发理解所有片段；超时或超大的片段逐轮对半拆小重试，
// 最多拆 breakdownMaxHalveDepth 层。返回按时间排序的成功片段和给用户的备注。
func runBreakdownSegments(segments []breakdown.Segment, durationMS int, progress func(round, done, total int), run breakdownSegmentRunner) ([]map[string]interface{}, []string) {
	sem := make(chan struct{}, breakdownSegmentConcurrency)
	runRound := func(tasks []breakdownSegmentTask, round int) []breakdownSegmentOutcome {
		results := make([]breakdownSegmentOutcome, len(tasks))
		var wg sync.WaitGroup
		var doneMu sync.Mutex
		done := 0
		for index, task := range tasks {
			wg.Add(1)
			go func(index int, task breakdownSegmentTask) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				parsed, err := run(task)
				results[index] = breakdownSegmentOutcome{task: task, parsed: parsed, err: err}
				if progress != nil {
					doneMu.Lock()
					done++
					progress(round, done, len(tasks))
					doneMu.Unlock()
				}
			}(index, task)
		}
		wg.Wait()
		return results
	}
	tasks := make([]breakdownSegmentTask, 0, len(segments))
	for _, segment := range segments {
		tasks = append(tasks, breakdownSegmentTask{root: segment.Index, segment: segment})
	}
	nextIndex := len(segments)
	var outcomes []breakdownSegmentOutcome
	splitRoots := map[int]bool{}
	terminalFailures := map[int]bool{}
	for round := 0; len(tasks) > 0; round++ {
		results := runRound(tasks, round)
		outcomes = append(outcomes, results...)
		var next []breakdownSegmentTask
		for _, outcome := range results {
			if outcome.err == nil && outcome.parsed != nil {
				continue
			}
			if outcome.task.depth < breakdownMaxHalveDepth && breakdownShouldHalve(outcome.err) {
				nextIndex++
				leftIndex := nextIndex
				nextIndex++
				rightIndex := nextIndex
				left, right, ok := breakdown.SplitSegment(outcome.task.segment, durationMS, leftIndex, rightIndex)
				if ok {
					splitRoots[outcome.task.root] = true
					next = append(next,
						breakdownSegmentTask{root: outcome.task.root, depth: outcome.task.depth + 1, segment: left},
						breakdownSegmentTask{root: outcome.task.root, depth: outcome.task.depth + 1, segment: right})
					continue
				}
			}
			terminalFailures[outcome.task.root] = true
		}
		tasks = next
	}
	parts := make([]map[string]interface{}, 0, len(outcomes))
	covered := map[int]bool{}
	for _, outcome := range outcomes {
		if outcome.err != nil || outcome.parsed == nil {
			continue
		}
		covered[outcome.task.root] = true
		outcome.parsed["start_ms"] = outcome.task.segment.StartMS
		outcome.parsed["end_ms"] = outcome.task.segment.EndMS
		parts = append(parts, outcome.parsed)
	}
	sortBreakdownParts(parts)
	var notes []string
	uncovered := 0
	for _, segment := range segments {
		if !covered[segment.Index] || terminalFailures[segment.Index] {
			uncovered++
		}
	}
	if uncovered > 0 {
		notes = append(notes, fmt.Sprintf("%d 段视频理解未完成，已用其余片段和关键帧合并。", uncovered))
	}
	recovered := 0
	for root := range splitRoots {
		if covered[root] && !terminalFailures[root] {
			recovered++
		}
	}
	if recovered > 0 {
		notes = append(notes, fmt.Sprintf("%d 段超时片段拆成小片段重试后补全。", recovered))
	}
	return parts, notes
}

func understandBreakdownSegments(ctx context.Context, progressMu *sync.Mutex, pool *pgxpool.Pool, projectID int64, baseURL, token, publicID, modelCode, source, dir string, durationMS int, hasAudio bool, shots []breakdown.Shot, cues []breakdown.Cue, segments []breakdown.Segment, outputs map[string]interface{}, stages map[string]string) ([]map[string]interface{}, []string) {
	if len(segments) == 0 {
		return nil, []string{"没有可理解的视频片段。"}
	}
	hints := breakdownSubtitleHints(cues)
	progress := func(round, done, total int) {
		if progressMu != nil {
			progressMu.Lock()
			defer progressMu.Unlock()
		}
		label := "正在理解视频片段"
		percent := 30 + done*20/maxInt(total, 1)
		if round > 0 {
			label = "正在重试超时片段"
			percent = 55
		}
		markBreakdown(ctx, pool, projectID, outputs, "understand", fmt.Sprintf("%s %d/%d", label, done, total), percent, stages)
	}
	run := func(task breakdownSegmentTask) (map[string]interface{}, error) {
		if progressMu != nil {
			progressMu.Lock()
		}
		cached, ok := cachedBreakdownSegment(outputs, task.segment, modelCode)
		if progressMu != nil {
			progressMu.Unlock()
		}
		if ok {
			return cached, nil
		}
		parsed, err := understandOneBreakdownSegment(ctx, pool, baseURL, token, publicID, modelCode, source, dir, durationMS, hasAudio, shots, cues, hints, task.segment)
		saveBreakdownSegmentCheckpoint(ctx, progressMu, pool, projectID, outputs, modelCode, task, parsed, err)
		return parsed, err
	}
	return runBreakdownSegments(segments, durationMS, progress, run)
}

func understandOneBreakdownSegment(ctx context.Context, pool *pgxpool.Pool, baseURL, token, publicID, modelCode, source, dir string, durationMS int, hasAudio bool, shots []breakdown.Shot, cues []breakdown.Cue, hints string, segment breakdown.Segment) (map[string]interface{}, error) {
	dest := filepath.Join(dir, fmt.Sprintf("segment-%02d.mp4", segment.Index))
	if err := cutBreakdownClip(ctx, source, dest, segment.ClipStartMS, segment.ClipEndMS); err != nil {
		return nil, err
	}
	info, err := os.Stat(dest)
	if err != nil || info.Size() == 0 || info.Size() > breakdownProxyBytes {
		return nil, fmt.Errorf("片段超过模型可读大小")
	}
	clipURL, err := objectStore.Upload(ctx, fmt.Sprintf("works/breakdown/%s/segment-%02d.mp4", publicID, segment.Index), "video/mp4", bytes.NewReader(mustRead(dest)), info.Size())
	if err != nil {
		return nil, err
	}
	clipMS := segment.ClipEndMS - segment.ClipStartMS
	if clipMS < 1 {
		clipMS = segment.EndMS - segment.StartMS
	}
	user := breakdownSegmentUser(durationMS, shots, cues, hints, segment)
	transcript := map[string]interface{}{"cues": []interface{}{}}
	if hasAudio {
		transcriptText, transcriptErr := breakdownLLM(ctx, pool, baseURL, token, fmt.Sprintf("%s_a%d", publicID, segment.Index), modelCode, breakdownSegmentTimeout(clipMS), breakdownTranscriptSystem, user, nil, []string{clipURL})
		if transcriptErr != nil {
			return nil, fmt.Errorf("片段音频转写失败：%w", transcriptErr)
		}
		transcript, transcriptErr = breakdown.ParseJSONObject(transcriptText)
		if transcriptErr != nil {
			return nil, fmt.Errorf("片段音频转写不可解析：%w", transcriptErr)
		}
	}
	text, err := breakdownLLM(ctx, pool, baseURL, token, fmt.Sprintf("%s_v%d", publicID, segment.Index), modelCode, breakdownSegmentTimeout(clipMS), breakdownVideoSystem, user, nil, []string{clipURL})
	if err != nil {
		return nil, err
	}
	parsed, err := breakdown.ParseJSONObject(text)
	if err != nil {
		return nil, err
	}
	parsed["transcript"] = transcript
	return parsed, nil
}

func breakdownCallTimeout(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "deadline exceeded") || strings.Contains(message, "timeout")
}

// 慢请求值得把片段对半拆小后重试：读取超时和超出模型可读大小都属于这类。
func breakdownShouldHalve(err error) bool {
	if err == nil {
		return false
	}
	return breakdownCallTimeout(err) || strings.Contains(err.Error(), "超过模型可读大小")
}

// 视频理解的耗时随片段长度增长：基础 4 分钟，每秒片段加 8 秒，上限 12 分钟。
func breakdownSegmentTimeout(clipMS int) time.Duration {
	seconds := clipMS / 1000
	if seconds < 1 {
		seconds = 1
	}
	timeout := 4*time.Minute + time.Duration(seconds)*8*time.Second
	if timeout > 12*time.Minute {
		timeout = 12 * time.Minute
	}
	return timeout
}

// 剧本整合要读完全部证据再输出整份 JSON：基础 5 分钟，每个视频片段加 15 秒，上限 12 分钟。
func breakdownScriptTimeout(videoParts int) time.Duration {
	timeout := 5*time.Minute + time.Duration(videoParts)*15*time.Second
	if timeout > 12*time.Minute {
		timeout = 12 * time.Minute
	}
	return timeout
}

func sortBreakdownParts(parts []map[string]interface{}) {
	for i := 1; i < len(parts); i++ {
		for j := i; j > 0 && intAny(parts[j]["start_ms"]) < intAny(parts[j-1]["start_ms"]); j-- {
			parts[j], parts[j-1] = parts[j-1], parts[j]
		}
	}
}

func cutBreakdownClip(ctx context.Context, source, dest string, startMS, endMS int) error {
	if endMS <= startMS {
		endMS = startMS + 1000
	}
	start := fmt.Sprintf("%.3f", float64(startMS)/1000)
	length := fmt.Sprintf("%.3f", float64(endMS-startMS)/1000)
	err := runFFmpeg(ctx, "-y", "-ss", start, "-i", source, "-t", length, "-vf", "scale=640:-2", "-c:v", "libx264", "-preset", "veryfast", "-crf", "32", "-c:a", "aac", "-b:a", "64k", "-movflags", "+faststart", dest)
	if err == nil {
		return nil
	}
	return runFFmpeg(ctx, "-y", "-ss", start, "-i", source, "-t", length, "-vf", "scale=640:-2", "-c:v", "libx264", "-preset", "veryfast", "-crf", "34", "-an", "-movflags", "+faststart", dest)
}

func breakdownSegmentUser(durationMS int, shots []breakdown.Shot, cues []breakdown.Cue, hints string, segment breakdown.Segment) string {
	var selected []breakdown.Shot
	for _, shot := range shots {
		if shot.EndMS <= segment.StartMS || shot.StartMS >= segment.EndMS {
			continue
		}
		selected = append(selected, shot)
	}
	var lines []string
	for _, cue := range cues {
		if cue.EndMS < segment.StartMS || cue.StartMS > segment.EndMS {
			continue
		}
		text := strings.TrimSpace(cue.Text)
		if text == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s %s", formatBreakdownClock(cue.StartMS), text))
	}
	return fmt.Sprintf("全片时长 %d 毫秒。本段正文是 %d–%d 毫秒。片段文件从 %d 毫秒放到 %d 毫秒，多出来的部分只供看清边界。\n本段切点：%s\n本段字幕：%s\n全片字幕线索，人物称呼请沿用，不要另起名字：\n%s",
		durationMS, segment.StartMS, segment.EndMS, segment.ClipStartMS, segment.ClipEndMS, string(mustJSON(selected)), strings.Join(lines, "；"), hints)
}

func breakdownSubtitleHints(cues []breakdown.Cue) string {
	var lines []string
	for _, cue := range cues {
		text := strings.TrimSpace(cue.Text)
		if text == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s %s", formatBreakdownClock(cue.StartMS), text))
		if len(lines) >= 24 {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func breakdownScriptUser(evidence map[string]interface{}, notes []string) string {
	return "证据如下。备注：" + strings.Join(notes, "；") + "\n" + string(mustJSON(evidence))
}

func compactBreakdownEvidence(evidence map[string]interface{}) map[string]interface{} {
	frames := make([]map[string]interface{}, 0)
	switch items := evidence["frames"].(type) {
	case []map[string]interface{}:
		step := 1
		if len(items) > 36 {
			step = (len(items) + 35) / 36
		}
		for index, frame := range items {
			if index%step != 0 {
				continue
			}
			frames = append(frames, map[string]interface{}{
				"t_ms": frame["t_ms"], "scene": frame["scene"], "action": frame["action"], "camera": frame["camera"], "on_screen_text": frame["on_screen_text"],
			})
		}
	}
	return map[string]interface{}{"shots": evidence["shots"], "cues": evidence["cues"], "frames": frames, "video": evidence["video"]}
}

func breakdownFrameFacts(value interface{}) []breakdown.FrameFact {
	items, ok := value.([]map[string]interface{})
	if !ok {
		return nil
	}
	facts := make([]breakdown.FrameFact, 0, len(items))
	for _, frame := range items {
		facts = append(facts, breakdown.FrameFact{
			TMS: intAny(frame["t_ms"]), Scene: stringAny(frame["scene"]), Action: stringAny(frame["action"]), Camera: stringAny(frame["camera"]), Text: stringAny(frame["on_screen_text"]),
		})
	}
	return facts
}

type breakdownTimeoutKey struct{}

func breakdownTimeoutFloor(ctx context.Context) time.Duration {
	floor, _ := ctx.Value(breakdownTimeoutKey{}).(time.Duration)
	return floor
}

func breakdownLLM(ctx context.Context, pool *pgxpool.Pool, baseURL, token, requestID, modelCode string, timeout time.Duration, system, user string, images, videos []string) (string, error) {
	model, message := loadAgentAnalysisModel(ctx, pool, modelCode)
	if message != "" {
		return "", fmt.Errorf("%s", message)
	}
	if timeout <= 0 {
		timeout = 4 * time.Minute
	}
	ctx = context.WithValue(ctx, breakdownTimeoutKey{}, timeout)
	result, err := executeWorkerLLMWithMedia(ctx, pool, baseURL, token, requestID, model, system, user, 0.2, timeout, images, videos)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(extractLLMText(result.ResponseBody))
	if text == "" {
		return "", fmt.Errorf("模型没有返回内容")
	}
	return text, nil
}

func downloadBreakdownVideo(ctx context.Context, rawURL, dest string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return "", fmt.Errorf("请粘贴抖音分享文本、视频链接，或上传本地视频")
	}
	client := breakdownHTTPClient()
	resp, err := breakdownGET(ctx, client, parsed.String(), "")
	if err != nil {
		return "", fmt.Errorf("视频下载失败，请改为本地上传")
	}
	finalURL := resp.Request.URL
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if strings.HasPrefix(contentType, "video/") || contentType == "application/octet-stream" {
		err = saveBreakdownBody(resp, dest)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		return dest, nil
	}
	resp.Body.Close()
	if !breakdownDouyinHost(parsed.Hostname()) && (finalURL == nil || !breakdownDouyinHost(finalURL.Hostname())) {
		if path, ytdlpErr := downloadBreakdownWithYtDlp(ctx, rawURL, dest); ytdlpErr == nil {
			return path, nil
		}
		return "", fmt.Errorf("这个链接没有直接返回视频文件，请改为本地上传")
	}
	if breakdownAwemeID(finalURL) == "" && breakdownAwemeID(parsed) == "" {
		if path, ytdlpErr := downloadBreakdownWithYtDlp(ctx, rawURL, dest); ytdlpErr == nil {
			return path, nil
		}
		return "", fmt.Errorf("未能识别抖音作品，请确认分享链接完整且公开，或改为本地上传")
	}
	playURL, resolveErr := resolveDouyinPlayURL(ctx, rawURL)
	if resolveErr != nil || playURL == "" {
		if path, ytdlpErr := downloadBreakdownWithYtDlp(ctx, rawURL, dest); ytdlpErr == nil {
			return path, nil
		}
		if resolveErr != nil {
			return "", resolveErr
		}
		return "", fmt.Errorf("未能解析抖音视频，请确认作品公开且可播放，或改为本地上传")
	}
	videoResp, err := breakdownGET(ctx, client, playURL, finalURL.String())
	if err != nil {
		return "", fmt.Errorf("抖音视频下载失败，请改为本地上传")
	}
	defer videoResp.Body.Close()
	if err = saveBreakdownBody(videoResp, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func resolveDouyinPlayURL(ctx context.Context, shareURL string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("DOUYIN_API_BASE")), "/")
	if base == "" {
		base = "https://api.douyin.wtf"
	}
	client := newBreakdownHTTPClient(45 * time.Second)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", fmt.Errorf("抖音解析服务不可用，请改为本地上传")
	}
	client.Jar = jar
	headers := map[string]string{}
	if key := strings.TrimSpace(os.Getenv("DOUYIN_API_KEY")); key != "" {
		headers["X-API-Key"] = key
	} else if err = douyinDemoLogin(ctx, client, base); err != nil {
		return "", err
	}
	body, _ := json.Marshal(map[string]string{"url": shareURL})
	status, payload, err := breakdownJSON(ctx, client, http.MethodPost, base+"/api/v1/parse", body, headers)
	if err != nil {
		return "", fmt.Errorf("抖音解析服务不可用，请改为本地上传")
	}
	if status == http.StatusAccepted || status == http.StatusOK {
		if link := breakdown.MediaVideoURL(payload); link != "" {
			return link, nil
		}
		taskID := nestedString(payload, "data", "task_id")
		if taskID != "" {
			return pollDouyinTask(ctx, client, base, taskID, headers)
		}
	}
	return "", fmt.Errorf("抖音解析服务没有返回视频地址，请改为本地上传")
}

func douyinDemoLogin(ctx context.Context, client *http.Client, base string) error {
	_, demo, err := breakdownJSON(ctx, client, http.MethodGet, base+"/api/v1/auth/demo", nil, nil)
	if err != nil {
		return fmt.Errorf("抖音解析服务不可用，请改为本地上传")
	}
	data, _ := demo["data"].(map[string]interface{})
	username := strings.TrimSpace(stringAny(data["username"]))
	password := strings.TrimSpace(stringAny(data["password"]))
	if username == "" || password == "" {
		return fmt.Errorf("抖音解析服务需要 API Key，请配置 DOUYIN_API_KEY，或改为本地上传")
	}
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	status, _, err := breakdownJSON(ctx, client, http.MethodPost, base+"/api/v1/auth/login", body, nil)
	if err != nil || status >= 300 {
		return fmt.Errorf("抖音解析服务登录失败，请配置 DOUYIN_API_KEY，或改为本地上传")
	}
	return nil
}

func pollDouyinTask(ctx context.Context, client *http.Client, base, taskID string, headers map[string]string) (string, error) {
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		status, payload, err := breakdownJSON(ctx, client, http.MethodGet, base+"/api/v1/tasks/"+url.PathEscape(taskID), nil, headers)
		if err != nil || status >= 300 {
			return "", fmt.Errorf("抖音解析服务没有返回视频地址，请改为本地上传")
		}
		if link := breakdown.MediaVideoURL(payload); link != "" {
			return link, nil
		}
		state := nestedString(payload, "data", "state")
		if state == "failed" || state == "error" {
			return "", fmt.Errorf("抖音解析服务没有返回视频地址，请改为本地上传")
		}
		if state == "done" || state == "succeeded" {
			return "", fmt.Errorf("抖音解析服务没有返回视频地址，请改为本地上传")
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return "", fmt.Errorf("抖音解析超时，请改为本地上传")
}

func nestedString(value map[string]interface{}, keys ...string) string {
	var current interface{} = value
	for _, key := range keys {
		item, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = item[key]
	}
	return strings.TrimSpace(stringAny(current))
}

func breakdownJSON(ctx context.Context, client *http.Client, method, rawURL string, body []byte, headers map[string]string) (int, map[string]interface{}, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	var value map[string]interface{}
	if json.Unmarshal(payload, &value) != nil {
		return resp.StatusCode, nil, fmt.Errorf("解析结果不是 JSON")
	}
	return resp.StatusCode, value, nil
}

func saveBreakdownBody(resp *http.Response, dest string) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("视频返回 HTTP %d，请改为本地上传", resp.StatusCode)
	}
	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("保存视频失败")
	}
	defer file.Close()
	size, err := io.Copy(file, io.LimitReader(resp.Body, breakdownMaxBytes+1))
	if err != nil || size == 0 || size > breakdownMaxBytes {
		return fmt.Errorf("视频文件不能超过 500MB")
	}
	return nil
}

func breakdownHTTPClient() *http.Client {
	return newBreakdownHTTPClient(8 * time.Minute)
}

func newBreakdownHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addresses) == 0 {
			return nil, fmt.Errorf("无法解析链接地址")
		}
		for _, item := range addresses {
			if breakdownBlockedIP(item.IP) {
				return nil, fmt.Errorf("禁止访问本机或内网地址")
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
	}
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("URL 重定向次数过多")
		}
		if req.URL.User != nil || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
			return fmt.Errorf("仅支持公开的 HTTP/HTTPS 链接")
		}
		return nil
	}}
}

func breakdownGET(ctx context.Context, client *http.Client, rawURL, referer string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "video/mp4,video/*;q=0.9,application/json;q=0.8,*/*;q=0.5")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return client.Do(req)
}

func breakdownBlockedIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64 {
		return true
	}
	return false
}

func breakdownDouyinHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "douyin.com" || strings.HasSuffix(host, ".douyin.com") || host == "iesdouyin.com" || strings.HasSuffix(host, ".iesdouyin.com")
}

func breakdownAwemeID(source *url.URL) string {
	if source == nil {
		return ""
	}
	for _, key := range []string{"modal_id", "aweme_id", "item_id"} {
		if value := source.Query().Get(key); breakdownAllDigits(value) && len(value) >= 15 {
			return value
		}
	}
	parts := strings.FieldsFunc(source.Path, func(r rune) bool { return r == '/' || r == '-' || r == '_' })
	for index := len(parts) - 1; index >= 0; index-- {
		if breakdownAllDigits(parts[index]) && len(parts[index]) >= 15 {
			return parts[index]
		}
	}
	return ""
}

func breakdownAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func findBreakdownURL(value interface{}, wanted string) string {
	switch item := value.(type) {
	case map[string]interface{}:
		for key, nested := range item {
			if strings.EqualFold(key, wanted) {
				if candidate := firstBreakdownHTTP(nested); candidate != "" {
					return candidate
				}
			}
		}
		for _, nested := range item {
			if candidate := findBreakdownURL(nested, wanted); candidate != "" {
				return candidate
			}
		}
	case []interface{}:
		for _, nested := range item {
			if candidate := findBreakdownURL(nested, wanted); candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func firstBreakdownHTTP(value interface{}) string {
	switch item := value.(type) {
	case string:
		for _, candidate := range append([]string{item}, strings.Fields(item)...) {
			parsed, err := url.Parse(strings.Trim(candidate, `"'`))
			if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil {
				return parsed.String()
			}
		}
	case []interface{}:
		for _, nested := range item {
			if candidate := firstBreakdownHTTP(nested); candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func downloadBreakdownWithYtDlp(ctx context.Context, rawURL, dest string) (string, error) {
	command := strings.TrimSpace(os.Getenv("YTDLP_PATH"))
	args := []string{}
	if command == "" {
		if discovered, err := exec.LookPath("yt-dlp"); err == nil {
			command = discovered
		}
	}
	if command == "" {
		return "", fmt.Errorf("yt-dlp 未安装")
	}
	args = append(args, "--no-playlist", "--no-warnings", "--no-progress", "--socket-timeout", "20", "--max-filesize", "500M", "--format", "best[ext=mp4]/best", "--merge-output-format", "mp4", "--output", dest, "--", rawURL)
	runCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	output, err := exec.CommandContext(runCtx, command, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yt-dlp: %s", strings.TrimSpace(string(output)))
	}
	info, statErr := os.Stat(dest)
	if statErr != nil || info.Size() == 0 || info.Size() > breakdownMaxBytes {
		return "", fmt.Errorf("视频下载失败")
	}
	return dest, nil
}
