package breakdown

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type ProductionLocation struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type ProductionShot struct {
	ID             string   `json:"id"`
	StartSec       float64  `json:"start_sec"`
	EndSec         float64  `json:"end_sec"`
	DurationSec    float64  `json:"duration_sec"`
	LocationCode   string   `json:"location_code"`
	Scene          string   `json:"scene"`
	CharacterCodes []string `json:"character_codes"`
	Camera         string   `json:"camera"`
	Action         string   `json:"action"`
	Speaker        string   `json:"speaker,omitempty"`
	Emotion        string   `json:"emotion,omitempty"`
	Dialogue       string   `json:"dialogue"`
	Continuity     string   `json:"continuity"`
	KeyframePrompt string   `json:"keyframe_prompt"`
	VideoPrompt    string   `json:"video_prompt"`
	NegativePrompt string   `json:"negative_prompt"`
	EvidenceStart  int      `json:"evidence_start_ms"`
	EvidenceEnd    int      `json:"evidence_end_ms"`
}

type QualityReport struct {
	Status          string   `json:"status"`
	CoveragePercent float64  `json:"coverage_percent"`
	MaxGapSec       float64  `json:"max_gap_sec"`
	Issues          []string `json:"issues"`
}

type ProductionPackage struct {
	Version      string               `json:"version"`
	AspectRatio  string               `json:"aspect_ratio"`
	DurationSec  float64              `json:"duration_sec"`
	Style        string               `json:"style"`
	Characters   []Character          `json:"characters"`
	Locations    []ProductionLocation `json:"locations"`
	Shots        []ProductionShot     `json:"shots"`
	Quality      QualityReport        `json:"quality"`
	SourcePolicy string               `json:"source_policy"`
}

func MergeScripts(parts []Script) Script {
	merged := Script{}
	styles, scenes, people, conflicts := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, part := range parts {
		if style := strings.TrimSpace(part.Style); style != "" && !styles[style] {
			styles[style] = true
			if merged.Style == "" {
				merged.Style = style
			}
		}
		for _, scene := range part.Scenes {
			scene = strings.TrimSpace(scene)
			if scene != "" && !scenes[scene] {
				scenes[scene] = true
				merged.Scenes = append(merged.Scenes, scene)
			}
		}
		for _, person := range part.Characters {
			key := strings.ToLower(strings.TrimSpace(person.Name))
			if key == "" {
				key = strings.ToLower(strings.TrimSpace(person.Appearance))
			}
			if key != "" && !people[key] {
				people[key] = true
				merged.Characters = append(merged.Characters, person)
			}
		}
		merged.Units = append(merged.Units, part.Units...)
		for _, conflict := range part.Conflicts {
			conflict = strings.TrimSpace(conflict)
			if conflict != "" && !conflicts[conflict] {
				conflicts[conflict] = true
				merged.Conflicts = append(merged.Conflicts, conflict)
			}
		}
	}
	sort.SliceStable(merged.Units, func(i, j int) bool {
		if merged.Units[i].StartSec == merged.Units[j].StartSec {
			return merged.Units[i].EndSec < merged.Units[j].EndSec
		}
		return merged.Units[i].StartSec < merged.Units[j].StartSec
	})
	for i := range merged.Characters {
		merged.Characters[i].Code = fmt.Sprintf("CHAR_%02d", i+1)
	}
	return merged
}

func BuildProductionPackage(script Script, durationMS int) ProductionPackage {
	duration := math.Max(1, float64(durationMS)/1000)
	pkg := ProductionPackage{
		Version:      "breakdown-production-v2",
		AspectRatio:  "9:16",
		DurationSec:  roundTenths(duration),
		Style:        strings.TrimSpace(script.Style),
		Characters:   append([]Character(nil), script.Characters...),
		SourcePolicy: "只使用原片时间码、画面、对白与可验证动作；未知信息留空，不补写原片不存在的剧情。",
	}
	if len(pkg.Characters) == 0 {
		pkg.Characters = inferCharacters(script)
	}
	knownCharacters := map[string]bool{}
	for _, person := range pkg.Characters {
		knownCharacters[strings.ToLower(strings.TrimSpace(person.Name))] = true
	}
	for _, unit := range script.Units {
		for _, take := range unit.Shots {
			for _, name := range append(append([]string(nil), take.Characters...), take.Speaker) {
				name = strings.TrimSpace(name)
				key := strings.ToLower(name)
				if name == "" || name == "无" || knownCharacters[key] {
					continue
				}
				knownCharacters[key] = true
				pkg.Characters = append(pkg.Characters, Character{Name: name, Appearance: "以原片关键帧中的脸型、发型、服装与配色为准"})
			}
		}
	}
	for i := range pkg.Characters {
		pkg.Characters[i].Code = fmt.Sprintf("CHAR_%02d", i+1)
	}
	locationCode := map[string]string{}
	allScenes := append([]string(nil), script.Scenes...)
	for _, unit := range script.Units {
		allScenes = append(allScenes, unit.Scene)
	}
	for _, scene := range allScenes {
		scene = strings.TrimSpace(scene)
		if scene == "" || locationCode[scene] != "" {
			continue
		}
		code := fmt.Sprintf("LOC_%02d", len(pkg.Locations)+1)
		locationCode[scene] = code
		pkg.Locations = append(pkg.Locations, ProductionLocation{Code: code, Description: scene})
	}
	characterCode := map[string]string{}
	for _, person := range pkg.Characters {
		characterCode[strings.ToLower(strings.TrimSpace(person.Name))] = person.Code
	}
	shotIndex := 0
	previousState := ""
	for _, unit := range script.Units {
		if len(unit.Shots) == 0 || unit.EndSec <= unit.StartSec {
			continue
		}
		span := float64(unit.EndSec - unit.StartSec)
		perShot := span / float64(len(unit.Shots))
		for index, take := range unit.Shots {
			start := float64(unit.StartSec) + float64(index)*perShot
			end := float64(unit.StartSec) + float64(index+1)*perShot
			if end > duration {
				end = duration
			}
			if end <= start {
				continue
			}
			shotIndex++
			codes := make([]string, 0, len(take.Characters))
			for _, name := range take.Characters {
				if code := characterCode[strings.ToLower(strings.TrimSpace(name))]; code != "" {
					codes = append(codes, code)
				}
			}
			continuity := strings.TrimSpace(take.Continuity)
			if continuity == "" && previousState != "" {
				continuity = "承接上一镜终态：" + previousState
			} else if continuity == "" {
				continuity = "首镜按原片证据建立人物外观、服装、道具和空间方向"
			}
			keyframe := strings.TrimSpace(take.KeyframePrompt)
			if keyframe == "" {
				keyframe = compactPrompt(unit.Scene, strings.Join(take.Characters, "、"), take.Camera, "动作起始瞬间："+take.Action)
			}
			videoPrompt := strings.TrimSpace(take.VideoPrompt)
			if videoPrompt == "" {
				videoPrompt = compactPrompt(fmt.Sprintf("%.1f秒竖屏单镜头", end-start), unit.Scene, take.Camera, take.Action, dialoguePrompt(take))
			}
			evidenceStart, evidenceEnd := take.EvidenceStart, take.EvidenceEnd
			if evidenceStart <= 0 && start > 0 {
				evidenceStart = int(math.Round(start * 1000))
			}
			if evidenceEnd <= evidenceStart {
				evidenceEnd = int(math.Round(end * 1000))
			}
			pkg.Shots = append(pkg.Shots, ProductionShot{
				ID: fmt.Sprintf("SHOT_%03d", shotIndex), StartSec: roundTenths(start), EndSec: roundTenths(end), DurationSec: roundTenths(end - start),
				LocationCode: locationCode[unit.Scene], Scene: unit.Scene, CharacterCodes: codes, Camera: take.Camera, Action: take.Action,
				Speaker: take.Speaker, Emotion: take.Emotion, Dialogue: take.Dialogue, Continuity: continuity,
				KeyframePrompt: keyframe, VideoPrompt: videoPrompt,
				NegativePrompt: "不要新增人物、场景、对白或文字水印；保持人物身份、服装、道具、空间方向与上一镜连续。",
				EvidenceStart:  evidenceStart, EvidenceEnd: evidenceEnd,
			})
			previousState = strings.TrimSpace(take.Action)
		}
	}
	pkg.Quality = ValidateProductionPackage(pkg)
	return pkg
}

func ValidateProductionPackage(pkg ProductionPackage) QualityReport {
	report := QualityReport{Status: "passed", Issues: []string{}}
	if strings.TrimSpace(pkg.Style) == "" {
		report.Issues = append(report.Issues, "缺少统一视频风格")
	}
	if len(pkg.Locations) == 0 {
		report.Issues = append(report.Issues, "缺少场景资产")
	}
	if len(pkg.Shots) == 0 {
		report.Issues = append(report.Issues, "没有生成可生产的单镜头")
		report.Status = "failed"
		return report
	}
	sorted := append([]ProductionShot(nil), pkg.Shots...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].StartSec < sorted[j].StartSec })
	cursor, covered, maxGap := 0.0, 0.0, 0.0
	for _, shot := range sorted {
		if shot.StartSec > cursor {
			gap := shot.StartSec - cursor
			if gap > maxGap {
				maxGap = gap
			}
		}
		start := math.Max(cursor, shot.StartSec)
		if shot.EndSec > start {
			covered += shot.EndSec - start
		}
		if shot.EndSec > cursor {
			cursor = shot.EndSec
		}
		if (shot.DurationSec < 2 && pkg.DurationSec >= 2) || shot.DurationSec > 9 {
			report.Issues = append(report.Issues, fmt.Sprintf("%s 时长 %.1f 秒不适合单镜生成", shot.ID, shot.DurationSec))
		}
		if strings.TrimSpace(shot.Scene) == "" || strings.TrimSpace(shot.LocationCode) == "" || strings.TrimSpace(shot.Action) == "" || strings.TrimSpace(shot.VideoPrompt) == "" || strings.TrimSpace(shot.KeyframePrompt) == "" || strings.TrimSpace(shot.Continuity) == "" {
			report.Issues = append(report.Issues, shot.ID+" 缺少动作、关键帧或视频提示词")
		}
		if shot.EvidenceEnd <= shot.EvidenceStart || shot.EvidenceStart < 0 {
			report.Issues = append(report.Issues, shot.ID+" 缺少有效原片时间证据")
		}
		if strings.TrimSpace(shot.Dialogue) != "" && shot.Dialogue != "无对白" && strings.TrimSpace(shot.Speaker) == "" {
			report.Issues = append(report.Issues, shot.ID+" 有对白但缺少说话人")
		}
	}
	if pkg.DurationSec > cursor {
		gap := pkg.DurationSec - cursor
		if gap > maxGap {
			maxGap = gap
		}
	}
	report.MaxGapSec = roundTenths(maxGap)
	report.CoveragePercent = math.Min(100, math.Round(covered/math.Max(pkg.DurationSec, 1)*1000)/10)
	if report.CoveragePercent < 99.0 {
		report.Issues = append(report.Issues, fmt.Sprintf("时间线覆盖率仅 %.1f%%", report.CoveragePercent))
	}
	if report.MaxGapSec > 2 {
		report.Issues = append(report.Issues, fmt.Sprintf("时间线最大缺口 %.1f 秒", report.MaxGapSec))
	}
	if len(report.Issues) > 0 {
		report.Status = "failed"
	}
	return report
}

func RenderProductionMarkdown(pkg ProductionPackage) string {
	var b strings.Builder
	b.WriteString("【生产规格】\n")
	b.WriteString(fmt.Sprintf("- 版本：%s\n- 画幅：%s\n- 总时长：%.1f 秒\n- 覆盖率：%.1f%%\n\n", pkg.Version, pkg.AspectRatio, pkg.DurationSec, pkg.Quality.CoveragePercent))
	b.WriteString("【角色资产】\n")
	for _, person := range pkg.Characters {
		b.WriteString(fmt.Sprintf("- %s %s：%s\n", person.Code, person.Name, person.Appearance))
	}
	b.WriteString("\n【场景资产】\n")
	for _, location := range pkg.Locations {
		b.WriteString(fmt.Sprintf("- %s：%s\n", location.Code, location.Description))
	}
	b.WriteString("\n【可生成镜头】\n")
	for _, shot := range pkg.Shots {
		b.WriteString(fmt.Sprintf("\n### %s｜%.1fs–%.1fs｜%.1fs\n", shot.ID, shot.StartSec, shot.EndSec, shot.DurationSec))
		b.WriteString("- 画面：" + compactPrompt(shot.Scene, shot.Camera, shot.Action) + "\n")
		if shot.Dialogue != "" && shot.Dialogue != "无对白" {
			b.WriteString(fmt.Sprintf("- 对白：%s（%s）：“%s”\n", shot.Speaker, shot.Emotion, shot.Dialogue))
		} else {
			b.WriteString("- 对白：无对白\n")
		}
		b.WriteString("- 关键帧提示词：" + shot.KeyframePrompt + "\n")
		b.WriteString("- 视频提示词：" + shot.VideoPrompt + "\n")
		b.WriteString("- 连续性：" + shot.Continuity + "\n")
		b.WriteString(fmt.Sprintf("- 原片证据：%.3fs–%.3fs\n", float64(shot.EvidenceStart)/1000, float64(shot.EvidenceEnd)/1000))
	}
	if len(pkg.Quality.Issues) > 0 {
		b.WriteString("\n【质量门禁】\n")
		for _, issue := range pkg.Quality.Issues {
			b.WriteString("- " + issue + "\n")
		}
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func inferCharacters(script Script) []Character {
	seen := map[string]bool{}
	var out []Character
	for _, unit := range script.Units {
		for _, shot := range unit.Shots {
			for _, name := range append(append([]string(nil), shot.Characters...), shot.Speaker) {
				name = strings.TrimSpace(name)
				if name == "" || name == "未知" || seen[name] {
					continue
				}
				seen[name] = true
				out = append(out, Character{Name: name, Appearance: "以原片关键帧中的脸型、发型、服装与配色为准"})
			}
		}
	}
	return out
}

func compactPrompt(parts ...string) string {
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, "；")
}

func dialoguePrompt(take Take) string {
	if take.Dialogue == "" || take.Dialogue == "无对白" {
		return "无对白，只保留环境音和动作音"
	}
	return compactPrompt(take.Speaker+"以"+take.Emotion+"情绪说", "“"+take.Dialogue+"”", "保持口型同步")
}

func roundTenths(value float64) float64 {
	return math.Round(value*10) / 10
}
