package handler

import "testing"

func TestAnswersIndistinguishableDetectsBlindModel(t *testing.T) {
	// A model that cannot see the images returns the same non-colour answer for
	// a red and a blue probe — the case that made an earlier refusal-only check
	// report "inconclusive" for a model that confidently hallucinated "白色".
	results := []visionProbeResult{
		{Label: "红", Answer: "白色"},
		{Label: "蓝", Answer: "白色"},
	}
	if !answersIndistinguishable(results, visionProbes) {
		t.Fatal("identical non-colour answers for two different colours must be treated as blind")
	}
}

func TestAnswersIndistinguishableIgnoresGenuineAnswers(t *testing.T) {
	// Same answer, but it names one of the probe colours: the model is reading
	// the images (and simply got one wrong), so this must not be a hard verdict.
	results := []visionProbeResult{
		{Label: "红", Answer: "红色"},
		{Label: "蓝", Answer: "红色"},
	}
	if answersIndistinguishable(results, visionProbes) {
		t.Fatal("an answer naming a probe colour must not count as indistinguishable")
	}
}

func TestAnswersIndistinguishableRequiresIdenticalAnswers(t *testing.T) {
	results := []visionProbeResult{
		{Label: "红", Answer: "不确定"},
		{Label: "蓝", Answer: "无法判断"},
	}
	if answersIndistinguishable(results, visionProbes) {
		t.Fatal("different answers must not be treated as indistinguishable")
	}
	empty := []visionProbeResult{
		{Label: "红", Answer: ""},
		{Label: "蓝", Answer: ""},
	}
	if answersIndistinguishable(empty, visionProbes) {
		t.Fatal("empty answers must not produce a verdict")
	}
}

func TestLooksLikeRefusal(t *testing.T) {
	expected := []string{"红", "red"}
	if !looksLikeRefusal("抱歉，我无法查看您提供的图片。", expected) {
		t.Fatal("a short refusal must be detected")
	}
	if !looksLikeRefusal("I cannot see the image.", expected) {
		t.Fatal("an English refusal must be detected")
	}
	// An answer that names the colour is an answer, not a refusal, even when it
	// opens with an apology.
	if looksLikeRefusal("抱歉让您久等了。这张图片整体是红色，边缘略深，接近正红。", expected) {
		t.Fatal("a substantive answer must not be treated as a refusal")
	}
	if looksLikeRefusal("红色", expected) {
		t.Fatal("a plain colour answer must not be treated as a refusal")
	}
	if looksLikeRefusal("白色", expected) {
		t.Fatal("a wrong but non-refusing answer is a hallucination, not a refusal")
	}
}

func TestSolidColorPNGIsDecodable(t *testing.T) {
	data, err := solidColorPNG([3]uint8{220, 20, 20}, 32)
	if err != nil {
		t.Fatalf("solidColorPNG: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("probe image is empty")
	}
	if string(data[1:4]) != "PNG" {
		t.Fatalf("probe image is not a PNG: %q", data[:8])
	}
}
