package billing

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// ceilMicros mirrors service.numericMicros, the helper that used to re-round
// multi-model aggregates and produced up to 2e-6 of drift.
func ceilMicros(value float64) float64 {
	return math.Ceil(value*1_000_000) / 1_000_000
}

func multiCollabItems(rng *rand.Rand, count int) []SettlementItem {
	items := make([]SettlementItem, 0, count)
	for i := 0; i < count; i++ {
		providerMicros := int64(5 + rng.Intn(6000))
		userMicros := providerMicros + int64(rng.Intn(int(providerMicros)+1))
		items = append(items, SettlementItem{
			Source:       "route_cost",
			RefType:      "chat_model",
			RefID:        fmt.Sprintf("req-%d", i),
			ProviderCost: float64(providerMicros) / 1_000_000,
			UserCharge:   float64(userMicros) / 1_000_000,
			Commission:   float64(userMicros-providerMicros) / 1_000_000,
		})
	}
	return items
}

// TestValidateSettlementItemsAcceptsIntegerMicroAggregates locks in the fix for
// the multi-model collaboration settlement failure: aggregating items in
// integer micro-credits (instead of ceil-rounding the float64 sum) must always
// reconcile, no matter how the per-item values fall on micro boundaries.
func TestValidateSettlementItemsAcceptsIntegerMicroAggregates(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 200000; trial++ {
		items := multiCollabItems(rng, 3)
		providerMicros, userMicros, commissionMicros := int64(0), int64(0), int64(0)
		for _, item := range items {
			providerMicros += int64(math.Round(item.ProviderCost * 1_000_000))
			userMicros += int64(math.Round(item.UserCharge * 1_000_000))
			commissionMicros += int64(math.Round(item.Commission * 1_000_000))
		}
		settlement := Settlement{
			RefType: "chat", RefID: "req",
			ProviderCost: float64(providerMicros) / 1_000_000,
			UserCharge:   float64(userMicros) / 1_000_000,
			Commission:   float64(commissionMicros) / 1_000_000,
			Items:        items,
		}
		if err := validateSettlementItems(settlement); err != nil {
			t.Fatalf("trial %d: integer-micro aggregate rejected: %v", trial, err)
		}
	}
}

// TestValidateSettlementItemsToleratesOneMicroOfCeilDrift documents the slack
// that keeps ceil-rounded aggregates (agent / operational-guard paths) valid.
func TestValidateSettlementItemsToleratesOneMicroOfCeilDrift(t *testing.T) {
	items := multiCollabItems(rand.New(rand.NewSource(11)), 3)
	provider, user, commission := 0.0, 0.0, 0.0
	for _, item := range items {
		provider += item.ProviderCost
		user += item.UserCharge
		commission += item.Commission
	}
	settlement := Settlement{
		RefType: "chat", RefID: "req",
		ProviderCost: ceilMicros(provider),
		UserCharge:   ceilMicros(user),
		Commission:   ceilMicros(ceilMicros(user) - ceilMicros(provider)),
		Items:        items,
	}
	if err := validateSettlementItems(settlement); err != nil {
		t.Fatalf("one micro of ceil drift must be tolerated: %v", err)
	}
}

// TestValidateSettlementItemsStillRejectsRealMismatch guards against the
// tolerance being widened so far that genuine reconciliation errors slip
// through.
func TestValidateSettlementItemsStillRejectsRealMismatch(t *testing.T) {
	items := multiCollabItems(rand.New(rand.NewSource(13)), 3)
	settlement := Settlement{
		RefType: "chat", RefID: "req",
		ProviderCost: items[0].ProviderCost + items[1].ProviderCost + items[2].ProviderCost,
		UserCharge:   items[0].UserCharge + items[1].UserCharge + items[2].UserCharge + 0.01,
		Commission:   items[0].Commission + items[1].Commission + items[2].Commission,
		Items:        items,
	}
	if err := validateSettlementItems(settlement); err == nil {
		t.Fatal("expected a real mismatch to be rejected")
	}
}
