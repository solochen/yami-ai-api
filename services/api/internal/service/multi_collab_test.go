package service

import "testing"

func TestApplyMultiChatEstimateDefaultsNarrowsReservation(t *testing.T) {
	params := map[string]interface{}{}
	applyMultiChatEstimateDefaults(params)
	if got := firstPositiveIntValue(params, "_estimated_output_tokens"); got != multiChatDefaultOutputTokens {
		t.Fatalf("_estimated_output_tokens = %d, want %d", got, multiChatDefaultOutputTokens)
	}
}

func TestApplyMultiChatEstimateDefaultsRespectsExplicitBudget(t *testing.T) {
	params := map[string]interface{}{"max_tokens": float64(4096)}
	applyMultiChatEstimateDefaults(params)
	if _, exists := params["_estimated_output_tokens"]; exists {
		t.Fatal("explicit max_tokens must win over the collaboration default")
	}
}

func TestApplyMultiChatEstimateDefaultsKeepsCallerEstimate(t *testing.T) {
	params := map[string]interface{}{"_estimated_output_tokens": float64(900)}
	applyMultiChatEstimateDefaults(params)
	if got := firstPositiveIntValue(params, "_estimated_output_tokens"); got != 900 {
		t.Fatalf("_estimated_output_tokens = %d, want 900", got)
	}
}

func TestIsMultiCollabAcceptsLegacyCategory(t *testing.T) {
	// The pseudo model was seeded with category='chat'; detection must not
	// depend on the migration having run.
	if !IsMultiCollab("chat", "multi_collab_chat") {
		t.Fatal("pseudo-model code must be recognised even with the legacy category")
	}
	if !IsMultiCollab("multi_collab", "some_other_code") {
		t.Fatal("multi_collab category must be recognised")
	}
	if IsMultiCollab("chat", "deepseek-v4-1-flash-1017") {
		t.Fatal("ordinary chat models must not be treated as collaboration entries")
	}
}

func TestIsMultiCollabMemberExcludesEntry(t *testing.T) {
	if IsMultiCollabMember(&ModelFull{ModelDTO: ModelDTO{Code: MultiCollabModelCode, Category: "chat"}}) {
		t.Fatal("the collaboration entry must not be usable as its own member")
	}
	if !IsMultiCollabMember(&ModelFull{ModelDTO: ModelDTO{Code: "deepseek-v4-1-flash-1017", Category: "chat"}}) {
		t.Fatal("an enabled chat model must be a valid member")
	}
	if IsMultiCollabMember(&ModelFull{ModelDTO: ModelDTO{Code: "seedream-5-0-pro", Category: "image"}}) {
		t.Fatal("non-chat models must not be valid members")
	}
	if IsMultiCollabMember(nil) {
		t.Fatal("nil model must not be a valid member")
	}
}
