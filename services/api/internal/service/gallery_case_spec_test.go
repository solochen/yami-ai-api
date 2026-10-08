package service

import "testing"

func TestDetectGalleryRiskFlagsIgnoresNegativePrompt(t *testing.T) {
	input := GalleryCaseSpecInput{
		Title:          "Original future school architecture",
		Purpose:        "A neutral product photography concept",
		Prompt:         "Create an original school architecture composition with soft studio light. 不出现真实品牌。",
		NegativePrompt: "brand logos, public figures, copyrighted characters, watermark",
	}
	if flags := detectGalleryRiskFlags(input); len(flags) != 0 {
		t.Fatalf("negative prompt created risk flags: %v", flags)
	}
}

func TestRiskReviewIsRequiredForDetectedFlags(t *testing.T) {
	input := normalizeGalleryCaseSpecDefaults(GalleryCaseSpecInput{
		Title:            "Nike shoe concept",
		Prompt:           "A product concept featuring Nike branding",
		RiskReviewStatus: "not_required",
	})
	if input.RiskReviewStatus != "pending" {
		t.Fatalf("risk review status = %q, want pending", input.RiskReviewStatus)
	}
	validationErrors := validateGalleryCaseSpec(input)
	found := false
	for _, item := range validationErrors {
		if item == "风险标记未完成人工审核" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("validation errors did not require risk review: %v", validationErrors)
	}
}

func TestGalleryCaseSpecUpdateArgsKeepUpdatedBy(t *testing.T) {
	input := normalizeGalleryCaseSpecDefaults(GalleryCaseSpecInput{
		Status:             "draft",
		Title:              "Test specification",
		Purpose:            "Test purpose",
		Prompt:             "Create a neutral test image",
		PromptLanguage:     "en",
		Category:           "Creative",
		Styles:             []string{"Photography"},
		Scenes:             []string{"Creative"},
		AspectRatio:        "1:1",
		OutputMediaType:    "image",
		CandidateModels:    []GalleryCaseSpecModelCandidate{{Code: "image_fast_v1"}},
		AcceptanceCriteria: []string{"one", "two", "three"},
	})
	base := galleryCaseSpecArgs("", 0, input, nil, 42)
	updateArgs := append(base[2:34], int64(7))
	if got, want := len(updateArgs)+1, 34; got != want {
		t.Fatalf("gallery case spec update argument count = %d, want %d", got, want)
	}
	if got := updateArgs[len(updateArgs)-2]; got != int64(42) {
		t.Fatalf("updated_by argument = %#v, want 42", got)
	}
}
