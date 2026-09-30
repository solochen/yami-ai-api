package breakdown

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	MinShotMS        = 300
	MaxDurationMS    = 180000
	MaxSampleFrames  = 90
	SampleIntervalMS = 333
	SegmentTargetMS  = 30000
	SegmentMinMS     = 20000
	SegmentOverlapMS = 1500
	SplitFloorMS     = 12000
)

type Shot struct {
	Index   int `json:"index"`
	StartMS int `json:"start_ms"`
	EndMS   int `json:"end_ms"`
}

type Cue struct {
	StartMS    int     `json:"start_ms"`
	EndMS      int     `json:"end_ms"`
	Text       string  `json:"text"`
	Speaker    string  `json:"speaker,omitempty"`
	Emotion    string  `json:"emotion,omitempty"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence,omitempty"`
}

type Script struct {
	Style      string      `json:"style"`
	Scenes     []string    `json:"scenes"`
	Characters []Character `json:"characters"`
	Units      []Unit      `json:"units"`
	Conflicts  []string    `json:"conflicts"`
}

type Character struct {
	Code       string `json:"code,omitempty"`
	Name       string `json:"name"`
	Appearance string `json:"appearance"`
}

type Unit struct {
	StartSec int    `json:"start_sec"`
	EndSec   int    `json:"end_sec"`
	Scene    string `json:"scene,omitempty"`
	Shots    []Take `json:"shots"`
}

type Take struct {
	Camera         string   `json:"camera"`
	Action         string   `json:"action"`
	Characters     []string `json:"characters,omitempty"`
	Speaker        string   `json:"speaker,omitempty"`
	Emotion        string   `json:"emotion,omitempty"`
	Dialogue       string   `json:"dialogue"`
	Continuity     string   `json:"continuity,omitempty"`
	KeyframePrompt string   `json:"keyframe_prompt,omitempty"`
	VideoPrompt    string   `json:"video_prompt,omitempty"`
	EvidenceStart  int      `json:"evidence_start_ms,omitempty"`
	EvidenceEnd    int      `json:"evidence_end_ms,omitempty"`
}

func ExtractHTTPURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "http")
	if start < 0 {
		return "", fmt.Errorf("未找到视频链接")
	}
	raw = raw[start:]
	if end := strings.IndexAny(raw, " \t\r\n"); end >= 0 {
		raw = raw[:end]
	}
	raw = strings.Trim(raw, `"'<>，。；;）)]】`)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", fmt.Errorf("仅支持公开的 HTTP/HTTPS 链接")
	}
	return parsed.String(), nil
}

func ParseSceneTimes(log string) []int {
	seen := map[int]bool{}
	var cuts []int
	for _, line := range strings.Split(log, "\n") {
		index := strings.Index(line, "pts_time:")
		if index < 0 {
			continue
		}
		rest := line[index+len("pts_time:"):]
		end := 0
		for end < len(rest) && (unicode.IsDigit(rune(rest[end])) || rest[end] == '.') {
			end++
		}
		seconds, err := strconv.ParseFloat(rest[:end], 64)
		if err != nil || seconds < 0 {
			continue
		}
		ms := int(math.Round(seconds * 1000))
		if seen[ms] {
			continue
		}
		seen[ms] = true
		cuts = append(cuts, ms)
	}
	sort.Ints(cuts)
	return cuts
}

func BuildShots(durationMS int, cuts []int) []Shot {
	if durationMS < 1 {
		durationMS = 1
	}
	bounds := []int{0}
	for _, cut := range cuts {
		if cut <= 0 || cut >= durationMS {
			continue
		}
		if bounds[len(bounds)-1] == cut {
			continue
		}
		bounds = append(bounds, cut)
	}
	bounds = append(bounds, durationMS)
	shots := make([]Shot, 0, len(bounds)-1)
	for i := 0; i < len(bounds)-1; i++ {
		shots = append(shots, Shot{StartMS: bounds[i], EndMS: bounds[i+1]})
	}
	merged := make([]Shot, 0, len(shots))
	for _, shot := range shots {
		if len(merged) > 0 && shot.EndMS-shot.StartMS < MinShotMS {
			merged[len(merged)-1].EndMS = shot.EndMS
			continue
		}
		merged = append(merged, shot)
	}
	if len(merged) >= 2 && merged[0].EndMS-merged[0].StartMS < MinShotMS {
		merged[1].StartMS = merged[0].StartMS
		merged = merged[1:]
	}
	for i := range merged {
		merged[i].Index = i + 1
	}
	return merged
}

type Segment struct {
	Index       int `json:"index"`
	StartMS     int `json:"start_ms"`
	EndMS       int `json:"end_ms"`
	ClipStartMS int `json:"clip_start_ms"`
	ClipEndMS   int `json:"clip_end_ms"`
}

func PlanSegments(durationMS int, shots []Shot) []Segment {
	if durationMS < 1 {
		durationMS = 1
	}
	if len(shots) == 0 {
		shots = []Shot{{Index: 1, StartMS: 0, EndMS: durationMS}}
	}
	var ends []int
	cursor := 0
	for cursor < durationMS {
		cut := 0
		for _, shot := range shots {
			if shot.EndMS <= cursor || shot.EndMS > durationMS {
				continue
			}
			span := shot.EndMS - cursor
			if span >= SegmentMinMS && span <= SegmentTargetMS {
				cut = shot.EndMS
			}
		}
		if cut <= cursor {
			cut = cursor + SegmentTargetMS
		}
		if cut > durationMS || durationMS-cut < 10000 {
			cut = durationMS
		}
		ends = append(ends, cut)
		if cut <= cursor {
			break
		}
		cursor = cut
	}
	segments := make([]Segment, 0, len(ends))
	start := 0
	for _, end := range ends {
		clipStart := start - SegmentOverlapMS
		if clipStart < 0 {
			clipStart = 0
		}
		clipEnd := end + SegmentOverlapMS
		if clipEnd > durationMS {
			clipEnd = durationMS
		}
		segments = append(segments, Segment{
			Index:       len(segments) + 1,
			StartMS:     start,
			EndMS:       end,
			ClipStartMS: clipStart,
			ClipEndMS:   clipEnd,
		})
		start = end
	}
	return segments
}

// SplitSegment 把超时片段对半拆成两段用于重试。编号由调用方分配，
// 避免旧的 Index*10 算术在片段数达到 10 以上时与根片段编号冲突。
// 片段短于 SplitFloorMS 或时长非法时返回 false。
func SplitSegment(segment Segment, durationMS, leftIndex, rightIndex int) (Segment, Segment, bool) {
	span := segment.EndMS - segment.StartMS
	if span < SplitFloorMS || durationMS < 1 {
		return Segment{}, Segment{}, false
	}
	mid := segment.StartMS + span/2
	return segmentPart(segment.StartMS, mid, durationMS, leftIndex), segmentPart(mid, segment.EndMS, durationMS, rightIndex), true
}

func segmentPart(start, end, durationMS, index int) Segment {
	clipStart := start - SegmentOverlapMS
	if clipStart < 0 {
		clipStart = 0
	}
	clipEnd := end + SegmentOverlapMS
	if clipEnd > durationMS {
		clipEnd = durationMS
	}
	return Segment{Index: index, StartMS: start, EndMS: end, ClipStartMS: clipStart, ClipEndMS: clipEnd}
}

func KeyframeMS(start, end int) int {
	span := end - start
	if span <= 0 {
		return start
	}
	offset := 400
	if scaled := int(float64(span) * 0.35); scaled < offset {
		offset = scaled
	}
	at := start + offset
	if at >= end {
		return start
	}
	return at
}

func SampleTimes(durationMS int) []int {
	if durationMS < 1 {
		return []int{0}
	}
	interval := SampleIntervalMS
	if count := durationMS / interval; count > MaxSampleFrames {
		interval = durationMS / MaxSampleFrames
		if interval < 1 {
			interval = 1
		}
	}
	var times []int
	for at := 0; at < durationMS && len(times) < MaxSampleFrames; at += interval {
		times = append(times, at)
	}
	if len(times) == 0 {
		return []int{0}
	}
	return times
}

func ParseSRT(raw string) []Cue {
	blocks := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n\n")
	var cues []Cue
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) < 2 {
			continue
		}
		timing := lines[0]
		textStart := 1
		if !strings.Contains(timing, "-->") && len(lines) >= 2 {
			timing = lines[1]
			textStart = 2
		}
		start, end, ok := parseSRTRange(timing)
		if !ok || textStart >= len(lines) {
			continue
		}
		text := strings.TrimSpace(strings.Join(lines[textStart:], " "))
		if text == "" {
			continue
		}
		cues = append(cues, Cue{StartMS: start, EndMS: end, Text: text, Source: "soft"})
	}
	return cues
}

func parseSRTRange(line string) (int, int, bool) {
	parts := strings.Split(line, "-->")
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, ok1 := parseSRTClock(parts[0])
	end, ok2 := parseSRTClock(parts[1])
	return start, end, ok1 && ok2 && end >= start
}

func parseSRTClock(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if fields := strings.Fields(raw); len(fields) > 0 {
		raw = fields[0]
	}
	raw = strings.ReplaceAll(raw, ",", ".")
	pieces := strings.Split(raw, ":")
	if len(pieces) != 3 {
		return 0, false
	}
	hours, err1 := strconv.Atoi(pieces[0])
	minutes, err2 := strconv.Atoi(pieces[1])
	sec, err3 := strconv.ParseFloat(pieces[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	return hours*3600000 + minutes*60000 + int(math.Round(sec*1000)), true
}

func MediaVideoURL(value interface{}) string {
	return mediaVideoURL(value, 0)
}

func mediaVideoURL(value interface{}, depth int) string {
	if depth > 6 {
		return ""
	}
	item, ok := value.(map[string]interface{})
	if !ok {
		return ""
	}
	if media, ok := item["media"].(map[string]interface{}); ok {
		if video, ok := media["video"].(map[string]interface{}); ok {
			if link := httpURL(video["url"]); link != "" {
				return link
			}
			if urls, ok := video["urls"].([]interface{}); ok {
				for _, candidate := range urls {
					if link := httpURL(candidate); link != "" {
						return link
					}
				}
			}
		}
	}
	for _, key := range []string{"play_addr", "playAddr", "play_addr_h264"} {
		if link := keyedHTTPURL(item, key); link != "" {
			return link
		}
	}
	if nested, ok := item["data"]; ok {
		if link := mediaVideoURL(nested, depth+1); link != "" {
			return link
		}
	}
	return ""
}

func keyedHTTPURL(item map[string]interface{}, wanted string) string {
	for key, nested := range item {
		if strings.EqualFold(key, wanted) {
			if link := httpURL(nested); link != "" {
				return link
			}
		}
	}
	for _, nested := range item {
		switch child := nested.(type) {
		case map[string]interface{}:
			if link := keyedHTTPURL(child, wanted); link != "" {
				return link
			}
		case []interface{}:
			for _, entry := range child {
				if childMap, ok := entry.(map[string]interface{}); ok {
					if link := keyedHTTPURL(childMap, wanted); link != "" {
						return link
					}
				}
			}
		}
	}
	return ""
}

func httpURL(value interface{}) string {
	switch item := value.(type) {
	case string:
		parsed, err := url.Parse(strings.TrimSpace(item))
		if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil {
			return parsed.String()
		}
	case map[string]interface{}:
		for _, key := range []string{"url", "url_list"} {
			if link := httpURL(item[key]); link != "" {
				return link
			}
		}
		if list, ok := item["url_list"].([]interface{}); ok {
			for _, entry := range list {
				if link := httpURL(entry); link != "" {
					return link
				}
			}
		}
	case []interface{}:
		for _, entry := range item {
			if link := httpURL(entry); link != "" {
				return link
			}
		}
	}
	return ""
}

func ParseJSONObject(raw string) (map[string]interface{}, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("模型没有返回 JSON")
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func DecodeScript(raw map[string]interface{}) Script {
	script := Script{Style: strings.TrimSpace(stringValue(raw["style"]))}
	script.Scenes = stringList(raw["scenes"])
	script.Conflicts = stringList(raw["conflicts"])
	for _, item := range sliceValue(raw["characters"]) {
		row, _ := item.(map[string]interface{})
		if row == nil {
			continue
		}
		name := strings.TrimSpace(stringValue(row["name"]))
		appearance := strings.TrimSpace(stringValue(row["appearance"]))
		if name == "" && appearance == "" {
			continue
		}
		script.Characters = append(script.Characters, Character{Code: strings.TrimSpace(stringValue(row["code"])), Name: name, Appearance: appearance})
	}
	for _, item := range sliceValue(raw["units"]) {
		row, _ := item.(map[string]interface{})
		if row == nil {
			continue
		}
		unit := Unit{StartSec: intValue(row["start_sec"]), EndSec: intValue(row["end_sec"]), Scene: strings.TrimSpace(stringValue(row["scene"]))}
		for _, shotRaw := range sliceValue(row["shots"]) {
			shot, _ := shotRaw.(map[string]interface{})
			if shot == nil {
				continue
			}
			take := Take{
				Camera:         strings.TrimSpace(stringValue(shot["camera"])),
				Action:         strings.TrimSpace(stringValue(shot["action"])),
				Characters:     stringList(shot["characters"]),
				Speaker:        strings.TrimSpace(stringValue(shot["speaker"])),
				Emotion:        strings.TrimSpace(stringValue(shot["emotion"])),
				Dialogue:       strings.TrimSpace(stringValue(shot["dialogue"])),
				Continuity:     strings.TrimSpace(stringValue(shot["continuity"])),
				KeyframePrompt: strings.TrimSpace(stringValue(shot["keyframe_prompt"])),
				VideoPrompt:    strings.TrimSpace(stringValue(shot["video_prompt"])),
				EvidenceStart:  intValue(shot["evidence_start_ms"]),
				EvidenceEnd:    intValue(shot["evidence_end_ms"]),
			}
			if take.Camera == "" && take.Action == "" && take.Dialogue == "" {
				continue
			}
			if take.Dialogue == "" {
				take.Dialogue = "无对白"
			}
			unit.Shots = append(unit.Shots, take)
		}
		if len(unit.Shots) == 0 {
			continue
		}
		script.Units = append(script.Units, unit)
	}
	return script
}

func RenderMarkdown(script Script) string {
	var b strings.Builder
	b.WriteString("【视频风格】\n")
	if script.Style == "" {
		b.WriteString("画面证据不足，未写入风格判断。\n\n")
	} else {
		b.WriteString(script.Style + "\n\n")
	}
	b.WriteString("【场景】\n")
	if len(script.Scenes) == 0 {
		b.WriteString("未从画面中确认独立场景。\n\n")
	} else {
		for _, scene := range script.Scenes {
			b.WriteString("- " + scene + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("【人物】\n")
	if len(script.Characters) == 0 {
		b.WriteString("未从画面中确认可连续识别的人物。\n\n")
	} else {
		for _, person := range script.Characters {
			line := person.Name
			if person.Appearance != "" {
				if line != "" {
					line += "："
				}
				line += person.Appearance
			}
			b.WriteString("- " + line + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("【分镜】\n")
	if len(script.Units) == 0 {
		b.WriteString("没有足够证据写成镜头。\n")
	}
	for index, unit := range script.Units {
		title := fmt.Sprintf("\n【Unit %d：%ds–%ds", index+1, unit.StartSec, unit.EndSec)
		if unit.Scene != "" {
			title += " · " + unit.Scene
		}
		b.WriteString(title + "】\n")
		for _, shot := range unit.Shots {
			b.WriteString("- " + joinShot(shot) + "\n")
		}
	}
	if len(script.Conflicts) > 0 {
		b.WriteString("\n【冲突】\n")
		for _, item := range script.Conflicts {
			b.WriteString("- " + item + "\n")
		}
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func ScriptFromFrames(durationMS int, frames []FrameFact, cues []Cue) Script {
	script := Script{Style: "由关键帧画面整理。整段视频理解未写入。"}
	seen := map[string]bool{}
	for _, frame := range frames {
		scene := strings.TrimSpace(frame.Scene)
		if scene != "" && !seen[scene] && len(script.Scenes) < 12 {
			seen[scene] = true
			script.Scenes = append(script.Scenes, scene)
		}
	}
	if durationMS < 1000 {
		durationMS = 1000
	}
	const windowMS = 12000
	for start := 0; start < durationMS; start += windowMS {
		end := start + windowMS
		if end > durationMS {
			end = durationMS
		}
		var actions []string
		camera := ""
		for _, frame := range frames {
			if frame.TMS < start || frame.TMS >= end {
				continue
			}
			if camera == "" {
				camera = strings.TrimSpace(frame.Camera)
			}
			action := strings.TrimSpace(frame.Action)
			if action == "" {
				action = strings.TrimSpace(frame.Scene)
			}
			if action != "" && (len(actions) == 0 || actions[len(actions)-1] != action) {
				actions = append(actions, action)
			}
		}
		dialogue := cueTextBetween(cues, start, end)
		if dialogue == "" {
			dialogue = "无对白"
		}
		if len(actions) == 0 && dialogue == "无对白" {
			continue
		}
		if camera == "" {
			camera = "关键帧"
		}
		script.Units = append(script.Units, Unit{
			StartSec: start / 1000,
			EndSec:   int(math.Ceil(float64(end) / 1000)),
			Shots: []Take{{
				Camera:   camera,
				Action:   strings.Join(actions, "；"),
				Dialogue: dialogue,
			}},
		})
	}
	return script
}

type FrameFact struct {
	TMS    int
	Scene  string
	Action string
	Camera string
	Text   string
}

func FallbackScript(durationMS int, shots []Shot, cues []Cue) Script {
	script := Script{Style: "根据切点和字幕整理的证据稿。未经过视频理解模型补写。"}
	unitShots := make([]Take, 0, len(shots))
	for _, shot := range shots {
		dialogue := cueTextBetween(cues, shot.StartMS, shot.EndMS)
		if dialogue == "" {
			dialogue = "无对白"
		}
		unitShots = append(unitShots, Take{
			Camera:   "原片切点",
			Action:   fmt.Sprintf("镜头 %d，%s–%s", shot.Index, formatClock(shot.StartMS), formatClock(shot.EndMS)),
			Dialogue: dialogue,
		})
	}
	if len(unitShots) > 0 {
		script.Units = []Unit{{StartSec: 0, EndSec: int(math.Ceil(float64(durationMS) / 1000)), Shots: unitShots}}
	}
	return script
}

func cueTextBetween(cues []Cue, start, end int) string {
	var parts []string
	for _, cue := range cues {
		if cue.EndMS < start || cue.StartMS > end {
			continue
		}
		text := strings.TrimSpace(cue.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func joinShot(shot Take) string {
	parts := make([]string, 0, 6)
	if shot.Camera != "" {
		parts = append(parts, shot.Camera)
	}
	if shot.Action != "" {
		parts = append(parts, shot.Action)
	}
	if shot.Dialogue != "" && shot.Dialogue != "无对白" {
		label := "台词"
		if shot.Speaker != "" {
			label = shot.Speaker
		}
		if shot.Emotion != "" {
			label += "（" + shot.Emotion + "）"
		}
		parts = append(parts, label+"："+shot.Dialogue)
	} else {
		parts = append(parts, "无对白")
	}
	if shot.Continuity != "" {
		parts = append(parts, "连续性："+shot.Continuity)
	}
	if shot.KeyframePrompt != "" {
		parts = append(parts, "关键帧："+shot.KeyframePrompt)
	}
	if shot.VideoPrompt != "" {
		parts = append(parts, "视频提示词："+shot.VideoPrompt)
	}
	return strings.Join(parts, "。")
}

func formatClock(ms int) string {
	if ms < 0 {
		ms = 0
	}
	total := ms / 1000
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func stringList(value interface{}) []string {
	var out []string
	for _, item := range sliceValue(value) {
		text := strings.TrimSpace(stringValue(item))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func sliceValue(value interface{}) []interface{} {
	items, _ := value.([]interface{})
	return items
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return text
}

func intValue(value interface{}) int {
	switch item := value.(type) {
	case float64:
		return int(item)
	case int:
		return item
	case json.Number:
		parsed, _ := item.Int64()
		return int(parsed)
	default:
		parsed, _ := strconv.Atoi(strings.TrimSpace(stringValue(value)))
		return parsed
	}
}
