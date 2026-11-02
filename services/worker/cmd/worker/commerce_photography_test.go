package main

import (
	"strings"
	"testing"
)

// 校对后的提示词必须继续带商业摄影执行参数（镜头/布光/材质），
// 否则出图会退回平涂、塑料感和廉价模板风。
func TestResolvedCommercePromptKeepsPhotographySpec(t *testing.T) {
	inputs := map[string]interface{}{
		"creative_scene":     "main_image",
		"_commerce_resolved": true,
	}
	got := agentPromptWithScene("深灰色金属音频隔离器主图，干净背景", inputs)
	if !strings.Contains(got, "PHOTOGRAPHY SPEC") {
		t.Fatalf("resolved prompt lost photography spec: %s", got)
	}
	for _, expected := range []string{"柔光箱", "材质", "纯白或极浅中性灰"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("photography spec missing %q: %s", expected, got)
		}
	}
}

// 详情模块是后期排版用底图，不应注入单张主图的摄影规格。
func TestDetailSceneHasNoMainImagePhotographySpec(t *testing.T) {
	if spec := commercePhotographySpec("detail_image"); spec != "" {
		t.Fatalf("detail scene should use module prompt instead of main image spec: %s", spec)
	}
}
