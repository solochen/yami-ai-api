package service

import "testing"

func TestPrepareSubtitleRemovalInputsCanonicalizesRegionAndAudio(t *testing.T) {
	inputs := map[string]interface{}{
		"video_url":       "https://cdn.example.com/a.mp4",
		"subtitle_region": "full_frame",
		"subtitle_mode":   "auto",
	}
	if err := prepareSubtitleRemovalInputs(map[string]interface{}{"max_input_duration_sec": 30, "preserve_audio": false}, inputs); err != nil {
		t.Fatal(err)
	}
	if inputs["subtitle_region"] != "full" || inputs["subtitle_mode"] != "auto" || inputs["preserve_audio"] != false || inputs["protect_watermark"] != true {
		t.Fatalf("unexpected normalized inputs: %#v", inputs)
	}
}

func TestPrepareSubtitleRemovalInputsRejectsLongVideoAndBadRegion(t *testing.T) {
	long := map[string]interface{}{"video_url": "https://cdn.example.com/a.mp4", "duration_sec": "40"}
	if err := prepareSubtitleRemovalInputs(map[string]interface{}{"max_input_duration_sec": 30}, long); err == nil {
		t.Fatal("expected duration rejection")
	}
	badRegion := map[string]interface{}{"video_url": "https://cdn.example.com/a.mp4", "subtitle_region": "middle"}
	if err := prepareSubtitleRemovalInputs(nil, badRegion); err == nil {
		t.Fatal("expected region rejection")
	}
	badMode := map[string]interface{}{"video_url": "https://cdn.example.com/a.mp4", "subtitle_mode": "magic"}
	if err := prepareSubtitleRemovalInputs(nil, badMode); err == nil {
		t.Fatal("expected mode rejection")
	}
}

func TestSubtitleRemovalReservesModelOnlyForHardcoded(t *testing.T) {
	if subtitleRemovalReservesModelCost("auto") || subtitleRemovalReservesModelCost("soft_track") || subtitleRemovalReservesModelCost("full_frame") {
		t.Fatal("only explicit hard-subtitle repair should reserve model cost up front")
	}
	if !subtitleRemovalReservesModelCost("hardcoded_ai") {
		t.Fatal("hardcoded repair should reserve model cost")
	}
}

func TestMetadataDurationSeconds(t *testing.T) {
	if got := metadataDurationSeconds([]byte(`{"duration_seconds":"12.5"}`)); got != 12.5 {
		t.Fatalf("duration=%v", got)
	}
}
