package service

import (
	"context"
	"testing"
)

func TestEcomFactoryPlanCount(t *testing.T) {
	cases := []struct {
		name   string
		inputs map[string]interface{}
		want   int
	}{
		{"默认套装", map[string]interface{}{"mode": "suite", "selectedSuite": "launch"}, 5},
		{"详情套装+每类2张", map[string]interface{}{"mode": "suite", "selectedSuite": "detail", "countPerModule": 2}, 10},
		{"自选模块", map[string]interface{}{"mode": "suite", "selectedModuleIds": []interface{}{"main-square", "seed-cover"}}, 2},
		{"批量默认", map[string]interface{}{"mode": "batch", "batchCount": 5}, 5},
		{"批量超上限截断24", map[string]interface{}{"mode": "batch", "batchCount": 200}, 24},
		{"每类超3截断", map[string]interface{}{"mode": "suite", "selectedSuite": "launch", "countPerModule": 9}, 15},
		{"快速生图", map[string]interface{}{"mode": "quick", "quickPrompt": "x"}, 1},
	}
	for _, tc := range cases {
		got, err := ecomFactoryPlanCount(tc.inputs)
		if err != nil {
			t.Errorf("%s: 意外错误 %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: 张数 = %d, want %d", tc.name, got, tc.want)
		}
	}
	if _, err := ecomFactoryPlanCount(map[string]interface{}{"mode": "suite", "selectedSuite": "unknown-suite"}); err == nil {
		t.Error("未知套装且无模块时应报错")
	}
}

func TestValidateEcomFactoryRequest(t *testing.T) {
	ctx := context.Background()
	assets := []interface{}{map[string]interface{}{"url": "http://x/1.jpg"}}
	if err := validateEcomFactoryRequest(ctx, 1, map[string]interface{}{}, map[string]interface{}{
		"mode": "suite", "selectedSuite": "launch", "assets": assets,
	}); err != nil {
		t.Errorf("有图应通过: %v", err)
	}
	if err := validateEcomFactoryRequest(ctx, 1, map[string]interface{}{}, map[string]interface{}{
		"mode": "suite", "selectedSuite": "launch", "productName": "咖啡机",
	}); err != nil {
		t.Errorf("有名称应通过: %v", err)
	}
	if err := validateEcomFactoryRequest(ctx, 1, map[string]interface{}{}, map[string]interface{}{
		"mode": "suite", "selectedSuite": "launch",
	}); err == nil {
		t.Error("无图无信息应拒绝")
	}
	if err := validateEcomFactoryRequest(ctx, 1, map[string]interface{}{}, map[string]interface{}{
		"mode": "batch",
	}); err == nil {
		t.Error("batch 缺提示词应拒绝")
	}
	err := validateEcomFactoryRequest(ctx, 1, map[string]interface{}{}, map[string]interface{}{
		"mode": "suite", "selectedModuleIds": []interface{}{"unknown-module"}, "assets": assets,
	})
	if err == nil {
		t.Error("未知模块应拒绝")
	}
	err = validateEcomFactoryRequest(ctx, 1, map[string]interface{}{"supported_resolutions": []interface{}{"1K", "2K"}}, map[string]interface{}{
		"mode": "suite", "selectedSuite": "launch", "assets": assets, "resolution": "4K",
	})
	if err == nil {
		t.Error("不支持的清晰度应拒绝")
	}
}

func TestNormalizeEcomFactoryInputs(t *testing.T) {
	inputs := normalizeEcomFactoryInputs(map[string]interface{}{}, map[string]interface{}{"productName": "LA-2"})
	for key, want := range map[string]string{
		"mode": "suite", "platform": "淘宝/天猫", "language": "简体中文",
		"visualStyle": "高端商业摄影", "resolution": "1K", "selectedSuite": "launch",
	} {
		if got := stringValue(inputs[key]); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if v, _ := inputs["copyEnabled"].(bool); v {
		t.Error("copyEnabled 默认应为 false（与参考站一致）")
	}
	if inputs["countPerModule"] != 1 {
		t.Errorf("countPerModule 默认 1, got %v", inputs["countPerModule"])
	}
}

func TestSanitizeEcomFileName(t *testing.T) {
	cases := map[string]string{
		"LA-2": "LA-2", `a/b:c*d?"e<f>g|h`: "a_b_c_d__e_f_g_h",
		"商品 图  名": "商品-图-名", "": "image",
	}
	for in, want := range cases {
		if got := sanitizeEcomFileName(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}
