package main

import (
	"testing"
	"time"
)

func TestWorkerCalculateChargeUsesGrossMargin(t *testing.T) {
	got, err := workerCalculateCharge(970, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderCost != 970 || got.UserCharge != 1000 || got.Commission != 30 {
		t.Fatalf("breakdown = %#v", got)
	}
}

func TestWorkerCalculateChargeRoundsUp(t *testing.T) {
	got, err := workerCalculateCharge(0.000001, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserCharge != 0.000002 || got.Commission != 0.000001 {
		t.Fatalf("breakdown = %#v", got)
	}
}

func TestWorkerRouteCostTimeMultiplier(t *testing.T) {
	rule := map[string]interface{}{
		"peak_multiplier":    2.0,
		"peak_ranges":        []interface{}{"09:00-12:00", "14:00-18:00"},
		"peak_weekdays_only": true,
	}
	beijing := time.FixedZone("CST", 8*60*60)
	// 2026-10-26 是周一。
	monday := func(hour, minute int) time.Time {
		return time.Date(2026, 10, 26, hour, minute, 0, 0, beijing)
	}
	if got := workerRouteCostTimeMultiplier(rule, monday(10, 0)); got != 2 {
		t.Fatalf("weekday peak = %v, want 2", got)
	}
	if got := workerRouteCostTimeMultiplier(rule, monday(12, 30)); got != 1 {
		t.Fatalf("weekday off-peak = %v, want 1", got)
	}
	if got := workerRouteCostTimeMultiplier(rule, time.Date(2026, 10, 31, 10, 0, 0, 0, beijing)); got != 1 {
		t.Fatalf("weekend = %v, want 1", got)
	}
	// UTC 01:00 = 北京 09:00，峰时。
	if got := workerRouteCostTimeMultiplier(rule, time.Date(2026, 10, 26, 1, 0, 0, 0, time.UTC)); got != 2 {
		t.Fatalf("utc conversion = %v, want 2", got)
	}
	if got := workerRouteCostTimeMultiplier(nil, monday(10, 0)); got != 1 {
		t.Fatalf("nil rule = %v, want 1", got)
	}
}
