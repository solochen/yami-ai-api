package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/starai/worker/internal/storage"
)

var errSubtitleMux = errors.New("mux failed")

func TestSubtitleRemovalDecisionMatrix(t *testing.T) {
	model := true
	cases := []struct {
		name   string
		in     subtitleDecisionInput
		action string
		mode   string
		clean  bool
	}{
		{name: "soft only", in: subtitleDecisionInput{Mode: "soft_track", HadTrack: true}, action: "soft", mode: "soft_track"},
		{name: "soft missing", in: subtitleDecisionInput{Mode: "soft_track"}, action: "fail"},
		{name: "auto track clean", in: subtitleDecisionInput{Mode: "auto", HadTrack: true, HasModel: model}, action: "soft", mode: "soft_track"},
		{name: "auto track and burned", in: subtitleDecisionInput{Mode: "auto", HadTrack: true, BurnedIn: true, HasModel: model}, action: "ai", mode: "soft_and_hard", clean: true},
		{name: "auto burned without model", in: subtitleDecisionInput{Mode: "auto", HadTrack: true, BurnedIn: true}, action: "fail"},
		{name: "auto no track uses model", in: subtitleDecisionInput{Mode: "auto", HasModel: model}, action: "ai", mode: "hardcoded_ai"},
		{name: "auto nothing", in: subtitleDecisionInput{Mode: "auto"}, action: "fail"},
		{name: "hardcoded", in: subtitleDecisionInput{Mode: "hardcoded_ai", HasModel: model}, action: "ai", mode: "hardcoded_ai"},
		{name: "hardcoded missing model", in: subtitleDecisionInput{Mode: "hardcoded_ai"}, action: "fail"},
		{name: "auto remux falls back to repair", in: subtitleDecisionInput{Mode: "auto", SoftErr: errSubtitleMux, HasModel: model}, action: "ai", mode: "hardcoded_ai"},
		{name: "auto remux without model fails", in: subtitleDecisionInput{Mode: "auto", SoftErr: errSubtitleMux}, action: "fail"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decideSubtitleRemoval(tc.in)
			if got.Action != tc.action || got.ResultMode != tc.mode || got.UseCleanedSource != tc.clean {
				t.Fatalf("decision=%+v", got)
			}
		})
	}
}

func TestBuildSubtitleRemovalPromptIncludesControls(t *testing.T) {
	protected := buildSubtitleRemovalPrompt("remove subtitles", "keep the logo", "bottom_15", true, true)
	if !strings.Contains(protected, "bottom 15%") || !strings.Contains(protected, "Keep every pixel outside") || !strings.Contains(protected, "Preserve the original audio") || !strings.Contains(protected, "keep the logo") {
		t.Fatalf("protected prompt = %s", protected)
	}
	open := buildSubtitleRemovalPrompt("", "", "full", false, false)
	if !strings.Contains(open, "full frame") || !strings.Contains(open, "silent video") {
		t.Fatalf("open prompt = %s", open)
	}
	region, ok := canonicalSubtitleRegion("full_frame")
	if !ok || region != "full" {
		t.Fatalf("region=%q ok=%v", region, ok)
	}
	if _, ok := canonicalSubtitleRegion("middle"); ok {
		t.Fatal("invalid region accepted")
	}
}

func TestSubtitleCopySpecChoosesCompatibleContainer(t *testing.T) {
	ext, mime := subtitleCopySpec(subtitleProbe{VideoCodec: "vp9", AudioCodec: "opus"})
	if ext != ".webm" || mime != "video/webm" {
		t.Fatalf("webm spec %s %s", ext, mime)
	}
	ext, mime = subtitleCopySpec(subtitleProbe{VideoCodec: "h264", AudioCodec: "aac"})
	if ext != ".mp4" || mime != "video/mp4" {
		t.Fatalf("mp4 spec %s %s", ext, mime)
	}
	ext, mime = subtitleCopySpec(subtitleProbe{VideoCodec: "vp9", AudioCodec: "aac"})
	if ext != ".mkv" || mime != "video/x-matroska" {
		t.Fatalf("mkv spec %s %s", ext, mime)
	}
}

func TestStageSubtitleSourceReadsStoredMedia(t *testing.T) {
	if _, err := ffmpegBinaryPath(); err != nil {
		t.Skip(err.Error())
	}
	dir := t.TempDir()
	store, err := storage.NewLocal(dir, "/media")
	if err != nil {
		t.Fatal(err)
	}
	previous := objectStore
	objectStore = store
	t.Cleanup(func() { objectStore = previous })
	data, err := os.ReadFile(paintVideo(t, false))
	if err != nil {
		t.Fatal(err)
	}
	url, err := store.Upload(context.Background(), "works/subtitle-source.mp4", "video/mp4", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := stageSubtitleSource(context.Background(), url, 20<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	probe, err := probeSubtitleFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if probe.Width != 320 || probe.Height != 180 {
		t.Fatalf("staged probe=%+v", probe)
	}
}

func TestSubtitleRemovalPipeline(t *testing.T) {
	if _, err := ffmpegBinaryPath(); err != nil {
		t.Skip(err.Error())
	}
	ctx := context.Background()

	t.Run("soft track keeps audio and does not call repair", func(t *testing.T) {
		source := muxSubtitleTrack(t, paintVideo(t, false), true)
		called := false
		result, err := executeSubtitleJob(ctx, subtitleJob{
			Mode: "auto", Region: "bottom_25", ProtectWatermark: true, PreserveAudio: true, HasModel: true,
			SourcePath: source, MaxDuration: 30,
		}, func(context.Context, string, string) (string, error) {
			called = true
			return "", nil
		})
		if err != nil {
			t.Fatal(err)
		}
		defer result.Cleanup()
		if called || result.AIUsed || result.Metadata["subtitle_mode"] != "soft_track" {
			t.Fatalf("unexpected soft result called=%v result=%+v", called, result.Metadata)
		}
		probe, err := probeSubtitleFile(ctx, result.OutputPath)
		if err != nil {
			t.Fatal(err)
		}
		if probe.HasSubtitle || probe.AudioCodec == "" {
			t.Fatalf("soft output probe=%+v", probe)
		}
	})

	t.Run("soft track can drop audio", func(t *testing.T) {
		source := muxSubtitleTrack(t, paintVideo(t, false), true)
		result, err := executeSubtitleJob(ctx, subtitleJob{
			Mode: "soft_track", Region: "bottom_25", PreserveAudio: false, SourcePath: source, MaxDuration: 30,
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer result.Cleanup()
		probe, err := probeSubtitleFile(ctx, result.OutputPath)
		if err != nil {
			t.Fatal(err)
		}
		if probe.HasSubtitle || probe.AudioCodec != "" {
			t.Fatalf("muted soft output probe=%+v", probe)
		}
	})

	t.Run("burned-in subtitles keep the area outside the mask", func(t *testing.T) {
		source := paintVideo(t, true)
		var repairedSource string
		result, err := executeSubtitleJob(ctx, subtitleJob{
			Mode: "auto", Region: "bottom_25", ProtectWatermark: true, PreserveAudio: false, HasModel: true,
			SourcePath: source, MaxDuration: 30, UserPrompt: "protect the corner mark",
		}, func(_ context.Context, sourcePath, prompt string) (string, error) {
			repairedSource = sourcePath
			if !strings.Contains(prompt, "bottom 25%") || !strings.Contains(prompt, "protect the corner mark") || !strings.Contains(prompt, "silent video") {
				t.Fatalf("prompt = %s", prompt)
			}
			return paintVideo(t, false, color.RGBA{G: 220, A: 255}), nil
		})
		if err != nil {
			t.Fatal(err)
		}
		defer result.Cleanup()
		if !result.AIUsed || result.Metadata["subtitle_mode"] != "hardcoded_ai" || repairedSource != source {
			t.Fatalf("metadata=%v source=%s repaired=%s", result.Metadata, source, repairedSource)
		}
		assertRegionComposite(t, result.OutputPath)
	})

	t.Run("track plus burned-in repairs the cleaned video", func(t *testing.T) {
		source := muxSubtitleTrack(t, paintVideo(t, true), true)
		var sawSubtitle bool
		result, err := executeSubtitleJob(ctx, subtitleJob{
			Mode: "auto", Region: "bottom_25", ProtectWatermark: true, PreserveAudio: true, HasModel: true,
			SourcePath: source, MaxDuration: 30,
		}, func(ctx context.Context, sourcePath, prompt string) (string, error) {
			probe, probeErr := probeSubtitleFile(ctx, sourcePath)
			if probeErr != nil {
				t.Fatal(probeErr)
			}
			sawSubtitle = probe.HasSubtitle
			if !strings.Contains(prompt, "Preserve the original audio") {
				t.Fatalf("prompt = %s", prompt)
			}
			return paintVideo(t, false, color.RGBA{G: 220, A: 255}), nil
		})
		if err != nil {
			t.Fatal(err)
		}
		defer result.Cleanup()
		if sawSubtitle || result.Metadata["subtitle_mode"] != "soft_and_hard" {
			t.Fatalf("sawSubtitle=%v metadata=%v", sawSubtitle, result.Metadata)
		}
		assertRegionComposite(t, result.OutputPath)
		probe, err := probeSubtitleFile(ctx, result.OutputPath)
		if err != nil {
			t.Fatal(err)
		}
		if probe.HasSubtitle || probe.AudioCodec == "" {
			t.Fatalf("combined output probe=%+v", probe)
		}
	})

	t.Run("duration limit is enforced from the file", func(t *testing.T) {
		_, err := executeSubtitleJob(ctx, subtitleJob{Mode: "auto", Region: "bottom_25", SourcePath: paintVideo(t, false), MaxDuration: 0.2}, nil)
		if err == nil || !strings.Contains(err.Error(), "视频时长不能超过") {
			t.Fatalf("expected duration error, got %v", err)
		}
	})

	t.Run("non-mp4 soft subtitle keeps a copyable container", func(t *testing.T) {
		source := webmWithSubtitle(t)
		if source == "" {
			t.Skip("webm encoder or subtitle mux is unavailable")
		}
		result, err := executeSubtitleJob(ctx, subtitleJob{Mode: "soft_track", Region: "full", PreserveAudio: true, SourcePath: source, MaxDuration: 30}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer result.Cleanup()
		probe, err := probeSubtitleFile(ctx, result.OutputPath)
		if err != nil {
			t.Fatal(err)
		}
		if probe.HasSubtitle || (probe.VideoCodec != "vp8" && probe.VideoCodec != "vp9" && probe.VideoCodec != "av1") {
			t.Fatalf("webm output probe=%+v path=%s", probe, result.OutputPath)
		}
	})
}

func paintVideo(t *testing.T, burned bool, fills ...color.RGBA) string {
	t.Helper()
	top := color.RGBA{R: 220, G: 32, B: 32, A: 255}
	solid := len(fills) > 0
	if solid {
		top = fills[0]
	}
	frames := make([]image.Image, 8)
	for i := range frames {
		frames[i] = subtitleTestFrame(top, burned && !solid, solid)
	}
	dir := t.TempDir()
	for i, frame := range frames {
		writeTestPNG(t, filepath.Join(dir, filenameFrame(i)), frame)
	}
	output := filepath.Join(dir, "video.mp4")
	if err := runFFmpeg(context.Background(), "-y", "-framerate", "8", "-start_number", "0", "-i", filepath.Join(dir, "frame-%03d.png"), "-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "18", output); err != nil {
		t.Fatal(err)
	}
	return output
}

func filenameFrame(index int) string {
	return "frame-" + leftPad3(index) + ".png"
}

func leftPad3(index int) string {
	raw := []byte{'0', '0', '0'}
	raw[2] = byte('0' + index%10)
	raw[1] = byte('0' + (index/10)%10)
	raw[0] = byte('0' + (index/100)%10)
	return string(raw)
}

func subtitleTestFrame(top color.RGBA, burned, solid bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 320, 180))
	fill(img, img.Bounds(), top)
	if solid {
		return img
	}
	fill(img, image.Rect(0, 136, 320, 180), color.RGBA{A: 255})
	if burned {
		fill(img, image.Rect(24, 146, 296, 174), color.RGBA{R: 255, G: 255, B: 255, A: 255})
		for x := 36; x < 286; x += 12 {
			fill(img, image.Rect(x, 150, x+3, 170), color.RGBA{A: 255})
		}
	}
	return img
}

func fill(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func writeTestPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func muxSubtitleTrack(t *testing.T, video string, withAudio bool) string {
	t.Helper()
	dir := t.TempDir()
	current := video
	if withAudio {
		audio := filepath.Join(dir, "audio.mp4")
		if err := runFFmpeg(context.Background(), "-y", "-i", video, "-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono", "-shortest", "-c:v", "copy", "-c:a", "aac", audio); err != nil {
			t.Fatal(err)
		}
		current = audio
	}
	srt := filepath.Join(dir, "subs.srt")
	if err := os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nHello subtitle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "subtitled.mp4")
	if err := runFFmpeg(context.Background(), "-y", "-i", current, "-i", srt, "-map", "0:v:0", "-map", "0:a?", "-map", "1", "-c", "copy", "-c:s", "mov_text", output); err != nil {
		t.Fatal(err)
	}
	probe, err := probeSubtitleFile(context.Background(), output)
	if err != nil || !probe.HasSubtitle {
		t.Fatalf("fixture missing subtitle track: %+v %v", probe, err)
	}
	return output
}

func webmWithSubtitle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	video := filepath.Join(dir, "source.webm")
	if err := runFFmpeg(context.Background(), "-y", "-f", "lavfi", "-i", "color=c=blue:s=320x180:d=1:r=8", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono", "-shortest", "-c:v", "libvpx-vp9", "-c:a", "libopus", video); err != nil {
		return ""
	}
	srt := filepath.Join(dir, "subs.srt")
	if err := os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nHello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "subtitled.webm")
	if err := runFFmpeg(context.Background(), "-y", "-i", video, "-i", srt, "-map", "0:v:0", "-map", "0:a?", "-map", "1", "-c", "copy", "-c:s", "webvtt", output); err != nil {
		return ""
	}
	return output
}

func assertRegionComposite(t *testing.T, path string) {
	t.Helper()
	frame := filepath.Join(t.TempDir(), "check.png")
	if err := runFFmpeg(context.Background(), "-y", "-i", path, "-frames:v", "1", frame); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(frame)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	top := img.At(20, 20)
	bottom := img.At(160, 165)
	tr, tg, tb, _ := top.RGBA()
	br, bg, bb, _ := bottom.RGBA()
	if tr <= tg || tr <= tb {
		t.Fatalf("top pixel lost original red: %v", top)
	}
	if bg <= br || bg <= bb {
		t.Fatalf("bottom pixel was not replaced: %v", bottom)
	}
}
