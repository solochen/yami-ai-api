package breakdown

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractHTTPURLFromShareText(t *testing.T) {
	raw := "2.56 L@W.MJ pqR:/ :3pm 08/31   https://v.douyin.com/yE121DSSYh8/ 复制此链接，打开Dou音搜索，直接观看视频！"
	got, err := ExtractHTTPURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://v.douyin.com/yE121DSSYh8/" {
		t.Fatalf("url = %q", got)
	}
}

func TestBuildShotsMergesFragments(t *testing.T) {
	shots := BuildShots(10000, []int{200, 3000, 3200, 8000})
	if len(shots) != 3 {
		t.Fatalf("shots = %#v", shots)
	}
	if shots[0].StartMS != 0 || shots[0].EndMS != 3200 || shots[1].StartMS != 3200 || shots[2].EndMS != 10000 {
		t.Fatalf("bounds = %#v", shots)
	}
}

func TestKeyframeAvoidsTheCut(t *testing.T) {
	if got := KeyframeMS(0, 2000); got != 400 {
		t.Fatalf("long shot keyframe = %d", got)
	}
	if got := KeyframeMS(0, 200); got != 70 {
		t.Fatalf("short shot keyframe = %d", got)
	}
}

func TestSampleTimesCapsDensity(t *testing.T) {
	short := SampleTimes(3000)
	if len(short) < 5 || len(short) > 12 {
		t.Fatalf("3s samples = %d", len(short))
	}
	long := SampleTimes(180000)
	if len(long) > MaxSampleFrames {
		t.Fatalf("samples = %d", len(long))
	}
}

func TestPlanSegmentsFollowsShotBounds(t *testing.T) {
	shots := BuildShots(262000, []int{34000, 70000, 120000, 180000, 220000})
	segments := PlanSegments(262000, shots)
	if len(segments) < 7 || len(segments) > 12 {
		t.Fatalf("segments = %#v", segments)
	}
	if segments[0].StartMS != 0 || segments[len(segments)-1].EndMS != 262000 {
		t.Fatalf("coverage = %#v", segments)
	}
	for index, segment := range segments {
		if segment.EndMS-segment.StartMS > SegmentTargetMS+15000 {
			t.Fatalf("segment too long: %#v", segment)
		}
		if segment.ClipStartMS > segment.StartMS || segment.ClipEndMS < segment.EndMS {
			t.Fatalf("clip does not cover content: %#v", segment)
		}
		if index > 0 && segment.StartMS != segments[index-1].EndMS {
			t.Fatalf("gap between %#v and %#v", segments[index-1], segment)
		}
	}
}

func TestPlanSegmentsIndexesStayUniqueOnLongVideos(t *testing.T) {
	segments := PlanSegments(330000, nil)
	if len(segments) < 10 {
		t.Fatalf("segments = %#v", segments)
	}
	seen := map[int]bool{}
	for _, segment := range segments {
		if segment.Index < 1 || seen[segment.Index] {
			t.Fatalf("duplicate or invalid index: %#v", segment)
		}
		seen[segment.Index] = true
	}
}

func TestSplitSegmentFloors(t *testing.T) {
	left, right, ok := SplitSegment(Segment{Index: 1, StartMS: 0, EndMS: 30000}, 30000, 11, 12)
	if !ok {
		t.Fatal("30s segment should split")
	}
	if left.StartMS != 0 || left.EndMS != 15000 || right.StartMS != 15000 || right.EndMS != 30000 {
		t.Fatalf("split = %#v %#v", left, right)
	}
	if left.Index != 11 || right.Index != 12 {
		t.Fatalf("indices = %d %d", left.Index, right.Index)
	}
	if left.ClipStartMS != 0 || right.ClipEndMS != 30000 {
		t.Fatalf("clip bounds = %#v %#v", left, right)
	}
	// 12 秒以上的片段还可以再拆一层，短于下限的不拆。
	if _, _, ok := SplitSegment(Segment{Index: 1, StartMS: 0, EndMS: 15000}, 15000, 21, 22); !ok {
		t.Fatal("15s segment should split again")
	}
	if _, _, ok := SplitSegment(Segment{Index: 1, StartMS: 0, EndMS: 11000}, 11000, 21, 22); ok {
		t.Fatal("11s segment must not split")
	}
}

func TestParseSRTAndMarkdown(t *testing.T) {
	cues := ParseSRT("1\n00:00:01,000 --> 00:00:02,500\n这一口才是夏日\n")
	if len(cues) != 1 || cues[0].Text != "这一口才是夏日" || cues[0].StartMS != 1000 || cues[0].Source != "soft" {
		t.Fatalf("cues = %#v", cues)
	}
	raw, err := ParseJSONObject("```json\n{\"style\":\"雨夜古装\",\"scenes\":[\"古巷\"],\"characters\":[{\"name\":\"男主\",\"appearance\":\"青衫\"}],\"units\":[{\"start_sec\":0,\"end_sec\":8,\"shots\":[{\"camera\":\"近景\",\"action\":\"抬头\",\"dialogue\":\"\"}]}]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	text := RenderMarkdown(DecodeScript(raw))
	for _, want := range []string{"【视频风格】", "雨夜古装", "【人物】", "男主", "无对白", "【Unit 1：0s–8s】"} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown missing %q:\n%s", want, text)
		}
	}
}

func TestMediaVideoURLReadsCurrentParser(t *testing.T) {
	raw := map[string]interface{}{
		"data": map[string]interface{}{
			"state": "done",
			"data": map[string]interface{}{
				"media": map[string]interface{}{
					"video": map[string]interface{}{
						"url": "https://cdn.example.com/video.mp4",
					},
				},
			},
		},
	}
	if got := MediaVideoURL(raw); got != "https://cdn.example.com/video.mp4" {
		t.Fatalf("url = %q", got)
	}
}

func TestScriptFromFramesUsesVision(t *testing.T) {
	script := ScriptFromFrames(20000, []FrameFact{{TMS: 1000, Scene: "雨夜古巷", Action: "女子回头", Camera: "近景"}}, nil)
	text := RenderMarkdown(script)
	if !strings.Contains(text, "雨夜古巷") || !strings.Contains(text, "女子回头") {
		t.Fatalf("markdown = %s", text)
	}
}

func TestParseSceneTimes(t *testing.T) {
	log := "[Parsed_showinfo_0 @ 0x1] n:0 pts:123 pts_time:1.5\n[Parsed_showinfo_0 @ 0x1] n:1 pts:456 pts_time:4\n"
	got := ParseSceneTimes(log)
	if len(got) != 2 || got[0] != 1500 || got[1] != 4000 {
		t.Fatalf("cuts = %#v", got)
	}
}

func TestMergeScriptsOrdersUnitsAndAssignsStableCharacterCodes(t *testing.T) {
	merged := MergeScripts([]Script{
		{Style: "雨夜古装", Scenes: []string{"古巷"}, Characters: []Character{{Name: "男主", Appearance: "灰衣"}}, Units: []Unit{{StartSec: 6, EndSec: 12}}},
		{Style: "雨夜古装", Scenes: []string{"古巷", "正厅"}, Characters: []Character{{Name: "男主", Appearance: "灰衣"}, {Name: "陈阿素", Appearance: "红衣"}}, Units: []Unit{{StartSec: 0, EndSec: 6}}},
	})
	if got := []int{merged.Units[0].StartSec, merged.Units[1].StartSec}; !reflect.DeepEqual(got, []int{0, 6}) {
		t.Fatalf("unit order = %#v", got)
	}
	if len(merged.Characters) != 2 || merged.Characters[0].Code != "CHAR_01" || merged.Characters[1].Code != "CHAR_02" {
		t.Fatalf("characters = %#v", merged.Characters)
	}
	if !reflect.DeepEqual(merged.Scenes, []string{"古巷", "正厅"}) {
		t.Fatalf("scenes = %#v", merged.Scenes)
	}
}

func TestBuildProductionPackagePassesCompleteTimeline(t *testing.T) {
	script := Script{
		Style:      "青绿色冷调江南雨夜，电影感低照度",
		Scenes:     []string{"江南古巷/外/雨夜"},
		Characters: []Character{{Name: "男主", Appearance: "灰衣束发，眼下有痣"}},
		Units: []Unit{
			{StartSec: 0, EndSec: 6, Scene: "江南古巷/外/雨夜", Shots: []Take{{Camera: "中景缓推", Characters: []string{"男主"}, Action: "男主从巷口走到井边停下", Dialogue: "无对白", EvidenceStart: 0, EvidenceEnd: 6000}}},
			{StartSec: 6, EndSec: 12, Scene: "江南古巷/外/雨夜", Shots: []Take{{Camera: "近景固定", Characters: []string{"男主"}, Action: "男主抬眼望向井边，终态为神情震惊", Speaker: "男主", Emotion: "诧异", Dialogue: "小师叔", EvidenceStart: 6000, EvidenceEnd: 12000}}},
		},
	}
	pkg := BuildProductionPackage(script, 12000)
	if pkg.Quality.Status != "passed" || pkg.Quality.CoveragePercent != 100 || len(pkg.Shots) != 2 {
		t.Fatalf("package quality = %#v, shots = %#v", pkg.Quality, pkg.Shots)
	}
	if pkg.Shots[0].ID != "SHOT_001" || pkg.Shots[0].LocationCode != "LOC_01" || len(pkg.Shots[0].CharacterCodes) != 1 || pkg.Shots[0].Continuity == "" {
		t.Fatalf("first shot = %#v", pkg.Shots[0])
	}
	markdown := RenderProductionMarkdown(pkg)
	for _, want := range []string{"【生产规格】", "CHAR_01", "LOC_01", "SHOT_001", "关键帧提示词", "视频提示词", "原片证据"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("production markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestValidateProductionPackageRejectsTimelineGap(t *testing.T) {
	pkg := ProductionPackage{DurationSec: 12, Style: "电影感", Locations: []ProductionLocation{{Code: "LOC_01", Description: "古巷"}}, Shots: []ProductionShot{
		{ID: "SHOT_001", StartSec: 0, EndSec: 4, DurationSec: 4, Scene: "古巷", LocationCode: "LOC_01", Action: "抬头", Continuity: "首镜", KeyframePrompt: "抬头前", VideoPrompt: "缓慢抬头", Dialogue: "无对白", EvidenceEnd: 4000},
		{ID: "SHOT_002", StartSec: 8, EndSec: 12, DurationSec: 4, Scene: "古巷", LocationCode: "LOC_01", Action: "转身", Continuity: "承接", KeyframePrompt: "转身前", VideoPrompt: "转身离开", Dialogue: "无对白", EvidenceStart: 8000, EvidenceEnd: 12000},
	}}
	report := ValidateProductionPackage(pkg)
	if report.Status != "failed" || report.MaxGapSec != 4 || report.CoveragePercent >= 99 {
		t.Fatalf("quality = %#v", report)
	}
}
