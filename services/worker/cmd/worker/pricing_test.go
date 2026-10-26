package main

import "testing"

func TestWorkerCalculateChargeUsesGrossMargin(t *testing.T) {
	got, err := workerCalculateCharge(970, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderCost != 970 || got.UserCharge != 1000 || got.Commission != 30 {
		t.Fatalf("breakdown = %#v", got)
	}
}

func TestWorkerCalculateChargeRoundsUp(t *testing.T) {
	got, err := workerCalculateCharge(0.000001, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserCharge != 0.000002 || got.Commission != 0.000001 {
		t.Fatalf("breakdown = %#v", got)
	}
}
