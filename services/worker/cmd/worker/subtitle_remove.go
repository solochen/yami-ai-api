package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type subtitleProbe struct {
	Duration    float64
	HasSubtitle bool
	VideoCodec  string
	AudioCodec  string
	Width       int
	Height      int
}

type subtitleDecisionInput struct {
	Mode      string
	HadTrack  bool
	SoftErr   error
	BurnedIn  bool
	DetectErr error
	HasModel  bool
}

type subtitleDecision struct {
	Action           string
	Message          string
	Prefix           string
	UseCleanedSource bool
	ResultMode       string
	Fallback         string
}

type subtitleUserError struct {
	msg    string
	prefix string
}

func (e subtitleUserError) Error() string { return e.msg }

type subtitleJob struct {
	Mode             string
	Region           string
	ProtectWatermark bool
	PreserveAudio    bool
	HasModel         bool
	SourcePath       string
	MaxDuration      float64
	SystemPrompt     string
	UserPrompt       string
}

type subtitleJobResult struct {
	OutputPath  string
	ContentType string
	Cleanup     func()
	Metadata    map[string]interface{}
	Prompt      string
	AIUsed      bool
}

type subtitleRepairFunc func(ctx context.Context, sourcePath, prompt string) (string, error)

func canonicalSubtitleMode(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto":
		return "auto", true
	case "soft_track":
		return "soft_track", true
	case "hardcoded_ai":
		return "hardcoded_ai", true
	default:
		return "", false
	}
}

func canonicalSubtitleRegion(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "bottom_25":
		return "bottom_25", true
	case "bottom_15":
		return "bottom_15", true
	case "bottom_35":
		return "bottom_35", true
	case "full", "full_frame":
		return "full", true
	default:
		return "", false
	}
}

func subtitleRegionFraction(region string) float64 {
	switch region {
	case "bottom_15":
		return 0.15
	case "bottom_35":
		return 0.35
	case "full":
		return 1
	default:
		return 0.25
	}
}

func subtitleBox(region string) map[string]float64 {
	height := subtitleRegionFraction(region)
	return map[string]float64{"x": 0, "y": 1 - height, "width": 1, "height": height}
}

func subtitleBandPixels(region string, width, height int) (y, h int) {
	width = subtitleEven(width)
	height = subtitleEven(height)
	if width < 2 {
		width = 2
	}
	if height < 2 {
		height = 2
	}
	fraction := subtitleRegionFraction(region)
	if fraction >= 0.999 {
		return 0, height
	}
	h = subtitleEven(int(math.Round(float64(height) * fraction)))
	if h < 2 {
		h = 2
	}
	if h > height {
		h = height
	}
	y = subtitleEven(height - h)
	if y+h > height {
		h = subtitleEven(height - y)
	}
	if h < 2 {
		y, h = 0, height
	}
	return y, h
}

func subtitleEven(value int) int {
	if value < 2 {
		return 2
	}
	if value%2 != 0 {
		return value - 1
	}
	return value
}

func buildSubtitleRemovalPrompt(systemPrompt, userPrompt, region string, protectWatermark, preserveAudio bool) string {
	regionLine := "Edit only the bottom 25% of every frame, where burned-in subtitles usually sit."
	switch region {
	case "bottom_15":
		regionLine = "Edit only the bottom 15% of every frame."
	case "bottom_35":
		regionLine = "Edit only the bottom 35% of every frame."
	case "full":
		regionLine = "Remove burned-in subtitle text anywhere in the full frame."
	}
	protectLine := "Keep every pixel outside that region unchanged, including logos, watermarks, faces, products and background."
	if region == "full" {
		protectLine = "Preserve logos, watermarks and brand marks. Remove subtitle glyphs without redesigning the scene."
	}
	if !protectWatermark && region != "full" {
		protectLine = "Watermark protection is disabled. Reconstruct outside the subtitle text only when that is required to remove the subtitles cleanly."
	}
	audioLine := "Preserve the original audio track exactly. Do not replace, mute or regenerate it."
	if !preserveAudio {
		audioLine = "Return a silent video. Do not keep, replace or regenerate the original audio."
	}
	base := strings.TrimSpace(systemPrompt)
	if base == "" {
		base = "Remove burned-in subtitles and reconstruct the covered background with consistent motion, lighting and texture. Do not add new subtitles, captions, letters or watermarks."
	}
	concrete := strings.Join([]string{
		"Hard-subtitle removal constraints:",
		regionLine,
		protectLine,
		audioLine,
		"Remove only burned-in subtitle glyphs. Avoid flicker, ghosting and warped faces.",
	}, "\n")
	return joinWorkflowInstruction(joinWorkflowInstruction(base, concrete), userPrompt)
}

func decideSubtitleRemoval(in subtitleDecisionInput) subtitleDecision {
	switch in.Mode {
	case "soft_track":
		if in.SoftErr != nil {
			return subtitleDecision{Action: "fail", Message: in.SoftErr.Error(), Prefix: "字幕轨移除失败："}
		}
		if !in.HadTrack {
			return subtitleDecision{Action: "fail", Message: "未检测到可移除的独立字幕轨", Prefix: "字幕轨移除失败："}
		}
		return subtitleDecision{Action: "soft", ResultMode: "soft_track"}
	case "hardcoded_ai":
		if !in.HasModel {
			return subtitleDecision{Action: "fail", Message: "硬字幕修复工作流尚未配置视频模型，请联系管理员"}
		}
		return subtitleDecision{Action: "ai", ResultMode: "hardcoded_ai"}
	default:
		if in.SoftErr != nil {
			if in.HasModel {
				return subtitleDecision{Action: "ai", ResultMode: "hardcoded_ai", Fallback: "soft_track_remux_failed"}
			}
			return subtitleDecision{Action: "fail", Message: in.SoftErr.Error(), Prefix: "字幕轨移除失败："}
		}
		if in.HadTrack && in.DetectErr != nil {
			if in.HasModel {
				return subtitleDecision{Action: "ai", UseCleanedSource: true, ResultMode: "soft_and_hard", Fallback: "burned_in_check_failed"}
			}
			return subtitleDecision{Action: "fail", Message: "已移除独立字幕轨，但无法确认画面是否仍有硬字幕，且未配置修复模型"}
		}
		if in.HadTrack && in.BurnedIn {
			if in.HasModel {
				return subtitleDecision{Action: "ai", UseCleanedSource: true, ResultMode: "soft_and_hard"}
			}
			return subtitleDecision{Action: "fail", Message: "检测到画面硬字幕，但后台未配置硬字幕 AI 修复模型"}
		}
		if in.HadTrack {
			return subtitleDecision{Action: "soft", ResultMode: "soft_track"}
		}
		if !in.HasModel {
			if in.BurnedIn {
				return subtitleDecision{Action: "fail", Message: "检测到画面硬字幕，但后台未配置硬字幕 AI 修复模型"}
			}
			return subtitleDecision{Action: "fail", Message: "未检测到可移除的独立字幕轨，且后台未配置硬字幕 AI 修复模型"}
		}
		return subtitleDecision{Action: "ai", ResultMode: "hardcoded_ai"}
	}
}

func executeSubtitleJob(ctx context.Context, job subtitleJob, repair subtitleRepairFunc) (subtitleJobResult, error) {
	mode, ok := canonicalSubtitleMode(job.Mode)
	if !ok {
		return subtitleJobResult{}, subtitleUserError{msg: "去字幕模式无效"}
	}
	region, ok := canonicalSubtitleRegion(job.Region)
	if !ok {
		return subtitleJobResult{}, subtitleUserError{msg: "字幕区域无效"}
	}
	job.Mode, job.Region = mode, region
	tmpDir, err := os.MkdirTemp("", "starai-subtitle-job-*")
	if err != nil {
		return subtitleJobResult{}, err
	}
	cleanup := func() { os.RemoveAll(tmpDir) }
	fail := func(err error) (subtitleJobResult, error) {
		cleanup()
		return subtitleJobResult{}, err
	}
	probe, err := probeSubtitleFile(ctx, job.SourcePath)
	if err != nil {
		return fail(subtitleUserError{msg: "检测视频失败：" + err.Error()})
	}
	if job.MaxDuration > 0 && probe.Duration > job.MaxDuration+0.05 {
		return fail(subtitleUserError{msg: fmt.Sprintf("视频时长不能超过 %d 秒", int(job.MaxDuration))})
	}
	cleanedPath := ""
	var hadTrack bool
	var softErr error
	if mode == "auto" || mode == "soft_track" {
		if probe.HasSubtitle {
			ext, _ := subtitleCopySpec(probe)
			cleanedPath = filepath.Join(tmpDir, "cleaned"+ext)
			if err := stripEmbeddedSubtitles(ctx, job.SourcePath, cleanedPath, job.PreserveAudio); err != nil {
				softErr = fmt.Errorf("字幕轨封装移除失败：%s", truncateText(err.Error(), 300))
				cleanedPath = ""
			} else {
				hadTrack = true
			}
		}
	}
	var burned bool
	var detectErr error
	if mode == "auto" {
		analysisPath := job.SourcePath
		if cleanedPath != "" {
			analysisPath = cleanedPath
		}
		burned, detectErr = detectBurnedInSubtitles(ctx, analysisPath, region)
	}
	decision := decideSubtitleRemoval(subtitleDecisionInput{
		Mode: mode, HadTrack: hadTrack, SoftErr: softErr, BurnedIn: burned, DetectErr: detectErr, HasModel: job.HasModel,
	})
	metadata := map[string]interface{}{
		"subtitle_mode":      decision.ResultMode,
		"subtitle_region":    region,
		"protect_watermark":  job.ProtectWatermark,
		"preserve_audio":     job.PreserveAudio,
		"had_subtitle_track": hadTrack,
		"burned_in_detected": burned,
	}
	if decision.Fallback != "" {
		metadata["subtitle_fallback"] = decision.Fallback
	}
	if decision.Action == "fail" {
		if decision.ResultMode == "" {
			metadata["subtitle_mode"] = mode
		}
		return fail(subtitleUserError{msg: decision.Message, prefix: decision.Prefix})
	}
	if decision.Action == "soft" {
		metadata["subtitle_mode"] = "soft_track"
		return subtitleJobResult{
			OutputPath: cleanedPath, ContentType: contentTypeForPath(cleanedPath), Cleanup: cleanup, Metadata: metadata,
		}, nil
	}
	if repair == nil {
		return fail(subtitleUserError{msg: "硬字幕修复工作流尚未配置视频模型，请联系管理员"})
	}
	sourcePath := job.SourcePath
	if decision.UseCleanedSource && cleanedPath != "" {
		sourcePath = cleanedPath
	}
	prompt := buildSubtitleRemovalPrompt(job.SystemPrompt, job.UserPrompt, region, job.ProtectWatermark, job.PreserveAudio)
	repairedPath, err := repair(ctx, sourcePath, prompt)
	if err != nil {
		prefix := "硬字幕 AI 修复失败："
		var userErr subtitleUserError
		if errors.As(err, &userErr) {
			if userErr.prefix != "" {
				prefix = userErr.prefix
			}
			err = userErr
		}
		return fail(subtitleUserError{msg: err.Error(), prefix: prefix})
	}
	baseProbe := probe
	if sourcePath != job.SourcePath {
		if probed, probeErr := probeSubtitleFile(ctx, sourcePath); probeErr == nil && probed.Width > 0 {
			baseProbe = probed
		}
	}
	outputPath := filepath.Join(tmpDir, "final.mp4")
	if err := constrainSubtitleOutput(ctx, sourcePath, repairedPath, outputPath, region, job.ProtectWatermark, job.PreserveAudio, baseProbe.Width, baseProbe.Height); err != nil {
		return fail(subtitleUserError{msg: "整理去字幕结果失败：" + truncateText(err.Error(), 300), prefix: "硬字幕 AI 修复失败："})
	}
	metadata["subtitle_mode"] = decision.ResultMode
	return subtitleJobResult{
		OutputPath: outputPath, ContentType: contentTypeForPath(outputPath), Cleanup: cleanup,
		Metadata: metadata, Prompt: prompt, AIUsed: true,
	}, nil
}

func runSubtitleRemovalWorkflow(ctx context.Context, pool *pgxpool.Pool, baseURL, token string, p WorkflowTaskPayload, publicID string, estimated float64, inputs, runtimeCfg map[string]interface{}) error {
	outputs := loadWorkflowOutputs(ctx, pool, p.ProjectID)
	if _, done := outputs["media_tasks"]; done && stringAny(outputs["current_step"]) == "result" {
		return completeSimpleAgentWorkflow(ctx, pool, p, publicID, estimated, outputs)
	}
	sourceVideo := firstNonEmpty(firstWorkerURL(inputs["video_url"]), firstWorkerURL(inputs["source_video_url"]), firstWorkerURL(inputs["reference_videos"]))
	if sourceVideo == "" {
		return failWorkflow(ctx, pool, p, publicID, estimated, "请先上传或从资产库选择源视频")
	}
	mode, modeOK := canonicalSubtitleMode(firstNonEmpty(stringAny(inputs["subtitle_mode"]), stringAny(runtimeCfg["default_subtitle_mode"]), "auto"))
	region, regionOK := canonicalSubtitleRegion(firstNonEmpty(stringAny(inputs["subtitle_region"]), stringAny(runtimeCfg["default_subtitle_region"]), "bottom_25"))
	nodeRunID := insertWorkflowNodeRun(ctx, pool, p.ProjectID, "remove_subtitle", "一键去字幕", "video", map[string]interface{}{
		"source_video_url": sourceVideo,
		"subtitle_mode":    mode,
		"subtitle_region":  region,
	}, 0)
	start := time.Now()
	fail := func(message, prefix string, metadata map[string]interface{}) error {
		if metadata == nil {
			metadata = map[string]interface{}{}
		}
		metadata["source_video_url"] = sourceVideo
		return finishVideoTransformWorkflow(ctx, pool, p, publicID, estimated, outputs, nodeRunID, start, nil, message, metadata, prefix)
	}
	if !modeOK {
		return fail("去字幕模式无效", "", nil)
	}
	if !regionOK {
		return fail("字幕区域无效", "", nil)
	}
	protect := boolDefault(inputs["protect_watermark"], boolDefault(runtimeCfg["protect_watermark"], true))
	preserve := boolDefault(inputs["preserve_audio"], boolDefault(runtimeCfg["preserve_audio"], true))
	maxDuration := float64(intAny(runtimeCfg["max_input_duration_sec"]))
	if maxDuration <= 0 {
		maxDuration = 300
	}
	maxBytes := subtitleMaxBytes(runtimeCfg)
	stagedPath, cleanupStage, err := stageSubtitleSource(ctx, sourceVideo, maxBytes)
	if err != nil {
		return fail(err.Error(), "", nil)
	}
	defer cleanupStage()

	var produced []map[string]interface{}
	var repairedDir string
	var modelCode string
	result, runErr := executeSubtitleJob(ctx, subtitleJob{
		Mode: mode, Region: region, ProtectWatermark: protect, PreserveAudio: preserve,
		HasModel:   strings.TrimSpace(stringAny(runtimeCfg["generation_model_code"])) != "",
		SourcePath: stagedPath, MaxDuration: maxDuration,
		SystemPrompt: stringAny(runtimeCfg["subtitle_remove_prompt"]),
		UserPrompt:   stringAny(inputs["prompt"]),
	}, func(ctx context.Context, sourcePath, prompt string) (string, error) {
		if objectStore == nil {
			return "", subtitleUserError{msg: "对象存储未配置，无法保存去字幕结果"}
		}
		modelCode = strings.TrimSpace(stringAny(runtimeCfg["generation_model_code"]))
		model, loadErr := loadEnabledWorkerMediaModel(ctx, pool, modelCode)
		if loadErr != nil {
			return "", subtitleUserError{msg: "生成模型不存在：" + modelCode, prefix: "硬字幕 AI 修复失败："}
		}
		videoURL := sourceVideo
		if sourcePath != stagedPath {
			data, readErr := os.ReadFile(sourcePath)
			if readErr != nil {
				return "", readErr
			}
			uploaded, uploadErr := objectStore.Upload(ctx, fmt.Sprintf("works/subtitle-remove/%s/cleaned_%d%s", publicID, time.Now().UnixNano(), filepath.Ext(sourcePath)), contentTypeForPath(sourcePath), bytes.NewReader(data), int64(len(data)))
			if uploadErr != nil {
				return "", subtitleUserError{msg: "上传去字幕中间结果失败：" + uploadErr.Error(), prefix: "硬字幕 AI 修复失败："}
			}
			videoURL = uploaded
		}
		taskInputs := map[string]interface{}{
			"count": 1, "n": 1,
			"video_url": videoURL, "source_video_url": videoURL,
			"reference_videos":  []string{videoURL},
			"operation":         firstNonEmpty(stringAny(runtimeCfg["subtitle_remove_operation"]), "subtitle_remove"),
			"subtitle_region":   region,
			"subtitle_box":      subtitleBox(region),
			"protect_watermark": protect,
			"preserve_audio":    preserve,
		}
		amount := estimateModelCostByIDWorker(ctx, pool, model.ID, taskInputs, 0, 0, 0, 0)
		if reserveErr := ensureWorkflowReservation(ctx, pool, p.UserID, publicID, p.ProjectID, amount); reserveErr != nil {
			return "", subtitleUserError{msg: reserveErr.Error(), prefix: "硬字幕 AI 修复失败："}
		}
		videoRuntime := copyMap(runtimeCfg)
		videoRuntime["generation_type"] = "video"
		mediaTasks, errMsg := runAgentMediaTasks(ctx, pool, baseURL, token, p.ProjectID, p.UserID, publicID, videoRuntime, taskInputs, prompt)
		if errMsg != "" {
			return "", subtitleUserError{msg: errMsg, prefix: "硬字幕 AI 修复失败："}
		}
		produced = mediaTasks
		repairedURL := ""
		for index := len(mediaTasks) - 1; index >= 0; index-- {
			if stringAny(mediaTasks[index]["status"]) != "succeeded" {
				continue
			}
			if url := subtitleTaskVideoURL(mediaTasks[index]); url != "" {
				repairedURL = url
				break
			}
		}
		if repairedURL == "" {
			return "", subtitleUserError{msg: "硬字幕修复完成但未返回视频", prefix: "硬字幕 AI 修复失败："}
		}
		data, _, downloadErr := readComposeSource(ctx, repairedURL, maxBytes)
		if downloadErr != nil {
			return "", subtitleUserError{msg: "下载修复结果失败：" + downloadErr.Error(), prefix: "硬字幕 AI 修复失败："}
		}
		dir, dirErr := os.MkdirTemp("", "starai-subtitle-repair-*")
		if dirErr != nil {
			return "", dirErr
		}
		repairedDir = dir
		repairedPath := filepath.Join(dir, "repaired.bin")
		if writeErr := os.WriteFile(repairedPath, data, 0o600); writeErr != nil {
			return "", writeErr
		}
		return repairedPath, nil
	})
	if repairedDir != "" {
		defer os.RemoveAll(repairedDir)
	}
	if runErr != nil {
		prefix := ""
		message := runErr.Error()
		var userErr subtitleUserError
		if errors.As(runErr, &userErr) {
			prefix = userErr.prefix
			message = userErr.msg
		}
		return fail(message, prefix, map[string]interface{}{"subtitle_region": region})
	}
	defer result.Cleanup()
	data, err := os.ReadFile(result.OutputPath)
	if err != nil || len(data) == 0 {
		return fail("读取去字幕结果失败", "", result.Metadata)
	}
	if modelCode != "" && result.AIUsed {
		result.Metadata["model_code"] = modelCode
	}
	result.Metadata["source_video_url"] = sourceVideo
	tasks, err := deliverSubtitleResult(ctx, pool, p, publicID, produced, data, result.ContentType, result.Metadata)
	if err != nil {
		return fail(err.Error(), "", result.Metadata)
	}
	return finishVideoTransformWorkflow(ctx, pool, p, publicID, estimated, outputs, nodeRunID, start, tasks, "", result.Metadata, "")
}

func subtitleMaxBytes(runtimeCfg map[string]interface{}) int64 {
	mb := intAny(runtimeCfg["max_input_size_mb"])
	if mb <= 0 {
		mb = 500
	}
	if mb > 8192 {
		mb = 8192
	}
	return int64(mb) << 20
}

func stageSubtitleSource(ctx context.Context, mediaURL string, maxBytes int64) (string, func(), error) {
	if _, err := ffmpegBinaryPath(); err != nil {
		return "", nil, err
	}
	if objectStore == nil && strings.HasPrefix(strings.TrimSpace(mediaURL), "/") {
		return "", nil, errors.New("对象存储未配置，无法读取源视频")
	}
	data, _, err := readComposeSource(ctx, mediaURL, maxBytes)
	if err != nil {
		message := err.Error()
		if strings.Contains(message, "exceeds size limit") || strings.Contains(message, "超过大小限制") {
			return "", nil, fmt.Errorf("源视频不能超过 %d MB", maxBytes>>20)
		}
		if strings.Contains(message, "下载") {
			return "", nil, err
		}
		return "", nil, fmt.Errorf("下载源视频失败：%w", err)
	}
	if len(data) == 0 {
		return "", nil, errors.New("下载源视频失败：文件为空")
	}
	dir, err := os.MkdirTemp("", "starai-subtitle-source-*")
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, "source.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	return path, func() { os.RemoveAll(dir) }, nil
}

func ffprobeBinary() string {
	ffprobePath := "ffprobe"
	if ffmpegPath, err := ffmpegBinaryPath(); err == nil {
		candidate := filepath.Join(filepath.Dir(ffmpegPath), ffprobeExecutableName())
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate
		}
	}
	return ffprobePath
}

func probeSubtitleFile(ctx context.Context, path string) (subtitleProbe, error) {
	cmd := exec.CommandContext(ctx, ffprobeBinary(), "-v", "error", "-show_entries", "stream=codec_type,codec_name,width,height:format=duration", "-of", "json", path)
	output, err := cmd.Output()
	if err != nil {
		return subtitleProbe{}, err
	}
	var parsed struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(output, &parsed); err != nil {
		return subtitleProbe{}, err
	}
	probe := subtitleProbe{}
	probe.Duration, _ = strconv.ParseFloat(strings.TrimSpace(parsed.Format.Duration), 64)
	for _, stream := range parsed.Streams {
		switch stream.CodecType {
		case "subtitle":
			probe.HasSubtitle = true
		case "video":
			if probe.VideoCodec == "" {
				probe.VideoCodec = stream.CodecName
				probe.Width = stream.Width
				probe.Height = stream.Height
			}
		case "audio":
			if probe.AudioCodec == "" {
				probe.AudioCodec = stream.CodecName
			}
		}
	}
	return probe, nil
}

func subtitleCopySpec(probe subtitleProbe) (ext, contentType string) {
	video := strings.ToLower(probe.VideoCodec)
	audio := strings.ToLower(probe.AudioCodec)
	webmVideo := video == "vp8" || video == "vp9" || video == "av1"
	webmAudio := audio == "" || audio == "opus" || audio == "vorbis"
	if webmVideo && webmAudio {
		return ".webm", "video/webm"
	}
	mp4Video := video == "h264" || video == "avc" || video == "hevc" || video == "h265" || video == "mpeg4"
	mp4Audio := audio == "" || audio == "aac" || audio == "mp3" || audio == "ac3"
	if mp4Video && mp4Audio {
		return ".mp4", "video/mp4"
	}
	return ".mkv", "video/x-matroska"
}

func contentTypeForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	default:
		return "video/mp4"
	}
}

func stripEmbeddedSubtitles(ctx context.Context, inputPath, outputPath string, preserveAudio bool) error {
	args := []string{"-y", "-i", inputPath, "-map", "0:v:0"}
	if preserveAudio {
		args = append(args, "-map", "0:a?")
	}
	args = append(args, "-c", "copy", "-sn", "-dn")
	if strings.HasSuffix(strings.ToLower(outputPath), ".mp4") {
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, outputPath)
	return runFFmpeg(ctx, args...)
}

func constrainSubtitleOutput(ctx context.Context, originalPath, repairedPath, outputPath, region string, protect, preserveAudio bool, width, height int) error {
	if protect && region != "full" {
		return compositeSubtitleRegion(ctx, originalPath, repairedPath, outputPath, region, preserveAudio, width, height)
	}
	return remuxSubtitleDelivery(ctx, originalPath, repairedPath, outputPath, preserveAudio)
}

func compositeSubtitleRegion(ctx context.Context, originalPath, repairedPath, outputPath, region string, preserveAudio bool, width, height int) error {
	if width < 2 || height < 2 {
		probe, err := probeSubtitleFile(ctx, originalPath)
		if err != nil {
			return err
		}
		width, height = probe.Width, probe.Height
	}
	width, height = subtitleEven(width), subtitleEven(height)
	y, h := subtitleBandPixels(region, width, height)
	filter := fmt.Sprintf("[1:v]scale=%d:%d:flags=bilinear[edit];[0:v]scale=%d:%d:flags=bilinear[base];[edit]crop=%d:%d:0:%d[band];[base][band]overlay=0:%d:format=auto[v]", width, height, width, height, width, h, y, y)
	args := []string{"-y", "-i", originalPath, "-i", repairedPath, "-filter_complex", filter, "-map", "[v]"}
	if preserveAudio {
		args = append(args, "-map", "0:a?", "-c:a", "aac", "-shortest")
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "18", "-preset", "veryfast", "-movflags", "+faststart", outputPath)
	return runFFmpeg(ctx, args...)
}

func remuxSubtitleDelivery(ctx context.Context, originalPath, repairedPath, outputPath string, preserveAudio bool) error {
	args := []string{"-y", "-i", repairedPath}
	if preserveAudio {
		args = append(args, "-i", originalPath, "-map", "0:v:0", "-map", "1:a?", "-c:v", "copy", "-c:a", "aac", "-shortest")
	} else {
		args = append(args, "-map", "0:v:0", "-an", "-c:v", "copy")
	}
	if strings.HasSuffix(strings.ToLower(outputPath), ".mp4") {
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, outputPath)
	if err := runFFmpeg(ctx, args...); err == nil {
		return nil
	}
	fallback := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".mp4"
	reencode := []string{"-y", "-i", repairedPath}
	if preserveAudio {
		reencode = append(reencode, "-i", originalPath, "-map", "0:v:0", "-map", "1:a?", "-c:a", "aac", "-shortest")
	} else {
		reencode = append(reencode, "-map", "0:v:0", "-an")
	}
	reencode = append(reencode, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "18", "-preset", "veryfast", "-movflags", "+faststart", fallback)
	if err := runFFmpeg(ctx, reencode...); err != nil {
		return err
	}
	if fallback != outputPath {
		return os.Rename(fallback, outputPath)
	}
	return nil
}

func detectBurnedInSubtitles(ctx context.Context, path, region string) (bool, error) {
	probe, err := probeSubtitleFile(ctx, path)
	if err != nil {
		return false, err
	}
	if probe.Width < 16 || probe.Height < 16 {
		return false, errors.New("视频尺寸无效")
	}
	duration := probe.Duration
	if duration <= 0 {
		duration = 1
	}
	count := 3
	if duration < 0.4 {
		count = 1
	}
	y, h := subtitleBandPixels(region, probe.Width, probe.Height)
	dir, err := os.MkdirTemp("", "starai-subtitle-detect-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)
	positives := 0
	crop := fmt.Sprintf("crop=%d:%d:0:%d", subtitleEven(probe.Width), h, y)
	for index := 0; index < count; index++ {
		timestamp := duration * (float64(index) + 1) / (float64(count) + 1)
		if timestamp < 0 {
			timestamp = 0
		}
		frame := filepath.Join(dir, fmt.Sprintf("frame-%d.png", index))
		if err := runFFmpeg(ctx, "-y", "-i", path, "-ss", fmt.Sprintf("%.3f", timestamp), "-frames:v", "1", "-vf", crop, frame); err != nil {
			return false, err
		}
		if frameHasSubtitleBand(frame) {
			positives++
		}
	}
	need := 1
	if count >= 3 {
		need = 2
	}
	return positives >= need, nil
}

func frameHasSubtitleBand(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return false
	}
	bounds := img.Bounds()
	height := bounds.Dy()
	if height < 4 || bounds.Dx() < 8 {
		return false
	}
	run, best := 0, 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		if rowLooksLikeSubtitle(img, y, bounds.Min.X, bounds.Max.X) {
			run++
			if run > best {
				best = run
			}
			continue
		}
		run = 0
	}
	return best >= 3 && float64(best) <= float64(height)*0.8
}

func rowLooksLikeSubtitle(img image.Image, y, x0, x1 int) bool {
	count := x1 - x0
	if count < 8 {
		return false
	}
	sum, sumSq := 0.0, 0.0
	minL, maxL := 255, 0
	edges, previous := 0, -1
	for x := x0; x < x1; x++ {
		red, green, blue, _ := img.At(x, y).RGBA()
		luma := int((0.299*float64(red) + 0.587*float64(green) + 0.114*float64(blue)) / 257)
		if luma < minL {
			minL = luma
		}
		if luma > maxL {
			maxL = luma
		}
		sum += float64(luma)
		sumSq += float64(luma * luma)
		if previous >= 0 && absInt(luma-previous) > 40 {
			edges++
		}
		previous = luma
	}
	mean := sum / float64(count)
	variance := sumSq/float64(count) - mean*mean
	if variance < 0 {
		variance = 0
	}
	edgeRatio := float64(edges) / float64(count-1)
	return maxL-minL >= 120 && math.Sqrt(variance) >= 16 && edgeRatio >= 0.03 && edgeRatio <= 0.55
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func subtitleTaskVideoURL(task map[string]interface{}) string {
	output, _ := task["output"].(map[string]interface{})
	if output == nil {
		return ""
	}
	return firstNonEmpty(stringAny(output["video_url"]), stringAny(output["url"]), firstWorkerURL(output["videos"]))
}

func deliverSubtitleResult(ctx context.Context, pool *pgxpool.Pool, p WorkflowTaskPayload, publicID string, tasks []map[string]interface{}, data []byte, contentType string, metadata map[string]interface{}) ([]map[string]interface{}, error) {
	if objectStore == nil {
		return nil, errors.New("对象存储未配置，无法保存去字幕结果")
	}
	ext := ".mp4"
	switch contentType {
	case "video/webm":
		ext = ".webm"
	case "video/x-matroska":
		ext = ".mkv"
	}
	publicURL, err := objectStore.Upload(ctx, fmt.Sprintf("works/subtitle-remove/%s/result_%d%s", publicID, time.Now().UnixNano(), ext), contentType, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("上传去字幕结果失败：%w", err)
	}
	for index := len(tasks) - 1; index >= 0; index-- {
		taskNo := stringAny(tasks[index]["task_no"])
		if taskNo == "" || stringAny(tasks[index]["status"]) != "succeeded" {
			continue
		}
		output, err := replaceTaskVideoOutput(ctx, pool, taskNo, publicURL, metadata)
		if err != nil {
			return nil, err
		}
		tasks[index]["output"] = output
		appendWorkflowMediaTask(ctx, pool, p.ProjectID, tasks[index])
		return tasks, nil
	}
	item, err := persistSubtitleFileTask(ctx, pool, p, publicID, publicURL, contentType, metadata)
	if err != nil {
		return nil, err
	}
	return []map[string]interface{}{item}, nil
}

func replaceTaskVideoOutput(ctx context.Context, pool *pgxpool.Pool, taskNo, publicURL string, metadata map[string]interface{}) (map[string]interface{}, error) {
	item := loadAgentMediaTask(ctx, pool, taskNo)
	output, _ := item["output"].(map[string]interface{})
	if output == nil {
		output = map[string]interface{}{}
	}
	output["video_url"] = publicURL
	output["url"] = publicURL
	if videos, ok := output["videos"].([]interface{}); ok && len(videos) > 0 {
		if first, ok := videos[0].(map[string]interface{}); ok {
			first["url"] = publicURL
			videos[0] = first
			output["videos"] = videos
		}
	}
	for key, value := range metadata {
		output[key] = value
	}
	outputJSON, _ := json.Marshal(output)
	if _, err := pool.Exec(ctx, `UPDATE tasks SET output=$1, updated_at=now() WHERE task_no=$2`, outputJSON, taskNo); err != nil {
		return nil, err
	}
	pool.Exec(ctx, `UPDATE works SET thumbnail_url=$1 WHERE task_id=(SELECT id FROM tasks WHERE task_no=$2)`, publicURL, taskNo)
	return output, nil
}

func persistSubtitleFileTask(ctx context.Context, pool *pgxpool.Pool, p WorkflowTaskPayload, publicID, publicURL, contentType string, metadata map[string]interface{}) (map[string]interface{}, error) {
	taskNo := newWorkflowTaskNo(0)
	taskInput := map[string]interface{}{"video_url": metadata["source_video_url"], "operation": "remove_subtitle_track", "_skip_billing": true, "_local_free": true, "_workflow_project": publicID}
	inputJSON, _ := json.Marshal(taskInput)
	if _, err := pool.Exec(ctx, `INSERT INTO tasks (task_no,user_id,model_id,type,status,input,estimated_cost,started_at,workflow_project_id) VALUES ($1,$2,NULL,'video','running',$3,0,now(),$4)`, taskNo, p.UserID, inputJSON, p.ProjectID); err != nil {
		return nil, err
	}
	output := map[string]interface{}{"video_url": publicURL, "url": publicURL, "content_type": contentType}
	for key, value := range metadata {
		output[key] = value
	}
	outputJSON, _ := json.Marshal(output)
	var taskID int64
	if err := pool.QueryRow(ctx, `UPDATE tasks SET status='succeeded',output=$1,actual_cost=0,error_code=NULL,error_message=NULL,finished_at=now(),updated_at=now() WHERE task_no=$2 RETURNING id`, outputJSON, taskNo).Scan(&taskID); err != nil {
		return nil, err
	}
	workID := fmt.Sprintf("work_%d", time.Now().UnixNano())
	expires := configuredWorkExpiration(ctx, pool, 7)
	prompt := "移除视频独立字幕轨"
	if stringAny(metadata["subtitle_mode"]) != "soft_track" {
		prompt = "移除画面硬字幕"
	}
	pool.Exec(ctx, `INSERT INTO works (public_id,user_id,task_id,model_id,type,prompt,thumbnail_url,metadata,expires_at) VALUES ($1,$2,$3,NULL,'video',$4,$5,$6,$7)`,
		workID, p.UserID, taskID, prompt, publicURL, outputJSON, expires)
	pool.Exec(ctx, `INSERT INTO task_events (task_id,event_type,payload) VALUES ($1,'completed',$2)`, taskID, outputJSON)
	item := map[string]interface{}{"task_no": taskNo, "type": "video", "status": "succeeded", "progress": 100, "output": output, "estimated_cost": 0, "actual_cost": 0, "provider_cost": 0}
	appendWorkflowMediaTask(ctx, pool, p.ProjectID, item)
	return item, nil
}

func ensureWorkflowReservation(ctx context.Context, pool *pgxpool.Pool, userID int64, publicID string, projectID int64, amount float64) error {
	if amount <= 0 || pool == nil {
		return nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var balance, frozen float64
	if err = tx.QueryRow(ctx, `SELECT compute_balance, frozen_compute FROM wallets WHERE user_id=$1 FOR UPDATE`, userID).Scan(&balance, &frozen); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("账户余额不足")
		}
		return err
	}
	var existingID int64
	var existingAmount float64
	existingErr := tx.QueryRow(ctx, `SELECT id, amount FROM balance_freezes WHERE user_id=$1 AND ref_type='workflow' AND ref_id=$2 AND status='frozen' FOR UPDATE`, userID, publicID).Scan(&existingID, &existingAmount)
	if existingErr != nil && !errors.Is(existingErr, pgx.ErrNoRows) {
		return existingErr
	}
	if existingErr == nil {
		delta := amount - existingAmount
		if delta > 0.000001 && balance-frozen < delta {
			return errors.New("账户余额不足")
		}
		if math.Abs(delta) > 0.000001 {
			if _, err = tx.Exec(ctx, `UPDATE wallets SET frozen_compute=GREATEST(frozen_compute+$1,0), updated_at=now() WHERE user_id=$2`, delta, userID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE balance_freezes SET amount=$1 WHERE id=$2`, amount, existingID); err != nil {
				return err
			}
		}
	} else if balance-frozen < amount {
		return errors.New("账户余额不足")
	} else {
		if _, err = tx.Exec(ctx, `UPDATE wallets SET frozen_compute=frozen_compute+$1, updated_at=now() WHERE user_id=$2`, amount, userID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO balance_freezes (user_id, amount, ref_type, ref_id, status) VALUES ($1,$2,'workflow',$3,'frozen')`, userID, amount, publicID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE workflow_projects SET estimated_cost=GREATEST(estimated_cost,$1), updated_at=now() WHERE id=$2`, amount, projectID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
