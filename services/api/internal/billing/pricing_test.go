package billing

import (
	"errors"
	"math"
	"testing"
)

func TestCalculateChargeAppliesGrossMarginToProviderCost(t *testing.T) {
	got, err := CalculateCharge(970, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderCost != 970 || got.UserCharge != 1000 || got.Commission != 30 {
		t.Fatalf("breakdown = %#v", got)
	}
}

func TestCalculateChargeRoundsUpToProtectMargin(t *testing.T) {
	got, err := CalculateCharge(0.000001, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserCharge != 0.000002 || got.Commission != 0.000001 {
		t.Fatalf("breakdown = %#v", got)
	}
}

func TestCalculateChargeRejectsInvalidInputs(t *testing.T) {
	if _, err := CalculateCharge(math.NaN(), 3); err == nil {
		t.Fatal("NaN provider cost was accepted")
	}
	if _, err := CalculateCharge(1, 100); err == nil {
		t.Fatal("100% margin was accepted")
	}
}

func TestCalculateChargeAllowsFreeComplimentaryCost(t *testing.T) {
	got, err := CalculateCharge(0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserCharge != 0 || got.Commission != 0 {
		t.Fatalf("breakdown = %#v", got)
	}
}

func TestProviderCostMissingErrorIsStable(t *testing.T) {
	if !errors.Is(ErrProviderCostMissing, ErrProviderCostMissing) {
		t.Fatal("sentinel changed")
	}
}
