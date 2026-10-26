package service

import (
	"strconv"
	"strings"
	"time"
)

// beijingLocation 返回北京时间时区。上游的峰谷时段（如腾讯云 DeepSeek 的
// 工作日 9:00-12:00、14:00-18:00）按北京时间判定，而容器通常跑在 UTC，
// 这里用固定偏移避免依赖镜像里的 tzdata。
func beijingLocation() *time.Location {
	return time.FixedZone("CST", 8*60*60)
}

// routeCostTimeMultiplier 返回线路成本在当前时刻的时段倍数。
//
// 上游存在按时段调价的模型（例如腾讯云 DeepSeek 的峰谷计价：闲时 1/4、
// 峰时 2/8），而平台的成本模型是静态的，直接写死任何一个价都会有一半时间
// 算错。cost_rule 里可这样声明：
//
//	"peak_multiplier": 2,
//	"peak_ranges": ["09:00-12:00", "14:00-18:00"],
//	"peak_weekdays_only": true
//
// 未配置或不在时段内返回 1。
func routeCostTimeMultiplier(rule map[string]interface{}, now time.Time) float64 {
	if rule == nil {
		return 1
	}
	multiplier := floatValue(rule["peak_multiplier"])
	if multiplier <= 0 || multiplier == 1 {
		return 1
	}
	ranges := clockRangesFromRule(rule["peak_ranges"])
	if len(ranges) == 0 {
		return 1
	}
	local := now.In(beijingLocation())
	weekdaysOnly := true
	if flag, ok := rule["peak_weekdays_only"].(bool); ok {
		weekdaysOnly = flag
	}
	if weekdaysOnly && (local.Weekday() == time.Saturday || local.Weekday() == time.Sunday) {
		return 1
	}
	minutes := local.Hour()*60 + local.Minute()
	for _, window := range ranges {
		if minutes >= window[0] && minutes < window[1] {
			return multiplier
		}
	}
	return 1
}

// clockRangesFromRule 解析 ["09:00-12:00", ...] 形式的时段配置，返回分钟区间。
func clockRangesFromRule(raw interface{}) [][2]int {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	windows := make([][2]int, 0, len(items))
	for _, item := range items {
		start, end, ok := parseClockRange(stringValue(item))
		if ok {
			windows = append(windows, [2]int{start, end})
		}
	}
	return windows
}

// parseClockRange 把 "09:00-12:00" 解析成分钟区间 [540, 720]。
func parseClockRange(raw string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, okStart := parseClockMinutes(parts[0])
	end, okEnd := parseClockMinutes(parts[1])
	if !okStart || !okEnd || end <= start {
		return 0, 0, false
	}
	return start, end, true
}

func parseClockMinutes(raw string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || hour < 0 || hour > 24 {
		return 0, false
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}
