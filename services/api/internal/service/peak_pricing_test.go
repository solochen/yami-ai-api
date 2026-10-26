package service

import (
	"testing"
	"time"
)

func TestRouteCostTimeMultiplier(t *testing.T) {
	rule := map[string]interface{}{
		"peak_multiplier":    2.0,
		"peak_ranges":        []interface{}{"09:00-12:00", "14:00-18:00"},
		"peak_weekdays_only": true,
	}
	beijing := beijingLocation()
	at := func(hour, minute int, weekday time.Weekday) time.Time {
		// 2026-10-26 是周一；用固定日期构造指定星期几的时刻（北京时间）。
		base := time.Date(2026, 10, 26, hour, minute, 0, 0, beijing)
		offset := (int(weekday) - int(time.Monday) + 7) % 7
		return base.AddDate(0, 0, offset)
	}

	cases := []struct {
		name string
		when time.Time
		want float64
	}{
		{"工作日峰时-上午", at(9, 0, time.Monday), 2},
		{"工作日峰时-上午中段", at(11, 59, time.Monday), 2},
		{"工作日闲时-午间", at(12, 0, time.Monday), 1},
		{"工作日峰时-下午", at(14, 0, time.Tuesday), 2},
		{"工作日闲时-下班后", at(18, 0, time.Wednesday), 1},
		{"工作日闲时-夜间", at(3, 0, time.Thursday), 1},
		{"周末全天闲时", at(10, 0, time.Saturday), 1},
		{"周日全天闲时", at(15, 0, time.Sunday), 1},
	}
	for _, c := range cases {
		if got := routeCostTimeMultiplier(rule, c.when); got != c.want {
			t.Fatalf("%s: multiplier = %v, want %v", c.name, got, c.want)
		}
	}

	if got := routeCostTimeMultiplier(nil, at(10, 0, time.Monday)); got != 1 {
		t.Fatalf("nil rule multiplier = %v", got)
	}
	if got := routeCostTimeMultiplier(map[string]interface{}{}, at(10, 0, time.Monday)); got != 1 {
		t.Fatalf("empty rule multiplier = %v", got)
	}
	// 未配置倍率时即使落在时段内也不放大。
	if got := routeCostTimeMultiplier(map[string]interface{}{"peak_ranges": []interface{}{"09:00-12:00"}}, at(10, 0, time.Monday)); got != 1 {
		t.Fatalf("missing multiplier = %v", got)
	}
	// 未配置时段时不应放大。
	if got := routeCostTimeMultiplier(map[string]interface{}{"peak_multiplier": 2.0}, at(10, 0, time.Monday)); got != 1 {
		t.Fatalf("missing ranges = %v", got)
	}
	// 关闭工作日限制后周末也按峰时计。
	weekendRule := map[string]interface{}{
		"peak_multiplier":    2.0,
		"peak_ranges":        []interface{}{"09:00-12:00"},
		"peak_weekdays_only": false,
	}
	if got := routeCostTimeMultiplier(weekendRule, at(10, 0, time.Saturday)); got != 2 {
		t.Fatalf("weekend allowed multiplier = %v", got)
	}
	// 容器在 UTC 时也应正确落到北京时间（UTC 01:00 = 北京 09:00）。
	if got := routeCostTimeMultiplier(rule, time.Date(2026, 10, 26, 1, 0, 0, 0, time.UTC)); got != 2 {
		t.Fatalf("utc conversion multiplier = %v", got)
	}
}

func TestParseClockRange(t *testing.T) {
	if start, end, ok := parseClockRange("09:00-12:00"); !ok || start != 540 || end != 720 {
		t.Fatalf("range = %d/%d/%v", start, end, ok)
	}
	for _, bad := range []string{"", "09:00", "12:00-09:00", "aa-bb", "25:00-26:00"} {
		if _, _, ok := parseClockRange(bad); ok {
			t.Fatalf("invalid range accepted: %q", bad)
		}
	}
}
