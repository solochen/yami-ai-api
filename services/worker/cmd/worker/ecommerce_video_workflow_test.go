package main

import "testing"

func TestRenderTemplateKeepsLongerCommercePlaceholders(t *testing.T) {
	vars := map[string]string{}
	absorbNodeOutputVars(vars, "copy", map[string]interface{}{
		"text": `{"copy":"商品文案","image_prompt":"静态画面","video_prompt":"连续运动"}`,
	})
	if vars["copy"] == "" || vars["copy_image_prompt"] != "静态画面" || vars["copy_video_prompt"] != "连续运动" {
		t.Fatalf("vars: %#v", vars)
	}
	for i := 0; i < 30; i++ {
		if got := renderTemplate("商品主视觉。{{copy_image_prompt}}", vars); got != "商品主视觉。静态画面" {
			t.Fatalf("image prompt corrupted: %s", got)
		}
		if got := renderTemplate("商品展示片段。{{copy_video_prompt}}", vars); got != "商品展示片段。连续运动" {
			t.Fatalf("video prompt corrupted: %s", got)
		}
	}
}

func TestCommerceDeliveryConfirmationOverridesPlan(t *testing.T) {
	vars := map[string]string{
		"copy_copy":         "旧文案",
		"copy_image_prompt": "旧海报",
		"copy_video_prompt": "旧镜头",
	}
	outputs := map[string]interface{}{
		"confirmation_payload": map[string]interface{}{
			"prompt": formatCommerceDeliveryPlan("新文案", "新海报", "新镜头"),
		},
	}
	applyCommerceDeliveryConfirmation(vars, outputs)
	if vars["copy_copy"] != "新文案" || vars["copy_image_prompt"] != "新海报" || vars["copy_video_prompt"] != "新镜头" {
		t.Fatalf("confirmation ignored: %#v", vars)
	}
	plan := formatCommerceDeliveryPlan(vars["copy_copy"], vars["copy_image_prompt"], vars["copy_video_prompt"])
	copyText, imagePrompt, videoPrompt, ok := parseCommerceDeliveryPlan(plan)
	if !ok || copyText != "新文案" || imagePrompt != "新海报" || videoPrompt != "新镜头" {
		t.Fatalf("plan roundtrip: %q %q %q %v", copyText, imagePrompt, videoPrompt, ok)
	}
}

func TestPosterBecomesVideoFirstFrameAndImageCountStaysOne(t *testing.T) {
	inputs := map[string]interface{}{
		"count":            4,
		"n":                4,
		"duration":         5,
		"generation_mode":  "text",
		"reference_images": []string{"https://example.com/product.jpg"},
	}
	outputs := map[string]interface{}{
		"poster": map[string]interface{}{"image_url": "https://example.com/poster.jpg"},
	}
	imageInputs := prepareCustomMediaInputs(workflowNode{Type: "image"}, inputs, outputs)
	if imageInputs["count"] != 1 || imageInputs["n"] != 1 {
		t.Fatalf("poster count: %#v", imageInputs)
	}
	if _, ok := imageInputs["duration"]; ok {
		t.Fatal("image task kept video duration")
	}
	if refs := referenceImageURLs(imageInputs); len(refs) != 1 || refs[0] != "https://example.com/product.jpg" {
		t.Fatalf("product reference lost: %#v", imageInputs)
	}
	videoInputs := prepareCustomMediaInputs(workflowNode{Type: "video"}, inputs, outputs)
	if videoInputs["first_frame"] != "https://example.com/poster.jpg" || videoInputs["generation_mode"] != "first_frame" {
		t.Fatalf("poster was not used as first frame: %#v", videoInputs)
	}
	if videoInputs["count"] != 4 {
		t.Fatalf("video count changed: %#v", videoInputs["count"])
	}

	explicit := map[string]interface{}{
		"generation_mode": "first_frame",
		"first_frame":     "https://example.com/user-frame.jpg",
	}
	kept := prepareCustomMediaInputs(workflowNode{Type: "video"}, explicit, outputs)
	if kept["first_frame"] != "https://example.com/user-frame.jpg" {
		t.Fatalf("user first frame overwritten: %#v", kept["first_frame"])
	}
}
