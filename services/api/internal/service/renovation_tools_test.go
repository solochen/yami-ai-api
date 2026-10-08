package service

import (
	"context"
	"encoding/json"
	"testing"
)

func runTool(t *testing.T, name, args string) map[string]interface{} {
	t.Helper()
	s := &RenovationService{}
	out, err := s.ExecuteRenovationTool(context.Background(), name, args, "")
	if err != nil {
		t.Fatalf("tool %s infra error: %v", name, err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("tool %s result not json: %v (%s)", name, err, out)
	}
	return m
}

func TestCalcTile(t *testing.T) {
	m := runTool(t, "calc_material_quantity",
		`{"material_type":"tile","room_name":"客厅","room_length":5.2,"room_width":3.8,"piece_area":0.36,"loss_rate":0.05}`)
	if e, ok := m["error"]; ok {
		t.Fatalf("unexpected error: %v", e)
	}
	// ceil(19.76 * 1.05 / 0.36) = ceil(57.63) = 58
	if m["quantity"] != float64(58) {
		t.Fatalf("tile pieces = %v, want 58", m["quantity"])
	}
	if m["unit"] != "片" {
		t.Fatalf("unit = %v", m["unit"])
	}
}

func TestCalcFloor(t *testing.T) {
	m := runTool(t, "calc_material_quantity",
		`{"material_type":"floor","room_name":"主卧","room_length":4.2,"room_width":3.5}`)
	if m["quantity"] != float64(16) { // ceil(14.7 * 1.05) = ceil(15.435)
		t.Fatalf("floor m2 = %v, want 16", m["quantity"])
	}
}

func TestCalcPaint(t *testing.T) {
	m := runTool(t, "calc_material_quantity",
		`{"material_type":"paint","room_name":"客厅","room_length":5.2,"room_width":3.8,"room_height":2.7,"coats":2,"include_ceiling":true}`)
	// wall = (5.2+3.8)*2*2.7*0.85 = 41.31; +ceiling 19.76 = 61.07; liters = 61.07*2*0.115 = 14.05; buckets = ceil(2.81) = 3
	if m["quantity"] != float64(3) {
		t.Fatalf("paint buckets = %v, want 3 (paint_area=%v liters=%v)", m["quantity"], m["paint_area"], m["liters"])
	}
}

func TestCalcWallpaper(t *testing.T) {
	m := runTool(t, "calc_material_quantity",
		`{"material_type":"wallpaper","room_length":4,"room_width":3.5,"room_height":2.8}`)
	// wall = (4+3.5)*2*2.8*0.85 = 35.7; rolls = ceil(35.7*1.12/5.3) = ceil(7.54) = 8
	if m["quantity"] != float64(8) {
		t.Fatalf("wallpaper rolls = %v, want 8", m["quantity"])
	}
}

func TestCalcTileMissingPieceArea(t *testing.T) {
	m := runTool(t, "calc_material_quantity",
		`{"material_type":"tile","room_length":5.2,"room_width":3.8}`)
	if _, ok := m["error"]; !ok {
		t.Fatalf("expected error for missing piece_area, got %v", m)
	}
}

func TestCalcBadDimensions(t *testing.T) {
	m := runTool(t, "calc_material_quantity",
		`{"material_type":"tile","room_length":0,"room_width":3.8,"piece_area":0.36}`)
	if _, ok := m["error"]; !ok {
		t.Fatalf("expected error for zero length, got %v", m)
	}
}

func TestKeywordSearchRanksRelevantTopic(t *testing.T) {
	kb := &RenovationKB{}
	kb.entries = []kbEntry{
		{topic: "奶油风", tags: []string{"风格", "奶油风"}, content: "奶油风以奶白燕麦浅驼等暖色构成，适合小户型"},
		{topic: "工业风", tags: []string{"风格"}, content: "水泥灰墙面黑钢框架"},
	}
	hits := kb.keywordSearch("奶油风适合小户型吗", 2)
	if len(hits) == 0 || hits[0].Topic != "奶油风" {
		t.Fatalf("top hit = %+v, want 奶油风", hits)
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float64{1, 0, 1}
	if s := cosineSimilarity(a, a); s < 0.999 {
		t.Fatalf("cos(a,a) = %v, want 1", s)
	}
	if s := cosineSimilarity(a, []float64{0, 1, 0}); s != 0 {
		t.Fatalf("cos(orthogonal) = %v, want 0", s)
	}
}

func TestGenerateReportValidation(t *testing.T) {
	m := runTool(t, "generate_report", `{"title":"x","markdown_content":"短"}`)
	if _, ok := m["error"]; !ok {
		t.Fatalf("expected validation error for short content, got %v", m)
	}
	m = runTool(t, "generate_report", `{"title":"89平预算","markdown_content":"`+longText(120)+`"}`)
	if m["delivered"] != true {
		t.Fatalf("expected delivered=true, got %v", m)
	}
	if fn, _ := m["filename"].(string); fn == "" || fn[len(fn)-3:] != ".md" {
		t.Fatalf("filename = %v, want .md", m["filename"])
	}
}

func longText(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = '字'
	}
	return string(b)
}
