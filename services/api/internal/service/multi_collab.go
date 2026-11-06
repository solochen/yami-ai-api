package service

// MultiCollabModelCode is the pseudo model code that fronts multi-model
// collaboration. It has no upstream connection of its own; requests are fanned
// out to the answer models configured in a channel preset and then fused by a
// summary model.
const MultiCollabModelCode = "multi_collab_chat"

// MultiCollabCategory is the model category reserved for collaboration entries.
const MultiCollabCategory = "multi_collab"

// IsMultiCollab reports whether a model is the multi-model collaboration entry.
//
// It intentionally accepts both the dedicated category and the pseudo-model
// code. The entry was originally seeded with category="chat"
// (see infra/migrations/010_multi_collab.up.sql) and databases that have not
// been migrated yet still carry that value, so checking only the category
// silently degrades collaboration requests into single-model requests.
func IsMultiCollab(category, code string) bool {
	return category == MultiCollabCategory || code == MultiCollabModelCode
}

// IsMultiCollabModel is the *ModelFull flavour of IsMultiCollab.
func IsMultiCollabModel(m *ModelFull) bool {
	if m == nil {
		return false
	}
	return IsMultiCollab(m.Category, m.Code)
}

// IsMultiCollabMember reports whether a model may participate in a
// collaboration channel as an answer or summary model. The collaboration entry
// itself is never a valid member.
func IsMultiCollabMember(m *ModelFull) bool {
	if m == nil || m.Category != "chat" {
		return false
	}
	return !IsMultiCollabModel(m)
}
